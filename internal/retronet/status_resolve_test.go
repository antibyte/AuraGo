package retronet

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"
)

// publicProbeAddr is a public address for the DNS tests; the fake dialer never connects to it.
const publicProbeAddr = "93.184.215.14"

var (
	errDNSTemporary = &net.DNSError{Err: "server misbehaving", Name: "probe-00.example", Server: "127.0.0.53:53", IsTemporary: true}
	errDNSTimeout   = &net.DNSError{Err: "i/o timeout", Name: "probe-00.example", Server: "127.0.0.53:53", IsTimeout: true}
	errDNSNotFound  = &net.DNSError{Err: "no such host", Name: "probe-00.example", Server: "127.0.0.53:53", IsNotFound: true}
)

// dnsStep is one scripted lookup: an answer, an error, an answer or error after a delay, or a
// stall that lasts until the lookup's context ends.
type dnsStep struct {
	addr  string
	err   error
	delay time.Duration
	stall bool
}

// dnsScript answers lookups step by step; the last step repeats. It records the budget of every
// lookup (time left until the context deadline when the lookup started).
type dnsScript struct {
	mu      sync.Mutex
	steps   []dnsStep
	budgets []time.Duration
}

func newDNSScript(steps ...dnsStep) *dnsScript { return &dnsScript{steps: steps} }

func (s *dnsScript) set(steps ...dnsStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = steps
}

func (s *dnsScript) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.budgets)
}

func (s *dnsScript) budget(i int) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.budgets[i]
}

func (s *dnsScript) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	s.mu.Lock()
	step := s.steps[0]
	if len(s.steps) > 1 {
		s.steps = s.steps[1:]
	}
	budget := time.Duration(-1)
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	s.budgets = append(s.budgets, budget)
	s.mu.Unlock()
	timedOut := func() error {
		return &net.DNSError{Err: ctx.Err().Error(), Name: host, IsTimeout: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	}
	if step.stall {
		<-ctx.Done()
		return nil, timedOut()
	}
	if step.delay > 0 {
		timer := time.NewTimer(step.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, timedOut()
		}
	}
	if step.err != nil {
		return nil, step.err
	}
	return []net.IPAddr{{IP: net.ParseIP(step.addr)}}, nil
}

// targetLog records the addresses probes dialed.
type targetLog struct {
	mu      sync.Mutex
	targets []string
}

func (l *targetLog) wrap(dial DialFunc) DialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		l.mu.Lock()
		l.targets = append(l.targets, address)
		l.mu.Unlock()
		return dial(ctx, network, address)
	}
}

func (l *targetLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.targets)
}

// dnsDialer is d's dialer with script as resolver and the public-address check in force.
func dnsDialer(d *probeDialer, script *dnsScript, log *targetLog) Dialer {
	dial := d.dial
	if log != nil {
		dial = log.wrap(dial)
	}
	return Dialer{Resolver: script, Dial: dial}
}

// shortRetryPause shortens the pause before the second lookup attempt for one test.
func shortRetryPause(t *testing.T) {
	previous := resolveRetryPause
	resolveRetryPause = 10 * time.Millisecond
	t.Cleanup(func() { resolveRetryPause = previous })
}

// Slow DNS must not eat the connect budget: an answer after 150 ms still leaves the whole 100 ms
// connect timeout, and the lookup runs under ResolveTimeout, not under the connect timeout.
func TestStatusProberLookupHasItsOwnBudget(t *testing.T) {
	d := newProbeDialer(false)
	script := newDNSScript(dnsStep{addr: publicProbeAddr, delay: 150 * time.Millisecond})
	p := &StatusProber{Dialer: dnsDialer(d, script, nil), Timeout: 100 * time.Millisecond, ResolveTimeout: 2 * time.Second}
	st := p.Refresh(context.Background(), probeEntries(1), 5*time.Second)["probe-00"]
	if st.State != statusOnline || d.calls.Load() != 1 {
		t.Fatalf("status %+v after %d dials, want online after one dial", st, d.calls.Load())
	}
	if budget := script.budget(0); budget < time.Second {
		t.Fatalf("lookup budget %v, want the 2 s ResolveTimeout", budget)
	}
}

// A lookup that fails with anything but "no such host" is tried once more; a third attempt never
// happens, and two failures leave the entry unknown without a dial.
func TestStatusProberRetriesAFailedLookupOnce(t *testing.T) {
	shortRetryPause(t)
	cases := []struct {
		name    string
		steps   []dnsStep
		lookups int
		state   string
		dials   int64
	}{
		{"temporary failure, then an answer", []dnsStep{{err: errDNSTemporary}, {addr: publicProbeAddr}}, 2, statusOnline, 1},
		{"timeout error, then an answer", []dnsStep{{err: errDNSTimeout}, {addr: publicProbeAddr}}, 2, statusOnline, 1},
		{"other resolver error, then an answer", []dnsStep{{err: errors.New("read udp: connection refused")}, {addr: publicProbeAddr}}, 2, statusOnline, 1},
		{"two failures", []dnsStep{{err: errDNSTemporary}}, 2, statusUnknown, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newProbeDialer(false)
			script := newDNSScript(tc.steps...)
			p := &StatusProber{Dialer: dnsDialer(d, script, nil)}
			st := p.Refresh(context.Background(), probeEntries(1), 5*time.Second)["probe-00"]
			if st.State != tc.state || script.calls() != tc.lookups || d.calls.Load() != tc.dials {
				t.Fatalf("status %+v after %d lookups and %d dials, want %s after %d and %d",
					st, script.calls(), d.calls.Load(), tc.state, tc.lookups, tc.dials)
			}
		})
	}
}

// "No such host" is a final answer: the board is offline, the lookup is not repeated and the
// address of an earlier answer is not dialed.
func TestStatusProberNoSuchHostIsOffline(t *testing.T) {
	shortRetryPause(t)
	clock := newProbeClock()
	d := newProbeDialer(false)
	script := newDNSScript(dnsStep{addr: publicProbeAddr})
	p := &StatusProber{Dialer: dnsDialer(d, script, nil), Now: clock.Now}
	entries := probeEntries(1)
	onlineAt := clock.Now()
	p.Refresh(context.Background(), entries, 5*time.Second)

	script.set(dnsStep{err: errDNSNotFound})
	clock.Advance(61 * time.Second)
	st := p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
	if st.State != statusOffline || st.CheckedAt == nil || !st.CheckedAt.Equal(clock.Now()) || st.LastOnlineAt == nil || !st.LastOnlineAt.Equal(onlineAt) {
		t.Fatalf("status = %+v, want offline checked now, last online %v", st, onlineAt)
	}
	if script.calls() != 2 || d.calls.Load() != 1 {
		t.Fatalf("lookups %d, dials %d; want 2 and 1 (no retry, no dial of the old answer)", script.calls(), d.calls.Load())
	}
}

// While lookups fail, the last answer of the past hour is dialed (a real TCP check); after that
// the entry keeps the result it got within the hour instead of turning offline.
func TestStatusProberDialsTheLastAnswerWhileLookupsFail(t *testing.T) {
	shortRetryPause(t)
	clock := newProbeClock()
	d := newProbeDialer(false)
	script := newDNSScript(dnsStep{addr: publicProbeAddr})
	log := &targetLog{}
	p := &StatusProber{Dialer: dnsDialer(d, script, log), Now: clock.Now}
	entries := probeEntries(1)
	p.Refresh(context.Background(), entries, 5*time.Second)

	script.set(dnsStep{err: errDNSTemporary})
	clock.Advance(59 * time.Minute)
	checked := clock.Now()
	st := p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
	if st.State != statusOnline || st.CheckedAt == nil || !st.CheckedAt.Equal(checked) {
		t.Fatalf("status with a 59 min old answer = %+v, want online checked now", st)
	}
	want := []string{publicProbeAddr + ":23", publicProbeAddr + ":23"}
	if got := log.list(); !slices.Equal(got, want) {
		t.Fatalf("dialed %v, want %v", got, want)
	}

	clock.Advance(2 * time.Minute) // the answer is 61 min old now, the last check 2 min
	st = p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
	if st.State != statusOnline || st.CheckedAt == nil || !st.CheckedAt.Equal(checked) {
		t.Fatalf("status without a usable answer = %+v, want the online result checked at %v", st, checked)
	}
	if d.calls.Load() != 2 {
		t.Fatalf("dials = %d, want 2: an answer older than an hour is not dialed", d.calls.Load())
	}
	p.mu.Lock()
	kept := len(p.answers)
	p.mu.Unlock()
	if kept != 0 {
		t.Fatalf("%d answers kept, want answers older than an hour dropped", kept)
	}
}

// A failed lookup never marks an entry offline: it is unknown when it was never checked and
// when its last check is more than an hour old (the last online time stays).
func TestStatusProberLookupFailureNeverMarksOffline(t *testing.T) {
	shortRetryPause(t)

	t.Run("never checked", func(t *testing.T) {
		clock := newProbeClock()
		d := newProbeDialer(false)
		p := &StatusProber{Dialer: dnsDialer(d, newDNSScript(dnsStep{err: errDNSTemporary}), nil), Now: clock.Now}
		entries := probeEntries(1)
		st := p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
		if st.State != statusUnknown || st.CheckedAt != nil || st.LastOnlineAt != nil {
			t.Fatalf("status = %+v, want unknown, never checked", st)
		}
		if _, stale := p.Snapshot(entries); stale {
			t.Fatal("an entry whose lookup failed must not keep the snapshot stale")
		}
	})

	t.Run("last check over an hour old", func(t *testing.T) {
		clock := newProbeClock()
		d := newProbeDialer(false)
		script := newDNSScript(dnsStep{addr: publicProbeAddr})
		p := &StatusProber{Dialer: dnsDialer(d, script, nil), Now: clock.Now}
		entries := probeEntries(1)
		onlineAt := clock.Now()
		p.Refresh(context.Background(), entries, 5*time.Second)
		script.set(dnsStep{err: errDNSTemporary})
		clock.Advance(61 * time.Minute)
		st := p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
		if st.State != statusUnknown || st.LastOnlineAt == nil || !st.LastOnlineAt.Equal(onlineAt) {
			t.Fatalf("status = %+v, want unknown, last online %v", st, onlineAt)
		}
	})
}

// A stalled lookup gives up after CachedResolveTimeout when an earlier answer can stand in, and
// after ResolveTimeout otherwise; a stalled first attempt leaves no time for a second one.
func TestStatusProberStalledLookupFallsBackQuickly(t *testing.T) {
	t.Run("with an earlier answer", func(t *testing.T) {
		clock := newProbeClock()
		d := newProbeDialer(false)
		script := newDNSScript(dnsStep{addr: publicProbeAddr})
		p := &StatusProber{Dialer: dnsDialer(d, script, nil), ResolveTimeout: 3 * time.Second, CachedResolveTimeout: 100 * time.Millisecond, Now: clock.Now}
		entries := probeEntries(1)
		p.Refresh(context.Background(), entries, 5*time.Second)
		script.set(dnsStep{stall: true})
		clock.Advance(61 * time.Second)
		started := time.Now()
		st := p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("run took %v, want about the 100 ms fallback budget", elapsed)
		}
		if st.State != statusOnline || d.calls.Load() != 2 || script.calls() != 2 {
			t.Fatalf("status %+v after %d dials and %d lookups, want online, 2 and 2", st, d.calls.Load(), script.calls())
		}
		if budget := script.budget(1); budget > 100*time.Millisecond {
			t.Fatalf("stalled lookup budget %v, want at most 100 ms", budget)
		}
	})

	t.Run("without an answer", func(t *testing.T) {
		d := newProbeDialer(false)
		script := newDNSScript(dnsStep{stall: true})
		p := &StatusProber{Dialer: dnsDialer(d, script, nil), ResolveTimeout: 200 * time.Millisecond, CachedResolveTimeout: 50 * time.Millisecond}
		started := time.Now()
		st := p.Refresh(context.Background(), probeEntries(1), 5*time.Second)["probe-00"]
		if elapsed := time.Since(started); elapsed < 150*time.Millisecond || elapsed > 2*time.Second {
			t.Fatalf("run took %v, want about the 200 ms ResolveTimeout", elapsed)
		}
		if st.State != statusUnknown || d.calls.Load() != 0 || script.calls() != 1 {
			t.Fatalf("status %+v after %d dials and %d lookups, want unknown, 0 and 1", st, d.calls.Load(), script.calls())
		}
	})
}

// Every address a probe dials passes DialEntry's public-address check, a kept answer included,
// and a fresh answer always replaces the kept one (a name that now points inside is refused even
// though an older answer was public).
func TestStatusProberKeptAnswersStayGuarded(t *testing.T) {
	shortRetryPause(t)
	clock := newProbeClock()
	d := newProbeDialer(false)
	script := newDNSScript(dnsStep{addr: "127.0.0.1"})
	p := &StatusProber{Dialer: dnsDialer(d, script, nil), Now: clock.Now}
	entries := probeEntries(1)
	run := func() Status {
		clock.Advance(61 * time.Second)
		return p.Refresh(context.Background(), entries, 5*time.Second)["probe-00"]
	}
	steps := []struct {
		name string
		dns  dnsStep
		want string
	}{
		{"loopback answer", dnsStep{addr: "127.0.0.1"}, statusOffline},
		{"lookup fails, kept loopback answer", dnsStep{err: errDNSTemporary}, statusOffline},
		{"public answer", dnsStep{addr: publicProbeAddr}, statusOnline},
		{"the name points to loopback again", dnsStep{addr: "127.0.0.1"}, statusOffline},
	}
	for _, step := range steps {
		script.set(step.dns)
		if st := run(); st.State != step.want {
			t.Fatalf("%s: status %+v, want %s", step.name, st, step.want)
		}
	}
	if d.calls.Load() != 1 {
		t.Fatalf("dials = %d, want 1: only the public answer may be dialed", d.calls.Load())
	}
}

// Answers belong to the directory of the last run: a host whose entry was removed loses its
// answer and cannot be dialed from it later.
func TestStatusProberForgetsAnswersOfRemovedEntries(t *testing.T) {
	shortRetryPause(t)
	clock := newProbeClock()
	d := newProbeDialer(false)
	script := newDNSScript(dnsStep{addr: publicProbeAddr})
	p := &StatusProber{Dialer: dnsDialer(d, script, nil), Now: clock.Now}
	both := probeEntries(2)
	p.Refresh(context.Background(), both, 5*time.Second)
	clock.Advance(61 * time.Second)
	p.Refresh(context.Background(), both[:1], 5*time.Second) // probe-01 left the directory
	script.set(dnsStep{err: errDNSTemporary})
	clock.Advance(61 * time.Second)
	statuses := p.Refresh(context.Background(), both, 5*time.Second)
	if statuses["probe-00"].State != statusOnline || statuses["probe-01"].State != statusUnknown {
		t.Fatalf("statuses = %+v, want probe-00 online from its kept answer, probe-01 unknown", statuses)
	}
	if d.calls.Load() != 4 {
		t.Fatalf("dials = %d, want 4 (2 + 1 + probe-00 again)", d.calls.Load())
	}
}

// IP literals need no lookup.
func TestStatusProberIPLiteralNeedsNoLookup(t *testing.T) {
	d := newProbeDialer(false)
	script := newDNSScript(dnsStep{err: errDNSTemporary})
	p := &StatusProber{Dialer: dnsDialer(d, script, nil)}
	entry := Entry{ID: "own-literal01", Protocol: ProtocolTelnet, Host: publicProbeAddr, Port: 23, Kind: KindBBS, Charset: CharsetCP437}
	st := p.Refresh(context.Background(), []Entry{entry}, 5*time.Second)[entry.ID]
	if st.State != statusOnline || script.calls() != 0 {
		t.Fatalf("status %+v after %d lookups, want online without a lookup", st, script.calls())
	}
}

func TestStatusProberLookupDefaults(t *testing.T) {
	zero := &StatusProber{}
	if zero.resolveTimeout() != 5*time.Second || zero.cachedResolveTimeout() != 2*time.Second {
		t.Errorf("defaults: resolve %v, cached %v; want 5s and 2s", zero.resolveTimeout(), zero.cachedResolveTimeout())
	}
	set := &StatusProber{ResolveTimeout: time.Second, CachedResolveTimeout: 300 * time.Millisecond}
	if set.resolveTimeout() != time.Second || set.cachedResolveTimeout() != 300*time.Millisecond {
		t.Errorf("configured values ignored: resolve %v, cached %v", set.resolveTimeout(), set.cachedResolveTimeout())
	}
	negative := &StatusProber{ResolveTimeout: -1, CachedResolveTimeout: -1}
	if negative.resolveTimeout() != 5*time.Second || negative.cachedResolveTimeout() != 2*time.Second {
		t.Errorf("negative values must fall back to the defaults")
	}
	if resolveRetryPause != 500*time.Millisecond || answerMaxAge != time.Hour {
		t.Errorf("retry pause %v, answer max age %v; want 500ms and 1h", resolveRetryPause, answerMaxAge)
	}
}
