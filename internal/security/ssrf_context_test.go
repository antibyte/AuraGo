package security

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSSRFClientInitialPinningHonorsOwnerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := NewSSRFProtectedHTTPClientForURLContext(ctx, "https://owner-cancel.invalid", time.Second)
	if client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled DNS pinning returned client=%v error=%v", client != nil, err)
	}
}

func TestSSRFContextVariantPreservesAddressPolicyAndNilOwner(t *testing.T) {
	if client, err := NewSSRFProtectedHTTPClientForURLContext(nil, "https://8.8.8.8", time.Second); err != nil || client == nil {
		t.Fatalf("public literal construction: client=%v error=%v", client != nil, err)
	}
	for _, raw := range []string{"file:///etc/passwd", "http://0.0.0.0", "http://169.254.169.254"} {
		if _, err := NewSSRFProtectedHTTPClientForURLContext(context.Background(), raw, time.Second); err == nil {
			t.Fatalf("address policy allowed %q", raw)
		}
	}
}
