package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestDispatchInfraManageWebhooksUsesActionAlias(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir temp dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Webhooks.Enabled = true
	cfg.Directories.DataDir = tmpDir

	out, ok := dispatchInfra(context.Background(), ToolCall{
		Action: "manage_webhooks",
		Params: map[string]interface{}{
			"action": "create",
			"name":   "Inbox Hook",
			"slug":   "inbox-hook",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchInfra to handle manage_webhooks")
	}
	if !strings.Contains(out, `"status": "ok"`) && !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected success output, got %s", out)
	}

	webhookFile := filepath.Join(tmpDir, "webhooks.json")
	data, err := os.ReadFile(webhookFile)
	if err != nil {
		t.Fatalf("expected webhook file to be written: %v", err)
	}
	if !strings.Contains(string(data), "inbox-hook") {
		t.Fatalf("expected webhook file to contain slug, got %s", string(data))
	}
}

func TestDispatchInfraMQTTPublishUsesParamsFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true

	out, ok := dispatchInfra(context.Background(), ToolCall{
		Action: "mqtt_publish",
		Params: map[string]interface{}{
			"topic":   "home/test",
			"payload": "hello",
			"qos":     float64(1),
			"retain":  true,
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchInfra to handle mqtt_publish")
	}
	if strings.Contains(out, "'topic' is required") {
		t.Fatalf("expected topic fallback from params, got %s", out)
	}
	if !strings.Contains(out, "MQTT publish failed") {
		t.Fatalf("expected downstream MQTT bridge error, got %s", out)
	}
}

func TestDispatchInfraMQTTReadOnlyBlocksSubscriptions(t *testing.T) {
	for _, action := range []string{"mqtt_subscribe", "mqtt_unsubscribe"} {
		t.Run(action, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.MQTT.Enabled = true
			cfg.MQTT.ReadOnly = true

			out, ok := dispatchInfra(context.Background(), ToolCall{
				Action: action,
				Params: map[string]interface{}{
					"topic": "home/test",
				},
			}, &DispatchContext{
				Cfg:    cfg,
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			if !ok {
				t.Fatalf("expected dispatchInfra to handle %s", action)
			}
			if !strings.Contains(out, "MQTT is in read-only mode") {
				t.Fatalf("expected read-only denial, got %s", out)
			}
		})
	}
}

func TestDispatchInfraMQTTPerRunGatesIntersectLiveResolver(t *testing.T) {
	tools.SetMQTTPermissionResolver(func() (bool, bool) { return true, false })
	t.Cleanup(func() { tools.SetMQTTPermissionResolver(nil) })

	tests := []struct {
		name       string
		invoke     bool
		enabled    bool
		readOnly   bool
		wantSubstr string
	}{
		{name: "native disabled", enabled: false, wantSubstr: "MQTT is not enabled"},
		{name: "native readonly", enabled: true, readOnly: true, wantSubstr: "MQTT is in read-only mode"},
		{name: "invoke disabled", invoke: true, enabled: false, wantSubstr: "MQTT is not enabled"},
		{name: "invoke readonly", invoke: true, enabled: true, readOnly: true, wantSubstr: "MQTT is in read-only mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.MQTT.Enabled = tt.enabled
			cfg.MQTT.ReadOnly = tt.readOnly
			sessionID := "mqtt-per-run-gate-" + strings.ReplaceAll(tt.name, " ", "-")
			if tt.invoke {
				t.Cleanup(func() { ClearDiscoverToolsState(sessionID) })
				SetDiscoverToolsState(sessionID, []openai.Tool{testToolSchema("mqtt_publish", "Publish MQTT message")}, nil, "")
			}
			var out string
			var handled bool
			dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionID: sessionID}
			if tt.invoke {
				out, handled = dispatchComm(context.Background(), ToolCall{
					Action: "invoke_tool",
					Params: map[string]interface{}{
						"tool_name": "mqtt_publish",
						"arguments": map[string]interface{}{"topic": "home/test", "payload": "payload"},
					},
				}, dc)
			} else {
				out, handled = dispatchInfra(context.Background(), ToolCall{
					Action: "mqtt_publish",
					Params: map[string]interface{}{"topic": "home/test", "payload": "payload"},
				}, dc)
			}
			if !handled || !strings.Contains(out, tt.wantSubstr) {
				t.Fatalf("handled=%v output=%s, want %q", handled, out, tt.wantSubstr)
			}
		})
	}
}

func TestDispatchInfraMQTTUnsubscribeIncludesRemainingOwners(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{MQTTEnabled: true})
	tools.RegisterMQTTUnsubscribeDetail(func(topic string, _ *slog.Logger) (tools.MQTTUnsubscribeResult, error) {
		return tools.MQTTUnsubscribeResult{
			Topic:   topic,
			Removed: false,
			RemainingOwners: []tools.MQTTUnsubscribeOwner{{
				Kind: "config",
				Key:  topic,
				QoS:  1,
			}},
		}, nil
	})
	t.Cleanup(func() {
		tools.RegisterMQTTUnsubscribeDetail(nil)
		tools.RegisterMQTTBridge(nil, nil, nil, nil)
		tools.ClearRuntimePermissionsForTest()
	})

	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	out, ok := dispatchInfra(context.Background(), ToolCall{
		Action: "mqtt_unsubscribe",
		Params: map[string]interface{}{"topic": "home/shared"},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchInfra to handle mqtt_unsubscribe")
	}
	if !strings.Contains(out, `"remaining_owners"`) || !strings.Contains(out, `"kind":"config"`) {
		t.Fatalf("unsubscribe output omitted remaining owners: %s", out)
	}
}

func TestMQTTToolOutputMarshalsQuotesAndBackslashes(t *testing.T) {
	raw := mqttToolOutput("error", map[string]interface{}{
		"message": `MQTT publish failed: broker said "bad"\\path`,
		"topic":   `home/"quoted"`,
	})
	const prefix = "Tool Output: "
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("output = %q, want Tool Output prefix", raw)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, prefix)), &envelope); err != nil {
		t.Fatalf("MQTT output is not valid JSON: %v; output=%s", err, raw)
	}
	if got, want := envelope["message"], `MQTT publish failed: broker said "bad"\\path`; got != want {
		t.Fatalf("message = %#v, want %q", got, want)
	}
	if got, want := envelope["topic"], `home/"quoted"`; got != want {
		t.Fatalf("topic = %#v, want %q", got, want)
	}
}

func TestDispatchGitHubTrackProjectDoesNotGrantRemoteRepoAccess(t *testing.T) {
	workspaceDir := t.TempDir()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("github_token", "token"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	cfg := &config.Config{}
	cfg.GitHub.Enabled = true
	cfg.GitHub.Owner = "owner"
	cfg.Directories.WorkspaceDir = workspaceDir
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dc := &DispatchContext{Cfg: cfg, Logger: logger, Vault: vault}

	out, ok := dispatchCloud(context.Background(), ToolCall{
		Action: "github",
		Params: map[string]interface{}{
			"operation": "track_project",
			"name":      "repo",
			"content":   "manual tracking only",
		},
	}, dc)
	if !ok {
		t.Fatal("expected dispatchCloud to handle github")
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected track_project success, got %s", out)
	}

	data, err := os.ReadFile(filepath.Join(workspaceDir, "github", "projects.json"))
	if err != nil {
		t.Fatalf("expected projects file: %v", err)
	}
	var projects []tools.TrackedProject
	if err := json.Unmarshal(data, &projects); err != nil {
		t.Fatalf("unmarshal projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects len = %d, want 1", len(projects))
	}
	if projects[0].AgentCreated {
		t.Fatalf("manual track_project must not mark project as agent-created: %+v", projects[0])
	}

	out, ok = dispatchCloud(context.Background(), ToolCall{
		Action: "github",
		Params: map[string]interface{}{
			"operation": "get_repo",
			"owner":     "owner",
			"name":      "repo",
		},
	}, dc)
	if !ok {
		t.Fatal("expected dispatchCloud to handle github get_repo")
	}
	if !strings.Contains(out, "allowed repos") {
		t.Fatalf("expected manual tracked repo to remain blocked, got %s", out)
	}
}

func TestDecodeGrafanaArgsSupportsUIDTypeAndPagination(t *testing.T) {
	req := decodeGrafanaArgs(ToolCall{
		Params: map[string]interface{}{
			"operation":       "query",
			"query":           "up",
			"datasource_uid":  "prom-main",
			"datasource_type": "prometheus",
			"from":            "now-30m",
			"to":              "now",
			"format":          "table",
			"max_data_points": float64(500),
			"interval_ms":     float64(15000),
			"limit":           float64(25),
			"page":            float64(2),
		},
	})

	if req.DatasourceUID != "prom-main" {
		t.Fatalf("DatasourceUID = %q, want prom-main", req.DatasourceUID)
	}
	if req.DatasourceType != "prometheus" {
		t.Fatalf("DatasourceType = %q, want prometheus", req.DatasourceType)
	}
	if req.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", req.Limit)
	}
	if req.Page != 2 {
		t.Fatalf("Page = %d, want 2", req.Page)
	}
	if req.From != "now-30m" || req.To != "now" {
		t.Fatalf("time range = %q/%q, want now-30m/now", req.From, req.To)
	}
	if req.Format != "table" {
		t.Fatalf("Format = %q, want table", req.Format)
	}
	if req.MaxDataPoints != 500 {
		t.Fatalf("MaxDataPoints = %d, want 500", req.MaxDataPoints)
	}
	if req.IntervalMS != 15000 {
		t.Fatalf("IntervalMS = %d, want 15000", req.IntervalMS)
	}
}

func TestGrafanaReadOnlyGuardBlocksFutureMutations(t *testing.T) {
	for _, op := range []string{"create_dashboard", "update_dashboard", "delete_dashboard", "pause_alert", "create_annotation"} {
		if !isGrafanaMutation(op) {
			t.Fatalf("isGrafanaMutation(%q) = false, want true", op)
		}
	}
	for _, op := range []string{"health", "list_dashboards", "get_dashboard", "list_datasources", "query", "list_alerts", "get_org"} {
		if isGrafanaMutation(op) {
			t.Fatalf("isGrafanaMutation(%q) = true, want false", op)
		}
	}
}
