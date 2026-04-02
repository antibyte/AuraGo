package agent

import (
	"strings"
	"testing"
)

func TestDecodeManageOutgoingWebhooksArgsUsesRawParams(t *testing.T) {
	tc := ToolCall{
		Action: "manage_outgoing_webhooks",
		Params: map[string]interface{}{
			"operation":     "create",
			"id":            "hook_123",
			"name":          "Deploy Hook",
			"description":   "Triggers deployment",
			"method":        "POST",
			"url":           "https://example.com/hook",
			"payload_type":  "json",
			"body_template": `{"ok":true}`,
			"headers": map[string]interface{}{
				"Authorization": "Bearer secret",
			},
			"parameters": []interface{}{
				map[string]interface{}{
					"name":        "branch",
					"type":        "string",
					"description": "Git branch",
					"required":    true,
				},
			},
		},
	}

	req := decodeManageOutgoingWebhooksArgs(tc)
	if req.Operation != "create" {
		t.Fatalf("Operation = %q, want create", req.Operation)
	}
	if req.Name != "Deploy Hook" {
		t.Fatalf("Name = %q, want Deploy Hook", req.Name)
	}
	if got := req.Headers["Authorization"]; got != "Bearer secret" {
		t.Fatalf("Headers[Authorization] = %q, want Bearer secret", got)
	}
	if len(req.Parameters) != 1 {
		t.Fatalf("Parameters len = %d, want 1", len(req.Parameters))
	}
	if req.Parameters[0].Name != "branch" || !req.Parameters[0].Required {
		t.Fatalf("decoded parameter = %+v, want branch/required", req.Parameters[0])
	}
}

func TestDecodeCreateSkillFromTemplateArgsUsesArrayFields(t *testing.T) {
	tc := ToolCall{
		Action:      "create_skill_from_template",
		Template:    "api_client",
		Name:        "weather_api",
		Description: "Reads weather data",
		URL:         "https://api.example.com",
		Params: map[string]interface{}{
			"dependencies": []interface{}{"requests", "pydantic"},
			"vault_keys":   []interface{}{"WEATHER_API_KEY"},
		},
	}

	req := decodeCreateSkillFromTemplateArgs(tc)
	if req.Template != "api_client" {
		t.Fatalf("Template = %q, want api_client", req.Template)
	}
	if len(req.Dependencies) != 2 {
		t.Fatalf("Dependencies len = %d, want 2", len(req.Dependencies))
	}
	if len(req.VaultKeys) != 1 || req.VaultKeys[0] != "WEATHER_API_KEY" {
		t.Fatalf("VaultKeys = %v, want [WEATHER_API_KEY]", req.VaultKeys)
	}
}

func TestDecodeGoogleWorkspaceArgsMergesParamsAndTopLevelFields(t *testing.T) {
	tc := ToolCall{
		Action:       "google_workspace",
		Operation:    "gmail_modify_labels",
		MessageID:    "msg_1",
		AddLabels:    []string{"LabelA"},
		RemoveLabels: []string{"LabelB"},
		Params: map[string]interface{}{
			"query":       "from:bob@example.com",
			"max_results": float64(15),
		},
	}

	req := decodeGoogleWorkspaceArgs(tc)
	if req.Operation != "gmail_modify_labels" {
		t.Fatalf("Operation = %q, want gmail_modify_labels", req.Operation)
	}
	if req.MessageID != "msg_1" {
		t.Fatalf("MessageID = %q, want msg_1", req.MessageID)
	}
	if req.Query != "from:bob@example.com" {
		t.Fatalf("Query = %q, want from:bob@example.com", req.Query)
	}
	if req.MaxResults != 15 {
		t.Fatalf("MaxResults = %d, want 15", req.MaxResults)
	}
	if len(req.AddLabels) != 1 || req.AddLabels[0] != "LabelA" {
		t.Fatalf("AddLabels = %v, want [LabelA]", req.AddLabels)
	}
	if len(req.RemoveLabels) != 1 || req.RemoveLabels[0] != "LabelB" {
		t.Fatalf("RemoveLabels = %v, want [LabelB]", req.RemoveLabels)
	}
}

func TestDecodeGoogleWorkspaceArgsFromMapParsesValues(t *testing.T) {
	req := decodeGoogleWorkspaceArgsFromMap(map[string]interface{}{
		"operation":   "sheets_update",
		"document_id": "sheet_1",
		"range":       "A1:B2",
		"values": []interface{}{
			[]interface{}{"a", "b"},
			[]interface{}{"c", "d"},
		},
	})

	if req.Operation != "sheets_update" {
		t.Fatalf("Operation = %q, want sheets_update", req.Operation)
	}
	if req.DocumentID != "sheet_1" {
		t.Fatalf("DocumentID = %q, want sheet_1", req.DocumentID)
	}
	if len(req.Values) != 2 {
		t.Fatalf("Values len = %d, want 2", len(req.Values))
	}
	if got := req.Values[1][1]; got != "d" {
		t.Fatalf("Values[1][1] = %v, want d", got)
	}
}

func TestDecodeProcessAnalyzerArgsUsesParamsFallback(t *testing.T) {
	req := decodeProcessAnalyzerArgs(ToolCall{
		Action: "process_analyzer",
		Params: map[string]interface{}{
			"operation": "find",
			"name":      "python",
			"pid":       float64(42),
			"limit":     float64(7),
		},
	})

	if req.Operation != "find" || req.Name != "python" || req.PID != 42 || req.Limit != 7 {
		t.Fatalf("decoded process args = %+v", req)
	}
}

func TestDecodeWebCaptureArgsUsesParamsFallback(t *testing.T) {
	req := decodeWebCaptureArgs(ToolCall{
		Action: "web_capture",
		Params: map[string]interface{}{
			"operation":  "screenshot",
			"url":        "https://example.com",
			"selector":   "#hero",
			"full_page":  true,
			"output_dir": "captures",
		},
	})

	if req.Operation != "screenshot" || req.URL != "https://example.com" || req.Selector != "#hero" || !req.FullPage || req.OutputDir != "captures" {
		t.Fatalf("decoded web capture args = %+v", req)
	}
}

func TestDecodeNetworkPingArgsUsesTargetAliases(t *testing.T) {
	req := decodeNetworkPingArgs(ToolCall{
		Action: "network_ping",
		Params: map[string]interface{}{
			"target":  "router.local",
			"count":   float64(3),
			"timeout": float64(2),
		},
	})

	if req.Host != "router.local" || req.Count != 3 || req.Timeout != 2 {
		t.Fatalf("decoded ping args = %+v", req)
	}
}

func TestDecodeDetectFileTypeArgsUsesPathAliasAndRecursive(t *testing.T) {
	req := decodeDetectFileTypeArgs(ToolCall{
		Action: "detect_file_type",
		Params: map[string]interface{}{
			"path":      "downloads",
			"recursive": true,
		},
	})

	if req.FilePath != "downloads" || !req.Recursive {
		t.Fatalf("decoded file type args = %+v", req)
	}
}

func TestDecodeSiteCrawlerArgsJoinsAllowedDomains(t *testing.T) {
	req := decodeSiteCrawlerArgs(ToolCall{
		Action: "site_crawler",
		Params: map[string]interface{}{
			"url":             "https://example.com",
			"max_depth":       float64(2),
			"max_pages":       float64(12),
			"allowed_domains": []interface{}{"example.com", "cdn.example.com"},
			"selector":        "main",
		},
	})

	if req.URL != "https://example.com" || req.MaxDepth != 2 || req.MaxPages != 12 || req.AllowedDomains != "example.com,cdn.example.com" || req.Selector != "main" {
		t.Fatalf("decoded crawler args = %+v", req)
	}
}

func TestDecodeWhoisLookupArgsFallsBackToURL(t *testing.T) {
	req := decodeWhoisLookupArgs(ToolCall{
		Action: "whois_lookup",
		Params: map[string]interface{}{
			"url":         "example.org",
			"include_raw": true,
		},
	})

	if req.domain() != "example.org" || !req.IncludeRaw {
		t.Fatalf("decoded whois args = %+v", req)
	}
}

func TestDecodeSiteMonitorArgsUsesParamsFallback(t *testing.T) {
	req := decodeSiteMonitorArgs(ToolCall{
		Action: "site_monitor",
		Params: map[string]interface{}{
			"operation":  "add_monitor",
			"monitor_id": "mon_1",
			"url":        "https://example.com",
			"selector":   "#status",
			"interval":   "10m",
			"limit":      float64(5),
		},
	})

	if req.Operation != "add_monitor" || req.MonitorID != "mon_1" || req.URL != "https://example.com" || req.Selector != "#status" || req.Interval != "10m" || req.Limit != 5 {
		t.Fatalf("decoded site monitor args = %+v", req)
	}
}

func TestDecodeFormAutomationArgsEncodesFieldMaps(t *testing.T) {
	req := decodeFormAutomationArgs(ToolCall{
		Action: "form_automation",
		Params: map[string]interface{}{
			"operation":      "fill_submit",
			"url":            "https://example.com/login",
			"fields":         map[string]interface{}{"#user": "alice", "#pass": "secret"},
			"selector":       "button[type=submit]",
			"screenshot_dir": "screens",
		},
	})

	if req.Operation != "fill_submit" || req.URL != "https://example.com/login" || req.Selector != "button[type=submit]" || req.ScreenshotDir != "screens" {
		t.Fatalf("decoded form args = %+v", req)
	}
	if !strings.Contains(req.Fields, "\"#user\":\"alice\"") || !strings.Contains(req.Fields, "\"#pass\":\"secret\"") {
		t.Fatalf("fields = %q, want JSON-encoded selectors", req.Fields)
	}
}

func TestDecodeUPnPScanArgsUsesParamsFallback(t *testing.T) {
	req := decodeUPnPScanArgs(ToolCall{
		Action: "upnp_scan",
		Params: map[string]interface{}{
			"search_target":      "ssdp:all",
			"timeout_secs":       float64(5),
			"auto_register":      true,
			"register_type":      "device",
			"register_tags":      []interface{}{"lan", "upnp"},
			"overwrite_existing": true,
		},
	})

	if req.SearchTarget != "ssdp:all" || req.TimeoutSecs != 5 || !req.AutoRegister || req.RegisterType != "device" || !req.OverwriteExisting {
		t.Fatalf("decoded upnp args = %+v", req)
	}
	if len(req.RegisterTags) != 2 || req.RegisterTags[0] != "lan" || req.RegisterTags[1] != "upnp" {
		t.Fatalf("RegisterTags = %#v, want [lan upnp]", req.RegisterTags)
	}
}

func TestDecodeManageProcessesArgsUsesParamsFallback(t *testing.T) {
	req := decodeManageProcessesArgs(ToolCall{
		Action: "manage_processes",
		Params: map[string]interface{}{
			"operation": "stop",
			"pid":       float64(123),
		},
	})

	if req.Operation != "stop" || req.PID != 123 {
		t.Fatalf("decoded manage_processes args = %+v", req)
	}
}

func TestDecodeRegisterDeviceArgsUsesAliases(t *testing.T) {
	req := decodeRegisterDeviceArgs(ToolCall{
		Action: "register_device",
		Params: map[string]interface{}{
			"host":        "nas.local",
			"device_type": "nas",
			"ip":          "192.168.1.20",
			"port":        float64(22),
			"user":        "root",
			"pass":        "secret",
			"key_path":    "/keys/id_ed25519",
			"description": "Main NAS",
			"tags":        []interface{}{"storage", "critical"},
			"mac_address": "00:11:22:33:44:55",
		},
	})

	if req.Hostname != "nas.local" || req.DeviceType != "nas" || req.IPAddress != "192.168.1.20" || req.Port != 22 || req.Username != "root" || req.Password != "secret" || req.PrivateKeyPath != "/keys/id_ed25519" || req.Description != "Main NAS" || req.Tags != "storage,critical" || req.MACAddress != "00:11:22:33:44:55" {
		t.Fatalf("decoded register_device args = %+v", req)
	}
}

func TestDecodeWakeOnLANArgsUsesParamsFallback(t *testing.T) {
	req := decodeWakeOnLANArgs(ToolCall{
		Action: "wake_on_lan",
		Params: map[string]interface{}{
			"server_id":   "srv-1",
			"mac_address": "AA:BB:CC:DD:EE:FF",
			"ip":          "192.168.1.255",
		},
	})

	if req.ServerID != "srv-1" || req.MACAddress != "AA:BB:CC:DD:EE:FF" || req.IPAddress != "192.168.1.255" {
		t.Fatalf("decoded wake args = %+v", req)
	}
}

func TestDecodePinMessageArgsUsesParamsFallback(t *testing.T) {
	req := decodePinMessageArgs(ToolCall{
		Action: "pin_message",
		Params: map[string]interface{}{
			"id":     "42",
			"pinned": true,
		},
	})

	if req.ID != "42" || !req.Pinned {
		t.Fatalf("decoded pin args = %+v", req)
	}
}

func TestDecodeDiscordMessageArgsUsesContentFallback(t *testing.T) {
	req := decodeDiscordMessageArgs(ToolCall{
		Action: "send_discord",
		Params: map[string]interface{}{
			"channel_id": "chan-1",
			"content":    "Hello Discord",
			"limit":      float64(12),
		},
	})

	if req.ChannelID != "chan-1" || req.Message != "Hello Discord" || req.Limit != 12 {
		t.Fatalf("decoded discord args = %+v", req)
	}
}

func TestDecodeMissionArgsUsesAliasesAndLockedPresence(t *testing.T) {
	req := decodeMissionArgs(ToolCall{
		Action:  "manage_missions",
		RawJSON: `{"locked":true}`,
		Params: map[string]interface{}{
			"operation": "update",
			"id":        "mission-1",
			"name":      "Nightly sync",
			"prompt":    "sync everything",
			"cron_expr": "0 2 * * *",
			"priority":  float64(3),
			"locked":    true,
		},
	})

	if req.Operation != "update" || req.ID != "mission-1" || req.Title != "Nightly sync" || req.Command != "sync everything" || req.CronExpr != "0 2 * * *" || req.Priority != 3 || !req.Locked || !req.LockedProvided {
		t.Fatalf("decoded mission args = %+v", req)
	}
}
