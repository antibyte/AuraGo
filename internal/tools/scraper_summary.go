package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

// SummariseScrapedContent sends already-scraped web content to a (typically
// cheaper) LLM and returns only the summary.  This saves tokens in the main
// agent model and prevents prompt injection because the raw external text
// never enters the agent's context.
//
// The searchQuery tells the summariser what specific information the agent
// is looking for so it can produce a focused answer.
func SummariseScrapedContent(ctx context.Context, cfg *config.Config, logger *slog.Logger, scraped string, searchQuery string) (string, error) {
	// Extract content from the scraped JSON envelope if present.
	var envelope struct {
		Status  string `json:"status"`
		Content string `json:"content"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(scraped), &envelope); err == nil {
		if envelope.Status == "error" {
			// Scraping itself failed — nothing to summarise.
			return "", fmt.Errorf("scrape returned error: %s", envelope.Message)
		}
		if envelope.Content != "" {
			scraped = envelope.Content
		}
	}

	// Build a focused system prompt.
	systemPrompt := "You are a web-content summariser. " +
		"You receive the plain text of a scraped web page. " +
		"Summarise ONLY the information relevant to the user's search query. " +
		"Be concise but accurate. Output plain text, no markdown. " +
		"If the page does not contain relevant information, say so briefly."

	userPrompt := fmt.Sprintf("Search query: %s\n\n--- PAGE CONTENT ---\n%s", searchQuery, scraped)

	// Truncate to avoid blowing up cheap-model context windows.
	const maxUserLen = 12000
	if len(userPrompt) > maxUserLen {
		userPrompt = userPrompt[:maxUserLen]
	}

	// Build the client from resolved config fields.
	apiKey := cfg.Tools.WebScraper.SummaryAPIKey
	if apiKey == "" {
		return "", fmt.Errorf("web_scraper summary_mode: no API key configured")
	}
	baseURL := cfg.Tools.WebScraper.SummaryBaseURL
	model := cfg.Tools.WebScraper.SummaryModel

	clientCfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		url := strings.TrimRight(baseURL, "/")
		if !strings.Contains(url, "/v1") {
			url += "/v1"
		}
		clientCfg.BaseURL = url
	}
	client := openai.NewClientWithConfig(clientCfg)

	req := openai.ChatCompletionRequest{
		Model: model,
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

	// Wrap the summary the same way the scraper wraps raw content so the
	// agent sees a consistent format.
	result := map[string]interface{}{
		"status":  "success",
		"content": fmt.Sprintf("<external_data source=\"summary\">\n%s\n</external_data>", summary),
	}
	b, _ := json.Marshal(result)

	logger.Info("web_scraper summary mode produced summary", "chars", len(summary))
	return string(b), nil
}
