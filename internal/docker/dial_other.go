//go:build !windows

package docker

import "fmt"

// defaultEndpoint is the engine endpoint when nothing selects another one.
const defaultEndpoint = "unix:///var/run/docker.sock"

func pipeDialer(path string) (dialFunc, error) {
	return nil, fmt.Errorf("%w: named pipe %q outside windows", ErrUnsupportedEndpoint, path)
}
