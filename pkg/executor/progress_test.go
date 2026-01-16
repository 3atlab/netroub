package executor

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewProgressTracker(t *testing.T) {
	tasks := []*Task{
		{ScenarioPath: "/path/to/A1.json", RunID: "A1_001"},
		{ScenarioPath: "/path/to/A1.json", RunID: "A1_002"},
		{ScenarioPath: "/path/to/B1.json", RunID: "B1_001"},
	}

	pt := NewProgressTracker(tasks, false)

	assert.Equal(t, 3, pt.total)
	assert.Equal(t, 0, pt.completed)
	assert.Equal(t, 2, len(pt.taskResults)) // A1 and B1

	// Check per-scenario counts
	assert.Equal(t, 2, pt.taskResults["A1"].total)
	assert.Equal(t, 1, pt.taskResults["B1"].total)
}

func TestProgressTracker_TaskCompleted(t *testing.T) {
	tasks := []*Task{
		{ScenarioPath: "/path/to/A1.json", RunID: "A1_001"},
		{ScenarioPath: "/path/to/A1.json", RunID: "A1_002"},
	}

	pt := NewProgressTracker(tasks, false)
	pt.Start()
	defer pt.Stop()

	// Complete first task successfully
	pt.TaskCompleted(tasks[0], nil)
	assert.Equal(t, 1, pt.completed)
	assert.Equal(t, 0, pt.failed)

	// Complete second task with error
	pt.TaskCompleted(tasks[1], assert.AnError)
	assert.Equal(t, 2, pt.completed)
	assert.Equal(t, 1, pt.failed)
}

func TestProgressTracker_GetStats(t *testing.T) {
	tasks := []*Task{
		{ScenarioPath: "/path/to/test.json", RunID: "test_001"},
		{ScenarioPath: "/path/to/test.json", RunID: "test_002"},
		{ScenarioPath: "/path/to/test.json", RunID: "test_003"},
	}

	pt := NewProgressTracker(tasks, false)
	pt.Start()
	defer pt.Stop()

	pt.TaskCompleted(tasks[0], nil)
	pt.TaskCompleted(tasks[1], assert.AnError)

	completed, total, failed, elapsed := pt.GetStats()
	assert.Equal(t, 2, completed)
	assert.Equal(t, 3, total)
	assert.Equal(t, 1, failed)
	assert.Greater(t, elapsed.Nanoseconds(), int64(0))
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{0, "--:--"},
		{30 * time.Second, "0m30s"},
		{90 * time.Second, "1m30s"},
		{65 * time.Minute, "1h05m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProgressTracker_BuildProgressBar(t *testing.T) {
	pt := &ProgressTracker{}

	tests := []struct {
		percent int
		width   int
		want    string
	}{
		{0, 10, "░░░░░░░░░░"},
		{50, 10, "█████░░░░░"},
		{100, 10, "██████████"},
		{25, 20, "█████░░░░░░░░░░░░░░░"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := pt.buildProgressBar(tt.percent, tt.width)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProgressTracker_Render(t *testing.T) {
	tasks := []*Task{
		{ScenarioPath: "/path/to/test.json", RunID: "test_001"},
		{ScenarioPath: "/path/to/test.json", RunID: "test_002"},
	}

	var buf bytes.Buffer
	pt := NewProgressTracker(tasks, true)
	pt.output = &buf
	pt.startTime = time.Now()

	pt.TaskCompleted(tasks[0], nil)
	pt.render()

	output := buf.String()
	// Should contain progress bar elements
	assert.Contains(t, output, "1/2")
	assert.Contains(t, output, "50%")
}

func TestProgressTracker_CalculateETA(t *testing.T) {
	tasks := make([]*Task, 10)
	for i := range tasks {
		tasks[i] = &Task{ScenarioPath: "test.json", RunID: "test"}
	}

	pt := NewProgressTracker(tasks, false)
	pt.completed = 5 // 50% done

	// If 5 tasks took 10 seconds, remaining 5 should take ~10 seconds
	elapsed := 10 * time.Second
	eta := pt.calculateETA(elapsed)

	// ETA should be approximately 10 seconds (5 remaining * 2s per task)
	assert.InDelta(t, 10*time.Second, eta, float64(time.Second))
}

func TestProgressTracker_CalculateETA_NoProgress(t *testing.T) {
	tasks := make([]*Task, 10)
	for i := range tasks {
		tasks[i] = &Task{ScenarioPath: "test.json", RunID: "test"}
	}

	pt := NewProgressTracker(tasks, false)
	pt.completed = 0

	eta := pt.calculateETA(time.Second)
	assert.Equal(t, time.Duration(0), eta)
}
