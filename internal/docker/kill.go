package docker

import (
	"context"
	"fmt"
	"net/url"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Kill sends signal to the main process of a container (POST /containers/{id}/kill). The error
// matches ErrNotFound for an unknown container.
func (c *Client) Kill(ctx context.Context, containerID string, signal model.Signal) error {
	query := url.Values{"signal": {string(signal)}}
	if err := c.postJSON(ctx, containerPath(containerID, "/kill"), query, nil, nil); err != nil {
		return fmt.Errorf("send %s to container %s: %w", signal, containerID, err)
	}
	return nil
}
