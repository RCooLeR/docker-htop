package docker

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// fakeAPIVersion is what the fake engine announces; lower than maxAPIVersion on purpose.
const fakeAPIVersion = "1.45"

type recordedRequest struct {
	Method string
	Path   string // without the version prefix
	Query  url.Values
	Body   []byte
}

// fakeEngine is an httptest server that answers /_ping, requires the version prefix and routes
// "METHOD /path" to handlers. Unknown routes answer 404.
type fakeEngine struct {
	server *httptest.Server
	routes map[string]http.HandlerFunc

	mu       sync.Mutex // guards requests
	requests []recordedRequest
}

func newFakeEngine(t *testing.T, routes map[string]http.HandlerFunc) (*fakeEngine, *Client) {
	t.Helper()
	engine := &fakeEngine{routes: routes}
	engine.server = httptest.NewServer(http.HandlerFunc(engine.serve))
	t.Cleanup(engine.server.Close)

	c, err := New("tcp://" + engine.server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return engine, c
}

func (e *fakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/_ping" {
		w.Header().Set("Api-Version", fakeAPIVersion)
		_, _ = io.WriteString(w, "OK")
		return
	}
	path, ok := strings.CutPrefix(r.URL.Path, "/v"+fakeAPIVersion)
	if !ok {
		writeEngineError(w, http.StatusBadRequest, "unversioned path "+r.URL.Path)
		return
	}
	body, _ := io.ReadAll(r.Body)
	e.mu.Lock()
	e.requests = append(e.requests, recordedRequest{Method: r.Method, Path: path, Query: r.URL.Query(), Body: body})
	e.mu.Unlock()

	handler, ok := e.routes[r.Method+" "+path]
	if !ok {
		writeEngineError(w, http.StatusNotFound, "no such route "+r.Method+" "+path)
		return
	}
	handler(w, r)
}

func (e *fakeEngine) recorded() []recordedRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]recordedRequest(nil), e.requests...)
}

// find returns the first recorded request for method and path.
func (e *fakeEngine) find(method, path string) (recordedRequest, bool) {
	for _, req := range e.recorded() {
		if req.Method == method && req.Path == path {
			return req, true
		}
	}
	return recordedRequest{}, false
}

func writeEngineError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func respondJSON(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

func respondStatus(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }
}

// frame encodes one multiplexed stream frame.
func frame(stream byte, payload string) []byte {
	header := make([]byte, frameHeaderLen, frameHeaderLen+len(payload))
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}
