package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// signalTimeout bounds one signal action.
const signalTimeout = 15 * time.Second

type snapshotMsg struct{ snapshot model.Snapshot }

type monitorStoppedMsg struct{}

// statusMsg replaces the status-area message.
type statusMsg struct{ text string }

// waitSnapshot waits for one snapshot; the caller re-arms it after each message.
func waitSnapshot(ch <-chan model.Snapshot) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		s, ok := <-ch
		if !ok {
			return monitorStoppedMsg{}
		}
		return snapshotMsg{snapshot: s}
	}
}

// target names what a signal goes to, for messages.
func (d signalDialog) target() string {
	if d.pid == 0 {
		return d.containerName
	}
	return fmt.Sprintf("process %d in %s", d.pid, d.containerName)
}

func signalCmd(actions Actions, d signalDialog, sig model.Signal) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), signalTimeout)
		defer cancel()
		var err error
		if d.pid == 0 {
			err = actions.SignalContainer(ctx, d.containerID, sig)
		} else {
			err = actions.SignalProcess(ctx, d.containerID, d.pid, sig)
		}
		switch {
		case errors.Is(err, ErrProcessSignalUnavailable):
			return statusMsg{text: fmt.Sprintf(
				"Cannot signal process %d: this run has no signal helper; signal the container instead", d.pid)}
		case err != nil:
			return statusMsg{text: fmt.Sprintf("Sending %s to %s failed: %v", sig, d.target(), err)}
		default:
			return statusMsg{text: fmt.Sprintf("Sent %s to %s", sig, d.target())}
		}
	}
}

// execCmd hands the terminal to an external command and reports a failure when it returns.
func execCmd(what, containerName string, c *exec.Cmd) tea.Cmd {
	if c == nil {
		return func() tea.Msg {
			return statusMsg{text: fmt.Sprintf("%s is not available for %s", what, containerName)}
		}
	}
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err == nil || interrupted(err) {
			return statusMsg{}
		}
		return statusMsg{text: fmt.Sprintf("%s for %s: %v", what, containerName, err)}
	})
}

// Exit codes of a command stopped with Ctrl+C: 128+SIGINT on Unix shells and
// STATUS_CONTROL_C_EXIT on Windows.
const (
	exitCodeSigint      = 130
	exitCodeControlCWin = 0xC000013A
)

// interrupted reports whether the command ended because the user pressed Ctrl+C, which is the
// normal way to leave docker logs -f and not worth an error message.
func interrupted(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	code := exitErr.ExitCode()
	// ExitCode is -1 when a signal ended the process.
	return code == -1 || code == exitCodeSigint || code == exitCodeControlCWin
}
