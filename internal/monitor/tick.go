package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// tickReadings is everything one tick read from the engine, the input of build.
type tickReadings struct {
	now        time.Time
	containers []model.ContainerInfo // helper containers already removed
	stats      map[string]model.ContainerStats
	tops       map[string][]model.TopProcess // API mode only
}

// tick reads the engine once and builds the snapshot. When the container listing fails it
// returns the previous containers with the error as notice, together with that error.
func (m *Monitor) tick(ctx context.Context) (model.Snapshot, error) {
	now := m.now()
	list, err := m.engine.Containers(ctx)
	if err != nil {
		err = fmt.Errorf("list containers: %w", err)
		return m.failed(now, err), err
	}
	return m.build(m.fetch(ctx, now, visibleContainers(list))), nil
}

func (m *Monitor) failed(now time.Time, err error) model.Snapshot {
	snapshot := m.last
	snapshot.Time = now
	snapshot.Mode = m.mode()
	snapshot.Engine = m.info
	snapshot.Notice = err.Error()
	m.last = snapshot
	return snapshot
}

func (m *Monitor) fetch(ctx context.Context, now time.Time, containers []model.ContainerInfo) tickReadings {
	var live, running []string
	for _, c := range containers {
		if isLive(c.State) {
			live = append(live, c.ID)
		}
		if c.State == model.StateRunning {
			running = append(running, c.ID)
		}
	}
	readings := tickReadings{now: now, containers: containers}
	readings.stats = fetchAll(ctx, live, m.callTimeout, m.engine.Stats)
	if m.mode() == model.ModeAPI {
		readings.tops = fetchAll(ctx, running, m.callTimeout, m.engine.Top)
	}
	return readings
}

// build turns one tick's readings into a snapshot and updates the per-container history. It does
// no I/O.
func (m *Monitor) build(r tickReadings) model.Snapshot {
	live := make(map[string]bool)
	for _, c := range r.containers {
		if isLive(c.State) {
			live[c.ID] = true
		}
	}

	snapshot := model.Snapshot{Time: r.now, Mode: m.mode(), Engine: m.info}
	var processes map[string][]model.Process
	if snapshot.Mode != model.ModeAPI {
		snapshot.Host = m.sampler.host()
		processes = m.sampler.processes(live, m.names.name)
	} else {
		snapshot.Notice = m.apiNotice
		processes = m.topProcesses(r.now, r.tops)
	}

	snapshot.Containers = make([]model.Container, 0, len(r.containers))
	for _, info := range r.containers {
		snapshot.Containers = append(snapshot.Containers, m.container(r, info, processes[info.ID]))
	}
	m.forget(live)
	m.last = snapshot
	return snapshot
}

func (m *Monitor) container(r tickReadings, info model.ContainerInfo, processes []model.Process) model.Container {
	c := model.Container{
		ID:      info.ID,
		Name:    info.Name,
		Image:   info.Image,
		Project: info.Labels[model.LabelComposeProject],
		State:   info.State,
		Status:  info.Status,
	}
	if !isLive(info.State) {
		return c
	}
	c.Processes = processes
	if stats, ok := r.stats[info.ID]; ok {
		m.applyStats(&c, r.now, stats)
	}
	return c
}

// forget drops history for containers that are gone or no longer running or paused.
func (m *Monitor) forget(live map[string]bool) {
	for id := range m.stats {
		if !live[id] {
			delete(m.stats, id)
		}
	}
	m.names.retain(live)
}

func visibleContainers(list []model.ContainerInfo) []model.ContainerInfo {
	visible := make([]model.ContainerInfo, 0, len(list))
	for _, c := range list {
		if _, helper := c.Labels[model.LabelRole]; !helper {
			visible = append(visible, c)
		}
	}
	return visible
}

func isLive(state model.ContainerState) bool {
	return state == model.StateRunning || state == model.StatePaused
}
