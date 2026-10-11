package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"aurago/internal/dockerutil"
)

// Archive download and streaming Engine requests for AuraGo-managed
// containers (CYD Creator builder). Both apply the runtime Docker gates of
// DockerRequestBytesContext: reads need docker.enabled, mutations also need
// docker.read_only off.

// ErrDockerArchiveNotFound is a GET /archive of a path that does not exist.
var ErrDockerArchiveNotFound = errors.New("docker archive path not found")

// ErrDockerArchiveTooLarge is a GET /archive answer above the caller's limit.
var ErrDockerArchiveTooLarge = errors.New("docker archive exceeds the size limit")

// DockerStreamContext sends an Engine API request and returns the open
// response so the caller can read the body while it arrives (exec output,
// image pull progress). Non-2xx answers are returned as responses; the
// caller must close the body. Cancelling ctx aborts the stream. The body is
// not size-capped: callers must bound what they read and keep (LimitReader,
// capped buffers, ReadErrorBody for error answers).
func DockerStreamContext(ctx context.Context, cfg DockerConfig, method, endpoint string, body []byte, contentType string) (*http.Response, error) {
	if dockerMethodMutates(method) {
		if err := requireDockerMutationPermission(); err != nil {
			return nil, err
		}
	} else if err := requireDockerPermission(); err != nil {
		return nil, err
	}
	if err := validateDockerCreateRequestBinds(cfg, method, endpoint, string(body)); err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost/"+dockerAPIVersion+endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := getPullDockerClient(cfg).Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker request failed: %w", err)
	}
	return resp, nil
}

// DockerGetArchiveContext downloads containerPath of a container as a tar
// archive (GET /containers/{id}/archive, socket proxy: CONTAINERS=1). An
// archive larger than maxBytes (required, > 0) is ErrDockerArchiveTooLarge.
// The Engine answers 404 both for a missing path and for a missing container;
// only its missing-path answer ("Could not find the file ...") is
// ErrDockerArchiveNotFound. "No such container" and every other 404 are plain
// errors, so a vanished container is never mistaken for an empty result.
func DockerGetArchiveContext(ctx context.Context, cfg DockerConfig, containerID, containerPath string, maxBytes int64) ([]byte, error) {
	if err := validateDockerName(containerID); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(containerPath, "/") || strings.Contains(containerPath, "..") {
		return nil, fmt.Errorf("invalid container path %q", containerPath)
	}
	if maxBytes <= 0 {
		return nil, errors.New("an archive size limit is required")
	}
	endpoint := "/containers/" + url.PathEscape(containerID) + "/archive?path=" + url.QueryEscape(containerPath)
	resp, err := DockerStreamContext(ctx, cfg, http.MethodGet, endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := dockerutil.ReadErrorBody(resp.Body)
		if resp.StatusCode == http.StatusNotFound && dockerArchivePathMissing(body) {
			return nil, ErrDockerArchiveNotFound
		}
		return nil, fmt.Errorf("docker archive download failed: HTTP %d: %s", resp.StatusCode, dockerBodyMessage(resp.StatusCode, body))
	}
	limit := maxBytes
	if limit < math.MaxInt64 {
		limit++ // one byte more detects an archive above maxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, fmt.Errorf("read docker archive: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrDockerArchiveTooLarge
	}
	return data, nil
}

// dockerArchivePathMissing reports whether a 404 body of GET /archive is the
// Engine's missing-path answer ("Could not find the file <path> in container
// <id>"), as opposed to "No such container: <id>" or a proxy's 404.
func dockerArchivePathMissing(body []byte) bool {
	msg := strings.ToLower(dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(body), 0))
	return strings.HasPrefix(msg, "could not find the file ")
}
