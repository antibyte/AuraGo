package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/acestep"
	"aurago/internal/dockerutil"
)

// ContainerResources holds optional Docker host resource limits.
type ContainerResources struct {
	MemoryMB  int
	CPUCores  int
	PidsLimit int
}

// ContainerCreateOptions holds hardening options for Docker container creation.
type ContainerCreateOptions struct {
	User        string
	SecurityOpt []string
	CapDrop     []string
	CapAdd      []string
	NetworkMode string
	AutoRemove  bool
}

// DockerCreateContainer creates a new container from a configuration.
func DockerCreateContainer(cfg DockerConfig, name, image string, env []string, ports map[string]string, volumes []string, cmd []string, restart string, resources *ContainerResources) string {
	return DockerCreateContainerWithOptions(cfg, name, image, env, ports, volumes, cmd, restart, resources, ContainerCreateOptions{})
}

// DockerCreateContainerWithOptions creates a new container with optional security hardening.
func DockerCreateContainerWithOptions(cfg DockerConfig, name, image string, env []string, ports map[string]string, volumes []string, cmd []string, restart string, resources *ContainerResources, options ContainerCreateOptions) string {
	if err := requireDockerPermission(); err != nil {
		return errJSON("%v", err)
	}
	if image == "" {
		return errJSON("image is required")
	}
	if acestep.IsResourceName(name) || dockerutil.IsLocalLLMContainerName(name) {
		return errJSON("reserved AuraGo local LLM container name")
	}
	if dockerutil.IsBoringGarageContainerName(name) || dockerutil.IsHomepageContainerName(name) || dockerutil.IsAuraGoAppContainerName(name) {
		return errJSON("reserved AuraGo managed container name")
	}

	for _, bind := range volumes {
		if err := validateDockerBindMount(cfg, bind); err != nil {
			return errJSON("%v", err)
		}
	}

	payload := buildDockerCreateContainerPayloadWithOptions(image, env, ports, volumes, cmd, restart, resources, options)
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
	if dockerCreateSucceeded(code) {
		id := ""
		var resp map[string]interface{}
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &resp); err != nil {
				return errJSON("Failed to parse create response: %v", err)
			}
			if rawID, ok := resp["Id"]; ok && rawID != nil {
				id = fmt.Sprintf("%v", rawID)
			}
		}
		return fmt.Sprintf(`{"status":"ok","message":"Container created","id":"%s"}`, id)
	}
	return dockerBodyErr(code, data)
}

func buildDockerCreateContainerPayload(image string, env []string, ports map[string]string, volumes []string, cmd []string, restart string, resources *ContainerResources) map[string]interface{} {
	return buildDockerCreateContainerPayloadWithOptions(image, env, ports, volumes, cmd, restart, resources, ContainerCreateOptions{})
}

func buildDockerCreateContainerPayloadWithOptions(image string, env []string, ports map[string]string, volumes []string, cmd []string, restart string, resources *ContainerResources, options ContainerCreateOptions) map[string]interface{} {
	exposedPorts := map[string]interface{}{}
	portBindings := map[string]interface{}{}
	for containerPort, hostPort := range ports {
		if !strings.Contains(containerPort, "/") {
			containerPort += "/tcp"
		}
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []map[string]string{{"HostPort": hostPort}}
	}

	hostConfig := map[string]interface{}{
		"Binds":         volumes,
		"PortBindings":  portBindings,
		"RestartPolicy": map[string]interface{}{"Name": restart},
	}
	if options.AutoRemove {
		hostConfig["AutoRemove"] = true
	}
	if resources != nil {
		if resources.MemoryMB > 0 {
			memoryBytes := int64(resources.MemoryMB) * 1024 * 1024
			hostConfig["Memory"] = memoryBytes
			hostConfig["MemorySwap"] = memoryBytes
		}
		if resources.CPUCores > 0 {
			hostConfig["CpuQuota"] = int64(resources.CPUCores) * 100000
			hostConfig["CpuPeriod"] = int64(100000)
		}
		if resources.PidsLimit > 0 {
			hostConfig["PidsLimit"] = int64(resources.PidsLimit)
		}
	}
	if len(options.SecurityOpt) > 0 {
		hostConfig["SecurityOpt"] = append([]string(nil), options.SecurityOpt...)
	}
	if len(options.CapDrop) > 0 {
		hostConfig["CapDrop"] = append([]string(nil), options.CapDrop...)
	}
	if len(options.CapAdd) > 0 {
		hostConfig["CapAdd"] = append([]string(nil), options.CapAdd...)
	}
	if networkMode := strings.TrimSpace(options.NetworkMode); networkMode != "" {
		hostConfig["NetworkMode"] = networkMode
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
	if strings.TrimSpace(options.User) != "" {
		payload["User"] = strings.TrimSpace(options.User)
	}
	return payload
}

func dockerCreateSucceeded(code int) bool {
	return code >= 200 && code < 300
}

// DockerListNetworks returns all Docker networks.
func DockerListNetworks(cfg DockerConfig) string {
	data, code, err := dockerRequest(cfg, "GET", "/networks", "")
	if err != nil {
		return errJSON("Failed to list networks: %v", err)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
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

// DockerListVolumes returns Docker volumes while optionally hiding managed
// owners from agent-facing callers.
func DockerListVolumes(cfg DockerConfig, excludedOwners ...string) string {
	data, code, err := dockerRequest(cfg, "GET", "/volumes", "")
	if err != nil {
		return errJSON("Failed to list volumes: %v", err)
	}
	if code != 200 {
		return dockerBodyErr(code, data)
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
				name := fmt.Sprintf("%v", vol["Name"])
				if dockerManagedResourceExcluded(
					dockerStringLabels(vol["Labels"]),
					[]string{name},
					true,
					excludedOwners,
				) {
					continue
				}
				result = append(result, compact{
					Name:       name,
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
		return dockerBodyErr(code, data)
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

func dockerExecRaw(cfg DockerConfig, containerID string, cmdArray []string, user string, env []string) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return dockerExecRawContext(ctx, cfg, containerID, cmdArray, user, env)
}

func dockerExecRawContext(ctx context.Context, cfg DockerConfig, containerID string, cmdArray []string, user string, env []string) (int, string, error) {
	payload := map[string]interface{}{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmdArray,
		"Tty":          false,
	}
	if user != "" {
		payload["User"] = user
	}
	if len(env) > 0 {
		payload["Env"] = env
	}
	body, _ := json.Marshal(payload)

	// Create exec instance
	data, code, err := DockerRequestContext(ctx, cfg, "POST", "/containers/"+url.PathEscape(containerID)+"/exec", string(body))
	if err != nil {
		return -1, "", fmt.Errorf("failed to map exec: %w", err)
	}
	if code != 201 {
		return -1, "", fmt.Errorf("API error: %s", dockerBodyErr(code, data))
	}

	var execResp map[string]interface{}
	if err := json.Unmarshal(data, &execResp); err != nil {
		return -1, "", fmt.Errorf("failed to parse exec response: %w", err)
	}
	execID, ok := execResp["Id"].(string)
	if !ok || execID == "" {
		return -1, "", fmt.Errorf("failed to obtain exec ID")
	}

	// Start exec instance and read output
	startPayload := `{"Detach": false, "Tty": false}`
	outData, outCode, err := DockerRequestContext(ctx, cfg, "POST", "/exec/"+url.PathEscape(execID)+"/start", startPayload)
	if err != nil {
		return -1, "", fmt.Errorf("exec %s outcome uncertain after start; inspect before retrying: %w", execID, err)
	}
	if outCode != 200 {
		return -1, "", fmt.Errorf("API error: %s", dockerBodyErr(outCode, outData))
	}

	outputStr := stripDockerLogHeaders(outData)

	// Inspect exec instance to get exit code
	exitCode := -1
	inspectData, inspectCode, inspectErr := DockerRequestContext(ctx, cfg, "GET", "/exec/"+url.PathEscape(execID)+"/json", "")
	if inspectErr == nil && inspectCode == 200 {
		var inspectResp map[string]interface{}
		if json.Unmarshal(inspectData, &inspectResp) == nil {
			if ec, ok := inspectResp["ExitCode"].(float64); ok {
				exitCode = int(ec)
			}
		}
	}

	return exitCode, outputStr, nil
}

func dockerDetectShell(cfg DockerConfig, containerID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return dockerDetectShellContext(ctx, cfg, containerID)
}

func dockerDetectShellContext(ctx context.Context, cfg DockerConfig, containerID string) string {
	shells := []string{"/bin/sh", "/bin/bash", "/bin/ash", "sh", "bash"}
	for _, shell := range shells {
		if ctx.Err() != nil {
			return ""
		}
		ec, out, err := dockerExecRawContext(ctx, cfg, containerID, []string{shell, "-c", "echo 1"}, "", nil)
		if err == nil && ec == 0 && strings.TrimSpace(out) == "1" {
			return shell
		}
	}
	return ""
}

// dockerExecInternal executes a command inside a running container using the REST API.
// Pass env as nil when no additional environment variables are needed.
func dockerExecInternal(cfg DockerConfig, containerID, cmd, user string, env []string) string {
	return dockerExecInternalContext(context.Background(), cfg, containerID, cmd, user, env)
}

func dockerExecInternalContext(ctx context.Context, cfg DockerConfig, containerID, cmd, user string, env []string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}

	shell := dockerDetectShellContext(ctx, cfg, containerID)
	if shell == "" {
		shell = "/bin/sh"
	}
	cmdArray := []string{shell, "-c", cmd}

	exitCode, outputStr, err := dockerExecRawContext(ctx, cfg, containerID, cmdArray, user, env)
	if err != nil {
		return errJSON("%v", err)
	}

	// Truncate output if too large to prevent memory issues
	const maxOutputLen = 64000 // ~64KB limit
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "\n... [output truncated due to size limit]"
	}

	result := map[string]interface{}{
		"status":       "ok",
		"container_id": containerID,
		"output":       outputStr,
		"exit_code":    exitCode,
	}
	if exitCode != 0 {
		result["status"] = "error"
	}

	out, _ := json.Marshal(result)
	return string(out)
}

// DockerExec executes a command inside a running container using the REST API.
func DockerExec(cfg DockerConfig, containerID, cmd, user string) string {
	return dockerExecInternal(cfg, containerID, cmd, user, nil)
}

// DockerExecContext cancels HTTP execution with the caller's run. An interrupted
// connection does not prove that the command in the container has terminated.
func DockerExecContext(ctx context.Context, cfg DockerConfig, containerID, cmd, user string) string {
	return dockerExecInternalContext(ctx, cfg, containerID, cmd, user, nil)
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
		return dockerBodyErr(code, data)
	}

	// Parse and extract the most useful fields from the stats response
	var raw struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"pre_cpu_stats"`
		MemoryStats struct {
			Usage   uint64  `json:"usage"`
			Limit   uint64  `json:"limit"`
			Percent float64 `json:"percent"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return errJSON("Failed to parse stats: %v", err)
	}

	// Calculate CPU percentage
	var cpuPercent float64
	if raw.CPUStats.SystemCPUUsage > 0 && raw.CPUStats.SystemCPUUsage > raw.PreCPUStats.SystemCPUUsage {
		cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(raw.CPUStats.SystemCPUUsage - raw.PreCPUStats.SystemCPUUsage)
		numCPUs := runtime.NumCPU()
		if systemDelta > 0 && numCPUs > 0 {
			cpuPercent = (cpuDelta / systemDelta) * float64(numCPUs) * 100.0
		}
	}

	result := map[string]interface{}{
		"status":             "ok",
		"container_id":       containerID,
		"cpu_percent":        cpuPercent,
		"memory_usage_bytes": raw.MemoryStats.Usage,
		"memory_limit_bytes": raw.MemoryStats.Limit,
		"memory_percent":     raw.MemoryStats.Percent,
	}
	if raw.Networks != nil {
		var rxBytes, txBytes uint64
		for _, net := range raw.Networks {
			rxBytes += net.RxBytes
			txBytes += net.TxBytes
		}
		result["network_rx_bytes"] = rxBytes
		result["network_tx_bytes"] = txBytes
	}
	out, _ := json.Marshal(result)
	return string(out)
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
		return dockerBodyErr(code, data)
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
		return dockerBodyErr(code, data)
	}
	var full map[string]interface{}
	if err := json.Unmarshal(data, &full); err != nil {
		return errJSON("Failed to parse container info: %v", err)
	}

	ports, err := extractDockerPorts(full)
	if err != nil {
		return errJSON("Failed to parse container port info: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "ports": ports})
	return string(out)
}

func extractDockerPorts(full map[string]interface{}) (map[string]interface{}, error) {
	ports := map[string]interface{}{}
	netSettings, ok := full["NetworkSettings"]
	if !ok || netSettings == nil {
		return ports, nil
	}
	netSettingsMap, ok := netSettings.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected NetworkSettings type %T", netSettings)
	}
	p, ok := netSettingsMap["Ports"]
	if !ok || p == nil {
		return ports, nil
	}
	portsMap, ok := p.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected Ports type %T", p)
	}
	return portsMap, nil
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
		if err := json.Unmarshal(data, &r); err != nil {
			return errJSON("Failed to parse network response: %v", err)
		}
		return fmt.Sprintf(`{"status":"ok","message":"Network created","id":"%v"}`, r["Id"])
	}
	return dockerBodyErr(code, data)
}

// DockerRemoveNetwork removes a network.
func DockerRemoveNetwork(cfg DockerConfig, name string) string {
	if name == "" {
		return errJSON("network name required")
	}
	if err := validateDockerName(name); err != nil {
		return errJSON("invalid network name: %v", err)
	}
	data, code, err := dockerRequest(cfg, "DELETE", "/networks/"+url.PathEscape(name), "")
	if err != nil {
		return errJSON("Failed to remove network: %v", err)
	}
	if code == 204 || code == 200 {
		return `{"status":"ok","message":"Network removed"}`
	}
	return dockerBodyErr(code, data)
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
	return dockerBodyErr(code, data)
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
	return dockerBodyErr(code, data)
}

// DockerCreateVolume creates a volume.
func DockerCreateVolume(cfg DockerConfig, name, driver string) string {
	if name == "" {
		return errJSON("volume name required")
	}
	if acestep.IsResourceName(name) || dockerutil.IsLocalLLMVolumeName(name) {
		return errJSON("reserved AuraGo local LLM volume name")
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
	return dockerBodyErr(code, data)
}

// DockerRemoveVolume removes a volume.
func DockerRemoveVolume(cfg DockerConfig, name string, force bool) string {
	if name == "" {
		return errJSON("volume name required")
	}
	if acestep.IsResourceName(name) || dockerutil.IsLocalLLMVolumeName(name) {
		return errJSON("reserved AuraGo local LLM volume name")
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
	return dockerBodyErr(code, data)
}

// runDockerCLIHelper is used for operations like cp and compose which are notoriously difficult strictly via REST API.
func runDockerCLIHelper(cfg DockerConfig, args ...string) string {
	if err := requireDockerPermission(); err != nil {
		return errJSON("%v", err)
	}
	cmdArgs := dockerCLIArgs(cfg, args...)
	cmd := dockerCLICommand(context.Background(), cmdArgs...)
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

// dockerCLICommand prepares a docker CLI child process whose environment is
// AuraGo's without the master key: Compose interpolates ${VAR}, bare
// `environment: [VAR]` entries and `--build-arg VAR` from its own
// environment, so the key must never reach it. Every other variable is kept.
func dockerCLICommand(ctx context.Context, args ...string) *exec.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = dockerCLIEnvironment(os.Environ())
	return cmd
}

// dockerCLIEnvironment returns env without AURAGO_MASTER_KEY (any case).
func dockerCLIEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(strings.TrimSpace(name), DockerComposeMasterKeyVariable) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func dockerCLIArgs(cfg DockerConfig, args ...string) []string {
	var cmdArgs []string
	if host := strings.TrimSpace(cfg.Host); host != "" {
		cmdArgs = append(cmdArgs, "-H", host)
	}
	cmdArgs = append(cmdArgs, args...)
	return cmdArgs
}

// DockerCopy uses CLI to copy files to/from container.
func DockerCopy(cfg DockerConfig, containerID, src, dest, direction string) string {
	if err := requireDockerMutationPermission(); err != nil {
		return errJSON("%v", err)
	}
	if err := validateDockerName(containerID); err != nil {
		return errJSON("%v", err)
	}
	if src == "" || dest == "" {
		return errJSON("src and dest required")
	}

	stagingDir, err := os.MkdirTemp("", "aurago-docker-copy-*")
	if err != nil {
		return errJSON("cannot stage Docker copy: %v", err)
	}
	defer os.RemoveAll(stagingDir)
	var args []string
	var hostDest string
	if direction == "from_container" {
		containerSrc, err := validateDockerCopyContainerPath(src)
		if err != nil {
			return errJSON("%v", err)
		}
		hostDest, err = resolveDockerCopyHostPath(cfg, dest)
		if err != nil {
			return errJSON("%v", err)
		}
		if info, statErr := rootedToolStat(hostDest); statErr == nil && info.IsDir() {
			hostDest = filepath.Join(hostDest, pathpkg.Base(containerSrc))
		}
		if hostDest, err = secureResolve(cfg.WorkspaceDir, hostDest); err != nil {
			return errJSON("invalid host destination: %v", err)
		}
		if err := requireUnprotectedNotesPath(hostDest, true); err != nil {
			return errJSON("%v", err)
		}
		args = []string{"cp", containerID + ":" + containerSrc, filepath.Join(stagingDir, "payload")}
	} else if direction == "to_container" {
		hostSrc, err := resolveDockerCopyHostPath(cfg, src)
		if err != nil {
			return errJSON("%v", err)
		}
		containerDest, err := validateDockerCopyContainerPath(dest)
		if err != nil {
			return errJSON("%v", err)
		}
		input, err := rootedToolOpen(hostSrc)
		if err != nil {
			return errJSON("cannot open host source: %v", err)
		}
		defer input.Close()
		if info, err := input.Stat(); err != nil || !info.Mode().IsRegular() {
			return errJSON("Docker copy requires a regular host file")
		}
		stagedPath := filepath.Join(stagingDir, filepath.Base(hostSrc))
		staged, err := os.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return errJSON("cannot stage host source: %v", err)
		}
		_, copyErr := io.Copy(staged, input)
		closeErr := staged.Close()
		if copyErr != nil || closeErr != nil {
			return errJSON("cannot stage host source: %v %v", copyErr, closeErr)
		}
		args = []string{"cp", stagedPath, containerID + ":" + containerDest}
	} else {
		return errJSON("direction must be from_container or to_container")
	}

	result := runDockerCLIHelper(cfg, args...)
	if direction != "from_container" {
		return result
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(result), &status); err != nil || status.Status != "ok" {
		return result
	}
	output, err := os.Open(filepath.Join(stagingDir, "payload"))
	if err != nil {
		return errJSON("cannot open copied output: %v", err)
	}
	defer output.Close()
	if info, err := output.Stat(); err != nil || !info.Mode().IsRegular() {
		return errJSON("Docker copy requires a regular container file")
	}
	if err := rootedToolWriteFromReaderAtomic(hostDest, output); err != nil {
		return errJSON("cannot save copied output: %v", err)
	}
	return result
}

func resolveDockerCopyHostPath(cfg DockerConfig, userPath string) (string, error) {
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		return "", fmt.Errorf("docker cp host path validation requires a configured workspace directory")
	}
	resolved, err := secureResolve(cfg.WorkspaceDir, userPath)
	if err != nil {
		return "", fmt.Errorf("invalid host path %q: %w", userPath, err)
	}
	return resolved, nil
}

func validateDockerCopyContainerPath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", fmt.Errorf("container path is required")
	}
	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := pathpkg.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(normalized, "../") || strings.Contains(normalized, "/../") {
		return "", fmt.Errorf("invalid container path %q: path traversal blocked", rawPath)
	}
	return cleaned, nil
}

// allowedComposeCommands is the allowlist of valid docker compose subcommands.
var allowedComposeCommands = map[string]bool{
	"up": true, "down": true, "build": true, "ps": true, "logs": true,
	"start": true, "stop": true, "restart": true, "config": true,
	"pull": true, "images": true, "version": true, "rm": true,
	"pause": true, "unpause": true, "top": true, "push": true, "create": true,
	"exec": true, "run": true, "events": true, "port": true, "ls": true,
	"cp": true, "kill": true, "convert": true,
}

// DockerCompose uses CLI to trigger compose.
func DockerCompose(cfg DockerConfig, file, cmd string) string {
	if file == "" || cmd == "" {
		return errJSON("file and command required")
	}
	composeFile, err := validateDockerComposeFilePath(cfg, file)
	if err != nil {
		return errJSON("%v", err)
	}
	parts, err := dockerComposeParts(cmd)
	if err != nil {
		return dockerComposeErrorJSON(err)
	}
	plan, err := planDockerComposeOutput(cfg, parts)
	if err != nil {
		return dockerComposeErrorJSON(err)
	}
	if !dockerComposeReadOnlySubcommand(parts[0]) {
		if err := requireDockerMutationPermission(); err != nil {
			return errJSON("%v", err)
		}
	}
	args := []string{"compose", "-f", composeFile}
	if !plan.writesFile() {
		return dockerComposeCLIRunner(cfg, append(args, plan.args...)...)
	}
	// `config -o`: Compose renders into a private staging directory and the file
	// is published into the workspace afterwards (like docker cp), so Compose
	// itself never opens a path chosen by the agent. config stays a read-only
	// command, so this also works under docker.read_only.
	stagingDir, err := os.MkdirTemp("", "aurago-docker-compose-*")
	if err != nil {
		return errJSON("cannot stage the Compose output: %v", err)
	}
	defer os.RemoveAll(stagingDir)
	staged := filepath.Join(stagingDir, "out")
	result := dockerComposeCLIRunner(cfg, append(args, plan.argsWithOutput(staged)...)...)
	if !dockerResultOK(result) {
		return result
	}
	published, err := plan.publish(staged)
	if err != nil {
		return errJSON("cannot save the Compose output to %s: %v", plan.target, err)
	}
	if !published {
		return result // -q or a list flag: Compose wrote no file
	}
	return dockerResultWithField(result, "output_file", plan.target)
}

// dockerComposeCLIRunner runs one `docker compose` invocation. Tests replace it.
var dockerComposeCLIRunner = runDockerCLIHelper

// DockerComposeResolvedConfig returns Compose's fully interpolated model for a
// read-only policy preflight: stdout of `docker compose config --format json`
// only, so stderr warnings never corrupt the JSON. Errors carry stderr and are
// returned so callers fail closed before executing any Compose action.
func DockerComposeResolvedConfig(cfg DockerConfig, file string) (string, error) {
	return DockerComposeResolvedConfigContext(context.Background(), cfg, file, DockerComposeConfigOptions{})
}

// DockerComposeResolvedConfigContext is DockerComposeResolvedConfig under the
// caller's context (a stopped agent run cancels Compose) and with the selected
// config variant. Compose is still bounded by 20 seconds.
func DockerComposeResolvedConfigContext(ctx context.Context, cfg DockerConfig, file string, opts DockerComposeConfigOptions) (string, error) {
	if err := requireDockerPermission(); err != nil {
		return "", err
	}
	composeFile, err := validateDockerComposeFilePath(cfg, file)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	stdout, stderr, err := runDockerComposeConfig(runCtx, dockerComposeConfigArgs(cfg, composeFile, opts))
	if err != nil && ctx.Err() != nil {
		return "", fmt.Errorf("resolve Compose config: %w", ctx.Err())
	}
	return dockerComposeConfigResult(stdout, stderr, err)
}

// DockerComposeCommandMutates reports whether an allowed Compose command can
// change Docker state. Invalid commands are treated as mutating.
func DockerComposeCommandMutates(command string) bool {
	parts, err := dockerComposeParts(command)
	return err != nil || !dockerComposeReadOnlySubcommand(parts[0])
}

func validateDockerComposeFilePath(cfg DockerConfig, file string) (string, error) {
	rawFile := strings.TrimSpace(file)
	workspace := strings.TrimSpace(cfg.WorkspaceDir)
	candidate := rawFile
	if workspace != "" && !filepath.IsAbs(rawFile) {
		candidate = filepath.Join(workspace, rawFile)
	}
	absFile, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid compose file path: %w", err)
	}
	cleanFile := filepath.Clean(absFile)
	if workspace == "" {
		return cleanFile, nil
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("invalid workspace path: %w", err)
	}
	cleanWorkspace := filepath.Clean(absWorkspace)
	if !dockerPathEqualOrWithin(cleanFile, cleanWorkspace) {
		return "", &dockerComposeJailError{message: fmt.Sprintf("compose file path %q must stay within the configured workspace", cleanFile)}
	}
	if _, err := os.Lstat(cleanFile); err == nil {
		if resolved, err := filepath.EvalSymlinks(cleanFile); err == nil {
			resolved = filepath.Clean(resolved)
			if !dockerPathEqualOrWithin(resolved, cleanWorkspace) {
				return "", &dockerComposeJailError{message: fmt.Sprintf("compose file symlink target %q must stay within the configured workspace", resolved)}
			}
		}
	}
	return cleanFile, nil
}

func dockerComposeReadOnlySubcommand(subcommand string) bool {
	switch subcommand {
	case "ps", "logs", "config", "images", "version", "events", "port", "ls", "top":
		return true
	default:
		return false
	}
}

func validateDockerBindMount(cfg DockerConfig, bind string) error {
	spec, ok := parseDockerBindMount(bind)
	if !ok {
		return nil
	}
	if !spec.isHostPath {
		if acestep.IsResourceName(spec.hostPath) || dockerutil.IsLocalLLMVolumeName(spec.hostPath) {
			return fmt.Errorf("mounting reserved AuraGo local LLM volume %q is not allowed", spec.hostPath)
		}
		return nil // named Docker volume
	}
	hostPath := cleanDockerHostPath(spec.hostPath)
	if hostPath == "." || hostPath == "" {
		return nil
	}
	if isSensitiveDockerHostPath(hostPath) {
		return fmt.Errorf("mounting sensitive host path %q is not allowed for security reasons", hostPath)
	}
	resolvedHost, err := secureResolveFinalPath(filepath.FromSlash(hostPath))
	if err != nil {
		return fmt.Errorf("resolve bind mount host path: %w", err)
	}
	hostPath = cleanDockerHostPath(resolvedHost)
	if isSensitiveDockerHostPath(hostPath) {
		return fmt.Errorf("mounting sensitive host path %q is not allowed for security reasons", hostPath)
	}
	if cfg.WorkspaceDir != "" {
		resolvedWorkspace, err := secureResolveFinalPath(cfg.WorkspaceDir)
		if err != nil {
			return fmt.Errorf("resolve workspace directory: %w", err)
		}
		workspace := cleanDockerHostPath(resolvedWorkspace)
		if !dockerPathEqualOrWithin(hostPath, workspace) {
			return fmt.Errorf("bind mount host path %q must stay within the configured workspace", hostPath)
		}
	}
	return nil
}

type dockerBindMountSpec struct {
	hostPath   string
	isHostPath bool
}

func parseDockerBindMount(bind string) (dockerBindMountSpec, bool) {
	raw := strings.TrimSpace(bind)
	if raw == "" {
		return dockerBindMountSpec{}, false
	}
	sep := dockerBindHostSeparator(raw)
	if sep < 0 {
		return dockerBindMountSpec{}, false
	}
	hostPath := strings.TrimSpace(raw[:sep])
	if hostPath == "" {
		return dockerBindMountSpec{}, false
	}
	return dockerBindMountSpec{
		hostPath:   hostPath,
		isHostPath: dockerBindHostLooksLikePath(hostPath),
	}, true
}

func dockerBindHostSeparator(bind string) int {
	if isWindowsAbsolutePath(bind) {
		if idx := strings.Index(bind[2:], ":"); idx >= 0 {
			return idx + 2
		}
		return -1
	}
	return strings.Index(bind, ":")
}

func dockerBindHostLooksLikePath(hostPath string) bool {
	normalized := dockerutil.NormalizeHostPathForBind(hostPath)
	return strings.HasPrefix(normalized, "/") ||
		strings.HasPrefix(normalized, "//") ||
		isWindowsAbsolutePath(normalized)
}

func isWindowsAbsolutePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	return ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) &&
		path[1] == ':' &&
		(path[2] == '/' || path[2] == '\\')
}

func cleanDockerHostPath(hostPath string) string {
	normalized := dockerutil.NormalizeHostPathForBind(hostPath)
	if isWindowsAbsolutePath(normalized) {
		volume := normalized[:2]
		rest := pathpkg.Clean(normalized[2:])
		if rest == "." || rest == "" {
			rest = "/"
		}
		if !strings.HasPrefix(rest, "/") {
			rest = "/" + rest
		}
		return volume + rest
	}
	return pathpkg.Clean(normalized)
}

// sensitiveDockerHostPaths are host locations a bind may not be equal to or
// inside ("/" matches only itself).
var sensitiveDockerHostPaths = []string{
	"/", "/mnt", "/hostfs", "/etc", "/root", "/proc", "/sys", "/dev",
	"/var/run/docker.sock", "/run/docker.sock", "/var/lib/docker", "/boot",
}

// sensitiveWindowsDrivePaths are the same for every Windows drive.
var sensitiveWindowsDrivePaths = []string{
	"/windows",
	"/program files",
	"/program files (x86)",
	"/programdata",
}

func isSensitiveDockerHostPath(hostPath string) bool {
	for _, sensitive := range sensitiveDockerHostPaths {
		if dockerPathEqualOrWithin(hostPath, sensitive) {
			return true
		}
	}
	return isSensitiveWindowsHostPath(hostPath)
}

func isSensitiveWindowsHostPath(hostPath string) bool {
	return sensitiveWindowsHostLocation(hostPath) != ""
}

// sensitiveWindowsHostLocation returns the sensitive Windows location that
// hostPath is equal to or inside ("drive root" for a bare drive), or "".
func sensitiveWindowsHostLocation(hostPath string) string {
	normalized := strings.ToLower(strings.TrimRight(dockerutil.NormalizeHostPathForBind(hostPath), "/"))
	if len(normalized) == 2 && normalized[1] == ':' {
		return "drive root"
	}
	if !isWindowsAbsolutePath(normalized) {
		return ""
	}
	drivePath := normalized[2:]
	for _, sensitive := range sensitiveWindowsDrivePaths {
		if drivePath == sensitive || strings.HasPrefix(drivePath, sensitive+"/") {
			return sensitive
		}
	}
	return ""
}

func dockerPathEqualOrWithin(path, root string) bool {
	path = strings.TrimRight(cleanDockerHostPath(path), "/")
	root = strings.TrimRight(cleanDockerHostPath(root), "/")
	if isWindowsAbsolutePath(path) || isWindowsAbsolutePath(root) {
		path = strings.ToLower(path)
		root = strings.ToLower(root)
	}
	if path == "" {
		path = "/"
	}
	if root == "" {
		root = "/"
	}
	if root == "/" {
		return path == "/"
	}
	return path == root || strings.HasPrefix(path, root+"/")
}

func validateDockerComposeArgs(cmd string) error {
	_, err := dockerComposeParts(cmd)
	return err
}

func dockerComposeParts(cmd string) ([]string, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("command is empty")
	}
	if !allowedComposeCommands[parts[0]] {
		return nil, fmt.Errorf("compose command %q is not allowed", parts[0])
	}
	switch parts[0] {
	case "exec", "run", "cp", "push":
		return nil, fmt.Errorf("compose command %q is not allowed by the safe compose policy", parts[0])
	}
	for _, arg := range parts[1:] {
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, "--host") || strings.HasPrefix(lower, "--context") ||
			strings.HasPrefix(lower, "--tls") || strings.HasPrefix(lower, "-h=") ||
			lower == "-v" || lower == "--volume" || strings.HasPrefix(lower, "-v=") ||
			strings.HasPrefix(lower, "--volume=") || strings.HasPrefix(lower, "--mount") {
			return nil, fmt.Errorf("compose argument %q is not allowed", arg)
		}
		// --environment prints AuraGo's whole process environment; --env-file
		// would read an arbitrary host file into the model.
		if strings.HasPrefix(lower, "--environment") || strings.HasPrefix(lower, "--env-file") {
			return nil, dockerComposeDenied(dockerComposeArgumentDeniedCode, "compose argument %q is not allowed", arg)
		}
	}
	return parts, nil
}
