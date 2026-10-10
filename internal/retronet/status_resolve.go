package retronet

import (
	"context"
	"errors"
	"net"
	"slices"
	"time"
)

// Name lookup defaults of status probes.
const (
	defaultResolveTimeout       = 5 * time.Second
	defaultCachedResolveTimeout = 2 * time.Second
	answerMaxAge                = time.Hour // how long an answer may stand in, and an unresolved entry keeps its result
)

// resolveRetryPause separates the two lookup attempts of a probe. It is a variable only so tests
// can shorten it.
var resolveRetryPause = 500 * time.Millisecond

// dnsAnswer is a successful lookup and when it was made.
type dnsAnswer struct {
	addrs []net.IPAddr
	at    time.Time
}

// lookupResult classifies the name lookup of one probe.
type lookupResult int

const (
	lookupAnswered lookupResult = iota // addresses to dial (fresh or standing in), or none needed
	lookupNotFound                     // final: the name does not exist or has no addresses
	lookupFailed                       // the resolver gave no usable answer: nothing is known
)

// lookup resolves host for a probe with its own budget, separate from the connect timeout:
// ResolveTimeout, or CachedResolveTimeout while an answer younger than answerMaxAge can stand
// in. A failure other than "no such host" is tried once more after resolveRetryPause within the
// same budget. A fresh answer always replaces the kept one; "no such host" drops it; any other
// failure falls back to it. IP literals and empty hosts need no lookup (nil, lookupAnswered):
// DialEntry handles them.
//
// The addresses are not checked here: probe hands them to DialEntry, which refuses restricted
// addresses and dials exactly the addresses it checked.
func (p *StatusProber) lookup(host string) ([]net.IPAddr, lookupResult) {
	if host == "" || net.ParseIP(host) != nil {
		return nil, lookupAnswered
	}
	kept, haveKept := p.keptAnswer(host)
	budget := p.resolveTimeout()
	if haveKept {
		budget = min(budget, p.cachedResolveTimeout())
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	resolver := p.Dialer.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil && !nameNotFound(err) && pauseForRetry(ctx) {
		addrs, err = resolver.LookupIPAddr(ctx, host)
	}
	switch {
	case err == nil && len(addrs) > 0:
		p.keepAnswer(host, addrs)
		return addrs, lookupAnswered
	case err == nil || nameNotFound(err):
		p.dropAnswer(host)
		return nil, lookupNotFound
	case haveKept:
		return kept, lookupAnswered
	default:
		return nil, lookupFailed
	}
}

// nameNotFound reports the resolver's final answer that a name does not exist.
func nameNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

// pauseForRetry waits resolveRetryPause and reports whether ctx still allows a second attempt.
func pauseForRetry(ctx context.Context) bool {
	timer := time.NewTimer(resolveRetryPause)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}

// keptAnswer returns the last answer for host when it is younger than answerMaxAge.
func (p *StatusProber) keptAnswer(host string) ([]net.IPAddr, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	answer, ok := p.answers[host]
	if !ok || p.now().Sub(answer.at) > answerMaxAge {
		return nil, false
	}
	return answer.addrs, true
}

func (p *StatusProber) keepAnswer(host string, addrs []net.IPAddr) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.answers == nil {
		p.answers = make(map[string]dnsAnswer)
	}
	p.answers[host] = dnsAnswer{addrs: slices.Clone(addrs), at: p.now()}
}

func (p *StatusProber) dropAnswer(host string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.answers, host)
}

// pruneAnswersLocked drops answers of hosts that are no longer in the directory and answers
// that are too old to stand in.
func (p *StatusProber) pruneAnswersLocked(entries []Entry, now time.Time) {
	hosts := make(map[string]bool, len(entries))
	for _, e := range entries {
		hosts[e.Host] = true
	}
	for host, answer := range p.answers {
		if !hosts[host] || now.Sub(answer.at) > answerMaxAge {
			delete(p.answers, host)
		}
	}
}

// answerResolver hands DialEntry the addresses a probe already looked up, so the connect budget
// is not spent on the lookup and DialEntry checks exactly the addresses it dials.
type answerResolver []net.IPAddr

// LookupIPAddr implements Resolver.
func (a answerResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return a, nil
}

func (p *StatusProber) resolveTimeout() time.Duration {
	if p.ResolveTimeout > 0 {
		return p.ResolveTimeout
	}
	return defaultResolveTimeout
}

func (p *StatusProber) cachedResolveTimeout() time.Duration {
	if p.CachedResolveTimeout > 0 {
		return p.CachedResolveTimeout
	}
	return defaultCachedResolveTimeout
}
