package agent

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

func intPtr(v int) *int { return &v }

func TestStreamToolCallAssemblerEmptyReturnsNil(t *testing.T) {
	asm := NewStreamToolCallAssembler()
	result := asm.Assemble()
	if result != nil {
		t.Fatalf("expected nil for empty assembler, got %d items", len(result))
	}
}

func TestStreamToolCallAssemblerMergesAndSorts(t *testing.T) {
	asm := NewStreamToolCallAssembler()
	asm.Merge(openai.ToolCall{
		Index: intPtr(7),
		ID:    "call-7",
		Function: openai.FunctionCall{
			Name:      "filesystem",
			Arguments: `{"operation":"stat"}`,
		},
	})
	asm.Merge(openai.ToolCall{
		Index: intPtr(2),
		ID:    "call-2",
		Function: openai.FunctionCall{
			Name:      "execute_shell",
			Arguments: `{"command":"pwd"}`,
		},
	})

	result := asm.Assemble()
	if len(result) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result))
	}
	if result[0].ID != "call-2" || result[1].ID != "call-7" {
		t.Fatalf("unexpected assembly order: %q, %q", result[0].ID, result[1].ID)
	}
}

func TestStreamToolCallAssemblerAppendsFragments(t *testing.T) {
	asm := NewStreamToolCallAssembler()
	asm.Merge(openai.ToolCall{
		Index: intPtr(1),
		ID:    "call-1",
		Function: openai.FunctionCall{
			Name:      "exec",
			Arguments: `{"comm`,
		},
	})
	asm.Merge(openai.ToolCall{
		Index: intPtr(1),
		Function: openai.FunctionCall{
			Name:      "ute_shell",
			Arguments: `and":"pwd"}`,
		},
	})

	result := asm.Assemble()
	if len(result) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result))
	}
	if result[0].Function.Name != "execute_shell" {
		t.Fatalf("Function.Name = %q, want %q", result[0].Function.Name, "execute_shell")
	}
	if result[0].Function.Arguments != `{"command":"pwd"}` {
		t.Fatalf("Function.Arguments = %q", result[0].Function.Arguments)
	}
}

func TestCompactMemoryForPromptStripsEscapedThinkingTags(t *testing.T) {
	input := "headline\n&lt;think&gt;private reasoning&lt;/think&gt;\nuseful memory"

	got := compactMemoryForPrompt(input, 1000)
	if strings.Contains(got, "private reasoning") || strings.Contains(strings.ToLower(got), "think") {
		t.Fatalf("expected escaped thinking block to be stripped, got %q", got)
	}
	if !strings.Contains(got, "useful memory") {
		t.Fatalf("expected useful memory to remain, got %q", got)
	}
}

func TestCompactMemoryForPromptPreservesUTF8WhenTruncating(t *testing.T) {
	input := "abcäxyz"

	got := compactMemoryForPrompt(input, 4)

	if !utf8.ValidString(got) {
		t.Fatalf("compactMemoryForPrompt returned invalid UTF-8: %q", got)
	}
	if got != "abc…" {
		t.Fatalf("compactMemoryForPrompt() = %q, want %q", got, "abc…")
	}
}

func TestSelectServedRAGMemoriesBackfillsAfterServeFilter(t *testing.T) {
	ranked := []rankedMemory{
		{text: "[tool_availability] old transient claim", docID: "stale-1", score: 0.99},
		{text: "servable memory one", docID: "good-1", score: 0.80},
		{text: "This tool is not available in the current installation", docID: "stale-2", score: 0.79},
		{text: "servable memory two", docID: "good-2", score: 0.78},
		{text: "servable memory three", docID: "good-3", score: 0.77},
	}

	served := selectServedRAGMemories(ranked, 3, nil)
	if len(served) != 3 {
		t.Fatalf("served count = %d, want 3", len(served))
	}
	wantIDs := []string{"good-1", "good-2", "good-3"}
	for i, want := range wantIDs {
		if served[i].docID != want {
			t.Fatalf("served[%d] docID = %q, want %q (served=%+v)", i, served[i].docID, want, served)
		}
	}
}

func TestSelectRAGMemoriesForOnDemandSplitsEssentialAndAvailable(t *testing.T) {
	ranked := []rankedMemory{
		{text: "primary memory", docID: "mem-1", score: 0.90},
		{text: "[tool_availability] stale tool memory", docID: "stale-1", score: 0.89},
		{text: "second memory", docID: "mem-2", score: 0.80},
		{text: "third memory", docID: "mem-3", score: 0.70},
		{text: "fourth memory", docID: "mem-4", score: 0.60},
	}
	cfg := config.MemoryOnDemandRetrievalConfig{
		Enabled:              true,
		MaxEssentialMemories: 1,
		MaxAvailableMemories: 2,
		MaxAvailableChars:    400,
	}

	essential, available := selectRAGMemoriesForOnDemand(ranked, cfg, nil)

	if len(essential) != 1 || essential[0].docID != "mem-1" {
		t.Fatalf("essential = %+v, want only mem-1", essential)
	}
	gotIDs := []string{}
	for _, item := range available {
		gotIDs = append(gotIDs, item.docID)
	}
	if strings.Join(gotIDs, ",") != "mem-2,mem-3" {
		t.Fatalf("available IDs = %v, want mem-2,mem-3", gotIDs)
	}
}

func TestSelectRAGMemoriesForOnDemandDisabledPreservesLegacySingleMemory(t *testing.T) {
	ranked := []rankedMemory{
		{text: "primary memory", docID: "mem-1", score: 0.90},
		{text: "second memory", docID: "mem-2", score: 0.80},
	}
	cfg := config.MemoryOnDemandRetrievalConfig{
		Enabled:              false,
		MaxEssentialMemories: 2,
		MaxAvailableMemories: 4,
	}

	essential, available := selectRAGMemoriesForOnDemand(ranked, cfg, nil)

	if len(essential) != 1 || essential[0].docID != "mem-1" {
		t.Fatalf("disabled on-demand should preserve legacy single direct memory, got %+v", essential)
	}
	if len(available) != 0 {
		t.Fatalf("disabled on-demand should not expose available memories, got %+v", available)
	}
}

func TestMemoryDedupeMapForScopeSessionPersistsAcrossTurns(t *testing.T) {
	sessionID := "test-session-" + strings.ReplaceAll(t.Name(), "/", "-")
	firstTurn := memoryDedupeMapForScope("session", sessionID, map[string]int{})
	firstTurn["mem-1"] = 2
	persistMemoryDedupeMapForScope("session", sessionID, firstTurn)
	t.Cleanup(func() { persistMemoryDedupeMapForScope("session", sessionID, map[string]int{}) })

	secondTurn := memoryDedupeMapForScope("session", sessionID, map[string]int{})
	if secondTurn["mem-1"] != 2 {
		t.Fatalf("session dedupe did not persist memory usage across turns: %+v", secondTurn)
	}

	turnScoped := memoryDedupeMapForScope("turn", sessionID, map[string]int{})
	if turnScoped["mem-1"] != 0 {
		t.Fatalf("turn dedupe should not reuse session map, got %+v", turnScoped)
	}
}

func TestBuildAvailableMemoryIndexUsesStableIDsAndLimits(t *testing.T) {
	available := []rankedMemory{
		{text: strings.Repeat("deployment detail ", 20), docID: "mem-2", score: 0.81},
		{text: "backup memory", docID: "mem-3", score: 0.70},
	}

	got := buildAvailableMemoryIndex(available, 170)

	if !strings.Contains(got, "[memory:mem-2]") || !strings.Contains(got, "score=0.81") {
		t.Fatalf("available index missing stable memory id/score: %q", got)
	}
	if strings.Contains(got, "mem-3") {
		t.Fatalf("available index should respect max chars before adding mem-3: %q", got)
	}
	if len([]rune(got)) > 171 {
		t.Fatalf("available index length = %d, want <= 171 including ellipsis", len([]rune(got)))
	}
}

func TestCountMemoryPromptTelemetryTokensIncludesAllMemorySections(t *testing.T) {
	flags := prompts.ContextFlags{
		RetrievedMemories:   "[Recent Day Anchors]\n- 2026-06-11: deployment",
		PredictedMemories:   "NAS backup retention",
		KnowledgeContext:    "KG: nas -> backup_server",
		ErrorPatternContext: "Tool: deploy | Error: timeout",
		LearnedRulesContext: "Rule: verify service health after restart",
	}
	got := countMemoryPromptTelemetryTokens(flags, "test-model")
	withoutLearned := flags
	withoutLearned.LearnedRulesContext = ""
	if got <= countMemoryPromptTelemetryTokens(withoutLearned, "test-model") {
		t.Fatalf("expected learned rules to add telemetry tokens, got %d", got)
	}
}

func TestBuildAggressiveRAGPromptEntriesKeepsOneCompactMemory(t *testing.T) {
	served := []rankedMemory{
		{text: strings.Repeat("primary memory detail ", 80), docID: "mem-1", score: 0.95},
		{text: "secondary memory", docID: "mem-2", score: 0.90},
	}

	entries := buildAggressiveRAGPromptEntries(served, false, nil)
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1: %+v", len(entries), entries)
	}
	if entries[0].docID != "mem-1" {
		t.Fatalf("entry docID = %q, want mem-1", entries[0].docID)
	}
	if len([]rune(entries[0].text)) > 321 {
		t.Fatalf("entry text length = %d, want <= 321 including ellipsis", len([]rune(entries[0].text)))
	}
}

func TestBuildAggressiveRAGPromptEntriesDetailedReplacesTopMemory(t *testing.T) {
	served := []rankedMemory{
		{text: "short", docID: "mem-1", score: 0.95},
	}
	entries := buildAggressiveRAGPromptEntries(served, true, func(id string) (string, error) {
		if id != "mem-1" {
			t.Fatalf("unexpected full memory id %q", id)
		}
		return strings.Repeat("full verified detail ", 80), nil
	})

	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if !strings.HasPrefix(entries[0].text, "[Detailed Memory]\n") {
		t.Fatalf("detailed entry should replace top memory, got %q", entries[0].text)
	}
	if strings.Contains(entries[0].text, "short") {
		t.Fatalf("detailed entry should replace, not append original short memory: %q", entries[0].text)
	}
}

func TestApplyAggressivePromptContextBudgetsCapsDynamicSections(t *testing.T) {
	flags := &prompts.ContextFlags{
		RetrievedMemories:      strings.Repeat("retrieved ", 400),
		PredictedMemories:      strings.Repeat("predicted ", 80),
		RecentActivityOverview: strings.Repeat("recent ", 200),
		KnowledgeContext:       strings.Repeat("kg ", 600),
		ErrorPatternContext:    strings.Repeat("error ", 120),
		LearnedRulesContext:    strings.Repeat("rule ", 120),
		ReuseContext:           strings.Repeat("reuse ", 120),
	}

	applyAggressivePromptContextBudgets(flags)

	if len([]rune(flags.RetrievedMemories)) > 1501 {
		t.Fatalf("RetrievedMemories length = %d, want <= 1501", len([]rune(flags.RetrievedMemories)))
	}
	if len([]rune(flags.PredictedMemories)) > 261 {
		t.Fatalf("PredictedMemories length = %d, want <= 261", len([]rune(flags.PredictedMemories)))
	}
	if len([]rune(flags.RecentActivityOverview)) > 701 {
		t.Fatalf("RecentActivityOverview length = %d, want <= 701", len([]rune(flags.RecentActivityOverview)))
	}
	if len([]rune(flags.KnowledgeContext)) > 801 {
		t.Fatalf("KnowledgeContext length = %d, want <= 801", len([]rune(flags.KnowledgeContext)))
	}
	if len([]rune(flags.ErrorPatternContext)) > 701 {
		t.Fatalf("ErrorPatternContext length = %d, want <= 701", len([]rune(flags.ErrorPatternContext)))
	}
	if len([]rune(flags.LearnedRulesContext)) > 481 {
		t.Fatalf("LearnedRulesContext length = %d, want <= 481", len([]rune(flags.LearnedRulesContext)))
	}
	if len([]rune(flags.ReuseContext)) > 701 {
		t.Fatalf("ReuseContext length = %d, want <= 701", len([]rune(flags.ReuseContext)))
	}
}

func TestLazySpecialistAndRuntimePathIntent(t *testing.T) {
	if shouldInjectSpecialistAwareness("prüfe das bitte normal", "") {
		t.Fatal("did not expect specialist awareness for ordinary request")
	}
	if !shouldInjectSpecialistAwareness("delegiere das an einen Security Spezialisten", "") {
		t.Fatal("expected specialist awareness for explicit specialist delegation")
	}
	if shouldExposeRuntimePaths("prüfe das bitte normal", nil, nil) {
		t.Fatal("did not expect runtime paths for ordinary request")
	}
	if !shouldExposeRuntimePaths("erstelle einen Python Skill für CSV Import", nil, nil) {
		t.Fatal("expected runtime paths for skill creation intent")
	}
	if !shouldExposeRuntimePaths("weiter mit dem Tool", []string{"execute_skill"}, nil) {
		t.Fatal("expected runtime paths after recent skill/tool usage")
	}
}

func TestRuntimePromptContextIntentGates(t *testing.T) {
	if shouldInjectReachableChatChannelsContext("prüfe den Prompt", "web_chat", nil, nil) {
		t.Fatal("did not expect reachable chat channels for ordinary web chat")
	}
	if !shouldInjectReachableChatChannelsContext("benachrichtige mich per ntfy", "web_chat", nil, nil) {
		t.Fatal("expected reachable chat channels for notification intent")
	}
	if !shouldInjectReachableChatChannelsContext("prüfe den Prompt", "telegram", nil, nil) {
		t.Fatal("expected reachable chat channels outside web chat")
	}

	if shouldInjectSpaceAgentRuntimePrompt("bewerte den Prompt", nil, nil) {
		t.Fatal("did not expect space-agent context for ordinary prompt review")
	}
	if !shouldInjectSpaceAgentRuntimePrompt("delegiere das an den Space Agent Sidecar", nil, nil) {
		t.Fatal("expected space-agent context for explicit delegation intent")
	}
	if !shouldInjectSpaceAgentRuntimePrompt("weiter damit", []string{"space_agent"}, nil) {
		t.Fatal("expected space-agent context after recent space_agent usage")
	}

	if shouldInjectUserProfilingPrompt("analysiere diesen Logauszug") {
		t.Fatal("did not expect user profiling for ordinary analysis")
	}
	if !shouldInjectUserProfilingPrompt("welche Präferenzen kennst du über mich?") {
		t.Fatal("expected user profiling for explicit profile intent")
	}
}

func TestRuntimePromptContextCapabilityDaemonAndInternetGates(t *testing.T) {
	if shouldInjectCapabilityCreationPrompt("pruefe den prompt", nil, nil) {
		t.Fatal("did not expect capability creation prompt for ordinary prompt review")
	}
	for _, text := range []string{
		"zeige mir die aktuelle capability routing bewertung",
		"pruefe das config template",
	} {
		if shouldInjectCapabilityCreationPrompt(text, nil, nil) {
			t.Fatalf("did not expect capability creation prompt for generic text %q", text)
		}
	}
	if !shouldInjectCapabilityCreationPrompt("erstelle einen Python Skill fuer CSV Import", nil, nil) {
		t.Fatal("expected capability creation prompt for Python skill intent")
	}
	if !shouldInjectCapabilityCreationPrompt("create a reusable capability for CSV import", nil, nil) {
		t.Fatal("expected capability creation prompt for reusable capability creation intent")
	}
	if !shouldInjectCapabilityCreationPrompt("weiter", []string{"create_skill_from_template"}, nil) {
		t.Fatal("expected capability creation prompt after recent skill template usage")
	}

	if shouldInjectDaemonSkillsPrompt("analysiere den log", nil, nil) {
		t.Fatal("did not expect daemon skills prompt for ordinary log analysis")
	}
	if !shouldInjectDaemonSkillsPrompt("erstelle einen background watcher daemon", nil, nil) {
		t.Fatal("expected daemon skills prompt for daemon watcher intent")
	}
	if !shouldInjectDaemonSkillsPrompt("status", []string{"manage_daemon"}, nil) {
		t.Fatal("expected daemon skills prompt after recent daemon tool usage")
	}

	flags := &prompts.ContextFlags{InternetExposed: true}
	if shouldInjectInternetExposureWarning("bewerte den prompt", nil, nil, flags) {
		t.Fatal("did not expect internet warning for ordinary prompt review")
	}
	if !shouldInjectInternetExposureWarning("deploy die homepage im caddy container", nil, nil, flags) {
		t.Fatal("expected internet warning for deployment/network intent")
	}
	if !shouldInjectInternetExposureWarning("weiter", []string{"network_scan"}, nil, flags) {
		t.Fatal("expected internet warning after recent network tool usage")
	}
	if !shouldInjectInternetExposureWarning("weiter", nil, nil, &prompts.ContextFlags{
		InternetExposed:   true,
		ActiveNativeTools: []string{"docker"},
	}) {
		t.Fatal("expected internet warning when visible native tools include network/deployment tools")
	}
}

func TestRuntimePromptContextIgnoresReasoningForIntentGates(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "bitte normal pruefen"},
		{Role: openai.ChatMessageRoleAssistant, ReasoningContent: "maybe create a daemon skill and expose public https"},
	}
	userText := collectRecentUserIntentText(messages, 4, 800)
	if strings.Contains(userText, "daemon") || strings.Contains(userText, "https") {
		t.Fatalf("recent user intent leaked reasoning content: %q", userText)
	}
	if shouldInjectDaemonSkillsPrompt(userText, nil, nil) {
		t.Fatal("reasoning-only daemon text must not trigger daemon prompt")
	}
	if shouldInjectInternetExposureWarning(userText, nil, nil, &prompts.ContextFlags{InternetExposed: true}) {
		t.Fatal("reasoning-only https text must not trigger internet warning")
	}
}

func TestApplyRuntimePromptContextPolicySetsIntentFlagsAndGatesInternetWarning(t *testing.T) {
	flags := &prompts.ContextFlags{
		InternetExposed: true,
	}
	applyRuntimePromptContextPolicy(flags, runtimePromptContextOptions{
		UserText: "erstelle einen Python Skill",
	})
	if !flags.CapabilityCreationIntent {
		t.Fatal("expected capability creation intent flag")
	}
	if flags.InternetExposed {
		t.Fatal("ordinary skill creation should clear internet exposure warning")
	}

	flags = &prompts.ContextFlags{InternetExposed: true}
	applyRuntimePromptContextPolicy(flags, runtimePromptContextOptions{
		UserText: "deploy die homepage im docker container",
	})
	if !flags.InternetExposed {
		t.Fatal("network/deployment intent should keep internet exposure warning")
	}
}

func TestApplyRuntimePromptContextBudgetsCapsOperationalIssueReminderOnly(t *testing.T) {
	originalTaskRules := "## Heavy Task Rule\n" + strings.Repeat("rule detail ", 200)
	flags := &prompts.ContextFlags{
		OperationalIssueReminder: strings.Repeat("issue ", 300),
		TaskRules:                originalTaskRules,
	}

	applyRuntimePromptContextBudgets(flags)

	if len([]rune(flags.OperationalIssueReminder)) > 601 {
		t.Fatalf("OperationalIssueReminder length = %d, want <= 601", len([]rune(flags.OperationalIssueReminder)))
	}
	// TaskRules are truncated once by compactTaskRulesForPrompt in the prompt builder,
	// not here in the runtime budgets.
	if flags.TaskRules != originalTaskRules {
		t.Fatalf("TaskRules should not be truncated by applyRuntimePromptContextBudgets")
	}
}

func TestAssembleSortedStreamToolCallsHandlesSparseIndices(t *testing.T) {
	streamToolCalls := map[int]*openai.ToolCall{}
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(7),
		ID:    "call-7",
		Function: openai.FunctionCall{
			Name:      "filesystem",
			Arguments: `{"operation":"stat"}`,
		},
	})
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(2),
		ID:    "call-2",
		Function: openai.FunctionCall{
			Name:      "execute_shell",
			Arguments: `{"command":"pwd"}`,
		},
	})

	assembled := assembleSortedStreamToolCalls(streamToolCalls)
	if len(assembled) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(assembled))
	}
	if assembled[0].ID != "call-2" || assembled[1].ID != "call-7" {
		t.Fatalf("unexpected assembly order: %q, %q", assembled[0].ID, assembled[1].ID)
	}
}

func TestMergeStreamToolCallChunkAppendsFragments(t *testing.T) {
	streamToolCalls := map[int]*openai.ToolCall{}
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(1),
		ID:    "call-1",
		Function: openai.FunctionCall{
			Name:      "exec",
			Arguments: `{"comm`,
		},
	})
	mergeStreamToolCallChunk(streamToolCalls, openai.ToolCall{
		Index: intPtr(1),
		Function: openai.FunctionCall{
			Name:      "ute_shell",
			Arguments: `and":"pwd"}`,
		},
	})

	assembled := assembleSortedStreamToolCalls(streamToolCalls)
	if len(assembled) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assembled))
	}
	if assembled[0].Function.Name != "execute_shell" {
		t.Fatalf("Function.Name = %q, want %q", assembled[0].Function.Name, "execute_shell")
	}
	if assembled[0].Function.Arguments != `{"command":"pwd"}` {
		t.Fatalf("Function.Arguments = %q", assembled[0].Function.Arguments)
	}
}

type fakeGuideSearcher struct {
	paths []string
	err   error
	delay time.Duration
}

func (f fakeGuideSearcher) SearchToolGuides(query string, topK int) ([]string, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.paths, f.err
}

// makeTool is a test helper that builds a minimal openai.Tool with the given function name.
func makeTool(name string) openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: name,
		},
	}
}

func makeSizedTool(name, description string) openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// toolNames extracts function names from a tool slice.
func toolNames(tools []openai.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if t.Function != nil {
			names = append(names, t.Function.Name)
		}
	}
	return names
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func TestFilterToolSchemas_AlwaysIncludeKept(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
		makeTool("rarely_used"),
	}
	result := filterToolSchemas(schemas, []string{}, []string{"filesystem"}, 10, nil)
	names := toolNames(result)
	if !containsName(names, "filesystem") {
		t.Error("alwaysInclude tool 'filesystem' should always be kept")
	}
}

func TestFilterToolSchemas_FrequentToolKept(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("docker"),
		makeTool("never_used"),
	}
	result := filterToolSchemas(schemas, []string{"docker"}, []string{}, 10, nil)
	names := toolNames(result)
	if !containsName(names, "docker") {
		t.Error("frequent tool 'docker' should be kept")
	}
}

func TestFilterToolSchemas_DynamicShortcutPrefixesAreAdaptive(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("skill__backup"),
		makeTool("tool__my_custom"),
		makeTool("obscure_tool"),
	}
	result := filterToolSchemas(schemas, []string{"obscure_tool"}, []string{}, 1, nil)
	names := toolNames(result)
	if containsName(names, "skill__backup") {
		t.Error("skill__-prefixed tool should not bypass adaptive filtering")
	}
	if containsName(names, "tool__my_custom") {
		t.Error("tool__-prefixed tool should not bypass adaptive filtering")
	}
	if !containsName(names, "obscure_tool") {
		t.Error("preferred tool should be kept")
	}
}

func TestFilterToolSchemas_MaxToolsLimit(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("a"), makeTool("b"), makeTool("c"), makeTool("d"), makeTool("e"),
	}
	result := filterToolSchemas(schemas, []string{"a", "b", "c", "d", "e"}, []string{}, 3, nil)
	if len(result) > 3 {
		t.Errorf("expected at most 3 tools, got %d", len(result))
	}
}

func TestFilterToolSchemas_MaxToolsCapsAdaptiveOnly(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
		makeTool("homepage"),
		makeTool("uptime_kuma"),
		makeTool("adguard"),
	}

	result := filterToolSchemas(
		schemas,
		[]string{"docker", "homepage", "uptime_kuma"},
		[]string{"filesystem", "adguard"},
		2,
		nil,
	)

	names := toolNames(result)
	if len(names) != 4 {
		t.Fatalf("expected 2 always tools plus 2 adaptive tools, got %d: %v", len(names), names)
	}
	for _, want := range []string{"filesystem", "adguard", "docker", "homepage"} {
		if !containsName(names, want) {
			t.Fatalf("expected %q in result, got %v", want, names)
		}
	}
	if containsName(names, "uptime_kuma") {
		t.Fatalf("expected uptime_kuma to be dropped by adaptive cap, got %v", names)
	}
}

func TestFilterToolSchemas_RegularChatKeepsCoreToolsAndCapsAdaptive(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("discover_tools"),
		makeTool("invoke_tool"),
		makeTool("filesystem"),
		makeTool("query_memory"),
		makeTool("manage_memory"),
		makeTool("docker"),
		makeTool("api_request"),
		makeTool("uptime_kuma"),
		makeTool("adguard"),
		makeTool("homepage"),
	}

	result := filterToolSchemas(
		schemas,
		[]string{"docker", "api_request", "uptime_kuma", "adguard", "homepage"},
		[]string{"discover_tools", "invoke_tool", "filesystem", "query_memory", "manage_memory"},
		3,
		nil,
	)

	names := toolNames(result)
	for _, want := range []string{"discover_tools", "invoke_tool", "filesystem", "query_memory", "manage_memory", "docker", "api_request", "uptime_kuma"} {
		if !containsName(names, want) {
			t.Fatalf("expected regular chat tool %q in result, got %v", want, names)
		}
	}
	if containsName(names, "homepage") {
		t.Fatalf("expected homepage to be dropped by regular-chat adaptive cap, got %v", names)
	}
}

func TestFilterToolSchemas_MaxToolsZeroDisablesLimit(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("a"), makeTool("b"), makeTool("c"),
	}
	// maxTools=0 → no limit; all frequent tools are kept
	result := filterToolSchemas(schemas, []string{"a", "b", "c"}, []string{}, 0, nil)
	if len(result) != 3 {
		t.Errorf("expected all 3 tools with maxTools=0, got %d", len(result))
	}
}

func TestFilterToolSchemas_EmptyFrequentFallsBackToDropped(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("x"), makeTool("y"),
	}
	// No frequent tools, no alwaysInclude, maxTools=5 → remaining slots filled from dropped list
	result := filterToolSchemas(schemas, []string{}, []string{}, 5, nil)
	// Both tools land in 'dropped', then are added via remaining-slots fill-up
	if len(result) != 2 {
		t.Errorf("expected 2 tools from fill-up, got %d", len(result))
	}
}

func TestToolResultFollowUpContent_TTSSuccessInVoiceModeGetsLoopGuard(t *testing.T) {
	tc := ToolCall{Action: "tts"}
	result := `Tool Output: {"status":"success","file":"hello.mp3"}`

	got := toolResultFollowUpContent(tc, result, true)

	if got == result {
		t.Fatal("expected TTS success in voice mode to be replaced with a loop guard note")
	}
	if !strings.Contains(got, "Do not call `tts` again") {
		t.Fatalf("expected loop guard note to warn against repeated TTS calls, got %q", got)
	}
}

func TestToolResultFollowUpContent_TTSErrorKeepsRawOutput(t *testing.T) {
	tc := ToolCall{Action: "tts"}
	result := `Tool Output: {"status":"error","message":"boom"}`

	got := toolResultFollowUpContent(tc, result, true)

	if got != result {
		t.Fatalf("expected TTS error output to pass through unchanged, got %q", got)
	}
}

func TestToolResultFollowUpContent_TTSSuccessOutsideVoiceModeKeepsRawOutput(t *testing.T) {
	tc := ToolCall{Action: "tts"}
	result := `Tool Output: {"status":"success","file":"hello.mp3"}`

	got := toolResultFollowUpContent(tc, result, false)

	if got != result {
		t.Fatalf("expected non-voice TTS output to pass through unchanged, got %q", got)
	}
}

func TestFilterToolSchemas_AlwaysIncludeNotDuplicatedByFrequent(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
	}
	result := filterToolSchemas(schemas,
		[]string{"filesystem"}, // also in frequentTools
		[]string{"filesystem"}, // and in alwaysInclude
		10, nil)
	count := 0
	for _, t2 := range result {
		if t2.Function != nil && t2.Function.Name == "filesystem" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("'filesystem' should appear exactly once, got %d", count)
	}
}

func TestFilterToolSchemas_OriginalOrderPreservedForDropped(t *testing.T) {
	// Dropped tools are appended in original schema order
	schemas := []openai.Tool{
		makeTool("rare1"), makeTool("rare2"), makeTool("rare3"),
	}
	result := filterToolSchemas(schemas, []string{}, []string{}, 2, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].Function.Name != "rare1" || result[1].Function.Name != "rare2" {
		t.Errorf("expected original order rare1,rare2; got %s,%s",
			result[0].Function.Name, result[1].Function.Name)
	}
}

func TestFilterToolSchemas_PrioritizesPreferredOrder(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("docker"),
		makeTool("homepage"),
	}

	result := filterToolSchemas(schemas, []string{"homepage", "docker"}, nil, 2, nil)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(names))
	}
	if names[0] != "homepage" || names[1] != "docker" {
		t.Fatalf("expected prioritized order homepage,docker got %v", names)
	}
}

func TestFilterToolSchemasWithReport_MaxTotalCapsSoftAndAdaptive(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("discover_tools"),
		makeTool("invoke_tool"),
		makeTool("filesystem"),
		makeTool("docker"),
		makeTool("homepage"),
		makeTool("uptime_kuma"),
	}

	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:   []string{"docker", "homepage", "uptime_kuma"},
		HardAlwaysTools:  []string{"discover_tools", "invoke_tool"},
		SoftAlwaysTools:  []string{"filesystem"},
		MaxAdaptiveTools: 3,
		MaxTotalTools:    4,
	}, nil)

	names := toolNames(result.Tools)
	for _, want := range []string{"discover_tools", "invoke_tool", "filesystem", "docker"} {
		if !containsName(names, want) {
			t.Fatalf("expected %q in result, got %v", want, names)
		}
	}
	if len(names) != 4 {
		t.Fatalf("expected 4 tools after total cap, got %d: %v", len(names), names)
	}
	if result.Report.KeptHardAlways != 2 || result.Report.KeptSoftAlways != 1 || result.Report.KeptAdaptive != 1 {
		t.Fatalf("unexpected report: %+v", result.Report)
	}
}

func TestFilterToolSchemasWithReport_HardAlwaysCanExceedTotalCap(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("discover_tools"),
		makeTool("invoke_tool"),
		makeTool("execute_skill"),
		makeTool("run_tool"),
		makeTool("filesystem"),
	}

	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		HardAlwaysTools: []string{"discover_tools", "invoke_tool", "execute_skill", "run_tool"},
		SoftAlwaysTools: []string{"filesystem"},
		MaxTotalTools:   2,
	}, nil)

	names := toolNames(result.Tools)
	if len(names) != 4 {
		t.Fatalf("expected hard-always tools to exceed total cap, got %d: %v", len(names), names)
	}
	if containsName(names, "filesystem") {
		t.Fatalf("expected soft tool to be trimmed before hard tools, got %v", names)
	}
	if result.Report.HardAlwaysExceededTotalCap != true {
		t.Fatalf("expected hard-always cap warning, got %+v", result.Report)
	}
}

func TestFilterToolSchemasWithReport_RegularSessionRespectsTotalCap(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("discover_tools"),
		makeTool("invoke_tool"),
		makeTool("filesystem"),
		makeTool("query_memory"),
		makeTool("manage_memory"),
		makeTool("docker"),
		makeTool("api_request"),
		makeTool("uptime_kuma"),
	}

	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:   []string{"docker", "api_request", "uptime_kuma"},
		HardAlwaysTools:  []string{"discover_tools", "invoke_tool"},
		SoftAlwaysTools:  []string{"filesystem", "query_memory", "manage_memory"},
		MaxAdaptiveTools: 3,
		MaxTotalTools:    6,
	}, nil)

	names := toolNames(result.Tools)
	if len(names) != 6 {
		t.Fatalf("expected 6 tools after regular-session total cap, got %d: %v", len(names), names)
	}
	for _, want := range []string{"discover_tools", "invoke_tool", "filesystem", "query_memory", "manage_memory", "docker"} {
		if !containsName(names, want) {
			t.Fatalf("expected %q in result, got %v", want, names)
		}
	}
}

func TestFilterToolSchemasWithReport_HardToolsSurviveSchemaTokenBudget(t *testing.T) {
	schemas := []openai.Tool{
		makeSizedTool("discover_tools", strings.Repeat("hard ", 300)),
		makeSizedTool("small_tool", "small focused helper"),
	}

	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:   []string{"small_tool"},
		HardAlwaysTools:  []string{"discover_tools"},
		MaxAdaptiveTools: 1,
		MaxSchemaTokens:  1,
		MaxTotalTools:    2,
	}, nil)

	names := toolNames(result.Tools)
	if !containsName(names, "discover_tools") {
		t.Fatalf("hard tool was dropped under token budget: %v", names)
	}
	if result.Report.HardAlwaysExceededTokenCap != true {
		t.Fatalf("expected hard-token-cap warning, got %+v", result.Report)
	}
}

func TestFilterToolSchemasWithReport_TokenBudgetPrefersSmallerAdaptiveTool(t *testing.T) {
	schemas := []openai.Tool{
		makeSizedTool("discover_tools", "hard catalog"),
		makeSizedTool("large_tool", strings.Repeat("large schema text ", 300)),
		makeSizedTool("small_tool", "small schema"),
	}

	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:   []string{"large_tool", "small_tool"},
		HardAlwaysTools:  []string{"discover_tools"},
		MaxAdaptiveTools: 2,
		MaxSchemaTokens:  120,
		MaxTotalTools:    3,
	}, nil)

	names := toolNames(result.Tools)
	if !containsName(names, "small_tool") {
		t.Fatalf("expected small adaptive tool under token budget, got %v", names)
	}
	if containsName(names, "large_tool") {
		t.Fatalf("expected large adaptive tool to lose to smaller tool, got %v", names)
	}
}

func TestFilterToolSchemasWithReport_ActivatedSoftToolPrecedesAdaptiveUnderTotalCap(t *testing.T) {
	schemas := []openai.Tool{
		makeSizedTool("discover_tools", "hard catalog"),
		makeSizedTool("activated_tool", strings.Repeat("activated ", 150)),
		makeSizedTool("small_adaptive", "small adaptive"),
	}

	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:   []string{"small_adaptive"},
		HardAlwaysTools:  []string{"discover_tools"},
		SoftAlwaysTools:  []string{"activated_tool"},
		MaxAdaptiveTools: 1,
		MaxTotalTools:    2,
	}, nil)

	names := toolNames(result.Tools)
	if !containsName(names, "activated_tool") {
		t.Fatalf("expected activated soft tool to be kept, got %v", names)
	}
	if containsName(names, "small_adaptive") {
		t.Fatalf("expected adaptive tool to be dropped after activated soft tool consumed cap, got %v", names)
	}
}

func TestRecentNativeToolNamesFromMessagesKeepsRecentCalls(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "old"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
			ID:       "call-old",
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: "homepage", Arguments: `{}`},
		}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-old", Content: "ok"},
		{Role: openai.ChatMessageRoleUser, Content: "new"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
			ID:       "call-new",
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: "docker", Arguments: `{}`},
		}}},
	}

	names := recentNativeToolNamesFromMessages(msgs, 1)
	if len(names) != 1 || names[0] != "docker" {
		t.Fatalf("names = %v, want [docker]", names)
	}
}

func TestExtractIntentMatchedToolsMatchesSplitToolNames(t *testing.T) {
	matches := extractIntentMatchedTools("please scan the network and then inspect devices", []string{"network_scan", "notes", "homepage"})
	if len(matches) == 0 || matches[0] != "network_scan" {
		t.Fatalf("expected network_scan intent match, got %v", matches)
	}
}

func TestAdaptiveFamilySeedsForQueryIncludesDocumentToolsForPDFRequests(t *testing.T) {
	seeds := adaptiveFamilySeedsForQuery("such mal die neuesten ki news und erstelle eine pdf")
	if !containsName(seeds, "pdf_operations") {
		t.Fatalf("expected pdf_operations family seed, got %v", seeds)
	}
	if !containsName(seeds, "document_creator") {
		t.Fatalf("expected document_creator family seed, got %v", seeds)
	}
}

func TestAdaptiveFamilySeedsForQueryIncludesResourceAndContainerTools(t *testing.T) {
	seeds := adaptiveFamilySeedsForQuery("wieviel ram verbraucht aurago und wieviel die container")
	for _, want := range []string{"system_metrics", "process_analyzer", "docker", "execute_shell"} {
		if !containsName(seeds, want) {
			t.Fatalf("expected %s for resource/container query, got %v", want, seeds)
		}
	}
}

func TestAdaptiveFamilySeedsForQueryIncludesGo2RTCForNetworkCameras(t *testing.T) {
	queries := []string{
		"mache einen snapshot der netzwerkkamera",
		"show the ONVIF camera live stream",
		"öffne das livebild der IP-Kamera",
	}
	for _, query := range queries {
		if seeds := adaptiveFamilySeedsForQuery(query); !containsName(seeds, "go2rtc") {
			t.Fatalf("expected go2rtc for camera query %q, got %v", query, seeds)
		}
	}
	if seeds := adaptiveFamilySeedsForQuery("erstelle einen snapshot der webseite"); containsName(seeds, "go2rtc") {
		t.Fatalf("generic snapshot request must not select go2rtc, got %v", seeds)
	}
}

func TestAdaptiveFamilySeedsForQueryIncludesBalancedProgressiveTools(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "web search and API request",
			query: "search the web and call an api endpoint",
			want:  []string{"ddg_search", "api_request"},
		},
		{
			name:  "mcp integration",
			query: "use the MCP server integration",
			want:  []string{"mcp_call"},
		},
		{
			name:  "composio integration",
			query: "use composio integrations",
			want:  []string{"composio_call"},
		},
		{
			name:  "composio gmail service alias",
			query: "prüfe gmail",
			want:  []string{"composio_call"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seeds := adaptiveFamilySeedsForQuery(tt.query)
			for _, want := range tt.want {
				if !containsName(seeds, want) {
					t.Fatalf("expected %s for query %q, got %v", want, tt.query, seeds)
				}
			}
		})
	}
}

func TestBuildAdaptiveToolPriorityKeepsBalancedProgressiveToolsReachable(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("file_editor"),
		makeTool("execute_python"),
		makeTool("docker"),
		makeTool("api_request"),
		makeTool("ddg_search"),
		makeTool("manage_missions"),
	}

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "file editing", query: "edit this json file", want: "file_editor"},
		{name: "python", query: "write a python script", want: "execute_python"},
		{name: "docker", query: "inspect the docker containers", want: "docker"},
		{name: "api", query: "call this api endpoint", want: "api_request"},
		{name: "search", query: "search the web for recent news", want: "ddg_search"},
		{name: "missions", query: "schedule a mission for tomorrow", want: "manage_missions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAdaptiveToolPriority(schemas, nil, tt.query, nil, nil)
			if !containsName(got, tt.want) {
				t.Fatalf("expected %s to be reachable for query %q, got %v", tt.want, tt.query, got)
			}
		})
	}
}

func TestCacheAwareAdaptiveAlwaysIncludeAddsIntentFamilyBundle(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("execute_shell"),
		makeTool("generate_music"),
		makeTool("generate_video"),
	}
	got := cacheAwareAdaptiveAlwaysInclude("generate a short video with music", []string{"execute_shell"}, schemas)
	if !containsName(got, "execute_shell") {
		t.Fatalf("expected existing always-include tool to remain, got %v", got)
	}
	if !containsName(got, "generate_video") {
		t.Fatalf("expected media family bundle to include generate_video, got %v", got)
	}
	if !containsName(got, "generate_music") {
		t.Fatalf("expected media family bundle to include generate_music, got %v", got)
	}
}

func TestCacheAwareAdaptiveAlwaysIncludeSkipsUnavailableFamilySeeds(t *testing.T) {
	got := cacheAwareAdaptiveAlwaysInclude("generate a short video with music", nil, []openai.Tool{
		makeTool("generate_video"),
	})
	if !containsName(got, "generate_video") {
		t.Fatalf("expected available media seed to be included, got %v", got)
	}
	if containsName(got, "generate_music") {
		t.Fatalf("did not expect unavailable media seed to be included, got %v", got)
	}
}

func TestBuildAdaptiveToolPriorityUsesSemanticAndFamilySignals(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("document_creator"),
		makeTool("pdf_operations"),
		makeTool("send_document"),
		makeTool("tts"),
	}

	got := buildAdaptiveToolPriority(
		schemas,
		nil,
		"erstelle bitte eine pdf aus den neuesten ki news",
		fakeGuideSearcher{paths: []string{
			filepath.Join("prompts", "tools_manuals", "document_creator.md"),
		}},
		nil,
	)

	if !containsName(got, "document_creator") {
		t.Fatalf("expected semantic priority to include document_creator, got %v", got)
	}
	if !containsName(got, "pdf_operations") {
		t.Fatalf("expected family priority to include pdf_operations, got %v", got)
	}
}

func TestSearchToolGuidesWithTimeoutFallsBackQuickly(t *testing.T) {
	start := time.Now()
	got := searchToolGuidesWithTimeout(
		fakeGuideSearcher{
			paths: []string{filepath.Join("tools_manuals", "document_creator.md")},
			delay: 100 * time.Millisecond,
		},
		"create a document",
		4,
		10*time.Millisecond,
		nil,
	)

	if len(got) != 0 {
		t.Fatalf("expected timeout fallback to return no semantic paths, got %v", got)
	}
	if elapsed := time.Since(start); elapsed > 80*time.Millisecond {
		t.Fatalf("semantic guide timeout waited too long: %s", elapsed)
	}
}

func TestHardAlwaysToolNamesUsesBalancedKernel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "configured"

	got := hardAlwaysToolNames(cfg)
	want := []string{"activate_tools", "discover_tools", "execute_skill", "invoke_tool", "run_tool"}
	if len(got) != len(want) {
		t.Fatalf("hard always tools = %v, want %v", got, want)
	}
	for _, name := range want {
		if !containsName(got, name) {
			t.Fatalf("hard always tools missing %q: %v", name, got)
		}
	}
	for _, notWant := range []string{"list_agent_skills", "activate_agent_skill", "run_agent_skill_script", "mcp_call", "composio_call"} {
		if containsName(got, notWant) {
			t.Fatalf("did not expect %q in hard always tools: %v", notWant, got)
		}
	}
}

func TestExpandAdaptiveAlwaysIncludeAlwaysKeepsDiscoverTools(t *testing.T) {
	cfg := &config.Config{}
	got := expandAdaptiveAlwaysInclude(cfg, []string{"filesystem"})
	if !containsName(got, "discover_tools") {
		t.Fatalf("expected discover_tools in always-include set, got %v", got)
	}
}

func TestExpandAdaptiveAlwaysIncludeAddsInvasionControlWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.InvasionControl.Enabled = true

	got := expandAdaptiveAlwaysInclude(cfg, []string{"filesystem"})
	if containsName(got, "invasion_control") {
		t.Fatalf("did not expect invasion_control to be always visible just because it is enabled, got %v", got)
	}
}

func TestExpandAdaptiveAlwaysIncludeMapsLegacyShellAlias(t *testing.T) {
	cfg := &config.Config{}
	got := expandAdaptiveAlwaysInclude(cfg, []string{"filesystem", "shell"})
	if !containsName(got, "execute_shell") {
		t.Fatalf("expected shell alias to expand to execute_shell, got %v", got)
	}
	if containsName(got, "shell") {
		t.Fatalf("did not expect legacy shell alias to remain in always-include set, got %v", got)
	}
}

func TestExpandAdaptiveAlwaysIncludeSkipsMCPCallWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	got := expandAdaptiveAlwaysInclude(cfg, []string{"filesystem"})
	if containsName(got, "mcp_call") {
		t.Fatalf("did not expect mcp_call in always-include set, got %v", got)
	}
}

func TestExpandAdaptiveAlwaysIncludeSkipsMCPAndComposioEvenWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "configured"

	got := expandAdaptiveAlwaysInclude(cfg, []string{"filesystem"})
	for _, notWant := range []string{"mcp_call", "composio_call"} {
		if containsName(got, notWant) {
			t.Fatalf("did not expect %s in always-include set, got %v", notWant, got)
		}
	}
}

func TestChannelAdaptiveAlwaysIncludeKeepsVirtualDesktopForDesktopChat(t *testing.T) {
	got := channelAdaptiveAlwaysInclude(
		RunConfig{MessageSource: "virtual_desktop_chat"},
		[]string{"filesystem"},
		ToolFeatureFlags{VirtualDesktopEnabled: true, OfficeDocumentEnabled: true, OfficeWorkbookEnabled: true},
	)
	for _, want := range []string{"virtual_desktop_files", "virtual_desktop_apps", "virtual_desktop_widgets", "office_document", "office_workbook", "question_user"} {
		if !containsName(got, want) {
			t.Fatalf("expected desktop chat always-include to contain %q, got %v", want, got)
		}
	}
}

func TestChannelAdaptiveAlwaysIncludeDoesNotAdvertiseDisabledDesktopTools(t *testing.T) {
	got := channelAdaptiveAlwaysInclude(
		RunConfig{MessageSource: "virtual_desktop_chat"},
		nil,
		ToolFeatureFlags{},
	)
	for _, notWant := range []string{"virtual_desktop_files", "virtual_desktop_apps", "virtual_desktop_widgets", "office_document", "office_workbook"} {
		if containsName(got, notWant) {
			t.Fatalf("did not expect disabled desktop tool %q in always-include set, got %v", notWant, got)
		}
	}
	if !containsName(got, "question_user") {
		t.Fatalf("expected desktop chat to keep question_user even when optional desktop tools are disabled, got %v", got)
	}
}

func TestChannelAdaptiveAlwaysIncludeRoutesHomepageStudioToHomepageTools(t *testing.T) {
	got := channelAdaptiveAlwaysInclude(
		RunConfig{MessageSource: "homepage_studio"},
		[]string{"filesystem"},
		ToolFeatureFlags{VirtualDesktopEnabled: true, OfficeDocumentEnabled: true, OfficeWorkbookEnabled: true},
	)
	for _, want := range []string{"homepage_project", "homepage_file", "homepage_quality", "homepage_deploy", "homepage_git", "homepage_registry"} {
		if !containsName(got, want) {
			t.Fatalf("expected Homepage Studio always-include to contain %q, got %v", want, got)
		}
	}
	for _, notWant := range []string{"virtual_desktop_files", "virtual_desktop_apps", "virtual_desktop_widgets", "office_document", "office_workbook"} {
		if containsName(got, notWant) {
			t.Fatalf("did not expect desktop tool %q in Homepage Studio always-include set, got %v", notWant, got)
		}
	}
}

func TestOutputRefAdaptiveAlwaysIncludeAddsReadToolOutput(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleTool, Content: `Tool Output: {"status":"success","output_ref":"toolout_abc123"}`},
	}

	got := outputRefAdaptiveAlwaysInclude(messages, []string{"filesystem"})
	if !containsName(got, "filesystem") {
		t.Fatalf("expected existing always-include tool to remain, got %v", got)
	}
	if !containsName(got, "read_tool_output") {
		t.Fatalf("expected read_tool_output for output_ref context, got %v", got)
	}

	got = outputRefAdaptiveAlwaysInclude(messages, []string{"read_tool_output"})
	count := 0
	for _, name := range got {
		if name == "read_tool_output" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected read_tool_output once, got %v", got)
	}
}

func TestOutputRefAdaptiveAlwaysIncludeIgnoresMessagesWithoutRefs(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "please continue"},
		{Role: openai.ChatMessageRoleAssistant, Content: "ok"},
	}

	got := outputRefAdaptiveAlwaysInclude(messages, []string{"filesystem"})
	if containsName(got, "read_tool_output") {
		t.Fatalf("did not expect read_tool_output without output refs, got %v", got)
	}
}

func TestCollectRecentUserIntentTextKeepsRecentUserContext(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "erste frage"},
		{Role: openai.ChatMessageRoleAssistant, Content: "antwort"},
		{Role: openai.ChatMessageRoleUser, Content: "hol dir von koofr einen song"},
		{Role: openai.ChatMessageRoleAssistant, Content: "ok"},
		{Role: openai.ChatMessageRoleUser, Content: "und spiele ihn auf google home mini ab"},
		{Role: openai.ChatMessageRoleUser, Content: "bugs gefixed, probier nochmal"},
	}

	got := collectRecentUserIntentText(messages, 3, 200)

	if !strings.Contains(got, "hol dir von koofr einen song") {
		t.Fatalf("expected older user intent to remain, got %q", got)
	}
	if !strings.Contains(got, "google home mini") {
		t.Fatalf("expected playback context to remain, got %q", got)
	}
	if !strings.Contains(got, "bugs gefixed, probier nochmal") {
		t.Fatalf("expected latest retry message to remain, got %q", got)
	}
}

func TestBuildAdaptiveToolPriorityUsesCombinedConversationIntent(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("chromecast"),
		makeTool("koofr"),
		makeTool("filesystem"),
	}

	prioritized := buildAdaptiveToolPriority(
		schemas,
		[]string{"filesystem"},
		"hol dir von koofr einen song und spiele ihn auf google home mini ab\nbugs gefixed, probier nochmal",
		fakeGuideSearcher{paths: []string{filepath.Join("tools_manuals", "chromecast.md")}},
		nil,
	)

	if !containsName(prioritized, "koofr") {
		t.Fatalf("expected koofr in prioritized tools, got %v", prioritized)
	}
	if !containsName(prioritized, "chromecast") {
		t.Fatalf("expected chromecast in prioritized tools, got %v", prioritized)
	}
}

func TestBuildAdaptiveToolPriorityPrefersIntentAndSemanticHits(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("shell"),
		makeTool("docker"),
		makeTool("homepage_deploy"),
		makeTool("netlify"),
	}

	prioritized := buildAdaptiveToolPriority(
		schemas,
		[]string{"shell", "docker"},
		"please deploy the homepage to netlify",
		fakeGuideSearcher{paths: []string{filepath.Join("tools_manuals", "homepage_deploy.md")}},
		nil,
	)

	if len(prioritized) < 3 {
		t.Fatalf("expected at least 3 prioritized tools, got %v", prioritized)
	}
	if prioritized[0] != "homepage_deploy" {
		t.Fatalf("expected homepage_deploy to be prioritized first, got %v", prioritized)
	}
	if !containsName(prioritized, "netlify") {
		t.Fatalf("expected netlify to be included from direct intent match, got %v", prioritized)
	}
	if !containsName(prioritized, "shell") {
		t.Fatalf("expected weighted usage fallback to remain, got %v", prioritized)
	}
}

func TestBuildAdaptiveToolPriorityAddsCuratedDependencyNeighbors(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("homepage_deploy"),
		makeTool("homepage_project"),
		makeTool("netlify"),
		makeTool("homepage_registry"),
		makeTool("filesystem"),
		makeTool("shell"),
	}

	prioritized := buildAdaptiveToolPriority(
		schemas,
		[]string{"shell"},
		"please deploy the homepage",
		nil,
		nil,
	)

	if len(prioritized) < 3 {
		t.Fatalf("expected at least 3 prioritized tools, got %v", prioritized)
	}
	if prioritized[0] != "homepage_deploy" {
		t.Fatalf("expected homepage_deploy first, got %v", prioritized)
	}
	if !containsName(prioritized, "netlify") {
		t.Fatalf("expected curated homepage neighbor netlify, got %v", prioritized)
	}
	if !containsName(prioritized, "homepage_registry") {
		t.Fatalf("expected curated homepage neighbor homepage_registry, got %v", prioritized)
	}
}

func TestBuildAdaptiveToolPriorityPrefersHomepageForGermanWebsiteDeploy(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("homepage_deploy"),
		makeTool("homepage_project"),
		makeTool("homepage_registry"),
		makeTool("netlify"),
		makeTool("shell"),
	}

	prioritized := buildAdaptiveToolPriority(
		schemas,
		[]string{"filesystem", "shell"},
		"aktualisiere die Webseite und veröffentliche sie",
		nil,
		nil,
	)

	if len(prioritized) == 0 {
		t.Fatal("expected homepage-oriented priority for German website deployment intent")
	}
	if prioritized[0] != "homepage_deploy" {
		t.Fatalf("expected homepage_deploy first for German website deployment intent, got %v", prioritized)
	}
	if !containsName(prioritized, "homepage_registry") {
		t.Fatalf("expected homepage_registry to be included for website deployment intent, got %v", prioritized)
	}
}

func TestCacheAwareAdaptiveAlwaysIncludeKeepsHomepageForGermanWebsiteDeploy(t *testing.T) {
	schemas := []openai.Tool{
		makeTool("filesystem"),
		makeTool("homepage_deploy"),
		makeTool("homepage_project"),
		makeTool("homepage_registry"),
		makeTool("netlify"),
		makeTool("shell"),
	}

	alwaysInclude := cacheAwareAdaptiveAlwaysInclude(
		"aktualisiere die Webseite und veröffentliche sie",
		[]string{"query_memory"},
		schemas,
	)

	for _, want := range []string{"homepage_deploy", "homepage_registry", "netlify"} {
		if !containsName(alwaysInclude, want) {
			t.Fatalf("expected %s to stay always-included for website deployment intent, got %v", want, alwaysInclude)
		}
	}

	filtered := filterToolSchemas(schemas, []string{"shell"}, alwaysInclude, 1, nil)
	if !containsName(toolNames(filtered), "homepage_deploy") {
		t.Fatalf("expected filtered schemas to keep homepage_deploy even with maxTools=1, got %v", toolNames(filtered))
	}
}

func TestStreamingAccountingState_RecordsProviderUsage(t *testing.T) {
	st := streamingAccountingState{}
	if st.hasProviderUsage {
		t.Fatal("expected hasProviderUsage=false initially")
	}
	st.recordProviderUsage(100, 50, 25)
	if !st.hasProviderUsage {
		t.Error("expected hasProviderUsage=true after recordProviderUsage")
	}
	if st.providerPrompt != 100 || st.providerCompletion != 50 || st.providerCached != 25 {
		t.Errorf("providerPrompt=%d providerCompletion=%d providerCached=%d, want 100, 50, 25", st.providerPrompt, st.providerCompletion, st.providerCached)
	}
}

func TestStreamingAccountingState_MergesNonZeroValuesOnMultipleRecords(t *testing.T) {
	st := streamingAccountingState{}
	// First chunk: prompt only
	st.recordProviderUsage(100, 0, 0)
	// Second chunk: completion only
	st.recordProviderUsage(0, 75, 0)
	if st.providerPrompt != 100 || st.providerCompletion != 75 {
		t.Errorf("expected merged record (100, 75), got (%d, %d)", st.providerPrompt, st.providerCompletion)
	}
	// Third chunk: both values updated
	st.recordProviderUsage(200, 80, 60)
	if st.providerPrompt != 200 || st.providerCompletion != 80 || st.providerCached != 60 {
		t.Errorf("expected last record (200, 80, 60), got (%d, %d, %d)", st.providerPrompt, st.providerCompletion, st.providerCached)
	}
}

func TestStreamingAccountingState_FinalizedDefaultsFalse(t *testing.T) {
	st := streamingAccountingState{}
	if st.finalized {
		t.Error("expected finalized=false initially")
	}
}

func TestShouldReloadCoreMemory_TTLExpired(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base.Add(6 * time.Minute) }

	loadedAt := base
	dbUpdatedAt := base
	cachedUpdatedAt := base

	if !ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected reload when TTL expired")
	}
}

func TestShouldReloadCoreMemory_TTLNotExpired(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base.Add(2 * time.Minute) }

	loadedAt := base
	dbUpdatedAt := base
	cachedUpdatedAt := base

	if ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected no reload when TTL not expired")
	}
}

func TestShouldReloadCoreMemory_VersionChanged(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base }

	loadedAt := base
	dbUpdatedAt := base.Add(1 * time.Minute)
	cachedUpdatedAt := base

	if !ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected reload when version changed")
	}
}

func TestShouldReloadCoreMemory_DBEmptied(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base }

	loadedAt := base
	dbUpdatedAt := time.Time{}
	cachedUpdatedAt := base

	if !ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected reload when core memory was externally emptied")
	}
}

func TestShouldReloadCoreMemory_DirtyFlag(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base }

	loadedAt := base
	dbUpdatedAt := base
	cachedUpdatedAt := base

	if !ShouldReloadCoreMemory(true, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected reload when dirty flag is set")
	}
}

func TestShouldReloadCoreMemory_NotYetLoaded(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base.Add(10 * time.Minute) }

	loadedAt := time.Time{}
	dbUpdatedAt := base
	cachedUpdatedAt := time.Time{}

	if ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected no reload when not yet loaded (loadedAt is zero)")
	}
}

func TestShouldReloadCoreMemory_RepeatedLoopsNoChange(t *testing.T) {
	orig := nowFunc
	defer func() { nowFunc = orig }()

	base := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return base.Add(1 * time.Minute) }

	loadedAt := base
	dbUpdatedAt := base
	cachedUpdatedAt := base

	if ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected no reload in repeated loop with no changes")
	}

	nowFunc = func() time.Time { return base.Add(2 * time.Minute) }
	if ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected no reload in second repeated loop with no changes")
	}

	nowFunc = func() time.Time { return base.Add(4 * time.Minute) }
	if ShouldReloadCoreMemory(false, loadedAt, dbUpdatedAt, cachedUpdatedAt) {
		t.Error("expected no reload in third repeated loop with no changes (still within TTL)")
	}
}
