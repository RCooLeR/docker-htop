package monitor

import (
	"slices"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const appID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestLocalSamples(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{running(appID, nil)}
	samples := make(chan model.Sample)
	m := newTestMonitor(engine, &fakeClock{t: testStart}, samples, "")
	m.local = true

	m.receive(model.Sample{TimeUnixMs: 10_000, Processes: []model.ProcessSample{
		{PID: 10, StartMs: 100, CPUTimeMs: 1000, ContainerID: appID},
	}}, true)
	snapshot, err := m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if snapshot.Mode != model.ModeLocal {
		t.Errorf("mode = %q, want %q", snapshot.Mode, model.ModeLocal)
	}
	if got := len(containerByID(t, snapshot, appID).Processes); got != 1 {
		t.Errorf("got %d processes from the local samples, want 1", got)
	}

	m.receive(model.Sample{}, false)
	snapshot, err = m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick after the samples ended: %v", err)
	}
	if snapshot.Mode != model.ModeAPI {
		t.Errorf("mode after the samples ended = %q, want %q", snapshot.Mode, model.ModeAPI)
	}
}

func TestSamplerProcessCPU(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{running(appID, nil)}
	samples := make(chan model.Sample)
	m := newTestMonitor(engine, &fakeClock{t: testStart}, samples, "")

	snapshot, err := m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if snapshot.Mode != model.ModeSampler || containerByID(t, snapshot, appID).Processes != nil {
		t.Fatalf("before any sample: mode %q, want sampler mode without processes", snapshot.Mode)
	}

	m.receive(model.Sample{TimeUnixMs: 10_000, Processes: []model.ProcessSample{
		{PID: 10, StartMs: 100, CPUTimeMs: 1000, ContainerID: appID},
		{PID: 11, StartMs: 200, CPUTimeMs: 1000, ContainerID: appID},
	}}, true)
	m.receive(model.Sample{TimeUnixMs: 12_000, Processes: []model.ProcessSample{
		{PID: 10, PPID: 1, UID: 1000, State: "R", Threads: 3, StartMs: 100, CPUTimeMs: 2000,
			RSSBytes: 4096, Command: "app --serve", ContainerID: appID},
		{PID: 11, StartMs: 300, CPUTimeMs: 1500, ContainerID: appID},
		{PID: 12, StartMs: 400, CPUTimeMs: 10, ContainerID: appID},
		{PID: 13, StartMs: 400, CPUTimeMs: 10, ContainerID: "other"},
	}}, true)
	snapshot, err = m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}

	processes := containerByID(t, snapshot, appID).Processes
	if len(processes) != 3 {
		t.Fatalf("got %d processes, want 3", len(processes))
	}
	byPID := make(map[int]model.Process)
	for _, p := range processes {
		byPID[p.PID] = p
	}
	tests := []struct {
		name string
		pid  int
		want *float64
	}{
		{name: "seen in both samples", pid: 10, want: new(50.0)},
		{name: "reused pid", pid: 11},
		{name: "new process", pid: 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertFloat(t, "CPUPercent", byPID[tt.pid].CPUPercent, tt.want)
		})
	}

	p := byPID[10]
	if p.User != "1000" || p.State != "R" || p.PPID != 1 || *p.Threads != 3 || *p.MemoryBytes != 4096 || p.Command != "app --serve" {
		t.Errorf("process 10 = %+v, want the sample's fields and the numeric user", p)
	}
}

func TestCoreBusy(t *testing.T) {
	tests := []struct {
		name     string
		previous []model.CPUTimes
		latest   []model.CPUTimes
		want     []float64
	}{
		{
			name:     "busy share",
			previous: []model.CPUTimes{{TotalMs: 1000, IdleMs: 500}, {TotalMs: 1000, IdleMs: 500}},
			latest:   []model.CPUTimes{{TotalMs: 2000, IdleMs: 750}, {TotalMs: 2000, IdleMs: 1500}},
			want:     []float64{0.75, 0},
		},
		{
			name:     "clamped",
			previous: []model.CPUTimes{{TotalMs: 1000, IdleMs: 500}, {TotalMs: 1000, IdleMs: 500}},
			latest:   []model.CPUTimes{{TotalMs: 2000, IdleMs: 2000}, {TotalMs: 2000, IdleMs: 400}},
			want:     []float64{0, 1},
		},
		{
			name:     "core count changed",
			previous: []model.CPUTimes{{TotalMs: 1000}},
			latest:   []model.CPUTimes{{TotalMs: 2000}, {TotalMs: 2000}},
		},
		{
			name:     "total did not advance",
			previous: []model.CPUTimes{{TotalMs: 1000}},
			latest:   []model.CPUTimes{{TotalMs: 1000}},
		},
		{name: "single sample", latest: []model.CPUTimes{{TotalMs: 1000}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h samplerHistory
			if tt.previous != nil {
				h.add(model.Sample{CPUs: tt.previous})
			}
			h.add(model.Sample{CPUs: tt.latest})
			if got := h.coreBusy(); !slices.Equal(got, tt.want) {
				t.Errorf("coreBusy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSamplerHost(t *testing.T) {
	var h samplerHistory
	if host := h.host(); host.Memory != nil || host.Tasks != nil {
		t.Errorf("host before any sample = %+v, want unknown", host)
	}
	h.add(model.Sample{
		Memory: model.SampleMemory{
			TotalBytes: 1000, FreeBytes: 300, BuffersBytes: 100, CachedBytes: 200,
			SwapTotalBytes: 500, SwapFreeBytes: 400,
		},
		Load:  []float64{1, 2, 3},
		Tasks: 7, Threads: 20,
	})
	host := h.host()
	want := model.HostMemory{TotalBytes: 1000, UsedBytes: 400, CacheBytes: 300, SwapTotalBytes: 500, SwapUsedBytes: 100}
	if *host.Memory != want {
		t.Errorf("memory = %+v, want %+v", *host.Memory, want)
	}
	if !slices.Equal(host.Load, []float64{1, 2, 3}) || *host.Tasks != 7 || *host.Threads != 20 {
		t.Errorf("host = %+v, want load, tasks and threads from the sample", host)
	}
	if hostMemory(model.SampleMemory{TotalBytes: 100, FreeBytes: 90, CachedBytes: 50}).UsedBytes != 0 {
		t.Error("used memory below zero not clamped")
	}
}

func TestSamplerClosedSwitchesToAPI(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{running(appID, nil)}
	engine.tops[appID] = []model.TopProcess{{PID: 10, User: "root"}}
	samples := make(chan model.Sample)
	m := newTestMonitor(engine, &fakeClock{t: testStart}, samples, "ignored while sampling")
	m.receive(model.Sample{CPUs: []model.CPUTimes{{TotalMs: 1}}}, true)

	m.receive(model.Sample{}, false)
	snapshot, err := m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if snapshot.Mode != model.ModeAPI || snapshot.Notice != noticeSamplerStopped {
		t.Errorf("mode %q notice %q, want api mode with the sampler-stopped notice", snapshot.Mode, snapshot.Notice)
	}
	if snapshot.Host.Memory != nil || snapshot.Host.Tasks != nil {
		t.Errorf("host = %+v, want unknown in api mode", snapshot.Host)
	}
	if processes := containerByID(t, snapshot, appID).Processes; len(processes) != 1 || processes[0].User != "root" {
		t.Errorf("processes = %+v, want the top listing", processes)
	}
}
