package sampler

import (
	"errors"
	"io"
	"os"
	"slices"
)

// readUpTo reads at most n bytes of path into buf, reusing its capacity, and reports whether the
// file had more. procfs files report size 0, so the file is read until EOF instead of by size.
func readUpTo(path string, buf []byte, n int) (data []byte, truncated bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return buf[:0], false, err
	}
	defer func() {
		_ = f.Close() // opened read-only: a close error cannot lose data
	}()

	buf = buf[:0]
	for len(buf) <= n {
		if len(buf) == cap(buf) {
			buf = slices.Grow(buf, min(max(cap(buf), 512), n+1-len(buf)))
		}
		m, err := f.Read(buf[len(buf):min(cap(buf), n+1)])
		buf = buf[:len(buf)+m]
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return buf[:0], false, err
		}
	}
	if len(buf) > n {
		return buf[:n], true, nil
	}
	return buf, false, nil
}
