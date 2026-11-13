package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// attach opens a hijacked connection to a container's stdin, stdout and stderr
// (POST /containers/{id}/attach with Upgrade: tcp). The returned reader holds the bytes already
// buffered after the response head; read the stream from it and close the connection when done.
func (c *Client) attach(ctx context.Context, containerID string) (net.Conn, *bufio.Reader, error) {
	version, err := c.version(ctx)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	conn, err := c.dial(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("dial engine at %s: %w", c.endpoint, err)
	}
	stopWatch := context.AfterFunc(ctx, func() { closeQuietly(conn) })
	br, err := c.attachHandshake(ctx, conn, version, containerID)
	if !stopWatch() {
		closeQuietly(conn)
		return nil, nil, fmt.Errorf("attach to %s: %w", containerID, errors.Join(ctx.Err(), err))
	}
	if err != nil {
		closeQuietly(conn)
		return nil, nil, fmt.Errorf("attach to %s: %w", containerID, err)
	}
	return conn, br, nil
}

func (c *Client) attachHandshake(ctx context.Context, conn net.Conn, version, containerID string) (*bufio.Reader, error) {
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set handshake deadline: %w", err)
		}
	}
	path := containerPath(containerID, "/attach")
	query := url.Values{"stream": {"1"}, "stdin": {"1"}, "stdout": {"1"}, "stderr": {"1"}}
	req, err := http.NewRequest(http.MethodPost, c.requestURL(version, path, query), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("write attach request: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, fmt.Errorf("read attach response: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		defer closeQuietly(resp.Body)
		return nil, readAPIError(http.MethodPost, path, resp)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear handshake deadline: %w", err)
	}
	return br, nil
}
