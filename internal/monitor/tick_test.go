package monitor

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestTickFiltersAndStates(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{
		running("web", map[string]string{model.LabelComposeProject: "shop"}),
		running("sampler", map[string]string{model.LabelRole: model.RoleSampler}),
		{ID: "old", Name: "old", State: model.StateExited},
	}
	engine.stats["web"] = model.ContainerStats{Read: testStart, MemoryBytes: 1, PIDs: 1}
	engine.stats["old"] = model.ContainerStats{Read: testStart, MemoryBytes: 1, PIDs: 1}
	engine.tops["web"] = []model.TopProcess{{PID: 1}}
	engine.tops["old"] = []model.TopProcess{{PID: 2}}
	m := newTestMonitor(engine, &fakeClock{t: testStart}, nil, "no sampler image")

	snapshot, err := m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(snapshot.Containers) != 2 {
		t.Fatalf("got %d containers, want 2 (helper hidden)", len(snapshot.Containers))
	}
	if snapshot.Mode != model.ModeAPI || snapshot.Notice != "no sampler image" {
		t.Errorf("mode %q notice %q, want api mode with the options notice", snapshot.Mode, snapshot.Notice)
	}
	if !snapshot.Time.Equal(testStart) || snapshot.Engine.Name != "docker-desktop" {
		t.Errorf("time %v engine %+v, want monitor clock and kept info", snapshot.Time, snapshot.Engine)
	}
	web := containerByID(t, snapshot, "web")
	if web.Project != "shop" || web.MemoryBytes == nil || len(web.Processes) != 1 {
		t.Errorf("web = %+v, want project, memory and one process", web)
	}
	old := containerByID(t, snapshot, "old")
	if old.MemoryBytes != nil || old.PIDs != nil || old.Processes != nil {
		t.Errorf("exited container has readings: %+v", old)
	}
}

func TestTickVanishedContainerHasNoReadings(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{running("gone", nil)}
	m := newTestMonitor(engine, &fakeClock{t: testStart}, nil, "")

	snapshot, err := m.tick(t.Context())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if c := containerByID(t, snapshot, "gone"); c.MemoryBytes != nil || c.CPUPercent != nil {
		t.Errorf("vanished container has readings: %+v", c)
	}
	if snapshot.Notice != "" {
		t.Errorf("notice = %q, want a per-container error not reported", snapshot.Notice)
	}
}

func TestTickContainersFailure(t *testing.T) {
	engine := newFakeEngine()
	engine.containers = []model.ContainerInfo{running("app", nil)}
	clock := &fakeClock{t: testStart}
	m := newTestMonitor(engine, clock, nil, "")
	if _, err := m.tick(t.Context()); err != nil {
		t.Fatalf("first tick: %v", err)
	}

	failure := errors.New("connection refused")
	engine.set(func(f *fakeEngine) { f.containers, f.containersErr = nil, failure })
	clock.advance(time.Second)
	snapshot, err := m.tick(t.Context())
	if !errors.Is(err, failure) {
		t.Fatalf("tick error = %v, want %v", err, failure)
	}
	if len(snapshot.Containers) != 1 || snapshot.Containers[0].ID != "app" {
		t.Errorf("containers = %+v, want the previous ones", snapshot.Containers)
	}
	if !strings.Contains(snapshot.Notice, "connection refused") || !snapshot.Time.Equal(clock.t) {
		t.Errorf("notice %q time %v, want the error and the new time", snapshot.Notice, snapshot.Time)
	}

	engine.set(func(f *fakeEngine) { f.containers, f.containersErr = []model.ContainerInfo{running("app", nil)}, nil })
	clock.advance(time.Second)
	snapshot, err = m.tick(t.Context())
	if err != nil || snapshot.Notice != "" {
		t.Errorf("after recovery: err %v notice %q, want both cleared", err, snapshot.Notice)
	}
}
