// Package executor provides parallel execution control for netroub scenarios.
package executor

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Task represents a single scenario execution task.
type Task struct {
	ScenarioPath string
	RunID        string // Unique ID for this run (e.g., "A1_delay_pause_001")
	YAML         bool
}

// Result represents the result of a task execution.
type Result struct {
	Task      *Task
	Error     error
	Duration  time.Duration
	StartTime time.Time // Trial start time
	LogDir    string    // Log directory path for this trial
}

// Executor manages parallel execution of scenario tasks.
type Executor struct {
	parallel    int
	runner      TaskRunner
	batchLogger *BatchLogger
}

// TaskRunner is the interface for executing a single task.
// This allows for testing with mock implementations.
type TaskRunner interface {
	Run(task *Task) error
}

// TaskRunnerResult contains the result of a task execution.
type TaskRunnerResult struct {
	LogDir string
	Error  error
}

// TaskRunnerWithResult extends TaskRunner to return detailed results.
type TaskRunnerWithResult interface {
	TaskRunner
	RunWithResult(task *Task, startTime time.Time) TaskRunnerResult
}

// NewExecutor creates a new Executor with the specified parallelism.
func NewExecutor(parallel int, runner TaskRunner) *Executor {
	if parallel < 1 {
		parallel = 1
	}
	return &Executor{
		parallel: parallel,
		runner:   runner,
	}
}

// SetBatchLogger sets the batch logger for this executor.
func (e *Executor) SetBatchLogger(logger *BatchLogger) {
	e.batchLogger = logger
}

// Execute runs all tasks with the configured parallelism.
// Returns a slice of results for all tasks.
func (e *Executor) Execute(tasks []*Task) []*Result {
	return e.ExecuteWithProgress(tasks, false)
}

// ExecuteWithProgress runs all tasks with optional progress display.
func (e *Executor) ExecuteWithProgress(tasks []*Task, showProgress bool) []*Result {
	results := make([]*Result, len(tasks))
	taskChan := make(chan int, len(tasks))
	var wg sync.WaitGroup

	// Check if runner supports extended interface
	runnerWithResult, hasExtended := e.runner.(TaskRunnerWithResult)

	// In progress mode, suppress INFO logs to keep the display clean
	// Only show WARN and above on stdout
	if showProgress {
		originalLevel := logrus.GetLevel()
		logrus.SetLevel(logrus.WarnLevel)
		defer logrus.SetLevel(originalLevel)
	}

	// Create progress tracker
	progress := NewProgressTracker(tasks, showProgress)
	progress.Start()
	defer progress.Stop()

	// Fill the task channel
	for i := range tasks {
		taskChan <- i
	}
	close(taskChan)

	// Start workers
	for w := 0; w < e.parallel; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range taskChan {
				task := tasks[i]
				startTime := time.Now()

				if !showProgress {
					logrus.Infof("[Worker %d] Starting task %s", workerID, task.RunID)
				}

				var err error
				var logDir string

				if hasExtended {
					result := runnerWithResult.RunWithResult(task, startTime)
					err = result.Error
					logDir = result.LogDir
				} else {
					err = e.runner.Run(task)
				}

				duration := time.Since(startTime)

				results[i] = &Result{
					Task:      task,
					Error:     err,
					Duration:  duration,
					StartTime: startTime,
					LogDir:    logDir,
				}

				// Update progress tracker
				progress.TaskCompleted(task, err)

				// Log to batch logger
				if e.batchLogger != nil {
					e.batchLogger.LogTaskCompleted(task, duration, err, logDir)
				}

				if !showProgress {
					if err != nil {
						logrus.Warnf("[Worker %d] Task %s failed: %v (%.1fs)", workerID, task.RunID, err, duration.Seconds())
					} else {
						logrus.Infof("[Worker %d] Task %s completed (%.1fs)", workerID, task.RunID, duration.Seconds())
					}
				}
			}
		}(w)
	}

	wg.Wait()
	return results
}

// GenerateTasks creates tasks for repeated execution of a scenario.
func GenerateTasks(scenarioPath string, count int, yaml bool) []*Task {
	tasks := make([]*Task, count)

	// Extract scenario name from path
	scenarioName := extractScenarioName(scenarioPath)

	for i := 0; i < count; i++ {
		tasks[i] = &Task{
			ScenarioPath: scenarioPath,
			RunID:        fmt.Sprintf("%s_%03d", scenarioName, i+1),
			YAML:         yaml,
		}
	}

	return tasks
}

// extractScenarioName extracts the scenario name from a file path.
// e.g., "/path/to/A1_delay_pause.json" -> "A1_delay_pause"
func extractScenarioName(path string) string {
	// Find the last slash
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}

	name := path
	if lastSlash >= 0 {
		name = path[lastSlash+1:]
	}

	// Remove extension
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			name = name[:i]
			break
		}
	}

	return name
}

// Summary returns a summary of execution results.
func Summary(results []*Result) (total, succeeded, failed int, totalDuration time.Duration) {
	total = len(results)
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else {
			succeeded++
		}
		totalDuration += r.Duration
	}
	return
}

// PrintSummary prints a summary of execution results.
func PrintSummary(results []*Result) {
	total, succeeded, failed, totalDuration := Summary(results)

	fmt.Println()
	fmt.Println("========== Execution Summary ==========")
	fmt.Printf("Total: %d, Succeeded: %d, Failed: %d\n", total, succeeded, failed)
	fmt.Printf("Total Duration: %s\n", totalDuration.Round(time.Second))

	if failed > 0 {
		fmt.Println("\nFailed tasks:")
		for _, r := range results {
			if r.Error != nil {
				fmt.Printf("  - %s: %v\n", r.Task.RunID, r.Error)
				if r.LogDir != "" {
					fmt.Printf("    Log: %s/control.log\n", r.LogDir)
				}
			}
		}
	}
	fmt.Println("========================================")
}
