package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/office"
)

type VirtualDesktopExecution struct {
	Output string
	Event  *desktop.Event
}

var virtualDesktopWidgetIDSanitizer = regexp.MustCompile(`[^a-z0-9_-]+`)

var (
	toolDesktopMu  sync.Mutex
	toolDesktopSvc *desktop.Service
	toolDesktopCfg desktop.Config
)

// SetToolDesktopService registers a shared desktop.Service instance for reuse
// by ExecuteVirtualDesktop.  Pass nil to clear the cached instance.
func SetToolDesktopService(svc *desktop.Service) {
	toolDesktopMu.Lock()
	defer toolDesktopMu.Unlock()
	if toolDesktopSvc != nil && toolDesktopSvc != svc {
		_ = toolDesktopSvc.Close()
	}
	toolDesktopSvc = svc
	if svc != nil {
		toolDesktopCfg = svc.Config()
	} else {
		toolDesktopCfg = desktop.Config{}
	}
}

// CloseToolDesktopService tears down the cached singleton.  Call this during
// application shutdown.
func CloseToolDesktopService() {
	toolDesktopMu.Lock()
	defer toolDesktopMu.Unlock()
	if toolDesktopSvc != nil {
		_ = toolDesktopSvc.Close()
		toolDesktopSvc = nil
	}
}

// getToolDesktopService returns a cached service when the config matches, or
// creates a fresh one.  The fresh path is used by tests that construct their
// own config; the cached path is the hot path in production.
func getToolDesktopService(ctx context.Context, cfg *config.Config) (*desktop.Service, func(), error) {
	desktopCfg := desktop.ConfigFromAuraConfig(cfg)

	toolDesktopMu.Lock()
	if toolDesktopSvc != nil && toolDesktopCfg == desktopCfg {
		svc := toolDesktopSvc
		toolDesktopMu.Unlock()
		return svc, func() {}, nil
	}
	toolDesktopMu.Unlock()

	svc, err := desktop.NewService(desktopCfg)
	if err != nil {
		return nil, nil, err
	}
	if err := svc.Init(ctx); err != nil {
		_ = svc.Close()
		return nil, nil, err
	}
	return svc, func() { _ = svc.Close() }, nil
}

// ExecuteVirtualDesktop performs one agent-requested operation against the
// first-party virtual desktop workspace.
func ExecuteVirtualDesktop(ctx context.Context, cfg *config.Config, args map[string]interface{}) VirtualDesktopExecution {
	if cfg == nil {
		return virtualDesktopJSON("error", "configuration is unavailable", nil, nil)
	}
	if !cfg.VirtualDesktop.Enabled {
		return virtualDesktopJSON("error", "virtual desktop is disabled in config", nil, nil)
	}
	if !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled {
		return virtualDesktopJSON("error", "agent control for the virtual desktop is disabled in config", nil, nil)
	}
	op := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "operation", "action_type")))
	if op == "" {
		op = "status"
	}
	svc, cleanup, err := getToolDesktopService(ctx, cfg)
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	defer cleanup()

	switch op {
	case "status", "bootstrap":
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return virtualDesktopJSON("ok", "virtual desktop status", payload, nil)
	case "list_files":
		path := virtualDesktopString(args, "path", "file_path")
		files, err := svc.ListFiles(ctx, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return virtualDesktopJSON("ok", "desktop files listed", map[string]interface{}{"path": path, "files": files}, nil)
	case "read_file":
		path := virtualDesktopString(args, "path", "file_path")
		content, entry, err := svc.ReadFile(ctx, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return virtualDesktopJSON("ok", "desktop file read", map[string]interface{}{"entry": entry, "content": content}, nil)
	case "write_file":
		path := virtualDesktopString(args, "path", "file_path")
		content := virtualDesktopString(args, "content")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		if isVirtualDesktopStandaloneWidgetHTML(path) && strings.TrimSpace(content) == "" {
			return virtualDesktopJSON("error", "desktop widget HTML file must not be empty", nil, nil)
		}
		content = rewriteVirtualDesktopPrinterCameraURLsForPath(cfg, path, content)
		if app, entryPath, ok := virtualDesktopRootHTMLAppFromFile(path); ok && strings.TrimSpace(content) != "" {
			if err := svc.WriteFile(ctx, path, content, desktop.SourceAgent); err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			if err := svc.InstallApp(ctx, app, map[string]string{app.Entry: content}, desktop.SourceAgent); err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": "write_file", "path": path, "app_id": app.ID, "entry_path": entryPath})
			return virtualDesktopJSON("ok", "desktop app file written and registered", map[string]interface{}{"path": cleanVirtualDesktopSlashPath(path), "app_id": app.ID, "entry_path": entryPath}, event)
		}
		if err := svc.WriteFile(ctx, path, content, desktop.SourceAgent); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		if widget, ok := virtualDesktopStandaloneWidgetFromFile(path); ok && strings.TrimSpace(content) != "" {
			if err := svc.UpsertWidget(ctx, widget, desktop.SourceAgent); err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": "write_file", "path": path, "widget_id": widget.ID})
			return virtualDesktopJSON("ok", "desktop widget file written and registered", map[string]interface{}{"path": path, "widget_id": widget.ID}, event)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": path})
		return virtualDesktopJSON("ok", "desktop file written", map[string]interface{}{"path": path}, event)
	case "delete", "delete_file", "delete_path", "delete_app":
		appID := virtualDesktopString(args, "app_id", "id")
		path := virtualDesktopString(args, "path", "file_path")
		if strings.TrimSpace(appID) == "" && strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "app_id or path is required", nil, nil)
		}
		deletedAppID := ""
		if strings.TrimSpace(appID) != "" {
			if err := svc.DeleteApp(ctx, appID, desktop.SourceAgent); err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			deletedAppID = strings.ToLower(strings.TrimSpace(appID))
		} else if app, _, ok := virtualDesktopRootHTMLAppFromFile(path); ok {
			if err := svc.DeleteApp(ctx, app.ID, desktop.SourceAgent); err != nil && !strings.Contains(err.Error(), "desktop app not found") {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			deletedAppID = app.ID
		}
		cleanPath := cleanVirtualDesktopSlashPath(path)
		if strings.TrimSpace(path) != "" {
			if err := svc.DeletePath(ctx, cleanPath, desktop.SourceAgent); err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": "delete", "path": cleanPath, "app_id": deletedAppID})
		return virtualDesktopJSON("ok", "desktop item deleted", map[string]interface{}{"path": cleanPath, "app_id": deletedAppID}, event)
	case "read_document", "write_document", "patch_document":
		if err := officeToolAllowed(cfg, "document", op); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return executeOfficeDocumentOperation(ctx, svc, args, op)
	case "read_workbook", "write_workbook", "set_cell", "set_range", "evaluate_formula":
		if err := officeToolAllowed(cfg, "workbook", op); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		return executeOfficeWorkbookOperation(ctx, svc, args, op)
	case "export_file":
		path := virtualDesktopString(args, "path", "file_path", "source_path")
		outputPath := virtualDesktopString(args, "output_path", "target_path")
		format := strings.ToLower(strings.TrimPrefix(virtualDesktopString(args, "format"), "."))
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		if strings.TrimSpace(outputPath) == "" {
			return virtualDesktopJSON("error", "output_path is required", nil, nil)
		}
		data, entry, err := svc.ReadFileBytes(ctx, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		exported, err := virtualDesktopExportOffice(entry.Name, data, outputPath, format)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		outEntry, err := svc.WriteFileBytesConditional(ctx, outputPath, exported, desktop.SourceAgent, nil)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": path, "output_path": outputPath})
		return virtualDesktopJSON("ok", "desktop office file exported", map[string]interface{}{
			"path":           entry.Path,
			"output_path":    outEntry.Path,
			"entry":          outEntry,
			"office_version": officeToolVersionForEntry(outEntry, exported),
		}, event)
	case "install_app":
		manifest, err := virtualDesktopManifest(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		files, err := virtualDesktopFiles(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		files = rewriteVirtualDesktopPrinterCameraURLsForFiles(cfg, files)
		if err := svc.InstallApp(ctx, manifest, files, desktop.SourceAgent); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "app_id": manifest.ID})
		return virtualDesktopJSON("ok", "desktop app installed", map[string]interface{}{"app_id": manifest.ID}, event)
	case "upsert_widget":
		widget, err := virtualDesktopWidget(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		normalizeVirtualDesktopStandaloneWidget(ctx, svc, &widget)
		if err := svc.UpsertWidget(ctx, widget, desktop.SourceAgent); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "widget_id": widget.ID})
		return virtualDesktopJSON("ok", "desktop widget saved", map[string]interface{}{"widget_id": widget.ID}, event)
	case "open_app", "open_in_app":
		appID := virtualDesktopString(args, "app_id", "id")
		filePath := virtualDesktopString(args, "path", "file_path")
		if filePath != "" {
			widget, event, ok, err := virtualDesktopStandaloneWidgetOpenEvent(ctx, svc, filePath)
			if err != nil {
				if op == "open_in_app" && isVirtualDesktopStandaloneWidgetHTML(filePath) {
					return virtualDesktopJSON("error", "open_in_app widget path must refer to an existing non-empty Widgets/*.html file; use write_file or upsert_widget first", map[string]string{
						"code": "desktop_widget_not_registered",
						"path": cleanVirtualDesktopSlashPath(filePath),
					}, nil)
				}
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
			if ok {
				payload := map[string]interface{}{
					"widget_id": widget.ID,
					"path":      cleanVirtualDesktopSlashPath(filePath),
					"title":     widget.Title,
					"icon":      widget.Icon,
				}
				return virtualDesktopJSON("ok", "desktop widget open event emitted", payload, event)
			}
		}
		if appID == "" {
			return virtualDesktopJSON("error", "app_id is required", nil, nil)
		}
		app, ok := virtualDesktopFindApp(ctx, svc, appID)
		if !ok {
			return virtualDesktopJSON("error", fmt.Sprintf("desktop app %q is not installed", appID), nil, nil)
		}
		if app.Health == "broken" || app.HealthReason != "" {
			entryPath := app.EntryPath
			if entryPath == "" {
				entryPath = path.Join("Apps", app.ID, app.Entry)
			}
			return virtualDesktopJSON("error", fmt.Sprintf("desktop app %q is registered but its entry file is unavailable", appID), map[string]string{
				"code":          "desktop_app_entry_missing",
				"app_id":        app.ID,
				"entry_path":    entryPath,
				"health":        app.Health,
				"health_reason": app.HealthReason,
			}, nil)
		}
		payload := map[string]interface{}{"app_id": appID}
		if filePath != "" {
			payload["path"] = filePath
		}
		event := virtualDesktopEvent("open_app", payload)
		return virtualDesktopJSON("ok", "desktop app open event emitted", payload, event)
	case "show_notification":
		title := virtualDesktopString(args, "title", "name")
		message := virtualDesktopString(args, "message", "content")
		if message == "" {
			return virtualDesktopJSON("error", "message is required", nil, nil)
		}
		event := virtualDesktopEvent("notification", map[string]interface{}{"title": title, "message": message})
		return virtualDesktopJSON("ok", "desktop notification event emitted", map[string]interface{}{"title": title, "message": message}, event)
	default:
		return virtualDesktopJSON("error", fmt.Sprintf("unsupported virtual desktop operation %q", op), nil, nil)
	}
}

func virtualDesktopJSON(status, message string, data interface{}, event *desktop.Event) VirtualDesktopExecution {
	payload := map[string]interface{}{
		"status":  status,
		"message": message,
	}
	if data != nil {
		payload["data"] = data
	}
	if event != nil {
		payload["event"] = event
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return VirtualDesktopExecution{Output: fmt.Sprintf(`{"status":"error","message":%q}`, err.Error()), Event: event}
	}
	return VirtualDesktopExecution{Output: string(b), Event: event}
}

func virtualDesktopEvent(eventType string, payload interface{}) *desktop.Event {
	return &desktop.Event{Type: eventType, Payload: payload, CreatedAt: time.Now().UTC()}
}

func virtualDesktopString(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := args[key]; ok {
			switch v := raw.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case fmt.Stringer:
				if strings.TrimSpace(v.String()) != "" {
					return strings.TrimSpace(v.String())
				}
			default:
				if raw != nil {
					s := strings.TrimSpace(fmt.Sprint(raw))
					if s != "" && s != "<nil>" {
						return s
					}
				}
			}
		}
	}
	return ""
}

func virtualDesktopManifest(args map[string]interface{}) (desktop.AppManifest, error) {
	var manifest desktop.AppManifest
	if raw, ok := args["manifest"]; ok {
		if err := mapToStruct(raw, &manifest); err != nil {
			return manifest, fmt.Errorf("invalid manifest: %w", err)
		}
	}
	if manifest.ID == "" {
		manifest.ID = virtualDesktopString(args, "app_id", "id")
	}
	if manifest.Name == "" {
		manifest.Name = virtualDesktopString(args, "name", "title")
	}
	if manifest.Entry == "" {
		manifest.Entry = virtualDesktopString(args, "entry", "file_path", "path")
	}
	if manifest.Icon == "" {
		manifest.Icon = virtualDesktopString(args, "icon")
	}
	if manifest.Runtime == "" {
		manifest.Runtime = virtualDesktopString(args, "runtime")
	}
	if manifest.Description == "" {
		manifest.Description = virtualDesktopString(args, "description")
	}
	return manifest, nil
}

func virtualDesktopWidget(args map[string]interface{}) (desktop.Widget, error) {
	var widget desktop.Widget
	if raw, ok := args["widget"]; ok {
		if err := mapToStruct(raw, &widget); err != nil {
			return widget, fmt.Errorf("invalid widget: %w", err)
		}
	}
	if widget.ID == "" {
		widget.ID = virtualDesktopString(args, "widget_id", "id")
	}
	if widget.AppID == "" {
		widget.AppID = virtualDesktopString(args, "app_id")
	}
	if widget.Title == "" {
		widget.Title = virtualDesktopString(args, "title", "name")
	}
	if widget.Type == "" {
		widget.Type = virtualDesktopString(args, "type", "widget_type")
	}
	if widget.Icon == "" {
		widget.Icon = virtualDesktopString(args, "icon")
	}
	if widget.Entry == "" {
		widget.Entry = virtualDesktopString(args, "entry", "widget_entry")
	}
	if widget.Runtime == "" {
		widget.Runtime = virtualDesktopString(args, "runtime")
	}
	return widget, nil
}

func normalizeVirtualDesktopStandaloneWidget(ctx context.Context, svc *desktop.Service, widget *desktop.Widget) {
	if svc == nil || widget == nil {
		return
	}
	cleanEntry := ""
	for _, candidate := range virtualDesktopStandaloneWidgetEntryCandidates(widget.ID, widget.Entry) {
		candidatePath := path.Join("Widgets", candidate)
		if _, _, err := svc.ReadFile(ctx, candidatePath); err == nil {
			cleanEntry = candidate
			break
		}
	}
	if cleanEntry == "" {
		return
	}
	widget.AppID = ""
	widget.Entry = cleanEntry
	if widget.Type == "" {
		widget.Type = desktop.WidgetTypeCustom
	}
	if widget.Runtime == "" {
		widget.Runtime = desktop.AuraDesktopRuntime
	}
	if widget.Title == "" {
		widget.Title = virtualDesktopTitleFromID(strings.TrimSuffix(cleanEntry, path.Ext(cleanEntry)))
	}
}

func virtualDesktopStandaloneWidgetEntryCandidates(rawID, rawEntry string) []string {
	id := strings.ToLower(strings.TrimSpace(rawID))
	id = virtualDesktopWidgetIDSanitizer.ReplaceAllString(id, "-")
	id = strings.Trim(id, "_-")
	entry := strings.TrimPrefix(cleanVirtualDesktopSlashPath(rawEntry), "Widgets/")
	seen := map[string]bool{}
	var candidates []string
	add := func(candidate string) {
		candidate = strings.TrimPrefix(cleanVirtualDesktopSlashPath(candidate), "Widgets/")
		if candidate == "." || candidate == "" || strings.HasPrefix(candidate, "../") || path.IsAbs(candidate) {
			return
		}
		if seen[candidate] {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	if entry != "." && entry != "" {
		add(entry)
		if id != "" && !strings.Contains(entry, "/") {
			add(path.Join(id, entry))
		}
	}
	if id != "" {
		add(id + ".html")
		add(path.Join(id, "index.html"))
	}
	return candidates
}

func virtualDesktopStandaloneWidgetOpenEvent(ctx context.Context, svc *desktop.Service, rawPath string) (desktop.Widget, *desktop.Event, bool, error) {
	widget, ok := virtualDesktopStandaloneWidgetFromFile(rawPath)
	if !ok {
		return desktop.Widget{}, nil, false, nil
	}
	cleanPath := cleanVirtualDesktopSlashPath(rawPath)
	content, _, err := svc.ReadFile(ctx, cleanPath)
	if err != nil {
		return desktop.Widget{}, nil, true, err
	}
	if strings.TrimSpace(content) == "" {
		return desktop.Widget{}, nil, true, fmt.Errorf("desktop widget entry file is empty")
	}
	if err := svc.UpsertWidget(ctx, widget, desktop.SourceAgent); err != nil {
		return desktop.Widget{}, nil, true, err
	}
	event := virtualDesktopEvent("open_widget", map[string]interface{}{
		"widget_id": widget.ID,
		"path":      cleanPath,
		"title":     widget.Title,
		"icon":      widget.Icon,
	})
	return widget, event, true, nil
}

func virtualDesktopFindApp(ctx context.Context, svc *desktop.Service, appID string) (desktop.AppManifest, bool) {
	if svc == nil || strings.TrimSpace(appID) == "" {
		return desktop.AppManifest{}, false
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		return desktop.AppManifest{}, false
	}
	for _, app := range append(append([]desktop.AppManifest{}, bootstrap.BuiltinApps...), bootstrap.InstalledApps...) {
		if app.ID == appID {
			return app, true
		}
	}
	return desktop.AppManifest{}, false
}

func virtualDesktopDocument(args map[string]interface{}) (office.Document, error) {
	var doc office.Document
	if raw, ok := args["document"]; ok {
		if err := mapToStruct(raw, &doc); err != nil {
			return doc, fmt.Errorf("invalid document: %w", err)
		}
	}
	if doc.Text == "" {
		doc.Text = virtualDesktopString(args, "content", "text")
	}
	if doc.HTML == "" {
		doc.HTML = virtualDesktopString(args, "html")
	}
	if doc.Title == "" {
		doc.Title = virtualDesktopString(args, "title", "name")
	}
	if doc.Path == "" {
		doc.Path = virtualDesktopString(args, "path", "file_path")
	}
	return doc, nil
}

func virtualDesktopWorkbook(args map[string]interface{}) (office.Workbook, error) {
	if raw, ok := args["workbook"]; ok {
		return office.MarshalWorkbook(raw)
	}
	sheet := virtualDesktopString(args, "sheet", "sheet_name")
	if sheet == "" {
		sheet = "Sheet1"
	}
	workbook := office.Workbook{
		Path: virtualDesktopString(args, "path", "file_path"),
		Sheets: []office.Sheet{{
			Name: sheet,
		}},
	}
	return workbook, nil
}

func virtualDesktopEncodeWorkbookForPath(rawPath string, workbook office.Workbook) ([]byte, error) {
	switch strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(rawPath))) {
	case ".csv":
		return office.EncodeCSV(workbook, "")
	case ".xlsx", ".xlsm", ".xltx", ".xltm", "":
		return office.EncodeWorkbook(workbook)
	default:
		return nil, fmt.Errorf("unsupported workbook type %q", path.Ext(rawPath))
	}
}

func virtualDesktopExportOffice(sourceName string, data []byte, outputPath, format string) ([]byte, error) {
	outputExt := strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(outputPath)))
	if format != "" {
		outputExt = "." + format
	}
	if outputExt == "" {
		switch strings.ToLower(path.Ext(cleanVirtualDesktopSlashPath(sourceName))) {
		case ".xlsx", ".xlsm", ".xltx", ".xltm", ".csv":
			outputExt = ".xlsx"
		case ".docx", ".html", ".htm", ".md", ".txt", "":
			outputExt = ".docx"
		}
	}
	switch outputExt {
	case ".docx", ".html", ".htm", ".md", ".txt":
		doc, err := office.DecodeDocument(sourceName, data)
		if err != nil {
			return nil, err
		}
		exportName := sourceName
		if outputExt != "" {
			exportName = strings.TrimSuffix(sourceName, path.Ext(sourceName)) + outputExt
		}
		exported, _, err := office.EncodeDocument(exportName, doc)
		return exported, err
	case ".xlsx", ".xlsm", ".csv":
		workbook, err := office.DecodeWorkbook(sourceName, data)
		if err != nil {
			return nil, err
		}
		if outputExt == ".csv" {
			return office.EncodeCSV(workbook, "")
		}
		return office.EncodeWorkbook(workbook)
	default:
		return nil, fmt.Errorf("unsupported export format %q", strings.TrimPrefix(outputExt, "."))
	}
}

func isVirtualDesktopStandaloneWidgetHTML(rawPath string) bool {
	_, _, ok := virtualDesktopStandaloneWidgetPath(rawPath)
	return ok
}

func virtualDesktopStandaloneWidgetFromFile(rawPath string) (desktop.Widget, bool) {
	id, entry, ok := virtualDesktopStandaloneWidgetPath(rawPath)
	if !ok {
		return desktop.Widget{}, false
	}
	return desktop.Widget{
		ID:      id,
		Type:    desktop.WidgetTypeCustom,
		Title:   virtualDesktopTitleFromID(id),
		Icon:    desktop.InferDesktopIconName(id, entry),
		Entry:   entry,
		Runtime: desktop.AuraDesktopRuntime,
		W:       2,
		H:       2,
		Config:  map[string]interface{}{},
	}, true
}

func virtualDesktopStandaloneWidgetPath(rawPath string) (string, string, bool) {
	clean := cleanVirtualDesktopSlashPath(rawPath)
	if !strings.HasPrefix(clean, "Widgets/") || !strings.EqualFold(path.Ext(clean), ".html") {
		return "", "", false
	}
	rel := strings.TrimPrefix(clean, "Widgets/")
	dir := path.Dir(rel)
	entry := path.Base(rel)
	id := ""
	if dir == "." {
		id = strings.TrimSuffix(entry, path.Ext(entry))
	} else if strings.EqualFold(entry, "index.html") && path.Dir(dir) == "." {
		id = dir
		entry = path.Join(dir, entry)
	} else {
		return "", "", false
	}
	id = strings.ToLower(strings.TrimSpace(id))
	id = virtualDesktopWidgetIDSanitizer.ReplaceAllString(id, "-")
	id = strings.Trim(id, "_-")
	if len(id) < 2 {
		return "", "", false
	}
	return id, entry, true
}

func virtualDesktopRootHTMLAppFromFile(rawPath string) (desktop.AppManifest, string, bool) {
	clean := cleanVirtualDesktopSlashPath(rawPath)
	if path.Dir(clean) != "Apps" || !strings.EqualFold(path.Ext(clean), ".html") {
		return desktop.AppManifest{}, "", false
	}
	id := strings.TrimSuffix(path.Base(clean), path.Ext(clean))
	id = strings.ToLower(strings.TrimSpace(id))
	id = virtualDesktopWidgetIDSanitizer.ReplaceAllString(id, "-")
	id = strings.Trim(id, "_-")
	if len(id) < 2 {
		return desktop.AppManifest{}, "", false
	}
	title := virtualDesktopTitleFromID(id)
	entry := "index.html"
	return desktop.AppManifest{
		ID:          id,
		Name:        title,
		Version:     "1.0.0",
		Icon:        desktop.InferDesktopIconName(id, title, entry),
		Entry:       entry,
		Runtime:     desktop.AuraDesktopRuntime,
		Description: "Generated desktop app.",
	}, path.Join("Apps", id, entry), true
}

func cleanVirtualDesktopSlashPath(rawPath string) string {
	p := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if p == "" {
		return "."
	}
	return path.Clean(p)
}

func rewriteVirtualDesktopPrinterCameraURLsForPath(cfg *config.Config, rawPath, content string) string {
	clean := cleanVirtualDesktopSlashPath(rawPath)
	if !strings.HasPrefix(clean, "Apps/") && !strings.HasPrefix(clean, "Widgets/") {
		return content
	}
	return RewriteVirtualDesktopPrinterCameraURLs(cfg, content)
}

func rewriteVirtualDesktopPrinterCameraURLsForFiles(cfg *config.Config, files map[string]string) map[string]string {
	if len(files) == 0 {
		return files
	}
	rewritten := make(map[string]string, len(files))
	for filePath, content := range files {
		rewritten[filePath] = RewriteVirtualDesktopPrinterCameraURLs(cfg, content)
	}
	return rewritten
}

// RewriteVirtualDesktopPrinterCameraURLs maps known configured printer camera
// stream URLs to AuraGo's same-origin proxy for generated desktop HTML/JS.
func RewriteVirtualDesktopPrinterCameraURLs(cfg *config.Config, content string) string {
	if cfg == nil || content == "" || !cfg.ThreeDPrinters.Enabled || !cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled {
		return content
	}
	rewritten := content
	for _, printer := range cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers {
		printerID := strings.TrimSpace(printer.ID)
		host := virtualDesktopPrinterHost(printer.URL)
		if printerID == "" || host == "" {
			continue
		}
		proxyURL := "/api/3d-printers/" + url.PathEscape(printerID) + "/camera/stream"
		for _, candidate := range []string{
			"http://" + host + ":3031/video",
			"https://" + host + ":3031/video",
			"//" + host + ":3031/video",
			host + ":3031/video",
		} {
			rewritten = strings.ReplaceAll(rewritten, candidate, proxyURL)
		}
	}
	return rewritten
}

func virtualDesktopPrinterHost(rawURL string) string {
	s := strings.TrimSpace(rawURL)
	if s == "" {
		return ""
	}
	parseInput := s
	if !strings.Contains(parseInput, "://") {
		parseInput = "ws://" + parseInput
	}
	parsed, err := url.Parse(parseInput)
	if err == nil && strings.TrimSpace(parsed.Hostname()) != "" {
		return strings.TrimSpace(parsed.Hostname())
	}
	host := strings.TrimPrefix(s, "//")
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, ":"); i > -1 {
		host = host[:i]
	}
	return strings.TrimSpace(host)
}

func virtualDesktopTitleFromID(id string) string {
	parts := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(id))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return id
	}
	return strings.Join(parts, " ")
}

func virtualDesktopFiles(args map[string]interface{}) (map[string]string, error) {
	files := map[string]string{}
	if raw, ok := args["files"]; ok {
		switch typed := raw.(type) {
		case map[string]string:
			for k, v := range typed {
				files[k] = v
			}
		case map[string]interface{}:
			for k, v := range typed {
				files[k] = fmt.Sprint(v)
			}
		default:
			return nil, fmt.Errorf("files must be an object of path to content")
		}
	}
	if len(files) == 0 {
		path := virtualDesktopString(args, "path", "file_path")
		content := virtualDesktopString(args, "content")
		if path != "" {
			files[path] = content
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("files are required")
	}
	return files, nil
}

func mapToStruct(raw interface{}, target interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
