package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"
)

func TestResolveDeviceSSHAccessUsesCredentialReference(t *testing.T) {
	t.Parallel()

	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("init inventory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := credentials.EnsureSchema(db); err != nil {
		t.Fatalf("ensure credentials schema: %v", err)
	}

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if err := vault.WriteSecret("cred-secret", "supersecret"); err != nil {
		t.Fatalf("write vault secret: %v", err)
	}

	credentialID, err := credentials.Create(db, credentials.Record{
		Name:            "Test SSH",
		Type:            "ssh",
		Host:            "10.0.0.5",
		Username:        "root",
		PasswordVaultID: "cred-secret",
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	deviceID, err := inventory.CreateDevice(db, "legacy-device", "server", "ssh", "192.168.1.10", 2222, "legacy", "legacy-secret", credentialID, "desc", nil, "")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	device, err := inventory.GetDeviceByID(db, deviceID)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}

	access, err := resolveDeviceSSHAccess(device, db, vault)
	if err != nil {
		t.Fatalf("resolve access: %v", err)
	}

	if access.Host != "10.0.0.5" {
		t.Fatalf("expected host from credential, got %q", access.Host)
	}
	if access.Username != "root" {
		t.Fatalf("expected username from credential, got %q", access.Username)
	}
	if access.Port != 2222 {
		t.Fatalf("expected port from device, got %d", access.Port)
	}
	if string(access.Secret) != "supersecret" {
		t.Fatalf("expected credential vault secret, got %q", string(access.Secret))
	}
}

func TestRemoteRevokeDeviceFailsWhenStatusPersistenceFails(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close remote db: %v", err)
	}

	hub := remote.NewRemoteHub(db, nil, nil)
	out := remoteRevokeDevice(hub, ToolCall{DeviceID: "device-1"}, nil)
	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("expected error output, got %s", out)
	}
	if !strings.Contains(out, "failed to persist revoked status") {
		t.Fatalf("expected persistence error, got %s", out)
	}
}

func TestRemoteDesktopScreenshotStoresImageDataByDefault(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	imageBytes := []byte("fake-png")
	outputPayload := map[string]interface{}{
		"source":      "display",
		"display_id":  "display-0",
		"format":      "png",
		"mime":        "image/png",
		"width":       640,
		"height":      480,
		"data_base64": base64.StdEncoding.EncodeToString(imageBytes),
	}
	outputJSON, _ := json.Marshal(outputPayload)
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    string(outputJSON),
	})

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := handleRemoteControl(ToolCall{
		Operation: "desktop_screenshot",
		DeviceID:  deviceID,
		Params: map[string]interface{}{
			"format": "png",
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected ok output, got %s", out)
	}
	if strings.Contains(out, "fake-png") || strings.Contains(out, "data_base64") {
		t.Fatalf("screenshot output should not expose base64 by default: %s", out)
	}

	var payload struct {
		Status         string `json:"status"`
		ScreenshotPath string `json:"screenshot_path"`
	}
	decodeToolOutputJSONForRemoteTest(t, out, &payload)
	if payload.ScreenshotPath == "" {
		t.Fatalf("missing screenshot_path in output: %s", out)
	}
	data, err := os.ReadFile(payload.ScreenshotPath)
	if err != nil {
		t.Fatalf("read stored screenshot: %v", err)
	}
	if string(data) != string(imageBytes) {
		t.Fatalf("stored screenshot bytes = %q, want %q", string(data), string(imageBytes))
	}
}

func TestRemoteDesktopScreenshotErrorsWhenImageDataMissing(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	outputJSON, _ := json.Marshal(map[string]interface{}{
		"source":     "display",
		"display_id": "display-0",
		"format":     "png",
		"mime":       "image/png",
		"width":      640,
		"height":     480,
	})
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    string(outputJSON),
	})

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := handleRemoteControl(ToolCall{
		Operation: "desktop_screenshot",
		DeviceID:  deviceID,
		Params: map[string]interface{}{
			"format": "png",
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "missing desktop screenshot data_base64") {
		t.Fatalf("expected missing data_base64 error, got %s", out)
	}
}

func TestRemoteDesktopInputMapsInputActionToClientAction(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    `{"accepted":true}`,
	}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true

	out := handleRemoteControl(ToolCall{
		Operation: "desktop_input",
		DeviceID:  deviceID,
		Params: map[string]interface{}{
			"kind":         "mouse_click",
			"x":            100,
			"y":            200,
			"button":       "left",
			"input_action": "click",
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected ok output, got %s", out)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(transport.calls))
	}
	if got := transport.calls[0].Args["action"]; got != "click" {
		t.Fatalf("desktop_input action = %#v, want click", got)
	}
	if _, ok := transport.calls[0].Args["input_action"]; ok {
		t.Fatalf("desktop_input should not forward input_action alias: %#v", transport.calls[0].Args)
	}
}

func TestSendAgoDeskChatRoutesThroughRemoteHubTransport(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
	}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	out, handled := dispatchMessagingCases(context.Background(), ToolCall{
		Action:   "send_agodesk_chat",
		DeviceID: deviceID,
		Params: map[string]interface{}{
			"message": "mission update",
		},
	}, &DispatchContext{
		Cfg:       &config.Config{},
		Logger:    slog.Default(),
		RemoteHub: hub,
	})
	if !handled {
		t.Fatal("send_agodesk_chat should be handled by messaging dispatch")
	}
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("expected success output, got %s", out)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(transport.calls))
	}
	if got := transport.calls[0].Operation; got != "agodesk_chat_message" {
		t.Fatalf("operation = %q, want agodesk_chat_message", got)
	}
	if got := transport.calls[0].Args["message"]; got != "mission update" {
		t.Fatalf("message arg = %#v, want mission update", got)
	}
}

func TestSendAgoDeskChatForwardsConversationID(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{connected: map[string]bool{deviceID: true}}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	out, handled := dispatchMessagingCases(context.Background(), ToolCall{
		Action: "send_agodesk_chat",
		Params: map[string]interface{}{
			"device_id":       deviceID,
			"message":         "mission update",
			"conversation_id": "sess-push",
		},
	}, &DispatchContext{
		Cfg:       &config.Config{},
		Logger:    slog.Default(),
		RemoteHub: hub,
	})
	if !handled || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("send_agodesk_chat output = %s, handled=%v", out, handled)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(transport.calls))
	}
	if got := transport.calls[0].Args["conversation_id"]; got != "sess-push" {
		t.Fatalf("conversation_id arg = %#v, want sess-push", got)
	}
}

func TestRemoteDesktopInputBlockedByGlobalReadOnly(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true
	cfg.RemoteControl.ReadOnly = true
	hub := remote.NewRemoteHub(nil, nil, slog.Default())

	out := handleRemoteControl(ToolCall{
		Operation: "desktop_input",
		DeviceID:  "agodesk-1",
		Params: map[string]interface{}{
			"kind": "mouse_click",
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "read-only") {
		t.Fatalf("expected read-only error, got %s", out)
	}
}

func TestRemoteControlComputerUseReadOnlyPolicy(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    `{"window_id":"win-1","element_count":1}`,
	}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true
	cfg.RemoteControl.ReadOnly = true

	for _, blocked := range []string{"desktop_ui_action", "desktop_browser_action"} {
		out := handleRemoteControl(ToolCall{
			Operation: blocked,
			DeviceID:  deviceID,
			Params: map[string]interface{}{
				"action":     "click",
				"element_id": "elem-1",
				"selector":   "#submit",
			},
		}, cfg, hub, slog.Default())
		if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "read-only") {
			t.Fatalf("%s expected read-only error, got %s", blocked, out)
		}
	}
	if len(transport.calls) != 0 {
		t.Fatalf("read-only action operations should not dispatch, calls = %#v", transport.calls)
	}

	out := handleRemoteControl(ToolCall{
		Operation: "desktop_ui_tree",
		DeviceID:  deviceID,
		Params: map[string]interface{}{
			"window_id": "win-1",
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("desktop_ui_tree should be allowed in read-only mode, got %s", out)
	}
	if len(transport.calls) != 1 || transport.calls[0].Operation != remote.OpDesktopUITree {
		t.Fatalf("read-only ui tree call = %#v", transport.calls)
	}
}

func TestRemoteControlComputerUseActionOperationsRequireAction(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true
	hub := remote.NewRemoteHub(nil, nil, slog.Default())

	for _, op := range []string{"desktop_ui_action", "desktop_browser_action"} {
		out := handleRemoteControl(ToolCall{
			Operation: op,
			DeviceID:  "agodesk-1",
			Params: map[string]interface{}{
				"element_id": "elem-1",
				"selector":   "#submit",
			},
		}, cfg, hub, slog.Default())
		if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "'action' is required") {
			t.Fatalf("%s expected missing action error, got %s", op, out)
		}
	}
}

func TestRemoteControlComputerUseOperationsForwardParams(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    `{"accepted":true}`,
	}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true

	for _, tc := range []struct {
		toolOperation string
		remoteOp      string
		params        map[string]interface{}
	}{
		{toolOperation: "desktop_list_displays", remoteOp: remote.OpDesktopListDisplays},
		{toolOperation: "desktop_list_windows", remoteOp: remote.OpDesktopListWindows},
		{toolOperation: "desktop_active_window", remoteOp: remote.OpDesktopActiveWindow},
		{toolOperation: "desktop_host_info", remoteOp: remote.OpDesktopHostInfo},
		{toolOperation: "desktop_ui_tree", remoteOp: remote.OpDesktopUITree, params: map[string]interface{}{"window_id": "win-1"}},
		{toolOperation: "desktop_ui_action", remoteOp: remote.OpDesktopUIAction, params: map[string]interface{}{"element_id": "elem-1", "action": "focus"}},
		{toolOperation: "desktop_browser_connect", remoteOp: remote.OpDesktopBrowserConnect, params: map[string]interface{}{"endpoint": "http://127.0.0.1:9222"}},
		{toolOperation: "desktop_browser_snapshot", remoteOp: remote.OpDesktopBrowserSnapshot, params: map[string]interface{}{"selector": "#app", "include_html": true}},
		{toolOperation: "desktop_browser_action", remoteOp: remote.OpDesktopBrowserAction, params: map[string]interface{}{"selector": "#name", "action": "fill", "value": "Ada"}},
		{toolOperation: "desktop_browser_disconnect", remoteOp: remote.OpDesktopBrowserDisconnect},
	} {
		out := handleRemoteControl(ToolCall{
			Operation: tc.toolOperation,
			DeviceID:  deviceID,
			Params:    tc.params,
		}, cfg, hub, slog.Default())
		if !strings.Contains(out, `"status":"ok"`) {
			t.Fatalf("%s expected ok output, got %s", tc.toolOperation, out)
		}
		if got := transport.calls[len(transport.calls)-1].Operation; got != tc.remoteOp {
			t.Fatalf("%s dispatched operation = %q, want %q", tc.toolOperation, got, tc.remoteOp)
		}
	}
	last := transport.calls[len(transport.calls)-2]
	if got := last.Args["value"]; got != "Ada" {
		t.Fatalf("desktop_browser_action value arg = %#v, want Ada", got)
	}
}

func TestRemoteFileOperationsForwardAgoDeskRootID(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    "package main\n",
	}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true

	out := handleRemoteControl(ToolCall{
		Operation: "read_file",
		DeviceID:  deviceID,
		Path:      "src/main.go",
		Params: map[string]interface{}{
			"root_id": "workspace",
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected ok output, got %s", out)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(transport.calls))
	}
	if got := transport.calls[0].Args["root_id"]; got != "workspace" {
		t.Fatalf("root_id arg = %#v, want workspace", got)
	}
}

func TestRemoteFileSearchUsesNestedAgoDeskSearchOperation(t *testing.T) {
	t.Parallel()

	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("init remote db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{
		Name:   "agodesk",
		Status: "approved",
		Tags:   []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	transport := &agentRecordingTransport{
		connected: map[string]bool{deviceID: true},
		output:    `{"matches":[]}`,
	}
	hub := remote.NewRemoteHub(db, nil, slog.Default())
	hub.RegisterCommandTransport("agodesk", transport)

	cfg := &config.Config{}
	cfg.RemoteControl.Enabled = true

	out := handleRemoteControl(ToolCall{
		Action:    "remote_control",
		Operation: "file_search",
		DeviceID:  deviceID,
		Params: map[string]interface{}{
			"operation": "file_search",
			"params": map[string]interface{}{
				"root_id":   "baf-g-872e1d24",
				"path":      ".",
				"operation": "grep_recursive",
				"pattern":   "Johannes",
			},
		},
	}, cfg, hub, slog.Default())
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected ok output, got %s", out)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(transport.calls))
	}
	call := transport.calls[0]
	if got := call.Operation; got != remote.OpFileSearch {
		t.Fatalf("operation = %q, want %q", got, remote.OpFileSearch)
	}
	if got := call.Args["operation"]; got != "grep_recursive" {
		t.Fatalf("file_search operation arg = %#v, want grep_recursive", got)
	}
	if got := call.Args["root_id"]; got != "baf-g-872e1d24" {
		t.Fatalf("root_id arg = %#v, want baf-g-872e1d24", got)
	}
	if got := call.Args["path"]; got != "." {
		t.Fatalf("path arg = %#v, want .", got)
	}
	if got := call.Args["pattern"]; got != "Johannes" {
		t.Fatalf("pattern arg = %#v, want Johannes", got)
	}
}

type agentRecordingTransport struct {
	connected map[string]bool
	output    string
	calls     []remote.CommandPayload
}

func (t *agentRecordingTransport) IsConnected(deviceID string) bool {
	return t.connected[deviceID]
}

func (t *agentRecordingTransport) SendCommand(deviceID string, cmd remote.CommandPayload, timeout time.Duration) (remote.ResultPayload, error) {
	t.calls = append(t.calls, cmd)
	return remote.ResultPayload{CommandID: cmd.CommandID, Status: "ok", Output: t.output}, nil
}

func decodeToolOutputJSONForRemoteTest(t *testing.T, out string, target interface{}) {
	t.Helper()
	raw := strings.TrimPrefix(out, "Tool Output: ")
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		t.Fatalf("decode tool output %q: %v", out, err)
	}
}
