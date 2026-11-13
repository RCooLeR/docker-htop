package docker

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// samplerRepository is the local image repository of the sampler image.
const samplerRepository = "docker-htop-sampler"

// samplerImageRef names the sampler image after the content of its binary, so a new plugin
// version gets a new image and an unchanged one reuses it.
func samplerImageRef(binary []byte) string {
	sum := sha256.Sum256(binary)
	return samplerRepository + ":" + hex.EncodeToString(sum[:])[:12]
}

// EnsureSamplerImage makes sure the sampler image for binary exists and returns its reference,
// docker-htop-sampler:<first 12 hex of sha256(binary)>. binary is a static Linux executable and
// architecture the one it was built for, in the OCI spelling ("amd64", "arm64"); the image is
// declared for that architecture. When the image is missing it is loaded through
// POST /images/load (no BuildKit, no pull), and other docker-htop-sampler tags are then removed
// best-effort.
func (c *Client) EnsureSamplerImage(ctx context.Context, binary []byte, architecture string) (string, error) {
	if len(binary) == 0 {
		return "", errors.New("ensure sampler image: empty binary")
	}
	if architecture == "" {
		return "", errors.New("ensure sampler image: unknown architecture")
	}
	ref := samplerImageRef(binary)
	exists, err := c.imageExists(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("ensure sampler image %s: %w", ref, err)
	}
	if exists {
		return ref, nil
	}

	archive, err := buildImageArchive(ref, binary, architecture)
	if err != nil {
		return "", fmt.Errorf("ensure sampler image %s: %w", ref, err)
	}
	if err := c.loadImage(ctx, archive); err != nil {
		return "", fmt.Errorf("load sampler image %s: %w", ref, err)
	}
	exists, err = c.imageExists(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("inspect loaded sampler image %s: %w", ref, err)
	}
	if !exists {
		return "", fmt.Errorf("load sampler image %s: the engine did not tag the loaded image", ref)
	}
	c.pruneSamplerTags(ctx, ref)
	return ref, nil
}

func (c *Client) imageExists(ctx context.Context, ref string) (bool, error) {
	var inspect struct {
		ID string `json:"Id"`
	}
	err := c.getJSON(ctx, "/images/"+ref+"/json", nil, &inspect)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// loadImage posts an image tarball and reports the first error of the engine's message stream.
func (c *Client) loadImage(ctx context.Context, archive []byte) error {
	query := url.Values{"quiet": {"1"}}
	resp, err := c.do(ctx, http.MethodPost, "/images/load", query, bytes.NewReader(archive), "application/x-tar")
	if err != nil {
		return err
	}
	defer closeQuietly(resp.Body)
	return firstStreamError(io.LimitReader(resp.Body, maxResponseBytes))
}

// firstStreamError reads a JSON message stream (one object per message) and returns the first
// message that carries an error.
func firstStreamError(r io.Reader) error {
	decoder := json.NewDecoder(r)
	for {
		var message struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		err := decoder.Decode(&message)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode engine message stream: %w", err)
		}
		if text := cmp.Or(message.ErrorDetail.Message, message.Error); text != "" {
			return errors.New(text)
		}
	}
}

// pruneSamplerTags untags older sampler images. Failures are ignored: an image still used by a
// running sampler of another plugin instance must stay, and a leftover tag only costs disk.
func (c *Client) pruneSamplerTags(ctx context.Context, keep string) {
	filters, err := json.Marshal(map[string][]string{"reference": {samplerRepository}})
	if err != nil {
		return
	}
	var images []struct {
		RepoTags []string `json:"RepoTags"`
	}
	if err := c.getJSON(ctx, "/images/json", url.Values{"filters": {string(filters)}}, &images); err != nil {
		return
	}
	for _, image := range images {
		for _, tag := range image.RepoTags {
			if tag == keep || !strings.HasPrefix(tag, samplerRepository+":") {
				continue
			}
			_ = c.removeImage(ctx, tag, false) // best-effort, see the function comment
		}
	}
}

// removeImage deletes an image reference (DELETE /images/{ref}).
func (c *Client) removeImage(ctx context.Context, ref string, force bool) error {
	query := url.Values{}
	if force {
		query.Set("force", "1")
	}
	resp, err := c.do(ctx, http.MethodDelete, "/images/"+ref, query, nil, "")
	if err != nil {
		return fmt.Errorf("remove image %s: %w", ref, err)
	}
	defer closeQuietly(resp.Body)
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes)); err != nil {
		return fmt.Errorf("remove image %s: %w", ref, err)
	}
	return nil
}
