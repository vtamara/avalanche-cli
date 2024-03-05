// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/onsi/gomega"
)

const e2eKeyPairName = "runner-avalanche-cli-keypair"

func NodeCreate(network, version string, numNodes int, separateMonitoring bool) string {
	home, err := os.UserHomeDir()
	gomega.Expect(err).Should(gomega.BeNil())
	_, err = os.Open(filepath.Join(home, ".ssh", e2eKeyPairName))
	gomega.Expect(err).Should(gomega.BeNil())
	_, err = os.Open(filepath.Join(home, ".ssh", e2eKeyPairName+".pub"))
	gomega.Expect(err).Should(gomega.BeNil())
	cmdVersion := "--latest-avalanchego-version=true"
	if version != "latest" && version != "" {
		cmdVersion = "--custom-avalanchego-version=" + version
	}
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"create",
		constants.E2EClusterName,
		"--use-static-ip=false",
		cmdVersion,
		"--separate-monitoring-instance="+strconv.FormatBool(separateMonitoring),
		"--region=local",
		"--num-nodes="+strconv.Itoa(numNodes),
		"--"+network,
		"--node-type=docker",
	)
	return runCmd(cmd)
}

func NodeDevnet(numNodes int) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"create",
		constants.E2EClusterName,
		"--use-static-ip=false",
		"--latest-avalanchego-version=true",
		"--region=local",
		"--num-nodes="+strconv.Itoa(numNodes),
		"--devnet",
		"--node-type=docker",
	)
	return runCmd(cmd)
}

func NodeStatus() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"status",
		constants.E2EClusterName,
	)
	return runCmd(cmd)
}

func NodeSSH(name, command string) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"ssh",
		name,
		command,
	)
	return runCmd(cmd)
}

func ConfigMetrics() {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"config",
		"metrics",
		"disable",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

func NodeList() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"list",
	)
	return runCmd(cmd)
}

func NodeWhitelistSSH(sshPubKey string) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"whitelist",
		"ssh",
		constants.E2EClusterName,
		"\""+sshPubKey+"\"",
	)
	return runCmd(cmd)
}

func NodeUpgrade() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"upgrade",
		constants.E2EClusterName,
	)
	return runCmd(cmd)
}

type StaticConfig struct {
	Targets []string `yaml:"targets"`
}
type ScrapeConfig struct {
	JobName       string         `yaml:"job_name"`
	StaticConfigs []StaticConfig `yaml:"static_configs"`
}
type PrometheusConfig struct {
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs"`
}

// ParsePrometheusYamlConfig parses prometheus config YAML file installed in separate monitoring
// host in /etc/prometheus/prometheus.yml
func ParsePrometheusYamlConfig(filePath string) PrometheusConfig {
	data, err := os.ReadFile(filePath)
	gomega.Expect(err).Should(gomega.BeNil())
	var prometheusConfig PrometheusConfig
	err = yaml.Unmarshal(data, &prometheusConfig)
	gomega.Expect(err).Should(gomega.BeNil())
	return prometheusConfig
}

func runCmd(cmd *exec.Cmd) string {
	cmd.Env = os.Environ()
	fmt.Println("About to run: " + cmd.String())
	output, err := cmd.Output()
	fmt.Println("---------------->")
	fmt.Println(string(output))
	fmt.Println("---------------->")
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}
