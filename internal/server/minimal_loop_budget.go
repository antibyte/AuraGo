package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"aurago/internal/budget"
	"aurago/internal/i18n"
)

func minimalLoopBudgetCode(err error) string {
	switch {
	case errors.Is(err, budget.ErrCompletionBudgetBlocked):
		return "budget_blocked"
	case errors.Is(err, budget.ErrCompletionBudgetUnavailable):
		return "budget_unavailable"
	default:
		return ""
	}
}

func writeMinimalLoopBudgetError(w http.ResponseWriter, err error, lang string) bool {
	code := minimalLoopBudgetCode(err)
	if code == "" {
		return false
	}
	status := http.StatusTooManyRequests
	if code == "budget_unavailable" {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": i18n.T(lang, "backend.stream_budget_blocked")})
	return true
}
