// Package executor provides parallel execution control for netroub scenarios.
package executor

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// BatchLogFileName is the default log file name for batch execution.
const BatchLogFileName = "netroub.log"

// BatchLogger handles logging for batch/repeat execution.
// It writes to a log file and optionally to stdout.
type BatchLogger struct {
	file      *os.File
	mu        sync.Mutex
	startTime time.Time
}

// NewBatchLogger creates a new BatchLogger that writes to the specified file.
// If the file already exists, it will be truncated.
func NewBatchLogger(path string) (*BatchLogger, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch log file: %w", err)
	}

	return &BatchLogger{
		file:      file,
		startTime: time.Now(),
	}, nil
}

// Close closes the log file.
func (l *BatchLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Log writes a log message with timestamp.
func (l *BatchLogger) Log(level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s\n", timestamp, level, message)

	if l.file != nil {
		l.file.WriteString(line)
	}
}

// Info logs an INFO level message.
func (l *BatchLogger) Info(format string, args ...any) {
	l.Log("INFO", format, args...)
}

// Error logs an ERROR level message.
func (l *BatchLogger) Error(format string, args ...any) {
	l.Log("ERROR", format, args...)
}

// Warn logs a WARN level message.
func (l *BatchLogger) Warn(format string, args ...any) {
	l.Log("WARN", format, args...)
}

// LogStart logs the start of batch execution.
func (l *BatchLogger) LogStart(command string, scenarios, totalRuns, parallel int, planFile string) {
	l.Info("=== Batch Execution Started ===")
	l.Info("Command: %s", command)
	if planFile != "" {
		l.Info("Plan file: %s", planFile)
	}
	l.Info("Scenarios: %d, Total runs: %d, Parallel: %d", scenarios, totalRuns, parallel)
	l.Info("")
}

// LogTaskCompleted logs the completion of a task.
func (l *BatchLogger) LogTaskCompleted(task *Task, duration time.Duration, err error, logDir string) {
	if err != nil {
		l.Error("[%s] Failed: %v (%.1fs)", task.RunID, err, duration.Seconds())
		if logDir != "" {
			l.Error("[%s] Log directory: %s", task.RunID, logDir)
		}
	} else {
		l.Info("[%s] Completed successfully (%.1fs)", task.RunID, duration.Seconds())
	}
}

// LogSummary logs the execution summary.
func (l *BatchLogger) LogSummary(results []*Result) {
	total, succeeded, failed, totalDuration := Summary(results)
	elapsed := time.Since(l.startTime)

	l.Info("")
	l.Info("=== Execution Summary ===")
	l.Info("Total: %d, Succeeded: %d, Failed: %d", total, succeeded, failed)
	l.Info("Total task duration: %s", totalDuration.Round(time.Second))
	l.Info("Wall clock time: %s", elapsed.Round(time.Second))

	if failed > 0 {
		l.Info("")
		l.Info("Failed tasks:")
		for _, r := range results {
			if r.Error != nil {
				l.Error("  - %s: %v", r.Task.RunID, r.Error)
				if r.LogDir != "" {
					l.Error("    Log: %s/control.log", r.LogDir)
				}
			}
		}
	}

	l.Info("=== Execution Completed ===")
}

// GetLogPath returns the log file path.
func (l *BatchLogger) GetLogPath() string {
	if l.file != nil {
		return l.file.Name()
	}
	return ""
}
