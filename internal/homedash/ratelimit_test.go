package homedash

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiterWindowsAreAtomic(t *testing.T) {
	var l rateLimiter
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	ws := []window{{"p", 3, time.Minute}, {"e", 2, 5 * time.Second}}
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow(base, ws); !ok {
			t.Fatalf("event %d refused", i)
		}
	}
	ok, wait := l.allow(base.Add(time.Second), ws)
	if ok || wait != 4*time.Second {
		t.Fatalf("third event: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.allow(base.Add(6*time.Second), ws); !ok {
		t.Fatal("refused after the entity window passed")
	}
	if ok, _ := l.allow(base.Add(7*time.Second), []window{{"p", 3, time.Minute}}); ok {
		t.Fatal("the refused event must not have counted, but the third accepted one must")
	}
}

func TestRateLimiterMemoryIsBounded(t *testing.T) {
	var l rateLimiter
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3*maxRateKeys; i++ {
		now := base.Add(time.Duration(i) * 2 * time.Second) // every earlier key has expired
		if ok, _ := l.allow(now, []window{{fmt.Sprintf("k%d", i), 1, time.Second}}); !ok {
			t.Fatalf("event %d refused although all older keys expired", i)
		}
	}
	l.mu.Lock()
	n := len(l.hits)
	l.mu.Unlock()
	if n > maxRateKeys {
		t.Fatalf("%d keys kept, cap %d", n, maxRateKeys)
	}
	// Live keys beyond the cap fail closed instead of growing the map.
	var m rateLimiter
	refused := false
	for i := 0; i < maxRateKeys+50; i++ {
		if ok, _ := m.allow(base, []window{{fmt.Sprintf("k%d", i), 1, time.Hour}}); !ok {
			refused = true
		}
	}
	m.mu.Lock()
	n = len(m.hits)
	m.mu.Unlock()
	if !refused || n > maxRateKeys {
		t.Fatalf("refused=%v keys=%d", refused, n)
	}
}
