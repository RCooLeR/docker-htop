package docker

import (
	"archive/tar"
	"bytes"
	"maps"
	"net/http"
	"testing"
)

const samplePasswd = `root:x:0:0:root:/root:/bin/bash
# a comment
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
toor:x:0:0:second root:/root:/bin/sh
broken-line
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
app:x:10001:10001::/app:
`

func passwdArchive(t *testing.T, typeflag byte, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	header := &tar.Header{Name: "passwd", Mode: 0o644, Typeflag: typeflag}
	if typeflag == tar.TypeReg {
		header.Size = int64(len(content))
	} else {
		header.Linkname = "/usr/share/passwd"
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if typeflag == tar.TypeReg {
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUserNames(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    map[uint32]string
		wantErr bool
	}{
		{
			name: "regular file",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-tar")
				_, _ = w.Write(passwdArchive(t, tar.TypeReg, samplePasswd))
			},
			want: map[uint32]string{0: "root", 1: "daemon", 65534: "nobody", 10001: "app"},
		},
		{
			name: "symlink",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(passwdArchive(t, tar.TypeSymlink, ""))
			},
			want: map[uint32]string{},
		},
		{
			name: "no such file",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeEngineError(w, http.StatusNotFound, "Could not find the file /etc/passwd in container abc")
			},
			want: map[uint32]string{},
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeEngineError(w, http.StatusInternalServerError, "boom")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, c := newFakeEngine(t, map[string]http.HandlerFunc{"GET /containers/abc/archive": tt.handler})
			got, err := c.UserNames(t.Context(), "abc")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("UserNames() = %v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || !maps.Equal(got, tt.want) {
				t.Fatalf("UserNames() = %v, want %v", got, tt.want)
			}
			req, _ := engine.find(http.MethodGet, "/containers/abc/archive")
			if req.Query.Get("path") != "/etc/passwd" {
				t.Fatalf("archive path = %q", req.Query.Get("path"))
			}
		})
	}
}

func TestParsePasswdTarEmptyArchive(t *testing.T) {
	var buf bytes.Buffer
	if err := tar.NewWriter(&buf).Close(); err != nil {
		t.Fatal(err)
	}
	got, err := parsePasswdTar(&buf)
	if err != nil || len(got) != 0 {
		t.Fatalf("parsePasswdTar(empty) = %v, %v", got, err)
	}
}
