package flows

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeClock is a manually advanced Clock for tests.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeWaiter
}

type fakeWaiter struct {
	at time.Time
	ch chan time.Time
}

func newFakeClock(now time.Time) *fakeClock { return &fakeClock{now: now} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.waiters = append(c.waiters, fakeWaiter{at: c.now.Add(d), ch: ch})
	return ch
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.at.After(c.now) {
			w.ch <- c.now
			continue
		}
		kept = append(kept, w)
	}
	clear(c.waiters[len(kept):]) // drop the stale tail so fired channels can be collected
	c.waiters = kept
}

// pending counts the After channels that have not fired yet. It also counts
// abandoned ones: a Sleep cancelled through its context leaves its channel here
// until Advance passes its deadline.
func (c *fakeClock) pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters)
}

// WaitForWaiters blocks until at least n After channels are pending (see
// pending). Abandoned channels of cancelled sleeps count too, so after a
// cancelled sleep a wait for the next sleeper can be satisfied by a stale entry;
// advance the clock past the abandoned deadline first when that matters.
func (c *fakeClock) WaitForWaiters(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for c.pending() < n {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d clock waiters, have %d", n, c.pending())
		}
		time.Sleep(time.Millisecond)
	}
}
