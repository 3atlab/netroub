package executor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/3atlab/netroub/pkg/events"
	"github.com/3atlab/netroub/pkg/model"
	"github.com/3atlab/netroub/pkg/network"
	"github.com/3atlab/netroub/pkg/runtime"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// scenarioLoadMu protects the scenario loading process.
// This is necessary because model.ReadYaml/ReadJsonScenar use global variables
// (os.Args, model.Scenar, model.Devices) that would cause race conditions
// when multiple scenarios are loaded in parallel.
var scenarioLoadMu sync.Mutex

// ScenarioRunner executes a single scenario task.
type ScenarioRunner struct {
	CLIContext *cli.Context
	QuietMode  bool // When true, suppress stdout logging (file only)
}

// NewScenarioRunner creates a new ScenarioRunner.
func NewScenarioRunner(c *cli.Context) *ScenarioRunner {
	return &ScenarioRunner{CLIContext: c}
}

// SetQuietMode enables or disables quiet mode.
// In quiet mode, logrus output goes only to control.log, not stdout.
func (r *ScenarioRunner) SetQuietMode(quiet bool) {
	r.QuietMode = quiet
}

// Run executes a single scenario task.
func (r *ScenarioRunner) Run(task *Task) error {
	result := r.RunWithResult(task, time.Now())
	return result.Error
}

// RunWithResult executes a single scenario task and returns detailed result.
func (r *ScenarioRunner) RunWithResult(task *Task, startTime time.Time) TaskRunnerResult {
	// Use task.RunID directly as lab name to avoid global state race conditions
	labName := task.RunID

	// Load scenario and devices (protected by mutex due to global state)
	scenario, devices, err := r.loadScenarioAndDevices(task)
	if err != nil {
		return TaskRunnerResult{Error: fmt.Errorf("failed to load scenario: %w", err)}
	}

	// Calculate log directory path using labName
	logDir := scenario.TrialLogDirectoryWithLabName(startTime, labName)

	// Setup logging to control.log
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("failed to create log directory: %w", err)}
	}
	controlLogPath := filepath.Join(logDir, "control.log")
	controlLogFile, err := os.Create(controlLogPath)
	if err != nil {
		return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("failed to create control.log: %w", err)}
	}
	defer controlLogFile.Close()

	// Configure logrus output based on QuietMode
	// IMPORTANT: Always use os.Stdout as the original output, not logrus.StandardLogger().Out.
	// In parallel execution, another worker might have changed logrus output to include
	// a file that is now closed, causing "file already closed" errors when we try to
	// write to the MultiWriter that references the closed file.
	if r.QuietMode {
		logrus.SetOutput(controlLogFile) // File only
	} else {
		logrus.SetOutput(io.MultiWriter(os.Stdout, controlLogFile)) // Stdout + file
	}
	defer logrus.SetOutput(os.Stdout) // Always restore to stdout

	// Check if we should skip deploy (noDeploy mode: when topo is empty)
	noDeploy := scenario.Topo == ""

	// Validate hosts (skip in noDeploy mode - no device data available)
	if !noDeploy {
		if err := validateHosts(scenario, devices); err != nil {
			return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("host validation failed: %w", err)}
		}
	}

	// Add dummy event for duration control
	scenario.Event = append(scenario.Event, model.Event{
		BeginTime: "0s",
		Type:      model.EventTypeDummy,
	})

	// Create runner and controllers
	cmdRunner := runtime.NewExecRunner()

	eventExecutor := events.NewEventExecutor(scenario, devices, labName, cmdRunner)
	eventExecutor.SetTrialLogDir(logDir)

	if noDeploy {
		logrus.Info("No topology specified, running in noDeploy mode (events only)")
	} else {
		networkController := network.NewNetworkController(scenario, devices, labName, cmdRunner)

		// Create Docker client for Pumba
		if err := network.CreateDockerClient(r.CLIContext); err != nil {
			return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("failed to create Docker client: %w", err)}
		}

		// Deploy network
		if err := networkController.Deploy(); err != nil {
			return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("failed to deploy network: %w", err)}
		}

		// Ensure cleanup on exit
		defer func() {
			if err := networkController.Destroy(); err != nil {
				logrus.Errorf("Failed to destroy network: %v", err)
			}
		}()

		// Setup tcpdump
		for _, node := range scenario.Hosts {
			if err := networkController.SetupTcpdump(node); err != nil {
				return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("failed to setup tcpdump on %s: %w", node, err)}
			}
		}

		// Execute events and collect logs
		defer func() {
			if err := r.collectLogs(scenario, networkController, logDir); err != nil {
				logrus.Warnf("Log collection failed: %v", err)
			}
		}()
	}

	// Create Docker client for Pumba (needed even in noDeploy mode for pumba events)
	if noDeploy {
		if err := network.CreateDockerClient(r.CLIContext); err != nil {
			return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("failed to create Docker client: %w", err)}
		}
	}

	// Execute events
	if err := r.executeEvents(scenario, eventExecutor); err != nil {
		return TaskRunnerResult{LogDir: logDir, Error: fmt.Errorf("event execution failed: %w", err)}
	}

	return TaskRunnerResult{LogDir: logDir, Error: nil}
}

// loadScenarioAndDevices loads the scenario and device data from files.
// This function is protected by a mutex because model.ReadYaml/ReadJsonScenar
// and model.ReadJsonData use global variables (os.Args, model.Scenar, model.Devices).
// Without the mutex, parallel scenario loading would cause race conditions.
func (r *ScenarioRunner) loadScenarioAndDevices(task *Task) (*model.Scenario, *model.Data, error) {
	scenarioLoadMu.Lock()
	defer scenarioLoadMu.Unlock()

	// Set the scenario path for model package
	os.Args = []string{"netroub", task.ScenarioPath}

	// Load scenario
	if task.YAML {
		if err := model.ReadYaml(); err != nil {
			return nil, nil, err
		}
	} else {
		if err := model.ReadJsonScenar(); err != nil {
			return nil, nil, err
		}
	}

	// Load device data (skip if no data file specified - noDeploy mode)
	if model.Scenar.Data != "" {
		if err := model.ReadJsonData(); err != nil {
			return nil, nil, err
		}
	}

	// Return copies to avoid global state issues
	scenario := model.Scenar
	devices := model.Devices
	return &scenario, &devices, nil
}

// validateHosts validates that all hosts exist in the topology.
func validateHosts(scenario *model.Scenario, devices *model.Data) error {
	for _, host := range scenario.Hosts {
		found := false
		for _, node := range devices.Nodes {
			if host == node.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("host %s not found in topology", host)
		}
	}
	return nil
}

// executeEvents executes all scenario events.
func (r *ScenarioRunner) executeEvents(scenario *model.Scenario, executor *events.EventExecutor) error {
	done := make(chan error, len(scenario.Event))

	// Parse begin times
	beginTimes := make([]time.Duration, len(scenario.Event))
	for i, event := range scenario.Event {
		if event.BeginTime == "" {
			beginTimes[i] = 0
		} else {
			dur, err := time.ParseDuration(event.BeginTime)
			if err != nil {
				return fmt.Errorf("invalid begin time for event %d: %w", i, err)
			}
			beginTimes[i] = dur
		}
	}

	// Execute events concurrently
	for i := range scenario.Event {
		go func(index int) {
			time.Sleep(beginTimes[index])
			err := executor.Execute(index)
			done <- err
		}(i)
	}

	// Wait for all events
	var lastError error
	for range scenario.Event {
		if err := <-done; err != nil {
			lastError = err
			logrus.Warnf("Event execution error: %v", err)
		}
	}

	return lastError
}

// collectLogs collects logs from the scenario execution.
func (r *ScenarioRunner) collectLogs(scenario *model.Scenario, controller *network.NetworkController, trialLogDir string) error {
	topoPath := filepath.Dir(scenario.Topo)

	// Get initial file sizes for comparison
	initialSizes := make(map[string]int64)
	initialSizes, err := model.StockInitialSize(initialSizes, topoPath)
	if err != nil {
		return fmt.Errorf("failed to get initial file sizes: %w", err)
	}

	// Find changed log files
	logFiles, err := network.SearchFiles(initialSizes, topoPath)
	if err != nil {
		return fmt.Errorf("failed to search log files: %w", err)
	}

	// Collect tcpdump logs
	if err := controller.CollectTcpdumpLogs(); err != nil {
		return fmt.Errorf("failed to collect tcpdump logs: %w", err)
	}

	// Move log files to trial log directory
	if err := controller.MoveLogFilesToDir(logFiles, trialLogDir); err != nil {
		return fmt.Errorf("failed to move log files: %w", err)
	}

	// Flush log files
	if err := network.FlushLogFiles(logFiles); err != nil {
		return fmt.Errorf("failed to flush log files: %w", err)
	}

	return nil
}

