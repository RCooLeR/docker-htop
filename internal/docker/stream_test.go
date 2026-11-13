package docker

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func joinFrames(frames ...[]byte) []byte {
	return bytes.Join(frames, nil)
}

func TestDemux(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		wantStdout string
		wantStderr string
		wantErr    string
	}{
		{
			name:       "interleaved",
			input:      joinFrames(frame(streamStdout, "out1 "), frame(streamStderr, "err1"), frame(streamStdout, "out2")),
			wantStdout: "out1 out2",
			wantStderr: "err1",
		},
		{name: "empty stream"},
		{
			name:       "empty frame",
			input:      joinFrames(frame(streamStdout, ""), frame(streamStdout, "x")),
			wantStdout: "x",
		},
		{
			name:       "truncated payload",
			input:      frame(streamStdout, "hello")[:10],
			wantStdout: "he",
			wantErr:    "unexpected EOF",
		},
		{
			name:    "truncated header",
			input:   []byte{1, 0, 0},
			wantErr: "frame header",
		},
		{
			name:    "engine error frame",
			input:   frame(streamSystem, "container stopped\n"),
			wantErr: "engine stream error: container stopped",
		},
		{
			name:    "unknown stream",
			input:   frame(9, "x"),
			wantErr: "unknown stream type 9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := demux(bytes.NewReader(tt.input), &stdout, &stderr)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("demux() error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("demux() error = %v, want %q", err, tt.wantErr)
			}
			if stdout.String() != tt.wantStdout || stderr.String() != tt.wantStderr {
				t.Fatalf("stdout %q stderr %q, want %q and %q", stdout.String(), stderr.String(), tt.wantStdout, tt.wantStderr)
			}
		})
	}
}

func TestLineWriter(t *testing.T) {
	var lines []string
	w := &lineWriter{max: 10, emit: func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	}}
	for _, chunk := range []string{"ab", "c\nde", "f\r\n\n", "g"} {
		if _, err := io.WriteString(w, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(lines, "|") != "abc|def" {
		t.Fatalf("lines = %q", lines)
	}
	if _, err := io.WriteString(w, "0123456789"); !errors.Is(err, errLineTooLong) {
		t.Fatalf("long line error = %v, want errLineTooLong", err)
	}
}

func TestTailAndCappedWriter(t *testing.T) {
	tail := &tailWriter{max: 5}
	capped := &cappedWriter{max: 5}
	for _, chunk := range []string{"abc", "defg", "hi"} {
		_, _ = io.WriteString(tail, chunk)
		_, _ = io.WriteString(capped, chunk)
	}
	if tail.String() != "efghi" {
		t.Fatalf("tail = %q, want efghi", tail.String())
	}
	if capped.String() != "abcde" {
		t.Fatalf("capped = %q, want abcde", capped.String())
	}
	_, _ = io.WriteString(tail, "0123456789")
	if tail.String() != "56789" {
		t.Fatalf("tail after a long write = %q", tail.String())
	}
}

const (
	sampleLine1 = `{"time_unix_ms":1762768800000,"cpus":[{"total_ms":1000,"idle_ms":900},{"total_ms":1000,"idle_ms":500}],` +
		`"memory":{"total_bytes":16624349184,"available_bytes":8000000000},"load":[0.5,0.4,0.3],"tasks":120,"threads":800,` +
		`"processes":[{"pid":1916,"ppid":1457,"uid":10001,"state":"S","threads":13,"start_ms":5000,"cpu_time_ms":750,` +
		`"rss_bytes":38117376,"command":"alpha-worker serve","container_id":"c0ffee123456"}]}` + "\n"
	sampleLine2 = `{"time_unix_ms":1762768801000,"cpus":[],"tasks":121}` + "\n"
)

func readAll(s *Sampler) []model.Sample {
	var samples []model.Sample
	for sample := range s.Samples() {
		samples = append(samples, sample)
	}
	return samples
}

func TestSamplerReadStreamEnd(t *testing.T) {
	stream := joinFrames(
		frame(streamStdout, sampleLine1[:40]),
		frame(streamStderr, "warning: no cgroup for pid 7\n"),
		frame(streamStdout, sampleLine1[40:]+sampleLine2),
		frame(streamStderr, "stdin closed, exiting\n"),
	)
	s := newSampler(nil, "abc", nil)
	go s.read(bytes.NewReader(stream))

	samples := readAll(s)
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2", len(samples))
	}
	first := samples[0]
	if first.TimeUnixMs != 1762768800000 || len(first.CPUs) != 2 || first.Tasks != 120 ||
		len(first.Processes) != 1 || first.Processes[0].Command != "alpha-worker serve" {
		t.Fatalf("first sample = %+v", first)
	}
	if samples[1].Tasks != 121 {
		t.Fatalf("second sample = %+v", samples[1])
	}
	err := s.Err()
	if !errors.Is(err, ErrSamplerExited) || !strings.Contains(err.Error(), "stdin closed, exiting") {
		t.Fatalf("Err() = %v, want ErrSamplerExited with the stderr tail", err)
	}
}

func TestSamplerReadMalformedLine(t *testing.T) {
	stream := joinFrames(
		frame(streamStdout, sampleLine2),
		frame(streamStderr, "panic: oops"),
		frame(streamStdout, "not json\n"),
		frame(streamStdout, sampleLine2),
	)
	s := newSampler(nil, "abc", nil)
	go s.read(bytes.NewReader(stream))

	if samples := readAll(s); len(samples) != 1 {
		t.Fatalf("got %d samples, want only the one before the malformed line", len(samples))
	}
	err := s.Err()
	if err == nil || !strings.Contains(err.Error(), "decode sample") || !strings.Contains(err.Error(), "panic: oops") {
		t.Fatalf("Err() = %v, want a decode error with the stderr tail", err)
	}
	if errors.Is(err, ErrSamplerExited) {
		t.Fatalf("Err() = %v matches ErrSamplerExited", err)
	}
}
