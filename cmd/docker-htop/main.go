// Command docker-htop is a Docker CLI plugin. Installed in a cli-plugins directory it adds
// `docker htop`, a live view of containers and their processes that merges docker stats with
// htop. The docker CLI runs it as `docker-htop [docker global flags] htop [flags]`.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"

	"github.com/RCooLeR/docker-htop/internal/app"
	"github.com/RCooLeR/docker-htop/internal/buildinfo"
	"github.com/RCooLeR/docker-htop/internal/config"
)

const (
	pluginName  = "htop"
	usageText   = "docker htop [options]"
	flagVersion = "version"
	// exitInterrupted is the conventional exit status after Ctrl+C.
	exitInterrupted = 130
)

func main() {
	os.Exit(run())
}

func run() int {
	// Ctrl+C is left to the UI: in the full-screen view it is a key press, and while a logs or
	// shell command owns the terminal it must stop only that command.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	err := newCommand().Run(ctx, os.Args)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, context.Canceled):
		return exitInterrupted
	default:
		fmt.Fprintf(os.Stderr, "docker htop: %v\n", err)
		return 1
	}
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:      "docker-htop",
		Usage:     app.Description,
		UsageText: usageText,
		// The library's own --version exists only on the root command, but the docker CLI
		// calls the plugin as `docker-htop htop --version`.
		Flags:           append(config.Flags(), &cli.BoolFlag{Name: flagVersion, Usage: "print the version and exit"}),
		Action:          runPlugin,
		HideHelpCommand: true,
		Commands: []*cli.Command{
			{
				Name:            pluginName,
				Usage:           app.Description,
				UsageText:       usageText,
				Action:          runPlugin,
				HideHelpCommand: true,
			},
			{
				Name:   app.MetadataCommand,
				Hidden: true,
				Action: func(context.Context, *cli.Command) error {
					return app.WriteMetadata(os.Stdout)
				},
			},
		},
	}
}

func runPlugin(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Present() {
		return fmt.Errorf("unexpected argument %q", cmd.Args().First())
	}
	if cmd.Bool(flagVersion) {
		_, err := fmt.Fprintf(os.Stdout, "docker htop version %s\n", buildinfo.Version)
		return err
	}
	cfg, err := config.FromCommand(cmd)
	if err != nil {
		return err
	}
	return app.Run(ctx, cfg, os.Stdout)
}
