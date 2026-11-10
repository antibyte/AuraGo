package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/prompts"
	openai "github.com/sashabaranov/go-openai"
)

func TestPersistentHistoryThresholdMatchesFullCount(t *testing.T) {
	history := []memory.HistoryMessage{
		{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "user", Content: "Eine unveränderte Nachricht mit Umlauten: Grüße."}},
		{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "assistant", Content: "Answer", ReasoningContent: "Private reasoning remains part of accounting."}},
		{ChatCompletionMessage: openai.ChatCompletionMessage{Role: "tool", Content: "line one\n第二行\n"}},
	}
	ctx := context.Background()
	cache := newTokenCountCache(16)
	total := 0
	for _, message := range history {
		text := messageTextWithReasoningForAccounting(message.ChatCompletionMessage)
		total += cache.Count(text, "test") + 4
	}
	for _, threshold := range []int{0, total - 1, total, total + 1} {
		for _, counter := range []*tokenCountCache{nil, cache} {
			if got := persistentHistoryExceedsTokenThreshold(ctx, history, "test", threshold, counter); got != (total > threshold) {
				t.Fatalf("threshold=%d cache=%t: got %t, full count=%d", threshold, counter != nil, got, total)
			}
		}
	}
	if persistentHistoryExceedsTokenThreshold(ctx, nil, "test", 0, cache) {
		t.Fatal("empty history exceeded a nonnegative threshold")
	}
}

func TestPersistentHistoryThresholdStopsBeforeOldUncachedMessages(t *testing.T) {
	history := []memory.HistoryMessage{
		{ChatCompletionMessage: openai.ChatCompletionMessage{Content: strings.Repeat("Old source text that must not be tokenized. ", 10000)}},
		{ChatCompletionMessage: openai.ChatCompletionMessage{Content: "recent message"}},
	}
	cache := newTokenCountCache(16)
	cache.counts[cacheHashKey("recent message", "test")] = 100
	if !persistentHistoryExceedsTokenThreshold(context.Background(), history, "test", 100, cache) {
		t.Fatal("recent cached count did not exceed the threshold")
	}
	if len(cache.counts) != 1 {
		t.Fatal("threshold check tokenized old messages after the decision was already known")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if persistentHistoryExceedsTokenThreshold(ctx, history, "test", 0, cache) {
		t.Fatal("cancelled threshold check continued")
	}
}

func TestPersistentHistoryThresholdKeepsModelCacheKeys(t *testing.T) {
	history := []memory.HistoryMessage{{ChatCompletionMessage: openai.ChatCompletionMessage{Content: "same text"}}}
	cache := newTokenCountCache(16)
	cache.counts[cacheHashKey("same text", "model-a")] = 100
	cache.counts[cacheHashKey("same text", "model-b")] = 5
	if !persistentHistoryExceedsTokenThreshold(context.Background(), history, "model-a", 20, cache) ||
		persistentHistoryExceedsTokenThreshold(context.Background(), history, "model-b", 20, cache) {
		t.Fatal("compression reused a count belonging to another model")
	}
}

func BenchmarkPersistentHistoryThreshold(b *testing.B) {
	ctx := context.Background()
	history := make([]memory.HistoryMessage, 512)
	for i := range history {
		history[i].Content = fmt.Sprintf("Record %d. %s", i, strings.Repeat("A neutral history record with ordinary source text.\n", 64))
	}
	const threshold = 4096
	b.Run("full_history_reference", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			total := 0
			for _, message := range history {
				total += prompts.CountTokensForModelContext(ctx, messageTextWithReasoningForAccounting(message.ChatCompletionMessage), "test") + 4
			}
			if total <= threshold {
				b.Fatal("fixture did not exceed the threshold")
			}
		}
	})
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprintf("bounded_cached=%t", cached), func(b *testing.B) {
			var cache *tokenCountCache
			if cached {
				cache = newTokenCountCache(64)
				for _, message := range history[len(history)-16:] {
					cache.Count(messageTextWithReasoningForAccounting(message.ChatCompletionMessage), "test")
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				if !persistentHistoryExceedsTokenThreshold(ctx, history, "test", threshold, cache) {
					b.Fatal("bounded check changed the threshold decision")
				}
			}
		})
	}
}
