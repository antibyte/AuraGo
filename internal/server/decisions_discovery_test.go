package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/decisions"
)

func TestDecisionsDiscoveryTimeoutKeepsItsCode(t *testing.T) {
	err := decisions.NewError(decisions.CodeRuntimeDiscoveryTimeout, context.DeadlineExceeded)
	w := httptest.NewRecorder()
	writeDecisionsFailure(w, err)
	if w.Code != http.StatusGatewayTimeout || !strings.Contains(w.Body.String(), decisions.CodeRuntimeDiscoveryTimeout) || !decisions.IsNonRetryable(err) {
		t.Fatalf("timeout response = %d %s", w.Code, w.Body.String())
	}
}
