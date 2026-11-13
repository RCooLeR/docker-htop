// Package app wires the Engine API client, the sampler, the monitor and the UI into the
// `docker htop` command. It is the only package that knows all the others.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/RCooLeR/docker-htop/internal/config"
	"github.com/RCooLeR/docker-htop/internal/docker"
	"github.com/RCooLeR/docker-htop/internal/monitor"
	"github.com/RCooLeR/docker-htop/internal/ui"
)

// engineInfoTimeout bounds the first call to the engine, which decides whether it is reachable.
const engineInfoTimeout = 10 * time.Second

var _ monitor.Engine = (*docker.Client)(nil)

// Run connects to the engine and shows the monitor until the user quits or ctx is done. With
// cfg.NoStream it prints one reading to stdout instead.
func Run(ctx context.Context, cfg config.Config, stdout *os.File) error {
	endpoint, err := docker.ResolveEndpoint(docker.EndpointOptions{
		Host:      cfg.Host,
		Context:   cfg.Context,
		ConfigDir: cfg.ConfigDir,
	})
	if err != nil {
		return fmt.Errorf("resolve the docker endpoint: %w", err)
	}
	client, err := docker.New(endpoint)
	if err != nil {
		return fmt.Errorf("open %s: %w", endpoint, err)
	}
	defer client.Close()

	infoCtx, cancelInfo := context.WithTimeout(ctx, engineInfoTimeout)
	info, err := client.Info(infoCtx)
	cancelInfo()
	if err != nil {
		return fmt.Errorf("reach the docker engine at %s: %w", endpoint, err)
	}

	src := openSource(ctx, client, info, cfg)
	defer src.stop()

	mon := monitor.New(client, monitor.Options{
		Interval: cfg.Interval,
		Samples:  src.samples,
		Local:    src.local,
		Notice:   src.notice,
	})
	monitorCtx, stopMonitor := context.WithCancel(ctx)
	var (
		wg         sync.WaitGroup
		monitorErr error
	)
	wg.Go(func() { monitorErr = mon.Run(monitorCtx) })

	options := ui.Options{Interval: cfg.Interval, ShowAll: cfg.ShowAll}
	var runErr error
	if cfg.NoStream {
		runErr = printOnce(ctx, mon.Snapshots(), stdout, options)
	} else {
		runErr = ui.Run(ctx, mon.Snapshots(), newActions(client, src.image, endpoint, cfg.ConfigDir), options)
	}

	stopMonitor()
	wg.Wait()
	return errors.Join(runErr, monitorErr)
}
