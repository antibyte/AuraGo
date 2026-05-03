package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
)

type VirtualDesktopExecution struct {
	Output string
	Event  *desktop.Event
}

var virtualDesktopWidgetIDSanitizer = regexp.MustCompile(`[^a-z0-9_-]+`)

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
	svc, err := desktop.NewService(desktop.ConfigFromAuraConfig(cfg))
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}
	defer svc.Close()
	if err := svc.Init(ctx); err != nil {
		return virtualDesktopJSON("error", err.Error(), nil, nil)
	}

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
	case "install_app":
		manifest, err := virtualDesktopManifest(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		files, err := virtualDesktopFiles(args)
		if err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
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
		if err := svc.UpsertWidget(ctx, widget, desktop.SourceAgent); err != nil {
			return virtualDesktopJSON("error", err.Error(), nil, nil)
		}
		event := virtualDesktopEvent("desktop_changed", map[string]interface{}{"operation": op, "widget_id": widget.ID})
		return virtualDesktopJSON("ok", "desktop widget saved", map[string]interface{}{"widget_id": widget.ID}, event)
	case "open_app":
		appID := virtualDesktopString(args, "app_id", "id")
		if appID == "" {
			return virtualDesktopJSON("error", "app_id is required", nil, nil)
		}
		event := virtualDesktopEvent("open_app", map[string]interface{}{"app_id": appID})
		return virtualDesktopJSON("ok", "desktop app open event emitted", map[string]interface{}{"app_id": appID}, event)
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

func isVirtualDesktopStandaloneWidgetHTML(rawPath string) bool {
	clean := cleanVirtualDesktopSlashPath(rawPath)
	return path.Dir(clean) == "Widgets" && strings.EqualFold(path.Ext(clean), ".html")
}

func virtualDesktopStandaloneWidgetFromFile(rawPath string) (desktop.Widget, bool) {
	if !isVirtualDesktopStandaloneWidgetHTML(rawPath) {
		return desktop.Widget{}, false
	}
	clean := cleanVirtualDesktopSlashPath(rawPath)
	entry := path.Base(clean)
	id := strings.TrimSuffix(entry, path.Ext(entry))
	id = strings.ToLower(strings.TrimSpace(id))
	id = virtualDesktopWidgetIDSanitizer.ReplaceAllString(id, "_")
	id = strings.Trim(id, "_-")
	if len(id) < 2 {
		return desktop.Widget{}, false
	}
	return desktop.Widget{
		ID:      id,
		Type:    desktop.WidgetTypeCustom,
		Title:   virtualDesktopTitleFromID(id),
		Icon:    "widgets",
		Entry:   entry,
		Runtime: desktop.AuraDesktopRuntime,
		W:       2,
		H:       2,
		Config:  map[string]interface{}{},
	}, true
}

func cleanVirtualDesktopSlashPath(rawPath string) string {
	p := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if p == "" {
		return "."
	}
	return path.Clean(p)
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
