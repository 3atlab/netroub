// Package runtime provides abstractions for external command execution.
// This enables testing without actual Docker/containerlab dependencies.
package runtime

import (
	"os/exec"
)

// CommandRunner abstracts shell command execution.
// Implementations can execute real commands or provide mock behavior for testing.
type CommandRunner interface {
	// Run executes a command and returns combined stdout/stderr output.
	Run(name string, args ...string) ([]byte, error)

	// RunDetached executes a command in the background (detached mode).
	// It does not wait for the command to complete.
	RunDetached(name string, args ...string) error
}

// ExecRunner is the production implementation of CommandRunner.
// It executes actual shell commands using os/exec.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner instance.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes a command and returns combined stdout/stderr output.
func (r *ExecRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// RunDetached executes a command in the background.
// It starts the command but does not wait for it to complete.
func (r *ExecRunner) RunDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Start()
}
