package agent

import (
	"aurago/internal/llm"
	"errors"
	"github.com/sashabaranov/go-openai"
	"strings"
	"testing"
)

func TestProjectedStrictSchemaBudgetShedsOptionalOrBlocksRequired(t *testing.T) {
	original := []openai.Tool{budgetTestTool("required", "Required tool"), budgetTestTool("optional", strings.Repeat("optional description ", 35))}
	// Optional fields become required/nullable in the Strict wire projection.
	for i := range original {
		original[i].Function.Parameters = map[string]interface{}{"type": "object", "properties": map[string]interface{}{"value": map[string]interface{}{"type": "string"}}, "required": []string{}}
	}
	snapshot := newNativeToolSchemaSnapshot(original)
	projected := strictSchemasForActiveTools(snapshot, original)
	cap := estimateToolSchemaListTokens(original)
	if estimateToolSchemaListTokens(projected) <= cap {
		t.Fatal("fixture does not exercise projection growth")
	}
	b := &RequestBudget{MaxSchemaTokens: cap, CompletionReserve: 100, SafetyMargin: 32, MinimumSystem: 100, Routes: []RequestRouteBudget{{Limits: llm.ModelLimits{ContextWindow: 64000, Route: llm.ModelRoute{Model: "fixture"}}}}}
	kept, dropped, err := b.shedOptionalTools(nil, projected, map[string]bool{"required": true}, "fixture", newTokenCountCache(8))
	if err != nil || len(kept) != 1 || len(dropped) != 1 || dropped[0] != "optional" {
		t.Fatalf("projected selection kept=%d dropped=%v err=%v", len(kept), dropped, err)
	}
	_, _, err = b.shedOptionalTools(nil, projected, map[string]bool{"required": true, "optional": true}, "fixture", newTokenCountCache(8))
	var exceeded *ToolSchemaBudgetExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("oversized required projection sent: %v", err)
	}
	if _, err = b.validate(nil, projected, newTokenCountCache(8)); !errors.As(err, &exceeded) {
		t.Fatalf("final wire check missed projected growth: %v", err)
	}
	if original[0].Function.Strict {
		t.Fatal("budget check mutated original schemas")
	}
}
