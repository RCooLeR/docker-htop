package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Core bars turn yellow and red from these busy shares.
const (
	busyMid  = 0.60
	busyHigh = 0.85
)

// minMeterInner is the narrowest bar interior a core meter is laid out with.
const minMeterInner = 16

// segment is one coloured part of a meter, as a share of the whole.
type segment struct {
	frac  float64
	style lipgloss.Style
}

// renderHeader draws the meters, the summary lines and the notice, each at most width cells.
func renderHeader(st styles, s model.Snapshot, width int, opts Options, sort sortKey) []string {
	var lines []string
	if len(s.Host.CoreBusy) > 0 {
		lines = append(lines, coreMeters(st, s.Host.CoreBusy, width)...)
		lines = append(lines, pairMeters(memoryMeter(st, s.Host.Memory), swapMeter(st, s.Host.Memory), width)...)
	} else {
		lines = append(lines, pairMeters(engineCPUMeter(st, s), engineMemoryMeter(st, s), width)...)
	}
	lines = append(lines, packParts(append(summaryParts(st, s), engineParts(st, s, opts, sort)...), width)...)
	if s.Notice != "" {
		lines = append(lines, st.warning.Render(truncate(s.Notice, width)))
	}
	for i, l := range lines {
		lines[i] = truncate(l, width)
	}
	return lines
}

// meter draws label[bars text] in exactly width cells.
func meter(st styles, label string, width int, segs []segment, text string) string {
	inner := width - lipgloss.Width(label) - 2
	if inner <= 0 {
		return truncate(st.label.Render(label), width)
	}
	textWidth := lipgloss.Width(text)
	slots := inner - textWidth - 1
	if slots < 0 {
		text, textWidth, slots = "", 0, inner
	}
	var b strings.Builder
	b.WriteString(st.label.Render(label))
	b.WriteString(st.bracket.Render("["))
	used, cum := 0, 0.0
	for _, sg := range segs {
		cum += max(0, sg.frac)
		end := min(slots, int(math.Round(min(cum, 1)*float64(slots))))
		if n := end - used; n > 0 {
			b.WriteString(sg.style.Render(strings.Repeat("|", n)))
			used = end
		}
	}
	b.WriteString(strings.Repeat(" ", inner-used-textWidth))
	b.WriteString(text)
	b.WriteString(st.bracket.Render("]"))
	return b.String()
}

// meterFunc draws a meter at a width chosen by the layout.
type meterFunc func(width int) string

func coreMeters(st styles, busy []float64, width int) []string {
	n := len(busy)
	labelWidth := len(strconv.Itoa(n - 1))
	minCell := labelWidth + 2 + minMeterInner
	cols := max(1, min(n, (width+1)/(minCell+1)))
	rows := (n + cols - 1) / cols
	cols = (n + rows - 1) / rows
	cellWidth := max(1, (width-(cols-1))/cols)

	lines := make([]string, rows)
	for r := range rows {
		cells := make([]string, 0, cols)
		for c := range cols {
			i := c*rows + r
			if i >= n {
				break
			}
			b := busy[i]
			pct := b * 100
			label := fmt.Sprintf("%*d", labelWidth, i)
			cells = append(cells, meter(st, label, cellWidth,
				[]segment{{frac: b, style: busyStyle(st, b)}}, formatPercent(&pct)+"%"))
		}
		lines[r] = strings.Join(cells, " ")
	}
	return lines
}

func busyStyle(st styles, busy float64) lipgloss.Style {
	switch {
	case busy >= busyHigh:
		return st.barHigh
	case busy >= busyMid:
		return st.barMid
	default:
		return st.barLow
	}
}

// pairMeters puts two meters on one line when there is room, otherwise one per line.
func pairMeters(left, right meterFunc, width int) []string {
	const gap = 3
	if width < 60 {
		return []string{left(width), right(width)}
	}
	leftWidth := (width - gap) * 3 / 5
	return []string{left(leftWidth) + strings.Repeat(" ", gap) + right(width-gap-leftWidth)}
}

func memoryMeter(st styles, m *model.HostMemory) meterFunc {
	return func(width int) string {
		if m == nil || m.TotalBytes <= 0 {
			return meter(st, "Mem", width, nil, unknown)
		}
		total := float64(m.TotalBytes)
		return meter(st, "Mem", width, []segment{
			{frac: float64(m.UsedBytes) / total, style: st.memUsed},
			{frac: float64(m.CacheBytes) / total, style: st.memCache},
		}, formatBytes(float64(m.UsedBytes))+"/"+formatBytes(total))
	}
}

func swapMeter(st styles, m *model.HostMemory) meterFunc {
	return func(width int) string {
		if m == nil {
			return meter(st, "Swp", width, nil, unknown)
		}
		var frac float64
		if m.SwapTotalBytes > 0 {
			frac = float64(m.SwapUsedBytes) / float64(m.SwapTotalBytes)
		}
		return meter(st, "Swp", width, []segment{{frac: frac, style: st.swap}},
			formatBytes(float64(m.SwapUsedBytes))+"/"+formatBytes(float64(m.SwapTotalBytes)))
	}
}

// engineCPUMeter is the API-mode CPU meter: the containers' summed CPU over all engine cores.
func engineCPUMeter(st styles, s model.Snapshot) meterFunc {
	return func(width int) string {
		var sum float64
		known := false
		for _, c := range s.Containers {
			if c.CPUPercent != nil {
				sum += *c.CPUPercent
				known = true
			}
		}
		if !known || s.Engine.CPUs <= 0 {
			return meter(st, "CPU", width, nil, unknown)
		}
		pct := sum / float64(s.Engine.CPUs)
		frac := pct / 100
		return meter(st, "CPU", width, []segment{{frac: frac, style: busyStyle(st, frac)}},
			formatPercent(&pct)+"%")
	}
}

// engineMemoryMeter is the API-mode memory meter: the containers' summed usage over the
// engine's memory.
func engineMemoryMeter(st styles, s model.Snapshot) meterFunc {
	return func(width int) string {
		var sum int64
		known := false
		for _, c := range s.Containers {
			if c.MemoryBytes != nil {
				sum += *c.MemoryBytes
				known = true
			}
		}
		if !known || s.Engine.MemoryBytes <= 0 {
			return meter(st, "Mem", width, nil, unknown)
		}
		total := float64(s.Engine.MemoryBytes)
		return meter(st, "Mem", width, []segment{{frac: float64(sum) / total, style: st.memUsed}},
			formatBytes(float64(sum))+"/"+formatBytes(total))
	}
}

// summaryPart is one "Label: value" item of the summary lines; breakBefore starts a new line.
type summaryPart struct {
	text        string
	breakBefore bool
}

// packParts joins the parts with gaps, starting a new line where a part is marked or would not
// fit in width.
func packParts(parts []summaryPart, width int) []string {
	const gap = "   "
	var lines []string
	for _, p := range parts {
		last := len(lines) - 1
		if last >= 0 && !p.breakBefore && lipgloss.Width(lines[last]+gap+p.text) <= width {
			lines[last] += gap + p.text
			continue
		}
		lines = append(lines, p.text)
	}
	return lines
}

func summaryParts(st styles, s model.Snapshot) []summaryPart {
	counts := map[model.ContainerState]int{}
	for _, c := range s.Containers {
		counts[c.State]++
	}
	parts := []string{fmt.Sprintf("%d running", counts[model.StateRunning])}
	for _, state := range []model.ContainerState{
		model.StatePaused, model.StateRestarting, model.StateCreated,
		model.StateRemoving, model.StateExited, model.StateDead,
	} {
		if counts[state] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[state], state))
		}
	}
	line := summaryPart{text: st.label.Render("Containers: ") + strings.Join(parts, ", ")}
	if s.Mode == model.ModeAPI {
		return []summaryPart{line}
	}
	tasks := formatInt(s.Host.Tasks)
	if s.Host.Tasks != nil && s.Host.Threads != nil {
		tasks += fmt.Sprintf(", %d thr", *s.Host.Threads)
	}
	load := unknown
	if len(s.Host.Load) == 3 {
		load = fmt.Sprintf("%.2f %.2f %.2f", s.Host.Load[0], s.Host.Load[1], s.Host.Load[2])
	}
	return []summaryPart{
		line,
		{text: st.label.Render("Tasks: ") + tasks},
		{text: st.label.Render("Load: ") + load},
	}
}

func engineParts(st styles, s model.Snapshot, opts Options, sort sortKey) []summaryPart {
	engine := strings.TrimSpace(s.Engine.Name + " " + s.Engine.Version)
	if engine == "" {
		engine = unknown
	}
	source := "sampler"
	switch s.Mode {
	case model.ModeAPI:
		source = "Engine API"
	case model.ModeLocal:
		source = "local /proc"
	}
	refresh := unknown
	if opts.Interval > 0 {
		refresh = opts.Interval.String()
	}
	return []summaryPart{
		{text: st.label.Render("Engine: ") + engine, breakBefore: true},
		{text: st.label.Render("Source: ") + source},
		{text: st.label.Render("Refresh: ") + refresh},
		{text: st.label.Render("Sort: ") + sort.String()},
	}
}
