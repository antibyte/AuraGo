package server

import (
	"strings"
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
		name         string
		history      []openai.ChatCompletionMessage
		stepName     string
		result       string
		ctxMode      string
		wantLastRole string
		wantContains string
		wantLen      int
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
			name:         "every_step after action resets",
			history:      baseHistory,
			stepName:     "action",
			result:       "I rewrote chapter 3.",
			ctxMode:      "every_step",
			wantLastRole: "user",
			wantLen:      2,
			wantContains: "rewrote chapter 3",
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

func TestBuildFinishHistoryIncludesActionAndDesktopHandoffRules(t *testing.T) {
	iterSeed := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "base system"},
		{Role: openai.ChatMessageRoleUser, Content: "Create a story"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Prepared context"},
	}

	got := buildFinishHistory(
		iterSeed,
		"last_action_test",
		"Final story saved to Documents/final-story.docx",
		"Rating: 10/10",
		"base system",
	)

	if len(got) != len(iterSeed)+1 {
		t.Fatalf("finish history len = %d, want %d", len(got), len(iterSeed)+1)
	}
	if !strings.Contains(got[0].Content, "open_in_app") || !strings.Contains(got[0].Content, "app_id \"writer\"") {
		t.Fatalf("finish system prompt missing desktop open rules: %q", got[0].Content)
	}
	finalContext := got[len(got)-1].Content
	for _, want := range []string{"Final action result:", "Documents/final-story.docx", "Final test result:", "Rating: 10/10"} {
		if !strings.Contains(finalContext, want) {
			t.Fatalf("finish context missing %q: %q", want, finalContext)
		}
	}
	if iterSeed[0].Content != "base system" {
		t.Fatalf("buildFinishHistory mutated iterSeed system prompt: %q", iterSeed[0].Content)
	}
}

func TestBuildFinishHistoryDefaultUsesLastTestOnly(t *testing.T) {
	got := buildFinishHistory(
		nil,
		"",
		"Action output should not be included by default",
		"Final review score: 9",
		"base system",
	)

	if len(got) != 2 {
		t.Fatalf("finish history len = %d, want 2", len(got))
	}
	if got[0].Role != openai.ChatMessageRoleSystem {
		t.Fatalf("first message role = %q, want system", got[0].Role)
	}
	finalContext := got[len(got)-1].Content
	if !strings.Contains(finalContext, "Final test result of the loop:") {
		t.Fatalf("missing default test context: %q", finalContext)
	}
	if strings.Contains(finalContext, "Action output should not be included") {
		t.Fatalf("default finish context unexpectedly included action output: %q", finalContext)
	}
}

func TestBuildActionFinishResultIncludesActionToolOutput(t *testing.T) {
	history := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Plan prompt"},
		{Role: openai.ChatMessageRoleTool, Name: "virtual_desktop", Content: `{"path":"Documents/old.docx"}`},
		{Role: openai.ChatMessageRoleUser, Content: "Action prompt"},
		{Role: openai.ChatMessageRoleTool, Name: "virtual_desktop", Content: `{"status":"ok","data":{"path":"Documents/final-story.docx"}}`},
	}

	got := buildActionFinishResult("Saved the final story.", history, "Action prompt")
	if !strings.Contains(got, "Saved the final story.") || !strings.Contains(got, "Documents/final-story.docx") {
		t.Fatalf("action finish result missing response or tool output: %q", got)
	}
	if strings.Contains(got, "Documents/old.docx") {
		t.Fatalf("action finish result included tool output before the action prompt: %q", got)
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

func TestNormalizeTestFingerprintCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	a := normalizeTestFingerprint("  Score:  8/10  \n nice ")
	b := normalizeTestFingerprint("score: 8/10 nice")
	if a != b {
		t.Fatalf("fingerprints differ: %q vs %q", a, b)
	}
}

func TestAllFingerprintsEqual(t *testing.T) {
	t.Parallel()
	if !allFingerprintsEqual([]string{"a", "a"}) {
		t.Fatal("expected equal")
	}
	if allFingerprintsEqual([]string{"a", "b"}) {
		t.Fatal("expected not equal")
	}
	if allFingerprintsEqual([]string{""}) {
		t.Fatal("empty fingerprint must not count as stuck")
	}
}

func TestEstimateLooperCostUSDPositive(t *testing.T) {
	t.Parallel()
	cost := estimateLooperCostUSD(1_000_000, 1_000_000)
	if cost <= 0 {
		t.Fatalf("cost = %v, want > 0", cost)
	}
}

func TestParseStructuredExitConfidenceFields(t *testing.T) {
	t.Parallel()
	decision, reason, conf, ok := parseStructuredExit(`{"decision": true, "reason": "good", "confidence": 0.9}`)
	if !ok || !decision || reason != "good" || conf != 0.9 {
		t.Fatalf("parsed = %v %q %v ok=%v", decision, reason, conf, ok)
	}
}
