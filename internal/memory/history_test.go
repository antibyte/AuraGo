package memory

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestHistoryManager_TotalChars(t *testing.T) {
	hm := &HistoryManager{
		Messages: []HistoryMessage{
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "user", Content: "Hello"}},    // 5 chars
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "assistant", Content: "Hi!"}}, // 3 chars
		},
	}

	expected := 8
	if actual := hm.TotalChars(); actual != expected {
		t.Errorf("Expected %d chars, got %d", expected, actual)
	}
}

func TestHistoryManager_GetOldestMessagesForPruning(t *testing.T) {
	hm := &HistoryManager{
		Messages: []HistoryMessage{
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "user", Content: "Message 1"}, ID: 1},       // 9 chars
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "assistant", Content: "Response 1"}, ID: 2}, // 10 chars
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "user", Content: "Message 2"}, ID: 3},       // 9 chars
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "assistant", Content: "Response 2"}, ID: 4}, // 10 chars
		},
	}

	// Case 1: Prune exactly first message (9 chars)
	msgs, chars := hm.GetOldestMessagesForPruning(5)
	if len(msgs) != 1 || msgs[0].Content != "Message 1" || chars != 9 {
		t.Errorf("Case 1 failed: got %d msgs, %d chars", len(msgs), chars)
	}

	// Case 2: Prune enough to cover 15 chars (should be first 2 messages: 9+10=19)
	msgs, chars = hm.GetOldestMessagesForPruning(15)
	if len(msgs) != 2 || chars != 19 {
		t.Errorf("Case 2 failed: got %d msgs, %d chars", len(msgs), chars)
	}

	// Case 3: Skip pinned messages. Pin message 1 and 2.
	hm.Messages[0].Pinned = true
	hm.Messages[1].Pinned = true
	msgs, chars = hm.GetOldestMessagesForPruning(5)
	// Only message 3 should be returned (9 chars)
	if len(msgs) != 1 || msgs[0].ID != 3 || chars != 9 {
		t.Errorf("Case 3 failed (pinning): got %d msgs, %d chars, first ID %d", len(msgs), chars, msgs[0].ID)
	}

	// Case 4: Target more than total chars
	msgs, chars = hm.GetOldestMessagesForPruning(100)
	// Only 3 and 4 should be pruned (19 chars) as 1 and 2 are pinned
	if len(msgs) != 2 || chars != 19 {
		t.Errorf("Case 4 failed: got %d msgs, %d chars", len(msgs), chars)
	}

	// Case 4: Target 0
	msgs, chars = hm.GetOldestMessagesForPruning(0)
	if len(msgs) != 0 || chars != 0 {
		t.Errorf("Case 4 failed: got %d msgs, %d chars", len(msgs), chars)
	}
}

// ── Lifecycle (disk persistence) ─────────────────────────────────────────────

func TestHistoryManager_PersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	// Create, add a message, close (flushes).
	hm := NewHistoryManager(path)
	if err := hm.Add("user", "hello world", 1, false, false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hm.Close()

	// Re-open from the same file and verify persistence.
	hm2 := NewHistoryManager(path)
	defer hm2.Close()
	all := hm2.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 message after reload, got %d", len(all))
	}
	if all[0].Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", all[0].Content)
	}
	if all[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", all[0].ID)
	}
}

func TestHistoryManager_PersistAndLoad_MultiContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	hm := NewHistoryManager(path)
	msg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{Type: openai.ChatMessagePartTypeText, Text: "what is in this image?"},
			{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64,AA=="}},
		},
	}
	if err := hm.AddMessage(msg, 7, false, false); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	hm.Close()

	hm2 := NewHistoryManager(path)
	defer hm2.Close()
	all := hm2.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 message after reload, got %d", len(all))
	}
	if all[0].Role != openai.ChatMessageRoleUser {
		t.Fatalf("expected role %q, got %q", openai.ChatMessageRoleUser, all[0].Role)
	}
	if all[0].Content != "" {
		t.Fatalf("expected empty Content for MultiContent message, got %q", all[0].Content)
	}
	if len(all[0].MultiContent) != 2 {
		t.Fatalf("expected 2 MultiContent parts, got %d", len(all[0].MultiContent))
	}
	if all[0].MultiContent[0].Type != openai.ChatMessagePartTypeText {
		t.Fatalf("expected first part type text, got %q", all[0].MultiContent[0].Type)
	}
	if all[0].MultiContent[1].Type != openai.ChatMessagePartTypeImageURL {
		t.Fatalf("expected second part type image_url, got %q", all[0].MultiContent[1].Type)
	}
}

func TestHistoryManager_EmptyFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	// Create an empty file.
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	hm := NewHistoryManager(path)
	defer hm.Close()
	all := hm.GetAll()
	if len(all) != 0 {
		t.Fatalf("expected 0 messages from empty file, got %d", len(all))
	}
}

func TestHistoryManager_SetPinned(t *testing.T) {
	hm := &HistoryManager{
		Messages: []HistoryMessage{
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "user", Content: "hi"}, ID: 42},
		},
		saveChan: make(chan struct{}, 1),
		doneChan: make(chan struct{}),
	}
	hm.saverWg.Add(1)
	go hm.backgroundSaver()
	defer hm.Close()

	if err := hm.SetPinned(42, true); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	all := hm.GetAll()
	if !all[0].Pinned {
		t.Error("expected message to be pinned")
	}

	// Setting pin on non-existent ID → error.
	if err := hm.SetPinned(999, true); err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestHistoryManager_GetReturnsOpenAIMessages(t *testing.T) {
	hm := &HistoryManager{
		Messages: []HistoryMessage{
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "user", Content: "q"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "assistant", Content: "a"}},
		},
		saveChan: make(chan struct{}, 1),
		doneChan: make(chan struct{}),
	}
	hm.saverWg.Add(1)
	go hm.backgroundSaver()
	defer hm.Close()

	msgs := hm.Get()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("unexpected roles: %v %v", msgs[0].Role, msgs[1].Role)
	}
}

func TestHistoryManager_GetForLLM_FiltersDanglingToolMessages(t *testing.T) {
	hm := &HistoryManager{
		Messages: []HistoryMessage{
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "sys"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{ID: "call_1", Function: openai.FunctionCall{Name: "get_weather", Arguments: `{"city":"Berlin"}`}},
					{ID: "call_2", Function: openai.FunctionCall{Name: "get_time", Arguments: `{}`}},
				},
			}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: "bad missing id"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: "ok", ToolCallID: "call_1"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "next"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: "late", ToolCallID: "call_2"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{ID: "", Function: openai.FunctionCall{Name: "broken", Arguments: `{}`}},
				},
			}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: "orphan", ToolCallID: "call_999"}},
			{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "user"}},
		},
		saveChan: make(chan struct{}, 1),
		doneChan: make(chan struct{}),
	}

	msgs := hm.GetForLLM()

	// Expected sequence:
	// system, assistant(tool_calls), tool(call_1), assistant("next"), assistant(broken tool_calls but cleared), user
	if len(msgs) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(msgs))
	}
	if msgs[0].Role != openai.ChatMessageRoleSystem {
		t.Fatalf("msgs[0].Role = %q, want system", msgs[0].Role)
	}
	if msgs[1].Role != openai.ChatMessageRoleAssistant || len(msgs[1].ToolCalls) != 2 {
		t.Fatalf("msgs[1] should be assistant with 2 tool calls, got role=%q tool_calls=%d", msgs[1].Role, len(msgs[1].ToolCalls))
	}
	if msgs[2].Role != openai.ChatMessageRoleTool || msgs[2].ToolCallID != "call_1" {
		t.Fatalf("msgs[2] should be tool result for call_1, got role=%q tool_call_id=%q", msgs[2].Role, msgs[2].ToolCallID)
	}
	if msgs[3].Role != openai.ChatMessageRoleAssistant || msgs[3].Content != "next" {
		t.Fatalf("msgs[3] should be assistant 'next', got role=%q content=%q", msgs[3].Role, msgs[3].Content)
	}
	if msgs[4].Role != openai.ChatMessageRoleAssistant || len(msgs[4].ToolCalls) != 0 {
		t.Fatalf("msgs[4] should be assistant with cleared tool_calls, got role=%q tool_calls=%d", msgs[4].Role, len(msgs[4].ToolCalls))
	}
	if msgs[5].Role != openai.ChatMessageRoleUser || msgs[5].Content != "user" {
		t.Fatalf("msgs[5] should be user, got role=%q content=%q", msgs[5].Role, msgs[5].Content)
	}
}

// ── Ephemeral HistoryManager ──────────────────────────────────────────────────

func TestEphemeralHistoryManager_NoDiskWrite(t *testing.T) {
	hm := NewEphemeralHistoryManager()
	defer hm.Close()
	_ = hm.Add("user", "test", 1, false, false)
	// No disk file → save() is a no-op. Verify message is present in memory.
	all := hm.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 message in ephemeral manager, got %d", len(all))
	}
}

func TestEphemeralHistoryManager_TrimsAtLimit(t *testing.T) {
	hm := NewEphemeralHistoryManager()
	defer hm.Close()

	// Add maxEphemeralMessages+10 non-pinned messages.
	for i := 0; i < maxEphemeralMessages+10; i++ {
		_ = hm.Add("user", "msg", int64(i), false, false)
	}

	all := hm.GetAll()
	if len(all) > maxEphemeralMessages {
		t.Errorf("ephemeral manager should not exceed %d messages, has %d", maxEphemeralMessages, len(all))
	}
}

func TestEphemeralHistoryManager_PinnedMessagesSurviveTrim(t *testing.T) {
	hm := NewEphemeralHistoryManager()
	defer hm.Close()

	// Add one pinned message first.
	_ = hm.Add("user", "pinned", 1, true, false)

	// Fill beyond limit with non-pinned messages.
	for i := 2; i < maxEphemeralMessages+20; i++ {
		_ = hm.Add("user", "regular", int64(i), false, false)
	}

	all := hm.GetAll()
	found := false
	for _, m := range all {
		if m.ID == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("pinned message (ID=1) should survive ephemeral trim")
	}
}

// ── Concurrent safety ─────────────────────────────────────────────────────────

func TestHistoryManager_ConcurrentAdd(t *testing.T) {
	hm := NewEphemeralHistoryManager()
	defer hm.Close()

	const workers = 10
	const msgsPerWorker = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; i < msgsPerWorker; i++ {
				_ = hm.Add("user", "concurrent", int64(w*1000+i), false, false)
			}
		}(w)
	}
	wg.Wait()
	// Give background saver a moment to drain.
	time.Sleep(10 * time.Millisecond)
	// No crash = success. Optionally check message count is bounded.
	all := hm.GetAll()
	if len(all) > maxEphemeralMessages {
		t.Errorf("concurrent adds exceeded ephemeral limit: %d > %d", len(all), maxEphemeralMessages)
	}
}
