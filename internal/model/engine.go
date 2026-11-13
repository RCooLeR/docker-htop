package model

import "time"

// Labels and values the plugin puts on the containers and images it creates, so they can be
// told apart from the user's containers and hidden from the list.
const (
	// LabelRole marks a helper container or image; its value is one of the Role constants.
	LabelRole = "io.github.rcooler.docker-htop.role"
	// RoleSampler is the long-running, capability-less /proc reader.
	RoleSampler = "sampler"
	// RoleSignal is the one-shot helper that sends a signal to one container process.
	RoleSignal = "signal"
)

// LabelComposeProject is the label Docker Compose sets on the containers of a project.
const LabelComposeProject = "com.docker.compose.project"

// ContainerInfo is the engine's listing entry for one container.
type ContainerInfo struct {
	ID      string
	Name    string // without the leading slash
	Image   string
	State   ContainerState
	Status  string
	Labels  map[string]string
	Created time.Time
}

// ContainerStats is one reading of a container's cumulative counters. Rates are derived by the
// monitor from two readings.
type ContainerStats struct {
	// Read is the engine's clock at the time of the reading.
	Read time.Time
	// CPUTotal is the CPU time the container has used since it started.
	CPUTotal time.Duration
	// MemoryBytes is the usage without inactive file cache, as docker stats shows it.
	MemoryBytes uint64
	// MemoryLimitBytes is the cgroup limit. The engine reports the host's memory when the
	// container has no limit.
	MemoryLimitBytes uint64

	// HasNetwork is false when the engine reports no interfaces (host or none networking).
	HasNetwork bool
	NetRxBytes uint64 // summed over the container's interfaces
	NetTxBytes uint64

	BlockReadBytes  uint64
	BlockWriteBytes uint64

	PIDs int
}

// TopProcess is one row of the engine's top listing (ps on the Docker host), the process source
// in ModeAPI.
type TopProcess struct {
	PID      int
	PPID     int
	User     string        // the numeric UID as text; a name only where ps printed one instead
	Threads  int           // 0 when ps did not report it
	RSSBytes int64         // resident set size
	CPUTime  time.Duration // cumulative, whole seconds
	State    string        // first letter of the ps state column
	Command  string
}
