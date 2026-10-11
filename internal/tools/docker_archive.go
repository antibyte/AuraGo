package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
// caller must close the body. Cancelling ctx aborts the stream.
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
// archive (GET /containers/{id}/archive, socket proxy: CONTAINERS=1). A
// missing path is ErrDockerArchiveNotFound, an archive larger than maxBytes
// ErrDockerArchiveTooLarge.
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
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrDockerArchiveNotFound
	}
	if resp.StatusCode != http.StatusOK {
		msg := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 300)
		return nil, fmt.Errorf("docker archive download failed: HTTP %d: %s", resp.StatusCode, msg)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read docker archive: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrDockerArchiveTooLarge
	}
	return data, nil
}
