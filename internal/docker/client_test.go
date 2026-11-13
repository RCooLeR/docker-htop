package docker

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestNegotiateVersion(t *testing.T) {
	tests := []struct{ server, want string }{
		{server: "1.45", want: "1.45"},
		{server: "1.56", want: maxAPIVersion},
		{server: maxAPIVersion, want: maxAPIVersion},
		{server: "1.9", want: "1.9"},
		{server: "2.0", want: maxAPIVersion},
		{server: "", want: maxAPIVersion},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			if got := negotiateVersion(tt.server); got != tt.want {
				t.Fatalf("negotiateVersion(%q) = %q, want %q", tt.server, got, tt.want)
			}
		})
	}
}

func TestVersionPrefixAndSinglePing(t *testing.T) {
	var pings, infos atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_ping":
			pings.Add(1)
			w.Header().Set("Api-Version", "1.56")
			_, _ = io.WriteString(w, "OK")
		case "/v" + maxAPIVersion + "/info":
			infos.Add(1)
			_, _ = io.WriteString(w, `{"Name":"docker-desktop"}`)
		default:
			writeEngineError(w, http.StatusBadRequest, "unexpected path "+r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	c, err := New("tcp://" + server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for range 3 {
		if _, err := c.Info(t.Context()); err != nil {
			t.Fatalf("Info() error: %v", err)
		}
	}
	if pings.Load() != 1 || infos.Load() != 3 {
		t.Fatalf("pings = %d, infos = %d; want 1 and 3", pings.Load(), infos.Load())
	}
}

func TestPingFailureIsReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeEngineError(w, http.StatusInternalServerError, "engine starting")
	}))
	t.Cleanup(server.Close)
	c, err := New("tcp://" + server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, err = c.Containers(t.Context())
	if err == nil || !strings.Contains(err.Error(), "engine starting") {
		t.Fatalf("Containers() error = %v, want the ping failure", err)
	}
}

func TestAPIErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.HandlerFunc
		wantNotFound bool
		wantText     string
	}{
		{
			name: "no such container",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeEngineError(w, http.StatusNotFound, "No such container: abc")
			},
			wantNotFound: true, wantText: "No such container: abc",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeEngineError(w, http.StatusInternalServerError, "cgroup read failed")
			},
			wantText: "status 500: cgroup read failed",
		},
		{
			name: "plain text body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = io.WriteString(w, "container is paused\n")
			},
			wantText: "status 409: container is paused",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newFakeEngine(t, map[string]http.HandlerFunc{"GET /containers/abc/stats": tt.handler})
			_, err := c.Stats(t.Context(), "abc")
			if err == nil {
				t.Fatal("Stats() succeeded, want an error")
			}
			if errors.Is(err, ErrNotFound) != tt.wantNotFound {
				t.Fatalf("errors.Is(%v, ErrNotFound) = %v", err, !tt.wantNotFound)
			}
			if !strings.Contains(err.Error(), tt.wantText) || !strings.Contains(err.Error(), "abc") {
				t.Fatalf("error %q does not contain %q and the container id", err, tt.wantText)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	_, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /info": respondJSON(`{"ID":"x","Containers":21,"Name":"docker-desktop","ServerVersion":"29.9.2",
			"Architecture":"x86_64","NCPU":16,"MemTotal":16624349184,"OperatingSystem":"Docker Desktop"}`),
	})
	got, err := c.Info(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := model.EngineInfo{Name: "docker-desktop", Version: "29.9.2", Architecture: "x86_64", CPUs: 16, MemoryBytes: 16624349184}
	if got != want {
		t.Fatalf("Info() = %+v, want %+v", got, want)
	}
}

func TestContainers(t *testing.T) {
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /containers/json": respondJSON(`[
			{"Id":"c0ffee123456aa","Names":["/alpha-worker"],"Image":"alpha-worker:latest","ImageID":"sha256:1",
			 "Command":"alpha-worker serve","Created":1762768800,"State":"running","Status":"Up 3 days (healthy)",
			 "Labels":{"com.docker.compose.project":"monitoring"}},
			{"Id":"ffee","Names":[],"Image":"busybox","Created":0,"State":"exited","Status":"Exited (0) 2 hours ago","Labels":null}
		]`),
	})
	got, err := c.Containers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if req, ok := engine.find(http.MethodGet, "/containers/json"); !ok || req.Query.Get("all") != "true" {
		t.Fatalf("list request = %+v, want all=true", req)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	first := got[0]
	if first.ID != "c0ffee123456aa" || first.Name != "alpha-worker" || first.State != model.StateRunning ||
		first.Status != "Up 3 days (healthy)" || first.Labels[model.LabelComposeProject] != "monitoring" ||
		!first.Created.Equal(time.Unix(1762768800, 0)) {
		t.Fatalf("first container = %+v", first)
	}
	if got[1].Name != "" || got[1].State != model.StateExited {
		t.Fatalf("second container = %+v", got[1])
	}
}

func TestKill(t *testing.T) {
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"POST /containers/abc/kill": respondStatus(http.StatusNoContent),
	})
	if err := c.Kill(t.Context(), "abc", model.SignalHup); err != nil {
		t.Fatal(err)
	}
	req, ok := engine.find(http.MethodPost, "/containers/abc/kill")
	if !ok || req.Query.Get("signal") != "SIGHUP" {
		t.Fatalf("kill request = %+v, want signal=SIGHUP", req)
	}
	if err := c.Kill(t.Context(), "gone", model.SignalTerm); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Kill(gone) error = %v, want ErrNotFound", err)
	}
}
