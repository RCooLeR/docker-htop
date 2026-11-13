package sampler

import (
	"bytes"
	"fmt"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// maxCPUs bounds the core index taken from /proc/stat, the kernel's largest NR_CPUS.
const maxCPUs = 8192

// parseCPUs reads the cpuN lines of /proc/stat into a slice indexed by N. Cores missing from the
// file (offline) stay zero.
func parseCPUs(data []byte) ([]model.CPUTimes, error) {
	var cpus []model.CPUTimes
	for line := range bytes.Lines(data) {
		rest, ok := bytes.CutPrefix(line, []byte("cpu"))
		if !ok || len(rest) == 0 || rest[0] < '0' || rest[0] > '9' {
			continue
		}
		index, times, err := parseCPULine(rest)
		if err != nil {
			return nil, err
		}
		if index >= len(cpus) {
			cpus = append(cpus, make([]model.CPUTimes, index+1-len(cpus))...)
		}
		cpus[index] = times
	}
	if len(cpus) == 0 {
		return nil, fmt.Errorf("parse cpu lines: none found: %w", ErrMalformed)
	}
	return cpus, nil
}

// parseCPULine parses "N user nice system idle iowait irq softirq steal ...", the part of a cpuN
// line after "cpu". Guest time is already counted in user and is not added again.
func parseCPULine(b []byte) (int, model.CPUTimes, error) {
	field, rest := nextField(b)
	index, err := parseUint(field)
	if err != nil {
		return 0, model.CPUTimes{}, fmt.Errorf("parse cpu index: %w", err)
	}
	if index >= maxCPUs {
		return 0, model.CPUTimes{}, fmt.Errorf("parse cpu index %d: %w", index, ErrMalformed)
	}
	var ticks [8]int64
	for i := range ticks {
		field, rest = nextField(rest)
		if ticks[i], err = parseUint(field); err != nil {
			return 0, model.CPUTimes{}, fmt.Errorf("parse cpu%d column %d: %w", index, i+1, err)
		}
	}
	var total int64
	for _, t := range ticks {
		total += t
	}
	return int(index), model.CPUTimes{
		TotalMs: total * msPerTick,
		IdleMs:  (ticks[3] + ticks[4]) * msPerTick,
	}, nil
}
