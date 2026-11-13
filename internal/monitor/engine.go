package monitor

import (
	"context"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Engine is the part of the Docker Engine API the monitor reads. Every call is read-only.
type Engine interface {
	// Info describes the engine; the monitor asks once and keeps the answer.
	Info(ctx context.Context) (model.EngineInfo, error)
	// Containers lists all containers, stopped ones included.
	Containers(ctx context.Context) ([]model.ContainerInfo, error)
	// Stats returns the current cumulative counters of a running or paused container without
	// waiting for a second reading.
	Stats(ctx context.Context, containerID string) (model.ContainerStats, error)
	// Top lists the processes of a running container. Used only in model.ModeAPI.
	Top(ctx context.Context, containerID string) ([]model.TopProcess, error)
	// UserNames maps UIDs to user names from the container's own /etc/passwd.
	UserNames(ctx context.Context, containerID string) (map[uint32]string, error)
}
