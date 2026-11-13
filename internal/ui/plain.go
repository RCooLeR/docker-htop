package ui

import (
	"os"
	"strings"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// RenderPlain renders one snapshot as text for the --no-stream flag: the same header, column
// line and tree with everything expanded, no cursor and no key bar. It emits no ANSI sequences
// when colour is false. A width of zero or less means 80 columns.
func RenderPlain(s model.Snapshot, width int, colour bool, opts Options) string {
	if width <= 0 {
		width = defaultWidth
	}
	s = sanitizeSnapshot(s)
	st := plainStyles()
	if colour {
		st = colourStyles(os.Stdout)
	}
	rows := buildRows(s, treeState{showAll: opts.ShowAll})
	lines := renderHeader(st, s, width, opts, sortCPU)
	lines = append(lines, columnLine(st, width))
	for _, r := range rows {
		lines = append(lines, renderRow(st, r, width, false))
	}
	return strings.Join(lines, "\n") + "\n"
}
