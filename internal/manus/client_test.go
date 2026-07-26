package manus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClientPreservesSafeAPIErrorDiagnostics(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"request_id":"req-denied","error":{"code":"permission_denied","message":"Invalid API key"}}`))
	}))
	defer server.Close()

	client, err := NewClient("test-secret", ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListSkills(context.Background(), "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ListSkills() error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Code != "permission_denied" || apiErr.RequestID != "req-denied" {
		t.Fatalf("APIError = %#v", apiErr)
	}
	if !strings.Contains(err.Error(), "request_id: req-denied") {
		t.Fatalf("error missing request id: %v", err)
	}
}
