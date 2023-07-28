// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/ava-labs/avalanchego/vms/platformvm"

	subnetcmd "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var (
	deployTestnet   bool
	deployMainnet   bool
	keyName         string
	subnetName      string
	useLedger       bool
	ledgerAddresses []string
	weight          uint64
	duration        time.Duration
	startTimeStr    string

	ErrMutuallyExlusiveKeyLedger = errors.New("--key and --ledger,--ledger-addrs are mutually exclusive")
	ErrStoredKeyOnMainnet        = errors.New("--key is not available for mainnet operations")
)

func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join [subnetName]",
		Short: "Join a subnet as a validator",
		Long: `The node join command enables a Primary Network Validator to also be a validator
of a Subnet. If The command is run before the node is bootstrapped on the Primary Network, 
the command will fail. You can check the bootstrap status by calling 
avalanche node status`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         joinSubnet,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is validating")

	return cmd
}

func runAnsiblePlaybookExportSubnet(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	exportOutput := "/tmp/" + subnetName + "-export.dat"
	exportedSubnet := "exportedSubnet=" + exportOutput
	cmd := exec.Command("ansible-playbook", "exportSubnet.yml", "-i", inventoryPath, "--extra-vars", exportedSubnet, "--ssh-extra-args='-o IdentitiesOnly=yes'")
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func runAnsiblePlaybookTrackSubnet(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	importedFileName := "/tmp/" + subnetName + "-export.dat"
	importedSubnet := "subnetExportFileName=" + importedFileName + " subnetName=" + subnetName
	cmd := exec.Command("ansible-playbook", "trackSubnet.yml", "-i", inventoryPath, "--extra-vars", importedSubnet, "--ssh-extra-args='-o IdentitiesOnly=yes'")
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func runAnsiblePlaybookCheckBootstrapped(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	cmd := exec.Command("ansible-playbook", "isBootstrapped.yml", "-i", inventoryPath, "--ssh-extra-args='-o IdentitiesOnly=yes'")
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func runAnsiblePlaybookGetNodeID(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	cmd := exec.Command("ansible-playbook", "getNodeID.yml", "-i", inventoryPath, "--ssh-extra-args='-o IdentitiesOnly=yes'")
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func createFile(fileName string) error {
	myfile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	myfile.Close()

	return nil
}
func removeFile(fileName string) error {
	if _, err := os.Stat(fileName); err == nil {
		err := os.Remove(fileName)
		if err != nil {
			return err
		}
	}
	return nil
}
func parseBootstrappedOutput(fileName string) (bool, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return false, err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	json.Unmarshal(byteValue, &result)
	isBootstrappedInterface, ok := result["result"].(map[string]interface{})
	if ok {
		isBootstrapped, ok := isBootstrappedInterface["isBootstrapped"].(bool)
		if ok {
			return isBootstrapped, nil
		}
	}
	return false, nil
}

func parseNodeIDOutput(fileName string) (string, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	json.Unmarshal(byteValue, &result)
	nodeIDInterface, ok := result["result"].(map[string]interface{})
	if ok {
		nodeID, ok := nodeIDInterface["nodeID"].(string)
		if ok {
			return nodeID, nil
		}
	}
	return "", nil
}

func trackSubnet(clusterName string, network models.Network) error {
	err := subnetcmd.CallExportSubnet(subnetName, network)
	if err != nil {
		return err
	}
	inventoryPath := "inventories/" + clusterName
	err = runAnsiblePlaybookExportSubnet(inventoryPath)
	if err != nil {
		return err
	}
	err = runAnsiblePlaybookTrackSubnet(inventoryPath)
	if err != nil {
		return err
	}

	return nil
}

func addNodeAsSubnetValidator(nodeID string, network models.Network) error {
	err := subnetcmd.CallAddValidator(subnetName, nodeID, network)
	if err != nil {
		return err
	}
	return nil
}
func getMinStakingAmount(network models.Network) (uint64, error) {
	var apiURL string
	switch network {
	case models.Mainnet:
		apiURL = constants.MainnetAPIEndpoint
	case models.Fuji:
		apiURL = constants.FujiAPIEndpoint
	}
	pClient := platformvm.NewClient(apiURL)
	ctx, cancel := context.WithTimeout(context.Background(), constants.E2ERequestTimeout)
	defer cancel()
	minValStake, _, err := pClient.GetMinStake(ctx, ids.Empty)
	if err != nil {
		return 0, err
	}
	return minValStake, nil
}
func validatePrimaryNetwork(nodeID ids.NodeID, network models.Network) error {
	var (
		start time.Time
		err   error
	)
	switch {
	case deployTestnet:
		network = models.Fuji
	case deployMainnet:
		network = models.Mainnet
	}
	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	switch network {
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, "pay transaction fees", app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}
	minValStake, err := getMinStakingAmount(network)
	if err != nil {
		return err
	}
	if weight == 0 {
		weight, err = promptWeightPrimaryNetwork(network)
		if err != nil {
			return err
		}
	} else if weight < minValStake {
		return fmt.Errorf("illegal weight, must be greater than or equal to %d: %d", minValStake, weight)
	}
	start, duration, err = getTimeParametersPrimaryNetwork(network)
	if err != nil {
		return err
	}

	kc, err := subnetcmd.GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}
	recipientAddr := kc.Addresses().List()[0]
	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	_, _, err = deployer.AddValidatorPrimaryNetwork(nodeID, weight, start, duration, recipientAddr, uint32(20000))
	if err != nil {
		return err
	}
	return nil
}
func promptWeightPrimaryNetwork(network models.Network) (uint64, error) {
	defaultStake := constants.DefaultFujiPrimaryNetworkWeight
	if network == models.Mainnet {
		defaultStake = constants.DefaultMainnetPrimaryNetworkWeight
	}
	defaultWeight := fmt.Sprintf("Default (%d)", defaultStake)
	txt := "What stake weight would you like to assign to the validator?"
	weightOptions := []string{defaultWeight, "Custom"}

	weightOption, err := app.Prompt.CaptureList(txt, weightOptions)
	if err != nil {
		return 0, err
	}

	switch weightOption {
	case defaultWeight:
		return uint64(defaultStake), nil
	default:
		return app.Prompt.CaptureWeight(txt)
	}
}

func getTimeParametersPrimaryNetwork(network models.Network) (time.Time, time.Duration, error) {
	const (
		//defaultStartOption    = "Start in one minute"
		defaultDurationOption = "Minimum staking duration on primary network"
		custom                = "Custom"
	)
	start := time.Now().Add(constants.StakingStartLeadTime)
	if duration == 0 {
		msg := "How long should your validator validate for?"
		durationOptions := []string{defaultDurationOption, custom}
		durationOption, err := app.Prompt.CaptureList(msg, durationOptions)
		if err != nil {
			return time.Time{}, 0, err
		}

		switch durationOption {
		case defaultDurationOption:
			duration, err = getDefaultMaxValidationTime(start, network)
			if err != nil {
				return time.Time{}, 0, err
			}
		default:
			duration, err = subnetcmd.PromptDuration(start, network)
			if err != nil {
				return time.Time{}, 0, err
			}
		}
	}
	return start, duration, nil
}
func getDefaultMaxValidationTime(start time.Time, network models.Network) (time.Duration, error) {
	durationStr := constants.DefaultFujiStakeDuration
	if network == models.Mainnet {
		durationStr = constants.DefaultMainnetStakeDuration
	}
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, err
	}
	end := start.Add(d)
	confirm := fmt.Sprintf("Your validator will finish staking by %s", end.Format(constants.TimeParseLayout))
	yes, err := app.Prompt.CaptureYesNo(confirm)
	if err != nil {
		return 0, err
	}
	if !yes {
		return 0, errors.New("you have to confirm staking duration")
	}
	return d, nil
}
func checkNodeIsBootstrapped(clusterName string) (bool, error) {
	fileName := "isBootstrapped.json"
	err := createFile(fileName)
	if err != nil {
		return false, err
	}
	inventoryPath := "inventories/" + clusterName
	if err := runAnsiblePlaybookCheckBootstrapped(inventoryPath); err != nil {
		return false, err
	}
	isBootstrapped, err := parseBootstrappedOutput(fileName)
	if err != nil {
		return false, err
	}
	err = removeFile(fileName)
	if err != nil {
		return false, err
	}
	if isBootstrapped {
		return true, nil
	}
	ux.Logger.PrintToUser("Node is not bootstrapped yet, please check again later.")
	return false, nil
}

func getNodeID(clusterName string) (string, error) {
	fileName := "nodeID.json"
	err := createFile(fileName)
	if err != nil {
		return "", err
	}
	inventoryPath := "inventories/" + clusterName
	if err := runAnsiblePlaybookGetNodeID(inventoryPath); err != nil {
		return "", err
	}
	nodeID, err := parseNodeIDOutput(fileName)
	if err != nil {
		return "", err
	}
	err = removeFile(fileName)
	if err != nil {
		return "", err
	}
	return nodeID, err
}
func joinSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if subnetName == "" {
		ux.Logger.PrintToUser("Please provide the name of the subnet that the node will be validating with --subnet flag")
		return errors.New("no subnet provided")
	}
	_, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
	if err != nil {
		return err
	}
	isBootstrapped, err := checkNodeIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	if !isBootstrapped {
		return errors.New("node is not bootstrapped yet, please try again later")
	}
	nodeIDStr, err := getNodeID(clusterName)
	if err != nil {
		return err
	}
	nodeID, err := ids.NodeIDFromString(nodeIDStr)
	if err != nil {
		return err
	}
	err = validatePrimaryNetwork(nodeID, models.Fuji)
	if err != nil {
		return err
	}
	err = trackSubnet(clusterName, models.Fuji)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Waiting 1 min for the node to be a Primary Network Validator...")
	time.Sleep(60 * time.Second)
	ux.Logger.PrintToUser("Adding the node as a Subnet Validator...")
	err = addNodeAsSubnetValidator(nodeIDStr, models.Fuji)
	if err != nil {
		return err
	}
	return nil
}