package agent

import (
	"encoding/json"
	"html"
	"strings"
)

func virtualDesktopAppLifecycleCall(tc ToolCall) (operation, appID string, ok bool) {
	action := strings.ToLower(strings.TrimSpace(tc.Action))
	operation = strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(
		tc.Operation,
		tc.ActionType,
		toolArgString(tc.Params, "operation", "op"),
	)))
	if action == "virtual_desktop_app_install" {
		operation = "install_app"
	}
	if action != "virtual_desktop_app_install" && action != "virtual_desktop" && action != "virtual_desktop_apps" {
		return "", "", false
	}
	if operation != "install_app" && operation != "diagnose_app" && operation != "open_app" && operation != "open_in_app" {
		return "", "", false
	}

	if operation == "install_app" {
		appID, ok = virtualDesktopInstallKey(tc)
		return operation, appID, ok && appID != "<unknown>"
	}

	appID = strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(
		toolArgString(tc.Params, "app_id", "id"),
		tc.ID,
	)))
	if appID == "" && operation == "open_in_app" {
		path := firstNonEmptyToolString(toolArgString(tc.Params, "path", "file_path"), tc.Path, tc.FilePath)
		parts := strings.Split(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"), "/")
		if len(parts) >= 2 && strings.EqualFold(parts[0], "Apps") {
			appID = strings.ToLower(strings.TrimSpace(parts[1]))
		}
	}
	return operation, appID, appID != ""
}

func precheckVirtualDesktopAppOpen(tc ToolCall, state *toolRecoveryState) (string, bool) {
	operation, appID, ok := virtualDesktopAppLifecycleCall(tc)
	if !ok || (operation != "open_app" && operation != "open_in_app") || state == nil {
		return "", false
	}
	state.ensureMutex()
	state.mu.RLock()
	installed := state.InstalledApps[appID]
	diagnosed := state.DiagnosedApps[appID]
	state.mu.RUnlock()
	if !installed || diagnosed {
		return "", false
	}
	return `{"status":"skipped","message":"App opening is deferred until diagnose_app succeeds for app_id ` + appID + `. Call virtual_desktop_apps with operation=diagnose_app and this app_id, then retry the open call."}`, true
}

func recordVirtualDesktopAppVerification(tc ToolCall, result string, failed bool, state *toolRecoveryState) {
	operation, appID, ok := virtualDesktopAppLifecycleCall(tc)
	if !ok || failed || state == nil {
		return
	}
	state.ensureMutex()
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.InstalledApps == nil {
		state.InstalledApps = make(map[string]bool)
	}
	if state.DiagnosedApps == nil {
		state.DiagnosedApps = make(map[string]bool)
	}
	switch operation {
	case "install_app":
		state.InstalledApps[appID] = true
		delete(state.DiagnosedApps, appID)
	case "diagnose_app":
		decoded := html.UnescapeString(result)
		start := strings.Index(decoded, "{")
		if start < 0 {
			return
		}
		var payload map[string]interface{}
		if json.NewDecoder(strings.NewReader(decoded[start:])).Decode(&payload) != nil {
			return
		}
		if !successfulToolResultPayload(payload) {
			return
		}
		data, _ := payload["data"].(map[string]interface{})
		healthy, _ := data["ok"].(bool)
		if healthy {
			state.DiagnosedApps[appID] = true
		}
	}
}
