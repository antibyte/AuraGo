package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/ruview"
)

// ruViewUIPolicy is the framed RuView UI policy. The sandbox has no
// allow-same-origin, so its origin is null and 'self' does not match the
// scripts served from this host. connect-src stays none: the frame cannot
// fetch or open a socket. The Desktop parent relays reviewed calls.
func ruViewUIPolicy(r *http.Request, nonce string) string {
	host := ""
	if r != nil {
		host = r.Host
	}
	origins := ""
	if host != "" && !strings.ContainsAny(host, " ;'\"\\") {
		origins = " http://" + host + " https://" + host
	}
	return "default-src 'none'; script-src 'self'" + origins + " 'nonce-" + nonce + "' 'wasm-unsafe-eval'; style-src 'self'" + origins + " 'unsafe-inline'; img-src 'self'" + origins + " data: blob:; font-src 'self'" + origins + "; connect-src 'none'; worker-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'none'"
}

// serveRuViewUI serves the pinned upstream files. The opaque frame cannot
// send the session cookie, so this route stays outside the admin gate. The
// bytes contain no session, measurement, or credential. Proxy and status stay
// admin-only. Access-Control-Allow-Origin is * without credentials: a null
// origin with credentials would let every sandboxed page read the response.
func serveRuViewUI(s *Server, w http.ResponseWriter, r *http.Request) {
	if s == nil || s.RuView == nil {
		jsonError(w, "RuView unavailable", 503)
		return
	}
	if r.Method != http.MethodGet || !s.ConfigSnapshot().RuView.Enabled {
		jsonError(w, "RuView disabled", 403)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/ruview/ui/")
	if name == "" {
		name = "index.html"
	}
	if path.Clean(name) != name || strings.ContainsAny(name, "\\%") || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(uiFiles, "js/vendor/ruview/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	nonce := rand.Text()
	if path.Ext(name) == ".html" {
		data = bytes.ReplaceAll(data, []byte("<script"), []byte("<script nonce=\""+nonce+"\""))
	}
	w.Header().Set("Content-Security-Policy", ruViewUIPolicy(r, nonce))
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Permissions-Policy", "serial=(), camera=(), microphone=(), geolocation=()")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	switch path.Ext(name) {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".wasm":
		w.Header().Set("Content-Type", "application/wasm")
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = w.Write(data)
}

func registerRuViewRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/ruview/ui/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveRuViewUI(s, w, r)
	}))
	mux.Handle("/api/ruview/", requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.RuView == nil {
			jsonError(w, "RuView unavailable", 503)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/api/ruview/")
		if strings.HasPrefix(rel, "proxy/") {
			_, write, stream := ruview.Endpoint(r.Method, "/"+strings.TrimPrefix(rel, "proxy/"))
			if (write && !checkCSRFOriginWithPolicy(r, true)) || (stream && !desktop.SameHostWebSocketOrigin(r)) {
				jsonError(w, "Origin rejected", 403)
				return
			}
			s.RuView.Proxy(w, r, "/"+strings.TrimPrefix(rel, "proxy/"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet {
			switch rel {
			case "nodes":
				if !s.ConfigSnapshot().RuView.Enabled {
					jsonError(w, "RuView is disabled", http.StatusForbidden)
					return
				}
				nodes, err := s.RuView.Nodes(r.Context())
				if err != nil {
					jsonError(w, err.Error(), http.StatusBadGateway)
					return
				}
				_ = json.NewEncoder(w).Encode(nodes)
				return
			case "status":
				_ = json.NewEncoder(w).Encode(s.RuView.Status())
				return
			case "board-setup":
				_ = json.NewEncoder(w).Encode(s.RuView.BoardSetup())
				return
			case "capabilities":
				_ = json.NewEncoder(w).Encode(map[string]any{"revision": ruview.Revision, "catalog": ruview.Artifacts(), "unavailable": []string{"model_load", "model_unload", "lora_activate", "pose_stats", "demo_zones", "camera", "timed_recording"}})
				return
			case "firmware":
				_ = json.NewEncoder(w).Encode(ruview.Artifacts())
				return
			}
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !checkCSRFOriginWithPolicy(r, true) {
			jsonError(w, "Origin rejected", 403)
			return
		}
		cfg := s.ConfigSnapshot().RuView
		if rel != "test" && (!cfg.Enabled || cfg.ReadOnly && !ruview.IsStop(r.Method, rel)) {
			jsonError(w, "RuView is disabled or read-only", 403)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()
		var err error
		switch rel {
		case "nodes":
			var body struct {
				NodeID int `json:"node_id"`
			}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || body.NodeID < 1 || body.NodeID > 255 {
				jsonError(w, "Invalid RuView node ID", http.StatusBadRequest)
				return
			}
			err = s.RuView.RememberNode(ctx, body.NodeID)
		case "test":
			var status ruview.Status
			status, err = s.RuView.Test(ctx)
			if err == nil {
				_ = json.NewEncoder(w).Encode(status)
				return
			}
		case "runtime/install":
			err = s.RuView.Install(ctx)
		case "runtime/start":
			err = s.RuView.Start(ctx)
		case "runtime/stop":
			if cfg.Mode == "external" {
				jsonError(w, "External RuView is managed by its operator", http.StatusForbidden)
				return
			}
			err = s.RuView.Stop(ctx)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(s.RuView.Status())
	})))
}
