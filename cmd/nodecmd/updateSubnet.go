// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newUpdateSubnetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subnet [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Update nodes in a cluster with latest subnet configuration and VM for custom VM",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node update subnet command updates all nodes in a cluster with latest Subnet configuration and VM for custom VM.
You can check the updated subnet bootstrap status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         updateSubnet,
	}

	return cmd
}

func updateSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	incompatibleNodes, err := checkAvalancheGoVersionCompatible(clusterName, subnetName)
	if err != nil {
		return err
	}
	if len(incompatibleNodes) > 0 {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your VM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		switch sc.VM {
		case models.SubnetEvm:
			ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		case models.CustomVM:
			ux.Logger.PrintToUser("To modify your Custom VM binary: avalanche subnet upgrade vm %s --config", subnetName)
		}
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with VM RPC version of %s", incompatibleNodes, subnetName)
	}

	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host models.Host) {
			defer wg.Done()
			if err := host.Connect(constants.SSHPOSTTimeout); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			defer func() {
				if err := host.Disconnect(); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
				}
			}()
			if err := ssh.RunSSHSetupBuildEnv(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return fmt.Errorf("failed to get setup build env for node(s) %s", wgResults.GetErrorHosts())
	}
	nonUpdatedNodes, err := doUpdateSubnet(clusterName, subnetName, models.FujiNetwork)
	if err != nil {
		return err
	}
	if len(nonUpdatedNodes) > 0 {
		return fmt.Errorf("node(s) %s failed to be updated for subnet %s", nonUpdatedNodes, subnetName)
	}
	ux.Logger.PrintToUser("Node(s) successfully updated for Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet status with avalanche node status %s --subnet %s", clusterName, subnetName))
	return nil
}

// doUpdateSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// restart tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func doUpdateSubnet(clusterName, subnetName string, network models.Network) ([]string, error) {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	if err := subnetcmd.CallExportSubnet(subnetName, subnetPath, network); err != nil {
		return nil, err
	}
	if err := ansible.RunAnsiblePlaybookExportSubnet(app.GetAnsibleDir(), app.GetAnsibleInventoryDirPath(clusterName), subnetPath, "all"); err != nil {
		return nil, err
	}
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	nonUpdatedNodes := []string{}
	for _, host := range hostAliases {
		// runs avalanche update subnet command
		if err = ansible.RunAnsiblePlaybookUpdateSubnet(app.GetAnsibleDir(), subnetName, subnetPath, app.GetAnsibleInventoryDirPath(clusterName), host); err != nil {
			nonUpdatedNodes = append(nonUpdatedNodes, host)
		}
	}
	return nonUpdatedNodes, nil
}
