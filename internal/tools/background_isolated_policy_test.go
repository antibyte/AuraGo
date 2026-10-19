package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIsolatedBackgroundRechecksPolicyAfterDurableAdmission(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{AllowPython: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	for _, mode := range []string{"revoked", "missing gate", "legacy identity", "canceled during gate"} {
		t.Run(mode, func(t *testing.T) {
			dir := tempSystemTaskDir(t)
			mgr := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
			executed := 0
			mgr.SetIsolatedCodeExecutor(func(context.Context, string, string, time.Duration) (string, error) {
				executed++
				return "fixture", nil
			})
			owner := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowPython: true})
			call := json.RawMessage(`{"action":"execute_python","code":"print('fixture')"}`)
			if mode != "legacy identity" {
				owner = WithBackgroundCodeAction(owner, BackgroundCodeAction{Call: call, SessionID: "fixture-session", MissionID: "fixture-mission", ToolScopeRestricted: true, AllowedTools: []string{"execute_python"}})
			}
			task, err := mgr.ScheduleIsolatedCodeContext(owner, "print('fixture')", "python", BackgroundTaskScheduleOptions{ExecutionKind: "python", MaxRetries: 5})
			if err != nil {
				t.Fatal(err)
			}
			// Reload a durable queue instead of retaining a caller context or lease.
			if err := mgr.Close(); err != nil {
				t.Fatal(err)
			}
			mgr = NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
			defer mgr.Close()
			mgr.SetIsolatedCodeExecutor(func(context.Context, string, string, time.Duration) (string, error) {
				executed++
				return "fixture", nil
			})
			checks := 0
			if mode != "missing gate" {
				mgr.SetIsolatedCodeAuthorizer(func(ctx context.Context, action BackgroundCodeAction) error {
					checks++
					if action.SessionID != "fixture-session" || action.MissionID != "fixture-mission" || !action.ToolScopeRestricted || len(action.AllowedTools) != 1 || action.AllowedTools[0] != "execute_python" || string(action.Call) != string(call) {
						t.Fatal("durable canonical action or scope changed")
					}
					if mode == "canceled during gate" {
						mgr.CancelTask(task.ID)
						return nil
					}
					return errors.New("new administrator deny rule")
				})
			}
			mgr.processDueTasks()
			mgr.processDueTasks()
			stored, _ := mgr.GetTask(task.ID)
			want := BackgroundTaskStatusBlocked
			if mode == "canceled during gate" {
				want = BackgroundTaskStatusCanceled
			}
			if executed != 0 || stored.Status != want || stored.ExecutionID != "" || stored.RetryCount != 0 {
				t.Fatalf("executed=%d task=%+v", executed, stored)
			}
			if want == BackgroundTaskStatusBlocked && !strings.HasPrefix(stored.LastError, "action_rule_required:") {
				t.Fatalf("missing visible blocked reason: %q", stored.LastError)
			}
			if mode == "revoked" && checks != 1 {
				t.Fatalf("policy checks=%d, want 1 without retry", checks)
			}
		})
	}
}

func TestBackgroundCodeActionSnapshotIsFrozenAndBounded(t *testing.T) {
	action := BackgroundCodeAction{Call: json.RawMessage(`{"action":"execute_python"}`), SessionID: "fixture", AllowedTools: []string{"execute_python"}}
	ctx := WithBackgroundCodeAction(context.Background(), action)
	action.Call[0] = '!'
	action.AllowedTools[0] = "execute_shell"
	frozen := ctx.Value(backgroundCodeActionKey{}).(BackgroundCodeAction)
	if !validBackgroundCodeAction(&frozen) || frozen.AllowedTools[0] != "execute_python" {
		t.Fatal("background action snapshot retained mutable caller data")
	}
	frozen.Call = json.RawMessage(`{"action":"execute_python","code":"` + strings.Repeat("x", 1<<20) + `"}`)
	if validBackgroundCodeAction(&frozen) {
		t.Fatal("oversized code action accepted")
	}
	frozen.Call = json.RawMessage(`{"action":"execute_python"}`)
	frozen.AllowedTools = []string{strings.Repeat("x", 65<<10)}
	if validBackgroundCodeAction(&frozen) {
		t.Fatal("oversized scope metadata accepted")
	}
}
