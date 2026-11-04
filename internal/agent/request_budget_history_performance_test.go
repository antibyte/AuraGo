package agent

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"

	openai "github.com/sashabaranov/go-openai"
)

func TestHistoryWorkingSetLargeHistoryWithSmallCache(t *testing.T) {
	budget, err := newRequestBudget(context.Background(), &config.Config{}, &budgetRouteClient{routes: []llm.ModelRoute{
		{Model: "agnes-3.0-flash", ContextWindowOverride: 512000, MaxOutputTokensOverride: 8192, Primary: true},
		{ProviderType: "stepfun", Model: "step-3.5-flash", ContextWindowOverride: 100000, MaxOutputTokensOverride: 8192},
	}}, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}}
	for i := 0; i < 4300; i++ {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf("[TaskStateSummary]\nSynthetic diagnostic entry %d:\n", i) + strings.Repeat("A tool returned a synthetic result and the operation completed. ", 25),
		})
		if i%100 == 0 {
			messages = append(messages,
				openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Earlier request %d", i)},
				openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Earlier answer"},
			)
		}
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "Required trusted addendum"})
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("recent-%d", i)
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Recent request " + id},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ReasoningContent: "Private reasoning", ToolCalls: []openai.ToolCall{{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`}}}},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: id, Content: "Recent result " + id},
		)
	}
	const current = "Please report the current status."
	anchor := len(messages)
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: current})
	original := append([]openai.ChatCompletionMessage(nil), messages...)
	started := time.Now()
	kept, dropped, stats := budget.trimHistoryWorkingSetWithTaskAnchor(messages, current, anchor, "system", nil, newTokenCountCache(8))
	t.Logf("messages=%d elapsed=%s kept=%d dropped=%d", len(messages), time.Since(started), len(kept), len(dropped))
	if len(dropped) < 4000 {
		t.Fatalf("did not exercise a large history reduction: dropped %d", len(dropped))
	}
	if !reflect.DeepEqual(messages, original) {
		t.Fatal("history reduction mutated its input")
	}
	if stats.CurrentUserIndex < 0 || kept[stats.CurrentUserIndex].Content != current {
		t.Fatal("the current request was not preserved at its rebased index")
	}
	if !reflect.DeepEqual(kept[len(kept)-8:], messages[len(messages)-8:]) {
		t.Fatal("trusted addendum or the two newest complete tool rounds were changed")
	}
	verification := newTokenCountCache(len(messages) * len(budget.Routes) * 2)
	if !budget.historyWorkingSetFits(kept, stats.CurrentUserIndex, "system", nil, verification) {
		t.Fatal("the reduced history exceeds a candidate route's capacity")
	}
	if stats.KeptTokens != budget.maxCarriedHistoryTokens(kept, stats.CurrentUserIndex, verification) ||
		stats.DroppedTokens != budget.maxMessagesTokens(dropped, verification) ||
		stats.SummaryTokens != budget.maxSummaryTokens(kept, verification) ||
		stats.CurrentTokens != budget.maxMessageTokensAt(kept, stats.CurrentUserIndex, verification) ||
		stats.LimitTokens != budget.historyWorkingSetLimitForMessages(kept, stats.CurrentUserIndex, "system", nil, verification) {
		t.Fatal("incremental token accounting differs from independent route accounting")
	}
}
