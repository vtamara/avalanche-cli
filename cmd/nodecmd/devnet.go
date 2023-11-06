// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	coreth_params "github.com/ava-labs/coreth/params"
	"github.com/spf13/cobra"
)

func newDevnetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devnet [clusterName]",
		Short: "(ALPHA Warning) Moves all nodes of a cluster into a new devnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node devnet command moves all nodes of a cluster into a new devnet.
`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         intoDevnet,
	}

	return cmd
}

// difference between unlock schedule locktime and startime in original genesis
const genesisLocktimeStartimeDelta = 2836800

//go:embed genesis_template.json
var genesisTemplateBytes []byte

func intoDevnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]

	if !flags.EnsureMutuallyExclusive([]bool{useLedger, useEwoq, keyName != ""}) {
		return subnetcmd.ErrMutuallyExlusiveKeySource
	}

	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := setupAnsible(clusterName); err != nil {
		return err
	}
	ansibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	ansibleHosts, err := ansible.GetHostMapfromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	cloudHostIDs, err := utils.MapWithError(ansibleHostIDs, func(s string) (string, error) { _, o, err := models.HostAnsibleIDToCloudID(s); return o, err })
	if err != nil {
		return err
	}
	nodeIDs, err := utils.MapWithError(cloudHostIDs, func(s string) (string, error) {
		n, err := getNodeID(app.GetNodeInstanceDirPath(s))
		return n.String(), err
	})
	if err != nil {
		return err
	}

	useEwoq := true
	network := models.DevnetNetwork
	network.Endpoint = "http://" + ansibleHosts[ansibleHostIDs[0]].IP + ":9650"

	ux.Logger.PrintToUser("Devnet Network Id: %d", network.Id)
	ux.Logger.PrintToUser("Devnet Endpoint: %s", network.Endpoint)

	k, err := key.NewSoft(network.Id)
	if err != nil {
		return err
	}
	stakingAddrStr := k.X()[0]

	kc, err := subnetcmd.GetKeychain(useEwoq, useLedger, nil, keyName, network)
	if err != nil {
		return err
	}
	walletAddr := kc.Addresses().List()[0]
	hrp := key.GetHRP(network.Id)
	walletAddrStr, err := address.Format("X", hrp, walletAddr[:])
	if err != nil {
		return err
	}

	genesisBytes, err := generateCustomGenesis(network.Id, walletAddrStr, stakingAddrStr, nodeIDs)
	if err != nil {
		return err
	}
	for _, cloudHostID := range cloudHostIDs {
		outFile := filepath.Join(app.GetNodeInstanceDirPath(cloudHostID), "genesis.json")
		if err := os.WriteFile(outFile, genesisBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
	}
	bootstrapIPs := []string{}
	bootstrapIDs := []string{}
	for i, ansibleHostID := range ansibleHostIDs {
		cloudHostID := cloudHostIDs[i]
		confMap := map[string]interface{}{}
		confMap[config.NetworkNameKey] = network.Id
		confMap[config.PublicIPKey] = ansibleHosts[ansibleHostID].IP
		confMap[config.BootstrapIDsKey] = strings.Join(bootstrapIDs, ",")
		confMap[config.BootstrapIPsKey] = strings.Join(bootstrapIPs, ",")
		confMap[config.GenesisFileKey] = "/home/ubuntu/.avalanchego/configs/genesis.json"
		confMap[config.HTTPHostKey] = ""
		bootstrapIDs = append(bootstrapIDs, nodeIDs[i])
		bootstrapIPs = append(bootstrapIPs, ansibleHosts[ansibleHostID].IP+":9651")
		confBytes, err := json.MarshalIndent(confMap, "", " ")
		if err != nil {
			return err
		}
		outFile := filepath.Join(app.GetNodeInstanceDirPath(cloudHostID), "node.json")
		if err := os.WriteFile(outFile, confBytes, constants.WriteReadReadPerms); err != nil {
			return err
		}
	}
	if err := ansible.RunAnsiblePlaybookSetupDevnet(
		app.GetAnsibleDir(),
		strings.Join(ansibleHostIDs, ","),
		app.GetNodesDir(),
		app.GetAnsibleInventoryDirPath(clusterName),
	); err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	clustersConfig.Clusters[clusterName] = models.ClusterConfig{
		Network: network,
		Nodes:   clusterConfig.Nodes,
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func generateCustomGenesis(networkID uint32, walletAddr string, stakingAddr string, nodeIDs []string) ([]byte, error) {
	genesisMap := map[string]interface{}{}
	if err := json.Unmarshal(genesisTemplateBytes, &genesisMap); err != nil {
		return nil, err
	}
	cChainGenesis := genesisMap["cChainGenesis"]
	cChainGenesisMap, ok := cChainGenesis.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected field 'cChainGenesis' of genesisMap to be a map[string]interface{}, but it failed with type %T", cChainGenesis)
	}
	cChainGenesisMap["config"] = coreth_params.AvalancheLocalChainConfig
	cChainGenesisBytes, err := json.Marshal(cChainGenesisMap)
	if err != nil {
		return nil, err
	}
	genesisMap["cChainGenesis"] = string(cChainGenesisBytes)
	genesisMap["networkID"] = networkID
	startTime := time.Now().Unix()
	genesisMap["startTime"] = startTime
	initialStakers := []map[string]interface{}{}
	for _, nodeID := range nodeIDs {
		initialStaker := map[string]interface{}{
			"nodeID":        nodeID,
			"rewardAddress": walletAddr,
			"delegationFee": 1000000,
		}
		initialStakers = append(initialStakers, initialStaker)
	}
	genesisMap["initialStakers"] = initialStakers
	lockTime := startTime + genesisLocktimeStartimeDelta
	allocations := []interface{}{}
	alloc := map[string]interface{}{
		"avaxAddr":      walletAddr,
		"ethAddr":       "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
		"initialAmount": 300000000000000000,
		"unlockSchedule": []interface{}{
			map[string]interface{}{"amount": 20000000000000000},
			map[string]interface{}{"amount": 10000000000000000, "locktime": lockTime},
		},
	}
	allocations = append(allocations, alloc)
	alloc = map[string]interface{}{
		"avaxAddr":      stakingAddr,
		"ethAddr":       "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
		"initialAmount": 0,
		"unlockSchedule": []interface{}{
			map[string]interface{}{"amount": 10000000000000000, "locktime": lockTime},
		},
	}
	allocations = append(allocations, alloc)
	genesisMap["allocations"] = allocations
	genesisMap["initialStakedFunds"] = []interface{}{
		stakingAddr,
	}

	updatedGenesis, err := json.MarshalIndent(genesisMap, "", " ")
	if err != nil {
		return nil, err
	}
	return updatedGenesis, nil
}
