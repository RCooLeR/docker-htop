package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Sizes used until the first tea.WindowSizeMsg arrives.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// promptKind is the line editor shown in the status area.
type promptKind int

const (
	promptNone promptKind = iota
	promptFilter
	promptSearch
)

// overlayKind is the box drawn over the tree.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayHelp
	overlaySignal
)

// signalDialog is the target and selection of the open signal dialog.
type signalDialog struct {
	containerID   string
	containerName string
	pid           int // 0 targets the container itself
	selected      int // index into model.Signals()
}

// screen is the Bubble Tea model of the full-screen UI.
type screen struct {
	st        styles
	actions   Actions
	opts      Options
	snapshots <-chan model.Snapshot

	snap    model.Snapshot
	hasSnap bool
	stopped bool
	width   int
	height  int

	tree   treeState
	rows   []row
	cursor int
	offset int

	prompt  promptKind
	input   string
	search  string
	overlay overlayKind
	dialog  signalDialog
	message string
}

func newScreen(snapshots <-chan model.Snapshot, actions Actions, opts Options, st styles) *screen {
	return &screen{
		st:        st,
		actions:   actions,
		opts:      opts,
		snapshots: snapshots,
		tree: treeState{
			showAll:             opts.ShowAll,
			collapsedProjects:   map[string]bool{},
			collapsedContainers: map[string]bool{},
		},
	}
}

// Init starts waiting for the first snapshot.
func (m *screen) Init() tea.Cmd {
	return waitSnapshot(m.snapshots)
}

func (m *screen) size() (width, height int) {
	width, height = m.width, m.height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	return width, height
}

// bodyHeight is the number of tree lines between the column line and the status line.
func (m *screen) bodyHeight() int {
	width, height := m.size()
	header := len(renderHeader(m.st, m.snap, width, m.opts, m.tree.sort))
	return max(1, height-header-3)
}

// rebuild recomputes the visible rows and keeps the cursor on the same row identity; when that
// row is gone the cursor moves to the nearest surviving row above it.
func (m *screen) rebuild() {
	old, oldCursor := m.rows, m.cursor
	m.rows = buildRows(m.snap, m.tree)
	m.cursor = 0
	if oldCursor < len(old) {
		for i := oldCursor; i >= 0; i-- {
			if j := m.indexOf(old[i].key); j >= 0 {
				m.cursor = j
				break
			}
		}
	}
	m.ensureVisible()
}

func (m *screen) indexOf(key rowKey) int {
	for i, r := range m.rows {
		if r.key == key {
			return i
		}
	}
	return -1
}

func (m *screen) current() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func (m *screen) moveTo(i int) {
	m.cursor = max(0, min(i, len(m.rows)-1))
	m.ensureVisible()
}

func (m *screen) ensureVisible() {
	body := m.bodyHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+body {
		m.offset = m.cursor - body + 1
	}
	m.offset = max(0, min(m.offset, len(m.rows)-body))
}
