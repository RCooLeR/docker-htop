package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

// parse runs the flags through a command shaped like the plugin's: flags on the root, the
// monitor reachable both directly and through the "htop" subcommand.
func parse(t *testing.T, args ...string) (Config, error) {
	t.Helper()
	var (
		cfg    Config
		cfgErr error
	)
	action := func(_ context.Context, cmd *cli.Command) error {
		cfg, cfgErr = FromCommand(cmd)
		return nil
	}
	root := &cli.Command{
		Name:     "docker-htop",
		Flags:    Flags(),
		Action:   action,
		Commands: []*cli.Command{{Name: "htop", Action: action}},
	}
	if err := root.Run(t.Context(), append([]string{"docker-htop"}, args...)); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return cfg, cfgErr
}

func TestFromCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Config
	}{
		{
			name: "defaults",
			args: []string{"htop"},
			want: Config{Interval: DefaultInterval},
		},
		{
			name: "plugin flags after the plugin name",
			args: []string{"htop", "--interval", "2s", "--all", "--no-stream", "--no-sampler"},
			want: Config{Interval: 2 * time.Second, ShowAll: true, NoStream: true, NoSampler: true},
		},
		{
			name: "docker global flags before the plugin name",
			args: []string{"--context", "desktop-linux", "--config", "/tmp/docker", "-D", "-l", "debug", "htop", "-d", "500ms"},
			want: Config{Context: "desktop-linux", ConfigDir: "/tmp/docker", Interval: 500 * time.Millisecond},
		},
		{
			name: "host flag and direct invocation",
			args: []string{"-H", "unix:///var/run/docker.sock", "-a"},
			want: Config{Host: "unix:///var/run/docker.sock", Interval: DefaultInterval, ShowAll: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse(t, tc.args...)
			if err != nil {
				t.Fatalf("FromCommand: %v", err)
			}
			if got != tc.want {
				t.Errorf("config = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestFromCommandEnvironment(t *testing.T) {
	t.Setenv("DOCKER_HTOP_INTERVAL", "3s")
	t.Setenv("DOCKER_HTOP_NO_SAMPLER", "true")
	got, err := parse(t, "htop")
	if err != nil {
		t.Fatalf("FromCommand: %v", err)
	}
	if got.Interval != 3*time.Second || !got.NoSampler {
		t.Errorf("config = %+v, want interval 3s and no sampler from the environment", got)
	}
}

func TestFromCommandRejects(t *testing.T) {
	tests := []struct {
		name string
		args []string
		is   error
	}{
		{name: "interval too short", args: []string{"htop", "--interval", "50ms"}},
		{name: "interval too long", args: []string{"htop", "--interval", "5m"}},
		{name: "tls", args: []string{"--tls", "htop"}, is: ErrTLSUnsupported},
		{name: "tlsverify", args: []string{"--tlsverify", "htop"}, is: ErrTLSUnsupported},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, tc.args...)
			if err == nil {
				t.Fatal("FromCommand accepted an invalid configuration")
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("error = %v, want %v", err, tc.is)
			}
		})
	}
}
