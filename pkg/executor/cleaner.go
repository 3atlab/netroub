// Package executor provides parallel execution control for netroub scenarios.
package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/sirupsen/logrus"
)

// GenerateLabNames generates all lab names for a plan.
// Returns a list of lab name patterns (e.g., "baseline_001", "baseline_002", ...).
func GenerateLabNamesFromPlan(plan *Plan, baseDir string) ([]string, error) {
	tasks, err := GenerateTasksFromPlan(plan, baseDir)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = task.RunID
	}
	return names, nil
}

// GenerateLabNamesFromScenario generates lab names for a single scenario.
// count specifies how many repetitions to generate names for.
// If count is 0, it generates a single name without suffix.
func GenerateLabNamesFromScenario(scenarioPath string, count int) []string {
	// Get the base directory of the scenario file for resolving relative paths
	baseDir := filepath.Dir(scenarioPath)
	if baseDir == "" || baseDir == "." {
		baseDir, _ = filepath.Abs(".")
	}

	// Get the actual lab name from the scenario file
	labName, err := model.GetLabNameFromScenario(scenarioPath, baseDir)
	if err != nil {
		logrus.Warnf("Failed to read scenario file, falling back to filename: %v", err)
		labName = extractScenarioName(scenarioPath)
	}

	if count <= 0 {
		// Single run without suffix
		return []string{labName}
	}

	// Generate names with numeric suffixes for parallel execution
	names := make([]string, count)
	for i := 0; i < count; i++ {
		names[i] = fmt.Sprintf("%s_%03d", labName, i+1)
	}
	return names
}

// CleanContainers removes Docker containers matching the given lab names.
// Returns the number of containers removed and any error encountered.
func CleanContainers(labNames []string, dryRun bool) (int, error) {
	if len(labNames) == 0 {
		return 0, nil
	}

	// Get all clab- containers at once (efficient single docker call)
	output, err := exec.Command("sudo", "docker", "ps", "-a", "--filter", "name=clab-", "--format", "{{.ID}}\t{{.Names}}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(output) == 0 {
		logrus.Info("No containers found to clean")
		return 0, nil
	}

	// Build a set of lab names for fast lookup
	labNameSet := make(map[string]bool)
	for _, name := range labNames {
		labNameSet[name] = true
	}

	// Filter containers that match our lab names
	// Container names are like "clab-{labName}-{nodeName}"
	var matchingContainers []string
	var matchingNames []string

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		containerID := parts[0]
		containerName := parts[1]

		// Extract lab name from container name: clab-{labName}-{nodeName}
		if !strings.HasPrefix(containerName, "clab-") {
			continue
		}

		// Find the lab name by matching against our set
		for labName := range labNameSet {
			prefix := fmt.Sprintf("clab-%s-", labName)
			if strings.HasPrefix(containerName, prefix) {
				matchingContainers = append(matchingContainers, containerID)
				matchingNames = append(matchingNames, containerName)
				break
			}
		}
	}

	if len(matchingContainers) == 0 {
		logrus.Info("No matching containers found to clean")
		return 0, nil
	}

	if dryRun {
		fmt.Printf("Found %d containers to remove:\n", len(matchingContainers))
		for _, name := range matchingNames {
			fmt.Printf("  %s\n", name)
		}
		return len(matchingContainers), nil
	}

	// Remove containers
	args := append([]string{"docker", "rm", "-f"}, matchingContainers...)
	cmd := exec.Command("sudo", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to remove containers: %w, output: %s", err, cmdOutput)
	}

	return len(matchingContainers), nil
}

// CleanLabDirectories removes containerlab lab directories matching the given lab names.
// Lab directories are typically located in the same directory as the topology file.
func CleanLabDirectories(topoDir string, labNames []string, dryRun bool) (int, error) {
	// Lab directories are named "clab-{labName}"
	removed := 0
	for _, labName := range labNames {
		labDir := fmt.Sprintf("%s/clab-%s", topoDir, labName)

		// Check if directory exists
		cmd := exec.Command("test", "-d", labDir)
		if err := cmd.Run(); err != nil {
			// Directory doesn't exist, skip
			continue
		}

		if dryRun {
			fmt.Printf("Would remove directory: %s\n", labDir)
			removed++
			continue
		}

		// Remove directory
		cmd = exec.Command("sudo", "rm", "-rf", labDir)
		if err := cmd.Run(); err != nil {
			logrus.Warnf("Failed to remove directory %s: %v", labDir, err)
			continue
		}

		logrus.Debugf("Removed directory: %s", labDir)
		removed++
	}

	if removed > 0 && !dryRun {
		logrus.Infof("Removed %d lab directories", removed)
	}

	return removed, nil
}

// CleanDockerNetworks removes Docker networks matching the given lab names.
// Network names are "clab-{labName}".
func CleanDockerNetworks(labNames []string, dryRun bool) (int, error) {
	if len(labNames) == 0 {
		return 0, nil
	}

	// Get all clab- networks
	output, err := exec.Command("sudo", "docker", "network", "ls", "--filter", "name=clab-", "--format", "{{.Name}}").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list networks: %w", err)
	}

	if len(output) == 0 {
		return 0, nil
	}

	// Build a set of expected network names
	networkNameSet := make(map[string]bool)
	for _, labName := range labNames {
		networkNameSet["clab-"+labName] = true
	}

	// Find matching networks
	var matchingNetworks []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, networkName := range lines {
		if networkName == "" {
			continue
		}
		if networkNameSet[networkName] {
			matchingNetworks = append(matchingNetworks, networkName)
		}
	}

	if len(matchingNetworks) == 0 {
		return 0, nil
	}

	if dryRun {
		fmt.Printf("Found %d Docker networks to remove:\n", len(matchingNetworks))
		for _, name := range matchingNetworks {
			fmt.Printf("  %s\n", name)
		}
		return len(matchingNetworks), nil
	}

	// Remove networks
	removed := 0
	for _, networkName := range matchingNetworks {
		cmd := exec.Command("sudo", "docker", "network", "rm", networkName)
		if err := cmd.Run(); err != nil {
			logrus.Warnf("Failed to remove network %s: %v", networkName, err)
			continue
		}
		logrus.Debugf("Removed network: %s", networkName)
		removed++
	}

	if removed > 0 {
		logrus.Infof("Removed %d Docker networks", removed)
	}

	return removed, nil
}
