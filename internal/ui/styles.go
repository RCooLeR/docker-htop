package ui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// ANSI palette indexes. Only the 16 base colours are used so the UI follows the terminal's own
// palette on dark and light backgrounds alike.
const (
	colourRed     = lipgloss.Color("1")
	colourGreen   = lipgloss.Color("2")
	colourYellow  = lipgloss.Color("3")
	colourBlue    = lipgloss.Color("4")
	colourMagenta = lipgloss.Color("5")
	colourCyan    = lipgloss.Color("6")
	colourMuted   = lipgloss.Color("8")
)

// styles holds every style the UI draws with, bound to one renderer so tests can use a
// colourless one.
type styles struct {
	renderer *lipgloss.Renderer

	barLow   lipgloss.Style
	barMid   lipgloss.Style
	barHigh  lipgloss.Style
	memUsed  lipgloss.Style
	memCache lipgloss.Style
	swap     lipgloss.Style
	label    lipgloss.Style
	bracket  lipgloss.Style

	project    lipgloss.Style
	warning    lipgloss.Style
	danger     lipgloss.Style
	muted      lipgloss.Style
	cursor     lipgloss.Style
	columnLine lipgloss.Style
	keyName    lipgloss.Style
	keyLabel   lipgloss.Style
	dialog     lipgloss.Style
	title      lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) styles {
	s := r.NewStyle()
	return styles{
		renderer:   r,
		barLow:     s.Foreground(colourGreen),
		barMid:     s.Foreground(colourYellow),
		barHigh:    s.Foreground(colourRed),
		memUsed:    s.Foreground(colourGreen),
		memCache:   s.Foreground(colourBlue),
		swap:       s.Foreground(colourRed),
		label:      s.Foreground(colourCyan),
		bracket:    s.Bold(true),
		project:    s.Bold(true).Foreground(colourMagenta),
		warning:    s.Foreground(colourYellow),
		danger:     s.Foreground(colourRed),
		muted:      s.Foreground(colourMuted),
		cursor:     s.Reverse(true),
		columnLine: s.Reverse(true),
		keyName:    s.Bold(true),
		keyLabel:   s.Reverse(true),
		dialog:     s.Border(lipgloss.NormalBorder()).Padding(0, 1),
		title:      s.Bold(true),
	}
}

// plainStyles returns styles whose output carries no escape sequences.
func plainStyles() styles {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.Ascii)
	return newStyles(r)
}

// colourStyles returns styles that always emit 16-colour ANSI sequences.
func colourStyles(w io.Writer) styles {
	r := lipgloss.NewRenderer(w)
	r.SetColorProfile(termenv.ANSI)
	return newStyles(r)
}
