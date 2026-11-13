package ui

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{44 * kib, "44K"},
		{9.8 * mib, "9.8M"},
		{398 * mib, "398M"},
		{1000 * kib, "1.0M"},
		{gib, "1.0G"},
		{15.6 * gib, "15.6G"},
		{-5, "0B"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatBytes(tt.in); got != tt.want {
				t.Errorf("formatBytes(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatReadings(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"percent nil", formatPercent(nil), "—"},
		{"percent zero", formatPercent(f64(0)), "0.0"},
		{"percent", formatPercent(f64(37.94)), "37.9"},
		{"memory no limit", formatMemory(i64(24*mib), nil), "24M/—"},
		{"memory unknown", formatMemory(nil, i64(gib)), "—"},
		{"memory", formatMemory(i64(398*mib), i64(gib)), "398M/1.0G"},
		{"net unknown", formatNet(nil, nil), "—"},
		{"net half", formatNet(f64(48*kib), nil), "48K/—"},
		{"int nil", formatInt(nil), "—"},
		{"int zero", formatInt(intp(0)), "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
