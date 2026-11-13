package sampler

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

const statTail = " 0 -1 4194560 120 0 0 0 250 50 0 0 20 0 3 0 5000 10485760 512 18446744073709551615 1 1 0\n"

func TestParseProcStat(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    procStat
		wantErr bool
	}{
		{
			name: "plain",
			line: "42 (nginx) S 1 42 42" + statTail,
			want: procStat{name: []byte("nginx"), state: 'S', ppid: 1, utime: 250, stime: 50,
				threads: 3, startTicks: 5000, rssPages: 512},
		},
		{
			name: "name with spaces and parentheses",
			line: "42 (a) b (c) R 7 42 42" + statTail,
			want: procStat{name: []byte("a) b (c"), state: 'R', ppid: 7, utime: 250, stime: 50,
				threads: 3, startTicks: 5000, rssPages: 512},
		},
		{
			name: "empty name",
			line: "42 () Z 0 42 42" + statTail,
			want: procStat{name: []byte{}, state: 'Z', utime: 250, stime: 50,
				threads: 3, startTicks: 5000, rssPages: 512},
		},
		{name: "no name", line: "42 nginx S 1", wantErr: true},
		{name: "truncated", line: "42 (nginx) S 1 42 42 0 -1", wantErr: true},
		{name: "long state", line: "42 (nginx) SS 1 42 42" + statTail, wantErr: true},
		{name: "negative utime", line: "42 (x) S 1 42 42 0 -1 4194560 120 0 0 0 -250 50 0 0 20 0 3 0 5000 1 512", wantErr: true},
		{name: "overflow", line: "42 (x) S 99999999999999999999 42 42" + statTail, wantErr: true},
		{name: "empty", line: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProcStat([]byte(tt.line))
			if tt.wantErr {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("error = %v, want ErrMalformed", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseProcStat: %v", err)
			}
			if !bytes.Equal(got.name, tt.want.name) {
				t.Errorf("name = %q, want %q", got.name, tt.want.name)
			}
			got.name, tt.want.name = nil, nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func FuzzParseProcStat(f *testing.F) {
	f.Add([]byte("42 (nginx) S 1 42 42" + statTail))
	f.Add([]byte("42 (a) b (c) R 7 42 42" + statTail))
	f.Add([]byte("1 ((((((((("))
	f.Add([]byte(")("))
	f.Fuzz(func(t *testing.T, line []byte) {
		st, err := parseProcStat(line)
		if err != nil {
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("error %v does not wrap ErrMalformed", err)
			}
			return
		}
		if !bytes.Contains(line, st.name) {
			t.Fatalf("name %q not in line", st.name)
		}
		if bytes.IndexByte(line[bytes.LastIndexByte(line, ')')+1:], st.state) < 0 {
			t.Fatalf("state %q not after the name", st.state)
		}
		if st.ppid < 0 || st.utime < 0 || st.stime < 0 || st.threads < 0 || st.startTicks < 0 || st.rssPages < 0 {
			t.Fatalf("negative field in %+v", st)
		}
	})
}
