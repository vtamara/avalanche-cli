// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"slices"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	instanceType string
	diskSizeGB   int
	diskType     string
)

func newResizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resize <clusterName> [--instance-type <instanceType>] [--disk-size-gb <diskSizeGB>] [--disk-type <diskType>]",
		Short: "(ALPHA Warning) resize the nodes in the cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node resize command changes the instance type, disk size, and disk type of the nodes in the cluster.`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE:         resize,
	}
	cmd.Flags().StringVar(&instanceType, "instance-type", "", "The new instance type for the nodes in the cluster")
	cmd.Flags().IntVar(&diskSizeGB, "disk-size-gb", 0, "The new disk size in GB for the nodes in the cluster")
	cmd.Flags().StringVar(&diskType, "disk-type", "", "The new disk type for the nodes in the cluster")
	return cmd
}

func resize(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if instanceType == "" && diskSizeGB == 0 && diskType == "" {
		return fmt.Errorf("at least one of --instance-type, --disk-size-gb, or --disk-type must be provided")
	}
	//get cloudIDs for the cluster
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			ux.Logger.RedXToUser("Failed to parse node %s due to %s", node, err.Error())
			return err
		}
		//resize disk first
		if diskSizeGB > 0 || diskType != "" {
			if err := resizeDisk(nodeConfig.CloudService, nodeConfig.Region, nodeConfig.NodeID, diskSizeGB, diskType); err != nil {
				return err
			}
		}
	}
	return nil
}

func resizeAWSDisk(cloudID string, diskSizeGB int, diskType string) error {
	return nil
}

func resizeGCPDisk(cloudID string, diskSizeGB int, diskType string) error {
	return nil
}

func resizeDisk(cloudName, region, cloudID string, diskSizeGB int, diskType string) error {
	if !checkDiskTypeSupported(cloudName, diskType) {
		return fmt.Errorf("disk type %s is not supported for cloud %s", diskType, cloudName)
	}
	switch cloudName {
	case constants.GCPCloudService:
		return resizeGCPDisk(cloudID, diskSizeGB, diskType)
	case constants.AWSCloudService:
		return resizeAWSDisk(cloudID, diskSizeGB, diskType)
	default:
		return fmt.Errorf("cloud %s is not supported", cloudName)
	}
}

func checkDiskTypeSupported(cloudName string, diskType string) bool {
	switch cloudName {
	case constants.AWSCloudService:
		return slices.Contains(awsAPI.SupportedVolumeTypes, diskType)
	case constants.GCPCloudService:
		return slices.Contains(gcpAPI.SupportedVolumeTypes, diskType)
	default:
		return false
	}
}
