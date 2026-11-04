package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/budget"

	"github.com/sashabaranov/go-openai"
)

func TestMinimalLoopBudgetError(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("wrapped: %w", budget.ErrCompletionBudgetBlocked), http.StatusTooManyRequests, "budget_blocked"},
		{budget.ErrCompletionBudgetUnavailable, http.StatusServiceUnavailable, "budget_unavailable"},
		{errors.New("provider failed"), 0, ""},
	} {
		w := httptest.NewRecorder()
		if got := writeMinimalLoopBudgetError(w, tc.err, "de"); got != (tc.status != 0) {
			t.Fatalf("handled=%v, err=%v", got, tc.err)
		}
		if tc.status == 0 {
			continue
		}
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != tc.status || body["error"] != tc.code || body["message"] == "" {
			t.Fatalf("status=%d body=%s err=%v", w.Code, w.Body.String(), err)
		}
		if reason := gameVisualFailureReason(tc.err, openai.FinishReasonStop); reason != tc.code {
			t.Fatalf("visual reason=%s, want %s", reason, tc.code)
		}
	}
}
