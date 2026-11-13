package ui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// sortKey orders containers inside a project and processes inside a container.
type sortKey int

const (
	sortCPU sortKey = iota
	sortMemory
	sortName
	sortTasks
)

func (k sortKey) String() string {
	switch k {
	case sortMemory:
		return "MEM"
	case sortName:
		return "Name"
	case sortTasks:
		return "Tasks"
	default:
		return "CPU%"
	}
}

// next cycles CPU, memory, name, tasks.
func (k sortKey) next() sortKey {
	return (k + 1) % 4
}

// compareDesc orders larger values first and nil values last; it reports 0 for a tie so the
// caller can fall back to a stable secondary key.
func compareDesc[T cmp.Ordered](a, b *T) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	default:
		return cmp.Compare(*b, *a)
	}
}

func sortContainers(cs []*model.Container, key sortKey) {
	slices.SortStableFunc(cs, func(a, b *model.Container) int {
		var c int
		switch key {
		case sortCPU:
			c = compareDesc(a.CPUPercent, b.CPUPercent)
		case sortMemory:
			c = compareDesc(a.MemoryBytes, b.MemoryBytes)
		case sortTasks:
			c = compareDesc(a.PIDs, b.PIDs)
		}
		return cmp.Or(c,
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID, b.ID))
	})
}

func sortProcesses(ps []*model.Process, key sortKey) {
	slices.SortStableFunc(ps, func(a, b *model.Process) int {
		var c int
		switch key {
		case sortCPU:
			c = compareDesc(a.CPUPercent, b.CPUPercent)
		case sortMemory:
			c = compareDesc(a.MemoryBytes, b.MemoryBytes)
		case sortTasks:
			c = compareDesc(a.Threads, b.Threads)
		case sortName:
			c = cmp.Compare(strings.ToLower(a.Command), strings.ToLower(b.Command))
		}
		return cmp.Or(c, cmp.Compare(a.PID, b.PID))
	})
}
