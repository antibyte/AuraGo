package dockerutil

import (
	"testing"
	"time"
)

func TestHTTPClientWithTimeoutKeepsTransportAndOverridesTimeout(t *testing.T) {
	client := NewClient("", 30*time.Second)
	streaming := client.HTTPClientWithTimeout(30 * time.Minute)
	if streaming == nil {
		t.Fatal("streaming client is nil")
	}
	if streaming.Timeout != 30*time.Minute {
		t.Fatalf("streaming timeout = %s, want 30m", streaming.Timeout)
	}
	if streaming.Transport != client.HTTPClient().Transport {
		t.Fatal("streaming client must reuse the Docker transport")
	}
}
