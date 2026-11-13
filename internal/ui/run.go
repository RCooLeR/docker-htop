package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Options configures the UI.
type Options struct {
	Interval time.Duration // shown in the header as the refresh interval
	ShowAll  bool          // start with stopped containers visible
}

// Run shows the full-screen UI until the user quits or ctx is done. It returns nil on a normal
// quit, including a quit caused by ctx. Snapshots are read from the channel until it closes;
// after that the last snapshot stays on screen. Actions are called only after the user confirms
// them.
func Run(ctx context.Context, snapshots <-chan model.Snapshot, actions Actions, opts Options) error {
	stop := ignoreInterrupts()
	defer stop()

	m := newScreen(snapshots, actions, opts, newStyles(lipgloss.NewRenderer(os.Stdout)))
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithoutSignalHandler())
	_, err := p.Run()
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("run terminal ui: %w", err)
	}
	return nil
}

// ignoreInterrupts drains os.Interrupt until the returned stop function is called. While a logs
// or shell command owns the terminal, Ctrl+C reaches the whole foreground process group and must
// stop only that command. signal.Ignore is not used because ignored signals are inherited by
// child processes, which would make docker logs -f unstoppable.
func ignoreInterrupts() (stop func()) {
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, os.Interrupt)
	go func() {
		for {
			select {
			case <-sigs:
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(sigs)
		close(done)
	}
}
