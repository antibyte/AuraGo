package tools

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	pathpkg "path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/acestep"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// DockerConfig holds the Docker Engine connection parameters.
type DockerConfig struct {
	Host         string // e.g. "unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine", or "tcp://localhost:2375"
	WorkspaceDir string // workspace root for validating host-side file paths used by docker cp
	// MinimalCLIEnvironment runs `docker compose` and its preflight resolution
	// with dockerCLIMinimalEnvironment instead of AuraGo's whole environment.
	// Agent dispatch sets it for compose while docker.allow_host_access is off.
	MinimalCLIEnvironment bool
}

// dockerHTTPClient is a lazily-initialized shared Docker API client (60s timeout).
var dockerHTTPClient *http.Client
var dockerHTTPClientHost string
var dockerClientMu sync.Mutex

// dockerPullHTTPClient is a no-timeout client for streaming pull responses.
// The caller-supplied context handles cancellation instead of a hard timeout.
var dockerPullHTTPClient *http.Client
var dockerPullHTTPClientHost string

// reDockerSafeName validates Docker container/image identifiers.
var reDockerSafeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:\-/]*$`)

// dockerAPIVersion is the Docker Engine API version used for all requests.
const dockerAPIVersion = dockerutil.APIVersion

// maxDockerNameLength is Docker's maximum allowed length for container/image names.
const maxDockerNameLength = 255

// getDockerClient returns a shared *http.Client that talks to the Docker Engine API.
// The client is reused across requests for connection pooling.
func getDockerClient(cfg DockerConfig) *http.Client {
	dockerClientMu.Lock()
	defer dockerClientMu.Unlock()

	host := dockerutil.NormalizeHost(cfg.Host)

	// Reuse client if host hasn't changed
	if dockerHTTPClient != nil && dockerHTTPClientHost == host {
		return dockerHTTPClient
	}

	transport := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dockerDialContext(ctx, host)
		},
	}

	dockerHTTPClient = &http.Client{Transport: dockerutil.NewVersionTransport(transport), Timeout: 60 * time.Second}
	dockerHTTPClientHost = host
	return dockerHTTPClient
}

// getPullDockerClient returns a shared *http.Client without a response timeout,
// suitable for streaming responses like image pulls. Cancellation is handled
// via the per-request context supplied by the caller.
func getPullDockerClient(cfg DockerConfig) *http.Client {
	dockerClientMu.Lock()
	defer dockerClientMu.Unlock()

	host := dockerutil.NormalizeHost(cfg.Host)

	if dockerPullHTTPClient != nil && dockerPullHTTPClientHost == host {
		return dockerPullHTTPClient
	}

	transport := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dockerDialContext(ctx, host)
		},
	}

	// No Timeout — streaming responses run until context is cancelled.
	dockerPullHTTPClient = &http.Client{Transport: dockerutil.NewVersionTransport(transport)}
	dockerPullHTTPClientHost = host
	return dockerPullHTTPClient
}

// dockerPullFallbackTimeout bounds an image pull whose context has no deadline.
const dockerPullFallbackTimeout = 15 * time.Minute

// dockerPullError is a failed image pull. StatusCode is the Engine's non-2xx
// answer (0 when the pull failed without one) and Message its text. Stream is
// true when the Engine answered 2xx and the progress stream then failed (an
// error event, a cut stream or a read error). Err is the transport, context or
// stream failure when StatusCode is 0. Error() keeps the historic
// "pull image <ref>: ..." wording of PullImageWait and PullImageForce.
type dockerPullError struct {
	Image      string
	StatusCode int
	Message    string
	Stream     bool
	Err        error
}

func (e *dockerPullError) Error() string {
	switch {
	case e.StatusCode != 0 && e.Message != "":
		return fmt.Sprintf("pull image %s: HTTP %d: %s", e.Image, e.StatusCode, e.Message)
	case e.StatusCode != 0:
		return fmt.Sprintf("pull image %s: HTTP %d", e.Image, e.StatusCode)
	default:
		return fmt.Sprintf("pull image %s: %v", e.Image, e.Err)
	}
}

func (e *dockerPullError) Unwrap() error { return e.Err }

// pullDockerImageStream pulls image (a reference as the Engine accepts it in
// fromImage) on the streaming pull client and reads the Engine's progress
// stream to its end, so a nil result means the pull completed. It does not
// check runtime permissions: every caller applies the gate it always had.
// ctx bounds the pull; without a deadline it is bounded to
// dockerPullFallbackTimeout. Every error is a *dockerPullError.
func pullDockerImageStream(ctx context.Context, cfg DockerConfig, image string) error {
	return pullDockerImageQuery(ctx, cfg, image, url.Values{"fromImage": {image}})
}

// pullDockerImageQuery is pullDockerImageStream with an explicit
// /images/create query (fromImage plus tag); image names the pull in errors.
func pullDockerImageQuery(ctx context.Context, cfg DockerConfig, image string, query url.Values) error {
	ctx, cancel := dockerContextWithFallbackTimeout(ctx, dockerPullFallbackTimeout)
	defer cancel()
	reqURL := "http://localhost/" + dockerAPIVersion + "/images/create?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return &dockerPullError{Image: image, Err: fmt.Errorf("create pull request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := getPullDockerClient(cfg).Do(req)
	if err != nil {
		return &dockerPullError{Image: image, Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &dockerPullError{Image: image, StatusCode: resp.StatusCode, Message: dockerBodyMessage(resp.StatusCode, dockerutil.ReadErrorBody(resp.Body))}
	}
	if err := dockerutil.DrainJSONMessages(resp.Body); err != nil {
		return &dockerPullError{Image: image, Stream: true, Err: err}
	}
	return nil
}

// detachedPullContext keeps ctx's values but drops its deadline and
// cancellation. It is for pull sites that ran on the 60-second request client
// and never honoured their caller's context: a short caller deadline must not
// cut a pull that works today. The pull is then bounded by
// dockerPullFallbackTimeout.
func detachedPullContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// lifetimePullContext is detachedPullContext bound to an owner's lifetime: the
// pull ignores ctx's deadline and cancellation but stops as soon as lifetime
// ends (server shutdown), so shutdown never waits for it. A nil lifetime
// leaves only dockerPullFallbackTimeout. Call the returned cancel when the
// pull is done.
func lifetimePullContext(ctx, lifetime context.Context) (context.Context, context.CancelFunc) {
	pullCtx, cancel := context.WithCancel(detachedPullContext(ctx))
	if lifetime == nil {
		return pullCtx, cancel
	}
	stop := context.AfterFunc(lifetime, cancel)
	return pullCtx, func() {
		stop()
		cancel()
	}
}

// pullImageBestEffort pulls image for a sidecar whose container create runs
// even when the pull fails (an image that already exists still works). It
// checks the Docker mutation gate first, like the request client it replaces,
// and runs on the streaming pull client. ctx bounds the pull; without a
// deadline it is bounded to dockerPullFallbackTimeout. A caller that holds a
// lock while it pulls must pass a deadline no longer than the lock may be held.
func pullImageBestEffort(ctx context.Context, dockerCfg DockerConfig, image string) error {
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	return pullDockerImageStream(ctx, dockerCfg, image)
}

// DockerPing checks if the Docker Engine is reachable at the given host.
// Returns nil on success, or an error describing the failure.
func DockerPing(host string) error {
	if err := requireDockerPermission(); err != nil {
		return err
	}
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

func dockerDialContext(ctx context.Context, host string) (net.Conn, error) {
	return dockerutil.DialContext(ctx, host)
}

func normalizeDockerNamedPipeHost(host string) (string, error) {
	return dockerutil.NormalizeNamedPipeHost(host)
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
	if dockerMethodMutates(method) {
		if err := requireDockerMutationPermission(); err != nil {
			return nil, 0, err
		}
	} else if err := requireDockerPermission(); err != nil {
		return nil, 0, err
	}
	if err := validateDockerCreateRequestBinds(cfg, method, endpoint, body); err != nil {
		return nil, 0, err
	}
	return dockerRequestWithRetry(cfg, method, endpoint, body, 3)
}

func dockerMethodMutates(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, "":
		return false
	default:
		return true
	}
}

// dockerRequestWithRetry performs a request against the Docker Engine API with retry logic.
// Only GET and HEAD may retry transient errors. A mutation may already have
// succeeded when its response is lost and must never be sent a second time.
func dockerRequestWithRetry(cfg DockerConfig, method, endpoint string, body string, maxRetries int) ([]byte, int, error) {
	if dockerMethodMutates(method) || maxRetries < 1 {
		maxRetries = 1
	}
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

		data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
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

// DockerRequestContext performs a Docker Engine API request with caller-managed
// cancellation. It is intended for long-running calls such as exec start, where
// a fixed client timeout would cut off valid work before the caller's deadline.
func DockerRequestContext(ctx context.Context, cfg DockerConfig, method, endpoint string, body string) ([]byte, int, error) {
	if dockerMethodMutates(method) {
		if err := requireDockerMutationPermission(); err != nil {
			return nil, 0, err
		}
	} else if err := requireDockerPermission(); err != nil {
		return nil, 0, err
	}
	if err := validateDockerCreateRequestBinds(cfg, method, endpoint, body); err != nil {
		return nil, 0, err
	}
	return dockerRequestContextValidated(ctx, cfg, method, endpoint, body)
}

// DockerCreateRequestContextWithTrustedBinds posts a /containers/create body
// like DockerRequestContext, but HostConfig.Binds strings listed exactly in
// trusted skip validateDockerBindMount. Only code-pinned callers use it (the
// Software Store adapter passes its catalog's own host binds); agent dispatch
// and every other caller keep the full bind policy. The endpoint path must be
// exactly /containers/create; a query such as ?name= is allowed.
func DockerCreateRequestContextWithTrustedBinds(ctx context.Context, cfg DockerConfig, endpoint, body string, trusted []string) ([]byte, int, error) {
	if !dockerEndpointIsContainerCreate(endpoint) {
		return nil, 0, fmt.Errorf("trusted Docker binds apply only to /containers/create")
	}
	if err := requireDockerMutationPermission(); err != nil {
		return nil, 0, err
	}
	if err := validateDockerCreateRequestBindsTrusted(cfg, http.MethodPost, endpoint, body, trusted); err != nil {
		return nil, 0, err
	}
	return dockerRequestContextValidated(ctx, cfg, http.MethodPost, endpoint, body)
}

// dockerEndpointIsContainerCreate reports whether endpoint is a relative
// reference whose path is exactly /containers/create, spelled without
// percent-encoding, with an optional query and no fragment.
func dockerEndpointIsContainerCreate(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	return parsed.Scheme == "" && parsed.Opaque == "" && parsed.User == nil && parsed.Host == "" &&
		parsed.Fragment == "" && parsed.Path == "/containers/create" && parsed.EscapedPath() == "/containers/create"
}

// dockerRequestContextValidated sends a request whose gates and bind checks
// already ran.
func dockerRequestContextValidated(ctx context.Context, cfg DockerConfig, method, endpoint, body string) ([]byte, int, error) {
	client := getPullDockerClient(cfg)
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	reqURL := "http://localhost/" + dockerAPIVersion + endpoint
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("docker request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read docker response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// DockerRequestBytesContext performs a Docker Engine API request with a binary
// request body and caller-selected content type.
func DockerRequestBytesContext(ctx context.Context, cfg DockerConfig, method, endpoint string, body []byte, contentType string) ([]byte, int, error) {
	if dockerMethodMutates(method) {
		if err := requireDockerMutationPermission(); err != nil {
			return nil, 0, err
		}
	} else if err := requireDockerPermission(); err != nil {
		return nil, 0, err
	}
	client := getPullDockerClient(cfg)
	reqURL := "http://localhost/" + dockerAPIVersion + endpoint
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("docker request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read docker response: %w", err)
	}
	return data, resp.StatusCode, nil
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
	msg := dockerBodyMessage(code, body)
	if msg != "" {
		return errJSON("Docker error (HTTP %d): %s", code, msg)
	}
	return errJSON("Docker error (HTTP %d)", code)
}

// dockerBodyMessageMaxBytes bounds the Engine text that tool results, errors
// and logs carry from one non-2xx answer.
const dockerBodyMessageMaxBytes = 500

// dockerBodyMessage returns the Engine's text for a non-2xx answer (the JSON
// "message", otherwise the body) as one printable line of at most
// dockerBodyMessageMaxBytes bytes, plus "..." when it was cut. code is kept
// for the existing call sites.
func dockerBodyMessage(code int, body []byte) string {
	text := dockerutil.EngineErrorMessage(body)
	line := dockerutil.SanitizeOneLine(text, 0)
	if len(line) <= dockerBodyMessageMaxBytes {
		return line
	}
	return dockerutil.SanitizeOneLine(text, dockerBodyMessageMaxBytes) + "..."
}

// ---------- Operations ----------

// DockerListContainers returns a list of containers (optionally all, not just
// running). excludedOwners is used by the agent surface to hide protected
// AuraGo-managed sidecars without changing administrator container APIs.
func DockerListContainers(cfg DockerConfig, all bool, excludedOwners ...string) string {
	if err := requireDockerPermission(); err != nil {
		return errJSON("%v", err)
	}
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
		labels := dockerStringLabels(c["Labels"])
		names := dockerInterfaceStrings(c["Names"])
		if dockerManagedResourceExcluded(labels, names, false, excludedOwners) {
			continue
		}
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
		entry.Names = append(entry.Names, names...)
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

// DockerContainerListEntry is one compact container list entry. FullID,
// Labels, NetworkIPs and NetworkMode are never serialized; administrator
// surfaces use them to classify the entry.
type DockerContainerListEntry struct {
	ID          string            `json:"id"`
	Names       []string          `json:"names"`
	Image       string            `json:"image"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Health      string            `json:"health,omitempty"`
	FullID      string            `json:"-"`
	Labels      map[string]string `json:"-"`
	NetworkIPs  []string          `json:"-"`
	NetworkMode string            `json:"-"` // HostConfig.NetworkMode, e.g. "container:<id>"
}

// DockerListContainerEntries returns the entries DockerListContainers
// serializes, plus their full ID, labels, network addresses and network mode.
// On failure entries is nil and failure holds the error JSON
// DockerListContainers returns.
// Keep the parsing in step with DockerListContainers
// (TestDockerListContainerEntriesMatchesDockerListContainers).
func DockerListContainerEntries(cfg DockerConfig, all bool, excludedOwners ...string) (entries []DockerContainerListEntry, failure string) {
	if err := requireDockerPermission(); err != nil {
		return nil, errJSON("%v", err)
	}
	endpoint := "/containers/json"
	if all {
		endpoint += "?all=true"
	}
	data, code, err := dockerRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return nil, errJSON("Failed to list containers: %v", err)
	}
	if code != 200 {
		return nil, dockerBodyErr(code, data)
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, errJSON("Failed to parse containers: %v", err)
	}

	for _, c := range containers {
		labels := dockerStringLabels(c["Labels"])
		names := dockerInterfaceStrings(c["Names"])
		if dockerManagedResourceExcluded(labels, names, false, excludedOwners) {
			continue
		}
		entry := DockerContainerListEntry{
			Image:  fmt.Sprintf("%v", c["Image"]),
			State:  fmt.Sprintf("%v", c["State"]),
			Status: fmt.Sprintf("%v", c["Status"]),
			Labels: labels,
		}
		if id, ok := c["Id"].(string); ok && len(id) > 12 {
			entry.ID = id[:12]
			entry.FullID = id
		} else {
			entry.ID = fmt.Sprintf("%v", c["Id"])
			entry.FullID = entry.ID
		}
		entry.Names = append(entry.Names, names...)
		// Extract health status from State object if available
		if state, ok := c["State"].(map[string]interface{}); ok {
			if health, ok := state["Health"].(map[string]interface{}); ok {
				if status, ok := health["Status"].(string); ok {
					entry.Health = status
				}
			}
		}
		if hostConfig, ok := c["HostConfig"].(map[string]interface{}); ok {
			if mode, ok := hostConfig["NetworkMode"].(string); ok {
				entry.NetworkMode = strings.TrimSpace(mode)
			}
		}
		if settings, ok := c["NetworkSettings"].(map[string]interface{}); ok {
			if networks, ok := settings["Networks"].(map[string]interface{}); ok {
				for _, raw := range networks {
					network, ok := raw.(map[string]interface{})
					if !ok {
						continue
					}
					for _, key := range []string{"IPAddress", "GlobalIPv6Address"} {
						if ip, ok := network[key].(string); ok && strings.TrimSpace(ip) != "" {
							entry.NetworkIPs = append(entry.NetworkIPs, strings.TrimSpace(ip))
						}
					}
				}
			}
		}
		entries = append(entries, entry)
	}
	return entries, ""
}

// dockerInspectRedacted replaces secret values in Docker inspect output.
const dockerInspectRedacted = "••••••••"

// DockerInspectContainer returns detailed info about a specific container for
// the agent and the UI: credential-like environment values, command-line
// secrets and labels are masked, and bind-mount sources are reduced to their
// last path element so host directory layouts are not disclosed.
func DockerInspectContainer(cfg DockerConfig, containerID string) string {
	return dockerInspectContainer(cfg, containerID, false)
}

// DockerInspectContainerWithMountSources is DockerInspectContainer with full
// bind-mount source paths, for trusted in-process callers that compare a
// mount with a host path (the Code Studio workspace check). Its output must
// not be handed to the model or the UI.
func DockerInspectContainerWithMountSources(cfg DockerConfig, containerID string) string {
	return dockerInspectContainer(cfg, containerID, true)
}

func dockerInspectContainer(cfg DockerConfig, containerID string, fullBindSources bool) string {
	if err := requireDockerPermission(); err != nil {
		return errJSON("%v", err)
	}
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
		"mounts": projectDockerInspectMounts(full["Mounts"], fullBindSources),
		"config": nil,
	}
	if cfg, ok := full["Config"].(map[string]interface{}); ok {
		result["config"] = map[string]interface{}{
			"image":  cfg["Image"],
			"env":    redactDockerInspectEnv(cfg["Env"]),
			"cmd":    redactDockerInspectArgs(cfg["Cmd"]),
			"labels": redactDockerInspectLabels(cfg["Labels"]),
		}
	}
	if netSettings, ok := full["NetworkSettings"].(map[string]interface{}); ok {
		result["network"] = map[string]interface{}{
			"ip_address": netSettings["IPAddress"],
			"ports":      netSettings["Ports"],
		}
	}
	if hostConfig, ok := full["HostConfig"].(map[string]interface{}); ok {
		if readonly, ok := hostConfig["ReadonlyRootfs"].(bool); ok && readonly {
			result["readonly_rootfs"] = true
		}
		if tmpfs := dockerInspectTmpfs(hostConfig["Tmpfs"]); len(tmpfs) > 0 {
			result["tmpfs"] = tmpfs
		}
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// dockerInspectTmpfs copies HostConfig.Tmpfs into string pairs. Non-string
// values are dropped so the trimmed inspect stays a plain map.
func dockerInspectTmpfs(value interface{}) map[string]string {
	items, ok := value.(map[string]interface{})
	if !ok || len(items) == 0 {
		return nil
	}
	copied := make(map[string]string, len(items))
	for key, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(key) == "" || text == "" {
			continue
		}
		copied[key] = text
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// dockerInspectMountFields maps the Docker mount fields kept in inspect output
// to their output names; driver options, propagation and the like are dropped.
var dockerInspectMountFields = map[string]string{
	"Type":        "type",
	"Name":        "name",
	"Destination": "destination",
	"Mode":        "mode",
	"RW":          "rw",
	"Source":      "source",
}

// projectDockerInspectMounts keeps the useful fields of each mount. A volume's
// source stays complete (it names Docker's own storage); any other source (a
// bind's host path) keeps only its last path element unless fullBindSources
// is set. Docker sends an array of objects; any other shape, and any entry
// that is not an object, is dropped rather than passed through unredacted.
func projectDockerInspectMounts(value interface{}, fullBindSources bool) interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	projected := make([]interface{}, 0, len(items))
	for _, item := range items {
		mount, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entry := make(map[string]interface{}, len(dockerInspectMountFields))
		for from, to := range dockerInspectMountFields {
			if v, present := mount[from]; present {
				entry[to] = v
			}
		}
		if source, ok := entry["source"].(string); ok && source != "" && !fullBindSources && entry["type"] != "volume" {
			entry["source"] = pathpkg.Base(strings.ReplaceAll(source, `\`, "/"))
		}
		projected = append(projected, entry)
	}
	return projected
}

func redactDockerInspectEnv(value interface{}) interface{} {
	switch items := value.(type) {
	case []string:
		converted := make([]interface{}, len(items))
		for i, item := range items {
			converted[i] = item
		}
		return redactDockerInspectEnv(converted)
	case []interface{}:
		redacted := make([]interface{}, len(items))
		for i, item := range items {
			text, ok := item.(string)
			if !ok {
				redacted[i] = item
				continue
			}
			if key, val, found := strings.Cut(text, "="); found {
				if dockerInspectEnvKeySensitive(key) {
					redacted[i] = key + "=" + dockerInspectRedacted
				} else {
					redacted[i] = key + "=" + redactDockerInspectEnvValue(val)
				}
				continue
			}
			redacted[i] = security.RedactSensitiveInfo(text)
		}
		return redacted
	default:
		return value
	}
}

func dockerInspectEnvKeySensitive(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "" {
		return false
	}
	if strings.HasPrefix(upper, "AURAGO_") {
		return true
	}
	// Unambiguous credential words are matched anywhere in the key, so a glued
	// prefix (WEBPASSWORD, PGPASSWORD, TS_AUTHKEY, APP_APIKEY, X_API_KEY_FILE)
	// is masked, not just a trailing _PASSWORD. These never occur inside a
	// non-secret variable name in practice.
	for _, marker := range []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "AUTHKEY", "APIKEY", "API_KEY"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	// Short/ambiguous markers stay anchored (exact or _SUFFIX) so PASSPORT,
	// COMPASS, a bare PWD (the working directory) and the like are not masked.
	for _, suffix := range []string{
		"ACCESS_KEY", "PRIVATE_KEY", "MASTER_KEY",
		"PASS", "PASSPHRASE", "CREDENTIALS", "REQUIREPASS", "MASTERAUTH",
		"SECRET_KEY", "SECRET_KEY_BASE", "ENCRYPTION_KEY", "APP_KEY",
	} {
		if upper == suffix || strings.HasSuffix(upper, "_"+suffix) {
			return true
		}
	}
	// PWD only as a suffix (DB_PWD): the shell's bare PWD is the working
	// directory and stays visible.
	return strings.HasSuffix(upper, "_PWD")
}

// redactDockerInspectEnvValue redacts the value of a non-sensitive environment
// variable in two passes. First the whole value goes through the scrubber, so
// its patterns that span whitespace (a bearer header, key = value, key: value,
// URL credentials) mask as they did before. Then the scrubbed value is treated
// as a command line, so a credential flag embedded in it
// (REDIS_ARGS=--requirepass x, VALKEY_EXTRA_FLAGS=--masterauth=y) loses the
// argument after the flag. Limits: the second pass splits on whitespace, so a
// quoted multi-word credential after a flag (--requirepass "two words") is only
// partly masked (its first word), and when that pass masks something the value's
// whitespace is collapsed to single spaces. A value with nothing for the second
// pass to mask is returned as scrubbed, spacing intact.
func redactDockerInspectEnvValue(val string) string {
	scrubbed := security.RedactSensitiveInfo(val)
	fields := strings.Fields(scrubbed)
	if len(fields) == 0 {
		return scrubbed
	}
	args := make([]interface{}, len(fields))
	for i, f := range fields {
		args[i] = f
	}
	redacted, ok := redactDockerInspectArgs(args).([]interface{})
	if !ok {
		return scrubbed
	}
	parts := make([]string, len(redacted))
	changed := false
	for i, r := range redacted {
		if s, isString := r.(string); isString {
			parts[i] = s
		} else {
			parts[i] = fmt.Sprint(r)
		}
		if parts[i] != fields[i] {
			changed = true
		}
	}
	if !changed {
		return scrubbed
	}
	return strings.Join(parts, " ")
}

// redactDockerInspectArgs masks credential values in a command line: the value
// of "--flag=value" and the argument after "--flag" when the flag names a
// credential, plus URL credentials and key=value secrets in any argument.
func redactDockerInspectArgs(value interface{}) interface{} {
	var items []interface{}
	switch typed := value.(type) {
	case []string:
		items = make([]interface{}, len(typed))
		for i, item := range typed {
			items[i] = item
		}
	case []interface{}:
		items = typed
	default:
		return value
	}
	redacted := make([]interface{}, len(items))
	maskNext := false
	for i, item := range items {
		text, ok := item.(string)
		if !ok {
			redacted[i] = item
			maskNext = false
			continue
		}
		if maskNext {
			redacted[i] = dockerInspectRedacted
			maskNext = false
			continue
		}
		if flag, _, found := strings.Cut(text, "="); found && dockerInspectFlagSensitive(flag) {
			redacted[i] = flag + "=" + dockerInspectRedacted
			continue
		}
		maskNext = dockerInspectFlagSensitive(text)
		redacted[i] = security.RedactSensitiveInfo(text)
	}
	return redacted
}

// dockerInspectFlagSensitive reports whether a command-line flag carries a credential value.
func dockerInspectFlagSensitive(arg string) bool {
	trimmed := strings.TrimSpace(arg)
	if !strings.HasPrefix(trimmed, "-") {
		return false
	}
	name := strings.ToUpper(strings.ReplaceAll(strings.TrimLeft(trimmed, "-"), "-", "_"))
	if name == "" {
		return false
	}
	for _, marker := range []string{"PASSWORD", "PASSWD", "PASS", "PWD", "SECRET", "TOKEN", "API_KEY", "APIKEY", "ACCESS_KEY", "PRIVATE_KEY", "MASTER_KEY", "REQUIREPASS", "MASTERAUTH", "PASSPHRASE", "CREDENTIALS"} {
		if name == marker || strings.HasSuffix(name, "_"+marker) {
			return true
		}
	}
	return false
}

// redactDockerInspectLabels masks labels whose key names a credential and
// redacts URL credentials or key=value secrets in the remaining values.
func redactDockerInspectLabels(value interface{}) interface{} {
	labels, ok := value.(map[string]interface{})
	if !ok {
		return value
	}
	redacted := make(map[string]interface{}, len(labels))
	for key, item := range labels {
		text, isString := item.(string)
		switch {
		case !isString:
			redacted[key] = item
		case dockerInspectLabelKeySensitive(key):
			redacted[key] = dockerInspectRedacted
		default:
			redacted[key] = security.RedactSensitiveInfo(text)
		}
	}
	return redacted
}

// dockerInspectLabelKeySensitive reports whether a label key names a credential.
// Unlike environment keys an aurago prefix is not sensitive, so ownership labels
// such as aurago.managed stay visible.
func dockerInspectLabelKeySensitive(key string) bool {
	normalized := "_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(strings.TrimSpace(key))) + "_"
	for _, marker := range []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "API_KEY", "ACCESS_KEY", "PRIVATE_KEY", "MASTER_KEY", "BASICAUTH", "CREDENTIAL", "CREDENTIALS"} {
		if strings.Contains(normalized, "_"+marker+"_") {
			return true
		}
	}
	return false
}

// ErrDockerOwnershipUnverified reports that Docker answered neither the inspect
// nor the list request, so the owner of a container is unknown.
var ErrDockerOwnershipUnverified = errors.New("docker container ownership could not be verified")

// dockerOwnerMatches applies the reserved-name rules for owner to name and, when
// labels are known, the AuraGo ownership labels.
func dockerOwnerMatches(owner, name string, labels map[string]string) bool {
	trimmed := strings.TrimSpace(owner)
	switch {
	case owner == acestep.Owner && acestep.IsResourceName(name):
		return true
	case strings.EqualFold(trimmed, dockerutil.LocalLLMOwner) && dockerutil.IsLocalLLMContainerName(name):
		return true
	case strings.EqualFold(trimmed, dockerutil.BoringGarageOwner) && dockerutil.IsBoringGarageContainerName(name):
		return true
	case strings.EqualFold(trimmed, dockerutil.HomepageOwner) && dockerutil.IsHomepageContainerName(name):
		return true
	case strings.EqualFold(trimmed, dockerutil.AppOwner) && dockerutil.IsAuraGoAppContainerName(name):
		return true
	case strings.EqualFold(trimmed, dockerutil.SecurityProxyOwner) && dockerutil.IsSecurityProxyContainerName(name):
		return true
	}
	return len(labels) > 0 && dockerutil.ManagedBy(labels, owner)
}

// DockerContainerOwnership reports which of owners manage containerID, using the
// reserved names first, then one inspect request, then one list request. When
// inspect failed (anything but 404) and the list fallback failed too, it returns
// an error wrapping ErrDockerOwnershipUnverified; callers must deny
// container-targeted operations then. An empty or invalid ID owns nothing.
func DockerContainerOwnership(cfg DockerConfig, containerID string, owners ...string) (map[string]bool, error) {
	owned := make(map[string]bool, len(owners))
	for _, owner := range owners {
		if dockerOwnerMatches(owner, containerID, nil) {
			owned[owner] = true
		}
	}
	if len(owned) == len(owners) || validateDockerName(containerID) != nil {
		return owned, nil
	}
	data, code, err := dockerRequest(cfg, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json", "")
	var inspectErr error
	switch {
	case err != nil:
		inspectErr = err
	case code == http.StatusOK:
		var info struct {
			Name   string `json:"Name"`
			Config struct {
				Labels map[string]string `json:"Labels"`
			} `json:"Config"`
		}
		if json.Unmarshal(data, &info) == nil {
			for _, owner := range owners {
				if dockerOwnerMatches(owner, info.Name, info.Config.Labels) {
					owned[owner] = true
				}
			}
			return owned, nil
		}
		inspectErr = fmt.Errorf("parse inspect response")
	case code != http.StatusNotFound:
		inspectErr = fmt.Errorf("inspect returned HTTP %d", code)
	}
	// An inspect race or error must not expose a managed container addressed by
	// ID. The list representation carries the same labels and matches ID prefixes.
	listData, listCode, listErr := dockerRequest(cfg, http.MethodGet, "/containers/json?all=true", "")
	var containers []map[string]interface{}
	if listErr == nil && listCode == http.StatusOK && json.Unmarshal(listData, &containers) == nil {
		target := strings.ToLower(strings.TrimSpace(containerID))
		for _, container := range containers {
			id := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", container["Id"])))
			names := dockerInterfaceStrings(container["Names"])
			matches := id != "" && strings.HasPrefix(id, target)
			for _, name := range names {
				matches = matches || strings.EqualFold(strings.TrimPrefix(name, "/"), target)
			}
			if !matches {
				continue
			}
			labels := dockerStringLabels(container["Labels"])
			for _, owner := range owners {
				if dockerManagedResourceExcluded(labels, names, false, []string{owner}) {
					owned[owner] = true
				}
			}
		}
		return owned, nil
	}
	if inspectErr == nil {
		// Inspect answered 404: no container has this ID or name.
		return owned, nil
	}
	listProblem := listErr
	if listProblem == nil {
		if listCode != http.StatusOK {
			listProblem = fmt.Errorf("list returned HTTP %d", listCode)
		} else {
			listProblem = fmt.Errorf("parse list response")
		}
	}
	return owned, fmt.Errorf("%w: inspect: %v; list: %v", ErrDockerOwnershipUnverified, inspectErr, listProblem)
}

// DockerContainerManagedBy checks a container's ownership label without exposing
// its config. It fails closed: unverifiable ownership counts as managed.
func DockerContainerManagedBy(cfg DockerConfig, containerID, owner string) bool {
	owned, err := DockerContainerOwnership(cfg, containerID, owner)
	return owned[owner] || err != nil
}

// DockerContainerOwnersFromMetadata applies the reserved-name and ownership
// label rules of DockerContainerOwnership to names and labels the caller
// already holds (a list entry or an inspect result). It sends no request.
func DockerContainerOwnersFromMetadata(names []string, labels map[string]string, owners ...string) map[string]bool {
	owned := make(map[string]bool, len(owners))
	for _, owner := range owners {
		if dockerOwnerMatches(owner, "", labels) {
			owned[owner] = true
			continue
		}
		for _, name := range names {
			if dockerOwnerMatches(owner, strings.TrimPrefix(strings.TrimSpace(name), "/"), labels) {
				owned[owner] = true
				break
			}
		}
	}
	return owned
}

func dockerStringLabels(value any) map[string]string {
	result := make(map[string]string)
	switch labels := value.(type) {
	case map[string]string:
		for key, item := range labels {
			result[key] = item
		}
	case map[string]interface{}:
		for key, item := range labels {
			if text, ok := item.(string); ok {
				result[key] = text
			}
		}
	}
	return result
}

func dockerInterfaceStrings(value any) []string {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...)
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, item := range values {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	default:
		return nil
	}
}

func dockerManagedResourceExcluded(labels map[string]string, names []string, volume bool, owners []string) bool {
	for _, owner := range owners {
		if dockerutil.ManagedBy(labels, owner) {
			return true
		}
		for _, name := range names {
			if owner == acestep.Owner && acestep.IsResourceName(name) {
				return true
			}
			if strings.EqualFold(strings.TrimSpace(owner), dockerutil.LocalLLMOwner) {
				if volume && dockerutil.IsLocalLLMVolumeName(name) {
					return true
				}
				if !volume && dockerutil.IsLocalLLMContainerName(name) {
					return true
				}
			}
			if !volume && strings.EqualFold(strings.TrimSpace(owner), dockerutil.HomepageOwner) &&
				dockerutil.IsHomepageContainerName(name) {
				return true
			}
			if !volume && strings.EqualFold(strings.TrimSpace(owner), dockerutil.AppOwner) &&
				dockerutil.IsAuraGoAppContainerName(name) {
				return true
			}
			if !volume && strings.EqualFold(strings.TrimSpace(owner), dockerutil.SecurityProxyOwner) &&
				dockerutil.IsSecurityProxyContainerName(name) {
				return true
			}
			if strings.EqualFold(strings.TrimSpace(owner), dockerutil.BoringGarageOwner) {
				if !volume && dockerutil.IsBoringGarageContainerName(name) {
					return true
				}
			}
		}
	}
	return false
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
	case "kill":
		method, endpoint = "POST", "/containers/"+safe+"/kill"
	case "remove", "rm":
		q := "?v=true"
		if force {
			q += "&force=true"
		}
		method, endpoint = "DELETE", "/containers/"+safe+q
	default:
		return errJSON("Unknown container action: %s. Use: start, stop, restart, pause, unpause, kill, remove", action)
	}

	data, code, err := dockerRequest(cfg, method, endpoint, "")
	if err != nil {
		return errJSON("Action '%s' failed: %v", action, err)
	}
	// 204 = success (no content), 304 = already in state
	if code == 204 || code == 304 {
		out, _ := json.Marshal(map[string]string{"status": "ok", "action": action, "container_id": containerID})
		return string(out)
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
		size := int(uint32(raw[4])<<24 | uint32(raw[5])<<16 | uint32(raw[6])<<8 | uint32(raw[7]))
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
// It returns nil if the image already exists locally. An error event in the
// Engine's progress stream fails the pull even though the status was 200.
func PullImageWait(ctx context.Context, cfg DockerConfig, image string, logger *slog.Logger) error {
	ctx, cancel := dockerContextWithFallbackTimeout(ctx, dockerPullFallbackTimeout)
	defer cancel()

	// Check if image already exists.
	filterURL := fmt.Sprintf("/images/json?filters=%%7B%%22reference%%22%%3A%%5B%%22%s%%22%%5D%%7D", url.QueryEscape(image))
	data, code, err := dockerRequest(cfg, "GET", filterURL, "")
	if err == nil && code == 200 {
		var images []interface{}
		if json.Unmarshal(data, &images) == nil && len(images) > 0 {
			return nil // already present
		}
	}

	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	if logger != nil {
		logger.Info("Pulling Docker image", "image", image)
	}

	// The helper reads the progress stream to its end, so the call blocks
	// until the pull is complete; an error event in it means the pull failed.
	if err := pullDockerImageStream(ctx, cfg, image); err != nil {
		return err
	}

	if logger != nil {
		logger.Info("Docker image pulled successfully", "image", image)
	}
	return nil
}

// PullImageForce pulls a Docker image even when the same reference already
// exists locally. Use this for user-triggered "update" actions on mutable tags.
func PullImageForce(ctx context.Context, cfg DockerConfig, image string, logger *slog.Logger) error {
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	ctx, cancel := dockerContextWithFallbackTimeout(ctx, dockerPullFallbackTimeout)
	defer cancel()
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image is required")
	}
	if logger != nil {
		logger.Info("Pulling Docker image", "image", image, "force", true)
	}
	// The helper reads the progress stream to its end, so the call blocks
	// until the pull is complete; an error event in it means the pull failed.
	if err := pullDockerImageStream(ctx, cfg, image); err != nil {
		return err
	}
	if logger != nil {
		logger.Info("Docker image pulled successfully", "image", image, "force", true)
	}
	return nil
}

// BuildImageWait builds a Docker image through the Docker Engine API using a
// minimal tar build context containing the supplied Dockerfile. This works from
// containerized AuraGo installs where DOCKER_HOST points at a socket proxy and
// the docker CLI is intentionally not installed.
func BuildImageWait(ctx context.Context, cfg DockerConfig, image, dockerfileName string, dockerfile []byte, buildArgs map[string]string, logger *slog.Logger) error {
	return BuildImageContextWait(ctx, cfg, image, dockerfileName, dockerfile, nil, buildArgs, logger)
}

// BuildImageContextWait builds a Docker image through the Docker Engine API
// using an in-memory tar context. The Dockerfile is always written under
// dockerfileName; additional files are written by their relative paths.
func BuildImageContextWait(ctx context.Context, cfg DockerConfig, image, dockerfileName string, dockerfile []byte, files map[string][]byte, buildArgs map[string]string, logger *slog.Logger) error {
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	ctx, cancel := dockerContextWithFallbackTimeout(ctx, 30*time.Minute)
	defer cancel()
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image is required")
	}
	if err := validateDockerName(image); err != nil {
		return err
	}
	dockerfileName = pathpkg.Clean(strings.TrimSpace(dockerfileName))
	if dockerfileName == "." || strings.HasPrefix(dockerfileName, "..") || strings.Contains(dockerfileName, "\\") {
		return fmt.Errorf("invalid Dockerfile name %q", dockerfileName)
	}
	if len(dockerfile) == 0 {
		return fmt.Errorf("Dockerfile content is required")
	}
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := writeDockerBuildContextFile(tw, dockerfileName, dockerfile, 0o644); err != nil {
		return fmt.Errorf("write Dockerfile to build context: %w", err)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if pathpkg.Clean(name) == dockerfileName {
			return fmt.Errorf("additional build context file %q conflicts with Dockerfile", name)
		}
		if err := writeDockerBuildContextFile(tw, name, files[name], 0o755); err != nil {
			return fmt.Errorf("write build context file %s: %w", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("finalize Docker build context: %w", err)
	}

	if logger != nil {
		logger.Info("Building Docker image via Engine API", "image", image, "dockerfile", dockerfileName)
	}
	argsJSON, err := json.Marshal(buildArgs)
	if err != nil {
		return fmt.Errorf("marshal Docker build args: %w", err)
	}
	query := url.Values{}
	query.Set("t", image)
	query.Set("dockerfile", dockerfileName)
	if len(buildArgs) > 0 {
		query.Set("buildargs", string(argsJSON))
	}
	reqURL := "http://localhost/" + dockerAPIVersion + "/build?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(tarBuf.Bytes()))
	if err != nil {
		return fmt.Errorf("create Docker build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := getPullDockerClient(cfg).Do(req)
	if err != nil {
		return fmt.Errorf("build image %s: %w", image, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// dockerBodyMessage already trims and sanitises the body; when it is
		// empty, the body has no printable text, so no raw fallback is added.
		msg := dockerBodyMessage(resp.StatusCode, dockerutil.ReadErrorBody(resp.Body))
		return fmt.Errorf("build image %s: HTTP %d: %s", image, resp.StatusCode, msg)
	}
	if err := dockerutil.DrainJSONMessages(resp.Body); err != nil {
		return fmt.Errorf("build image %s: %w", image, err)
	}
	if logger != nil {
		logger.Info("Docker image built successfully", "image", image)
	}
	return nil
}

func writeDockerBuildContextFile(tw *tar.Writer, name string, content []byte, mode int64) error {
	cleanName := pathpkg.Clean(strings.TrimSpace(name))
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(cleanName, "/") || strings.Contains(cleanName, "\\") {
		return fmt.Errorf("invalid build context path %q", name)
	}
	if err := tw.WriteHeader(&tar.Header{Name: cleanName, Mode: mode, Size: int64(len(content))}); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

func dockerContextWithFallbackTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func validateDockerCreateRequestBinds(cfg DockerConfig, method, endpoint, body string) error {
	return validateDockerCreateRequestBindsTrusted(cfg, method, endpoint, body, nil)
}

// validateDockerCreateRequestBindsTrusted validates the binds of a
// /containers/create body; binds listed exactly in trusted are skipped.
func validateDockerCreateRequestBindsTrusted(cfg DockerConfig, method, endpoint, body string, trusted []string) error {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return nil
	}
	if !dockerEndpointMayCreateContainer(endpoint) {
		return nil
	}
	if strings.TrimSpace(body) == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return fmt.Errorf("invalid Docker create payload: %w", err)
	}
	return validateDockerCreatePayloadBindsTrusted(cfg, payload, trusted)
}

// reDockerVersionedPath splits an Engine API path into its /vX.Y version
// segment and the route below it, like the Engine's own /v{version:[0-9.]+}
// route prefix.
var reDockerVersionedPath = regexp.MustCompile(`^/v[0-9.]+(/.*)$`)

// dockerEndpointMayCreateContainer reports whether a POST to endpoint can
// reach the Engine's container create route, so its binds must be checked.
// It keeps the raw "/containers/create" prefix match and adds every spelling
// the Engine routes to create after decoding and cleaning the request path
// (/containers/%63reate, /containers%2Fcreate, //containers/create,
// /containers/./create, /x/../containers/create, /../v1.40/containers/create).
// The endpoint is parsed exactly as the request URL is built
// ("http://localhost/" + dockerAPIVersion + endpoint); the decoded path is
// cleaned and one leading /vX.Y segment is removed. An endpoint that does not
// parse is treated as a create request (fail closed); such a request cannot
// be sent anyway. Any other path still skips the bind check, so detection
// only widens.
func dockerEndpointMayCreateContainer(endpoint string) bool {
	if strings.HasPrefix(strings.TrimSpace(endpoint), "/containers/create") {
		return true
	}
	parsed, err := url.Parse("http://localhost/" + dockerAPIVersion + endpoint)
	if err != nil {
		return true
	}
	route := pathpkg.Clean(parsed.Path)
	if match := reDockerVersionedPath.FindStringSubmatch(route); match != nil {
		route = match[1]
	}
	return route == "/containers/create"
}

// validateDockerCreatePayloadBindsTrusted validates every HostConfig.Binds
// entry with validateDockerBindMount, except entries listed exactly in trusted.
func validateDockerCreatePayloadBindsTrusted(cfg DockerConfig, payload map[string]interface{}, trusted []string) error {
	hostConfig, ok := payload["HostConfig"].(map[string]interface{})
	if !ok {
		return nil
	}
	rawBinds, ok := hostConfig["Binds"]
	if !ok || rawBinds == nil {
		return nil
	}
	validate := func(bind string) error {
		if dockerBindTrusted(bind, trusted) {
			return nil
		}
		return validateDockerBindMount(cfg, bind)
	}
	switch binds := rawBinds.(type) {
	case []string:
		for _, bind := range binds {
			if err := validate(bind); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, raw := range binds {
			bind, ok := raw.(string)
			if !ok {
				return fmt.Errorf("invalid Docker create payload HostConfig.Binds entry type %T", raw)
			}
			if err := validate(bind); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid Docker create payload HostConfig.Binds type %T", rawBinds)
	}
	return nil
}

// dockerBindTrusted reports whether bind is listed exactly in trusted.
func dockerBindTrusted(bind string, trusted []string) bool {
	for _, candidate := range trusted {
		if candidate != "" && candidate == bind {
			return true
		}
	}
	return false
}

// DockerPullImage pulls an image from a registry.
func DockerPullImage(cfg DockerConfig, image string) string {
	return DockerPullImageContext(context.Background(), cfg, image)
}

// DockerPullImageContext pulls an image and reports success only after the
// Engine's progress stream ended without an error event. The caller's context
// cancels the pull; without a deadline the pull is bounded to 15 minutes.
func DockerPullImageContext(ctx context.Context, cfg DockerConfig, image string) string {
	if image == "" {
		return errJSON("image name is required")
	}
	if err := requireDockerMutationPermission(); err != nil {
		return errJSON("Failed to pull image: %v", err)
	}
	ctx, cancel := dockerContextWithFallbackTimeout(ctx, dockerPullFallbackTimeout)
	defer cancel()
	if err := pullDockerImageStream(ctx, cfg, image); err != nil {
		var pullErr *dockerPullError
		if !errors.As(err, &pullErr) {
			return errJSON("Failed to pull image: %v", err)
		}
		if pullErr.StatusCode != 0 {
			if pullErr.Message != "" {
				return errJSON("Docker error (HTTP %d): %s", pullErr.StatusCode, pullErr.Message)
			}
			return errJSON("Docker error (HTTP %d)", pullErr.StatusCode)
		}
		return errJSON("Failed to pull image: %v", pullErr.Err)
	}
	out, _ := json.Marshal(map[string]string{"status": "ok", "message": "Image '" + image + "' pulled successfully"})
	return string(out)
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
		out, _ := json.Marshal(map[string]string{"status": "ok", "message": "Image '" + image + "' removed"})
		return string(out)
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
	// The names DockerCreateContainerWithOptions reserves: a rename must not
	// plant a managed AuraGo container either.
	if acestep.IsResourceName(newName) || dockerutil.IsLocalLLMContainerName(newName) {
		return errJSON("reserved AuraGo local LLM container name")
	}
	if dockerutil.IsBoringGarageContainerName(newName) || dockerutil.IsHomepageContainerName(newName) || dockerutil.IsAuraGoAppContainerName(newName) ||
		dockerutil.IsSecurityProxyContainerName(newName) {
		return errJSON("reserved AuraGo managed container name")
	}
	endpoint := "/containers/" + url.PathEscape(containerID) + "/rename?name=" + url.QueryEscape(newName)
	data, code, err := dockerRequest(cfg, "POST", endpoint, "")
	if err != nil {
		return errJSON("Failed to rename container: %v", err)
	}
	if code == 204 || code == 200 {
		out, _ := json.Marshal(map[string]string{"status": "ok", "message": "Container renamed to '" + newName + "'"})
		return string(out)
	}
	if code == 404 {
		return errJSON("Container '%s' not found", containerID)
	}
	return dockerBodyErr(code, data)
}
