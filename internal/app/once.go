package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/RCooLeR/docker-htop/internal/model"
	"github.com/RCooLeR/docker-htop/internal/ui"
)

const (
	// plainWidth is the line width of --no-stream output when stdout is not a terminal.
	plainWidth = 120
	// rateReadings is how many readings a rate needs. --no-stream prints the reading after
	// that, and gives up waiting after those intervals plus a grace period.
	rateReadings  = 2
	noStreamGrace = 5 * time.Second
)

// envNoColor is the conventional switch for plain output on a terminal (no-color.org).
const envNoColor = "NO_COLOR"

// printOnce writes one complete snapshot to stdout as text. Rates need two readings, so it
// skips snapshots that do not have them yet, up to a deadline. It then takes one reading more:
// user names are looked up in the background once processes are known and arrive a reading
// later.
func printOnce(ctx context.Context, snapshots <-chan model.Snapshot, stdout *os.File, options ui.Options) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	wait := time.Duration(rateReadings+2)*options.Interval + noStreamGrace
	deadline := time.NewTimer(wait)
	defer deadline.Stop()

	var latest *model.Snapshot
	settling := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if latest == nil {
				return fmt.Errorf("no reading from the engine within %s", wait)
			}
			return writePlain(stdout, *latest, options)
		case snapshot, ok := <-snapshots:
			if !ok {
				if latest == nil {
					return errors.New("the monitor stopped before the first reading")
				}
				return writePlain(stdout, *latest, options)
			}
			latest = &snapshot
			if !hasRates(snapshot) {
				continue
			}
			if settling {
				return writePlain(stdout, snapshot, options)
			}
			settling = true
		}
	}
}

// hasRates reports whether the snapshot carries the readings that need two samples.
func hasRates(s model.Snapshot) bool {
	if s.Mode != model.ModeAPI && s.Host.CoreBusy == nil {
		return false
	}
	running := false
	for _, c := range s.Containers {
		if c.State != model.StateRunning {
			continue
		}
		running = true
		if c.CPUPercent != nil {
			return true
		}
	}
	return !running
}

func writePlain(stdout *os.File, s model.Snapshot, options ui.Options) error {
	width, colour := plainWidth, false
	if term.IsTerminal(stdout.Fd()) {
		colour = os.Getenv(envNoColor) == ""
		if w, _, err := term.GetSize(stdout.Fd()); err == nil && w > 0 {
			width = w
		}
	}
	if _, err := fmt.Fprint(stdout, ui.RenderPlain(s, width, colour, options)); err != nil {
		return fmt.Errorf("write the reading: %w", err)
	}
	return nil
}
