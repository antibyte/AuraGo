package server

import (
	"context"
	"encoding/json"
	"errors"
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

// retroNetHostKeyPersistKey marks a session context whose first-contact SSH
// host keys may be stored; see withRetroNetHostKeyPersistence.
type retroNetHostKeyPersistKey struct{}

// withRetroNetHostKeyPersistence records whether the session's caller may pin
// first-contact host keys. Any write-scope user may accept an unknown key for
// their own session, but a stored key applies to everyone, so only an
// administrator's acceptance is persisted.
func withRetroNetHostKeyPersistence(ctx context.Context, allowed bool) context.Context {
	return context.WithValue(ctx, retroNetHostKeyPersistKey{}, allowed)
}

// retroNetDialedEntryKey carries the entry a session dialed; see
// withRetroNetDialedEntry.
type retroNetDialedEntryKey struct{}

// withRetroNetDialedEntry records the entry the session dialed, so a
// first-contact key is stored only while the entry still names that target.
func withRetroNetDialedEntry(ctx context.Context, entry retronet.Entry) context.Context {
	return context.WithValue(ctx, retroNetDialedEntryKey{}, entry)
}

// storeRetroNetHostKey persists a first-contact SSH fingerprint for an own
// entry when the session context allows it (fail closed: no mark, no store)
// and names the dialed entry (no dialed entry, no store); the session
// continues either way. SetRetroNetHostKey refuses the key when the entry was
// pointed elsewhere while the prompt was open. Manager ignores the returned
// error, so a failure is logged here with the entry ID and the error only. A
// stored key changes the retronet.entries setting, which is announced like
// handleDesktopSettings does so open settings views refresh
// (filterDesktopEvent keeps it from non-admin clients).
func (s *Server) storeRetroNetHostKey(ctx context.Context, entryID, fingerprint string) error {
	if allowed, _ := ctx.Value(retroNetHostKeyPersistKey{}).(bool); !allowed {
		s.retroNetLogger().Info("Retro-Net host key not persisted: non-admin", "entry", entryID)
		return nil
	}
	dialed, ok := ctx.Value(retroNetDialedEntryKey{}).(retronet.Entry)
	if !ok || dialed.ID != entryID {
		err := errors.New("the session does not name the dialed entry")
		s.retroNetLogger().Warn("Retro-Net host key was not stored", "entry", entryID, "error", err)
		return err
	}
	svc, hub, err := s.getDesktopService(ctx)
	if err == nil {
		err = svc.SetRetroNetHostKey(ctx, dialed, fingerprint)
	}
	if err != nil {
		s.retroNetLogger().Warn("Retro-Net host key was not stored", "entry", entryID, "error", err)
		return err
	}
	own, err := svc.RetroNetEntries(ctx)
	var document string
	if err == nil {
		document, err = retronet.EncodeEntriesDocument(own)
	}
	if err != nil {
		// The key is stored; only the announcement is skipped.
		s.retroNetLogger().Warn("Retro-Net host key stored but not announced", "entry", entryID, "error", err)
		return nil
	}
	settings := map[string]string{retronet.EntriesSetting: document}
	event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_settings", "settings": settings}, CreatedAt: time.Now().UTC()}
	broadcastDesktopEvent(s, hub, event)
	return nil
}

func (s *Server) retroNetLogger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// retroNetDirectory loads the curated catalog and the validated own entries.
func (s *Server) retroNetDirectory(ctx context.Context) ([]retronet.Entry, []retronet.Entry, error) {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, nil, err
	}
	own, err := svc.RetroNetEntries(ctx)
	if err != nil {
		return nil, nil, err
	}
	return retronet.DefaultCatalog(), own, nil
}

func (s *Server) handleRetroNetDirectory(w http.ResponseWriter, r *http.Request) {
	catalog, own, err := s.retroNetDirectory(r.Context())
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
	catalog, own, err := s.retroNetDirectory(r.Context())
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
	catalog, own, err := s.retroNetDirectory(r.Context())
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
		// Upgrade has answered the handshake with an HTTP error.
		auditDesktopRemoteAttempt(s, r, "desktop_retronet_connect", entry.ID, "blocked", "upgrade_failed")
		return
	}
	client := newRetroNetWSClient(conn)
	defer client.closeSocket()
	ctx, stop := s.retroNetSessionContext(r)
	defer stop()
	ctx = withRetroNetDialedEntry(withRetroNetHostKeyPersistence(ctx, desktopRequestIsAdmin(s, r)), entry)
	manager, _ := s.retroNet()
	result := manager.Run(ctx, entry, size, client)
	// Run has sent the final result frame: hang up first, then audit.
	stop()
	client.closeSocket()
	s.auditRetroNetSession(r, entry.ID, result)
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
				// HTTP drain or a revoked Desktop run grant. Readonly and disable
				// revocations publish the new policy first (ErrDisabled); a grant
				// revoked by shutdownDesktopStorage can precede the server context
				// and the drain flag, so the fallback is a shutdown.
				cancel(s.retroNetEndCause(r, retronet.ErrShutdown))
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

// auditRetroNetSession records how a session ended. It never records payload
// bytes. A session can outlive the Desktop service it started with (a desktop
// config change replaces and closes it), so the current service is fetched
// here, under a short background deadline because the request may be done.
func (s *Server) auditRetroNetSession(r *http.Request, entryID string, result retronet.Result) {
	ctx, cancel := context.WithTimeout(context.Background(), retroNetAuditTimeout)
	defer cancel()
	details := map[string]interface{}{
		"code":        result.Code,
		"reason":      result.Reason,
		"target":      result.Target,
		"bytes_in":    result.BytesIn,
		"bytes_out":   result.BytesOut,
		"duration_ms": result.Duration.Milliseconds(),
	}
	svc, _, err := s.getDesktopService(ctx)
	if err == nil {
		err = svc.AuditWithRequest(ctx, "desktop_retronet_session", entryID, details, desktop.SourceUser, desktopAuditRequestInfo(s, r))
	}
	if err != nil {
		s.retroNetLogger().Warn("Retro-Net session end was not audited", "entry", entryID, "error", err)
	}
}
