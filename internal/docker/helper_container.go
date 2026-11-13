package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const (
	// cleanupTimeout bounds the removal of a helper container, also after the caller's context
	// is cancelled.
	cleanupTimeout = 10 * time.Second

	samplerMemoryBytes = 64 << 20
	samplerPIDsLimit   = 16
	signalMemoryBytes  = 32 << 20
	signalPIDsLimit    = 8
)

// containerSpec is the body of POST /containers/create, limited to the fields the helpers set.
type containerSpec struct {
	Image        string            `json:"Image"`
	Cmd          []string          `json:"Cmd"`
	Labels       map[string]string `json:"Labels"`
	AttachStdin  bool              `json:"AttachStdin"`
	AttachStdout bool              `json:"AttachStdout"`
	AttachStderr bool              `json:"AttachStderr"`
	OpenStdin    bool              `json:"OpenStdin"`
	StdinOnce    bool              `json:"StdinOnce"`
	Tty          bool              `json:"Tty"`
	HostConfig   hostConfig        `json:"HostConfig"`
}

type hostConfig struct {
	PidMode        string     `json:"PidMode"`
	NetworkMode    string     `json:"NetworkMode"`
	ReadonlyRootfs bool       `json:"ReadonlyRootfs"`
	CapDrop        []string   `json:"CapDrop"`
	CapAdd         []string   `json:"CapAdd,omitempty"`
	SecurityOpt    []string   `json:"SecurityOpt"`
	AutoRemove     bool       `json:"AutoRemove"`
	LogConfig      *logConfig `json:"LogConfig,omitempty"`
	Memory         int64      `json:"Memory"`
	PidsLimit      int64      `json:"PidsLimit"`
}

type logConfig struct {
	Type string `json:"Type"`
}

// hardenedHostConfig shares the host PID namespace and takes away everything else.
func hardenedHostConfig(memoryBytes, pidsLimit int64) hostConfig {
	return hostConfig{
		PidMode:        "host",
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		Memory:         memoryBytes,
		PidsLimit:      pidsLimit,
	}
}

func samplerSpec(imageRef string, interval time.Duration) containerSpec {
	host := hardenedHostConfig(samplerMemoryBytes, samplerPIDsLimit)
	host.AutoRemove = true
	host.LogConfig = &logConfig{Type: "none"}
	return containerSpec{
		Image:        imageRef,
		Cmd:          []string{model.SamplerCommandStream, "--interval", interval.String()},
		Labels:       map[string]string{model.LabelRole: model.RoleSampler},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		HostConfig:   host,
	}
}

// signalSpec keeps the daemon's default log driver, because the helper's output is read back
// through the logs endpoint when it fails.
func signalSpec(imageRef, containerID string, pid int, signal model.Signal) containerSpec {
	host := hardenedHostConfig(signalMemoryBytes, signalPIDsLimit)
	host.CapAdd = []string{"KILL"}
	return containerSpec{
		Image: imageRef,
		Cmd: []string{
			model.SamplerCommandSignal,
			"--container", containerID,
			"--pid", strconv.Itoa(pid),
			"--signal", string(signal),
		},
		Labels:     map[string]string{model.LabelRole: model.RoleSignal},
		HostConfig: host,
	}
}

// helperName returns prefix plus a random suffix, for example docker-htop-sampler-1a2b3c4d5e6f.
func helperName(prefix string) string {
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix) // crypto/rand.Read never returns an error
	return prefix + "-" + hex.EncodeToString(suffix)
}

func (c *Client) createContainer(ctx context.Context, name string, spec containerSpec) (string, error) {
	var created struct {
		ID string `json:"Id"`
	}
	if err := c.postJSON(ctx, "/containers/create", url.Values{"name": {name}}, spec, &created); err != nil {
		return "", fmt.Errorf("create container %s: %w", name, err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("create container %s: the engine returned no id", name)
	}
	return created.ID, nil
}

func (c *Client) startContainer(ctx context.Context, containerID string) error {
	if err := c.postJSON(ctx, containerPath(containerID, "/start"), nil, nil, nil); err != nil {
		return fmt.Errorf("start container %s: %w", containerID, err)
	}
	return nil
}

// removeHelper force-removes a helper container with its own timeout, so it also runs after the
// caller's context ended. A container that is already gone or being auto-removed is fine.
func (c *Client) removeHelper(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	resp, err := c.do(ctx, http.MethodDelete, containerPath(containerID, ""), url.Values{"force": {"1"}}, nil, "")
	if errors.Is(err, ErrNotFound) || hasStatus(err, http.StatusConflict) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove helper container %s: %w", containerID, err)
	}
	closeQuietly(resp.Body)
	return nil
}

// waitContainer waits until the container is not running and returns its exit code.
func (c *Client) waitContainer(ctx context.Context, containerID string) (int64, error) {
	var result struct {
		StatusCode int64 `json:"StatusCode"`
		Error      *struct {
			Message string `json:"Message"`
		} `json:"Error"`
	}
	query := url.Values{"condition": {"not-running"}}
	if err := c.postJSON(ctx, containerPath(containerID, "/wait"), query, nil, &result); err != nil {
		return 0, fmt.Errorf("wait for container %s: %w", containerID, err)
	}
	if result.Error != nil && result.Error.Message != "" {
		return 0, fmt.Errorf("wait for container %s: %s", containerID, result.Error.Message)
	}
	return result.StatusCode, nil
}

// containerLogs returns the first maxLogBytes of a container's combined stdout and stderr.
func (c *Client) containerLogs(ctx context.Context, containerID string) (string, error) {
	query := url.Values{"stdout": {"1"}, "stderr": {"1"}}
	resp, err := c.do(ctx, http.MethodGet, containerPath(containerID, "/logs"), query, nil, "")
	if err != nil {
		return "", fmt.Errorf("read logs of %s: %w", containerID, err)
	}
	defer closeQuietly(resp.Body)
	out := &cappedWriter{max: maxLogBytes}
	if err := demux(io.LimitReader(resp.Body, maxResponseBytes), out, out); err != nil {
		return out.String(), fmt.Errorf("read logs of %s: %w", containerID, err)
	}
	return out.String(), nil
}
