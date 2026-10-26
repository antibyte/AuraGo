package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sync"
	"time"
)

const (
	// yepAPIRateLimitPerMinute is the request limit documented for one API
	// key in prompts/tools_manuals/yepapi.md.
	yepAPIRateLimitPerMinute = 60
	// yepAPIRateLimitWindow is the period the limit applies to.
	yepAPIRateLimitWindow = time.Minute
	// yepAPITokensPerSecond is the refill rate of one key.
	yepAPITokensPerSecond = float64(yepAPIRateLimitPerMinute) / float64(yepAPIRateLimitWindow/time.Second)
)

// yepAPITokenBucket holds the remaining request budget of one API key.
type yepAPITokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}

// yepAPIRateLimiter paces outgoing requests so that one API key stays inside
// the documented per-minute limit. It only delays a request before it is
// sent; a limited request is never resent.
type yepAPIRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*yepAPITokenBucket
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error
}

// yepAPIGlobalRateLimiter is the limiter shared by every YepAPI client. It
// lives next to the client cache because both are keyed by the API key.
var yepAPIGlobalRateLimiter = newYepAPIRateLimiter()

// newYepAPIRateLimiter returns a limiter with a full budget per key.
func newYepAPIRateLimiter() *yepAPIRateLimiter {
	return &yepAPIRateLimiter{
		buckets: map[string]*yepAPITokenBucket{},
		now:     time.Now,
		sleep:   yepAPIWaitForContext,
	}
}

// wait blocks until the key may send one more request. It returns the context
// error when the context ends before the request may be sent.
func (l *yepAPIRateLimiter) wait(ctx context.Context, apiKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bucket := l.bucketFor(apiKey)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	for {
		l.refill(bucket)
		if bucket.tokens >= 1 {
			bucket.tokens--
			return nil
		}
		delay := time.Duration((1 - bucket.tokens) / yepAPITokensPerSecond * float64(time.Second))
		if err := l.sleep(ctx, delay); err != nil {
			return err
		}
	}
}

// bucketFor returns the budget of one API key and creates it on first use.
func (l *yepAPIRateLimiter) bucketFor(apiKey string) *yepAPITokenBucket {
	key := yepAPIRateLimitKey(apiKey)
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &yepAPITokenBucket{tokens: yepAPIRateLimitPerMinute, lastRefill: l.now()}
		l.buckets[key] = bucket
	}
	return bucket
}

// refill adds the tokens that accrued since the last refill.
func (l *yepAPIRateLimiter) refill(bucket *yepAPITokenBucket) {
	now := l.now()
	elapsed := now.Sub(bucket.lastRefill)
	if elapsed <= 0 {
		return
	}
	bucket.tokens = math.Min(yepAPIRateLimitPerMinute, bucket.tokens+elapsed.Seconds()*yepAPITokensPerSecond)
	bucket.lastRefill = now
}

// yepAPIRateLimitKey hashes the API key so that the secret never becomes a
// map key in memory.
func yepAPIRateLimitKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey + "|yepapi-rate-limit"))
	return hex.EncodeToString(sum[:])
}

// yepAPIWaitForContext waits for the given duration unless the context ends
// first.
func yepAPIWaitForContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
