package manus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientAvailableCreditsAuthenticatesAndDecodesEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v2/usage.availableCredits" {
			t.Fatalf("path = %q, want /v2/usage.availableCredits", r.URL.Path)
		}
		if got := r.Header.Get("x-manus-api-key"); got != "test-secret" {
			t.Fatalf("x-manus-api-key = %q, want test-secret", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"request_id":"req-1","data":{"total_credits":42,"refresh_interval":"daily"}}`))
	}))
	defer server.Close()

	client, err := NewClient("test-secret", ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	credits, err := client.AvailableCredits(context.Background())
	if err != nil {
		t.Fatalf("AvailableCredits() error = %v", err)
	}
	if credits.RequestID != "req-1" || credits.Data.TotalCredits != 42 || credits.Data.RefreshInterval != "daily" {
		t.Fatalf("AvailableCredits() = %#v", credits)
	}
}
