package desktopstore

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// ToolsDockerAdapter implements DockerAdapter through AuraGo's Docker Engine
// API helpers.
type ToolsDockerAdapter struct {
	Config tools.DockerConfig
	Logger *slog.Logger
}

// NewToolsDockerAdapter creates the production Docker adapter.
func NewToolsDockerAdapter(host, workspaceDir string, logger *slog.Logger) ToolsDockerAdapter {
	return ToolsDockerAdapter{
		Config: tools.DockerConfig{Host: host, WorkspaceDir: workspaceDir},
		Logger: logger,
	}
}

func (a ToolsDockerAdapter) PullImage(ctx context.Context, image string) error {
	return tools.PullImageForce(ctx, a.Config, image, a.Logger)
}

func (a ToolsDockerAdapter) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	payload := dockerCreatePayload(spec)
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal docker create payload: %w", err)
	}
	endpoint := "/containers/create"
	if strings.TrimSpace(spec.Name) != "" {
		endpoint += "?name=" + url.QueryEscape(spec.Name)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.Config, http.MethodPost, endpoint, string(body))
	if err != nil {
		return "", err
	}
	if code < 200 || code >= 300 {
		return "", dockerHTTPError("create container", code, data)
	}
	var resp struct {
		ID string `json:"Id"`
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &resp); err != nil {
			return "", fmt.Errorf("parse docker create response: %w", err)
		}
	}
	return resp.ID, nil
}

func (a ToolsDockerAdapter) CopyToContainer(ctx context.Context, containerName, destDir string, files map[string]string) error {
	containerName = strings.TrimSpace(containerName)
	destDir = pathpkg.Clean("/" + strings.TrimLeft(strings.TrimSpace(destDir), "/"))
	if containerName == "" {
		return fmt.Errorf("container name is required")
	}
	if destDir == "." || destDir == "" {
		destDir = "/"
	}
	if len(files) == 0 {
		return nil
	}
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for name, content := range files {
		cleanName := pathpkg.Clean(strings.TrimLeft(strings.TrimSpace(name), "/"))
		if cleanName == "." || cleanName == "" || strings.HasPrefix(cleanName, "../") || strings.Contains(cleanName, "/../") {
			_ = tw.Close()
			return fmt.Errorf("invalid container copy file name %q", name)
		}
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{Name: cleanName, Mode: 0o644, Size: int64(len(data))}); err != nil {
			_ = tw.Close()
			return fmt.Errorf("write container copy tar header: %w", err)
		}
		if _, err := tw.Write(data); err != nil {
			_ = tw.Close()
			return fmt.Errorf("write container copy tar body: %w", err)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close container copy tar: %w", err)
	}
	endpoint := "/containers/" + url.PathEscape(containerName) + "/archive?path=" + url.QueryEscape(destDir)
	data, code, err := tools.DockerRequestBytesContext(ctx, a.Config, http.MethodPut, endpoint, tarBuf.Bytes(), "application/x-tar")
	if err != nil {
		return err
	}
	if code == http.StatusOK || code == http.StatusNoContent {
		return nil
	}
	return dockerHTTPError("copy files to container", code, data)
}

func (a ToolsDockerAdapter) StartContainer(ctx context.Context, name string) error {
	return a.containerAction(ctx, name, http.MethodPost, "start")
}

func (a ToolsDockerAdapter) StopContainer(ctx context.Context, name string) error {
	return a.containerAction(ctx, name, http.MethodPost, "stop?t=10")
}

func (a ToolsDockerAdapter) RestartContainer(ctx context.Context, name string) error {
	return a.containerAction(ctx, name, http.MethodPost, "restart?t=10")
}

func (a ToolsDockerAdapter) RemoveContainer(ctx context.Context, name string, force bool) error {
	query := url.Values{}
	query.Set("v", "false")
	if force {
		query.Set("force", "true")
	}
	endpoint := "/containers/" + url.PathEscape(name) + "?" + query.Encode()
	data, code, err := tools.DockerRequestContext(ctx, a.Config, http.MethodDelete, endpoint, "")
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		return nil
	}
	if code == http.StatusNoContent || code == http.StatusOK {
		return nil
	}
	return dockerHTTPError("remove container", code, data)
}

func (a ToolsDockerAdapter) RemoveVolume(ctx context.Context, name string, force bool) error {
	endpoint := "/volumes/" + url.PathEscape(name)
	if force {
		endpoint += "?force=true"
	}
	data, code, err := tools.DockerRequestContext(ctx, a.Config, http.MethodDelete, endpoint, "")
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		return nil
	}
	if code == http.StatusNoContent || code == http.StatusOK {
		return nil
	}
	return dockerHTTPError("remove volume", code, data)
}

func (a ToolsDockerAdapter) CreateNetwork(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	payload := map[string]any{"Name": name, "Driver": "bridge"}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal docker network payload: %w", err)
	}
	data, code, err := tools.DockerRequestContext(ctx, a.Config, http.MethodPost, "/networks/create", string(body))
	if err != nil {
		return err
	}
	if code == http.StatusCreated || code == http.StatusConflict {
		return nil
	}
	return dockerHTTPError("create network", code, data)
}

func (a ToolsDockerAdapter) RemoveNetwork(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	data, code, err := tools.DockerRequestContext(ctx, a.Config, http.MethodDelete, "/networks/"+url.PathEscape(name), "")
	if err != nil {
		return err
	}
	if code == http.StatusNoContent || code == http.StatusOK || code == http.StatusNotFound {
		return nil
	}
	return dockerHTTPError("remove network", code, data)
}

func (a ToolsDockerAdapter) InspectContainer(ctx context.Context, name string) (ContainerState, error) {
	data, code, err := tools.DockerRequestContext(ctx, a.Config, http.MethodGet, "/containers/"+url.PathEscape(name)+"/json", "")
	if err != nil {
		return ContainerState{}, err
	}
	if code == http.StatusNotFound {
		return ContainerState{}, fmt.Errorf("container %s not found", name)
	}
	if code != http.StatusOK {
		return ContainerState{}, dockerHTTPError("inspect container", code, data)
	}
	var raw struct {
		Name  string `json:"Name"`
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
			Health  *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ContainerState{}, fmt.Errorf("parse docker inspect: %w", err)
	}
	state := ContainerState{
		Name:    strings.TrimPrefix(raw.Name, "/"),
		Running: raw.State.Running,
		Status:  raw.State.Status,
	}
	if raw.State.Health != nil {
		state.Health = raw.State.Health.Status
	}
	return state, nil
}

func (a ToolsDockerAdapter) containerAction(ctx context.Context, name, method, action string) error {
	endpoint := "/containers/" + url.PathEscape(name) + "/" + action
	data, code, err := tools.DockerRequestContext(ctx, a.Config, method, endpoint, "")
	if err != nil {
		return err
	}
	if code == http.StatusNotModified || code == http.StatusNoContent || code == http.StatusOK {
		return nil
	}
	if code == http.StatusNotFound {
		return fmt.Errorf("container %s not found", name)
	}
	return dockerHTTPError("container action", code, data)
}

func dockerCreatePayload(spec ContainerSpec) map[string]any {
	exposedPorts := map[string]any{}
	portBindings := map[string]any{}
	for _, binding := range spec.PortBindings {
		protocol := strings.ToLower(strings.TrimSpace(binding.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		key := strconv.Itoa(binding.ContainerPort) + "/" + protocol
		exposedPorts[key] = struct{}{}
		hostIP := strings.TrimSpace(binding.HostIP)
		if hostIP == "" {
			hostIP = "127.0.0.1"
		}
		portBindings[key] = []map[string]string{{
			"HostIp":   hostIP,
			"HostPort": strconv.Itoa(binding.HostPort),
		}}
	}
	binds := make([]string, 0, len(spec.Volumes))
	for _, volume := range spec.Volumes {
		if strings.TrimSpace(volume.Name) == "" || strings.TrimSpace(volume.ContainerPath) == "" {
			continue
		}
		binds = append(binds, volume.Name+":"+volume.ContainerPath)
	}
	for _, bind := range spec.HostBinds {
		if strings.TrimSpace(bind.HostPath) == "" || strings.TrimSpace(bind.ContainerPath) == "" {
			continue
		}
		mode := "rw"
		if bind.ReadOnly {
			mode = "ro"
		}
		binds = append(binds, dockerutil.FormatBindMount(bind.HostPath, bind.ContainerPath, mode))
	}
	restart := strings.TrimSpace(spec.Restart)
	if restart == "" {
		restart = "unless-stopped"
	}
	hostConfig := map[string]any{
		"Binds":         binds,
		"PortBindings":  portBindings,
		"RestartPolicy": map[string]any{"Name": restart},
		"SecurityOpt":   []string{"no-new-privileges:true"},
	}
	if len(spec.ExtraHosts) > 0 {
		hostConfig["ExtraHosts"] = append([]string(nil), spec.ExtraHosts...)
	}
	if strings.TrimSpace(spec.NetworkMode) != "" {
		hostConfig["NetworkMode"] = strings.TrimSpace(spec.NetworkMode)
	}
	payload := map[string]any{
		"Image":        spec.Image,
		"Env":          append([]string(nil), spec.Env...),
		"ExposedPorts": exposedPorts,
		"HostConfig":   hostConfig,
		"Labels":       spec.Labels,
	}
	return payload
}

func dockerHTTPError(action string, code int, data []byte) error {
	var dockerMsg struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &dockerMsg) == nil && strings.TrimSpace(dockerMsg.Message) != "" {
		return fmt.Errorf("%s failed: Docker HTTP %d: %s", action, code, dockerMsg.Message)
	}
	msg := strings.TrimSpace(string(data))
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	if msg == "" {
		msg = http.StatusText(code)
	}
	return fmt.Errorf("%s failed: Docker HTTP %d: %s", action, code, msg)
}
