package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// maxPasswdBytes caps what is read of a container's /etc/passwd.
	maxPasswdBytes = 1 << 20
	// tarOverheadBytes leaves room for the tar headers around the file.
	tarOverheadBytes = 8 << 10
)

// UserNames maps UIDs to user names from the container's own /etc/passwd
// (GET /containers/{id}/archive). A missing file, a non-regular entry (a symlink, for example)
// or any 404 answer gives an empty map and a nil error. The first name wins per UID.
func (c *Client) UserNames(ctx context.Context, containerID string) (map[uint32]string, error) {
	query := url.Values{"path": {"/etc/passwd"}}
	resp, err := c.do(ctx, http.MethodGet, containerPath(containerID, "/archive"), query, nil, "")
	if errors.Is(err, ErrNotFound) {
		return map[uint32]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read /etc/passwd of %s: %w", containerID, err)
	}
	defer closeQuietly(resp.Body)
	names, err := parsePasswdTar(io.LimitReader(resp.Body, maxPasswdBytes+tarOverheadBytes))
	if err != nil {
		return nil, fmt.Errorf("read /etc/passwd of %s: %w", containerID, err)
	}
	return names, nil
}

func parsePasswdTar(r io.Reader) (map[uint32]string, error) {
	tr := tar.NewReader(r)
	header, err := tr.Next()
	if errors.Is(err, io.EOF) {
		return map[uint32]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	if header.Typeflag != tar.TypeReg {
		return map[uint32]string{}, nil
	}
	data, err := io.ReadAll(io.LimitReader(tr, maxPasswdBytes))
	if err != nil {
		return nil, fmt.Errorf("read archive entry %s: %w", header.Name, err)
	}
	return parsePasswd(data), nil
}

// parsePasswd reads name:password:uid:... lines and skips malformed ones.
func parsePasswd(data []byte) map[uint32]string {
	names := map[uint32]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 4096), maxPasswdBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, ":", 4)
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		uid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		if _, seen := names[uint32(uid)]; !seen {
			names[uint32(uid)] = fields[0]
		}
	}
	return names
}
