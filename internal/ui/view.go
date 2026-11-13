package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// waitingText is shown until the first snapshot arrives.
const waitingText = "Waiting for the first reading…"

// keyBarEntries are the function keys shown at the bottom, as key and label.
func keyBarEntries() [][2]string {
	return [][2]string{
		{"F1", "Help"}, {"F3", "Search"}, {"F4", "Filter"}, {"F5", "Tree"}, {"F6", "Sort"},
		{"F7", "Logs"}, {"F8", "Shell"}, {"F9", "Signal"}, {"F10", "Quit"},
	}
}

// View renders the whole screen; no line is wider than the terminal.
func (m *screen) View() string {
	width, height := m.size()
	if !m.hasSnap {
		lines := []string{truncate(waitingText, width)}
		if m.stopped {
			lines = append(lines, truncate("monitor stopped", width))
		}
		return strings.Join(lines, "\n")
	}

	lines := renderHeader(m.st, m.snap, width, m.opts, m.tree.sort)
	lines = append(lines, columnLine(m.st, width))
	body := max(1, height-len(lines)-2)
	lines = append(lines, m.body(width, body)...)
	lines = append(lines, m.statusLine(width), keyBar(m.st, width))
	return strings.Join(lines, "\n")
}

// body returns exactly n lines: the visible slice of the tree, or the open overlay.
func (m *screen) body(width, n int) []string {
	var lines []string
	switch m.overlay {
	case overlayHelp:
		lines = centerBox(m.st, helpContent(), width)
	case overlaySignal:
		lines = centerBox(m.st, m.signalContent(), width)
	default:
		offset := max(0, min(m.offset, len(m.rows)-n))
		for i := offset; i < min(len(m.rows), offset+n); i++ {
			lines = append(lines, renderRow(m.st, m.rows[i], width, i == m.cursor))
		}
		if len(m.rows) == 0 {
			lines = append(lines, m.st.muted.Render(truncate(m.emptyText(), width)))
		}
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines
}

func (m *screen) emptyText() string {
	if m.tree.filter != "" {
		return "No container matches the filter"
	}
	return "No containers"
}

func (m *screen) statusLine(width int) string {
	var parts []string
	switch m.prompt {
	case promptFilter:
		parts = append(parts, "Filter: "+m.input+"▏  Enter keep, Esc clear")
	case promptSearch:
		parts = append(parts, "Search: "+m.input+"▏  Enter find, Esc cancel")
	default:
		if m.message != "" {
			parts = append(parts, m.message)
		} else if m.stopped {
			parts = append(parts, "monitor stopped")
		}
		if m.tree.filter != "" {
			parts = append(parts, m.st.warning.Render("Filter: "+m.tree.filter))
		}
	}
	return truncate(strings.Join(parts, "   "), width)
}

func keyBar(st styles, width int) string {
	var b strings.Builder
	for i, e := range keyBarEntries() {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(st.keyName.Render(e[0]))
		b.WriteString(st.keyLabel.Render(e[1]))
	}
	return truncate(" "+b.String(), width)
}

func helpContent() []string {
	return []string{
		"Keys",
		"",
		"Up/Down k/j PgUp/PgDn Home/End  move the cursor",
		"Enter Space                     expand or collapse the row",
		"Left / Right                    collapse or go to parent / expand",
		"F5 t                            collapse or expand all containers",
		"F6                              cycle sort: CPU%, MEM, Name, Tasks",
		"P M N                           sort by CPU%, memory, name",
		"a                               show or hide stopped containers",
		"F4 \\                            filter by name, image, command, user",
		"F3 /  n                         search, next match",
		"F7 l                            follow container logs",
		"F8 s                            open a shell in the container",
		"F9 K                            send a signal to the row",
		"F1 ? h                          this help",
		"F10 q Ctrl+C                    quit",
		"",
		"Press any key to close",
	}
}

func (m *screen) signalContent() []string {
	lines := []string{m.st.title.Render("Send signal to " + m.dialog.target()), ""}
	for i, sig := range model.Signals() {
		if i == m.dialog.selected {
			lines = append(lines, m.st.cursor.Render("> "+string(sig)))
			continue
		}
		lines = append(lines, "  "+string(sig))
	}
	return append(lines, "", "Enter send   Esc cancel")
}

// centerBox draws content in a bordered box centred in width.
func centerBox(st styles, content []string, width int) []string {
	box := st.dialog.Render(strings.Join(content, "\n"))
	pad := strings.Repeat(" ", max(0, (width-lipgloss.Width(box))/2))
	lines := strings.Split(box, "\n")
	for i, l := range lines {
		lines[i] = truncate(pad+l, width)
	}
	return append([]string{""}, lines...)
}
