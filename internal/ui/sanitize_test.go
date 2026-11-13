package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean", "postgres: walwriter", "postgres: walwriter"},
		{"newline", "a\nb", "a b"},
		{"tab", "a\tb", "a b"},
		{"escape sequence", "a\x1b[31mred", "a [31mred"},
		{"replaced next to spaces", "a \n b", "a b"},
		{"plain spaces kept", "a  b", "a  b"},
		{"invalid utf-8", "a\xff\xfeb", "a b"},
		{"del and c1", "a\x7f\u0085b", "a b"},
		{"line separator", "a b", "a b"},
		{"unicode kept", "naïve — ok", "naïve — ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.in); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHostileStringsRenderOnOneLine(t *testing.T) {
	const hostile = "evil\ncmd\targ \x1b[31mred\x1b[0m \xff\xfe end"
	s := samplerSnapshot()
	s.Containers[2].Name = "pg\r\nname"
	s.Containers[2].Status = "Up\x1b]0;title\x07"
	s.Containers[2].Processes[1].Command = hostile
	s.Containers[2].Processes[1].User = "post\tgres"
	s.Notice = "notice\nsecond line"

	views := map[string]string{
		"plain": RenderPlain(s, 120, false, Options{Interval: time.Second}),
	}
	m, _ := newTestScreen(t, s, 120, 40)
	views["screen"] = m.View()

	for name, out := range views {
		t.Run(name, func(t *testing.T) {
			if strings.ContainsRune(out, '\x1b') || strings.ContainsRune(out, '\r') || strings.ContainsRune(out, '\t') {
				t.Fatalf("output contains control characters:\n%q", out)
			}
			assertMaxWidth(t, out, 120)
			var row string
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "2304") {
					row = line
				}
			}
			if !strings.Contains(row, "evil cmd arg [31mred [0m end") {
				t.Errorf("process row = %q", row)
			}
			if !strings.Contains(out, "notice second line") || !strings.Contains(out, "pg name") {
				t.Errorf("notice or name not joined onto one line:\n%s", out)
			}
		})
	}
	if s.Containers[2].Processes[1].Command != hostile {
		t.Error("sanitizing modified the caller's snapshot")
	}
}

func TestSanitizeSnapshotCopies(t *testing.T) {
	s := model.Snapshot{Containers: []model.Container{{Name: "a\nb"}}}
	_ = sanitizeSnapshot(s) // only the side effect on s matters here
	if s.Containers[0].Name != "a\nb" {
		t.Error("caller's container modified")
	}
}
