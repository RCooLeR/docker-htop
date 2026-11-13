package monitor

import (
	"maps"
	"strconv"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// apiCPUWindow is the shortest span over which API-mode process CPU is computed: the engine's
// top counter has whole-second resolution, so a shorter span would mostly show rounding.
const apiCPUWindow = 3 * time.Second

// cpuReading is one cumulative CPU reading of a process at a monitor-clock time.
type cpuReading struct {
	at  time.Time
	cpu time.Duration
}

// topProcesses maps the engine's top listings to processes by container ID and updates the
// per-process CPU history, forgetting processes that were not listed.
func (m *Monitor) topProcesses(now time.Time, tops map[string][]model.TopProcess) map[string][]model.Process {
	seen := make(map[int]bool)
	byContainer := make(map[string][]model.Process, len(tops))
	for id, rows := range tops {
		processes := make([]model.Process, 0, len(rows))
		for _, row := range rows {
			seen[row.PID] = true
			process := topProcess(row, m.topCPU(now, row))
			process.User = m.topUser(id, row.User)
			processes = append(processes, process)
		}
		byContainer[id] = processes
	}
	maps.DeleteFunc(m.tops, func(pid int, _ []cpuReading) bool { return !seen[pid] })
	return byContainer
}

// topCPU records row's CPU counter and compares it with the newest reading at least
// apiCPUWindow old; nil while no such reading exists. A counter that went backwards means the
// PID was reused, and the history starts over.
func (m *Monitor) topCPU(now time.Time, row model.TopProcess) *float64 {
	history := m.tops[row.PID]
	if n := len(history); n > 0 && row.CPUTime < history[n-1].cpu {
		history = nil
	}
	history = append(history, cpuReading{at: now, cpu: row.CPUTime})
	for len(history) > 1 && now.Sub(history[1].at) >= apiCPUWindow {
		history = history[1:]
	}
	m.tops[row.PID] = history

	base := history[0]
	elapsed := now.Sub(base.at)
	if elapsed < apiCPUWindow {
		return nil
	}
	return new((row.CPUTime - base.cpu).Seconds() / elapsed.Seconds() * 100)
}

// topUser turns the UID that top reports into the name from the container's own passwd file,
// the same way sampler mode does. Anything that is not a UID is kept as it is.
func (m *Monitor) topUser(containerID, user string) string {
	uid, err := strconv.ParseUint(user, 10, 32)
	if err != nil {
		return user
	}
	return m.names.name(containerID, uint32(uid))
}

func topProcess(row model.TopProcess, cpuPercent *float64) model.Process {
	process := model.Process{
		PID:         row.PID,
		PPID:        row.PPID,
		User:        row.User,
		State:       row.State,
		CPUPercent:  cpuPercent,
		MemoryBytes: new(row.RSSBytes),
		Command:     row.Command,
	}
	if row.Threads > 0 {
		process.Threads = new(row.Threads)
	}
	return process
}
