package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
)

type VirtualDesktopExecution struct {
	Output string
	Event  *desktop.Event
}

var virtualDesktopWidgetIDSanitizer = regexp.MustCompile(`[^a-z0-9_-]+`)

const virtualDesktopCodeStudioWorkspaceRoot = "/workspace"
const virtualDesktopLargeReadLimitBytes = 8 * 1024

var (
	toolDesktopMu               sync.Mutex
	toolDesktopSvc              *desktop.Service
	toolDesktopCfg              desktop.Config
	toolDesktopIntegritySecrets desktop.IntegritySecretStore
)

// SetToolDesktopService registers a shared desktop.Service instance for reuse
// by ExecuteVirtualDesktop.  Pass nil to clear the cached instance.
func SetToolDesktopService(svc *desktop.Service) {
	toolDesktopMu.Lock()
	defer toolDesktopMu.Unlock()
	// Only store the pointer. Lifecycle (Close) is managed by the creator
	// (server getDesktopService on config change, or explicit shutdown).
	// This avoids cross-test interference when many short-lived *Server
	// instances exist in the same process during parallel tests.
	toolDesktopSvc = svc
	if svc != nil {
		svc.SetIntegritySecretStore(toolDesktopIntegritySecrets)
		toolDesktopCfg = svc.Config()
	} else {
		toolDesktopCfg = desktop.Config{}
	}
}

// SetToolDesktopIntegritySecretStore registers the vault-backed signer store
// used by transient virtual desktop services.
func SetToolDesktopIntegritySecretStore(store desktop.IntegritySecretStore) {
	toolDesktopMu.Lock()
	defer toolDesktopMu.Unlock()
	toolDesktopIntegritySecrets = store
	if toolDesktopSvc != nil {
		toolDesktopSvc.SetIntegritySecretStore(store)
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
	svc.SetIntegritySecretStore(toolDesktopIntegritySecrets)
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
		payload := map[string]interface{}{"entry": entry, "content": content}
		if len(content) > virtualDesktopLargeReadLimitBytes {
			payload = virtualDesktopLargeReadPayload(entry, content)
		}
		return virtualDesktopJSON("ok", "desktop file read", payload, nil)
	case "search_file":
		path := virtualDesktopString(args, "path", "file_path")
		query := virtualDesktopString(args, "query", "pattern", "search")
		if strings.TrimSpace(path) == "" || strings.TrimSpace(query) == "" {
			return virtualDesktopJSON("error", "path and query are required", nil, nil)
		}
		content, entry, err := svc.ReadFile(ctx, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		matches := virtualDesktopSearchText(content, query, virtualDesktopBool(args, "case_sensitive"), virtualDesktopInt(args, 8, "max_matches"), virtualDesktopInt(args, 2, "context_lines"))
		return virtualDesktopJSON("ok", "desktop file searched", map[string]interface{}{
			"path":         entry.Path,
			"query":        query,
			"match_count":  len(matches),
			"matches":      matches,
			"editing_hint": "Use search_file/read_file_excerpt to locate anchors yourself, then use virtual_desktop.patch_file with exact replacements. Do not ask the user for block anchors when the desktop file was truncated.",
		}, nil)
	case "read_file_excerpt":
		path := virtualDesktopString(args, "path", "file_path")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		content, entry, err := svc.ReadFile(ctx, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		lineStart := virtualDesktopInt(args, 1, "line_start", "start_line")
		lineCount := virtualDesktopInt(args, 80, "line_count", "lines")
		excerpt, lineEnd, totalLines := virtualDesktopLineExcerpt(content, lineStart, lineCount)
		return virtualDesktopJSON("ok", "desktop file excerpt read", map[string]interface{}{
			"path":         entry.Path,
			"line_start":   lineStart,
			"line_end":     lineEnd,
			"line_count":   lineCount,
			"total_lines":  totalLines,
			"excerpt":      excerpt,
			"editing_hint": "Use this excerpt to construct exact virtual_desktop.patch_file replacements; do not request anchors from the user.",
		}, nil)
	case "write_file":
		path := virtualDesktopString(args, "path", "file_path")
		content, hasContent := virtualDesktopRawString(args, "content")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		cleanPath := cleanVirtualDesktopSlashPath(path)
		if !hasContent || strings.TrimSpace(content) == "" {
			if isVirtualDesktopGeneratedRuntimePath(cleanPath) {
				return virtualDesktopJSON("error", "content is required for write_file on Apps/ or Widgets/ paths; refusing to overwrite a runnable desktop file with empty content. Use patch_file for edits or delete for intentional removal.", nil, nil)
			}
			if !virtualDesktopBool(args, "allow_empty", "allow_empty_content") {
				return virtualDesktopJSON("error", "content is required for write_file; refusing to overwrite the desktop file with empty content. Set allow_empty=true only when intentionally creating an empty non-app file.", nil, nil)
			}
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
	case "patch_file":
		path := virtualDesktopString(args, "path", "file_path")
		if strings.TrimSpace(path) == "" {
			return virtualDesktopJSON("error", "path is required", nil, nil)
		}
		content, entry, err := svc.ReadFile(ctx, path)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		nextContent, replacements, operations, err := virtualDesktopApplyTextPatch(content, args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		if operations == 0 {
			return virtualDesktopJSON("error", "patch_file requires replacements, prepend_text, or append_text", nil, nil)
		}
		if err := svc.WriteFile(ctx, entry.Path, nextContent, desktop.SourceAgent); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		if widget, ok := virtualDesktopStandaloneWidgetFromFile(entry.Path); ok && strings.TrimSpace(nextContent) != "" {
			if err := svc.UpsertWidget(ctx, widget, desktop.SourceAgent); err != nil {
				return virtualDesktopJSON("error", err.Error(), nil, nil)
			}
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "path": entry.Path})
		return virtualDesktopJSON("ok", "desktop file patched", map[string]interface{}{
			"path":               entry.Path,
			"replacements":       replacements,
			"applied_operations": operations,
			"size":               len(nextContent),
		}, event)
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
		codeStudioPathIgnored := false
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
		if appID == "" && filePath != "" {
			if app, ok := virtualDesktopFindGeneratedAppByEntryPath(ctx, svc, filePath); ok {
				appID = app.ID
			}
		}
		if appID == "" {
			return virtualDesktopJSON("error", "app_id is required", nil, nil)
		}
		if strings.EqualFold(appID, "code-studio") && filePath != "" {
			cleanCodePath, ok := normalizeVirtualDesktopCodeStudioOpenPath(filePath)
			if !ok {
				filePath = ""
				codeStudioPathIgnored = true
			} else {
				filePath = cleanCodePath
			}
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
		if codeStudioPathIgnored {
			payload["path_ignored"] = true
			payload["path_policy"] = "code_studio_paths_must_be_inside_workspace"
		}
		event := virtualDesktopEvent("open_app", payload)
		message := "desktop app open event emitted"
		if codeStudioPathIgnored {
			message = "desktop app open event emitted; ignored path outside Code Studio workspace"
		}
		return virtualDesktopJSON("ok", message, payload, event)
	case "list_apps":
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		allApps := append([]desktop.AppManifest{}, payload.BuiltinApps...)
		allApps = append(allApps, payload.InstalledApps...)
		return virtualDesktopJSON("ok", "desktop apps listed", map[string]interface{}{
			"builtin_apps":   payload.BuiltinApps,
			"installed_apps": payload.InstalledApps,
			"all_apps":       allApps,
			"counts": map[string]interface{}{
				"builtin":   len(payload.BuiltinApps),
				"installed": len(payload.InstalledApps),
				"total":     len(allApps),
			},
		}, nil)
	case "get_app":
		appID := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "app_id", "id")))
		if appID == "" {
			return virtualDesktopJSON("error", "app_id is required", nil, nil)
		}
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		for _, app := range payload.BuiltinApps {
			if strings.EqualFold(app.ID, appID) {
				return virtualDesktopJSON("ok", "desktop app found", map[string]interface{}{
					"app":    app,
					"found":  true,
					"source": "builtin",
				}, nil)
			}
		}
		for _, app := range payload.InstalledApps {
			if strings.EqualFold(app.ID, appID) {
				return virtualDesktopJSON("ok", "desktop app found", map[string]interface{}{
					"app":    app,
					"found":  true,
					"source": "installed",
				}, nil)
			}
		}
		return virtualDesktopJSON("error", fmt.Sprintf("desktop app %q not found", appID), map[string]interface{}{
			"code":   "desktop_app_not_found",
			"app_id": appID,
		}, nil)
	case "list_widgets":
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		widgets := payload.AllWidgets
		if len(widgets) == 0 {
			widgets = payload.Widgets
		}
		visibleWidgets := []desktop.Widget{}
		for _, w := range widgets {
			if w.Visible {
				visibleWidgets = append(visibleWidgets, w)
			}
		}
		return virtualDesktopJSON("ok", "desktop widgets listed", map[string]interface{}{
			"widgets":         widgets,
			"visible_widgets": visibleWidgets,
			"counts": map[string]interface{}{
				"total":   len(widgets),
				"visible": len(visibleWidgets),
			},
		}, nil)
	case "get_widget":
		widgetID := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "widget_id", "id")))
		if widgetID == "" {
			return virtualDesktopJSON("error", "widget_id is required", nil, nil)
		}
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		widgets := payload.AllWidgets
		if len(widgets) == 0 {
			widgets = payload.Widgets
		}
		for _, w := range widgets {
			if strings.EqualFold(w.ID, widgetID) {
				return virtualDesktopJSON("ok", "desktop widget found", map[string]interface{}{
					"widget":  w,
					"found":   true,
					"visible": w.Visible,
				}, nil)
			}
		}
		return virtualDesktopJSON("error", fmt.Sprintf("desktop widget %q not found", widgetID), map[string]interface{}{
			"code":      "desktop_widget_not_found",
			"widget_id": widgetID,
		}, nil)
	case "diagnose_app":
		appID := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "app_id", "id")))
		if appID == "" {
			return virtualDesktopJSON("error", "app_id is required", nil, nil)
		}
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		var app desktop.AppManifest
		appFound := false
		for _, a := range payload.BuiltinApps {
			if strings.EqualFold(a.ID, appID) {
				app = a
				appFound = true
				break
			}
		}
		if !appFound {
			for _, a := range payload.InstalledApps {
				if strings.EqualFold(a.ID, appID) {
					app = a
					appFound = true
					break
				}
			}
		}
		if !appFound {
			return virtualDesktopJSON("error", fmt.Sprintf("desktop app %q not found", appID), map[string]interface{}{
				"code":   "desktop_app_not_found",
				"app_id": appID,
			}, nil)
		}
		checks := []map[string]interface{}{}
		recommendations := []string{}
		if app.Builtin {
			checks = append(checks, map[string]interface{}{"check": "builtin", "ok": true, "detail": "built-in apps are always healthy"})
			return virtualDesktopJSON("ok", "desktop app diagnosed", map[string]interface{}{
				"app_id":          appID,
				"ok":              true,
				"builtin":         true,
				"checks":          checks,
				"health":          app.Health,
				"health_reason":   app.HealthReason,
				"recommendations": recommendations,
			}, nil)
		}
		entryPath := app.EntryPath
		if entryPath == "" {
			entryPath = path.Join("Apps", app.ID, app.Entry)
		}
		content, _, readErr := svc.ReadFile(ctx, entryPath)
		ok := readErr == nil && strings.TrimSpace(content) != ""
		checks = append(checks, map[string]interface{}{
			"check":      "entry_readable",
			"ok":         readErr == nil,
			"entry_path": entryPath,
			"detail":     virtualDesktopErrDetail(readErr),
		})
		if readErr == nil {
			if strings.TrimSpace(content) == "" {
				checks = append(checks, map[string]interface{}{"check": "entry_nonempty", "ok": false, "detail": "entry file is empty"})
				recommendations = append(recommendations, "Reinstall or rewrite the app entry file")
			} else {
				checks = append(checks, map[string]interface{}{"check": "entry_nonempty", "ok": true, "detail": "entry file has content"})
			}
		} else {
			recommendations = append(recommendations, "Reinstall or rewrite the app entry file")
		}
		return virtualDesktopJSON("ok", "desktop app diagnosed", map[string]interface{}{
			"app_id":          appID,
			"ok":              ok,
			"builtin":         false,
			"checks":          checks,
			"health":          app.Health,
			"health_reason":   app.HealthReason,
			"entry_path":      entryPath,
			"recommendations": recommendations,
		}, nil)
	case "diagnose_widget":
		widgetID := strings.ToLower(strings.TrimSpace(virtualDesktopString(args, "widget_id", "id")))
		if widgetID == "" {
			return virtualDesktopJSON("error", "widget_id is required", nil, nil)
		}
		payload, err := svc.Bootstrap(ctx)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		widgets := payload.AllWidgets
		if len(widgets) == 0 {
			widgets = payload.Widgets
		}
		var widget desktop.Widget
		widgetFound := false
		for _, w := range widgets {
			if strings.EqualFold(w.ID, widgetID) {
				widget = w
				widgetFound = true
				break
			}
		}
		if !widgetFound {
			return virtualDesktopJSON("error", fmt.Sprintf("desktop widget %q not found", widgetID), map[string]interface{}{
				"code":      "desktop_widget_not_found",
				"widget_id": widgetID,
			}, nil)
		}
		checks := []map[string]interface{}{}
		recommendations := []string{}
		if widget.Entry == "" {
			checks = append(checks, map[string]interface{}{
				"check":          "has_entry",
				"ok":             true,
				"entry_required": false,
				"detail":         "widget has no entry file",
			})
			return virtualDesktopJSON("ok", "desktop widget diagnosed", map[string]interface{}{
				"widget_id":       widgetID,
				"ok":              true,
				"widget":          widget,
				"checks":          checks,
				"recommendations": recommendations,
			}, nil)
		}
		var entryPath string
		if widget.AppID != "" {
			entryPath = path.Join("Apps", widget.AppID, widget.Entry)
		} else {
			entryPath = path.Join("Widgets", widget.Entry)
		}
		content, _, readErr := svc.ReadFile(ctx, entryPath)
		checks = append(checks, map[string]interface{}{
			"check":      "entry_readable",
			"ok":         readErr == nil,
			"entry_path": entryPath,
			"detail":     virtualDesktopErrDetail(readErr),
		})
		ok := readErr == nil && strings.TrimSpace(content) != ""
		if readErr == nil {
			if strings.TrimSpace(content) == "" {
				checks = append(checks, map[string]interface{}{"check": "entry_nonempty", "ok": false, "detail": "widget entry file is empty"})
				recommendations = append(recommendations, "Rewrite the widget HTML with non-empty content")
			} else {
				checks = append(checks, map[string]interface{}{"check": "entry_nonempty", "ok": true, "detail": "widget entry file has content"})
			}
		} else {
			ok = false
			recommendations = append(recommendations, "Ensure the widget entry file exists")
		}
		standalone := widget.AppID == ""
		return virtualDesktopJSON("ok", "desktop widget diagnosed", map[string]interface{}{
			"widget_id":       widgetID,
			"ok":              ok,
			"widget":          widget,
			"entry_path":      entryPath,
			"app_id":          widget.AppID,
			"standalone":      standalone,
			"app_backed":      !standalone,
			"checks":          checks,
			"recommendations": recommendations,
		}, nil)
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

func virtualDesktopRawString(args map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case string:
			return v, true
		case fmt.Stringer:
			return v.String(), true
		case nil:
			return "", true
		default:
			return fmt.Sprint(raw), true
		}
	}
	return "", false
}

func virtualDesktopInt(args map[string]interface{}, fallback int, keys ...string) int {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return int(i)
			}
		case string:
			if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return i
			}
		}
	}
	return fallback
}

func virtualDesktopBool(args map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		raw, ok := args[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(strings.TrimSpace(v), "true") || strings.EqualFold(strings.TrimSpace(v), "yes") || strings.TrimSpace(v) == "1"
		}
	}
	return false
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

func virtualDesktopFindGeneratedAppByEntryPath(ctx context.Context, svc *desktop.Service, rawPath string) (desktop.AppManifest, bool) {
	if svc == nil {
		return desktop.AppManifest{}, false
	}
	cleanPath := cleanVirtualDesktopSlashPath(rawPath)
	if cleanPath == "" {
		return desktop.AppManifest{}, false
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		return desktop.AppManifest{}, false
	}
	for _, app := range bootstrap.InstalledApps {
		entryPath := app.EntryPath
		if entryPath == "" {
			entryPath = path.Join("Apps", app.ID, app.Entry)
		}
		if cleanVirtualDesktopSlashPath(entryPath) == cleanPath {
			return app, true
		}
	}
	return desktop.AppManifest{}, false
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

func isVirtualDesktopGeneratedRuntimePath(cleanPath string) bool {
	return strings.HasPrefix(cleanPath, "Apps/") || strings.HasPrefix(cleanPath, "Widgets/")
}

func normalizeVirtualDesktopCodeStudioOpenPath(rawPath string) (string, bool) {
	p := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if p == "" {
		return "", true
	}
	if strings.ContainsRune(p, 0) {
		return "", false
	}
	p = strings.TrimPrefix(p, "file://")
	lower := strings.ToLower(p)
	if strings.HasPrefix(p, "~") || strings.HasPrefix(lower, "/home/") || strings.HasPrefix(lower, "/users/") || strings.HasPrefix(lower, "/var/") || strings.HasPrefix(lower, "/tmp/") {
		return "", false
	}
	if len(p) >= 3 && p[1] == ':' && p[2] == '/' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) {
		return "", false
	}
	if strings.EqualFold(p, "workspace") || strings.HasPrefix(strings.ToLower(p), "workspace/") {
		p = "/" + p
	} else if !strings.HasPrefix(p, "/") {
		p = path.Join(virtualDesktopCodeStudioWorkspaceRoot, strings.TrimPrefix(p, "./"))
	}
	cleaned := path.Clean(p)
	if cleaned != virtualDesktopCodeStudioWorkspaceRoot && !strings.HasPrefix(cleaned, virtualDesktopCodeStudioWorkspaceRoot+"/") {
		return "", false
	}
	return cleaned, true
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

func virtualDesktopErrDetail(err error) interface{} {
	if err == nil {
		return nil
	}
	return err.Error()
}

func mapToStruct(raw interface{}, target interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
