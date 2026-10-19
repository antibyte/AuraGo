package prompts

import (
	"aurago/internal/security"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRequiredTaskRulesRemainAtomicUnderBudgetPressure(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	rules := "First mandatory constraint.\n# TOOL GUIDES\n" + strings.Repeat("middle rule ", 400) + "\nNever delete durable data."
	text := "# CORE IDENTITY\nKeep core.\n# TASK RULES\n" + escapePromptBoundaryLines(compactTaskRulesForPrompt(rules)) + "\n# OPTIONAL FOLLOWUP\nend"
	result, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: text, Tokens: -1, TokenBudget: 32}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if result.BudgetExceeded == nil || !strings.Contains(result.Text, "Never delete durable data.") {
		t.Fatalf("required rules silently omitted: %+v", result)
	}
	for _, section := range splitPromptSections(text) {
		if strings.Contains(section.Text, "middle rule") && !section.Required {
			t.Fatal("task-rule headings made their body optional")
		}
	}
	for _, removed := range result.RemovedSections {
		if removed == promptSectionTaskRules {
			t.Fatal("task rules were shed")
		}
	}
}
func TestTrimRetrievedMemoryEntriesKeepsClosedIsolation(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	first := "first memory " + strings.Repeat("a", 100)
	last := "last memory " + strings.Repeat("z", 500)
	text := "# CORE\nrequired\n# RETRIEVED MEMORIES\nadvisory\n" + isolatePromptExternalData(first+"\n---\n"+last) + "\n\n# REQUIRED AFTER\nkeep this"
	initial := CountTokens(text)
	limit := initial - CountTokens(last) - CountTokens("\n---\n") + 1
	got, partial, full, _, err := trimRetrievedMemoriesLedgerContext(context.Background(), text, limit, initial, "gpt-4o")
	if err != nil || !partial || full {
		t.Fatalf("partial=%v full=%v err=%v", partial, full, err)
	}
	if strings.Count(got, "<external_data>") != 1 || strings.Count(got, "</external_data>") != 1 {
		t.Fatalf("unclosed boundary: %s", got)
	}
	if !strings.Contains(got, first) || strings.Contains(got, last) || strings.Index(got, "</external_data>") > strings.Index(got, "# REQUIRED AFTER") {
		t.Fatal("memory body or following boundary corrupted")
	}
}
func TestUnifiedMemoryCompactsSourceBeforeReisolating(t *testing.T) {
	body := security.IsolateExternalData("Useful memory " + strings.Repeat("detail ", 200))
	got, ok := appendBudgetedUnifiedMemorySection("# UNIFIED MEMORY CONTEXT", "Retrieved Memories", body, 700)
	if !ok || len(got) > 700 || strings.Count(got, "<external_data>") != 1 || strings.Count(got, "</external_data>") != 1 {
		t.Fatalf("bad compact wrapper: %s", got)
	}
	if strings.Contains(got, "&lt;external_data&gt;") {
		t.Fatal("old isolation wrapper consumed the snippet budget")
	}
}
func TestToolRankingTiesAreDeterministic(t *testing.T) {
	globalToolStats.mu.Lock()
	savedRecords := globalToolStats.records
	globalToolStats.records = []ToolUsageRecord{{Tool: "zeta"}, {Tool: "alpha"}}
	globalToolStats.mu.Unlock()
	adaptiveToolState.mu.Lock()
	savedEntries := adaptiveToolState.entries
	used := time.Now().Add(-48 * time.Hour)
	adaptiveToolState.entries = map[string]*ToolUsageEntry{"zeta": {ToolName: "zeta", TotalCount: 10, SuccessCount: 10, LastUsed: used}, "alpha": {ToolName: "alpha", TotalCount: 10, SuccessCount: 10, LastUsed: used}}
	adaptiveToolState.mu.Unlock()
	defer func() {
		globalToolStats.mu.Lock()
		globalToolStats.records = savedRecords
		globalToolStats.mu.Unlock()
		adaptiveToolState.mu.Lock()
		adaptiveToolState.entries = savedEntries
		adaptiveToolState.mu.Unlock()
	}()
	want := []string{"alpha", "zeta"}
	for range 30 {
		if got := GetFrequentTools(2); !reflect.DeepEqual(got, want) {
			t.Fatalf("frequency tie: %v", got)
		}
		if got := GetFrequentToolsWeighted(2, 7, true); !reflect.DeepEqual(got, want) {
			t.Fatalf("weighted tie: %v", got)
		}
		if got := GetAdaptiveToolScores(7, true); got[0].Tool != "alpha" {
			t.Fatalf("dashboard tie: %v", got)
		}
	}
}
