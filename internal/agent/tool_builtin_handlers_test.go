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

	direct := dispatchComm(context.Background(), ToolCall{
		Action: "web_scraper",
		URL:    "https://example.com",
	}, dc)

	viaSkill := dispatchComm(context.Background(), ToolCall{
		Action: "execute_skill",
		Skill:  "web_scraper",
		Params: map[string]interface{}{
			"url": "https://example.com",
		},
	}, dc)

	if direct != viaSkill {
		t.Fatalf("direct result = %q, via skill = %q", direct, viaSkill)
	}
}

func TestDispatchServicesPaperlessMatchesExecuteSkillWhenReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.PaperlessNGX.Enabled = true
	cfg.PaperlessNGX.ReadOnly = true
	dc := newBuiltinHandlerTestDispatchContext(cfg)

	direct := dispatchServices(context.Background(), ToolCall{
		Action:    "paperless",
		Operation: "upload",
		Title:     "Quarterly Report",
		Content:   "hello",
	}, dc)

	viaSkill := dispatchComm(context.Background(), ToolCall{
		Action: "execute_skill",
		Skill:  "paperless",
		Params: map[string]interface{}{
			"operation": "upload",
			"title":     "Quarterly Report",
			"content":   "hello",
		},
	}, dc)

	if direct != viaSkill {
		t.Fatalf("direct result = %q, via skill = %q", direct, viaSkill)
	}
}
