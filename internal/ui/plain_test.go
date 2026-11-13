package ui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func TestRenderPlainGolden(t *testing.T) {
	tests := []struct {
		name  string
		snap  model.Snapshot
		width int
		opts  Options
	}{
		{"plain_sampler", samplerSnapshot(), 120, Options{Interval: time.Second, ShowAll: true}},
		{"plain_api", apiSnapshot(), 100, Options{Interval: 2 * time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderPlain(tt.snap, tt.width, false, tt.opts)
			if strings.Contains(got, "\x1b") {
				t.Fatalf("plain output contains an escape sequence:\n%q", got)
			}
			assertMaxWidth(t, got, tt.width)
			path := filepath.Join("testdata", tt.name+".golden")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden %s: %v", path, err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v", path, err)
			}
			if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
				t.Errorf("output differs from %s:\n--- got\n%s\n--- want\n%s", path, got, want)
			}
		})
	}
}

func TestRenderPlainColour(t *testing.T) {
	got := RenderPlain(samplerSnapshot(), 120, true, Options{})
	if !strings.Contains(got, "\x1b[") {
		t.Fatal("colour output has no escape sequences")
	}
	assertMaxWidth(t, got, 120)
}
