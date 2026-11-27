package server

import (
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt"
	"aurago/internal/config"
	"aurago/internal/security"
)

// chibiRuntimeInstallTimeout bounds one background download of the llama.cpp runtime.
const chibiRuntimeInstallTimeout = 60 * time.Minute

// chibiChatConfig feeds the chat runtime manager from the live configuration at every call.
func (s *Server) chibiChatConfig(dataDir string) func() chatrt.Config {
	cacheDir := filepath.Join(chibisensei.ChibiDir(dataDir), "runtime")
	return func() chatrt.Config {
		out := chatrt.Config{CacheDir: cacheDir}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			return out
		}
		cs := cfg.ChibiSensei
		_ = config.NormalizeChibiSenseiConfig(&cs) // a range error elsewhere leaves the runtime values usable
		out.Enabled = cs.Enabled && cs.Runtime.Enabled && cfg.VirtualDesktop.Enabled
		out.Backend, out.IdleMinutes, out.ContextSize = cs.Runtime.Backend, cs.Runtime.IdleMinutes, cs.Runtime.ContextSize
		return out
	}
}

// chibiRuntimeError answers an error of the chat runtime with fixed English text; the raw error is logged scrubbed.
func chibiRuntimeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chatrt.ErrDisabled), errors.Is(err, chatrt.ErrUnsupported), errors.Is(err, chatrt.ErrClosed):
		writeChibiError(w, http.StatusServiceUnavailable, "runtime_unavailable", "The local chat runtime is not available.", nil)
	case errors.Is(err, chatrt.ErrNotInstalled):
		writeChibiError(w, http.StatusConflict, "runtime_not_installed", "The local chat runtime is not installed yet.", nil)
	case errors.Is(err, chatrt.ErrInstalling), errors.Is(err, chatrt.ErrBusy):
		writeChibiError(w, http.StatusConflict, "runtime_busy", "The local chat runtime is busy. Try again in a moment.", nil)
	case errors.Is(err, chatrt.ErrModelMissing):
		writeChibiError(w, http.StatusNotFound, "model_missing", "The model file is not available.", nil)
	default:
		slog.Warn("[ChibiSensei] Chat runtime request failed", "error", security.Scrub(err.Error()))
		writeChibiError(w, http.StatusServiceUnavailable, "runtime_unavailable", "The local chat runtime is not available.", nil)
	}
}

// chibiRuntimeView builds the Runtime document of the contract from the manager's status.
func chibiRuntimeView(s *Server, r *http.Request, acc *chibiSenseiAccess) map[string]any {
	st := acc.rt.chat.Status(r.Context())
	cfg := acc.cfg.ChibiSensei.Runtime
	instances := make([]map[string]any, 0, len(st.Instances))
	acc.rt.chatMu.Lock()
	for _, in := range st.Instances {
		kind, runID := "base", acc.rt.chatRuns[in.Key]
		if id, ok := strings.CutPrefix(in.Key, "tuned:"); ok {
			kind, runID = "tuned", id
		}
		instances = append(instances, map[string]any{"key": in.Key, "run_id": runID, "kind": kind, "state": in.State,
			"backend": in.Backend, "gpu_verified": in.GPUVerified, "idle_s": in.IdleSeconds, "started": in.Started})
	}
	acc.rt.chatMu.Unlock()
	notice := ""
	if st.Fallback {
		notice = "gpu_fallback"
	}
	return map[string]any{
		"state": st.State, "admin": chibiRequestIsAdmin(s, r), "build": st.Build,
		"backend":     map[string]any{"requested": cfg.Backend, "selected": st.Selected, "fallback": st.Fallback},
		"download":    map[string]any{"bytes": st.Download.Bytes, "done_bytes": st.Download.DoneBytes, "host": st.Download.Host, "error_code": st.Download.ErrorCode},
		"instances":   instances,
		"limits":      map[string]any{"max_instances": chatrt.MaxInstances, "idle_minutes": cfg.IdleMinutes, "context_size": cfg.ContextSize},
		"notice_code": notice,
	}
}

// handleChibiRuntime answers GET /runtime.
func handleChibiRuntime(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"runtime": chibiRuntimeView(s, r, acc)})
}

// handleChibiRuntimeInstall answers POST /runtime/install: an administrator starts the one-time download. The
// download runs in the background (a desktop run that a read-only switch or the shutdown cancels) and the answer
// follows at once; the client polls GET /runtime.
func handleChibiRuntimeInstall(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	if !chibiRequestIsAdmin(s, r) {
		writeChibiError(w, http.StatusForbidden, "admin_required", "Only an administrator can install the local chat runtime.", nil)
		return
	}
	ctx, cancel, err := s.beginDesktopBackgroundRun(chibiRuntimeInstallTimeout)
	if err != nil {
		writeChibiError(w, http.StatusForbidden, "desktop_readonly", "The desktop does not allow this action.", nil)
		return
	}
	done, err := acc.rt.chat.BeginInstall(ctx)
	if err != nil {
		cancel()
		chibiRuntimeError(w, err)
		return
	}
	go func() { // owner of the background context: ends with the download (Close cancels it)
		<-done
		cancel()
	}()
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"runtime": chibiRuntimeView(s, r, acc)})
}

// handleChibiRuntimeStop answers POST /runtime/stop: a stop operation that works on a read-only desktop.
func handleChibiRuntimeStop(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopStop, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	acc.rt.chat.StopAll(r.Context())
	writeChibiJSON(w, http.StatusOK, map[string]any{"runtime": chibiRuntimeView(s, r, acc)})
}
