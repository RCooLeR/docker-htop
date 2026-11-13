package docker

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func signalEngineRoutes(exitCode string) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /containers/create":        respondJSON(`{"Id":"signal1"}`),
		"POST /containers/signal1/start": respondStatus(http.StatusNoContent),
		"POST /containers/signal1/wait":  respondJSON(`{"StatusCode":` + exitCode + `}`),
		"GET /containers/signal1/logs": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(frame(streamStderr, "pid 4242 does not belong to container abc\n"))
		},
		"DELETE /containers/signal1": respondStatus(http.StatusNoContent),
	}
}

func TestSignalProcess(t *testing.T) {
	engine, c := newFakeEngine(t, signalEngineRoutes("0"))
	if err := c.SignalProcess(t.Context(), "docker-htop-sampler:0123456789ab", "abc", 4242, model.SignalTerm); err != nil {
		t.Fatal(err)
	}
	create, _ := engine.find(http.MethodPost, "/containers/create")
	if !strings.HasPrefix(create.Query.Get("name"), "docker-htop-signal-") {
		t.Fatalf("helper name = %q", create.Query.Get("name"))
	}
	var spec containerSpec
	if err := json.Unmarshal(create.Body, &spec); err != nil {
		t.Fatal(err)
	}
	wantCmd := []string{model.SamplerCommandSignal, "--container", "abc", "--pid", "4242", "--signal", "SIGTERM"}
	if !slices.Equal(spec.Cmd, wantCmd) || spec.Labels[model.LabelRole] != model.RoleSignal {
		t.Fatalf("spec = %+v", spec)
	}
	host := spec.HostConfig
	if host.PidMode != "host" || host.NetworkMode != "none" || !host.ReadonlyRootfs || host.AutoRemove ||
		!slices.Equal(host.CapDrop, []string{"ALL"}) || !slices.Equal(host.CapAdd, []string{"KILL"}) ||
		!slices.Equal(host.SecurityOpt, []string{"no-new-privileges"}) {
		t.Fatalf("host config = %+v", host)
	}
	if _, ok := engine.find(http.MethodGet, "/containers/signal1/logs"); ok {
		t.Fatal("logs read after a clean exit")
	}
	if _, ok := engine.find(http.MethodDelete, "/containers/signal1"); !ok {
		t.Fatal("helper not removed")
	}
}

func TestSignalProcessHelperFailure(t *testing.T) {
	engine, c := newFakeEngine(t, signalEngineRoutes("3"))
	err := c.SignalProcess(t.Context(), "docker-htop-sampler:0123456789ab", "abc", 4242, model.SignalKill)
	if err == nil || !strings.Contains(err.Error(), "code 3") ||
		!strings.Contains(err.Error(), "pid 4242 does not belong to container abc") {
		t.Fatalf("SignalProcess() error = %v, want the exit code and helper output", err)
	}
	if _, ok := engine.find(http.MethodDelete, "/containers/signal1"); !ok {
		t.Fatal("helper not removed after a failure")
	}
}

func TestSignalProcessInvalidTarget(t *testing.T) {
	_, c := newFakeEngine(t, nil)
	if err := c.SignalProcess(t.Context(), "img", "abc", 0, model.SignalTerm); err == nil {
		t.Fatal("SignalProcess(pid 0) succeeded")
	}
}
