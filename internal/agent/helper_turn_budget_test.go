package agent

import (
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"context"
	"errors"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func newHelperTurnBudget(t *testing.T) *budget.Tracker {
	t.Helper()
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 1
	cfg.Budget.Enforcement = "full"
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	t.Cleanup(tracker.Flush)
	return tracker
}

func TestHelperTurnBudgetDenialAndPhysicalRetryAccounting(t *testing.T) {
	const valid = `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{},"personality_analysis":{}}`
	for _, kind := range []string{"truncated", "transport"} {
		t.Run(kind, func(t *testing.T) {
			tracker := newHelperTurnBudget(t)
			calls := 0
			client := maintenanceCompletionTestClient{complete: func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				calls++
				if req.MaxTokens <= 0 || req.MaxTokens > 400 {
					t.Fatalf("provider cap escaped: %d", req.MaxTokens)
				}
				response := helperRecoveryResponse(valid, openai.FinishReasonStop)
				response.Model = "helper-fixture"
				response.Usage = openai.Usage{PromptTokens: 100 * calls, CompletionTokens: 20 * calls, TotalTokens: 120 * calls}
				if calls == 1 {
					if kind == "transport" {
						return response, errors.New("503 temporary provider unavailable")
					}
					response.Choices[0].FinishReason = openai.FinishReasonLength
				}
				return response, nil
			}}
			manager := &helperLLMManager{client: client, model: "helper-fixture", route: llm.ModelRoute{Model: "helper-fixture", ContextWindowOverride: 32768, MaxOutputTokensOverride: 400}, responseCache: map[string]string{}}
			input := &helperTurnPersonalityInput{Language: "English"}
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, nil, true); err == nil || calls != 0 {
				t.Fatal("untracked batch sent an LLM request")
			}
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, tracker, true); err != nil {
				t.Fatal(err)
			}
			usage := tracker.GetStatus().Models["helper-fixture"]
			if calls != 2 || usage.Calls != 2 || usage.InputTokens != 300 || usage.OutputTokens != 60 {
				t.Fatalf("physical usage lost: calls=%d usage=%+v", calls, usage)
			}
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, tracker, true); err != nil || calls != 2 {
				t.Fatal("cache charged a physical request")
			}
			tracker.RecordCostForCategory("helper_turn", 2)
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, tracker, true); err == nil || calls != 2 {
				t.Fatal("exhausted budget sent a batch request")
			}
		})
	}
}

func TestHelperTurnNilTrackerRunsUntrackedWhenBudgetDisabled(t *testing.T) {
	const valid = `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{},"personality_analysis":{}}`
	client := &mockChatClient{response: valid}
	manager := &helperLLMManager{client: client, model: "helper-fixture", route: llm.ModelRoute{Model: "helper-fixture", ContextWindowOverride: 32768, MaxOutputTokensOverride: 400}, responseCache: map[string]string{}}
	if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, &helperTurnPersonalityInput{Language: "English"}, nil, false); err != nil || client.calls != 1 {
		t.Fatalf("disabled budgeting must run the turn untracked: calls=%d err=%v", client.calls, err)
	}
}

// recordingHelperClient is a helper-route fake that records every request and
// answers with one fixed batched-turn JSON document.
type recordingHelperClient struct {
	mu       sync.Mutex
	requests []openai.ChatCompletionRequest
	response string
}

func (c *recordingHelperClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	return helperRecoveryResponse(c.response, openai.FinishReasonStop), nil
}

func (c *recordingHelperClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("helper turn analysis never streams")
}

func (c *recordingHelperClient) recorded() []openai.ChatCompletionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]openai.ChatCompletionRequest(nil), c.requests...)
}

// With budgeting disabled the loop has no tracker by design. The post-turn
// helper batch (memory, digest, mood, emotion, inner voice) must still run.
func TestTurnAnalysisRunsThroughLoopWhenBudgetingIsDisabled(t *testing.T) {
	const sessionID = "budget-off-session"
	const text = "What will the weather be like tomorrow?"
	const helperResponse = `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{"intent":"Weather","user_goal":"Know tomorrow's weather","actions_taken":["Answered"],"outcomes":["Answered"],"important_points":[],"pending_items":[],"importance":1,"entities":[]},"personality_analysis":{"mood_analysis":{"user_sentiment":"curious","agent_appropriate_response_mood":"playful","relationship_delta":0.01,"trait_deltas":{"curiosity":0.02},"user_profile_updates":[]},"emotion_state":{"description":"I feel light and curious.","primary_mood":"playful","secondary_mood":"","valence":0.3,"arousal":0.4,"confidence":0.8,"cause":"a simple friendly question","recommended_response_style":"warm"},"inner_voice":{"inner_thought":"I feel at ease and curious about what comes next.","nudge_category":"curiosity","confidence":0.8}}}`

	oldFailures, oldPause := v2FailCount.Swap(0), v2PausedUntil.Swap(0)
	ResetGlobalHelperLLMManager()
	ResetInnerVoiceState()
	t.Cleanup(func() {
		v2FailCount.Store(oldFailures)
		v2PausedUntil.Store(oldPause)
		ResetGlobalHelperLLMManager()
		ResetInnerVoiceState()
	})

	run, _, cleanup := newPromptPipelineTestRunConfig(t, sessionID, "web_chat")
	t.Cleanup(cleanup)
	cfg := run.Config
	cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
	cfg.Personality.CorePersonality = "punk"
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EmotionSynthesizer.MinIntervalSecs = 60
	cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange = true
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.InnerVoice.MinIntervalSecs = 60
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.DecayTurns = 3
	cfg.Personality.InnerVoice.DecayMaxAgeSecs = 300
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperBaseURL = "https://helper.invalid/v1"
	cfg.LLM.HelperAPIKey = "k"
	cfg.LLM.HelperResolvedModel = "helper-model"
	if cfg.Budget.Enabled || run.BudgetTracker != nil {
		t.Fatal("fixture must model disabled budgeting: no tracker")
	}

	helper := &recordingHelperClient{response: helperResponse}
	manager := newHelperLLMManager(cfg, run.Logger)
	if manager == nil {
		t.Fatal("helper manager unavailable")
	}
	manager.client = helper

	sideEffects := NewAsyncTaskGroup(context.Background())
	t.Cleanup(func() { _ = sideEffects.Shutdown(5 * time.Second) })
	run.SideEffects = sideEffects
	run.HumanInput = memory.NewHumanInput(text, sessionID)
	request := openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: text}}}
	if _, err := ExecuteAgentLoop(context.Background(), request, run, false, NoopBroker{}); err != nil {
		t.Fatal(err)
	}

	// Do not Shutdown here: it cancels the group context and fails the session
	// registration of the still running background analysis.
	var voices []memory.InnerVoiceEntry
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		voices, _ = run.ShortTermMem.GetRecentInnerVoices(3)
		if len(voices) > 0 {
			break
		}
	}

	requests := helper.recorded()
	if len(requests) != 1 {
		t.Fatalf("helper requests=%d, want the single batched turn request", len(requests))
	}
	var prompt strings.Builder
	for _, message := range requests[0].Messages {
		prompt.WriteString(message.Content)
		prompt.WriteString("\n")
	}
	for _, want := range []string{"PERSONALITY CONTEXT", "INNER VOICE REQUESTED"} {
		if !strings.Contains(prompt.String(), want) {
			t.Fatalf("batched turn request lacks %q", want)
		}
	}
	if emotion, err := run.ShortTermMem.GetLatestEmotion(); err != nil || emotion == nil {
		t.Fatalf("emotion was not persisted: %v %v", emotion, err)
	}
	if len(voices) == 0 || !strings.Contains(voices[0].InnerThought, "curious about what comes next") {
		t.Fatalf("inner voice was not persisted: %+v", voices)
	}
}

func TestHelperTurnBudgetOverflowDefersWithoutProviderCall(t *testing.T) {
	tracker := newHelperTurnBudget(t)
	client := &mockChatClient{response: `{}`}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-overflow-fixture",
		route: llm.ModelRoute{
			Model: "helper-overflow-fixture", ContextWindowOverride: 2048, MaxOutputTokensOverride: 64,
		},
	}
	_, err := manager.AnalyzeTurn(t.Context(), strings.Repeat("<& complete source > ", 1000), "Assistant source", nil, nil,
		&helperTurnPersonalityInput{Language: "English", CurrentUserMessage: strings.Repeat("<& complete source > ", 1000)}, tracker, true)
	if !errors.Is(err, errHelperBudgetUnavailable) || !strings.Contains(err.Error(), "json_completion_context_budget") {
		t.Fatalf("overflow did not carry the no-fallback budget category: %v", err)
	}
	if client.calls != 0 || tracker.GetStatus().Models[manager.model].Calls != 0 {
		t.Fatal("oversize source dispatched or billed a provider call")
	}
}
