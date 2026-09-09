package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/webassets"
)

//go:embed recovery.html
var recoveryHTML string

var recoveryTemplate = template.Must(template.New("recovery").Parse(recoveryHTML))

// RunAssetRecovery gives a freshly copied binary a local repair page without
// creating a vault, installing a service, or initializing integrations.
// Existing configurations always follow normal startup and authentication.
func RunAssetRecovery(address string, logger *slog.Logger) error {
	cfg := &config.Config{}
	cfg.Server.UILanguage = "en"
	s := &Server{Cfg: cfg, Logger: logger}
	mux := http.NewServeMux()
	s.registerAssetRecoveryRoutes(mux)
	s.registerRecoveryUI(mux)
	mux.HandleFunc("/api/auth/status", handleAuthStatus(s))
	logger.Info("Web resources required; recovery page available", "address", address, "asset_set", webassets.Default.Pin.ID)
	httpServer := &http.Server{Addr: address, Handler: securityHeadersMiddleware(authMiddleware(s, mux), false, false), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	return httpServer.ListenAndServe()
}

func (s *Server) recoveryPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.CfgMu.RLock()
	lang := normalizeLang(s.Cfg.Server.UILanguage)
	s.CfgMu.RUnlock()
	_ = recoveryTemplate.Execute(w, map[string]any{"Lang": lang, "Pin": webassets.Default.Pin})
}

func (s *Server) registerRecoveryUI(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/setup", "/config", "/desktop":
			s.recoveryPage(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handleAuthLogin(s)(w, r)
			return
		}
		s.recoveryPage(w, r)
	})
	mux.HandleFunc("/auth/logout", handleAuthLogout(s))
}

func (s *Server) registerAssetRecoveryRoutes(mux *http.ServeMux) {
	mux.Handle("/api/assets/status", requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"pin": webassets.Default.Pin, "ready": webassets.Default.Ready()})
	})))
	mux.Handle("/api/assets/install", requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Enforce same-origin even when ordinary authentication is disabled.
		if !checkCSRFOriginWithPolicy(r, true) {
			http.Error(w, "csrf_check_failed", http.StatusForbidden)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
		defer cancel()
		err := webassets.Default.Download(ctx)
		if err != nil {
			s.Logger.Warn("Asset installation failed", "error", err)
			http.Error(w, "asset_install_failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"installed": true, "restart_required": true})
	})))
	mux.Handle("/api/assets/restart", requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !checkCSRFOriginWithPolicy(r, true) {
			http.Error(w, "csrf_check_failed", http.StatusForbidden)
			return
		}
		handleRestart(s)(w, r)
	})))
}

func versionedUIHandler(files fs.FS, version string) http.Handler {
	static := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		requested := r.URL.Query().Get("v")
		if requested != "" && requested != version {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "Asset version changed. Reload this page.", http.StatusConflict)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		info, err := fs.Stat(files, name)
		if err == nil && info.IsDir() {
			http.NotFound(w, r)
			return
		}
		// Unversioned CSS/vendor subresources must be revalidated after upgrades.
		w.Header().Set("Cache-Control", "no-store")
		if requested != "" {
			w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		}
		static.ServeHTTP(w, r)
	})
}
