package ui

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const (
	kib = 1024
	mib = 1024 * kib
	gib = 1024 * mib
)

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }
func intp(v int) *int        { return &v }

func proc(pid, ppid int, user string, cpu float64, rss int64, threads int, command string) model.Process {
	return model.Process{
		PID: pid, PPID: ppid, User: user, State: "S", Threads: intp(threads),
		CPUPercent: f64(cpu), MemoryBytes: i64(rss), Command: command,
	}
}

func fixtureContainers() []model.Container {
	return []model.Container{
		{
			ID: "a1", Name: "alpha-worker", Image: "ghcr.io/example/alpha-worker:1", Project: "homelab",
			State: model.StateRunning, Status: "Up 3 days",
			CPUPercent: f64(37.9), MemoryBytes: i64(398 * mib), MemoryLimitBytes: i64(gib),
			NetRxBytesPerSec: f64(1.2 * mib), NetTxBytesPerSec: f64(340 * kib), PIDs: intp(12),
			Processes: []model.Process{
				proc(1916, 1457, "10001", 37.9, 398*mib, 11, "alpha-worker --config /etc/alpha-worker.yaml"),
				proc(1457, 1400, "10001", 0, 44*kib, 1, "docker-init -- alpha-worker"),
			},
		},
		{
			ID: "o1", Name: "omada-bridge", Image: "omada-bridge:latest", Project: "homelab",
			State: model.StateRunning, Status: "Up 3 days",
			CPUPercent: f64(2.1), MemoryBytes: i64(86 * mib), MemoryLimitBytes: i64(512 * mib),
			NetRxBytesPerSec: f64(48 * kib), NetTxBytesPerSec: f64(12 * kib), PIDs: intp(9),
			Processes: []model.Process{proc(2001, 1990, "10001", 2.1, 80*mib, 9, "omada-bridge")},
		},
		{
			ID: "p1", Name: "postgres", Image: "postgres:16", Project: "homelab",
			State: model.StateRunning, Status: "Up 3 days (healthy)",
			CPUPercent: f64(61.6), MemoryBytes: i64(470 * mib), MemoryLimitBytes: i64(2 * gib),
			NetRxBytesPerSec: f64(96 * kib), NetTxBytesPerSec: f64(1.4 * mib), PIDs: intp(7),
			Processes: []model.Process{
				proc(2231, 2200, "postgres", 0.3, 142*mib, 1, "postgres -D /var/lib/postgresql/data"),
				proc(2304, 2231, "postgres", 61.2, 310*mib, 1, "postgres: autovacuum worker app"),
				proc(2298, 2231, "postgres", 0.1, 18*mib, 1, "postgres: walwriter"),
			},
		},
		{
			ID: "n1", Name: "nginx", Image: "nginx:1.27", Project: "datastore",
			State: model.StateRunning, Status: "Up 6 hours",
			CPUPercent: f64(0.2), MemoryBytes: i64(24 * mib),
			NetRxBytesPerSec: f64(2.4 * mib), NetTxBytesPerSec: f64(8.1 * mib), PIDs: intp(5),
			Processes: []model.Process{proc(3001, 2990, "root", 0.2, 12*mib, 1, "nginx: master process")},
		},
		{
			ID: "m1", Name: "mariadb", Image: "mariadb:11", Project: "datastore",
			State: model.StatePaused, Status: "Paused",
			MemoryBytes: i64(1100 * mib), MemoryLimitBytes: i64(2 * gib), PIDs: intp(38),
			Processes: []model.Process{proc(3101, 3090, "mysql", 0, 1000*mib, 38, "mariadbd")},
		},
		{
			ID: "r1", Name: "old-redis", Image: "redis:6", State: model.StateExited,
			Status: "Exited (0) 2 days ago", PIDs: intp(0),
		},
	}
}

func samplerSnapshot() model.Snapshot {
	return model.Snapshot{
		Time:   time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		Mode:   model.ModeSampler,
		Engine: model.EngineInfo{Name: "docker-desktop", Version: "29.9.2", CPUs: 12, MemoryBytes: 15974 * mib},
		Host: model.Host{
			CoreBusy: []float64{0.382, 0.125, 0.71, 0.041, 0.227, 0.093, 0.554, 0.03, 0.189, 0.062, 0.916, 0.14},
			Memory: &model.HostMemory{
				TotalBytes: 15974 * mib, UsedBytes: 10035 * mib, CacheBytes: 3000 * mib,
				SwapTotalBytes: 4 * gib, SwapUsedBytes: 300 * mib,
			},
			Load:    []float64{1.84, 1.32, 0.97},
			Tasks:   intp(154),
			Threads: intp(412),
		},
		Containers: fixtureContainers(),
	}
}

func apiSnapshot() model.Snapshot {
	s := samplerSnapshot()
	s.Mode = model.ModeAPI
	s.Host = model.Host{}
	s.Notice = "sampler image not available: process CPU is coarse and host readings are unknown"
	return s
}

// signalCall records one call to fakeActions.
type signalCall struct {
	method      string
	containerID string
	pid         int
	signal      model.Signal
}

type fakeActions struct {
	calls []signalCall
	err   error
}

func (f *fakeActions) SignalContainer(_ context.Context, containerID string, sig model.Signal) error {
	f.calls = append(f.calls, signalCall{method: "container", containerID: containerID, signal: sig})
	return f.err
}

func (f *fakeActions) SignalProcess(_ context.Context, containerID string, pid int, sig model.Signal) error {
	f.calls = append(f.calls, signalCall{method: "process", containerID: containerID, pid: pid, signal: sig})
	return f.err
}

func (f *fakeActions) LogsCommand(string) *exec.Cmd  { return nil }
func (f *fakeActions) ShellCommand(string) *exec.Cmd { return nil }

// newTestScreen returns a colourless screen of the given size showing s.
func newTestScreen(t *testing.T, s model.Snapshot, width, height int) (*screen, *fakeActions) {
	t.Helper()
	fake := &fakeActions{}
	m := newScreen(nil, fake, Options{Interval: time.Second}, plainStyles())
	send(m, tea.WindowSizeMsg{Width: width, Height: height})
	send(m, snapshotMsg{snapshot: s})
	return m, fake
}

func send(m *screen, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

func keys(m *screen, ks ...string) {
	for _, k := range ks {
		send(m, keyMsg(k))
	}
}

// keyMsg builds the tea.KeyMsg whose String() is k.
func keyMsg(k string) tea.KeyMsg {
	special := map[string]tea.KeyType{
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"enter": tea.KeyEnter, "esc": tea.KeyEsc, "backspace": tea.KeyBackspace,
		"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "home": tea.KeyHome, "end": tea.KeyEnd,
		"ctrl+c": tea.KeyCtrlC, "f3": tea.KeyF3, "f4": tea.KeyF4, "f5": tea.KeyF5,
		"f6": tea.KeyF6, "f9": tea.KeyF9, "f10": tea.KeyF10, " ": tea.KeySpace,
	}
	if t, ok := special[k]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func typeText(m *screen, text string) {
	for _, r := range text {
		send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func selectRow(t *testing.T, m *screen, key rowKey) {
	t.Helper()
	i := m.indexOf(key)
	if i < 0 {
		t.Fatalf("row %+v not visible", key)
	}
	m.moveTo(i)
}

func currentKey(t *testing.T, m *screen) rowKey {
	t.Helper()
	r, ok := m.current()
	if !ok {
		t.Fatal("no current row")
	}
	return r.key
}

func processKey(project, containerID string, pid int) rowKey {
	return rowKey{kind: rowProcess, project: project, containerID: containerID, pid: pid}
}

func containerKey(project, containerID string) rowKey {
	return rowKey{kind: rowContainer, project: project, containerID: containerID}
}

func assertMaxWidth(t *testing.T, out string, width int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %d is %d cells wide, limit %d: %q", i, w, width, line)
		}
	}
}
