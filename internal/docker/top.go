package docker

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// topPSArgs asks ps on the Docker host for every column TopProcess needs, the full command line
// last because the engine puts the rest of each line into the last column. It asks for the
// numeric UID, not the user name: ps would take the name from the Docker host's passwd file,
// which belongs to a different system than the container's. Verified against Docker Desktop 29
// (procps): titles PID PPID UID NLWP RSS TIME STAT COMMAND.
const topPSArgs = "-eo pid,ppid,uid,nlwp,rss,cputimes,stat,args"

type topColumn int

const (
	topPID topColumn = iota
	topPPID
	topUser
	topThreads
	topRSS
	topCPUTime
	topState
	topCommand
	topColumnCount
)

// topColumnFor maps a ps column title, including those of equivalent format keywords, to a
// column.
func topColumnFor(title string) (topColumn, bool) {
	switch strings.ToUpper(strings.TrimSpace(title)) {
	case "PID":
		return topPID, true
	case "PPID":
		return topPPID, true
	case "USER", "UID", "RUSER":
		return topUser, true
	case "NLWP", "THCNT":
		return topThreads, true
	case "RSS", "RSZ":
		return topRSS, true
	case "TIME":
		return topCPUTime, true
	case "STAT", "S":
		return topState, true
	case "COMMAND", "CMD", "ARGS":
		return topCommand, true
	}
	return 0, false
}

type topResponse struct {
	Titles    []string   `json:"Titles"`
	Processes [][]string `json:"Processes"`
}

// Top lists the processes of a running container as ps on the Docker host sees them
// (GET /containers/{id}/top). The error matches ErrNotFound for an unknown container.
func (c *Client) Top(ctx context.Context, containerID string) ([]model.TopProcess, error) {
	var top topResponse
	query := url.Values{"ps_args": {topPSArgs}}
	if err := c.getJSON(ctx, containerPath(containerID, "/top"), query, &top); err != nil {
		return nil, fmt.Errorf("list processes of %s: %w", containerID, err)
	}
	processes, err := parseTop(top)
	if err != nil {
		return nil, fmt.Errorf("list processes of %s: %w", containerID, err)
	}
	return processes, nil
}

// parseTop maps rows by their titles. PID is required; any other column may be missing.
func parseTop(top topResponse) ([]model.TopProcess, error) {
	var index [topColumnCount]int
	for i := range index {
		index[i] = -1
	}
	for i, title := range top.Titles {
		if column, ok := topColumnFor(title); ok && index[column] < 0 {
			index[column] = i
		}
	}
	if index[topPID] < 0 {
		return nil, fmt.Errorf("parse top: no PID column in %q", top.Titles)
	}

	processes := make([]model.TopProcess, 0, len(top.Processes))
	for _, row := range top.Processes {
		field := func(column topColumn) string {
			if i := index[column]; i >= 0 && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		pid, err := strconv.Atoi(field(topPID))
		if err != nil {
			return nil, fmt.Errorf("parse top: pid %q: %w", field(topPID), err)
		}
		process := model.TopProcess{
			PID:     pid,
			PPID:    atoiOrZero(field(topPPID)),
			User:    field(topUser),
			Threads: atoiOrZero(field(topThreads)),
			CPUTime: parseCPUTime(field(topCPUTime)),
			Command: field(topCommand),
		}
		if rss, err := strconv.ParseInt(field(topRSS), 10, 64); err == nil {
			process.RSSBytes = rss * 1024
		}
		if state := field(topState); state != "" {
			process.State = state[:1]
		}
		processes = append(processes, process)
	}
	return processes, nil
}

// atoiOrZero returns 0 for a missing or malformed optional column, which TopProcess documents
// as unknown.
func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// parseCPUTime reads whole seconds (cputimes) or the [[dd-]hh:]mm:ss form (time).
func parseCPUTime(s string) time.Duration {
	if s == "" {
		return 0
	}
	var days int
	if dayText, rest, ok := strings.Cut(s, "-"); ok {
		days = atoiOrZero(dayText)
		s = rest
	}
	var seconds int
	for part := range strings.SplitSeq(s, ":") {
		seconds = seconds*60 + atoiOrZero(part)
	}
	return time.Duration(days*86400+seconds) * time.Second
}
