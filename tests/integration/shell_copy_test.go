package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellAndCopyEvent tests shell command execution and file copy operations
func TestShellAndCopyEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectRoot := getProjectRoot(t)

	// Create test_logs directory
	testLogsDir := filepath.Join(projectRoot, "test_logs")
	err := os.MkdirAll(testLogsDir, 0755)
	require.NoError(t, err, "Failed to create test_logs directory")
	defer os.RemoveAll(testLogsDir)

	// Execute the scenario (netroub handles deploy/destroy)
	executeScenario(t, filepath.Join(projectRoot, "tests/scenarios/shell_copy_test.json"))

	// Verify the output file was copied
	outputFile := filepath.Join(testLogsDir, "shell_output.txt")
	require.FileExists(t, outputFile, "Output file should exist after copy")

	// Read and verify content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err, "Failed to read output file")

	contentStr := string(content)
	assert.Contains(t, contentStr, "shell test output", "Output should contain expected text")
	t.Logf("Shell output content:\n%s", contentStr)
}

// TestCopyBidirectional tests both toContainer and fromContainer operations
func TestCopyBidirectional(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectRoot := getProjectRoot(t)

	// Create test_logs directory
	testLogsDir := filepath.Join(projectRoot, "test_logs")
	err := os.MkdirAll(testLogsDir, 0755)
	require.NoError(t, err, "Failed to create test_logs directory")
	defer os.RemoveAll(testLogsDir)

	// Verify input file exists
	inputFile := filepath.Join(projectRoot, "tests/data/test_input.txt")
	require.FileExists(t, inputFile, "Input file should exist")

	// Execute the scenario (netroub handles deploy/destroy)
	executeScenario(t, filepath.Join(projectRoot, "tests/scenarios/copy_bidirectional_test.json"))

	// Verify the output file was copied back
	outputFile := filepath.Join(testLogsDir, "test_output.txt")
	require.FileExists(t, outputFile, "Output file should exist after copy")

	// Read and verify content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err, "Failed to read output file")

	contentStr := string(content)
	assert.Contains(t, contentStr, "test input file", "Output should contain original input")
	assert.Contains(t, contentStr, "processed by r1", "Output should contain processed marker")
	t.Logf("Bidirectional copy output:\n%s", contentStr)
}
