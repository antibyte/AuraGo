package decisions

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/dockerutil"
)

func ensureDockerRuntimeImages(ctx context.Context, access RuntimeAccess, cpuAllowed bool, progress func(string, int64, int64)) error {
	if !dockerMayMutate(access) {
		return NewError(CodeDeploymentDenied, nil)
	}
	if len(releaseRuntimeCatalog.Images) == 0 {
		err := verifyLocalDockerRuntimeImages(ctx, access, nil)
		if ErrorCode(err) == CodeRuntimeArtifactMissing {
			return NewError("runtime_release_unavailable", err)
		}
		return err
	}
	docker := newDecisionsDocker(access.DockerHost)
	defer docker.client.CloseIdleConnections()
	// Engine hardware belongs to the Docker host, not necessarily AuraGo's host.
	var info struct {
		Runtimes map[string]any `json:"Runtimes"`
	}
	if _, err := docker.client.DoJSON(ctx, http.MethodGet, "info", nil, &info); err != nil {
		return NewError(CodeDockerUnavailable, err)
	}
	backends := []string{"vulkan"}
	if _, ok := info.Runtimes["nvidia"]; ok {
		backends = append([]string{"cuda"}, backends...)
	}
	if cpuAllowed {
		backends = append(backends, "cpu")
	}
	for _, backend := range backends {
		reference := releaseRuntimeCatalog.image(backend)
		const prefix = "ghcr.io/antibyte/aurago-decisions-runtime@sha256:"
		if !strings.HasPrefix(reference, prefix) || !validSHA256(strings.TrimPrefix(reference, prefix)) {
			return NewError("runtime_release_unavailable", nil)
		}
		var existing dockerImageMetadata
		status, err := docker.client.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(reference)+"/json", nil, &existing)
		if err != nil && status != http.StatusNotFound {
			return NewError(CodeDockerUnavailable, err)
		}
		if status == http.StatusNotFound {
			if progress != nil {
				progress("downloading_runtime", 0, 0)
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(reference)), nil)
			if err != nil {
				return NewError("runtime_download_failed", err)
			}
			response, err := docker.client.HTTPClientWithTimeout(6 * time.Hour).Do(request)
			if err != nil {
				return NewError("runtime_download_failed", err)
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				response.Body.Close()
				return NewError("runtime_download_failed", fmt.Errorf("Docker pull returned %d", response.StatusCode))
			}
			layers := map[string][2]int64{}
			err = dockerutil.DrainJSONMessagesWithProgress(response.Body, func(id string, current, total int64) {
				if len(layers) >= 1024 {
					return
				}
				layers[id] = [2]int64{current, total}
				var downloaded, size int64
				for _, layer := range layers {
					downloaded += layer[0]
					size += layer[1]
				}
				if progress != nil {
					progress("downloading_runtime", downloaded, size)
				}
			})
			response.Body.Close()
			if err != nil {
				return NewError("runtime_download_failed", err)
			}
		}
		if _, err := docker.image(ctx, backend); err != nil {
			return NewError(CodeRuntimeAttestationFailed, err)
		}
	}
	return nil
}
