# Development Guide

This document provides guidance for developers contributing to Netroub.

## Prerequisites

### Required Tools

- **Go** 1.21 or later
- **Docker** for container operations
- **Containerlab** for network topology deployment (integration tests only)
- **Pumba** for chaos injection (runtime only)

### Optional Tools

- **dot2net** for topology-driven configuration generation

## Project Structure

```
netroub/
├── main.go                 # CLI entry point (urfave/cli)
├── pkg/
│   ├── model/              # Data models and scenario parsing
│   │   └── scenario.go     # Event, Scenario, FileCopy structs
│   ├── events/             # Event execution logic
│   │   ├── event.go        # Event dispatcher
│   │   ├── pumba.go        # Pumba event handler
│   │   ├── shell.go        # Shell event handler
│   │   ├── config.go       # Config event handler
│   │   ├── copy.go         # Copy event handler
│   │   ├── copy_test.go    # Unit tests for copy
│   │   └── shell_test.go   # Unit tests for shell
│   └── network/            # Network emulation control
├── example/                # Example scenario files
├── topo/                   # Example topology files
├── tests/                  # Integration tests
│   ├── integration/        # Integration test code
│   ├── topology/           # Test topology files
│   ├── scenarios/          # Test scenario files
│   └── data/               # Test data files
└── tool/                   # Utility scripts
```

## Building

### Local Build

```bash
go build .
```

### Docker Build (cross-platform)

```bash
docker run --rm -i -v $PWD:/v -w /v golang:1.23.4 go build -buildvcs=false
```

## Testing

### Test Categories

| Category | Location | Requirements |
|----------|----------|--------------|
| Unit tests | `pkg/events/*_test.go` | Go only |
| Integration tests | `tests/integration/` | Docker, Containerlab, sudo |

### Running Unit Tests

Unit tests verify command generation logic without executing Docker commands.

```bash
# Run all unit tests
go test ./pkg/...

# Run with verbose output
go test -v ./pkg/...

# Run specific package tests
go test -v ./pkg/events/
```

### Running Integration Tests

Integration tests require a Docker environment with Containerlab installed.

```bash
# Run integration tests (requires sudo)
sudo -E go test -v ./tests/integration/

# Skip integration tests (use -short flag)
go test -short ./...
```

### Running All Tests

```bash
# All tests (integration tests require proper environment)
go test ./...

# Skip integration tests
go test -short ./...
```

## Adding a New Event Type

To add a new event type (e.g., `myevent`):

### 1. Define the Event Type Constant

In `pkg/model/scenario.go`:

```go
const EventTypeMyEvent = "myevent"
```

### 2. Add Event Fields (if needed)

In `pkg/model/scenario.go`, add fields to the `Event` struct:

```go
type Event struct {
    // ... existing fields ...
    MyEventOptions MyEventOptions `json:"myEventOptions" yaml:"myEventOptions"`
}
```

### 3. Implement the Event Handler

Create `pkg/events/myevent.go`:

```go
package events

import (
    "github.com/3atlab/netroub/pkg/model"
    "github.com/sirupsen/logrus"
)

func ExecMyEventCommand(index int) error {
    event := model.Scenar.Event[index]

    for _, host := range event.GetHosts() {
        containerName := model.ClabHostName(host)
        // Implementation here
        logrus.Debugf("Event %d: Execute myevent on %s", index, containerName)
    }
    return nil
}
```

### 4. Register in Event Dispatcher

In `pkg/events/event.go`, add a case to the switch:

```go
case model.EventTypeMyEvent:
    err := ExecMyEventCommand(index)
    if err != nil {
        return err
    }
```

### 5. Add Unit Tests

Create `pkg/events/myevent_test.go` to test command generation:

```go
package events

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestBuildMyEventCommand(t *testing.T) {
    // Test cases here
}
```

### 6. Update Documentation

- Update `doc/SCENARIO_FORMAT.md` with the new event type specification
- Add examples to `example/` directory

## Event Types Reference

| Type | Description | Handler |
|------|-------------|---------|
| `pumba` | Network chaos injection | `ExecPumbaCommand` |
| `shell` | Execute shell commands | `ExecShellCommand` |
| `config` | FRR configuration changes | `ExecConfigCommand` |
| `copy` | File copy between host and container | `ExecCopyCommand` |
| `dummy` | Internal timing control | `ExecDummyCommand` |

## Code Style

- **Comments**: All code comments should be in English
- **Error Handling**: Use `logrus.Warnf` for non-fatal errors, return errors for fatal cases
- **Logging**: Use `logrus.Debugf` for debug information

## Debugging

### Enable Debug Logging

Debug output is enabled by default. Check `control.log` for detailed execution logs.

### Common Issues

1. **Permission denied**: Run with `sudo` for Docker operations
2. **Container not found**: Verify container names match Containerlab naming convention (`clab-<topo>-<host>`)
3. **Command failed**: Check `control.log` for detailed error messages
