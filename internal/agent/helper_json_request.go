package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/llm"
	"aurago/internal/prompts"
	"github.com/sashabaranov/go-openai"
)

type helperJSONPolicy struct {
	retryPrompt func() string
	validate    func(string) error
}

// helperSourceText reduces source fields before rendering their structural wrappers.
func helperSourceText(text string, limit, divisor int) string {
	if divisor > 1 {
		limit = max(1, min(limit, len(strings.TrimSpace(text)))/divisor)
	}
	return truncateActivityDigestInput(text, limit)
}

func requireHelperFields(raw string, fields ...string) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return err
	}
	for _, field := range fields {
		if value := strings.TrimSpace(string(values[field])); value == "" || value == "null" {
			return fmt.Errorf("helper response missing %s", field)
		}
	}
	return nil
}

func requireHelperIDs(expected, actual []string) error {
	remaining := make(map[string]bool, len(expected))
	for _, id := range expected {
		remaining[strings.TrimSpace(id)] = true
	}
	for _, id := range actual {
		id = strings.TrimSpace(id)
		if !remaining[id] {
			return fmt.Errorf("helper response has unexpected or duplicate ID")
		}
		delete(remaining, id)
	}
	if len(remaining) != 0 {
		return fmt.Errorf("helper response missing %d IDs", len(remaining))
	}
	return nil
}

func (m *helperLLMManager) requestJSONResponse(ctx context.Context, operation, cacheKey, systemPrompt, userPrompt string, maxTokens int, policies ...helperJSONPolicy) (string, error) {
	if m == nil || m.client == nil || m.model == "" {
		return "", fmt.Errorf("helper llm manager unavailable")
	}
	var policy helperJSONPolicy
	if len(policies) > 0 {
		policy = policies[0]
	}
	validate := func(raw string) (string, error) {
		normalized, err := llm.NormalizeJSONContent(raw)
		if err == nil && policy.validate != nil {
			err = policy.validate(normalized)
		}
		return normalized, err
	}
	m.observeStat(operation, func(stat *HelperLLMOperationStats) { stat.Requests++ })
	if cached, ok := m.getCachedResponse(cacheKey); ok {
		if normalized, err := validate(cached); err == nil {
			m.observeStat(operation, func(stat *HelperLLMOperationStats) { stat.CacheHits++; stat.LastDetail = "cache_hit" })
			return normalized, nil
		}
		m.deleteCachedResponse(cacheKey)
	}
	if m.sem != nil {
		select {
		case m.sem <- struct{}{}:
		case <-ctx.Done():
			return "", fmt.Errorf("helper llm concurrency wait cancelled: %w", ctx.Err())
		}
		defer func() { <-m.sem }()
	}
	route := m.route
	if route.Model == "" {
		route.Model, route.ProviderType = m.model, m.providerType
	}
	limits := llm.ResolveModelLimitsCached(route, m.contextWindow)
	structured := false
	if caps, ok := llm.CapabilitiesFromRegistry(m.providerType, m.model); ok {
		structured = caps.StructuredOutputs
	}
	var lastErr error
	attempts := 0
	for attempt := 0; attempt <= helperMaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if attempt > 0 {
			timer := time.NewTimer(helperRetryDelay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return "", fmt.Errorf("helper llm retry cancelled: %w", ctx.Err())
			}
		}
		inputTokens := prompts.CountTokensForModel(systemPrompt, m.model) + prompts.CountTokensForModel(userPrompt, m.model) + 32
		outputTokens, err := llm.JSONCompletionOutputBudget(limits, maxTokens, inputTokens)
		if err != nil {
			return "", err
		}
		attempts++
		m.observeStat(operation, func(stat *HelperLLMOperationStats) { stat.LLMCalls++; stat.LastDetail = "llm_call" })
		resp, err := m.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: m.model, Temperature: 0.1, MaxTokens: outputTokens,
			Messages:       []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: systemPrompt}, {Role: openai.ChatMessageRoleUser, Content: userPrompt}},
			ResponseFormat: llm.JSONResponseFormat(structured),
		})
		if err != nil {
			lastErr = err
			if !isTransientHelperError(err) || ctx.Err() != nil {
				break
			}
			continue
		}
		raw, err := llm.JSONContentFromResponse(resp)
		if err == nil {
			raw, err = validate(raw)
		}
		if err == nil {
			m.setCachedResponse(cacheKey, raw)
			return raw, nil
		}
		lastErr = err
		if attempt == helperMaxRetries || policy.retryPrompt == nil {
			break
		}
		reduced := policy.retryPrompt()
		if strings.TrimSpace(reduced) == "" || len(reduced) >= len(userPrompt) {
			break
		}
		userPrompt = reduced
	}
	m.observeStat(operation, func(stat *HelperLLMOperationStats) { stat.LastDetail = "llm_call_failed" })
	return "", fmt.Errorf("helper llm failed after %d attempts: %w", attempts, lastErr)
}
