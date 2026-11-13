package docker

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// defaultEndpoint is the engine endpoint when nothing selects another one.
const defaultEndpoint = "npipe:////./pipe/docker_engine"

func pipeDialer(path string) (dialFunc, error) {
	return func(ctx context.Context) (net.Conn, error) {
		return winio.DialPipeContext(ctx, path)
	}, nil
}
