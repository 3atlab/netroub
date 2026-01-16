package network

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/stretchr/testify/assert"
)

// slowMockRunner simulates slow command execution to test mutex behavior
type slowMockRunner struct {
	mu            sync.Mutex
	calls         [][]string
	deployDelay   time.Duration
	destroyDelay  time.Duration
	concurrentOps int32 // tracks concurrent operations (should never exceed 1)
	maxConcurrent int32 // maximum concurrent operations observed
}

func newSlowMockRunner(deployDelay, destroyDelay time.Duration) *slowMockRunner {
	return &slowMockRunner{
		deployDelay:  deployDelay,
		destroyDelay: destroyDelay,
	}
}

func (m *slowMockRunner) Run(name string, args ...string) ([]byte, error) {
	// Track concurrent operations
	current := atomic.AddInt32(&m.concurrentOps, 1)
	defer atomic.AddInt32(&m.concurrentOps, -1)

	// Update max concurrent if this is higher
	for {
		max := atomic.LoadInt32(&m.maxConcurrent)
		if current <= max {
			break
		}
		if atomic.CompareAndSwapInt32(&m.maxConcurrent, max, current) {
			break
		}
	}

	// Record the call
	m.mu.Lock()
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	m.mu.Unlock()

	// Simulate slow operation based on command type
	for _, arg := range args {
		if arg == "deploy" {
			time.Sleep(m.deployDelay)
			return []byte("deploy success"), nil
		}
		if arg == "destroy" {
			time.Sleep(m.destroyDelay)
			return []byte("destroy success"), nil
		}
	}

	return []byte("ok"), nil
}

func (m *slowMockRunner) RunDetached(name string, args ...string) error {
	m.mu.Lock()
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	m.mu.Unlock()
	return nil
}

func (m *slowMockRunner) getCalls() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([][]string, len(m.calls))
	copy(result, m.calls)
	return result
}

// TestNetworkMutex_SerializesDeployOperations verifies that multiple
// Deploy operations are serialized and don't run concurrently.
func TestNetworkMutex_SerializesDeployOperations(t *testing.T) {
	mock := newSlowMockRunner(50*time.Millisecond, 50*time.Millisecond)

	// Create multiple controllers
	controllers := make([]*NetworkController, 4)
	for i := 0; i < 4; i++ {
		scenario := &model.Scenario{Topo: "/path/to/topo.yaml"}
		devices := &model.Data{}
		controllers[i] = NewNetworkController(scenario, devices, "test-lab", mock)
	}

	// Run all deploys concurrently
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := controllers[idx].Deploy()
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all deploys completed
	assert.Equal(t, 4, len(mock.getCalls()))

	// Verify max concurrent operations was 1 (serialized)
	assert.Equal(t, int32(1), atomic.LoadInt32(&mock.maxConcurrent),
		"Deploy operations should be serialized (max concurrent = 1)")
}

// TestNetworkMutex_SerializesDestroyOperations verifies that multiple
// Destroy operations are serialized and don't run concurrently.
func TestNetworkMutex_SerializesDestroyOperations(t *testing.T) {
	mock := newSlowMockRunner(50*time.Millisecond, 50*time.Millisecond)

	// Create multiple controllers
	controllers := make([]*NetworkController, 4)
	for i := 0; i < 4; i++ {
		scenario := &model.Scenario{Topo: "/path/to/topo.yaml"}
		devices := &model.Data{}
		controllers[i] = NewNetworkController(scenario, devices, "test-lab", mock)
	}

	// Run all destroys concurrently
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := controllers[idx].Destroy()
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all destroys completed
	assert.Equal(t, 4, len(mock.getCalls()))

	// Verify max concurrent operations was 1 (serialized)
	assert.Equal(t, int32(1), atomic.LoadInt32(&mock.maxConcurrent),
		"Destroy operations should be serialized (max concurrent = 1)")
}

// TestNetworkMutex_SerializesMixedOperations verifies that Deploy and
// Destroy operations share the same mutex and are all serialized.
func TestNetworkMutex_SerializesMixedOperations(t *testing.T) {
	mock := newSlowMockRunner(30*time.Millisecond, 30*time.Millisecond)

	// Create multiple controllers
	numOps := 8
	controllers := make([]*NetworkController, numOps)
	for i := 0; i < numOps; i++ {
		scenario := &model.Scenario{Topo: "/path/to/topo.yaml"}
		devices := &model.Data{}
		controllers[i] = NewNetworkController(scenario, devices, "test-lab", mock)
	}

	// Run alternating deploy/destroy concurrently
	var wg sync.WaitGroup
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				err := controllers[idx].Deploy()
				assert.NoError(t, err)
			} else {
				err := controllers[idx].Destroy()
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations completed
	assert.Equal(t, numOps, len(mock.getCalls()))

	// Verify max concurrent operations was 1 (serialized)
	assert.Equal(t, int32(1), atomic.LoadInt32(&mock.maxConcurrent),
		"Mixed deploy/destroy operations should be serialized (max concurrent = 1)")
}

// TestNetworkMutex_NoDeadlock verifies that many concurrent operations
// complete without deadlock.
func TestNetworkMutex_NoDeadlock(t *testing.T) {
	mock := newSlowMockRunner(5*time.Millisecond, 5*time.Millisecond)

	numOps := 20
	controllers := make([]*NetworkController, numOps)
	for i := 0; i < numOps; i++ {
		scenario := &model.Scenario{Topo: "/path/to/topo.yaml"}
		devices := &model.Data{}
		controllers[i] = NewNetworkController(scenario, devices, "test-lab", mock)
	}

	// Use a timeout to detect deadlock
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				if idx%2 == 0 {
					_ = controllers[idx].Deploy()
				} else {
					_ = controllers[idx].Destroy()
				}
			}(i)
		}
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - all operations completed
		assert.Equal(t, numOps, len(mock.getCalls()))
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected: operations did not complete within timeout")
	}
}

// TestNetworkMutex_TotalDurationReflectsSerialization verifies that
// the total duration of serialized operations is approximately the
// sum of individual operation durations.
func TestNetworkMutex_TotalDurationReflectsSerialization(t *testing.T) {
	opDuration := 20 * time.Millisecond
	mock := newSlowMockRunner(opDuration, opDuration)

	numOps := 4
	controllers := make([]*NetworkController, numOps)
	for i := 0; i < numOps; i++ {
		scenario := &model.Scenario{Topo: "/path/to/topo.yaml"}
		devices := &model.Data{}
		controllers[i] = NewNetworkController(scenario, devices, "test-lab", mock)
	}

	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = controllers[idx].Deploy()
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(start)

	// If operations were parallel, elapsed would be ~opDuration
	// If serialized, elapsed should be ~numOps * opDuration
	expectedMin := time.Duration(numOps-1) * opDuration // Allow some margin
	expectedMax := time.Duration(numOps+1) * opDuration // Allow some margin

	assert.GreaterOrEqual(t, elapsed, expectedMin,
		"Total duration should reflect serialization (expected >= %v, got %v)", expectedMin, elapsed)
	assert.LessOrEqual(t, elapsed, expectedMax,
		"Total duration should not be excessively long (expected <= %v, got %v)", expectedMax, elapsed)
}

// timestampedMockRunner records timestamps for each operation
type timestampedMockRunner struct {
	mu           sync.Mutex
	deployStart  []time.Time
	deployEnd    []time.Time
	destroyStart []time.Time
	destroyEnd   []time.Time
	opDuration   time.Duration
}

func newTimestampedMockRunner(opDuration time.Duration) *timestampedMockRunner {
	return &timestampedMockRunner{
		opDuration: opDuration,
	}
}

func (m *timestampedMockRunner) Run(name string, args ...string) ([]byte, error) {
	isDeploy := false
	isDestroy := false
	for _, arg := range args {
		if arg == "deploy" {
			isDeploy = true
		}
		if arg == "destroy" {
			isDestroy = true
		}
	}

	m.mu.Lock()
	if isDeploy {
		m.deployStart = append(m.deployStart, time.Now())
	}
	if isDestroy {
		m.destroyStart = append(m.destroyStart, time.Now())
	}
	m.mu.Unlock()

	time.Sleep(m.opDuration)

	m.mu.Lock()
	if isDeploy {
		m.deployEnd = append(m.deployEnd, time.Now())
	}
	if isDestroy {
		m.destroyEnd = append(m.destroyEnd, time.Now())
	}
	m.mu.Unlock()

	return []byte("ok"), nil
}

func (m *timestampedMockRunner) RunDetached(name string, args ...string) error {
	return nil
}

// TestNetworkMutex_NoOverlappingOperations verifies that no two
// operations overlap in time.
func TestNetworkMutex_NoOverlappingOperations(t *testing.T) {
	opDuration := 30 * time.Millisecond
	mock := newTimestampedMockRunner(opDuration)

	numOps := 6
	controllers := make([]*NetworkController, numOps)
	for i := 0; i < numOps; i++ {
		scenario := &model.Scenario{Topo: "/path/to/topo.yaml"}
		devices := &model.Data{}
		controllers[i] = NewNetworkController(scenario, devices, "test-lab", mock)
	}

	var wg sync.WaitGroup
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_ = controllers[idx].Deploy()
			} else {
				_ = controllers[idx].Destroy()
			}
		}(i)
	}
	wg.Wait()

	// Collect all intervals
	type interval struct {
		start time.Time
		end   time.Time
		op    string
	}

	mock.mu.Lock()
	var intervals []interval
	for i := range mock.deployStart {
		intervals = append(intervals, interval{mock.deployStart[i], mock.deployEnd[i], "deploy"})
	}
	for i := range mock.destroyStart {
		intervals = append(intervals, interval{mock.destroyStart[i], mock.destroyEnd[i], "destroy"})
	}
	mock.mu.Unlock()

	// Check for overlapping intervals
	for i := 0; i < len(intervals); i++ {
		for j := i + 1; j < len(intervals); j++ {
			a := intervals[i]
			b := intervals[j]

			// Check if intervals overlap
			// Two intervals [a.start, a.end] and [b.start, b.end] overlap if:
			// a.start < b.end AND b.start < a.end
			overlap := a.start.Before(b.end) && b.start.Before(a.end)
			assert.False(t, overlap,
				"Operations should not overlap: %s [%v - %v] and %s [%v - %v]",
				a.op, a.start.UnixMilli(), a.end.UnixMilli(),
				b.op, b.start.UnixMilli(), b.end.UnixMilli())
		}
	}
}
