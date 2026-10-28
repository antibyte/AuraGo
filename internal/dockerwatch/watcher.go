package dockerwatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/dockerhost"
)

const (
	endsToPause     = 3                // stream ends within endsWindow that pause a host
	endsWindow      = 60 * time.Second // see endsToPause
	failuresToPause = 3                // connects answered with an HTTP error, in a row, that pause a host
)

// watcher follows the events of one host. It lives from the first user until
// IdleStop after the last one and holds one stream slot of the host while it
// runs (it runs without one when the budget is used up: it is essential and
// costs a single connection).
//
// States: connecting until the Engine accepts /events, live while the feed
// runs, unreachable when the host does not answer (the reason is the
// dockerhost classification), paused when the host answers but /events does
// not work (refused three times in a row, or the stream ended three times
// within a minute): the container list is then polled and the feed is retried
// now and then. A feed that is established again after any interruption sends
// `resync` (reason reconnect); a poll that sees a different container list
// sends it with reason poll.
type watcher struct {
	h      *Hub
	rt     *hostRT
	cancel context.CancelFunc

	slot     func() // gives the stream slot back; nil without one
	slotOnce sync.Once

	// The loop state below belongs to the watcher's goroutine.
	attempts   int         // failed connects since the feed was last established
	failures   int         // connects answered with an HTTP error, in a row
	ends       []time.Time // recent ends of an established feed
	needResync bool        // the feed was interrupted since the last resync
	digest     string      // container list digest of the paused poll
}

// startWatcherLocked starts the watcher of a host. The new status is recorded
// but not published: the client whose subscription started the watcher is sent
// it by Set.
func (h *Hub) startWatcherLocked(rt *hostRT) {
	ctx, cancel := context.WithCancel(h.ctx)
	w := &watcher{h: h, rt: rt, cancel: cancel}
	if release, err := h.Acquire(rt.id); err == nil {
		w.slot = release
	}
	rt.watcher = w
	rt.status = HostStatus{Host: rt.id, State: StateConnecting, T: h.opts.Now().UnixMilli()}
	h.wg.Add(1)
	go w.run(ctx)
}

// stop ends the watcher's goroutine and gives its slot back.
func (w *watcher) stop() {
	w.cancel()
	w.freeSlot()
}

func (w *watcher) freeSlot() {
	w.slotOnce.Do(func() {
		if w.slot != nil {
			w.slot()
		}
	})
}

func (w *watcher) run(ctx context.Context) {
	defer w.h.wg.Done()
	defer w.freeSlot()
	for ctx.Err() == nil {
		opened, err := w.connect(ctx)
		if ctx.Err() != nil {
			return
		}
		var wait time.Duration
		switch {
		case opened: // the feed ran and ended
			w.needResync = true
			now := w.h.opts.Now()
			w.ends = append(w.recentEnds(now), now)
			if len(w.ends) >= endsToPause {
				w.pause(ctx)
				wait = w.backoff(0)
				break
			}
			w.setStatus(StateConnecting, "")
			wait = w.backoff(0)
		case dockerhost.StatusOf(err) != 0: // the host answers, /events does not
			w.needResync = true
			w.failures++
			if w.failures >= failuresToPause {
				w.pause(ctx)
				wait = w.backoff(0)
				break
			}
			w.setStatus(StateConnecting, "")
			wait = w.backoff(w.attempts)
			w.attempts++
		default: // no answer at all
			w.needResync = true
			w.failures = 0
			w.setStatus(StateUnreachable, reasonOf(err))
			wait = w.backoff(w.attempts)
			w.attempts++
			w.h.log.Debug("docker events unreachable", "host", w.rt.id, "error", err)
		}
		if !sleepCtx(ctx, wait) {
			return
		}
	}
}

// connect opens /events and follows it until it ends. opened tells whether the
// Engine accepted the request; err is why the call ended.
func (w *watcher) connect(ctx context.Context) (opened bool, err error) {
	h := w.h
	c, err := h.hosts.Client(ctx, w.rt.id)
	if err != nil {
		return false, err
	}
	attempt, cancel := context.WithCancel(ctx)
	defer cancel()
	var timedOut atomic.Bool
	timer := time.AfterFunc(h.opts.ConnectTimeout, func() {
		timedOut.Store(true)
		cancel()
	})
	defer timer.Stop()
	err = dockerhost.StreamEventsOpen(attempt, c, time.Time{}, func() {
		if !timer.Stop() {
			return // the budget ran out just now; the request is being cancelled
		}
		opened = true
		w.attempts, w.failures = 0, 0
		w.setStatus(StateLive, "")
		if w.needResync {
			w.needResync = false
			w.resync("reconnect")
		}
	}, func(ev dockerhost.EngineEvent) error {
		w.dispatch(ev)
		return nil
	})
	if !opened && timedOut.Load() {
		err = fmt.Errorf("Docker events request timed out after %s: %w", h.opts.ConnectTimeout, context.DeadlineExceeded)
	}
	return opened, err
}

// pause is the fallback while /events does not work: it polls the container
// list and retries the feed. It returns when the context ends or the feed was
// established again (and has ended since).
func (w *watcher) pause(ctx context.Context) {
	w.ends, w.failures, w.attempts, w.digest = nil, 0, 0, ""
	w.setStatus(StatePaused, "")
	poll := time.NewTicker(w.h.opts.PollInterval)
	retry := time.NewTicker(w.h.opts.RetryPaused)
	defer poll.Stop()
	defer retry.Stop()
	w.pollOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			w.pollOnce(ctx)
		case <-retry.C:
			opened, _ := w.connect(ctx)
			if ctx.Err() != nil {
				return
			}
			if opened {
				w.needResync = true
				w.ends = []time.Time{w.h.opts.Now()}
				w.setStatus(StateConnecting, "")
				return
			}
		}
	}
}

// pollOnce lists the containers of a paused host and sends `resync` (poll)
// when the list differs from the previous one.
func (w *watcher) pollOnce(ctx context.Context) {
	c, err := w.h.hosts.Client(ctx, w.rt.id)
	if err != nil {
		w.setStatus(StateUnreachable, reasonOf(err))
		return
	}
	list, err := dockerhost.ListContainers(ctx, c)
	if err != nil {
		if ctx.Err() == nil && dockerhost.StatusOf(err) == 0 {
			w.setStatus(StateUnreachable, reasonOf(err))
		}
		return
	}
	w.setStatus(StatePaused, "")
	if digest := listDigest(list); digest != w.digest {
		w.digest = digest
		w.resync("poll")
	}
}

// listDigest fingerprints what the UI shows of a container list: id, state,
// status text and health.
func listDigest(list []dockerhost.ContainerSummary) string {
	lines := make([]string, 0, len(list))
	for _, c := range list {
		lines = append(lines, c.FullID+"|"+c.State+"|"+c.Status+"|"+c.Health)
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// recentEnds drops the stream ends that fall outside the pause window.
func (w *watcher) recentEnds(now time.Time) []time.Time {
	kept := w.ends[:0]
	for _, t := range w.ends {
		if now.Sub(t) < endsWindow {
			kept = append(kept, t)
		}
	}
	return kept
}

// backoff is the delay after n failed attempts; the last entry repeats.
func (w *watcher) backoff(n int) time.Duration {
	delays := w.h.opts.Backoff
	return delays[min(n, len(delays)-1)]
}

// setStatus publishes a state change, unless this watcher was replaced or stopped.
func (w *watcher) setStatus(state, reason string) {
	h := w.h
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || w.rt.watcher != w {
		return
	}
	h.setStatusLocked(w.rt, state, reason)
}

// dispatch forwards one Engine event to the clients and callbacks of the host.
func (w *watcher) dispatch(ev dockerhost.EngineEvent) {
	h := w.h
	out, ok := normalizeEvent(w.rt.id, ev, h.opts.Now())
	if !ok {
		return
	}
	h.mu.Lock()
	if h.closed || w.rt.watcher != w {
		h.mu.Unlock()
		return
	}
	h.publishLocked(w.rt.id, Message{Event: MsgEvent, Data: out})
	if out.Type == "container" {
		switch out.Action {
		case "start", "restart", "unpause": // sampled again after a stats_gone
			delete(w.rt.st.gone, out.ShortID)
		}
	}
	callbacks := w.callbacksLocked()
	h.mu.Unlock()
	for _, fn := range callbacks {
		fn(out)
	}
}

// resync tells clients and callbacks that the host's lists may have changed.
func (w *watcher) resync(reason string) {
	h := w.h
	h.mu.Lock()
	if h.closed || w.rt.watcher != w {
		h.mu.Unlock()
		return
	}
	h.publishLocked(w.rt.id, Message{Event: MsgResync, Data: Resync{Host: w.rt.id, Reason: reason}})
	callbacks := w.callbacksLocked()
	h.mu.Unlock()
	ev := Event{Host: w.rt.id, Type: "resync", Action: reason, T: h.opts.Now().UnixMilli()}
	for _, fn := range callbacks {
		fn(ev)
	}
}

func (w *watcher) callbacksLocked() []func(Event) {
	out := make([]func(Event), 0, len(w.rt.callbacks))
	for _, fn := range w.rt.callbacks {
		out = append(out, fn)
	}
	return out
}

// reasonOf is the reason of an unreachable host.
func reasonOf(err error) string {
	if errors.Is(err, dockerhost.ErrHostNotFound) {
		return ReasonHostNotFound
	}
	return dockerhost.ClassifyError(err)
}

// sleepCtx waits d; it returns false when the context ended first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
