package model

import (
	"fmt"
	"sync"
)

type Connections struct {
	SrcNode      string `json:"src_node"`
	SrcInterface string `json:"src_interface"`
	DstNode      string `json:"dst_node"`
	DstInterface string `json:"dst_interface"`
}

type ParamsInterface struct {
	Name     string `json:"name"`
	Priority string `json:"priority"`
}

type Interfaces struct {
	Name   string          `json:"name"`
	Params ParamsInterface `json:"params"`
}

type Params struct {
	As   string `json:"as"`
	Name string `json:"name"`
}

type Nodes struct {
	Name       string       `json:"name"`
	Params     Params       `json:"params"`
	Interfaces []Interfaces `json:"interfaces"`
}

type Data struct {
	Name        string        `json:"name"`
	Nodes       []Nodes       `json:"nodes"`
	Connections []Connections `json:"connections"`
}

var Devices Data

// labName is the custom lab name for containerlab.
// If empty, Devices.Name (topology name) is used.
// Access is protected by labNameMu for concurrent safety.
var (
	labName   string
	labNameMu sync.RWMutex
)

// GetLabName returns the lab name to use for containerlab.
// Returns custom labName if set, otherwise returns Devices.Name.
// Falls back to Scenar.ScenarioName for noDeploy mode where Devices is not loaded.
// Thread-safe for concurrent access.
func GetLabName() string {
	labNameMu.RLock()
	defer labNameMu.RUnlock()
	if labName != "" {
		return labName
	}
	if Devices.Name != "" {
		return Devices.Name
	}
	// Fallback for noDeploy mode
	return Scenar.ScenarioName
}

// SetLabName sets a custom lab name for containerlab.
// Thread-safe for concurrent access.
func SetLabName(name string) {
	labNameMu.Lock()
	defer labNameMu.Unlock()
	labName = name
}

// ResetLabName clears the custom lab name.
// Thread-safe for concurrent access.
func ResetLabName() {
	labNameMu.Lock()
	defer labNameMu.Unlock()
	labName = ""
}

func ValidateHostNames(hosts []string) error {
	for _, host := range hosts {
		ok := false
		for _, device := range Devices.Nodes {
			if host == device.Name {
				ok = true
			}
		}
		if !ok {
			return fmt.Errorf("host %s not found in the topology", host)
		}
	}
	return nil
}

func ClabHostName(host string) string {
	return "clab-" + GetLabName() + "-" + host
}

func GetDeviceIndex(device string) int {
	for i, node := range Devices.Nodes {
		if device == node.Name {
			return i
		}
	}
	return -1
}
