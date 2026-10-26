package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/decisions"
)

func TestDecisionsDockerAPIFailureIsConflict(t *testing.T) {
	w := httptest.NewRecorder()
	writeDecisionsFailure(w, decisions.NewError(decisions.CodeDockerAPITooOld, nil))
	if w.Code != http.StatusConflict || w.Body.String() != "{\"code\":\"docker_api_too_old\",\"error\":\"docker_api_too_old\"}" {
		t.Fatalf("unsupported Docker API response: %d %s", w.Code, w.Body.String())
	}
}

func TestDecisionsScoreTimeoutIsGatewayTimeout(t *testing.T) {
	w := httptest.NewRecorder()
	err := decisions.NewError(decisions.CodeRuntimeScoreTimeout, context.DeadlineExceeded)
	writeDecisionsFailure(w, err)
	if w.Code != http.StatusGatewayTimeout || decisionsFailureCode(err) != decisions.CodeRuntimeScoreTimeout {
		t.Fatalf("score timeout: HTTP %d", w.Code)
	}
}
