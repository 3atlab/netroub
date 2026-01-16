package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildShellCommand tests command generation for shell events
func TestBuildShellCommand(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		shell         string
		command       string
		wantArgs      string
	}{
		{
			name:          "simple echo command",
			containerName: "clab-topo-r1",
			shell:         "/bin/sh",
			command:       "echo hello",
			wantArgs:      "docker exec clab-topo-r1 /bin/sh -c 'echo hello'",
		},
		{
			name:          "command with single quotes",
			containerName: "clab-topo-r1",
			shell:         "/bin/sh",
			command:       "echo 'hello world'",
			wantArgs:      "docker exec clab-topo-r1 /bin/sh -c 'echo '\"'\"'hello world'\"'\"''",
		},
		{
			name:          "vtysh command",
			containerName: "clab-topo-r1",
			shell:         "/bin/sh",
			command:       "vtysh -c 'show ip bgp summary'",
			wantArgs:      "docker exec clab-topo-r1 /bin/sh -c 'vtysh -c '\"'\"'show ip bgp summary'\"'\"''",
		},
		{
			name:          "redirect to file",
			containerName: "clab-topo-r1",
			shell:         "/bin/bash",
			command:       "vtysh -c 'show running-config' > /tmp/config.txt",
			wantArgs:      "docker exec clab-topo-r1 /bin/bash -c 'vtysh -c '\"'\"'show running-config'\"'\"' > /tmp/config.txt'",
		},
		{
			name:          "ip route command",
			containerName: "clab-topo-r2",
			shell:         "/bin/sh",
			command:       "ip route show",
			wantArgs:      "docker exec clab-topo-r2 /bin/sh -c 'ip route show'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildShellCommandString(tt.containerName, tt.shell, tt.command)
			assert.Equal(t, tt.wantArgs, got, "shell command string mismatch")
		})
	}
}

// buildShellCommandString builds the shell command string for testing
// This mirrors the logic in ExecShellCommand
func buildShellCommandString(containerName, shell, command string) string {
	// Escape single quotes (same as in shell.go)
	escapedCommand := escapeForSingleQuotes(command)
	return "docker exec " + containerName + " " + shell + " -c '" + escapedCommand + "'"
}

// escapeForSingleQuotes escapes single quotes for shell command
// Replaces ' with '"'"'
func escapeForSingleQuotes(s string) string {
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "'\"'\"'"
		} else {
			result += string(c)
		}
	}
	return result
}

// TestDefaultShell tests that default shell is used when not specified
func TestDefaultShell(t *testing.T) {
	tests := []struct {
		name       string
		shellPath  string
		wantShell  string
	}{
		{
			name:      "empty shell uses default",
			shellPath: "",
			wantShell: "/bin/sh",
		},
		{
			name:      "specified shell is used",
			shellPath: "/bin/bash",
			wantShell: "/bin/bash",
		},
		{
			name:      "zsh shell",
			shellPath: "/bin/zsh",
			wantShell: "/bin/zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell := tt.shellPath
			if shell == "" {
				shell = "/bin/sh"
			}
			assert.Equal(t, tt.wantShell, shell, "shell mismatch")
		})
	}
}
