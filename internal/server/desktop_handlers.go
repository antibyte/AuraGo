package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"aurago/internal/config"
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

const desktopSmallJSONBodyLimit = int64(64 * 1024)
const desktopMediumJSONBodyLimit = int64(1024 * 1024)
const desktopLargeJSONBodyLimit = int64(8 * 1024 * 1024)
const desktopChatJSONBodyLimit = int64(2 * 1024 * 1024)

func decodeDesktopJSON(w http.ResponseWriter, r *http.Request, dst interface{}, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = desktopMediumJSONBodyLimit
	}
	// Desktop JSON handlers should call decodeDesktopJSON(w, r, ...) so every
	// request body is capped before decoding.
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}

func desktopFileJSONBodyLimit(svc *desktop.Service) int64 {
	if svc == nil {
		return 50*1024*1024 + 4096
	}
	maxSize := int64(svc.Config().MaxFileSizeMB) * 1024 * 1024
	if maxSize <= 0 {
		maxSize = 50 * 1024 * 1024
	}
	return maxSize + maxSize/4 + 4096
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
		if s.DesktopHub != nil {
			s.DesktopHub.Close()
		}
		_ = s.DesktopService.Close()
		s.DesktopService = nil
		if s.DesktopStore != nil {
			_ = s.DesktopStore.Close()
			s.DesktopStore = nil
		}
		s.DesktopHub = nil
		// Intentionally do not call CloseToolDesktopService() here — the next
		// creation block will overwrite the cache with the fresh service via Set.
		// Calling Close globally would break parallel tests that share the process.
	}
	if s.DesktopService == nil {
		svc, err := desktop.NewService(desktopCfg)
		if err != nil {
			return nil, nil, err
		}
		svc.SetIntegritySecretStore(s.Vault)
		if err := svc.Init(ctx); err != nil {
			_ = svc.Close()
			return nil, nil, err
		}
		if codeContainer := svc.CodeContainer(); codeContainer != nil {
			codeContainer.SetDockerClient(newCodeStudioDockerAdapter(desktopCfg, s.Logger))
		}
		s.DesktopService = svc
		s.DesktopHub = desktop.NewHub(desktopCfg.MaxWSClients)
		// Share the long-lived instance with the agent tool layer so that
		// virtual_desktop / office tool calls reuse the same service instead of
		// spinning up transient instances on every call.
		tools.SetToolDesktopIntegritySecretStore(s.Vault)
		tools.SetToolDesktopService(svc)
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
			Root:        "/",
			Directories: desktop.DefaultDirectories(),
			MaxFileSize: int64(desktopCfg.MaxFileSizeMB) * 1024 * 1024,
		},
		BuiltinApps: desktop.BuiltinApps(),
		Shortcuts: []desktop.Shortcut{
			{ID: "app-files", TargetType: desktop.ShortcutTargetApp, TargetID: "files", Name: "Files", Icon: "folder"},
			{ID: "dir-Trash", TargetType: desktop.ShortcutTargetDirectory, Path: "Trash", Name: "Trash", Icon: "trash"},
		},
		Settings:    settings,
		Providers:   s.desktopProviderOptions(),
		IconCatalog: desktop.DesktopIconCatalog(settings),
	}
}

func (s *Server) enrichDesktopBootstrap(payload *desktop.BootstrapPayload) {
	if payload == nil {
		return
	}
	payload.Providers = s.desktopProviderOptions()
}

func (s *Server) desktopProviderOptions() []desktop.ProviderOption {
	if s == nil || s.Cfg == nil {
		return nil
	}
	s.CfgMu.RLock()
	providers := append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	s.CfgMu.RUnlock()
	out := make([]desktop.ProviderOption, 0, len(providers))
	for _, p := range providers {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = id
		}
		out = append(out, desktop.ProviderOption{
			ID:    id,
			Name:  name,
			Type:  strings.TrimSpace(p.Type),
			Model: strings.TrimSpace(p.Model),
		})
	}
	return out
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
			s.enrichDesktopBootstrap(&bootstrap)
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
					_ = conn.WriteJSON(map[string]interface{}{"type": "pong"})
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
