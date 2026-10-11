package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/ruview"
	"aurago/internal/webassets"
)

func registerRuViewRoutes(mux *http.ServeMux, s *Server) {
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
		if strings.HasPrefix(rel, "ui/") {
			if r.Method != http.MethodGet || !s.ConfigSnapshot().RuView.Enabled {
				jsonError(w, "RuView disabled", 403)
				return
			}
			name := strings.TrimPrefix(rel, "ui/")
			if name == "" {
				name = "index.html"
			}
			if path.Clean(name) != name || strings.ContainsAny(name, "\\%") || strings.HasPrefix(name, ".") {
				http.NotFound(w, r)
				return
			}
			data, err := fs.ReadFile(webassets.Namespace("ui"), "js/vendor/ruview/"+name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; worker-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'none'")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("Permissions-Policy", "serial=(), camera=(), microphone=(), geolocation=()")
			w.Header().Set("Cache-Control", "no-store")
			switch path.Ext(name) {
			case ".js":
				w.Header().Set("Content-Type", "application/javascript")
			case ".css":
				w.Header().Set("Content-Type", "text/css")
			case ".html":
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			}
			_, _ = w.Write(data)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet {
			switch rel {
			case "status":
				_ = json.NewEncoder(w).Encode(s.RuView.Status())
				return
			case "capabilities":
				_ = json.NewEncoder(w).Encode(map[string]any{"revision": ruview.Revision, "catalog": ruview.Artifacts(), "unavailable": []string{"model_load", "model_unload", "lora_activate", "pose_stats", "demo_zones"}})
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
		if rel != "test" && (s.ConfigSnapshot().RuView.ReadOnly || !s.ConfigSnapshot().RuView.Enabled) {
			jsonError(w, "RuView is disabled or read-only", 403)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()
		var err error
		switch rel {
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
