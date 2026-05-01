package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const (
	spaceAgentBridgeMaxBodyBytes int64 = 64 * 1024
	spaceAgentProxyPrefix              = "/integrations/space-agent"
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
					URL:         spaceAgentProxyURL(),
					Icon:        "space_agent",
				})
			}
		}
		writeSpaceAgentJSON(w, map[string]interface{}{"status": "ok", "webhosts": webhosts})
	}
}

func handleSpaceAgentProxy(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spaceAgentSetProxySecurityHeaders(w.Header())
		cfg := s.currentSpaceAgentConfig()
		if !cfg.SpaceAgent.Enabled {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == spaceAgentProxyPrefix {
			http.Redirect(w, r, spaceAgentProxyPrefix+"/", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == spaceAgentProxyPrefix+"/site.webmanifest" {
			spaceAgentWriteManifest(w)
			return
		}
		port := cfg.SpaceAgent.Port
		if port <= 0 {
			port = 3100
		}
		targetHost := spaceAgentProxyTargetHost(cfg.SpaceAgent.Host)
		target, err := url.Parse(fmt.Sprintf("http://%s", net.JoinHostPort(targetHost, fmt.Sprintf("%d", port))))
		if err != nil {
			http.Error(w, "Space Agent proxy target is invalid", http.StatusInternalServerError)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(req.URL.Path, spaceAgentProxyPrefix), "/")
			req.URL.RawPath = ""
			req.Host = target.Host
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-Prefix", spaceAgentProxyPrefix)
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			spaceAgentSetProxySecurityHeaders(resp.Header)
			spaceAgentRewriteProxyLocation(resp.Header, spaceAgentProxyPrefix)
			spaceAgentRewriteProxyCookies(resp.Header, spaceAgentProxyPrefix)
			contentType := strings.ToLower(resp.Header.Get("Content-Type"))
			if !spaceAgentShouldRewriteBody(contentType) {
				return nil
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
			if closeErr := resp.Body.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
			body = spaceAgentRewriteBody(body, spaceAgentProxyPrefix)
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			s.Logger.Warn("[SpaceAgent] Proxy request failed", "target", target.String(), "error", err)
			http.Error(w, "Space Agent is running but not reachable from AuraGo on "+target.String(), http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	}
}

func handleSpaceAgentRootAPIProxy(s *Server) http.HandlerFunc {
	proxy := handleSpaceAgentProxy(s)
	return func(w http.ResponseWriter, r *http.Request) {
		if !spaceAgentShouldProxyRootAPIRequest(r) {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	}
}

func spaceAgentShouldProxyRootAPIRequest(r *http.Request) bool {
	if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
		return false
	}
	switch r.URL.Path {
	case "/api/login", "/api/login_challenge", "/api/user_self_info", "/api/file_read", "/api/file_paths", "/api/file_list", "/api/file_info":
		return true
	}
	referer := strings.TrimSpace(r.Referer())
	if referer == "" {
		return false
	}
	refURL, err := url.Parse(referer)
	if err != nil {
		return false
	}
	return refURL.Path == spaceAgentProxyPrefix || strings.HasPrefix(refURL.Path, spaceAgentProxyPrefix+"/")
}

func spaceAgentSetProxySecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob:; script-src-elem 'self' 'unsafe-inline' data: blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' ws: wss:; worker-src 'self' blob: data:; object-src 'none'; form-action 'self'; base-uri 'self'; frame-ancestors 'none'; manifest-src 'self' data:;")
}

func spaceAgentWriteManifest(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(`{"name":"Space Agent","short_name":"Space Agent","display":"standalone","start_url":"/integrations/space-agent/","scope":"/integrations/space-agent/","theme_color":"#111827","background_color":"#111827","icons":[]}`))
}

func spaceAgentProxyURL() string {
	return "/integrations/space-agent/"
}

func spaceAgentProxyTargetHost(host string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || host == "0.0.0.0" || host == "::" || strings.EqualFold(host, "localhost") || host == "::1" {
		return "127.0.0.1"
	}
	return host
}

func spaceAgentRewriteProxyLocation(header http.Header, prefix string) {
	location := strings.TrimSpace(header.Get("Location"))
	if strings.HasPrefix(location, "/") && !strings.HasPrefix(location, prefix+"/") {
		header.Set("Location", prefix+location)
	}
}

func spaceAgentRewriteProxyCookies(header http.Header, prefix string) {
	cookies := header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	header.Del("Set-Cookie")
	for _, cookie := range cookies {
		header.Add("Set-Cookie", spaceAgentCookieWithPath(cookie, prefix+"/"))
		header.Add("Set-Cookie", spaceAgentCookieWithPath(cookie, "/api/"))
	}
}

func spaceAgentCookieWithPath(cookie string, path string) string {
	parts := strings.Split(cookie, ";")
	for i, part := range parts {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(part)), "path=") {
			prefix := ""
			if strings.HasPrefix(part, " ") {
				prefix = " "
			}
			parts[i] = prefix + "Path=" + path
			return strings.Join(parts, ";")
		}
	}
	return cookie + "; Path=" + path
}

func spaceAgentShouldRewriteBody(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "javascript") ||
		strings.Contains(contentType, "ecmascript") ||
		strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/json")
}

func spaceAgentRewriteBody(body []byte, prefix string) []byte {
	replacements := []struct {
		old string
		new string
	}{
		{`href="/`, `href="` + prefix + `/`},
		{`src="/`, `src="` + prefix + `/`},
		{`action="/`, `action="` + prefix + `/`},
		{`fetch("/`, `fetch("` + prefix + `/`},
		{`fetch('/`, `fetch('` + prefix + `/`},
		{`import("/`, `import("` + prefix + `/`},
		{`import('/`, `import('` + prefix + `/`},
		{`from "/`, `from "` + prefix + `/`},
		{`from '/`, `from '` + prefix + `/`},
		{`new URL("/`, `new URL("` + prefix + `/`},
		{`new URL('/`, `new URL('` + prefix + `/`},
		{`Worker("/`, `Worker("` + prefix + `/`},
		{`Worker('/`, `Worker('` + prefix + `/`},
		{`"/api/`, `"` + prefix + `/api/`},
		{`'/api/`, `'` + prefix + `/api/`},
		{"`/api/", "`" + prefix + `/api/`},
		{`"/mod/`, `"` + prefix + `/mod/`},
		{`'/mod/`, `'` + prefix + `/mod/`},
		{"`/mod/", "`" + prefix + `/mod/`},
		{`"/enter`, `"` + prefix + `/enter`},
		{`'/enter`, `'` + prefix + `/enter`},
		{"`/enter", "`" + prefix + `/enter`},
		{`"/login`, `"` + prefix + `/login`},
		{`'/login`, `'` + prefix + `/login`},
		{"`/login", "`" + prefix + `/login`},
		{`"/logout`, `"` + prefix + `/logout`},
		{`'/logout`, `'` + prefix + `/logout`},
		{"`/logout", "`" + prefix + `/logout`},
		{`href="site.webmanifest"`, `href="` + prefix + `/site.webmanifest"`},
		{`href='site.webmanifest'`, `href='` + prefix + `/site.webmanifest'`},
		{`href="/site.webmanifest"`, `href="` + prefix + `/site.webmanifest"`},
		{`href='/site.webmanifest'`, `href='` + prefix + `/site.webmanifest'`},
		{`"/"`, `"` + prefix + `/"`},
		{`'/'`, `'` + prefix + `/'`},
		{"`/`", "`" + prefix + `/` + "`"},
	}
	out := string(body)
	for _, repl := range replacements {
		out = strings.ReplaceAll(out, repl.old, repl.new)
	}
	return []byte(out)
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
		if reqHost, _, err := net.SplitHostPort(r.Host); err == nil && reqHost != "" {
			host = reqHost
		} else if r.Host != "" {
			host = r.Host
		}
	}
	port := cfg.SpaceAgent.Port
	if port <= 0 {
		port = 3100
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
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
