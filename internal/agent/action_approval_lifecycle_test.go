package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"

	"github.com/sashabaranov/go-openai"
)

func TestMainLoopApprovalWaitsBeforeStartAndRechecksScope(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "execute once", true: "scope revoked"}[revoke], func(t *testing.T) {
			previous := actionApprovals
			actionApprovals = newActionApprovalManager()
			t.Cleanup(func() { CancelActionApprovals(t.Name()); actionApprovals = previous })
			run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
			defer cleanup()
			run.SuppressTurnSideEffects = true
			run.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
			run.Config.Docker.Enabled = true
			run.Config.Agent.ActionPolicy.Enabled = true
			calls := 0
			run.ExecutionHooks = &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
				calls++
				return `{"status":"success"}`, true
			}}
			run.LLMClient = &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
				{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
					ID: "approval-remove", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "docker", Arguments: `{"operation":"remove","container_id":"fixture-container"}`},
				}}}}}},
				{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Waiting for your decision."}}}},
			}}
			broker := &typedActionLedgerCaptureBroker{}
			_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Remove fixture-container."}}}, run, false, broker)
			if err != nil {
				t.Fatal(err)
			}
			pending := PendingActionApprovals(run.SessionID)
			if len(pending) != 1 || calls != 0 {
				t.Fatalf("pending=%+v calls=%d", pending, calls)
			}
			item := actionApprovals.entries[pending[0].ID]
			if got := strings.Join(item.event.StateHistory, ","); got != "proposed,accepted,needs_human_approval" {
				t.Fatalf("pre-execution state history = %s", got)
			}
			if revoke {
				item.dc.ToolScopeRestricted = true
				item.dc.AllowedTools = map[string]struct{}{}
			}
			result, err := DecideActionApproval(context.Background(), run.SessionID, pending[0].ID, "approve")
			if err != nil {
				t.Fatal(err)
			}
			if revoke {
				if result.Status != ToolResultDenied || calls != 0 {
					t.Fatalf("revoked approval executed: %+v calls=%d", result, calls)
				}
			} else if result.Status != ToolResultSuccess || calls != 1 {
				t.Fatalf("approved action: %+v calls=%d", result, calls)
			}
			if _, err := DecideActionApproval(context.Background(), run.SessionID, pending[0].ID, "approve"); !errors.Is(err, ErrActionApprovalUnavailable) {
				t.Fatalf("replay = %v", err)
			}
			for _, wire := range broker.typedEvents {
				event, ok := wire.payload.(AgentActionEvent)
				if ok && event.ID == pending[0].ID && event.State == string(AgentActionStateStarted) && revoke {
					t.Fatal("revoked action was marked started")
				}
			}
		})
	}
}

func TestActionPolicyDisabledStillEnforcesHardGatesInPreviewAndDispatch(t *testing.T) {
	for _, mode := range []string{"disabled", "readonly"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.Agent.ActionPolicy.Enabled = false
			if mode == "disabled" {
				dc.Cfg.Docker.Enabled = false
			} else {
				dc.Cfg.Docker.ReadOnly = true
			}
			call := actionSecurityCall()
			if _, decision := PreviewActionPolicy(call, dc); decision != "denied" {
				t.Fatalf("preview = %s", decision)
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if result.Status != ToolResultDenied || calls != 0 || len(PendingActionApprovals(dc.SessionID)) != 0 {
				t.Fatalf("hard gate bypass: %+v calls=%d", result, calls)
			}
		})
	}
}

func TestActionPolicyUsesActualRuntimeHandlesInCatalog(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "policy off", true: "policy on"}[enabled], func(t *testing.T) {
			dc := actionSecurityFixture(t, nil)
			dc.ExecutionHooks = nil // Exercise the real read-only Contacts handler.
			dc.Cfg.Agent.ActionPolicy.Enabled = enabled
			dc.Cfg.Tools.Contacts.Enabled = true
			db, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			dc.ContactsDB = db
			call := ToolCall{Action: "address_book", Operation: "list"}
			if _, decision := PreviewActionPolicy(call, dc); decision != "allowed" {
				t.Fatalf("available Contacts preview = %s", decision)
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "list contacts")
			if result.Status != ToolResultSuccess {
				t.Fatalf("available Contacts rejected: %+v", result)
			}
			dc.ContactsDB = nil
			if _, decision := PreviewActionPolicy(call, dc); decision != "denied" {
				t.Fatalf("missing Contacts runtime preview = %s", decision)
			}
		})
	}
}

func TestActionApprovalExpiresWhileWaitingForSessionGate(t *testing.T) {
	calls := atomic.Int32{}
	dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls.Add(1); return `{"status":"success"}`, true })
	run, _, cleanup := newPromptPipelineTestRunConfig(t, dc.SessionID, "web_chat")
	defer cleanup()
	dc.ShortTermMem = run.ShortTermMem
	call := actionSecurityCall()
	approval := queueSecurityApproval(t, dc, &call)
	// A short actual expiry proves that waiting is bounded without releasing the
	// gate. It is not an execution timeout and must not replace Stop's context.
	actionApprovals.mu.Lock()
	actionApprovals.entries[approval.ID].view.ExpiresAt = time.Now().Add(100 * time.Millisecond)
	actionApprovals.mu.Unlock()
	release, err := dc.ShortTermMem.LockSessionTurn(context.Background(), dc.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := make(chan error, 1)
	go func() {
		_, err := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve")
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrActionApprovalUnavailable) {
			t.Fatalf("gate expiry = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expiry did not cancel the queued decision")
	}
	if calls.Load() != 0 {
		t.Fatal("expired action executed")
	}
}

func TestActionApprovalFinalStartBoundaryRechecksExpiry(t *testing.T) {
	calls := 0
	dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	call := prepareToolCall(actionSecurityCall(), dc)
	var clock atomic.Int64
	clock.Store(time.Now().UnixNano())
	grant := approvedActionGrant{SessionID: dc.SessionID, ArgumentsHash: DescribeAction(call, dc).ArgumentsHash, ExpiresAt: time.Unix(0, clock.Load()).Add(time.Minute), now: func() time.Time { return time.Unix(0, clock.Load()) }}
	ctx := context.WithValue(context.Background(), approvedActionKey{}, grant)
	if refusal := checkDispatchActionPolicy(ctx, call, dc); refusal != nil {
		t.Fatalf("valid approval refused: %+v", refusal)
	}
	clock.Store(grant.ExpiresAt.UnixNano())
	// Even a publication that disables the supplementary policy cannot extend
	// an already leased approval. The final start guard owns that deadline.
	dc.Cfg.Agent.ActionPolicy.Enabled = false
	result := dispatchInner(ctx, call, dc)
	if !strings.Contains(result, "approval_expired") || calls != 0 {
		t.Fatalf("expired start boundary: %s calls=%d", result, calls)
	}
}

func TestExpiredParentApprovalDoesNotBlockIndependentlyAuthorizedChild(t *testing.T) {
	calls := 0
	dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) {
		calls++
		return `{"status":"success"}`, true
	})
	parent := prepareToolCall(actionSecurityCall(), dc)
	grant := approvedActionGrant{SessionID: dc.SessionID, ArgumentsHash: DescribeAction(parent, dc).ArgumentsHash, ExpiresAt: time.Now().Add(-time.Minute)}
	child := actionSecurityCall()
	child.Params["container_id"] = "independently-authorized-child"
	child = prepareToolCall(child, dc)
	action := DescribeAction(child, dc)
	dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: action.Tool, Operation: action.Operation, Target: action.Target, ArgumentsHash: action.ArgumentsHash}}
	ledger := newAgentActionLedger(nil, dc.Logger, nil, dc.SessionID, "web_chat")
	event, err := ledger.ProposeTool("child-turn", child)
	if err != nil {
		t.Fatal(err)
	}
	event, err = ledger.Accept(event, "independently authorized child")
	if err != nil {
		t.Fatal(err)
	}
	dc.actionLedger, dc.actionEvent = ledger, &event
	result := DispatchToolCallResult(context.WithValue(context.Background(), approvedActionKey{}, grant), &child, dc, "fixture")
	if result.Status != ToolResultSuccess || calls != 1 || event.State != string(AgentActionStateStarted) {
		t.Fatalf("parent expiry blocked authorized child: result=%+v calls=%d state=%s", result, calls, event.State)
	}
}

func TestStartedActionApprovalKeepsStopContextPastExpiry(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "finish", true: "Stop registered run"}[stop], func(t *testing.T) {
			started := make(chan struct{})
			finish := make(chan struct{})
			dc := actionSecurityFixture(t, func(ctx context.Context, _ ToolCall) (string, bool) {
				close(started)
				select {
				case <-finish:
					return `{"status":"success"}`, true
				case <-ctx.Done():
					return `{"status":"error"}`, true
				}
			})
			run, _, cleanup := newPromptPipelineTestRunConfig(t, dc.SessionID, "web_chat")
			defer cleanup()
			dc.ShortTermMem = run.ShortTermMem
			call := actionSecurityCall()
			approval := queueSecurityApproval(t, dc, &call)
			var clock atomic.Int64
			clock.Store(time.Now().UnixNano())
			actionApprovals.now = func() time.Time { return time.Unix(0, clock.Load()) }
			done := make(chan ToolDispatchResult, 1)
			go func() {
				result, _ := DecideActionApproval(context.Background(), dc.SessionID, approval.ID, "approve")
				done <- result
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("call did not start")
			}
			clock.Store(approval.ExpiresAt.UnixNano())
			want := ToolResultSuccess
			if stop {
				dc.ShortTermMem.CancelSessionRuns(dc.SessionID)
				want = ToolResultCancelled
			} else {
				close(finish)
			}
			select {
			case result := <-done:
				if result.Status != want {
					t.Fatalf("started call: %+v want %s", result, want)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("call did not finish")
			}
		})
	}
}
