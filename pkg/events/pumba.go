package events

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/chaos/stress"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ExecNetemCommand(index int) error {
	dur, err := time.ParseDuration(model.Scenar.Event[index].PumbaCommand.Options.Duration)
	if err != nil {
		return err
	}

	hosts := model.Scenar.Event[index].GetHosts()
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts specified for Pumba command")
	}
	containerNames := make([]string, 0, len(hosts))
	for _, host := range hosts {
		err = model.ValidateHostNames([]string{host})
		if err != nil {
			return err
		}
		containerNames = append(containerNames, model.ClabHostName(host))
	}

	globalParams := chaos.GlobalParams{
		Random:     false,
		Labels:     nil,
		Pattern:    "",
		Names:      containerNames,
		Interval:   0,
		DryRun:     false,
		SkipErrors: false,
	}
	netemParams := netem.Params{
		Iface:    model.Scenar.Event[index].PumbaCommand.Options.Interface,
		Ips:      nil,
		Sports:   nil,
		Dports:   nil,
		Duration: dur,
		Image:    "",
		Pull:     true,
		Limit:    0,
	}

	ctx := handleSignals()

	delayCmd, err := parseNetemCommands(index, globalParams, netemParams)
	if err != nil {
		return errors.Wrap(err, "error creating netem delay command")
	}

	err = chaos.RunChaosCommand(ctx, delayCmd, &globalParams)
	if err != nil {
		return errors.Wrap(err, "error running netem delay command")

	}
	return nil
}

func ExecStressCommand(index int) error {

	globalParams := chaos.GlobalParams{
		Random:     false,
		Labels:     nil,
		Pattern:    "",
		Names:      []string{model.Scenar.Event[index].Host},
		Interval:   0,
		DryRun:     false,
		SkipErrors: false,
	}

	ctx := handleSignals()

	stressCmd, err := parseStressCommands(index, globalParams)
	if err != nil {
		return errors.Wrap(err, "error creating stress command")
	}

	err = chaos.RunChaosCommand(ctx, stressCmd, &globalParams)
	if err != nil {
		return errors.Wrap(err, "error running stress command")
	}
	return nil
}

func handleSignals() context.Context {
	// Graceful shut-down on SIGINT/SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// create cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		sid := <-sig
		logrus.Debugf("Received signal: %d\n", sid)
		logrus.Debug("Canceling running chaos commands ...")
		logrus.Debug("Gracefully exiting after some cleanup ...")
	}()

	return ctx
}

func parseNetemCommands(index int, globalParams chaos.GlobalParams, netemParams netem.Params) (chaos.Command, error) {

	cmdOption := model.Scenar.Event[index].PumbaCommand.Options

	switch model.Scenar.Event[index].PumbaCommand.Name {
	case "delay":
		return netem.NewDelayCommand(chaos.DockerClient, &globalParams, &netemParams, cmdOption.Time, cmdOption.Jitter, cmdOption.Correlation, cmdOption.Distribution)
	case "corrupt":
		return netem.NewCorruptCommand(chaos.DockerClient, &globalParams, &netemParams, cmdOption.Percent, cmdOption.Correlation)
	case "duplicate":
		return netem.NewDuplicateCommand(chaos.DockerClient, &globalParams, &netemParams, cmdOption.Percent, cmdOption.Correlation)
	case "loss":
		return netem.NewLossCommand(chaos.DockerClient, &globalParams, &netemParams, cmdOption.Percent, cmdOption.Correlation)
	case "stop":
		return docker.NewPauseCommand(chaos.DockerClient, &globalParams, netemParams.Duration, cmdOption.Limit), nil
	case "pause":
		return docker.NewStopCommand(chaos.DockerClient, &globalParams, true, netemParams.Duration, 0, cmdOption.Limit), nil
	case "rate":
		return netem.NewRateCommand(chaos.DockerClient, &globalParams, &netemParams, cmdOption.Rate, cmdOption.PacketOverhead, cmdOption.CellSize, cmdOption.CellOverhead)
	default:
		return nil, nil
	}

}

func parseStressCommands(index int, globalParams chaos.GlobalParams) (chaos.Command, error) {
	cmdOption := model.Scenar.Event[index].PumbaCommand.Options

	dur, err := time.ParseDuration(cmdOption.Duration)
	if err != nil {
		return nil, err
	}

	switch model.Scenar.Event[index].PumbaCommand.Name {
	case "stress":
		return stress.NewStressCommand(chaos.DockerClient, &globalParams, cmdOption.StressImage, cmdOption.PullImage, cmdOption.Stressors, dur, cmdOption.Limit), nil
	default:
		return nil, nil
	}
}

func ExecPumbaCommand(index int) error {
	switch model.Scenar.Event[index].PumbaCommand.Name {
	case "delay", "corrupt", "duplicate", "loss", "rate", "stop", "pause":
		return ExecNetemCommand(index)
	case "stress":
		return ExecStressCommand(index)
	default:
		return errors.New("unknown Pumba command")
	}
}
