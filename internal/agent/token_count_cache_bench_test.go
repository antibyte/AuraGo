package agent

import (
	"strings"
	"testing"

	"aurago/internal/prompts"
)

func BenchmarkTokenCountCacheHit(b *testing.B) {
	cache := newTokenCountCache(1024)
	text := strings.Repeat("hello world ", 100)
	model := "gpt-4"
	// Warm cache.
	_ = cache.Count(text, model)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Count(text, model)
	}
}

func BenchmarkTokenCountCacheMiss(b *testing.B) {
	cache := newTokenCountCache(1024)
	model := "gpt-4"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Count(strings.Repeat("x", i%1000+1), model)
	}
}

func BenchmarkTokenCountNoCache(b *testing.B) {
	text := strings.Repeat("hello world ", 100)
	model := "gpt-4"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = prompts.CountTokensForModel(text, model)
	}
}
