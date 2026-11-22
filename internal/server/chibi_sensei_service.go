package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"aurago/internal/chibisensei"
	"aurago/internal/config"
	"aurago/internal/desktop"
)

const (
	chibiJSONLimit     = 1 << 20
	chibiRowsJSONLimit = 16 << 20
)

// chibiSenseiRuntime is the lazily opened state of the app: the dataset store of one data directory.
type chibiSenseiRuntime struct {
	dataDir string
	store   *chibisensei.Store
}

// chibiSenseiAccess is what a handler gets after the gates passed.
type chibiSenseiAccess struct {
	cfg *config.Config
	rt  *chibiSenseiRuntime
	svc *desktop.Service
}

// chibiSenseiRuntime returns the runtime for the current data directory, opening
// <data_dir>/chibi_sensei.db on first use.
func (s *Server) chibiSenseiRuntime() (*chibiSenseiRuntime, error) {
	if s == nil {
		return nil, errors.New("chibi sensei unavailable")
	}
	s.chibiSenseiMu.Lock()
	defer s.chibiSenseiMu.Unlock()
	if s.chibiSenseiClosed {
		return nil, errors.New("chibi sensei is shutting down")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" {
		return nil, errors.New("the data directory is unavailable")
	}
	dataDir := normalizedVideoStudioRoot(cfg.Directories.DataDir)
	if s.chibiSensei != nil {
		if s.chibiSensei.dataDir == dataDir {
			return s.chibiSensei, nil
		}
		old := s.chibiSensei
		s.chibiSensei = nil
		_ = old.store.Close()
	}
	store, err := chibisensei.OpenStore(filepath.Join(dataDir, config.ChibiSenseiDBFilename))
	if err != nil {
		return nil, err
	}
	s.chibiSensei = &chibiSenseiRuntime{dataDir: dataDir, store: store}
	return s.chibiSensei, nil
}

// closeChibiSensei closes the store and refuses to reopen it.
func (s *Server) closeChibiSensei() {
	if s == nil {
		return
	}
	s.chibiSenseiMu.Lock()
	s.chibiSenseiClosed = true
	rt := s.chibiSensei
	s.chibiSensei = nil
	s.chibiSenseiMu.Unlock()
	if rt != nil {
		_ = rt.store.Close()
	}
}

// initChibiSensei opens the runtime at boot when the app is enabled. A failure only logs a warning;
// the routes answer 503 storage_unavailable until the store can be opened.
func (s *Server) initChibiSensei(ctx context.Context) {
	if s == nil || ctx.Err() != nil || !s.chibiSenseiAvailable() {
		return
	}
	if _, err := s.chibiSenseiRuntime(); err != nil && s.Logger != nil {
		s.Logger.Warn("[ChibiSensei] Store initialization failed")
	}
}

// chibiSenseiAvailable is the "chibi_sensei" capability of the desktop app list.
func (s *Server) chibiSenseiAvailable() bool {
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.ChibiSensei.Enabled && cfg.VirtualDesktop.Enabled
}

// normalizedChibiSenseiConfig returns a copy of the configuration with the Chibi Sensei section
// normalized.
func normalizedChibiSenseiConfig(s *Server) (*config.Config, error) {
	if s == nil {
		return nil, errors.New("server unavailable")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, errors.New("configuration unavailable")
	}
	cp := *cfg
	if err := config.NormalizeChibiSenseiConfig(&cp.ChibiSensei); err != nil {
		return nil, err
	}
	return &cp, nil
}

func writeChibiJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeChibiError writes the flat error document of the desktop APIs:
// {"error": code, "code": code, "message": ..., "issues"?: [...]}. Issues only go with a 422.
func writeChibiError(w http.ResponseWriter, status int, code, message string, issues []chibisensei.Issue) {
	body := map[string]any{"error": code, "code": code, "message": message}
	if status == http.StatusUnprocessableEntity && len(issues) > 0 {
		body["issues"] = issues
	}
	writeChibiJSON(w, status, body)
}

func writeChibiMethodError(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeChibiError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed for this endpoint.", nil)
}

// chibiDecode decodes exactly one JSON document with unknown fields refused, bounded by limit.
// Bad JSON answers 400 invalid_request, an oversize body 413 too_large. It returns false after answering.
func chibiDecode(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(dst)
	if err == nil {
		if _, err2 := d.Token(); err2 != io.EOF {
			err = errors.New("trailing JSON data")
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeChibiError(w, http.StatusRequestEntityTooLarge, "too_large", "The request is too large.", nil)
			return false
		}
		writeChibiError(w, http.StatusBadRequest, "invalid_request", "The request body is not valid JSON for this endpoint.", nil)
		return false
	}
	return true
}

// chibiRequestIsAdmin: a Bearer token needs the "admin" scope; a session or disabled auth is the
// built-in admin.
func chibiRequestIsAdmin(s *Server, r *http.Request) bool {
	if rawToken, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer {
		mgr := s.currentTokenManager()
		if rawToken == "" || mgr == nil {
			return false
		}
		_, ok := mgr.Validate(rawToken, "admin")
		return ok
	}
	return true
}

// chibiSenseiAuthorize runs the gates every endpoint shares: desktop scope and operation (read-only
// refuses mutations with 403 desktop_readonly), the Chibi Sensei feature flag (403 chibi_disabled),
// the desktop being enabled and the runtime. withService also resolves the desktop workspace.
func chibiSenseiAuthorize(s *Server, w http.ResponseWriter, r *http.Request, scope string, op desktopOperation, withService bool) (*chibiSenseiAccess, bool) {
	if !requireDesktopOperation(s, w, r, scope, op) {
		return nil, false
	}
	cfg, err := normalizedChibiSenseiConfig(s)
	if err != nil {
		writeChibiError(w, http.StatusInternalServerError, "configuration_invalid", "The Chibi Sensei configuration is invalid.", nil)
		return nil, false
	}
	if !cfg.ChibiSensei.Enabled {
		writeChibiError(w, http.StatusForbidden, "chibi_disabled", "Chibi Sensei is disabled in the configuration.", nil)
		return nil, false
	}
	if !cfg.VirtualDesktop.Enabled {
		writeChibiError(w, http.StatusServiceUnavailable, "desktop_disabled", "The virtual desktop is disabled in the configuration.", nil)
		return nil, false
	}
	rt, err := s.chibiSenseiRuntime()
	if err != nil {
		writeChibiError(w, http.StatusServiceUnavailable, "storage_unavailable", "Chibi Sensei storage is unavailable.", nil)
		return nil, false
	}
	acc := &chibiSenseiAccess{cfg: cfg, rt: rt}
	if withService {
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			writeChibiError(w, http.StatusServiceUnavailable, "desktop_unavailable", "The virtual desktop storage is unavailable.", nil)
			return nil, false
		}
		acc.svc = svc
	}
	return acc, true
}

// chibiGeneratorStatus reports whether generation jobs can run and which model they would use.
// A configured provider must exist and allow general chat; otherwise the main LLM client counts.
func (s *Server) chibiGeneratorStatus(cfg *config.Config) (available bool, model string) {
	model = cfg.ChibiSensei.Generator.Model
	if id := cfg.ChibiSensei.Generator.Provider; id != "" {
		p := cfg.FindProvider(id)
		available = p != nil && config.ProviderUsageAllowsGeneralChat(p)
		if model == "" && p != nil {
			model = p.Model
		}
	} else {
		available = s.LLMClient != nil
	}
	if model == "" {
		model = cfg.LLM.Model
	}
	return available, model
}

// handleChibiMeta answers GET /meta: the limits, recipes and generator state the UI needs.
func handleChibiMeta(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	available, model := s.chibiGeneratorStatus(acc.cfg)
	writeChibiJSON(w, http.StatusOK, map[string]any{
		"readonly": acc.cfg.VirtualDesktop.ReadOnly,
		"limits": map[string]any{
			"max_rows":                   acc.cfg.ChibiSensei.Dataset.MaxRows,
			"max_message_chars":          chibisensei.MaxMessageChars,
			"max_messages":               chibisensei.MaxMessages,
			"import_max_file_mb":         acc.cfg.ChibiSensei.Import.MaxFileMB,
			"generator_max_rows_per_job": acc.cfg.ChibiSensei.Generator.MaxRowsPerJob,
			"page_max":                   chibisensei.PageMax,
		},
		"recipes":   chibisensei.Recipes(),
		"profiles":  []any{},
		"generator": map[string]any{"available": available, "model": model},
	})
}
