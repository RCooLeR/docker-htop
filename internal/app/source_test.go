package app

import (
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestSeesContainers(t *testing.T) {
	const (
		web = "1111111111111111111111111111111111111111111111111111111111111111"
		db  = "2222222222222222222222222222222222222222222222222222222222222222"
	)
	container := func(id string, state model.ContainerState) model.ContainerInfo {
		return model.ContainerInfo{ID: id, State: state}
	}
	process := func(containerID string) model.ProcessSample {
		return model.ProcessSample{PID: 100, ContainerID: containerID}
	}

	tests := []struct {
		name       string
		processes  []model.ProcessSample
		containers []model.ContainerInfo
		engineName string
		hostName   string
		want       bool
	}{
		{
			name:       "a running container's process is visible",
			processes:  []model.ProcessSample{process(db)},
			containers: []model.ContainerInfo{container(web, model.StateRunning), container(db, model.StateRunning)},
			engineName: "docker-desktop",
			hostName:   "laptop",
			want:       true,
		},
		{
			name:       "containers run but none is visible, as with Docker Desktop",
			containers: []model.ContainerInfo{container(web, model.StateRunning)},
			engineName: "docker-desktop",
			hostName:   "laptop",
		},
		{
			name:       "same host name does not help when running containers are invisible",
			containers: []model.ContainerInfo{container(web, model.StateRunning)},
			engineName: "server",
			hostName:   "server",
		},
		{
			name:       "only processes of other containers are visible",
			processes:  []model.ProcessSample{process(db)},
			containers: []model.ContainerInfo{container(web, model.StateRunning)},
			engineName: "server",
			hostName:   "server",
		},
		{
			name:       "a stopped container proves nothing",
			processes:  []model.ProcessSample{process(web)},
			containers: []model.ContainerInfo{container(web, model.StateExited)},
			engineName: "docker-desktop",
			hostName:   "laptop",
		},
		{
			name:       "nothing runs and the engine is this machine",
			containers: []model.ContainerInfo{container(web, model.StateExited)},
			engineName: "Server",
			hostName:   "server",
			want:       true,
		},
		{
			name:       "nothing runs and the engine is another machine",
			engineName: "docker-desktop",
			hostName:   "laptop",
		},
		{
			name: "nothing runs and the names are unknown",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sample := model.Sample{Processes: tc.processes}
			if got := seesContainers(sample, tc.containers, tc.engineName, tc.hostName); got != tc.want {
				t.Errorf("seesContainers = %v, want %v", got, tc.want)
			}
		})
	}
}
