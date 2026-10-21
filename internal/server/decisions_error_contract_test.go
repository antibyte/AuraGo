package server

import (
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
