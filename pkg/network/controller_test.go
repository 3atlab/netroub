package network

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

// --- NetworkController Tests ---

func TestNetworkController_Deploy(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Topo: "/path/to/topo.yaml",
	}
	devices := &model.Data{}

	controller := NewNetworkController(scenario, devices, "test-lab", mock)
	err := controller.Deploy()

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mock.calls))
	assert.True(t, mock.callContains("sudo", "containerlab", "deploy"))
	assert.True(t, mock.callContains("--name", "test-lab"))
	assert.True(t, mock.callContains("--topo", "/path/to/topo.yaml"))
}

func TestNetworkController_Deploy_Error(t *testing.T) {
	mock := &mockRunner{
		err:    errors.New("containerlab not found"),
		output: []byte("command not found: containerlab"),
	}
	scenario := &model.Scenario{
		Topo: "/path/to/topo.yaml",
	}
	devices := &model.Data{}

	controller := NewNetworkController(scenario, devices, "test-lab", mock)
	err := controller.Deploy()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "containerlab deploy failed")
}

func TestNetworkController_Destroy(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Topo: "/path/to/topo.yaml",
	}
	devices := &model.Data{}

	controller := NewNetworkController(scenario, devices, "test-lab", mock)
	err := controller.Destroy()

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mock.calls))
	assert.True(t, mock.callContains("sudo", "containerlab", "destroy"))
	assert.True(t, mock.callContains("--name", "test-lab"))
	assert.True(t, mock.callContains("--cleanup"))
	// Note: --topo is intentionally NOT passed to destroy to avoid
	// containerlab recreating default network settings
}

func TestNetworkController_Destroy_Error(t *testing.T) {
	mock := &mockRunner{
		err:    errors.New("lab not found"),
		output: []byte("Error: lab test-lab not found"),
	}
	scenario := &model.Scenario{
		Topo: "/path/to/topo.yaml",
	}
	devices := &model.Data{}

	controller := NewNetworkController(scenario, devices, "test-lab", mock)
	err := controller.Destroy()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "containerlab destroy failed")
}

func TestNetworkController_CollectTcpdumpLogs(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Topo:  "/path/to/topo.yaml",
		Hosts: []string{"r1", "r2"},
	}
	devices := &model.Data{}

	controller := NewNetworkController(scenario, devices, "test-lab", mock)
	err := controller.CollectTcpdumpLogs()

	assert.NoError(t, err)
	// docker cp for each host
	assert.Equal(t, 2, len(mock.calls))
	assert.True(t, mock.callContains("sudo", "docker", "cp", "clab-test-lab-r1:/tcpdump"))
	assert.True(t, mock.callContains("sudo", "docker", "cp", "clab-test-lab-r2:/tcpdump"))
}

func TestNetworkController_ClabHostName(t *testing.T) {
	controller := &NetworkController{LabName: "my-lab"}

	assert.Equal(t, "clab-my-lab-r1", controller.ClabHostName("r1"))
	assert.Equal(t, "clab-my-lab-router", controller.ClabHostName("router"))
}

func TestNetworkController_Deploy_CustomLabName(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		Topo: "/path/to/topo.yaml",
	}
	devices := &model.Data{}

	// Test with custom lab name (for parallel execution)
	controller := NewNetworkController(scenario, devices, "A1_delay_pause_001", mock)
	err := controller.Deploy()

	assert.NoError(t, err)
	assert.True(t, mock.callContains("--name", "A1_delay_pause_001"))
}

// --- Scenario Flow Test ---

func TestScenarioFlow_DeployExecuteDestroy(t *testing.T) {
	mock := &mockRunner{}
	scenario := &model.Scenario{
		ScenarioName: "test-scenario",
		Topo:         "/path/to/topo.yaml",
		Hosts:        []string{"r1"},
	}
	devices := &model.Data{}

	controller := NewNetworkController(scenario, devices, "test-lab", mock)

	// Simulate the flow: Deploy -> CollectLogs -> Destroy
	err := controller.Deploy()
	assert.NoError(t, err)

	err = controller.CollectTcpdumpLogs()
	assert.NoError(t, err)

	err = controller.Destroy()
	assert.NoError(t, err)

	// Verify call order
	assert.Equal(t, 3, len(mock.calls))

	// First call: containerlab deploy
	assert.True(t, strings.Contains(strings.Join(mock.calls[0], " "), "containerlab deploy"))

	// Second call: docker cp (collect tcpdump)
	assert.True(t, strings.Contains(strings.Join(mock.calls[1], " "), "docker cp"))

	// Third call: containerlab destroy
	assert.True(t, strings.Contains(strings.Join(mock.calls[2], " "), "containerlab destroy"))
}
