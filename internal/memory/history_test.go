package memory

import (
	"testing"

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
