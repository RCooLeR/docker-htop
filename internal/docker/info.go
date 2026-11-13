package docker

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

type infoResponse struct {
	Name          string `json:"Name"`
	ServerVersion string `json:"ServerVersion"`
	Architecture  string `json:"Architecture"`
	NCPU          int    `json:"NCPU"`
	MemTotal      int64  `json:"MemTotal"`
}

// Info describes the engine (GET /info).
func (c *Client) Info(ctx context.Context) (model.EngineInfo, error) {
	var info infoResponse
	if err := c.getJSON(ctx, "/info", nil, &info); err != nil {
		return model.EngineInfo{}, fmt.Errorf("read engine info: %w", err)
	}
	return model.EngineInfo{
		Name:         info.Name,
		Version:      info.ServerVersion,
		Architecture: info.Architecture,
		CPUs:         info.NCPU,
		MemoryBytes:  info.MemTotal,
	}, nil
}

type containerListEntry struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

// Containers lists all containers, stopped ones included (GET /containers/json?all=true).
func (c *Client) Containers(ctx context.Context) ([]model.ContainerInfo, error) {
	var entries []containerListEntry
	if err := c.getJSON(ctx, "/containers/json", url.Values{"all": {"true"}}, &entries); err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	containers := make([]model.ContainerInfo, 0, len(entries))
	for _, entry := range entries {
		containers = append(containers, entry.toModel())
	}
	return containers, nil
}

func (e containerListEntry) toModel() model.ContainerInfo {
	var name string
	if len(e.Names) > 0 {
		name = strings.TrimPrefix(e.Names[0], "/")
	}
	return model.ContainerInfo{
		ID:      e.ID,
		Name:    name,
		Image:   e.Image,
		State:   model.ContainerState(e.State),
		Status:  e.Status,
		Labels:  e.Labels,
		Created: time.Unix(e.Created, 0),
	}
}
