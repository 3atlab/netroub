package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"

	"github.com/3atlab/netroub/pkg/events"
	"github.com/3atlab/netroub/pkg/model"
	"github.com/3atlab/netroub/pkg/network"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var initalSizes map[string]int64

type ConsoleHook struct{}

func (h *ConsoleHook) Fire(entry *logrus.Entry) error {
	if entry.Level <= logrus.InfoLevel {
		t := entry.Time
		fmt.Print(color.BlueString("[INFO]"))
		fmt.Print(t.Format("2006-01-02 15:04:05 "))
		fmt.Print(color.GreenString(entry.Message), " ")
		if entry.Data["command"] != nil {
			fmt.Println(entry.Data["command"], "duration :", entry.Data["duration"])
		} else {
			fmt.Println("")
		}
	}
	return nil
}
func (h *ConsoleHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
func NewConsoleHook() *ConsoleHook {
	return &ConsoleHook{}
}

func main() {
	app := cli.NewApp()
	app.Name = "Netroub"
	app.Usage = "Netroub is a synthetic data generator from network trouble scenarios"
	app.Version = "0.0.2"
	app.Authors = []cli.Author{
		{
			Name:  "Colin Regal-Mezin",
			Email: "colin.regalmezin@gmail.com",
		},
		{
			Name:  "Satoru Kobayashi",
			Email: "sat@okayama-u.ac.jp",
		},
	}
	app.EnableBashCompletion = true
	if len(os.Args) > 1 {
		model.SudoCheck()
		app.Action = runScenario
		app.After = after

	}
	app.Before = before
	app.CustomAppHelpTemplate = model.ConfigTemplate()

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "yaml",
			Usage: "Use a .yaml scenario file",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

func runScenario(c *cli.Context) error {
	var err error

	controlLogFile, err := os.Create("control.log")
	if err != nil {
		fmt.Println("Error while creating control log file")
		return err
	}
	// logrus.SetOutput(controlLogFile)
	logrus.SetOutput(io.MultiWriter(os.Stdout, controlLogFile))

	//Read the scenario file and sort it by time in an array
	if c.Bool("yaml") {
		err = model.ReadYaml()
		if err != nil {
			return err
		}
	} else {
		err = model.ReadJsonScenar()
		if err != nil {
			return err
		}
	}

	//Read the dot2net data json file containing device information
	err = model.ReadJsonData()
	if err != nil {
		return err
	}
	err = model.ValidateHostNames(model.Scenar.Hosts)
	if err != nil {
		return err
	}

	//Set dummy event to control the whole duration of the scenario
	model.Scenar.Event = append(model.Scenar.Event, model.Event{BeginTime: "0s", Type: model.EventTypeDummy})

	//Stock the size of all the log file present in the directory of the topo file
	path := model.FindTopoPath()
	initalSizes = make(map[string]int64)
	initalSizes, err = model.StockInitialSize(initalSizes, path)
	if err != nil {
		return err
	}

	//Create the DockerClient which is mandatory for pumba command
	err = network.CreateDockerClient(c)
	if err != nil {
		return err
	}
	//Emulate the network with Containerlab
	err = network.EmulateNetwork()
	if err != nil {
		return err
	}

	// nbFile, err := countSubDir()
	// if err != nil {
	// 	return err
	// }

	//Setup tcpdump logging
	for _, node := range model.Scenar.Hosts {
		err = network.TcpdumpLog(node)
		if err != nil {
			return err
		}
	}

	//Create a channel to verify routine states
	done := make(chan bool)

	// Load and parse beginTime for each event
	beginTimes := make([]time.Duration, 0, len(model.Scenar.Event))
	for _, event := range model.Scenar.Event {
		var dur time.Duration
		if event.BeginTime == "" {
			dur = time.Duration(0)
		} else {
			dur, err = time.ParseDuration(event.BeginTime)
			if err != nil {
				return err
			}
		}
		beginTimes = append(beginTimes, dur)
	}

	logrus.Debugf("Starting scenario %s\n", model.Scenar.ScenarioName)

	//Run for all the events in the scenario file
	for i := 0; i < len(model.Scenar.Event); i++ {
		logrus.Debugf("Adding new event %d %+v\n", i, model.Scenar.Event[i]) // DEBUG
		go func(index int) {
			dur := beginTimes[index]
			if dur.Seconds() > 0 {
				time.Sleep(dur)
			}
			logrus.Debugf("Starting event %d\n", index)

			err := events.ExecuteEvent(index)
			if err != nil {
				logrus.Errorf("Error executing event %d: %v\n", index, err)
			}

			logrus.Debugf("Completed event %d\n", index)

			done <- true
		}(i)
	}

	//Wait here until all routines are finished
	for i := 0; i < len(model.Scenar.Event); i++ {
		<-done
	}

	logrus.Debugf("Completed scenario %s\n", model.Scenar.ScenarioName)

	return nil
}

func before(c *cli.Context) error {

	/*Useless*/
	c.Args() //Permit to remove an unsed paramater warning
	/*Useless*/
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05.000", FullTimestamp: true})
	logrus.SetOutput(os.Stdout)

	logrus.AddHook(NewConsoleHook())
	return nil
}

func after(c *cli.Context) error {

	/*Useless*/
	c.Args() //Permit to remove a unsed paramater warning
	/*Useless*/

	//Find the directory to search log file
	path := model.FindTopoPath()
	//Fill an array with all log file path
	logFiles, err := network.SearchFiles(initalSizes, path)
	if err != nil {
		return err
	}
	logrus.Debugf("Log files: %v\n", logFiles)
	//Move tcpdump log files
	err = network.GetTcpdumpLogs()
	if err != nil {
		return err
	}
	//Destroy the emulated network
	err = network.DestroyNetwork()
	if err != nil {
		return err
	}

	err = network.MoveLogFiles(logFiles, path)
	if err != nil {
		return err
	}
	//Flush log files for the next scenario
	err = network.FlushLogFiles(logFiles)
	if err != nil {
		return err
	}
	return nil
}

// func countSubDir() (int, error) {
// 	count := 0
//
// 	file, err := os.Open(model.FindTopoPath())
// 	if err != nil {
// 		return count, err
// 	}
// 	defer file.Close()
//
// 	dir, err := file.ReadDir(-1)
// 	if err != nil {
// 		fmt.Println("Error while reading topo dir")
// 		return count, err
// 	}
// 	for _, subDir := range dir {
// 		if subDir.IsDir() {
// 			count++
// 		}
// 	}
// 	return count, nil
// }
