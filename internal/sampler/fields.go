package sampler

import (
	"fmt"
	"math"
)

// msPerTick converts clock ticks to milliseconds. USER_HZ is 100 on every Linux architecture
// Docker runs on, whatever the kernel's internal HZ.
const msPerTick = 10

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// nextField returns the first whitespace-separated field of b and what follows it. field is nil
// when b holds no more fields.
func nextField(b []byte) (field, rest []byte) {
	i := 0
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	if i == len(b) {
		return nil, nil
	}
	j := i
	for j < len(b) && !isSpace(b[j]) {
		j++
	}
	return b[i:j], b[j:]
}

// parseUint parses an unsigned decimal number without allocating.
func parseUint(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, fmt.Errorf("parse number: empty: %w", ErrMalformed)
	}
	var n int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("parse number %q: %w", b, ErrMalformed)
		}
		d := int64(c - '0')
		if n > (math.MaxInt64-d)/10 {
			return 0, fmt.Errorf("parse number %q: overflow: %w", b, ErrMalformed)
		}
		n = n*10 + d
	}
	return n, nil
}
