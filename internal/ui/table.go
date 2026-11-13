package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Below these widths the network and then the user column are dropped.
const (
	minWidthNet  = 90
	minWidthUser = 72
)

// The name column is nameWidth cells up to nameGrowFrom columns and then takes a third of every
// extra column up to nameMaxWidth, so Compose names like project-service-1 fit on wide terminals.
const (
	nameWidth    = 21
	nameMaxWidth = 37
	nameGrowFrom = 100
)

// dangerCPU is the CPU% from which a value is drawn in the danger colour.
const dangerCPU = 50.0

// columns is the table layout for one terminal width; a zero width means the column is hidden.
type columns struct {
	name, user, cpu, mem, net, tasks, command int
}

func layoutColumns(width int) columns {
	name := min(nameMaxWidth, nameWidth+max(0, width-nameGrowFrom)/3)
	c := columns{name: name, user: 9, cpu: 6, mem: 11, net: 11, tasks: 6}
	if width < minWidthNet {
		c.net = 0
	}
	if width < minWidthUser {
		c.user = 0
	}
	fixed := 0
	for _, w := range []int{c.name, c.user, c.cpu, c.mem, c.net, c.tasks} {
		if w > 0 {
			fixed += w + 1
		}
	}
	const minCommand = 8
	if rest := width - fixed; rest < minCommand {
		c.name = max(12, c.name-(minCommand-rest))
		fixed += c.name - name
	}
	c.command = max(0, width-fixed)
	return c
}

// cell is one column of a row, already padded to its width. A nil style means the row's
// base style applies.
type cell struct {
	text  string
	style *lipgloss.Style
}

func (c columns) cells(name, user, cpu, mem, net, tasks, command string) []cell {
	out := []cell{{text: padRight(name, c.name)}}
	add := func(w int, s string, right bool) {
		if w == 0 {
			return
		}
		out = append(out, cell{text: " "})
		if right {
			out = append(out, cell{text: padLeft(s, w)})
		} else {
			out = append(out, cell{text: padRight(s, w)})
		}
	}
	add(c.user, user, false)
	add(c.cpu, cpu, true)
	add(c.mem, mem, true)
	add(c.net, net, true)
	add(c.tasks, tasks, true)
	if c.command > 0 {
		out = append(out, cell{text: " "}, cell{text: truncate(command, c.command)})
	}
	return out
}

func columnLine(st styles, width int) string {
	c := layoutColumns(width)
	cells := c.cells("Name / PID", "User", "CPU%", "Mem", "Net rx/tx", "Tasks", "Command / status")
	return st.columnLine.Render(padRight(joinPlain(cells), width))
}

// renderRow draws one tree row; the cursor row is drawn in reverse video without other colours.
func renderRow(st styles, r row, width int, cursor bool) string {
	cells, base := rowCells(st, r, layoutColumns(width))
	if cursor {
		return st.cursor.Render(padRight(joinPlain(cells), width))
	}
	var b strings.Builder
	for _, c := range cells {
		style := base
		if c.style != nil {
			style = c.style
		}
		if style == nil || strings.TrimSpace(c.text) == "" {
			b.WriteString(c.text)
			continue
		}
		b.WriteString(style.Render(c.text))
	}
	return truncate(b.String(), width)
}

func joinPlain(cells []cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.text)
	}
	return b.String()
}

func rowCells(st styles, r row, c columns) ([]cell, *lipgloss.Style) {
	switch r.key.kind {
	case rowProject:
		return projectCells(st, r), nil
	case rowContainer:
		return containerCells(st, r, c), stateStyle(st, r.container.State)
	default:
		return processCells(st, r, c), stateStyle(st, r.container.State)
	}
}

func projectCells(st styles, r row) []cell {
	glyph := "▸ "
	if r.expanded {
		glyph = "▾ "
	}
	noun := "containers"
	if r.count == 1 {
		noun = "container"
	}
	desc := fmt.Sprintf("  %d %s", r.count, noun)
	if r.key.project != "" {
		desc = fmt.Sprintf("  compose, %d %s", r.count, noun)
	}
	return []cell{{text: glyph + projectName(r.key.project), style: &st.project}, {text: desc}}
}

func containerCells(st styles, r row, c columns) []cell {
	ct := r.container
	glyph := "  ▸ "
	if r.expanded {
		glyph = "  ▾ "
	}
	cells := c.cells(glyph+ct.Name, r.user, formatPercent(ct.CPUPercent),
		formatMemory(ct.MemoryBytes, ct.MemoryLimitBytes),
		formatNet(ct.NetRxBytesPerSec, ct.NetTxBytesPerSec), formatInt(ct.PIDs), ct.Status)
	markDanger(st, cells, c, ct.CPUPercent)
	return cells
}

func processCells(st styles, r row, c columns) []cell {
	p := r.process
	branch := "      ├─ "
	if r.last {
		branch = "      └─ "
	}
	user := p.User
	if user == "" {
		user = unknown
	}
	cells := c.cells(fmt.Sprintf("%s%d", branch, p.PID), user, formatPercent(p.CPUPercent),
		formatBytesPtr(p.MemoryBytes), "", formatInt(p.Threads), p.Command)
	markDanger(st, cells, c, p.CPUPercent)
	return cells
}

// markDanger colours the CPU cell when the value is high. The CPU cell follows the name cell
// and, when shown, the user cell, each preceded by a separator.
func markDanger(st styles, cells []cell, c columns, cpu *float64) {
	if cpu == nil || *cpu < dangerCPU {
		return
	}
	i := 2
	if c.user > 0 {
		i = 4
	}
	cells[i].style = &st.danger
}

func stateStyle(st styles, state model.ContainerState) *lipgloss.Style {
	switch state {
	case model.StateRunning:
		return nil
	case model.StatePaused:
		return &st.warning
	default:
		return &st.muted
	}
}
