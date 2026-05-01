package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
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
	spaceAgentFileReadBodyHeader       = "X-Aurago-Space-Agent-File-Read-Body"
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
			if body, ok := spaceAgentOptionalFileReadFallback(resp); ok {
				resp.StatusCode = http.StatusOK
				resp.Status = "200 OK"
				resp.Header.Set("Content-Type", "application/json")
				resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
				if resp.Body != nil {
					_ = resp.Body.Close()
				}
				resp.Body = io.NopCloser(bytes.NewReader(body))
				resp.ContentLength = int64(len(body))
				return nil
			}
			contentType := strings.ToLower(resp.Header.Get("Content-Type"))
			if !spaceAgentShouldRewriteResponseBody(contentType, resp.Request.URL.Path) {
				return nil
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
			if closeErr := resp.Body.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
			if strings.Contains(contentType, "json") && resp.Request.URL.Path == "/api/login" {
				body = spaceAgentRewriteLoginJSONRedirects(body, spaceAgentProxyPrefix)
			} else {
				body = spaceAgentRewriteBody(body, spaceAgentProxyPrefix)
			}
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
		if r.URL.Path == "/api/file_read" && r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, 256*1024))
			if closeErr := r.Body.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
			if err != nil {
				http.Error(w, "Invalid file_read body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			if len(body) > 0 {
				r.Header.Set(spaceAgentFileReadBodyHeader, base64.RawURLEncoding.EncodeToString(body))
			}
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
	return spaceAgentShouldRewriteResponseBody(contentType, "")
}

func spaceAgentShouldRewriteResponseBody(contentType string, path string) bool {
	contentType = strings.ToLower(contentType)
	path = strings.TrimSpace(path)
	if path == "/api/login_challenge" && strings.Contains(contentType, "json") {
		return false
	}
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
		{`location.href = "/"`, `location.href = "` + prefix + `/"`},
		{`location.href="/"`, `location.href="` + prefix + `/"`},
		{`window.location = "/"`, `window.location = "` + prefix + `/"`},
		{`window.location="/"`, `window.location="` + prefix + `/"`},
		{`window.location.href = "/"`, `window.location.href = "` + prefix + `/"`},
		{`window.location.href="/"`, `window.location.href="` + prefix + `/"`},
	}
	out := string(body)
	for _, repl := range replacements {
		out = strings.ReplaceAll(out, repl.old, repl.new)
	}
	return []byte(out)
}

func spaceAgentRewriteLoginJSONRedirects(body []byte, prefix string) []byte {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	changed := spaceAgentRewriteRedirectFields(payload, prefix)
	if !changed {
		return body
	}
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

func spaceAgentRewriteRedirectFields(value interface{}, prefix string) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, raw := range typed {
			if str, ok := raw.(string); ok && spaceAgentIsRedirectField(key) && strings.HasPrefix(str, "/") && !strings.HasPrefix(str, prefix+"/") {
				typed[key] = prefix + str
				changed = true
				continue
			}
			if spaceAgentRewriteRedirectFields(raw, prefix) {
				changed = true
			}
		}
	case []interface{}:
		for _, item := range typed {
			if spaceAgentRewriteRedirectFields(item, prefix) {
				changed = true
			}
		}
	}
	return changed
}

func spaceAgentIsRedirectField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "redirect", "redirect_url", "redirecturl", "next", "next_url", "nexturl", "location", "url", "href":
		return true
	default:
		return false
	}
}

func spaceAgentOptionalFileReadFallback(resp *http.Response) ([]byte, bool) {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return nil, false
	}
	if resp.StatusCode != http.StatusNotFound || resp.Request.URL.Path != "/api/file_read" {
		return nil, false
	}
	encoded := strings.TrimSpace(resp.Request.Header.Get(spaceAgentFileReadBodyHeader))
	if encoded == "" {
		return nil, false
	}
	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, false
	}
	fallback, ok := spaceAgentBuildOptionalFileReadResponse(body)
	if !ok {
		return nil, false
	}
	return fallback, true
}

func spaceAgentBuildOptionalFileReadResponse(body []byte) ([]byte, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	encoding := strings.TrimSpace(stringValue(payload["encoding"]))
	if encoding == "" {
		encoding = "utf8"
	}
	if rawFiles, ok := payload["files"].([]interface{}); ok {
		if len(rawFiles) == 0 {
			return nil, false
		}
		files := make([]map[string]string, 0, len(rawFiles))
		for _, raw := range rawFiles {
			path, itemEncoding := spaceAgentFileReadRequestInfo(raw, encoding)
			content, ok := spaceAgentOptionalFileContent(path)
			if !ok {
				return nil, false
			}
			if itemEncoding == "" {
				itemEncoding = encoding
			}
			files = append(files, map[string]string{"content": content, "encoding": itemEncoding, "path": path})
		}
		out, err := json.Marshal(map[string]interface{}{"count": len(files), "files": files})
		return out, err == nil
	}
	path := strings.TrimSpace(stringValue(payload["path"]))
	content, ok := spaceAgentOptionalFileContent(path)
	if !ok {
		return nil, false
	}
	out, err := json.Marshal(map[string]string{"content": content, "encoding": encoding, "path": path})
	return out, err == nil
}

func spaceAgentFileReadRequestInfo(raw interface{}, defaultEncoding string) (string, string) {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed), defaultEncoding
	case map[string]interface{}:
		path := strings.TrimSpace(stringValue(typed["path"]))
		encoding := strings.TrimSpace(stringValue(typed["encoding"]))
		return path, encoding
	default:
		return "", defaultEncoding
	}
}

func stringValue(value interface{}) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func spaceAgentOptionalFileContent(path string) (string, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"))
	if normalized == "" || !(strings.HasPrefix(normalized, "~/") || strings.HasPrefix(normalized, "/~")) {
		return "", false
	}
	optionalPrefixes := []string{
		"~/meta/",
		"~/.config/",
		"~/dashboard/",
		"~/onscreen-agent/",
		"~/onscreen_agent/",
	}
	matchesPrefix := false
	for _, prefix := range optionalPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			matchesPrefix = true
			break
		}
	}
	if !matchesPrefix {
		return "", false
	}
	optionalNames := []string{
		"login_hooks",
		"dashboard-prefs",
		"dashboard_prefs",
		"prefs",
		"onscreen-agent",
		"onscreen_agent",
		"config",
		"history",
	}
	matchesName := false
	for _, name := range optionalNames {
		if strings.Contains(normalized, name) {
			matchesName = true
			break
		}
	}
	if !matchesName {
		return "", false
	}
	if strings.Contains(normalized, "history") || strings.Contains(normalized, "hooks") {
		return "[]\n", true
	}
	return "{}\n", true
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
