package events

import (
	"testing"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/stretchr/testify/assert"
)

// TestBuildCopyToContainerCommand tests command generation for toContainer operations
func TestBuildCopyToContainerCommand(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		fc            model.FileCopy
		wantCpArgs    []string
		wantChownArgs []string
		wantChmodArgs []string
	}{
		{
			name:          "simple copy without permissions",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src: "./config.conf",
				Dst: "/etc/frr/",
			},
			wantCpArgs:    []string{"cp", "./config.conf", "clab-topo-r1:/etc/frr/"},
			wantChownArgs: nil,
			wantChmodArgs: nil,
		},
		{
			name:          "copy with owner",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src:   "./config.conf",
				Dst:   "/etc/frr/",
				Owner: "frr:frr",
			},
			wantCpArgs:    []string{"cp", "./config.conf", "clab-topo-r1:/etc/frr/"},
			wantChownArgs: []string{"exec", "clab-topo-r1", "chown", "frr:frr", "/etc/frr/config.conf"},
			wantChmodArgs: nil,
		},
		{
			name:          "copy with mode",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src:  "./config.conf",
				Dst:  "/etc/frr/",
				Mode: "644",
			},
			wantCpArgs:    []string{"cp", "./config.conf", "clab-topo-r1:/etc/frr/"},
			wantChownArgs: nil,
			wantChmodArgs: []string{"exec", "clab-topo-r1", "chmod", "644", "/etc/frr/config.conf"},
		},
		{
			name:          "copy with owner and mode",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src:   "./bgp.conf",
				Dst:   "/etc/frr/bgp.conf",
				Owner: "frr:frr",
				Mode:  "600",
			},
			wantCpArgs:    []string{"cp", "./bgp.conf", "clab-topo-r1:/etc/frr/bgp.conf"},
			wantChownArgs: []string{"exec", "clab-topo-r1", "chown", "frr:frr", "/etc/frr/bgp.conf"},
			wantChmodArgs: []string{"exec", "clab-topo-r1", "chmod", "600", "/etc/frr/bgp.conf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpArgs, chownArgs, chmodArgs := buildToContainerCommands(tt.containerName, tt.fc)

			assert.Equal(t, tt.wantCpArgs, cpArgs, "docker cp args mismatch")
			assert.Equal(t, tt.wantChownArgs, chownArgs, "chown args mismatch")
			assert.Equal(t, tt.wantChmodArgs, chmodArgs, "chmod args mismatch")
		})
	}
}

// TestBuildCopyFromContainerCommand tests command generation for fromContainer operations
func TestBuildCopyFromContainerCommand(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		fc            model.FileCopy
		wantCpArgs    []string
		wantChownArgs []string
		wantChmodArgs []string
	}{
		{
			name:          "simple copy without permissions",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src: "/tmp/output.txt",
				Dst: "./logs/",
			},
			wantCpArgs:    []string{"cp", "clab-topo-r1:/tmp/output.txt", "./logs/"},
			wantChownArgs: nil,
			wantChmodArgs: nil,
		},
		{
			name:          "copy with owner",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src:   "/tmp/output.txt",
				Dst:   "./logs/",
				Owner: "testuser:testgroup",
			},
			wantCpArgs:    []string{"cp", "clab-topo-r1:/tmp/output.txt", "./logs/"},
			wantChownArgs: []string{"testuser:testgroup", "./logs/output.txt"},
			wantChmodArgs: nil,
		},
		{
			name:          "copy with owner and mode",
			containerName: "clab-topo-r1",
			fc: model.FileCopy{
				Src:   "/var/log/frr/frr.log",
				Dst:   "./logs/r1_frr.log",
				Owner: "testuser:testgroup",
				Mode:  "644",
			},
			wantCpArgs:    []string{"cp", "clab-topo-r1:/var/log/frr/frr.log", "./logs/r1_frr.log"},
			wantChownArgs: []string{"testuser:testgroup", "./logs/r1_frr.log"},
			wantChmodArgs: []string{"644", "./logs/r1_frr.log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpArgs, chownArgs, chmodArgs := buildFromContainerCommands(tt.containerName, tt.fc)

			assert.Equal(t, tt.wantCpArgs, cpArgs, "docker cp args mismatch")
			assert.Equal(t, tt.wantChownArgs, chownArgs, "chown args mismatch")
			assert.Equal(t, tt.wantChmodArgs, chmodArgs, "chmod args mismatch")
		})
	}
}

// buildToContainerCommands builds command arguments for toContainer copy (for testing)
func buildToContainerCommands(containerName string, fc model.FileCopy) (cpArgs, chownArgs, chmodArgs []string) {
	// docker cp args
	dst := containerName + ":" + fc.Dst
	cpArgs = []string{"cp", fc.Src, dst}

	// Determine destination path for chown/chmod
	dstPath := fc.Dst
	if len(fc.Dst) > 0 && fc.Dst[len(fc.Dst)-1] == '/' {
		// If dst is a directory, append source filename
		dstPath = fc.Dst + getBaseName(fc.Src)
	}

	// chown args (docker exec container chown owner path)
	if fc.Owner != "" {
		chownArgs = []string{"exec", containerName, "chown", fc.Owner, dstPath}
	}

	// chmod args (docker exec container chmod mode path)
	if fc.Mode != "" {
		chmodArgs = []string{"exec", containerName, "chmod", fc.Mode, dstPath}
	}

	return cpArgs, chownArgs, chmodArgs
}

// buildFromContainerCommands builds command arguments for fromContainer copy (for testing)
func buildFromContainerCommands(containerName string, fc model.FileCopy) (cpArgs, chownArgs, chmodArgs []string) {
	// docker cp args
	src := containerName + ":" + fc.Src
	cpArgs = []string{"cp", src, fc.Dst}

	// Determine destination path for chown/chmod
	dstPath := fc.Dst
	if len(fc.Dst) > 0 && fc.Dst[len(fc.Dst)-1] == '/' {
		// If dst is a directory, append source filename
		dstPath = fc.Dst + getBaseName(fc.Src)
	}

	// chown args (on host side: chown owner path)
	if fc.Owner != "" {
		chownArgs = []string{fc.Owner, dstPath}
	}

	// chmod args (on host side: chmod mode path)
	if fc.Mode != "" {
		chmodArgs = []string{fc.Mode, dstPath}
	}

	return cpArgs, chownArgs, chmodArgs
}

// getBaseName returns the base name of a path (simplified version for testing)
func getBaseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
