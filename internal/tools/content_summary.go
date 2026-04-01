package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// SummaryLLMConfig holds the credentials for the summary LLM call,
// decoupled from any specific tool's config section.
type SummaryLLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type summaryEnvelope struct {
	Status  string `json:"status"`
	Content string `json:"content"`
	Message string `json:"message"`
}

// ResolveSummaryLLMConfig prefers the dedicated helper LLM when it is explicitly
// enabled and resolved. Otherwise it falls back to the caller-provided config.
func ResolveSummaryLLMConfig(cfg *config.Config, fallback SummaryLLMConfig) SummaryLLMConfig {
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		return SummaryLLMConfig{
			APIKey:  helperCfg.APIKey,
			BaseURL: helperCfg.BaseURL,
			Model:   helperCfg.Model,
		}
	}
	return fallback
}

// ExtractSummarySourceContent unwraps common tool output envelopes before a summary call.
func ExtractSummarySourceContent(rawContent string) (string, error) {
	var envelope summaryEnvelope
	if err := json.Unmarshal([]byte(rawContent), &envelope); err == nil {
		if envelope.Status == "error" {
			return "", fmt.Errorf("source returned error: %s", envelope.Message)
		}
		if envelope.Content != "" {
			return envelope.Content, nil
		}
	}
	return rawContent, nil
}

// EncodeSummaryContent wraps plain-text summary content in the standard success envelope.
func EncodeSummaryContent(summary string) string {
	result := map[string]interface{}{
		"status":  "success",
		"content": security.IsolateExternalData(summary),
	}
	b, _ := json.Marshal(result)
	return string(b)
}

// SummariseContent sends raw content to a (typically cheaper) LLM and returns
// a focused summary.  The searchQuery tells the summariser what specific
// information to extract; sourceName describes the content type for the system
// prompt (e.g. "web page", "PDF document", "Wikipedia article", "search results").
func SummariseContent(ctx context.Context, llmCfg SummaryLLMConfig, logger *slog.Logger, rawContent string, searchQuery string, sourceName string) (string, error) {
	extracted, err := ExtractSummarySourceContent(rawContent)
	if err != nil {
		return "", err
	}
	rawContent = extracted

	systemPrompt := fmt.Sprintf(
		"You are a content summariser. "+
			"You receive the plain text of a %s. "+
			"Summarise ONLY the information relevant to the user's search query. "+
			"Be concise but accurate. Output plain text, no markdown. "+
			"If the content does not contain relevant information, say so briefly.",
		sourceName,
	)

	userPrompt := fmt.Sprintf("Search query: %s\n\n--- CONTENT ---\n%s", searchQuery, rawContent)

	const maxUserLen = 12000
	if len(userPrompt) > maxUserLen {
		userPrompt = userPrompt[:maxUserLen]
	}

	apiKey := llmCfg.APIKey
	if apiKey == "" {
		return "", fmt.Errorf("summary_mode: no API key configured")
	}

	clientCfg := openai.DefaultConfig(apiKey)
	if llmCfg.BaseURL != "" {
		url := strings.TrimRight(llmCfg.BaseURL, "/")
		if !strings.Contains(url, "/v1") {
			url += "/v1"
		}
		clientCfg.BaseURL = url
	}
	client := openai.NewClientWithConfig(clientCfg)

	req := openai.ChatCompletionRequest{
		Model: llmCfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.2,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summary LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("summary LLM returned no choices")
	}

	summary := strings.TrimSpace(resp.Choices[0].Message.Content)
	if summary == "" {
		return "", fmt.Errorf("summary LLM returned empty content")
	}

	logger.Info("summary mode produced summary", "source", sourceName, "chars", len(summary))
	return EncodeSummaryContent(summary), nil
}
