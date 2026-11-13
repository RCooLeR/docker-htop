package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// maxAPIVersion is the newest Engine API version this code was written against.
	maxAPIVersion = "1.47"
	// requestTimeout bounds every non-streaming request, body included.
	requestTimeout = 30 * time.Second
	// maxResponseBytes caps a JSON answer.
	maxResponseBytes = 16 << 20
	// maxErrorBytes caps the body read from an error answer.
	maxErrorBytes = 64 << 10
)

// Client talks to one Docker engine. It is safe for concurrent use.
type Client struct {
	endpoint  string
	baseURL   string
	dial      dialFunc
	transport *http.Transport
	http      *http.Client

	versionMu  sync.Mutex // guards apiVersion
	apiVersion string     // empty until the first successful ping
}

// New returns a client for endpoint, for example "unix:///var/run/docker.sock",
// "npipe:////./pipe/docker_engine" or "tcp://127.0.0.1:2375". It does not contact the engine;
// the first request does. TLS, SSH and unknown schemes give an error matching
// ErrUnsupportedEndpoint.
func New(endpoint string) (*Client, error) {
	dial, host, err := dialerFor(endpoint)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx)
		},
		MaxIdleConns:       8,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	return &Client{
		endpoint:  endpoint,
		baseURL:   "http://" + host,
		dial:      dial,
		transport: transport,
		http:      &http.Client{Transport: transport},
	}, nil
}

// Endpoint returns the endpoint the client was created with.
func (c *Client) Endpoint() string {
	return c.endpoint
}

// Close closes idle connections. Running samplers keep their own connections until their Close.
func (c *Client) Close() {
	c.transport.CloseIdleConnections()
}

// version returns the API version to use, asking the engine once. A failed ping is not cached.
func (c *Client) version(ctx context.Context) (string, error) {
	c.versionMu.Lock()
	version := c.apiVersion
	c.versionMu.Unlock()
	if version != "" {
		return version, nil
	}

	version, err := c.ping(ctx)
	if err != nil {
		return "", err
	}
	c.versionMu.Lock()
	c.apiVersion = version
	c.versionMu.Unlock()
	return version, nil
}

func (c *Client) ping(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/_ping", nil)
	if err != nil {
		return "", fmt.Errorf("ping engine at %s: %w", c.endpoint, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ping engine at %s: %w", c.endpoint, err)
	}
	defer closeQuietly(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("ping engine at %s: %w", c.endpoint, readAPIError(http.MethodGet, "/_ping", resp))
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBytes)); err != nil {
		return "", fmt.Errorf("ping engine at %s: %w", c.endpoint, err)
	}
	return negotiateVersion(resp.Header.Get("Api-Version")), nil
}

// negotiateVersion picks the lower of the server's version and maxAPIVersion.
func negotiateVersion(server string) string {
	if server == "" || !versionLess(server, maxAPIVersion) {
		return maxAPIVersion
	}
	return server
}

// versionLess compares "major.minor" versions numerically; unparsable parts count as 0.
func versionLess(a, b string) bool {
	aMajor, aMinor := splitVersion(a)
	bMajor, bMinor := splitVersion(b)
	if aMajor != bMajor {
		return aMajor < bMajor
	}
	return aMinor < bMinor
}

func splitVersion(v string) (int, int) {
	majorText, minorText, _ := strings.Cut(v, ".")
	major, _ := strconv.Atoi(majorText) // a malformed part sorts as 0, the oldest
	minor, _ := strconv.Atoi(minorText)
	return major, minor
}

// requestURL builds the versioned URL for path.
func (c *Client) requestURL(version, path string, query url.Values) string {
	u := c.baseURL + "/v" + version + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// do sends one request with requestTimeout. An answer of 400 or more becomes an *apiError. On
// success the caller closes the body, which also releases the timeout.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	version, err := c.version(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	req, err := http.NewRequestWithContext(ctx, method, c.requestURL(version, path, query), body)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer cancel()
		defer closeQuietly(resp.Body)
		return nil, readAPIError(method, path, resp)
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// getJSON sends a GET and decodes the JSON answer into out.
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, query, nil, "")
	if err != nil {
		return err
	}
	defer closeQuietly(resp.Body)
	return decodeJSON(resp.Body, out)
}

// postJSON sends a POST with in as JSON body (none when nil) and decodes the answer into out
// (discarded when nil).
func (c *Client) postJSON(ctx context.Context, path string, query url.Values, in, out any) error {
	var body io.Reader
	contentType := ""
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode %s body: %w", path, err)
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	}
	resp, err := c.do(ctx, http.MethodPost, path, query, body, contentType)
	if err != nil {
		return err
	}
	defer closeQuietly(resp.Body)
	if out == nil {
		_, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return err
	}
	return decodeJSON(resp.Body, out)
}

func decodeJSON(r io.Reader, out any) error {
	if err := json.NewDecoder(io.LimitReader(r, maxResponseBytes)).Decode(out); err != nil {
		return fmt.Errorf("decode engine answer: %w", err)
	}
	return nil
}

func readAPIError(method, path string, resp *http.Response) error {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
	message := strings.TrimSpace(string(data))
	var body struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &body) == nil && body.Message != "" {
		message = body.Message
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return &apiError{Method: method, Path: path, StatusCode: resp.StatusCode, Message: message}
}

// containerPath builds an escaped /containers/{id}/... path.
func containerPath(containerID, suffix string) string {
	return "/containers/" + url.PathEscape(containerID) + suffix
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}

// closeQuietly closes a body or connection that is fully read or no longer needed; a close error
// carries nothing the caller could act on.
func closeQuietly(c io.Closer) {
	_ = c.Close()
}
