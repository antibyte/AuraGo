package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/desktop"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

var desktopWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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
		Settings:    map[string]string{},
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
		files, err := svc.ListFiles(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "files": files})
	}
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
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
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
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
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
