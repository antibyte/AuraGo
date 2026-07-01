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
	if !strings.Contains(output, `"status": "success"`) {
		t.Fatalf("output = %s, want success", output)
	}
	if strings.Contains(logs.String(), privateCommand) {
		t.Fatalf("meshcentral command body leaked into logs: %s", logs.String())
	}
	if strings.Contains(logs.String(), "!BADKEY") {
		t.Fatalf("meshcentral client produced malformed slog output: %s", logs.String())
	}
}

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
			switch data["action"] {
			case "serverinfo":
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":         int(reqid),
					"action":        "serverinfo",
					"serverVersion": "test",
				})
			case "meshes":
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":  int(reqid),
					"action": "meshes",
					"meshes": []interface{}{
						map[string]interface{}{"name": groupName},
					},
				})
			case "runcommand":
				_ = ws.WriteJSON(map[string]interface{}{
					"reqid":  int(reqid),
					"action": "runcommand",
					"result": "ok",
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
