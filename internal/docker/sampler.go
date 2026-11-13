package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

var errSamplerClosed = errors.New("sampler closed")

// Sampler is a running sampler container and the stream of samples it writes. Its methods are
// safe for concurrent use.
type Sampler struct {
	client      *Client
	containerID string
	conn        net.Conn
	samples     chan model.Sample
	done        chan struct{} // closed by Close
	wg          sync.WaitGroup

	errMu sync.Mutex // guards err
	err   error

	closeOnce sync.Once
	closeErr  error
}

// StartSampler creates a sampler container from imageRef that shares the Docker host's PID
// namespace, attaches to it, starts it and decodes its stdout into samples written every
// interval. The container is created with StdinOnce, so the engine stops it when the plugin's
// connection drops, and with AutoRemove. If a step fails, everything created so far is removed
// and the error names the step (create, attach or start), so the caller can fall back to
// model.ModeAPI.
func (c *Client) StartSampler(ctx context.Context, imageRef string, interval time.Duration) (*Sampler, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("start sampler: interval %s is not positive", interval)
	}
	containerID, err := c.createContainer(ctx, helperName("docker-htop-sampler"), samplerSpec(imageRef, interval))
	if err != nil {
		return nil, fmt.Errorf("start sampler: %w", err)
	}
	conn, stream, err := c.attach(ctx, containerID)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("start sampler: %w", err), c.removeHelper(ctx, containerID))
	}
	if err := c.startContainer(ctx, containerID); err != nil {
		closeQuietly(conn)
		return nil, errors.Join(fmt.Errorf("start sampler: %w", err), c.removeHelper(ctx, containerID))
	}

	s := newSampler(c, containerID, conn)
	s.wg.Go(func() { s.read(stream) })
	return s, nil
}

func newSampler(c *Client, containerID string, conn net.Conn) *Sampler {
	return &Sampler{
		client:      c,
		containerID: containerID,
		conn:        conn,
		samples:     make(chan model.Sample, 1),
		done:        make(chan struct{}),
	}
}

// Samples returns the decoded samples in order. The channel is closed when the stream ends;
// Err then says why.
func (s *Sampler) Samples() <-chan model.Sample {
	return s.samples
}

// Err returns why the stream ended: nil after a clean Close, an error matching ErrSamplerExited
// when the sampler stopped on its own (with the tail of its stderr), or a read or decode error.
// It is valid once the Samples channel is closed and nil before.
func (s *Sampler) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// Close closes the connection, which also closes the sampler's stdin, waits for the reader and
// force-removes the container with a short timeout. It is idempotent and returns the first
// call's result.
func (s *Sampler) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		closeQuietly(s.conn)
		s.wg.Wait()
		s.closeErr = s.client.removeHelper(context.Background(), s.containerID)
	})
	return s.closeErr
}

// read demultiplexes the stream until it ends, sends each stdout line as a sample and records
// why the stream ended before closing Samples.
func (s *Sampler) read(stream io.Reader) {
	defer close(s.samples)
	stdout := &lineWriter{max: maxSampleLineBytes, emit: s.emit}
	stderr := &tailWriter{max: maxStderrTailBytes}
	err := demux(stream, stdout, stderr)

	select {
	case <-s.done:
		err = nil
	default:
		err = streamEndError(err, stderr.String())
	}
	s.errMu.Lock()
	s.err = err
	s.errMu.Unlock()
}

func (s *Sampler) emit(line []byte) error {
	var sample model.Sample
	if err := json.Unmarshal(line, &sample); err != nil {
		return fmt.Errorf("decode sample: %w", err)
	}
	select {
	case s.samples <- sample:
		return nil
	case <-s.done:
		return errSamplerClosed
	}
}

func streamEndError(err error, stderr string) error {
	if stderr == "" {
		stderr = "no output on stderr"
	}
	if err == nil {
		return fmt.Errorf("%w: %s", ErrSamplerExited, stderr)
	}
	return fmt.Errorf("read sampler stream: %w (stderr: %s)", err, stderr)
}
