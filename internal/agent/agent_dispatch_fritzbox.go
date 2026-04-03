package agent

// agent_dispatch_fritzbox.go – handles Fritz!Box tool calls dispatched from dispatchInfra.
// All 6 feature groups (system, network, telephony, smarthome, storage, tv) are routed here.
// External data (call names, phonebook entries) is wrapped in <external_data> tags before
// being included in the output returned to the LLM.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/fritzbox"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// handleFritzBoxToolCall routes the tool call to the appropriate Fritz!Box service method.
// tc.Action determines the feature group; tc.Operation determines the specific action.
func handleFritzBoxToolCall(tc ToolCall, c *fritzbox.Client, cfg *config.Config, logger *slog.Logger, budgetTracker *budget.Tracker) string {
	req := decodeFritzBoxArgs(tc)
	op := strings.ToLower(strings.TrimSpace(req.Operation))

	// Route by action name (e.g. "fritzbox_system") or via op prefix.
	switch req.Action {
	case "fritzbox_system", "fritzbox":
		if req.Action == "fritzbox" && !strings.HasPrefix(op, "get_info") &&
			!strings.HasPrefix(op, "get_log") && op != "reboot" {
			// Fall through to network, telephony etc. below based on op name.
			goto routeByOp
		}
		return fbSystemOp(c, op, logger)

	case "fritzbox_network":
		return fbNetworkOp(c, req, op, logger)
	case "fritzbox_telephony":
		return fbTelephonyOp(c, req, op, cfg, logger, budgetTracker)
	case "fritzbox_smarthome":
		return fbSmartHomeOp(c, req, op, logger)
	case "fritzbox_storage":
		return fbStorageOp(c, req, op, logger)
	case "fritzbox_tv":
		return fbTVOp(c, logger)
	}

routeByOp:
	// When action is "fritzbox" (fallback / old-style), route by operation name.
	switch {
	case op == "get_info" || op == "system_info":
		return fbSystemOp(c, "get_info", logger)
	case op == "get_log" || op == "system_log":
		return fbSystemOp(c, "get_log", logger)
	case op == "reboot":
		return fbSystemOp(c, "reboot", logger)
	case strings.HasPrefix(op, "get_wlan") || strings.HasPrefix(op, "set_wlan") ||
		op == "get_hosts" || op == "wake_on_lan" ||
		strings.Contains(op, "port_forward"):
		return fbNetworkOp(c, req, op, logger)
	case strings.Contains(op, "call") || strings.Contains(op, "phonebook") ||
		strings.Contains(op, "tam"):
		return fbTelephonyOp(c, req, op, cfg, logger, budgetTracker)
	case strings.HasPrefix(op, "get_device") || strings.HasPrefix(op, "set_switch") ||
		strings.HasPrefix(op, "set_heat") || strings.HasPrefix(op, "set_bright") ||
		strings.Contains(op, "template"):
		return fbSmartHomeOp(c, req, op, logger)
	case strings.Contains(op, "storage") || strings.Contains(op, "ftp") ||
		strings.Contains(op, "media_server"):
		return fbStorageOp(c, req, op, logger)
	case strings.Contains(op, "channel"):
		return fbTVOp(c, logger)
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown fritzbox operation %q"}`, op)
	}
}

// ────────────────────────────────────────────────────────────────
// System group
// ────────────────────────────────────────────────────────────────

func fbSystemOp(c *fritzbox.Client, op string, logger *slog.Logger) string {
	if !c.SystemEnabled() {
		return `Tool Output: {"status":"error","message":"Fritz!Box system integration is not enabled."}`
	}
	switch op {
	case "get_info":
		logger.Info("LLM requested Fritz!Box system info")
		info, err := c.GetSystemInfo()
		if err != nil {
			return fbError("get_info", err)
		}
		return fbOK(map[string]interface{}{
			"model":          info.ModelName,
			"firmware":       info.SoftwareVersion,
			"hardware":       info.HardwareVersion,
			"uptime_seconds": info.Uptime,
			"serial":         info.Serial,
		})
	case "get_log":
		logger.Info("LLM requested Fritz!Box system log")
		lines, err := c.GetSystemLog()
		if err != nil {
			return fbError("get_log", err)
		}
		// Wrap log lines as external data to guard against prompt injection.
		return fbOK(map[string]interface{}{
			"log": security.IsolateExternalData(strings.Join(lines, "\n")),
		})
	case "reboot":
		logger.Info("LLM requested Fritz!Box reboot")
		if err := c.Reboot(); err != nil {
			return fbError("reboot", err)
		}
		return fbOK(map[string]interface{}{"rebooting": true})
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown fritzbox_system operation %q. Use: get_info, get_log, reboot"}`, op)
	}
}

// ────────────────────────────────────────────────────────────────
// Network group
// ────────────────────────────────────────────────────────────────

func fbNetworkOp(c *fritzbox.Client, req fritzBoxArgs, op string, logger *slog.Logger) string {
	if !c.NetworkEnabled() {
		return `Tool Output: {"status":"error","message":"Fritz!Box network integration is not enabled."}`
	}
	switch op {
	case "get_wlan":
		idx := req.WLANIndex
		if idx == 0 {
			idx = 1
		}
		logger.Info("LLM requested Fritz!Box WLAN info", "index", idx)
		info, err := c.GetWLANInfo(idx)
		if err != nil {
			return fbError("get_wlan", err)
		}
		return fbOK(info)

	case "set_wlan":
		if c.NetworkReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box network is in read-only mode."}`
		}
		idx := req.WLANIndex
		if idx == 0 {
			idx = 1
		}
		logger.Info("LLM requested Fritz!Box WLAN toggle", "index", idx, "enabled", req.Enabled)
		if err := c.SetWLANEnabled(idx, req.Enabled); err != nil {
			return fbError("set_wlan", err)
		}
		return fbOK(map[string]interface{}{"wlan_index": idx, "enabled": req.Enabled})

	case "get_hosts":
		logger.Info("LLM requested Fritz!Box host list")
		hosts, err := c.GetHostList()
		if err != nil {
			return fbError("get_hosts", err)
		}
		return fbOK(map[string]interface{}{"count": len(hosts), "hosts": hosts})

	case "wake_on_lan":
		if c.NetworkReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box network is in read-only mode."}`
		}
		mac := req.MACAddress
		logger.Info("LLM requested Fritz!Box Wake-on-LAN", "mac", mac)
		if mac == "" {
			return `Tool Output: {"status":"error","message":"mac_address is required for wake_on_lan"}`
		}
		if err := c.WakeOnLAN(mac); err != nil {
			return fbError("wake_on_lan", err)
		}
		return fbOK(map[string]interface{}{"sent": true, "mac": mac})

	case "get_port_forwards":
		logger.Info("LLM requested Fritz!Box port forwarding list")
		list, err := c.GetPortForwardingList()
		if err != nil {
			return fbError("get_port_forwards", err)
		}
		return fbOK(map[string]interface{}{"count": len(list), "port_forwards": list})

	case "add_port_forward":
		if c.NetworkReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box network is in read-only mode."}`
		}
		logger.Info("LLM requested Fritz!Box add port forward", "ext", req.ExternalPort, "proto", req.Protocol)
		if req.ExternalPort == "" || req.InternalPort == "" || req.InternalClient == "" || req.Protocol == "" {
			return `Tool Output: {"status":"error","message":"external_port, internal_port, internal_client, and protocol are required for add_port_forward"}`
		}
		entry := fritzbox.PortForwardEntry{
			RemoteHost:     "",
			ExternalPort:   req.ExternalPort,
			Protocol:       strings.ToUpper(req.Protocol),
			InternalPort:   req.InternalPort,
			InternalClient: req.InternalClient,
			Enabled:        true,
			Description:    req.Description,
		}
		if err := c.AddPortForwarding(entry); err != nil {
			return fbError("add_port_forward", err)
		}
		return fbOK(map[string]interface{}{"added": true})

	case "delete_port_forward":
		if c.NetworkReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box network is in read-only mode."}`
		}
		logger.Info("LLM requested Fritz!Box delete port forward", "ext", req.ExternalPort, "proto", req.Protocol)
		if req.ExternalPort == "" || req.Protocol == "" {
			return `Tool Output: {"status":"error","message":"external_port and protocol are required for delete_port_forward"}`
		}
		if err := c.DeletePortForwarding("", req.ExternalPort, strings.ToUpper(req.Protocol)); err != nil {
			return fbError("delete_port_forward", err)
		}
		return fbOK(map[string]interface{}{"deleted": true, "port": req.ExternalPort, "protocol": req.Protocol})

	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown fritzbox_network operation %q. Use: get_wlan, set_wlan, get_hosts, wake_on_lan, get_port_forwards, add_port_forward, delete_port_forward"}`, op)
	}
}

// ────────────────────────────────────────────────────────────────
// Telephony group
// ────────────────────────────────────────────────────────────────

func fbTelephonyOp(c *fritzbox.Client, req fritzBoxArgs, op string, cfg *config.Config, logger *slog.Logger, budgetTracker *budget.Tracker) string {
	if !c.TelephonyEnabled() {
		return `Tool Output: {"status":"error","message":"Fritz!Box telephony integration is not enabled."}`
	}
	switch op {
	case "get_call_list":
		logger.Info("LLM requested Fritz!Box call list")
		calls, err := c.GetCallList()
		if err != nil {
			return fbError("get_call_list", err)
		}
		// Wrap entire call list as external data to prevent prompt injection from call names/numbers.
		raw, _ := json.Marshal(calls)
		return fbOK(map[string]interface{}{
			"count": len(calls),
			"calls": security.IsolateExternalData(string(raw)),
		})

	case "get_phonebooks":
		logger.Info("LLM requested Fritz!Box phonebook list")
		ids, err := c.GetPhonebookList()
		if err != nil {
			return fbError("get_phonebooks", err)
		}
		return fbOK(map[string]interface{}{"phonebook_ids": ids})

	case "get_phonebook_entries":
		logger.Info("LLM requested Fritz!Box phonebook entries", "id", req.PhonebookID)
		entries, err := c.GetPhonebookEntries(req.PhonebookID)
		if err != nil {
			return fbError("get_phonebook_entries", err)
		}
		raw, _ := json.Marshal(entries)
		return fbOK(map[string]interface{}{
			"count":   len(entries),
			"entries": security.IsolateExternalData(string(raw)),
		})

	case "get_tam_messages":
		logger.Info("LLM requested Fritz!Box TAM messages", "tam", req.TamIndex)
		msgs, err := c.GetTAMList(req.TamIndex)
		if err != nil {
			return fbError("get_tam_messages", err)
		}
		raw, _ := json.Marshal(msgs)
		return fbOK(map[string]interface{}{
			"count":    len(msgs),
			"messages": security.IsolateExternalData(string(raw)),
		})

	case "mark_tam_message_read":
		if c.TelephonyReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box telephony is in read-only mode."}`
		}
		logger.Info("LLM requested mark TAM message read", "tam", req.TamIndex, "msg", req.MsgIndex)
		if err := c.MarkTAMMessageRead(req.TamIndex, req.MsgIndex); err != nil {
			return fbError("mark_tam_message_read", err)
		}
		return fbOK(map[string]interface{}{"marked_read": true})

	case "get_tam_message_url":
		logger.Info("LLM requested Fritz!Box TAM message URL (diagnostic)", "tam", req.TamIndex, "msg", req.MsgIndex)
		audioURL, err := c.GetTAMMessageURL(req.TamIndex, req.MsgIndex)
		if err != nil {
			return fbError("get_tam_message_url", err)
		}
		return fbOK(map[string]interface{}{
			"url":       audioURL,
			"tam_index": req.TamIndex,
			"msg_index": req.MsgIndex,
			"note":      "This is the URL that download_tam_message/transcribe_tam_message will request (with SID appended as ?sid=...)",
		})

	case "download_tam_message":
		logger.Info("LLM requested Fritz!Box TAM audio download", "tam", req.TamIndex, "msg", req.MsgIndex)
		destDir := filepath.Join(cfg.Directories.WorkspaceDir, "workdir", "tam")
		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return fbError("download_tam_message", fmt.Errorf("create tam dir: %w", err))
		}
		destPath := filepath.Join(destDir, fmt.Sprintf("tam%d_msg%d.wav", req.TamIndex, req.MsgIndex))
		if err := c.DownloadTAMMessage(req.TamIndex, req.MsgIndex, destPath); err != nil {
			return fbError("download_tam_message", err)
		}
		return fbOK(map[string]interface{}{
			"file_path": destPath,
			"message":   fmt.Sprintf("TAM message %d from TAM %d saved to %s", req.MsgIndex, req.TamIndex, destPath),
		})

	case "transcribe_tam_message":
		logger.Info("LLM requested Fritz!Box TAM transcription", "tam", req.TamIndex, "msg", req.MsgIndex)
		tmpDir := filepath.Join(cfg.Directories.WorkspaceDir, "workdir", "tam")
		if err := os.MkdirAll(tmpDir, 0o750); err != nil {
			return fbError("transcribe_tam_message", fmt.Errorf("create tam dir: %w", err))
		}
		tmpPath := filepath.Join(tmpDir, fmt.Sprintf("tam%d_msg%d_tmp.wav", req.TamIndex, req.MsgIndex))
		if err := c.DownloadTAMMessage(req.TamIndex, req.MsgIndex, tmpPath); err != nil {
			return fbError("transcribe_tam_message", err)
		}
		defer os.Remove(tmpPath)

		text, fbSttCost, err := tools.TranscribeAudioFile(tmpPath, cfg)
		if err != nil {
			return fbError("transcribe_tam_message", fmt.Errorf("transcription failed: %w", err))
		}
		if budgetTracker != nil {
			budgetTracker.RecordCostForCategory("stt", fbSttCost)
		}
		return fbOK(map[string]interface{}{
			"transcription": security.IsolateExternalData(text),
			"tam_index":     req.TamIndex,
			"msg_index":     req.MsgIndex,
		})

	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown fritzbox_telephony operation %q. Use: get_call_list, get_phonebooks, get_phonebook_entries, get_tam_messages, mark_tam_message_read, download_tam_message, transcribe_tam_message, get_tam_message_url"}`, op)
	}
}

// ────────────────────────────────────────────────────────────────
// Smart Home group
// ────────────────────────────────────────────────────────────────

func fbSmartHomeOp(c *fritzbox.Client, req fritzBoxArgs, op string, logger *slog.Logger) string {
	if !c.SmartHomeEnabled() {
		return `Tool Output: {"status":"error","message":"Fritz!Box smart home integration is not enabled."}`
	}
	switch op {
	case "get_devices":
		logger.Info("LLM requested Fritz!Box smart home device list")
		devs, err := c.GetSmartHomeDevices()
		if err != nil {
			return fbError("get_devices", err)
		}
		return fbOK(map[string]interface{}{"count": len(devs), "devices": devs})

	case "set_switch":
		if c.SmartHomeReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box smart home is in read-only mode."}`
		}
		if req.AIN == "" {
			return `Tool Output: {"status":"error","message":"ain is required for set_switch"}`
		}
		logger.Info("LLM requested Fritz!Box switch toggle", "ain", req.AIN, "on", req.Enabled)
		if err := c.SetSwitch(req.AIN, req.Enabled); err != nil {
			return fbError("set_switch", err)
		}
		return fbOK(map[string]interface{}{"ain": req.AIN, "on": req.Enabled})

	case "set_heating":
		if c.SmartHomeReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box smart home is in read-only mode."}`
		}
		if req.AIN == "" {
			return `Tool Output: {"status":"error","message":"ain is required for set_heating"}`
		}
		logger.Info("LLM requested Fritz!Box heating control", "ain", req.AIN, "temp", req.TempC)
		if err := c.SetHeatingTarget(req.AIN, req.TempC); err != nil {
			return fbError("set_heating", err)
		}
		return fbOK(map[string]interface{}{"ain": req.AIN, "target_temp_c": req.TempC})

	case "set_brightness":
		if c.SmartHomeReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box smart home is in read-only mode."}`
		}
		if req.AIN == "" {
			return `Tool Output: {"status":"error","message":"ain is required for set_brightness"}`
		}
		logger.Info("LLM requested Fritz!Box lamp brightness", "ain", req.AIN, "pct", req.Brightness)
		if err := c.SetLampBrightness(req.AIN, req.Brightness); err != nil {
			return fbError("set_brightness", err)
		}
		return fbOK(map[string]interface{}{"ain": req.AIN, "brightness_pct": req.Brightness})

	case "get_templates":
		logger.Info("LLM requested Fritz!Box smart home templates")
		names, err := c.GetSmartHomeTemplates()
		if err != nil {
			return fbError("get_templates", err)
		}
		return fbOK(map[string]interface{}{"count": len(names), "templates": names})

	case "apply_template":
		if c.SmartHomeReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box smart home is in read-only mode."}`
		}
		if req.AIN == "" {
			return `Tool Output: {"status":"error","message":"ain (template ID) is required for apply_template"}`
		}
		logger.Info("LLM requested Fritz!Box apply template", "ain", req.AIN)
		if err := c.ApplySmartHomeTemplate(req.AIN); err != nil {
			return fbError("apply_template", err)
		}
		return fbOK(map[string]interface{}{"applied": true, "template_id": req.AIN})

	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown fritzbox_smarthome operation %q. Use: get_devices, set_switch, set_heating, set_brightness, get_templates, apply_template"}`, op)
	}
}

// ────────────────────────────────────────────────────────────────
// Storage group
// ────────────────────────────────────────────────────────────────

func fbStorageOp(c *fritzbox.Client, req fritzBoxArgs, op string, logger *slog.Logger) string {
	if !c.StorageEnabled() {
		return `Tool Output: {"status":"error","message":"Fritz!Box storage integration is not enabled."}`
	}
	switch op {
	case "get_storage_info":
		logger.Info("LLM requested Fritz!Box storage info")
		info, err := c.GetStorageInfo()
		if err != nil {
			return fbError("get_storage_info", err)
		}
		return fbOK(info)

	case "get_ftp_status":
		logger.Info("LLM requested Fritz!Box FTP status")
		enabled, err := c.GetFTPStatus()
		if err != nil {
			return fbError("get_ftp_status", err)
		}
		return fbOK(map[string]interface{}{"ftp_enabled": enabled})

	case "set_ftp":
		if c.StorageReadOnly() {
			return `Tool Output: {"status":"error","message":"Fritz!Box storage is in read-only mode."}`
		}
		logger.Info("LLM requested Fritz!Box FTP toggle", "enabled", req.Enabled)
		if err := c.SetFTPEnabled(req.Enabled); err != nil {
			return fbError("set_ftp", err)
		}
		return fbOK(map[string]interface{}{"ftp_enabled": req.Enabled})

	case "get_media_server_status":
		logger.Info("LLM requested Fritz!Box media server status")
		enabled, err := c.GetMediaServerStatus()
		if err != nil {
			return fbError("get_media_server_status", err)
		}
		return fbOK(map[string]interface{}{"dlna_enabled": enabled})

	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown fritzbox_storage operation %q. Use: get_storage_info, get_ftp_status, set_ftp, get_media_server_status"}`, op)
	}
}

// ────────────────────────────────────────────────────────────────
// TV group
// ────────────────────────────────────────────────────────────────

func fbTVOp(c *fritzbox.Client, logger *slog.Logger) string {
	if !c.TVEnabled() {
		return `Tool Output: {"status":"error","message":"Fritz!Box TV integration is not enabled (cable models only)."}`
	}
	logger.Info("LLM requested Fritz!Box TV channel list")
	channels, err := c.GetTVChannelList()
	if err != nil {
		return fbError("get_channels", err)
	}
	return fbOK(map[string]interface{}{"count": len(channels), "channels": channels})
}

// ────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────

func fbOK(data interface{}) string {
	out := map[string]interface{}{"status": "ok", "data": data}
	b, _ := json.Marshal(out)
	return "Tool Output: " + string(b)
}

func fbError(op string, err error) string {
	return fmt.Sprintf(`Tool Output: {"status":"error","operation":%q,"message":%q}`, op, err.Error())
}
