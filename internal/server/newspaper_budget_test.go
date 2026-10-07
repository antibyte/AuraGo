package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/newspaper"
	"aurago/internal/scraper"

	openai "github.com/sashabaranov/go-openai"
)

func TestNewspaperChargesPlanningAndRejectedResponses(t *testing.T) {
	cfg := newspaperFixtureConfig()
	cfg.Budget.Enabled, cfg.Budget.DailyLimitUSD, cfg.Budget.Enforcement = true, 1, "full"
	cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1000, OutputPerMillion: 1000}
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	defer tracker.Flush()
	client := &newspaperStoryTestClient{response: openai.ChatCompletionResponse{Model: "fixture-editor", Usage: openai.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonLength, Message: openai.ChatCompletionMessage{Content: `{"unfinished":`}}}}}
	s := &Server{Cfg: cfg, LLMClient: client, BudgetTracker: tracker}
	for _, guide := range []string{"Task: PLAN RESEARCH", "Write a sourced story"} {
		if _, err := s.newspaperCompletion(context.Background(), cfg, client, guide, "{}"); err == nil {
			t.Fatal("truncated JSON accepted")
		}
	}
	if spent := tracker.CategorySpendUSD("newspaper"); spent < .399 || spent > .401 {
		t.Fatalf("two completed calls must be charged once each: %f", spent)
	}
	tracker.RecordCostForCategory("chat", 1)
	if _, err := s.newspaperCompletion(context.Background(), cfg, client, "Task: PLAN RESEARCH", "{}"); err == nil || !strings.Contains(err.Error(), "spending") {
		t.Fatalf("budget not enforced: %v", err)
	}
	if spent := tracker.CategorySpendUSD("newspaper"); spent > .401 {
		t.Fatalf("blocked request charged: %f", spent)
	}
}

func TestNewspaperReservesFinalTimeForEditing(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science"}
	deps := newspaperFixtureIO(p)
	var reads atomic.Int32
	deps.Fetch = func(ctx context.Context, raw string) (*scraper.ScrapeResult, error) {
		if reads.Add(1) == 1 {
			return newspaperFixturePage(raw), nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	var editorStarted, editorFinished time.Time
	deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
		if strings.Contains(guide, "Task: PLAN RESEARCH") {
			return "{}", nil
		}
		editorStarted = time.Now()
		if err := newspaperWait(ctx, 450*time.Millisecond); err != nil {
			return "", err
		}
		editorFinished = time.Now()
		return newspaperFixtureComplete(ctx, guide, input)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 550*time.Millisecond)
	defer cancel()
	started := time.Now()
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(ctx, p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) != 1 || !draft.Partial || stats.Plans > 2 {
		t.Fatalf("reserve: stories=%d stats=%+v err=%v", len(draft.Stories), stats, err)
	}
	if editorStarted.IsZero() || editorFinished.Sub(started) < 440*time.Millisecond {
		t.Fatal("editor did not survive discovery cutoff")
	}
}

func TestNewspaperRejectsStaleExcludedAndCopiedPages(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science"}
	p.Exclusions = []string{"football"}
	deps := newspaperFixtureIO(p)
	deps.Search = func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error) {
		hits := []newspaperHit{}
		for _, name := range []string{"stale", "excluded", "accepted", "copy", "accepted?utm_source=x"} {
			hits = append(hits, newspaperHit{Title: "Search result " + name, URL: "https://source.example/" + name})
		}
		return newspaperSearchBatch{Hits: hits}, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		page := newspaperFixturePage("https://source.example/accepted")
		switch {
		case strings.HasSuffix(raw, "stale"):
			page.RawHTML = `<meta property="article:published_time" content="2000-01-01T00:00:00Z">`
		case strings.HasSuffix(raw, "excluded"):
			page.Markdown += " football"
		}
		return page, nil
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 10, 8, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) != 1 || stats.Rejected["stale"] != 1 || stats.Rejected["excluded"] != 1 || stats.Rejected["duplicate"] < 2 {
		t.Fatalf("filtering: %+v %+v %v", draft, stats, err)
	}
	deps.Spending = func() *newspaperSpendingBudget { return &newspaperSpendingBudget{Blocked: true} }
	deps.Search = func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error) {
		t.Error("search after cost budget exhausted")
		return newspaperSearchBatch{}, errors.New("blocked")
	}
	_, _ = runNewspaperResearch(context.Background(), p, time.Now(), 10, 8, deps, nil)
}
