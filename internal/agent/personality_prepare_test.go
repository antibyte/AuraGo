package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

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
	run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default"}
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
	prepareTurnEmotion(context.Background(), RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default"}, prompts.ContextFlags{}, "Danke, das ist super!", memory.DefaultPersonalityMeta(), slog.Default())
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
	adapter := &emotionCompletionClient{client: client, cfg: cfg}
	_, err := adapter.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "gpt-4o-mini", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Return JSON emotion."}}})
	if !errors.Is(err, llm.ErrJSONCompletionTruncated) {
		t.Fatalf("accepted truncated completion: %v", err)
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
