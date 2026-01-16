package events

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/sirupsen/logrus"
)

func ExecConfigCommand(index int) error {
	if model.Scenar.Event[index].VtyshChanges != nil {
		err := ExecVtyshChanges(index)
		if err != nil {
			return err
		}
	}
	if model.Scenar.Event[index].ConfigFileChanges != nil {
		err := ExecConfigFileChanges(index)
		if err != nil {
			return err
		}
	}
	return nil
}

func ExecConfigFileChanges(index int) error {

	// //Get the device name
	// device := strings.SplitAfter(model.Scenar.Event[index].Host, "-")[len(strings.SplitAfter(model.Scenar.Event[index].Host, "-"))-1]

	//Copy frr-reload.py in the host container
	/*cmd := exec.Command("sudo", "docker", "cp", "frr-reload.py", model.Scenar.Event[index].Host+":/")
	_, err := cmd.Output()
	if err != nil {
		fmt.Println("Error while copying config file in the container")
		return err
	}*/
	host := model.Scenar.Event[index].Host

	for _, modif := range model.Scenar.Event[index].ConfigFileChanges {
		//Get the path of the topology file
		file, err := os.Open(model.FindTopoPath() + host + "/" + modif.File)
		if err != nil {
			fmt.Println("Error while opening config file")
			return err
		}
		defer file.Close()

		//Read the configuration file
		byteArray, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("Error while reading config file")
			return err
		}

		//Store the config file in an array
		configFile := strings.Split(string(byteArray), "\n")

		//Modify the selected line
		configFile[modif.Line-1] = modif.Command

		//Recompose the array into a string
		writeString := strings.Join(configFile, "\n")

		//Write the new config in the configuration file
		err = os.WriteFile(model.FindTopoPath()+host+"/"+modif.File, []byte(writeString), 0666)
		if err != nil {
			fmt.Println("Error while writig changes in config file")
			return err
		}

		/*cmd = exec.Command("sudo", "docker", "exec", model.Scenar.Event[index].Host, "python", "frr-reload.py", "--reload", "/etc/frr/ospfd.conf")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("Error while restarting frr services:")
			log.Println(string(output))
			return err
		}
		logrus.Info("FRR restarted successfully with modified configuration file")*/
	}

	return nil
}

func ExecVtyshChanges(index int) error {
	host := model.Scenar.Event[index].Host
	containerName := model.ClabHostName(host)

	// Build vtysh command with multiple -c options
	// Example: vtysh -c 'conf t' -c 'interface net0' -c 'ip ospf cost 100'
	args := []string{"docker", "exec", containerName, "vtysh"}
	for _, vtyCommand := range model.Scenar.Event[index].VtyshChanges {
		args = append(args, "-c", vtyCommand)
		logrus.WithFields(logrus.Fields{
			"command":   vtyCommand,
			"container": host,
		}).Debug("Adding vtysh command:")
	}

	cmd := exec.Command("sudo", args...)
	logrus.Debugf("Event %d: Execute %s", index, cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run vtysh command on %s: %w, command: %s, output: %s",
			containerName, err, cmd.String(), strings.TrimSpace(string(output)))
	}

	logrus.Info("configuration changes applied")
	return nil
}
