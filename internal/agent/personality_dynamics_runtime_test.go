package agent

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"github.com/sashabaranov/go-openai"
)

func TestPersonalityDynamicsRuntimeDeduplicatesTurnWithoutHelper(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true
	run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", DiscoveryRunID: "one-turn"}
	first := prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "danke", memory.DefaultPersonalityMeta(), slog.Default())
	second := prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "danke", memory.DefaultPersonalityMeta(), slog.Default())
	if first == nil || second == nil || first.Dynamics.Revision != second.Dynamics.Revision || !reflect.DeepEqual(first.Traits, second.Traits) {
		t.Fatalf("turn replay changed state: %#v %#v", first, second)
	}
	if first.Dynamics.Familiarity <= .5 {
		t.Fatal("explicit local praise did not develop familiarity")
	}
}

func TestPersonalityDynamicsRuntimeRejectsSupersededHelper(t *testing.T) {
	for _, change := range []string{"reset", "persona", "channel"} {
		t.Run(change, func(t *testing.T) {
			stm := newTestPersonalityRuntimeMemory(t)
			cfg := &config.Config{}
			cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
			run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", DiscoveryRunID: "older-turn"}
			basis := prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "Check this task", memory.DefaultPersonalityMeta(), slog.Default())
			if basis == nil {
				t.Fatal("missing snapshot")
			}
			switch change {
			case "reset":
				if _, err := stm.ResetPersonalityDynamics(time.Now()); err != nil {
					t.Fatal(err)
				}
			case "persona":
				if err := stm.SetPersonalityContext("punk", memory.DefaultPersonalityMeta()); err != nil {
					t.Fatal(err)
				}
			case "channel":
				run.SessionID, run.DiscoveryRunID = "telegram", "new-turn"
				prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "danke", memory.DefaultPersonalityMeta(), slog.Default())
			}
			before, _ := stm.GetPersonalitySnapshotAt(time.Now())
			applyPersonalityV2AnalysisResult("default", cfg, slog.Default(), stm, nil, nil, "", "", "", 0, true, 0, 0, personalityV2AnalysisResult{
				Basis: basis, ObservationID: basis.TurnID, Mood: memory.MoodFrustrated,
				Appraisal:     &memory.PersonalityAppraisal{Signal: "criticism", Target: "agent", Confidence: 1, Reference: "current_user_message"},
				AffinityDelta: -.1, TraitDeltas: map[string]float64{memory.TraitConfidence: -.1},
				ProfileUpdates: []memory.ProfileUpdate{{Category: "tech", Key: "language", Value: "rust"}},
			})
			after, _ := stm.GetPersonalitySnapshotAt(time.Now())
			profiles, _ := stm.GetProfileEntries("tech")
			if after.Dynamics.Revision != before.Dynamics.Revision || !reflect.DeepEqual(after.Traits, before.Traits) || len(profiles) != 0 {
				t.Fatal("superseded helper mutated state")
			}
		})
	}
}

func TestPersonalityDynamicsRuntimeRequiresHumanRelationshipEvidence(t *testing.T) {
	for _, tc := range []struct {
		message           string
		confidence        float64
		target, reference string
		wantFriction      bool
	}{
		{"Ja klar, sehr hilfreich... /s", .4, "agent", "current_user_message", false},
		{"The server keeps crashing", 1, "task", "current_user_message", false},
		{"Try the fix", 1, "agent", "assistant_apology", false},
		{"Your response ignored my actual request", .95, "agent", "current_user_message", true},
	} {
		t.Run(tc.message, func(t *testing.T) {
			stm := newTestPersonalityRuntimeMemory(t)
			cfg := &config.Config{}
			cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
			basis := prepareTurnEmotion(context.Background(), RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", DiscoveryRunID: "turn"}, prompts.ContextFlags{}, tc.message, memory.DefaultPersonalityMeta(), slog.Default())
			if basis == nil {
				t.Fatal("missing snapshot")
			}
			applyPersonalityV2AnalysisResult("default", cfg, slog.Default(), stm, nil, nil, "", "", "", 0, false, 0, 0, personalityV2AnalysisResult{
				Basis: basis, ObservationID: "turn", AffinityDelta: -.1,
				Appraisal: &memory.PersonalityAppraisal{Signal: "criticism", Target: tc.target, Confidence: tc.confidence, Reference: tc.reference},
			})
			after, _ := stm.GetPersonalitySnapshotAt(time.Now())
			if (after.Dynamics.Friction > 0) != tc.wantFriction {
				t.Fatalf("unexpected relationship effect: %+v", after.Dynamics)
			}
			if !tc.wantFriction && after.Dynamics.Familiarity != basis.Dynamics.Familiarity {
				t.Fatal("ambiguous evidence changed familiarity")
			}
		})
	}
}

func TestPersonalityDynamicsPublishedConfigOwnsContext(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true
	live := *cfg
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &live }
	meta := memory.DefaultPersonalityMeta()
	if err := stm.SetPersonalityContext("", meta); err != nil {
		t.Fatal(err)
	}
	run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", DiscoveryRunID: "bound-turn"}
	if prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "danke", meta, slog.Default()) == nil {
		t.Fatal("empty configured persona did not match the neutral context")
	}
	live.Personality.CorePersonality = "punk"
	if err := stm.SetPersonalityContext("punk", meta); err != nil {
		t.Fatal(err)
	}
	before, _ := stm.GetPersonalitySnapshotAt(time.Now())
	if prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, "danke", meta, slog.Default()) != nil {
		t.Fatal("old run restored the previous persona")
	}
	after, _ := stm.GetPersonalitySnapshotAt(time.Now())
	if after.Persona != "punk" || after.Dynamics.Revision != before.Dynamics.Revision {
		t.Fatal("stale run changed the published context")
	}
}

func TestPersonalityDynamicsToolEvidenceAndSuppression(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true
	run := RunConfig{Config: cfg, DiscoveryRunID: "tool-run"}
	call := ToolCall{Action: "execute_shell", NativeCallID: "call1"}
	for _, status := range []ToolResultStatus{ToolResultDenied, ToolResultUnknown, ToolResultDeferred} {
		emitConfirmedToolPersonality(stm, cfg, run, call, call, status, slog.Default())
	}
	before, _ := stm.GetPersonalitySnapshotAt(time.Now())
	if before.Dynamics.Revision != 0 {
		t.Fatal("unconfirmed tool result changed personality")
	}
	emitConfirmedToolPersonality(stm, cfg, run, call, call, ToolResultFailed, slog.Default())
	first, _ := stm.GetPersonalitySnapshotAt(time.Now())
	emitConfirmedToolPersonality(stm, cfg, run, call, call, ToolResultFailed, slog.Default())
	second, _ := stm.GetPersonalitySnapshotAt(time.Now())
	if first.Dynamics.Load <= 0 || second.Dynamics.Revision != first.Dynamics.Revision || second.Dynamics.Familiarity != before.Dynamics.Familiarity || second.Dynamics.Friction != 0 {
		t.Fatal("tool evidence was duplicated or affected the relationship")
	}
	run.SuppressTurnSideEffects = true
	call.NativeCallID = "relay"
	emitConfirmedToolPersonality(stm, cfg, run, call, call, ToolResultFailed, slog.Default())
	after, _ := stm.GetPersonalitySnapshotAt(time.Now())
	if after.Dynamics.Revision != second.Dynamics.Revision {
		t.Fatal("suppressed relay changed state")
	}
	run.SuppressTurnSideEffects = false
	disabled := *cfg
	disabled.Personality.Engine = false
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &disabled }
	call.NativeCallID = "after-disable"
	emitConfirmedToolPersonality(stm, cfg, run, call, call, ToolResultFailed, slog.Default())
	after, _ = stm.GetPersonalitySnapshotAt(time.Now())
	if after.Dynamics.Revision != second.Dynamics.Revision {
		t.Fatal("late tool completion ignored the disabled engine")
	}
}

func TestPersonalityDynamicsKeepsDistinctPersonasAndTask(t *testing.T) {
	previousPause := v2PausedUntil.Swap(time.Now().Add(time.Minute).UnixNano())
	t.Cleanup(func() { v2PausedUntil.Store(previousPause) })
	for _, persona := range []string{"friend", "punk", "professional"} {
		t.Run(persona, func(t *testing.T) {
			run, _, cleanup := newPromptPipelineTestRunConfig(t, "dynamics-"+persona, "web_chat")
			defer cleanup()
			run.Config.Personality.Engine, run.Config.Personality.EngineV2 = true, true
			run.Config.Personality.CorePersonality = persona
			if err := run.ShortTermMem.SetTrait(memory.TraitAffinity, .85); err != nil {
				t.Fatal(err)
			}
			if _, err := run.ShortTermMem.ApplyPersonalityObservation(memory.PersonalityObservation{
				Source: "feedback", Target: "agent", Human: true, Confidence: 1, Explicit: true, Signal: "criticism",
			}); err != nil {
				t.Fatal(err)
			}
			client := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{gameMakerCacheFinalResponse()}}}
			run.LLMClient = client
			request := openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Explain what a checksum verifies. Do not change files."}}}
			if _, err := ExecuteAgentLoop(context.Background(), request, run, false, NoopBroker{}); err != nil {
				t.Fatal(err)
			}
			if len(client.requests) != 1 {
				t.Fatal("local personality added a blocking model call")
			}
			system := client.requests[0].Messages[0].Content
			if !strings.Contains(system, "familiar warmth") || !strings.Contains(system, "persona") {
				t.Fatal("mixed relationship state did not reach the actual prompt")
			}
			personaText := prompts.GetCorePersonalityPromptSummary(run.Config.Directories.PromptsDir, persona, 100)
			personaText = strings.Join(strings.Fields(strings.TrimSuffix(personaText, "…")), " ")
			if personaText == "" || !strings.Contains(strings.Join(strings.Fields(system), " "), personaText) {
				t.Fatal("selected persona disappeared")
			}
			found := false
			for _, message := range client.requests[0].Messages {
				if strings.Contains(message.Content, request.Messages[0].Content) {
					found = true
				}
			}
			if !found {
				t.Fatal("dynamics overwrote the user task")
			}
		})
	}
}
