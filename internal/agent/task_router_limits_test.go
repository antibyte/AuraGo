package agent

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/decisions"
)

func TestTaskRouterDecisionsReportsBusyAndScoreTimeout(t *testing.T) {
	for _, tc := range []struct{ code, reason string }{{decisions.CodeQueueFull, "decisions_busy"}, {decisions.CodeRuntimeBusy, "decisions_busy"}, {decisions.CodeRuntimeScoreTimeout, "decisions_timeout"}} {
		t.Run(tc.code, func(t *testing.T) {
			helperCalls := 0
			_, run, req := taskRouterHelperFixture(t, func(w http.ResponseWriter, r *http.Request) { helperCalls++ })
			fake := &fakeTaskDecisions{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
				return decisions.Response{}, decisions.NewError(tc.code, nil)
			}}
			run.Config.Decisions.Enabled = true
			run.RoutingDecisions = fake
			if err := PrepareTaskRouting(t.Context(), &req, &run); err != nil {
				t.Fatal(err)
			}
			if run.TaskRouting.Reason != tc.reason || fake.calls != 1 || helperCalls != 0 {
				t.Fatalf("route=%+v calls=%d helper=%d", run.TaskRouting, fake.calls, helperCalls)
			}
		})
	}
}

func TestTaskRouterPreviewCooldownSurvivesTextChangesAndExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resetTaskRouterTestState(t)
		_, original, request := taskRouterFixture()
		original.UserIntent = "Bitte mach daraus etwas Passendes"
		original.SessionID = "router-preview"
		original.TaskRoutingPreview = true
		original.TaskRoutingClassifier = "decisions"
		fake := &fakeTaskDecisions{available: true}
		original.Config.Decisions.Enabled = true
		original.RoutingDecisions = fake
		preview := func(classifier, text string) *TaskRoutingDecision {
			run, req := original, request
			run.TaskRoutingClassifier, run.UserIntent = classifier, text
			if err := PrepareTaskRouting(context.Background(), &req, &run); err != nil {
				t.Fatal(err)
			}
			return run.TaskRouting
		}
		first := preview("decisions", original.UserIntent)
		if first.Source != "decisions" || first.RetryAfterSeconds != 60 || fake.calls != 1 {
			t.Fatalf("first preview: %+v", first)
		}
		second := preview("decisions", "Mach es bitte anders passend")
		if second.Reason != "decisions_cooldown" || second.RetryAfterSeconds != 60 || fake.calls != 1 {
			t.Fatalf("second preview: %+v", second)
		}
		local := preview("local", "Noch eine passende Variante bitte")
		if local.RetryAfterSeconds != 60 || fake.calls != 1 {
			t.Fatalf("local preview bypassed or consumed cooldown: %+v", local)
		}
		time.Sleep(60 * time.Second)
		third := preview("decisions", original.UserIntent)
		if third.Source != "decisions" || fake.calls != 2 {
			t.Fatalf("expired cooldown: %+v", third)
		}
		taskRouterState.Lock()
		taskRouterState.tokens = 0
		taskRouterState.refilled = time.Now()
		taskRouterState.Unlock()
		original.SessionID = "other-session"
		limited := preview("decisions", original.UserIntent)
		if limited.Reason != "decisions_limited" || limited.RetryAfterSeconds != 0 || fake.calls != 2 {
			t.Fatalf("quota confused with cooldown: %+v", limited)
		}
	})
}
