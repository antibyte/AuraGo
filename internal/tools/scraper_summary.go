package tools

import (
	"context"
	"log/slog"

	"aurago/internal/config"
)

// SummariseScrapedContent sends already-scraped web content to a (typically
// cheaper) LLM and returns only the summary.  This is a thin wrapper around
// the generic SummariseContent function using the WebScraper config fields.
func SummariseScrapedContent(ctx context.Context, cfg *config.Config, logger *slog.Logger, scraped string, searchQuery string) (string, error) {
	return SummariseContent(ctx, ResolveSummaryLLMConfig(cfg, SummaryLLMConfig{
		APIKey:  cfg.Tools.WebScraper.SummaryAPIKey,
		BaseURL: cfg.Tools.WebScraper.SummaryBaseURL,
		Model:   cfg.Tools.WebScraper.SummaryModel,
	}), logger, scraped, searchQuery, "web page")
}
