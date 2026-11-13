package ui

import (
	"slices"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestSortContainers(t *testing.T) {
	base := []model.Container{
		{ID: "4", Name: "d", CPUPercent: f64(10), MemoryBytes: i64(200), PIDs: intp(9)},
		{ID: "2", Name: "b"},
		{ID: "3", Name: "c", CPUPercent: f64(50), MemoryBytes: i64(50), PIDs: intp(3)},
		{ID: "1", Name: "a", CPUPercent: f64(10), MemoryBytes: i64(100), PIDs: intp(3)},
	}
	tests := []struct {
		key  sortKey
		want []string
	}{
		{sortCPU, []string{"c", "a", "d", "b"}},
		{sortMemory, []string{"d", "a", "c", "b"}},
		{sortName, []string{"a", "b", "c", "d"}},
		{sortTasks, []string{"d", "a", "c", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			cs := make([]*model.Container, len(base))
			for i := range base {
				cs[i] = &base[i]
			}
			sortContainers(cs, tt.key)
			var got []string
			for _, c := range cs {
				got = append(got, c.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortProcesses(t *testing.T) {
	base := []model.Process{
		{PID: 40, Command: "b", CPUPercent: f64(1), MemoryBytes: i64(10), Threads: intp(2)},
		{PID: 30, Command: "a"},
		{PID: 20, Command: "c", CPUPercent: f64(5), MemoryBytes: i64(10), Threads: intp(2)},
		{PID: 10, Command: "b", CPUPercent: f64(1), MemoryBytes: i64(30), Threads: intp(1)},
	}
	tests := []struct {
		key  sortKey
		want []int
	}{
		{sortCPU, []int{20, 10, 40, 30}},
		{sortMemory, []int{10, 20, 40, 30}},
		{sortName, []int{30, 10, 40, 20}},
		{sortTasks, []int{20, 40, 10, 30}},
	}
	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			ps := make([]*model.Process, len(base))
			for i := range base {
				ps[i] = &base[i]
			}
			sortProcesses(ps, tt.key)
			var got []int
			for _, p := range ps {
				got = append(got, p.PID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortKeys(t *testing.T) {
	m, _ := newTestScreen(t, samplerSnapshot(), 120, 40)
	tests := []struct {
		key  string
		want sortKey
	}{
		{"f6", sortMemory},
		{"f6", sortName},
		{"f6", sortTasks},
		{"f6", sortCPU},
		{"M", sortMemory},
		{"N", sortName},
		{"P", sortCPU},
	}
	for _, tt := range tests {
		keys(m, tt.key)
		if m.tree.sort != tt.want {
			t.Fatalf("after %q sort = %v, want %v", tt.key, m.tree.sort, tt.want)
		}
	}
}
