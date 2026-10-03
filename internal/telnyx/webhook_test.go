package telnyx

import (
	"crypto/ed25519"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestWebhookUsesAccountPublicKeyAndObjectSender(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Telnyx.WebhookPublicKey = base64.StdEncoding.EncodeToString(pubKey)
	cfg.Telnyx.AllowedNumbers = []string{"+13125550001"}
	cfg.Telnyx.RelayToAgent = true
	cfg.Telnyx.MaxSMSPerMinute = 10
	gotSMS := make(chan string, 1)
	h := NewWebhookHandler(cfg, slog.Default(), func(from, text string, _ []string) {
		gotSMS <- from + ":" + text
	}, nil)
	body := []byte(`{"data":{"event_type":"message.received","id":"event-1","payload":{"from":{"phone_number":"+13125550001"},"text":"Hello"}}}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := []byte(timestamp + "|")
	message = append(message, body...)
	signature := ed25519.Sign(privKey, message)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(signature))
	req.Header.Set(TimestampHeader, timestamp)
	if !verifyWebhookSignature(req, body, cfg.Telnyx.WebhookPublicKey) {
		t.Fatal("valid account signature rejected")
	}
	w := httptest.NewRecorder()
	h.HandleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	select {
	case got := <-gotSMS:
		if got != "+13125550001:Hello" {
			t.Fatalf("SMS = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("SMS callback not called")
	}
}

func TestVerifyWebhookSignature_MissingHeaders(t *testing.T) {
	body := []byte(`{"test": true}`)

	// No headers
	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	if verifyWebhookSignature(req, body, "") {
		t.Error("expected false for missing headers")
	}

	// Only signature
	req.Header.Set(SignatureHeader, "dGVzdA==")
	if verifyWebhookSignature(req, body, "") {
		t.Error("expected false for missing timestamp")
	}
}

func TestVerifyWebhookSignature_ExpiredTimestamp(t *testing.T) {
	body := []byte(`{"test": true}`)
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	oldTs := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)

	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(ed25519.Sign(privKey, append([]byte(oldTs+"|"), body...))))
	req.Header.Set(TimestampHeader, oldTs)

	if verifyWebhookSignature(req, body, base64.StdEncoding.EncodeToString(pubKey)) {
		t.Error("expected false for expired timestamp")
	}
}

func TestVerifyWebhookSignatureRejectsTampering(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	covered := []byte(`{"test":true}`)
	req, _ := http.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set(TimestampHeader, timestamp)
	req.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(ed25519.Sign(privKey, append([]byte(timestamp+"|"), covered...))))
	if verifyWebhookSignature(req, []byte(`{"test":false}`), base64.StdEncoding.EncodeToString(pubKey)) {
		t.Fatal("modified body passed signature check")
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
