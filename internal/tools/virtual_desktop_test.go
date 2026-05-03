package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func testVirtualDesktopConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = filepath.Join(root, "workspace")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "desktop.db")
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.VirtualDesktop.AllowGeneratedApps = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.VirtualDesktop.MaxFileSizeMB = 1
	cfg.Tools.VirtualDesktop.Enabled = true
	return cfg
}

func TestExecuteVirtualDesktopWriteFileAutoRegistersStandaloneWidget(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/weather_pforzheim.html",
		"content":   "<main>Weather</main>",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			WidgetID string `json:"widget_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output = %s", payload.Status, exec.Output)
	}
	if payload.Data.WidgetID != "weather_pforzheim" {
		t.Fatalf("widget_id = %q, want weather_pforzheim", payload.Data.WidgetID)
	}

	status := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{"operation": "status"})
	var bootstrap struct {
		Status string `json:"status"`
		Data   struct {
			Widgets []struct {
				ID    string `json:"id"`
				AppID string `json:"app_id"`
				Entry string `json:"entry"`
			} `json:"widgets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(status.Output), &bootstrap); err != nil {
		t.Fatalf("unmarshal status: %v\n%s", err, status.Output)
	}
	for _, widget := range bootstrap.Data.Widgets {
		if widget.ID == "weather_pforzheim" {
			if widget.AppID != "" {
				t.Fatalf("app_id = %q, want standalone widget", widget.AppID)
			}
			if widget.Entry != "weather_pforzheim.html" {
				t.Fatalf("entry = %q, want weather_pforzheim.html", widget.Entry)
			}
			return
		}
	}
	t.Fatalf("weather_pforzheim widget not registered: %+v", bootstrap.Data.Widgets)
}

func TestExecuteVirtualDesktopWriteFileRejectsEmptyStandaloneWidgetHTML(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/weather_pforzheim.html",
		"content":   " \n\t",
	})
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "error" {
		t.Fatalf("status = %q, want error: %s", payload.Status, exec.Output)
	}
}
