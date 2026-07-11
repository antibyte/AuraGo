package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		case "execute_command", "shell_session_start", "shell_session_read", "shell_session_input", "shell_session_stop", "shell_session_list", "write_file", "file_patch", "revoke_device", "edit_file", "json_edit", "yaml_edit", "xml_edit", "desktop_input", "desktop_ui_action", "desktop_browser_action":
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
	case "shell_session_start", "shell_session_read", "shell_session_input", "shell_session_stop", "shell_session_list":
		return remoteShellSessionCommand(hub, tc, logger)
	case "read_file":
		return remoteReadFile(hub, tc, logger)
	case "write_file":
		return remoteWriteFile(cfg, hub, tc, logger)
	case "file_patch":
		return remoteFilePatch(cfg, hub, tc, logger)
	case "list_files":
		return remoteListFiles(hub, tc, logger)
	case "sysinfo":
		return remoteSysinfo(hub, tc, logger)
	case "revoke_device":
		return remoteRevokeDevice(hub, tc, logger)
	case "edit_file":
		return remoteEditFile(hub, tc, logger)
	case "json_edit":
		return remoteJsonEdit(hub, tc, logger)
	case "yaml_edit":
		return remoteYamlEdit(hub, tc, logger)
	case "xml_edit":
		return remoteXmlEdit(hub, tc, logger)
	case "file_search":
		return remoteFileSearch(hub, tc, logger)
	case "file_read_advanced":
		return remoteFileReadAdvanced(hub, tc, logger)
	case "desktop_screenshot":
		return remoteDesktopScreenshot(cfg, hub, tc, logger)
	case "desktop_permission_request":
		return remoteDesktopPermissionRequest(hub, tc, logger)
	case "desktop_input":
		return remoteDesktopInput(hub, tc, logger)
	case "desktop_list_displays":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopListDisplays, 15*time.Second, false)
	case "desktop_list_windows":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopListWindows, 15*time.Second, false)
	case "desktop_active_window":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopActiveWindow, 15*time.Second, false)
	case "desktop_host_info":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopHostInfo, 15*time.Second, false)
	case "desktop_ui_tree":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopUITree, 30*time.Second, false)
	case "desktop_ui_action":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopUIAction, 15*time.Second, true)
	case "desktop_browser_connect":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopBrowserConnect, 30*time.Second, false)
	case "desktop_browser_snapshot":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopBrowserSnapshot, 30*time.Second, false)
	case "desktop_browser_action":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopBrowserAction, 15*time.Second, true)
	case "desktop_browser_disconnect":
		return remoteDesktopJSONCommand(hub, tc, remote.OpDesktopBrowserDisconnect, 15*time.Second, false)
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown remote_control operation '%s'. Use: list_devices, device_status, execute_command, shell_session_start, shell_session_read, shell_session_input, shell_session_stop, shell_session_list, read_file, write_file, file_patch, list_files, sysinfo, revoke_device, edit_file, json_edit, yaml_edit, xml_edit, file_search, file_read_advanced, desktop_screenshot, desktop_permission_request, desktop_input, desktop_list_displays, desktop_list_windows, desktop_active_window, desktop_host_info, desktop_ui_tree, desktop_ui_action, desktop_browser_connect, desktop_browser_snapshot, desktop_browser_action, desktop_browser_disconnect"}`, tc.Operation)
	}
}

func resolveRemoteDevice(hub *remote.RemoteHub, tc ToolCall) (string, error) {
	deviceID := strings.TrimSpace(firstNonEmptyToolString(
		tc.DeviceID,
		tc.ID,
		tc.Target,
		toolArgString(tc.Params, "device_id", "deviceId", "deviceID"),
	))
	if deviceID != "" {
		return deviceID, nil
	}
	name := strings.TrimSpace(firstNonEmptyToolString(
		tc.DeviceName,
		toolArgString(tc.Params, "device_name", "deviceName"),
	))
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

func remoteShellSessionCommand(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	params := nestedRemoteToolParams(tc.Params)
	operation := strings.TrimSpace(firstNonEmptyToolString(tc.Operation, toolArgString(params, "operation")))
	remoteOp := ""
	args := map[string]interface{}{}
	switch operation {
	case "shell_session_start":
		remoteOp = remote.OpShellSessionStart
		command := firstNonEmptyToolString(toolArgString(params, "command"), tc.Command, tc.Content)
		if command == "" {
			return `Tool Output: {"status":"error","message":"'command' is required for shell_session_start"}`
		}
		args["command"] = command
		copyRemoteArgIfPresent(args, params, "cwd_id")
		copyRemoteArgIfPresent(args, params, "initial_wait_ms")
	case "shell_session_read":
		remoteOp = remote.OpShellSessionRead
		if !copyRequiredRemoteStringArg(args, params, "session_id") {
			return `Tool Output: {"status":"error","message":"'session_id' is required for shell_session_read"}`
		}
		copyRemoteArgIfPresent(args, params, "offset")
		copyRemoteArgIfPresent(args, params, "limit")
		copyRemoteArgIfPresent(args, params, "wait_ms")
	case "shell_session_input":
		remoteOp = remote.OpShellSessionInput
		if !copyRequiredRemoteStringArg(args, params, "session_id") {
			return `Tool Output: {"status":"error","message":"'session_id' is required for shell_session_input"}`
		}
		input := firstNonEmptyToolString(toolArgString(params, "input"), tc.Content, tc.Value)
		if input == "" {
			return `Tool Output: {"status":"error","message":"'input' is required for shell_session_input"}`
		}
		args["input"] = input
	case "shell_session_stop":
		remoteOp = remote.OpShellSessionStop
		if !copyRequiredRemoteStringArg(args, params, "session_id") {
			return `Tool Output: {"status":"error","message":"'session_id' is required for shell_session_stop"}`
		}
	case "shell_session_list":
		remoteOp = remote.OpShellSessionList
		copyRemoteArgIfPresent(args, params, "limit")
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"unsupported shell session operation %q"}`, operation)
	}
	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remoteOp,
		Args:      args,
	}, remoteShellSessionTimeout(params))
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	return remoteCommandResultOutput(result, "result")
}

func copyRemoteArgIfPresent(args map[string]interface{}, params map[string]interface{}, key string) {
	if raw, ok := toolArgRaw(params, key); ok {
		args[key] = raw
	}
}

func copyRequiredRemoteStringArg(args map[string]interface{}, params map[string]interface{}, key string) bool {
	value := strings.TrimSpace(toolArgString(params, key))
	if value == "" {
		return false
	}
	args[key] = value
	return true
}

func remoteShellSessionTimeout(params map[string]interface{}) time.Duration {
	waitMS := toolArgInt(params, 0, "wait_ms")
	if waitMS <= 0 {
		waitMS = toolArgInt(params, 0, "initial_wait_ms")
	}
	if waitMS <= 0 {
		return 60 * time.Second
	}
	timeout := time.Duration(waitMS+5000) * time.Millisecond
	if timeout < 10*time.Second {
		return 10 * time.Second
	}
	if timeout > 120*time.Second {
		return 120 * time.Second
	}
	return timeout
}

func remoteCommandResultOutput(result remote.ResultPayload, resultKey string) string {
	status := strings.TrimSpace(result.Status)
	if status == "" {
		status = "ok"
	}
	if result.Error != "" || !strings.EqualFold(status, "ok") {
		if status == "" || strings.EqualFold(status, "ok") {
			status = "error"
		}
		data := map[string]interface{}{
			"status":  status,
			"message": result.Error,
		}
		if result.ErrorCode != "" {
			data["error_code"] = result.ErrorCode
		}
		if result.Output != "" {
			data[resultKey] = result.Output
		}
		raw, _ := json.Marshal(data)
		return "Tool Output: " + string(raw)
	}
	data := map[string]interface{}{
		"status": status,
	}
	if result.Output != "" {
		data[resultKey] = result.Output
	}
	if result.DurationMs > 0 {
		data["duration_ms"] = result.DurationMs
	}
	raw, _ := json.Marshal(data)
	return "Tool Output: " + string(raw)
}

func remoteReadFile(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	params := nestedRemoteToolParams(tc.Params)
	path := firstNonEmptyToolString(toolArgString(params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for read_file"}`
	}

	args := map[string]interface{}{"path": path}
	if rootID := strings.TrimSpace(toolArgString(params, "root_id")); rootID != "" {
		args["root_id"] = rootID
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileRead,
		Args:      args,
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

func remoteWriteFile(cfg *config.Config, hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
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
	if err := validateRemoteWriteContentSize(cfg, content); err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}

	args := map[string]interface{}{
		"path":    path,
		"content": content,
	}
	if rootID := strings.TrimSpace(toolArgString(tc.Params, "root_id")); rootID != "" {
		args["root_id"] = rootID
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileWrite,
		Args:      args,
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

func remoteFilePatch(cfg *config.Config, hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	params := nestedRemoteToolParams(tc.Params)
	path := firstNonEmptyToolString(toolArgString(params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for file_patch"}`
	}
	expectedSHA := strings.TrimSpace(toolArgString(params, "expected_sha256"))
	if expectedSHA == "" {
		return `Tool Output: {"status":"error","message":"'expected_sha256' is required for file_patch"}`
	}
	patches, ok := toolArgRaw(params, "patches")
	if !ok || patches == nil {
		return `Tool Output: {"status":"error","message":"'patches' is required for file_patch"}`
	}
	if err := validateRemotePatchPayloadSize(cfg, patches); err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	dryRun := true
	if value, ok := toolArgBool(params, "dry_run"); ok {
		dryRun = value
	}
	args := map[string]interface{}{
		"path":            path,
		"expected_sha256": expectedSHA,
		"patches":         patches,
		"dry_run":         dryRun,
	}
	if rootID := strings.TrimSpace(toolArgString(params, "root_id")); rootID != "" {
		args["root_id"] = rootID
	}
	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFilePatch,
		Args:      args,
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	return remoteCommandResultOutput(result, "result")
}

func validateRemoteWriteContentSize(cfg *config.Config, content string) error {
	maxMB := cfg.RemoteControl.MaxFileSizeMB
	if maxMB <= 0 {
		maxMB = remote.DefaultMaxFileSizeMB
	}
	maxBytes := int64(maxMB) * 1024 * 1024
	if int64(base64.StdEncoding.DecodedLen(len(content))) > maxBytes {
		return fmt.Errorf("file exceeds remote_control.max_file_size_mb limit")
	}
	return nil
}

func validateRemotePatchPayloadSize(cfg *config.Config, patches interface{}) error {
	if cfg == nil {
		return nil
	}
	maxMB := cfg.RemoteControl.MaxFileSizeMB
	if maxMB <= 0 {
		maxMB = remote.DefaultMaxFileSizeMB
	}
	maxBytes := int64(maxMB) * 1024 * 1024
	raw, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("invalid patches payload: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return fmt.Errorf("patches exceed remote_control.max_file_size_mb limit")
	}
	return nil
}

func remoteListFiles(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	params := nestedRemoteToolParams(tc.Params)
	path := firstNonEmptyToolString(toolArgString(params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for list_files"}`
	}

	args := map[string]interface{}{"path": path}
	if rootID := strings.TrimSpace(toolArgString(params, "root_id")); rootID != "" {
		args["root_id"] = rootID
	}
	if recursive, ok := toolArgBool(params, "recursive"); ok {
		args["recursive"] = recursive
	} else if tc.Recursive {
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

func remoteDesktopScreenshot(cfg *config.Config, hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	args := map[string]interface{}{}
	if displayID := strings.TrimSpace(toolArgString(tc.Params, "display_id")); displayID != "" {
		args["display_id"] = displayID
	}
	if windowID := strings.TrimSpace(toolArgString(tc.Params, "window_id")); windowID != "" {
		args["window_id"] = windowID
	}
	if format := strings.TrimSpace(toolArgString(tc.Params, "format")); format != "" {
		args["format"] = format
	}
	if quality := toolArgInt(tc.Params, 0, "quality"); quality > 0 {
		args["quality"] = quality
	}
	includeData, _ := toolArgBool(tc.Params, "include_data_base64")

	result, err := remoteDesktopCommand(hub, tc, remote.OpDesktopScreenshot, args, 30*time.Second)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data, err := parseRemoteDesktopOutput(result.Output)
	if err != nil {
		return remoteToolError(err.Error())
	}
	if err := storeRemoteDesktopScreenshot(cfg, result.CommandID, data, includeData); err != nil {
		return remoteToolError(err.Error())
	}
	data["status"] = "ok"
	respData, _ := json.Marshal(data)
	return "Tool Output: " + string(respData)
}

func remoteDesktopPermissionRequest(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	result, err := remoteDesktopCommand(hub, tc, remote.OpDesktopPermissionRequest, copyRemoteDesktopParams(tc.Params), 15*time.Second)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data, err := parseRemoteDesktopOutput(result.Output)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data["status"] = "ok"
	respData, _ := json.Marshal(data)
	return "Tool Output: " + string(respData)
}

func remoteDesktopInput(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	args := copyRemoteDesktopParams(tc.Params)
	if strings.TrimSpace(toolArgString(args, "kind")) == "" {
		return `Tool Output: {"status":"error","message":"'kind' is required for desktop_input"}`
	}
	if inputAction := strings.TrimSpace(toolArgString(args, "input_action")); inputAction != "" {
		if strings.TrimSpace(toolArgString(args, "action")) == "" {
			args["action"] = inputAction
		}
		delete(args, "input_action")
	}
	result, err := remoteDesktopCommand(hub, tc, remote.OpDesktopInput, args, 10*time.Second)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data, err := parseRemoteDesktopOutput(result.Output)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data["status"] = "ok"
	respData, _ := json.Marshal(data)
	return "Tool Output: " + string(respData)
}

func remoteDesktopJSONCommand(hub *remote.RemoteHub, tc ToolCall, operation string, timeout time.Duration, requireAction bool) string {
	args := copyRemoteDesktopParams(tc.Params)
	if tc.Action != "" && strings.TrimSpace(toolArgString(args, "action")) == "" {
		args["action"] = tc.Action
	}
	if requireAction && strings.TrimSpace(toolArgString(args, "action")) == "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"'action' is required for %s"}`, operation)
	}
	result, err := remoteDesktopCommand(hub, tc, operation, args, timeout)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data, err := parseRemoteDesktopOutput(result.Output)
	if err != nil {
		return remoteToolError(err.Error())
	}
	data["status"] = "ok"
	respData, _ := json.Marshal(data)
	return "Tool Output: " + string(respData)
}

func remoteDesktopCommand(hub *remote.RemoteHub, tc ToolCall, operation string, args map[string]interface{}, timeout time.Duration) (remote.ResultPayload, error) {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return remote.ResultPayload{}, err
	}
	if !hub.IsConnected(deviceID) {
		return remote.ResultPayload{}, fmt.Errorf("device %s is not connected", deviceID)
	}
	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: operation,
		Args:      args,
	}, timeout)
	if err != nil {
		return remote.ResultPayload{}, err
	}
	if result.Status != "" && result.Status != "ok" {
		message := result.Error
		if message == "" {
			message = "desktop command returned status " + result.Status
		}
		return result, fmt.Errorf("%s", message)
	}
	if result.Error != "" {
		return result, fmt.Errorf("%s", result.Error)
	}
	return result, nil
}

func parseRemoteDesktopOutput(output string) (map[string]interface{}, error) {
	if strings.TrimSpace(output) == "" {
		return map[string]interface{}{}, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("invalid desktop result payload: %w", err)
	}
	return data, nil
}

func storeRemoteDesktopScreenshot(cfg *config.Config, commandID string, data map[string]interface{}, includeData bool) error {
	encoded, _ := data["data_base64"].(string)
	if strings.TrimSpace(encoded) == "" {
		return fmt.Errorf("missing desktop screenshot data_base64")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid desktop screenshot data: %w", err)
	}
	root := ""
	if cfg != nil {
		root = strings.TrimSpace(cfg.Directories.WorkspaceDir)
	}
	if root == "" {
		root = filepath.Join("agent_workspace", "workdir")
	}
	dir := filepath.Join(root, "agodesk_screenshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create screenshot directory: %w", err)
	}
	ext := remoteDesktopScreenshotExt(data)
	filename := sanitizeRemoteDesktopFilename(commandID)
	if filename == "" {
		filename = fmt.Sprintf("screenshot-%d", time.Now().UnixNano())
	}
	path := filepath.Join(dir, filename+ext)
	if err := os.WriteFile(path, decoded, 0600); err != nil {
		return fmt.Errorf("write screenshot: %w", err)
	}
	if !includeData {
		delete(data, "data_base64")
	}
	data["screenshot_path"] = path
	data["bytes"] = len(decoded)
	return nil
}

func remoteDesktopScreenshotExt(data map[string]interface{}) string {
	format := strings.ToLower(strings.TrimSpace(fmt.Sprint(data["format"])))
	mime := strings.ToLower(strings.TrimSpace(fmt.Sprint(data["mime"])))
	switch {
	case format == "jpeg" || format == "jpg" || mime == "image/jpeg":
		return ".jpg"
	case format == "webp" || mime == "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func sanitizeRemoteDesktopFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func copyRemoteDesktopParams(params map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for key, value := range params {
		switch key {
		case "operation", "device_id", "device_name", "include_data_base64":
			continue
		default:
			out[key] = value
		}
	}
	return out
}

func remoteToolError(message string) string {
	return `Tool Output: {"status":"error","message":"` + escapeJSONMessage(message) + `"}`
}

func remoteRevokeDevice(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}

	if hub.IsConnected(deviceID) {
		if err := hub.SendRevoke(deviceID); err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to revoke device %s: %s"}`, deviceID, err.Error())
		}
	} else if hub.DB() != nil {
		if err := remote.UpdateDeviceStatus(hub.DB(), deviceID, "revoked"); err != nil {
			if logger != nil {
				logger.Warn("Failed to persist revoked device status", "device_id", deviceID, "error", err)
			}
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to persist revoked status for device %s: %s"}`, deviceID, err.Error())
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("device %s has been revoked", deviceID),
	})
	return "Tool Output: " + string(data)
}

func remoteEditFile(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for edit_file"}`
	}
	editOp := tc.Action
	if editOp == "" {
		return `Tool Output: {"status":"error","message":"'action' is required for edit_file (str_replace, insert_after, append, etc.)"}`
	}

	args := map[string]interface{}{
		"path":      path,
		"operation": editOp,
	}
	if old := toolArgString(tc.Params, "old"); old != "" {
		args["old"] = old
	}
	if value := toolArgString(tc.Params, "new"); value != "" {
		args["new"] = value
	}
	if marker := toolArgString(tc.Params, "marker"); marker != "" {
		args["marker"] = marker
	}
	if content := firstNonEmptyToolString(toolArgString(tc.Params, "content"), tc.Content); content != "" {
		args["content"] = content
	}
	if startLine := toolArgInt(tc.Params, 0, "start_line"); startLine > 0 {
		args["start_line"] = startLine
	}
	if endLine := toolArgInt(tc.Params, 0, "end_line"); endLine > 0 {
		args["end_line"] = endLine
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileEdit,
		Args:      args,
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if result.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, result.Error)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": result.Output,
	})
	return "Tool Output: " + string(data)
}

func remoteJsonEdit(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for json_edit"}`
	}
	editOp := tc.Action
	if editOp == "" {
		return `Tool Output: {"status":"error","message":"'action' is required for json_edit (get, set, delete, keys, validate, format)"}`
	}

	args := map[string]interface{}{
		"path":      path,
		"operation": editOp,
	}
	if jsonPath := toolArgString(tc.Params, "json_path"); jsonPath != "" {
		args["json_path"] = jsonPath
	}
	if setValue, ok := toolArgRaw(tc.Params, "set_value"); ok && setValue != nil {
		args["set_value"] = setValue
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpJsonEdit,
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
		"result": result.Output,
	})
	return "Tool Output: " + string(data)
}

func remoteYamlEdit(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for yaml_edit"}`
	}
	editOp := tc.Action
	if editOp == "" {
		return `Tool Output: {"status":"error","message":"'action' is required for yaml_edit (get, set, delete, keys, validate)"}`
	}

	args := map[string]interface{}{
		"path":      path,
		"operation": editOp,
	}
	if jsonPath := toolArgString(tc.Params, "json_path"); jsonPath != "" {
		args["json_path"] = jsonPath
	}
	if setValue, ok := toolArgRaw(tc.Params, "set_value"); ok && setValue != nil {
		args["set_value"] = setValue
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpYamlEdit,
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
		"result": result.Output,
	})
	return "Tool Output: " + string(data)
}

func remoteXmlEdit(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}

	path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for xml_edit"}`
	}

	op := tc.Action
	if op == "" {
		return `Tool Output: {"status":"error","message":"'action' is required for xml_edit (get, set_text, set_attribute, add_element, delete, validate, format)"}`
	}

	args := map[string]interface{}{
		"path":      path,
		"operation": op,
	}
	xpath := firstNonEmptyToolString(toolArgString(tc.Params, "xpath"), toolArgString(tc.Params, "json_path"))
	if xpath != "" {
		args["xpath"] = xpath
	}
	if setValue, ok := toolArgRaw(tc.Params, "set_value"); ok && setValue != nil {
		args["set_value"] = setValue
	}

	result, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpXmlEdit,
		Args:      args,
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if result.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, result.Error)
	}

	respData, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"result": result.Output,
	})
	return "Tool Output: " + string(respData)
}

func remoteFileSearch(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}

	params := nestedRemoteToolParams(tc.Params)
	op := remoteFileSearchOperation(tc, params)

	args := map[string]interface{}{
		"operation": op,
		"pattern":   toolArgString(params, "pattern"),
	}
	path := firstNonEmptyToolString(toolArgString(params, "path", "file_path"), tc.Path, tc.FilePath)
	if path != "" {
		args["path"] = path
	}
	if rootID := strings.TrimSpace(toolArgString(params, "root_id")); rootID != "" {
		args["root_id"] = rootID
	}
	if glob := toolArgString(params, "glob"); glob != "" {
		args["glob"] = glob
	}
	if outputMode := toolArgString(params, "output_mode"); outputMode != "" {
		args["output_mode"] = outputMode
	}

	cmdResult, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileSearch,
		Args:      args,
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if cmdResult.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, cmdResult.Error)
	}

	respData, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"result": cmdResult.Output,
	})
	return "Tool Output: " + string(respData)
}

func nestedRemoteToolParams(params map[string]interface{}) map[string]interface{} {
	nested, ok := params["params"].(map[string]interface{})
	if !ok || len(nested) == 0 {
		return params
	}
	merged := make(map[string]interface{}, len(params)+len(nested))
	for key, value := range params {
		if key == "params" {
			continue
		}
		merged[key] = value
	}
	for key, value := range nested {
		merged[key] = value
	}
	return merged
}

func remoteFileSearchOperation(tc ToolCall, params map[string]interface{}) string {
	for _, candidate := range []string{
		toolArgString(params, "operation"),
		tc.SubOperation,
		toolArgString(params, "action"),
		tc.Action,
	} {
		if isRemoteFileSearchOperation(candidate) {
			return strings.TrimSpace(candidate)
		}
	}
	rawOp := strings.TrimSpace(toolArgString(params, "operation"))
	if rawOp != "" && rawOp != remote.OpFileSearch && rawOp != "remote_control" {
		return rawOp
	}
	return "grep"
}

func isRemoteFileSearchOperation(op string) bool {
	switch strings.TrimSpace(op) {
	case "grep", "grep_recursive", "find":
		return true
	default:
		return false
	}
}

func remoteFileReadAdvanced(hub *remote.RemoteHub, tc ToolCall, logger *slog.Logger) string {
	deviceID, err := resolveRemoteDevice(hub, tc)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if !hub.IsConnected(deviceID) {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"device %s is not connected"}`, deviceID)
	}
	path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
	if path == "" {
		return `Tool Output: {"status":"error","message":"'path' is required for file_read_advanced"}`
	}

	op := tc.Action
	if op == "" {
		op = "read_lines"
	}

	args := map[string]interface{}{
		"path":      path,
		"operation": op,
	}
	if pattern := toolArgString(tc.Params, "pattern"); pattern != "" {
		args["pattern"] = pattern
	}
	if startLine := toolArgInt(tc.Params, 0, "start_line"); startLine > 0 {
		args["start_line"] = startLine
	}
	if endLine := toolArgInt(tc.Params, 0, "end_line"); endLine > 0 {
		args["end_line"] = endLine
	}
	if lineCount := toolArgInt(tc.Params, 0, "line_count"); lineCount > 0 {
		args["line_count"] = lineCount
	}

	cmdResult, err := hub.SendCommand(deviceID, remote.CommandPayload{
		Operation: remote.OpFileReadAdv,
		Args:      args,
	}, 30*time.Second)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
	}
	if cmdResult.Error != "" {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, cmdResult.Error)
	}

	respData, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"result": cmdResult.Output,
	})
	return "Tool Output: " + string(respData)
}
