package monitor

import (
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// statsReading is the previous stats reading of a container and when it was taken.
type statsReading struct {
	at    time.Time
	stats model.ContainerStats
}

// applyStats fills the readings of c from cur and derives rates against the previous reading of
// the same container, which it then replaces.
func (m *Monitor) applyStats(c *model.Container, now time.Time, cur model.ContainerStats) {
	at := cur.Read
	if at.IsZero() {
		at = now
	}
	c.MemoryBytes = new(int64(cur.MemoryBytes))
	c.MemoryLimitBytes = memoryLimit(cur.MemoryLimitBytes, m.info.MemoryBytes)
	c.PIDs = new(cur.PIDs)

	prev, ok := m.stats[c.ID]
	m.stats[c.ID] = statsReading{at: at, stats: cur}
	if !ok {
		return
	}
	seconds := at.Sub(prev.at).Seconds()
	if seconds <= 0 {
		return
	}
	c.CPUPercent = cpuPercent(prev.stats.CPUTotal, cur.CPUTotal, seconds)
	c.BlockReadBytesPerSec = counterRate(prev.stats.BlockReadBytes, cur.BlockReadBytes, seconds)
	c.BlockWriteBytesPerSec = counterRate(prev.stats.BlockWriteBytes, cur.BlockWriteBytes, seconds)
	if prev.stats.HasNetwork && cur.HasNetwork {
		c.NetRxBytesPerSec = counterRate(prev.stats.NetRxBytes, cur.NetRxBytes, seconds)
		c.NetTxBytesPerSec = counterRate(prev.stats.NetTxBytes, cur.NetTxBytes, seconds)
	}
}

// cpuPercent is nil when the counter went backwards (the container restarted).
func cpuPercent(prev, cur time.Duration, seconds float64) *float64 {
	if cur < prev {
		return nil
	}
	return new((cur - prev).Seconds() / seconds * 100)
}

// counterRate is nil when the counter went backwards (the container restarted).
func counterRate(prev, cur uint64, seconds float64) *float64 {
	if cur < prev {
		return nil
	}
	return new(float64(cur-prev) / seconds)
}

// memoryLimit is nil for no limit: the engine reports the host's memory (or more) then.
func memoryLimit(limitBytes uint64, hostBytes int64) *int64 {
	if limitBytes == 0 || limitBytes >= uint64(max(hostBytes, 0)) {
		return nil
	}
	return new(int64(limitBytes))
}
