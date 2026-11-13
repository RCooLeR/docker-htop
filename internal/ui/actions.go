package ui

import (
	"context"
	"errors"
	"os/exec"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// ErrProcessSignalUnavailable is returned by Actions.SignalProcess when the plugin runs without
// the helper image (switched off, or not part of this build), so it has no way to reach a single
// process.
var ErrProcessSignalUnavailable = errors.New("signalling a single process needs the helper image")

// Actions are the side effects the UI can trigger. Each one runs only after the user asked for
// it on the selected row; the UI never calls them on its own.
type Actions interface {
	// SignalContainer sends the signal to the container's main process.
	SignalContainer(ctx context.Context, containerID string, signal model.Signal) error
	// SignalProcess sends the signal to one process of the container, identified by its PID on
	// the Docker host.
	SignalProcess(ctx context.Context, containerID string, pid int, signal model.Signal) error
	// LogsCommand returns the command that follows the container's logs in the terminal.
	LogsCommand(containerID string) *exec.Cmd
	// ShellCommand returns the command that opens an interactive shell in the container.
	ShellCommand(containerID string) *exec.Cmd
}
