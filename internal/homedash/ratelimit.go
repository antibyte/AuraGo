package homedash

import (
	"sync"
	"time"
)

type window struct {
	key   string
	limit int
	per   time.Duration
}

type hitList struct {
	per time.Duration
	ts  []time.Time
}

// rateLimiter is a sliding-window limiter (flowRateLimiter semantics, flows_handlers.go:72-107)
// that checks several windows atomically. The zero value is ready. mu guards hits and lastSweep.
type rateLimiter struct {
	mu        sync.Mutex
	hits      map[string]*hitList
	lastSweep time.Time
}

const (
	rateKeysSweep = 256
	// maxRateKeys bounds the map: when every key still holds live hits, a new key is refused.
	maxRateKeys = 8192
)

// sweepLocked drops keys without a live hit.
func (l *rateLimiter) sweepLocked(now time.Time) {
	for k, h := range l.hits {
		if len(h.ts) == 0 || !h.ts[len(h.ts)-1].After(now.Add(-h.per)) {
			delete(l.hits, k)
		}
	}
	l.lastSweep = now
}

// allow records one event in every window when all have room; otherwise it records nothing and
// returns how long until the fullest window frees a slot.
func (l *rateLimiter) allow(now time.Time, ws []window) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = map[string]*hitList{}
	}
	if len(l.hits) > rateKeysSweep && (len(l.hits) >= maxRateKeys || now.Sub(l.lastSweep) >= time.Second || now.Before(l.lastSweep)) {
		l.sweepLocked(now)
	}
	ok, wait := true, time.Duration(0)
	var fresh []string // keys this call created; removed again when the event is refused
	for _, w := range ws {
		if w.limit <= 0 {
			continue
		}
		h := l.hits[w.key]
		if h == nil {
			if len(l.hits) >= maxRateKeys {
				ok = false
				wait = max(wait, time.Second) // fail closed instead of growing without bound
				continue
			}
			h = &hitList{}
			l.hits[w.key] = h
			fresh = append(fresh, w.key)
		}
		h.per = w.per
		cut := now.Add(-w.per)
		i := 0
		for i < len(h.ts) && !h.ts[i].After(cut) {
			i++
		}
		h.ts = h.ts[i:]
		if len(h.ts) >= w.limit {
			ok = false
			if d := h.ts[len(h.ts)-w.limit].Add(w.per).Sub(now); d > wait {
				wait = d
			}
		}
	}
	if !ok {
		for _, k := range fresh {
			delete(l.hits, k)
		}
		return false, wait
	}
	for _, w := range ws {
		if w.limit > 0 {
			l.hits[w.key].ts = append(l.hits[w.key].ts, now)
		}
	}
	return true, 0
}
