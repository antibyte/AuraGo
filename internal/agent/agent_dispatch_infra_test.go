package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
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

func TestDecodeGrafanaArgsSupportsUIDTypeAndPagination(t *testing.T) {
	req := decodeGrafanaArgs(ToolCall{
		Params: map[string]interface{}{
			"operation":       "query",
			"query":           "up",
			"datasource_uid":  "prom-main",
			"datasource_type": "prometheus",
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
