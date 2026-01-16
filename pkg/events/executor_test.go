package events

import (
	"errors"
	"strings"
	"testing"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/stretchr/testify/assert"
)

// mockRunner records command calls for testing
type mockRunner struct {
	calls  [][]string // recorded calls: [[name, arg1, arg2, ...], ...]
	err    error      // error to return
	output []byte     // output to return
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	return m.output, m.err
}

func (m *mockRunner) RunDetached(name string, args ...string) error {
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	return m.err
}

// callContains checks if any recorded call contains all the specified substrings
func (m *mockRunner) callContains(substrings ...string) bool {
	for _, call := range m.calls {
		joined := strings.Join(call, " ")
		allFound := true
		for _, sub := range substrings {
			if !strings.Contains(joined, sub) {
				allFound = false
				break
			}
		}
		if allFound {
			return true
		}
	}
	return false
}

// --- EventExecutor Tests ---

func TestEventExecutor_Execute_Shell(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:          model.EventTypeShell,
				Host:          "r1",
				ShellCommands: []string{"echo hello"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mock.calls))
	// Verify docker exec command was called
	assert.True(t, mock.callContains("sh", "-c", "docker exec clab-test-lab-r1"))
}

func TestEventExecutor_Execute_Shell_MultipleHosts(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:          model.EventTypeShell,
				Hosts:         []string{"r1", "r2"},
				ShellCommands: []string{"echo hello"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(mock.calls))
	assert.True(t, mock.callContains("clab-test-lab-r1"))
	assert.True(t, mock.callContains("clab-test-lab-r2"))
}

func TestEventExecutor_Execute_Shell_MultipleCommands(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:          model.EventTypeShell,
				Host:          "r1",
				ShellCommands: []string{"echo hello", "echo world"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(mock.calls))
}

func TestEventExecutor_Execute_Copy_ToContainer(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type: model.EventTypeCopy,
				Host: "r1",
				ToContainer: []model.FileCopy{
					{Src: "./config.conf", Dst: "/etc/frr/"},
				},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mock.calls))
	assert.True(t, mock.callContains("docker", "cp", "./config.conf", "clab-test-lab-r1:/etc/frr/"))
}

func TestEventExecutor_Execute_Copy_WithOwner(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type: model.EventTypeCopy,
				Host: "r1",
				ToContainer: []model.FileCopy{
					{Src: "./config.conf", Dst: "/etc/frr/", Owner: "frr:frr"},
				},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	// docker cp + docker exec chown
	assert.Equal(t, 2, len(mock.calls))
	assert.True(t, mock.callContains("docker", "cp"))
	assert.True(t, mock.callContains("docker", "exec", "chown", "frr:frr"))
}

func TestEventExecutor_Execute_Copy_WithOwnerAndMode(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type: model.EventTypeCopy,
				Host: "r1",
				ToContainer: []model.FileCopy{
					{Src: "./config.conf", Dst: "/etc/frr/", Owner: "frr:frr", Mode: "644"},
				},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	// docker cp + docker exec chown + docker exec chmod
	assert.Equal(t, 3, len(mock.calls))
	assert.True(t, mock.callContains("docker", "cp"))
	assert.True(t, mock.callContains("chown", "frr:frr"))
	assert.True(t, mock.callContains("chmod", "644"))
}

func TestEventExecutor_Execute_Config_Vtysh(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:         model.EventTypeConfig,
				Host:         "r1",
				VtyshChanges: []string{"conf t", "router bgp 65001", "neighbor 10.0.0.2 remote-as 65002"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mock.calls))
	// Verify vtysh command with multiple -c options
	assert.True(t, mock.callContains("sudo", "docker", "exec", "clab-test-lab-r1", "vtysh"))
	assert.True(t, mock.callContains("-c", "conf t"))
	assert.True(t, mock.callContains("-c", "router bgp 65001"))
}

func TestEventExecutor_Execute_Dummy(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Duration: "10ms", // Short duration for test
		Event: []model.Event{
			{
				Type: model.EventTypeDummy,
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	// Dummy event should not call any commands
	assert.Equal(t, 0, len(mock.calls))
}

func TestEventExecutor_Execute_InvalidType(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type: "invalid_type",
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event type")
}

func TestEventExecutor_Execute_CommandError(t *testing.T) {
	mock := &mockRunner{
		err: errors.New("container not found"),
	}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:         model.EventTypeConfig,
				Host:         "r1",
				VtyshChanges: []string{"conf t"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	// Config event should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")
}

func TestEventExecutor_ClabHostName(t *testing.T) {
	executor := &EventExecutor{LabName: "my-lab"}

	assert.Equal(t, "clab-my-lab-r1", executor.ClabHostName("r1"))
	assert.Equal(t, "clab-my-lab-router", executor.ClabHostName("router"))
}

func TestEventExecutor_Execute_Shell_CustomShell(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:          model.EventTypeShell,
				Host:          "r1",
				ShellPath:     "/bin/bash",
				ShellCommands: []string{"echo hello"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	err := executor.Execute(0)

	assert.NoError(t, err)
	assert.True(t, mock.callContains("/bin/bash"))
}

func TestEventExecutor_Execute_Collect(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:  model.EventTypeCollect,
				Host:  "r1",
				Files: []string{"/var/log/frr/frr.log", "/tmp/result.txt"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	executor.SetTrialLogDir("/tmp/test-logs")
	err := executor.Execute(0)

	assert.NoError(t, err)
	// Should have 2 docker cp calls (one per file)
	assert.Equal(t, 2, len(mock.calls))
	assert.True(t, mock.callContains("docker", "cp", "clab-test-lab-r1:/var/log/frr/frr.log"))
	assert.True(t, mock.callContains("docker", "cp", "clab-test-lab-r1:/tmp/result.txt"))
}

func TestEventExecutor_Execute_Collect_MultipleHosts(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:  model.EventTypeCollect,
				Hosts: []string{"r1", "r2"},
				Files: []string{"/var/log/frr/frr.log"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	executor.SetTrialLogDir("/tmp/test-logs")
	err := executor.Execute(0)

	assert.NoError(t, err)
	// Should have 2 docker cp calls (one per host)
	assert.Equal(t, 2, len(mock.calls))
	assert.True(t, mock.callContains("docker", "cp", "clab-test-lab-r1:/var/log/frr/frr.log"))
	assert.True(t, mock.callContains("docker", "cp", "clab-test-lab-r2:/var/log/frr/frr.log"))
}

func TestEventExecutor_Execute_Collect_NoTrialLogDir(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Event: []model.Event{
			{
				Type:  model.EventTypeCollect,
				Host:  "r1",
				Files: []string{"/var/log/frr/frr.log"},
			},
		},
	}
	devices := &model.Data{}

	executor := NewEventExecutor(scenario, devices, "test-lab", mock)
	// TrialLogDir is not set
	err := executor.Execute(0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TrialLogDir is not set")
}
