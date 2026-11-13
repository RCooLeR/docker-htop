package sampler

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// DefaultProcRoot is where procfs is mounted.
const DefaultProcRoot = "/proc"

// Size limits for procfs files. Files with a limit are rejected when larger; files read by
// prefix are cut, since only their start is used.
const (
	hostStatLimit    = 1 << 20
	meminfoLimit     = 64 << 10
	loadavgLimit     = 4 << 10
	procStatLimit    = 4 << 10
	cgroupLimit      = 64 << 10
	statusPrefixLen  = 4 << 10
	cmdlinePrefixLen = 4 << 10
)

// Reader produces model.Sample readings from a procfs tree. It reuses buffers between reads, so
// it is not safe for concurrent use.
type Reader struct {
	procRoot string
	pageSize int64
	now      func() time.Time

	buf  []byte // file contents, valid until the next file read
	name []byte // the current process's name
	cmd  []byte // the current process's command line while it is built
}

// NewReader returns a Reader for the procfs tree at procRoot, usually DefaultProcRoot. Resident
// memory uses the page size of the running system.
func NewReader(procRoot string) *Reader {
	return &Reader{
		procRoot: procRoot,
		pageSize: int64(os.Getpagesize()),
		now:      time.Now,
		buf:      make([]byte, 0, 4096),
	}
}

// Read takes one reading. It fails when /proc/stat, /proc/meminfo, /proc/loadavg or the proc
// root listing cannot be read or parsed. A process whose files vanish or cannot be read is
// left out of Tasks, Threads and Processes. Processes is sorted by PID and never nil.
func (r *Reader) Read() (model.Sample, error) {
	s := model.Sample{TimeUnixMs: r.now().UnixMilli()}

	data, err := r.read(r.procRoot+"/stat", hostStatLimit)
	if err != nil {
		return model.Sample{}, err
	}
	if s.CPUs, err = parseCPUs(data); err != nil {
		return model.Sample{}, fmt.Errorf("read %s/stat: %w", r.procRoot, err)
	}

	if data, err = r.read(r.procRoot+"/meminfo", meminfoLimit); err != nil {
		return model.Sample{}, err
	}
	if s.Memory, err = parseMeminfo(data); err != nil {
		return model.Sample{}, fmt.Errorf("read %s/meminfo: %w", r.procRoot, err)
	}

	if data, err = r.read(r.procRoot+"/loadavg", loadavgLimit); err != nil {
		return model.Sample{}, err
	}
	if s.Load, err = parseLoadavg(data); err != nil {
		return model.Sample{}, fmt.Errorf("read %s/loadavg: %w", r.procRoot, err)
	}

	if err := r.readProcesses(&s); err != nil {
		return model.Sample{}, err
	}
	return s, nil
}

// readProcesses fills Tasks, Threads and Processes from every numeric directory of the proc root.
func (r *Reader) readProcesses(s *model.Sample) error {
	names, err := listDir(r.procRoot)
	if err != nil {
		return err
	}
	ids := make(map[string]string)
	s.Processes = make([]model.ProcessSample, 0, 64)
	for _, name := range names {
		pid, ok := parsePID(name)
		if !ok {
			continue
		}
		proc, threads, inContainer, err := r.readProcess(r.procRoot+"/"+name, pid, ids)
		if err != nil {
			// The process exited after the listing (ENOENT, or ESRCH from a read of an open
			// file) or is unreadable: leave it out rather than report zeros.
			continue
		}
		s.Tasks++
		s.Threads += threads
		if inContainer {
			s.Processes = append(s.Processes, proc)
		}
	}
	slices.SortFunc(s.Processes, func(a, b model.ProcessSample) int { return cmp.Compare(a.PID, b.PID) })
	return nil
}

func listDir(path string) ([]string, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = dir.Close() // opened read-only: a close error cannot lose data
	}()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", path, err)
	}
	return names, nil
}

func parsePID(name string) (int, bool) {
	if name == "" || name[0] < '0' || name[0] > '9' {
		return 0, false
	}
	pid, err := strconv.Atoi(name)
	return pid, err == nil && pid > 0
}

// read returns the whole file, failing with ErrFileTooLarge past limit.
func (r *Reader) read(path string, limit int) ([]byte, error) {
	data, truncated, err := readUpTo(path, r.buf, limit)
	r.buf = data[:0]
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if truncated {
		return nil, fmt.Errorf("read %s: over %d bytes: %w", path, limit, ErrFileTooLarge)
	}
	return data, nil
}

// readPrefix returns at most the first n bytes of the file.
func (r *Reader) readPrefix(path string, n int) ([]byte, error) {
	data, _, err := readUpTo(path, r.buf, n)
	r.buf = data[:0]
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}
