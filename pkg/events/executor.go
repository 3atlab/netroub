// Package events provides event execution for netroub scenarios.
package events

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/3atlab/netroub/pkg/runtime"
	"github.com/sirupsen/logrus"
)

// EventExecutor executes scenario events with injected dependencies.
// This enables testing without actual Docker/containerlab.
type EventExecutor struct {
	Scenario    *model.Scenario
	Devices     *model.Data
	LabName     string
	Runner      runtime.CommandRunner
	TrialLogDir string // Log directory for the current trial (for collect event)
}

// NewEventExecutor creates a new EventExecutor instance.
func NewEventExecutor(scenario *model.Scenario, devices *model.Data, labName string, runner runtime.CommandRunner) *EventExecutor {
	return &EventExecutor{
		Scenario: scenario,
		Devices:  devices,
		LabName:  labName,
		Runner:   runner,
	}
}

// SetTrialLogDir sets the log directory for the current trial.
func (e *EventExecutor) SetTrialLogDir(logDir string) {
	e.TrialLogDir = logDir
}

// ClabHostName returns the containerlab container name for a host.
func (e *EventExecutor) ClabHostName(host string) string {
	return "clab-" + e.LabName + "-" + host
}

// Execute runs the event at the given index.
func (e *EventExecutor) Execute(index int) error {
	event := e.Scenario.Event[index]
	switch event.Type {
	case model.EventTypeDummy:
		return e.execDummy(index)
	case model.EventTypePumba:
		return e.execPumba(index)
	case model.EventTypeShell:
		return e.execShell(index)
	case model.EventTypeConfig:
		return e.execConfig(index)
	case model.EventTypeCopy:
		return e.execCopy(index)
	case model.EventTypeCollect:
		return e.execCollect(index)
	default:
		return fmt.Errorf("invalid event type %s", event.Type)
	}
}

// execDummy waits for the scenario duration (dummy event).
func (e *EventExecutor) execDummy(index int) error {
	dur, err := time.ParseDuration(e.Scenario.Duration)
	if err != nil {
		return err
	}
	if dur.Seconds() > 0 {
		time.Sleep(dur)
	}
	return nil
}

// execShell executes shell commands in containers.
func (e *EventExecutor) execShell(index int) error {
	event := e.Scenario.Event[index]
	shell := event.ShellPath
	if shell == "" {
		shell = "/bin/sh"
	}

	for _, host := range event.GetHosts() {
		containerName := e.ClabHostName(host)
		for _, shellCommand := range event.ShellCommands {
			escapedCommand := strings.ReplaceAll(shellCommand, `'`, `'"'"'`)
			input := fmt.Sprintf(`docker exec %s %s -c '%s'`, containerName, shell, escapedCommand)

			logrus.Debugf("Event %d: Execute command: sh -c %s", index, input)
			_, err := e.Runner.Run("sh", "-c", input)
			if err != nil {
				logrus.Warnf("Error while running %s: %s", shellCommand, err)
			}
		}
	}
	return nil
}

// execCopy executes file copy operations between host and containers.
func (e *EventExecutor) execCopy(index int) error {
	event := e.Scenario.Event[index]

	for _, host := range event.GetHosts() {
		containerName := e.ClabHostName(host)

		// Process toContainer (host -> container)
		for _, fc := range event.ToContainer {
			if err := e.copyToContainer(index, containerName, fc); err != nil {
				logrus.Warnf("Error copying to container %s: %s", containerName, err)
			}
		}

		// Process fromContainer (container -> host)
		for _, fc := range event.FromContainer {
			if err := e.copyFromContainer(index, containerName, fc); err != nil {
				logrus.Warnf("Error copying from container %s: %s", containerName, err)
			}
		}
	}
	return nil
}

// copyToContainer copies a file from host to container.
func (e *EventExecutor) copyToContainer(index int, containerName string, fc model.FileCopy) error {
	dst := fmt.Sprintf("%s:%s", containerName, fc.Dst)
	logrus.Debugf("Event %d: Execute docker cp %s %s", index, fc.Src, dst)

	output, err := e.Runner.Run("docker", "cp", fc.Src, dst)
	if err != nil {
		return fmt.Errorf("docker cp from %s to %s failed: %w, output: %s", fc.Src, dst, err, strings.TrimSpace(string(output)))
	}

	// Determine the destination path for chown/chmod
	dstPath := fc.Dst
	if strings.HasSuffix(fc.Dst, "/") {
		dstPath = filepath.Join(fc.Dst, filepath.Base(fc.Src))
	}

	// Apply owner if specified
	if fc.Owner != "" {
		logrus.Debugf("Event %d: Execute docker exec %s chown %s %s", index, containerName, fc.Owner, dstPath)
		output, err := e.Runner.Run("docker", "exec", containerName, "chown", fc.Owner, dstPath)
		if err != nil {
			return fmt.Errorf("chown failed: %s, output: %s", err, string(output))
		}
	}

	// Apply mode if specified
	if fc.Mode != "" {
		logrus.Debugf("Event %d: Execute docker exec %s chmod %s %s", index, containerName, fc.Mode, dstPath)
		output, err := e.Runner.Run("docker", "exec", containerName, "chmod", fc.Mode, dstPath)
		if err != nil {
			return fmt.Errorf("chmod failed: %s, output: %s", err, string(output))
		}
	}

	return nil
}

// copyFromContainer copies a file from container to host.
func (e *EventExecutor) copyFromContainer(index int, containerName string, fc model.FileCopy) error {
	// Ensure destination directory exists
	dstDir := fc.Dst
	if !strings.HasSuffix(fc.Dst, "/") {
		dstDir = filepath.Dir(fc.Dst)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	src := fmt.Sprintf("%s:%s", containerName, fc.Src)
	logrus.Debugf("Event %d: Execute docker cp %s %s", index, src, fc.Dst)

	output, err := e.Runner.Run("docker", "cp", src, fc.Dst)
	if err != nil {
		return fmt.Errorf("docker cp from %s to %s failed: %w, output: %s", src, fc.Dst, err, strings.TrimSpace(string(output)))
	}

	// Determine the destination path for chown/chmod
	dstPath := fc.Dst
	if strings.HasSuffix(fc.Dst, "/") {
		dstPath = filepath.Join(fc.Dst, filepath.Base(fc.Src))
	}

	// Apply owner if specified (on host side)
	if fc.Owner != "" {
		logrus.Debugf("Event %d: Execute chown %s %s", index, fc.Owner, dstPath)
		output, err := e.Runner.Run("chown", fc.Owner, dstPath)
		if err != nil {
			return fmt.Errorf("chown failed: %s, output: %s", err, string(output))
		}
	}

	// Apply mode if specified (on host side)
	if fc.Mode != "" {
		logrus.Debugf("Event %d: Execute chmod %s %s", index, fc.Mode, dstPath)
		output, err := e.Runner.Run("chmod", fc.Mode, dstPath)
		if err != nil {
			return fmt.Errorf("chmod failed: %s, output: %s", err, string(output))
		}
	}

	return nil
}

// execConfig executes configuration changes (vtysh or config file).
func (e *EventExecutor) execConfig(index int) error {
	event := e.Scenario.Event[index]

	if event.VtyshChanges != nil {
		if err := e.execVtyshChanges(index); err != nil {
			return err
		}
	}
	if event.ConfigFileChanges != nil {
		if err := e.execConfigFileChanges(index); err != nil {
			return err
		}
	}
	return nil
}

// execVtyshChanges executes vtysh commands.
func (e *EventExecutor) execVtyshChanges(index int) error {
	event := e.Scenario.Event[index]
	containerName := e.ClabHostName(event.Host)

	// Build vtysh command with multiple -c options
	args := []string{"docker", "exec", containerName, "vtysh"}
	for _, vtyCommand := range event.VtyshChanges {
		args = append(args, "-c", vtyCommand)
		logrus.WithFields(logrus.Fields{
			"command":   vtyCommand,
			"container": event.Host,
		}).Debug("Adding vtysh command")
	}

	logrus.Debugf("Event %d: Execute sudo %s", index, strings.Join(args, " "))
	output, err := e.Runner.Run("sudo", args...)
	if err != nil {
		return fmt.Errorf("failed to run vtysh command on %s: %w, command: sudo %s, output: %s",
			containerName, err, strings.Join(args, " "), strings.TrimSpace(string(output)))
	}

	logrus.Info("configuration changes applied")
	return nil
}

// execConfigFileChanges modifies configuration files.
func (e *EventExecutor) execConfigFileChanges(index int) error {
	event := e.Scenario.Event[index]
	host := event.Host

	for _, modif := range event.ConfigFileChanges {
		topoPath := e.findTopoPath()
		filePath := topoPath + host + "/" + modif.File

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("error opening config file %s: %w", filePath, err)
		}
		defer file.Close()

		byteArray, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("error reading config file: %w", err)
		}

		configFile := strings.Split(string(byteArray), "\n")
		configFile[modif.Line-1] = modif.Command
		writeString := strings.Join(configFile, "\n")

		err = os.WriteFile(filePath, []byte(writeString), 0666)
		if err != nil {
			return fmt.Errorf("error writing changes to config file: %w", err)
		}
	}

	return nil
}

// findTopoPath returns the directory path of the topology file.
func (e *EventExecutor) findTopoPath() string {
	return filepath.Dir(e.Scenario.Topo) + "/"
}

// execPumba executes Pumba chaos commands.
// Note: This method currently delegates to the global Pumba functions
// because Pumba has its own dependency injection (chaos.DockerClient).
// Full integration with EventExecutor would require refactoring Pumba usage.
func (e *EventExecutor) execPumba(index int) error {
	// For now, delegate to the existing implementation
	// This maintains compatibility while we migrate other events
	return ExecPumbaCommand(index)
}

// PumbaClient interface for future Pumba abstraction
type PumbaClient interface {
	RunNetem(ctx context.Context, containers []string, params interface{}) error
	RunStress(ctx context.Context, container string, params interface{}) error
}

// execCollect collects files from containers to the trial log directory.
func (e *EventExecutor) execCollect(index int) error {
	event := e.Scenario.Event[index]

	if e.TrialLogDir == "" {
		return fmt.Errorf("TrialLogDir is not set for collect event")
	}

	for _, host := range event.GetHosts() {
		containerName := e.ClabHostName(host)
		hostLogDir := filepath.Join(e.TrialLogDir, host)

		// Ensure host log directory exists
		if err := os.MkdirAll(hostLogDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory %s: %w", hostLogDir, err)
		}

		for _, file := range event.Files {
			if err := e.collectFile(index, containerName, file, hostLogDir); err != nil {
				logrus.Warnf("Error collecting file %s from %s: %v", file, containerName, err)
			}
		}
	}
	return nil
}

// collectFile copies a single file from container to the host log directory.
func (e *EventExecutor) collectFile(index int, containerName, srcPath, hostLogDir string) error {
	src := fmt.Sprintf("%s:%s", containerName, srcPath)
	dst := filepath.Join(hostLogDir, filepath.Base(srcPath))

	logrus.Debugf("Event %d: Collect docker cp %s %s", index, src, dst)

	output, err := e.Runner.Run("docker", "cp", src, dst)
	if err != nil {
		return fmt.Errorf("docker cp from %s to %s failed: %w, output: %s",
			src, dst, err, strings.TrimSpace(string(output)))
	}

	return nil
}
