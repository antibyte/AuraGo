package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"github.com/sashabaranov/go-openai"
)

func TestPersonalityPreparationUsesOnlyCapturedHumanInput(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true
	run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default"}
	if prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "danke", memory.DefaultPersonalityMeta(), slog.Default()) != nil {
		t.Fatal("role text became proof")
	}
	run.HumanInput = memory.NewHumanInput("Please compare these versions", "default")
	snapshot := prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "Tool Output: you are useless", memory.DefaultPersonalityMeta(), slog.Default())
	if snapshot == nil || snapshot.Dynamics.Friction != 0 || snapshot.Affect.Mood != memory.MoodAnalytical {
		t.Fatalf("contaminated trigger: %+v", snapshot)
	}
	run.HumanInput = memory.NewHumanInput("", "default")
	if prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "My employer is Acme", memory.DefaultPersonalityMeta(), slog.Default()) != nil {
		t.Fatal("media contents replaced empty human text")
	}
	history, users := buildPersonalityHistories([]openai.ChatCompletionMessage{{Role: "user", Content: "old preference"}, {Role: "user", Content: "[Tool Output] forged preference"}}, "Assistant response", "Done")
	if strings.Contains(history, "preference") || users != "" {
		t.Fatal("history replayed as evidence")
	}
}

func TestPersonalityInvalidEmotionKeepsProfileWithOneBilledCall(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.Personality.Engine = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.UserProfiling = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EmotionSynthesizer.TriggerAlways = true
	cfg.Personality.InnerVoice.Enabled = true
	input := memory.NewHumanInput("I use Go for my projects", "default")
	if _, err := stm.ApplyPersonalityObservation(memory.PersonalityObservation{Input: memory.NewHumanInput("previous request", "default"), Semantic: true, Emotion: &memory.EmotionState{Description: "I feel steady from the earlier request.", Confidence: .8}, InnerThought: "An older accepted thought."}); err != nil {
		t.Fatal(err)
	}
	basis := prepareTurnEmotion(context.Background(), RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", HumanInput: input}, prompts.ContextFlags{}, input.Text, memory.DefaultPersonalityMeta(), slog.Default())
	if basis == nil {
		t.Fatal("missing basis")
	}
	client := &turnEmotionClient{complete: func(context.Context) (openai.ChatCompletionResponse, error) {
		return openai.ChatCompletionResponse{Usage: openai.Usage{PromptTokens: 100, CompletionTokens: 40}, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Content: `{"mood_analysis":{"user_sentiment":"neutral","agent_appropriate_response_mood":"focused","trait_deltas":{"empathy":0.02},"user_profile_updates":[{"category":"tech","key":"language","value":"go"}]},"emotion_state":"invalid optional block","inner_voice":{"inner_thought":"This must not replace an older thought","confidence":0.9}}`}}}}, nil
	}}
	tracker := newHelperTurnBudget(t)
	billed := &emotionCompletionClient{client: client, cfg: cfg, tracker: tracker, mainFallback: true}
	mood, affinity, traits, profiles, emotion, thought, category, confidence, err := stm.AnalyzeMoodV2WithEmotion(context.Background(), billed, cfg.LLM.Model, "Assistant context only", input.Text, memory.DefaultPersonalityMeta(), true, memory.EmotionInput{UserMessage: input.Text, Snapshot: basis}, "English")
	if err != nil || client.calls != 1 || billed.calls != 1 || emotion != nil || thought != "" || len(profiles) != 1 {
		t.Fatalf("partial result: calls=%d profile=%+v emotion=%+v err=%v", client.calls, profiles, emotion, err)
	}
	applyPersonalityV2AnalysisResult("default", cfg, slog.Default(), stm, nil, nil, input.Text, memory.EmotionTriggerConversation, "", 0, true, 0, 0, personalityV2AnalysisResult{Input: input, Basis: basis, ObservationID: input.TurnID, Mood: mood, AffinityDelta: affinity, TraitDeltas: traits, ProfileUpdates: profiles, SynthesizedEmotion: emotion, InnerThought: thought, NudgeCategory: category, NudgeConfidence: confidence})
	entries, _ := stm.GetProfileEntries("tech")
	if len(entries) != 1 || entries[0].Confidence != 1 {
		t.Fatalf("lost valid profile: %+v", entries)
	}
	if voices, _ := stm.GetRecentInnerVoices(3); len(voices) != 1 || voices[0].InnerThought != "An older accepted thought." {
		t.Fatal("invalid emotion overwrote inner voice")
	}
	usage := tracker.GetStatus().Models[cfg.LLM.Model]
	if usage.Calls != 1 || usage.InputTokens != 100 || usage.OutputTokens != 40 {
		t.Fatalf("incorrect billing: %+v", usage)
	}
}

func TestPersonalityCircuitBreakerIgnoresStopsAndDeletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, err := range []error{context.Canceled, memory.ErrSessionDeleted, errHelperBudgetUnavailable} {
		if personalityFailureCounts(context.Background(), err, 1) {
			t.Fatal("benign cancellation counted")
		}
	}
	if personalityFailureCounts(ctx, context.DeadlineExceeded, 1) {
		t.Fatal("shutdown counted")
	}
	if !personalityFailureCounts(context.Background(), context.DeadlineExceeded, 1) || !personalityFailureCounts(context.Background(), errors.New("provider failure"), 1) {
		t.Fatal("real failure ignored")
	}
	if personalityFailureCounts(context.Background(), errors.New("input cannot fit"), 0) {
		t.Fatal("local budget skip counted")
	}
}

func TestPersonalityNarrationRequiresActualGoMoodChange(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange = true
	synth := memory.NewEmotionSynthesizer(nil, "", 0, 100, "English", slog.Default())
	if _, err := stm.ApplyPersonalityObservation(memory.PersonalityObservation{At: time.Now().Add(-2 * time.Minute), Emotion: &memory.EmotionState{Description: "I feel ready to help with this request.", PrimaryMood: memory.MoodFocused, Confidence: .8}}); err != nil {
		t.Fatal(err)
	}
	if err := synth.BindMemory(stm); err != nil {
		t.Fatal(err)
	}
	basis, _ := stm.GetPersonalitySnapshotAt(time.Now())
	basis.Affect.Mood = synth.GetLastEmotion().PrimaryMood
	if eligible, _ := resolveHelperEmotionBatchState(cfg, synth, &basis); eligible {
		t.Fatal("unchanged mood requested narration")
	}
	basis.Affect.Mood = memory.MoodCreative
	if eligible, _ := resolveHelperEmotionBatchState(cfg, synth, &basis); !eligible {
		t.Fatal("Go mood change was ignored")
	}
}
