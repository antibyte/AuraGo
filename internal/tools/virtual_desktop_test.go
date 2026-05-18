package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestExecuteVirtualDesktopWriteFileAutoRegistersStandaloneWidgetIndexFile(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/printer-camera/index.html",
		"content":   "<main>Printer camera</main>",
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
	if payload.Data.WidgetID != "printer-camera" {
		t.Fatalf("widget_id = %q, want printer-camera", payload.Data.WidgetID)
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
		if widget.ID == "printer-camera" {
			if widget.AppID != "" {
				t.Fatalf("app_id = %q, want standalone widget", widget.AppID)
			}
			if widget.Entry != "printer-camera/index.html" {
				t.Fatalf("entry = %q, want printer-camera/index.html", widget.Entry)
			}
			return
		}
	}
	t.Fatalf("printer-camera widget not registered: %+v", bootstrap.Data.Widgets)
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

func TestExecuteVirtualDesktopWriteFileRejectsEmptyContentWithoutDestroyingExistingFile(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	initial := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders/game.js",
		"content":   "window.spaceInvaders = true;\n",
	})
	if !strings.Contains(initial.Output, `"status":"ok"`) {
		t.Fatalf("initial write failed: %s", initial.Output)
	}

	empty := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders/game.js",
	})
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(empty.Output), &payload); err != nil {
		t.Fatalf("unmarshal empty write output: %v\n%s", err, empty.Output)
	}
	if payload.Status != "error" {
		t.Fatalf("empty write status = %q, want error: %s", payload.Status, empty.Output)
	}
	if !strings.Contains(payload.Message, "content") {
		t.Fatalf("empty write error should mention content, got %q", payload.Message)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_file",
		"path":      "Apps/space-invaders/game.js",
	})
	if !strings.Contains(read.Output, "window.spaceInvaders = true;") {
		t.Fatalf("empty write destroyed existing desktop file: %s", read.Output)
	}
}

func TestExecuteVirtualDesktopWriteFileAllowsExplicitEmptyContent(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	initial := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Notes/empty.txt",
		"content":   "temporary",
	})
	if !strings.Contains(initial.Output, `"status":"ok"`) {
		t.Fatalf("initial write failed: %s", initial.Output)
	}

	empty := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation":   "write_file",
		"path":        "Notes/empty.txt",
		"content":     "",
		"allow_empty": true,
	})
	if !strings.Contains(empty.Output, `"status":"ok"`) {
		t.Fatalf("explicit empty write failed: %s", empty.Output)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_file",
		"path":      "Notes/empty.txt",
	})
	if !strings.Contains(read.Output, `"content":""`) {
		t.Fatalf("explicit empty file content not persisted as empty: %s", read.Output)
	}
}

func TestExecuteVirtualDesktopUpsertWidgetNormalizesExistingStandaloneWidgetIndexFile(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/printer-camera/index.html",
		"content":   "<main>Printer camera</main>",
	})
	var writePayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(write.Output), &writePayload); err != nil {
		t.Fatalf("unmarshal write output: %v\n%s", err, write.Output)
	}
	if writePayload.Status != "ok" {
		t.Fatalf("write_file status = %q, output = %s", writePayload.Status, write.Output)
	}

	upsert := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "upsert_widget",
		"widget": map[string]interface{}{
			"id":     "printer-camera",
			"app_id": "printer-camera",
			"title":  "Printer Camera",
			"entry":  "index.html",
			"icon":   "camera",
		},
	})
	var upsertPayload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(upsert.Output), &upsertPayload); err != nil {
		t.Fatalf("unmarshal upsert output: %v\n%s", err, upsert.Output)
	}
	if upsertPayload.Status != "ok" {
		t.Fatalf("upsert_widget status = %q message = %q output = %s", upsertPayload.Status, upsertPayload.Message, upsert.Output)
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
		if widget.ID == "printer-camera" {
			if widget.AppID != "" {
				t.Fatalf("app_id = %q, want standalone widget", widget.AppID)
			}
			if widget.Entry != "printer-camera/index.html" {
				t.Fatalf("entry = %q, want printer-camera/index.html", widget.Entry)
			}
			return
		}
	}
	t.Fatalf("printer-camera widget not registered: %+v", bootstrap.Data.Widgets)
}

func TestExecuteVirtualDesktopUpsertWidgetNormalizesExistingStandaloneWidgetFile(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/space-invaders.html",
		"content":   "<main>Space Invaders</main>",
	})
	var writePayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(write.Output), &writePayload); err != nil {
		t.Fatalf("unmarshal write output: %v\n%s", err, write.Output)
	}
	if writePayload.Status != "ok" {
		t.Fatalf("write_file status = %q, output = %s", writePayload.Status, write.Output)
	}

	upsert := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "upsert_widget",
		"widget": map[string]interface{}{
			"id":     "space-invaders",
			"app_id": "space-invaders",
			"title":  "Space Invaders",
			"entry":  "space-invaders.html",
		},
	})
	var upsertPayload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(upsert.Output), &upsertPayload); err != nil {
		t.Fatalf("unmarshal upsert output: %v\n%s", err, upsert.Output)
	}
	if upsertPayload.Status != "ok" {
		t.Fatalf("upsert_widget status = %q message = %q output = %s", upsertPayload.Status, upsertPayload.Message, upsert.Output)
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
		if widget.ID == "space-invaders" {
			if widget.AppID != "" {
				t.Fatalf("app_id = %q, want standalone widget", widget.AppID)
			}
			if widget.Entry != "space-invaders.html" {
				t.Fatalf("entry = %q, want space-invaders.html", widget.Entry)
			}
			return
		}
	}
	t.Fatalf("space-invaders widget not registered: %+v", bootstrap.Data.Widgets)
}

func TestExecuteVirtualDesktopOpenInAppOpensStandaloneWidgetFile(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/space-invaders.html",
		"content":   "<main>Space Invaders</main>",
	})
	var writePayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(write.Output), &writePayload); err != nil {
		t.Fatalf("unmarshal write output: %v\n%s", err, write.Output)
	}
	if writePayload.Status != "ok" {
		t.Fatalf("write_file status = %q, output = %s", writePayload.Status, write.Output)
	}

	open := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_in_app",
		"app_id":    "space-invaders",
		"path":      "Widgets/space-invaders.html",
	})
	var openPayload struct {
		Status string `json:"status"`
		Data   struct {
			WidgetID string `json:"widget_id"`
			Path     string `json:"path"`
			Icon     string `json:"icon"`
		} `json:"data"`
		Event struct {
			Type    string `json:"type"`
			Payload struct {
				WidgetID string `json:"widget_id"`
				Path     string `json:"path"`
			} `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(open.Output), &openPayload); err != nil {
		t.Fatalf("unmarshal open output: %v\n%s", err, open.Output)
	}
	if openPayload.Status != "ok" {
		t.Fatalf("open_in_app status = %q, output = %s", openPayload.Status, open.Output)
	}
	if openPayload.Event.Type != "open_widget" {
		t.Fatalf("event type = %q, want open_widget: %s", openPayload.Event.Type, open.Output)
	}
	if openPayload.Data.WidgetID != "space-invaders" || openPayload.Event.Payload.WidgetID != "space-invaders" {
		t.Fatalf("widget IDs = data:%q event:%q, want space-invaders", openPayload.Data.WidgetID, openPayload.Event.Payload.WidgetID)
	}
	if openPayload.Data.Path != "Widgets/space-invaders.html" || openPayload.Event.Payload.Path != "Widgets/space-invaders.html" {
		t.Fatalf("paths = data:%q event:%q, want Widgets/space-invaders.html", openPayload.Data.Path, openPayload.Event.Payload.Path)
	}
	if openPayload.Data.Icon == "" {
		t.Fatalf("icon is empty: %s", open.Output)
	}
	if open.Event == nil || open.Event.Type != "open_widget" {
		t.Fatalf("execution event = %#v, want open_widget", open.Event)
	}
}

func TestExecuteVirtualDesktopOpenInAppRejectsMissingStandaloneWidgetFileWithCode(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	open := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_in_app",
		"app_id":    "space-invaders",
		"path":      "Widgets/space-invaders.html",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Code string `json:"code"`
			Path string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(open.Output), &payload); err != nil {
		t.Fatalf("unmarshal open output: %v\n%s", err, open.Output)
	}
	if payload.Status != "error" {
		t.Fatalf("status = %s, want error: %s", payload.Status, open.Output)
	}
	if payload.Data.Code != "desktop_widget_not_registered" {
		t.Fatalf("code = %s, want desktop_widget_not_registered: %s", payload.Data.Code, open.Output)
	}
	if open.Event != nil {
		t.Fatalf("event = %#v, want nil", open.Event)
	}
}

func TestExecuteVirtualDesktopOpenInAppRejectsMissingApp(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	open := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_in_app",
		"app_id":    "missing-app",
	})
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(open.Output), &payload); err != nil {
		t.Fatalf("unmarshal open output: %v\n%s", err, open.Output)
	}
	if payload.Status != "error" {
		t.Fatalf("status = %q, want error: %s", payload.Status, open.Output)
	}
	if open.Event != nil {
		t.Fatalf("event = %#v, want nil", open.Event)
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

func TestExecuteVirtualDesktopWriteFileRewritesConfiguredPrinterCameraURLToProxy(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:  "printer-1",
		URL: "ws://192.168.6.181/websocket",
	}}
	rawURL := "http://192.168.6.181:3031/video"
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Widgets/printer-camera/index.html",
		"content":   `<img src="` + rawURL + `?t=123" alt="printer">`,
	})
	if !strings.Contains(exec.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", exec.Output)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_file",
		"path":      "Widgets/printer-camera/index.html",
	})
	if strings.Contains(read.Output, rawURL) {
		t.Fatalf("desktop widget kept raw camera URL: %s", read.Output)
	}
	if !strings.Contains(read.Output, `/api/3d-printers/printer-1/camera/stream?t=123`) {
		t.Fatalf("desktop widget did not use same-origin camera proxy: %s", read.Output)
	}
}

func TestExecuteVirtualDesktopInstallAppRewritesConfiguredPrinterCameraURLToProxy(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:  "printer-1",
		URL: "ws://192.168.6.181/websocket",
	}}
	rawURL := "http://192.168.6.181:3031/video"
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "install_app",
		"manifest": map[string]interface{}{
			"id":      "printer-camera",
			"name":    "Printer Camera",
			"version": "1.0.0",
			"icon":    "camera",
			"entry":   "index.html",
		},
		"files": map[string]interface{}{
			"index.html": `<video src="` + rawURL + `"></video>`,
		},
	})
	if !strings.Contains(exec.Output, `"status":"ok"`) {
		t.Fatalf("install_app failed: %s", exec.Output)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_file",
		"path":      "Apps/printer-camera/index.html",
	})
	if strings.Contains(read.Output, rawURL) {
		t.Fatalf("desktop app kept raw camera URL: %s", read.Output)
	}
	if !strings.Contains(read.Output, `/api/3d-printers/printer-1/camera/stream`) {
		t.Fatalf("desktop app did not use same-origin camera proxy: %s", read.Output)
	}
}

func TestExecuteVirtualDesktopWriteFileRootAppHTMLRegistersRunnableGeneratedApp(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	html := "<!doctype html><html><body><canvas id=\"game\"></canvas><script>window.spaceInvaders = true;</script></body></html>"
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders.html",
		"content":   html,
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			AppID     string `json:"app_id"`
			EntryPath string `json:"entry_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal write_file output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output = %s", payload.Status, exec.Output)
	}
	if payload.Data.AppID != "space-invaders" {
		t.Fatalf("app_id = %q, want space-invaders; output=%s", payload.Data.AppID, exec.Output)
	}
	if payload.Data.EntryPath != "Apps/space-invaders/index.html" {
		t.Fatalf("entry_path = %q, want Apps/space-invaders/index.html", payload.Data.EntryPath)
	}

	status := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{"operation": "status"})
	var bootstrap struct {
		Data struct {
			InstalledApps []struct {
				ID           string `json:"id"`
				Entry        string `json:"entry"`
				Health       string `json:"health"`
				HealthReason string `json:"health_reason"`
			} `json:"installed_apps"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(status.Output), &bootstrap); err != nil {
		t.Fatalf("unmarshal status: %v\n%s", err, status.Output)
	}
	for _, app := range bootstrap.Data.InstalledApps {
		if app.ID == "space-invaders" {
			if app.Entry != "index.html" {
				t.Fatalf("entry = %q, want index.html", app.Entry)
			}
			if app.Health != "" || app.HealthReason != "" {
				t.Fatalf("newly registered app should be healthy, got health=%q reason=%q", app.Health, app.HealthReason)
			}
			read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
				"operation": "read_file",
				"path":      "Apps/space-invaders/index.html",
			})
			if !strings.Contains(read.Output, "window.spaceInvaders") {
				t.Fatalf("registered app entry did not contain generated HTML: %s", read.Output)
			}
			return
		}
	}
	t.Fatalf("space-invaders app not installed: %+v", bootstrap.Data.InstalledApps)
}

func TestExecuteVirtualDesktopOpenAppRejectsBrokenGeneratedApp(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders.html",
		"content":   "<main>Space Invaders</main>",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", write.Output)
	}
	entryPath := filepath.Join(cfg.VirtualDesktop.WorkspaceDir, "Apps", "space-invaders", "index.html")
	if err := os.Remove(entryPath); err != nil {
		t.Fatalf("remove generated app entry: %v", err)
	}

	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_app",
		"app_id":    "space-invaders",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Code      string `json:"code"`
			AppID     string `json:"app_id"`
			EntryPath string `json:"entry_path"`
		} `json:"data"`
		Event interface{} `json:"event"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal open_app output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "error" {
		t.Fatalf("status = %q, want error; output=%s", payload.Status, exec.Output)
	}
	if payload.Data.Code != "desktop_app_entry_missing" {
		t.Fatalf("code = %q, want desktop_app_entry_missing; output=%s", payload.Data.Code, exec.Output)
	}
	if payload.Data.AppID != "space-invaders" {
		t.Fatalf("app_id = %q, want space-invaders", payload.Data.AppID)
	}
	if payload.Data.EntryPath != "Apps/space-invaders/index.html" {
		t.Fatalf("entry_path = %q, want Apps/space-invaders/index.html", payload.Data.EntryPath)
	}
	if payload.Event != nil || exec.Event != nil {
		t.Fatalf("broken app must not emit open event: payload=%+v exec=%+v", payload.Event, exec.Event)
	}
}

func TestExecuteVirtualDesktopOpenInAppInfersGeneratedAppFromEntryPath(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	install := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "install_app",
		"manifest": map[string]interface{}{
			"id":      "space-invaders",
			"name":    "Space Invaders",
			"entry":   "index.html",
			"runtime": "aura-desktop-sdk@1",
		},
		"files": map[string]interface{}{
			"index.html": "<main>Space Invaders</main><script src=\"game.js\"></script>",
			"game.js":    "window.spaceInvaders = true;",
		},
	})
	if !strings.Contains(install.Output, `"status":"ok"`) {
		t.Fatalf("install_app failed: %s", install.Output)
	}

	open := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_in_app",
		"path":      "Apps/space-invaders/index.html",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			AppID string `json:"app_id"`
			Path  string `json:"path"`
		} `json:"data"`
		Event struct {
			Type    string `json:"type"`
			Payload struct {
				AppID string `json:"app_id"`
				Path  string `json:"path"`
			} `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(open.Output), &payload); err != nil {
		t.Fatalf("unmarshal open_in_app output: %v\n%s", err, open.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output=%s", payload.Status, open.Output)
	}
	if payload.Data.AppID != "space-invaders" || payload.Data.Path != "Apps/space-invaders/index.html" {
		t.Fatalf("data = %+v, want generated app open payload", payload.Data)
	}
	if payload.Event.Type != "open_app" || payload.Event.Payload.AppID != "space-invaders" {
		t.Fatalf("event = %+v, want open_app for generated app", payload.Event)
	}
}

func TestExecuteVirtualDesktopOpenCodeStudioNormalizesWorkspacePath(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_in_app",
		"app_id":    "code-studio",
		"path":      "workspace/src/main.go",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			AppID string `json:"app_id"`
			Path  string `json:"path"`
		} `json:"data"`
		Event struct {
			Payload struct {
				AppID string `json:"app_id"`
				Path  string `json:"path"`
			} `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal open_in_app output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output=%s", payload.Status, exec.Output)
	}
	if payload.Data.AppID != "code-studio" || payload.Data.Path != "/workspace/src/main.go" {
		t.Fatalf("data = %+v, want normalized Code Studio path", payload.Data)
	}
	if payload.Event.Payload.Path != "/workspace/src/main.go" {
		t.Fatalf("event path = %q, want normalized Code Studio path", payload.Event.Payload.Path)
	}
}

func TestExecuteVirtualDesktopOpenCodeStudioIgnoresHostPath(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "open_in_app",
		"app_id":    "code-studio",
		"path":      `C:\Users\Andi\Documents\repo\AuraGo\internal\agent\agent.go`,
	})
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AppID       string `json:"app_id"`
			Path        string `json:"path"`
			PathIgnored bool   `json:"path_ignored"`
			PathPolicy  string `json:"path_policy"`
		} `json:"data"`
		Event struct {
			Payload struct {
				AppID       string `json:"app_id"`
				Path        string `json:"path"`
				PathIgnored bool   `json:"path_ignored"`
			} `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal open_in_app output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output=%s", payload.Status, exec.Output)
	}
	if payload.Data.AppID != "code-studio" || !payload.Data.PathIgnored || payload.Data.Path != "" {
		t.Fatalf("data = %+v, want host path ignored", payload.Data)
	}
	if payload.Data.PathPolicy != "code_studio_paths_must_be_inside_workspace" || !payload.Event.Payload.PathIgnored || payload.Event.Payload.Path != "" {
		t.Fatalf("policy/event = data:%+v event:%+v", payload.Data, payload.Event.Payload)
	}
	if strings.Contains(exec.Output, "Users") || strings.Contains(exec.Output, "AuraGo") {
		t.Fatalf("ignored host path leaked into output: %s", exec.Output)
	}
}

func TestExecuteVirtualDesktopDeleteRootAppHTMLRemovesGeneratedApp(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders.html",
		"content":   "<main>Space Invaders</main>",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", write.Output)
	}

	exec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "delete",
		"path":      "Apps/space-invaders.html",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			AppID string `json:"app_id"`
			Path  string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(exec.Output), &payload); err != nil {
		t.Fatalf("unmarshal delete output: %v\n%s", err, exec.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("delete status = %q, output = %s", payload.Status, exec.Output)
	}
	if payload.Data.AppID != "space-invaders" || payload.Data.Path != "Apps/space-invaders.html" {
		t.Fatalf("delete payload = %+v", payload.Data)
	}

	status := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{"operation": "status"})
	if strings.Contains(status.Output, `"id":"space-invaders"`) {
		t.Fatalf("deleted generated app still registered: %s", status.Output)
	}
}

func TestExecuteVirtualDesktopReadLargeFileTruncatesAndSuggestsPatchFile(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	largeContent := strings.Repeat("0123456789abcdef\n", 600)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders/game.js",
		"content":   largeContent,
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", write.Output)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_file",
		"path":      "Apps/space-invaders/game.js",
	})
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Content          string   `json:"content"`
			ContentTruncated bool     `json:"content_truncated"`
			OriginalSize     int      `json:"original_size"`
			SuggestedTools   []string `json:"suggested_tools"`
			EditingHint      string   `json:"editing_hint"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(read.Output), &payload); err != nil {
		t.Fatalf("unmarshal read_file output: %v\n%s", err, read.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("status = %q, output=%s", payload.Status, read.Output)
	}
	if !payload.Data.ContentTruncated {
		t.Fatalf("expected large read_file to be truncated: %+v", payload.Data)
	}
	if payload.Data.OriginalSize <= 8*1024 || payload.Data.OriginalSize > len(largeContent) {
		t.Fatalf("original_size = %d, want large stored size up to %d", payload.Data.OriginalSize, len(largeContent))
	}
	if len(payload.Data.Content) >= len(largeContent) || len(read.Output) >= len(largeContent) {
		t.Fatalf("large content was not capped: content_len=%d output_len=%d", len(payload.Data.Content), len(read.Output))
	}
	if !strings.Contains(payload.Data.EditingHint, "patch_file") || !strings.Contains(strings.Join(payload.Data.SuggestedTools, ","), "virtual_desktop.patch_file") {
		t.Fatalf("expected patch_file guidance, data=%+v", payload.Data)
	}
}

func TestExecuteVirtualDesktopPatchFileAppliesExactReplacements(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders/game.js",
		"content":   "const lives = 3;\nfunction explode() {}\n",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", write.Output)
	}

	patch := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "patch_file",
		"path":      "Apps/space-invaders/game.js",
		"replacements": []map[string]string{{
			"find":    "const lives = 3;",
			"replace": "const lives = 5;",
		}},
		"append_text": "\nfunction shakeCamera() {}\n",
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Path              string `json:"path"`
			Replacements      int    `json:"replacements"`
			AppliedOperations int    `json:"applied_operations"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(patch.Output), &payload); err != nil {
		t.Fatalf("unmarshal patch_file output: %v\n%s", err, patch.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("patch status = %q, output=%s", payload.Status, patch.Output)
	}
	if payload.Data.Replacements != 1 || payload.Data.AppliedOperations != 2 {
		t.Fatalf("patch data = %+v, want one replacement plus append", payload.Data)
	}

	read := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "read_file",
		"path":      "Apps/space-invaders/game.js",
	})
	if !strings.Contains(read.Output, "const lives = 5;") || !strings.Contains(read.Output, "shakeCamera") {
		t.Fatalf("patched content missing expected changes: %s", read.Output)
	}
}

func TestExecuteVirtualDesktopSearchFileReturnsContextForLargeFiles(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	largeContent := strings.Repeat("filler line\n", 500) +
		"function destroyAlien(alien) {\n  alien.destroy();\n}\n" +
		strings.Repeat("tail line\n", 500)
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders/game.js",
		"content":   largeContent,
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", write.Output)
	}

	search := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation":     "search_file",
		"path":          "Apps/space-invaders/game.js",
		"query":         "destroyAlien",
		"context_lines": 1,
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Path        string `json:"path"`
			Query       string `json:"query"`
			MatchCount  int    `json:"match_count"`
			EditingHint string `json:"editing_hint"`
			Matches     []struct {
				Line    int    `json:"line"`
				Column  int    `json:"column"`
				Context string `json:"context"`
			} `json:"matches"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(search.Output), &payload); err != nil {
		t.Fatalf("unmarshal search_file output: %v\n%s", err, search.Output)
	}
	if payload.Status != "ok" || payload.Data.MatchCount != 1 || len(payload.Data.Matches) != 1 {
		t.Fatalf("search result = %+v, output=%s", payload, search.Output)
	}
	if !strings.Contains(payload.Data.Matches[0].Context, "function destroyAlien") || !strings.Contains(payload.Data.EditingHint, "patch_file") {
		t.Fatalf("search context/hint missing expected guidance: %+v", payload.Data)
	}
	if len(search.Output) >= len(largeContent) {
		t.Fatalf("search_file returned too much content: output_len=%d file_len=%d", len(search.Output), len(largeContent))
	}
}

func TestExecuteVirtualDesktopReadFileExcerptReturnsLineWindow(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	content := "one\nanchor\nthree\nfour\nfive\n"
	write := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "write_file",
		"path":      "Apps/space-invaders/game.js",
		"content":   content,
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write_file failed: %s", write.Output)
	}

	excerpt := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation":  "read_file_excerpt",
		"path":       "Apps/space-invaders/game.js",
		"line_start": 2,
		"line_count": 2,
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			LineStart   int    `json:"line_start"`
			LineEnd     int    `json:"line_end"`
			Excerpt     string `json:"excerpt"`
			EditingHint string `json:"editing_hint"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(excerpt.Output), &payload); err != nil {
		t.Fatalf("unmarshal read_file_excerpt output: %v\n%s", err, excerpt.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("excerpt status = %q, output=%s", payload.Status, excerpt.Output)
	}
	if payload.Data.LineStart != 2 || payload.Data.LineEnd != 3 || payload.Data.Excerpt != "anchor\nthree" {
		t.Fatalf("excerpt data = %+v", payload.Data)
	}
	if !strings.Contains(payload.Data.EditingHint, "patch_file") {
		t.Fatalf("excerpt should guide patch_file usage: %+v", payload.Data)
	}
}
