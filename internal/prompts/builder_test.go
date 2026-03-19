package prompts

import (
	"log/slog"
	"os"
	"path/filepath"
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

func TestDetermineTierAdaptive(t *testing.T) {
	tests := []struct {
		name     string
		flags    ContextFlags
		expected string
	}{
		{
			name:     "Empty conversation",
			flags:    ContextFlags{MessageCount: 0},
			expected: "full",
		},
		{
			name:     "Short simple chat stays full",
			flags:    ContextFlags{MessageCount: 5},
			expected: "full",
		},
		{
			name:     "Medium simple chat goes compact",
			flags:    ContextFlags{MessageCount: 10},
			expected: "compact",
		},
		{
			name:     "Long simple chat goes minimal",
			flags:    ContextFlags{MessageCount: 25},
			expected: "minimal",
		},
		{
			name: "Tool-heavy session at msg 10 stays full",
			flags: ContextFlags{
				MessageCount:      10,
				RecentlyUsedTools: []string{"shell", "filesystem", "docker"},
				PredictedGuides:   []string{"guide1"},
			},
			expected: "full",
		},
		{
			name: "Error state at msg 10 stays full",
			flags: ContextFlags{
				MessageCount: 10,
				IsErrorState: true,
			},
			expected: "full",
		},
		{
			name: "Coding + tools at msg 15 stays full",
			flags: ContextFlags{
				MessageCount:      15,
				RequiresCoding:    true,
				RecentlyUsedTools: []string{"shell", "filesystem", "docker"},
			},
			expected: "full",
		},
		{
			name: "All complexity factors at msg 20 stays compact",
			flags: ContextFlags{
				MessageCount:      20,
				IsErrorState:      true,
				RequiresCoding:    true,
				RecentlyUsedTools: []string{"a", "b", "c"},
				PredictedGuides:   []string{"guide"},
			},
			expected: "full",
		},
		{
			name: "Very long with complexity stays full",
			flags: ContextFlags{
				MessageCount:      50,
				IsErrorState:      true,
				RequiresCoding:    true,
				RecentlyUsedTools: []string{"a", "b", "c"},
			},
			expected: "full",
		},
		{
			name: "Very long with only coding goes compact",
			flags: ContextFlags{
				MessageCount:   50,
				RequiresCoding: true,
			},
			expected: "minimal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineTierAdaptive(tt.flags)
			if got != tt.expected {
				t.Errorf("DetermineTierAdaptive(%v) = %q, want %q", tt.name, got, tt.expected)
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

func TestReadToolGuideEmbed(t *testing.T) {
	// "docker.md" exists in the embedded prompts/tools_manuals/ directory.
	data, ok := readToolGuideEmbed("/any/path/tools_manuals/docker.md")
	if !ok || len(data) == 0 {
		t.Fatal("readToolGuideEmbed should find docker.md in embedded FS")
	}
	if !strings.Contains(string(data), "docker") && !strings.Contains(string(data), "Docker") {
		t.Error("embedded docker.md should mention docker")
	}
}

func TestReadToolGuideEmbedNotFound(t *testing.T) {
	_, ok := readToolGuideEmbed("/any/path/tools_manuals/nonexistent_tool_xyz.md")
	if ok {
		t.Error("readToolGuideEmbed should return false for non-existent guides")
	}
}

func TestReadToolGuideEmbedNoMarker(t *testing.T) {
	// Path without "tools_manuals/" should fail gracefully.
	_, ok := readToolGuideEmbed("/some/random/path/docker.md")
	if ok {
		t.Error("readToolGuideEmbed should return false when path has no tools_manuals/ segment")
	}
}

func TestReadToolGuideFallbackToEmbed(t *testing.T) {
	// Clear cache to ensure a fresh lookup.
	guideCacheMu.Lock()
	delete(guideCache, "/nonexistent/dir/tools_manuals/docker.md")
	guideCacheMu.Unlock()

	// The disk path does not exist, so readToolGuide should fall back to embed.
	content, ok := readToolGuide("/nonexistent/dir/tools_manuals/docker.md")
	if !ok {
		t.Fatal("readToolGuide should fall back to embedded FS when disk file is missing")
	}
	if len(content) == 0 {
		t.Error("readToolGuide fallback should return non-empty content")
	}
}

func TestTruncateGuide(t *testing.T) {
	short := "hello world"
	if truncateGuide(short, 100) != short {
		t.Error("truncateGuide should not modify short content")
	}
	long := strings.Repeat("x", 200)
	result := truncateGuide(long, 100)
	if len(result) <= 100 {
		t.Error("truncateGuide result should contain the truncated marker")
	}
	if !strings.HasSuffix(result, "[...truncated]") {
		t.Error("truncateGuide should append truncation marker")
	}
}

// ── ShouldInclude ─────────────────────────────────────────────────────────────────────────────
func TestShouldInclude_MandatoryAlwaysIncluded(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{Tags: []string{"mandatory"}}}
	if !mod.ShouldInclude(ContextFlags{}) {
		t.Error("mandatory tag should always include regardless of flags")
	}
}

func TestShouldInclude_CoreTagNoConditions(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{Tags: []string{"core"}}}
	if !mod.ShouldInclude(ContextFlags{}) {
		t.Error("core tag with no conditions should include")
	}
}

func TestShouldInclude_UnknownTagExcluded(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{Tags: []string{"optional"}}}
	if mod.ShouldInclude(ContextFlags{}) {
		t.Error("unknown tag with no conditions should exclude")
	}
}

func TestShouldInclude_ConditionIsError(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{
		Tags: []string{"ctx"}, Conditions: []string{"is_error"},
	}}
	if mod.ShouldInclude(ContextFlags{IsErrorState: false}) {
		t.Error("is_error condition should exclude when IsErrorState=false")
	}
	if !mod.ShouldInclude(ContextFlags{IsErrorState: true}) {
		t.Error("is_error condition should include when IsErrorState=true")
	}
}

func TestShouldInclude_ConditionRequiresCoding(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{
		Tags: []string{"ctx"}, Conditions: []string{"requires_coding"},
	}}
	if mod.ShouldInclude(ContextFlags{RequiresCoding: false}) {
		t.Error("requires_coding condition should exclude when RequiresCoding=false")
	}
	if !mod.ShouldInclude(ContextFlags{RequiresCoding: true}) {
		t.Error("requires_coding condition should include when RequiresCoding=true")
	}
}

func TestShouldInclude_MultipleConditionsOneMatches(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{
		Tags: []string{"ctx"}, Conditions: []string{"is_error", "requires_coding"},
	}}
	if !mod.ShouldInclude(ContextFlags{RequiresCoding: true}) {
		t.Error("should include when at least one of multiple conditions matches")
	}
}

func TestShouldInclude_MandatoryOverridesConditions(t *testing.T) {
	mod := PromptModule{Metadata: PromptMetadata{
		Tags: []string{"mandatory"}, Conditions: []string{"is_error"},
	}}
	if !mod.ShouldInclude(ContextFlags{}) {
		t.Error("mandatory tag should override condition check and always include")
	}
}

// ── parsePromptModule ──────────────────────────────────────────────────────────────────────
func TestParsePromptModuleWithBOM(t *testing.T) {
	// UTF-8 BOM (\xEF\xBB\xBF) is prepended by some Windows text editors.
	withBOM := "\xEF\xBB\xBF---\nid: bom_test\ntags:\n  - core\n---\nContent after BOM"
	mod, err := parsePromptModule(withBOM)
	if err != nil {
		t.Fatalf("parsePromptModule with BOM failed: %v", err)
	}
	if mod.Metadata.ID != "bom_test" {
		t.Errorf("expected id=bom_test, got %q", mod.Metadata.ID)
	}
	if mod.Content != "Content after BOM" {
		t.Errorf("unexpected content: %q", mod.Content)
	}
}

func TestParsePromptModuleLeadingNewlines(t *testing.T) {
	withNewlines := "\n\n---\nid: newline_test\ntags:\n  - core\n---\nBody text"
	mod, err := parsePromptModule(withNewlines)
	if err != nil {
		t.Fatalf("parsePromptModule with leading newlines failed: %v", err)
	}
	if mod.Metadata.ID != "newline_test" {
		t.Errorf("expected id=newline_test, got %q", mod.Metadata.ID)
	}
}

// ── PrepareDynamicGuides path traversal ────────────────────────────────────────────
func TestPrepareDynamicGuidesPathTraversal(t *testing.T) {
	parent := t.TempDir()
	toolsDir := filepath.Join(parent, "tools")
	secretDir := filepath.Join(parent, "secret")
	for _, d := range []string{toolsDir, secretDir} {
		if err := os.Mkdir(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Write sensitive content outside toolsDir — should never be read.
	const secretContent = "TOP_SECRET_KEY=abc123"
	if err := os.WriteFile(filepath.Join(secretDir, "secret.md"), []byte(secretContent), 0644); err != nil {
		t.Fatal(err)
	}

	// "../secret/secret" traverses from toolsDir into secretDir.
	traversalTool := "../secret/secret"
	guides := PrepareDynamicGuides(
		nil, nil, "test query", "", toolsDir,
		[]string{traversalTool}, // recentTools
		[]string{traversalTool}, // explicitTools
		10, testLogger,
	)
	for _, g := range guides {
		if strings.Contains(g, secretContent) {
			t.Fatal("path traversal attack succeeded — secret content leaked across directory boundary")
		}
	}
}
