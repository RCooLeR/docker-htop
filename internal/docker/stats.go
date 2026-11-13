package docker

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

type statsResponse struct {
	Read     time.Time `json:"read"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
	} `json:"cpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IOServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
	PidsStats struct {
		Current uint64 `json:"current"`
	} `json:"pids_stats"`
}

// Stats returns one reading of a container's cumulative counters without waiting for a second
// one (GET /containers/{id}/stats?stream=false&one-shot=true). The error matches ErrNotFound
// for an unknown container.
func (c *Client) Stats(ctx context.Context, containerID string) (model.ContainerStats, error) {
	query := url.Values{"stream": {"false"}, "one-shot": {"true"}}
	var stats statsResponse
	if err := c.getJSON(ctx, containerPath(containerID, "/stats"), query, &stats); err != nil {
		return model.ContainerStats{}, fmt.Errorf("read stats for %s: %w", containerID, err)
	}
	return stats.toModel(), nil
}

func (s statsResponse) toModel() model.ContainerStats {
	out := model.ContainerStats{
		Read:             s.Read,
		CPUTotal:         time.Duration(s.CPUStats.CPUUsage.TotalUsage),
		MemoryBytes:      s.memoryWithoutCache(),
		MemoryLimitBytes: s.MemoryStats.Limit,
		HasNetwork:       len(s.Networks) > 0,
		PIDs:             int(s.PidsStats.Current),
	}
	for _, network := range s.Networks {
		out.NetRxBytes += network.RxBytes
		out.NetTxBytes += network.TxBytes
	}
	for _, entry := range s.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			out.BlockReadBytes += entry.Value
		case "write":
			out.BlockWriteBytes += entry.Value
		}
	}
	return out
}

// memoryWithoutCache subtracts the inactive file cache the way docker stats does: inactive_file
// on cgroup v2, total_inactive_file on v1.
func (s statsResponse) memoryWithoutCache() uint64 {
	usage := s.MemoryStats.Usage
	cache, ok := s.MemoryStats.Stats["inactive_file"]
	if !ok {
		cache = s.MemoryStats.Stats["total_inactive_file"]
	}
	if cache > usage {
		return 0
	}
	return usage - cache
}
