package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const otherID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestUserNameResolution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		engine := newFakeEngine()
		engine.containers = []model.ContainerInfo{running(appID, nil), running(otherID, nil)}
		engine.names[appID] = map[uint32]string{0: "root", 1000: "app"}
		samples := make(chan model.Sample)
		m := newTestMonitor(engine, &fakeClock{t: testStart}, samples, "")
		m.receive(model.Sample{TimeUnixMs: 1, Processes: []model.ProcessSample{
			{PID: 10, UID: 1000, ContainerID: appID},
			{PID: 11, UID: 7, ContainerID: appID},
			{PID: 20, UID: 1000, ContainerID: otherID},
		}}, true)

		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		wg.Go(func() { m.names.run(ctx) })
		defer wg.Wait()
		defer cancel()

		users := func() map[int]string {
			snapshot, err := m.tick(t.Context())
			if err != nil {
				t.Fatalf("tick: %v", err)
			}
			byPID := make(map[int]string)
			for _, c := range snapshot.Containers {
				for _, p := range c.Processes {
					byPID[p.PID] = p.User
				}
			}
			return byPID
		}

		if got := users(); got[10] != "1000" {
			t.Errorf("first tick user = %q, want the numeric UID while names load", got[10])
		}
		synctest.Wait()
		for range 2 {
			got := users()
			if got[10] != "app" || got[11] != "7" || got[20] != "1000" {
				t.Errorf("users = %v, want app, 7 (unknown UID) and 1000 (failed fetch)", got)
			}
			synctest.Wait()
		}
		if engine.nameCallCount(appID) != 1 || engine.nameCallCount(otherID) != 1 {
			t.Errorf("name calls %d and %d, want one per container including the failed one",
				engine.nameCallCount(appID), engine.nameCallCount(otherID))
		}
	})
}

func TestRunStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		engine := newFakeEngine()
		engine.containers = []model.ContainerInfo{running("app", nil)}
		m := New(engine, Options{Interval: time.Second})
		ctx, cancel := context.WithCancel(t.Context())

		var runErr error
		var wg sync.WaitGroup
		wg.Go(func() { runErr = m.Run(ctx) })

		first := <-m.Snapshots()
		if len(first.Containers) != 1 || first.Engine.Name != "docker-desktop" {
			t.Errorf("first snapshot = %+v, want the container and the engine info", first)
		}
		second := <-m.Snapshots()
		if got := second.Time.Sub(first.Time); got != time.Second {
			t.Errorf("snapshots %v apart, want one interval", got)
		}

		cancel()
		wg.Wait()
		if runErr != nil {
			t.Errorf("Run = %v, want nil after cancel", runErr)
		}
		for range m.Snapshots() {
		}
	})
}

func TestRunFailsWhenEngineUnreachable(t *testing.T) {
	failure := errors.New("engine unreachable")
	tests := []struct {
		name string
		fail func(f *fakeEngine)
	}{
		{name: "info", fail: func(f *fakeEngine) { f.infoErr = failure }},
		{name: "containers", fail: func(f *fakeEngine) { f.containersErr = failure }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				engine := newFakeEngine()
				engine.set(tt.fail)
				m := New(engine, Options{Interval: time.Second})
				if err := m.Run(t.Context()); !errors.Is(err, failure) {
					t.Errorf("Run = %v, want %v", err, failure)
				}
				if _, open := <-m.Snapshots(); open {
					t.Error("snapshot channel still open")
				}
			})
		})
	}
}

func TestPublishKeepsNewest(t *testing.T) {
	m := New(newFakeEngine(), Options{Interval: time.Second})
	for i := range 3 {
		m.publish(model.Snapshot{Time: testStart.Add(time.Duration(i) * time.Second)})
	}
	got := <-m.Snapshots()
	if want := testStart.Add(2 * time.Second); !got.Time.Equal(want) {
		t.Errorf("slow reader got %v, want the newest %v", got.Time, want)
	}
	select {
	case s := <-m.Snapshots():
		t.Errorf("unexpected second snapshot %v", s.Time)
	default:
	}
}
