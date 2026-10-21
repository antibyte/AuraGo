package agent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

type turnEmotionClient struct {
	calls    int
	request  openai.ChatCompletionRequest
	complete func(context.Context) (openai.ChatCompletionResponse, error)
}

func (c *turnEmotionClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls++
	c.request = req
	return c.complete(ctx)
}

func TestCurrentTurnEmotionReachesPromptBeforeReply(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine, cfg.Personality.EngineV2, cfg.Personality.EmotionSynthesizer.Enabled = true, true, true
	cfg.Personality.CorePersonality = "punk"
	flags := &prompts.ContextFlags{CorePersonality: "punk", Tier: "full", TokenBudget: 12000}
	run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", HumanInput: memory.NewHumanInput("Please brainstorm a creative idea.", "default")}
	prepareTurnEmotion(context.Background(), run, *flags, "Please brainstorm a creative idea.", memory.DefaultPersonalityMeta(), slog.Default())
	flags.PersonalityLine = stm.GetPersonalityLineWithMeta(true, memory.DefaultPersonalityMeta())
	prompt, _ := prompts.BuildSystemPrompt(t.TempDir(), flags, "", slog.Default())
	for _, want := range []string{"CREATIVE", "imaginative and energized", "sharp, rebellious hacker"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("current reply lacks %q", want)
		}
	}
}

func TestCurrentTurnEmotionRetainsPositiveAffect(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine, cfg.Personality.EmotionSynthesizer.Enabled = true, true
	emitAffectFromTrigger(stm, cfg, slog.Default(), memory.EmotionTriggerPositiveFeedback, "", "chat")
	prepareTurnEmotion(context.Background(), RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", HumanInput: memory.NewHumanInput("Danke, das ist super!", "default")}, prompts.ContextFlags{}, "Danke, das ist super!", memory.DefaultPersonalityMeta(), slog.Default())
	if got := stm.GetCurrentMood(); got != memory.MoodRelaxed {
		t.Fatalf("generic heuristic overwrote positive feedback: %s", got)
	}
}

func TestCurrentTurnEmotionSkipsAutonomousAndDelegatedRuns(t *testing.T) {
	for _, kind := range []string{"mission", "coagent", "maintenance", "suppressed", "cron"} {
		t.Run(kind, func(t *testing.T) {
			stm := newTestPersonalityRuntimeMemory(t)
			cfg := &config.Config{}
			cfg.Personality.Engine = true
			cfg.Personality.EmotionSynthesizer.Enabled = true
			run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", IsMission: kind == "mission", IsCoAgent: kind == "coagent", IsMaintenance: kind == "maintenance"}
			run.HumanInput = memory.NewHumanInput("Please brainstorm a creative idea.", "default")
			run.SuppressTurnSideEffects = kind == "suppressed"
			if kind == "cron" {
				run.MessageSource = "cron"
			}
			prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "Please brainstorm a creative idea.", memory.DefaultPersonalityMeta(), slog.Default())
			if stm.GetCurrentMood() != memory.MoodCurious {
				t.Fatal("background/delegated request changed chat mood")
			}
		})
	}
}

func TestEmotionCompletionClientHonorsHelperLimitsAndRejectsTruncation(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.Personality.V2Provider = "emotion"
	cfg.Personality.V2ProviderType = "openai"
	cfg.Providers = []config.ProviderEntry{
		{ID: "main", ContextWindow: 128000, MaxOutputTokens: 16000},
		{ID: "emotion", ContextWindow: 16384, MaxOutputTokens: 320},
	}
	client := &turnEmotionClient{complete: func(context.Context) (openai.ChatCompletionResponse, error) {
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
			Message:      openai.ChatCompletionMessage{Content: `{"description":"valid JSON still truncated"}`},
			FinishReason: openai.FinishReasonLength,
		}}}, nil
	}}
	cfg.Budget.Enabled = true
	tracker := budget.NewTracker(cfg, slog.Default(), t.TempDir())
	defer tracker.Flush()
	adapter := &emotionCompletionClient{client: client, cfg: cfg, tracker: tracker}
	_, err := adapter.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "gpt-4o-mini", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Return JSON emotion."}}})
	if !errors.Is(err, llm.ErrJSONCompletionTruncated) {
		t.Fatalf("accepted truncated completion: %v", err)
	}
	if tracker.GetStatus().Models["gpt-4o-mini"].Calls != 1 {
		t.Fatal("truncated physical call was not billed")
	}
	if client.request.MaxTokens != 320 {
		t.Fatalf("wrong route output budget: %d", client.request.MaxTokens)
	}
}

func TestPersonalityReachesActualChatRequestAcrossToolRounds(t *testing.T) {
	// Keep asynchronous post-turn learning out of this request-content check.
	previousPause := v2PausedUntil.Swap(time.Now().Add(time.Minute).UnixNano())
	t.Cleanup(func() { v2PausedUntil.Store(previousPause) })
	run, _, cleanup := newPromptPipelineTestRunConfig(t, "personality-request", "web_chat")
	defer cleanup()
	run.Config.Personality.Engine, run.Config.Personality.EngineV2 = true, true
	run.HumanInput = memory.NewHumanInput("Please show the current process list.", run.SessionID)
	run.Config.Personality.CorePersonality = "punk"
	run.Config.Personality.EmotionSynthesizer.Enabled = true
	es := memory.NewEmotionSynthesizer(nil, run.Config.LLM.Model, 60, 100, "English", run.Logger)
	if err := es.BindMemory(run.ShortTermMem); err != nil {
		t.Fatal(err)
	}
	if err := es.ApplyExternalState(run.ShortTermMem, &memory.EmotionState{
		Description: "A fresh creative challenge gives me energy. A little wit belongs in the next reply.",
		PrimaryMood: memory.MoodCreative, Valence: .4, Arousal: .5, Confidence: .9,
	}, "test"); err != nil {
		t.Fatal(err)
	}
	client := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		gameMakerCacheToolResponse("first", "status"), gameMakerCacheFinalResponse(),
	}}}
	run.LLMClient = client
	run.Config.LLM.UseNativeFunctions = true
	run.AllowedTools = []string{"list_processes"}
	run.NativeToolSchemas = []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "list_processes", Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
	}}}
	_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{
		{Role: "user", Content: "Please show the current process list."},
	}}, run, false, NoopBroker{})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("model rounds=%d", len(client.requests))
	}
	for _, request := range client.requests {
		for _, want := range []string{"sharp, rebellious hacker", "CREATIVE", "imaginative and energized", "A little wit belongs in the next reply"} {
			if !strings.Contains(request.Messages[0].Content, want) {
				t.Fatalf("final request lost %q", want)
			}
		}
	}
	events, err := run.ShortTermMem.ListAffectEvents(50)
	if err != nil {
		t.Fatal(err)
	}
	conversationEvents := 0
	for _, event := range events {
		if event.Source == "chat" {
			conversationEvents++
		}
	}
	if conversationEvents != 1 {
		t.Fatalf("user stimulus applied %d times in one turn", conversationEvents)
	}
}

func TestPersonalityBothAnalysisPathsUseBoundedBilledRequests(t *testing.T) {
	for _, combined := range []bool{false, true} {
		t.Run(map[bool]string{false: "mood", true: "mood_and_emotion"}[combined], func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Budget.Enabled = true
			cfg.LLM.Provider = "main"
			cfg.LLM.ProviderType = "openai"
			cfg.Providers = []config.ProviderEntry{{ID: "main", ContextWindow: 128000, MaxOutputTokens: 400}}
			tracker := budget.NewTracker(cfg, slog.Default(), t.TempDir())
			defer tracker.Flush()
			client := &turnEmotionClient{complete: func(context.Context) (openai.ChatCompletionResponse, error) {
				return openai.ChatCompletionResponse{Usage: openai.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120}, Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{}`}, FinishReason: openai.FinishReasonLength}}}, nil
			}}
			adapter := &emotionCompletionClient{client: client, cfg: cfg, tracker: tracker}
			stm := newTestPersonalityRuntimeMemory(t)
			var err error
			if combined {
				_, _, _, _, _, _, _, _, err = stm.AnalyzeMoodV2WithEmotion(t.Context(), adapter, "gpt-4o-mini", "user: hello", "hello", memory.DefaultPersonalityMeta(), false, memory.EmotionInput{}, "English")
			} else {
				_, _, _, _, err = stm.AnalyzeMoodV2(t.Context(), adapter, "gpt-4o-mini", "user: hello", "hello", memory.DefaultPersonalityMeta(), false)
			}
			usage := tracker.GetStatus().Models["gpt-4o-mini"]
			if !errors.Is(err, llm.ErrJSONCompletionTruncated) || client.calls != 1 || client.request.MaxTokens != 400 || usage.InputTokens != 100 || usage.OutputTokens != 20 || usage.Calls != 1 {
				t.Fatalf("unbounded/unbilled request: calls=%d cap=%d usage=%+v err=%v", client.calls, client.request.MaxTokens, usage, err)
			}
		})
	}
}

func TestPersonalityBudgetCancellationAndSingleMainFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	tracker := budget.NewTracker(cfg, slog.Default(), t.TempDir())
	defer tracker.Flush()
	client := &turnEmotionClient{complete: func(context.Context) (openai.ChatCompletionResponse, error) {
		return openai.ChatCompletionResponse{Usage: openai.Usage{PromptTokens: 12, CompletionTokens: 3}, Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{}`}}}}, errors.New("provider failure with usage")
	}}
	request := openai.ChatCompletionRequest{Model: "test-model", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}}
	absent := &emotionCompletionClient{cfg: cfg, client: client}
	if _, err := absent.CreateChatCompletion(t.Context(), request); err == nil || client.calls != 0 {
		t.Fatal("untracked request sent")
	}
	adapter := &emotionCompletionClient{cfg: cfg, client: client, tracker: tracker, mainFallback: true}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := adapter.CreateChatCompletion(cancelled, request); !errors.Is(err, context.Canceled) || client.calls != 0 {
		t.Fatal("cancelled request sent")
	}
	_, _ = adapter.CreateChatCompletion(t.Context(), request)
	if _, err := adapter.CreateChatCompletion(t.Context(), request); err == nil || client.calls != 1 || tracker.GetStatus().Models["test-model"].Calls != 1 {
		t.Fatal("main fallback retried or provider usage lost")
	}
	cfg.Budget.DailyLimitUSD = 0.000001
	cfg.Budget.Enforcement = "full"
	tracker.RecordCostForCategory("personality", 1)
	blocked := &emotionCompletionClient{cfg: cfg, client: client, tracker: tracker}
	if _, err := blocked.CreateChatCompletion(t.Context(), request); err == nil || client.calls != 1 {
		t.Fatal("exhausted budget generated a response")
	}
}

func TestPersonalityRuntimeMainFallbackIsVisibleBoundedAndSingle(t *testing.T) {
	oldFailures, oldPause := v2FailCount.Swap(0), v2PausedUntil.Swap(0)
	defer func() { v2FailCount.Store(oldFailures); v2PausedUntil.Store(oldPause) }()
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.LLM.Model = "test-model"
	cfg.Personality.EmotionSynthesizer.Enabled = true
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	tracker := budget.NewTracker(cfg, logger, t.TempDir())
	defer tracker.Flush()
	group := NewAsyncTaskGroup(t.Context())
	defer group.Shutdown(time.Second)
	called := make(chan time.Duration, 1)
	client := &turnEmotionClient{complete: func(ctx context.Context) (openai.ChatCompletionResponse, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			called <- time.Hour
		} else {
			called <- time.Until(deadline)
		}
		return openai.ChatCompletionResponse{}, errors.New("synthetic provider unavailable")
	}}
	synth := memory.NewEmotionSynthesizer(nil, "", 60, 100, "English", logger)
	launchAsyncPersonalityV2Analysis("fallback-session", cfg, logger, client, RunConfig{BudgetTracker: tracker, SideEffects: group, HumanInput: memory.NewHumanInput("hello", "fallback-session")}, stm, synth, nil, "hello", memory.EmotionTriggerConversation, "", 0, "assistant", "hello", memory.DefaultPersonalityMeta(), false, 0, 0, 0, false, false)
	select {
	case duration := <-called:
		if duration <= 0 || duration > 20*time.Second {
			t.Fatalf("fallback timeout %s", duration)
		}
	case <-time.After(time.Second):
		t.Fatal("fallback did not start")
	}
	if err := group.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 || !strings.Contains(logs.String(), "Using bounded main-model fallback") {
		t.Fatalf("fallback hidden or repeated: calls=%d", client.calls)
	}
}

func TestPersonalityAndHelperReasoningRequestsUseSupportedOutputField(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "openai", ContextWindow: 32768, MaxOutputTokens: 2048}}
	client := &turnEmotionClient{complete: func(context.Context) (openai.ChatCompletionResponse, error) {
		return helperRecoveryResponse(`{}`, openai.FinishReasonStop), nil
	}}
	adapter := &emotionCompletionClient{cfg: cfg, tracker: newHelperTurnBudget(t), client: client}
	if _, err := adapter.CreateChatCompletion(t.Context(), openai.ChatCompletionRequest{Model: "o3-mini", MaxTokens: 1200, Temperature: .1, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Return JSON."}}}); err != nil {
		t.Fatal(err)
	}
	if client.request.MaxTokens != 0 || client.request.MaxCompletionTokens != 2048 || client.request.Temperature != 0 {
		t.Fatal("personality reasoning request uses unsupported parameters")
	}
	helperClient := &mockChatClient{response: `{}`}
	manager := &helperLLMManager{client: helperClient, model: "o3-mini", route: llm.ModelRoute{Model: "o3-mini", ProviderType: "openai", ContextWindowOverride: 32768, MaxOutputTokensOverride: 2048}}
	if _, err := manager.requestJSONResponse(t.Context(), "reasoning", "fixture", "Return JSON.", "Source.", 1200); err != nil {
		t.Fatal(err)
	}
	if helperClient.lastReq.MaxTokens != 0 || helperClient.lastReq.MaxCompletionTokens != 2048 || helperClient.lastReq.Temperature != 0 {
		t.Fatal("helper reasoning request uses unsupported parameters")
	}
}
