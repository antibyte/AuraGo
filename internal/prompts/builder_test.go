package prompts

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestOptimizePrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Collapse Newlines",
			input:    "Line 1\n\n\n\nLine 2",
			expected: "Line 1\n\nLine 2",
		},
		{
			name:     "Trim Whitespace",
			input:    "  Line with spaces  \n  Another line  ",
			expected: "Line with spaces\nAnother line",
		},
		{
			name:     "Remove Comments",
			input:    "Start<!-- comment -->End",
			expected: "StartEnd",
		},
		{
			name:     "Simplify Separators",
			input:    "----------\n==========\n**********",
			expected: "---\n===\n***",
		},
		{
			name:     "Complex Mix",
			input:    "Header\n\n\n\n   Content   \n<!-- note -->\n\n   ---   \n   Footer   ",
			expected: "Header\n\nContent\n\n---\nFooter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := OptimizePrompt(tt.input)
			if got != tt.expected {
				t.Errorf("OptimizePrompt() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOptimizePromptTechnical(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Code Block Protection (Python)",
			input:    "Analyze this:\n```python\ndef hello():\n    print(\"world\")\n\n\n```\n",
			expected: "Analyze this:\n```python\ndef hello():\n    print(\"world\")\n\n\n```",
		},
		{
			name:     "Template Placeholder Safety",
			input:    "Hello {{ .User }}!\n",
			expected: "Hello {{ .User }}!",
		},
		{
			name:     "Saved Chars Count",
			input:    "Line 1\n\n\nLine 2",
			expected: "Line 1\n\nLine 2",
		},
		{
			name:     "Extreme Separator",
			input:    "--------------------------------------------------\n",
			expected: "---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, saved := OptimizePrompt(tt.input)
			if got != tt.expected {
				t.Errorf("OptimizePrompt() gotContent = %q, want %q", got, tt.expected)
			}
			if saved != len(tt.input)-len(tt.expected) {
				t.Errorf("OptimizePrompt() saved = %v, want %v", saved, len(tt.input)-len(tt.expected))
			}
		})
	}
}

func TestDetermineTier(t *testing.T) {
	tests := []struct {
		name     string
		msgCount int
		expected string
	}{
		{"Empty conversation", 0, "full"},
		{"Few messages", 3, "full"},
		{"Boundary full", 6, "full"},
		{"Medium conversation", 7, "compact"},
		{"Boundary compact", 12, "compact"},
		{"Long conversation", 13, "minimal"},
		{"Very long", 50, "minimal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineTier(tt.msgCount)
			if got != tt.expected {
				t.Errorf("DetermineTier(%d) = %q, want %q", tt.msgCount, got, tt.expected)
			}
		})
	}
}

func TestCountTokens(t *testing.T) {
	// Basic sanity check: a simple sentence should return a reasonable token count
	text := "Hello, world! This is a test sentence."
	tokens := CountTokens(text)
	if tokens <= 0 {
		t.Errorf("CountTokens() returned %d, expected > 0", tokens)
	}
	if tokens > len(text) {
		t.Errorf("CountTokens() returned %d, which exceeds character length %d", tokens, len(text))
	}
}

func TestCountTokensEmpty(t *testing.T) {
	tokens := CountTokens("")
	if tokens != 0 {
		t.Errorf("CountTokens(\"\") = %d, want 0", tokens)
	}
}

func TestRemoveSection(t *testing.T) {
	input := "# IDENTITY\nI am AuraGo.\n\n# RETRIEVED MEMORIES\nSome old memory.\nAnother line.\n\n# NOW\n2026-02-27"
	result := removeSection(input, "# RETRIEVED MEMORIES")
	if strings.Contains(result, "RETRIEVED MEMORIES") {
		t.Errorf("removeSection() did not remove the section: %s", result)
	}
	if !strings.Contains(result, "# IDENTITY") {
		t.Error("removeSection() removed the wrong section")
	}
	if !strings.Contains(result, "# NOW") {
		t.Error("removeSection() removed the trailing section")
	}
}

func TestRemoveSectionNotFound(t *testing.T) {
	input := "# IDENTITY\nI am AuraGo."
	result := removeSection(input, "# NONEXISTENT")
	if result != input {
		t.Errorf("removeSection() modified text when header not found")
	}
}

func TestBudgetShedNoAction(t *testing.T) {
	// Small prompt, big budget → no shedding
	prompt := "# IDENTITY\nI am AuraGo.\n\n# NOW\n2026-02-27"
	flags := ContextFlags{TokenBudget: 5000}
	result, shed := budgetShed(prompt, flags, "", "", time.Now(), testLogger)
	if result != prompt {
		t.Errorf("budgetShed() modified prompt when under budget")
	}
	if len(shed) != 0 {
		t.Errorf("budgetShed() should not shed anything under budget, got %v", shed)
	}
}

func TestBudgetShedRemovesGuides(t *testing.T) {
	prompt := "# IDENTITY\nI am AuraGo.\n\n# TOOL GUIDES\nSome guide content here that is long enough to matter.\n\n# NOW\n2026-02-27"
	flags := ContextFlags{TokenBudget: 10} // Tiny budget forces shedding
	result, shed := budgetShed(prompt, flags, "", "", time.Now(), testLogger)
	if strings.Contains(result, "TOOL GUIDES") {
		t.Error("budgetShed() should have removed TOOL GUIDES section")
	}
	if !strings.Contains(result, "IDENTITY") {
		t.Error("budgetShed() should NOT have removed IDENTITY section")
	}
	if len(shed) == 0 {
		t.Error("budgetShed() should report shed sections")
	}
}
