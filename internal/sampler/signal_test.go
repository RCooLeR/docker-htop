package sampler

import (
	"errors"
	"io/fs"
	"runtime"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestSignalRefuses(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		pid         int
		sig         model.Signal
		wantErr     error
	}{
		{"pid 1", fixtureID1, 1, model.SignalTerm, ErrInvalidPID},
		{"pid 0", fixtureID1, 0, model.SignalTerm, ErrInvalidPID},
		{"short container id", fixtureID1[:12], 100, model.SignalTerm, ErrInvalidContainerID},
		{"unknown signal", fixtureID1, 100, "SIGSEGV", ErrUnknownSignal},
		{"other container", fixtureID1, 200, model.SignalTerm, ErrNotInContainer},
		{"outside any container", fixtureID1, 2, model.SignalKill, ErrNotInContainer},
		{"vanished", fixtureID1, 500, model.SignalTerm, fs.ErrNotExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Signal(fixtureRoot, tt.containerID, tt.pid, tt.sig)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Signal error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignalUnsupported(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("on Linux a matching fixture would signal a real host process")
	}
	if err := Signal(fixtureRoot, fixtureID1, 100, model.SignalTerm); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Signal error = %v, want ErrUnsupported", err)
	}
}
