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
