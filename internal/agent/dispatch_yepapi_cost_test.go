package agent

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// yepAPIDroppedConnectionServer accepts the request and closes the connection
// without answering, so the request reached the provider but the outcome is
// unknown. It returns the base URL and a counter of the requests it saw.
func yepAPIDroppedConnectionServer(t *testing.T) (string, *int32) {
	t.Helper()
	var seen int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server does not support connection hijacking")
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack connection: %v", err)
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(server.Close)
	return server.URL, &seen
}

// A call that reached the provider may have been billed even when it failed,
// so it must be booked against the budget. A call that never left the client
// must not be booked.
func TestDispatchYepAPIBooksCostForUncertainOutcome(t *testing.T) {
	t.Run("uncertain_outcome_is_booked", func(t *testing.T) {
		baseURL, seen := yepAPIDroppedConnectionServer(t)
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.YepAPI.BaseURL = baseURL

		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err == nil {
			t.Fatal("expected the dropped connection to fail the call")
		}
		if !strings.Contains(out, `"status":"error"`) {
			t.Fatalf("out = %q, want an error envelope", out)
		}
		if got := tracker.CategorySpendUSD("yepapi"); math.Abs(got-0.08) > 1e-9 {
			t.Fatalf("yepapi spend = %v, want 0.08 for a call that reached the provider", got)
		}
		if requests := atomic.LoadInt32(seen); requests != 1 {
			t.Fatalf("server saw %d requests, want exactly 1 (a failed call is never resent)", requests)
		}
	})

	t.Run("call_rejected_before_sending_is_not_booked", func(t *testing.T) {
		baseURL, seen := yepAPIDroppedConnectionServer(t)
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.YepAPI.BaseURL = baseURL

		cfg.Providers = nil

		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "no YepAPI API key found") {
			t.Fatalf("out = %q, want the missing-key message", out)
		}
		if got := tracker.CategorySpendUSD("yepapi"); got != 0 {
			t.Fatalf("yepapi spend = %v, want 0 for a call that never left the client", got)
		}
		if requests := atomic.LoadInt32(seen); requests != 0 {
			t.Fatalf("server saw %d requests, want none", requests)
		}
	})
	t.Run("provider_refusal_is_not_booked", func(t *testing.T) {
		baseURL, seen := yepAPIStatusServer(t, http.StatusForbidden)
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.YepAPI.BaseURL = baseURL
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err == nil {
			t.Fatal("expected the refused call to fail")
		}
		if !strings.Contains(out, `"status":"error"`) {
			t.Fatalf("out = %q, want an error envelope", out)
		}
		if got := tracker.CategorySpendUSD("yepapi"); got != 0 {
			t.Fatalf("yepapi spend = %v, want 0 for a request the provider refused", got)
		}
		if requests := atomic.LoadInt32(seen); requests != 1 {
			t.Fatalf("server saw %d requests, want exactly 1", requests)
		}
	})
}

// yepAPIStatusServer answers every request with the given status and reports
// how many requests it saw.
func yepAPIStatusServer(t *testing.T, status int) (string, *int32) {
	t.Helper()
	var seen int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"REFUSED","message":"refused"}}`))
	}))
	t.Cleanup(server.Close)
	return server.URL, &seen
}
