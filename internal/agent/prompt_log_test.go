package agent

import (
	"reflect"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestPromptLogEntryCapturesRuntimeDriftAndRecoveryState(t *testing.T) {
	recovery := newToolRecoveryState()
	recovery.ConsecutiveErrorCount = 2
	recovery.TotalErrorCount = 5
	recovery.DuplicateToolCount = 1
	req := openai.ChatCompletionRequest{
		Model: "weak-model",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "prompt revision A"},
			{Role: openai.ChatMessageRoleUser, Content: "retry the camera analysis"},
		},
		Tools: []openai.Tool{makeTool("z_tool"), makeTool("a_tool")},
	}

	entry := newPromptLogEntry(req, "openrouter", "builder-revision-a", recovery, 3, 7)
	if entry.Provider != "openrouter" || entry.Model != "weak-model" {
		t.Fatalf("provider/model = %q/%q", entry.Provider, entry.Model)
	}
	if entry.BuildID == "" || entry.PromptRevision == "" || entry.ToolCatalogHash == "" {
		t.Fatalf("missing drift identifiers: %#v", entry)
	}
	if entry.BuilderRevision != "builder-revision-a" {
		t.Fatalf("builder revision = %q", entry.BuilderRevision)
	}
	if !reflect.DeepEqual(entry.ActiveTools, []string{"a_tool", "z_tool"}) || entry.ToolsCount != 2 {
		t.Fatalf("active tools = %#v, count %d", entry.ActiveTools, entry.ToolsCount)
	}
	if entry.Recovery.ConsecutiveErrors != 2 || entry.Recovery.TotalErrors != 5 || entry.Recovery.DuplicateTools != 1 || entry.Recovery.Retry422Count != 3 || entry.Recovery.ToolCallCount != 7 {
		t.Fatalf("recovery metadata = %#v", entry.Recovery)
	}

	changed := req
	changed.Messages = append([]openai.ChatCompletionMessage(nil), req.Messages...)
	changed.Messages[0].Content = "prompt revision B"
	changedEntry := newPromptLogEntry(changed, "openrouter", "builder-revision-a", recovery, 3, 7)
	if changedEntry.PromptRevision == entry.PromptRevision {
		t.Fatalf("prompt revision did not change: %q", entry.PromptRevision)
	}
	if changedEntry.ToolCatalogHash != entry.ToolCatalogHash {
		t.Fatalf("unchanged tool catalog hash drifted: %q != %q", changedEntry.ToolCatalogHash, entry.ToolCatalogHash)
	}
}

func TestPromptMessagesRevisionIncludesAllSystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "builder prompt"},
		{Role: openai.ChatMessageRoleSystem, Content: "co-agent addendum"},
		{Role: openai.ChatMessageRoleUser, Content: "task"},
	}
	first := promptMessagesRevision(messages)
	messages[1].Content = "changed co-agent addendum"
	if second := promptMessagesRevision(messages); second == first {
		t.Fatal("second system message did not affect request revision")
	}
}
