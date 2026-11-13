package model

// The sampler wire format: the docker-htop-sampler binary runs inside a helper container that
// shares the Docker host's PID namespace and writes one Sample per interval to stdout as a single
// JSON line. Counters are cumulative; the monitor turns two samples into rates.
//
// Sampler command line (the Engine API client builds it, the sampler binary parses it):
//
//	docker-htop-sampler stream --interval <duration>
//	    Write samples until stdin reaches EOF or the process is signalled.
//	docker-htop-sampler signal --container <full container id> --pid <host pid> --signal <SIGNAME>
//	    Send one signal to a process after checking that it belongs to that container.

// SamplerBinaryPath is where the sampler binary sits inside its image.
const SamplerBinaryPath = "/docker-htop-sampler"

// Sampler subcommands.
const (
	SamplerCommandStream = "stream"
	SamplerCommandSignal = "signal"
)

// Sample is one reading of the Docker host's /proc.
type Sample struct {
	// TimeUnixMs is the sampler's wall clock at the reading, in Unix milliseconds.
	TimeUnixMs int64 `json:"time_unix_ms"`
	// CPUs is indexed by core number.
	CPUs   []CPUTimes   `json:"cpus"`
	Memory SampleMemory `json:"memory"`
	// Load is the 1, 5 and 15 minute load average.
	Load []float64 `json:"load"`
	// Tasks and Threads count every process on the host, in a container or not.
	Tasks   int `json:"tasks"`
	Threads int `json:"threads"`
	// Processes lists only processes that belong to a Docker container.
	Processes []ProcessSample `json:"processes"`
}

// CPUTimes holds the cumulative time counters of one core since boot.
type CPUTimes struct {
	TotalMs int64 `json:"total_ms"`
	// IdleMs includes time waiting for I/O.
	IdleMs int64 `json:"idle_ms"`
}

// SampleMemory holds the /proc/meminfo values the header needs.
type SampleMemory struct {
	TotalBytes     int64 `json:"total_bytes"`
	FreeBytes      int64 `json:"free_bytes"`
	AvailableBytes int64 `json:"available_bytes"`
	BuffersBytes   int64 `json:"buffers_bytes"`
	// CachedBytes is the page cache plus reclaimable slab, as htop counts cache.
	CachedBytes    int64 `json:"cached_bytes"`
	SwapTotalBytes int64 `json:"swap_total_bytes"`
	SwapFreeBytes  int64 `json:"swap_free_bytes"`
}

// ProcessSample is one container process in a Sample.
type ProcessSample struct {
	PID  int    `json:"pid"`
	PPID int    `json:"ppid"`
	UID  uint32 `json:"uid"`
	// State is the one-letter kernel state.
	State   string `json:"state"`
	Threads int    `json:"threads"`
	// StartMs is the process start time in milliseconds since boot. Together with PID it
	// identifies a process across samples, so a reused PID is not mistaken for the old process.
	StartMs int64 `json:"start_ms"`
	// CPUTimeMs is user plus system time since the process started.
	CPUTimeMs int64 `json:"cpu_time_ms"`
	RSSBytes  int64 `json:"rss_bytes"`
	// Command is the command line with arguments joined by spaces, capped in length, or the
	// process name in brackets when the command line is empty.
	Command string `json:"command"`
	// ContainerID is the full 64-character container ID taken from the process's cgroup path.
	ContainerID string `json:"container_id"`
}
