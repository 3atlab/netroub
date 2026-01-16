package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"

	"github.com/3atlab/netroub/pkg/events"
	"github.com/3atlab/netroub/pkg/executor"
	"github.com/3atlab/netroub/pkg/model"
	"github.com/3atlab/netroub/pkg/network"
	"github.com/3atlab/netroub/pkg/runtime"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var initalSizes map[string]int64

// Shared instances for scenario execution
var (
	cmdRunner         runtime.CommandRunner
	eventExecutor     *events.EventExecutor
	networkController *network.NetworkController
)

type ConsoleHook struct{}

func (h *ConsoleHook) Fire(entry *logrus.Entry) error {
	if entry.Level <= logrus.InfoLevel {
		t := entry.Time
		fmt.Print(color.BlueString("[INFO]"))
		fmt.Print(t.Format("2006-01-02 15:04:05 "))
		fmt.Print(color.GreenString(entry.Message), " ")
		if entry.Data["command"] != nil {
			fmt.Println(entry.Data["command"], "duration :", entry.Data["duration"])
		} else {
			fmt.Println("")
		}
	}
	return nil
}
func (h *ConsoleHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
func NewConsoleHook() *ConsoleHook {
	return &ConsoleHook{}
}

func main() {
	app := cli.NewApp()
	app.Name = "Netroub"
	app.Usage = "Netroub is a synthetic data generator from network trouble scenarios"
	app.Version = "0.1.0"
	app.Authors = []cli.Author{
		{
			Name:  "Colin Regal-Mezin",
			Email: "colin.regalmezin@gmail.com",
		},
		{
			Name:  "Satoru Kobayashi",
			Email: "sat@okayama-u.ac.jp",
		},
	}
	app.EnableBashCompletion = true

	// Global flags
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "yaml",
			Usage: "Use a .yaml scenario file",
		},
	}

	// Default action (backward compatible: netroub scenario.json)
	app.Action = func(c *cli.Context) error {
		if c.NArg() == 0 {
			return cli.ShowAppHelp(c)
		}
		model.SudoCheck()
		return runScenarioWithAfter(c)
	}

	// Subcommands
	app.Commands = []cli.Command{
		{
			Name:  "run",
			Usage: "Run a single scenario (same as default)",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "yaml",
					Usage: "Use a .yaml scenario file",
				},
			},
			Action: func(c *cli.Context) error {
				model.SudoCheck()
				return runScenarioWithAfter(c)
			},
		},
		{
			Name:  "repeat",
			Usage: "Run a scenario multiple times with optional parallelism",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "n, count",
					Usage: "Number of repetitions",
					Value: 1,
				},
				cli.IntFlag{
					Name:  "p, parallel",
					Usage: "Maximum parallel executions",
					Value: 1,
				},
				cli.BoolFlag{
					Name:  "yaml",
					Usage: "Use a .yaml scenario file",
				},
				cli.BoolFlag{
					Name:  "progress",
					Usage: "Show progress bar instead of detailed logs",
				},
			},
			Action: repeatScenario,
		},
		{
			Name:  "batch",
			Usage: "Run multiple scenarios from a plan file",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "p, parallel",
					Usage: "Override parallel setting in plan file",
					Value: 0,
				},
				cli.BoolFlag{
					Name:  "progress",
					Usage: "Show progress bar instead of detailed logs",
				},
			},
			Action: batchScenario,
		},
		{
			Name:  "clean",
			Usage: "Clean up containers from a plan or scenario file (auto-detects file type)",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "dry-run",
					Usage: "Show what would be removed without actually removing",
				},
				cli.IntFlag{
					Name:  "n, count",
					Usage: "Number of repetitions (for scenario file only)",
					Value: 0,
				},
			},
			Action: cleanContainers,
		},
	}

	app.Before = before
	app.CustomAppHelpTemplate = model.ConfigTemplate()

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runScenarioWithAfter runs a scenario and then executes cleanup.
func runScenarioWithAfter(c *cli.Context) error {
	err := runScenario(c)
	afterErr := after(c)
	if err != nil {
		return err
	}
	return afterErr
}

// repeatScenario executes a scenario multiple times with parallelism.
func repeatScenario(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("scenario file path is required")
	}

	scenarioPath := c.Args().First()
	count := c.Int("count")
	parallel := c.Int("parallel")
	showProgress := c.Bool("progress")

	// Auto-detect YAML format: explicit flag takes precedence, otherwise detect by extension
	isYAML := c.Bool("yaml") || executor.IsYAMLExtension(scenarioPath)

	if count < 1 {
		return fmt.Errorf("count must be at least 1")
	}
	if parallel < 1 {
		parallel = 1
	}

	model.SudoCheck()

	// Create batch logger
	batchLogger, err := executor.NewBatchLogger(executor.BatchLogFileName)
	if err != nil {
		logrus.Warnf("Failed to create batch log file: %v", err)
	} else {
		defer batchLogger.Close()
		batchLogger.LogStart("repeat", 1, count, parallel, "")
		logrus.Infof("Batch log: %s", batchLogger.GetLogPath())
	}

	logrus.Infof("Starting repeat execution: %s x %d (parallel: %d)", scenarioPath, count, parallel)

	// Generate tasks
	tasks := executor.GenerateTasks(scenarioPath, count, isYAML)

	// Create runner and executor
	runner := executor.NewScenarioRunner(c)
	runner.SetQuietMode(showProgress) // Suppress stdout logging when progress bar is shown
	exec := executor.NewExecutor(parallel, runner)
	if batchLogger != nil {
		exec.SetBatchLogger(batchLogger)
	}

	// Execute all tasks
	results := exec.ExecuteWithProgress(tasks, showProgress)

	// Log summary to batch log
	if batchLogger != nil {
		batchLogger.LogSummary(results)
	}

	// Print summary to stdout
	executor.PrintSummary(results)

	// Return error if any task failed
	_, _, failed, _ := executor.Summary(results)
	if failed > 0 {
		return fmt.Errorf("%d/%d tasks failed", failed, len(results))
	}

	return nil
}

// cleanContainers removes containers from a plan file or scenario file.
// File type (Plan vs Scenario) is detected automatically based on content.
func cleanContainers(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("plan or scenario file path is required")
	}

	filePath := c.Args().First()
	dryRun := c.Bool("dry-run")
	count := c.Int("count")

	model.SudoCheck()

	// Detect file type by content (not extension)
	fileType, err := executor.DetectFileType(filePath)
	if err != nil {
		return fmt.Errorf("failed to detect file type: %w", err)
	}

	var labNames []string

	switch fileType {
	case executor.FileTypePlan:
		// Plan file: load and generate lab names
		plan, err := executor.LoadPlan(filePath)
		if err != nil {
			return fmt.Errorf("failed to load plan: %w", err)
		}

		baseDir := "."
		if absPath, err := filepath.Abs(filePath); err == nil {
			baseDir = filepath.Dir(absPath)
		}

		labNames, err = executor.GenerateLabNamesFromPlan(plan, baseDir)
		if err != nil {
			return fmt.Errorf("failed to generate lab names: %w", err)
		}

		scenarios, totalRuns := plan.Summary()
		logrus.Infof("Cleaning containers for plan: %d scenarios, %d total runs", scenarios, totalRuns)

	case executor.FileTypeScenario:
		// Scenario file: generate lab names for single scenario
		labNames = executor.GenerateLabNamesFromScenario(filePath, count)
		if count > 0 {
			logrus.Infof("Cleaning containers for scenario %s x %d", filePath, count)
		} else {
			logrus.Infof("Cleaning containers for scenario %s", filePath)
		}

	default:
		return fmt.Errorf("unable to determine file type: %s (file should contain 'scenarios' key for plan or 'event'/'scenarioName' key for scenario)", filePath)
	}

	// Clean containers
	removed, err := executor.CleanContainers(labNames, dryRun)
	if err != nil {
		return fmt.Errorf("failed to clean containers: %w", err)
	}

	if dryRun {
		logrus.Infof("Dry run: would remove %d containers", removed)
	} else {
		logrus.Infof("Removed %d containers", removed)
	}

	// Clean Docker networks
	networksRemoved, err := executor.CleanDockerNetworks(labNames, dryRun)
	if err != nil {
		return fmt.Errorf("failed to clean Docker networks: %w", err)
	}

	if dryRun {
		logrus.Infof("Dry run: would remove %d Docker networks", networksRemoved)
	} else if networksRemoved > 0 {
		logrus.Infof("Removed %d Docker networks", networksRemoved)
	}

	return nil
}

// batchScenario executes multiple scenarios from a plan file.
func batchScenario(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("plan file path is required")
	}

	planPath := c.Args().First()
	parallelOverride := c.Int("parallel")
	showProgress := c.Bool("progress")

	model.SudoCheck()

	// Load the plan file
	plan, err := executor.LoadPlan(planPath)
	if err != nil {
		return fmt.Errorf("failed to load plan: %w", err)
	}

	// Override parallel setting if specified
	parallel := plan.Parallel
	if parallelOverride > 0 {
		parallel = parallelOverride
	}

	// Get base directory for relative paths in plan
	baseDir := "."
	if absPath, err := filepath.Abs(planPath); err == nil {
		baseDir = filepath.Dir(absPath)
	}

	// Generate tasks from plan
	tasks, err := executor.GenerateTasksFromPlan(plan, baseDir)
	if err != nil {
		return fmt.Errorf("failed to generate tasks: %w", err)
	}

	scenarios, totalRuns := plan.Summary()

	// Create batch logger
	batchLogger, err := executor.NewBatchLogger(executor.BatchLogFileName)
	if err != nil {
		logrus.Warnf("Failed to create batch log file: %v", err)
	} else {
		defer batchLogger.Close()
		batchLogger.LogStart("batch", scenarios, totalRuns, parallel, planPath)
		logrus.Infof("Batch log: %s", batchLogger.GetLogPath())
	}

	logrus.Infof("Starting batch execution: %d scenarios, %d total runs (parallel: %d)", scenarios, totalRuns, parallel)

	// Create runner and executor
	runner := executor.NewScenarioRunner(c)
	runner.SetQuietMode(showProgress) // Suppress stdout logging when progress bar is shown
	exec := executor.NewExecutor(parallel, runner)
	if batchLogger != nil {
		exec.SetBatchLogger(batchLogger)
	}

	// Execute all tasks
	results := exec.ExecuteWithProgress(tasks, showProgress)

	// Log summary to batch log
	if batchLogger != nil {
		batchLogger.LogSummary(results)
	}

	// Print summary to stdout
	executor.PrintSummary(results)

	// Return error if any task failed
	_, _, failed, _ := executor.Summary(results)
	if failed > 0 {
		return fmt.Errorf("%d/%d tasks failed", failed, len(results))
	}

	return nil
}

func runScenario(c *cli.Context) error {
	var err error

	controlLogFile, err := os.Create("control.log")
	if err != nil {
		fmt.Println("Error while creating control log file")
		return err
	}
	logrus.SetOutput(io.MultiWriter(os.Stdout, controlLogFile))

	// Auto-detect YAML format: explicit flag takes precedence, otherwise detect by extension
	scenarioPath := c.Args().First()
	isYAML := c.Bool("yaml") || c.GlobalBool("yaml") || executor.IsYAMLExtension(scenarioPath)

	// Read the scenario file and sort it by time in an array
	if isYAML {
		err = model.ReadYaml()
		if err != nil {
			return err
		}
	} else {
		err = model.ReadJsonScenar()
		if err != nil {
			return err
		}
	}

	// Check if we should skip deploy (noDeploy mode: when topo is empty)
	noDeploy := model.Scenar.Topo == ""

	// Read the dot2net data json file containing device information (skip in noDeploy mode)
	if !noDeploy {
		err = model.ReadJsonData()
		if err != nil {
			return err
		}
		err = model.ValidateHostNames(model.Scenar.Hosts)
		if err != nil {
			return err
		}
	}

	// Set dummy event to control the whole duration of the scenario
	model.Scenar.Event = append(model.Scenar.Event, model.Event{BeginTime: "0s", Type: model.EventTypeDummy})

	// Initialize runner and controllers
	cmdRunner = runtime.NewExecRunner()
	labName := model.GetLabName()

	eventExecutor = events.NewEventExecutor(&model.Scenar, &model.Devices, labName, cmdRunner)

	// Create the DockerClient which is mandatory for pumba command
	err = network.CreateDockerClient(c)
	if err != nil {
		return err
	}

	if noDeploy {
		logrus.Info("No topology specified, running in noDeploy mode (events only)")
	} else {
		// Stock the size of all the log file present in the directory of the topo file
		path := model.FindTopoPath()
		initalSizes = make(map[string]int64)
		initalSizes, err = model.StockInitialSize(initalSizes, path)
		if err != nil {
			return err
		}

		networkController = network.NewNetworkController(&model.Scenar, &model.Devices, labName, cmdRunner)

		// Emulate the network with Containerlab
		err = networkController.Deploy()
		if err != nil {
			return err
		}

		// Setup tcpdump logging
		for _, node := range model.Scenar.Hosts {
			err = networkController.SetupTcpdump(node)
			if err != nil {
				return err
			}
		}
	}

	// Create a channel to verify routine states
	done := make(chan bool)

	// Load and parse beginTime for each event
	beginTimes := make([]time.Duration, 0, len(model.Scenar.Event))
	for _, event := range model.Scenar.Event {
		var dur time.Duration
		if event.BeginTime == "" {
			dur = time.Duration(0)
		} else {
			dur, err = time.ParseDuration(event.BeginTime)
			if err != nil {
				return err
			}
		}
		beginTimes = append(beginTimes, dur)
	}

	logrus.Debugf("Starting scenario %s\n", model.Scenar.ScenarioName)

	// Run for all the events in the scenario file
	for i := 0; i < len(model.Scenar.Event); i++ {
		logrus.Debugf("Adding new event %d %+v\n", i, model.Scenar.Event[i])
		go func(index int) {
			dur := beginTimes[index]
			if dur.Seconds() > 0 {
				time.Sleep(dur)
			}
			logrus.Debugf("Starting event %d\n", index)

			err := eventExecutor.Execute(index)
			if err != nil {
				logrus.Errorf("Error executing event %d: %v\n", index, err)
			}

			logrus.Debugf("Completed event %d\n", index)

			done <- true
		}(i)
	}

	// Wait here until all routines are finished
	for i := 0; i < len(model.Scenar.Event); i++ {
		<-done
	}

	logrus.Debugf("Completed scenario %s\n", model.Scenar.ScenarioName)

	return nil
}

func before(c *cli.Context) error {
	_ = c.Args() // Suppress unused parameter warning
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05.000", FullTimestamp: true})
	logrus.SetOutput(os.Stdout)

	logrus.AddHook(NewConsoleHook())
	return nil
}

func after(c *cli.Context) error {
	_ = c.Args() // Suppress unused parameter warning

	// Skip cleanup in noDeploy mode (networkController is nil)
	if networkController == nil {
		logrus.Debug("noDeploy mode: skipping cleanup")
		return nil
	}

	// Ensure network is destroyed regardless of errors in subsequent operations
	defer func() {
		if err := networkController.Destroy(); err != nil {
			logrus.Errorf("Failed to destroy network: %v", err)
		}
	}()

	// Find the directory to search log file
	path := model.FindTopoPath()

	// Fill an array with all log file path
	logFiles, err := network.SearchFiles(initalSizes, path)
	if err != nil {
		return err
	}
	logrus.Debugf("Log files: %v\n", logFiles)

	// Move tcpdump log files from containers
	err = networkController.CollectTcpdumpLogs()
	if err != nil {
		return err
	}

	err = networkController.MoveLogFiles(logFiles)
	if err != nil {
		return err
	}

	// Flush log files for the next scenario
	err = network.FlushLogFiles(logFiles)
	if err != nil {
		return err
	}

	return nil
}
