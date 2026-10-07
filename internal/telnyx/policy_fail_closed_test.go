package telnyx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
)

// newAllowlistedTestClient is a writable client whose number policy allows
// exactly the given destinations, for tests that exercise mutation paths.
func newAllowlistedTestClient(apiKey string, allowed ...string) *Client {
	c := NewClient(apiKey, nil)
	c.policy = &numberPolicy{allowed: append([]string(nil), allowed...)}
	return c
}

// A bare NewClient has no allowlist and no read-only flag, so it may read but
// never send, call, transfer or otherwise mutate.
func TestTelnyxClientWithoutPolicyFailsClosedOnMutation(t *testing.T) {
	c := NewClient("k", nil)
	var requests []string
	c.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"balance":"1.00"}}`)), Header: make(http.Header)}, nil
	})
	ctx := context.Background()
	mutations := map[string]func() error{
		"SendSMS": func() error {
			_, err := c.SendSMS(ctx, "+15550000001", "+15550000002", "fixture", "")
			return err
		},
		"SendMMS": func() error {
			_, err := c.SendMMS(ctx, "+15550000001", "+15550000002", "fixture", []string{"https://example.com/a.jpg"}, "")
			return err
		},
		"InitiateCall": func() error {
			_, err := c.InitiateCall(ctx, "application", "+15550000001", "+15550000002", "", 30, 300)
			return err
		},
		"TransferCall": func() error { return c.TransferCall(ctx, "call", "+15550000002", "+15550000001") },
		"AnswerCall":   func() error { return c.AnswerCall(ctx, "call") },
		"HangUp":       func() error { return c.HangUp(ctx, "call") },
	}
	for name, mutate := range mutations {
		if err := mutate(); err == nil || !strings.Contains(err.Error(), "number policy") {
			t.Errorf("%s without a number policy: error = %v, want the number policy refusal", name, err)
		}
	}
	if len(requests) != 0 {
		t.Fatalf("policy-less mutations reached the provider: %v", requests)
	}
	if _, err := c.GetBalance(ctx); err != nil {
		t.Fatalf("GetBalance without a number policy: %v", err)
	}
	if len(requests) != 1 || requests[0] != "GET /v2/balance" {
		t.Fatalf("balance read requests = %v", requests)
	}
}

func TestNewConfiguredClientAppliesTheNumberPolicy(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telnyx.Enabled = true
	cfg.Telnyx.APIKey = "k"
	cfg.Telnyx.AllowedNumbers = []string{"+15550000002"}
	c := NewConfiguredClient(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := c.validateDestination("+15550000002"); err != nil {
		t.Fatalf("allowed destination refused: %v", err)
	}
	if err := c.validateDestination("+15550000003"); err == nil || !strings.Contains(err.Error(), "allowed_numbers") {
		t.Fatalf("unlisted destination: error = %v, want the allowlist refusal", err)
	}
	cfg.Telnyx.ReadOnly = true
	readOnly := NewConfiguredClient(cfg, nil)
	readOnly.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("read-only client reached the provider: %s %s", r.Method, r.URL.Path)
		return nil, nil
	})
	if _, err := readOnly.SendSMS(context.Background(), "+15550000001", "+15550000002", "fixture", ""); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("read-only send: error = %v, want the read-only refusal", err)
	}
}
