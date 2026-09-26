package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestTregDirectInvokePolicyParity(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		for _, op := range []string{"read", "create", "update", "delete"} {
			for _, denial := range []string{"enabled", "network", "grant", "readonly", "role", "mismatch", "token", "revoke"} {
				t.Run(op+"/"+denial+map[bool]string{false: "/direct", true: "/invoke"}[wrapped], func(t *testing.T) {
					cfg := &config.Config{}
					cfg.Treg = config.DefaultTregConfig()
					cfg.Treg.Enabled = true
					cfg.Treg.ReadOnly = false
					cfg.Agent.AllowNetworkRequests = true
					cfg.Treg.AllowedEndpoints = []config.TregEndpointGrant{{EndpointID: "p.x", Method: "POST", Path: "/x", Operation: op}}
					dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
					args := map[string]interface{}{"operation": op, "endpoint_id": "p.x", "parameters_json": "{}"}
					switch denial {
					case "enabled":
						cfg.Treg.Enabled = false
					case "network":
						cfg.Agent.AllowNetworkRequests = false
					case "grant":
						cfg.Treg.AllowedEndpoints = nil
					case "readonly":
						cfg.Treg.ReadOnly = true
					case "role":
						dc.CoAgentSpecialist = "security"
					case "mismatch":
						args["operation"] = "invalid"
					case "revoke":
						current := *cfg
						current.Treg.AllowedEndpoints = nil
						cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &current }
					}
					call := ToolCall{Action: "treg_call", Operation: args["operation"].(string), Params: args}
					if wrapped {
						setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
						t.Cleanup(func() { ClearDiscoverToolsState(t.Name()) })
						call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "treg_call", "arguments": args}}
					}
					result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
					want := ToolResultDenied
					if denial == "token" || (op == "read" && (denial == "role" || denial == "readonly")) {
						want = ToolResultNeedsSetup
					}
					if result.Status != want {
						t.Fatalf("wanted %s: %+v", want, result)
					}
				})
			}
		}
	}
}

func TestTregLiveCostAndGrantsNeverWiden(t *testing.T) {
	initial := &config.Config{}
	initial.Treg = config.DefaultTregConfig()
	initial.Treg.Enabled = true
	current := *initial
	current.Treg.MaxCallCostMicro = 0
	initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
	merged, ok := dispatchAuthorization(initial)
	if !ok || merged.Treg.MaxCallCostMicro != 0 || initial.Treg.MaxCallCostMicro != 1_000_000 {
		t.Fatal("cost restriction or snapshot lost")
	}
	current.Treg.MaxCallCostMicro = 2_000_000
	merged, ok = dispatchAuthorization(initial)
	if !ok || merged.Treg.MaxCallCostMicro != 1_000_000 {
		t.Fatal("cost grant widened")
	}
	current.Treg.AllowedEndpoints = []config.TregEndpointGrant{{EndpointID: "p.x", Method: "GET", Path: "/x", Operation: "read"}}
	if _, ok = dispatchAuthorization(initial); ok {
		t.Fatal("endpoint grant widened running request")
	}
}

func TestTregUnknownSentOutcomeSurvivesCancellation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Treg.Enabled, cfg.Agent.AllowNetworkRequests = true, true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ExecutionHooks: &ExecutionHooks{HandleTool: func(ctx context.Context, call ToolCall) (string, bool) {
		cancel()
		setToolOutcome(ctx, ToolResultUnknown)
		return externalToolOutput(ctx, `{"status":"unknown","call_id":"fixture","data":{"status":"success"}}`), true
	}}}
	call := ToolCall{Action: "treg_call", Operation: "create"}
	result := DispatchToolCallResult(ctx, &call, dc, "fixture")
	if result.Status != ToolResultUnknown {
		t.Fatalf("ambiguous sent request became %s", result.Status)
	}
}

func TestTregDiscoveryAndTrustedOutcome(t *testing.T) {
	if (ToolFeatureFlags{}).Key() == (ToolFeatureFlags{TregEnabled: true}).Key() {
		t.Fatal("schema cache collision")
	}
	for _, enabled := range []bool{false, true} {
		count := 0
		for _, schema := range builtinToolSchemas(ToolFeatureFlags{TregEnabled: enabled}) {
			if strings.HasPrefix(schema.Function.Name, "treg_") {
				count++
			}
		}
		if (enabled && count != 3) || (!enabled && count != 0) {
			t.Fatalf("wrong treg tool count %d", count)
		}
	}
	for _, fixture := range []struct {
		raw  string
		want ToolResultStatus
	}{{`{"status":"pending","data":{"status":"success"}}`, ToolResultDeferred}, {`{"status":"unknown","data":{"status":"success"}}`, ToolResultUnknown}, {`{"status":"error","data":{"status":"success"}}`, ToolResultFailed}} {
		if got := classifyLegacyToolResult(fixture.raw); got != fixture.want {
			t.Fatalf("untrusted status: %s", got)
		}
	}
}
