package agent

import (
	"context"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
)

// yepAPITestServiceSetters enables or disables one native YepAPI tool family.
var yepAPITestServiceSetters = map[string]func(*config.Config, bool){
	"yepapi_seo":       func(c *config.Config, v bool) { c.YepAPI.SEO.Enabled = v },
	"yepapi_serp":      func(c *config.Config, v bool) { c.YepAPI.SERP.Enabled = v },
	"yepapi_scrape":    func(c *config.Config, v bool) { c.YepAPI.Scraping.Enabled = v },
	"yepapi_youtube":   func(c *config.Config, v bool) { c.YepAPI.YouTube.Enabled = v },
	"yepapi_tiktok":    func(c *config.Config, v bool) { c.YepAPI.TikTok.Enabled = v },
	"yepapi_instagram": func(c *config.Config, v bool) { c.YepAPI.Instagram.Enabled = v },
	"yepapi_amazon":    func(c *config.Config, v bool) { c.YepAPI.Amazon.Enabled = v },
}

// yepAPIDispatchFixture builds a config, a budget tracker and a dispatch
// context for one YepAPI tool family. The API base URL points at a local
// server that answers every path with a successful envelope.
func yepAPIDispatchFixture(t *testing.T, tool string, enableService bool) (*config.Config, *budget.Tracker, *DispatchContext) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"domain":"example.com"}}`))
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{}
	cfg.YepAPI.Enabled = true
	cfg.YepAPI.BaseURL = server.URL
	cfg.Providers = []config.ProviderEntry{
		{ID: "yepapi_test", Type: "yepapi", APIKey: "test_key"},
	}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 10

	setter, ok := yepAPITestServiceSetters[tool]
	if !ok {
		t.Fatalf("unknown YepAPI tool %q", tool)
	}
	setter(cfg, enableService)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := budget.NewTracker(cfg, logger, t.TempDir())
	t.Cleanup(tracker.Flush)

	return cfg, tracker, &DispatchContext{Cfg: cfg, Logger: logger, BudgetTracker: tracker}
}

func yepAPITestCall(tool, operation string) ToolCall {
	return ToolCall{
		Action:    tool,
		Operation: operation,
		Params:    map[string]interface{}{"domain": "example.com", "query": "example"},
	}
}

func TestDispatchYepAPIGates(t *testing.T) {
	t.Run("service_disabled", func(t *testing.T) {
		_, _, dc := yepAPIDispatchFixture(t, "yepapi_seo", false)
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `Tool Output: {"status":"error","message":"YepAPI SEO is disabled. Enable it in Settings > YepAPI > SEO."}`
		if out != want {
			t.Fatalf("out = %q, want %q", out, want)
		}
	})

	t.Run("globally_disabled", func(t *testing.T) {
		cfg, _, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.YepAPI.Enabled = false
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `Tool Output: {"status":"error","message":"YepAPI SEO is disabled. Enable it in Settings > YepAPI > SEO."}`
		if out != want {
			t.Fatalf("out = %q, want %q", out, want)
		}
	})

	t.Run("budget_blocked", func(t *testing.T) {
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.Budget.Enforcement = "full"
		cfg.Budget.DailyLimitUSD = 0.0001
		tracker.RecordCostForCategory("yepapi", 1)

		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `Tool Output: {"status":"error","message":"Daily budget exceeded. YepAPI calls are blocked until the budget resets."}`
		if out != want {
			t.Fatalf("out = %q, want %q", out, want)
		}
	})

	t.Run("missing_api_key", func(t *testing.T) {
		// This also guards the nil-vault path: a typed-nil *security.Vault must
		// not reach the SecretReader interface, because ReadSecret would panic
		// on the nil receiver.
		cfg, _, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.Providers = nil

		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "no YepAPI API key found") {
			t.Fatalf("out = %q, want the missing-key message", out)
		}
	})

	t.Run("successful_call", func(t *testing.T) {
		_, _, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("out = %q, want a success envelope", out)
		}
	})
}

func TestDispatchYepAPIRecordsCostOnSuccess(t *testing.T) {
	_, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
	out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("out = %q, want a success envelope", out)
	}
	if got := tracker.CategorySpendUSD("yepapi"); math.Abs(got-0.08) > 1e-9 {
		t.Fatalf("yepapi spend = %v, want 0.08", got)
	}
}

func TestDispatchYepAPIDoesNotRecordCostWhenBlocked(t *testing.T) {
	cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
	cfg.Budget.Enforcement = "full"
	cfg.Budget.DailyLimitUSD = 0.0001
	tracker.RecordCostForCategory("yepapi", 1)
	before := tracker.CategorySpendUSD("yepapi")

	if _, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tracker.CategorySpendUSD("yepapi"); got != before {
		t.Fatalf("yepapi spend = %v, want %v (a blocked call must not be charged)", got, before)
	}
}

// The cost table is the single source of truth for the per-call budget
// charge of every native YepAPI tool.
func TestYepAPICostTableMatchesDocumentedValues(t *testing.T) {
	want := map[string]float64{
		"yepapi_seo":       0.08,
		"yepapi_serp":      0.01,
		"yepapi_scrape":    0.02,
		"yepapi_youtube":   0.01,
		"yepapi_tiktok":    0.01,
		"yepapi_instagram": 0.01,
		"yepapi_amazon":    0.015,
	}
	if len(yepAPIServiceTable) != len(want) {
		t.Fatalf("yepAPIServiceTable has %d entries, want %d", len(yepAPIServiceTable), len(want))
	}
	for tool, cost := range want {
		svc, ok := yepAPIServiceTable[tool]
		if !ok {
			t.Fatalf("missing service spec for %s", tool)
		}
		if math.Abs(svc.cost-cost) > 1e-9 {
			t.Fatalf("%s cost = %v, want %v", tool, svc.cost, cost)
		}
		if svc.dispatch == nil {
			t.Fatalf("%s has no dispatcher", tool)
		}
	}
}

// The config must know every native tool family, so the shared dispatch path
// can resolve its label and enablement.
func TestYepAPIConfigKnowsEveryNativeTool(t *testing.T) {
	for tool := range yepAPIServiceTable {
		label, _, ok := config.YepAPIConfig{}.ServiceEnabled(tool)
		if !ok {
			t.Fatalf("config does not know %s", tool)
		}
		if label == "" {
			t.Fatalf("%s has no label", tool)
		}
	}
}

// The YepAPI quota caps the category spend independently of the global daily
// budget.
func TestDispatchYepAPIBlocksOnCategoryQuota(t *testing.T) {
	t.Run("quota_blocks_the_call", func(t *testing.T) {
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.YepAPI.BudgetQuotaPercent = 10
		// The daily limit is 10 USD, so a 10 percent quota is 1 USD.
		tracker.RecordCostForCategory("yepapi", 1)
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `Tool Output: {"status":"error","message":"YepAPI quota reached for today. Continue with other work or use another integration."}`
		if out != want {
			t.Fatalf("out = %q, want %q", out, want)
		}
	})
	t.Run("zero_quota_disables_the_gate", func(t *testing.T) {
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.YepAPI.BudgetQuotaPercent = 0
		tracker.RecordCostForCategory("yepapi", 5)
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("out = %q, want a success envelope: a quota of 0 must not block", out)
		}
	})
	t.Run("the_global_budget_message_comes_first", func(t *testing.T) {
		cfg, tracker, dc := yepAPIDispatchFixture(t, "yepapi_seo", true)
		cfg.Budget.Enforcement = "full"
		cfg.Budget.DailyLimitUSD = 0.0001
		cfg.YepAPI.BudgetQuotaPercent = 10
		tracker.RecordCostForCategory("yepapi", 1)
		out, err := dispatchYepAPI(context.Background(), yepAPITestCall("yepapi_seo", "domain_overview"), dc, "yepapi_seo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `Tool Output: {"status":"error","message":"Daily budget exceeded. YepAPI calls are blocked until the budget resets."}`
		if out != want {
			t.Fatalf("out = %q, want %q", out, want)
		}
	})
}
