package sampler

import (
	"bytes"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// maxCommandBytes caps ProcessSample.Command.
const maxCommandBytes = 512

// readProcess reads the process in dir. Processes outside any container are only counted, so
// for them only stat and cgroup are read and inContainer is false. ids interns container IDs so
// the processes of one container share one string.
func (r *Reader) readProcess(dir string, pid int, ids map[string]string) (
	proc model.ProcessSample, threads int, inContainer bool, err error,
) {
	data, err := r.read(dir+"/stat", procStatLimit)
	if err != nil {
		return proc, 0, false, err
	}
	st, err := parseProcStat(data)
	if err != nil {
		return proc, 0, false, fmt.Errorf("read %s/stat: %w", dir, err)
	}
	r.name = append(r.name[:0], st.name...)
	threads = int(st.threads)

	if data, err = r.read(dir+"/cgroup", cgroupLimit); err != nil {
		return proc, 0, false, err
	}
	id := firstContainerID(data)
	if id == nil {
		return proc, threads, false, nil
	}
	containerID, ok := ids[string(id)]
	if !ok {
		containerID = string(id)
		ids[containerID] = containerID
	}

	if data, err = r.readPrefix(dir+"/status", statusPrefixLen); err != nil {
		return proc, 0, false, err
	}
	uid, err := parseUID(data)
	if err != nil {
		return proc, 0, false, fmt.Errorf("read %s/status: %w", dir, err)
	}

	if data, err = r.readPrefix(dir+"/cmdline", cmdlinePrefixLen); err != nil {
		return proc, 0, false, err
	}
	return model.ProcessSample{
		PID:         pid,
		PPID:        int(st.ppid),
		UID:         uid,
		State:       string(st.state),
		Threads:     threads,
		StartMs:     st.startTicks * msPerTick,
		CPUTimeMs:   (st.utime + st.stime) * msPerTick,
		RSSBytes:    st.rssPages * r.pageSize,
		Command:     r.formatCommand(data, r.name),
		ContainerID: containerID,
	}, threads, true, nil
}

// parseUID returns the real UID, the first value of the Uid: line of /proc/<pid>/status.
func parseUID(data []byte) (uint32, error) {
	for line := range bytes.Lines(data) {
		rest, ok := bytes.CutPrefix(line, []byte("Uid:"))
		if !ok {
			continue
		}
		field, _ := nextField(rest)
		uid, err := parseUint(field)
		if err != nil {
			return 0, fmt.Errorf("parse uid: %w", err)
		}
		if uid > math.MaxUint32 {
			return 0, fmt.Errorf("parse uid %d: %w", uid, ErrMalformed)
		}
		return uint32(uid), nil
	}
	return 0, fmt.Errorf("parse uid: no Uid line: %w", ErrMalformed)
}

// formatCommand joins the NUL-separated arguments of a cmdline file with single spaces, trims
// it and caps it at maxCommandBytes without splitting a UTF-8 sequence. An empty command line
// (kernel threads, zombies) becomes the process name in brackets.
func (r *Reader) formatCommand(raw, name []byte) string {
	b := r.cmd[:0]
	for arg := range bytes.SplitSeq(raw, []byte{0}) {
		if len(arg) == 0 {
			continue
		}
		if len(b) > 0 {
			b = append(b, ' ')
		}
		b = append(b, arg...)
		if len(b) > maxCommandBytes {
			break
		}
	}
	r.cmd = b[:0]

	b = bytes.TrimSpace(b)
	if len(b) > maxCommandBytes {
		cut := maxCommandBytes
		for cut > 0 && !utf8.RuneStart(b[cut]) {
			cut--
		}
		b = bytes.TrimSpace(b[:cut])
	}
	if len(b) == 0 {
		return "[" + string(name) + "]"
	}
	return string(b)
}
