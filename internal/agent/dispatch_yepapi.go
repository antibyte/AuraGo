package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// yepClient is the YepAPI HTTP client exported by internal/tools.
type yepClient = tools.YepAPIClient

// yepAPIDispatchFn performs one YepAPI call for a tool family.
type yepAPIDispatchFn func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error)

// yepAPIService describes one native YepAPI tool family.
type yepAPIService struct {
	tool     string
	label    string
	enabled  bool
	cost     float64
	dispatch yepAPIDispatchFn
}

// yepAPIServiceTable maps every native YepAPI tool to its dispatcher and the
// budget charge of one successful call in USD. The values are the per-call
// cost estimates documented in prompts/tools_manuals/yepapi.md.
var yepAPIServiceTable = map[string]yepAPIService{
	"yepapi_seo": {
		tool: "yepapi_seo",
		cost: 0.08,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPISEO(ctx, client, operation, params)
		},
	},
	"yepapi_serp": {
		tool: "yepapi_serp",
		cost: 0.01,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPISERP(ctx, client, operation, params)
		},
	},
	"yepapi_scrape": {
		tool: "yepapi_scrape",
		cost: 0.02,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPIScrape(ctx, client, operation, params)
		},
	},
	"yepapi_youtube": {
		tool: "yepapi_youtube",
		cost: 0.01,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPIYouTube(ctx, client, operation, params)
		},
	},
	"yepapi_tiktok": {
		tool: "yepapi_tiktok",
		cost: 0.01,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPITikTok(ctx, client, operation, params)
		},
	},
	"yepapi_instagram": {
		tool: "yepapi_instagram",
		cost: 0.01,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPIInstagram(ctx, client, operation, params)
		},
	},
	"yepapi_amazon": {
		tool: "yepapi_amazon",
		cost: 0.015,
		dispatch: func(ctx context.Context, client *yepClient, operation string, params map[string]interface{}) (string, error) {
			return tools.DispatchYepAPIAmazon(ctx, client, operation, params)
		},
	},
}

// yepAPIServiceFor returns the dispatch spec of a native YepAPI tool.
func yepAPIServiceFor(cfg *config.Config, tool string) (yepAPIService, bool) {
	svc, ok := yepAPIServiceTable[tool]
	if !ok {
		return yepAPIService{}, false
	}
	label, enabled, _ := cfg.YepAPI.ServiceEnabled(tool)
	svc.label = label
	svc.enabled = enabled
	return svc, true
}

// dispatchYepAPI runs the shared YepAPI path for every native tool family:
// the enable gate, the budget gate, key resolution, the call itself, the
// error envelope and the cost booking. It returns the tool output and, when
// the call itself failed, the underlying error.
func dispatchYepAPI(ctx context.Context, tc ToolCall, dc *DispatchContext, tool string) (string, error) {
	svc, ok := yepAPIServiceFor(dc.Cfg, tool)
	if !ok {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"unknown YepAPI tool %q"}`, tool), nil
	}
	yepCfg := dc.Cfg.YepAPI
	if !yepCfg.Enabled || !svc.enabled {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"YepAPI %s is disabled. Enable it in Settings > YepAPI > %s."}`, svc.label, svc.label), nil
	}
	if dc.BudgetTracker != nil && dc.BudgetTracker.IsBlocked("yepapi") {
		return `Tool Output: {"status":"error","message":"Daily budget exceeded. YepAPI calls are blocked until the budget resets."}`, nil
	}
	// A nil *security.Vault must not reach the SecretReader interface: a typed
	// nil pointer is not a nil interface and would panic inside ReadSecret.
	var secrets config.SecretReader
	if dc.Vault != nil {
		secrets = dc.Vault
	}
	apiKey, keyErr := tools.ResolveYepAPIKey(dc.Cfg, secrets)
	if keyErr != nil {
		b, _ := json.Marshal(keyErr.Error())
		return fmt.Sprintf(`Tool Output: {"status":"error","message":%s}`, b), nil
	}
	client := tools.GetYepAPIClient(apiKey, yepCfg.BaseURL)
	res, err := svc.dispatch(tools.WithYepAPILogger(ctx, dc.Logger), client, tc.Operation, tc.Params)
	if err != nil {
		b, _ := json.Marshal(err.Error())
		// A call that reached the provider may have been billed even though it
		// failed, so it is booked against the budget. A call that never left
		// the client cannot have been billed and stays free.
		if dc.BudgetTracker != nil && yepAPICallReachedProvider(err) {
			dc.BudgetTracker.RecordCostForCategory("yepapi", svc.cost)
		}
		return fmt.Sprintf(`Tool Output: {"status":"error","message":%s}`, b), err
	}
	if dc.BudgetTracker != nil {
		dc.BudgetTracker.RecordCostForCategory("yepapi", svc.cost)
	}
	return res, nil
}

// yepAPICallReachedProvider reports whether the failed call left the client
// and may therefore have been billed by the provider.
func yepAPICallReachedProvider(err error) bool {
	var callErr *tools.YepAPICallError
	if !errors.As(err, &callErr) {
		return false
	}
	return callErr.RequestSent
}
