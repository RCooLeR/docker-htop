package sampler

import (
	"errors"
	"reflect"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func TestParseCPUs(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []model.CPUTimes
		wantErr bool
	}{
		{
			name: "aggregate line skipped, guest not added",
			data: "cpu  9 9 9 9 9 9 9 9 9 9\ncpu0 1 2 3 4 5 6 7 8 100 100\nintr 5\n",
			want: []model.CPUTimes{{TotalMs: 360, IdleMs: 90}},
		},
		{
			name: "offline core leaves a gap",
			data: "cpu0 1 0 0 1 0 0 0 0\ncpu2 2 0 0 2 0 0 0 0\n",
			want: []model.CPUTimes{{TotalMs: 20, IdleMs: 10}, {}, {TotalMs: 40, IdleMs: 20}},
		},
		{name: "too few columns", data: "cpu0 1 2 3 4\n", wantErr: true},
		{name: "huge index", data: "cpu99999 1 0 0 1 0 0 0 0\n", wantErr: true},
		{name: "no cores", data: "cpu  1 0 0 1 0 0 0 0\n", wantErr: true},
		{name: "garbage", data: "cpu0 a b c d e f g h\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCPUs([]byte(tt.data))
			if tt.wantErr {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("error = %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCPUs = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestParseMeminfo(t *testing.T) {
	const full = "MemTotal: 100 kB\nMemFree: 10 kB\nMemAvailable: 50 kB\nBuffers: 5 kB\n" +
		"Cached: 20 kB\nSwapTotal: 8 kB\nSwapFree: 4 kB\n"
	tests := []struct {
		name    string
		data    string
		want    model.SampleMemory
		wantErr bool
	}{
		{
			name: "without SReclaimable",
			data: full,
			want: model.SampleMemory{TotalBytes: 102400, FreeBytes: 10240, AvailableBytes: 51200,
				BuffersBytes: 5120, CachedBytes: 20480, SwapTotalBytes: 8192, SwapFreeBytes: 4096},
		},
		{
			name: "SReclaimable added to cache",
			data: full + "SReclaimable: 2 kB\nSUnreclaim: 1 kB\n",
			want: model.SampleMemory{TotalBytes: 102400, FreeBytes: 10240, AvailableBytes: 51200,
				BuffersBytes: 5120, CachedBytes: 22528, SwapTotalBytes: 8192, SwapFreeBytes: 4096},
		},
		{name: "missing field", data: "MemTotal: 100 kB\n", wantErr: true},
		{name: "bad unit", data: "MemTotal: 100 MB\n", wantErr: true},
		{name: "bad number", data: "MemTotal: x kB\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMeminfo([]byte(tt.data))
			if tt.wantErr {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("error = %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("parseMeminfo = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestParseLoadavg(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []float64
		wantErr bool
	}{
		{name: "full", data: "0.52 0.58 0.59 2/1024 12345\n", want: []float64{0.52, 0.58, 0.59}},
		{name: "short", data: "0.52 0.58\n", wantErr: true},
		{name: "garbage", data: "a b c\n", wantErr: true},
		{name: "negative", data: "-1 0 0\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLoadavg([]byte(tt.data))
			if tt.wantErr {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("error = %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLoadavg = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}
