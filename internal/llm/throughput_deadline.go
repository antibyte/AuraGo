package llm

import (
	"context"
	"sync"
	"time"
)

// ThroughputPolicy bounds one streamed call by its progress instead of a fixed
// duration. Small reasoning models are slow and think long; a fixed per-call
// timeout cut them off while they were still producing output.
type ThroughputPolicy struct {
	// Base applies until output is measured and is never shortened.
	Base time.Duration
	// ReserveTokens is the output the call may produce (its max_tokens).
	ReserveTokens int
	// Streams slower than this keep the base deadline; they look stalled.
	MinTokensPerSecond float64
	// MaxFactor caps any extension at Base*MaxFactor.
	MaxFactor float64
	// MinSample is the measured streaming time required before extending.
	MinSample time.Duration
}

// Roughly four characters of text, reasoning or tool arguments per token.
const throughputCharsPerToken = 4

type throughputDeadlineKey struct{}

type throughputContext struct {
	parent context.Context
	policy ThroughputPolicy
	done   chan struct{}

	mu       sync.Mutex
	err      error
	start    time.Time
	firstAt  time.Time
	tokens   float64
	deadline time.Time
	timer    *time.Timer
	stop     func() bool
}

// NewThroughputDeadline returns a context that expires at policy.Base unless
// the stream reports steady output. Once it does, the deadline moves to the
// time the remaining reserve needs at the measured rate (plus 10%), never
// earlier than the base and never beyond Base*MaxFactor or the parent's own
// deadline. Expiry reports context.DeadlineExceeded to derived contexts.
func NewThroughputDeadline(parent context.Context, policy ThroughputPolicy) (context.Context, context.CancelFunc) {
	if policy.MaxFactor < 1 {
		policy.MaxFactor = 1
	}
	if policy.MinSample <= 0 {
		policy.MinSample = 5 * time.Second
	}
	c := &throughputContext{parent: parent, policy: policy, done: make(chan struct{}), start: time.Now()}
	// Both callbacks run on their own goroutines and lock c.mu first, so they
	// observe the fields assigned here.
	c.mu.Lock()
	c.deadline = c.start.Add(policy.Base)
	c.timer = time.AfterFunc(policy.Base, func() { c.expire() })
	c.stop = context.AfterFunc(parent, func() { c.finish(parent.Err()) })
	c.mu.Unlock()
	return c, func() { c.finish(context.Canceled) }
}

// ObserveStreamOutput reports newly streamed characters to the throughput
// deadline governing ctx, if any.
func ObserveStreamOutput(ctx context.Context, chars int) {
	if c, ok := ctx.Value(throughputDeadlineKey{}).(*throughputContext); ok && chars > 0 {
		c.observe(chars)
	}
}

func hasThroughputDeadline(ctx context.Context) bool {
	_, ok := ctx.Value(throughputDeadlineKey{}).(*throughputContext)
	return ok
}

func (c *throughputContext) observe(chars int) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	if c.firstAt.IsZero() {
		c.firstAt = now
	}
	c.tokens += float64(chars) / throughputCharsPerToken
	elapsed := now.Sub(c.firstAt)
	if elapsed < c.policy.MinSample {
		return
	}
	rate := c.tokens / elapsed.Seconds()
	if rate < c.policy.MinTokensPerSecond {
		return
	}
	remaining := max(0, float64(c.policy.ReserveTokens)-c.tokens)
	needed := now.Add(time.Duration(remaining / rate * 1.1 * float64(time.Second)))
	limit := c.start.Add(time.Duration(float64(c.policy.Base) * c.policy.MaxFactor))
	next := needed
	if next.After(limit) {
		next = limit
	}
	// Only ever extend, and skip tiny resets on every chunk.
	if next.Sub(c.deadline) < min(time.Second, c.policy.Base/20) {
		return
	}
	c.deadline = next
	c.timer.Reset(time.Until(next))
}

func (c *throughputContext) expire() {
	c.mu.Lock()
	if time.Now().Before(c.deadline) && c.err == nil {
		// A concurrent extension raced the timer; it was already reset.
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	c.finish(context.DeadlineExceeded)
}

func (c *throughputContext) finish(err error) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = err
	c.timer.Stop()
	close(c.done)
	stop := c.stop
	c.mu.Unlock()
	stop()
}

func (c *throughputContext) Deadline() (time.Time, bool) {
	c.mu.Lock()
	deadline := c.deadline
	c.mu.Unlock()
	if parent, ok := c.parent.Deadline(); ok && parent.Before(deadline) {
		return parent, true
	}
	return deadline, true
}

func (c *throughputContext) Done() <-chan struct{} { return c.done }

func (c *throughputContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *throughputContext) Value(key any) any {
	if key == (throughputDeadlineKey{}) {
		return c
	}
	return c.parent.Value(key)
}
