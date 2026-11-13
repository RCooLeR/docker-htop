// Package config defines the plugin's command-line flags and the validated settings they
// produce. It knows nothing about the engine or the UI.
package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/urfave/cli/v3"
)

// Bounds and default of the refresh interval. The lower bound is the shortest interval the
// sampler accepts.
const (
	DefaultInterval = time.Second
	MinInterval     = 200 * time.Millisecond
	MaxInterval     = time.Minute
)

// Flag names shared by Flags and FromCommand.
const (
	flagHost      = "host"
	flagContext   = "context"
	flagConfigDir = "config"
	flagTLS       = "tls"
	flagTLSVerify = "tlsverify"
	flagInterval  = "interval"
	flagNoStream  = "no-stream"
	flagNoSampler = "no-sampler"
	flagAll       = "all"
)

// ErrTLSUnsupported is returned when the docker CLI was asked for a TLS connection, which the
// plugin's Engine API client does not implement.
var ErrTLSUnsupported = errors.New("TLS connections to the engine are not supported")

// Config is the validated set of options for one run.
type Config struct {
	// Host, Context and ConfigDir are the docker global flags --host, --context and --config,
	// which the docker CLI passes to a plugin in front of its name. Empty when not given.
	Host      string
	Context   string
	ConfigDir string
	// TLS is true when --tls or --tlsverify was given.
	TLS bool

	// Interval is the time between two readings.
	Interval time.Duration
	// NoStream prints one reading as text and exits instead of opening the UI.
	NoStream bool
	// NoSampler keeps the plugin on the Engine API: it reads no /proc and starts no helper
	// container.
	NoSampler bool
	// ShowAll starts with stopped containers visible.
	ShowAll bool
}

// Flags returns the plugin's flags. The hidden ones mirror the docker CLI's global flags: the
// CLI hands a plugin its whole command line, so they must parse even where they are ignored.
func Flags() []cli.Flag {
	return []cli.Flag{
		&cli.DurationFlag{
			Name:    flagInterval,
			Aliases: []string{"d"},
			Usage:   "time between two readings",
			Value:   DefaultInterval,
			Sources: cli.EnvVars("DOCKER_HTOP_INTERVAL"),
		},
		&cli.BoolFlag{
			Name:    flagAll,
			Aliases: []string{"a"},
			Usage:   "show stopped containers too",
			Sources: cli.EnvVars("DOCKER_HTOP_ALL"),
		},
		&cli.BoolFlag{
			Name:  flagNoStream,
			Usage: "print one reading as text and exit",
		},
		&cli.BoolFlag{
			Name:    flagNoSampler,
			Usage:   "use only the Engine API: read no /proc, start no helper container",
			Sources: cli.EnvVars("DOCKER_HTOP_NO_SAMPLER"),
		},

		&cli.StringFlag{Name: flagHost, Aliases: []string{"H"}, Hidden: true},
		&cli.StringFlag{Name: flagContext, Aliases: []string{"c"}, Hidden: true},
		&cli.StringFlag{Name: flagConfigDir, Hidden: true},
		&cli.BoolFlag{Name: flagTLS, Hidden: true},
		&cli.BoolFlag{Name: flagTLSVerify, Hidden: true},
		&cli.StringFlag{Name: "tlscacert", Hidden: true},
		&cli.StringFlag{Name: "tlscert", Hidden: true},
		&cli.StringFlag{Name: "tlskey", Hidden: true},
		&cli.BoolFlag{Name: "debug", Aliases: []string{"D"}, Hidden: true},
		&cli.StringFlag{Name: "log-level", Aliases: []string{"l"}, Hidden: true},
	}
}

// FromCommand reads the parsed flags of cmd into a Config and validates it.
func FromCommand(cmd *cli.Command) (Config, error) {
	cfg := Config{
		Host:      cmd.String(flagHost),
		Context:   cmd.String(flagContext),
		ConfigDir: cmd.String(flagConfigDir),
		TLS:       cmd.Bool(flagTLS) || cmd.Bool(flagTLSVerify),
		Interval:  cmd.Duration(flagInterval),
		NoStream:  cmd.Bool(flagNoStream),
		NoSampler: cmd.Bool(flagNoSampler),
		ShowAll:   cmd.Bool(flagAll),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate reports the first setting the plugin cannot run with.
func (c Config) Validate() error {
	if c.TLS {
		return ErrTLSUnsupported
	}
	if c.Interval < MinInterval || c.Interval > MaxInterval {
		return fmt.Errorf("--%s %s is outside %s to %s", flagInterval, c.Interval, MinInterval, MaxInterval)
	}
	return nil
}
