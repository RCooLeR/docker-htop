package ui

import (
	"slices"
	"strings"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// standaloneGroup is the display name of the group holding containers outside Compose.
const standaloneGroup = "standalone"

// rowKind tells the three levels of the tree apart.
type rowKind int

const (
	rowProject rowKind = iota
	rowContainer
	rowProcess
)

// rowKey identifies a row across refreshes: a project by name (empty for standalone), a
// container by ID, a process by container ID and PID.
type rowKey struct {
	kind        rowKind
	project     string
	containerID string
	pid         int
}

// row is one visible line of the tree.
type row struct {
	key       rowKey
	count     int // project rows: visible containers
	container *model.Container
	process   *model.Process
	user      string // container rows: user of the root process
	last      bool   // process rows: last process of its container
	expanded  bool   // project and container rows: children are shown
}

// treeState is everything besides the snapshot that decides which rows are visible.
type treeState struct {
	sort                sortKey
	showAll             bool
	filter              string
	collapsedProjects   map[string]bool
	collapsedContainers map[string]bool
}

// buildRows flattens the snapshot into the visible tree rows.
func buildRows(s model.Snapshot, st treeState) []row {
	groups := map[string][]*model.Container{}
	procs := map[string][]*model.Process{}
	for i := range s.Containers {
		c := &s.Containers[i]
		if !st.showAll && c.State != model.StateRunning && c.State != model.StatePaused {
			continue
		}
		visible, ok := filterContainer(c, st.filter)
		if !ok {
			continue
		}
		groups[c.Project] = append(groups[c.Project], c)
		procs[c.ID] = visible
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	slices.SortFunc(names, compareProjects)

	var rows []row
	for _, name := range names {
		cs := groups[name]
		sortContainers(cs, st.sort)
		expanded := !st.collapsedProjects[name]
		rows = append(rows, row{
			key:      rowKey{kind: rowProject, project: name},
			count:    len(cs),
			expanded: expanded,
		})
		if !expanded {
			continue
		}
		for _, c := range cs {
			rows = appendContainer(rows, c, procs[c.ID], st)
		}
	}
	return rows
}

// compareProjects sorts projects by name with the standalone group (empty name) last.
func compareProjects(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	default:
		return strings.Compare(a, b)
	}
}

func appendContainer(rows []row, c *model.Container, ps []*model.Process, st treeState) []row {
	expanded := !st.collapsedContainers[c.ID]
	rows = append(rows, row{
		key:       rowKey{kind: rowContainer, project: c.Project, containerID: c.ID},
		container: c,
		user:      rootUser(c.Processes),
		expanded:  expanded && len(ps) > 0,
	})
	if !expanded {
		return rows
	}
	sortProcesses(ps, st.sort)
	for i, p := range ps {
		rows = append(rows, row{
			key:       rowKey{kind: rowProcess, project: c.Project, containerID: c.ID, pid: p.PID},
			container: c,
			process:   p,
			last:      i == len(ps)-1,
		})
	}
	return rows
}

// filterContainer applies the filter: a matching container keeps all its processes, otherwise
// only matching processes are kept and the container stays only if one matched.
func filterContainer(c *model.Container, filter string) ([]*model.Process, bool) {
	all := make([]*model.Process, 0, len(c.Processes))
	for i := range c.Processes {
		all = append(all, &c.Processes[i])
	}
	q := strings.ToLower(filter)
	if q == "" || contains(c.Name, q) || contains(c.Image, q) {
		return all, true
	}
	matched := slices.DeleteFunc(all, func(p *model.Process) bool {
		return !contains(p.Command, q) && !contains(p.User, q)
	})
	return matched, len(matched) > 0
}

// rootUser is the user of the container's root process: the one whose parent is not a process
// of the same container, lowest PID first.
func rootUser(ps []model.Process) string {
	pids := make(map[int]bool, len(ps))
	for _, p := range ps {
		pids[p.PID] = true
	}
	user, best := unknown, -1
	for _, p := range ps {
		if pids[p.PPID] {
			continue
		}
		if best == -1 || p.PID < best {
			best, user = p.PID, p.User
		}
	}
	if user == "" {
		return unknown
	}
	return user
}

// rowMatches reports whether a search query (lower case) matches the row's visible text.
func rowMatches(r row, q string) bool {
	switch r.key.kind {
	case rowProject:
		return contains(projectName(r.key.project), q)
	case rowContainer:
		return contains(r.container.Name, q) || contains(r.container.Image, q)
	default:
		return contains(r.process.Command, q) || contains(r.process.User, q)
	}
}

func projectName(project string) string {
	if project == "" {
		return standaloneGroup
	}
	return project
}

// contains is a case-insensitive substring test; q must already be lower case.
func contains(s, q string) bool {
	return strings.Contains(strings.ToLower(s), q)
}
