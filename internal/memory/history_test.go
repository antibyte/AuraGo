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
