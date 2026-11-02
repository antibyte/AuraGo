package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/findit"
	"aurago/internal/tools"
	"gopkg.in/yaml.v3"
)

const findItAPI = "/api/desktop/findit"

type findItRuntime struct {
	store   *findit.Store
	dataDir string
	err     error
}

func (s *Server) startFindIt() {
	s.findItMu.Lock()
	defer s.findItMu.Unlock()
	if s.findIt != nil {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || cfg.Directories.DataDir == "" {
		return
	}
	dir, err := filepath.Abs(cfg.Directories.DataDir)
	runtime := &findItRuntime{dataDir: dir, err: err}
	if err == nil {
		runtime.store, runtime.err = findit.Open(filepath.Join(dir, "findit.db"))
	}
	s.findIt = runtime
	if runtime.err != nil {
		logger := s.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("FindIt storage initialization failed", "error", runtime.err)
		return
	}
	tools.SetFindItStore(runtime.store, func() *config.Config {
		current := s.ConfigSnapshot()
		if current == nil {
			return nil
		}
		currentDir, err := filepath.Abs(current.Directories.DataDir)
		if err != nil || currentDir != runtime.dataDir {
			return nil
		}
		return current
	})
}

func (s *Server) closeFindIt() {
	s.findItMu.Lock()
	defer s.findItMu.Unlock()
	if s.findIt != nil && s.findIt.store != nil {
		tools.WithdrawFindItStore(s.findIt.store)
		_ = s.findIt.store.Close()
	}
	s.findIt = nil
}

func registerFindItRoutes(mux *http.ServeMux, s *Server) {
	handler := handleFindIt(s)
	mux.HandleFunc(findItAPI, handler)
	mux.HandleFunc(findItAPI+"/", handler)
}

func findItJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func findItHTTPError(w http.ResponseWriter, status int, code string) {
	findItJSON(w, status, map[string]string{"error": code, "code": code})
}

func findItStoreError(w http.ResponseWriter, err error) {
	code := findit.ErrorCode(err)
	status := http.StatusBadRequest
	switch code {
	case "not_found":
		status = http.StatusNotFound
	case "conflict":
		status = http.StatusConflict
	case "unavailable":
		status = http.StatusServiceUnavailable
	case "invalid":
		status = http.StatusUnprocessableEntity
	default:
		code, status = "unavailable", http.StatusServiceUnavailable
	}
	if errors.Is(err, context.Canceled) {
		code, status = "policy_changed", http.StatusConflict
	}
	detail := findit.ErrorDetail(err)
	findItJSON(w, status, map[string]any{"error": code, "code": code, "reason": detail.Reason, "field": detail.Field, "details": detail.Details})
}

func decodeFindItJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		findItDecodeError(w, err)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		findItDecodeError(w, err)
		return false
	}
	return true
}

func findItDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		findItHTTPError(w, http.StatusRequestEntityTooLarge, "request_too_large")
		return
	}
	findItHTTPError(w, http.StatusBadRequest, "invalid_json")
}

func findItConfigRevision(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	data, _ := json.Marshal(cfg.FindIt)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func findItHasScope(s *Server, r *http.Request, scope string) bool {
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		return token != "" && desktopTokenHasScope(s, token, scope)
	}
	cfg := s.ConfigSnapshot()
	return cfg != nil && (!cfg.Auth.Enabled || IsAuthenticated(r, cfg.Auth.SessionSecret))
}

func findItCanManage(s *Server, r *http.Request) bool {
	if !findItHasScope(s, r, desktopScopeAdmin) {
		return false
	}
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		manager := s.currentTokenManager()
		if manager == nil {
			return false
		}
		_, ok := manager.Validate(token, "admin")
		return ok
	}
	return true
}

func (s *Server) findItStatus(r *http.Request) map[string]any {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return map[string]any{"enabled": false, "ready": false}
	}
	s.findItMu.RLock()
	dir, dirErr := filepath.Abs(cfg.Directories.DataDir)
	ready := dirErr == nil && s.findIt != nil && s.findIt.store != nil && s.findIt.err == nil && s.findIt.dataDir == dir
	s.findItMu.RUnlock()
	return map[string]any{
		"enabled": cfg.FindIt.Enabled, "agent_access": cfg.FindIt.AgentAccess,
		"readonly":        cfg.FindIt.ReadOnly || cfg.VirtualDesktop.ReadOnly,
		"findit_readonly": cfg.FindIt.ReadOnly, "ready": ready,
		"can_manage":      findItCanManage(s, r) && !cfg.VirtualDesktop.ReadOnly,
		"can_print":       !cfg.VirtualDesktop.ReadOnly && findItHasScope(s, r, desktopScopeWrite),
		"can_write":       ready && cfg.VirtualDesktop.Enabled && cfg.FindIt.Enabled && !cfg.FindIt.ReadOnly && !cfg.VirtualDesktop.ReadOnly && findItHasScope(s, r, desktopScopeWrite),
		"config_revision": findItConfigRevision(cfg),
	}
}

// Read results must not outlive the permission that admitted them. This also
// catches bearer revocation while a query or CSV preview was running.
func (s *Server) allowFindItReadResult(w http.ResponseWriter, r *http.Request, admitted *config.Config) bool {
	current := s.ConfigSnapshot()
	if current == nil || !current.VirtualDesktop.Enabled || !current.FindIt.Enabled || current.Directories.DataDir != admitted.Directories.DataDir || !findItHasScope(s, r, desktopScopeRead) || r.Context().Err() != nil {
		findItHTTPError(w, http.StatusForbidden, "policy_changed")
		return false
	}
	return true
}

func (s *Server) findItReadError(w http.ResponseWriter, r *http.Request, admitted *config.Config, err error) {
	if s.allowFindItReadResult(w, r, admitted) {
		findItStoreError(w, err)
	}
}

func handleFindIt(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		path := strings.TrimPrefix(r.URL.Path, findItAPI)
		if path == "" {
			path = "/status"
		}
		read := r.Method == http.MethodGet || (r.Method == http.MethodPost && (path == "/query" || path == "/import/preview"))
		scope, operation := desktopScopeRead, desktopRead
		if !read {
			scope, operation = desktopScopeWrite, desktopWrite
		}
		if path == "/settings" {
			scope = desktopScopeAdmin
		}
		if !requireDesktopOperation(s, w, r, scope, operation) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if _, bearer := bearerCredential(r.Header.Get("Authorization")); !bearer && !checkCSRFOriginWithPolicy(r, true) {
				findItHTTPError(w, http.StatusForbidden, "forbidden_origin")
				return
			}
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.VirtualDesktop.Enabled {
			findItHTTPError(w, 403, "desktop_disabled")
			return
		}
		if path == "/status" && r.Method == http.MethodGet {
			findItJSON(w, 200, s.findItStatus(r))
			return
		}
		if path == "/settings" && r.Method == http.MethodPost {
			s.saveFindItSettings(w, r)
			return
		}
		if !cfg.FindIt.Enabled {
			findItHTTPError(w, 403, "findit_disabled")
			return
		}
		if !read && cfg.FindIt.ReadOnly {
			findItHTTPError(w, 403, "findit_readonly")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		s.findItMu.RLock()
		defer s.findItMu.RUnlock()
		runtime := s.findIt
		dir, _ := filepath.Abs(cfg.Directories.DataDir)
		if runtime == nil || runtime.store == nil || runtime.err != nil || runtime.dataDir != dir {
			findItHTTPError(w, 503, "unavailable")
			return
		}
		// A request admitted before a policy replacement must not acquire a new grant.
		current := s.ConfigSnapshot()
		if current == nil || !current.VirtualDesktop.Enabled || !current.FindIt.Enabled || current.Directories.DataDir != cfg.Directories.DataDir || (!read && (current.FindIt.ReadOnly || current.VirtualDesktop.ReadOnly)) {
			findItHTTPError(w, 403, "policy_changed")
			return
		}
		switch {
		case path == "/catalog" && r.Method == http.MethodGet:
			catalog, err := runtime.store.Catalog(ctx)
			if err != nil {
				s.findItReadError(w, r, cfg, err)
				return
			}
			if !s.allowFindItReadResult(w, r, cfg) {
				return
			}
			findItJSON(w, 200, catalog)
		case path == "/query" && r.Method == http.MethodPost:
			var req findit.QueryRequest
			if !decodeFindItJSON(w, r, &req, 128<<10) {
				return
			}
			result, err := runtime.store.Query(ctx, req)
			if err != nil {
				s.findItReadError(w, r, cfg, err)
				return
			}
			if !s.allowFindItReadResult(w, r, cfg) {
				return
			}
			findItJSON(w, 200, result)
		case path == "/command" && r.Method == http.MethodPost:
			var req findit.CommandRequest
			if !decodeFindItJSON(w, r, &req, 1<<20) {
				return
			}
			if strings.TrimSpace(req.RequestID) == "" {
				findItHTTPError(w, 422, "request_id_required")
				return
			}
			result, err := runtime.store.Execute(ctx, req)
			if err != nil {
				findItStoreError(w, err)
				return
			}
			findItJSON(w, 200, result)
		case (path == "/import/preview" || path == "/import") && r.Method == http.MethodPost:
			var req findit.ImportRequest
			// JSON escaping can expand the domain's bounded 10 MiB CSV by up to 6x.
			if !decodeFindItJSON(w, r, &req, 64<<20) {
				return
			}
			var result any
			var err error
			if path == "/import/preview" {
				result, err = runtime.store.PreviewCSV(ctx, req)
			} else {
				result, err = runtime.store.ImportCSV(ctx, req)
			}
			if err != nil {
				if path == "/import/preview" {
					s.findItReadError(w, r, cfg, err)
				} else {
					findItStoreError(w, err)
				}
				return
			}
			if path == "/import/preview" && !s.allowFindItReadResult(w, r, cfg) {
				return
			}
			findItJSON(w, 200, result)
		case path == "/export" && r.Method == http.MethodGet:
			data, err := runtime.store.ExportCSV(ctx, r.URL.Query().Get("entity"), r.URL.Query().Get("project_id"))
			if err != nil {
				s.findItReadError(w, r, cfg, err)
				return
			}
			if !s.allowFindItReadResult(w, r, cfg) {
				return
			}
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="findit.csv"`)
			_, _ = w.Write(data)
		case path == "/images" && r.Method == http.MethodPost:
			s.uploadFindItImage(w, r, runtime)
		case strings.HasPrefix(path, "/images/") && r.Method == http.MethodGet:
			if s.allowFindItReadResult(w, r, cfg) {
				serveFindItImage(w, r, runtime, strings.TrimPrefix(path, "/images/"))
			}
		default:
			findItHTTPError(w, 405, "method_not_allowed")
		}
	}
}

func (s *Server) saveFindItSettings(w http.ResponseWriter, r *http.Request) {
	if !findItCanManage(s, r) {
		findItHTTPError(w, 403, "admin_required")
		return
	}
	var req struct {
		Enabled         bool   `json:"enabled"`
		AgentAccess     bool   `json:"agent_access"`
		ReadOnly        bool   `json:"readonly"`
		ExpectedVersion string `json:"expected_version"`
	}
	if !decodeFindItJSON(w, r, &req, 4096) {
		return
	}
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	cfg := s.ConfigSnapshot()
	if req.ExpectedVersion == "" || req.ExpectedVersion != findItConfigRevision(cfg) {
		findItHTTPError(w, 409, "conflict")
		return
	}
	before, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		findItHTTPError(w, 503, "config_unavailable")
		return
	}
	var raw map[string]any
	if yaml.Unmarshal(before, &raw) != nil {
		findItHTTPError(w, 503, "config_unavailable")
		return
	}
	raw["findit"] = config.FindItConfig{Enabled: req.Enabled, AgentAccess: req.AgentAccess, ReadOnly: req.ReadOnly}
	out, err := yaml.Marshal(raw)
	if err != nil {
		findItHTTPError(w, 503, "config_unavailable")
		return
	}
	next, err := saveProviderConfig(s, cfg.ConfigPath, before, out, nil)
	if err != nil {
		findItHTTPError(w, 503, "config_save_failed")
		return
	}
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	findItJSON(w, 200, s.findItStatus(r))
}
