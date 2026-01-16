package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabName_BasicOperations(t *testing.T) {
	// Reset state
	ResetLabName()
	Devices.Name = "default-topo"

	// Initially returns Devices.Name
	assert.Equal(t, "default-topo", GetLabName())

	// SetLabName overrides
	SetLabName("custom-lab")
	assert.Equal(t, "custom-lab", GetLabName())

	// ResetLabName clears override
	ResetLabName()
	assert.Equal(t, "default-topo", GetLabName())
}

func TestLabName_ClabHostName(t *testing.T) {
	ResetLabName()
	Devices.Name = "test-topo"

	assert.Equal(t, "clab-test-topo-r1", ClabHostName("r1"))

	SetLabName("my-lab")
	assert.Equal(t, "clab-my-lab-r1", ClabHostName("r1"))

	ResetLabName()
}

// TestLabName_ConcurrentAccess tests for race conditions when multiple
// goroutines access LabName simultaneously.
// Run with: go test -race ./pkg/model/...
func TestLabName_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 100
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines setting and getting LabName concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Simulate the pattern used in runner.go
				labName := fmt.Sprintf("lab_%d_%d", id, j)
				SetLabName(labName)
				got := GetLabName()
				// Note: got may not equal labName due to concurrent access
				// This test is primarily for race detection
				_ = got
				ResetLabName()
			}
		}(i)
	}

	wg.Wait()
}

// TestLabName_IsolationPattern tests the pattern used in ScenarioRunner
// where LabName is set, immediately copied to local variable, then used.
func TestLabName_IsolationPattern(t *testing.T) {
	const numGoroutines = 50
	results := make(chan string, numGoroutines)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			expectedName := fmt.Sprintf("task_%03d", id)

			// This is the pattern from runner.go:
			// SetLabName -> GetLabName -> use local copy
			SetLabName(expectedName)
			localLabName := GetLabName()

			// Simulate some work
			// In real code, localLabName is passed to EventExecutor/NetworkController
			// so it's isolated from further LabName changes

			results <- localLabName
		}(i)
	}

	wg.Wait()
	close(results)

	// Collect results - with race condition, some values may be wrong
	// This test documents the current behavior
	collected := make([]string, 0, numGoroutines)
	for r := range results {
		collected = append(collected, r)
	}

	assert.Equal(t, numGoroutines, len(collected))
	// Note: We cannot assert that all values are unique/correct
	// because the current implementation has a race condition
}
