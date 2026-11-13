package docker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Stream types of the engine's multiplexed stream header.
const (
	streamStdin    = 0
	streamStdout   = 1
	streamStderr   = 2
	streamSystem   = 3
	frameHeaderLen = 8
)

const (
	// maxSampleLineBytes caps one JSON line of the sampler.
	maxSampleLineBytes = 8 << 20
	// maxStderrTailBytes is how much of the end of the sampler's stderr is kept for errors.
	maxStderrTailBytes = 4 << 10
	// maxLogBytes caps the helper output carried in an error.
	maxLogBytes = 4 << 10
	// maxSystemFrameBytes caps an engine error frame.
	maxSystemFrameBytes = 64 << 10
)

var errLineTooLong = errors.New("line too long")

// demux splits the engine's multiplexed stream (8-byte header: stream type, three zero bytes,
// big-endian payload size) into stdout and stderr until r ends. A clean end between frames
// returns nil.
func demux(r io.Reader, stdout, stderr io.Writer) error {
	var header [frameHeaderLen]byte
	for {
		if _, err := io.ReadFull(r, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read frame header: %w", err)
		}
		size := int64(binary.BigEndian.Uint32(header[4:]))
		var w io.Writer
		switch header[0] {
		case streamStdin, streamStdout:
			w = stdout
		case streamStderr:
			w = stderr
		case streamSystem:
			message, err := io.ReadAll(io.LimitReader(r, min(size, maxSystemFrameBytes)))
			if err != nil {
				return fmt.Errorf("read engine error frame: %w", err)
			}
			return fmt.Errorf("engine stream error: %s", bytes.TrimSpace(message))
		default:
			return fmt.Errorf("read frame: unknown stream type %d", header[0])
		}
		if _, err := io.CopyN(w, r, size); err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("read frame payload: %w", err)
		}
	}
}

// lineWriter calls emit for every complete, non-empty line written to it, without the line
// ending. emit must not keep the slice. A line longer than max fails the write.
type lineWriter struct {
	buf  []byte
	max  int
	emit func(line []byte) error
}

func (w *lineWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if len(w.buf)+len(p) > w.max {
				return 0, errLineTooLong
			}
			w.buf = append(w.buf, p...)
			break
		}
		line := p[:i]
		if len(w.buf) > 0 {
			if len(w.buf)+len(line) > w.max {
				return 0, errLineTooLong
			}
			w.buf = append(w.buf, line...)
			line = w.buf
		}
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			if err := w.emit(line); err != nil {
				return 0, err
			}
		}
		w.buf = w.buf[:0]
		p = p[i+1:]
	}
	return n, nil
}

// tailWriter keeps the last max bytes written to it.
type tailWriter struct {
	buf []byte
	max int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) >= w.max {
		w.buf = append(w.buf[:0], p[len(p)-w.max:]...)
		return n, nil
	}
	if overflow := len(w.buf) + len(p) - w.max; overflow > 0 {
		w.buf = append(w.buf[:0], w.buf[overflow:]...)
	}
	w.buf = append(w.buf, p...)
	return n, nil
}

func (w *tailWriter) String() string {
	return string(bytes.TrimSpace(w.buf))
}

// cappedWriter keeps the first max bytes written to it and discards the rest.
type cappedWriter struct {
	buf []byte
	max int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if room := w.max - len(w.buf); room > 0 {
		w.buf = append(w.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (w *cappedWriter) String() string {
	return string(bytes.TrimSpace(w.buf))
}
