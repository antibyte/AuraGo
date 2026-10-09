package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/retronet"

	"github.com/gorilla/websocket"
)

const (
	retroNetConnectPath     = "/api/desktop/retronet/connect"
	retroNetStatusWait      = 10 * time.Second // POST /status waits at most this long for the probe in flight
	retroNetRevalidateEvery = time.Second      // policy and authorization recheck while a session runs
	retroNetAuditTimeout    = 5 * time.Second
)

type retroNetDirectoryResponse struct {
	Entries []retronet.Entry           `json:"entries"`
	Status  map[string]retronet.Status `json:"status"`
	Stale   bool                       `json:"stale"`
	CanEdit bool                       `json:"can_edit"`
}

func registerDesktopRetroNetRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/retronet/directory", withDesktopRetroNetGuard(s, http.MethodGet, s.handleRetroNetDirectory))
	mux.HandleFunc("/api/desktop/retronet/status", withDesktopRetroNetGuard(s, http.MethodPost, s.handleRetroNetStatus))
	mux.HandleFunc(retroNetConnectPath, withDesktopRetroNetGuard(s, http.MethodGet, s.handleRetroNetConnect))
}

// withDesktopRetroNetGuard runs the shared gates in contract order, before any
// WebSocket upgrade: Desktop write scope plus the path's operation class
// (connect is execution, so readonly blocks it), the live Retro-Net grant,
// then the method.
func withDesktopRetroNetGuard(s *Server, method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopRequestOperation(r)) {
			return
		}
		if !s.desktopSerialPolicy(r).RetroNetEnabled {
			writeDesktopPolicyError(w, "retronet_disabled", "Retro-Net is disabled.")
			return
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

// retroNet returns the lazily created session manager and status prober.
// Tests pre-seed s.retroNetManager / s.retroNetStatus before the first request
// (for example with a test-only dialer that admits loopback and a fake
// resolver); this initializer keeps them and only fills what is missing.
func (s *Server) retroNet() (*retronet.Manager, *retronet.StatusProber) {
	s.retroNetOnce.Do(func() {
		if s.retroNetManager == nil {
			s.retroNetManager = &retronet.Manager{}
		}
		if s.retroNetManager.OnHostKeyAccepted == nil {
			s.retroNetManager.OnHostKeyAccepted = s.storeRetroNetHostKey
		}
		if s.retroNetStatus == nil {
			s.retroNetStatus = &retronet.StatusProber{}
		}
	})
	return s.retroNetManager, s.retroNetStatus
}

// storeRetroNetHostKey persists a first-contact SSH fingerprint for an own
// entry. Manager ignores the returned error, so a failure is logged here with
// the entry ID and the error only. A stored key changes the retronet.entries
// setting, which is announced like handleDesktopSettings does so open settings
// views refresh (filterDesktopEvent keeps it from non-admin clients).
func (s *Server) storeRetroNetHostKey(ctx context.Context, entryID, fingerprint string) error {
	svc, hub, err := s.getDesktopService(ctx)
	if err == nil {
		err = svc.SetRetroNetHostKey(ctx, entryID, fingerprint)
	}
	if err != nil {
		logger := s.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("Retro-Net host key was not stored", "entry", entryID, "error", err)
		return err
	}
	settings := map[string]string{}
	if own, err := svc.RetroNetEntries(ctx); err == nil {
		if document, err := retronet.EncodeEntriesDocument(own); err == nil {
			settings[retronet.EntriesSetting] = document
		}
	}
	event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_settings", "settings": settings}, CreatedAt: time.Now().UTC()}
	broadcastDesktopEvent(s, hub, event)
	return nil
}

// retroNetDirectory loads the curated catalog and the validated own entries.
func (s *Server) retroNetDirectory(ctx context.Context) (*desktop.Service, []retronet.Entry, []retronet.Entry, error) {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	own, err := svc.RetroNetEntries(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return svc, retronet.DefaultCatalog(), own, nil
}

func (s *Server) handleRetroNetDirectory(w http.ResponseWriter, r *http.Request) {
	_, catalog, own, err := s.retroNetDirectory(r.Context())
	if err != nil {
		jsonError(w, "Retro-Net directory is unavailable.", http.StatusServiceUnavailable)
		return
	}
	entries := append(catalog, own...)
	_, prober := s.retroNet()
	status, stale := prober.Snapshot(entries)
	writeRetroNetJSON(w, retroNetDirectoryResponse{
		Entries: entries,
		Status:  status,
		Stale:   stale,
		CanEdit: desktopRequestIsAdmin(s, r) && !s.desktopSerialPolicy(r).ReadOnly,
	})
}

func (s *Server) handleRetroNetStatus(w http.ResponseWriter, r *http.Request) {
	_, catalog, own, err := s.retroNetDirectory(r.Context())
	if err != nil {
		jsonError(w, "Retro-Net directory is unavailable.", http.StatusServiceUnavailable)
		return
	}
	_, prober := s.retroNet()
	status := prober.Refresh(r.Context(), append(catalog, own...), retroNetStatusWait)
	writeRetroNetJSON(w, map[string]map[string]retronet.Status{"status": status})
}

func writeRetroNetJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(payload)
}

// handleRetroNetConnect bridges one browser WebSocket to one catalog or own
// entry. The browser names only the entry ID; host and port come from the
// directory. Hijacked sockets are tracked by trackHTTP, so shutdown closes
// them before draining handlers.
func (s *Server) handleRetroNetConnect(w http.ResponseWriter, r *http.Request) {
	svc, catalog, own, err := s.retroNetDirectory(r.Context())
	if err != nil {
		jsonError(w, "Retro-Net directory is unavailable.", http.StatusServiceUnavailable)
		return
	}
	entry, ok := retronet.Lookup(catalog, own, strings.TrimSpace(r.URL.Query().Get("entry")))
	if !ok {
		jsonError(w, "Unknown Retro-Net entry.", http.StatusNotFound)
		return
	}
	if !desktop.SameHostWebSocketOrigin(r) {
		jsonError(w, "Same-host origin required.", http.StatusForbidden)
		return
	}
	if !websocket.IsWebSocketUpgrade(r) {
		jsonError(w, "WebSocket upgrade required.", http.StatusBadRequest)
		return
	}
	size := retroNetSizeFromQuery(r.URL.Query())
	auditDesktopRemoteAttempt(s, r, "desktop_retronet_connect", entry.ID, "attempt", "")
	conn, err := retroNetUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := newRetroNetWSClient(conn)
	defer client.closeSocket()
	ctx, stop := s.retroNetSessionContext(r)
	defer stop()
	manager, _ := s.retroNet()
	result := manager.Run(ctx, entry, size, client)
	// Run has sent the final result frame: hang up first, then audit.
	stop()
	client.closeSocket()
	auditRetroNetSession(svc, s, r, entry.ID, result)
}

// retroNetSessionContext detaches the session from request cancellation so the
// end reason is known through context.Cause: policy or authorization loss ends
// it with retronet.ErrDisabled, server shutdown with retronet.ErrShutdown. Like
// withDesktopSerialGuard it rechecks every second, even while no bytes flow.
func (s *Server) retroNetSessionContext(r *http.Request) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(context.WithoutCancel(r.Context()))
	var shutdown <-chan struct{}
	if s.integrationCtx != nil {
		shutdown = s.integrationCtx.Done()
	}
	requestDone := r.Context().Done()
	go func() {
		tick := time.NewTicker(retroNetRevalidateEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-shutdown:
				cancel(retronet.ErrShutdown)
				return
			case <-requestDone:
				// HTTP drain or a revoked Desktop run grant.
				cancel(s.retroNetEndCause(r, context.Canceled))
				return
			case <-tick.C:
				if cause := s.retroNetEndCause(r, nil); cause != nil {
					cancel(cause)
					return
				}
			}
		}
	}()
	return ctx, func() { cancel(context.Canceled) }
}

// retroNetEndCause returns why a running session must end now, or fallback
// while the live policy and the caller's authorization still allow it.
func (s *Server) retroNetEndCause(r *http.Request, fallback error) error {
	if s.integrationCtx != nil && s.integrationCtx.Err() != nil {
		return retronet.ErrShutdown
	}
	s.httpDrainMu.Lock()
	draining := s.httpDraining
	s.httpDrainMu.Unlock()
	if draining {
		return retronet.ErrShutdown
	}
	if !s.desktopSerialPolicy(r).RetroNetEnabled || !desktopWSAuthorizationValid(s, r, desktopScopeWrite) {
		return retronet.ErrDisabled
	}
	return fallback
}

// auditRetroNetSession records how a session ended. It never records payload bytes.
func auditRetroNetSession(svc *desktop.Service, s *Server, r *http.Request, entryID string, result retronet.Result) {
	if svc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), retroNetAuditTimeout)
	defer cancel()
	details := map[string]interface{}{
		"code":        result.Code,
		"reason":      result.Reason,
		"target":      result.Target,
		"bytes_in":    result.BytesIn,
		"bytes_out":   result.BytesOut,
		"duration_ms": result.Duration.Milliseconds(),
	}
	_ = svc.AuditWithRequest(ctx, "desktop_retronet_session", entryID, details, desktop.SourceUser, desktopAuditRequestInfo(s, r))
}
