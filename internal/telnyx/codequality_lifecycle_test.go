package telnyx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

type boundaryTransport func(*http.Request) (*http.Response, error)

func (f boundaryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProductionCallAndSMSPathsDoNotReplayMutations(t *testing.T) {
	c := newAllowlistedTestClient("synthetic-test-key", "+15550000002")
	c.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	var paths []string
	c.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.EscapedPath())
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`{"error":"unconfirmed"}`)), Header: make(http.Header)}, nil
	})
	if err := c.AnswerCall(context.Background(), "call/id"); err == nil || !strings.Contains(err.Error(), "not retried") {
		t.Fatalf("answer error=%v", err)
	}
	_, err := c.SendSMS(context.Background(), "+15550000001", "+15550000002", "synthetic message", "")
	if err == nil {
		t.Fatal("expected unconfirmed SMS failure")
	}
	if len(paths) != 2 || paths[0] != "/v2/calls/call%2Fid/actions/answer" || paths[1] != "/v2/messages" {
		t.Fatalf("mutation requests=%v", paths)
	}
}

func TestReconciliationRetainsUncertainCallsAndRemovesConfirmedEnds(t *testing.T) {
	for _, tc := range []struct {
		name, alive, end, id string
		status               int
		remove               bool
	}{
		{"ended", "false", "2026-10-03T12:00:00Z", "call", 200, true},
		{"alive", "true", "", "call", 200, false},
		{"dial-pending", "false", "", "call", 200, false},
		{"missing-alive", "null", "2026-10-03T12:00:00Z", "call", 200, false},
		{"wrong-id", "false", "2026-10-03T12:00:00Z", "different", 200, false},
		{"provider-error", "false", "2026-10-03T12:00:00Z", "call", 401, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient("synthetic-test-key", slog.New(slog.NewTextHandler(io.Discard, nil)))
			c.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/v2/calls/call" || r.Method != http.MethodGet {
					t.Errorf("status request=%s %s", r.Method, r.URL.Path)
				}
				body := fmt.Sprintf(`{"data":{"call_control_id":%q,"is_alive":%s,"end_time":%q}}`, tc.id, tc.alive, tc.end)
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			h := &WebhookHandler{cfg: &config.Config{}, client: c, logger: c.logger, activeCalls: map[string]*CallSession{"call": {LastActivity: time.Now()}}}
			h.reconcileActiveCalls(context.Background())
			if (len(h.activeCalls) == 0) != tc.remove {
				t.Fatalf("calls retained=%d remove=%v", len(h.activeCalls), tc.remove)
			}
		})
	}
}
