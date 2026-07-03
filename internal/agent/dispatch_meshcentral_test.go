package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"

	"github.com/gorilla/websocket"
)

func TestDispatchMeshCentralWithoutVaultReturnsConfigError(t *testing.T) {
	CloseMeshCentralClient()
	defer CloseMeshCentralClient()

	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.URL = "http://127.0.0.1:1"
	cfg.MeshCentral.Username = "admin"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("dispatchServices panicked without vault: %v", r)
		}
	}()

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "meshcentral",
		Operation: "list_groups",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	if !strings.Contains(output, "No password or token found") {
		t.Fatalf("output = %s, want missing credential error", output)
	}
}

func TestGetMeshCentralClientRebuildsWhenSettingsChange(t *testing.T) {
	CloseMeshCentralClient()
	defer CloseMeshCentralClient()

	first := newMeshCentralGroupsServer(t, "old-group")
	defer first.Close()
	second := newMeshCentralGroupsServer(t, "new-group")
	defer second.Close()

	client := getMeshCentralClient(first.URL, "admin", "pass", "", true, testLogger)
	groups, err := client.ListDeviceGroups()
	if err != nil {
		t.Fatalf("first ListDeviceGroups: %v", err)
	}
	if !meshCentralGroupsContain(groups, "old-group") {
		t.Fatalf("first groups = %#v, want old-group", groups)
	}

	client = getMeshCentralClient(second.URL, "admin", "pass", "", true, testLogger)
	groups, err = client.ListDeviceGroups()
	if err != nil {
		t.Fatalf("second ListDeviceGroups: %v", err)
	}
	if !meshCentralGroupsContain(groups, "new-group") {
		t.Fatalf("second groups = %#v, want new-group after settings changed", groups)
	}
}

func TestDispatchMeshCentralDoesNotLogCommandBody(t *testing.T) {
	CloseMeshCentralClient()
	defer CloseMeshCentralClient()

	server := newMeshCentralGroupsServer(t, "commands")
	defer server.Close()

	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.URL = server.URL
	cfg.MeshCentral.Username = "admin"
	cfg.MeshCentral.Password = "pass"

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	privateCommand := "echo private-command-value"

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "meshcentral",
		Operation: "run_command",
		NodeID:    "node//device1",
		Command:   privateCommand,
	}, &DispatchContext{Cfg: cfg, Logger: logger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	if !strings.Contains(output, `"status":"success"`) && !strings.Contains(output, `"status": "success"`) {
		t.Fatalf("output = %s, want success", output)
	}
	if strings.Contains(logs.String(), privateCommand) {
		t.Fatalf("meshcentral command body leaked into logs: %s", logs.String())
	}
	if strings.Contains(logs.String(), "!BADKEY") {
		t.Fatalf("meshcentral client produced malformed slog output: %s", logs.String())
	}
}

func TestDispatchMeshCentralReadOnlyAllowsNewReadOperations(t *testing.T) {
	CloseMeshCentralClient()
	defer CloseMeshCentralClient()

	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.ReadOnly = true
	cfg.MeshCentral.URL = "http://127.0.0.1:1"
	cfg.MeshCentral.Username = "admin"

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "meshcentral",
		Operation: "list_events",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	if strings.Contains(output, "readonly is enabled") {
		t.Fatalf("list_events should be allowed in read-only mode: %s", output)
	}
	if !strings.Contains(output, "No password or token found") {
		t.Fatalf("output = %s, want missing credential after read-only gate", output)
	}
}

func TestDispatchMeshCentralShellUnsupportedDoesNotRequireCredentials(t *testing.T) {
	CloseMeshCentralClient()
	defer CloseMeshCentralClient()

	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.URL = "http://127.0.0.1:1"

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "meshcentral",
		Operation: "shell",
		NodeID:    "node//dev1",
		Command:   "hostname",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	if !json.Valid([]byte(strings.TrimPrefix(output, "Tool Output: "))) {
		t.Fatalf("output is not valid JSON: %s", output)
	}
	if !strings.Contains(output, "unsupported") {
		t.Fatalf("output = %s, want unsupported shell error", output)
	}
}

func TestDispatchMeshCentralUnknownOperationEscapesJSON(t *testing.T) {
	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.URL = "http://127.0.0.1:1"

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "meshcentral",
		Operation: `bad"op`,
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	payload := strings.TrimPrefix(output, "Tool Output: ")
	if !json.Valid([]byte(payload)) {
		t.Fatalf("output is not valid JSON: %s", output)
	}
}

func TestDispatchMeshCentralPowerActionMapsSemanticReset(t *testing.T) {
	CloseMeshCentralClient()
	defer CloseMeshCentralClient()

	server := newMeshCentralGroupsServer(t, "power")
	defer server.Close()

	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.URL = server.URL
	cfg.MeshCentral.Username = "admin"
	cfg.MeshCentral.Password = "pass"

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "meshcentral",
		Operation: "power_action",
		NodeID:    "node//dev1",
		Params: map[string]interface{}{
			"power_action": "reset",
		},
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	if !strings.Contains(output, `"status":"success"`) && !strings.Contains(output, `"status": "success"`) {
		t.Fatalf("output = %s, want success", output)
	}
	if got := <-lastMeshCentralPowerAction; got != 3 {
		t.Fatalf("power action type = %d, want 3 for reset", got)
	}
}

func TestDispatchMeshCentralLegacyHibernateIsUnsupported(t *testing.T) {
	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.URL = "http://127.0.0.1:1"

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:      "meshcentral",
		Operation:   "power_action",
		NodeID:      "node//dev1",
		PowerAction: 2,
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatalf("expected meshcentral operation to be handled")
	}
	if !strings.Contains(output, "unsupported") || !strings.Contains(strings.ToLower(output), "hibernate") {
		t.Fatalf("output = %s, want unsupported hibernate", output)
	}
}

var lastMeshCentralPowerAction = make(chan int, 1)

func newMeshCentralGroupsServer(t *testing.T, groupName string) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html random="nonce123"></html>`))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "meshcom", Value: "session123"})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/control.ashx", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer ws.Close()
		_ = ws.WriteJSON(map[string]interface{}{
			"action": "serverinfo",
			"serverinfo": map[string]interface{}{
				"serverVersion": "test",
				"domain":        "",
			},
		})
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			reqid, _ := data["reqid"].(float64)
			responseID, _ := data["responseid"].(string)
			switch data["action"] {
			case "serverinfo":
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":         int(reqid),
					"action":        "serverinfo",
					"serverVersion": "test",
				})
			case "meshes":
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":      int(reqid),
					"action":     "meshes",
					"responseid": responseID,
					"meshes": []interface{}{
						map[string]interface{}{"name": groupName},
					},
				})
			case "runcommands":
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":      int(reqid),
					"action":     "runcommands",
					"type":       "runcommands",
					"responseid": responseID,
					"result":     "ok",
				})
			case "poweraction":
				actionType, _ := data["actiontype"].(float64)
				select {
				case lastMeshCentralPowerAction <- int(actionType):
				default:
				}
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":      int(reqid),
					"action":     "poweraction",
					"responseid": responseID,
					"result":     "OK",
				})
			case "events":
				_ = ws.WriteJSON(map[string]interface{}{
					"action":     "events",
					"responseid": responseID,
					"events":     []interface{}{},
				})
			}
		}
	})

	return testutil.NewHTTPServer(t, mux)
}

func meshCentralGroupsContain(groups []interface{}, name string) bool {
	for _, group := range groups {
		values, ok := group.(map[string]interface{})
		if ok && values["name"] == name {
			return true
		}
	}
	return false
}
