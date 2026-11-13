package app

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/RCooLeR/docker-htop/internal/docker"
	"github.com/RCooLeR/docker-htop/internal/model"
	"github.com/RCooLeR/docker-htop/internal/ui"
)

// envOriginalCLI is set by the docker CLI when it runs a plugin and names the docker binary the
// user invoked.
const envOriginalCLI = "DOCKER_CLI_PLUGIN_ORIGINAL_CLI_COMMAND"

// logsTail is how many existing log lines the logs view shows before it follows new ones.
const logsTail = "200"

// shellScript picks bash where the image has it. It is a constant and carries no user input.
const shellScript = "command -v bash >/dev/null 2>&1 && exec bash || exec sh"

// actions implements ui.Actions on top of the Engine API client and the docker CLI.
type actions struct {
	client *docker.Client
	// image is the helper image; nil when the plugin may not or cannot use helper containers.
	image *helperImage
	// dockerCLI and globalArgs make logs and shell commands talk to the same engine as the
	// plugin, whatever context the user's shell has.
	dockerCLI  string
	globalArgs []string
}

var _ ui.Actions = (*actions)(nil)

func newActions(client *docker.Client, image *helperImage, endpoint, configDir string) *actions {
	globalArgs := []string{"--host", endpoint}
	if configDir != "" {
		globalArgs = append(globalArgs, "--config", configDir)
	}
	return &actions{
		client:     client,
		image:      image,
		dockerCLI:  cmp.Or(os.Getenv(envOriginalCLI), "docker"),
		globalArgs: globalArgs,
	}
}

func (a *actions) SignalContainer(ctx context.Context, containerID string, signal model.Signal) error {
	return a.client.Kill(ctx, containerID, signal)
}

func (a *actions) SignalProcess(ctx context.Context, containerID string, pid int, signal model.Signal) error {
	if a.image == nil {
		return ui.ErrProcessSignalUnavailable
	}
	imageRef, err := a.image.reference(ctx)
	if err != nil {
		return fmt.Errorf("prepare the signal helper image: %w", err)
	}
	return a.client.SignalProcess(ctx, imageRef, containerID, pid, signal)
}

func (a *actions) LogsCommand(containerID string) *exec.Cmd {
	return a.docker("logs", "--follow", "--tail", logsTail, containerID)
}

func (a *actions) ShellCommand(containerID string) *exec.Cmd {
	return a.docker("exec", "--interactive", "--tty", containerID, "sh", "-c", shellScript)
}

// docker builds a docker CLI command. The UI runs it through the terminal and ends it with the
// user's Ctrl+C, so it carries no context.
func (a *actions) docker(args ...string) *exec.Cmd {
	argv := make([]string, 0, len(a.globalArgs)+len(args))
	argv = append(argv, a.globalArgs...)
	argv = append(argv, args...)
	return exec.Command(a.dockerCLI, argv...) //nolint:gosec,noctx // G204: argv, no shell; the container ID comes from the engine
}
