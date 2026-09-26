package agent

import (
	"context"
	"sync"
	"time"

	"aurago/internal/llm"
	"github.com/sashabaranov/go-openai"
)

type PromptUsageObservation struct {
	llm.RequestUsage
	ProfileRevision     string `json:"profile_revision"`
	SchemaFingerprint   string `json:"schema_fingerprint"`
	PrefixFingerprint   string `json:"prefix_fingerprint"`
	ContextGeneration   int    `json:"context_generation"`
	ContextChange       string `json:"context_change,omitempty"`
	LocalPromptCacheHit bool   `json:"local_prompt_cache_hit"`
	Failed              bool   `json:"failed"`
}

// PromptUsageObserver retains hashes only, never prompt text or source. Reuse it
// across phases/retries to measure context changes separately from provider hits.
type PromptUsageObserver struct {
	Observe  func(PromptUsageObservation)
	mu       sync.Mutex
	last     PromptUsageObservation
	messages []string
}

func (o *PromptUsageObserver) begin(ctx context.Context, req openai.ChatCompletionRequest, provider, revision string, localHit bool) (context.Context, func(openai.ChatCompletionResponse, error)) {
	if o == nil || o.Observe == nil {
		return ctx, func(openai.ChatCompletionResponse, error) {}
	}
	ctx, capture := llm.CaptureUsage(ctx)
	started := time.Now()
	hashes := make([]string, len(req.Messages))
	var system []openai.ChatCompletionMessage
	for i, message := range req.Messages {
		hashes[i] = promptFingerprint(message)
		if message.Role == openai.ChatMessageRoleSystem && i == len(system) {
			system = append(system, message)
		}
	}
	base := PromptUsageObservation{ProfileRevision: revision, SchemaFingerprint: promptFingerprint(req.Tools), PrefixFingerprint: promptFingerprint(struct {
		System []openai.ChatCompletionMessage
		Tools  []openai.Tool
		Format *openai.ChatCompletionResponseFormat
	}{system, req.Tools, req.ResponseFormat}), LocalPromptCacheHit: localHit}
	return ctx, func(response openai.ChatCompletionResponse, err error) {
		items := capture.Snapshot()
		if len(items) == 0 {
			// Mock/custom clients may not use our HTTP transport. Zero SDK fields
			// cannot establish that usage was actually reported.
			u := response.Usage
			item := llm.RequestUsage{Provider: provider, Model: req.Model, DurationMS: time.Since(started).Milliseconds()}
			if response.Model != "" {
				item.Model = response.Model
			}
			if u.PromptTokens > 0 || u.CompletionTokens > 0 {
				item.InputTokens, item.OutputTokens = &u.PromptTokens, &u.CompletionTokens
			}
			if u.PromptTokensDetails != nil {
				n := u.PromptTokensDetails.CachedTokens
				item.CacheReadTokens = &n
			}
			items = []llm.RequestUsage{item}
		}
		for _, item := range items {
			event := base
			event.RequestUsage, event.Failed = item, err != nil || item.TransportError || item.Status >= 400
			o.mu.Lock()
			event.ContextGeneration = o.last.ContextGeneration
			switch {
			case event.ContextGeneration == 0:
				event.ContextChange = "initial"
			case o.last.Provider != event.Provider || o.last.Model != event.Model:
				event.ContextChange = "route_changed"
			case o.last.ProfileRevision != event.ProfileRevision:
				event.ContextChange = "profile_changed"
			case o.last.PrefixFingerprint != event.PrefixFingerprint:
				event.ContextChange = "prefix_changed"
			case !hashPrefix(o.messages, hashes):
				event.ContextChange = "history_rebased"
			}
			if event.ContextChange != "" {
				event.ContextGeneration++
			}
			o.last, o.messages = event, hashes
			o.mu.Unlock()
			o.Observe(event)
		}
	}
}

func hashPrefix(previous, current []string) bool {
	if len(previous) > len(current) {
		return false
	}
	for i, hash := range previous {
		if current[i] != hash {
			return false
		}
	}
	return true
}
