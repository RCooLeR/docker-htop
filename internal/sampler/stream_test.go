package sampler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// lineWriter records what was flushed, so the test sees only complete flushed output.
type lineWriter struct {
	pending bytes.Buffer
	flushed bytes.Buffer
	flushes int
	failAt  int // fail the n-th write when > 0
	writes  int
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errors.New("broken pipe")
	}
	return w.pending.Write(p)
}

func (w *lineWriter) Flush() error {
	w.flushes++
	_, err := w.pending.WriteTo(&w.flushed)
	return err
}

func TestStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		w := &lineWriter{}
		done := make(chan error, 1)
		go func() { done <- Stream(ctx, newTestReader(fixtureRoot), w, time.Second) }()

		synctest.Wait()
		checkLines(t, w, 1)
		time.Sleep(time.Second)
		synctest.Wait()
		checkLines(t, w, 2)
		time.Sleep(2 * time.Second)
		synctest.Wait()
		checkLines(t, w, 4)

		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Stream after cancel: %v", err)
		}
		checkLines(t, w, 4)
	})
}

func checkLines(t *testing.T, w *lineWriter, want int) {
	t.Helper()
	if w.pending.Len() != 0 {
		t.Errorf("%d unflushed bytes", w.pending.Len())
	}
	if w.flushes != want {
		t.Errorf("flushes = %d, want %d", w.flushes, want)
	}
	data := w.flushed.Bytes()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("output does not end with a newline: %q", data)
	}
	lines := bytes.Split(data[:len(data)-1], []byte("\n"))
	if len(lines) != want {
		t.Fatalf("%d lines, want %d", len(lines), want)
	}
	for i, line := range lines {
		var s model.Sample
		if err := json.Unmarshal(line, &s); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if len(s.Processes) != 4 || len(s.CPUs) != 2 {
			t.Errorf("line %d: %d processes, %d cpus", i, len(s.Processes), len(s.CPUs))
		}
	}
}

func TestStreamWriteFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &lineWriter{failAt: 2}
		err := Stream(t.Context(), newTestReader(fixtureRoot), w, time.Second)
		if err == nil || w.flushes != 1 {
			t.Errorf("Stream = %v after %d flushes, want an error after 1", err, w.flushes)
		}
	})
}

func TestDeliver(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		out := make(chan model.Sample)
		done := make(chan error, 1)
		go func() { done <- Deliver(ctx, newTestReader(fixtureRoot), out, time.Second) }()

		start := time.Now()
		for i := range 3 {
			s := <-out
			if len(s.Processes) != 4 || len(s.CPUs) != 2 {
				t.Errorf("sample %d: %d processes, %d cpus", i, len(s.Processes), len(s.CPUs))
			}
		}
		if elapsed := time.Since(start); elapsed != 2*time.Second {
			t.Errorf("three samples took %v, want 2s", elapsed)
		}

		// The fourth sample is waiting for a receiver; cancelling must release the send.
		time.Sleep(time.Second)
		synctest.Wait()
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Deliver after cancel: %v", err)
		}
	})
}

func TestDeliverReadFailure(t *testing.T) {
	err := Deliver(t.Context(), newTestReader(t.TempDir()), make(chan model.Sample, 1), time.Second)
	if err == nil {
		t.Error("Deliver over an empty proc root succeeded")
	}
}

func TestStreamReadFailure(t *testing.T) {
	err := Stream(t.Context(), newTestReader(t.TempDir()), &lineWriter{}, time.Second)
	if err == nil {
		t.Error("Stream over an empty proc root succeeded")
	}
}

func TestStreamInvalidInterval(t *testing.T) {
	err := Stream(t.Context(), newTestReader(fixtureRoot), &lineWriter{}, MinInterval-1)
	if !errors.Is(err, ErrInvalidInterval) {
		t.Errorf("error = %v, want ErrInvalidInterval", err)
	}
}
