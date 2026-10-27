package dockerhost

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// SSH login protection. A wrong SSH password makes every Engine request dial
// and fail a login; sshd's MaxStartups and fail2ban (5 failures in 10 minutes
// by default) then lock out the whole AuraGo machine, which also breaks
// Invasion's SSH. Each stored SSH host therefore gets an authBreaker that stops
// further logins for a while after an authentication failure, and a gate that
// lets one login at a time through.
const (
	// authBackoffMin is the pause after the first authentication failure; every
	// further consecutive failure doubles it.
	authBackoffMin = 2 * time.Minute
	// authBackoffMax caps the pause.
	authBackoffMax = 30 * time.Minute
)

// authBreaker pauses the logins of one host after authentication failures. The
// pause starts at authBackoffMin and doubles per consecutive failure up to
// authBackoffMax; a success or reset closes the breaker again.
type authBreaker struct {
	now func() time.Time

	mu       sync.Mutex
	failures int
	until    time.Time
}

func newAuthBreaker(now func() time.Time) *authBreaker {
	if now == nil {
		now = time.Now
	}
	return &authBreaker{now: now}
}

// remaining returns how long logins stay paused; 0 means the breaker is closed.
func (b *authBreaker) remaining() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	if left := b.until.Sub(b.now()); left > 0 {
		return left
	}
	return 0
}

// fail records an authentication failure and opens the breaker for the next
// pause of the series.
func (b *authBreaker) fail() {
	b.mu.Lock()
	defer b.mu.Unlock()
	pause := authBackoffMin
	for i := 0; i < b.failures && pause < authBackoffMax; i++ {
		pause *= 2
	}
	if pause > authBackoffMax {
		pause = authBackoffMax
	}
	if b.failures < 1<<20 { // the count only needs to outgrow the cap
		b.failures++
	}
	b.until = b.now().Add(pause)
}

// reset closes the breaker and forgets the failure series.
func (b *authBreaker) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.until = time.Time{}
}

// pausedError is the error of a login that was not attempted because the
// breaker is open. It classifies as CodeAuth.
func pausedError(left time.Duration) error {
	return withCode(CodeAuth, fmt.Errorf(
		"SSH login paused after an authentication failure; next attempt in %s. Fix the credentials or press Test",
		left.Round(time.Second)))
}

// sshGuard protects one SSH host: the breaker plus a gate of one concurrent
// dial, so a burst of parallel requests causes at most one login attempt.
// Connections that are already pooled are not affected.
type sshGuard struct {
	breaker *authBreaker
	gate    chan struct{}
}

func newSSHGuard(now func() time.Time) *sshGuard {
	return &sshGuard{breaker: newAuthBreaker(now), gate: make(chan struct{}, 1)}
}

// dial runs one login attempt through the guard: fail fast while the breaker is
// open, otherwise wait for the gate (ctx ends the wait), look at the breaker
// again - the dial ahead may have just failed to log in - and then dial.
func (g *sshGuard) dial(ctx context.Context, do func(ctx context.Context) (net.Conn, error)) (net.Conn, error) {
	if left := g.breaker.remaining(); left > 0 {
		return nil, pausedError(left)
	}
	select {
	case g.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-g.gate }()
	if left := g.breaker.remaining(); left > 0 {
		return nil, pausedError(left)
	}
	conn, err := do(ctx)
	switch {
	case err == nil:
		g.breaker.reset()
	case isLoginFailure(err):
		g.breaker.fail()
	}
	return conn, err
}

// isLoginFailure reports a dial error that is a rejected SSH login. A failure
// to open the Engine socket after a successful login can read like "permission
// denied" too, but it is no failed login and must not pause the host.
func isLoginFailure(err error) bool {
	return ClassifyError(err) == CodeAuth && !strings.Contains(strings.ToLower(err.Error()), "docker over ssh: open ")
}
