package sampler

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Signal sends sig to the host process pid after checking that /proc/<pid>/cgroup under procRoot
// names containerID, which guards against a reused PID and against processes outside that
// container. It fails with ErrInvalidPID for pid <= 1, ErrInvalidContainerID unless containerID
// is 64 lowercase hex characters, ErrUnknownSignal for a signal outside model.Signals,
// ErrNotInContainer when the cgroup does not match, an fs.ErrNotExist error when the process is
// gone (an os.ErrProcessDone error when it exits after the check), and ErrUnsupported on
// operating systems other than Linux.
func Signal(procRoot, containerID string, pid int, sig model.Signal) error {
	if pid <= 1 {
		return fmt.Errorf("signal pid %d: %w", pid, ErrInvalidPID)
	}
	if !isContainerID(containerID) {
		return fmt.Errorf("signal pid %d: container %q: %w", pid, containerID, ErrInvalidContainerID)
	}
	if !slices.Contains(model.Signals(), sig) {
		return fmt.Errorf("signal pid %d: %q: %w", pid, sig, ErrUnknownSignal)
	}

	return signalProcess(procRoot, containerID, pid, sig)
}

// checkCgroup verifies that /proc/<pid>/cgroup names containerID.
func checkCgroup(procRoot, containerID string, pid int) error {
	path := procRoot + "/" + strconv.Itoa(pid) + "/cgroup"
	data, truncated, err := readUpTo(path, nil, cgroupLimit)
	if err != nil {
		return fmt.Errorf("signal pid %d: read %s: %w", pid, path, err)
	}
	if truncated {
		return fmt.Errorf("signal pid %d: read %s: %w", pid, path, ErrFileTooLarge)
	}
	if !cgroupHasContainer(data, containerID) {
		return fmt.Errorf("signal pid %d: container %s: %w", pid, containerID, ErrNotInContainer)
	}
	return nil
}
