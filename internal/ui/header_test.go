package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestHeaderSampler(t *testing.T) {
	tests := []struct {
		width    int
		coreRows int
		lines    int
		firstRow []string
	}{
		{80, 4, 8, []string{" 0[", " 4[", " 8["}},
		{120, 3, 6, []string{" 0[", " 3[", " 6[", " 9["}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.width), func(t *testing.T) {
			lines := renderHeader(plainStyles(), samplerSnapshot(), tt.width, Options{Interval: time.Second}, sortCPU)
			if len(lines) != tt.lines {
				t.Fatalf("got %d header lines, want %d:\n%s", len(lines), tt.lines, strings.Join(lines, "\n"))
			}
			assertMaxWidth(t, strings.Join(lines, "\n"), tt.width)
			for _, cell := range tt.firstRow {
				if !strings.HasPrefix(lines[0], cell) && !strings.Contains(lines[0], " "+cell) {
					t.Errorf("first core row %q lacks %q", lines[0], cell)
				}
			}
			for _, want := range []string{"38.2%", "91.6%", "11["} {
				if !strings.Contains(strings.Join(lines[:tt.coreRows], "\n"), want) {
					t.Errorf("core rows lack %q", want)
				}
			}
			mem := lines[tt.coreRows]
			for _, want := range []string{"Mem[", "9.8G/15.6G]", "Swp[", "300M/4.0G]"} {
				if !strings.Contains(mem, want) {
					t.Errorf("memory line %q lacks %q", mem, want)
				}
			}
			summary := strings.Join(lines[tt.coreRows+1:], "\n")
			for _, want := range []string{
				"Containers: 4 running, 1 paused, 1 exited", "Tasks: 154, 412 thr", "Load: 1.84 1.32 0.97",
				"Engine: docker-desktop 29.9.2", "Source: sampler", "Refresh: 1s", "Sort: CPU%",
			} {
				if !strings.Contains(summary, want) {
					t.Errorf("summary lacks %q:\n%s", want, summary)
				}
			}
		})
	}
}

func TestHeaderLocal(t *testing.T) {
	s := samplerSnapshot()
	s.Mode = model.ModeLocal
	out := strings.Join(renderHeader(plainStyles(), s, 120, Options{Interval: time.Second}, sortCPU), "\n")
	for _, want := range []string{"Source: local /proc", "Swp[", "Tasks:", "Load:"} {
		if !strings.Contains(out, want) {
			t.Errorf("local header lacks %q:\n%s", want, out)
		}
	}
}

func TestHeaderAPI(t *testing.T) {
	const width = 100
	lines := renderHeader(plainStyles(), apiSnapshot(), width, Options{Interval: 2 * time.Second}, sortMemory)
	out := strings.Join(lines, "\n")
	assertMaxWidth(t, out, width)
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4:\n%s", len(lines), out)
	}
	for _, want := range []string{"CPU[", "8.5%]", "Mem[", "2.0G/15.6G]", "Source: Engine API", "Sort: MEM", "Refresh: 2s"} {
		if !strings.Contains(out, want) {
			t.Errorf("header lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"Swp", "Tasks:", "Load:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("API header shows %q:\n%s", unwanted, out)
		}
	}
	if !strings.HasPrefix(lines[3], "sampler image not available") {
		t.Errorf("notice line = %q", lines[3])
	}

	narrow := renderHeader(plainStyles(), apiSnapshot(), 40, Options{}, sortCPU)
	assertMaxWidth(t, strings.Join(narrow, "\n"), 40)
	if last := narrow[len(narrow)-1]; !strings.HasSuffix(last, "…") {
		t.Errorf("long notice not truncated: %q", last)
	}
}

func TestHeaderMissingReadings(t *testing.T) {
	s := samplerSnapshot()
	s.Host.Memory, s.Host.Load, s.Host.Tasks = nil, nil, nil
	out := strings.Join(renderHeader(plainStyles(), s, 120, Options{}, sortCPU), "\n")
	for _, want := range []string{"—]", "Tasks: —", "Load: —", "Refresh: —"} {
		if !strings.Contains(out, want) {
			t.Errorf("header lacks %q:\n%s", want, out)
		}
	}
}
