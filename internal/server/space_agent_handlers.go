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
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const (
	spaceAgentBridgeMaxBodyBytes int64 = 64 * 1024
	spaceAgentBridgeSessionID          = "space-agent-bridge"
	defaultManifestTailscalePort       = 443
	legacyManifestTailscalePort        = 8444
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

var (
	webhostsCache    []webhostIntegration
	webhostsCacheMu  sync.RWMutex
	webhostsCachedAt time.Time
	webhostsCacheTTL = 10 * time.Second
)

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
		sidecarCfg, err := tools.ResolveSpaceAgentSidecarConfig(&cfg, spaceAgentBridgeBaseURL(s, &cfg, r))
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
		allowSpaceAgentBridgeCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
		if shouldRunSpaceAgentBridgeMessage(msg) {
			answer, delivery := runSpaceAgentBridgeMessage(s, msg)
			response := spaceAgentBridgeResponse(msg, answer, delivery)
			if s.SSE != nil {
				s.SSE.BroadcastType(EventSpaceAgentMessage, response)
			}
			writeSpaceAgentJSON(w, response)
			return
		}
		if s.SSE != nil {
			s.SSE.BroadcastType(EventSpaceAgentMessage, msg)
		}
		writeSpaceAgentJSON(w, map[string]interface{}{"status": "ok", "message": msg})
	}
}

func spaceAgentBridgeResponse(msg spaceAgentBridgeMessage, answer string, delivery map[string]interface{}) map[string]interface{} {
	response := map[string]interface{}{"status": "ok", "message": msg}
	if strings.TrimSpace(answer) != "" {
		response["answer"] = answer
	}
	if delivery != nil {
		response["space_agent_delivery"] = delivery
	}
	return response
}

func shouldRunSpaceAgentBridgeMessage(msg spaceAgentBridgeMessage) bool {
	return strings.EqualFold(strings.TrimSpace(msg.Type), "question") && strings.TrimSpace(msg.Content) != ""
}

func runSpaceAgentBridgeMessage(s *Server, msg spaceAgentBridgeMessage) (string, map[string]interface{}) {
	if s == nil || s.Cfg == nil || s.SSE == nil {
		return "", nil
	}
	cfg := s.currentSpaceAgentConfig()
	sessionID := spaceAgentBridgeSessionID
	content := spaceAgentBridgeQuestionPrompt(msg)
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
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
		WorkspaceSearch:    s.WorkspaceSearch,
		SessionID:          sessionID,
		MessageSource:      "space_agent_bridge",
		VoiceOutputActive:  GetSpeakerMode(),
	}
	broker := &spaceAgentReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, sessionID)}
	agent.Loopback(runCfg, content, broker)
	answer := strings.TrimSpace(broker.finalResponse)
	if answer == "" {
		if s.Logger != nil {
			s.Logger.Warn("[SpaceAgent] Bridge question completed without final response", "session_id", msg.SessionID)
		}
		return "", nil
	}
	if !shouldPostBackSpaceAgentBridgeAnswer(msg) {
		if s.Logger != nil {
			s.Logger.Info("[SpaceAgent] Bridge answer returned synchronously; no Space Agent postback needed")
		}
		return answer, nil
	}
	reply := tools.SpaceAgentInstruction{
		Instruction: "AuraGo answered your bridge question.",
		Information: answer,
		SessionID:   msg.SessionID,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	result := tools.SendSpaceAgentInstruction(ctx, &cfg, reply)
	if rawStatus, _ := result["status"].(string); rawStatus != "ok" {
		if s.Logger != nil {
			s.Logger.Warn("[SpaceAgent] Failed to send bridge answer back to Space Agent", "result", result, "session_id", msg.SessionID)
		}
		return answer, result
	}
	if s.Logger != nil {
		s.Logger.Info("[SpaceAgent] Bridge answer sent back to Space Agent", "session_id", msg.SessionID)
	}
	return answer, result
}

func shouldPostBackSpaceAgentBridgeAnswer(msg spaceAgentBridgeMessage) bool {
	return strings.TrimSpace(msg.SessionID) != ""
}

func spaceAgentBridgeQuestionPrompt(msg spaceAgentBridgeMessage) string {
	parts := []string{"Space Agent sent this bridge question to AuraGo."}
	if source := strings.TrimSpace(msg.Source); source != "" {
		parts = append(parts, "Source: "+source)
	}
	if sessionID := strings.TrimSpace(msg.SessionID); sessionID != "" {
		parts = append(parts, "Correlation ID: "+sessionID)
	}
	if summary := strings.TrimSpace(msg.Summary); summary != "" {
		parts = append(parts, "Summary: "+summary)
	}
	parts = append(parts,
		"Question:",
		strings.TrimSpace(msg.Content),
		"Answer the Space Agent using AuraGo's current tools and integrations. If live system state is requested, query it now rather than relying on memory.",
	)
	return strings.Join(parts, "\n\n")
}

type spaceAgentReplyBroker struct {
	agent.FeedbackBroker
	finalResponse string
}

func (b *spaceAgentReplyBroker) Send(event, message string) {
	if event == "final_response" {
		b.finalResponse = message
	}
	b.FeedbackBroker.Send(event, message)
}

func allowSpaceAgentBridgeCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Vary", "Origin")
}

func handleIntegrationWebhosts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeSpaceAgentJSON(w, map[string]interface{}{"status": "ok", "webhosts": integrationWebhostsForRequest(s, r)})
	}
}

func integrationWebhostsForRequest(s *Server, r *http.Request) []webhostIntegration {
	if s == nil {
		return []webhostIntegration{}
	}

	webhostsCacheMu.RLock()
	if len(webhostsCache) > 0 && time.Since(webhostsCachedAt) < webhostsCacheTTL {
		cached := make([]webhostIntegration, len(webhostsCache))
		copy(cached, webhostsCache)
		webhostsCacheMu.RUnlock()
		return cached
	}
	webhostsCacheMu.RUnlock()

	cfg := s.currentSpaceAgentConfig()
	webhosts := make([]webhostIntegration, 0, 5)

	var mu sync.Mutex
	var wg sync.WaitGroup

	if cfg.SpaceAgent.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := "starting"
			if payload := spaceAgentStatusPayload(s, &cfg); payload != nil {
				if raw, ok := payload["status"].(string); ok && raw != "" && raw != "disabled" && raw != "stopped" {
					status = raw
				}
			}
			if status == "running" || status == "starting" {
				mu.Lock()
				webhosts = append(webhosts, webhostIntegration{
					ID:          "space_agent",
					Name:        "Space Agent",
					Description: "Managed Space Agent workspace",
					Status:      status,
					URL:         spaceAgentBrowserURL(s, &cfg, r),
					Icon:        "space_agent",
				})
				mu.Unlock()
			}
		}()
	}

	if cfg.Manifest.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manifestPayload := manifestStatus(r.Context(), s, &cfg)
			manifestURL := ""
			if u, ok := manifestPayload["url"].(string); ok {
				manifestURL = u
			}
			browserURL := manifestBrowserURL(s, &cfg, r, manifestURL)
			status := "starting"
			if raw, ok := manifestPayload["status"].(string); ok && raw != "" {
				status = raw
			}
			mu.Lock()
			webhosts = append(webhosts, webhostIntegration{
				ID:          "manifest",
				Name:        "Manifest",
				Description: "Manifest.build gateway",
				Status:      status,
				URL:         browserURL,
				Icon:        "link",
			})
			mu.Unlock()
		}()
	}

	if cfg.Dograh.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dograhPayload := dograhStatusForRequest(r.Context(), s, &cfg, r)
			dograhURL := ""
			if u, ok := dograhPayload["ui_url"].(string); ok {
				dograhURL = u
			}
			status := "starting"
			if raw, ok := dograhPayload["status"].(string); ok && raw != "" {
				status = raw
			}
			mu.Lock()
			webhosts = append(webhosts, webhostIntegration{
				ID:          "dograh",
				Name:        "Dograh",
				Description: "Dograh workflow automation",
				Status:      status,
				URL:         dograhURL,
				Icon:        "link",
			})
			mu.Unlock()
		}()
	}

	if cfg.VirtualDesktop.Enabled {
		webhosts = append(webhosts, webhostIntegration{
			ID:          "virtual_desktop",
			Name:        "Virtual Desktop",
			Description: "Browser-based virtual desktop",
			Status:      "running",
			URL:         "/desktop",
			Icon:        "expand",
		})
	}

	if cfg.Homepage.Enabled {
		homepageURL := homepageBrowserURL(s, &cfg, r)
		if homepageURL != "" {
			webhosts = append(webhosts, webhostIntegration{
				ID:          "homepage",
				Name:        "Homepage",
				Description: "Homepage web preview",
				Status:      "running",
				URL:         homepageURL,
				Icon:        "web",
			})
		}
	}

	wg.Wait()

	sort.Slice(webhosts, func(i, j int) bool {
		return webhosts[i].ID < webhosts[j].ID
	})

	webhostsCacheMu.Lock()
	webhostsCache = make([]webhostIntegration, len(webhosts))
	copy(webhostsCache, webhosts)
	webhostsCachedAt = time.Now()
	webhostsCacheMu.Unlock()

	return webhosts
}

func clearWebhostsCache() {
	webhostsCacheMu.Lock()
	webhostsCache = nil
	webhostsCachedAt = time.Time{}
	webhostsCacheMu.Unlock()
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
		target := spaceAgentBrowserURL(s, &cfg, r)
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
	sidecarCfg, err := tools.ResolveSpaceAgentSidecarConfig(cfg, spaceAgentBridgeBaseURL(s, cfg, nil))
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

func spaceAgentBridgeBaseURL(s *Server, cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	if requestLooksTailscale(r) {
		if host := requestForwardedHost(r); host != "" {
			return "https://" + host
		}
	}
	if s != nil && s.TsNetManager != nil && cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.ServeHTTP {
		status := s.TsNetManager.GetStatus()
		host := strings.TrimSuffix(strings.TrimSpace(status.DNS), ".")
		if status.ServingHTTP && host != "" {
			return "https://" + host
		}
		if host == "" {
			host = strings.TrimSpace(cfg.Tailscale.TsNet.Hostname)
			if host != "" && strings.Contains(host, ".") {
				return "https://" + strings.TrimSuffix(host, ".")
			}
		}
	}
	return InternalAPIURL(cfg)
}

func requestForwardedHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.IndexByte(host, ','); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost != "" {
		host = parsedHost
	}
	return strings.TrimSuffix(strings.Trim(strings.ToLower(host), "[]"), ".")
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

func spaceAgentBrowserURL(s *Server, cfg *config.Config, r *http.Request) string {
	if s != nil && cfg != nil && cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.ExposeSpaceAgent && requestLooksTailscale(r) && s.TsNetManager != nil {
		status := s.TsNetManager.GetStatus()
		host := strings.TrimSuffix(strings.TrimSpace(status.SpaceAgentDNS), ".")
		if status.SpaceAgentServing && host != "" {
			return "https://" + host
		}
		if derived := deriveSpaceAgentTailscaleURL(cfg, r); derived != "" {
			return derived
		}
	}
	return spaceAgentPublicURL(cfg, r)
}

func homepageBrowserURL(s *Server, cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	if s != nil && s.TsNetManager != nil && cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.ExposeHomepage {
		status := s.TsNetManager.GetStatus()
		if status.HomepageServing {
			if host := tsnetStatusHost(status.DNS, status.CertDNS); host != "" {
				return fmt.Sprintf("https://%s:8443", host)
			}
		}
		if requestLooksTailscale(r) {
			if host := requestForwardedHost(r); host != "" {
				return fmt.Sprintf("https://%s:8443", host)
			}
		}
	}
	if tunnelURL := tools.GetTunnelURL(); tunnelURL != "" {
		return tunnelURL
	}
	if cfg.Homepage.WebServerPort > 0 {
		host := homepageRequestHost(r)
		if host == "" {
			host = "localhost"
		}
		return fmt.Sprintf("http://%s:%d", host, cfg.Homepage.WebServerPort)
	}
	return ""
}

func homepageRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := requestForwardedHost(r)
	if host == "" {
		return ""
	}
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		return "localhost"
	}
	return host
}

func manifestBrowserURL(s *Server, cfg *config.Config, r *http.Request, fallbackURL string) string {
	if cfg == nil {
		return fallbackURL
	}
	if cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.ExposeManifest {
		if s != nil && s.TsNetManager != nil {
			status := s.TsNetManager.GetStatus()
			if status.ManifestServing {
				host := strings.TrimSuffix(strings.TrimSpace(status.ManifestDNS), ".")
				if host == "" {
					host = tsnetStatusHost(status.DNS, status.CertDNS)
				}
				if host != "" {
					return formatManifestTailscaleURL(host, tsnetCfgManifestPort(s))
				}
			}
			if requestLooksTailscale(r) {
				return ""
			}
		}
		if requestLooksTailscale(r) {
			port := cfg.Tailscale.TsNet.ManifestPort
			if s != nil && s.Cfg != nil {
				port = tsnetCfgManifestPort(s)
			}
			if derived := deriveManifestTailscaleURL(cfg, r, port); derived != "" {
				return derived
			}
		}
	}
	if tunnelURL := tools.GetTunnelURL(); tunnelURL != "" {
		return tunnelURL
	}
	if requestLooksTailscale(r) {
		return ""
	}
	return manifestURLWithRequestHost(fallbackURL, r)
}

func deriveManifestTailscaleURL(cfg *config.Config, r *http.Request, port int) string {
	if cfg == nil {
		return ""
	}
	host := deriveNamedTailscaleHost(cfg, r, strings.TrimSpace(cfg.Tailscale.TsNet.ManifestHostname), "-manifest")
	if host == "" {
		return ""
	}
	return formatManifestTailscaleURL(host, port)
}

func effectiveManifestTailscalePort(port int) int {
	if port <= 0 || port == legacyManifestTailscalePort {
		return defaultManifestTailscalePort
	}
	return port
}

func formatManifestTailscaleURL(host string, port int) string {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" {
		return ""
	}
	port = effectiveManifestTailscalePort(port)
	if port == defaultManifestTailscalePort {
		return "https://" + host
	}
	return fmt.Sprintf("https://%s:%d", host, port)
}

func manifestURLWithRequestHost(rawURL string, r *http.Request) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return rawURL
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	if host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "0.0.0.0" && host != "::" {
		return rawURL
	}
	reqHost := effectiveRequestHost(r)
	if reqHost == "" {
		return rawURL
	}
	port := parsed.Port()
	if port != "" {
		parsed.Host = net.JoinHostPort(reqHost, port)
	} else {
		parsed.Host = reqHost
	}
	return parsed.String()
}

func effectiveRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.IndexByte(host, ','); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		host = h
	}
	return strings.TrimSuffix(strings.Trim(strings.ToLower(host), "[]"), ".")
}

func tsnetStatusHost(dns string, certDNS []string) string {
	host := strings.TrimSuffix(strings.TrimSpace(dns), ".")
	if host == "" && len(certDNS) > 0 {
		host = strings.TrimSuffix(strings.TrimSpace(certDNS[0]), ".")
	}
	return host
}

func requestLooksTailscale(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.IndexByte(host, ','); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost != "" {
		host = parsedHost
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	return strings.HasSuffix(host, ".ts.net")
}

func deriveSpaceAgentTailscaleURL(cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	host := deriveNamedTailscaleHost(cfg, r, strings.TrimSpace(cfg.Tailscale.TsNet.SpaceAgentHostname), "-space-agent")
	if host == "" {
		return ""
	}
	return "https://" + host
}

func deriveNamedTailscaleHost(cfg *config.Config, r *http.Request, configuredHost string, fallbackSuffix string) string {
	if cfg == nil || r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if idx := strings.IndexByte(host, ','); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost != "" {
		host = parsedHost
	}
	host = strings.TrimSuffix(strings.Trim(strings.ToLower(host), "[]"), ".")
	if !strings.HasSuffix(host, ".ts.net") {
		return ""
	}
	parts := strings.Split(host, ".")
	if len(parts) < 4 {
		return ""
	}
	namedHost := strings.TrimSpace(configuredHost)
	if namedHost == "" {
		base := strings.TrimSpace(cfg.Tailscale.TsNet.Hostname)
		if base == "" {
			base = parts[0]
		}
		namedHost = base + fallbackSuffix
	}
	parts[0] = strings.ToLower(namedHost)
	return strings.Join(parts, ".")
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
