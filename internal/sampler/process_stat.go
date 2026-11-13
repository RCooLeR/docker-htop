package sampler

import (
	"bytes"
	"fmt"
)

// procStat holds the fields of /proc/<pid>/stat the sampler uses. Counters are in clock ticks,
// rss in pages.
type procStat struct {
	name       []byte // aliases the parsed line
	state      byte
	ppid       int64
	utime      int64
	stime      int64
	threads    int64
	startTicks int64
	rssPages   int64
}

// parseProcStat parses one /proc/<pid>/stat line. The name may contain spaces and parentheses, so
// it ends at the last ')' and fields are counted from there (state is field 3).
func parseProcStat(line []byte) (procStat, error) {
	open := bytes.IndexByte(line, '(')
	end := bytes.LastIndexByte(line, ')')
	if open < 0 || end < open {
		return procStat{}, fmt.Errorf("parse stat: no process name: %w", ErrMalformed)
	}
	st := procStat{name: line[open+1 : end]}
	rest := line[end+1:]
	for n := 3; n <= 24; n++ {
		var field []byte
		field, rest = nextField(rest)
		if field == nil {
			return procStat{}, fmt.Errorf("parse stat: field %d missing: %w", n, ErrMalformed)
		}
		var target *int64
		switch n {
		case 3:
			if len(field) != 1 {
				return procStat{}, fmt.Errorf("parse stat state %q: %w", field, ErrMalformed)
			}
			st.state = field[0]
			continue
		case 4:
			target = &st.ppid
		case 14:
			target = &st.utime
		case 15:
			target = &st.stime
		case 20:
			target = &st.threads
		case 22:
			target = &st.startTicks
		case 24:
			target = &st.rssPages
		default:
			continue
		}
		v, err := parseUint(field)
		if err != nil {
			return procStat{}, fmt.Errorf("parse stat field %d: %w", n, err)
		}
		*target = v
	}
	return st, nil
}
