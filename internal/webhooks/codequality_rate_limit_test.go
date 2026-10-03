package webhooks

import (
	"fmt"
	"testing"
	"time"
)

func TestWebhookBucketsAreBoundedAndExpire(t *testing.T) {
	now := time.Now()
	limiter := newRateLimiterWithClock(1, func() time.Time { return now })
	for i := 0; i < 4096; i++ {
		if !limiter.Allow(fmt.Sprint(i)) {
			t.Fatal("capacity rejected early")
		}
	}
	if limiter.Allow("overflow") || len(limiter.buckets) != 4096 {
		t.Fatal("unbounded buckets")
	}
	now = now.Add(6 * time.Minute)
	if !limiter.Allow("new-client") || len(limiter.buckets) != 1 {
		t.Fatal("expired clients retained")
	}
}
