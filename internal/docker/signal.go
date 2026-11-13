package docker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// signalTimeout bounds one SignalProcess call from create to exit.
const signalTimeout = 30 * time.Second

// SignalProcess sends signal to one process of a container, identified by its PID on the Docker
// host, through a one-shot helper container from imageRef that shares the host PID namespace
// and holds only CAP_KILL. The helper checks that the process belongs to containerID. A non-zero
// helper exit gives an error carrying its output. The helper is always force-removed.
func (c *Client) SignalProcess(ctx context.Context, imageRef, containerID string, pid int, signal model.Signal) (err error) {
	if containerID == "" || pid <= 0 || signal == "" {
		return fmt.Errorf("signal process: invalid target container %q pid %d signal %q", containerID, pid, signal)
	}
	ctx, cancel := context.WithTimeout(ctx, signalTimeout)
	defer cancel()

	helperID, err := c.createContainer(ctx, helperName("docker-htop-signal"), signalSpec(imageRef, containerID, pid, signal))
	if err != nil {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}
	defer func() {
		if removeErr := c.removeHelper(ctx, helperID); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
	}()

	if err := c.startContainer(ctx, helperID); err != nil {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}
	exitCode, err := c.waitContainer(ctx, helperID)
	if err != nil {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}
	if exitCode == 0 {
		return nil
	}
	output, logErr := c.containerLogs(ctx, helperID)
	if output == "" && logErr != nil {
		output = logErr.Error()
	}
	return fmt.Errorf("send %s to pid %d of container %s: helper exited with code %d: %s",
		signal, pid, containerID, exitCode, output)
}
