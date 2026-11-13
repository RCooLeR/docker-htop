package docker

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RCooLeR/docker-htop/internal/model"
)

func readTarEntries(t *testing.T, data []byte) map[string]*tarEntry {
	t.Helper()
	entries := map[string]*tarEntry{}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = &tarEntry{header: header, data: content}
	}
}

type tarEntry struct {
	header *tar.Header
	data   []byte
}

func TestBuildImageArchive(t *testing.T) {
	binary := []byte("\x7fELF fake sampler")
	ref := samplerImageRef(binary)
	if !strings.HasPrefix(ref, "docker-htop-sampler:") || len(ref) != len("docker-htop-sampler:")+12 {
		t.Fatalf("ref = %q", ref)
	}
	archive, err := buildImageArchive(ref, binary, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	again, err := buildImageArchive(ref, binary, "amd64")
	if err != nil || !bytes.Equal(archive, again) {
		t.Fatal("archive is not deterministic")
	}
	entries := readTarEntries(t, archive)

	var dockerManifest []dockerManifestEntry
	if err := json.Unmarshal(entries["manifest.json"].data, &dockerManifest); err != nil {
		t.Fatal(err)
	}
	if len(dockerManifest) != 1 || !slices.Equal(dockerManifest[0].RepoTags, []string{ref}) {
		t.Fatalf("manifest.json = %+v", dockerManifest)
	}
	var index ociIndex
	if err := json.Unmarshal(entries["index.json"].data, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Manifests) != 1 || index.Manifests[0].Annotations["io.containerd.image.name"] != "docker.io/library/"+ref {
		t.Fatalf("index.json = %+v", index)
	}
	if entries["oci-layout"] == nil {
		t.Fatal("no oci-layout")
	}

	var config imageConfig
	configEntry := entries[dockerManifest[0].Config]
	if err := json.Unmarshal(configEntry.data, &config); err != nil {
		t.Fatal(err)
	}
	if config.OS != "linux" || config.Architecture != "amd64" ||
		!slices.Equal(config.Config.Entrypoint, []string{model.SamplerBinaryPath}) ||
		config.Config.Labels[model.LabelRole] != model.RoleSampler {
		t.Fatalf("config = %+v", config)
	}
	if digestOf(configEntry.data) != "sha256:"+strings.TrimPrefix(dockerManifest[0].Config, "blobs/sha256/") {
		t.Fatal("config blob name does not match its digest")
	}

	layer := entries[dockerManifest[0].Layers[0]]
	if config.RootFS.DiffIDs[0] != digestOf(layer.data) {
		t.Fatal("diff id does not match the layer")
	}
	files := readTarEntries(t, layer.data)
	file := files[strings.TrimPrefix(model.SamplerBinaryPath, "/")]
	if file == nil || !bytes.Equal(file.data, binary) || file.header.Mode != 0o755 {
		t.Fatalf("layer entries = %v", files)
	}
}

func TestEnsureSamplerImage(t *testing.T) {
	binary := []byte("sampler v2")
	ref := samplerImageRef(binary)
	var loaded atomic.Bool
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /images/" + ref + "/json": func(w http.ResponseWriter, _ *http.Request) {
			if !loaded.Load() {
				writeEngineError(w, http.StatusNotFound, "No such image: "+ref)
				return
			}
			_, _ = io.WriteString(w, `{"Id":"sha256:abc"}`)
		},
		"POST /images/load": func(w http.ResponseWriter, _ *http.Request) {
			loaded.Store(true)
			_, _ = io.WriteString(w, `{"stream":"Loaded image: `+ref+`\n"}`+"\n")
		},
		"GET /images/json": respondJSON(`[{"RepoTags":["` + ref + `","docker-htop-sampler:old000000000"]},` +
			`{"RepoTags":["docker-htop-sampler-other:1","busybox:latest"]}]`),
		"DELETE /images/docker-htop-sampler:old000000000": respondJSON(`[{"Untagged":"docker-htop-sampler:old000000000"}]`),
	})

	got, err := c.EnsureSamplerImage(t.Context(), binary, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got != ref {
		t.Fatalf("EnsureSamplerImage() = %q, want %q", got, ref)
	}
	load, ok := engine.find(http.MethodPost, "/images/load")
	if !ok || len(readTarEntries(t, load.Body)) == 0 {
		t.Fatal("no image tarball posted")
	}
	var deleted []string
	for _, req := range engine.recorded() {
		if req.Method == http.MethodDelete {
			deleted = append(deleted, req.Path)
		}
	}
	if !slices.Equal(deleted, []string{"/images/docker-htop-sampler:old000000000"}) {
		t.Fatalf("deleted = %q, want only the old sampler tag", deleted)
	}
}

func TestEnsureSamplerImageExisting(t *testing.T) {
	binary := []byte("sampler v1")
	ref := samplerImageRef(binary)
	engine, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /images/" + ref + "/json": respondJSON(`{"Id":"sha256:abc"}`),
	})
	got, err := c.EnsureSamplerImage(t.Context(), binary, "amd64")
	if err != nil || got != ref {
		t.Fatalf("EnsureSamplerImage() = %q, %v", got, err)
	}
	if n := len(engine.recorded()); n != 1 {
		t.Fatalf("made %d requests, want only the inspect", n)
	}
}

func TestEnsureSamplerImageLoadError(t *testing.T) {
	binary := []byte("sampler v3")
	ref := samplerImageRef(binary)
	_, c := newFakeEngine(t, map[string]http.HandlerFunc{
		"GET /images/" + ref + "/json": respondStatus(http.StatusNotFound),
		"POST /images/load":            respondJSON(`{"errorDetail":{"message":"unexpected EOF"},"error":"unexpected EOF"}` + "\n"),
	})
	_, err := c.EnsureSamplerImage(t.Context(), binary, "amd64")
	if err == nil || !strings.Contains(err.Error(), "load sampler image") || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("EnsureSamplerImage() error = %v", err)
	}
}
