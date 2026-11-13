package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

const (
	defaultInterval      = time.Second
	minCallTimeout       = 2 * time.Second
	noticeSamplerStopped = "sampler stopped; showing Engine API data"
)

// Options configures a Monitor.
type Options struct {
	// Interval is the polling interval; values <= 0 mean one second.
	Interval time.Duration
	// Now is the monitor clock; nil means time.Now.
	Now func() time.Time
	// Samples delivers sampler readings. Nil starts the monitor in model.ModeAPI. When the
	// channel closes, the monitor switches to model.ModeAPI for good.
	Samples <-chan model.Sample
	// Local says the samples come from the plugin reading /proc itself, not from the sampler
	// container: snapshots then report model.ModeLocal instead of model.ModeSampler.
	Local bool
	// Notice is shown in model.ModeAPI snapshots to say why the sampler is not used; may be
	// empty.
	Notice string
}

// Monitor polls an Engine and merges its readings with sampler samples into snapshots. Create it
// with New; call Run once.
type Monitor struct {
	engine      Engine
	interval    time.Duration
	callTimeout time.Duration
	now         func() time.Time
	out         chan model.Snapshot
	names       *userNames

	// The fields below belong to the goroutine that runs Run (or to a test calling tick).
	samples   <-chan model.Sample
	local     bool
	apiNotice string
	info      model.EngineInfo
	last      model.Snapshot
	stats     map[string]statsReading
	sampler   samplerHistory
	tops      map[int][]cpuReading
}

// New returns a Monitor reading from engine. It does no I/O; Run starts the polling.
func New(engine Engine, opts Options) *Monitor {
	interval := opts.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	callTimeout := max(interval, minCallTimeout)
	return &Monitor{
		engine:      engine,
		interval:    interval,
		callTimeout: callTimeout,
		now:         now,
		out:         make(chan model.Snapshot, 1),
		names:       newUserNames(engine, callTimeout),
		samples:     opts.Samples,
		local:       opts.Local,
		apiNotice:   opts.Notice,
		stats:       make(map[string]statsReading),
		tops:        make(map[int][]cpuReading),
	}
}

// Snapshots delivers the newest snapshot. It holds at most one pending snapshot: a slow reader
// misses intermediate ones, and the monitor never blocks on it. Run closes the channel when it
// returns.
func (m *Monitor) Snapshots() <-chan model.Snapshot {
	return m.out
}

// Run reads the engine info, takes a first tick at once and then one tick per interval until ctx
// is done, and returns nil then. It returns an error only when the engine info or the first
// container listing fails (engine unreachable). Every goroutine it starts has ended and the
// Snapshots channel is closed when it returns. Run must be called at most once.
func (m *Monitor) Run(ctx context.Context) error {
	defer close(m.out)

	info, err := m.engine.Info(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("read engine info: %w", err)
	}
	m.info = info

	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()
	wg.Go(func() { m.names.run(runCtx) })

	snapshot, err := m.tick(runCtx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	m.publish(snapshot)
	m.loop(runCtx)
	return nil
}

func (m *Monitor) loop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// A failed tick carries its error in the snapshot notice.
			snapshot, _ := m.tick(ctx)
			m.publish(snapshot)
		case sample, ok := <-m.samples:
			m.receive(sample, ok)
		}
	}
}

// receive records one sampler reading, or switches to API mode for good when the channel closed.
func (m *Monitor) receive(sample model.Sample, ok bool) {
	if !ok {
		m.samples = nil
		m.sampler = samplerHistory{}
		m.apiNotice = noticeSamplerStopped
		return
	}
	m.sampler.add(sample)
}

func (m *Monitor) mode() model.Mode {
	switch {
	case m.samples == nil:
		return model.ModeAPI
	case m.local:
		return model.ModeLocal
	default:
		return model.ModeSampler
	}
}

// publish replaces a pending snapshot the reader has not taken yet. Run is the only writer, so
// the loop ends after at most two rounds.
func (m *Monitor) publish(snapshot model.Snapshot) {
	for {
		select {
		case m.out <- snapshot:
			return
		default:
		}
		select {
		case <-m.out:
		default:
		}
	}
}
