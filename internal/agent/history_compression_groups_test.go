package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestPersistentCompressionMakesProgressThroughLargeToolConversation(t *testing.T) {
	dir := t.TempDir()
	stm, err := memory.NewSQLiteMemory(filepath.Join(dir, "memory.db"), testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewHistoryManager(filepath.Join(dir, "history.json"))
	t.Cleanup(history.Close)
	add := func(message openai.ChatCompletionMessage) {
		t.Helper()
		id, err := stm.InsertMessage("default", message.Role, message.Content, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := history.AddMessage(message, id, false, false); err != nil {
			t.Fatal(err)
		}
	}
	add(openai.ChatCompletionMessage{Role: "user", Content: "Keep working on this long task"})
	for round := 0; round < 160; round++ {
		ids := []string{fmt.Sprintf("call-%d-a", round), fmt.Sprintf("call-%d-b", round)}
		calls := []openai.ToolCall{}
		for _, id := range ids {
			calls = append(calls, openai.ToolCall{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"example.txt"}`}})
		}
		add(openai.ChatCompletionMessage{Role: "assistant", ToolCalls: calls})
		for _, id := range ids {
			add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: id, Content: strings.Repeat("source content ", 150)})
		}
	}
	client := &mockChatClient{response: "Earlier tool rounds summarized"}
	result := compressPersistentHistory(context.Background(), RunConfig{Config: &config.Config{}, Logger: testLogger, HistoryManager: history, ShortTermMem: stm, SessionID: "default"}, 0, 24576, true, "test", client, testLogger)
	if !result.Compressed || len(result.Dropped) == 0 {
		t.Fatal("oversized conversation still blocks compression")
	}
	if len(result.Dropped) > persistentCompressionMaxMessages {
		t.Fatal("helper message cap exceeded")
	}
	if len(client.lastReq.Messages[0].Content) > persistentCompressionMaxTranscriptChars+4096 {
		t.Fatal("helper input cap exceeded")
	}
	for _, m := range result.Dropped {
		if m.Role == "user" {
			t.Fatal("current human request was compressed")
		}
	}
	remaining := history.Get()
	if remaining[0].Role != "user" {
		t.Fatal("lost original human intent")
	}
	rounds := findCompleteNativeToolRounds(remaining)
	for _, id := range []string{"call-158-a", "call-158-b", "call-159-a", "call-159-b"} {
		found := false
		for _, round := range rounds {
			for _, call := range round.calls {
				if call.ID == id {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("newest tool round %s was not preserved", id)
		}
	}
	_, orphaned := SanitizeToolMessages(remaining)
	if orphaned != 0 {
		t.Fatalf("compression created %d orphaned messages", orphaned)
	}
}

func TestPersistentCompressionGroupsNeverSplitIncompleteToolRound(t *testing.T) {
	messages := []openai.ChatCompletionMessage{{Role: "user", Content: "task"}}
	for i := 0; i < 30; i++ {
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: strings.Repeat("x", 2000)})
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "missing", Type: openai.ToolTypeFunction}}})
	for _, group := range persistentCompressionGroups(messages) {
		if group.end > len(messages)-1 {
			t.Fatal("incomplete call is eligible for deletion")
		}
	}
}
