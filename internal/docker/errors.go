package docker

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	// ErrNotFound matches an engine answer of 404: no such container, image or path.
	ErrNotFound = errors.New("not found")
	// ErrUnsupportedEndpoint matches an endpoint the client cannot talk to: TLS, SSH, or an
	// unknown scheme. The wrapping error names the endpoint kind.
	ErrUnsupportedEndpoint = errors.New("unsupported docker endpoint")
	// ErrSamplerExited matches a sampler stream that ended without Close being called.
	ErrSamplerExited = errors.New("sampler exited")
)

// apiError is a non-2xx answer of the engine.
type apiError struct {
	Method     string
	Path       string
	StatusCode int
	Message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("engine api %s %s: status %d: %s", e.Method, e.Path, e.StatusCode, e.Message)
}

// Is reports ErrNotFound for a 404 answer.
func (e *apiError) Is(target error) bool {
	return target == ErrNotFound && e.StatusCode == http.StatusNotFound
}

// hasStatus reports whether err is an engine answer with the given status code.
func hasStatus(err error, statusCode int) bool {
	apiErr, ok := errors.AsType[*apiError](err)
	return ok && apiErr.StatusCode == statusCode
}
