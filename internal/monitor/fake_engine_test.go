package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

var errNoSuchContainer = errors.New("no such container")

// fakeEngine is an Engine whose answers the test sets; it is safe for concurrent use.
type fakeEngine struct {
	mu            sync.Mutex // guards every field below
	info          model.EngineInfo
	infoErr       error
	containers    []model.ContainerInfo
	containersErr error
	stats         map[string]model.ContainerStats
	tops          map[string][]model.TopProcess
	names         map[string]map[uint32]string
	nameCalls     map[string]int
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		info:      model.EngineInfo{Name: "docker-desktop", CPUs: 4, MemoryBytes: 8 << 30},
		stats:     make(map[string]model.ContainerStats),
		tops:      make(map[string][]model.TopProcess),
		names:     make(map[string]map[uint32]string),
		nameCalls: make(map[string]int),
	}
}

func (f *fakeEngine) Info(context.Context) (model.EngineInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.info, f.infoErr
}

func (f *fakeEngine) Containers(context.Context) ([]model.ContainerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.containers, f.containersErr
}

func (f *fakeEngine) Stats(_ context.Context, containerID string) (model.ContainerStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stats, ok := f.stats[containerID]
	if !ok {
		return model.ContainerStats{}, errNoSuchContainer
	}
	return stats, nil
}

func (f *fakeEngine) Top(_ context.Context, containerID string) ([]model.TopProcess, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows, ok := f.tops[containerID]
	if !ok {
		return nil, errNoSuchContainer
	}
	return rows, nil
}

func (f *fakeEngine) UserNames(_ context.Context, containerID string) (map[uint32]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nameCalls[containerID]++
	names, ok := f.names[containerID]
	if !ok {
		return nil, errNoSuchContainer
	}
	return names, nil
}

func (f *fakeEngine) set(update func(f *fakeEngine)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	update(f)
}

func (f *fakeEngine) nameCallCount(containerID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nameCalls[containerID]
}

// fakeClock is a monitor clock the test moves by hand.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

var testStart = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// newTestMonitor returns a monitor whose engine info is already read, as Run would do.
func newTestMonitor(engine *fakeEngine, clock *fakeClock, samples <-chan model.Sample, notice string) *Monitor {
	m := New(engine, Options{Interval: time.Second, Now: clock.now, Samples: samples, Notice: notice})
	m.info = engine.info
	return m
}

func running(id string, labels map[string]string) model.ContainerInfo {
	return model.ContainerInfo{ID: id, Name: id, State: model.StateRunning, Labels: labels}
}

func containerByID(t testing.TB, s model.Snapshot, id string) model.Container {
	t.Helper()
	for _, c := range s.Containers {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("container %q missing from snapshot", id)
	return model.Container{}
}
