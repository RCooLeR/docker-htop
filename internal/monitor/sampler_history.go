package monitor

import (
	"slices"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// samplerHistory keeps the latest sampler reading and the one before it.
type samplerHistory struct {
	latest   *model.Sample
	previous *model.Sample
}

// processKey identifies a process across samples; PID alone could be a reused one.
type processKey struct {
	pid     int
	startMs int64
}

func (h *samplerHistory) add(sample model.Sample) {
	h.previous = h.latest
	h.latest = &sample
}

// host is entirely unknown until a sample has arrived.
func (h *samplerHistory) host() model.Host {
	if h.latest == nil {
		return model.Host{}
	}
	return model.Host{
		CoreBusy: h.coreBusy(),
		Memory:   hostMemory(h.latest.Memory),
		Load:     slices.Clone(h.latest.Load),
		Tasks:    new(h.latest.Tasks),
		Threads:  new(h.latest.Threads),
	}
}

// coreBusy is nil until two samples exist, when the core count changed, or when any core's
// total counter did not advance (the counters are not comparable then).
func (h *samplerHistory) coreBusy() []float64 {
	if h.latest == nil || h.previous == nil || len(h.latest.CPUs) != len(h.previous.CPUs) {
		return nil
	}
	busy := make([]float64, len(h.latest.CPUs))
	for i, cur := range h.latest.CPUs {
		prev := h.previous.CPUs[i]
		total := cur.TotalMs - prev.TotalMs
		if total <= 0 {
			return nil
		}
		idle := cur.IdleMs - prev.IdleMs
		busy[i] = min(max(1-float64(idle)/float64(total), 0), 1)
	}
	return busy
}

func hostMemory(m model.SampleMemory) *model.HostMemory {
	cache := m.BuffersBytes + m.CachedBytes
	return &model.HostMemory{
		TotalBytes:     m.TotalBytes,
		UsedBytes:      max(m.TotalBytes-m.FreeBytes-cache, 0),
		CacheBytes:     cache,
		SwapTotalBytes: m.SwapTotalBytes,
		SwapUsedBytes:  max(m.SwapTotalBytes-m.SwapFreeBytes, 0),
	}
}

// processes groups the latest sample's processes by container ID, keeping only containers in
// live. userName resolves a UID within a container.
func (h *samplerHistory) processes(live map[string]bool,
	userName func(containerID string, uid uint32) string) map[string][]model.Process {
	if h.latest == nil {
		return nil
	}
	previousCPU := h.previousCPU()
	elapsedMs := int64(0)
	if h.previous != nil {
		elapsedMs = h.latest.TimeUnixMs - h.previous.TimeUnixMs
	}

	byContainer := make(map[string][]model.Process)
	for _, p := range h.latest.Processes {
		if !live[p.ContainerID] {
			continue
		}
		process := model.Process{
			PID:         p.PID,
			PPID:        p.PPID,
			User:        userName(p.ContainerID, p.UID),
			State:       p.State,
			Threads:     new(p.Threads),
			MemoryBytes: new(p.RSSBytes),
			Command:     p.Command,
		}
		if prev, ok := previousCPU[processKey{p.PID, p.StartMs}]; ok && elapsedMs > 0 && p.CPUTimeMs >= prev {
			process.CPUPercent = new(float64(p.CPUTimeMs-prev) / float64(elapsedMs) * 100)
		}
		byContainer[p.ContainerID] = append(byContainer[p.ContainerID], process)
	}
	return byContainer
}

func (h *samplerHistory) previousCPU() map[processKey]int64 {
	if h.previous == nil {
		return nil
	}
	cpu := make(map[processKey]int64, len(h.previous.Processes))
	for _, p := range h.previous.Processes {
		cpu[processKey{p.PID, p.StartMs}] = p.CPUTimeMs
	}
	return cpu
}
