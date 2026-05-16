package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DockerUpdateContainerImage pulls the container's configured image and
// recreates the container under the same name with the same Docker settings.
func DockerUpdateContainerImage(ctx context.Context, cfg DockerConfig, containerID string, logger *slog.Logger) string {
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	if err := requireDockerMutationPermission(); err != nil {
		return errJSON("%v", err)
	}

	inspect, err := inspectContainerForUpdate(cfg, containerID)
	if err != nil {
		return errJSON("%v", err)
	}
	name := dockerContainerNameFromInspect(inspect)
	if name == "" {
		return errJSON("container has no reusable name")
	}
	if err := validateDockerName(name); err != nil {
		return errJSON("container name cannot be reused: %v", err)
	}
	image := dockerContainerImageFromInspect(inspect)
	if err := validateDockerImageReferenceForUpdate(image); err != nil {
		return errJSON("%v", err)
	}
	wasRunning := dockerInspectBool(inspect, "State", "Running")

	if logger != nil {
		logger.Info("[Docker] Updating container image", "container", name, "image", image)
	}
	if err := pullDockerImageForUpdate(ctx, cfg, image); err != nil {
		return errJSON("Failed to pull latest image for %s: %v", image, err)
	}

	payload, err := dockerReplacementCreatePayload(inspect, image)
	if err != nil {
		return errJSON("%v", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return errJSON("Failed to prepare replacement container payload: %v", err)
	}

	backupName := dockerUpdateBackupName(name)
	if wasRunning {
		if err := dockerStopForUpdate(cfg, containerID); err != nil {
			return errJSON("%v", err)
		}
	}
	if err := dockerRenameForUpdate(cfg, containerID, backupName); err != nil {
		if wasRunning {
			_ = dockerStartForUpdate(cfg, containerID)
		}
		return errJSON("%v", err)
	}

	newID, err := dockerCreateForUpdate(cfg, name, string(body))
	if err != nil {
		dockerRollbackUpdatedContainer(cfg, "", backupName, name, wasRunning)
		return errJSON("Failed to create replacement container: %v", err)
	}
	started := false
	if wasRunning {
		if err := dockerStartForUpdate(cfg, newID); err != nil {
			dockerRollbackUpdatedContainer(cfg, newID, backupName, name, wasRunning)
			return errJSON("Replacement container was created but failed to start; old container was restored: %v", err)
		}
		started = true
	}

	oldRemoved := true
	removeWarning := ""
	if err := dockerRemoveForUpdate(cfg, backupName); err != nil {
		oldRemoved = false
		removeWarning = err.Error()
		if logger != nil {
			logger.Warn("[Docker] Replacement container started but old backup removal failed", "container", backupName, "error", err)
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":             "ok",
		"action":             "update",
		"container_id":       containerID,
		"name":               name,
		"image":              image,
		"new_container_id":   newID,
		"started":            started,
		"old_removed":        oldRemoved,
		"cleanup_warning":    removeWarning,
		"message":            fmt.Sprintf("Container %s updated from image %s", name, image),
		"requires_refresh":   true,
		"replacement_method": "pull-recreate",
	})
	return string(out)
}

func inspectContainerForUpdate(cfg DockerConfig, containerID string) (map[string]interface{}, error) {
	data, code, err := dockerRequest(cfg, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json", "")
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}
	if code == http.StatusNotFound {
		return nil, fmt.Errorf("container %q not found", containerID)
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("inspect container %q returned %s", containerID, dockerBodyMessage(code, data))
	}
	var inspect map[string]interface{}
	if err := json.Unmarshal(data, &inspect); err != nil {
		return nil, fmt.Errorf("failed to parse inspect data: %w", err)
	}
	return inspect, nil
}

func dockerContainerNameFromInspect(inspect map[string]interface{}) string {
	name, _ := inspect["Name"].(string)
	return strings.Trim(strings.TrimSpace(name), "/")
}

func dockerContainerImageFromInspect(inspect map[string]interface{}) string {
	if cfg, ok := inspect["Config"].(map[string]interface{}); ok {
		if image, ok := cfg["Image"].(string); ok {
			return strings.TrimSpace(image)
		}
	}
	return ""
}

func validateDockerImageReferenceForUpdate(image string) error {
	if image == "" {
		return fmt.Errorf("container has no reusable image reference")
	}
	if strings.HasPrefix(image, "sha256:") {
		return fmt.Errorf("container has no reusable image reference; it was created from an image ID")
	}
	if strings.ContainsAny(image, " \t\r\n") {
		return fmt.Errorf("image reference contains unsafe whitespace")
	}
	return nil
}

func pullDockerImageForUpdate(ctx context.Context, cfg DockerConfig, image string) error {
	reqURL := "http://localhost/" + dockerAPIVersion + "/images/create?fromImage=" + url.QueryEscape(image)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := getPullDockerClient(cfg).Do(req)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read pull stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := dockerBodyMessage(resp.StatusCode, data)
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}
	if msg := dockerPullStreamError(data); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func dockerPullStreamError(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.ErrorDetail.Message != "" {
			return event.ErrorDetail.Message
		}
		if event.Error != "" {
			return event.Error
		}
	}
	return ""
}

func dockerReplacementCreatePayload(inspect map[string]interface{}, image string) (map[string]interface{}, error) {
	config, ok := cloneDockerStringMap(inspect["Config"])
	if !ok {
		return nil, fmt.Errorf("container inspect data has no reusable Config")
	}
	config["Image"] = image
	delete(config, "Hostname")
	delete(config, "Domainname")

	if hostConfig, ok := cloneDockerStringMap(inspect["HostConfig"]); ok {
		config["HostConfig"] = hostConfig
	}
	if networking := dockerNetworkingConfigFromInspect(inspect); len(networking) > 0 {
		config["NetworkingConfig"] = networking
	}
	return config, nil
}

func dockerNetworkingConfigFromInspect(inspect map[string]interface{}) map[string]interface{} {
	networkSettings, ok := inspect["NetworkSettings"].(map[string]interface{})
	if !ok {
		return nil
	}
	networks, ok := networkSettings["Networks"].(map[string]interface{})
	if !ok || len(networks) == 0 {
		return nil
	}
	endpoints := map[string]interface{}{}
	for name, raw := range networks {
		source, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		endpoint := map[string]interface{}{}
		for _, key := range []string{"Aliases", "Links", "MacAddress", "DriverOpts"} {
			if value, ok := source[key]; ok && value != nil {
				endpoint[key] = value
			}
		}
		ipam, _ := cloneDockerStringMap(source["IPAMConfig"])
		if len(ipam) > 0 {
			endpoint["IPAMConfig"] = ipam
		}
		endpoints[name] = endpoint
	}
	if len(endpoints) == 0 {
		return nil
	}
	return map[string]interface{}{"EndpointsConfig": endpoints}
}

func cloneDockerStringMap(value interface{}) (map[string]interface{}, bool) {
	source, ok := value.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, false
	}
	clone := make(map[string]interface{}, len(source))
	for key, raw := range source {
		clone[key] = raw
	}
	return clone, true
}

func dockerInspectBool(inspect map[string]interface{}, section, key string) bool {
	m, ok := inspect[section].(map[string]interface{})
	if !ok {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

func dockerUpdateBackupName(name string) string {
	base := strings.Trim(strings.TrimSpace(name), "/")
	suffix := fmt.Sprintf("-aurago-old-%d", time.Now().Unix())
	maxBase := maxDockerNameLength - len(suffix)
	if maxBase < 1 {
		maxBase = 1
	}
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + suffix
}

func dockerStopForUpdate(cfg DockerConfig, containerID string) error {
	data, code, err := dockerRequest(cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/stop?t=10", "")
	if err != nil {
		return fmt.Errorf("failed to stop old container: %w", err)
	}
	if code == http.StatusNoContent || code == http.StatusNotModified {
		return nil
	}
	return fmt.Errorf("stop old container returned HTTP %d: %s", code, dockerBodyMessage(code, data))
}

func dockerRenameForUpdate(cfg DockerConfig, containerID, backupName string) error {
	data, code, err := dockerRequest(cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/rename?name="+url.QueryEscape(backupName), "")
	if err != nil {
		return fmt.Errorf("failed to rename old container: %w", err)
	}
	if code == http.StatusNoContent || code == http.StatusOK {
		return nil
	}
	return fmt.Errorf("rename old container returned HTTP %d: %s", code, dockerBodyMessage(code, data))
}

func dockerCreateForUpdate(cfg DockerConfig, name, body string) (string, error) {
	data, code, err := dockerRequest(cfg, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), body)
	if err != nil {
		return "", err
	}
	if code != http.StatusCreated {
		return "", fmt.Errorf("HTTP %d: %s", code, dockerBodyMessage(code, data))
	}
	var resp struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse create response: %w", err)
	}
	if strings.TrimSpace(resp.ID) == "" {
		return name, nil
	}
	return resp.ID, nil
}

func dockerStartForUpdate(cfg DockerConfig, containerID string) error {
	data, code, err := dockerRequest(cfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/start", "")
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	if code == http.StatusNoContent || code == http.StatusNotModified {
		return nil
	}
	return fmt.Errorf("start container returned HTTP %d: %s", code, dockerBodyMessage(code, data))
}

func dockerRemoveForUpdate(cfg DockerConfig, containerID string) error {
	data, code, err := dockerRequest(cfg, http.MethodDelete, "/containers/"+url.PathEscape(containerID)+"?v=true&force=true", "")
	if err != nil {
		return fmt.Errorf("failed to remove old container: %w", err)
	}
	if code == http.StatusNoContent || code == http.StatusOK {
		return nil
	}
	return fmt.Errorf("remove old container returned HTTP %d: %s", code, dockerBodyMessage(code, data))
}

func dockerRollbackUpdatedContainer(cfg DockerConfig, newID, backupName, originalName string, wasRunning bool) {
	if strings.TrimSpace(newID) != "" {
		_ = dockerRemoveForUpdate(cfg, newID)
	}
	_ = dockerRenameForUpdate(cfg, backupName, originalName)
	if wasRunning {
		_ = dockerStartForUpdate(cfg, originalName)
	}
}
