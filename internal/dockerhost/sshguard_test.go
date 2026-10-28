package dockerhost

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a settable clock for the breaker.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

var errBadPassword = errors.New("docker over SSH: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain")

func TestAuthBreakerDoublesFromTwoToThirtyMinutesAndResets(t *testing.T) {
	clock := newFakeClock()
	b := newAuthBreaker(clock.Now)
	if b.remaining() != 0 {
		t.Fatal("a new breaker must be closed")
	}

	want := []time.Duration{2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 30 * time.Minute, 30 * time.Minute, 30 * time.Minute}
	for i, window := range want {
		b.fail()
		if got := b.remaining(); got != window {
			t.Fatalf("pause after failure %d = %v, want %v", i+1, got, window)
		}
		clock.advance(window - time.Second)
		if b.remaining() != time.Second {
			t.Fatalf("pause after %v = %v, want 1s left", window-time.Second, b.remaining())
		}
		clock.advance(time.Second)
		if b.remaining() != 0 {
			t.Fatalf("the breaker must close once the pause is over (failure %d)", i+1)
		}
	}

	// lift ends the running pause but keeps the series: the next failure doubles.
	lifted := newAuthBreaker(clock.Now)
	lifted.fail()
	lifted.lift()
	if lifted.remaining() != 0 {
		t.Fatal("lift must end the pause")
	}
	lifted.fail()
	if got := lifted.remaining(); got != 2*authBackoffMin {
		t.Fatalf("pause after lift and a second failure = %v, want the series to continue at %v", got, 2*authBackoffMin)
	}

	b.fail()
	b.reset()
	if b.remaining() != 0 {
		t.Fatal("reset must close the breaker")
	}
	b.fail()
	if got := b.remaining(); got != authBackoffMin {
		t.Fatalf("pause after a reset = %v, want the first window %v", got, authBackoffMin)
	}
}

func TestSSHGuardOnlyAuthFailuresPauseAndOneLoginRunsAtATime(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock()
	guard := newSSHGuard(clock.Now)

	var dials atomic.Int32
	fail := func(err error) func(context.Context) (net.Conn, error) {
		return func(context.Context) (net.Conn, error) {
			dials.Add(1)
			return nil, err
		}
	}

	// Failures that never reached a login (refused, timeout, host key, socket
	// open) leave the breaker closed.
	for _, err := range []error{
		errors.New("dial tcp 10.0.0.7:22: connect: connection refused"),
		errors.New("docker over SSH: dial: i/o timeout"),
		errors.New("docker over SSH: knownhosts: key mismatch"),
		errors.New("docker over SSH: open /var/run/docker.sock: ssh: rejected: connect failed (Permission denied)"),
	} {
		if _, got := guard.dial(ctx, fail(err)); got != err {
			t.Fatalf("dial = %v, want the dial error unchanged", got)
		}
		if guard.breaker.remaining() != 0 {
			t.Fatalf("%q must not pause SSH logins", err)
		}
	}

	// A rejected login pauses the host: later dials fail fast, without dialing,
	// with an error that classifies as auth and names the wait.
	before := dials.Load()
	if _, err := guard.dial(ctx, fail(errBadPassword)); err != errBadPassword {
		t.Fatalf("dial = %v", err)
	}
	for i := 0; i < 3; i++ {
		_, err := guard.dial(ctx, fail(errBadPassword))
		if err == nil || ClassifyError(err) != CodeAuth || !strings.Contains(err.Error(), "SSH login paused after an authentication failure; next attempt in 2m0s") {
			t.Fatalf("dial while paused = %v, want a paused error classified as auth", err)
		}
	}
	if got := dials.Load() - before; got != 1 {
		t.Fatalf("%d logins were attempted, want 1", got)
	}

	// After the pause one login is allowed again; a second failure doubles it.
	clock.advance(authBackoffMin)
	if _, err := guard.dial(ctx, fail(errBadPassword)); err != errBadPassword {
		t.Fatalf("dial after the pause = %v", err)
	}
	if got := guard.breaker.remaining(); got != 2*authBackoffMin {
		t.Fatalf("second pause = %v, want %v", got, 2*authBackoffMin)
	}

	// A successful dial resets the series.
	clock.advance(2 * authBackoffMin)
	a, b := net.Pipe()
	defer b.Close()
	conn, err := guard.dial(ctx, func(context.Context) (net.Conn, error) { return a, nil })
	if err != nil || conn != a {
		t.Fatalf("dial = (%v, %v), want the connection", conn, err)
	}
	_ = conn.Close()
	if _, err := guard.dial(ctx, fail(errBadPassword)); err != errBadPassword || guard.breaker.remaining() != authBackoffMin {
		t.Fatalf("after a success the next failure must start at %v, got %v (%v)", authBackoffMin, guard.breaker.remaining(), err)
	}
}

func TestSSHGuardBurstCausesAtMostOneLoginAttempt(t *testing.T) {
	guard := newSSHGuard(newFakeClock().Now)
	var dials, running, peak atomic.Int32
	slowFail := func(context.Context) (net.Conn, error) {
		dials.Add(1)
		if n := running.Add(1); n > peak.Load() {
			peak.Store(n)
		}
		time.Sleep(50 * time.Millisecond) // long enough for the whole burst to queue at the gate
		running.Add(-1)
		return nil, errBadPassword
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := guard.dial(context.Background(), slowFail)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	if dials.Load() != 1 || peak.Load() != 1 {
		t.Fatalf("burst made %d login attempts (%d at once), want 1", dials.Load(), peak.Load())
	}
	for err := range errs {
		if ClassifyError(err) != CodeAuth {
			t.Fatalf("burst error %v must classify as auth", err)
		}
	}

	// The wait for the gate ends with the context.
	held := newSSHGuard(nil)
	held.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := held.dial(ctx, slowFail); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("dial behind a held gate = %v, want the context error", err)
	}
}
