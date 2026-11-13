package docker

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// apiHost is the Host header sent over sockets and pipes, which have no host name of their own.
const apiHost = "api.moby.localhost"

// dialFunc opens one connection to the engine.
type dialFunc func(ctx context.Context) (net.Conn, error)

// dialerFor returns the dial function and the HTTP host for an endpoint.
func dialerFor(endpoint string) (dialFunc, string, error) {
	if err := validateEndpoint(endpoint); err != nil {
		return nil, "", err
	}
	scheme, rest, _ := strings.Cut(endpoint, "://")
	switch scheme {
	case "unix":
		return func(ctx context.Context) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", rest)
		}, apiHost, nil
	case "npipe":
		dial, err := pipeDialer(strings.ReplaceAll(rest, "/", `\`))
		if err != nil {
			return nil, "", err
		}
		return dial, apiHost, nil
	default: // tcp, the only other scheme validateEndpoint accepts
		address := strings.TrimSuffix(rest, "/")
		if _, _, err := net.SplitHostPort(address); err != nil {
			return nil, "", fmt.Errorf("parse docker endpoint %q: %w", endpoint, err)
		}
		return func(ctx context.Context) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", address)
		}, address, nil
	}
}
