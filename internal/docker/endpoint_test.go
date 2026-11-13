package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTestContext(t *testing.T, configDir, name, host string, withTLS bool) {
	t.Helper()
	sum := sha256.Sum256([]byte(name))
	hash := hex.EncodeToString(sum[:])
	metaDir := filepath.Join(configDir, "contexts", "meta", hash)
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"Name":"` + name + `","Metadata":{},"Endpoints":{"docker":{"Host":"` + host + `","SkipTLSVerify":false}}}`
	if err := os.WriteFile(filepath.Join(metaDir, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if withTLS {
		tlsDir := filepath.Join(configDir, "contexts", "tls", hash, "docker")
		if err := os.MkdirAll(tlsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tlsDir, "ca.pem"), []byte("cert"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestConfig(t *testing.T, configDir, currentContext string) {
	t.Helper()
	data := `{"auths":{},"currentContext":"` + currentContext + `"}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveEndpoint(t *testing.T) {
	const (
		desktopHost = "npipe:////./pipe/dockerDesktopLinuxEngine"
		remoteHost  = "tcp://10.0.0.5:2375"
		envHost     = "unix:///run/user/1000/docker.sock"
	)
	tests := []struct {
		name        string
		opts        func(configDir string) EndpointOptions
		env         map[string]string
		setup       func(t *testing.T, configDir string)
		want        string
		wantErr     bool
		unsupported bool
	}{
		{name: "platform default", want: defaultEndpoint},
		{
			name: "host flag",
			opts: func(string) EndpointOptions { return EndpointOptions{Host: remoteHost} },
			want: remoteHost,
		},
		{
			name: "DOCKER_HOST",
			env:  map[string]string{"DOCKER_HOST": envHost},
			want: envHost,
		},
		{
			name: "host flag beats DOCKER_HOST",
			opts: func(string) EndpointOptions { return EndpointOptions{Host: remoteHost} },
			env:  map[string]string{"DOCKER_HOST": envHost},
			want: remoteHost,
		},
		{
			name:  "DOCKER_CONTEXT",
			env:   map[string]string{"DOCKER_CONTEXT": "desktop-linux"},
			setup: func(t *testing.T, dir string) { writeTestContext(t, dir, "desktop-linux", desktopHost, false) },
			want:  desktopHost,
		},
		{
			name: "current context of config.json",
			setup: func(t *testing.T, dir string) {
				writeTestContext(t, dir, "desktop-linux", desktopHost, false)
				writeTestConfig(t, dir, "desktop-linux")
			},
			want: desktopHost,
		},
		{
			name: "DOCKER_HOST beats config.json",
			env:  map[string]string{"DOCKER_HOST": envHost},
			setup: func(t *testing.T, dir string) {
				writeTestContext(t, dir, "desktop-linux", desktopHost, false)
				writeTestConfig(t, dir, "desktop-linux")
			},
			want: envHost,
		},
		{
			name: "context flag beats host flag",
			opts: func(string) EndpointOptions {
				return EndpointOptions{Context: "desktop-linux", Host: remoteHost}
			},
			setup: func(t *testing.T, dir string) { writeTestContext(t, dir, "desktop-linux", desktopHost, false) },
			want:  desktopHost,
		},
		{
			name: "default context uses DOCKER_HOST",
			opts: func(string) EndpointOptions { return EndpointOptions{Context: "default"} },
			env:  map[string]string{"DOCKER_HOST": envHost},
			want: envHost,
		},
		{
			name:  "default current context",
			setup: func(t *testing.T, dir string) { writeTestConfig(t, dir, "default") },
			want:  defaultEndpoint,
		},
		{
			name: "context with tls material",
			opts: func(string) EndpointOptions { return EndpointOptions{Context: "secure"} },
			setup: func(t *testing.T, dir string) {
				writeTestContext(t, dir, "secure", "tcp://10.0.0.5:2376", true)
			},
			wantErr: true, unsupported: true,
		},
		{
			name:    "ssh host",
			opts:    func(string) EndpointOptions { return EndpointOptions{Host: "ssh://me@build-box"} },
			wantErr: true, unsupported: true,
		},
		{
			name:    "ssh context",
			opts:    func(string) EndpointOptions { return EndpointOptions{Context: "remote"} },
			setup:   func(t *testing.T, dir string) { writeTestContext(t, dir, "remote", "ssh://me@build-box", false) },
			wantErr: true, unsupported: true,
		},
		{
			name:    "tcp with DOCKER_TLS_VERIFY",
			env:     map[string]string{"DOCKER_HOST": "tcp://10.0.0.5:2376", "DOCKER_TLS_VERIFY": "1"},
			wantErr: true, unsupported: true,
		},
		{
			name:    "unknown context",
			opts:    func(string) EndpointOptions { return EndpointOptions{Context: "missing"} },
			wantErr: true,
		},
		{
			name:    "missing scheme",
			opts:    func(string) EndpointOptions { return EndpointOptions{Host: "localhost:2375"} },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := t.TempDir()
			for _, key := range []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS_VERIFY"} {
				t.Setenv(key, tt.env[key])
			}
			t.Setenv("DOCKER_CONFIG", configDir)
			if tt.setup != nil {
				tt.setup(t, configDir)
			}
			var opts EndpointOptions
			if tt.opts != nil {
				opts = tt.opts(configDir)
			}

			got, err := ResolveEndpoint(opts)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveEndpoint() = %q, want an error", got)
				}
				if errors.Is(err, ErrUnsupportedEndpoint) != tt.unsupported {
					t.Fatalf("ResolveEndpoint() error %v, unsupported = %v", err, tt.unsupported)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveEndpoint() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEndpointConfigDirFlag(t *testing.T) {
	flagDir := t.TempDir()
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	writeTestContext(t, flagDir, "lab", "tcp://127.0.0.1:2375", false)
	writeTestConfig(t, flagDir, "lab")

	got, err := ResolveEndpoint(EndpointOptions{ConfigDir: flagDir})
	if err != nil {
		t.Fatal(err)
	}
	if got != "tcp://127.0.0.1:2375" {
		t.Fatalf("ResolveEndpoint() = %q, want the context of --config", got)
	}
}

func TestNewEndpoints(t *testing.T) {
	tests := []struct {
		endpoint    string
		wantErr     bool
		unsupported bool
	}{
		{endpoint: "unix:///var/run/docker.sock"},
		{endpoint: "tcp://127.0.0.1:2375"},
		{endpoint: "npipe:////./pipe/docker_engine", wantErr: runtime.GOOS != "windows", unsupported: true},
		{endpoint: "ssh://me@host", wantErr: true, unsupported: true},
		{endpoint: "https://host:2376", wantErr: true, unsupported: true},
		{endpoint: "fd://", wantErr: true, unsupported: true},
		{endpoint: "tcp://no-port", wantErr: true},
		{endpoint: "unix://", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			c, err := New(tt.endpoint)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("New(%q) error: %v", tt.endpoint, err)
				}
				if c.Endpoint() != tt.endpoint {
					t.Fatalf("Endpoint() = %q", c.Endpoint())
				}
				c.Close()
				return
			}
			if err == nil {
				t.Fatalf("New(%q) succeeded, want an error", tt.endpoint)
			}
			if errors.Is(err, ErrUnsupportedEndpoint) != tt.unsupported {
				t.Fatalf("New(%q) error %v, unsupported = %v", tt.endpoint, err, tt.unsupported)
			}
		})
	}
}
