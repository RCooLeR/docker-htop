package monitor

import (
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestAPIProcessUser(t *testing.T) {
	m := newTestMonitor(newFakeEngine(), &fakeClock{t: testStart}, nil, "")
	m.names.known["db"] = map[uint32]string{999: "postgres"}

	tests := []struct {
		name string
		user string
		want string
	}{
		{name: "uid with a name in the container", user: "999", want: "postgres"},
		{name: "uid without a name", user: "10001", want: "10001"},
		{name: "name printed by ps is kept", user: "root", want: "root"},
		{name: "empty is kept", user: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.topUser("db", tc.user); got != tc.want {
				t.Errorf("topUser(%q) = %q, want %q", tc.user, got, tc.want)
			}
		})
	}
}

func TestAPIProcessCPUWindow(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{running("app", nil)}
	clock := &fakeClock{t: testStart}
	m := newTestMonitor(engine, clock, nil, "")

	steps := []struct {
		name    string
		cpu     time.Duration
		threads int
		want    *float64
	}{
		{name: "first reading", cpu: 0},
		{name: "one second", cpu: time.Second},
		{name: "two seconds", cpu: 2 * time.Second},
		{name: "three seconds against the first", cpu: 3 * time.Second, threads: 2, want: new(100.0)},
		{name: "window slides", cpu: 3 * time.Second, want: new(200.0 / 3)},
		{name: "counter reset starts over", cpu: time.Second},
	}
	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			if i > 0 {
				clock.advance(time.Second)
			}
			engine.set(func(f *fakeEngine) {
				f.tops["app"] = []model.TopProcess{{PID: 42, CPUTime: step.cpu, Threads: step.threads, RSSBytes: 1024}}
			})
			snapshot, err := m.tick(t.Context())
			if err != nil {
				t.Fatalf("tick: %v", err)
			}
			p := containerByID(t, snapshot, "app").Processes[0]
			assertFloat(t, "CPUPercent", p.CPUPercent, step.want)
			if (step.threads == 0) != (p.Threads == nil) || *p.MemoryBytes != 1024 {
				t.Errorf("threads %v memory %v, want threads nil only when 0 and memory from RSS", p.Threads, *p.MemoryBytes)
			}
		})
	}
}
