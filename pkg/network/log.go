package network

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/sirupsen/logrus"
)

const ControlLogFileName = "control.log"

func SearchFiles(initalSizes map[string]int64, root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		initalSize, exist := initalSizes[path]

		if exist && info.Size() != initalSize && !strings.Contains(path, ControlLogFileName) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error while searching log file")
		return nil, err
	}

	return files, nil
}

func trialDirectoryName(t time.Time) string {
	// Datetime of ISO Format
	return t.Format("2006-01-02T15:04:05")
}

func MoveLogFiles(logFiles []string, topoPath string) error {
	//Create log directory if it does not exist
	if _, err := os.Stat(model.Scenar.LogPath); os.IsNotExist(err) {
		err = os.Mkdir(model.Scenar.LogPath, os.ModePerm)
		if err != nil {
			return err
		}
	}

	//Retrieve the time for the name
	t := time.Now()
	//Generate a name for the directory
	// dirName := strconv.Itoa(int(t.Month())) + "_" + strconv.Itoa(t.Day()) + "_" + strconv.Itoa(t.Hour()) + ":" + strconv.Itoa(t.Minute()) + ":" + strconv.Itoa(t.Second()) + "_" + model.Scenar.ScenarioName
	dirName := trialDirectoryName(t)

	if _, err := os.Stat(model.Scenar.LogPath + "/" + model.Scenar.ScenarioName); os.IsNotExist(err) {
		err = os.Mkdir(model.Scenar.LogPath+"/"+model.Scenar.ScenarioName, os.ModePerm)
		if err != nil {
			fmt.Println("Error while creating new directory")
			return err
		}
	}

	//Create the directory for the scenario
	err := os.Mkdir(model.Scenar.LogPath+"/"+model.Scenar.ScenarioName+"/"+dirName, os.ModePerm)
	if err != nil {
		fmt.Println("Error while creating log directory")
		return err
	}

	//Fill the directory with the different logs generated
	for _, path := range logFiles {

		// Get path to place the collected log file
		relativePath, err := filepath.Rel(topoPath, path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(model.Scenar.LogPath, model.Scenar.ScenarioName, dirName, relativePath)

		err = os.MkdirAll(filepath.Dir(newPath), os.ModePerm)
		if err != nil {
			fmt.Println("Error while creating device directory")
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			fmt.Println("Error while opening log file")
			return err
		}
		defer src.Close()
		// destFile := filepath.Join(model.Scenar.LogPath+"/"+model.Scenar.ScenarioName+"/"+dirName+"/r"+strconv.Itoa(path+1), filepath.Base(logFiles[path]))
		dst, err := os.Create(newPath)
		if err != nil {
			fmt.Println("Error while creating new file")
			return err
		}

		_, err = io.Copy(dst, src)
		if err != nil {
			fmt.Println("Error while copying log into the new file")
			return err
		}

		logrus.Debugf("move log file %s to %s", path, newPath)
	}

	err = MoveControlLogs(dirName)
	if err != nil {
		return err
	}

	for _, host := range model.Scenar.Hosts {

		i := model.GetDeviceIndex(host)
		err = MoveTcpdumpLogs(dirName, host, i)
		if err != nil {
			return err
		}

	}

	return nil
}

func MoveControlLogs(dirName string) error {
	//Move the control log file in the created directory
	control, err := os.Open(ControlLogFileName)
	if err != nil {
		fmt.Println("Error while opening control log file")
		return err
	}
	defer control.Close()
	destFile := filepath.Join(model.Scenar.LogPath+"/"+model.Scenar.ScenarioName+"/"+dirName, filepath.Base(ControlLogFileName))
	dst, err := os.Create(destFile)
	if err != nil {
		fmt.Println("Error while creating new control log file")
		return err
	}
	_, err = io.Copy(dst, control)
	if err != nil {
		fmt.Println("Error while copying control log into the new file")
		return err
	}

	logrus.Debugf("move control log file %s to %s", ControlLogFileName, destFile)

	err = os.Remove(ControlLogFileName)
	if err != nil {
		return err
	}
	return nil
}

func MoveTcpdumpLogs(dirName string, device string, index int) error {
	tcpdumpDir := filepath.Join(model.Scenar.LogPath, model.Scenar.ScenarioName, dirName, device, "tcpdump")
	err := os.MkdirAll(tcpdumpDir, 0777)
	if err != nil {
		return fmt.Errorf("failed to create tcpdump log directory %s: %w", tcpdumpDir, err)
	}

	for _, inter := range model.Devices.Nodes[index].Interfaces {
		srcPath := filepath.Join(model.FindTopoPath(), device, "tcpdump", "tcpdump_"+inter.Name+".log")
		tcpdumpFile, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open tcpdump log file %s: %w", srcPath, err)
		}
		defer tcpdumpFile.Close()

		dstPath := filepath.Join(tcpdumpDir, "tcpdump_"+inter.Name+".log")
		dst, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("failed to create tcpdump log file %s: %w", dstPath, err)
		}
		_, err = io.Copy(dst, tcpdumpFile)
		if err != nil {
			return fmt.Errorf("failed to copy tcpdump log from %s to %s: %w", srcPath, dstPath, err)
		}

	}

	/*err = os.Remove(model.FindTopoPath() + device + "/tcpdump")
	if err != nil {
		return err
	}*/
	return nil

}

func GetTcpdumpLogs() error {
	for _, node := range model.Scenar.Hosts {
		containerName := model.ClabHostName(node)
		dstPath := filepath.Join(model.FindTopoPath(), node) + "/"
		cmd := exec.Command("sudo", "docker", "cp", containerName+":/tcpdump", dstPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to copy tcpdump directory from container %s to %s: %w, output: %s",
				containerName, dstPath, err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}

func FlushLogFiles(logFiles []string) error {
	for path := range logFiles {
		err := os.Truncate(logFiles[path], 0)
		if err != nil {
			fmt.Println("Error while flushing file")
			return err
		}
	}
	return nil
}

func TcpdumpLog(node string) error {
	containerName := model.ClabHostName(node)
	// containerNameArray := strings.Split(model.Scenar.Event[0].Host, "-")
	// containerName := strings.Join(containerNameArray[:len(containerNameArray)-1], "-")
	// fmt.Println("Container Name: ", containerName) // Debug print
	// if containerName == "" {
	// 	return fmt.Errorf("container name is empty, failed to setup tcpdump logs")
	// }

	// Build directory path
	topoPath := model.FindTopoPath() + "/" + node
	scriptPath := topoPath + "/tcpdump.sh"

	// Create directory if necessary
	err := os.MkdirAll(topoPath, 0755)
	if err != nil {
		fmt.Println("Error while creating directory:", err)
		return err
	}

	// Create the tcpdump.sh file
	file, err := os.Create(scriptPath)
	if err != nil {
		fmt.Println("Error while creating tcpdump.sh file:", err)
		return err
	}
	defer file.Close()

	// Change the permissions of the tcpdump.sh file
	err = os.Chmod(scriptPath, 0775)
	if err != nil {
		fmt.Println("Error while changing tcpdump.sh permission:", err)
		return err
	}

	// Create the tcpdump directory in the container (use absolute path for consistency)
	cmd := exec.Command("sudo", "docker", "exec", "-d", containerName, "mkdir", "/tcpdump")
	var output []byte
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error while creating tcpdump directory:", err)
		fmt.Println(cmd.String())
		log.Println(string(output))
		return err
	}
	logrus.Debugf("execute %s\n", cmd.String())

	// Write the script in tcpdump.sh
	_, err = file.WriteString("#!/bin/sh \n")
	if err != nil {
		fmt.Println("Error while writing in tcpdump.sh file:", err)
		return err
	}

	// Add tcpdump commands for each interface
	index := model.GetDeviceIndex(node)
	for _, inter := range model.Devices.Nodes[index].Interfaces {
		_, err = file.WriteString("tcpdump -i " + inter.Name + " -n -v > /tcpdump/tcpdump" + "_" + inter.Name + ".log & \n")
		if err != nil {
			fmt.Println("Error while writing in tcpdump.sh file:", err)
			return err
		}
	}

	// Copy the tcpdump.sh script into the container
	cmd = exec.Command("sudo", "docker", "cp", scriptPath, containerName+":/")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error while copying tcpdump script in the host container:", err)
		log.Println(string(output))
		return err
	}
	logrus.Debugf("execute %s\n", cmd.String())

	// Run the tcpdump.sh script in the container (use absolute path since working directory may vary)
	cmd = exec.Command("sudo", "docker", "exec", "-d", containerName, "/tcpdump.sh")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error while starting tcpdump:", err)
		log.Println(string(output))
		return err
	}
	logrus.Debugf("execute %s\n", cmd.String())

	return nil
}
