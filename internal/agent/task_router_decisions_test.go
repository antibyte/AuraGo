package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/security"
	openai "github.com/sashabaranov/go-openai"
)

type fakeTaskDecisions struct {
	calls     int
	available bool
	version   byte
	evaluate  func(context.Context, decisions.Request) (decisions.Response, error)
}

func blockedBudgetTrackerForRouting(t *testing.T, cfg *config.Config) *budget.Tracker {
	t.Helper()
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = .001
	cfg.Budget.Enforcement = "full"
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	tracker.RecordCostForCategory("chat", 1)
	t.Cleanup(tracker.Flush)
	return tracker
}

func (f *fakeTaskDecisions) RoutingSnapshot(*config.Config) TaskDecisionSnapshot {
	version := f.version
	return TaskDecisionSnapshot{Available: f.available, Backend: "provider", Identity: [32]byte{version},
		Current: func() bool { return f.available && f.version == version },
		Evaluate: func(ctx context.Context, request decisions.Request) (decisions.Response, error) {
			f.calls++
			if f.evaluate != nil {
				return f.evaluate(ctx, request)
			}
			return taskDecisionReply("coding", .8), nil
		},
	}
}

func taskDecisionReply(area string, probability float64) decisions.Response {
	probabilities := map[string]float64{}
	for _, option := range taskDecisionRequest("").Questions[0].Options {
		probabilities[option.ID] = (1 - probability) / 9
	}
	probabilities[area] = probability
	return decisions.Response{Answers: map[string]decisions.Answer{"task_area": {Type: "choice", Decision: area, Probabilities: probabilities}}}
}

func TestTaskRouterDecisionsStrictCategories(t *testing.T) {
	for _, area := range config.LLMRouterAreas {
		t.Run(area, func(t *testing.T) {
			response := taskDecisionReply(area, .8)
			for _, option := range taskDecisionRequest("").Questions[0].Options {
				if option.ID == area {
					answer := response.Answers["task_area"]
					answer.Choice = option.Label
					response.Answers["task_area"] = answer
				}
			}
			got, err := parseTaskDecision(response)
			if err != nil || got.Area() != area {
				t.Fatalf("category: %+v %v", got, err)
			}
		})
	}
	for _, test := range []struct {
		name   string
		mutate func(*decisions.Response)
	}{
		{"unknown", func(r *decisions.Response) { *r = taskDecisionReply("unknown", .9) }},
		{"low", func(r *decisions.Response) { *r = taskDecisionReply("coding", .59) }},
		{"tie", func(r *decisions.Response) {
			*r = taskDecisionReply("coding", .5)
			for k := range r.Answers["task_area"].Probabilities {
				r.Answers["task_area"].Probabilities[k] = 0
			}
			r.Answers["task_area"].Probabilities["coding"] = .5
			r.Answers["task_area"].Probabilities["writing"] = .5
		}},
		{"missing", func(r *decisions.Response) { delete(r.Answers["task_area"].Probabilities, "unknown") }},
		{"extra", func(r *decisions.Response) { r.Answers["other"] = r.Answers["task_area"] }},
		{"nan", func(r *decisions.Response) { r.Answers["task_area"].Probabilities["coding"] = math.NaN() }},
		{"sum", func(r *decisions.Response) { r.Answers["task_area"].Probabilities["coding"] = .9 }},
		{"invalid_id", func(r *decisions.Response) {
			a := r.Answers["task_area"]
			a.Decision = "execute_shell"
			r.Answers["task_area"] = a
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := taskDecisionReply("coding", .8)
			test.mutate(&response)
			if _, err := parseTaskDecision(response); err == nil {
				t.Fatal("accepted invalid/uncertain decision")
			}
		})
	}
}

func TestTaskRouterDecisionsSingleAttemptAndPolicy(t *testing.T) {
	for _, test := range []string{"selected", "error", "uncertain", "invalid", "timeout", "late", "disabled", "toggle", "quota", "rules", "pinned", "mission", "unassigned", "budget"} {
		t.Run(test, func(t *testing.T) {
			helperCalls := 0
			cfg, run, req := taskRouterHelperFixture(t, func(w http.ResponseWriter, r *http.Request) { helperCalls++; w.WriteHeader(500) })
			cfg.Decisions.Enabled = true
			fake := &fakeTaskDecisions{available: true}
			run.RoutingDecisions = fake
			wantCalls := 1
			switch test {
			case "error":
				fake.evaluate = func(context.Context, decisions.Request) (decisions.Response, error) {
					return decisions.Response{}, errors.New("test failure")
				}
			case "uncertain":
				fake.evaluate = func(context.Context, decisions.Request) (decisions.Response, error) {
					return taskDecisionReply("unknown", .9), nil
				}
			case "invalid":
				fake.evaluate = func(context.Context, decisions.Request) (decisions.Response, error) { return decisions.Response{}, nil }
			case "timeout":
				cfg.LLMRouter.HelperTimeoutMS = 1
				fake.evaluate = func(ctx context.Context, _ decisions.Request) (decisions.Response, error) {
					<-ctx.Done()
					return taskDecisionReply("coding", .8), nil
				}
			case "late":
				fake.evaluate = func(context.Context, decisions.Request) (decisions.Response, error) {
					fake.version++
					return taskDecisionReply("coding", .8), nil
				}
			case "disabled":
				cfg.Decisions.Enabled = false
				cfg.LLMRouter.HelperFallback = false
				wantCalls = 0
			case "toggle":
				cfg.LLMRouter.DecisionsFallback = false
				cfg.LLMRouter.HelperFallback = false
				wantCalls = 0
			case "quota":
				cfg.LLMRouter.HelperMaxCallsPerHour = 0
				wantCalls = 0
			case "rules":
				run.UserIntent = "Implement a Go function"
				wantCalls = 0
			case "pinned":
				run.TaskRoutingMode = "pinned"
				wantCalls = 0
			case "mission":
				run.IsMission = true
				wantCalls = 0
			case "unassigned":
				fake.evaluate = func(context.Context, decisions.Request) (decisions.Response, error) {
					return taskDecisionReply("writing", .8), nil
				}
			case "budget":
				run.BudgetTracker = blockedBudgetTrackerForRouting(t, cfg)
				wantCalls = 0
			}
			if err := PrepareTaskRouting(context.Background(), &req, &run); err != nil {
				t.Fatal(err)
			}
			if fake.calls != wantCalls || helperCalls != 0 {
				t.Fatalf("decision calls=%d helper=%d", fake.calls, helperCalls)
			}
			if test == "selected" || test == "rules" {
				if req.Model != "gpt-4o-mini" {
					t.Fatal("classification not applied")
				}
			} else if req.Model != cfg.LLM.Model {
				t.Fatalf("unexpected route: %+v", run.TaskRouting)
			}
		})
	}
}

func TestTaskRouterDecisionsCacheAndGeneration(t *testing.T) {
	resetTaskRouterTestState(t)
	cfg, original, request := taskRouterFixture()
	cfg.Decisions.Enabled = true
	original.UserIntent = "Bitte mach daraus etwas Passendes"
	fake := &fakeTaskDecisions{available: true}
	original.RoutingDecisions = fake
	for i := 0; i < 2; i++ {
		run, req := original, request
		if err := PrepareTaskRouting(context.Background(), &req, &run); err != nil {
			t.Fatal(err)
		}
		if i == 1 && run.TaskRouting.Source != "cache" {
			t.Fatal("missing cache hit")
		}
	}
	if fake.calls != 1 {
		t.Fatal("duplicate classification")
	}
	fake.version++
	run := original
	_ = PrepareTaskRouting(context.Background(), &request, &run)
	if run.TaskRouting.Source == "cache" || request.Model != cfg.LLM.Model {
		t.Fatal("stale generation cache reused")
	}
	// Preview bypasses cached inference and leaves production caches unchanged.
	run = original
	run.TaskRoutingPreview = true
	run.TaskRoutingClassifier = "local"
	_ = PrepareTaskRouting(context.Background(), &request, &run)
	if fake.calls != 1 || run.TaskRouting.Source == "cache" {
		t.Fatal("local preview used inference")
	}
}

func TestTaskRouterDecisionsImmutableBoundedInputAndChannels(t *testing.T) {
	const secret = "router-sensitive-test-value"
	release := security.RegisterScopedSensitiveExact(secret)
	defer release()
	for _, channel := range []string{"web_chat", "desktop", "agodesk", "telegram", "discord", "rocketchat", "sms", "meshcore"} {
		t.Run(channel, func(t *testing.T) {
			resetTaskRouterTestState(t)
			cfg, run, req := taskRouterFixture()
			cfg.Decisions.Enabled = true
			run.MessageSource = channel
			original := "Bitte mach daraus etwas Passendes " + secret + strings.Repeat("ä", 2400)
			run.TaskRoutingText = &original
			run.UserIntent = "MEDIA-CONTAMINATION"
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: "user", Content: "MEDIA-CONTAMINATION"})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: "tool", Content: "TOOL-CONTAMINATION"})
			fake := &fakeTaskDecisions{available: true, evaluate: func(ctx context.Context, request decisions.Request) (decisions.Response, error) {
				data, _ := json.Marshal(request)
				if len(request.Questions) != 1 || len(request.Questions[0].Options) != 10 || len(request.Images) != 0 || len(data) > 12000 || strings.Contains(string(data), secret) || strings.Contains(string(data), "CONTAMINATION") || !strings.Contains(string(data), "external_data") {
					t.Error("unsafe classifier input")
				}
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 1500*time.Millisecond {
					t.Error("missing deadline")
				}
				return taskDecisionReply("coding", .8), nil
			}}
			run.RoutingDecisions = fake
			if err := PrepareTaskRouting(context.Background(), &req, &run); err != nil {
				t.Fatal(err)
			}
			if fake.calls != 1 {
				t.Fatal("channel not routed")
			}
			_ = PrepareTaskRouting(context.Background(), &req, &run)
			if fake.calls != 1 {
				t.Fatal("tool round reclassified")
			}
		})
	}
}

func TestTaskRouterDecisionsAttachmentOnlyDoesNotClassify(t *testing.T) {
	resetTaskRouterTestState(t)
	cfg, run, req := taskRouterFixture()
	cfg.Decisions.Enabled = true
	text := ""
	run.TaskRoutingText = &text
	run.UserIntent = "Write software from this attached document"
	fake := &fakeTaskDecisions{available: true}
	run.RoutingDecisions = fake
	if err := PrepareTaskRouting(context.Background(), &req, &run); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 0 || run.TaskRouting.Source != "default" || req.Model != cfg.LLM.Model {
		t.Fatal("attachment-only input reused enriched text or history")
	}
}

func TestTaskRouterDecisionsUnavailableUsesHelper(t *testing.T) {
	calls := 0
	cfg, run, req := taskRouterHelperFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"{\"domain\":\"coding\",\"complexity\":\"normal\",\"uncertain\":false}"}}]}`)
	})
	cfg.Decisions.Enabled = true
	fake := &fakeTaskDecisions{}
	run.RoutingDecisions = fake
	if err := PrepareTaskRouting(context.Background(), &req, &run); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || fake.calls != 0 || run.TaskRouting.Source != "helper" {
		t.Fatal("unavailable Decisions did not leave helper available")
	}
}

func TestTaskRouterDecisionsParentCancellation(t *testing.T) {
	resetTaskRouterTestState(t)
	cfg, run, req := taskRouterFixture()
	cfg.Decisions.Enabled = true
	run.UserIntent = "Bitte mach daraus etwas Passendes"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run.RoutingDecisions = &fakeTaskDecisions{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
		cancel()
		return taskDecisionReply("coding", .8), nil
	}}
	if err := PrepareTaskRouting(ctx, &req, &run); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation swallowed: %v", err)
	}
}
