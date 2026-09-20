package llm

import "testing"

func TestJSONCompletionOutputBudget(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		limits                 ModelLimits
		input, requested, want int
		wantErr                bool
	}{
		{"ordinary", ModelLimits{ContextWindow: 32768, MaxOutputTokens: 4096}, 1000, 1500, 1500, false},
		{"reasoning", ModelLimits{ContextWindow: 32768, MaxOutputTokens: 16384, Reasoning: true}, 1000, 1500, ReasoningOutputTokens, false},
		{"explicit_output_cap", ModelLimits{ContextWindow: 32768, MaxOutputTokens: 2048, Reasoning: true}, 1000, 1500, 2048, false},
		{"larger_requested_output", ModelLimits{ContextWindow: 32768, MaxOutputTokens: 16384, Reasoning: true}, 1000, 12000, 12000, false},
		{"context_exhausted", ModelLimits{ContextWindow: 8192, MaxOutputTokens: 8192, Reasoning: true}, 500, 1500, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := JSONCompletionOutputBudget(tc.limits, tc.requested, tc.input)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("budget = %d, %v; want %d, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
