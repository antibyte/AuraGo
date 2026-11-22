package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/config"
	"aurago/internal/desktop"
)

const (
	chibiJSONLimit     = 1 << 20
	chibiRowsJSONLimit = 16 << 20
)

// chibiSenseiRuntime is the lazily opened state of the app for one data directory: the dataset store, the
// profile catalog, the event hub, the job runner and the delete-preview tokens.
//
// A handler holds the runtime through acquireChibiSensei until it is done. A runtime that is replaced
// (data directory change) or shut down is retired: no new handler gets it, its job runner stops at once,
// and its store is closed only after the last holder released it.
type chibiSenseiRuntime struct {
	dataDir string
	store   *chibisensei.Store
	catalog *chibisensei.Catalog
	hub     *chibisensei.Hub
	jobs    *chibisensei.Runner
	deletes *chibisensei.DeleteTokens

	retiring chan struct{} // closed when the runtime is retired; long streams end on it
	drained  chan struct{} // closed after the store is closed

	// guarded by Server.chibiSenseiMu
	users      int
	retired    bool
	jobsClosed bool
	closed     bool
}

// chibiSenseiAccess is what a handler gets after the gates passed.
type chibiSenseiAccess struct {
	cfg *config.Config
	rt  *chibiSenseiRuntime
	svc *desktop.Service
}

// chibiLease carries the release of the runtime a request holds. handleChibiSensei puts one into the
// request context and releases it when the handler returns, so no route can leak a holder.
type chibiLease struct{ release func() }

type chibiLeaseKey struct{}

// openChibiRuntime opens the store of dataDir and everything that hangs on it.
func (s *Server) openChibiRuntime(dataDir string) (*chibiSenseiRuntime, error) {
	catalog, err := chibisensei.LoadCatalog()
	if err != nil {
		return nil, err
	}
	store, err := chibisensei.OpenStore(filepath.Join(dataDir, config.ChibiSenseiDBFilename))
	if err != nil {
		return nil, err
	}
	hub := chibisensei.NewHub()
	store.SetNotifier(hub.DataChanged)
	jobs := chibisensei.NewRunner(store, hub)
	if s.Logger != nil {
		jobs.SetLogger(s.Logger)
	}
	if err := jobs.Recover(context.Background()); err != nil {
		_ = store.Close()
		return nil, err
	}
	return &chibiSenseiRuntime{
		dataDir: dataDir, store: store, catalog: catalog, hub: hub, jobs: jobs,
		deletes:  chibisensei.NewDeleteTokens(nil),
		retiring: make(chan struct{}), drained: make(chan struct{}),
	}, nil
}

// acquireChibiSensei returns the runtime for the current data directory, opening
// <data_dir>/chibi_sensei.db on first use, and a release func the caller must call exactly when it no
// longer touches the runtime. A data directory change retires the old runtime.
func (s *Server) acquireChibiSensei() (*chibiSenseiRuntime, func(), error) {
	if s == nil {
		return nil, nil, errors.New("chibi sensei unavailable")
	}
	s.chibiSenseiMu.Lock()
	if s.chibiSenseiClosed {
		s.chibiSenseiMu.Unlock()
		return nil, nil, errors.New("chibi sensei is shutting down")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" {
		s.chibiSenseiMu.Unlock()
		return nil, nil, errors.New("the data directory is unavailable")
	}
	dataDir := normalizedVideoStudioRoot(cfg.Directories.DataDir)
	var old *chibiSenseiRuntime
	if s.chibiSensei != nil {
		if s.chibiSensei.dataDir == dataDir {
			rt := s.chibiSensei
			rt.users++
			s.chibiSenseiMu.Unlock()
			return rt, s.chibiLeaseRelease(rt), nil
		}
		old = s.chibiSensei
		s.chibiSensei = nil
		old.retired = true
		close(old.retiring)
	}
	rt, err := s.openChibiRuntime(dataDir)
	if err == nil {
		rt.users++
		s.chibiSensei = rt
	}
	s.chibiSenseiMu.Unlock()
	if old != nil {
		s.finishChibiRetire(old)
	}
	if err != nil {
		return nil, nil, err
	}
	return rt, s.chibiLeaseRelease(rt), nil
}

// chibiLeaseRelease returns the one-shot release of a holder of rt.
func (s *Server) chibiLeaseRelease(rt *chibiSenseiRuntime) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			s.chibiSenseiMu.Lock()
			rt.users--
			closeNow := rt.retired && rt.jobsClosed && rt.users == 0 && !rt.closed
			if closeNow {
				rt.closed = true
			}
			s.chibiSenseiMu.Unlock()
			if closeNow {
				rt.closeStore()
			}
		})
	}
}

// finishChibiRetire stops the job runner of a retired runtime (before its store goes) and closes the
// store when no handler holds the runtime any more; otherwise the last release closes it.
func (s *Server) finishChibiRetire(rt *chibiSenseiRuntime) {
	rt.jobs.Close()
	s.chibiSenseiMu.Lock()
	rt.jobsClosed = true
	closeNow := rt.users == 0 && !rt.closed
	if closeNow {
		rt.closed = true
	}
	s.chibiSenseiMu.Unlock()
	if closeNow {
		rt.closeStore()
	}
}

func (rt *chibiSenseiRuntime) closeStore() {
	_ = rt.store.Close()
	close(rt.drained)
}

// chibiShutdownWait bounds how long closeChibiSensei waits for running handlers to let go.
const chibiShutdownWait = 15 * time.Second

// closeChibiSensei retires the runtime, refuses to reopen it and returns when its store is closed (or
// after chibiShutdownWait). The job runner stops first; the store closes after the last handler left.
func (s *Server) closeChibiSensei() {
	if s == nil {
		return
	}
	s.chibiSenseiMu.Lock()
	s.chibiSenseiClosed = true
	rt := s.chibiSensei
	s.chibiSensei = nil
	if rt != nil {
		rt.retired = true
		close(rt.retiring)
	}
	s.chibiSenseiMu.Unlock()
	if rt == nil {
		return
	}
	s.finishChibiRetire(rt)
	select {
	case <-rt.drained:
	case <-time.After(chibiShutdownWait):
	}
}

// initChibiSensei opens the runtime at boot when the app is enabled. A failure only logs a warning;
// the routes answer 503 storage_unavailable until the store can be opened.
func (s *Server) initChibiSensei(ctx context.Context) {
	if s == nil || ctx.Err() != nil || !s.chibiSenseiAvailable() {
		return
	}
	_, release, err := s.acquireChibiSensei()
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("[ChibiSensei] Store initialization failed")
		}
		return
	}
	release()
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
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		writeChibiError(w, http.StatusServiceUnavailable, "storage_unavailable", "Chibi Sensei storage is unavailable.", nil)
		return nil, false
	}
	acc := &chibiSenseiAccess{cfg: cfg, rt: rt}
	if withService {
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			release()
			writeChibiError(w, http.StatusServiceUnavailable, "desktop_unavailable", "The virtual desktop storage is unavailable.", nil)
			return nil, false
		}
		acc.svc = svc
	}
	if lease, _ := r.Context().Value(chibiLeaseKey{}).(*chibiLease); lease != nil {
		lease.release = release
	} else {
		release() // called outside the router: nothing would release it later
	}
	return acc, true
}

// chibiDomainError answers a domain error:
//
//	*ValidationError → 422 validation_failed (+issues; unsupported_file for an unsupported file type);
//	ErrNotFound → 404 not_found; ErrConflict → 412 conflict; ErrDatasetFull → 409 dataset_full;
//	ErrNotArchived → 409 not_archived; ErrAlreadyUndone → 409 already_undone; ErrUndoConflict → 409 undo_conflict; ErrJobActive → 409 job_active;
//	ErrDataChanged → 409 data_changed; ErrFileTooLarge → 413 file_too_large;
//	other → 500 storage_failed (logged, never the error text).
func chibiDomainError(w http.ResponseWriter, err error) {
	var ve *chibisensei.ValidationError
	switch {
	case errors.As(err, &ve):
		code := "validation_failed"
		if len(ve.Issues) > 0 && strings.HasSuffix(ve.Issues[0].MessageKey, ".unsupported_file") {
			code = "unsupported_file"
		}
		writeChibiError(w, http.StatusUnprocessableEntity, code, "The request is not valid.", ve.Issues)
	case errors.Is(err, chibisensei.ErrNotFound):
		writeChibiError(w, http.StatusNotFound, "not_found", "Not found.", nil)
	case errors.Is(err, chibisensei.ErrConflict):
		writeChibiError(w, http.StatusPreconditionFailed, "conflict", "The item changed since it was loaded.", nil)
	case errors.Is(err, chibisensei.ErrDatasetFull):
		writeChibiError(w, http.StatusConflict, "dataset_full", "The dataset reached its row limit.", nil)
	case errors.Is(err, chibisensei.ErrNotArchived):
		writeChibiError(w, http.StatusConflict, "not_archived", "Only an archived project can be deleted.", nil)
	case errors.Is(err, chibisensei.ErrAlreadyUndone):
		writeChibiError(w, http.StatusConflict, "already_undone", "This change was already undone.", nil)
	case errors.Is(err, chibisensei.ErrUndoConflict):
		writeChibiError(w, http.StatusConflict, "undo_conflict", "Rows of this change were changed later; it cannot be undone.", nil)
	case errors.Is(err, chibisensei.ErrJobActive):
		writeChibiError(w, http.StatusConflict, "job_active", "A job of this kind is already running.", nil)
	case errors.Is(err, chibisensei.ErrDataChanged):
		writeChibiError(w, http.StatusConflict, "data_changed", "The rows changed during the check.", nil)
	case errors.Is(err, chibisensei.ErrFileTooLarge):
		writeChibiError(w, http.StatusRequestEntityTooLarge, "file_too_large", "The file is too large.", nil)
	default:
		if !errors.Is(err, context.Canceled) {
			slog.Warn("[ChibiSensei] Request failed", "error", err)
		}
		writeChibiError(w, http.StatusInternalServerError, "storage_failed", "The Chibi Sensei storage failed.", nil)
	}
}

// chibiValidation answers 422 validation_failed with one issue.
func chibiValidation(w http.ResponseWriter, field, key, message string) {
	chibiDomainError(w, &chibisensei.ValidationError{Issues: []chibisensei.Issue{
		{Field: field, MessageKey: "chibi_sensei." + key, Message: message}}})
}

// chibiProfileUnknown answers 422 profile_unknown.
func chibiProfileUnknown(w http.ResponseWriter) {
	writeChibiError(w, http.StatusUnprocessableEntity, "profile_unknown", "The profile is not in the catalog.",
		[]chibisensei.Issue{{Field: "profile_id", MessageKey: "chibi_sensei.profile_unknown", Message: "The profile is not in the catalog."}})
}

// chibiIfMatch reads If-Match as an integer revision (quotes allowed): missing → 428 precondition_required,
// malformed → 400 precondition_invalid.
func chibiIfMatch(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		writeChibiError(w, http.StatusPreconditionRequired, "precondition_required", "Send the current revision in If-Match.", nil)
		return 0, false
	}
	rev, err := strconv.ParseInt(strings.Trim(raw, `"`), 10, 64)
	if err != nil || rev < 0 {
		writeChibiError(w, http.StatusBadRequest, "precondition_invalid", "If-Match must be a revision taken from an earlier answer.", nil)
		return 0, false
	}
	return rev, true
}

// chibiStorageFull answers 409 storage_full when the store reached chibi_sensei.storage.max_gb.
func chibiStorageFull(w http.ResponseWriter, acc *chibiSenseiAccess) bool {
	if acc.rt.store.SizeBytes() >= int64(acc.cfg.ChibiSensei.Storage.MaxGB)<<30 {
		writeChibiError(w, http.StatusConflict, "storage_full", "The Chibi Sensei storage limit is reached.", nil)
		return true
	}
	return false
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
	profiles := []chibisensei.ProfileSummary{}
	for _, p := range acc.rt.catalog.List() {
		profiles = append(profiles, p.Summary())
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{
		"readonly": acc.cfg.VirtualDesktop.ReadOnly,
		"admin":    chibiRequestIsAdmin(s, r),
		"limits": map[string]any{
			"max_rows":                   acc.cfg.ChibiSensei.Dataset.MaxRows,
			"max_message_chars":          chibisensei.MaxMessageChars,
			"max_messages":               chibisensei.MaxMessages,
			"import_max_file_mb":         acc.cfg.ChibiSensei.Import.MaxFileMB,
			"generator_max_rows_per_job": acc.cfg.ChibiSensei.Generator.MaxRowsPerJob,
			"page_max":                   chibisensei.PageMax,
		},
		"recipes":   chibisensei.Recipes(),
		"profiles":  profiles,
		"generator": map[string]any{"available": available, "model": model},
	})
}
