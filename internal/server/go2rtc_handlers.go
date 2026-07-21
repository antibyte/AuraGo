package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"aurago/internal/tools"
)

const go2RTCViewScope = "go2rtc.view"

func registerGo2RTCRoutes(mux *http.ServeMux, s *Server) {
	admin := func(handler http.HandlerFunc) http.HandlerFunc {
		return requireAdmin(s, handler).ServeHTTP
	}
	mux.HandleFunc("/api/go2rtc/status", admin(handleGo2RTCStatus(s)))
	mux.HandleFunc("/api/go2rtc/test", admin(handleGo2RTCTest(s)))
	mux.HandleFunc("/api/go2rtc/start", admin(handleGo2RTCStart(s)))
	mux.HandleFunc("/api/go2rtc/stop", admin(handleGo2RTCStop(s)))
	mux.HandleFunc("/api/go2rtc/restart", admin(handleGo2RTCRestart(s)))
	mux.HandleFunc("/api/go2rtc/app/state", requireGo2RTCView(s, handleGo2RTCAppState(s)))
	mux.HandleFunc("/api/go2rtc/thumbnail/", requireGo2RTCView(s, handleGo2RTCThumbnail(s)))
	mux.HandleFunc("/api/go2rtc/setup/enable", admin(handleGo2RTCSetupEnable(s)))
	mux.HandleFunc("/api/go2rtc/discovery", admin(handleGo2RTCDiscovery(s)))
	mux.HandleFunc("/api/go2rtc/discovery/profiles", admin(handleGo2RTCDiscoveryProfiles(s)))
	mux.HandleFunc("/api/go2rtc/streams", handleGo2RTCStreamsRoute(s))
	mux.HandleFunc("/api/go2rtc/streams/", admin(handleGo2RTCStreamMutation(s)))
	mux.HandleFunc("/api/go2rtc/snapshot", requireGo2RTCView(s, handleGo2RTCSnapshot(s)))
	mux.HandleFunc("/api/go2rtc/viewer/", requireGo2RTCView(s, handleGo2RTCViewer(s)))
	mux.HandleFunc("/api/go2rtc/proxy/", requireGo2RTCView(s, handleGo2RTCProxy(s)))
}

func requireGo2RTCView(s *Server, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if rawToken != "" && s != nil && s.TokenManager != nil {
				if _, ok := s.TokenManager.Validate(rawToken, go2RTCViewScope); ok {
					next(w, r)
					return
				}
				if _, ok := s.TokenManager.Validate(rawToken, "admin"); ok {
					next(w, r)
					return
				}
			}
			jsonError(w, "go2rtc.view scope is required", http.StatusForbidden)
			return
		}
		if s == nil || s.Cfg == nil {
			jsonError(w, "go2rtc is unavailable", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		sessionSecret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if !authEnabled || IsAuthenticated(r, sessionSecret) {
			next(w, r)
			return
		}
		jsonError(w, "Authentication required", http.StatusUnauthorized)
	}
}

func go2RTCManager(s *Server, w http.ResponseWriter) *tools.Go2RTCManager {
	if s == nil || s.Go2RTC == nil {
		jsonError(w, "go2rtc manager is not initialized", http.StatusServiceUnavailable)
		return nil
	}
	return s.Go2RTC
}

func handleGo2RTCStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		writeGo2RTCJSON(w, manager.Status(ctx))
	}
}

func handleGo2RTCTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		status, err := manager.Test(ctx)
		if err != nil {
			writeGo2RTCJSONStatus(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": err.Error(), "runtime": status})
			return
		}
		writeGo2RTCJSON(w, map[string]interface{}{"status": "ok", "runtime": status})
	}
}

func handleGo2RTCStart(s *Server) http.HandlerFunc {
	return go2RTCControlHandler(s, func(ctx context.Context, manager *tools.Go2RTCManager) error {
		if err := manager.StartContainer(ctx); err != nil {
			return err
		}
		_, err := manager.ReconcileStreams(ctx)
		return err
	})
}

func handleGo2RTCStop(s *Server) http.HandlerFunc {
	return go2RTCControlHandler(s, func(_ context.Context, manager *tools.Go2RTCManager) error {
		return manager.StopContainer()
	})
}

func handleGo2RTCRestart(s *Server) http.HandlerFunc {
	return go2RTCControlHandler(s, func(ctx context.Context, manager *tools.Go2RTCManager) error {
		return manager.RestartContainer(ctx)
	})
}

func go2RTCControlHandler(s *Server, action func(context.Context, *tools.Go2RTCManager) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		if err := action(ctx, manager); err != nil {
			writeGo2RTCJSONStatus(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		writeGo2RTCJSON(w, map[string]interface{}{"status": "ok"})
	}
}

func handleGo2RTCStreams(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		streams, err := manager.ListStreams(ctx)
		if err != nil {
			writeGo2RTCJSONStatus(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		writeGo2RTCJSON(w, map[string]interface{}{"status": "ok", "streams": streams})
	}
}

func handleGo2RTCSnapshot(s *Server) http.HandlerFunc {
	type request struct {
		StreamID     string `json:"stream_id"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		Rotate       int    `json:"rotate"`
		CacheSeconds int    `json:"cache_seconds"`
		Store        *bool  `json:"store"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		var payload request
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&payload); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		store := false
		if payload.Store != nil {
			store = *payload.Store
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		result, _, err := manager.Snapshot(ctx, payload.StreamID, tools.Go2RTCSnapshotOptions{
			Width: payload.Width, Height: payload.Height, Rotate: payload.Rotate,
			CacheSeconds: payload.CacheSeconds, Store: store,
		})
		if err != nil {
			writeGo2RTCJSONStatus(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		writeGo2RTCJSON(w, result)
	}
}

func handleGo2RTCViewer(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		streamID, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/go2rtc/viewer/"))
		if err != nil || strings.TrimSpace(streamID) == "" {
			jsonError(w, "Invalid stream ID", http.StatusBadRequest)
			return
		}
		if _, err := manager.EnabledStreamAlias(streamID); err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		mode := "mse,hls,mp4,mjpeg"
		if manager.Config().WebRTC.Enabled {
			mode = "webrtc," + mode
		}
		nonceBytes := make([]byte, 18)
		if _, err := rand.Read(nonceBytes); err != nil {
			jsonError(w, "Failed to create viewer security context", http.StatusInternalServerError)
			return
		}
		nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
		streamJSON, _ := json.Marshal(streamID)
		modeJSON, _ := json.Marshal(mode)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'nonce-"+nonce+"'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self' ws: wss:; frame-ancestors 'self';")
		_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>%s</title><style>html,body{margin:0;width:100%%;height:100%%;background:#090b0f}video-stream{display:block;width:100%%;height:100%%;object-fit:contain}</style></head><body><script type="module" nonce="%s">import "/api/go2rtc/proxy/video-stream.js";const video=document.createElement("video-stream");video.src="/api/go2rtc/proxy/api/ws?src="+encodeURIComponent(%s);video.mode=%s;document.body.appendChild(video);</script></body></html>`, html.EscapeString(streamID), nonce, streamJSON, modeJSON)
	}
}

func handleGo2RTCProxy(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		cfg := manager.Config()
		if !cfg.Enabled {
			jsonError(w, "go2rtc integration is disabled", http.StatusServiceUnavailable)
			return
		}
		subpath := strings.TrimPrefix(r.URL.Path, "/api/go2rtc/proxy/")
		adminUI := go2RTCProxyNeedsAdmin(subpath)
		if adminUI {
			if !cfg.WebUIEnabled {
				http.NotFound(w, r)
				return
			}
			if !go2RTCRequestIsAdmin(s, r) {
				jsonError(w, "Admin scope is required for the go2rtc web UI", http.StatusForbidden)
				return
			}
		}
		if !go2RTCProxyPathAllowed(subpath, r.Method, adminUI) {
			http.NotFound(w, r)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		query := r.URL.Query()
		if strings.HasPrefix(subpath, "api/hls/") {
			sanitized, err := sanitizeGo2RTCHLSQuery(query)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			query = sanitized
			r.URL.RawQuery = query.Encode()
		}
		rawSrc := strings.TrimSpace(query.Get("src"))
		if go2RTCProxyRequiresStream(subpath) && rawSrc == "" {
			jsonError(w, "A configured stream id is required", http.StatusBadRequest)
			return
		}
		if rawSrc != "" {
			alias, err := manager.EnabledStreamAlias(rawSrc)
			if err != nil {
				jsonError(w, "Stream is not configured or enabled", http.StatusForbidden)
				return
			}
			query.Set("src", alias)
			r.URL.RawQuery = query.Encode()
		}
		target, err := manager.ProxyTarget()
		if err != nil {
			jsonError(w, "Invalid go2rtc upstream", http.StatusServiceUnavailable)
			return
		}
		username, password, err := manager.ProxyCredentials()
		if err != nil {
			jsonError(w, "go2rtc credential unavailable", http.StatusServiceUnavailable)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Header.Del("Cookie")
			req.Header.Del("Authorization")
			req.Header.Del("Proxy-Authorization")
			req.SetBasicAuth(username, password)
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			resp.Header.Set("X-Content-Type-Options", "nosniff")
			if subpath == "api/streams" && r.Method != http.MethodHead {
				if err := maskGo2RTCStreamsResponse(resp); err != nil {
					return err
				}
			}
			return nil
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, proxyErr error) {
			if s.Logger != nil {
				s.Logger.Warn("[go2rtc] Proxy request failed", "path", subpath, "error", proxyErr)
			}
			jsonError(rw, "go2rtc upstream unavailable", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	}
}

func maskGo2RTCStreamsResponse(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("go2rtc streams response is empty")
	}
	const maxBody = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("read go2rtc streams response: %w", err)
	}
	if len(data) > maxBody {
		return fmt.Errorf("go2rtc streams response exceeds safe limit")
	}
	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode go2rtc streams response")
	}
	maskGo2RTCRuntimeSources(payload)
	data, err = json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode sanitized go2rtc streams response")
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))
	resp.Header.Set("Content-Type", "application/json")
	return nil
}

func maskGo2RTCRuntimeSources(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "url", "source", "src":
				typed[key] = "••••••••"
			default:
				maskGo2RTCRuntimeSources(child)
			}
		}
	case []interface{}:
		for _, child := range typed {
			maskGo2RTCRuntimeSources(child)
		}
	}
}

func go2RTCProxyPathAllowed(path, method string, adminUI bool) bool {
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return false
	}
	if adminUI {
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
			return false
		}
		for _, blocked := range []string{
			"api/config", "api/log", "api/exit", "api/restart", "api/onvif",
			"add", "add.html", "config", "config.html", "log", "log.html",
		} {
			if path == blocked || strings.HasPrefix(path, blocked+"/") {
				return false
			}
		}
		return true
	}
	switch {
	case path == "stream.html", path == "video-rtc.js", path == "video-stream.js", path == "video-rtc.css":
		return method == http.MethodGet || method == http.MethodHead
	case path == "api/frame.jpeg", path == "api/ws", path == "api/stream.m3u8", path == "api/stream.mp4", path == "api/stream.mjpeg":
		return method == http.MethodGet || method == http.MethodHead
	case path == "api/hls/playlist.m3u8", path == "api/hls/segment.ts",
		path == "api/hls/init.mp4", path == "api/hls/segment.m4s":
		return method == http.MethodGet || method == http.MethodHead
	case path == "api/webrtc":
		return method == http.MethodPost || method == http.MethodOptions
	default:
		return false
	}
}

func go2RTCProxyRequiresStream(path string) bool {
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	switch path {
	case "api/frame.jpeg", "api/ws", "api/stream.m3u8", "api/stream.mp4", "api/stream.mjpeg", "api/webrtc":
		return true
	default:
		return false
	}
}

func sanitizeGo2RTCHLSQuery(query url.Values) (url.Values, error) {
	if strings.TrimSpace(query.Get("src")) != "" {
		return nil, fmt.Errorf("HLS segment requests must use the authorized session id")
	}
	sessionID := strings.TrimSpace(query.Get("id"))
	if len(sessionID) != 8 {
		return nil, fmt.Errorf("a valid HLS session id is required")
	}
	for _, r := range sessionID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return nil, fmt.Errorf("a valid HLS session id is required")
		}
	}
	sanitized := url.Values{"id": {sessionID}}
	if sequence := strings.TrimSpace(query.Get("n")); sequence != "" {
		for _, r := range sequence {
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("invalid HLS segment sequence")
			}
		}
		if len(sequence) > 12 {
			return nil, fmt.Errorf("invalid HLS segment sequence")
		}
		sanitized.Set("n", sequence)
	}
	return sanitized, nil
}

func go2RTCProxyNeedsAdmin(path string) bool {
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	switch {
	case path == "stream.html", path == "video-rtc.js", path == "video-stream.js", path == "video-rtc.css":
		return false
	case strings.HasPrefix(path, "api/frame.jpeg"), strings.HasPrefix(path, "api/ws"),
		strings.HasPrefix(path, "api/stream."), strings.HasPrefix(path, "api/hls/"),
		strings.HasPrefix(path, "api/webrtc"):
		return false
	default:
		return true
	}
}

func go2RTCRequestIsAdmin(s *Server, r *http.Request) bool {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if s != nil && s.TokenManager != nil {
			_, ok := s.TokenManager.Validate(token, "admin")
			return ok
		}
		return false
	}
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	enabled := s.Cfg.Auth.Enabled
	secret := s.Cfg.Auth.SessionSecret
	s.CfgMu.RUnlock()
	return !enabled || IsAuthenticated(r, secret)
}

func writeGo2RTCJSON(w http.ResponseWriter, value interface{}) {
	writeGo2RTCJSONStatus(w, http.StatusOK, value)
}

func writeGo2RTCJSONStatus(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
