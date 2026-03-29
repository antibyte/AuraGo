package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/webhooks"
)

func TestHandleUpdateWebhookNotFoundReturnsGenericError(t *testing.T) {
	t.Parallel()

	mgr, err := webhooks.NewManager(t.TempDir()+"\\webhooks.json", t.TempDir()+"\\webhooks.log")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/webhooks/missing-id", strings.NewReader(`{"name":"updated"}`))
	rec := httptest.NewRecorder()

	handleUpdateWebhook(mgr).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "Webhook not found") || strings.Contains(strings.ToLower(body), "missing-id") {
		t.Fatalf("expected generic not-found response, got %q", body)
	}
}
