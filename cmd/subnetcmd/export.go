// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
)

var (
	exportOutput        string
	customVMRepoURL     string
	customVMBranch      string
	customVMBuildScript string
)

// avalanche subnet list
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [subnetName]",
		Short: "Export deployment details",
		Long: `The subnet export command write the details of an existing Subnet deploy to a file.

The command prompts for an output path. You can also provide one with
the --output flag.`,
		RunE:         exportSubnet,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	cmd.Flags().StringVarP(
		&exportOutput,
		"output",
		"o",
		"",
		"write the export data to the provided file path",
	)
	cmd.Flags().BoolVar(&deployMainnet, "mainnet", false, "export `mainnet` genesis")
	cmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "export `local` genesis")
	cmd.Flags().BoolVarP(&deployDevnet, "devnet", "d", false, "export `devnet` genesis")
	cmd.Flags().BoolVarP(&deployTestnet, "testnet", "t", false, "export `fuji` genesis")
	cmd.Flags().BoolVarP(&deployTestnet, "fuji", "f", false, "export `fuji` genesis")
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	return cmd
}

func CallExportSubnet(subnetName, exportPath string, network models.Network) error {
	switch network {
	case models.Mainnet:
		deployMainnet = true
	case models.Fuji:
		deployTestnet = true
	case models.Local:
		deployLocal = true
	case models.Devnet:
		deployDevnet = true
	}
	exportOutput = exportPath
	return exportSubnet(nil, []string{subnetName})
}

func exportSubnet(_ *cobra.Command, args []string) error {
	var err error
	if exportOutput == "" {
		pathPrompt := "Enter file path to write export data to"
		exportOutput, err = app.Prompt.CaptureString(pathPrompt)
		if err != nil {
			return err
		}
	}
	var network models.Network
	switch {
	case deployLocal:
		network = models.Local
	case deployDevnet:
		network = models.Devnet
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}
	if network == models.Undefined {
		networkStr, err := app.Prompt.CaptureList(
			"Choose which network's genesis to export",
			[]string{models.Local.String(), models.Devnet.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	subnetName := args[0]

	if !app.SidecarExists(subnetName) {
		return fmt.Errorf("invalid subnet %q", subnetName)
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	if sc.VM == models.CustomVM {
		if sc.CustomVMRepoURL == "" {
			ux.Logger.PrintToUser("Custom VM source code repository, branch and build script not defined for subnet. Filling in the details now.")
			if err := vm.SetCustomVMSourceCodeFields(app, &sc, customVMRepoURL, customVMBranch, customVMBuildScript); err != nil {
				return err
			}
			if err := app.UpdateSidecar(&sc); err != nil {
				return err
			}
		}
	}

	gen, err := app.LoadRawGenesis(subnetName, network)
	if err != nil {
		return err
	}

	var nodeConfig, chainConfig, subnetConfig, networkUpgrades []byte

	if app.AvagoNodeConfigExists(subnetName) {
		nodeConfig, err = app.LoadRawAvagoNodeConfig(subnetName)
		if err != nil {
			return err
		}
	}
	if app.ChainConfigExists(subnetName) {
		chainConfig, err = app.LoadRawChainConfig(subnetName)
		if err != nil {
			return err
		}
	}
	if app.AvagoSubnetConfigExists(subnetName) {
		subnetConfig, err = app.LoadRawAvagoSubnetConfig(subnetName)
		if err != nil {
			return err
		}
	}
	if app.NetworkUpgradeExists(subnetName) {
		networkUpgrades, err = app.LoadRawNetworkUpgrades(subnetName)
		if err != nil {
			return err
		}
	}

	exportData := models.Exportable{
		Sidecar:         sc,
		Genesis:         gen,
		NodeConfig:      nodeConfig,
		ChainConfig:     chainConfig,
		SubnetConfig:    subnetConfig,
		NetworkUpgrades: networkUpgrades,
	}

	exportBytes, err := json.Marshal(exportData)
	if err != nil {
		return err
	}
	return os.WriteFile(exportOutput, exportBytes, constants.WriteReadReadPerms)
}
