package executor

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// getTestDataDir returns the absolute path to the testdata directory.
func getTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// createTestScenario creates a temporary scenario file with the given name.
// Returns the path to the created file.
func createTestScenario(t *testing.T, dir, name, duration string) string {
	t.Helper()
	dataPath := filepath.Join(dir, "data.json")
	content := `{
    "scenarioName": "` + name + `",
    "logPath": "./log",
    "topo": "./topo.yaml",
    "data": "` + dataPath + `",
    "duration": "` + duration + `",
    "hosts": ["r1"],
    "event": []
}`
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test scenario: %v", err)
	}
	return path
}

// TestLoadScenarioAndDevices_Parallel tests that parallel scenario loading
// returns the correct scenario for each goroutine.
// This test verifies that the mutex protection in loadScenarioAndDevices
// prevents race conditions when multiple goroutines load scenarios simultaneously.
//
// Run with: go test -race ./pkg/executor/...
func TestLoadScenarioAndDevices_Parallel(t *testing.T) {
	testDataDir := getTestDataDir()

	// Create test scenarios with absolute paths to data.json
	scenarios := []struct {
		path         string
		expectedName string
		expectedDur  string
	}{
		{createTestScenario(t, testDataDir, "test_scenario_A", "10s"), "test_scenario_A", "10s"},
		{createTestScenario(t, testDataDir, "test_scenario_B", "20s"), "test_scenario_B", "20s"},
		{createTestScenario(t, testDataDir, "test_scenario_C", "30s"), "test_scenario_C", "30s"},
		{createTestScenario(t, testDataDir, "test_scenario_D", "40s"), "test_scenario_D", "40s"},
	}

	// Cleanup generated files after test
	defer func() {
		for _, s := range scenarios {
			os.Remove(s.path)
		}
	}()

	// Number of iterations to increase chance of detecting race conditions
	const iterations = 50

	for iter := 0; iter < iterations; iter++ {
		var wg sync.WaitGroup
		results := make(chan struct {
			taskPath     string
			scenarioName string
			duration     string
			err          error
		}, len(scenarios))

		// Launch goroutines to load scenarios in parallel
		for _, s := range scenarios {
			wg.Add(1)
			go func(scenarioPath, expectedName, expectedDur string) {
				defer wg.Done()

				runner := &ScenarioRunner{}
				task := &Task{
					ScenarioPath: scenarioPath,
					YAML:         false,
				}

				scenario, _, err := runner.loadScenarioAndDevices(task)

				result := struct {
					taskPath     string
					scenarioName string
					duration     string
					err          error
				}{
					taskPath: scenarioPath,
					err:      err,
				}
				if scenario != nil {
					result.scenarioName = scenario.ScenarioName
					result.duration = scenario.Duration
				}
				results <- result
			}(s.path, s.expectedName, s.expectedDur)
		}

		wg.Wait()
		close(results)

		// Verify each goroutine got the correct scenario
		for result := range results {
			if result.err != nil {
				t.Errorf("Failed to load scenario %s: %v", result.taskPath, result.err)
				continue
			}

			// Find expected values for this path
			var expectedName, expectedDur string
			for _, s := range scenarios {
				if s.path == result.taskPath {
					expectedName = s.expectedName
					expectedDur = s.expectedDur
					break
				}
			}

			assert.Equal(t, expectedName, result.scenarioName,
				"Iteration %d: scenario %s got wrong name (expected %s, got %s)",
				iter, result.taskPath, expectedName, result.scenarioName)

			assert.Equal(t, expectedDur, result.duration,
				"Iteration %d: scenario %s got wrong duration (expected %s, got %s)",
				iter, result.taskPath, expectedDur, result.duration)
		}
	}
}

// TestLoadScenarioAndDevices_ParallelStress is a stress test with more goroutines
// and iterations to thoroughly test the mutex protection.
func TestLoadScenarioAndDevices_ParallelStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	testDataDir := getTestDataDir()

	// Create test scenarios with absolute paths
	scenarioPaths := []string{
		createTestScenario(t, testDataDir, "stress_scenario_A", "10s"),
		createTestScenario(t, testDataDir, "stress_scenario_B", "20s"),
		createTestScenario(t, testDataDir, "stress_scenario_C", "30s"),
		createTestScenario(t, testDataDir, "stress_scenario_D", "40s"),
	}
	defer func() {
		for _, p := range scenarioPaths {
			os.Remove(p)
		}
	}()

	expectedNames := map[string]string{
		scenarioPaths[0]: "stress_scenario_A",
		scenarioPaths[1]: "stress_scenario_B",
		scenarioPaths[2]: "stress_scenario_C",
		scenarioPaths[3]: "stress_scenario_D",
	}

	const numGoroutines = 20
	const iterationsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan string, numGoroutines*iterationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < iterationsPerGoroutine; j++ {
				// Each goroutine picks a scenario based on its iteration
				scenarioPath := scenarioPaths[(goroutineID+j)%len(scenarioPaths)]
				expectedName := expectedNames[scenarioPath]

				runner := &ScenarioRunner{}
				task := &Task{
					ScenarioPath: scenarioPath,
					YAML:         false,
				}

				scenario, _, err := runner.loadScenarioAndDevices(task)
				if err != nil {
					errors <- err.Error()
					continue
				}

				if scenario.ScenarioName != expectedName {
					errors <- "goroutine " + string(rune(goroutineID)) +
						": expected " + expectedName + ", got " + scenario.ScenarioName
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect all errors
	var errorList []string
	for err := range errors {
		errorList = append(errorList, err)
	}

	if len(errorList) > 0 {
		t.Errorf("Found %d errors in parallel loading:\n", len(errorList))
		for i, err := range errorList {
			if i < 10 { // Only show first 10 errors
				t.Errorf("  - %s", err)
			}
		}
	}
}
