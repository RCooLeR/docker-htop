package sampler

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const (
	fixtureRoot = "testdata/proc"
	fixtureID1  = "7692c3ad3540bb803c020b3aee66cd8887123234ea0c6e7143c0add73ff431ed"
	fixtureID2  = "3fc4ccfe745870e2c0d99f71f30ff0656c8dedd41cc1d7d3d376b0dbe685e2f3"
	fixtureID3  = "8b5b9db0c13db24256c829aa364aa90c6d2eba318b9232a4ab9313b954d3555f"
	fixtureID4  = "04efaf080f5a3e74e1c29d1ca6a48569382cbbcd324e8d59d2b83ef21c039f00"
	testPage    = 4096
)

var fixtureTime = time.UnixMilli(1_700_000_000_123)

func newTestReader(root string) *Reader {
	r := NewReader(root)
	r.pageSize = testPage
	r.now = func() time.Time { return fixtureTime }
	return r
}

func TestReaderReadFixture(t *testing.T) {
	got, err := newTestReader(fixtureRoot).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := model.Sample{
		TimeUnixMs: fixtureTime.UnixMilli(),
		CPUs: []model.CPUTimes{
			{TotalMs: 11700, IdleMs: 10100},
			{TotalMs: 22950, IdleMs: 20200},
		},
		Memory: model.SampleMemory{
			TotalBytes:     16318412 * 1024,
			FreeBytes:      8000000 * 1024,
			AvailableBytes: 12000000 * 1024,
			BuffersBytes:   100000 * 1024,
			CachedBytes:    3200000 * 1024,
			SwapTotalBytes: 4194304 * 1024,
			SwapFreeBytes:  4000000 * 1024,
		},
		Load:    []float64{0.52, 0.58, 0.59},
		Tasks:   6, // pids 1, 2, 100, 200, 300, 400; 500 has vanished
		Threads: 12,
		Processes: []model.ProcessSample{
			{PID: 100, PPID: 1, UID: 1000, State: "S", Threads: 3, StartMs: 50000, CPUTimeMs: 3000,
				RSSBytes: 512 * testPage, Command: "[a) b (c]", ContainerID: fixtureID1},
			{PID: 200, PPID: 1, UID: 101, State: "S", Threads: 2, StartMs: 60000, CPUTimeMs: 100,
				RSSBytes: 1024 * testPage, Command: "nginx -g daemon off;", ContainerID: fixtureID2},
			{PID: 300, PPID: 1, UID: 0, State: "S", Threads: 1, StartMs: 70000, CPUTimeMs: 20,
				RSSBytes: 100 * testPage, Command: "python3 -c " + strings.Repeat("x", 500), ContainerID: fixtureID3},
			{PID: 400, PPID: 1, UID: 999, State: "S", Threads: 4, StartMs: 80000, CPUTimeMs: 200,
				RSSBytes: 2048 * testPage, Command: "redis-server *:6379", ContainerID: fixtureID4},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Read mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestReaderReadRootErrors(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string // empty removes the file
		wantErr error
	}{
		{"missing stat", "stat", "", fs.ErrNotExist},
		{"missing meminfo", "meminfo", "", fs.ErrNotExist},
		{"malformed loadavg", "loadavg", "x y z\n", ErrMalformed},
		{"no cpu lines", "stat", "intr 1\n", ErrMalformed},
		{"oversized stat", "stat", strings.Repeat("a", hostStatLimit+1), ErrFileTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyFixture(t)
			path := filepath.Join(root, tt.file)
			if tt.content == "" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, path, tt.content)
			}
			if _, err := newTestReader(root).Read(); !errors.Is(err, tt.wantErr) {
				t.Errorf("Read error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestReaderSkipsBadProcesses(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
	}{
		{"oversized stat", "100/stat", strings.Repeat("1 ", procStatLimit)},
		{"malformed stat", "100/stat", "100 (x) S 1\n"},
		{"malformed status", "100/status", "Name:\tx\n"},
		{"missing cmdline", "100/cmdline", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyFixture(t)
			path := filepath.Join(root, tt.file)
			if tt.content == "" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, path, tt.content)
			}
			got, err := newTestReader(root).Read()
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if got.Tasks != 5 || got.Threads != 9 || len(got.Processes) != 3 {
				t.Errorf("tasks %d threads %d processes %d, want 5, 9, 3", got.Tasks, got.Threads, len(got.Processes))
			}
			for _, p := range got.Processes {
				if p.PID == 100 {
					t.Errorf("pid 100 reported: %+v", p)
				}
			}
		})
	}
}

// copyFixture copies the fixture tree into a temporary directory the test may change.
func copyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixtureRoot)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
