package invasion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

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
	image := "ghcr.io/antibyte/aurago:latest"

	// 1. Pull image
	if err := c.pullImage(ctx, nest, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// 2. Remove old container if exists
	_ = c.removeContainer(ctx, nest, containerName)

	// 3. Create container with egg configuration via env vars
	envVars := []string{
		"AURAGO_EGG_MODE=true",
		fmt.Sprintf("AURAGO_MASTER_URL=%s", extractMasterURL(payload.ConfigYAML)),
		fmt.Sprintf("AURAGO_SHARED_KEY=%s", payload.SharedKey),
		fmt.Sprintf("AURAGO_EGG_ID=%s", extractField(payload.ConfigYAML, "egg_id")),
		fmt.Sprintf("AURAGO_NEST_ID=%s", extractField(payload.ConfigYAML, "nest_id")),
		fmt.Sprintf("AURAGO_MASTER_KEY=%s", payload.MasterKey),
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

	// 4. Start container
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
	resp.Body.Close()

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
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", "/var/run/docker.sock")
				},
			},
		}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *DockerConnector) apiURL(nest NestRecord, path string) string {
	if nest.DeployMethod == "docker_local" {
		return fmt.Sprintf("http://localhost/v1.45%s", path)
	}
	port := nest.Port
	if port == 0 {
		port = 2375
	}
	return fmt.Sprintf("http://%s:%d/v1.45%s", nest.Host, port, path)
}

func (c *DockerConnector) pullImage(ctx context.Context, nest NestRecord, image string) error {
	client := c.httpClient(nest)
	url := c.apiURL(nest, fmt.Sprintf("/images/create?fromImage=%s", image))
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
		return err
	}
	resp.Body.Close()
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
	// Simple extraction — find "field: value" in YAML
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, field+":") {
			val := strings.TrimPrefix(trimmed, field+":")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, "\"'")
			return val
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
