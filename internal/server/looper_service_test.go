package server

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestAppendStepResult(t *testing.T) {
	sysPrompt := "You are a helpful assistant."

	baseHistory := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: "Original task"},
	}

	tests := []struct {
		name           string
		history        []openai.ChatCompletionMessage
		stepName       string
		result         string
		ctxMode        string
		wantLastRole   string
		wantContains   string
		wantLen        int
	}{
		{
			name:         "every_step after plan",
			history:      baseHistory,
			stepName:     "plan",
			result:       "The plan is to improve the story.",
			ctxMode:      "every_step",
			wantLastRole: "user",
			wantContains: "improve the story",
			wantLen:      2, // system + new user message (reset)
		},
		{
			name:         "every_iteration after plan",
			history:      baseHistory,
			stepName:     "plan",
			result:       "The plan is to improve the story.",
			ctxMode:      "every_iteration",
			wantLastRole: "user",
			wantContains: "Plan result:",
			wantLen:      3, // original 2 + new message
		},
		{
			name:         "never after test keeps accumulating",
			history:      baseHistory,
			stepName:     "test",
			result:       "The story is now much better.",
			ctxMode:      "never",
			wantLastRole: "user",
			wantContains: "Test result:",
			wantLen:      3,
		},
		{
			name:           "every_step after action resets",
			history:        baseHistory,
			stepName:       "action",
			result:         "I rewrote chapter 3.",
			ctxMode:        "every_step",
			wantLastRole:   "user",
			wantLen:        2,
			wantContains:   "rewrote chapter 3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appendStepResult(tc.history, tc.stepName, tc.result, tc.ctxMode, sysPrompt)

			if len(got) != tc.wantLen {
				t.Errorf("expected len %d, got %d", tc.wantLen, len(got))
			}

			last := got[len(got)-1]
			if last.Role != tc.wantLastRole {
				t.Errorf("expected last role %s, got %s", tc.wantLastRole, last.Role)
			}
			if tc.wantContains != "" && !contains(last.Content, tc.wantContains) {
				t.Errorf("expected content to contain %q, got %q", tc.wantContains, last.Content)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}