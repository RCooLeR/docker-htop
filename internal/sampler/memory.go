package sampler

import (
	"bytes"
	"fmt"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const (
	memTotal = 1 << iota
	memFree
	memAvailable
	memBuffers
	memCached
	memSwapTotal
	memSwapFree

	memRequired = memTotal | memFree | memAvailable | memBuffers | memCached | memSwapTotal | memSwapFree
)

// parseMeminfo reads /proc/meminfo. Every field of model.SampleMemory is required except
// SReclaimable, which only adds to CachedBytes.
func parseMeminfo(data []byte) (model.SampleMemory, error) {
	var mem model.SampleMemory
	found := 0
	for line := range bytes.Lines(data) {
		key, rest, ok := bytes.Cut(line, []byte(":"))
		if !ok {
			continue
		}
		var target *int64
		var bit int
		switch string(key) {
		case "MemTotal":
			target, bit = &mem.TotalBytes, memTotal
		case "MemFree":
			target, bit = &mem.FreeBytes, memFree
		case "MemAvailable":
			target, bit = &mem.AvailableBytes, memAvailable
		case "Buffers":
			target, bit = &mem.BuffersBytes, memBuffers
		case "Cached", "SReclaimable":
			target, bit = &mem.CachedBytes, memCached
		case "SwapTotal":
			target, bit = &mem.SwapTotalBytes, memSwapTotal
		case "SwapFree":
			target, bit = &mem.SwapFreeBytes, memSwapFree
		default:
			continue
		}
		bytesValue, err := parseKB(rest)
		if err != nil {
			return model.SampleMemory{}, fmt.Errorf("parse meminfo %s: %w", key, err)
		}
		*target += bytesValue
		if string(key) != "SReclaimable" {
			found |= bit
		}
	}
	if found != memRequired {
		return model.SampleMemory{}, fmt.Errorf("parse meminfo: missing fields: %w", ErrMalformed)
	}
	return mem, nil
}

// parseKB parses " 12345 kB" into bytes.
func parseKB(b []byte) (int64, error) {
	field, rest := nextField(b)
	kb, err := parseUint(field)
	if err != nil {
		return 0, err
	}
	if unit, _ := nextField(rest); unit != nil && string(unit) != "kB" {
		return 0, fmt.Errorf("unit %q: %w", unit, ErrMalformed)
	}
	if kb > 1<<53 {
		return 0, fmt.Errorf("value %d kB: %w", kb, ErrMalformed)
	}
	return kb * 1024, nil
}
