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

	lastCallIdx     int    // index of the most recently seen call entry
	lastCallSummary string // summary string of the most recently seen call (for dedup)
	lastTAMIdx      int    // index of the most recently seen TAM message

	cancel context.CancelFunc
}

// NewPoller creates a Poller. Start it with Poller.Run(ctx).
func NewPoller(cfg config.Config, callback CallbackFunc, logger *slog.Logger) *Poller {
	return &Poller{
		cfg:         cfg,
		callback:    callback,
		logger:      logger,
		lastCallIdx: -1,
		lastTAMIdx:  -1,
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
}

func (p *Poller) run(ctx context.Context) {
	interval := p.cfg.FritzBox.Telephony.Polling.IntervalSeconds
	if interval <= 0 {
		interval = 60
	}
	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()

	p.logger.Info("[FritzBox Poller] started", "interval_s", interval)

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

func (p *Poller) poll() {
	c, err := NewClient(p.cfg)
	if err != nil {
		p.logger.Warn("[FritzBox Poller] client init failed", "error", err)
		return
	}
	defer c.Close()

	p.pollCalls(c)

	if p.cfg.FritzBox.Telephony.SubFeatures.TAM {
		p.pollTAM(c)
	}
}

func (p *Poller) pollCalls(c *Client) {
	calls, err := c.GetCallList()
	if err != nil {
		p.logger.Debug("[FritzBox Poller] call list fetch failed", "error", err)
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
		return
	}
	// Check if the newest entry is different from the last one we saw.
	// We use the Date+Number fields as a unique key approximation.
	newest := calls[0]
	prev := p.lastCallSummary
	cur := summariseCall(newest)
	if cur == prev {
		return // nothing new
	}
	p.lastCallSummary = cur
	p.logger.Info("[FritzBox Poller] new call detected", "type", newest.Type, "caller", "<redacted>")
	if p.callback != nil {
		p.callback("call", cur)
	}
}

// pollTAM checks the first TAM for new unread messages.
func (p *Poller) pollTAM(c *Client) {
	msgs, err := c.GetTAMList(0)
	if err != nil {
		p.logger.Debug("[FritzBox Poller] TAM fetch failed", "error", err)
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
		// Net new messages appeared.
		delta := len(msgs) - p.lastTAMIdx
		p.lastTAMIdx = len(msgs)
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
