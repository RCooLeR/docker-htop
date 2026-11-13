//go:build !linux

package sampler

import "github.com/RCooLeR/docker-htop/internal/model"

func signalProcess(procRoot, containerID string, pid int, _ model.Signal) error {
	if err := checkCgroup(procRoot, containerID, pid); err != nil {
		return err
	}
	return ErrUnsupported
}
