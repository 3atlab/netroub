package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBatchLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewBatchLogger(logPath)
	require.NoError(t, err)
	defer logger.Close()

	assert.NotNil(t, logger)
	assert.Equal(t, logPath, logger.GetLogPath())
}

func TestBatchLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewBatchLogger(logPath)
	require.NoError(t, err)

	logger.Info("Test info message")
	logger.Error("Test error message")
	logger.Warn("Test warning message")

	logger.Close()

	// Read the log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "[INFO] Test info message")
	assert.Contains(t, string(content), "[ERROR] Test error message")
	assert.Contains(t, string(content), "[WARN] Test warning message")
}

func TestBatchLogger_LogStart(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewBatchLogger(logPath)
	require.NoError(t, err)

	logger.LogStart("batch", 10, 100, 4, "plan.yaml")
	logger.Close()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logStr := string(content)
	assert.Contains(t, logStr, "Batch Execution Started")
	assert.Contains(t, logStr, "Command: batch")
	assert.Contains(t, logStr, "Plan file: plan.yaml")
	assert.Contains(t, logStr, "Scenarios: 10")
	assert.Contains(t, logStr, "Total runs: 100")
	assert.Contains(t, logStr, "Parallel: 4")
}

func TestBatchLogger_LogTaskCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewBatchLogger(logPath)
	require.NoError(t, err)

	task := &Task{RunID: "test_001", ScenarioPath: "test.json"}

	// Test successful completion
	logger.LogTaskCompleted(task, 30*time.Second, nil, "")

	// Test failed completion
	task2 := &Task{RunID: "test_002", ScenarioPath: "test.json"}
	logger.LogTaskCompleted(task2, 45*time.Second, assert.AnError, "/path/to/log")

	logger.Close()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logStr := string(content)
	assert.Contains(t, logStr, "[test_001] Completed successfully")
	assert.Contains(t, logStr, "[test_002] Failed")
	assert.Contains(t, logStr, "/path/to/log")
}

func TestBatchLogger_LogSummary(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewBatchLogger(logPath)
	require.NoError(t, err)

	results := []*Result{
		{Task: &Task{RunID: "test_001"}, Duration: 30 * time.Second, Error: nil},
		{Task: &Task{RunID: "test_002"}, Duration: 45 * time.Second, Error: nil},
		{Task: &Task{RunID: "test_003"}, Duration: 20 * time.Second, Error: assert.AnError, LogDir: "/path/to/log"},
	}

	logger.LogSummary(results)
	logger.Close()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logStr := string(content)
	assert.Contains(t, logStr, "Execution Summary")
	assert.Contains(t, logStr, "Total: 3")
	assert.Contains(t, logStr, "Succeeded: 2")
	assert.Contains(t, logStr, "Failed: 1")
	assert.Contains(t, logStr, "Failed tasks:")
	assert.Contains(t, logStr, "test_003")
}

func TestBatchLogger_Timestamp(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewBatchLogger(logPath)
	require.NoError(t, err)

	logger.Info("Test message")
	logger.Close()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	// Check that timestamp is present (format: 2006-01-02 15:04:05)
	lines := strings.Split(string(content), "\n")
	require.True(t, len(lines) > 0)

	// First line should start with a timestamp
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`, lines[0])
}
