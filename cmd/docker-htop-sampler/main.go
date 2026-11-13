// Command docker-htop-sampler runs inside the docker htop helper container, which shares the
// Docker host's PID namespace. It streams /proc readings as JSON lines or sends one signal to a
// container process; the command line is documented in internal/model/sample.go.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
	"github.com/RCooLeR/docker-htop/internal/sampler"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

const usageText = `usage:
  docker-htop-sampler stream --interval <duration>
  docker-htop-sampler signal --container <full container id> --pid <host pid> --signal <SIGNAME>`

// usageError marks an error caused by the command line.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var err error
	switch {
	case len(args) == 0:
		err = usageError{errors.New("missing command")}
	case args[0] == model.SamplerCommandStream:
		err = runStream(args[1:], stdin, stdout)
	case args[0] == model.SamplerCommandSignal:
		err = runSignal(args[1:])
	default:
		err = usageError{fmt.Errorf("unknown command %q", args[0])}
	}

	var usageErr usageError
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, flag.ErrHelp):
		_, _ = fmt.Fprintln(stderr, usageText) // nothing useful to do when stderr is gone
		return exitOK
	case errors.As(err, &usageErr):
		_, _ = fmt.Fprintf(stderr, "docker-htop-sampler: %v\n%s\n", err, usageText)
		return exitUsage
	default:
		_, _ = fmt.Fprintf(stderr, "docker-htop-sampler: %v\n", err)
		return exitRuntime
	}
}

func runStream(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet(model.SamplerCommandStream, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	interval := flags.Duration("interval", time.Second, "time between samples")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if *interval < sampler.MinInterval {
		return usageError{fmt.Errorf("interval %v is below %v", *interval, sampler.MinInterval)}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The plugin holds stdin open for the sampler's whole life and the engine closes it when the
	// plugin dies, so EOF ends the stream. The goroutine ends at EOF or with the process.
	go func() {
		_, _ = io.Copy(io.Discard, stdin) // a read error means the same as EOF here
		cancel()
	}()

	out := bufio.NewWriter(stdout)
	return sampler.Stream(ctx, sampler.NewReader(sampler.DefaultProcRoot), out, *interval)
}

func runSignal(args []string) error {
	flags := flag.NewFlagSet(model.SamplerCommandSignal, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	containerID := flags.String("container", "", "full container id")
	pid := flags.Int("pid", 0, "host pid")
	sig := flags.String("signal", "", "signal name, for example SIGTERM")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if *containerID == "" || *pid == 0 || *sig == "" {
		return usageError{errors.New("signal needs --container, --pid and --signal")}
	}

	err := sampler.Signal(sampler.DefaultProcRoot, *containerID, *pid, model.Signal(*sig))
	if errors.Is(err, sampler.ErrInvalidPID) || errors.Is(err, sampler.ErrInvalidContainerID) ||
		errors.Is(err, sampler.ErrUnknownSignal) {
		return usageError{err}
	}
	return err
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return usageError{fmt.Errorf("%s: %w", flags.Name(), err)}
	}
	if flags.NArg() > 0 {
		return usageError{fmt.Errorf("%s: unexpected argument %q", flags.Name(), flags.Arg(0))}
	}
	return nil
}
