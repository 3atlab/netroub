package executor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockTaskRunner is a mock implementation of TaskRunner for testing
type mockTaskRunner struct {
	runCount  int32
	runDelay  time.Duration
	runError  error
	runCalled []string // RunIDs that were called
	mu        sync.Mutex
}

func (m *mockTaskRunner) Run(task *Task) error {
	atomic.AddInt32(&m.runCount, 1)
	m.mu.Lock()
	m.runCalled = append(m.runCalled, task.RunID)
	m.mu.Unlock()
	if m.runDelay > 0 {
		time.Sleep(m.runDelay)
	}
	return m.runError
}

func TestGenerateTasks(t *testing.T) {
	tests := []struct {
		name         string
		scenarioPath string
		count        int
		yaml         bool
		wantLen      int
		wantFirstID  string
		wantLastID   string
	}{
		{
			name:         "generate 3 tasks",
			scenarioPath: "/path/to/A1_delay_pause.json",
			count:        3,
			yaml:         false,
			wantLen:      3,
			wantFirstID:  "A1_delay_pause_001",
			wantLastID:   "A1_delay_pause_003",
		},
		{
			name:         "generate 110 tasks",
			scenarioPath: "baseline.json",
			count:        110,
			yaml:         false,
			wantLen:      110,
			wantFirstID:  "baseline_001",
			wantLastID:   "baseline_110",
		},
		{
			name:         "yaml scenario",
			scenarioPath: "/path/to/scenario.yaml",
			count:        2,
			yaml:         true,
			wantLen:      2,
			wantFirstID:  "scenario_001",
			wantLastID:   "scenario_002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := GenerateTasks(tt.scenarioPath, tt.count, tt.yaml)

			assert.Equal(t, tt.wantLen, len(tasks))
			assert.Equal(t, tt.wantFirstID, tasks[0].RunID)
			assert.Equal(t, tt.wantLastID, tasks[len(tasks)-1].RunID)
			assert.Equal(t, tt.scenarioPath, tasks[0].ScenarioPath)
			assert.Equal(t, tt.yaml, tasks[0].YAML)
		})
	}
}

func TestExtractScenarioName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/A1_delay_pause.json", "A1_delay_pause"},
		{"baseline.json", "baseline"},
		{"/path/to/scenario.yaml", "scenario"},
		{"test", "test"},
		{"/a/b/c/file.tar.gz", "file.tar"}, // Only removes last extension
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractScenarioName(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecutor_Execute_Sequential(t *testing.T) {
	mock := &mockTaskRunner{}
	tasks := GenerateTasks("test.json", 5, false)

	exec := NewExecutor(1, mock) // Sequential (parallel=1)
	results := exec.Execute(tasks)

	assert.Equal(t, 5, len(results))
	assert.Equal(t, int32(5), mock.runCount)
	for _, r := range results {
		assert.NoError(t, r.Error)
	}
}

func TestExecutor_Execute_Parallel(t *testing.T) {
	mock := &mockTaskRunner{
		runDelay: 10 * time.Millisecond,
	}
	tasks := GenerateTasks("test.json", 10, false)

	exec := NewExecutor(4, mock) // 4 parallel workers
	start := time.Now()
	results := exec.Execute(tasks)
	duration := time.Since(start)

	assert.Equal(t, 10, len(results))
	assert.Equal(t, int32(10), mock.runCount)

	// With 4 workers and 10 tasks of 10ms each, should take ~30ms (3 rounds)
	// Sequential would take ~100ms
	assert.Less(t, duration, 80*time.Millisecond, "parallel execution should be faster")
}

func TestExecutor_Execute_WithErrors(t *testing.T) {
	mock := &mockTaskRunner{
		runError: errors.New("task failed"),
	}
	tasks := GenerateTasks("test.json", 3, false)

	exec := NewExecutor(2, mock)
	results := exec.Execute(tasks)

	assert.Equal(t, 3, len(results))
	for _, r := range results {
		assert.Error(t, r.Error)
		assert.Contains(t, r.Error.Error(), "task failed")
	}
}

func TestSummary(t *testing.T) {
	results := []*Result{
		{Task: &Task{RunID: "test_001"}, Error: nil, Duration: 10 * time.Second},
		{Task: &Task{RunID: "test_002"}, Error: errors.New("failed"), Duration: 5 * time.Second},
		{Task: &Task{RunID: "test_003"}, Error: nil, Duration: 15 * time.Second},
	}

	total, succeeded, failed, totalDuration := Summary(results)

	assert.Equal(t, 3, total)
	assert.Equal(t, 2, succeeded)
	assert.Equal(t, 1, failed)
	assert.Equal(t, 30*time.Second, totalDuration)
}

func TestNewExecutor_MinParallel(t *testing.T) {
	mock := &mockTaskRunner{}

	// parallel < 1 should be set to 1
	exec := NewExecutor(0, mock)
	assert.Equal(t, 1, exec.parallel)

	exec = NewExecutor(-5, mock)
	assert.Equal(t, 1, exec.parallel)

	exec = NewExecutor(4, mock)
	assert.Equal(t, 4, exec.parallel)
}
