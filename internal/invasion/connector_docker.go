package invasion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/dockerutil"
	"gopkg.in/yaml.v3"
)

// dockerAPIVersion is the Docker Engine API version used for all requests.
// Increment when requiring features from a newer Docker Engine.
const dockerAPIVersion = dockerutil.APIVersion
const dockerEggConfigArchivePath = "/app/data"
const dockerEggConfigFileName = "config.yaml"

// DockerConnector deploys eggs as Docker containers, either on a remote host
// or on the local Docker daemon.
type DockerConnector struct{}

func (c *DockerConnector) Validate(ctx context.Context, nest NestRecord, secret []byte) error {
	client := c.httpClient(nest)
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL(nest, "/version"), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("docker API unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker API returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) Deploy(ctx context.Context, nest NestRecord, secret []byte, payload EggDeployPayload) error {
	containerName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	backupName := containerName + "-prev"
	// TODO: derive image tag from master version when build version is available at runtime
	image := "ghcr.io/antibyte/aurago:latest"

	// 1. Pull image
	if err := c.pullImage(ctx, nest, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// 2. Remove any stale backup, then rename current container as backup
	_ = c.removeContainer(ctx, nest, backupName)
	_ = c.renameContainer(ctx, nest, containerName, backupName)

	// 3. Create container with minimal env vars.
	// The full configuration (including secrets like the shared key and API keys)
	// is copied into the container via the archive API in step 4, avoiding
	// exposure via "docker inspect" which displays environment variables.
	envVars := []string{
		"AURAGO_EGG_MODE=true",
		"AURAGO_SERVER_HOST=0.0.0.0",
	}

	createBody := map[string]interface{}{
		"Image": image,
		"Env":   envVars,
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{
				"Name": "unless-stopped",
			},
			"Binds": []string{
				fmt.Sprintf("aurago-egg-%s-data:/app/data", nest.ID[:8]),
				fmt.Sprintf("aurago-egg-%s-log:/app/log", nest.ID[:8]),
			},
			// Allow the egg to reach the master via host.docker.internal.
			// On Linux Docker Engine this is not injected automatically;
			// host-gateway resolves to the Docker bridge gateway (typically 172.17.0.1).
			// Safe no-op on Docker Desktop (Windows/Mac) where the name already resolves.
			"ExtraHosts": []string{"host.docker.internal:host-gateway"},
			// Security hardening — mirror the main AuraGo container's security profile.
			"SecurityOpt": []string{"no-new-privileges:true"},
			"CapDrop":     []string{"ALL"},
		},
	}

	bodyJSON, _ := json.Marshal(createBody)
	url := c.apiURL(nest, fmt.Sprintf("/containers/create?name=%s", containerName))
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := c.httpClient(nest)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("container creation failed (%d): %s", resp.StatusCode, string(body))
	}

	// 4. Copy config.yaml into the container via the Docker Archive API.
	// This ensures secrets (shared key, API keys) are not visible in "docker inspect".
	if err := c.copyConfigToContainer(ctx, nest, containerName, payload.ConfigYAML); err != nil {
		// Clean up the created container on failure
		_ = c.removeContainer(ctx, nest, containerName)
		return fmt.Errorf("failed to copy config to container: %w", err)
	}

	// 5. Start container
	startURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/start", containerName))
	startReq, err := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create start request: %w", err)
	}
	startResp, err := client.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusNoContent && startResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(startResp.Body)
		return fmt.Errorf("container start failed (%d): %s", startResp.StatusCode, string(body))
	}

	return nil
}

// copyConfigToContainer copies the egg config YAML into a container via the
// Docker Engine Archive API (PUT /containers/{id}/archive). The config is
// written to /app/data/config.yaml with mode 0600 (owner read/write only).
func (c *DockerConnector) copyConfigToContainer(ctx context.Context, nest NestRecord, containerName string, configYAML []byte) error {
	// Build a tar archive containing config.yaml
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: dockerEggConfigFileName,
		Mode: 0600,
		Size: int64(len(configYAML)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(configYAML); err != nil {
		return fmt.Errorf("failed to write config to tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar archive: %w", err)
	}

	// Upload to the persisted config path read by docker-entrypoint.sh.
	url := c.apiURL(nest, fmt.Sprintf("/containers/%s/archive?path=%s", containerName, dockerEggConfigArchivePath))
	req, err := http.NewRequestWithContext(ctx, "PUT", url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create archive request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")

	uploadClient := c.httpClient(nest)
	resp, err := uploadClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config upload failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) Stop(ctx context.Context, nest NestRecord, secret []byte) error {
	containerName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	client := c.httpClient(nest)

	// Stop container
	stopURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/stop?t=10", containerName))
	req, err := http.NewRequestWithContext(ctx, "POST", stopURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create stop request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	defer resp.Body.Close()
	// 204 = stopped, 304 = already stopped are both acceptable
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop container failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) Status(ctx context.Context, nest NestRecord, secret []byte) (string, error) {
	containerName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	client := c.httpClient(nest)

	url := c.apiURL(nest, fmt.Sprintf("/containers/%s/json", containerName))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "unknown", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "unknown", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "stopped", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "unknown", fmt.Errorf("inspect failed with status %d", resp.StatusCode)
	}

	var info struct {
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
		} `json:"State"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "unknown", err
	}

	if info.State.Running {
		return "running", nil
	}
	return "stopped", nil
}

func (c *DockerConnector) httpClient(nest NestRecord) *http.Client {
	isLocal := nest.DeployMethod == "docker_local"
	if isLocal {
		dockerHost := dockerLocalHost()
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dockerutil.DialContext(ctx, dockerHost)
				},
			},
		}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func dockerLocalHost() string {
	if dh := strings.TrimSpace(os.Getenv("DOCKER_HOST")); dh != "" {
		return dh
	}
	return dockerutil.DefaultHost()
}

func (c *DockerConnector) apiURL(nest NestRecord, path string) string {
	if nest.DeployMethod == "docker_local" {
		return fmt.Sprintf("http://localhost/%s%s", dockerAPIVersion, path)
	}
	port := nest.Port
	if port == 0 {
		port = 2375
	}
	return fmt.Sprintf("http://%s:%d/%s%s", nest.Host, port, dockerAPIVersion, path)
}

// pullClient returns an HTTP client with an extended timeout suitable for
// image pull operations, which can take minutes on slow connections.
func (c *DockerConnector) pullClient(nest NestRecord) *http.Client {
	base := c.httpClient(nest)
	return &http.Client{
		Timeout:   10 * time.Minute,
		Transport: base.Transport,
	}
}

func (c *DockerConnector) pullImage(ctx context.Context, nest NestRecord, image string) error {
	client := c.pullClient(nest)
	url := c.apiURL(nest, fmt.Sprintf("/images/create?fromImage=%s", image))
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	// Drain the response (Docker streams progress)
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *DockerConnector) removeContainer(ctx context.Context, nest NestRecord, name string) error {
	client := c.httpClient(nest)
	url := c.apiURL(nest, fmt.Sprintf("/containers/%s?force=true", name))
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("remove container request failed: %w", err)
	}
	defer resp.Body.Close()
	// 204 = deleted, 404 = not found (already gone) are both acceptable
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove container failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) renameContainer(ctx context.Context, nest NestRecord, oldName, newName string) error {
	client := c.httpClient(nest)
	// Stop the container first so it can be renamed cleanly
	stopURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/stop?t=5", oldName))
	stopReq, _ := http.NewRequestWithContext(ctx, "POST", stopURL, nil)
	if resp, err := client.Do(stopReq); err == nil {
		resp.Body.Close()
	}

	url := c.apiURL(nest, fmt.Sprintf("/containers/%s/rename?name=%s", oldName, newName))
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("rename container failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rename container failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *DockerConnector) HealthCheck(ctx context.Context, nest NestRecord, secret []byte) error {
	containerName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	client := c.httpClient(nest)

	url := c.apiURL(nest, fmt.Sprintf("/containers/%s/json", containerName))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("container not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inspect failed with status %d", resp.StatusCode)
	}

	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("failed to decode container state: %w", err)
	}
	if !info.State.Running {
		return fmt.Errorf("container is not running")
	}
	return nil
}

// Reconfigure writes a patched config.yaml into the running egg container and restarts it.
// The container is stopped, the config is replaced via the archive API, then restarted.
func (c *DockerConnector) Reconfigure(ctx context.Context, nest NestRecord, secret []byte, configYAML []byte) error {
	containerName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])

	// 1. Stop the container
	if err := c.Stop(ctx, nest, secret); err != nil {
		return fmt.Errorf("failed to stop container for reconfigure: %w", err)
	}

	// 2. Copy the patched config into the container
	if err := c.copyConfigToContainer(ctx, nest, containerName, configYAML); err != nil {
		return fmt.Errorf("failed to copy patched config to container: %w", err)
	}

	// 3. Start the container
	client := c.httpClient(nest)
	startURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/start", containerName))
	startReq, err := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create start request: %w", err)
	}
	startResp, err := client.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start container after reconfigure: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusNoContent && startResp.StatusCode != http.StatusOK && startResp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(startResp.Body)
		return fmt.Errorf("container start failed after reconfigure (%d): %s", startResp.StatusCode, string(body))
	}

	return nil
}

func (c *DockerConnector) Rollback(ctx context.Context, nest NestRecord, secret []byte) error {
	containerName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	backupName := containerName + "-prev"

	// Check if backup container exists
	client := c.httpClient(nest)
	checkURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/json", backupName))
	checkReq, _ := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	resp, err := client.Do(checkReq)
	if err != nil {
		return fmt.Errorf("failed to check backup container: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no backup container found for rollback")
	}

	// Remove the failed new container
	_ = c.removeContainer(ctx, nest, containerName)

	// Rename backup back to primary name
	if err := c.renameContainer(ctx, nest, backupName, containerName); err != nil {
		return fmt.Errorf("failed to restore backup container: %w", err)
	}

	// Start the restored container
	startURL := c.apiURL(nest, fmt.Sprintf("/containers/%s/start", containerName))
	startReq, _ := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	startResp, err := client.Do(startReq)
	if err != nil {
		return fmt.Errorf("failed to start restored container: %w", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusNoContent && startResp.StatusCode != http.StatusOK && startResp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(startResp.Body)
		return fmt.Errorf("failed to start restored container (%d): %s", startResp.StatusCode, string(body))
	}

	return nil
}

// extractMasterURL extracts egg_mode.master_url from YAML config bytes.
func extractMasterURL(cfgYAML []byte) string {
	return extractYAMLField(cfgYAML, "master_url")
}

// extractField extracts a field value from YAML config bytes.
func extractField(cfgYAML []byte, field string) string {
	return extractYAMLField(cfgYAML, field)
}

func extractYAMLField(data []byte, field string) string {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	// Check top-level keys first
	if v, ok := raw[field]; ok {
		return fmt.Sprint(v)
	}
	// Check one level deep (e.g. egg_mode.master_url)
	for _, section := range raw {
		if m, ok := section.(map[string]interface{}); ok {
			if v, ok := m[field]; ok {
				return fmt.Sprint(v)
			}
		}
	}
	return ""
}

// GetConnector returns the appropriate NestConnector for the given nest.
func GetConnector(nest NestRecord) NestConnector {
	switch nest.DeployMethod {
	case "docker_remote", "docker_local":
		return &DockerConnector{}
	default: // "ssh" and everything else
		return &SSHConnector{}
	}
}
