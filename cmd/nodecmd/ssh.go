// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [clusterName] [cmd]",
		Short: "(ALPHA Warning) Execute ssh command on node/s",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node ssh command execute a given command using ssh on all nodes in the cluster.
If no command is given, just prints the ssh cmdLine to be used to connect to each node.
`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE:         sshNode,
	}

	return cmd
}

func sshNode(_ *cobra.Command, args []string) error {
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
	ansibleHosts, err := ansible.GetHostMapfromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	for _, host := range ansibleHostIDs {
		_, cloudID, err := models.HostAnsibleIDToCloudID(host)
		if err != nil {
			return err
		}
		cmdLine := utils.GetSSHConnectionString(ansibleHosts[host].IP, ansibleHosts[host].SSHPrivateKeyPath) + " " + strings.Join(args[1:], " ")
		ux.Logger.PrintToUser("[%s] %s", cloudID, cmdLine)
		if len(args) > 1 {
			splitCmdLine := strings.Split(cmdLine, " ")
			cmd := exec.Command(splitCmdLine[0], splitCmdLine[1:]...)
			_, _ = utils.SetupRealtimeCLIOutput(cmd, true, true)
			err = cmd.Run()
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser("")
		}
	}
	return nil
}
