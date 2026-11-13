package monitor

import (
	"math"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestContainerRates(t *testing.T) {
	first := model.ContainerStats{
		Read:            testStart,
		CPUTotal:        10 * time.Second,
		MemoryBytes:     100 << 20,
		HasNetwork:      true,
		NetRxBytes:      1000,
		NetTxBytes:      2000,
		BlockReadBytes:  4000,
		BlockWriteBytes: 8000,
		PIDs:            5,
	}
	advanced := func(edit func(*model.ContainerStats)) model.ContainerStats {
		s := first
		s.Read = testStart.Add(2 * time.Second)
		s.CPUTotal = 11 * time.Second
		s.NetRxBytes, s.NetTxBytes = 3000, 6000
		s.BlockReadBytes, s.BlockWriteBytes = 4000, 9000
		if edit != nil {
			edit(&s)
		}
		return s
	}

	tests := []struct {
		name                       string
		readings                   []model.ContainerStats
		wantCPU, wantRx, wantWrite *float64
	}{
		{name: "first reading", readings: []model.ContainerStats{first}},
		{
			name:     "two readings",
			readings: []model.ContainerStats{first, advanced(nil)},
			wantCPU:  new(50.0), wantRx: new(1000.0), wantWrite: new(500.0),
		},
		{
			name:     "cpu counter reset",
			readings: []model.ContainerStats{first, advanced(func(s *model.ContainerStats) { s.CPUTotal = time.Second })},
			wantRx:   new(1000.0), wantWrite: new(500.0),
		},
		{
			name:     "network counter reset",
			readings: []model.ContainerStats{first, advanced(func(s *model.ContainerStats) { s.NetRxBytes = 10 })},
			wantCPU:  new(50.0), wantWrite: new(500.0),
		},
		{
			name:     "no network",
			readings: []model.ContainerStats{first, advanced(func(s *model.ContainerStats) { s.HasNetwork = false })},
			wantCPU:  new(50.0), wantWrite: new(500.0),
		},
		{
			name:     "same read time",
			readings: []model.ContainerStats{first, advanced(func(s *model.ContainerStats) { s.Read = testStart })},
		},
		{
			name: "zero read time uses the monitor clock",
			readings: []model.ContainerStats{
				zeroRead(first),
				zeroRead(advanced(nil)),
			},
			wantCPU: new(50.0), wantRx: new(1000.0), wantWrite: new(500.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newFakeEngine()
			engine.containers = []model.ContainerInfo{running("app", nil)}
			clock := &fakeClock{t: testStart}
			m := newTestMonitor(engine, clock, nil, "")

			var c model.Container
			for i, reading := range tt.readings {
				if i > 0 {
					clock.advance(2 * time.Second)
				}
				engine.set(func(f *fakeEngine) { f.stats["app"] = reading })
				snapshot, err := m.tick(t.Context())
				if err != nil {
					t.Fatalf("tick: %v", err)
				}
				c = containerByID(t, snapshot, "app")
			}
			assertFloat(t, "CPUPercent", c.CPUPercent, tt.wantCPU)
			assertFloat(t, "NetRxBytesPerSec", c.NetRxBytesPerSec, tt.wantRx)
			assertFloat(t, "BlockWriteBytesPerSec", c.BlockWriteBytesPerSec, tt.wantWrite)
			if c.MemoryBytes == nil || *c.MemoryBytes != 100<<20 {
				t.Errorf("MemoryBytes = %v, want %d", c.MemoryBytes, 100<<20)
			}
			if c.PIDs == nil || *c.PIDs != 5 {
				t.Errorf("PIDs = %v, want 5", c.PIDs)
			}
		})
	}
}

func zeroRead(s model.ContainerStats) model.ContainerStats {
	s.Read = time.Time{}
	return s
}

func TestMemoryLimit(t *testing.T) {
	const host = 8 << 30
	tests := []struct {
		name  string
		limit uint64
		want  *int64
	}{
		{name: "zero", limit: 0},
		{name: "host memory means no limit", limit: host},
		{name: "above host memory", limit: host + 1},
		{name: "real limit", limit: 512 << 20, want: new(int64(512 << 20))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := memoryLimit(tt.limit, host)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("memoryLimit(%d) = %v, want %v", tt.limit, got, tt.want)
			}
		})
	}
}

func assertFloat(t *testing.T, name string, got, want *float64) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s = %v, want %v", name, ptrString(got), ptrString(want))
	case math.Abs(*got-*want) > 1e-9:
		t.Errorf("%s = %v, want %v", name, *got, *want)
	}
}

func ptrString(p *float64) any {
	if p == nil {
		return "nil"
	}
	return *p
}
