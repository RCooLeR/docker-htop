package sampler

import (
	"fmt"
	"strconv"
)

// parseLoadavg reads the 1, 5 and 15 minute load averages from /proc/loadavg.
func parseLoadavg(data []byte) ([]float64, error) {
	load := make([]float64, 3)
	rest := data
	for i := range load {
		var field []byte
		field, rest = nextField(rest)
		v, err := strconv.ParseFloat(string(field), 64)
		if err != nil || v < 0 {
			return nil, fmt.Errorf("parse loadavg field %d %q: %w", i+1, field, ErrMalformed)
		}
		load[i] = v
	}
	return load, nil
}
