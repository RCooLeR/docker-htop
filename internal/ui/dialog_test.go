package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestSignalDialog(t *testing.T) {
	tests := []struct {
		name        string
		row         rowKey
		keys        []string
		err         error
		wantTitle   string
		wantCalls   []signalCall
		wantMessage string
	}{
		{
			name:        "process with second signal",
			row:         processKey("homelab", "p1", 2304),
			keys:        []string{"down", "enter"},
			wantTitle:   "Send signal to process 2304 in postgres",
			wantCalls:   []signalCall{{method: "process", containerID: "p1", pid: 2304, signal: model.SignalKill}},
			wantMessage: "Sent SIGKILL to process 2304 in postgres",
		},
		{
			name:        "container with default signal",
			row:         containerKey("homelab", "p1"),
			keys:        []string{"enter"},
			wantTitle:   "Send signal to postgres",
			wantCalls:   []signalCall{{method: "container", containerID: "p1", signal: model.SignalTerm}},
			wantMessage: "Sent SIGTERM to postgres",
		},
		{
			name:        "escape sends nothing",
			row:         containerKey("homelab", "p1"),
			keys:        []string{"down", "esc"},
			wantTitle:   "Send signal to postgres",
			wantMessage: "Signal cancelled",
		},
		{
			name:        "process signal unavailable",
			row:         processKey("homelab", "a1", 1916),
			keys:        []string{"enter"},
			err:         ErrProcessSignalUnavailable,
			wantTitle:   "Send signal to process 1916 in alpha-worker",
			wantCalls:   []signalCall{{method: "process", containerID: "a1", pid: 1916, signal: model.SignalTerm}},
			wantMessage: "has no signal helper",
		},
		{
			name:        "engine error",
			row:         containerKey("homelab", "o1"),
			keys:        []string{"enter"},
			err:         errors.New("container o1 is not running"),
			wantTitle:   "Send signal to omada-bridge",
			wantCalls:   []signalCall{{method: "container", containerID: "o1", signal: model.SignalTerm}},
			wantMessage: "Sending SIGTERM to omada-bridge failed: container o1 is not running",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fake := newTestScreen(t, samplerSnapshot(), 120, 40)
			fake.err = tt.err
			selectRow(t, m, tt.row)
			keys(m, "f9")
			if m.overlay != overlaySignal {
				t.Fatal("signal dialog not open")
			}
			view := m.View()
			assertMaxWidth(t, view, 120)
			if !strings.Contains(view, tt.wantTitle) || !strings.Contains(view, "> SIGTERM") {
				t.Fatalf("dialog lacks title or default selection:\n%s", view)
			}
			for _, k := range tt.keys {
				cmd := send(m, keyMsg(k))
				if len(fake.calls) != 0 {
					t.Fatalf("action called before its command ran, after %q", k)
				}
				if cmd != nil {
					send(m, cmd())
				}
			}
			if !slices.Equal(fake.calls, tt.wantCalls) {
				t.Errorf("calls = %+v, want %+v", fake.calls, tt.wantCalls)
			}
			if !strings.Contains(m.message, tt.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", m.message, tt.wantMessage)
			}
			if m.overlay != overlayNone {
				t.Error("dialog still open")
			}
		})
	}
}

func TestSignalDialogNeedsTarget(t *testing.T) {
	m, fake := newTestScreen(t, samplerSnapshot(), 120, 40)
	selectRow(t, m, rowKey{kind: rowProject, project: "homelab"})
	keys(m, "K", "enter")
	if m.overlay != overlayNone || len(fake.calls) != 0 {
		t.Fatalf("project row opened a dialog or sent a signal: overlay %v, calls %v", m.overlay, fake.calls)
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	keys(m, "?")
	if view := m.View(); !strings.Contains(view, "send a signal to the row") {
		t.Fatalf("help not shown:\n%s", view)
	}
	cursor := m.cursor
	keys(m, "j")
	if m.overlay != overlayNone || m.cursor != cursor {
		t.Errorf("closing key also acted: overlay %v, cursor %d -> %d", m.overlay, cursor, m.cursor)
	}
}

func TestShellNeedsRunningContainer(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	selectRow(t, m, containerKey("datastore", "m1"))
	if cmd := send(m, keyMsg("s")); cmd != nil {
		t.Fatal("shell started for a paused container")
	}
	if !strings.Contains(m.message, "not running") {
		t.Errorf("message = %q", m.message)
	}
}
