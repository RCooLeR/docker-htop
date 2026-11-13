package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// sanitize makes a string from the engine or a process safe to draw on one line: every control
// character (C0 including ESC, DEL, C1), line or paragraph separator and invalid UTF-8 byte
// becomes a space, and each run of such spaces collapses to one.
func sanitize(s string) string {
	clean := true
	for _, r := range s {
		if unsafeRune(r) {
			clean = false
			break
		}
	}
	if clean && utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	// A run of spaces that contains a replaced character collapses to a single space; runs of
	// plain spaces are kept as they are.
	spaces, replaced := 0, false
	flush := func() {
		switch {
		case replaced:
			b.WriteByte(' ')
		case spaces > 0:
			b.WriteString(strings.Repeat(" ", spaces))
		}
		spaces, replaced = 0, false
	}
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		switch {
		case (r == utf8.RuneError && size <= 1) || unsafeRune(r):
			replaced = true
		case r == ' ':
			spaces++
		default:
			flush()
			b.WriteRune(r)
		}
	}
	flush()
	return b.String()
}

func unsafeRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == ' ' || r == ' '
}

// sanitizeSnapshot returns a copy of s with every drawn string sanitized. The containers and
// processes are copied so the sender's slices are never modified.
func sanitizeSnapshot(s model.Snapshot) model.Snapshot {
	s.Notice = sanitize(s.Notice)
	s.Engine.Name = sanitize(s.Engine.Name)
	s.Engine.Version = sanitize(s.Engine.Version)
	s.Containers = slices.Clone(s.Containers)
	for i := range s.Containers {
		c := &s.Containers[i]
		c.Name = sanitize(c.Name)
		c.Image = sanitize(c.Image)
		c.Project = sanitize(c.Project)
		c.Status = sanitize(c.Status)
		c.Processes = slices.Clone(c.Processes)
		for j := range c.Processes {
			p := &c.Processes[j]
			p.User = sanitize(p.User)
			p.State = sanitize(p.State)
			p.Command = sanitize(p.Command)
		}
	}
	return s
}
