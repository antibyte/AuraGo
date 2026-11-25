package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"aurago/internal/homedash"
)

// Stream timing; variables so that tests can shorten them.
var (
	homeDashHeartbeat   = 15 * time.Second
	homeDashReauthEvery = 5 * time.Second
)

// homeDashWriteTimeout bounds each frame; httpstream.WithWriteTimeout renews its own budget on the
// API listeners. The deadline is never cleared (a stuck client must time out).
const homeDashWriteTimeout = 30 * time.Second

type homeDashHelloLimits struct {
	Entities int `json:"entities"`
}

type homeDashHello struct {
	StreamID    string              `json:"stream_id"`
	Protocol    int                 `json:"protocol"`
	HeartbeatMS int64               `json:"heartbeat_ms"`
	Limits      homeDashHelloLimits `json:"limits"`
	ServerTime  time.Time           `json:"server_time"`
	Principal   string              `json:"principal"`
}

// homeDashEventStream writes server-sent events: one compact, scrubbed JSON line per frame, a write
// deadline armed before each frame.
type homeDashEventStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	rc      *http.ResponseController
}

func (st *homeDashEventStream) arm() error {
	if err := st.rc.SetWriteDeadline(time.Now().Add(homeDashWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

func (st *homeDashEventStream) send(event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	payload := scrubSSEJSON(string(raw))
	if err := st.arm(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(st.w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return err
	}
	st.flusher.Flush()
	return nil
}

// writeBatch sends one drained batch in protocol order: snapshot (with the pending source statuses),
// entity, removed, state, source (only without a snapshot), dashboard_changed, revoked. done is true
// after revoked.
func (st *homeDashEventStream) writeBatch(b homedash.Batch) (done bool, err error) {
	if len(b.Snapshot) > 0 {
		sources := b.Sources
		if sources == nil {
			sources = []homedash.SourceStatus{}
		}
		if err := st.send("snapshot", map[string]any{"entities": b.Snapshot, "sources": sources}); err != nil {
			return false, err
		}
		b.Sources = nil
	}
	if len(b.Entities) > 0 {
		if err := st.send("entity", map[string]any{"entities": b.Entities}); err != nil {
			return false, err
		}
	}
	if len(b.Removed) > 0 {
		if err := st.send("removed", map[string]any{"ids": b.Removed}); err != nil {
			return false, err
		}
	}
	if len(b.States) > 0 {
		if err := st.send("state", map[string]any{"changes": b.States}); err != nil {
			return false, err
		}
	}
	for _, src := range b.Sources {
		if err := st.send("source", src); err != nil {
			return false, err
		}
	}
	if b.Dashboard != nil {
		if err := st.send("dashboard_changed", b.Dashboard); err != nil {
			return false, err
		}
	}
	if b.Closed != "" {
		_ = st.send("revoked", map[string]string{"reason": b.Closed})
		return true, nil
	}
	return false, nil
}

// homeDashStreamRevoked re-validates an open stream: the caller's desktop:admin authorization (as the
// desktop websocket does) and HomeDash availability. A read-only desktop does not end a stream.
func homeDashStreamRevoked(s *Server, r *http.Request) string {
	if !desktopWSAuthorizationValid(s, r, desktopScopeAdmin) {
		return homedash.CloseAuthLost
	}
	if !homeDashEnabled(s.ConfigSnapshot()) {
		return homedash.CloseDisabled
	}
	return ""
}

// stream serves GET stream. One goroutine writes; it returns when the request ends, a write fails,
// re-validation fails (revoked) or the hub closes the subscriber (revoked with the hub's reason).
// The subscriber is always closed. Flushes are coalesced to at most one per svc.coalesce.
func (h *hdRequest) stream() {
	flusher, ok := h.w.(http.Flusher)
	if !ok {
		homeDashWriteError(h.w, http.StatusInternalServerError, homeDashCodeInternal, "streaming is not supported", nil)
		return
	}
	sub, err := h.svc.hub.Attach(homeDashPrincipal(h.r))
	if err != nil {
		h.fail(err)
		return
	}
	defer sub.Close()

	header := h.w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Accel-Buffering", "no")
	h.w.WriteHeader(http.StatusOK)
	st := &homeDashEventStream{w: h.w, flusher: flusher, rc: http.NewResponseController(h.w)}

	limit := h.cfg.HomeDash.Limits.MaxSubscriptions
	if limit <= 0 {
		limit = 300
	}
	if st.send("hello", homeDashHello{
		StreamID: sub.ID(), Protocol: homeDashStreamProtocol, HeartbeatMS: homeDashHeartbeat.Milliseconds(),
		Limits: homeDashHelloLimits{Entities: limit}, ServerTime: time.Now().UTC(), Principal: string(homedash.PrincipalSession),
	}) != nil {
		return
	}
	beat := time.NewTicker(homeDashHeartbeat)
	defer beat.Stop()
	reauth := time.NewTicker(homeDashReauthEvery)
	defer reauth.Stop()
	ctx := h.r.Context()
	var lastFlush time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if st.send("hb", map[string]any{"server_time": time.Now().UTC()}) != nil {
				return
			}
		case <-reauth.C:
			if reason := homeDashStreamRevoked(h.s, h.r); reason != "" {
				_ = st.send("revoked", map[string]string{"reason": reason})
				return
			}
		case <-sub.Ready():
			if wait := h.svc.coalesce - time.Since(lastFlush); wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
			lastFlush = time.Now()
			b := sub.Drain()
			if b.Empty() {
				continue
			}
			if done, err := st.writeBatch(b); done || err != nil {
				return
			}
		}
	}
}

type homeDashSubscriptionRequest struct {
	StreamID string   `json:"stream_id"`
	Entities []string `json:"entities"`
}

// subscriptions serves PUT stream/subscriptions: full replacement. Catalogs of the named sources are
// fetched first when stale, so the snapshot carries real entities. The stream must belong to the same
// principal (404 otherwise).
func (h *hdRequest) subscriptions() {
	var req homeDashSubscriptionRequest
	if !h.decode(&req, homeDashSubscriptionBody) {
		return
	}
	sub, ok := h.svc.hub.Subscriber(req.StreamID, homeDashPrincipal(h.r))
	if !ok {
		homeDashWriteError(h.w, http.StatusNotFound, homedash.CodeStreamNotFound, "there is no open stream with this id", nil)
		return
	}
	ids := make([]homedash.EntityID, 0, len(req.Entities))
	for _, id := range req.Entities {
		ids = append(ids, homedash.EntityID(id))
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	h.svc.hub.PrefetchCatalogs(ctx, ids)
	cancel()
	accepted, rejected, err := sub.Set(ids)
	if err != nil {
		h.fail(err)
		return
	}
	if accepted == nil {
		accepted = []homedash.EntityID{}
	}
	if rejected == nil {
		rejected = []homedash.Rejection{}
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"accepted": accepted, "rejected": rejected})
}
