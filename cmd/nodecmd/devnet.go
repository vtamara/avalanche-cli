// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
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

//go:embed ewoq_key.pk
var ewoqKeyBytes []byte

func intoDevnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
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
	networkID := uint32(constants.LocalNetworkID)
	k, err := key.LoadSoftFromBytes(networkID, ewoqKeyBytes)
	if err != nil {
		return err
	}
	walletAddr := k.X()[0]
	k, err = key.NewSoft(networkID)
	if err != nil {
		return err
	}
	stakingAddr := k.X()[0]
	return generateCustomGenesis(networkID, walletAddr, stakingAddr, nodeIDs, "pepe.json")
}

func generateCustomGenesis(networkID uint32, walletAddr string, stakingAddr string, nodeIDs []string, outPath string) error {
	genesisMap := map[string]interface{}{}
	if err := json.Unmarshal(genesisTemplateBytes, &genesisMap); err != nil {
		return err
	}
	cChainGenesis := genesisMap["cChainGenesis"]
	cChainGenesisMap, ok := cChainGenesis.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected field 'cChainGenesis' of genesisMap to be a map[string]interface{}, but it failed with type %T", cChainGenesis)
	}
	cChainGenesisMap["config"] = coreth_params.AvalancheLocalChainConfig
	cChainGenesisBytes, err := json.Marshal(cChainGenesisMap)
	if err != nil {
		return err
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
		return err
	}
	return os.WriteFile(outPath, updatedGenesis, constants.DefaultPerms755)
}
