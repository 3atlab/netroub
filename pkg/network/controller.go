// Package network provides network emulation control for netroub.
package network

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/3atlab/netroub/pkg/runtime"
	"github.com/sirupsen/logrus"
)

// networkOpMu serializes containerlab deploy/destroy operations to avoid
// netlink race conditions when running multiple scenarios in parallel.
// The actual scenario execution (events) runs concurrently; only the
// network setup/teardown is serialized.
var networkOpMu sync.Mutex

// NetworkController manages containerlab network lifecycle and log collection.
// This enables testing without actual containerlab/Docker dependencies.
type NetworkController struct {
	Scenario *model.Scenario
	Devices  *model.Data
	LabName  string
	Runner   runtime.CommandRunner
}

// NewNetworkController creates a new NetworkController instance.
func NewNetworkController(scenario *model.Scenario, devices *model.Data, labName string, runner runtime.CommandRunner) *NetworkController {
	return &NetworkController{
		Scenario: scenario,
		Devices:  devices,
		LabName:  labName,
		Runner:   runner,
	}
}

// ClabHostName returns the containerlab container name for a host.
func (c *NetworkController) ClabHostName(host string) string {
	return "clab-" + c.LabName + "-" + host
}

// Deploy starts the containerlab network.
// Deploy/Destroy operations are serialized via networkOpMu to prevent
// netlink race conditions during parallel execution.
func (c *NetworkController) Deploy() error {
	// Get device count for subnet size calculation
	deviceCount := len(c.Devices.Nodes)
	if deviceCount == 0 {
		deviceCount = 254 // Default to /24 if no devices loaded
	}

	// Generate unique IPv4 subnet based on device count and lab index
	ipv4Subnet, err := generateSubnet(c.LabName, deviceCount)
	if err != nil {
		return fmt.Errorf("failed to allocate IPv4 subnet: %w", err)
	}

	// Generate unique IPv6 subnet for parallel execution
	ipv6Subnet, err := generateIPv6Subnet(c.LabName)
	if err != nil {
		return fmt.Errorf("failed to allocate IPv6 subnet: %w", err)
	}

	// Serialize containerlab deploy to avoid netlink race conditions
	networkOpMu.Lock()
	defer networkOpMu.Unlock()

	// Log after acquiring lock so log order reflects actual execution order
	logrus.Infof("Deploying network with lab name: %s", c.LabName)

	// Use unique network name for parallel execution
	networkName := "clab-" + c.LabName
	output, err := c.Runner.Run("sudo", "containerlab", "deploy",
		"--name", c.LabName,
		"--topo", c.Scenario.Topo,
		"--network", networkName,
		"--ipv4-subnet", ipv4Subnet,
		"--ipv6-subnet", ipv6Subnet)
	if err != nil {
		return fmt.Errorf("containerlab deploy failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}

	logrus.Debug(string(output))
	return nil
}

// Destroy stops and removes the containerlab network.
// Deploy/Destroy operations are serialized via networkOpMu to prevent
// netlink race conditions during parallel execution.
func (c *NetworkController) Destroy() error {
	// Serialize containerlab destroy to avoid netlink race conditions
	networkOpMu.Lock()
	defer networkOpMu.Unlock()

	// Log after acquiring lock so log order reflects actual execution order
	logrus.Infof("Destroying network with lab name: %s", c.LabName)

	// Use --name only (without --topo) to avoid containerlab trying to
	// create a clab instance with default network settings.
	// --cleanup ensures Docker network is also removed.
	output, err := c.Runner.Run("sudo", "containerlab", "destroy",
		"--name", c.LabName,
		"--cleanup")
	if err != nil {
		return fmt.Errorf("containerlab destroy failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}

	logrus.Debug(string(output))
	return nil
}

// SetupTcpdump sets up tcpdump on a host container.
func (c *NetworkController) SetupTcpdump(node string) error {
	containerName := c.ClabHostName(node)
	topoPath := c.findTopoPath() + "/" + node
	scriptPath := topoPath + "/tcpdump.sh"

	// Create directory if necessary
	if err := os.MkdirAll(topoPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", topoPath, err)
	}

	// Create the tcpdump.sh file
	file, err := os.Create(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to create tcpdump.sh: %w", err)
	}
	defer file.Close()

	// Change permissions
	if err := os.Chmod(scriptPath, 0775); err != nil {
		return fmt.Errorf("failed to chmod tcpdump.sh: %w", err)
	}

	// Create tcpdump directory in container (use absolute path for consistency)
	output, err := c.Runner.Run("sudo", "docker", "exec", "-d", containerName, "mkdir", "/tcpdump")
	if err != nil {
		return fmt.Errorf("failed to create tcpdump directory in container: %w, output: %s", err, string(output))
	}
	logrus.Debugf("Created tcpdump directory in %s", containerName)

	// Write script header
	if _, err := file.WriteString("#!/bin/sh \n"); err != nil {
		return fmt.Errorf("failed to write tcpdump.sh: %w", err)
	}

	// Add tcpdump commands for each interface
	nodeIndex := c.getDeviceIndex(node)
	if nodeIndex < 0 {
		return fmt.Errorf("device %s not found", node)
	}
	for _, inter := range c.Devices.Nodes[nodeIndex].Interfaces {
		line := fmt.Sprintf("tcpdump -i %s -n -v > /tcpdump/tcpdump_%s.log & \n", inter.Name, inter.Name)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write tcpdump.sh: %w", err)
		}
	}

	// Copy script to container
	output, err = c.Runner.Run("sudo", "docker", "cp", scriptPath, containerName+":/")
	if err != nil {
		return fmt.Errorf("failed to copy tcpdump.sh to container: %w, output: %s", err, string(output))
	}
	logrus.Debugf("Copied tcpdump.sh to %s", containerName)

	// Run the script (use absolute path since working directory may vary by container image)
	output, err = c.Runner.Run("sudo", "docker", "exec", "-d", containerName, "/tcpdump.sh")
	if err != nil {
		return fmt.Errorf("failed to start tcpdump: %w, output: %s", err, string(output))
	}
	logrus.Debugf("Started tcpdump on %s", containerName)

	return nil
}

// CollectTcpdumpLogs copies tcpdump logs from containers to host.
func (c *NetworkController) CollectTcpdumpLogs() error {
	for _, node := range c.Scenario.Hosts {
		containerName := c.ClabHostName(node)
		dstPath := filepath.Join(c.findTopoPath(), node) + "/"

		output, err := c.Runner.Run("sudo", "docker", "cp", containerName+":/tcpdump", dstPath)
		if err != nil {
			return fmt.Errorf("failed to copy tcpdump directory from container %s to %s: %w, output: %s",
				containerName, dstPath, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

// MoveLogFiles moves collected log files to the scenario log directory.
// Deprecated: Use MoveLogFilesToDir instead for parallel execution.
func (c *NetworkController) MoveLogFiles(logFiles []string) error {
	// Create log directory if it does not exist
	if _, err := os.Stat(c.Scenario.LogPath); os.IsNotExist(err) {
		if err := os.Mkdir(c.Scenario.LogPath, os.ModePerm); err != nil {
			return err
		}
	}

	// Generate directory name with timestamp
	t := time.Now()
	dirName := t.Format("2006-01-02T15:04:05")

	scenarioLogPath := filepath.Join(c.Scenario.LogPath, c.Scenario.ScenarioName)
	if _, err := os.Stat(scenarioLogPath); os.IsNotExist(err) {
		if err := os.Mkdir(scenarioLogPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create scenario directory: %w", err)
		}
	}

	trialLogPath := filepath.Join(scenarioLogPath, dirName)
	if err := os.Mkdir(trialLogPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create trial directory: %w", err)
	}

	topoPath := c.findTopoPath()

	// Copy log files
	for _, path := range logFiles {
		relativePath, err := filepath.Rel(topoPath, path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(trialLogPath, relativePath)

		if err := os.MkdirAll(filepath.Dir(newPath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create device directory: %w", err)
		}

		if err := copyFile(path, newPath); err != nil {
			return err
		}
		logrus.Debugf("Moved log file %s to %s", path, newPath)
	}

	// Move control log
	if err := c.moveControlLog(trialLogPath); err != nil {
		return err
	}

	// Move tcpdump logs
	for _, host := range c.Scenario.Hosts {
		if err := c.moveTcpdumpLogs(trialLogPath, host); err != nil {
			return err
		}
	}

	return nil
}

// MoveLogFilesToDir moves collected log files to the specified trial log directory.
// This is used for parallel execution where the log directory is pre-determined.
// Note: control.log is already created in trialLogDir by the runner, so we skip it here.
func (c *NetworkController) MoveLogFilesToDir(logFiles []string, trialLogDir string) error {
	topoPath := c.findTopoPath()

	// Copy FRR/syslog files
	for _, path := range logFiles {
		relativePath, err := filepath.Rel(topoPath, path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(trialLogDir, relativePath)

		if err := os.MkdirAll(filepath.Dir(newPath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create device directory: %w", err)
		}

		if err := copyFile(path, newPath); err != nil {
			return err
		}
		logrus.Debugf("Moved log file %s to %s", path, newPath)
	}

	// Move tcpdump logs
	for _, host := range c.Scenario.Hosts {
		if err := c.moveTcpdumpLogs(trialLogDir, host); err != nil {
			return err
		}
	}

	return nil
}

// moveControlLog moves the control log to the trial directory.
func (c *NetworkController) moveControlLog(trialLogPath string) error {
	srcPath := ControlLogFileName
	dstPath := filepath.Join(trialLogPath, ControlLogFileName)

	if err := copyFile(srcPath, dstPath); err != nil {
		return fmt.Errorf("failed to copy control log: %w", err)
	}
	logrus.Debugf("Moved control log to %s", dstPath)

	return os.Remove(srcPath)
}

// moveTcpdumpLogs moves tcpdump logs for a device to the trial directory.
func (c *NetworkController) moveTcpdumpLogs(trialLogPath, device string) error {
	tcpdumpDir := filepath.Join(trialLogPath, device, "tcpdump")
	if err := os.MkdirAll(tcpdumpDir, 0777); err != nil {
		return fmt.Errorf("failed to create tcpdump log directory: %w", err)
	}

	deviceIndex := c.getDeviceIndex(device)
	if deviceIndex < 0 {
		return fmt.Errorf("device %s not found", device)
	}

	for _, inter := range c.Devices.Nodes[deviceIndex].Interfaces {
		srcPath := filepath.Join(c.findTopoPath(), device, "tcpdump", "tcpdump_"+inter.Name+".log")
		dstPath := filepath.Join(tcpdumpDir, "tcpdump_"+inter.Name+".log")

		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy tcpdump log: %w", err)
		}
	}

	return nil
}

// findTopoPath returns the directory path of the topology file.
func (c *NetworkController) findTopoPath() string {
	return filepath.Dir(c.Scenario.Topo)
}

// getDeviceIndex returns the index of a device in the Devices list.
func (c *NetworkController) getDeviceIndex(device string) int {
	for i, node := range c.Devices.Nodes {
		if device == node.Name {
			return i
		}
	}
	return -1
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}
