package hass

import (
	"context"
	"net/http"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/tools"
)

// pollingPhase reads GET /api/states every poll interval while there is interest, for at most
// RetryWebsocket (design-backend 8.1). It returns true when the window (or ctx) ended and false when
// a poll failed: Home Assistant itself is unreachable, so the websocket backoff takes over again.
func (a *Adapter) pollingPhase(ctx context.Context) bool {
	a.status(homedash.StatusPolling, ReasonWebsocketUnavailable)
	window := time.NewTimer(a.o.RetryWebsocket)
	defer window.Stop()
	tick := time.NewTicker(a.poll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return true
		case <-window.C:
			return true
		case <-tick.C:
		}
		if !a.interested() {
			continue
		}
		if _, err := a.pollOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return true
			}
			a.degrade(ReasonUnreachable)
			return false
		}
	}
}

// pollOnce reads every state over REST and applies it like a websocket get_states. applied is false
// when the websocket delivered states meanwhile: a REST list read before that must not overwrite
// newer event states. No lock is held during the request.
func (a *Adapter) pollOnce(ctx context.Context) (applied bool, err error) {
	pctx, cancel := context.WithTimeout(ctx, a.o.FetchTimeout)
	defer cancel()
	var list []HAState
	if err := tools.HARequestJSON(pctx, a.ha, http.MethodGet, "/api/states", nil, &list); err != nil {
		a.log.Debug("homedash: Home Assistant REST poll failed", "error", err)
		return false, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wsLive {
		return false, nil
	}
	a.runStates = true
	a.replaceStatesLocked(list)
	return true, nil
}

// Temporary until Task 8 (removed there).
func (a *Adapter) History(context.Context, string, time.Time, time.Time, int) ([]homedash.Sample, error) {
	return []homedash.Sample{}, nil
}
func (a *Adapter) Snapshot(context.Context, string, int, int) ([]byte, string, error) {
	return nil, "", &homedash.Error{Code: homedash.CodeUnsupportedCommand, Message: "not yet"}
}
