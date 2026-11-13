// Package model holds the types shared by the Engine API client, the sampler, the monitor and
// the UI. It owns no behaviour beyond small helpers on those types.
//
// Optional readings are nil pointers or nil slices when unknown. Unknown is never reported as
// zero: a container that has no CPU reading yet has CPUPercent == nil, not 0.
package model

import "time"

// Mode says where the process and host data of a Snapshot comes from.
type Mode string

const (
	// ModeSampler means a host-PID sampler container reads /proc of the Docker host, which gives
	// exact per-process and per-core numbers.
	ModeSampler Mode = "sampler"
	// ModeLocal means the plugin reads /proc itself: it runs on the Docker host, in the PID
	// namespace of the containers, and needs no helper container. The data is the same as in
	// ModeSampler.
	ModeLocal Mode = "local"
	// ModeAPI means only the Engine API is used: process CPU comes from whole-second counters,
	// and per-core, host memory and load readings are unknown.
	ModeAPI Mode = "api"
)

// Snapshot is one consistent view of the engine, built by the monitor once per interval.
type Snapshot struct {
	Time time.Time
	Mode Mode
	// Notice says why the monitor is not in sampler mode, or carries the last non-fatal error.
	// Empty when there is nothing to report.
	Notice     string
	Engine     EngineInfo
	Host       Host
	Containers []Container
}

// EngineInfo describes the Docker engine the plugin is connected to.
type EngineInfo struct {
	Name         string // engine host name, "docker-desktop" on Docker Desktop
	Version      string // server version
	Architecture string // as the engine reports it: "x86_64", "aarch64"
	CPUs         int
	MemoryBytes  int64 // total memory of the Docker host (the VM on Docker Desktop)
}

// Host carries readings about the Docker host as a whole. Every field is unknown in ModeAPI.
type Host struct {
	// CoreBusy is the busy share of each core over the last interval, 0..1, indexed by core.
	CoreBusy []float64
	Memory   *HostMemory
	// Load is the 1, 5 and 15 minute load average; nil or three values.
	Load    []float64
	Tasks   *int // processes on the host, including those outside containers
	Threads *int
}

// HostMemory is the memory use of the Docker host, split the way htop draws it.
type HostMemory struct {
	TotalBytes     int64
	UsedBytes      int64 // total minus free, buffers and page cache
	CacheBytes     int64 // buffers plus page cache
	SwapTotalBytes int64
	SwapUsedBytes  int64
}

// ContainerState is the engine's container state.
type ContainerState string

// Container states as the Engine API reports them.
const (
	StateCreated    ContainerState = "created"
	StateRunning    ContainerState = "running"
	StatePaused     ContainerState = "paused"
	StateRestarting ContainerState = "restarting"
	StateRemoving   ContainerState = "removing"
	StateExited     ContainerState = "exited"
	StateDead       ContainerState = "dead"
)

// Container is one container with its latest readings and processes.
type Container struct {
	ID      string
	Name    string
	Image   string
	Project string // Compose project; empty for a container outside Compose
	State   ContainerState
	Status  string // the engine's own text, for example "Up 3 days (healthy)"

	// CPUPercent uses the docker stats scale: 100 is one fully used core. Nil until two readings
	// exist and for containers that are not running.
	CPUPercent *float64
	// MemoryBytes is the usage docker stats shows (without inactive file cache).
	MemoryBytes *int64
	// MemoryLimitBytes is nil when the container has no memory limit.
	MemoryLimitBytes *int64

	NetRxBytesPerSec      *float64
	NetTxBytesPerSec      *float64
	BlockReadBytesPerSec  *float64
	BlockWriteBytesPerSec *float64

	PIDs *int // processes and threads the engine counts in the container

	// Processes is unordered; the UI sorts it. Empty for containers that are not running.
	Processes []Process
}

// Process is one process of a container.
type Process struct {
	PID  int // PID on the Docker host, not inside the container's PID namespace
	PPID int
	// User is the name from the container's /etc/passwd, or the numeric UID when the name is
	// unknown.
	User    string
	State   string // one-letter kernel state (R, S, D, Z, T, I); empty when unknown
	Threads *int
	// CPUPercent uses the same scale as Container.CPUPercent. Nil until two readings exist.
	CPUPercent  *float64
	MemoryBytes *int64 // resident set size
	Command     string // full command line, or the bracketed process name for kernel-style entries
}
