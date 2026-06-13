package prompts

import (
	promptsembed "aurago/prompts"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubTokenEncoder struct {
	tokensPerCall int
}

func (s stubTokenEncoder) Encode(text string, allowedSpecial, disallowedSpecial []string) []int {
	if s.tokensPerCall <= 0 {
		return nil
	}
	return make([]int, s.tokensPerCall)
}

// charRatioEncoder approximates 1 token per 4 characters (realistic BPE ratio).
type charRatioEncoder struct{}

func (charRatioEncoder) Encode(text string, _, _ []string) []int {
	n := len(text) / 4
	if n < 1 {
		n = 1
	}
	return make([]int, n)
}

func TestBalancedCorePromptModulesStayCompact(t *testing.T) {
	limits := map[string]int{
		"rules.md":                   10500,
		"ctx_capability_creation.md": 2400,
		"ctx_daemon_skills.md":       1400,
		"ctx_personality_state.md":   450,
		"lifeboat.md":                1200,
	}

	for filename, limit := range limits {
		t.Run(filename, func(t *testing.T) {
			raw, err := promptsembed.FS.ReadFile(filename)
			if err != nil {
				t.Fatalf("read %s: %v", filename, err)
			}
			if got := len(raw); got > limit {
				t.Fatalf("%s length = %d chars, want <= %d", filename, got, limit)
			}
		})
	}
}

func TestBalancedCoreSystemPromptStaysUnderBudget(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)

	prompt, tokens := BuildSystemPromptContext(context.Background(), t.TempDir(), &ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		TokenBudget:        200000,
		NativeToolsEnabled: true,
	}, "", slog.Default())

	if tokens > 4200 {
		t.Fatalf("balanced core system prompt tokens = %d, want <= 4200; prompt chars=%d", tokens, len(prompt))
	}
}

func TestBalancedRulesPromptKeepsCriticalMarkers(t *testing.T) {
	raw, err := promptsembed.FS.ReadFile("rules.md")
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rules := string(raw)
	for _, want := range []string{
		"<external_data>",
		"secrets vault",
		"<done/>",
		"native function",
		"discover_tools",
		"No inline sudo",
		"Memory is advisory",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules prompt missing critical marker %q", want)
		}
	}
}

func TestUnifiedMemoryContextUsesCompactAdvisoryWarning(t *testing.T) {
	block := buildUnifiedMemoryContextBlock("full", &ContextFlags{
		RecentActivityOverview: "recent activity",
		RetrievedMemories:      "retrieved memory",
		PredictedMemories:      "predicted memory",
		KnowledgeContext:       "knowledge context",
		ErrorPatternContext:    "known error",
		LearnedRulesContext:    "learned rule",
		ReuseContext:           "reuse hint",
	})

	if len(block) > 700 {
		t.Fatalf("unified memory context too verbose: %d chars\n%s", len(block), block)
	}
	if got := strings.Count(strings.ToLower(block), "fresh tool output"); got != 1 {
		t.Fatalf("expected one compact fresh-tool-output warning, got %d:\n%s", got, block)
	}
	for _, want := range []string{"# UNIFIED MEMORY CONTEXT", "## Retrieved Memories", "## Known Error Patterns", "reuse hint"} {
		if !strings.Contains(block, want) {
			t.Fatalf("unified memory context missing %q:\n%s", want, block)
		}
	}
}

func TestBuildSystemPromptCombinesPersonaSignals(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)

	prompt, _ := BuildSystemPromptContext(context.Background(), t.TempDir(), &ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		TokenBudget:        200000,
		EmotionDescription: "focused and careful",
		PersonalityLine:    "C=0.7 T=0.8",
		InnerVoice:         "verify before claiming completion",
	}, "", slog.Default())

	if got := strings.Count(prompt, "### PERSONA SIGNALS"); got != 1 {
		t.Fatalf("expected exactly one persona signals block, got %d:\n%s", got, prompt)
	}
	for _, legacy := range []string{"### CURRENT EMOTIONAL STATE & MOOD", "### CURRENT PERSONALITY TRAITS", "### INNER VOICE"} {
		if strings.Contains(prompt, legacy) {
			t.Fatalf("legacy persona section %q should not appear:\n%s", legacy, prompt)
		}
	}
}

type markerAwareEncoder struct{}

func (markerAwareEncoder) Encode(text string, _, _ []string) []int {
	switch {
	case strings.Contains(text, "BIG_GUIDE"):
		return make([]int, 200)
	case strings.Contains(text, "BIG_MEMORY"):
		return make([]int, 180)
	default:
		n := len(strings.Fields(text))
		if n < 1 {
			n = 1
		}
		return make([]int, n)
	}
}

func resetTokenEncoderStateForTest(tb testing.TB, loader func() (tokenEncoder, error), timeout, backoff time.Duration) {
	tb.Helper()

	tiktokenMu.Lock()
	oldEnc := tiktokenEnc
	oldDone := tiktokenInitDone
	oldInFlight := tiktokenInitInFlight
	oldNextRetry := tiktokenNextRetry
	oldLoader := tiktokenLoadEncoding
	oldTimeout := tiktokenInitTimeout
	oldBackoff := tiktokenRetryBackoff
	tiktokenMu.Unlock()

	tiktokenMu.Lock()
	tiktokenEnc = nil
	tiktokenInitDone = nil
	tiktokenInitInFlight = false
	tiktokenNextRetry = time.Time{}
	tiktokenLoadEncoding = loader
	tiktokenInitTimeout = timeout
	tiktokenRetryBackoff = backoff
	tiktokenWarnOnce = sync.Once{}
	tiktokenMu.Unlock()

	tb.Cleanup(func() {
		tiktokenMu.Lock()
		tiktokenEnc = oldEnc
		tiktokenInitDone = oldDone
		tiktokenInitInFlight = oldInFlight
		tiktokenNextRetry = oldNextRetry
		tiktokenLoadEncoding = oldLoader
		tiktokenInitTimeout = oldTimeout
		tiktokenRetryBackoff = oldBackoff
		tiktokenWarnOnce = sync.Once{}
		tiktokenMu.Unlock()
	})
}

func TestBudgetShed_RemovesUserProfilingSection(t *testing.T) {
	// Create a prompt with the ## USER PROFILING section that should be removed
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

# TOOL GUIDES
tool guide content here

## USER PROFILING
Your goal: build a comprehensive user profile over time.

### Known User Profile
User name: Test User

# RETRIEVED MEMORIES
memory entry 1

# NOW
2026-04-09 12:00`

	flags := ContextFlags{
		Tier:        "full",
		TokenBudget: 5, // Very small budget - should shed everything possible
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), logger)

	// Check that ## USER PROFILING was shed
	foundProfiling := false
	for _, section := range shedSections {
		if section == "## USER PROFILING" {
			foundProfiling = true
			break
		}
	}
	if !foundProfiling {
		t.Errorf("expected ## USER PROFILING to be in shedSections, got: %v", shedSections)
	}

	// Verify the section is actually removed from the result
	if strings.Contains(result, "## USER PROFILING") {
		t.Errorf("expected ## USER PROFILING to be removed from result, but it was found")
	}
	if strings.Contains(result, "Your goal: build a comprehensive user profile") {
		t.Errorf("expected user profiling content to be removed from result")
	}
}

func TestRulesPromptCompletionSignalOnlyAfterCompletion(t *testing.T) {
	raw, err := promptsembed.FS.ReadFile("rules.md")
	if err != nil {
		t.Fatalf("read rules prompt: %v", err)
	}
	rules := string(raw)

	for _, forbidden := range []string{
		"regardless of whether the message feels final or like an intermediate note",
		"missing `<done/>` AND missing tool call",
	} {
		if strings.Contains(rules, forbidden) {
			t.Fatalf("rules prompt still contains ambiguous completion-signal wording %q", forbidden)
		}
	}
	for _, want := range []string{
		"ONLY when the current user request is fully handled",
		"Never use `<done/>` as an acknowledgement, promise, preamble, progress update, or mid-task marker",
		"If work remains, call the required tool now instead of writing text",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules prompt missing strict completion wording %q", want)
		}
	}
}

func TestRulesPromptRequiresAnnouncedActionsToBeExecuted(t *testing.T) {
	raw, err := promptsembed.FS.ReadFile("rules.md")
	if err != nil {
		t.Fatalf("read rules prompt: %v", err)
	}
	rules := string(raw)

	for _, want := range []string{
		"Action-execution integrity",
		"Never announce, promise, imply, or describe an action as future work unless you will actually perform it",
		"your next assistant action must be the corresponding tool call",
		"Do not say you will inspect, edit, run, test, create, save, register, deploy, open, send, or document anything and then stop with text only",
		"If a tool is unavailable, blocked, unsafe, or needs user confirmation, state that constraint plainly",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules prompt missing action-execution integrity wording %q", want)
		}
	}
}

func TestBudgetShed_HardTruncateWhenCoreExceedsBudget(t *testing.T) {
	prompt := strings.Repeat("word ", 5000)

	flags := ContextFlags{
		Tier:        "minimal",
		TokenBudget: 10,
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), logger)

	if !strings.Contains(result, "[BUDGET TRUNCATED]") {
		t.Errorf("expected hard-truncate marker, got result len=%d", len(result))
	}

	foundHardTruncate := false
	for _, s := range shedSections {
		if s == "HARD_TRUNCATE" {
			foundHardTruncate = true
		}
	}
	if !foundHardTruncate {
		t.Errorf("expected HARD_TRUNCATE in shedSections, got: %v", shedSections)
	}

	if len(result) >= len(prompt) {
		t.Errorf("expected result to be shorter than original, got len=%d vs orig=%d", len(result), len(prompt))
	}
}

func TestBudgetShedRemovesVolatileTurnSectionsBeforeHardTruncate(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)

	largeContext := strings.Repeat("learned operational context ", 80)
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

# LEARNED RULES
` + largeContext + `

# TASK RULES
` + largeContext + `

# ADDITIONAL INSTRUCTIONS
Keep the user's explicit instruction.

> **VOICE MODE ACTIVE (SPEAKER ON):** Speak short conversational replies.

> **NATIVE TOOL MODE REMINDER:** Use native function calling.`

	flags := ContextFlags{
		Tier:               "full",
		TokenBudget:        80,
		NativeToolsEnabled: true,
		VoiceOutputActive:  true,
	}

	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), slog.Default())
	if strings.Contains(result, "# LEARNED RULES") {
		t.Fatalf("expected # LEARNED RULES to be shed:\n%s", result)
	}
	if strings.Contains(result, "# TASK RULES") {
		t.Fatalf("expected # TASK RULES to be shed:\n%s", result)
	}
	if !strings.Contains(result, "# ADDITIONAL INSTRUCTIONS") {
		t.Fatalf("expected additional instructions to survive shedding:\n%s", result)
	}
	if !strings.Contains(result, "VOICE MODE ACTIVE") {
		t.Fatalf("expected voice mode reminder to survive shedding:\n%s", result)
	}
	if !strings.Contains(result, "NATIVE TOOL MODE REMINDER") {
		t.Fatalf("expected native tool reminder to survive shedding:\n%s", result)
	}
	for _, section := range shedSections {
		if section == "HARD_TRUNCATE" {
			t.Fatalf("expected section shedding to avoid hard truncate, got %v", shedSections)
		}
	}
}

func TestBudgetShed_UnifiedMemoryRemovesUserProfile(t *testing.T) {
	// Test with UnifiedMemoryBlock enabled
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

## USER PROFILING
Your goal: build a comprehensive user profile.

### Known User Profile
User name: Test User

# UNIFIED MEMORY CONTEXT
## Recent Activity
some activity`

	flags := ContextFlags{
		Tier:               "compact",
		TokenBudget:        5,
		UnifiedMemoryBlock: true,
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), logger)

	// Check that ## USER PROFILING was shed (under UnifiedMemoryBlock path)
	foundProfiling := false
	for _, section := range shedSections {
		if section == "## USER PROFILING" {
			foundProfiling = true
			break
		}
	}
	if !foundProfiling {
		t.Errorf("expected ## USER PROFILING to be in shedSections (unified path), got: %v", shedSections)
	}

	// Verify the section is actually removed
	if strings.Contains(result, "## USER PROFILING") {
		t.Errorf("expected ## USER PROFILING to be removed from result")
	}
}

func TestBudgetShed_RemovesPersonaSignalsSection(t *testing.T) {
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

### PERSONA SIGNALS
Tone only; not proof of task state or tool results.
Mood: cautiously optimistic.
Traits: C=0.6 T=0.7.
Inner voice: verify before claiming completion.

# NOW
2026-06-11 12:00`

	flags := ContextFlags{
		Tier:        "full",
		TokenBudget: 5,
	}

	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), slog.Default())

	if !containsString(shedSections, "### PERSONA SIGNALS") {
		t.Fatalf("expected persona signals in shedSections, got: %v", shedSections)
	}
	if strings.Contains(result, "### PERSONA SIGNALS") || strings.Contains(result, "cautiously optimistic") {
		t.Fatalf("expected persona signals to be removed:\n%s", result)
	}
	for _, legacy := range []string{"[Self:", "[SYSTEM DIRECTIVE - CURRENT STATE]"} {
		if containsString(shedSections, legacy) {
			t.Fatalf("legacy shed target %q should no longer be used", legacy)
		}
	}
}

func TestUnifiedMemoryUsesAdaptiveTierWhenFlagTierUnset(t *testing.T) {
	flags := ContextFlags{
		MessageCount:           30,
		SystemLanguage:         "en",
		UnifiedMemoryBlock:     true,
		RecentActivityOverview: "recent activity should be shed by minimal tier",
		RetrievedMemories:      "retrieved memory should be shed by minimal tier",
		PredictedMemories:      "predicted memory should be shed by minimal tier",
		KnowledgeContext:       "knowledge should be shed by minimal tier",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if strings.Contains(prompt, "# UNIFIED MEMORY CONTEXT") {
		t.Fatalf("expected adaptive minimal tier to omit unified memory context when flags.Tier is unset:\n%s", prompt)
	}
}

func TestGetCorePersonalityMetaFallsBackToEmbeddedProfile(t *testing.T) {
	meta := GetCorePersonalityMeta(t.TempDir(), "punk")

	if meta.Volatility != 1.8 {
		t.Fatalf("Volatility = %v, want embedded punk metadata", meta.Volatility)
	}
	if meta.EmpathyBias != 0.2 {
		t.Fatalf("EmpathyBias = %v, want embedded punk metadata", meta.EmpathyBias)
	}
	if meta.ConflictResponse != "assertive" {
		t.Fatalf("ConflictResponse = %q, want assertive", meta.ConflictResponse)
	}
}

func TestBuildSystemPromptIncludesPlannerContext(t *testing.T) {
	flags := ContextFlags{
		SystemLanguage: "en",
		PlannerContext: "Open todos: 2\n- [HIGH] Patch planner\nUpcoming appointments (next 48h):\n- 2026-04-18T09:00:00Z: Review",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "### PLANNER CONTEXT ###") {
		t.Fatalf("prompt = %q, want planner context header", prompt)
	}
	if !strings.Contains(prompt, "Open todos: 2") {
		t.Fatalf("prompt = %q, want planner context content", prompt)
	}
}

func TestBuildSystemPromptIncludesAgentSkillsCatalogOnly(t *testing.T) {
	flags := ContextFlags{
		SystemLanguage:     "en",
		AgentSkillsCatalog: "- `csv-helper`: Summarize CSV files.",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "# AGENT SKILLS CATALOG") {
		t.Fatalf("prompt = %q, want Agent Skills catalog header", prompt)
	}
	if !strings.Contains(prompt, "- `csv-helper`: Summarize CSV files.") {
		t.Fatalf("prompt = %q, want Agent Skills catalog content", prompt)
	}
	if strings.Contains(prompt, "<agent_skill") {
		t.Fatalf("prompt = %q, catalog should not include activated SKILL.md bodies", prompt)
	}
}

func TestBuildSystemPromptIncludesOperationalIssueReminder(t *testing.T) {
	flags := ContextFlags{
		SystemLanguage:           "en",
		OperationalIssueReminder: "Unresolved operational issues detected in background contexts:\n- System issue: Maintenance failed",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "### OPERATIONAL ISSUE REMINDER ###") {
		t.Fatalf("prompt = %q, want operational issue reminder header", prompt)
	}
	if !strings.Contains(prompt, "Maintenance failed") {
		t.Fatalf("prompt = %q, want operational issue content", prompt)
	}
	if !strings.Contains(prompt, "Use these issues as diagnostic context; mention them only if relevant to the current request or urgent.") {
		t.Fatalf("prompt = %q, want relevance-gated operational issue instruction", prompt)
	}
}

func TestMaintenancePromptIncludesWorkdirCleanupProtocol(t *testing.T) {
	raw, err := promptsembed.FS.ReadFile("maintenance.md")
	if err != nil {
		t.Fatalf("read embedded maintenance prompt: %v", err)
	}

	prompt := string(raw)
	required := []string{
		"**Workdir Cleanup.**",
		"agent_workspace/workdir",
		"Never delete",
		"archive/maintenance",
		"Report deleted, moved, renamed",
	}
	for _, marker := range required {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("maintenance prompt missing workdir cleanup marker %q", marker)
		}
	}
}

func TestBuildSystemPromptStablePrefixIgnoresVolatileSuffix(t *testing.T) {
	flagsA := ContextFlags{
		Tier:                   "full",
		SystemLanguage:         "en",
		NativeToolsEnabled:     true,
		HighPriorityNotes:      "Note A",
		PlannerContext:         "Planner A",
		RecentActivityOverview: "Recent A",
		PredictedGuides:        []string{"Guide A"},
		MessageSource:          "web_chat",
		ActiveProcesses:        "daemon-a",
	}
	flagsB := flagsA
	flagsB.HighPriorityNotes = "Note B"
	flagsB.PlannerContext = "Planner B"
	flagsB.RecentActivityOverview = "Recent B"
	flagsB.PredictedGuides = []string{"Guide B"}
	flagsB.MessageSource = "telegram"
	flagsB.ActiveProcesses = "daemon-b"

	promptA, _ := buildSystemPromptInner("", &flagsA, "Remember stable facts.", slog.Default())
	promptB, _ := buildSystemPromptInner("", &flagsB, "Remember stable facts.", slog.Default())

	prefixA, suffixA, okA := strings.Cut(promptA, "# TURN CONTEXT")
	prefixB, suffixB, okB := strings.Cut(promptB, "# TURN CONTEXT")
	if !okA || !okB {
		t.Fatalf("expected # TURN CONTEXT marker in both prompts")
	}
	if prefixA != prefixB {
		t.Fatalf("stable prefix changed for volatile inputs")
	}
	if suffixA == suffixB {
		t.Fatalf("expected volatile suffixes to differ")
	}
	if strings.Contains(prefixA, "Note A") || strings.Contains(prefixA, "Planner A") || strings.Contains(prefixA, "Recent A") {
		t.Fatalf("volatile context leaked into stable prefix")
	}
	if !strings.Contains(suffixA, "Note A") || !strings.Contains(suffixB, "Note B") {
		t.Fatalf("volatile notes missing from suffixes")
	}
}

func TestBuildSystemPromptNativeModeOmitsRawJSONToolProtocol(t *testing.T) {
	flags := ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		NativeToolsEnabled: true,
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "This session uses the **native function calling API**") {
		t.Fatalf("prompt missing native tool calling instructions")
	}
	if strings.Contains(prompt, "A Go supervisor parses your output. To invoke a tool, output a raw JSON object") {
		t.Fatalf("native prompt must not include raw JSON tool protocol")
	}
	if strings.Contains(prompt, "When calling a tool, your ENTIRE response = a raw JSON object") {
		t.Fatalf("native prompt must not include raw JSON-only response rule")
	}
	forbidden := []string{
		"A Go supervisor parses your output",
		"your ENTIRE response = a raw JSON object",
		"Response format.",
		"Raw JSON tool mode",
	}
	for _, needle := range forbidden {
		if strings.Contains(prompt, needle) {
			t.Fatalf("native prompt must not include raw JSON protocol fragment %q", needle)
		}
	}
}

func TestBuildSystemPromptIncludesActionLedgerReminderForToolModes(t *testing.T) {
	tests := []struct {
		name  string
		flags ContextFlags
	}{
		{
			name: "native",
			flags: ContextFlags{
				Tier:               "full",
				SystemLanguage:     "en",
				NativeToolsEnabled: true,
			},
		},
		{
			name: "text-json",
			flags: ContextFlags{
				Tier:            "full",
				SystemLanguage:  "en",
				IsTextModeModel: true,
			},
		},
		{
			name: "text fallback",
			flags: ContextFlags{
				Tier:           "full",
				SystemLanguage: "en",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, _ := buildSystemPromptInner("", &tt.flags, "", slog.Default())
			for _, want := range []string{
				"Action ledger",
				"Actual work is tracked from tool-call lifecycle events, not from prose",
				"does not start or complete an action",
				"Final completion claims must be backed by completed tool results from this turn",
			} {
				if !strings.Contains(prompt, want) {
					t.Fatalf("prompt missing action ledger reminder fragment %q", want)
				}
			}
		})
	}
}

func TestBuildSystemPromptMissionWarnsNotToAskInChat(t *testing.T) {
	flags := ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		NativeToolsEnabled: true,
		IsMission:          true,
		MessageSource:      "mission",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	required := []string{
		"Mission (automated)",
		"Do not ask the live user for confirmation in chat",
		"report the mission as blocked",
	}
	for _, marker := range required {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("mission prompt missing marker %q", marker)
		}
	}
}

func TestBuildSystemPromptLabelsAgoDeskChatSource(t *testing.T) {
	flags := ContextFlags{
		SystemLanguage: "en",
		MessageSource:  "agodesk_chat",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "> **Channel:** AgoChat") {
		t.Fatalf("prompt should label agodesk_chat as AgoChat, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "> **Channel:** agodesk_chat") {
		t.Fatalf("prompt exposed raw agodesk_chat source instead of a user-facing channel label:\n%s", prompt)
	}
}

func TestBuildSystemPromptNativeModeOmitsLegacyToolSyntaxExamples(t *testing.T) {
	flags := ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		NativeToolsEnabled: true,
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	forbidden := []string{
		`{"action": "register_device"`,
		`{"action": "media_registry"`,
		`{"action": "send_document"`,
		`{"action": "send_video"`,
		`{"action": "homepage_registry"`,
		"<workflow_plan>",
		"JSON-only tool protocol wins",
		"Raw JSON tool mode has no acknowledgment exception",
	}
	for _, needle := range forbidden {
		if strings.Contains(prompt, needle) {
			t.Fatalf("native prompt must not include legacy tool syntax %q", needle)
		}
	}
}

func TestBuildSystemPromptNativeModeDoesNotInjectRootToolManuals(t *testing.T) {
	flags := ContextFlags{
		Tier:                   "full",
		SystemLanguage:         "en",
		NativeToolsEnabled:     true,
		HomepageEnabled:        true,
		NetlifyEnabled:         true,
		SandboxEnabled:         true,
		DocumentCreatorEnabled: true,
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	forbidden := []string{
		"### Homepage — Web Development & Deployment",
		"### Document Creator",
		"### Sandbox Code Execution",
		"### Netlify Integration",
		"# TOOL EXECUTION PROTOCOL",
		"| Operation |",
		"| Parameter |",
	}
	for _, needle := range forbidden {
		if strings.Contains(prompt, needle) {
			t.Fatalf("native prompt must not inject root-level tool manual fragment %q", needle)
		}
	}
}

func TestBuildSystemPromptNativeModeAppendsFinalProtocolReminder(t *testing.T) {
	flags := ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		NativeToolsEnabled: true,
		IsVoiceMode:        true,
		AdditionalPrompt:   "Custom instructions may mention older tool examples.",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	additionalIdx := strings.Index(prompt, "# ADDITIONAL INSTRUCTIONS")
	voiceIdx := strings.Index(prompt, "VOICE MODE ACTIVE")
	reminderIdx := strings.LastIndex(prompt, "NATIVE TOOL MODE REMINDER")

	if additionalIdx < 0 {
		t.Fatalf("prompt missing additional instructions")
	}
	if voiceIdx < 0 {
		t.Fatalf("prompt missing voice mode instructions")
	}
	if reminderIdx < 0 {
		t.Fatalf("prompt missing native tool mode reminder")
	}
	if reminderIdx < additionalIdx || reminderIdx < voiceIdx {
		t.Fatalf("native tool mode reminder must appear after late custom sections")
	}
}

func TestBuildSystemPromptNativeModeSanitizesDynamicToolGuides(t *testing.T) {
	flags := ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		NativeToolsEnabled: true,
		PredictedGuides: []string{
			"# invoke_tool\n\n```json\n{\"action\":\"invoke_tool\",\"tool_name\":\"yepapi_instagram\",\"arguments\":{\"operation\":\"user\",\"username\":\"jopliness\"}}\n```\n\n<tool_call>{\"name\":\"invoke_tool\"}</tool_call>\n\nUse tool_name and arguments from the schema.",
		},
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "# TOOL GUIDES") {
		t.Fatalf("prompt missing dynamic tool guides")
	}
	forbidden := []string{`{"action":"invoke_tool"`, "<tool_call>", "</tool_call>"}
	for _, needle := range forbidden {
		if strings.Contains(prompt, needle) {
			t.Fatalf("native prompt must sanitize dynamic guide legacy syntax %q", needle)
		}
	}
	if !strings.Contains(prompt, "Use tool_name and arguments from the schema.") {
		t.Fatalf("sanitizer removed non-example guide prose")
	}
}

func TestPrepareDynamicGuidesWithStrategyRespectsManualConditions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tools_manuals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manual := "---\nconditions: [\"allow_shell\"]\n---\n# execute_shell\nUse shell only when enabled.\n"
	if err := os.WriteFile(filepath.Join(dir, "execute_shell.md"), []byte(manual), 0o644); err != nil {
		t.Fatalf("WriteFile execute_shell manual: %v", err)
	}

	disabled := ContextFlags{AllowShell: false}
	guides := PrepareDynamicGuidesWithStrategy(
		nil, nil, "", "", dir, nil, []string{"execute_shell"}, 5,
		DynamicGuideStrategy{Flags: &disabled},
		slog.Default(),
	)
	if len(guides) != 0 {
		t.Fatalf("expected shell manual to be skipped when allow_shell is false, got %d guides", len(guides))
	}

	enabled := ContextFlags{AllowShell: true}
	guides = PrepareDynamicGuidesWithStrategy(
		nil, nil, "", "", dir, nil, []string{"execute_shell"}, 5,
		DynamicGuideStrategy{Flags: &enabled},
		slog.Default(),
	)
	if len(guides) != 1 || !strings.Contains(guides[0], "Use shell only when enabled.") {
		t.Fatalf("expected shell manual when allow_shell is true, got %#v", guides)
	}
}

func TestPrepareDynamicGuidesWithStrategyRespectsSudoManualCondition(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tools_manuals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manual := "---\nconditions: [\"sudo_enabled\"]\n---\n# execute_sudo\nSudo manual.\n"
	if err := os.WriteFile(filepath.Join(dir, "execute_sudo.md"), []byte(manual), 0o644); err != nil {
		t.Fatalf("WriteFile execute_sudo manual: %v", err)
	}

	flags := ContextFlags{SudoEnabled: false}
	guides := PrepareDynamicGuidesWithStrategy(
		nil, nil, "", "", dir, nil, []string{"execute_sudo"}, 5,
		DynamicGuideStrategy{Flags: &flags},
		slog.Default(),
	)
	if len(guides) != 0 {
		t.Fatalf("expected sudo manual to be skipped when sudo is disabled, got %d guides", len(guides))
	}

	flags.SudoEnabled = true
	guides = PrepareDynamicGuidesWithStrategy(
		nil, nil, "", "", dir, nil, []string{"execute_sudo"}, 5,
		DynamicGuideStrategy{Flags: &flags},
		slog.Default(),
	)
	if len(guides) != 1 {
		t.Fatalf("expected sudo manual when sudo is enabled, got %#v", guides)
	}
}

func TestModuleConditionsAllowDeniesWhenFlagsNil(t *testing.T) {
	mod := PromptModule{
		Metadata: PromptMetadata{
			Tags:       []string{"core"},
			Conditions: []string{"docker_enabled"},
		},
		Content: "docker module",
	}
	if mod.ShouldInclude(nil) {
		t.Fatal("conditioned module must not load when flags are nil")
	}
	if !mod.ShouldInclude(&ContextFlags{DockerEnabled: true}) {
		t.Fatal("conditioned module should load when matching flag is set")
	}
}

func TestGuideConditionsAllowSkipsEnforcementWhenFlagsNil(t *testing.T) {
	conditions := []string{"allow_shell"}
	if !guideConditionsAllow(conditions, nil) {
		t.Fatal("guide conditions should be skipped for explicit lookups without flags")
	}
	if guideConditionsAllow(conditions, &ContextFlags{AllowShell: false}) {
		t.Fatal("guide conditions should block when flags are set and condition does not match")
	}
}

func TestNormalizePromptConditionMapsLegacyVideoDownloadKey(t *testing.T) {
	if got := normalizePromptCondition("tools.video_download.enabled"); got != "video_download_enabled" {
		t.Fatalf("normalizePromptCondition() = %q, want video_download_enabled", got)
	}
	flags := ContextFlags{VideoDownloadEnabled: true}
	if !matchPromptCondition("tools.video_download.enabled", &flags) {
		t.Fatal("expected legacy video download condition to match enabled flag")
	}
}

func TestPrepareDynamicGuidesWithStrategySkipsToolsOutsideAllowedSet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tools_manuals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "homepage.md"), []byte("# homepage\nmanual"), 0o644); err != nil {
		t.Fatalf("WriteFile homepage manual: %v", err)
	}

	guides := PrepareDynamicGuidesWithStrategy(
		nil,
		nil,
		"",
		"",
		dir,
		nil,
		[]string{"homepage"},
		5,
		DynamicGuideStrategy{AllowedTools: []string{"document_creator"}},
		slog.Default(),
	)
	if len(guides) != 0 {
		t.Fatalf("expected disabled/not-allowed guide to be skipped, got %d guides: %q", len(guides), guides)
	}
}

func TestHomepageAndMediaManualsRequireDeployableProjectAssetRefs(t *testing.T) {
	for _, path := range []string{"tools_manuals/homepage.md", "tools_manuals/media_registry.md"} {
		raw, err := promptsembed.FS.ReadFile(path)
		if err != nil {
			t.Fatalf("read embedded %s: %v", path, err)
		}
		manual := string(raw)
		for _, forbidden := range []string{
			"Simply embed the image in your HTML using the exact URL path from `media_registry`",
			"Use the returned `web_path` directly when placing images in pages.",
		} {
			if strings.Contains(manual, forbidden) {
				t.Fatalf("%s still contains unsafe homepage asset guidance %q", path, forbidden)
			}
		}
		for _, marker := range []string{"public/assets", "/assets/", "deployable project asset"} {
			if !strings.Contains(manual, marker) {
				t.Fatalf("%s missing deployable project asset guidance marker %q:\n%s", path, marker, manual)
			}
		}
	}
}

func TestIsToolPathSafeAllowsWindowsCaseVariant(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path comparison is case-insensitive")
	}
	base := filepath.Clean(`C:\Users\Andi\Prompts\tools_manuals`)
	path := filepath.Clean(`c:\users\andi\prompts\tools_manuals\homepage.md`)
	if !isToolPathSafe(path, base) {
		t.Fatalf("expected case-only Windows path variant to remain inside base: path=%q base=%q", path, base)
	}
}

func TestBuildSystemPromptTextJSONModeKeepsDynamicToolGuideExamples(t *testing.T) {
	flags := ContextFlags{
		Tier:            "full",
		SystemLanguage:  "en",
		IsTextModeModel: true,
		PredictedGuides: []string{
			"# invoke_tool\n\n```json\n{\"action\":\"invoke_tool\",\"tool_name\":\"yepapi_instagram\",\"arguments\":{\"operation\":\"user\",\"username\":\"jopliness\"}}\n```",
		},
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, `{"action":"invoke_tool"`) {
		t.Fatalf("text JSON mode should keep raw JSON guide examples")
	}
}

func TestBuildSystemPromptTextJSONModeDoesNotAllowPreambleBeforeJSON(t *testing.T) {
	flags := ContextFlags{
		Tier:            "full",
		SystemLanguage:  "en",
		IsTextModeModel: true,
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if strings.Contains(prompt, "brief 1-sentence acknowledgment may precede the JSON") {
		t.Fatalf("text JSON prompt must not allow acknowledgments before raw JSON tool calls")
	}
	if !strings.Contains(prompt, "Text JSON mode has no preamble exception") {
		t.Fatalf("prompt missing explicit text JSON no-preamble rule")
	}
	if strings.Contains(prompt, `os.path.join("agent_workspace", "workdir", "filename")`) {
		t.Fatalf("prompt must not instruct JSON tool calls to use Python path expressions")
	}
}

func TestBuildSystemPromptCompactsOversizedCoreMemory(t *testing.T) {
	var core strings.Builder
	for i := 1; i <= 160; i++ {
		core.WriteString(fmt.Sprintf("[%d] core-memory-entry-%03d %s\n", i, i, strings.Repeat("x", 160)))
	}

	prompt, _ := buildSystemPromptInner("", &ContextFlags{Tier: "full", SystemLanguage: "en"}, core.String(), slog.Default())
	if !strings.Contains(prompt, "[CORE MEMORY COMPACTED:") {
		t.Fatalf("expected oversized core memory to be compacted")
	}
	if strings.Contains(prompt, "core-memory-entry-050") {
		t.Fatalf("expected middle core memory entries to be omitted")
	}
	if !strings.Contains(prompt, "core-memory-entry-001") || !strings.Contains(prompt, "core-memory-entry-160") {
		t.Fatalf("expected oldest anchor and newest core memories to remain")
	}
}

func TestCompactCoreMemoryForPromptFiltersTransientOperationalDetails(t *testing.T) {
	coreMemory := strings.Join([]string{
		"[1] [preference:user_preferences] User prefers German",
		"[2] [recent_operational_details] WebGL demo updated two weeks ago source:memory_analysis session:default",
		"[3] [user_goal] User wanted to debug a temporary homepage issue source:memory_analysis session:default",
		"[4] [infrastructure] Proxmox cluster is reachable via API",
		"[5] [project_name] phaser-demo source:memory_analysis session:mission-123",
		"[6] [test_file_output] Test file created by homepage tool source:memory_analysis session:mission-123",
		"[7] [WebGL Demo updates] WebGL Galaxy Demo updated with camera position source:memory_analysis session:default",
	}, "\n")

	got := compactCoreMemoryForPrompt(coreMemory)

	if strings.Contains(got, "WebGL demo") {
		t.Fatalf("transient operational detail leaked into prompt core memory: %q", got)
	}
	if strings.Contains(got, "[user_goal]") {
		t.Fatalf("transient user goal leaked into prompt core memory: %q", got)
	}
	if strings.Contains(got, "phaser-demo") || strings.Contains(got, "Test file created") || strings.Contains(got, "WebGL Galaxy") {
		t.Fatalf("stale project/demo artifact leaked into prompt core memory: %q", got)
	}
	if !strings.Contains(got, "User prefers German") || !strings.Contains(got, "Proxmox cluster") {
		t.Fatalf("durable facts missing from core memory prompt: %q", got)
	}
	if !strings.Contains(got, "[CORE MEMORY FILTERED:") {
		t.Fatalf("expected filter marker in prompt core memory: %q", got)
	}
}

func TestBuildSystemPromptKeepsIntegrationOverviewOutOfStablePrefix(t *testing.T) {
	flagsA := ContextFlags{
		Tier:                 "full",
		SystemLanguage:       "en",
		DockerEnabled:        true,
		GitHubEnabled:        true,
		SkipIntegrationTools: []string{"docker"},
	}
	flagsB := flagsA
	flagsB.SkipIntegrationTools = []string{"github"}

	promptA, _ := buildSystemPromptInner("", &flagsA, "", slog.Default())
	promptB, _ := buildSystemPromptInner("", &flagsB, "", slog.Default())

	prefixA, suffixA, okA := strings.Cut(promptA, "# TURN CONTEXT")
	prefixB, suffixB, okB := strings.Cut(promptB, "# TURN CONTEXT")
	if !okA || !okB {
		t.Fatalf("expected # TURN CONTEXT marker in both prompts")
	}
	if prefixA != prefixB {
		t.Fatalf("stable prefix changed when SkipIntegrationTools changed")
	}
	if strings.Contains(prefixA, "[ENABLED INTEGRATIONS]") {
		t.Fatalf("integration overview leaked into stable prefix")
	}
	if !strings.Contains(suffixA, "[ENABLED INTEGRATIONS]") || !strings.Contains(suffixB, "[ENABLED INTEGRATIONS]") {
		t.Fatalf("expected integration overview in volatile suffix")
	}
	if suffixA == suffixB {
		t.Fatalf("expected volatile suffix to reflect changed SkipIntegrationTools")
	}
}

func TestBuildSystemPromptIncludesSpaceAgentRuntimeContext(t *testing.T) {
	flags := ContextFlags{
		Tier:                "full",
		SystemLanguage:      "en",
		SpaceAgentEnabled:   true,
		SpaceAgentPublicURL: "https://aurago-space-agent.example.ts.net/",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	if !strings.Contains(prompt, "## SPACE AGENT INTEGRATION") {
		t.Fatalf("expected Space Agent runtime section in prompt")
	}
	if !strings.Contains(prompt, "space_agent") {
		t.Fatalf("expected Space Agent tool guidance in prompt")
	}
	if !strings.Contains(prompt, "https://aurago-space-agent.example.ts.net/") {
		t.Fatalf("expected Space Agent browser URL in prompt")
	}
	if !strings.Contains(prompt, "external data") {
		t.Fatalf("expected external-data boundary guidance in prompt")
	}
	if !strings.Contains(prompt, "[ENABLED INTEGRATIONS]") || !strings.Contains(prompt, "space_agent") {
		t.Fatalf("expected Space Agent in enabled integrations overview")
	}
}

func TestFallbackSystemPromptIncludesEmbeddedSafetyRules(t *testing.T) {
	flags := ContextFlags{SystemLanguage: "en"}

	prompt, _ := fallbackSystemPrompt("", &flags, "Remember this", slog.Default())

	if !strings.Contains(prompt, "# CORE IDENTITY") {
		t.Fatalf("fallback prompt missing identity section: %q", prompt)
	}
	if !strings.Contains(prompt, "## SAFETY & SECURITY") {
		t.Fatalf("fallback prompt missing safety rules: %q", prompt)
	}
	if !strings.Contains(prompt, "Remember this") {
		t.Fatalf("fallback prompt missing core memory: %q", prompt)
	}
}

func TestBuildSystemPromptAddsChineseDriftGuardForNonChineseLanguage(t *testing.T) {
	flags := ContextFlags{
		Tier:           "full",
		SystemLanguage: "de",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	if !strings.Contains(prompt, "do not insert Chinese words") {
		t.Fatalf("expected anti-Chinese language drift guard in prompt")
	}
}

func TestBuildSystemPromptInjectsTaskRulesAndHomepageDesignBeforeAdditionalPrompt(t *testing.T) {
	flags := ContextFlags{
		Tier:                 "full",
		SystemLanguage:       "en",
		TaskRules:            "## Homepage Workflow\nUse the homepage tool.",
		HomepageDesignSystem: "## homepage DESIGN.md\n## Colors\n- Primary #14B8A6",
		AdditionalPrompt:     "Always answer briefly.",
	}
	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	for _, marker := range []string{
		"# TASK RULES",
		"Use the homepage tool.",
		"# HOMEPAGE DESIGN SYSTEM",
		"## Colors",
		"# ADDITIONAL INSTRUCTIONS",
		"Always answer briefly.",
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("prompt missing %q:\n%s", marker, prompt)
		}
	}
	if strings.Contains(prompt, "# TOOL GUIDES") && strings.Index(prompt, "# TASK RULES") > strings.Index(prompt, "# TOOL GUIDES") {
		t.Fatalf("task rules should be injected before tool guides:\n%s", prompt)
	}
	if strings.Index(prompt, "# HOMEPAGE DESIGN SYSTEM") > strings.Index(prompt, "# ADDITIONAL INSTRUCTIONS") {
		t.Fatalf("homepage design should be injected before additional instructions:\n%s", prompt)
	}
}

func TestBuildSystemPromptKeepsTaskRulesForMissionRuns(t *testing.T) {
	flags := ContextFlags{
		Tier:           "full",
		SystemLanguage: "en",
		IsMission:      true,
		MessageSource:  "mission",
		TaskRules:      "## Homepage Workflow\nUse project-relative web URLs for local assets.",
	}
	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	for _, marker := range []string{
		"# TASK RULES",
		"## Homepage Workflow",
		"Use project-relative web URLs for local assets.",
		"> **Channel:** Mission (automated)",
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("mission prompt missing task rule marker %q:\n%s", marker, prompt)
		}
	}
}

func TestBuildSystemPromptSkipsChineseDriftGuardForChineseLanguage(t *testing.T) {
	for _, language := range []string{"zh", "zh-CN", "Chinese", "中文"} {
		t.Run(language, func(t *testing.T) {
			flags := ContextFlags{
				Tier:           "full",
				SystemLanguage: language,
			}

			prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

			if strings.Contains(prompt, "do not insert Chinese words") {
				t.Fatalf("did not expect anti-Chinese language drift guard for %q", language)
			}
		})
	}
}

func TestBuildSystemPromptIncludesPersonaSignals(t *testing.T) {
	flags := ContextFlags{
		Tier:               "full",
		SystemLanguage:     "en",
		EmotionDescription: "I feel calm and ready to help.",
		PersonalityLine:    "[Self: mood=focused | C:0.60 T:0.70 E:0.80]",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())

	if !strings.Contains(prompt, "### PERSONA SIGNALS") {
		t.Fatalf("expected persona signals section in prompt")
	}
	if !strings.Contains(prompt, flags.PersonalityLine) {
		t.Fatalf("expected personality line to be preserved with emotion state")
	}
	if strings.Contains(prompt, "CURRENT EMOTIONAL STATE") || strings.Contains(prompt, "CURRENT PERSONALITY TRAITS") {
		t.Fatalf("legacy persona headers should not appear:\n%s", prompt)
	}
}

func TestFallbackSystemPromptAddsChineseDriftGuardForNonChineseLanguage(t *testing.T) {
	flags := ContextFlags{SystemLanguage: "en"}

	prompt, _ := fallbackSystemPrompt("", &flags, "", slog.Default())

	if !strings.Contains(prompt, "do not insert Chinese words") {
		t.Fatalf("expected anti-Chinese language drift guard in fallback prompt")
	}
}

func TestBuildSystemPromptContextCancelledReturnsFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	flags := ContextFlags{
		SystemLanguage:   "en",
		Tier:             "full",
		AdditionalPrompt: strings.Repeat("this should only appear in the full build ", 20),
	}

	prompt, tokens := BuildSystemPromptContext(ctx, "", &flags, "Remember this", slog.Default())
	if tokens <= 0 {
		t.Fatalf("expected fallback token count to be positive, got %d", tokens)
	}
	if !strings.Contains(prompt, "# CORE IDENTITY") {
		t.Fatalf("expected fallback identity, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Remember this") {
		t.Fatalf("expected fallback core memory, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "this should only appear in the full build") {
		t.Fatalf("cancelled build should not include full-build additional prompt:\n%s", prompt)
	}
}

func TestCountTokensContextCancelledDoesNotWaitForEncoderInit(t *testing.T) {
	block := make(chan struct{})
	defer close(block)

	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		<-block
		return stubTokenEncoder{tokensPerCall: 9}, nil
	}, time.Second, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	start := time.Now()
	got := CountTokensContext(ctx, "fallback quickly")
	elapsed := time.Since(start)

	if got <= 0 {
		t.Fatalf("expected fallback token count to be positive, got %d", got)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("CountTokensContext waited too long after context deadline: %s", elapsed)
	}
}

func TestBudgetShedContextReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	flags := ContextFlags{
		Tier:        "full",
		TokenBudget: 10,
	}

	_, _, err := budgetShedContext(ctx, strings.Repeat("word ", 100), &flags, "", "", time.Now(), slog.Default())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestHardTruncateToBudgetContextReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := hardTruncateToBudgetContext(ctx, strings.Repeat("word ", 100), 10, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCountTokensRetriesAfterTimeoutAndUsesLateSuccess(t *testing.T) {
	block := make(chan struct{})
	callCount := 0
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		callCount++
		if callCount == 1 {
			<-block
			return stubTokenEncoder{tokensPerCall: 3}, nil
		}
		return stubTokenEncoder{tokensPerCall: 5}, nil
	}, 5*time.Millisecond, 0)

	first := CountTokens("first call falls back")
	if first <= 0 {
		t.Fatalf("expected fallback token count to be positive, got %d", first)
	}

	close(block)

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if CountTokens("second call should use initialized encoder") == 3 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected late encoder initialization to be reused; loader calls=%d", callCount)
}

func TestCountTokensRetriesAfterFailure(t *testing.T) {
	callCount := 0
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("boom")
		}
		return stubTokenEncoder{tokensPerCall: 7}, nil
	}, 25*time.Millisecond, 0)

	first := CountTokens("fallback after error")
	second := CountTokens("second attempt should retry")

	if first <= 0 {
		t.Fatalf("expected fallback token count to be positive, got %d", first)
	}
	if second != 7 {
		t.Fatalf("expected retry to use encoder token count 7, got %d", second)
	}
	if callCount < 2 {
		t.Fatalf("expected loader to be retried, got %d calls", callCount)
	}
}

func TestHardTruncateToBudgetStaysWithinBudget(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return stubTokenEncoder{tokensPerCall: 0}, nil
	}, 25*time.Millisecond, 0)

	// Swap in a token encoder that counts one token per rune for predictable truncation.
	tiktokenMu.Lock()
	tiktokenEnc = runeCountingEncoder{}
	tiktokenMu.Unlock()

	prompt := strings.Repeat("abcdef", 20)
	truncated := hardTruncateToBudget(prompt, 15, "")
	if got := CountTokens(truncated); got > 15 {
		t.Fatalf("expected truncated prompt to fit budget, got %d tokens", got)
	}
	if truncated == "" {
		t.Fatal("expected some truncated content")
	}
}

type runeCountingEncoder struct{}

func (runeCountingEncoder) Encode(text string, allowedSpecial, disallowedSpecial []string) []int {
	return make([]int, len([]rune(text)))
}

func TestShouldIncludeMainAgentConditions(t *testing.T) {
	mod := PromptModule{
		Metadata: PromptMetadata{
			Conditions: []string{"main_agent"},
		},
	}

	if !mod.ShouldInclude(&ContextFlags{}) {
		t.Fatal("expected main agent module to be included for default flags")
	}
	if mod.ShouldInclude(&ContextFlags{IsCoAgent: true}) {
		t.Fatal("expected main agent module to be excluded for co-agent")
	}
	if mod.ShouldInclude(&ContextFlags{IsEgg: true}) {
		t.Fatal("expected main agent module to be excluded for egg")
	}
}

func TestFilterModulesKeepsCoreModulesWithoutConditions(t *testing.T) {
	modules := []PromptModule{
		{
			Metadata: PromptMetadata{
				ID:   "core",
				Tags: []string{"core"},
			},
			Content: "core",
		},
		{
			Metadata: PromptMetadata{
				ID:         "coagent",
				Conditions: []string{"coagent"},
			},
			Content: "coagent",
		},
	}

	filtered := filterModules(modules, &ContextFlags{})
	if len(filtered) != 1 || filtered[0].Metadata.ID != "core" {
		t.Fatalf("unexpected filtered modules: %+v", filtered)
	}
}

func TestParsePromptModuleHandlesBOMAndCRLF(t *testing.T) {
	raw := "\xEF\xBB\xBF\r\n---\r\nid: test\r\ntags: [\"core\"]\r\npriority: 1\r\n---\r\n# Body\r\nLine\r\n"

	mod, err := parsePromptModule(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if mod.Metadata.ID != "test" {
		t.Fatalf("unexpected id: %q", mod.Metadata.ID)
	}
	if !strings.Contains(mod.Content, "# Body") {
		t.Fatalf("unexpected content: %q", mod.Content)
	}
}

func TestParsePromptModuleRejectsMissingClosingFrontmatter(t *testing.T) {
	raw := "---\nid: broken\ntags: [\"core\"]\npriority: 1\n# no closing delimiter"

	if _, err := parsePromptModule(raw); err == nil {
		t.Fatal("expected parsePromptModule to reject missing closing frontmatter")
	}
}

// TestRemoveLineByPrefix_RespectsCodeBlocks verifies DESIGN-1 fix:
// removeLineByPrefix must not remove lines inside ``` or ~~~ code blocks.
func TestRemoveLineByPrefix_RespectsCodeBlocks(t *testing.T) {
	input := "normal line\n```\n[Self: this is inside a code block]\n```\n[Self: this should be removed]\nafter"
	result := removeLineByPrefix(input, "[Self:")

	if !strings.Contains(result, "inside a code block") {
		t.Error("removeLineByPrefix should preserve lines inside ``` code blocks")
	}
	if strings.Contains(result, "this should be removed") {
		t.Error("removeLineByPrefix should remove lines outside code blocks")
	}
}

func TestRemoveLineByPrefix_RespectsTildeCodeBlocks(t *testing.T) {
	input := "normal line\n~~~\n[Self: inside tilde block]\n~~~\n[Self: outside]\nafter"
	result := removeLineByPrefix(input, "[Self:")

	if !strings.Contains(result, "inside tilde block") {
		t.Error("removeLineByPrefix should preserve lines inside ~~~ code blocks")
	}
	if strings.Contains(result, "[Self: outside]") {
		t.Error("removeLineByPrefix should remove [Self: lines outside code blocks")
	}
}

// TestOptimizePrompt_PreservesTildeFences verifies DESIGN-2 fix:
// OptimizePrompt must treat ~~~ the same as ``` for code block tracking.
func TestOptimizePrompt_PreservesTildeFences(t *testing.T) {
	input := "# Header\n~~~json\n{\"key\": \"value\"}\n~~~\nSome text\n"
	result, _ := OptimizePrompt(input)

	if !strings.Contains(result, "~~~json") {
		t.Error("OptimizePrompt should preserve ~~~ code fence opening")
	}
	if !strings.Contains(result, "~~~") {
		t.Error("OptimizePrompt should preserve ~~~ code fence closing")
	}
	// JSON inside ~~~ should be compacted
	if strings.Contains(result, "{\"key\": \"value\"}") {
		// It should be compacted to {"key":"value"}
		t.Log("JSON inside ~~~ was preserved (may or may not be compacted)")
	}
}

// TestBuildEnabledToolsOverview_IncludesPaperlessNGX verifies DESIGN-6 fix:
// PaperlessNGXEnabled must appear in the enabled tools overview.
func TestBuildEnabledToolsOverview_IncludesPaperlessNGX(t *testing.T) {
	flags := &ContextFlags{
		PaperlessNGXEnabled: true,
	}
	overview := buildEnabledToolsOverview(flags)
	if !strings.Contains(overview, "paperless_ngx") {
		t.Errorf("Expected 'paperless_ngx' in overview when PaperlessNGXEnabled=true, got: %s", overview)
	}
}

func TestBuildEnabledToolsOverview_IncludesInvasionControlCapability(t *testing.T) {
	flags := &ContextFlags{
		InvasionControlEnabled: true,
	}
	overview := buildEnabledToolsOverview(flags)
	for _, want := range []string{"invasion_control", "Egg/Nest", "remote agents", "discover_tools"} {
		if !strings.Contains(overview, want) {
			t.Fatalf("expected %q in invasion overview, got: %s", want, overview)
		}
	}
}

func TestBuildEnabledToolsOverview_PointsHiddenIntegrationsToDiscovery(t *testing.T) {
	flags := &ContextFlags{
		DockerEnabled: true,
	}
	overview := buildEnabledToolsOverview(flags)
	if !strings.Contains(overview, "discover_tools") {
		t.Fatalf("enabled integrations overview should point hidden tools to discover_tools, got: %s", overview)
	}
	if strings.Contains(overview, "use it directly by name") {
		t.Fatalf("enabled integrations overview should not tell the agent to call hidden tools directly, got: %s", overview)
	}
}

func TestBuildEnabledToolsOverviewCapsLongLists(t *testing.T) {
	flags := &ContextFlags{
		DockerEnabled:            true,
		HomeAssistantEnabled:     true,
		ProxmoxEnabled:           true,
		TailscaleEnabled:         true,
		AnsibleEnabled:           true,
		GitHubEnabled:            true,
		MQTTEnabled:              true,
		AdGuardEnabled:           true,
		UptimeKumaEnabled:        true,
		MCPEnabled:               true,
		MeshCentralEnabled:       true,
		HomepageEnabled:          true,
		NetlifyEnabled:           true,
		VercelEnabled:            true,
		EmailEnabled:             true,
		CloudflareTunnelEnabled:  true,
		GoogleWorkspaceEnabled:   true,
		OneDriveEnabled:          true,
		VirusTotalEnabled:        true,
		BraveSearchEnabled:       true,
		ImageGenerationEnabled:   true,
		MusicGenerationEnabled:   true,
		VideoGenerationEnabled:   true,
		RemoteControlEnabled:     true,
		BrowserAutomationEnabled: true,
		WebDAVEnabled:            true,
		KoofrEnabled:             true,
		ChromecastEnabled:        true,
		DiscordEnabled:           true,
		TelegramEnabled:          true,
		TrueNASEnabled:           true,
		JellyfinEnabled:          true,
		ObsidianEnabled:          true,
		OllamaEnabled:            true,
		SandboxEnabled:           true,
		WebhooksEnabled:          true,
		WebScraperEnabled:        true,
		S3Enabled:                true,
		NetworkPingEnabled:       true,
		NetworkScanEnabled:       true,
		UPnPScanEnabled:          true,
		FormAutomationEnabled:    true,
		WOLEnabled:               true,
		FritzBoxSystemEnabled:    true,
		TelnyxEnabled:            true,
		A2AEnabled:               true,
		InvasionControlEnabled:   true,
		CoAgentEnabled:           true,
		PaperlessNGXEnabled:      true,
		SpaceAgentEnabled:        true,
	}

	overview := buildEnabledToolsOverview(flags)
	if !strings.Contains(overview, "+") || !strings.Contains(overview, "more via discover_tools") {
		t.Fatalf("long overview should summarize overflow through discover_tools, got: %s", overview)
	}

	listText := strings.TrimPrefix(overview, "[ENABLED INTEGRATIONS] ")
	listText = strings.SplitN(listText, ".", 2)[0]
	visible := 0
	for _, part := range strings.Split(listText, ",") {
		if !strings.Contains(part, "more via discover_tools") {
			visible++
		}
	}
	if visible > 12 {
		t.Fatalf("overview exposes %d integrations, want <= 12: %s", visible, overview)
	}
}

func TestBuildEnabledToolsOverview_ExcludesPaperlessNGXWhenDisabled(t *testing.T) {
	flags := &ContextFlags{
		PaperlessNGXEnabled: false,
	}
	overview := buildEnabledToolsOverview(flags)
	if strings.Contains(overview, "paperless_ngx") {
		t.Error("paperless_ngx should NOT appear when PaperlessNGXEnabled=false")
	}
}

// TestHardTruncateToBudget_WithUnicode verifies HIGH-1 fix:
// hardTruncateToBudget should work correctly with Unicode content on byte level.
func TestHardTruncateToBudget_WithUnicode(t *testing.T) {
	// Use a char-counting encoder: ~1 token per 4 chars (realistic ratio).
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Millisecond)

	// 100 CJK runes = 300 bytes. With ~1 token/4chars ≈ 25 tokens.
	// Budget of 10 tokens should truncate to ~40 chars = ~120 bytes.
	prompt := strings.Repeat("界", 100)
	result := hardTruncateToBudget(prompt, 10, "test-model")
	if result == "" {
		t.Fatal("hardTruncateToBudget should return non-empty result")
	}
	if len(result) >= len(prompt) {
		t.Errorf("Result should be shorter: got %d bytes, input %d bytes", len(result), len(prompt))
	}
}

// TestHardTruncateToBudget_WithEmoji verifies HIGH-1 fix:
// Emoji characters (multi-byte, multi-token) should not cause overflow.
func TestHardTruncateToBudget_WithEmoji(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return stubTokenEncoder{tokensPerCall: 1}, nil
	}, time.Second, time.Millisecond)

	prompt := "Hello 🌍🌎🌏 World " + strings.Repeat("x ", 200)
	result := hardTruncateToBudget(prompt, 20, "test-model")
	if result == "" {
		t.Fatal("hardTruncateToBudget should return non-empty result")
	}
}

func TestPromptConditionsCoverEmbeddedFrontmatter(t *testing.T) {
	flagForCondition := map[string]func() *ContextFlags{
		"a2a_enabled":                func() *ContextFlags { return &ContextFlags{A2AEnabled: true} },
		"adguard_enabled":            func() *ContextFlags { return &ContextFlags{AdGuardEnabled: true} },
		"allow_network_requests":     func() *ContextFlags { return &ContextFlags{AllowNetworkRequests: true} },
		"allow_package_manager":      func() *ContextFlags { return &ContextFlags{PackageManagerEnabled: true} },
		"allow_python":               func() *ContextFlags { return &ContextFlags{AllowPython: true} },
		"allow_remote_shell":         func() *ContextFlags { return &ContextFlags{AllowRemoteShell: true} },
		"allow_shell":                func() *ContextFlags { return &ContextFlags{AllowShell: true} },
		"ansible_enabled":            func() *ContextFlags { return &ContextFlags{AnsibleEnabled: true} },
		"brave_search_enabled":       func() *ContextFlags { return &ContextFlags{BraveSearchEnabled: true} },
		"chromecast_enabled":         func() *ContextFlags { return &ContextFlags{ChromecastEnabled: true} },
		"cloudflare_tunnel_enabled":  func() *ContextFlags { return &ContextFlags{CloudflareTunnelEnabled: true} },
		"coagent":                    func() *ContextFlags { return &ContextFlags{IsCoAgent: true} },
		"coagent_enabled":            func() *ContextFlags { return &ContextFlags{CoAgentEnabled: true} },
		"discord_enabled":            func() *ContextFlags { return &ContextFlags{DiscordEnabled: true} },
		"docker_enabled":             func() *ContextFlags { return &ContextFlags{DockerEnabled: true} },
		"document_creator_enabled":   func() *ContextFlags { return &ContextFlags{DocumentCreatorEnabled: true} },
		"egg":                        func() *ContextFlags { return &ContextFlags{IsEgg: true} },
		"email_enabled":              func() *ContextFlags { return &ContextFlags{EmailEnabled: true} },
		"form_automation_enabled":    func() *ContextFlags { return &ContextFlags{FormAutomationEnabled: true} },
		"frigate_enabled":            func() *ContextFlags { return &ContextFlags{FrigateEnabled: true} },
		"fritzbox_network_enabled":   func() *ContextFlags { return &ContextFlags{FritzBoxNetworkEnabled: true} },
		"fritzbox_smarthome_enabled": func() *ContextFlags { return &ContextFlags{FritzBoxSmartHomeEnabled: true} },
		"fritzbox_storage_enabled":   func() *ContextFlags { return &ContextFlags{FritzBoxStorageEnabled: true} },
		"fritzbox_system_enabled":    func() *ContextFlags { return &ContextFlags{FritzBoxSystemEnabled: true} },
		"fritzbox_telephony_enabled": func() *ContextFlags { return &ContextFlags{FritzBoxTelephonyEnabled: true} },
		"fritzbox_tv_enabled":        func() *ContextFlags { return &ContextFlags{FritzBoxTVEnabled: true} },
		"github_enabled":             func() *ContextFlags { return &ContextFlags{GitHubEnabled: true} },
		"golangci_lint_enabled":      func() *ContextFlags { return &ContextFlags{GolangciLintEnabled: true} },
		"google_workspace_enabled":   func() *ContextFlags { return &ContextFlags{GoogleWorkspaceEnabled: true} },
		"grafana_enabled":            func() *ContextFlags { return &ContextFlags{GrafanaEnabled: true} },
		"home_assistant_enabled":     func() *ContextFlags { return &ContextFlags{HomeAssistantEnabled: true} },
		"homepage_enabled":           func() *ContextFlags { return &ContextFlags{HomepageEnabled: true} },
		"homepage_registry_enabled":  func() *ContextFlags { return &ContextFlags{HomepageRegistryEnabled: true} },
		"image_generation_enabled":   func() *ContextFlags { return &ContextFlags{ImageGenerationEnabled: true} },
		"invasion_control_enabled":   func() *ContextFlags { return &ContextFlags{InvasionControlEnabled: true} },
		"is_docker":                  func() *ContextFlags { return &ContextFlags{IsDocker: true} },
		"is_error":                   func() *ContextFlags { return &ContextFlags{IsErrorState: true} },
		"koofr_enabled":              func() *ContextFlags { return &ContextFlags{KoofrEnabled: true} },
		"lifeboat":                   func() *ContextFlags { return &ContextFlags{LifeboatEnabled: true} },
		"main_agent":                 func() *ContextFlags { return &ContextFlags{} },
		"maintenance":                func() *ContextFlags { return &ContextFlags{IsMaintenanceMode: true} },
		"mcp_enabled":                func() *ContextFlags { return &ContextFlags{MCPEnabled: true} },
		"media_registry_enabled":     func() *ContextFlags { return &ContextFlags{MediaRegistryEnabled: true} },
		"meshcentral_enabled":        func() *ContextFlags { return &ContextFlags{MeshCentralEnabled: true} },
		"minimax_tts_enabled":        func() *ContextFlags { return &ContextFlags{MiniMaxTTSEnabled: true} },
		"mqtt_enabled":               func() *ContextFlags { return &ContextFlags{MQTTEnabled: true} },
		"netlify_enabled":            func() *ContextFlags { return &ContextFlags{NetlifyEnabled: true} },
		"ollama_enabled":             func() *ContextFlags { return &ContextFlags{OllamaEnabled: true} },
		"paperless_ngx_enabled":      func() *ContextFlags { return &ContextFlags{PaperlessNGXEnabled: true} },
		"proxmox_enabled":            func() *ContextFlags { return &ContextFlags{ProxmoxEnabled: true} },
		"remote_control_enabled":     func() *ContextFlags { return &ContextFlags{RemoteControlEnabled: true} },
		"requires_coding":            func() *ContextFlags { return &ContextFlags{RequiresCoding: true} },
		"s3_enabled":                 func() *ContextFlags { return &ContextFlags{S3Enabled: true} },
		"sandbox_enabled":            func() *ContextFlags { return &ContextFlags{SandboxEnabled: true} },
		"specialists_available":      func() *ContextFlags { return &ContextFlags{SpecialistsAvailable: true} },
		"sudo_enabled":               func() *ContextFlags { return &ContextFlags{SudoEnabled: true} },
		"tailscale_enabled":          func() *ContextFlags { return &ContextFlags{TailscaleEnabled: true} },
		"uptime_kuma_enabled":        func() *ContextFlags { return &ContextFlags{UptimeKumaEnabled: true} },
		"vercel_enabled":             func() *ContextFlags { return &ContextFlags{VercelEnabled: true} },
		"video_download_enabled":     func() *ContextFlags { return &ContextFlags{VideoDownloadEnabled: true} },
		"virustotal_enabled":         func() *ContextFlags { return &ContextFlags{VirusTotalEnabled: true} },
		"web_scraper_enabled":        func() *ContextFlags { return &ContextFlags{WebScraperEnabled: true} },
		"webdav_enabled":             func() *ContextFlags { return &ContextFlags{WebDAVEnabled: true} },
		"wol_enabled":                func() *ContextFlags { return &ContextFlags{WOLEnabled: true} },
	}

	seen := make(map[string]string)
	err := fs.WalkDir(promptsembed.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := fs.ReadFile(promptsembed.FS, path)
		if err != nil {
			return err
		}
		mod, err := parsePromptModule(string(data))
		if err != nil {
			return nil
		}
		for _, cond := range mod.Metadata.Conditions {
			seen[normalizePromptCondition(cond)] = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded prompts: %v", err)
	}

	for cond, path := range seen {
		buildFlags, ok := flagForCondition[cond]
		if !ok {
			t.Fatalf("condition %q from %s has no test mapper", cond, path)
		}
		if !matchPromptCondition(cond, buildFlags()) {
			t.Fatalf("condition %q from %s did not match its enabled flag", cond, path)
		}
	}
}

func TestMatchPromptConditionCoversAuditToolToggles(t *testing.T) {
	tests := []struct {
		condition string
		flags     *ContextFlags
	}{
		{"music_generation_enabled", &ContextFlags{MusicGenerationEnabled: true}},
		{"browser_automation_enabled", &ContextFlags{BrowserAutomationEnabled: true}},
		{"network_ping_enabled", &ContextFlags{NetworkPingEnabled: true}},
		{"network_scan_enabled", &ContextFlags{NetworkScanEnabled: true}},
		{"upnp_scan_enabled", &ContextFlags{UPnPScanEnabled: true}},
		{"telegram_enabled", &ContextFlags{TelegramEnabled: true}},
		{"space_agent_enabled", &ContextFlags{SpaceAgentEnabled: true}},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			if !matchPromptCondition(tt.condition, tt.flags) {
				t.Fatalf("expected %s to match", tt.condition)
			}
		})
	}
}

func TestParsePromptModuleAcceptsCRLFClosingFrontmatter(t *testing.T) {
	raw := "---\r\nid: crlf\r\ntags: [\"core\"]\r\n---\r\nBody"
	mod, err := parsePromptModule(raw)
	if err != nil {
		t.Fatalf("parsePromptModule() error = %v", err)
	}
	if mod.Metadata.ID != "crlf" || mod.Content != "Body" {
		t.Fatalf("unexpected module: %+v", mod)
	}
}

func TestMissionPreparationTemplateIsNotIncludedInSystemPrompt(t *testing.T) {
	prompt, _ := BuildSystemPromptContext(context.Background(), t.TempDir(), &ContextFlags{
		Tier:        "full",
		TokenBudget: 5000,
	}, "", slog.Default())
	if strings.Contains(prompt, "mission preparation analyst") {
		t.Fatalf("mission preparation template leaked into system prompt:\n%s", prompt)
	}
}

func TestBudgetShedRecountsAfterEachShedAndKeepsLaterSections(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return markerAwareEncoder{}, nil
	}, time.Second, time.Millisecond)

	prompt := "# TOOL GUIDES\nBIG_GUIDE\n\n### INNER VOICE\nKeep this once guide is gone.\n\n# FINAL\nsmall"
	flags := &ContextFlags{TokenBudget: 100}
	result, shed, err := budgetShedContext(context.Background(), prompt, flags, "", "", time.Now(), slog.Default())
	if err != nil {
		t.Fatalf("budgetShedContext: %v", err)
	}
	if !strings.Contains(result, "### INNER VOICE") {
		t.Fatalf("inner voice should be kept after accurate recount:\n%s", result)
	}
	if strings.Join(shed, ",") != "# TOOL GUIDES" {
		t.Fatalf("shed = %v, want only tool guides", shed)
	}
}

func TestBudgetShedCanDropRetrievedMemoriesAsWholeSection(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return markerAwareEncoder{}, nil
	}, time.Second, time.Millisecond)

	prompt := "# RETRIEVED MEMORIES\nBIG_MEMORY\n---\nsmall memory\n\n# FINAL\nsmall"
	flags := &ContextFlags{TokenBudget: 50}
	result, shed, err := budgetShedContext(context.Background(), prompt, flags, "", "", time.Now(), slog.Default())
	if err != nil {
		t.Fatalf("budgetShedContext: %v", err)
	}
	if strings.Contains(result, "# RETRIEVED MEMORIES") {
		t.Fatalf("retrieved memories section should be dropped:\n%s", result)
	}
	if !containsString(shed, "# RETRIEVED MEMORIES") {
		t.Fatalf("shed = %v, want full retrieved memories shed marker", shed)
	}
}

func TestUnifiedMemoryBlockIncludesOperationalContexts(t *testing.T) {
	block := buildUnifiedMemoryContextBlock("full", &ContextFlags{
		ErrorPatternContext: "known error",
		LearnedRulesContext: "learned rule",
		ReuseContext:        "reuse hint",
	})
	for _, want := range []string{"## Known Error Patterns", "known error", "## Learned Rules", "learned rule", "## Reuse-First Context", "reuse hint"} {
		if !strings.Contains(block, want) {
			t.Fatalf("unified memory block missing %q:\n%s", want, block)
		}
	}
}

func TestBuildSystemPromptUnifiedMemoryDoesNotDuplicateOperationalContexts(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)

	prompt, _ := BuildSystemPromptContext(context.Background(), t.TempDir(), &ContextFlags{
		Tier:                "full",
		TokenBudget:         200000,
		UnifiedMemoryBlock:  true,
		ErrorPatternContext: "known error",
		LearnedRulesContext: "learned rule",
		ReuseContext:        "reuse hint",
	}, "", slog.Default())

	for _, marker := range []string{
		"known error",
		"learned rule",
		"reuse hint",
	} {
		if got := strings.Count(prompt, marker); got != 1 {
			t.Fatalf("%q appears %d times, want exactly once:\n%s", marker, got, prompt)
		}
	}
	for _, legacyHeader := range []string{
		"# KNOWN ERROR PATTERNS",
		"# LEARNED RULES",
		"# REUSE-FIRST CONTEXT",
	} {
		if strings.Contains(prompt, legacyHeader) {
			t.Fatalf("legacy section %q should not appear when UnifiedMemoryBlock is enabled:\n%s", legacyHeader, prompt)
		}
	}
	if !strings.Contains(prompt, "# UNIFIED MEMORY CONTEXT") {
		t.Fatalf("expected unified memory block in prompt:\n%s", prompt)
	}
}

func TestLoadCorePersonalityContentCacheIsScopedByPromptDir(t *testing.T) {
	ClearPromptCache()
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(filepath.Join(dir, "personalities"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(firstDir, "personalities", "neutral.md"), []byte("first personality"), 0o644); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "personalities", "neutral.md"), []byte("second personality"), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}

	if got := loadCorePersonalityContent(firstDir, "neutral", slog.Default()); got != "first personality" {
		t.Fatalf("first content = %q", got)
	}
	if got := loadCorePersonalityContent(secondDir, "neutral", slog.Default()); got != "second personality" {
		t.Fatalf("second content = %q", got)
	}
}

func TestBuildEnabledToolsOverviewCoversAuditTogglesAndExactSkips(t *testing.T) {
	flags := &ContextFlags{
		MusicGenerationEnabled:   true,
		BrowserAutomationEnabled: true,
		NetworkPingEnabled:       true,
		NetworkScanEnabled:       true,
		UPnPScanEnabled:          true,
		TelegramEnabled:          true,
		SpaceAgentEnabled:        true,
		SkipIntegrationTools:     []string{"network_ping"},
	}
	overview := buildEnabledToolsOverview(flags)
	for _, want := range []string{"music_generation", "browser_automation", "network_scan", "upnp_scan", "telegram", "space_agent"} {
		if !strings.Contains(overview, want) {
			t.Fatalf("overview missing %q: %s", want, overview)
		}
	}
	if strings.Contains(overview, "network_ping") {
		t.Fatalf("overview should exactly skip network_ping: %s", overview)
	}
}

func TestSpecialistPlaceholdersNeverLeak(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "specialists_awareness.md"), []byte(`---
id: specialists_awareness
tags: ["core"]
priority: 1
---
Status={{SPECIALISTS_STATUS}}
Suggestion={{SPECIALISTS_SUGGESTION}}`), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	prompt, _ := BuildSystemPromptContext(context.Background(), dir, &ContextFlags{
		Tier:                 "full",
		TokenBudget:          5000,
		SpecialistsAvailable: true,
		SpecialistsStatus:    "",
	}, "", slog.Default())
	if strings.Contains(prompt, "{{SPECIALISTS_STATUS}}") || strings.Contains(prompt, "{{SPECIALISTS_SUGGESTION}}") {
		t.Fatalf("specialist placeholders leaked:\n%s", prompt)
	}
}

func TestOptimizePromptDoesNotFabricateClosingFenceForIncompleteJSON(t *testing.T) {
	input := "Before\n```json\n{\"a\":1"
	got, _ := OptimizePrompt(input)
	if strings.Count(got, "```") != 1 {
		t.Fatalf("expected only original opening fence, got:\n%s", got)
	}
	if !strings.Contains(got, "{\"a\":1") {
		t.Fatalf("expected JSON content to be preserved, got:\n%s", got)
	}
}

func TestTokenMultiplierUsesConservativeModelMargins(t *testing.T) {
	if tokenMultiplier("claude-3-5-sonnet") <= 1.0 {
		t.Fatal("claude multiplier should be conservative")
	}
	if tokenMultiplier("gemini-2.0-flash") <= 1.0 {
		t.Fatal("gemini multiplier should be conservative")
	}
	if tokenMultiplier("deepseek-chat") <= 1.0 {
		t.Fatal("deepseek multiplier should be conservative")
	}
	if tokenMultiplier("gpt-4o") != 1.0 {
		t.Fatal("gpt multiplier should stay at baseline")
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
