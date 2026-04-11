package tools

import (
	"aurago/internal/budget"
	"encoding/json"
)

// BudgetStatusResult represents the JSON response for budget_status operations.
type BudgetStatusResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteBudgetStatus returns the current token budget and usage statistics.
func ExecuteBudgetStatus(tracker *budget.Tracker) string {
	encode := func(r BudgetStatusResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if tracker == nil {
		return encode(BudgetStatusResult{Status: "error", Message: "Budget tracking is disabled in configuration"})
	}

	status := tracker.GetStatus()
	return encode(BudgetStatusResult{Status: "success", Data: status})
}
