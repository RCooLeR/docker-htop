package sampler

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadUpTo(t *testing.T) {
	tests := []struct {
		name          string
		size          int
		limit         int
		wantLen       int
		wantTruncated bool
	}{
		{"empty", 0, 10, 0, false},
		{"under", 5, 10, 5, false},
		{"exact", 10, 10, 10, false},
		{"over", 11, 10, 10, true},
		{"far over", 100_000, 10, 10, true},
		{"large under", 100_000, 1 << 20, 100_000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "f")
			content := strings.Repeat("a", tt.size)
			writeFile(t, path, content)
			for _, buf := range [][]byte{nil, make([]byte, 0, 3), make([]byte, 0, 1<<21)} {
				data, truncated, err := readUpTo(path, buf, tt.limit)
				if err != nil {
					t.Fatalf("readUpTo: %v", err)
				}
				if len(data) != tt.wantLen || truncated != tt.wantTruncated || string(data) != content[:tt.wantLen] {
					t.Errorf("cap %d: got %d bytes, truncated %v; want %d, %v",
						cap(buf), len(data), truncated, tt.wantLen, tt.wantTruncated)
				}
			}
		})
	}
}

func TestReadUpToMissing(t *testing.T) {
	_, _, err := readUpTo(filepath.Join(t.TempDir(), "missing"), nil, 10)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want fs.ErrNotExist", err)
	}
}
