package docker

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestParseTop(t *testing.T) {
	tests := []struct {
		name    string
		top     topResponse
		want    []model.TopProcess
		wantErr bool
	}{
		{
			name: "full column set as Docker Desktop returns it",
			top: topResponse{
				Titles: []string{"PID", "PPID", "USER", "NLWP", "RSS", "TIME", "STAT", "COMMAND"},
				Processes: [][]string{
					{"1457", "1276", "10001", "1", "44", "0", "Ss", "/sbin/docker-init -- alpha-worker serve -config /app/config.yaml"},
					{"1916", "1457", "10001", "13", "37224", "75", "Sl", "alpha-worker serve -config /app/config.yaml"},
				},
			},
			want: []model.TopProcess{
				{PID: 1457, PPID: 1276, User: "10001", Threads: 1, RSSBytes: 44 * 1024, CPUTime: 0, State: "S",
					Command: "/sbin/docker-init -- alpha-worker serve -config /app/config.yaml"},
				{PID: 1916, PPID: 1457, User: "10001", Threads: 13, RSSBytes: 37224 * 1024, CPUTime: 75 * time.Second, State: "S",
					Command: "alpha-worker serve -config /app/config.yaml"},
			},
		},
		{
			name: "reordered titles, optional columns missing, clock time",
			top: topResponse{
				Titles:    []string{"UID", "PID", "PPID", "C", "STIME", "TTY", "TIME", "CMD"},
				Processes: [][]string{{"root", "42", "1", "0", "10:00", "?", "1-02:03:04", "nginx: master process"}},
			},
			want: []model.TopProcess{
				{PID: 42, PPID: 1, User: "root", CPUTime: (26*3600 + 3*60 + 4) * time.Second, Command: "nginx: master process"},
			},
		},
		{
			name: "short row",
			top: topResponse{
				Titles:    []string{"PID", "USER", "COMMAND"},
				Processes: [][]string{{"7"}},
			},
			want: []model.TopProcess{{PID: 7}},
		},
		{
			name:    "no pid column",
			top:     topResponse{Titles: []string{"USER", "COMMAND"}, Processes: [][]string{{"root", "sh"}}},
			wantErr: true,
		},
		{
			name:    "malformed pid",
			top:     topResponse{Titles: []string{"PID"}, Processes: [][]string{{"x"}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTop(tt.top)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTop() = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("parseTop() = %+v\nwant         %+v", got, tt.want)
			}
		})
	}
}

func TestTopRequestsColumns(t *testing.T) {
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /containers/abc/top": respondJSON(`{"Titles":["PID","PPID","USER","NLWP","RSS","TIME","STAT","COMMAND"],
			"Processes":[["1","0","root","1","100","2","R+","sleep 1000"]]}`),
	})
	got, err := c.Top(t.Context(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != "R" || got[0].Command != "sleep 1000" {
		t.Fatalf("Top() = %+v", got)
	}
	req, _ := engine.find(http.MethodGet, "/containers/abc/top")
	if req.Query.Get("ps_args") != topPSArgs {
		t.Fatalf("ps_args = %q, want %q", req.Query.Get("ps_args"), topPSArgs)
	}
}
