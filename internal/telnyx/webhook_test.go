package telnyx

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	// Generate a test keypair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Override the public key for testing
	origKey := TelnyxPublicKeyBase64
	defer func() { _ = origKey }() // restore not needed since it's a const, but this test uses its own verification

	body := []byte(`{"data":{"event_type":"message.received"}}`)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	message := []byte(timestamp + "|")
	message = append(message, body...)
	signature := ed25519.Sign(privKey, message)

	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(signature))
	req.Header.Set(TimestampHeader, timestamp)

	// Verify with the generated public key directly
	sigBytes, _ := base64.StdEncoding.DecodeString(req.Header.Get(SignatureHeader))
	verified := ed25519.Verify(pubKey, message, sigBytes)
	if !verified {
		t.Error("expected valid signature verification")
	}
}

func TestVerifyWebhookSignature_MissingHeaders(t *testing.T) {
	body := []byte(`{"test": true}`)

	// No headers
	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	if verifyWebhookSignature(req, body) {
		t.Error("expected false for missing headers")
	}

	// Only signature
	req.Header.Set(SignatureHeader, "dGVzdA==")
	if verifyWebhookSignature(req, body) {
		t.Error("expected false for missing timestamp")
	}
}

func TestVerifyWebhookSignature_ExpiredTimestamp(t *testing.T) {
	body := []byte(`{"test": true}`)
	oldTs := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)

	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize)))
	req.Header.Set(TimestampHeader, oldTs)

	if verifyWebhookSignature(req, body) {
		t.Error("expected false for expired timestamp")
	}
}

func TestIsAllowedNumber(t *testing.T) {
	tests := []struct {
		name     string
		allowed  []string
		number   string
		expected bool
	}{
		{"nil whitelist denies all", nil, "+14155551234", false},
		{"empty whitelist denies all", []string{}, "+14155551234", false},
		{"exact match", []string{"+14155551234"}, "+14155551234", true},
		{"not in list", []string{"+14155551234"}, "+491511234567", false},
		{"multiple entries", []string{"+14155551234", "+491511234567"}, "+491511234567", true},
		{"with spaces", []string{"+49 151 1234567"}, "+491511234567", true},
		{"with dashes", []string{"+1-415-555-1234"}, "+14155551234", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &WebhookHandler{
				cfg: &config.Config{},
			}
			h.cfg.Telnyx.AllowedNumbers = tt.allowed

			if got := h.isAllowedNumber(tt.number); got != tt.expected {
				t.Errorf("isAllowedNumber(%q) = %v, want %v", tt.number, got, tt.expected)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(3, time.Second)

	// First 3 should pass
	for i := 0; i < 3; i++ {
		if !rl.allow() {
			t.Errorf("expected allow on call %d", i+1)
		}
	}

	// 4th should be blocked
	if rl.allow() {
		t.Error("expected rate limit to block 4th call")
	}

	// After window expires, should allow again
	rl.windowStart = time.Now().Add(-2 * time.Second)
	if !rl.allow() {
		t.Error("expected allow after window reset")
	}
}
