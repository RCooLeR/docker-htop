package docker

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/RCooLeR/docker-htop/internal/model"
)

// Media types of the OCI image layout.
const (
	mediaTypeIndex    = "application/vnd.oci.image.index.v1+json"
	mediaTypeManifest = "application/vnd.oci.image.manifest.v1+json"
	mediaTypeConfig   = "application/vnd.oci.image.config.v1+json"
	mediaTypeLayer    = "application/vnd.oci.image.layer.v1.tar"
)

type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type imageConfig struct {
	Architecture string             `json:"architecture"`
	OS           string             `json:"os"`
	Config       imageRuntimeConfig `json:"config"`
	RootFS       imageRootFS        `json:"rootfs"`
}

type imageRuntimeConfig struct {
	Entrypoint []string          `json:"Entrypoint"`
	Labels     map[string]string `json:"Labels"`
}

type imageRootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type ociManifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}

type ociIndex struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []descriptor `json:"manifests"`
}

type dockerManifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

type archiveEntry struct {
	name string
	mode int64
	dir  bool
	data []byte
}

// imageBlobs holds the content-addressed parts of the sampler image.
type imageBlobs struct {
	layer, config, manifest []byte
}

// buildImageArchive returns an image tarball for POST /images/load: a single-layer Linux image
// for architecture holding binary at model.SamplerBinaryPath, tagged ref. It carries both the
// OCI layout (read by the containerd image store) and manifest.json (read by the classic store),
// the same shape docker save writes. The output is deterministic for the same input.
func buildImageArchive(ref string, binary []byte, architecture string) ([]byte, error) {
	blobs, err := buildImageBlobs(binary, architecture)
	if err != nil {
		return nil, err
	}
	layerDigest, configDigest, manifestDigest := digestOf(blobs.layer), digestOf(blobs.config), digestOf(blobs.manifest)

	_, tag, _ := strings.Cut(ref, ":")
	indexJSON, err := json.Marshal(ociIndex{
		SchemaVersion: 2,
		MediaType:     mediaTypeIndex,
		Manifests: []descriptor{{
			MediaType: mediaTypeManifest,
			Digest:    manifestDigest,
			Size:      len(blobs.manifest),
			Annotations: map[string]string{
				"io.containerd.image.name":          "docker.io/library/" + ref,
				"org.opencontainers.image.ref.name": tag,
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("encode image index: %w", err)
	}
	dockerManifestJSON, err := json.Marshal([]dockerManifestEntry{{
		Config:   blobPath(configDigest),
		RepoTags: []string{ref},
		Layers:   []string{blobPath(layerDigest)},
	}})
	if err != nil {
		return nil, fmt.Errorf("encode docker manifest: %w", err)
	}

	archive, err := writeTar([]archiveEntry{
		{name: "blobs/", mode: 0o755, dir: true},
		{name: "blobs/sha256/", mode: 0o755, dir: true},
		{name: blobPath(layerDigest), mode: 0o644, data: blobs.layer},
		{name: blobPath(configDigest), mode: 0o644, data: blobs.config},
		{name: blobPath(manifestDigest), mode: 0o644, data: blobs.manifest},
		{name: "index.json", mode: 0o644, data: indexJSON},
		{name: "manifest.json", mode: 0o644, data: dockerManifestJSON},
		{name: "oci-layout", mode: 0o644, data: []byte(`{"imageLayoutVersion":"1.0.0"}`)},
	})
	if err != nil {
		return nil, fmt.Errorf("write image archive: %w", err)
	}
	return archive, nil
}

func buildImageBlobs(binary []byte, architecture string) (imageBlobs, error) {
	layer, err := writeTar([]archiveEntry{
		{name: strings.TrimPrefix(model.SamplerBinaryPath, "/"), mode: 0o755, data: binary},
	})
	if err != nil {
		return imageBlobs{}, fmt.Errorf("write image layer: %w", err)
	}
	config, err := json.Marshal(imageConfig{
		Architecture: architecture,
		OS:           "linux",
		Config: imageRuntimeConfig{
			Entrypoint: []string{model.SamplerBinaryPath},
			Labels:     map[string]string{model.LabelRole: model.RoleSampler},
		},
		RootFS: imageRootFS{Type: "layers", DiffIDs: []string{digestOf(layer)}},
	})
	if err != nil {
		return imageBlobs{}, fmt.Errorf("encode image config: %w", err)
	}
	manifest, err := json.Marshal(ociManifest{
		SchemaVersion: 2,
		MediaType:     mediaTypeManifest,
		Config:        descriptor{MediaType: mediaTypeConfig, Digest: digestOf(config), Size: len(config)},
		Layers:        []descriptor{{MediaType: mediaTypeLayer, Digest: digestOf(layer), Size: len(layer)}},
	})
	if err != nil {
		return imageBlobs{}, fmt.Errorf("encode image manifest: %w", err)
	}
	return imageBlobs{layer: layer, config: config, manifest: manifest}, nil
}

func writeTar(entries []archiveEntry) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		header := &tar.Header{
			Name:    entry.name,
			Mode:    entry.mode,
			Size:    int64(len(entry.data)),
			ModTime: time.Unix(0, 0),
			Format:  tar.FormatPAX,
		}
		header.Typeflag = tar.TypeReg
		if entry.dir {
			header.Typeflag = tar.TypeDir
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write tar header %s: %w", entry.name, err)
		}
		if _, err := tw.Write(entry.data); err != nil {
			return nil, fmt.Errorf("write tar entry %s: %w", entry.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	return buf.Bytes(), nil
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func blobPath(digest string) string {
	return "blobs/sha256/" + strings.TrimPrefix(digest, "sha256:")
}
