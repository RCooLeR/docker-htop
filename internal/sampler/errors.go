package sampler

import "errors"

var (
	// ErrMalformed means a procfs file did not have the expected shape.
	ErrMalformed = errors.New("malformed procfs data")
	// ErrFileTooLarge means a procfs file was larger than the sampler is willing to read.
	ErrFileTooLarge = errors.New("file too large")
	// ErrInvalidInterval means a stream interval is below MinInterval.
	ErrInvalidInterval = errors.New("invalid interval")
	// ErrInvalidPID means a PID cannot be signalled: it is 1 or lower.
	ErrInvalidPID = errors.New("invalid pid")
	// ErrInvalidContainerID means a container ID is not 64 lowercase hex characters.
	ErrInvalidContainerID = errors.New("invalid container id")
	// ErrUnknownSignal means a signal is not one of model.Signals.
	ErrUnknownSignal = errors.New("unknown signal")
	// ErrNotInContainer means a process does not belong to the container it was addressed by.
	ErrNotInContainer = errors.New("process does not belong to the container")
	// ErrUnsupported means the operation needs Linux.
	ErrUnsupported = errors.New("unsupported on this operating system")
)
