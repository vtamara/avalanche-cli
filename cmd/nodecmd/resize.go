// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/spf13/cobra"
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
	return cmd
}

func resize(_ *cobra.Command, args []string) error {
	return nil
}
