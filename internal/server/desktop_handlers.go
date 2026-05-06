package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/desktop"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

var desktopWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	},
}

func (s *Server) getDesktopService(ctx context.Context) (*desktop.Service, *desktop.Hub, error) {
	s.CfgMu.RLock()
	cfgSnapshot := *s.Cfg
	s.CfgMu.RUnlock()
	desktopCfg := desktop.ConfigFromAuraConfig(&cfgSnapshot)
	if !desktopCfg.Enabled {
		return nil, nil, fmt.Errorf("virtual desktop is disabled")
	}

	s.DesktopMu.Lock()
	defer s.DesktopMu.Unlock()
	if s.DesktopService != nil && s.DesktopService.Config() != desktopCfg {
		_ = s.DesktopService.Close()
		s.DesktopService = nil
		s.DesktopHub = nil
	}
	if s.DesktopService == nil {
		svc, err := desktop.NewService(desktopCfg)
		if err != nil {
			return nil, nil, err
		}
		if err := svc.Init(ctx); err != nil {
			_ = svc.Close()
			return nil, nil, err
		}
		if codeContainer := svc.CodeContainer(); codeContainer != nil {
			codeContainer.SetDockerClient(newCodeStudioDockerAdapter(desktopCfg, s.Logger))
		}
		s.DesktopService = svc
		s.DesktopHub = desktop.NewHub(desktopCfg.MaxWSClients)
	}
	return s.DesktopService, s.DesktopHub, nil
}

func (s *Server) disabledDesktopBootstrap() desktop.BootstrapPayload {
	s.CfgMu.RLock()
	cfgSnapshot := *s.Cfg
	s.CfgMu.RUnlock()
	desktopCfg := desktop.ConfigFromAuraConfig(&cfgSnapshot)
	settings := desktop.DesktopSettingDefaults()
	return desktop.BootstrapPayload{
		Enabled:            false,
		ReadOnly:           desktopCfg.ReadOnly,
		AllowAgentControl:  desktopCfg.AllowAgentControl,
		AllowGeneratedApps: desktopCfg.AllowGeneratedApps,
		AllowPythonJobs:    desktopCfg.AllowPythonJobs,
		ControlLevel:       desktopCfg.ControlLevel,
		Workspace: desktop.WorkspaceInfo{
			Root:        desktopCfg.WorkspaceDir,
			Directories: desktop.DefaultDirectories(),
			MaxFileSize: int64(desktopCfg.MaxFileSizeMB) * 1024 * 1024,
		},
		BuiltinApps: desktop.BuiltinApps(),
		Shortcuts: []desktop.Shortcut{
			{ID: "app-files", TargetType: desktop.ShortcutTargetApp, TargetID: "files", Name: "Files", Icon: "folder"},
			{ID: "dir-Trash", TargetType: desktop.ShortcutTargetDirectory, Path: "Trash", Name: "Trash", Icon: "trash"},
		},
		Settings:    settings,
		IconCatalog: desktop.DesktopIconCatalog(settings),
	}
}

func handleDesktopBootstrap(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		var payload desktop.BootstrapPayload
		if err != nil {
			payload = s.disabledDesktopBootstrap()
		} else if payload, err = svc.Bootstrap(r.Context()); err != nil {
			jsonError(w, "Failed to load desktop", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func handleDesktopFiles(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		query := r.URL.Query()
		if query.Get("recursive") == "true" {
			offset := parseDesktopFilesInt(query.Get("offset"), 0)
			limit := parseDesktopFilesInt(query.Get("limit"), 500)
			if limit > 1000 {
				limit = 1000
			}
			files, hasMore, err := svc.ListFilesRecursive(r.Context(), query.Get("path"), offset, limit)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "files": files, "offset": offset, "limit": limit, "has_more": hasMore})
			return
		}
		files, err := svc.ListFiles(r.Context(), query.Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "files": files})
	}
}

func parseDesktopFilesInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func handleDesktopFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			content, entry, err := svc.ReadFile(r.Context(), r.URL.Query().Get("path"))
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "entry": entry, "content": content})
		case http.MethodPut:
			var body struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Path == "" {
				body.Path = r.URL.Query().Get("path")
			}
			if err := svc.WriteFile(r.Context(), body.Path, body.Content, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": body.Path}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodPatch:
			var body struct {
				OldPath string `json:"old_path"`
				NewPath string `json:"new_path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if err := svc.MovePath(r.Context(), body.OldPath, body.NewPath, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "move_path", "old_path": body.OldPath, "new_path": body.NewPath}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodDelete:
			path := r.URL.Query().Get("path")
			if err := svc.DeletePath(r.Context(), path, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_path", "path": path}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopDirectory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := svc.CreateDirectory(r.Context(), body.Path, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "create_directory", "path": body.Path}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopCopy(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		var body struct {
			SourcePath string `json:"source_path"`
			DestPath   string `json:"dest_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.SourcePath == "" || body.DestPath == "" {
			jsonError(w, "source_path and dest_path are required", http.StatusBadRequest)
			return
		}
		if err := svc.CopyPath(r.Context(), body.SourcePath, body.DestPath, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "copy_path", "source_path": body.SourcePath, "dest_path": body.DestPath}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopPreview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		file, entry, mimeType, err := svc.OpenPreviewFile(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		if !isDesktopPreviewImageMIME(mimeType) {
			jsonError(w, "desktop preview only supports image files", http.StatusUnsupportedMediaType)
			return
		}
		maxSize := int64(svc.Config().MaxFileSizeMB) * 1024 * 1024
		if maxSize <= 0 {
			maxSize = 50 * 1024 * 1024
		}
		if entry.Size > maxSize {
			jsonError(w, "desktop preview file exceeds max size", http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=120")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(entry.Name, `"`, "")))
		http.ServeContent(w, r, entry.Name, entry.ModTime, file)
	}
}

func isDesktopPreviewImageMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/avif", "image/bmp", "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func handleDesktopUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		maxSize := int64(svc.Config().MaxFileSizeMB) * 1024 * 1024
		if maxSize <= 0 {
			maxSize = 50 * 1024 * 1024
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxSize+1024)
		if err := r.ParseMultipartForm(maxSize); err != nil {
			jsonError(w, "File too large or invalid form data", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "Missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()
		destDir := r.FormValue("path")
		if destDir == "" {
			destDir = ""
		}
		content, readErr := io.ReadAll(file)
		if readErr != nil {
			jsonError(w, "Failed to read upload", http.StatusBadRequest)
			return
		}
		destPath := strings.TrimRight(destDir, "/") + "/" + header.Filename
		if err := svc.WriteFileBytes(r.Context(), destPath, content, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "upload_file", "path": destPath}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": destPath})
	}
}

func handleDesktopDownload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		data, entry, err := svc.ReadFileBytes(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		mimeType := entry.MIMEType
		if mimeType == "" {
			mimeType = desktop.MIMETypeForName(entry.Name)
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(entry.Name, `"`, "")))
		http.ServeContent(w, r, entry.Name, entry.ModTime, bytes.NewReader(data))
	}
}

func handleDesktopApps(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodDelete {
			id := r.URL.Query().Get("id")
			if err := svc.DeleteApp(r.Context(), id, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_app", "app_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Manifest desktop.AppManifest `json:"manifest"`
			Files    map[string]string   `json:"files"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := svc.InstallApp(r.Context(), body.Manifest, body.Files, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "install_app", "app_id": body.Manifest.ID}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopShortcuts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body struct {
				AppID string `json:"app_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if err := svc.AddDesktopAppShortcut(r.Context(), body.AppID, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "add_shortcut", "app_id": body.AppID}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if err := svc.RemoveDesktopShortcut(r.Context(), id, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "remove_shortcut", "shortcut_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopWidgets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodGet {
			allWidgets, err := svc.ListAllWidgets(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(allWidgets)
			return
		}
		if r.Method == http.MethodDelete {
			id := r.URL.Query().Get("id")
			if err := svc.DeleteWidget(r.Context(), id, desktop.SourceUser); err != nil {
				status := http.StatusBadRequest
				if strings.Contains(err.Error(), "built-in") {
					status = http.StatusForbidden
				}
				jsonError(w, err.Error(), status)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_widget", "widget_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method == http.MethodPatch {
			id := r.URL.Query().Get("id")
			var body struct {
				Visible *bool `json:"visible"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Visible == nil {
				jsonError(w, "visible field is required", http.StatusBadRequest)
				return
			}
			if err := svc.SetWidgetVisible(r.Context(), id, *body.Visible, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_widget_visible", "widget_id": id, "visible": *body.Visible}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var widget desktop.Widget
		if err := json.NewDecoder(r.Body).Decode(&widget); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := svc.UpsertWidget(r.Context(), widget, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "upsert_widget", "widget_id": widget.ID}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopSettings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			payload, err := svc.Bootstrap(r.Context())
			if err != nil {
				jsonError(w, "Failed to load desktop settings", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "settings": payload.Settings})
		case http.MethodPut:
			var body struct {
				Key      string            `json:"key"`
				Value    string            `json:"value"`
				Settings map[string]string `json:"settings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Settings == nil {
				body.Settings = map[string]string{body.Key: body.Value}
			}
			if err := svc.SetSettings(r.Context(), body.Settings, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload, err := svc.Bootstrap(r.Context())
			if err != nil {
				jsonError(w, "Failed to load desktop settings", http.StatusInternalServerError)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_settings", "settings": body.Settings}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "settings": payload.Settings})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopEmbedToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
		if rawPath == "" {
			jsonError(w, "path is required", http.StatusBadRequest)
			return
		}
		if _, err := svc.ResolvePath(rawPath); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		normalizedPath, err := normalizeDesktopEmbedPath(rawPath)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		now := time.Now().UTC()
		token := ""
		if authEnabled {
			var err error
			token, err = issueDesktopEmbedToken(secret, normalizedPath, now)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"token":      token,
			"expires_at": now.Add(desktopEmbedTokenTTL).Format(time.RFC3339Nano),
		})
	}
}

func handleDesktopChat(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Message string             `json:"message"`
			Context desktopChatContext `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Message = strings.TrimSpace(body.Message)
		if body.Message == "" {
			jsonError(w, "Message is required", http.StatusBadRequest)
			return
		}
		answer := runDesktopAgentChat(s, body.Message, body.Context)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "answer": answer})
	}
}

func handleDesktopChatStream(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Message string             `json:"message"`
			Context desktopChatContext `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Message = strings.TrimSpace(body.Message)
		if body.Message == "" {
			jsonError(w, "Message is required", http.StatusBadRequest)
			return
		}

		flusher, canFlush := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		if s == nil || s.Cfg == nil {
			sseWriteData(w, "error", "server not configured")
			return
		}

		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		sessionID := "virtual-desktop"
		runCfg := agent.RunConfig{
			Config:             &cfg,
			Logger:             s.Logger,
			LLMClient:          s.LLMClient,
			ShortTermMem:       s.ShortTermMem,
			HistoryManager:     s.HistoryManager,
			LongTermMem:        s.LongTermMem,
			KG:                 s.KG,
			InventoryDB:        s.InventoryDB,
			InvasionDB:         s.InvasionDB,
			CheatsheetDB:       s.CheatsheetDB,
			ImageGalleryDB:     s.ImageGalleryDB,
			MediaRegistryDB:    s.MediaRegistryDB,
			HomepageRegistryDB: s.HomepageRegistryDB,
			ContactsDB:         s.ContactsDB,
			PlannerDB:          s.PlannerDB,
			SQLConnectionsDB:   s.SQLConnectionsDB,
			SQLConnectionPool:  s.SQLConnectionPool,
			RemoteHub:          s.RemoteHub,
			Vault:              s.Vault,
			Registry:           s.Registry,
			Manifest:           tools.NewManifest(cfg.Directories.ToolsDir),
			CronManager:        s.CronManager,
			MissionManagerV2:   s.MissionManagerV2,
			CoAgentRegistry:    s.CoAgentRegistry,
			BudgetTracker:      s.BudgetTracker,
			DaemonSupervisor:   s.DaemonSupervisor,
			LLMGuardian:        s.LLMGuardian,
			PreparationService: s.PreparationService,
			SessionID:          sessionID,
			MessageSource:      "virtual_desktop_chat",
			VoiceOutputActive:  GetSpeakerMode(),
		}
		prompt := buildDesktopAgentPrompt(body.Message, body.Context)
		broker := &desktopStreamBroker{
			w:        w,
			flusher:  flusher,
			canFlush: canFlush,
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			sseBroker := NewSSEBrokerAdapterWithSession(s.SSE, sessionID)
			combinedBroker := &desktopStreamCombinedBroker{
				stream: broker,
				sse:    sseBroker,
			}
			agent.Loopback(runCfg, prompt, combinedBroker)
		}()
		select {
		case <-done:
		case <-ctx.Done():
		}
		broker.mu.Lock()
		broker.closed = true
		broker.mu.Unlock()
		sseWriteDone(w)
		if canFlush {
			flusher.Flush()
		}
	}
}

type desktopStreamBroker struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	canFlush bool
	mu       sync.Mutex
	closed   bool
}

type desktopStreamCombinedBroker struct {
	stream *desktopStreamBroker
	sse    *SSEBrokerAdapter
}

func (b *desktopStreamCombinedBroker) Send(event, message string) {
	b.sse.Send(event, message)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	sseWriteData(b.stream.w, event, message)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendJSON(jsonStr string) {
	b.sse.SendJSON(jsonStr)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	fmt.Fprintf(b.stream.w, "data: %s\n\n", jsonStr)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
	b.sse.SendLLMStreamDelta(content, toolName, toolID, index, finishReason)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	payload := LLMStreamDeltaPayload{
		SessionID:    b.sse.sessionID,
		Content:      content,
		ToolName:     toolName,
		ToolID:       toolID,
		Index:        index,
		FinishReason: finishReason,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "llm_stream_delta", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendLLMStreamDone(finishReason string) {
	b.sse.SendLLMStreamDone(finishReason)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	payload := LLMStreamDonePayload{
		SessionID:    b.sse.sessionID,
		FinishReason: finishReason,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "llm_stream_done", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
	b.sse.SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal, isEstimated, isFinal, source)
}

func (b *desktopStreamCombinedBroker) SendThinkingBlock(provider, content, state string) {
	b.sse.SendThinkingBlock(provider, content, state)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	payload := ThinkingBlockPayload{
		SessionID: b.sse.sessionID,
		Provider:  provider,
		Content:   content,
		State:     state,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "thinking_block", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) Scrub(s string) string {
	return security.Scrub(s)
}

func sseWriteData(w http.ResponseWriter, event, data string) {
	encoded, _ := json.Marshal(map[string]string{"event": event, "detail": data})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
}

func sseWriteJSON(w http.ResponseWriter, event string, jsonPayload []byte) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(jsonPayload, &raw); err == nil {
		evt, _ := json.Marshal(event)
		raw["event"] = evt
		if enriched, err := json.Marshal(raw); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", enriched)
			return
		}
	}
	encoded, _ := json.Marshal(map[string]string{"event": event, "detail": string(jsonPayload)})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
}

func sseWriteDone(w http.ResponseWriter) {
	fmt.Fprint(w, "data: [DONE]\n\n")
}

type desktopChatContext struct {
	Source          string   `json:"source"`
	CurrentFile     string   `json:"current_file"`
	CurrentLanguage string   `json:"current_language"`
	CurrentContent  string   `json:"current_content"`
	CursorLine      int      `json:"cursor_line"`
	CursorColumn    int      `json:"cursor_column"`
	SelectedText    string   `json:"selected_text"`
	OpenFiles       []string `json:"open_files"`
}

func handleDesktopWS(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		conn, err := desktopWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		events, cancel, err := hub.Subscribe()
		if err != nil {
			_ = conn.WriteJSON(map[string]interface{}{"type": "error", "message": err.Error()})
			return
		}
		defer cancel()
		if bootstrap, err := svc.Bootstrap(r.Context()); err == nil {
			_ = conn.WriteJSON(map[string]interface{}{"type": "welcome", "payload": bootstrap})
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				var msg map[string]interface{}
				if err := conn.ReadJSON(&msg); err != nil {
					return
				}
				if msgType, _ := msg["type"].(string); msgType == "ping" {
					continue
				}
			}
		}()

		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				if err := conn.WriteJSON(event); err != nil {
					return
				}
			case <-done:
				return
			case <-r.Context().Done():
				return
			}
		}
	}
}

func broadcastDesktopEvent(s *Server, hub *desktop.Hub, event desktop.Event) {
	if hub != nil {
		hub.Broadcast(event)
	}
	if s != nil && s.SSE != nil {
		s.SSE.BroadcastType(EventVirtualDesktop, event)
	}
}

func runDesktopAgentChat(s *Server, message string, chatContext desktopChatContext) string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	sessionID := "virtual-desktop"
	runCfg := agent.RunConfig{
		Config:             &cfg,
		Logger:             s.Logger,
		LLMClient:          s.LLMClient,
		ShortTermMem:       s.ShortTermMem,
		HistoryManager:     s.HistoryManager,
		LongTermMem:        s.LongTermMem,
		KG:                 s.KG,
		InventoryDB:        s.InventoryDB,
		InvasionDB:         s.InvasionDB,
		CheatsheetDB:       s.CheatsheetDB,
		ImageGalleryDB:     s.ImageGalleryDB,
		MediaRegistryDB:    s.MediaRegistryDB,
		HomepageRegistryDB: s.HomepageRegistryDB,
		ContactsDB:         s.ContactsDB,
		PlannerDB:          s.PlannerDB,
		SQLConnectionsDB:   s.SQLConnectionsDB,
		SQLConnectionPool:  s.SQLConnectionPool,
		RemoteHub:          s.RemoteHub,
		Vault:              s.Vault,
		Registry:           s.Registry,
		Manifest:           tools.NewManifest(cfg.Directories.ToolsDir),
		CronManager:        s.CronManager,
		MissionManagerV2:   s.MissionManagerV2,
		CoAgentRegistry:    s.CoAgentRegistry,
		BudgetTracker:      s.BudgetTracker,
		DaemonSupervisor:   s.DaemonSupervisor,
		LLMGuardian:        s.LLMGuardian,
		PreparationService: s.PreparationService,
		SessionID:          sessionID,
		MessageSource:      "virtual_desktop_chat",
		VoiceOutputActive:  GetSpeakerMode(),
	}
	prompt := buildDesktopAgentPrompt(message, chatContext)
	broker := &desktopReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, sessionID)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		agent.Loopback(runCfg, prompt, broker)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return "The desktop agent request timed out."
	}
	return strings.TrimSpace(broker.finalResponse)
}

func buildDesktopAgentPrompt(message string, chatContext desktopChatContext) string {
	var b strings.Builder
	b.WriteString("The user is chatting from AuraGo Virtual Desktop. If they ask for desktop apps, widgets, or files, use the virtual_desktop tool and keep the browser desktop updated.")
	b.WriteString("\n\nYou can open files in desktop apps using the virtual_desktop tool with operation \"open_in_app\". Available apps: writer (documents, docx, html, md, txt), sheets (spreadsheets, xlsx, csv), code-studio (code files, scripts). After creating or writing a file, proactively open it in the appropriate app so the user can see it immediately. Example: after writing a document, use open_in_app with app_id \"writer\" and path to the file.")
	if chatContext.Source == "code-studio" {
		b.WriteString("\n\nThe user is coding in Code Studio.")
		b.WriteString("\nImportant: Code Studio files live inside the dedicated Code Studio container workspace, not the homepage workspace and not agent_workspace. Do not use the homepage tool for Code Studio file questions. Prefer the code/content supplied in this prompt; if content is supplied, answer from it without trying to locate the file elsewhere.")
		if strings.TrimSpace(chatContext.CurrentFile) != "" {
			b.WriteString("\nCurrent file:\n")
			b.WriteString(desktopExternalData("desktop_current_file", chatContext.CurrentFile, 2048))
		}
		if strings.TrimSpace(chatContext.CurrentLanguage) != "" {
			b.WriteString("\nLanguage:\n")
			b.WriteString(desktopExternalData("desktop_current_language", chatContext.CurrentLanguage, 128))
		}
		if chatContext.CursorLine > 0 || chatContext.CursorColumn > 0 {
			b.WriteString(fmt.Sprintf("\nCursor: line %d, column %d", chatContext.CursorLine, chatContext.CursorColumn))
		}
		if len(chatContext.OpenFiles) > 0 {
			b.WriteString("\nOpen files:\n")
			b.WriteString(desktopExternalData("desktop_open_files", strings.Join(chatContext.OpenFiles, "\n"), 8192))
		}
		if strings.TrimSpace(chatContext.SelectedText) != "" {
			b.WriteString("\nSelected text:\n")
			b.WriteString(desktopExternalData("desktop_selected_text", chatContext.SelectedText, 24000))
		}
		if strings.TrimSpace(chatContext.SelectedText) == "" && strings.TrimSpace(chatContext.CurrentContent) != "" {
			b.WriteString("\nCurrent file content:\n")
			b.WriteString(desktopExternalData("desktop_current_content", chatContext.CurrentContent, 48000))
		}
	}
	b.WriteString("\n\nUser request:\n")
	b.WriteString(desktopExternalData("desktop_user_request", message, 12000))
	return b.String()
}

func desktopExternalData(kind, value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if maxBytes > 0 && len(value) > maxBytes {
		value = value[:maxBytes] + "\n[truncated]"
	}
	return fmt.Sprintf("<external_data type=%q>\n%s\n</external_data>", kind, value)
}

type desktopReplyBroker struct {
	agent.FeedbackBroker
	finalResponse string
}

func (b *desktopReplyBroker) Send(event, message string) {
	if event == "final_response" {
		b.finalResponse = message
	}
	b.FeedbackBroker.Send(event, message)
}
