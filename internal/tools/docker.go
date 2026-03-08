package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DockerConfig holds the Docker Engine connection parameters.
type DockerConfig struct {
	Host string // e.g. "unix:///var/run/docker.sock" or "tcp://localhost:2375"
}

// dockerHTTPClient is a lazily-initialized shared Docker API client.
var dockerHTTPClient *http.Client
var dockerHTTPClientHost string

// reDockerSafeName validates Docker container/image identifiers.
var reDockerSafeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:\-/]*$`)

// getDockerClient returns a shared *http.Client that talks to the Docker Engine API.
// The client is reused across requests for connection pooling.
func getDockerClient(cfg DockerConfig) *http.Client {
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
				return net.DialTimeout("tcp", "localhost:2375", 5*time.Second)
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
	reqURL := "http://localhost/v1.45/_ping"
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
	client := getDockerClient(cfg)

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	// Docker Engine API is accessed via http://localhost but routed through the Unix socket.
	reqURL := "http://localhost/v1.45" + endpoint
	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("docker request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read docker response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// errJSON is a helper that returns a JSON error string.
func errJSON(msg string, args ...interface{}) string {
	return fmt.Sprintf(`{"status":"error","message":"`+msg+`"}`, args...)
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
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerCreateContainer creates a new container from a configuration.
func DockerCreateContainer(cfg DockerConfig, name, image string, env []string, ports map[string]string, volumes []string, cmd []string, restart string) string {
	if image == "" {
		return errJSON("image is required")
	}

	// Build HostConfig.PortBindings and ExposedPorts
	exposedPorts := map[string]interface{}{}
	portBindings := map[string]interface{}{}
	for containerPort, hostPort := range ports {
		// Ensure container port has /tcp suffix
		if !strings.Contains(containerPort, "/") {
			containerPort += "/tcp"
		}
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []map[string]string{{"HostPort": hostPort}}
	}

	// Build volume binds
	hostConfig := map[string]interface{}{
		"Binds":         volumes,
		"PortBindings":  portBindings,
		"RestartPolicy": map[string]interface{}{"Name": restart},
	}

	payload := map[string]interface{}{
		"Image":        image,
		"Env":          env,
		"ExposedPorts": exposedPorts,
		"HostConfig":   hostConfig,
	}
	if len(cmd) > 0 {
		payload["Cmd"] = cmd
	}

	body, _ := json.Marshal(payload)

	endpoint := "/containers/create"
	if name != "" {
		if err := validateDockerName(name); err != nil {
			return errJSON("%v", err)
		}
		endpoint += "?name=" + url.QueryEscape(name)
	}

	data, code, err := dockerRequest(cfg, "POST", endpoint, string(body))
	if err != nil {
		return errJSON("Failed to create container: %v", err)
	}
	if code == 201 {
		var resp map[string]interface{}
		json.Unmarshal(data, &resp)
		return fmt.Sprintf(`{"status":"ok","message":"Container created","id":"%v"}`, resp["Id"])
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerListNetworks returns all Docker networks.
func DockerListNetworks(cfg DockerConfig) string {
	data, code, err := dockerRequest(cfg, "GET", "/networks", "")
	if err != nil {
		return errJSON("Failed to list networks: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var networks []map[string]interface{}
	if err := json.Unmarshal(data, &networks); err != nil {
		return errJSON("Failed to parse networks: %v", err)
	}

	type compact struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Driver string `json:"driver"`
		Scope  string `json:"scope"`
	}
	var result []compact
	for _, n := range networks {
		entry := compact{
			Name:   fmt.Sprintf("%v", n["Name"]),
			Driver: fmt.Sprintf("%v", n["Driver"]),
			Scope:  fmt.Sprintf("%v", n["Scope"]),
		}
		if id, ok := n["Id"].(string); ok && len(id) > 12 {
			entry.ID = id[:12]
		}
		result = append(result, entry)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "networks": result})
	return string(out)
}

// DockerListVolumes returns all Docker volumes.
func DockerListVolumes(cfg DockerConfig) string {
	data, code, err := dockerRequest(cfg, "GET", "/volumes", "")
	if err != nil {
		return errJSON("Failed to list volumes: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse volumes: %v", err)
	}

	type compact struct {
		Name       string `json:"name"`
		Driver     string `json:"driver"`
		Mountpoint string `json:"mountpoint"`
	}
	var result []compact
	if vols, ok := resp["Volumes"].([]interface{}); ok {
		for _, v := range vols {
			if vol, ok := v.(map[string]interface{}); ok {
				result = append(result, compact{
					Name:       fmt.Sprintf("%v", vol["Name"]),
					Driver:     fmt.Sprintf("%v", vol["Driver"]),
					Mountpoint: fmt.Sprintf("%v", vol["Mountpoint"]),
				})
			}
		}
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "volumes": result})
	return string(out)
}

// DockerSystemInfo returns a summary of the Docker engine (version, containers, images count).
func DockerSystemInfo(cfg DockerConfig) string {
	data, code, err := dockerRequest(cfg, "GET", "/info", "")
	if err != nil {
		return errJSON("Failed to get Docker info: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var info map[string]interface{}
	if err := json.Unmarshal(data, &info); err != nil {
		return errJSON("Failed to parse info: %v", err)
	}

	result := map[string]interface{}{
		"status":             "ok",
		"server_version":     info["ServerVersion"],
		"os":                 info["OperatingSystem"],
		"architecture":       info["Architecture"],
		"cpus":               info["NCPU"],
		"memory_bytes":       info["MemTotal"],
		"containers_total":   info["Containers"],
		"containers_running": info["ContainersRunning"],
		"containers_stopped": info["ContainersStopped"],
		"containers_paused":  info["ContainersPaused"],
		"images":             info["Images"],
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// DockerExec executes a command inside a running container using the REST API.
func DockerExec(cfg DockerConfig, containerID, cmd, user string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}

	cmdArray := []string{"sh", "-c", cmd}
	payload := map[string]interface{}{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmdArray,
		"Tty":          false,
	}
	if user != "" {
		payload["User"] = user
	}
	body, _ := json.Marshal(payload)

	// Create exec instance
	data, code, err := dockerRequest(cfg, "POST", "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return errJSON("Failed to map exec: %v", err)
	}
	if code != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var execResp map[string]interface{}
	json.Unmarshal(data, &execResp)
	execID, ok := execResp["Id"].(string)
	if !ok || execID == "" {
		return errJSON("Failed to obtain exec ID")
	}

	// Start exec instance and read output
	startPayload := `{"Detach": false, "Tty": false}`
	outData, outCode, err := dockerRequest(cfg, "POST", "/exec/"+execID+"/start", startPayload)
	if err != nil {
		return errJSON("Failed to start exec: %v", err)
	}
	if outCode != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, outCode, string(outData))
	}

	outputStr := stripDockerLogHeaders(outData)
	result := map[string]interface{}{
		"status":       "ok",
		"container_id": containerID,
		"output":       outputStr,
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// DockerStats retrieves real-time resource usage of a container.
func DockerStats(cfg DockerConfig, containerID string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	data, code, err := dockerRequest(cfg, "GET", "/containers/"+url.PathEscape(containerID)+"/stats?stream=false", "")
	if err != nil {
		return errJSON("Failed to get stats: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return string(data)
}

// DockerTop lists running processes inside a container.
func DockerTop(cfg DockerConfig, containerID string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	data, code, err := dockerRequest(cfg, "GET", "/containers/"+url.PathEscape(containerID)+"/top", "")
	if err != nil {
		return errJSON("Failed to get top: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return string(data)
}

// DockerPort shows mapped ports for a container.
func DockerPort(cfg DockerConfig, containerID string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	data, code, err := dockerRequest(cfg, "GET", "/containers/"+url.PathEscape(containerID)+"/json", "")
	if err != nil {
		return errJSON("Failed to get port info: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	var full map[string]interface{}
	json.Unmarshal(data, &full)

	ports := map[string]interface{}{}
	if netSettings, ok := full["NetworkSettings"].(map[string]interface{}); ok {
		if p, ok := netSettings["Ports"]; ok {
			ports = p.(map[string]interface{})
		}
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "ports": ports})
	return string(out)
}

// DockerCreateNetwork creates a network.
func DockerCreateNetwork(cfg DockerConfig, name, driver string) string {
	if name == "" {
		return errJSON("network name required")
	}
	if driver == "" {
		driver = "bridge"
	}
	payload, _ := json.Marshal(map[string]interface{}{"Name": name, "Driver": driver})
	data, code, err := dockerRequest(cfg, "POST", "/networks/create", string(payload))
	if err != nil {
		return errJSON("Failed to create network: %v", err)
	}
	if code == 201 {
		var r map[string]interface{}
		json.Unmarshal(data, &r)
		return fmt.Sprintf(`{"status":"ok","message":"Network created","id":"%v"}`, r["Id"])
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerRemoveNetwork removes a network.
func DockerRemoveNetwork(cfg DockerConfig, name string) string {
	if name == "" {
		return errJSON("network name required")
	}
	data, code, err := dockerRequest(cfg, "DELETE", "/networks/"+url.PathEscape(name), "")
	if err != nil {
		return errJSON("Failed to remove network: %v", err)
	}
	if code == 204 || code == 200 {
		return `{"status":"ok","message":"Network removed"}`
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerConnectNetwork connects a container to a network.
func DockerConnectNetwork(cfg DockerConfig, containerID, network string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{"Container": containerID})
	data, code, err := dockerRequest(cfg, "POST", "/networks/"+url.PathEscape(network)+"/connect", string(payload))
	if err != nil {
		return errJSON("Failed to connect to network: %v", err)
	}
	if code == 200 || code == 204 {
		return fmt.Sprintf(`{"status":"ok","message":"Connected to %s"}`, network)
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerDisconnectNetwork disconnects a container from a network.
func DockerDisconnectNetwork(cfg DockerConfig, containerID, network string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{"Container": containerID, "Force": true})
	data, code, err := dockerRequest(cfg, "POST", "/networks/"+url.PathEscape(network)+"/disconnect", string(payload))
	if err != nil {
		return errJSON("Failed to disconnect from network: %v", err)
	}
	if code == 200 || code == 204 {
		return fmt.Sprintf(`{"status":"ok","message":"Disconnected from %s"}`, network)
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerCreateVolume creates a volume.
func DockerCreateVolume(cfg DockerConfig, name, driver string) string {
	if name == "" {
		return errJSON("volume name required")
	}
	if driver == "" {
		driver = "local"
	}
	payload, _ := json.Marshal(map[string]interface{}{"Name": name, "Driver": driver})
	data, code, err := dockerRequest(cfg, "POST", "/volumes/create", string(payload))
	if err != nil {
		return errJSON("Failed to create volume: %v", err)
	}
	if code == 201 || code == 200 {
		return fmt.Sprintf(`{"status":"ok","message":"Volume created","name":"%s"}`, name)
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// DockerRemoveVolume removes a volume.
func DockerRemoveVolume(cfg DockerConfig, name string, force bool) string {
	if name == "" {
		return errJSON("volume name required")
	}
	forceParam := ""
	if force {
		forceParam = "?force=true"
	}
	data, code, err := dockerRequest(cfg, "DELETE", "/volumes/"+url.PathEscape(name)+forceParam, "")
	if err != nil {
		return errJSON("Failed to remove volume: %v", err)
	}
	if code == 204 || code == 200 {
		return `{"status":"ok","message":"Volume removed"}`
	}
	return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
}

// runDockerCLIHelper is used for operations like cp and compose which are notoriously difficult strictly via REST API.
func runDockerCLIHelper(cfg DockerConfig, args ...string) string {
	var cmdArgs []string
	if cfg.Host != "" && !strings.HasPrefix(cfg.Host, "unix://") && !strings.HasPrefix(cfg.Host, "npipe://") {
		cmdArgs = append(cmdArgs, "-H", cfg.Host)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("docker", cmdArgs...)
	out, err := cmd.CombinedOutput()

	msg := string(out)
	if err != nil {
		return errJSON("Command failed: %v | Output: %s", err, msg)
	}

	res, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"output": msg,
	})
	return string(res)
}

// DockerCopy uses CLI to copy files to/from container.
func DockerCopy(cfg DockerConfig, containerID, src, dest, direction string) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	if src == "" || dest == "" {
		return errJSON("src and dest required")
	}

	var args []string
	if direction == "from_container" {
		args = []string{"cp", containerID + ":" + src, dest}
	} else if direction == "to_container" {
		args = []string{"cp", src, containerID + ":" + dest}
	} else {
		return errJSON("direction must be from_container or to_container")
	}

	return runDockerCLIHelper(cfg, args...)
}

// DockerCompose uses CLI to trigger compose.
func DockerCompose(cfg DockerConfig, file, cmd string) string {
	if file == "" || cmd == "" {
		return errJSON("file and command required")
	}
	args := []string{"compose", "-f", file}
	args = append(args, strings.Fields(cmd)...)
	return runDockerCLIHelper(cfg, args...)
}
