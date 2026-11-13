package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// unknown is how every missing reading is rendered.
const unknown = "—"

const byteUnits = "KMGTPE"

// formatBytes renders a byte count in binary units with about three significant characters:
// 512B, 44K, 9.8M, 398M, 1.0G, 15.6G.
func formatBytes(n float64) string {
	if n < 0 {
		n = 0
	}
	if n < 1024 {
		return fmt.Sprintf("%.0fB", n)
	}
	v := n
	for i, unit := range byteUnits {
		v /= 1024
		last := i == len(byteUnits)-1
		if v >= 999.5 && !last {
			continue
		}
		if i >= 2 || v < 9.95 {
			return fmt.Sprintf("%.1f%c", v, unit)
		}
		return fmt.Sprintf("%.0f%c", v, unit)
	}
	return unknown
}

func formatBytesPtr(n *int64) string {
	if n == nil {
		return unknown
	}
	return formatBytes(float64(*n))
}

func formatRatePtr(n *float64) string {
	if n == nil {
		return unknown
	}
	return formatBytes(*n)
}

func formatPercent(p *float64) string {
	if p == nil {
		return unknown
	}
	return fmt.Sprintf("%.1f", *p)
}

func formatInt(n *int) string {
	if n == nil {
		return unknown
	}
	return fmt.Sprintf("%d", *n)
}

// formatMemory renders used/limit; a missing usage hides the limit, a missing limit shows —.
func formatMemory(used, limit *int64) string {
	if used == nil {
		return unknown
	}
	return formatBytesPtr(used) + "/" + formatBytesPtr(limit)
}

func formatNet(rx, tx *float64) string {
	if rx == nil && tx == nil {
		return unknown
	}
	return formatRatePtr(rx) + "/" + formatRatePtr(tx)
}

// padRight pads or truncates s to exactly w cells.
func padRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = truncate(s, w)
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// padLeft right-aligns s in exactly w cells.
func padLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = truncate(s, w)
	return strings.Repeat(" ", w-lipgloss.Width(s)) + s
}

// truncate cuts s, which may contain escape sequences, to at most w cells with an ellipsis.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}
