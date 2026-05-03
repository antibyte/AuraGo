package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/desktop"
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
		Settings:    settings,
		IconCatalog: desktop.DesktopIconCatalog(settings),
	}
}

func handleDesktopBootstrap(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func handleDesktopApps(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func handleDesktopWidgets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodDelete {
			id := r.URL.Query().Get("id")
			if err := svc.DeleteWidget(r.Context(), id, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_widget", "widget_id": id}, CreatedAt: time.Now().UTC()}
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
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Message string `json:"message"`
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
		answer := runDesktopAgentChat(s, body.Message)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "answer": answer})
	}
}

func handleDesktopWS(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func runDesktopAgentChat(s *Server, message string) string {
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
	prompt := "The user is chatting from AuraGo Virtual Desktop. If they ask for desktop apps, widgets, or files, use the virtual_desktop tool and keep the browser desktop updated.\n\nUser request:\n\n" + message
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
