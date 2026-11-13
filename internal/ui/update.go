package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Update applies one message to the screen.
func (m *screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ensureVisible()
	case snapshotMsg:
		m.snap, m.hasSnap = sanitizeSnapshot(msg.snapshot), true
		m.rebuild()
		return m, waitSnapshot(m.snapshots)
	case monitorStoppedMsg:
		m.stopped = true
	case statusMsg:
		m.message = sanitize(msg.text)
	case tea.KeyMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *screen) key(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "ctrl+c" {
		return tea.Quit
	}
	switch {
	case m.overlay == overlayHelp:
		m.overlay = overlayNone
		return nil
	case m.overlay == overlaySignal:
		return m.signalKey(k)
	case m.prompt != promptNone:
		m.promptKey(msg)
		return nil
	}
	m.message = ""
	if m.navigationKey(k) || m.treeKey(k) {
		return nil
	}
	return m.commandKey(k)
}

// navigationKey moves the cursor; it reports whether k was handled.
func (m *screen) navigationKey(k string) bool {
	switch k {
	case "up", "k":
		m.moveTo(m.cursor - 1)
	case "down", "j":
		m.moveTo(m.cursor + 1)
	case "pgup":
		m.moveTo(m.cursor - m.bodyHeight())
	case "pgdown":
		m.moveTo(m.cursor + m.bodyHeight())
	case "home":
		m.moveTo(0)
	case "end":
		m.moveTo(len(m.rows) - 1)
	default:
		return false
	}
	return true
}

// treeKey changes what the tree shows; it reports whether k was handled.
func (m *screen) treeKey(k string) bool {
	switch k {
	case "enter", " ":
		m.toggle()
	case "left":
		m.collapseOrParent()
	case "right":
		m.setExpanded(true)
	case "f5", "t":
		m.toggleAllContainers()
	case "f6":
		m.tree.sort = m.tree.sort.next()
	case "P":
		m.tree.sort = sortCPU
	case "M":
		m.tree.sort = sortMemory
	case "N":
		m.tree.sort = sortName
	case "a":
		m.tree.showAll = !m.tree.showAll
		m.message = "Hiding stopped containers"
		if m.tree.showAll {
			m.message = "Showing all containers"
		}
	default:
		return false
	}
	m.rebuild()
	return true
}

func (m *screen) commandKey(k string) tea.Cmd {
	switch k {
	case "f10", "q":
		return tea.Quit
	case "f4", "\\":
		m.prompt, m.input = promptFilter, m.tree.filter
	case "f3", "/":
		m.prompt, m.input = promptSearch, ""
	case "n":
		m.searchNext()
	case "f1", "?", "h":
		m.overlay = overlayHelp
	case "f9", "K":
		m.openSignalDialog()
	case "f7", "l":
		return m.logs()
	case "f8", "s":
		return m.shell()
	}
	return nil
}

func (m *screen) toggle() {
	r, ok := m.current()
	if !ok {
		return
	}
	switch r.key.kind {
	case rowProject:
		m.tree.collapsedProjects[r.key.project] = !m.tree.collapsedProjects[r.key.project]
	case rowContainer:
		m.tree.collapsedContainers[r.key.containerID] = !m.tree.collapsedContainers[r.key.containerID]
	}
}

func (m *screen) setExpanded(expanded bool) {
	r, ok := m.current()
	if !ok {
		return
	}
	switch r.key.kind {
	case rowProject:
		m.tree.collapsedProjects[r.key.project] = !expanded
	case rowContainer:
		m.tree.collapsedContainers[r.key.containerID] = !expanded
	}
}

// collapseOrParent collapses an expanded row, or moves to the parent of a collapsed one.
func (m *screen) collapseOrParent() {
	r, ok := m.current()
	if !ok {
		return
	}
	if r.key.kind != rowProcess && r.expanded {
		m.setExpanded(false)
		return
	}
	parent := rowKey{kind: rowProject, project: r.key.project}
	if r.key.kind == rowProcess {
		parent = rowKey{kind: rowContainer, project: r.key.project, containerID: r.key.containerID}
	}
	if i := m.indexOf(parent); i >= 0 {
		m.cursor = i
	}
}

// toggleAllContainers collapses every container when any is expanded, else expands them all.
func (m *screen) toggleAllContainers() {
	anyExpanded := false
	for _, c := range m.snap.Containers {
		if !m.tree.collapsedContainers[c.ID] {
			anyExpanded = true
			break
		}
	}
	if !anyExpanded {
		clear(m.tree.collapsedContainers)
		return
	}
	for _, c := range m.snap.Containers {
		m.tree.collapsedContainers[c.ID] = true
	}
}

func (m *screen) promptKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.prompt == promptSearch {
			m.search = strings.ToLower(m.input)
			m.prompt = promptNone
			m.searchNext()
			return
		}
		m.prompt = promptNone
	case tea.KeyEsc:
		if m.prompt == promptFilter {
			m.tree.filter = ""
			m.rebuild()
		}
		m.prompt, m.input = promptNone, ""
	case tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		m.input += sanitize(string(msg.Runes))
		if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
			m.input += " "
		}
	default:
		return
	}
	if m.prompt == promptFilter {
		m.tree.filter = m.input
		m.rebuild()
	}
}

// searchNext moves the cursor to the next row matching the search, wrapping at the end.
func (m *screen) searchNext() {
	if m.search == "" {
		m.message = "No search; press F3 or / to search"
		return
	}
	n := len(m.rows)
	for step := 1; step <= n; step++ {
		i := (m.cursor + step) % n
		if rowMatches(m.rows[i], m.search) {
			m.moveTo(i)
			return
		}
	}
	m.message = "No row matches \"" + m.search + "\""
}

func (m *screen) openSignalDialog() {
	r, ok := m.current()
	if !ok {
		return
	}
	if r.key.kind == rowProject {
		m.message = "Signals go to a container or a process; select one first"
		return
	}
	m.dialog = signalDialog{containerID: r.container.ID, containerName: r.container.Name}
	if r.key.kind == rowProcess {
		m.dialog.pid = r.process.PID
	}
	m.overlay = overlaySignal
}

func (m *screen) signalKey(k string) tea.Cmd {
	signals := model.Signals()
	switch k {
	case "up", "k":
		m.dialog.selected = max(0, m.dialog.selected-1)
	case "down", "j":
		m.dialog.selected = min(len(signals)-1, m.dialog.selected+1)
	case "esc":
		m.overlay = overlayNone
		m.message = "Signal cancelled"
	case "enter":
		m.overlay = overlayNone
		sig := signals[m.dialog.selected]
		m.message = "Sending " + string(sig) + " to " + m.dialog.target() + "…"
		return signalCmd(m.actions, m.dialog, sig)
	}
	return nil
}

func (m *screen) logs() tea.Cmd {
	r, ok := m.current()
	if !ok || r.key.kind == rowProject {
		m.message = "Logs need a container; select one first"
		return nil
	}
	return execCmd("Logs", r.container.Name, m.actions.LogsCommand(r.container.ID))
}

func (m *screen) shell() tea.Cmd {
	r, ok := m.current()
	if !ok || r.key.kind == rowProject {
		m.message = "A shell needs a container; select one first"
		return nil
	}
	if r.container.State != model.StateRunning {
		m.message = r.container.Name + " is not running; no shell available"
		return nil
	}
	return execCmd("Shell", r.container.Name, m.actions.ShellCommand(r.container.ID))
}
