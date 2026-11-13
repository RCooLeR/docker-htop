package sampler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// MinInterval is the shortest interval Stream and Deliver accept.
const MinInterval = 200 * time.Millisecond

// Stream writes one Sample per interval to w as a single JSON line, the first one immediately,
// until ctx is done, and then returns nil. When w has a Flush() error method (a bufio.Writer), it
// is flushed after every line. A failed read or write ends the stream with that error. An
// interval below MinInterval fails with ErrInvalidInterval.
func Stream(ctx context.Context, r *Reader, w io.Writer, interval time.Duration) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	flusher, _ := w.(interface{ Flush() error })
	return every(ctx, interval, func() error { return writeSample(r, enc, flusher) })
}

// Deliver sends one Sample per interval to out, the first one immediately, until ctx is done,
// and then returns nil. It is Stream for a reader in the same process as the consumer. A send
// waits for the receiver. A failed read ends the delivery with that error, and an interval below
// MinInterval fails with ErrInvalidInterval. Deliver never closes out.
func Deliver(ctx context.Context, r *Reader, out chan<- model.Sample, interval time.Duration) error {
	return every(ctx, interval, func() error {
		sample, err := r.Read()
		if err != nil {
			return err
		}
		select {
		case out <- sample:
		case <-ctx.Done():
		}
		return nil
	})
}

// every calls step at once and then once per interval until ctx is done or step fails.
func every(ctx context.Context, interval time.Duration, step func() error) error {
	if interval < MinInterval {
		return fmt.Errorf("stream every %v: %w", interval, ErrInvalidInterval)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := step(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func writeSample(r *Reader, enc *json.Encoder, flusher interface{ Flush() error }) error {
	sample, err := r.Read()
	if err != nil {
		return err
	}
	if err := enc.Encode(sample); err != nil {
		return fmt.Errorf("write sample: %w", err)
	}
	if flusher != nil {
		if err := flusher.Flush(); err != nil {
			return fmt.Errorf("flush sample: %w", err)
		}
	}
	return nil
}
