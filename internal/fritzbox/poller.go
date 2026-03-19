package fritzbox

// poller.go – optional background poller for Fritz!Box telephony events.
// Polls the call list and TAM at a configurable interval; invokes the
// OnNewCall / OnNewTAMMessage callbacks if new entries are detected.
// The poller is entirely opt-in (config.fritzbox.telephony.polling.enabled)
// and deliberately does NOT import any agent or server packages to avoid
// import cycles.

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aurago/internal/config"
)

// CallbackFunc is called when a new telephony event is detected.
// kind is "call" or "tam_message"; summary is a short human-readable
// description that can be forwarded to the agent or a notification channel.
type CallbackFunc func(kind, summary string)

// Poller polls Fritz!Box telephony at regular intervals.
type Poller struct {
	cfg      config.Config
	callback CallbackFunc
	logger   *slog.Logger

	// Dedup state
	lastCallIdx     int       // index of the most recently seen call entry
	lastCallSummary string    // summary string of the most recently seen call
	lastCallTime    time.Time // timestamp of the last callback (for dedup window)
	lastTAMIdx      int       // index of the most recently seen TAM message
	lastTAMTime     time.Time // timestamp of the last TAM callback

	// Rate limiting: sliding window of callback timestamps within the current hour
	mu            sync.Mutex
	callbackTimes []time.Time
	dedupWindow   time.Duration
	maxPerHour    int

	// Client pooling: reuse the client across poll cycles
	pooledClient    *Client
	clientCreatedAt time.Time

	cancel context.CancelFunc
}

// clientMaxAge is the maximum age of a pooled client before it is re-created.
// Fritz!Box SID sessions are valid for ~20 min; we re-auth well before that.
const clientMaxAge = 15 * time.Minute

// NewPoller creates a Poller. Start it with Poller.Start().
func NewPoller(cfg config.Config, callback CallbackFunc, logger *slog.Logger) *Poller {
	dedupMin := cfg.FritzBox.Telephony.Polling.DedupWindowMinutes
	if dedupMin <= 0 {
		dedupMin = 5
	}
	maxCb := cfg.FritzBox.Telephony.Polling.MaxCallbacksPerHour
	if maxCb <= 0 {
		maxCb = 20
	}
	return &Poller{
		cfg:         cfg,
		callback:    callback,
		logger:      logger,
		lastCallIdx: -1,
		lastTAMIdx:  -1,
		dedupWindow: time.Duration(dedupMin) * time.Minute,
		maxPerHour:  maxCb,
	}
}

// Start launches the polling loop in a background goroutine.
// Call Stop to shut it down.
func (p *Poller) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.run(ctx)
}

// Stop terminates the polling loop gracefully.
func (p *Poller) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	// Close pooled client on shutdown.
	if p.pooledClient != nil {
		p.pooledClient.Close()
		p.pooledClient = nil
	}
}

func (p *Poller) run(ctx context.Context) {
	interval := p.cfg.FritzBox.Telephony.Polling.IntervalSeconds
	if interval <= 0 {
		interval = 60
	}
	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()

	p.logger.Info("[FritzBox Poller] started",
		"interval_s", interval,
		"dedup_window_min", int(p.dedupWindow.Minutes()),
		"max_callbacks_per_hour", p.maxPerHour)

	// Do one initial poll immediately.
	p.poll()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("[FritzBox Poller] stopped")
			return
		case <-tick.C:
			p.poll()
		}
	}
}

// getClient returns a pooled client, creating or refreshing it as needed.
func (p *Poller) getClient() (*Client, error) {
	if p.pooledClient != nil && time.Since(p.clientCreatedAt) < clientMaxAge {
		return p.pooledClient, nil
	}
	// Close stale client.
	if p.pooledClient != nil {
		p.pooledClient.Close()
		p.pooledClient = nil
	}
	c, err := NewClient(p.cfg)
	if err != nil {
		return nil, err
	}
	p.pooledClient = c
	p.clientCreatedAt = time.Now()
	return c, nil
}

func (p *Poller) poll() {
	c, err := p.getClient()
	if err != nil {
		p.logger.Warn("[FritzBox Poller] client init failed", "error", err)
		return
	}

	p.pollCalls(c)

	if p.cfg.FritzBox.Telephony.SubFeatures.TAM {
		p.pollTAM(c)
	}
}

// rateLimitOK returns true if we're still under the per-hour callback limit.
func (p *Poller) rateLimitOK() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	// Prune entries older than 1 hour.
	fresh := p.callbackTimes[:0]
	for _, t := range p.callbackTimes {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	p.callbackTimes = fresh

	if len(p.callbackTimes) >= p.maxPerHour {
		return false
	}
	p.callbackTimes = append(p.callbackTimes, now)
	return true
}

func (p *Poller) pollCalls(c *Client) {
	calls, err := c.GetCallList()
	if err != nil {
		p.logger.Warn("[FritzBox Poller] call list fetch failed", "error", err)
		return
	}
	if len(calls) == 0 {
		return
	}
	// The Fritz!Box returns calls newest-first. Index 0 is the most recent.
	// On the first successful poll, record the current head index and do NOT
	// fire callbacks (we'd spam all historical calls).
	if p.lastCallIdx == -1 {
		p.lastCallIdx = 0
		p.lastCallSummary = summariseCall(calls[0])
		return
	}
	// Check if the newest entry is different from the last one we saw.
	newest := calls[0]
	cur := summariseCall(newest)
	if cur == p.lastCallSummary {
		return // nothing new
	}

	// Dedup window: ignore if within the configured time window of the last callback.
	if !p.lastCallTime.IsZero() && time.Since(p.lastCallTime) < p.dedupWindow {
		p.logger.Debug("[FritzBox Poller] call event suppressed (dedup window)", "window", p.dedupWindow)
		p.lastCallSummary = cur
		return
	}

	// Rate limit check.
	if !p.rateLimitOK() {
		p.logger.Warn("[FritzBox Poller] call callback suppressed (rate limit)", "max_per_hour", p.maxPerHour)
		p.lastCallSummary = cur
		return
	}

	p.lastCallSummary = cur
	p.lastCallTime = time.Now()
	p.logger.Info("[FritzBox Poller] new call detected", "type", newest.Type, "caller", "<redacted>")
	if p.callback != nil {
		p.callback("call", cur)
	}
}

// pollTAM checks the first TAM for new unread messages.
func (p *Poller) pollTAM(c *Client) {
	msgs, err := c.GetTAMList(0)
	if err != nil {
		p.logger.Warn("[FritzBox Poller] TAM fetch failed", "error", err)
		return
	}
	// Count unread messages.
	unread := 0
	for _, m := range msgs {
		if !m.Read {
			unread++
		}
	}
	if unread == 0 {
		p.lastTAMIdx = len(msgs)
		return
	}
	if p.lastTAMIdx == -1 {
		p.lastTAMIdx = len(msgs)
		return
	}
	if len(msgs) > p.lastTAMIdx {
		delta := len(msgs) - p.lastTAMIdx
		p.lastTAMIdx = len(msgs)

		// Dedup window for TAM.
		if !p.lastTAMTime.IsZero() && time.Since(p.lastTAMTime) < p.dedupWindow {
			p.logger.Debug("[FritzBox Poller] TAM callback suppressed (dedup window)")
			return
		}
		// Rate limit check.
		if !p.rateLimitOK() {
			p.logger.Warn("[FritzBox Poller] TAM callback suppressed (rate limit)", "max_per_hour", p.maxPerHour)
			return
		}

		p.lastTAMTime = time.Now()
		p.logger.Info("[FritzBox Poller] new TAM messages", "count", delta)
		if p.callback != nil {
			p.callback("tam_message", summariseTAM(delta))
		}
	}
}

// summariseCall returns a short, sanitised description suitable for passing to a
// notification system. Caller numbers are not included to avoid leaking PII in
// logs. The agent can always call fritzbox_telephony/get_call_list for details.
func summariseCall(c CallEntry) string {
	kind := c.Type
	return "New Fritz!Box call event (type: " + kind + "). Use fritzbox_telephony/get_call_list to see details."
}

func summariseTAM(count int) string {
	if count == 1 {
		return "1 new answering machine message on Fritz!Box. Use fritzbox_telephony/get_tam_messages to listen."
	}
	return fmt.Sprintf("%d new answering machine messages on Fritz!Box. Use fritzbox_telephony/get_tam_messages.", count)
}
