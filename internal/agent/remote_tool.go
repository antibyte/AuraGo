package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/config"
	"aurago/internal/remote"
)

func handleRemoteControl(tc ToolCall, cfg *config.Config, hub *remote.RemoteHub, logger *slog.Logger) string {
	if !cfg.RemoteControl.Enabled || hub == nil {
		return `Tool Output: {"status":"error","message":"Remote Control is disabled. Set remote_control.enabled=true in config.yaml."}`
	}
	if cfg.RemoteControl.ReadOnly {
		switch tc.Operation {
		case "execute_command", "write_file", "revoke_device":
			return `Tool Output: {"status":"error","message":"Remote Control is in read-only mode. Disable remote_control.read_only to allow changes."}`
		}
	}

	switch tc.Operation {
	case "list_devices":
		return remoteListDevices(hub, logger)
	case "device_status":
		return remoteDeviceStatus(hub, tc, logger)
	case "execute_command":
		return remoteExecuteCommand(hub, tc, logger)
	case "read_file":
		return remoteReadFile(hub, tc, logger)
	case "write_file":
		return remoteWriteFile(hub, tc, logger)
	case "list_files":
		return remoteListFiles(hub, tc, logger)
	case "sysinfo":
		return remoteSysinfo(hub, tc, logger)
	case "revoke_device":
		return remoteRevokeDevice(hub, tc, logger)
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown remote_control operation '%s'. Use: list_devices, device_status, execute_command, read_file, write_file, list_files, sysinfo, revoke_device"}`, tc.Operation)
	}
}

func resolveRemoteDevice(hub *remote.RemoteHub, tc ToolCall) (string, error) {
	deviceID := tc.DeviceID
	if deviceID == "" {
		deviceID = tc.ID
	}
	if deviceID == "" {
		deviceID = tc.Target
	}
	if deviceID != "" {
		return deviceID, nil
	}
	name := tc.DeviceName
	if name == "" {
		return "", fmt.Errorf("device_id or device_name is required")
	}
	device, err := remote.GetDeviceByName(hub.DB(), name)
	if err != nil {
		return "", fmt.Errorf("device not found by name '%s'", name)
	}
	return device.ID, nil
}

func remoteListDevices(hub *remote.RemoteHub, logger *slog.Logger) string {
	devices, err := remote.ListDevices(hub.DB())
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}

	type deviceView struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Hostname  string `json:"hostname"`
		OS        string `json:"os"`
		Arch      string `json:"arch"`
		IP        string `json:"ip_address"`
		Status    string `json:"status"`
		ReadOnly  bool   `json:"read_only"`
		Connected bool   `json:"connected"`
		LastSeen  string `json:"last_seen"`
		Version   string `json:"version"`
	}

	views := make([]deviceView, len(devices))
	for i, d := range devices {
		views[i] = deviceView{
			ID:        d.ID,
			Name:      d.Name,
			Hostname:  d.Hostname,
			OS:        d.OS,
			Arch:      d.Arch,
			IP:        d.IPAddress,
			Status:    d.Status,
			ReadOnly:  d.ReadOnly,
			Connected: hub.IsConnected(d.ID),
			LastSeen:  d.LastSeen,
			Version:   d.Version,
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(views),
		"devices": views,
	})
	return "Tool Output: " + string(data)
}

func remoteDeviceStatus(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}

	device, err := remote.GetDevice(hub.DB(), deviceID)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device not found: %s"}`, deviceID)
	}

	info := map[string]interface{}{
		"status":        "ok",
		"device_id":     device.ID,
		"name":          device.Name,
		"hostname":      device.Hostname,
		"os":            device.OS,
		"arch":          device.Arch,
		"ip":            device.IPAddress,
		"device_status": device.Status,
		"read_only":     device.ReadOnly,
		"allowed_paths": device.AllowedPaths,
		"connected":     hub.IsConnected(device.ID),
		"last_seen":     device.LastSeen,
		"version":       device.Version,
		"tags":          device.Tags,
	}

	// Add live telemetry if connected
	conn := hub.GetConnection(device.ID)
	if conn != nil {
		info["telemetry"] = conn.Telemetry
	}

	data, _ := json.Marshal(info)
	return "Tool Output: " + string(data)
}

func remoteExecuteCommand(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	command := tc.Command
	if command == "" {
		command = tc.Content
	}
	if command == "" {
		return `Tool Output: {"status":"error","message":"'command' is required for execute_command"}`
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpShellExec,
		Args: map[string]interface{}{
			"command": command,
		},
	}, 60*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"output":      result.Output,
		"error":       result.Error,
		"duration_ms": result.DurationMs,
	})
	return "Tool Output: " + string(data)
}

func remoteReadFile(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := tc.Path
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for read_file"}`
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileRead,
		Args:      map[string]interface{}{"path": path},
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if result.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, result.Error)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"path":    path,
		"content": result.Output,
	})
	return "Tool Output: " + string(data)
}

func remoteWriteFile(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := tc.Path
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for write_file"}`
	}
	content := tc.Content
	if content == "" {
		return `Tool Output: {"status":"error","message":"'content' is required for write_file"}`
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    path,
			"content": content,
		},
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if result.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, result.Error)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("file written to %s on device", path),
	})
	return "Tool Output: " + string(data)
}

func remoteListFiles(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := tc.Path
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for list_files"}`
	}

	args := map[string]interface{}{"path": path}
	if tc.Recursive {
		args["recursive"] = true
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileList,
		Args:      args,
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if result.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, result.Error)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"path":   path,
		"files":  result.Output,
	})
	return "Tool Output: " + string(data)
}

func remoteSysinfo(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpSysinfo,
	}, 15*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if result.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, result.Error)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"sysinfo": result.Output,
	})
	return "Tool Output: " + string(data)
}

func remoteRevokeDevice(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}

	if hub.IsConnected(deviceID) {
		_ = hub.SendRevoke(deviceID)
	}
	_ = remote.UpdateDeviceStatus(hub.DB(), deviceID, "revoked")

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("device %s has been revoked", deviceID),
	})
	return "Tool Output: " + string(data)
}
