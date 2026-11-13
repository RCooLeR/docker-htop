package docker

import (
	"net/http"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const cgroupV2Stats = `{
  "read": "2026-11-10T10:00:00.123456789Z",
  "preread": "0001-01-01T00:00:00Z",
  "pids_stats": {"current": 13, "limit": 18446744073709551615},
  "blkio_stats": {
    "io_service_bytes_recursive": [
      {"major": 254, "minor": 0, "op": "read", "value": 1048576},
      {"major": 254, "minor": 0, "op": "write", "value": 2097152},
      {"major": 254, "minor": 16, "op": "Read", "value": 4096}
    ],
    "io_serviced_recursive": null
  },
  "num_procs": 0,
  "storage_stats": {},
  "cpu_stats": {
    "cpu_usage": {"total_usage": 123456789000, "usage_in_kernelmode": 23456789000, "usage_in_usermode": 100000000000},
    "system_cpu_usage": 987654321000000,
    "online_cpus": 16,
    "throttling_data": {"periods": 0, "throttled_periods": 0, "throttled_time": 0}
  },
  "precpu_stats": {"cpu_usage": {"total_usage": 0}, "throttling_data": {}},
  "memory_stats": {
    "usage": 104857600,
    "stats": {"active_anon": 50000000, "active_file": 1000000, "anon": 60000000, "file": 30000000,
              "inactive_anon": 0, "inactive_file": 20971520, "pgfault": 12345},
    "limit": 16624349184
  },
  "name": "/web",
  "id": "abc",
  "networks": {
    "eth0": {"rx_bytes": 1000, "rx_packets": 10, "tx_bytes": 2000, "tx_packets": 20},
    "eth1": {"rx_bytes": 10, "rx_packets": 1, "tx_bytes": 20, "tx_packets": 2}
  }
}`

const noNetworkStats = `{
  "read": "2026-11-10T10:00:01Z",
  "pids_stats": {"current": 1},
  "blkio_stats": {"io_service_bytes_recursive": null},
  "cpu_stats": {"cpu_usage": {"total_usage": 5000000}},
  "memory_stats": {"usage": 1000, "stats": {"total_inactive_file": 4000}, "limit": 2000000},
  "name": "/isolated",
  "id": "def"
}`

func TestStats(t *testing.T) {
	tests := []struct {
		name string
		body string
		want model.ContainerStats
	}{
		{
			name: "cgroup v2 with networks",
			body: cgroupV2Stats,
			want: model.ContainerStats{
				Read:             time.Date(2026, 11, 10, 10, 0, 0, 123456789, time.UTC),
				CPUTotal:         123456789 * time.Microsecond,
				MemoryBytes:      104857600 - 20971520,
				MemoryLimitBytes: 16624349184,
				HasNetwork:       true,
				NetRxBytes:       1010,
				NetTxBytes:       2020,
				BlockReadBytes:   1048576 + 4096,
				BlockWriteBytes:  2097152,
				PIDs:             13,
			},
		},
		{
			name: "no networks and cache above usage",
			body: noNetworkStats,
			want: model.ContainerStats{
				Read:             time.Date(2026, 11, 10, 10, 0, 1, 0, time.UTC),
				CPUTotal:         5 * time.Millisecond,
				MemoryBytes:      0,
				MemoryLimitBytes: 2000000,
				PIDs:             1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
				"GET /containers/abc/stats": respondJSON(tt.body),
			})
			got, err := c.Stats(t.Context(), "abc")
			if err != nil {
				t.Fatal(err)
			}
			if !got.Read.Equal(tt.want.Read) {
				t.Fatalf("Read = %v, want %v", got.Read, tt.want.Read)
			}
			got.Read = tt.want.Read
			if got != tt.want {
				t.Fatalf("Stats() = %+v\nwant      %+v", got, tt.want)
			}
			req, _ := engine.find(http.MethodGet, "/containers/abc/stats")
			if req.Query.Get("stream") != "false" || req.Query.Get("one-shot") != "true" {
				t.Fatalf("stats query = %v", req.Query)
			}
		})
	}
}

func TestStatsEmptyNetworks(t *testing.T) {
	_, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /containers/abc/stats": respondJSON(`{"networks": {}, "memory_stats": {"usage": 10, "stats": {}}}`),
	})
	got, err := c.Stats(t.Context(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.HasNetwork || got.MemoryBytes != 10 {
		t.Fatalf("Stats() = %+v, want no network and the full usage", got)
	}
}
