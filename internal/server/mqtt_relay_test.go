package server

import (
	"testing"
	"time"
)

func TestMQTTRelayLimiterDebouncesPerTopic(t *testing.T) {
	limiter := newMQTTRelayLimiter(2 * time.Second)
	now := time.Unix(100, 0)

	if !limiter.Allow("home/a", now) {
		t.Fatal("first message for topic should be allowed")
	}
	if limiter.Allow("home/a", now.Add(time.Second)) {
		t.Fatal("second message inside debounce window should be blocked")
	}
	if !limiter.Allow("home/b", now.Add(time.Second)) {
		t.Fatal("different topic should have its own debounce window")
	}
	if !limiter.Allow("home/a", now.Add(2*time.Second)) {
		t.Fatal("message at debounce boundary should be allowed")
	}
}
