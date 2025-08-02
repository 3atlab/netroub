package events

import (
	"fmt"
	"time"

	"github.com/3atlab/netroub/pkg/model"
)

func ExecuteEvent(index int) error {
	switch model.Scenar.Event[index].Type {
	case model.EventTypeDummy:
		err := ExecDummyCommand(index)
		if err != nil {
			return err
		}
	case model.EventTypePumba:
		err := ExecPumbaCommand(index)
		if err != nil {
			return err
		}
	case model.EventTypeShell:
		err := ExecShellCommand(index)
		if err != nil {
			return err
		}
	case model.EventTypeConfig:
		err := ExecConfigCommand(index)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid event type %s", model.Scenar.Event[index].Type)
	}
	return nil
}

func ExecDummyCommand(index int) error {
	dur, err := time.ParseDuration(model.Scenar.Duration)
	if err != nil {
		return err
	}
	sec := dur.Seconds()
	if sec > 0 {
		time.Sleep(dur)
	}
	return nil
}
