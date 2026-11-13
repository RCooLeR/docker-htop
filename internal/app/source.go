package app

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/RCooLeR/docker-htop/internal/config"
	"github.com/RCooLeR/docker-htop/internal/docker"
	"github.com/RCooLeR/docker-htop/internal/model"
	"github.com/RCooLeR/docker-htop/internal/sampler"
	"github.com/RCooLeR/docker-htop/internal/samplerbin"
)

// Bounds for preparing a source: the helper image is loaded once per sampler version, so the
// first run may take a while; the other steps are quick.
const (
	helperImageTimeout  = 60 * time.Second
	samplerStartTimeout = 20 * time.Second
	localListingTimeout = 10 * time.Second
)

// procOperatingSystem is the only operating system whose /proc the sampler package can read.
const procOperatingSystem = "linux"

// Reasons for Engine API mode that do not come from an error.
const (
	noticeSamplerOff     = "sampler disabled by --no-sampler"
	noticeNoSamplerBuilt = "this build has no sampler binary (build with scripts/build.sh or scripts/build.ps1)"
)

// source is where the process and host readings of one run come from. A nil samples channel
// means Engine API mode, and notice says why.
type source struct {
	samples <-chan model.Sample
	// local is true when the plugin reads /proc itself instead of through the sampler container.
	local  bool
	notice string
	// image is the helper image for signalling a single process. Nil when helper containers
	// are switched off or this build cannot make the image.
	image *helperImage
	// stop ends whatever produces the samples and waits for it.
	stop func()
}

// openSource picks the best source the engine allows. It never fails: every obstacle turns into
// the next source, down to Engine API mode with a notice for the header.
//
// Reading /proc directly comes first, because it needs nothing from the engine. It works only
// where the plugin can see the containers' processes, which is not the case with Docker Desktop.
func openSource(ctx context.Context, client *docker.Client, info model.EngineInfo, cfg config.Config) source {
	if cfg.NoSampler {
		return source{notice: noticeSamplerOff, stop: func() {}}
	}
	image, imageNotice := newHelperImage(client, info)

	if samples, stop, ok := openLocal(ctx, client, info, cfg.Interval); ok {
		return source{samples: samples, local: true, image: image, stop: stop}
	}
	if image == nil {
		return source{notice: imageNotice, stop: func() {}}
	}
	return openSampler(ctx, client, image, cfg.Interval)
}

// openLocal starts reading /proc in this process when it shows the engine's containers.
func openLocal(ctx context.Context, client *docker.Client, info model.EngineInfo, interval time.Duration) (<-chan model.Sample, func(), bool) {
	if runtime.GOOS != procOperatingSystem {
		return nil, nil, false
	}
	reader := sampler.NewReader(sampler.DefaultProcRoot)
	sample, err := reader.Read()
	if err != nil {
		return nil, nil, false
	}
	listCtx, cancelList := context.WithTimeout(ctx, localListingTimeout)
	containers, err := client.Containers(listCtx)
	cancelList()
	if err != nil {
		return nil, nil, false
	}
	hostName, err := os.Hostname()
	if err != nil {
		hostName = ""
	}
	if !seesContainers(sample, containers, info.Name, hostName) {
		return nil, nil, false
	}

	samples := make(chan model.Sample, 1)
	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		defer close(samples)
		// A failed read ends the delivery; the closed channel makes the monitor fall back to
		// the Engine API and say so.
		_ = sampler.Deliver(runCtx, reader, samples, interval)
	})
	return samples, func() { cancel(); wg.Wait() }, true
}

// seesContainers reports whether the processes in sample, read from this machine's /proc, are
// the engine's: at least one running container has a process among them. A container ID cannot
// match by accident, so one hit is proof. While no container runs there is nothing to compare,
// and the engine's host name has to match this machine's.
func seesContainers(sample model.Sample, containers []model.ContainerInfo, engineName, hostName string) bool {
	running := make(map[string]bool)
	for _, c := range containers {
		if c.State == model.StateRunning {
			running[c.ID] = true
		}
	}
	if len(running) == 0 {
		return engineName != "" && strings.EqualFold(engineName, hostName)
	}
	for _, p := range sample.Processes {
		if running[p.ContainerID] {
			return true
		}
	}
	return false
}

// openSampler starts the sampler container, loading its image first if needed.
func openSampler(ctx context.Context, client *docker.Client, image *helperImage, interval time.Duration) source {
	imageCtx, cancelImage := context.WithTimeout(ctx, helperImageTimeout)
	imageRef, err := image.reference(imageCtx)
	cancelImage()
	if err != nil {
		return source{notice: fmt.Sprintf("sampler image: %v", err), image: image, stop: func() {}}
	}

	startCtx, cancelStart := context.WithTimeout(ctx, samplerStartTimeout)
	started, err := client.StartSampler(startCtx, imageRef, interval)
	cancelStart()
	if err != nil {
		return source{notice: fmt.Sprintf("sampler: %v", err), image: image, stop: func() {}}
	}
	return source{
		samples: started.Samples(),
		image:   image,
		stop: func() {
			// The sampler container removes itself when the connection drops; a failed
			// tidy-up here leaves nothing the user could act on.
			_ = started.Close()
		},
	}
}

// helperImage is the image of the sampler and signal helper containers. It is loaded into the
// engine on first use, so a run that needs no helper container creates nothing.
type helperImage struct {
	client *docker.Client
	binary []byte
	// architecture is the one binary was built for, in the OCI spelling.
	architecture string

	// mu guards ref and is held while the image is loaded, so that two requests arriving
	// together load it once.
	mu  sync.Mutex
	ref string
}

// newHelperImage returns nil and the reason when this build or this engine cannot use the image.
func newHelperImage(client *docker.Client, info model.EngineInfo) (*helperImage, string) {
	binary, ok := samplerbin.Binary()
	if !ok {
		return nil, noticeNoSamplerBuilt
	}
	if !samplerbin.SupportsEngine(info.Architecture) {
		return nil, fmt.Sprintf("this build's sampler is for %s engines (engine is %s)",
			cmp.Or(samplerbin.Architecture(), "other"), info.Architecture)
	}
	return &helperImage{client: client, binary: binary, architecture: samplerbin.Architecture()}, ""
}

// reference returns the image reference, loading the image into the engine if it is missing.
func (h *helperImage) reference(ctx context.Context) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ref != "" {
		return h.ref, nil
	}
	ref, err := h.client.EnsureSamplerImage(ctx, h.binary, h.architecture)
	if err != nil {
		return "", err
	}
	h.ref = ref
	return ref, nil
}
