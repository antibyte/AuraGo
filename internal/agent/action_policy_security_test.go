package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// These fixtures use an enabled first-party catalog entry and a trusted hook:
// policy and argument preparation run normally, while no Docker call is made.
func actionSecurityFixture(t *testing.T, handle func(context.Context, ToolCall) (string, bool)) *DispatchContext {
	t.Helper()
	previous := actionApprovals
	actionApprovals = newActionApprovalManager()
	t.Cleanup(func() {
		CancelActionApprovals(t.Name())
		actionApprovals = previous
	})
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Agent.ActionPolicy.Enabled = true
	dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ExecutionHooks: &ExecutionHooks{HandleTool: handle}}
	found := false
	for _, schema := range dispatchCatalogSchemas(dc) {
		found = found || schema.Function != nil && schema.Function.Name == "docker"
	}
	if !found {
		t.Fatal("fixture must expose the real enabled Docker catalog entry")
	}
	return dc
}

func actionSecurityCall() ToolCall {
	return ToolCall{Action: "docker", Operation: "remove", Params: map[string]interface{}{
		"operation": "remove", "container_id": "fixture-container", "options": map[string]interface{}{"labels": []interface{}{"original"}},
	}}
}

func queueSecurityApproval(t *testing.T, dc *DispatchContext, call *ToolCall) ActionApproval {
	t.Helper()
	result := DispatchToolCallResult(context.Background(), call, dc, "remove the fixture container")
	if result.Status != ToolResultDeferred || !strings.Contains(result.Output, "approval_required") {
		t.Fatalf("expected pending approval, got %+v", result)
	}
	pending := PendingActionApprovals(dc.SessionID)
	if len(pending) != 1 {
		t.Fatalf("pending count = %d, want 1", len(pending))
	}
	return pending[0]
}

func TestActionApprovalExecutesFrozenExactArgumentsOnce(t *testing.T) {
	var calls int
	var executed ToolCall
	dc := actionSecurityFixture(t, func(_ context.Context, call ToolCall) (string, bool) {
		calls++
		executed = call
		return `{"status":"success"}`, true
	})
	call := actionSecurityCall()
	approval := queueSecurityApproval(t, dc, &call)
	duplicate := actionSecurityCall()
	if again := queueSecurityApproval(t, dc, &duplicate); again.ID != approval.ID {
		t.Fatal("an identical pending call was duplicated")
	}
	if calls != 0 {
		t.Fatal("requesting approval executed the hook")
	}
	if _, err := DecideActionApproval(context.Background(), "another-session", approval.ID, "approve"); !errors.Is(err, ErrActionApprovalUnavailable) {
		t.Fatalf("cross-session approval accepted: %v", err)
	}
	// Mutating the caller's nested maps after proposal cannot alter the leased call.
	call.Operation = "restart"
	call.Params["container_id"] = "different-container"
	call.Params["options"].(map[string]interface{})["labels"].([]interface{})[0] = "changed"
	result, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve")
	if err != nil || result.Status != ToolResultSuccess || calls != 1 {
		t.Fatalf("approval did not execute exactly once: result=%+v err=%v calls=%d", result, err, calls)
	}
	if executed.Operation != "remove" || executed.Params["container_id"] != "fixture-container" || !reflect.DeepEqual(executed.Params["options"], map[string]interface{}{"labels": []interface{}{"original"}}) {
		t.Fatalf("approved arguments changed: %+v", executed.Params)
	}
	if DescribeAction(executed, dc).ArgumentsHash != approval.Action.ArgumentsHash {
		t.Fatal("executed call differs from the exact approved hash")
	}
	if _, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve"); !errors.Is(err, ErrActionApprovalUnavailable) || calls != 1 {
		t.Fatalf("a consumed approval was replayed: err=%v calls=%d", err, calls)
	}
}

func TestActionApprovalGrantCannotOverrideHardDenials(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*DispatchContext)
	}{
		{"disabled capability", func(dc *DispatchContext) { dc.Cfg.Docker.Enabled = false }},
		{"readonly integration", func(dc *DispatchContext) { dc.Cfg.Docker.ReadOnly = true }},
		{"empty inherited scope", func(dc *DispatchContext) { dc.ToolScopeRestricted = true; dc.AllowedTools = map[string]struct{}{} }},
		{"explicit deny precedes allow", func(dc *DispatchContext) {
			rule := config.ActionPolicyRule{Tool: "docker", Operation: "remove", Target: "*"}
			dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{rule}
			dc.Cfg.Agent.ActionPolicy.Deny = []config.ActionPolicyRule{rule}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			call := prepareToolCall(actionSecurityCall(), dc)
			grant := approvedActionGrant{SessionID: dc.SessionID, ArgumentsHash: DescribeAction(call, dc).ArgumentsHash}
			test.change(dc)
			result := DispatchToolCallResult(context.WithValue(context.Background(), approvedActionKey{}, grant), &call, dc, "fixture")
			if result.Status != ToolResultDenied || calls != 0 || len(PendingActionApprovals(dc.SessionID)) != 0 {
				t.Fatalf("approval overrode a hard denial: result=%+v calls=%d", result, calls)
			}
		})
	}
}

func TestActionApprovalRevalidatesPublishedConfig(t *testing.T) {
	for _, mode := range []string{"disable", "readonly", "deny"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			initial := dc.Cfg
			current := *initial
			initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
			call := actionSecurityCall()
			approval := queueSecurityApproval(t, dc, &call)
			switch mode {
			case "disable":
				current.Docker.Enabled = false
			case "readonly":
				current.Docker.ReadOnly = true
			case "deny":
				current.Agent.ActionPolicy.Deny = []config.ActionPolicyRule{{Tool: "docker", Operation: "remove", Target: "*"}}
			}
			result, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve")
			if err != nil || result.Status != ToolResultDenied || calls != 0 {
				t.Fatalf("stale approval retained revoked authority: result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}

func TestActionApprovalRejectStopAndExpiryAreTerminal(t *testing.T) {
	for _, mode := range []string{"reject", "stop", "expiry"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			now := time.Now()
			actionApprovals.now = func() time.Time { return now }
			call := actionSecurityCall()
			approval := queueSecurityApproval(t, dc, &call)
			switch mode {
			case "reject":
				result, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "reject")
				if err != nil || result.Status != ToolResultDenied {
					t.Fatalf("reject: %+v %v", result, err)
				}
			case "stop":
				CancelActionApprovals(dc.SessionID)
			case "expiry":
				now = approval.ExpiresAt
			}
			if len(PendingActionApprovals(dc.SessionID)) != 0 {
				t.Fatal("terminal approval remained pending")
			}
			if _, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve"); !errors.Is(err, ErrActionApprovalUnavailable) || calls != 0 {
				t.Fatalf("terminal approval executed: err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestActionApprovalConcurrentDecisionAndStopCancelRunningCall(t *testing.T) {
	started := make(chan struct{})
	var calls atomic.Int32
	dc := actionSecurityFixture(t, func(ctx context.Context, _ ToolCall) (string, bool) {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		return `{"status":"error","message":"cancelled fixture"}`, true
	})
	call := actionSecurityCall()
	approval := queueSecurityApproval(t, dc, &call)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := DecideActionApproval(ctx, dc.SessionID, approval.ID, "approve")
		if err == nil && result.Status != ToolResultCancelled {
			err = errors.New("running approval did not report cancellation")
		}
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("approved fixture did not enter hook")
	}
	if _, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve"); !errors.Is(err, ErrActionApprovalUnavailable) {
		t.Fatalf("concurrent decision was accepted: %v", err)
	}
	CancelActionApprovals(dc.SessionID)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not cancel running approval")
	}
	if calls.Load() != 1 {
		t.Fatalf("hook calls = %d, want 1", calls.Load())
	}
}

func TestActionPolicyPreviewAndUnattendedCallsNeverCreatePendingApproval(t *testing.T) {
	calls := 0
	dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	call := actionSecurityCall()
	if _, decision := PreviewActionPolicy(call, dc); decision != "rule_required" {
		t.Fatalf("preview decision = %q", decision)
	}
	dc.MissionID = "fixture-mission"
	result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
	if result.Status != ToolResultDenied || !strings.Contains(result.Output, "action_rule_required") || calls != 0 || len(PendingActionApprovals(dc.SessionID)) != 0 {
		t.Fatalf("unattended/preview execution waited for approval: %+v calls=%d", result, calls)
	}
}

func TestActionPolicyExactRuleBindsArgumentsTargetAndMission(t *testing.T) {
	calls := 0
	dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	dc.MissionID = "fixture-mission"
	call := prepareToolCall(actionSecurityCall(), dc)
	action := DescribeAction(call, dc)
	dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: action.Tool, Operation: action.Operation, Target: action.Target, RunType: action.RunType, ScopeID: action.ScopeID, ArgumentsHash: action.ArgumentsHash}}
	if result := DispatchToolCallResult(context.Background(), &call, dc, "fixture"); result.Status != ToolResultSuccess || calls != 1 {
		t.Fatalf("exact mission rule failed: %+v calls=%d", result, calls)
	}
	for _, mode := range []string{"arguments", "target", "mission", "run type"} {
		changed := actionSecurityCall()
		local := *dc
		switch mode {
		case "arguments":
			changed.Params["options"].(map[string]interface{})["labels"] = []interface{}{"different"}
		case "target":
			changed.Params["container_id"] = "different-container"
		case "mission":
			local.MissionID = "different-mission"
		case "run type":
			local.IsMaintenance = true
		}
		result := DispatchToolCallResult(context.Background(), &changed, &local, "fixture")
		if result.Status != ToolResultDenied || calls != 1 {
			t.Fatalf("exact rule widened across %s: %+v calls=%d", mode, result, calls)
		}
	}
}

func TestQueuedCodeRevalidatesCurrentPolicyAndKeepsOriginalScope(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowPython, cfg.Agent.ActionPolicy.Enabled, cfg.Sandbox.Enabled = true, true, true
	current := cfg.Clone()
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, current }
	call := ToolCall{Action: "execute_python", Code: "print('fixture')", Background: true}
	data, err := json.Marshal(call)
	if err != nil {
		t.Fatal(err)
	}
	action := tools.BackgroundCodeAction{Call: data, SessionID: t.Name(), MessageSource: "web_chat", AllowedTools: []string{"execute_python"}, ToolScopeRestricted: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := ValidateBackgroundCodeAction(context.Background(), cfg, logger, action); err != nil {
		t.Fatalf("isolated code should be allowed initially: %v", err)
	}
	current.Agent.ActionPolicy.Deny = []config.ActionPolicyRule{{Tool: "execute_python", Operation: "execute", Target: "host:local", RunType: "background"}}
	if err := ValidateBackgroundCodeAction(context.Background(), cfg, logger, action); !errors.Is(err, tools.ErrBackgroundTaskBlocked) {
		t.Fatalf("queued action retained revoked permission: %v", err)
	}
	current.Agent.ActionPolicy.Deny = nil
	current.Agent.AllowPython = false
	if err := ValidateBackgroundCodeAction(context.Background(), cfg, logger, action); !errors.Is(err, tools.ErrBackgroundTaskBlocked) {
		t.Fatalf("disabled Python retained permission: %v", err)
	}
	current.Agent.ActionPolicy.Enabled = false
	if err := ValidateBackgroundCodeAction(context.Background(), cfg, logger, action); !errors.Is(err, tools.ErrBackgroundTaskBlocked) {
		t.Fatalf("disabled action rules overrode a hard capability gate: %v", err)
	}
	current.Agent.ActionPolicy.Enabled = true
	current.Agent.AllowPython = true
	action.AllowedTools = nil
	if err := ValidateBackgroundCodeAction(context.Background(), cfg, logger, action); !errors.Is(err, tools.ErrBackgroundTaskBlocked) {
		t.Fatalf("empty original scope was widened: %v", err)
	}
	if len(PendingActionApprovals(action.SessionID)) != 0 {
		t.Fatal("queued validation created a human approval request")
	}
}
