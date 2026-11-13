package sampler

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func signalProcess(procRoot, containerID string, pid int, sig model.Signal) error {
	num, err := signalNumber(sig)
	if err != nil {
		return err
	}
	// Take the handle (a pidfd where the kernel has one) before the cgroup check, so a PID reused
	// after the check cannot receive the signal.
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("signal pid %d: find process: %w", pid, err)
	}
	defer func() {
		_ = process.Release() // only frees the handle; nothing to recover from
	}()

	if err := checkCgroup(procRoot, containerID, pid); err != nil {
		return err
	}
	if err := process.Signal(num); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("process %d has already exited: %w", pid, err)
		}
		return fmt.Errorf("send %s to pid %d: %w", sig, pid, err)
	}
	return nil
}

func signalNumber(sig model.Signal) (syscall.Signal, error) {
	switch sig {
	case model.SignalTerm:
		return syscall.SIGTERM, nil
	case model.SignalKill:
		return syscall.SIGKILL, nil
	case model.SignalInt:
		return syscall.SIGINT, nil
	case model.SignalHup:
		return syscall.SIGHUP, nil
	case model.SignalQuit:
		return syscall.SIGQUIT, nil
	case model.SignalUsr1:
		return syscall.SIGUSR1, nil
	case model.SignalUsr2:
		return syscall.SIGUSR2, nil
	case model.SignalStop:
		return syscall.SIGSTOP, nil
	case model.SignalCont:
		return syscall.SIGCONT, nil
	}
	return 0, fmt.Errorf("%q: %w", sig, ErrUnknownSignal)
}
