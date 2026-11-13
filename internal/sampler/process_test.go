package sampler

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatCommand(t *testing.T) {
	long := strings.Repeat("x", 510)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"single", "/sbin/init\x00", "/sbin/init"},
		{"arguments", "nginx\x00-g\x00daemon off;\x00", "nginx -g daemon off;"},
		{"empty arguments", "a\x00\x00b\x00", "a b"},
		{"no trailing nul", "sleep\x00infinity", "sleep infinity"},
		{"padded", "  top  \x00", "top"},
		{"empty", "", "[a) b (c]"},
		{"only nuls", "\x00\x00", "[a) b (c]"},
		{"capped", strings.Repeat("y", 600), strings.Repeat("y", maxCommandBytes)},
		{"capped before rune", long + "a" + "é" + "zz", long + "a"},
		{"capped after rune", long + "é" + "zz", long + "é"},
		{"capped inside 4-byte rune", long + "\U0001F600" + "zz", long},
	}
	r := newTestReader(fixtureRoot)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.formatCommand([]byte(tt.raw), []byte("a) b (c"))
			if got != tt.want {
				t.Errorf("formatCommand = %q, want %q", got, tt.want)
			}
			if len(got) > maxCommandBytes || !utf8.ValidString(got) {
				t.Errorf("result of %d bytes, valid UTF-8 %v", len(got), utf8.ValidString(got))
			}
		})
	}
}

func TestParseUID(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    uint32
		wantErr bool
	}{
		{"real uid first", "Name:\tx\nUid:\t1000\t0\t0\t0\nGid:\t5\t5\t5\t5\n", 1000, false},
		{"root", "Uid:\t0\t0\t0\t0\n", 0, false},
		{"max", "Uid:\t4294967295\t0\t0\t0\n", 4294967295, false},
		{"too large", "Uid:\t4294967296\t0\t0\t0\n", 0, true},
		{"missing", "Name:\tx\nGid:\t5\n", 0, true},
		{"garbage", "Uid:\tabc\n", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUID([]byte(tt.data))
			if tt.wantErr {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("error = %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("parseUID = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
}
