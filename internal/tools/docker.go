package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DockerConfig holds the Docker Engine connection parameters.
type DockerConfig struct {
	Host string // e.g. "unix:///var/run/docker.sock" or "tcp://localhost:2375"
}

// dockerHTTPClient is a lazily-initialized shared Docker API client.
var dockerHTTPClient *http.Client
var dockerHTTPClientHost string
var dockerClientMu sync.Mutex

// reDockerSafeName validates Docker container/image identifiers.
var reDockerSafeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:\-/]*$`)

// dockerAPIVersion is the Docker Engine API version used for all requests.
const dockerAPIVersion = "v1.45"

// maxDockerNameLength is Docker's maximum allowed length for container/image names.
const maxDockerNameLength = 255

// getDockerClient returns a shared *http.Client that talks to the Docker Engine API.
// The client is reused across requests for connection pooling.
func getDockerClient(cfg DockerConfig) *http.Client {
	dockerClientMu.Lock()
	defer dockerClientMu.Unlock()

	host := cfg.Host
	if host == "" {
		if runtime.GOOS == "windows" {
			host = "npipe:////./pipe/docker_engine"
		} else {
			host = "unix:///var/run/docker.sock"
		}
	}

	// Reuse client if host hasn't changed
	if dockerHTTPClient != nil && dockerHTTPClientHost == host {
		return dockerHTTPClient
	}

	transport := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			switch {
			case strings.HasPrefix(host, "unix://"):
				return net.DialTimeout("unix", strings.TrimPrefix(host, "unix://"), 5*time.Second)
			case strings.HasPrefix(host, "npipe://"):
				// Windows named pipes cannot be dialed with the standard net package.
				// Docker Desktop exposes the API via TCP on localhost:2375 by default,
				// but TCP must be enabled in Docker Desktop settings first.
				return nil, fmt.Errorf("windows named pipes (npipe://) require Docker Desktop settings to enable TCP (Settings > General > Expose daemon on tcp://localhost:2375), then use DOCKER_HOST=tcp://localhost:2375 instead")
			case strings.HasPrefix(host, "tcp://"):
				return net.DialTimeout("tcp", strings.TrimPrefix(host, "tcp://"), 5*time.Second)
			default:
				return net.DialTimeout("tcp", host, 5*time.Second)
			}
		},
	}

	dockerHTTPClient = &http.Client{Transport: transport, Timeout: 60 * time.Second}
	dockerHTTPClientHost = host
	return dockerHTTPClient
}

// DockerPing checks if the Docker Engine is reachable at the given host.
// Returns nil on success, or an error describing the failure.
func DockerPing(host string) error {
	cfg := DockerConfig{Host: host}
	client := getDockerClient(cfg)
	reqURL := "http://localhost/" + dockerAPIVersion + "/_ping"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("build ping request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("docker unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker _ping returned status %d", resp.StatusCode)
	}
	return nil
}

// validateDockerName checks that a container/image identifier is safe to use in API paths.
func validateDockerName(name string) error {
	if name == "" {
		return fmt.Errorf("name/ID is required")
	}
	if len(name) > maxDockerNameLength {
		return fmt.Errorf("Docker name exceeds maximum length of %d characters", maxDockerNameLength)
	}
	if !reDockerSafeName.MatchString(name) {
		return fmt.Errorf("invalid Docker name/ID: contains unsafe characters")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid Docker name/ID: path traversal blocked")
	}
	return nil
}

// dockerRequest performs a request against the Docker Engine API.
func dockerRequest(cfg DockerConfig, method, endpoint string, body string) ([]byte, int, error) {
	return dockerRequestWithRetry(cfg, method, endpoint, body, 3)
}

// dockerRequestWithRetry performs a request against the Docker Engine API with retry logic.
// It retries on transient errors (network timeouts, 5xx errors) with exponential backoff.
func dockerRequestWithRetry(cfg DockerConfig, method, endpoint string, body string, maxRetries int) ([]byte, int, error) {
	var lastErr error
	var lastCode int

	for i := 0; i < maxRetries; i++ {
		client := getDockerClient(cfg)

		var reqBody io.Reader
		if body != "" {
			reqBody = strings.NewReader(body)
		}

		// Docker Engine API is accessed via http://localhost but routed through the Unix socket.
		reqURL := "http://localhost/" + dockerAPIVersion + endpoint
		req, err := http.NewRequest(method, reqURL, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// Retry on network errors
			if i < maxRetries-1 {
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
			return nil, 0, fmt.Errorf("docker request failed after %d retries: %w", maxRetries, err)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close() // Close immediately instead of defer to avoid FD leak in retry loop
		if err != nil {
			lastErr = err
			lastCode = resp.StatusCode
			// Retry on read errors
			if i < maxRetries-1 {
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
			return nil, resp.StatusCode, fmt.Errorf("failed to read docker response after %d retries: %w", maxRetries, err)
		}

		// Retry on 5xx server errors
		if resp.StatusCode >= 500 && i < maxRetries-1 {
			lastErr = fmt.Errorf("server error HTTP %d", resp.StatusCode)
			lastCode = resp.StatusCode
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		return data, resp.StatusCode, nil
	}

	return nil, lastCode, fmt.Errorf("docker request failed after %d retries: %w", maxRetries, lastErr)
}

// DockerRequest is the exported variant of dockerRequest for use by other packages.
func DockerRequest(cfg DockerConfig, method, endpoint string, body string) ([]byte, int, error) {
	return dockerRequest(cfg, method, endpoint, body)
}

// errJSON is a helper that returns a JSON error string.
// Uses proper marshaling to handle special characters (quotes, newlines, etc.) in messages.
func errJSON(msg string, args ...interface{}) string {
	text := fmt.Sprintf(msg, args...)
	b, _ := json.Marshal(map[string]string{"status": "error", "message": text})
	return string(b)
}

// dockerBodyErr extracts the human-readable message from a Docker Engine API error body
// (which wraps errors as {"message":"..."}) and returns a safe JSON error string.
// Falls back to a generic message if the body cannot be parsed.
func dockerBodyErr(code int, body []byte) string {
	var dockerMsg struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &dockerMsg) == nil && dockerMsg.Message != "" {
		return errJSON("Docker error (HTTP %d): %s", code, dockerMsg.Message)
	}
	return errJSON("Docker error (HTTP %d)", code)
}

// ---------- Operations ----------

// DockerListContainers returns a list of containers (optionally all, not just running).
func DockerListContainers(cfg DockerConfig, all bool) string {
	endpoint := "/containers/json"
	if all {
		endpoint += "?all=true"
	}
	data, code, err := dockerRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to list containers: %v", err)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(data, &containers); err != nil {
		return errJSON("Failed to parse containers: %v", err)
	}

	// Compact: return only the most useful fields
	type compact struct {
		ID     string   `json:"id"`
		Names  []string `json:"names"`
		Image  string   `json:"image"`
		State  string   `json:"state"`
		Status string   `json:"status"`
		Health string   `json:"health,omitempty"`
	}
	var result []compact
	for _, c := range containers {
		entry := compact{
			Image:  fmt.Sprintf("%v", c["Image"]),
			State:  fmt.Sprintf("%v", c["State"]),
			Status: fmt.Sprintf("%v", c["Status"]),
		}
		if id, ok := c["Id"].(string); ok && len(id) > 12 {
			entry.ID = id[:12]
		} else {
			entry.ID = fmt.Sprintf("%v", c["Id"])
		}
		if names, ok := c["Names"].([]interface{}); ok {
			for _, n := range names {
				entry.Names = append(entry.Names, fmt.Sprintf("%v", n))
			}
		}
		// Extract health status from State object if available
		if state, ok := c["State"].(map[string]interface{}); ok {
			if health, ok := state["Health"].(map[string]interface{}); ok {
				if status, ok := health["Status"].(string); ok {
					entry.Health = status
				}
			}
		}
		result = append(result, entry)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "containers": result})
	return string(out)
}

// DockerInspectContainer returns detailed info about a specific container.
func DockerInspectContainer(cfg DockerConfig, containerID string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	data, code, err := dockerRequest(cfg, "GET", "/containers/"+url.PathEscape(containerID)+"/json", "")
	if err != nil {
		return errJSON("Failed to inspect container: %v", err)
	}
	if code == 404 {
		return errJSON("Container '%s' not found", containerID)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
	}

	// Parse and return a trimmed version
	var full map[string]interface{}
	if err := json.Unmarshal(data, &full); err != nil {
		return errJSON("Failed to parse inspect data: %v", err)
	}

	// Extract the most useful fields
	result := map[string]interface{}{
		"status": "ok",
		"id":     full["Id"],
		"name":   full["Name"],
		"state":  full["State"],
		"config": nil,
	}
	if cfg, ok := full["Config"].(map[string]interface{}); ok {
		result["config"] = map[string]interface{}{
			"image":  cfg["Image"],
			"env":    cfg["Env"],
			"cmd":    cfg["Cmd"],
			"labels": cfg["Labels"],
		}
	}
	if netSettings, ok := full["NetworkSettings"].(map[string]interface{}); ok {
		result["network"] = map[string]interface{}{
			"ip_address": netSettings["IPAddress"],
			"ports":      netSettings["Ports"],
		}
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// DockerContainerAction performs start, stop, restart, pause, unpause, or remove on a container.
func DockerContainerAction(cfg DockerConfig, containerID, action string, force bool) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}

	safe := url.PathEscape(containerID)
	var method, endpoint string
	switch action {
	case "start":
		method, endpoint = "POST", "/containers/"+safe+"/start"
	case "stop":
		method, endpoint = "POST", "/containers/"+safe+"/stop?t=10"
	case "restart":
		method, endpoint = "POST", "/containers/"+safe+"/restart?t=10"
	case "pause":
		method, endpoint = "POST", "/containers/"+safe+"/pause"
	case "unpause":
		method, endpoint = "POST", "/containers/"+safe+"/unpause"
	case "remove", "rm":
		q := "?v=true"
		if force {
			q += "&force=true"
		}
		method, endpoint = "DELETE", "/containers/"+safe+q
	default:
		return errJSON("Unknown container action: %s. Use: start, stop, restart, pause, unpause, remove", action)
	}

	data, code, err := dockerRequest(cfg, method, endpoint, "")
	if err != nil {
		return errJSON("Action '%s' failed: %v", action, err)
	}
	// 204 = success (no content), 304 = already in state
	if code == 204 || code == 304 {
		return fmt.Sprintf(`{"status":"ok","action":"%s","container_id":"%s"}`, action, containerID)
	}
	if code == 404 {
		return errJSON("Container '%s' not found", containerID)
	}
	return dockerBodyErr(code, data)
}

// DockerContainerLogs retrieves the last N lines of container logs.
func DockerContainerLogs(cfg DockerConfig, containerID string, tail int) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	if tail <= 0 {
		tail = 100
	}
	endpoint := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%d&timestamps=true", url.PathEscape(containerID), tail)
	data, code, err := dockerRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to get logs: %v", err)
	}
	if code == 404 {
		return errJSON("Container '%s' not found", containerID)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
	}

	// Docker log stream has 8-byte header per frame — strip it for readability
	lines := stripDockerLogHeaders(data)

	// Truncate if output is very large
	const maxLen = 8000
	if len(lines) > maxLen {
		lines = lines[len(lines)-maxLen:]
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "container_id": containerID, "logs": lines})
	return string(out)
}

// stripDockerLogHeaders removes the 8-byte Docker log stream headers.
func stripDockerLogHeaders(raw []byte) string {
	var sb strings.Builder
	for len(raw) >= 8 {
		// bytes 0: stream type (0=stdin, 1=stdout, 2=stderr)
		// bytes 4-7: big-endian uint32 frame size
		size := int(raw[4])<<24 | int(raw[5])<<16 | int(raw[6])<<8 | int(raw[7])
		raw = raw[8:]
		if size > len(raw) {
			size = len(raw)
		}
		sb.Write(raw[:size])
		raw = raw[size:]
	}
	// If parsing failed (e.g. TTY mode), return raw string
	if sb.Len() == 0 {
		return string(raw)
	}
	return sb.String()
}

// DockerListImages returns a list of local Docker images.
func DockerListImages(cfg DockerConfig) string {
	data, code, err := dockerRequest(cfg, "GET", "/images/json", "")
	if err != nil {
		return errJSON("Failed to list images: %v", err)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
	}

	var images []map[string]interface{}
	if err := json.Unmarshal(data, &images); err != nil {
		return errJSON("Failed to parse images: %v", err)
	}

	type compact struct {
		ID      string   `json:"id"`
		Tags    []string `json:"tags"`
		Size    int64    `json:"size_mb"`
		Created int64    `json:"created"`
	}
	var result []compact
	for _, img := range images {
		entry := compact{}
		if id, ok := img["Id"].(string); ok {
			entry.ID = strings.TrimPrefix(id, "sha256:")
			if len(entry.ID) > 12 {
				entry.ID = entry.ID[:12]
			}
		}
		if tags, ok := img["RepoTags"].([]interface{}); ok {
			for _, t := range tags {
				entry.Tags = append(entry.Tags, fmt.Sprintf("%v", t))
			}
		}
		if s, ok := img["Size"].(float64); ok {
			entry.Size = int64(s) / (1024 * 1024)
		}
		if c, ok := img["Created"].(float64); ok {
			entry.Created = int64(c)
		}
		result = append(result, entry)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "images": result})
	return string(out)
}

// PullImageWait pulls a Docker image and blocks until the pull completes (or the
// context expires). Unlike the shared HTTP client's 60-second timeout this uses a
// per-request context so long pulls don't get killed prematurely.
// It returns nil if the image already exists locally.
func PullImageWait(ctx context.Context, cfg DockerConfig, image string, logger *slog.Logger) error {
	// Check if image already exists.
	filterURL := fmt.Sprintf("/images/json?filters=%%7B%%22reference%%22%%3A%%5B%%22%s%%22%%5D%%7D", url.QueryEscape(image))
	data, code, err := dockerRequest(cfg, "GET", filterURL, "")
	if err == nil && code == 200 {
		var images []interface{}
		if json.Unmarshal(data, &images) == nil && len(images) > 0 {
			return nil // already present
		}
	}

	logger.Info("Pulling Docker image", "image", image)

	// Build the pull request with the caller-supplied context so long pulls
	// are not cut short by the 60-second client timeout.
	client := getDockerClient(cfg)
	reqURL := "http://localhost/" + dockerAPIVersion + "/images/create?fromImage=" + url.QueryEscape(image)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}
	defer resp.Body.Close()

	// Drain the streaming response (progress JSON) so we block until the
	// pull is fully complete.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("pull image %s (reading stream): %w", image, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull image %s: HTTP %d", image, resp.StatusCode)
	}

	logger.Info("Docker image pulled successfully", "image", image)
	return nil
}

// DockerPullImage pulls an image from a registry.
func DockerPullImage(cfg DockerConfig, image string) string {
	if image == "" {
		return errJSON("image name is required")
	}
	endpoint := "/images/create?fromImage=" + url.QueryEscape(image)
	data, code, err := dockerRequest(cfg, "POST", endpoint, "")
	if err != nil {
		return errJSON("Failed to pull image: %v", err)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
	}
	return fmt.Sprintf(`{"status":"ok","message":"Image '%s' pulled successfully"}`, image)
}

// DockerRemoveImage deletes a local image.
func DockerRemoveImage(cfg DockerConfig, image string, force bool) string {
	if image == "" {
		return errJSON("image name or ID is required")
	}
	endpoint := "/images/" + url.PathEscape(image)
	if force {
		endpoint += "?force=true"
	}
	data, code, err := dockerRequest(cfg, "DELETE", endpoint, "")
	if err != nil {
		return errJSON("Failed to remove image: %v", err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","message":"Image '%s' removed"}`, image)
	}
	if code == 404 {
		return errJSON("Image '%s' not found", image)
	}
	return dockerBodyErr(code, data)
}

// DockerRenameContainer renames a container.
func DockerRenameContainer(cfg DockerConfig, containerID, newName string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	if newName == "" {
		return errJSON("new name is required")
	}
	if err := validateDockerName(newName); err != nil {
		return errJSON("invalid new name: %v", err)
	}
	endpoint := "/containers/" + url.PathEscape(containerID) + "/rename?name=" + url.QueryEscape(newName)
	data, code, err := dockerRequest(cfg, "POST", endpoint, "")
	if err != nil {
		return errJSON("Failed to rename container: %v", err)
	}
	if code == 204 || code == 200 {
		return fmt.Sprintf(`{"status":"ok","message":"Container renamed to '%s'"}`, newName)
	}
	if code == 404 {
		return errJSON("Container '%s' not found", containerID)
	}
	return dockerBodyErr(code, data)
}

// DockerSystemPrune removes unused data (containers, networks, images, volumes).
// Warning: This is a destructive operation.
func DockerSystemPrune(cfg DockerConfig, all, volumes bool) string {
	endpoint := "/system/prune"
	if all {
		endpoint += "?all=true"
	}
	if volumes {
		if strings.Contains(endpoint, "?") {
			endpoint += "&volumes=true"
		} else {
			endpoint += "?volumes=true"
		}
	}
	data, code, err := dockerRequest(cfg, "POST", endpoint, "")
	if err != nil {
		return errJSON("Failed to prune system: %v", err)
	}
	if code == 200 {
		// Parse the response to show what was deleted
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return string(data)
		}
		out, _ := json.Marshal(map[string]interface{}{
			"status":                "ok",
			"message":               "System prune completed",
			"containers_deleted":    result["ContainersDeleted"],
			"space_reclaimed_bytes": result["SpaceReclaimed"],
			"images_deleted":        result["ImagesDeleted"],
			"networks_deleted":      result["NetworksDeleted"],
			"volumes_deleted":       result["VolumesDeleted"],
		})
		return string(out)
	}
	return dockerBodyErr(code, data)
}
