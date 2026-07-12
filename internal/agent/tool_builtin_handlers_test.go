package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
)

func newBuiltinHandlerTestDispatchContext(cfg *config.Config) *DispatchContext {
	return &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestDispatchCommWebScraperMatchesExecuteSkillWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.WebScraper.Enabled = false
	dc := newBuiltinHandlerTestDispatchContext(cfg)

	direct, ok := dispatchComm(context.Background(), ToolCall{
		Action: "web_scraper",
		URL:    "https://example.com",
	}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle direct web_scraper")
	}

	viaSkill, ok := dispatchComm(context.Background(), ToolCall{
		Action: "execute_skill",
		Skill:  "web_scraper",
		Params: map[string]interface{}{
			"url": "https://example.com",
		},
	}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle execute_skill web_scraper")
	}

	if direct != viaSkill {
		t.Fatalf("direct result = %q, via skill = %q", direct, viaSkill)
	}
}

func TestDecodeWebScraperArgsIncludesModeAndWaitSelector(t *testing.T) {
	req := decodeWebScraperArgs(map[string]interface{}{
		"url":               "https://example.com/feed.xml",
		"search_query":      "latest AI headlines",
		"mode":              "rss",
		"wait_for_selector": "article",
		"selector":          ".product",
		"fields": map[string]interface{}{
			"name":  "h2",
			"price": ".price",
		},
		"output_format": "rows",
		"attribute":     "href",
		"limit":         25,
	})
	if req.URL != "https://example.com/feed.xml" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.SearchQuery != "latest AI headlines" {
		t.Fatalf("SearchQuery = %q", req.SearchQuery)
	}
	if req.Mode != "rss" {
		t.Fatalf("Mode = %q, want rss", req.Mode)
	}
	if req.WaitForSelector != "article" {
		t.Fatalf("WaitForSelector = %q, want article", req.WaitForSelector)
	}
	if req.Selector != ".product" {
		t.Fatalf("Selector = %q, want .product", req.Selector)
	}
	if len(req.Fields) != 2 || req.Fields["name"] != "h2" || req.Fields["price"] != ".price" {
		t.Fatalf("Fields = %+v", req.Fields)
	}
	if req.OutputFormat != "rows" {
		t.Fatalf("OutputFormat = %q, want rows", req.OutputFormat)
	}
	if req.Attribute != "href" {
		t.Fatalf("Attribute = %q, want href", req.Attribute)
	}
	if req.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", req.Limit)
	}
}

func TestDispatchServicesPaperlessMatchesExecuteSkillWhenReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.PaperlessNGX.Enabled = true
	cfg.PaperlessNGX.ReadOnly = true
	dc := newBuiltinHandlerTestDispatchContext(cfg)

	direct, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "paperless",
		Operation: "upload",
		Title:     "Quarterly Report",
		Content:   "hello",
	}, dc)
	if !ok {
		t.Fatal("expected dispatchServices to handle paperless")
	}

	viaSkill, ok := dispatchComm(context.Background(), ToolCall{
		Action: "execute_skill",
		Skill:  "paperless",
		Params: map[string]interface{}{
			"operation": "upload",
			"title":     "Quarterly Report",
			"content":   "hello",
		},
	}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle execute_skill paperless")
	}

	if direct != viaSkill {
		t.Fatalf("direct result = %q, via skill = %q", direct, viaSkill)
	}
}

func TestResolveWikipediaLanguageUsesSystemLanguageDefault(t *testing.T) {
	if got := resolveWikipediaLanguage("", "Deutsch"); got != "de" {
		t.Fatalf("resolveWikipediaLanguage(empty, Deutsch) = %q, want de", got)
	}
	if got := resolveWikipediaLanguage("ja", "Deutsch"); got != "ja" {
		t.Fatalf("resolveWikipediaLanguage(ja, Deutsch) = %q, want ja", got)
	}
}
