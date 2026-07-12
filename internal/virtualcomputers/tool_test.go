package virtualcomputers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteToolReadonlyBlocksMutations(t *testing.T) {
	cfg := ToolConfig{
		Enabled:    true,
		ToolGate:   true,
		ReadOnly:   true,
		BoringdURL: "http://127.0.0.1:1",
	}

	blocked := ExecuteTool(context.Background(), cfg, map[string]interface{}{
		"operation": "launch",
		"template":  "python",
	})
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(blocked), &payload); err != nil {
		t.Fatalf("unmarshal blocked: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("status = %v", payload["status"])
	}
	if payload["code"] != "readonly" {
		t.Fatalf("code = %v", payload["code"])
	}
}

func TestExecuteToolReadonlyAllowsListAndScreenshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/machines":
			_, _ = w.Write([]byte(`[{"id":"vm-1","status":"running"}]`))
		case "/v1/machines/vm-1/screenshot":
			_, _ = w.Write([]byte(`{"mime_type":"image/png","data_base64":"aGVsbG8="}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := ToolConfig{
		Enabled:    true,
		ToolGate:   true,
		ReadOnly:   true,
		BoringdURL: server.URL,
	}

	for _, args := range []map[string]interface{}{
		{"operation": "list_machines"},
		{"operation": "screenshot", "machine_id": "vm-1"},
	} {
		out := ExecuteTool(context.Background(), cfg, args)
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}
		if payload["status"] != "ok" {
			t.Fatalf("operation %v status = %v output=%s", args["operation"], payload["status"], out)
		}
	}
}

func TestExecuteToolEnforcesPersistentInternetAndTaskGates(t *testing.T) {
	cfg := ToolConfig{
		Enabled:           true,
		ToolGate:          true,
		BoringdURL:        "http://127.0.0.1:1",
		AllowInternet:     false,
		AllowPersistent:   false,
		AllowAgentTasks:   false,
		DefaultTTLSeconds: 600,
		MaxTTLSeconds:     900,
	}

	cases := []struct {
		name string
		args map[string]interface{}
		code string
	}{
		{name: "internet", args: map[string]interface{}{"operation": "launch", "allow_internet": true}, code: "internet_disabled"},
		{name: "persistent", args: map[string]interface{}{"operation": "launch", "persistent": true}, code: "persistent_disabled"},
		{name: "desktop task", args: map[string]interface{}{"operation": "run_desktop_task", "machine_id": "vm-1", "instruction": "open browser"}, code: "agent_tasks_disabled"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := ExecuteTool(context.Background(), cfg, tc.args)
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(out), &payload); err != nil {
				t.Fatalf("unmarshal output: %v", err)
			}
			if payload["code"] != tc.code {
				t.Fatalf("code = %v, want %s output=%s", payload["code"], tc.code, out)
			}
		})
	}
}
