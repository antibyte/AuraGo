package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

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
		if err := json.Unmarshal(data, &resp); err != nil {
			return errJSON("Failed to parse create response: %v", err)
		}
		return fmt.Sprintf(`{"status":"ok","message":"Container created","id":"%v"}`, resp["Id"])
	}
	return dockerBodyErr(code, data)
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

// DockerListVolumes returns all Docker volumes.
func DockerListVolumes(cfg DockerConfig) string {
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
		return dockerBodyErr(code, data)
	}

	var execResp map[string]interface{}
	if err := json.Unmarshal(data, &execResp); err != nil {
		return errJSON("Failed to parse exec response: %v", err)
	}
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
		return dockerBodyErr(outCode, outData)
	}

	outputStr := stripDockerLogHeaders(outData)

	// Truncate output if too large to prevent memory issues
	const maxOutputLen = 64000 // ~64KB limit
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "\n... [output truncated due to size limit]"
	}

	result := map[string]interface{}{
		"status":       "ok",
		"container_id": containerID,
		"output":       outputStr,
	}

	// Inspect exec instance to get exit code
	inspectData, inspectCode, inspectErr := dockerRequest(cfg, "GET", "/exec/"+execID+"/json", "")
	if inspectErr == nil && inspectCode == 200 {
		var inspectResp map[string]interface{}
		if json.Unmarshal(inspectData, &inspectResp) == nil {
			if ec, ok := inspectResp["ExitCode"].(float64); ok {
				result["exit_code"] = int(ec)
				if int(ec) != 0 {
					result["status"] = "error"
				}
			}
		}
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
			Usage      uint64 `json:"usage"`
			Limit      uint64 `json:"limit"`
			Percent    float64 `json:"percent"`
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
		"status": "ok",
		"container_id": containerID,
		"cpu_percent": cpuPercent,
		"memory_usage_bytes": raw.MemoryStats.Usage,
		"memory_limit_bytes": raw.MemoryStats.Limit,
		"memory_percent": raw.MemoryStats.Percent,
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
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return errJSON("command is empty")
	}
	if !allowedComposeCommands[parts[0]] {
		return errJSON("compose command %q is not allowed", parts[0])
	}
	args := []string{"compose", "-f", file}
	args = append(args, parts...)
	return runDockerCLIHelper(cfg, args...)
}
