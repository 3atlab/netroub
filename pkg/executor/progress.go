package executor

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressTracker tracks the progress of parallel task execution.
type ProgressTracker struct {
	mu          sync.Mutex
	total       int
	completed   int
	failed      int
	startTime   time.Time
	taskResults map[string]*taskProgress // Track per-scenario progress
	output      io.Writer
	updateCh    chan struct{}
	doneCh      chan struct{}
	enabled     bool
}

// taskProgress tracks progress for a single scenario (all its repetitions).
type taskProgress struct {
	scenarioName string
	total        int
	completed    int
	failed       int
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker(tasks []*Task, enabled bool) *ProgressTracker {
	pt := &ProgressTracker{
		total:       len(tasks),
		taskResults: make(map[string]*taskProgress),
		output:      os.Stdout,
		updateCh:    make(chan struct{}, 100),
		doneCh:      make(chan struct{}),
		enabled:     enabled,
	}

	// Group tasks by scenario name
	for _, task := range tasks {
		scenarioName := extractScenarioName(task.ScenarioPath)
		if _, exists := pt.taskResults[scenarioName]; !exists {
			pt.taskResults[scenarioName] = &taskProgress{
				scenarioName: scenarioName,
			}
		}
		pt.taskResults[scenarioName].total++
	}

	return pt
}

// Start begins tracking progress.
func (pt *ProgressTracker) Start() {
	pt.startTime = time.Now()
	if pt.enabled {
		go pt.displayLoop()
	}
}

// TaskCompleted records a completed task.
func (pt *ProgressTracker) TaskCompleted(task *Task, err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.completed++
	scenarioName := extractScenarioName(task.ScenarioPath)
	if tp, exists := pt.taskResults[scenarioName]; exists {
		tp.completed++
		if err != nil {
			tp.failed++
			pt.failed++
		}
	}

	if pt.enabled {
		// Print failure immediately
		if err != nil {
			fmt.Fprintf(pt.output, "\n✗ %s failed: %v\n", task.RunID, err)
		}

		select {
		case pt.updateCh <- struct{}{}:
		default:
		}
	}
}

// Stop stops the progress display.
func (pt *ProgressTracker) Stop() {
	if pt.enabled {
		close(pt.doneCh)
		// Clear the progress line
		fmt.Fprint(pt.output, "\r\033[K")
	}
}

// displayLoop periodically updates the progress display.
func (pt *ProgressTracker) displayLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-pt.doneCh:
			return
		case <-ticker.C:
			pt.render()
		case <-pt.updateCh:
			pt.render()
		}
	}
}

// render displays the current progress.
func (pt *ProgressTracker) render() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	elapsed := time.Since(pt.startTime)
	eta := pt.calculateETA(elapsed)

	// Build progress bar
	percent := 0
	if pt.total > 0 {
		percent = pt.completed * 100 / pt.total
	}
	bar := pt.buildProgressBar(percent, 20)

	// Format status line
	status := fmt.Sprintf("\r[%s] %d/%d (%d%%) %s  Elapsed: %s  ETA: %s",
		bar,
		pt.completed,
		pt.total,
		percent,
		pt.failedStr(),
		formatDuration(elapsed),
		formatDuration(eta),
	)

	// Clear line and print status
	fmt.Fprintf(pt.output, "\033[K%s", status)
}

// calculateETA estimates the remaining time.
func (pt *ProgressTracker) calculateETA(elapsed time.Duration) time.Duration {
	if pt.completed == 0 {
		return 0
	}
	avgTime := elapsed / time.Duration(pt.completed)
	remaining := pt.total - pt.completed
	return avgTime * time.Duration(remaining)
}

// buildProgressBar creates a visual progress bar.
func (pt *ProgressTracker) buildProgressBar(percent, width int) string {
	filled := width * percent / 100
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// failedStr returns the failed count string if any failures occurred.
func (pt *ProgressTracker) failedStr() string {
	if pt.failed > 0 {
		return fmt.Sprintf("(failed: %d)", pt.failed)
	}
	return ""
}

// formatDuration formats a duration in a human-readable format.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "--:--"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}

// GetStats returns the current progress statistics.
func (pt *ProgressTracker) GetStats() (completed, total, failed int, elapsed time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.completed, pt.total, pt.failed, time.Since(pt.startTime)
}
