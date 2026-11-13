package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Integration tests talk to the real engine of the current docker context. They create only the
// docker-htop-sampler image built from testdata and helper containers, and remove both.
func integrationClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("DOCKER_HTOP_INTEGRATION") != "1" {
		t.Skip("set DOCKER_HTOP_INTEGRATION=1 to run against the real Docker engine")
	}
	endpoint, err := ResolveEndpoint(EndpointOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestIntegrationReadOnly(t *testing.T) {
	c := integrationClient(t)
	ctx := t.Context()
	info, err := c.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("engine %s %s %s, %d CPUs, %d bytes", info.Name, info.Version, info.Architecture, info.CPUs, info.MemoryBytes)

	containers, err := c.Containers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(containers, func(ci model.ContainerInfo) bool { return ci.State == model.StateRunning })
	if i < 0 {
		t.Skip("no running container")
	}
	target := containers[i]

	stats, err := c.Stats(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: cpu %s, mem %d/%d, net %v %d/%d, pids %d", target.Name, stats.CPUTotal, stats.MemoryBytes,
		stats.MemoryLimitBytes, stats.HasNetwork, stats.NetRxBytes, stats.NetTxBytes, stats.PIDs)

	processes, err := c.Top(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) == 0 {
		t.Fatal("Top() returned no processes for a running container")
	}
	t.Logf("first process: %+v", processes[0])

	names, err := c.UserNames(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d user names", len(names))
}

// fakeSampler returns the linux/amd64 fake sampler: the file named by
// DOCKER_HTOP_FAKE_SAMPLER when set (for hosts without a Go toolchain), otherwise one compiled
// from testdata/fakesampler.
func fakeSampler(t *testing.T) []byte {
	t.Helper()
	if path := os.Getenv("DOCKER_HTOP_FAKE_SAMPLER"); path != "" {
		binary, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return binary
	}
	if os.Getenv("DOCKER_HTOP_BUILD_FAKE_SAMPLER") != "1" {
		t.Skip("set DOCKER_HTOP_FAKE_SAMPLER to a prebuilt testdata/fakesampler, or DOCKER_HTOP_BUILD_FAKE_SAMPLER=1 to compile it")
	}
	out := filepath.Join(t.TempDir(), "fakesampler")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", out, "./testdata/fakesampler")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake sampler: %v\n%s", err, output)
	}
	binary, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return binary
}

func TestIntegrationSamplerRoundTrip(t *testing.T) {
	c := integrationClient(t)
	binary := fakeSampler(t)

	ref, err := c.EnsureSamplerImage(t.Context(), binary, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		if err := c.removeImage(ctx, ref, true); err != nil {
			t.Errorf("remove test image: %v", err)
		}
	})
	if again, err := c.EnsureSamplerImage(t.Context(), binary, "amd64"); err != nil || again != ref {
		t.Fatalf("second EnsureSamplerImage() = %q, %v", again, err)
	}

	s, err := c.StartSampler(t.Context(), ref, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()
	var samples []model.Sample
	for len(samples) < 2 {
		select {
		case sample, ok := <-s.Samples():
			if !ok {
				t.Fatalf("stream ended after %d samples: %v", len(samples), s.Err())
			}
			samples = append(samples, sample)
		case <-timeout.C:
			t.Fatal("no samples within 30s")
		}
	}
	if samples[0].TimeUnixMs != 1 || samples[1].Tasks != 2 {
		t.Fatalf("samples = %+v", samples)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if err := s.Err(); err != nil {
		t.Fatalf("Err() after Close = %v", err)
	}

	err = c.SignalProcess(t.Context(), ref, strings.Repeat("0", 64), 1, model.SignalTerm)
	if err == nil || !strings.Contains(err.Error(), "fake signal helper refuses") || !strings.Contains(err.Error(), "code 3") {
		t.Fatalf("SignalProcess() error = %v, want the fake helper's refusal", err)
	}
}
