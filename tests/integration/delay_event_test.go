package integration

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDelayEvent tests the delay event functionality using a scenario with embedded measurements.
// The scenario executes ping before and after applying delay, then copies results for analysis.
func TestDelayEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectRoot := getProjectRoot(t)

	// Create test_logs directory
	testLogsDir := filepath.Join(projectRoot, "test_logs")
	err := os.MkdirAll(testLogsDir, 0755)
	require.NoError(t, err, "Failed to create test_logs directory")
	defer os.RemoveAll(testLogsDir)

	// Execute the scenario (includes baseline measurement, delay injection, and delayed measurement)
	scenarioPath := filepath.Join(projectRoot, "tests/scenarios/delay_50ms_test.json")
	executeScenario(t, scenarioPath)

	// Read and parse baseline RTT
	baselineFile := filepath.Join(testLogsDir, "baseline_rtt.txt")
	require.FileExists(t, baselineFile, "Baseline RTT file should exist")
	baselineRTT := parseRTTFile(t, baselineFile)
	t.Logf("Baseline RTT: mean=%.2fms, count=%d", calculateMean(baselineRTT), len(baselineRTT))

	// Read and parse delayed RTT
	delayedFile := filepath.Join(testLogsDir, "delayed_rtt.txt")
	require.FileExists(t, delayedFile, "Delayed RTT file should exist")
	delayedRTT := parseRTTFile(t, delayedFile)
	t.Logf("Delayed RTT: mean=%.2fms, count=%d", calculateMean(delayedRTT), len(delayedRTT))

	// Verify delay was applied
	assertDelayApplied(t, baselineRTT, delayedRTT, 50.0)
}

func parseRTTFile(t *testing.T, filePath string) []float64 {
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read RTT file")

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	values := make([]float64, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(line), 64)
		if err == nil {
			values = append(values, value)
		}
	}

	return values
}

// cleanupLab destroys any existing containerlab topology to ensure a clean state
func cleanupLab(t *testing.T, topoPath string) {
	t.Logf("Cleaning up lab: %s", topoPath)

	cmd := exec.Command("sudo", "clab", "destroy", "-t", topoPath, "--cleanup")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore errors - lab may not exist
		t.Logf("Cleanup output (may be expected): %s", string(output))
	}
}

func executeScenario(t *testing.T, scenarioPath string) {
	t.Logf("Executing scenario: %s", scenarioPath)

	projectRoot := getProjectRoot(t)
	netroubPath := filepath.Join(projectRoot, "netroub")

	// Clean up any existing lab before running
	topoPath := filepath.Join(projectRoot, "tests/topology/minimal_delay_test.yaml")
	cleanupLab(t, topoPath)

	cmd := exec.Command("sudo", netroubPath, scenarioPath)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Scenario output: %s", string(output))
	}
	require.NoError(t, err, "Failed to execute scenario: %s", string(output))

	t.Log("Scenario executed successfully")
}

func assertDelayApplied(t *testing.T, baseline, delayed []float64, expectedDelay float64) {
	require.NotEmpty(t, baseline, "Baseline measurements should not be empty")
	require.NotEmpty(t, delayed, "Delayed measurements should not be empty")

	baselineMean := calculateMean(baseline)
	delayedMean := calculateMean(delayed)
	actualDelay := delayedMean - baselineMean

	t.Logf("RTT Analysis: baseline=%.2fms, delayed=%.2fms, actual_delay=%.2fms, expected=%.2fms",
		baselineMean, delayedMean, actualDelay, expectedDelay)

	// Tolerance: +/-20% or +/-10ms, whichever is larger
	tolerance := math.Max(expectedDelay*0.2, 10.0)

	assert.InDelta(t, expectedDelay, actualDelay, tolerance,
		"Expected delay %.1fms, got %.1fms (baseline: %.1fms, delayed: %.1fms)",
		expectedDelay, actualDelay, baselineMean, delayedMean)

	// Verify delay is statistically significant
	assert.Greater(t, actualDelay, 5.0,
		"Delay should be at least 5ms to be measurable")
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func getProjectRoot(t *testing.T) string {
	wd, err := os.Getwd()
	require.NoError(t, err)
	// If we're in tests/integration, go up two levels
	if strings.HasSuffix(wd, "tests/integration") {
		return filepath.Join(wd, "../..")
	}
	return wd
}
