package sampler

import (
	"strings"
	"testing"
)

func TestFirstContainerID(t *testing.T) {
	const id = fixtureID1
	tests := []struct {
		name string
		data string
		want string
	}{
		{"v2 docker", "0::/docker/" + id + "\n", id},
		{"v2 other namespace", "0::/../" + id + "\n", id},
		{"v2 systemd scope", "0::/system.slice/docker-" + id + ".scope\n", id},
		{"v1", "12:pids:/docker/" + id + "\n5:memory:/docker/" + id + "\n", id},
		{"no trailing newline", "0::/docker/" + id, id},
		{"nested takes outermost", "0::/docker/" + id + "/docker/" + fixtureID2 + "\n", id},
		{"host init", "0::/init.scope\n", ""},
		{"root", "0::/\n", ""},
		{"63 characters", "0::/docker/" + id[:63] + "\n", ""},
		{"65 characters", "0::/docker/" + id + "a\n", ""},
		{"uppercase", "0::/docker/" + strings.ToUpper(id) + "\n", ""},
		{"uppercase neighbour", "0::/docker/" + id + "F\n", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(firstContainerID([]byte(tt.data))); got != tt.want {
				t.Errorf("firstContainerID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCgroupHasContainer(t *testing.T) {
	data := []byte("0::/docker/" + fixtureID1 + "/docker/" + fixtureID2 + "\n")
	for _, tt := range []struct {
		id   string
		want bool
	}{{fixtureID1, true}, {fixtureID2, true}, {fixtureID3, false}, {fixtureID1[:12], false}} {
		t.Run(tt.id, func(t *testing.T) {
			if got := cgroupHasContainer(data, tt.id); got != tt.want {
				t.Errorf("cgroupHasContainer = %v, want %v", got, tt.want)
			}
		})
	}
}

func FuzzContainerIDs(f *testing.F) {
	f.Add([]byte("0::/docker/" + fixtureID1 + "\n"))
	f.Add([]byte("0::/system.slice/docker-" + fixtureID1 + ".scope"))
	f.Fuzz(func(t *testing.T, data []byte) {
		for id := range containerIDs(data) {
			if !isContainerID(string(id)) {
				t.Fatalf("yielded %q", id)
			}
		}
	})
}
