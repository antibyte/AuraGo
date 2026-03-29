package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONLoggedErrorReturnsClientMessageOnly(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	jsonLoggedError(rec, nil, http.StatusInternalServerError, "Public message", "ignored", errors.New("secret backend detail"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Public message") {
		t.Fatalf("expected public message in body, got %s", body)
	}
	if strings.Contains(body, "secret backend detail") {
		t.Fatalf("response leaked internal error detail: %s", body)
	}
}
