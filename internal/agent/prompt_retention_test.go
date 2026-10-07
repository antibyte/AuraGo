package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestRequestBudgetImportanceTrimKeepsCurrentHumanRequest(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "anchor-test", Primary: true,
		ContextWindowOverride: 5000, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	anchor := "ok"
	messages := []openai.ChatCompletionMessage{{Role: "system", Content: "fixed"}, {Role: "user", Content: anchor}}
	for range 11 {
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: strings.Repeat("historical assistant response ", 200)})
	}

	trimmed, dropped, err := budget.trimHistoryWithCurrentUser(messages, nil, true, budgetTestLogger(), newTokenCountCache(512), anchor)
	if err != nil {
		t.Fatalf("trimHistoryWithCurrentUser: %v", err)
	}
	if len(dropped) == 0 {
		t.Fatal("expected over-budget history to be trimmed")
	}
	if !containsMessage(trimmed, openai.ChatMessageRoleUser, anchor) {
		t.Fatalf("importance trim dropped the current request: %#v", trimmed)
	}
}

func TestRequestBudgetAnchorIndexDistinguishesIdenticalCorrection(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "anchor-identity", Primary: true,
		ContextWindowOverride: 5000, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	const sameText = "ok"
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "fixed"},
		{Role: "user", Name: "older", Content: sameText},
		{Role: "user", Name: "original-task", Content: sameText},
	}
	for range 7 {
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: strings.Repeat("historical assistant response ", 200)})
	}
	messages = append(messages,
		openai.ChatCompletionMessage{Role: "user", Name: "synthetic-correction", Content: sameText},
		openai.ChatCompletionMessage{Role: "assistant", Content: "latest response"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "tail one"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "tail two"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "tail three"},
	)
	trimmed, _, anchorIndex, err := budget.trimHistoryWithTaskAnchor(messages, nil, true, budgetTestLogger(), newTokenCountCache(512), sameText, 2)
	if err != nil {
		t.Fatalf("trimHistoryWithTaskAnchor: %v", err)
	}
	if anchorIndex < 0 || anchorIndex >= len(trimmed) || trimmed[anchorIndex].Name != "original-task" {
		t.Fatalf("anchor index=%d, want original task identity in trimmed history: %#v", anchorIndex, trimmed)
	}
}

func TestRequestBudgetTrimmingKeepsTaskAndNewestTwoNativeToolRounds(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "round-retention", Primary: true,
		ContextWindowOverride: 5000, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "fixed"},
		{Role: "user", Name: "original-task", Content: "Finish the original request."},
		{Role: "assistant", Content: strings.Repeat("older work should be shed ", 1600)},
	}
	appendRound := func(label string, calls int) {
		messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: "Continue task step " + label})
		toolCalls := make([]openai.ToolCall, calls)
		for i := range calls {
			toolCalls[i] = openai.ToolCall{ID: label + "-" + string(rune('a'+i)), Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "read_file", Arguments: `{}`}}
		}
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", ToolCalls: toolCalls})
		for _, call := range toolCalls {
			messages = append(messages, openai.ChatCompletionMessage{Role: "tool", ToolCallID: call.ID, Content: "result " + call.ID})
		}
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: "Completed step " + label})
	}
	appendRound("one", 1)
	appendRound("two", 1)
	appendRound("three", 2)
	appendRound("four", 4) // The newest round spans more than the usual four-message trim tail.

	trimmed, dropped, anchorIndex, trimErr := budget.trimHistoryWithTaskAnchor(messages, nil, true, budgetTestLogger(), newTokenCountCache(512), messages[1].Content, 1)
	if trimErr != nil && !IsContextBudgetExceeded(trimErr) {
		t.Fatalf("trim error = %v, want success or typed context budget error", trimErr)
	}
	if anchorIndex < 0 || anchorIndex >= len(trimmed) || trimmed[anchorIndex].Name != "original-task" {
		t.Fatalf("current task anchor index=%d was not preserved: %#v", anchorIndex, trimmed)
	}
	for _, id := range []string{"three-a", "three-b", "four-a", "four-b", "four-c", "four-d"} {
		foundCall, foundResult := false, false
		for _, message := range trimmed {
			if message.Role == openai.ChatMessageRoleAssistant {
				for _, call := range message.ToolCalls {
					foundCall = foundCall || call.ID == id
				}
			}
			foundResult = foundResult || (message.Role == openai.ChatMessageRoleTool && message.ToolCallID == id)
		}
		if !foundCall || !foundResult {
			t.Fatalf("newest tool round ID %q was not retained completely (call=%v result=%v, dropped=%d)", id, foundCall, foundResult, len(dropped))
		}
	}
	if _, orphanCount := SanitizeToolMessages(trimmed); orphanCount != 0 {
		t.Fatalf("trim left an incomplete native tool round (%d orphan messages)", orphanCount)
	}
}

func TestPersistentRequestProjectionCannotDropProtectedDuplicateTask(t *testing.T) {
	anchor := openai.ChatCompletionMessage{Role: "user", Name: "original-task", Content: "same text as archived source"}
	messages := []openai.ChatCompletionMessage{{Role: "system", Content: "fixed"}, anchor}
	result := persistentCompressionResult{
		Compressed: true,
		Summary:    "Earlier history recap.",
		Dropped:    []memory.HistoryMessage{{ChatCompletionMessage: anchor, ID: 7}},
	}
	updated, anchorIndex := applyPersistentCompressionToRequestWithTaskAnchor(messages, result, 1)
	if anchorIndex < 0 || anchorIndex >= len(updated) || updated[anchorIndex].Name != "original-task" || updated[anchorIndex].Content != anchor.Content {
		t.Fatalf("persistent request projection lost the protected duplicate task: index=%d messages=%#v", anchorIndex, updated)
	}
}

func TestRequestBudgetReturnsTypedErrorWhenProtectedTaskCannotFit(t *testing.T) {
	client := &budgetRouteClient{routes: []llm.ModelRoute{{
		ProviderType: "custom", Model: "anchor-too-large", Primary: true,
		ContextWindowOverride: 4096, MaxOutputTokensOverride: 1000,
	}}}
	budget, err := newRequestBudget(context.Background(), &config.Config{}, client, openai.ChatCompletionRequest{}, budgetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	anchor := strings.Repeat("required-task ", 5000)
	messages := []openai.ChatCompletionMessage{{Role: "system", Content: "fixed"}, {Role: "user", Content: anchor}}
	trimmed, _, err := budget.trimHistoryWithCurrentUser(messages, nil, true, budgetTestLogger(), newTokenCountCache(128), anchor)
	if !IsContextBudgetExceeded(err) {
		t.Fatalf("error = %v, want typed context budget error", err)
	}
	if !containsMessage(trimmed, openai.ChatMessageRoleUser, anchor) {
		t.Fatal("failed request lost its protected task while reporting the budget error")
	}
}

func TestPreparedPromptAnchorRemainsOriginalTaskAfterCorrection(t *testing.T) {
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	profile, err := NewPreparedPromptProfile("anchor/v1", "Fixed workflow and safety rules.", nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "Return the requested concise format."
	correction := original
	req := openai.ChatCompletionRequest{Model: "primary-model", Messages: []openai.ChatCompletionMessage{
		{Role: "user", Name: "original-task", Content: original},
		{Role: "assistant", Content: "The prior response missed the format."},
		{Role: "user", Name: "synthetic-correction", Content: correction},
	}}
	prepared, err := prepareMinimalLoopRequestWithProfileAndTaskAnchor(context.Background(), cfg, client, &req, profile.SystemPrompt(), nil, budgetTestLogger(), newTokenCountCache(128), 0, false, profile, original, 0)
	if err != nil {
		t.Fatalf("prepare with original request anchor: %v", err)
	}
	if exactTaskMessageIndex(req.Messages, original) < 0 || exactTaskMessageIndex(req.Messages, correction) < 0 {
		t.Fatalf("prepared history lost original task or correction: %#v", req.Messages)
	}
	if got := prepared.CurrentUserIndex; got < 0 || got >= len(req.Messages) || req.Messages[got].Name != "original-task" {
		t.Fatalf("protected anchor index = %d, want original task identity: %#v", got, req.Messages)
	}
}

func TestPreparedPromptAnchorRejectsMissingOriginalTask(t *testing.T) {
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	profile, _ := NewPreparedPromptProfile("anchor/v1", "Fixed workflow and safety rules.", nil)
	req := openai.ChatCompletionRequest{Model: "primary-model", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "synthetic correction"}}}
	_, err := prepareMinimalLoopRequestWithProfileAndTaskAnchor(context.Background(), cfg, client, &req, profile.SystemPrompt(), nil, budgetTestLogger(), newTokenCountCache(128), 0, false, profile, "original human request", 0)
	if err == nil || !strings.Contains(err.Error(), "original user request") {
		t.Fatalf("error = %v, want missing original task guard", err)
	}
}

func TestTransientCompressionPreservesCurrentTaskAndFollowingToolRound(t *testing.T) {
	messages := []openai.ChatCompletionMessage{{Role: "system", Content: "fixed"}}
	for i := range 7 {
		messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("historical context ", 150) + string(rune('a'+i))})
	}
	anchor := "immutable current task"
	messages = append(messages,
		openai.ChatCompletionMessage{Role: "user", Content: anchor},
		openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "current-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "read_file", Arguments: `{}`}}}},
		openai.ChatCompletionMessage{Role: "tool", ToolCallID: "current-call", Content: "current tool result"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "current tool response"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "follow-up a"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "follow-up b"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "follow-up c"},
		openai.ChatCompletionMessage{Role: "assistant", Content: "follow-up d"},
	)
	client := &mockChatClient{response: "Earlier history recap."}
	anchorIndex := exactTaskMessageIndex(messages, anchor)
	result, _, compression := compressHistoryForTaskAnchor(context.Background(), messages, 120, "test", client, 0, testLogger, anchor, anchorIndex)
	if !compression.Compressed {
		t.Fatal("expected old history compression")
	}
	if !containsMessage(result, openai.ChatMessageRoleUser, anchor) || !containsMessage(result, openai.ChatMessageRoleTool, "current tool result") {
		t.Fatalf("compression dropped current task or its tool result: %#v", result)
	}
	if _, dropped := SanitizeToolMessages(result); dropped != 0 {
		t.Fatalf("compression split current native tool round; sanitizer would drop %d messages", dropped)
	}
	if strings.Contains(client.lastReq.Messages[0].Content, anchor) {
		t.Fatal("summary transcript included the protected current task")
	}
}

func TestTransientCompressionRebasesTaskIndexAfterDroppingOldPrefix(t *testing.T) {
	messages := []openai.ChatCompletionMessage{{Role: "system", Content: "fixed"}}
	for i := range 11 {
		messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("historical context ", 150) + string(rune('a'+i))})
	}
	anchor := openai.ChatCompletionMessage{Role: "user", Name: "original-task", Content: "Keep this protected task."}
	messages = append(messages, anchor)
	for i := range 4 {
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: "recent follow-up " + string(rune('a'+i))})
	}
	client := &mockChatClient{response: "Earlier history recap."}
	anchorIndex := len(messages) - 5
	result, _, compression := compressHistoryForTaskAnchor(context.Background(), messages, 120, "test", client, 0, testLogger, anchor.Content, anchorIndex)
	if !compression.Compressed {
		t.Fatal("expected old history compression")
	}
	if compression.CurrentUserIndex < 0 || compression.CurrentUserIndex >= len(result) || result[compression.CurrentUserIndex].Name != "original-task" {
		t.Fatalf("rebased anchor index=%d, want original task in compressed history: %#v", compression.CurrentUserIndex, result)
	}
	if compression.CurrentUserIndex != 3 {
		t.Fatalf("rebased anchor index=%d, want 3 after removing old prefix", compression.CurrentUserIndex)
	}
	if containsMessage(result, openai.ChatMessageRoleUser, strings.Repeat("historical context ", 150)+"a") {
		t.Fatal("compression retained an old task from the removed prefix")
	}
}

func TestTransientCompressionRejectsOversizedSummaryWithoutChangingHistory(t *testing.T) {
	messages := []openai.ChatCompletionMessage{{Role: "system", Content: "fixed"}}
	for i := range 8 {
		messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("historical context ", 150) + string(rune('a'+i))})
	}
	anchor := openai.ChatCompletionMessage{Role: "user", Name: "original-task", Content: "Current task."}
	messages = append(messages, anchor)
	messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: "recent follow-up"})
	client := &mockChatClient{response: strings.Repeat("oversized ", historySummaryMaxTokens+1)}
	anchorIndex := exactTaskMessageIndex(messages, anchor.Content)
	result, lastCompression, compression := compressHistoryForTaskAnchor(context.Background(), messages, 120, "test", client, 0, testLogger, anchor.Content, anchorIndex)
	if compression.Compressed || lastCompression != 0 {
		t.Fatalf("oversized summary changed compression state: result=%+v last=%d", compression, lastCompression)
	}
	if len(result) != len(messages) {
		t.Fatalf("oversized summary changed message count from %d to %d", len(messages), len(result))
	}
	for i := range messages {
		if result[i].Role != messages[i].Role || result[i].Content != messages[i].Content || result[i].Name != messages[i].Name {
			t.Fatalf("oversized summary changed history at index %d: got %+v want %+v", i, result[i], messages[i])
		}
	}
}

func TestCompressionCrashBoundaryRetainsHistorySourcesAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	historyPath := filepath.Join(dir, "history.json")
	stm, err := memory.NewSQLiteMemory(dbPath, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	history := memory.NewHistoryManager(historyPath)
	var archivedIDs []int64
	for _, message := range []openai.ChatCompletionMessage{
		{Role: "user", Content: "source user turn"},
		{Role: "assistant", Content: "source assistant turn"},
		{Role: "user", Content: "retained current task"},
	} {
		id, insertErr := stm.InsertMessage("default", message.Role, message.Content, false, false)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		if addErr := history.AddMessage(message, id, false, false); addErr != nil {
			t.Fatal(addErr)
		}
		if message.Content != "retained current task" {
			archivedIDs = append(archivedIDs, id)
		}
	}
	history.Close() // Persist the raw snapshot as if the prior process had exited.
	if err := stm.DeleteMessagesByID("default", archivedIDs); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after SQLite archive/delete but before ApplyCompression.
	stm, err = memory.NewSQLiteMemory(dbPath, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	restartedHistory := memory.NewHistoryManager(historyPath)
	t.Cleanup(restartedHistory.Close)
	if got := restartedHistory.GetSummary(); got != "" {
		t.Fatalf("unexpected summary after pre-ApplyCompression crash: %q", got)
	}
	all := restartedHistory.GetAll()
	if len(all) != 3 || all[0].Content != "source user turn" || all[1].Content != "source assistant turn" || all[2].Content != "retained current task" {
		t.Fatalf("raw history sources did not survive restart: %+v", all)
	}
	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) < len(archivedIDs) {
		t.Fatalf("SQLite archive retained %d sources, want at least %d", len(archived), len(archivedIDs))
	}
}
