package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const (
	spaceAgentBridgeMaxBodyBytes int64 = 64 * 1024
)

type spaceAgentBridgeMessage struct {
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Content   string `json:"content"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id,omitempty"`
}

type webhostIntegration struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	Icon        string `json:"icon,omitempty"`
}

func handleSpaceAgentStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.currentSpaceAgentConfig()
		writeSpaceAgentJSON(w, spaceAgentStatusPayload(s, &cfg))
	}
}

func handleSpaceAgentRecreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.currentSpaceAgentConfig()
		if !cfg.SpaceAgent.Enabled {
			writeSpaceAgentJSON(w, map[string]interface{}{"status": "disabled", "message": "Space Agent integration is disabled"})
			return
		}
		if err := s.ensureSpaceAgentSecrets(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sidecarCfg, err := tools.ResolveSpaceAgentSidecarConfig(&cfg, InternalAPIURL(&cfg))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		go tools.RecreateSpaceAgentSidecar(cfg.Docker.Host, sidecarCfg, s.Logger)
		writeSpaceAgentJSON(w, map[string]interface{}{"status": "starting", "message": "Space Agent sidecar recreation started"})
	}
}

func handleSpaceAgentSend(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.currentSpaceAgentConfig()
		if !cfg.SpaceAgent.Enabled {
			writeSpaceAgentJSON(w, map[string]interface{}{"status": "disabled", "message": "Space Agent integration is disabled"})
			return
		}
		var req tools.SpaceAgentInstruction
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
		defer cancel()
		writeSpaceAgentJSON(w, tools.SendSpaceAgentInstruction(ctx, &cfg, req))
	}
}

func handleSpaceAgentBridgeMessages(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.currentSpaceAgentConfig()
		if !cfg.SpaceAgent.Enabled {
			writeSpaceAgentJSON(w, map[string]interface{}{"status": "disabled", "message": "Space Agent integration is disabled"})
			return
		}
		token := strings.TrimSpace(cfg.SpaceAgent.BridgeToken)
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		expectedAuth := "Bearer " + token
		if token == "" || subtle.ConstantTimeCompare([]byte(authHeader), []byte(expectedAuth)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var msg spaceAgentBridgeMessage
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, spaceAgentBridgeMaxBodyBytes)).Decode(&msg); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		msg.Type = strings.TrimSpace(msg.Type)
		if msg.Type == "" {
			msg.Type = "message"
		}
		msg.Source = strings.TrimSpace(msg.Source)
		if msg.Source == "" {
			msg.Source = "space-agent"
		}
		msg.Timestamp = strings.TrimSpace(msg.Timestamp)
		if msg.Timestamp == "" {
			msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}
		msg.Summary = security.IsolateExternalData(strings.TrimSpace(msg.Summary))
		msg.Content = security.IsolateExternalData(strings.TrimSpace(msg.Content))
		if s.SSE != nil {
			s.SSE.BroadcastType(EventSpaceAgentMessage, msg)
		}
		writeSpaceAgentJSON(w, map[string]interface{}{"status": "ok", "message": msg})
	}
}

func handleIntegrationWebhosts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.currentSpaceAgentConfig()
		webhosts := make([]webhostIntegration, 0, 1)
		if cfg.SpaceAgent.Enabled {
			status := "starting"
			if payload := spaceAgentStatusPayload(s, &cfg); payload != nil {
				if raw, ok := payload["status"].(string); ok && raw != "" && raw != "disabled" && raw != "stopped" {
					status = raw
				}
			}
			if status == "running" || status == "starting" {
				webhosts = append(webhosts, webhostIntegration{
					ID:          "space_agent",
					Name:        "Space Agent",
					Description: "Managed Space Agent workspace",
					Status:      status,
					URL:         spaceAgentPublicURL(&cfg, r),
					Icon:        "space_agent",
				})
			}
		}
		writeSpaceAgentJSON(w, map[string]interface{}{"status": "ok", "webhosts": webhosts})
	}
}

func handleSpaceAgentLegacyRedirect(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.currentSpaceAgentConfig()
		if !cfg.SpaceAgent.Enabled {
			http.NotFound(w, r)
			return
		}
		target := spaceAgentPublicURL(&cfg, r)
		if target == "" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}
}

func (s *Server) currentSpaceAgentConfig() config.Config {
	if s == nil || s.Cfg == nil {
		return config.Config{}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return *s.Cfg
}

func spaceAgentStatusPayload(s *Server, cfg *config.Config) map[string]interface{} {
	if cfg == nil || !cfg.SpaceAgent.Enabled {
		return map[string]interface{}{"status": "disabled", "enabled": false}
	}
	sidecarCfg, err := tools.ResolveSpaceAgentSidecarConfig(cfg, InternalAPIURL(cfg))
	if err != nil {
		return map[string]interface{}{"status": "error", "enabled": true, "message": err.Error()}
	}
	payload := tools.SpaceAgentDockerStatus(cfg.Docker.Host, sidecarCfg)
	if _, ok := payload["url"]; !ok {
		payload["url"] = cfg.SpaceAgent.PublicURL
	}
	return payload
}

func (s *Server) ensureSpaceAgentSecrets(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if strings.TrimSpace(cfg.SpaceAgent.AdminPassword) == "" {
		secret, err := randomSpaceAgentSecret(24)
		if err != nil {
			return err
		}
		cfg.SpaceAgent.AdminPassword = secret
		if s.Vault != nil {
			if err := s.Vault.WriteSecret("space_agent_admin_password", secret); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(cfg.SpaceAgent.BridgeToken) == "" {
		token, err := randomSpaceAgentSecret(32)
		if err != nil {
			return err
		}
		cfg.SpaceAgent.BridgeToken = token
		if s.Vault != nil {
			if err := s.Vault.WriteSecret("space_agent_bridge_token", token); err != nil {
				return err
			}
		}
	}
	if s.Cfg != nil {
		s.CfgMu.Lock()
		s.Cfg.SpaceAgent.AdminPassword = cfg.SpaceAgent.AdminPassword
		s.Cfg.SpaceAgent.BridgeToken = cfg.SpaceAgent.BridgeToken
		s.CfgMu.Unlock()
	}
	return nil
}

func randomSpaceAgentSecret(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func spaceAgentPublicURL(cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	if raw := strings.TrimSpace(cfg.SpaceAgent.PublicURL); raw != "" && !spaceAgentURLUsesLoopbackHost(raw) {
		return raw
	}
	host := "127.0.0.1"
	if r != nil {
		reqHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
		if reqHost == "" {
			reqHost = r.Host
		}
		if idx := strings.IndexByte(reqHost, ','); idx >= 0 {
			reqHost = strings.TrimSpace(reqHost[:idx])
		}
		if parsedHost, _, err := net.SplitHostPort(reqHost); err == nil && parsedHost != "" {
			host = parsedHost
		} else if reqHost != "" {
			host = reqHost
		}
	}
	port := cfg.SpaceAgent.Port
	scheme := "http"
	if cfg.SpaceAgent.HTTPSEnabled {
		scheme = "https"
		port = cfg.SpaceAgent.HTTPSPort
	}
	if port <= 0 {
		if scheme == "https" {
			port = 3101
		} else {
			port = 3100
		}
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

func spaceAgentURLUsesLoopbackHost(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func writeSpaceAgentJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
