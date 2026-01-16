package events

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/sirupsen/logrus"
)

// ExecCopyCommand executes file copy operations between host and container
func ExecCopyCommand(index int) error {
	event := model.Scenar.Event[index]

	for _, host := range event.GetHosts() {
		containerName := model.ClabHostName(host)

		// Process toContainer (host -> container)
		for _, fc := range event.ToContainer {
			if err := copyToContainer(index, containerName, fc); err != nil {
				logrus.Warnf("Error copying to container %s: %s", containerName, err)
			}
		}

		// Process fromContainer (container -> host)
		for _, fc := range event.FromContainer {
			if err := copyFromContainer(index, containerName, fc); err != nil {
				logrus.Warnf("Error copying from container %s: %s", containerName, err)
			}
		}
	}
	return nil
}

// copyToContainer copies a file from host to container using docker cp
func copyToContainer(index int, containerName string, fc model.FileCopy) error {
	// docker cp <src> <container>:<dst>
	dst := fmt.Sprintf("%s:%s", containerName, fc.Dst)
	cmd := exec.Command("docker", "cp", fc.Src, dst)
	logrus.Debugf("Event %d: Execute docker cp %s %s", index, fc.Src, dst)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp from %s to %s failed: %w, output: %s", fc.Src, dst, err, strings.TrimSpace(string(output)))
	}

	// Determine the destination path for chown/chmod
	dstPath := fc.Dst
	if strings.HasSuffix(fc.Dst, "/") {
		// If dst is a directory, append the source filename
		dstPath = filepath.Join(fc.Dst, filepath.Base(fc.Src))
	}

	// Apply owner if specified
	if fc.Owner != "" {
		chownCmd := exec.Command("docker", "exec", containerName, "chown", fc.Owner, dstPath)
		logrus.Debugf("Event %d: Execute docker exec %s chown %s %s", index, containerName, fc.Owner, dstPath)

		output, err := chownCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("chown failed: %s, output: %s", err, string(output))
		}
	}

	// Apply mode if specified
	if fc.Mode != "" {
		chmodCmd := exec.Command("docker", "exec", containerName, "chmod", fc.Mode, dstPath)
		logrus.Debugf("Event %d: Execute docker exec %s chmod %s %s", index, containerName, fc.Mode, dstPath)

		output, err := chmodCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("chmod failed: %s, output: %s", err, string(output))
		}
	}

	return nil
}

// copyFromContainer copies a file from container to host using docker cp
func copyFromContainer(index int, containerName string, fc model.FileCopy) error {
	// Ensure destination directory exists
	dstDir := fc.Dst
	if !strings.HasSuffix(fc.Dst, "/") {
		dstDir = filepath.Dir(fc.Dst)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// docker cp <container>:<src> <dst>
	src := fmt.Sprintf("%s:%s", containerName, fc.Src)
	cmd := exec.Command("docker", "cp", src, fc.Dst)
	logrus.Debugf("Event %d: Execute docker cp %s %s", index, src, fc.Dst)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp from %s to %s failed: %w, output: %s", src, fc.Dst, err, strings.TrimSpace(string(output)))
	}

	// Determine the destination path for chown/chmod
	dstPath := fc.Dst
	if strings.HasSuffix(fc.Dst, "/") {
		// If dst is a directory, append the source filename
		dstPath = filepath.Join(fc.Dst, filepath.Base(fc.Src))
	}

	// Apply owner if specified (on host side)
	if fc.Owner != "" {
		chownCmd := exec.Command("chown", fc.Owner, dstPath)
		logrus.Debugf("Event %d: Execute chown %s %s", index, fc.Owner, dstPath)

		output, err := chownCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("chown failed: %s, output: %s", err, string(output))
		}
	}

	// Apply mode if specified (on host side)
	if fc.Mode != "" {
		chmodCmd := exec.Command("chmod", fc.Mode, dstPath)
		logrus.Debugf("Event %d: Execute chmod %s %s", index, fc.Mode, dstPath)

		output, err := chmodCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("chmod failed: %s, output: %s", err, string(output))
		}
	}

	return nil
}
