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
				Icon  string `json:"icon"`
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
			if widget.Icon != "weather" {
				t.Fatalf("icon = %q, want weather", widget.Icon)
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

func TestExecuteVirtualDesktopStatusExposesIconCatalog(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{"operation": "status"})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			IconCatalog struct {
				Theme              string            `json:"theme"`
				DefaultTheme       string            `json:"default_theme"`
				Preferred          []string          `json:"preferred"`
				Aliases            map[string]string `json:"aliases"`
				LegacySpritePrefix string            `json:"legacy_sprite_prefix"`
			} `json:"icon_catalog"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output = %s", payload.Status, exec.Output)
	}
	if payload.Data.IconCatalog.Theme != "papirus" || payload.Data.IconCatalog.DefaultTheme != "papirus" {
		t.Fatalf("icon catalog theme = %+v", payload.Data.IconCatalog)
	}
	if payload.Data.IconCatalog.LegacySpritePrefix != "sprite:" {
		t.Fatalf("legacy sprite prefix = %q", payload.Data.IconCatalog.LegacySpritePrefix)
	}
	if len(payload.Data.IconCatalog.Preferred) < 20 {
		t.Fatalf("expected generated app icon names, got %+v", payload.Data.IconCatalog.Preferred)
	}
	if payload.Data.IconCatalog.Aliases["widgets"] != "apps" {
		t.Fatalf("widgets alias = %q, want apps", payload.Data.IconCatalog.Aliases["widgets"])
	}
}

func TestExecuteVirtualDesktopOfficeDocumentOperations(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_document",
		"path":      "Documents/notes.docx",
		"title":     "Notes",
		"content":   "Hello Writer\nFrom the agent",
	})
	var writePayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(write.Output), &writePayload); err != nil {
		t.Fatalf("decode write_document: %v output=%s", err, write.Output)
	}
	if writePayload.Status != "ok" {
		t.Fatalf("write_document = %s", write.Output)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_document",
		"path":      "Documents/notes.docx",
	})
	var readPayload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				Text string `json:"text"`
			} `json:"document"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(read.Output), &readPayload); err != nil {
		t.Fatalf("decode read_document: %v output=%s", err, read.Output)
	}
	if readPayload.Status != "ok" || readPayload.Data.Document.Text != "Hello Writer\nFrom the agent" {
		t.Fatalf("read_document payload = %+v", readPayload)
	}
}

func TestExecuteVirtualDesktopWorkbookOperations(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true
	workbook := map[string]interface{}{
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Budget",
				"rows": []interface{}{
					[]interface{}{map[string]interface{}{"value": "Item"}, map[string]interface{}{"value": "Amount"}},
					[]interface{}{map[string]interface{}{"value": "Coffee"}, map[string]interface{}{"value": "12.50"}},
				},
			},
		},
	}
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_workbook",
		"path":      "Documents/budget.xlsx",
		"workbook":  workbook,
	})
	var statusPayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(write.Output), &statusPayload); err != nil {
		t.Fatalf("decode write_workbook: %v output=%s", err, write.Output)
	}
	if statusPayload.Status != "ok" {
		t.Fatalf("write_workbook = %s", write.Output)
	}

	set := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/budget.xlsx",
		"sheet":     "Budget",
		"cell":      "B3",
		"formula":   "SUM(B2:B2)",
	})
	statusPayload = struct {
		Status string `json:"status"`
	}{}
	if err := json.Unmarshal([]byte(set.Output), &statusPayload); err != nil {
		t.Fatalf("decode set_cell: %v output=%s", err, set.Output)
	}
	if statusPayload.Status != "ok" {
		t.Fatalf("set_cell = %s", set.Output)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_workbook",
		"path":      "Documents/budget.xlsx",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Workbook struct {
				Sheets []struct {
					Rows [][]struct {
						Value   string `json:"value"`
						Formula string `json:"formula"`
					} `json:"rows"`
				} `json:"sheets"`
			} `json:"workbook"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(read.Output), &payload); err != nil {
		t.Fatalf("decode read_workbook: %v output=%s", err, read.Output)
	}
	if payload.Status != "ok" || payload.Data.Workbook.Sheets[0].Rows[2][1].Formula != "SUM(B2:B2)" {
		t.Fatalf("read_workbook payload = %+v", payload)
	}
}

func TestExecuteVirtualDesktopExportFileReturnsVersionedEntry(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_document",
		"path":      "Documents/export-source.md",
		"content":   "Export me",
	})
	var writePayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(write.Output), &writePayload); err != nil {
		t.Fatalf("decode write_document: %v output=%s", err, write.Output)
	}
	if writePayload.Status != "ok" {
		t.Fatalf("write_document = %s", write.Output)
	}

	exported := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation":   "export_file",
		"path":        "Documents/export-source.md",
		"output_path": "Documents/export-target.txt",
		"format":      "txt",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			OutputPath    string                 `json:"output_path"`
			OfficeVersion map[string]interface{} `json:"office_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(exported.Output), &payload); err != nil {
		t.Fatalf("decode export_file: %v output=%s", err, exported.Output)
	}
	if payload.Status != "ok" || payload.Data.OutputPath != "Documents/export-target.txt" || payload.Data.OfficeVersion["etag"] == "" {
		t.Fatalf("export_file payload = %+v output=%s", payload, exported.Output)
	}
}

func TestExecuteVirtualDesktopInstallAppNormalizesIconAlias(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "install_app",
		"manifest": map[string]interface{}{
			"id":      "todo-board",
			"name":    "Todo Board",
			"version": "1.0.0",
			"icon":    "todo",
			"entry":   "index.html",
		},
		"files": map[string]interface{}{
			"index.html": "<main>Todo</main>",
		},
	})
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output = %s", payload.Status, exec.Output)
	}

	status := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{"operation": "status"})
	var bootstrap struct {
		Data struct {
			InstalledApps []struct {
				ID   string `json:"id"`
				Icon string `json:"icon"`
			} `json:"installed_apps"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(status.Output), &bootstrap); err != nil {
		t.Fatalf("unmarshal status: %v\n%s", err, status.Output)
	}
	for _, app := range bootstrap.Data.InstalledApps {
		if app.ID == "todo-board" {
			if app.Icon != "notes" {
				t.Fatalf("icon = %q, want notes", app.Icon)
			}
			return
		}
	}
	t.Fatalf("todo-board not installed: %+v", bootstrap.Data.InstalledApps)
}
