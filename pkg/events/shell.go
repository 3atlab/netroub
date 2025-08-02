package events

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/sirupsen/logrus"
)

func ExecShellCommand(index int) error {
	shell := model.Scenar.Event[index].ShellPath
	if shell == "" {
		shell = "/bin/sh" // Default shell if not specified
	}

	for _, host := range model.Scenar.Event[index].GetHosts() {
		containerName := model.ClabHostName(host)
		for _, shellCommand := range model.Scenar.Event[index].ShellCommands {
			escapedCommand := strings.ReplaceAll(shellCommand, `'`, `'"'"'`) // Escape single quotes
			input := fmt.Sprintf(`docker exec %s %s -c '%s'`, containerName, shell, escapedCommand)
			cmd := exec.Command("sh", "-c", input)

			logrus.Debugf(`Event %d: Execute command "%s"`, index, cmd)
			_, err := cmd.CombinedOutput()
			if err != nil {
				logrus.Warnf("Error while running %s: %s\n", shellCommand, err)
			}
		}
	}
	return nil
}
