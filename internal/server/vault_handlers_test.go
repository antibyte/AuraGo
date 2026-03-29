package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSetVaultSecretInvalidJSONIsGeneric(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":`))
	rec := httptest.NewRecorder()

	handleSetVaultSecret(&Server{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "Invalid JSON") || strings.Contains(strings.ToLower(body), "unexpected eof") {
		t.Fatalf("expected generic invalid JSON response, got %q", body)
	}
}
