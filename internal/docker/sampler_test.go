package docker

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// hijackAttach answers an attach request like the engine: 101, then raw frames once started is
// closed, then reads until the client closes the connection.
func hijackAttach(t *testing.T, started <-chan struct{}, frames []byte, wg *sync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "tcp" || !strings.EqualFold(r.Header.Get("Connection"), "Upgrade") {
			writeEngineError(w, http.StatusBadRequest, "attach without upgrade")
			return
		}
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		wg.Go(func() {
			defer closeQuietly(conn)
			_, _ = buf.WriteString("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.multiplexed-stream\r\n" +
				"Connection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
			_ = buf.Flush()
			<-started
			_, _ = conn.Write(frames)
			_, _ = io.Copy(io.Discard, conn)
		})
	}
}

func TestStartSampler(t *testing.T) {
	started := make(chan struct{})
	var attachWG sync.WaitGroup
	frames := joinFrames(frame(streamStdout, sampleLine1), frame(streamStdout, sampleLine2))
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"POST /containers/create":          respondJSON(`{"Id":"sampler1","Warnings":[]}`),
		"POST /containers/sampler1/attach": hijackAttach(t, started, frames, &attachWG),
		"POST /containers/sampler1/start": func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			w.WriteHeader(http.StatusNoContent)
		},
		"DELETE /containers/sampler1": respondStatus(http.StatusNoContent),
	})

	s, err := c.StartSampler(t.Context(), "docker-htop-sampler:0123456789ab", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	first, second := <-s.Samples(), <-s.Samples()
	if first.Tasks != 120 || second.Tasks != 121 {
		t.Fatalf("samples = %+v, %+v", first, second)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
	if _, open := <-s.Samples(); open {
		t.Fatal("Samples() still open after Close")
	}
	if err := s.Err(); err != nil {
		t.Fatalf("Err() after Close = %v, want nil", err)
	}
	attachWG.Wait()

	var order []string
	for _, req := range engine.recorded() {
		order = append(order, req.Method+" "+req.Path)
	}
	wantOrder := []string{
		"POST /containers/create", "POST /containers/sampler1/attach",
		"POST /containers/sampler1/start", "DELETE /containers/sampler1",
	}
	if !slices.Equal(order, wantOrder) {
		t.Fatalf("request order = %q, want %q", order, wantOrder)
	}
	assertSamplerCreate(t, engine)
	attach, _ := engine.find(http.MethodPost, "/containers/sampler1/attach")
	for _, key := range []string{"stream", "stdin", "stdout", "stderr"} {
		if attach.Query.Get(key) != "1" {
			t.Fatalf("attach query %v lacks %s=1", attach.Query, key)
		}
	}
	remove, _ := engine.find(http.MethodDelete, "/containers/sampler1")
	if remove.Query.Get("force") != "1" {
		t.Fatalf("remove query = %v, want force=1", remove.Query)
	}
}

func assertSamplerCreate(t *testing.T, engine *fakeEngine) {
	t.Helper()
	create, ok := engine.find(http.MethodPost, "/containers/create")
	if !ok {
		t.Fatal("no create request")
	}
	if !strings.HasPrefix(create.Query.Get("name"), "docker-htop-sampler-") {
		t.Fatalf("container name = %q", create.Query.Get("name"))
	}
	var spec containerSpec
	if err := json.Unmarshal(create.Body, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Image != "docker-htop-sampler:0123456789ab" ||
		!slices.Equal(spec.Cmd, []string{model.SamplerCommandStream, "--interval", "2s"}) ||
		spec.Labels[model.LabelRole] != model.RoleSampler {
		t.Fatalf("create spec = %+v", spec)
	}
	if !spec.AttachStdin || !spec.AttachStdout || !spec.AttachStderr || !spec.OpenStdin || !spec.StdinOnce || spec.Tty {
		t.Fatalf("stdio flags = %+v", spec)
	}
	host := spec.HostConfig
	if host.PidMode != "host" || host.NetworkMode != "none" || !host.ReadonlyRootfs || !host.AutoRemove ||
		!slices.Equal(host.CapDrop, []string{"ALL"}) || len(host.CapAdd) != 0 ||
		!slices.Equal(host.SecurityOpt, []string{"no-new-privileges"}) ||
		host.LogConfig == nil || host.LogConfig.Type != "none" || host.Memory <= 0 || host.PidsLimit <= 0 {
		t.Fatalf("host config = %+v", host)
	}
}

func TestStartSamplerStartFailureCleansUp(t *testing.T) {
	started := make(chan struct{})
	close(started)
	var attachWG sync.WaitGroup
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"POST /containers/create":          respondJSON(`{"Id":"sampler2"}`),
		"POST /containers/sampler2/attach": hijackAttach(t, started, nil, &attachWG),
		"POST /containers/sampler2/start": func(w http.ResponseWriter, _ *http.Request) {
			writeEngineError(w, http.StatusInternalServerError, "host pid namespace is not allowed")
		},
		"DELETE /containers/sampler2": respondStatus(http.StatusNoContent),
	})

	_, err := c.StartSampler(t.Context(), "docker-htop-sampler:0123456789ab", time.Second)
	if err == nil || !strings.Contains(err.Error(), "start container sampler2") ||
		!strings.Contains(err.Error(), "host pid namespace is not allowed") {
		t.Fatalf("StartSampler() error = %v, want the failed start step", err)
	}
	attachWG.Wait()
	if _, ok := engine.find(http.MethodDelete, "/containers/sampler2"); !ok {
		t.Fatal("container not removed after a failed start")
	}
}

func TestStartSamplerCreateFailure(t *testing.T) {
	_, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"POST /containers/create": func(w http.ResponseWriter, _ *http.Request) {
			writeEngineError(w, http.StatusNotFound, "No such image: docker-htop-sampler:0123456789ab")
		},
	})
	_, err := c.StartSampler(t.Context(), "docker-htop-sampler:0123456789ab", time.Second)
	if err == nil || !strings.Contains(err.Error(), "create container docker-htop-sampler-") {
		t.Fatalf("StartSampler() error = %v, want the failed create step", err)
	}
}
