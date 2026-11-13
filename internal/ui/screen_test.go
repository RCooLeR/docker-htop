package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func rowKeys(m *screen) []rowKey {
	keys := make([]rowKey, len(m.rows))
	for i, r := range m.rows {
		keys[i] = r.key
	}
	return keys
}

func TestTreeOrder(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	want := []rowKey{
		{kind: rowProject, project: "datastore"},
		containerKey("datastore", "n1"),
		processKey("datastore", "n1", 3001),
		containerKey("datastore", "m1"),
		processKey("datastore", "m1", 3101),
		{kind: rowProject, project: "homelab"},
		containerKey("homelab", "p1"),
		processKey("homelab", "p1", 2304),
		processKey("homelab", "p1", 2231),
		processKey("homelab", "p1", 2298),
		containerKey("homelab", "a1"),
		processKey("homelab", "a1", 1916),
		processKey("homelab", "a1", 1457),
		containerKey("homelab", "o1"),
		processKey("homelab", "o1", 2001),
	}
	if got := rowKeys(m); !slices.Equal(got, want) {
		t.Fatalf("rows:\n got %v\nwant %v", got, want)
	}
	if user := m.rows[6].user; user != "postgres" {
		t.Errorf("postgres root user = %q", user)
	}
	if user := m.rows[10].user; user != "10001" {
		t.Errorf("alpha-worker root user = %q", user)
	}
}

func TestCursorFollowsRowIdentity(t *testing.T) {
	tests := []struct {
		name   string
		start  rowKey
		mutate func(s *model.Snapshot)
		want   rowKey
	}{
		{
			name:  "reordered",
			start: processKey("homelab", "p1", 2304),
			mutate: func(s *model.Snapshot) {
				s.Containers[2].CPUPercent = f64(0.5)
				s.Containers[2].Processes[1].CPUPercent = f64(0)
			},
			want: processKey("homelab", "p1", 2304),
		},
		{
			name:  "process gone moves to row above",
			start: processKey("homelab", "p1", 2298),
			mutate: func(s *model.Snapshot) {
				s.Containers[2].Processes = s.Containers[2].Processes[:2]
			},
			want: processKey("homelab", "p1", 2231),
		},
		{
			name:  "container gone moves to row above",
			start: processKey("homelab", "a1", 1457),
			mutate: func(s *model.Snapshot) {
				s.Containers = slices.Delete(s.Containers, 0, 1)
			},
			want: processKey("homelab", "p1", 2298),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
			selectRow(t, m, tt.start)
			next := samplerSnapshot()
			tt.mutate(&next)
			send(m, snapshotMsg{snapshot: next})
			if got := currentKey(t, m); got != tt.want {
				t.Errorf("cursor on %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReorderedRowMoves(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	selectRow(t, m, containerKey("homelab", "p1"))
	before := m.cursor
	next := samplerSnapshot()
	next.Containers[2].CPUPercent = f64(0.5)
	send(m, snapshotMsg{snapshot: next})
	if m.cursor == before {
		t.Fatalf("postgres did not move in the CPU order")
	}
	if got := currentKey(t, m); got != containerKey("homelab", "p1") {
		t.Errorf("cursor on %+v", got)
	}
}

func TestCollapseSurvivesRefresh(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	selectRow(t, m, containerKey("homelab", "a1"))
	keys(m, "enter")
	send(m, snapshotMsg{snapshot: samplerSnapshot()})
	if m.indexOf(processKey("homelab", "a1", 1916)) >= 0 {
		t.Fatal("collapsed container still shows processes after a refresh")
	}
	if got := currentKey(t, m); got != containerKey("homelab", "a1") {
		t.Errorf("cursor on %+v", got)
	}

	selectRow(t, m, rowKey{kind: rowProject, project: "datastore"})
	keys(m, "left")
	send(m, snapshotMsg{snapshot: samplerSnapshot()})
	if m.indexOf(containerKey("datastore", "n1")) >= 0 {
		t.Fatal("collapsed project still shows containers")
	}
	keys(m, "right")
	if m.indexOf(containerKey("datastore", "n1")) < 0 {
		t.Fatal("expanded project hides containers")
	}

	keys(m, "t")
	for _, r := range m.rows {
		if r.key.kind == rowProcess {
			t.Fatalf("collapse all left process row %+v", r.key)
		}
	}
	keys(m, "f5")
	if m.indexOf(processKey("homelab", "a1", 1916)) < 0 {
		t.Fatal("expand all did not expand alpha-worker")
	}
}

func TestLeftJumpsToParent(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	selectRow(t, m, processKey("homelab", "p1", 2231))
	keys(m, "left")
	if got := currentKey(t, m); got != containerKey("homelab", "p1") {
		t.Fatalf("cursor on %+v", got)
	}
	keys(m, "left", "left")
	if got := currentKey(t, m); got != (rowKey{kind: rowProject, project: "homelab"}) {
		t.Fatalf("cursor on %+v", got)
	}
}

func TestShowAllToggle(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	standalone := rowKey{kind: rowProject, project: ""}
	if m.indexOf(standalone) >= 0 {
		t.Fatal("exited container visible without show-all")
	}
	keys(m, "a")
	if m.indexOf(containerKey("", "r1")) < 0 {
		t.Fatal("show-all did not reveal the exited container")
	}
	if !strings.Contains(m.View(), "▾ standalone  1 container") {
		t.Errorf("standalone group missing from view:\n%s", m.View())
	}
	keys(m, "a")
	if m.indexOf(standalone) >= 0 {
		t.Fatal("second toggle did not hide the exited container")
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []rowKey
	}{
		{
			name:  "process command keeps its container",
			input: "WALWRITER",
			want: []rowKey{
				{kind: rowProject, project: "homelab"},
				containerKey("homelab", "p1"),
				processKey("homelab", "p1", 2298),
			},
		},
		{
			name:  "container name keeps all processes",
			input: "alpha",
			want: []rowKey{
				{kind: rowProject, project: "homelab"},
				containerKey("homelab", "a1"),
				processKey("homelab", "a1", 1916),
				processKey("homelab", "a1", 1457),
			},
		},
		{
			name:  "process user",
			input: "mysql",
			want: []rowKey{
				{kind: rowProject, project: "datastore"},
				containerKey("datastore", "m1"),
				processKey("datastore", "m1", 3101),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
			keys(m, "\\")
			typeText(m, tt.input)
			if got := rowKeys(m); !slices.Equal(got, tt.want) {
				t.Fatalf("while typing:\n got %v\nwant %v", got, tt.want)
			}
			keys(m, "enter")
			send(m, snapshotMsg{snapshot: samplerSnapshot()})
			if got := rowKeys(m); !slices.Equal(got, tt.want) {
				t.Fatalf("after Enter and refresh:\n got %v\nwant %v", got, tt.want)
			}
			keys(m, "f4", "esc")
			if m.tree.filter != "" || len(m.rows) != 15 {
				t.Errorf("Esc left filter %q with %d rows", m.tree.filter, len(m.rows))
			}
		})
	}
}

func TestSearch(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	keys(m, "/")
	typeText(m, "Postgres")
	keys(m, "enter")
	want := []rowKey{
		containerKey("homelab", "p1"),
		processKey("homelab", "p1", 2304),
		processKey("homelab", "p1", 2231),
		processKey("homelab", "p1", 2298),
		containerKey("homelab", "p1"),
	}
	for i, w := range want {
		if i > 0 {
			keys(m, "n")
		}
		if got := currentKey(t, m); got != w {
			t.Fatalf("match %d: cursor on %+v, want %+v", i, got, w)
		}
	}
	keys(m, "/")
	typeText(m, "nothing-like-this")
	keys(m, "enter")
	if !strings.Contains(m.message, "No row matches") {
		t.Errorf("message = %q", m.message)
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 14)
	keys(m, "end")
	view := m.View()
	if !strings.Contains(view, "└─ 2001") {
		t.Fatalf("last row not visible after End:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines != 14 {
		t.Errorf("view has %d lines, want 14", lines)
	}
	keys(m, "home")
	if !strings.Contains(m.View(), "▾ datastore") {
		t.Errorf("first row not visible after Home")
	}
}

func TestColumnDropping(t *testing.T) {
	tests := []struct {
		width    int
		wantNet  bool
		wantUser bool
	}{
		{100, true, true},
		{85, false, true},
		{70, false, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.width), func(t *testing.T) {
			m, _ := newTestScreen(t, samplerSnapshot(), tt.width, 40)
			keys(m, "a")
			view := m.View()
			assertMaxWidth(t, view, tt.width)
			if got := strings.Contains(view, "Net rx/tx"); got != tt.wantNet {
				t.Errorf("net column shown = %v", got)
			}
			if got := strings.Contains(view, " User "); got != tt.wantUser {
				t.Errorf("user column shown = %v", got)
			}
		})
	}
}

func TestWaitingView(t *testing.T) {
	m := newScreen(nil, &fakeActions{}, Options{}, plainStyles())
	if got := m.View(); !strings.Contains(got, "Waiting for the first reading") {
		t.Errorf("view = %q", got)
	}
	send(m, monitorStoppedMsg{})
	if got := m.View(); !strings.Contains(got, "monitor stopped") {
		t.Errorf("view = %q", got)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "f10", "ctrl+c"} {
		t.Run(k, func(t *testing.T) {
			m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
			cmd := send(m, keyMsg(k))
			if cmd == nil {
				t.Fatal("no command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Error("command is not tea.Quit")
			}
		})
	}
}
