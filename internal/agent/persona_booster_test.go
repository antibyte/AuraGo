package agent

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/personabooster"
	"aurago/internal/prompts"
)

type personaBoosterConstSource uint64

func (c personaBoosterConstSource) Uint64() uint64 { return uint64(c) }

func fixedPersonaBoosterRand(t *testing.T, value uint64) {
	t.Helper()
	previous := newPersonaBoosterRand
	newPersonaBoosterRand = func() *rand.Rand { return rand.New(personaBoosterConstSource(value)) }
	t.Cleanup(func() { newPersonaBoosterRand = previous })
}

func personaBoosterTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Personality.CorePersonality = "punk"
	cfg.Personality.PersonaBooster = config.DefaultPersonaBoosterConfig()
	return cfg
}

func TestPersonaBoosterTurnStartsPhaseFromTextSignal(t *testing.T) {
	fixedPersonaBoosterRand(t, 0)
	stm := newTestPersonalityRuntimeMemory(t)
	run := RunConfig{Config: personaBoosterTestConfig(), ShortTermMem: stm, SessionID: "default",
		HumanInput: memory.NewHumanInput("verdammt, schon wieder kaputt", "default")}
	flags := prompts.ContextFlags{CorePersonality: "punk"}
	turn := personaBoosterTurn{Now: time.Date(2026, 11, 1, 15, 0, 0, 0, time.Local)}
	got := preparePersonaBoosterTurn(run, flags, slog.Default(), turn)
	if !got.Enabled || !strings.Contains(got.Amplifier, "punk edge") {
		t.Fatalf("amplifier missing: %+v", got)
	}
	if !strings.HasPrefix(got.Phase, "Temporary phase: Rage – ") || !strings.Contains(got.Phase, personabooster.LimitsLine) {
		t.Fatalf("phase = %q", got.Phase)
	}
	state, err := stm.LoadPersonaBoosterState()
	if err != nil || state.PhaseID != "rage" || state.Trigger != personabooster.SignalTextSwearing || state.Sessions["default"].Turns != 1 {
		t.Fatalf("state = %+v, err = %v", state, err)
	}
	turn.Now = turn.Now.Add(time.Minute)
	again := preparePersonaBoosterTurn(run, flags, slog.Default(), turn)
	if again.Phase != got.Phase {
		t.Fatal("phase text changed within the same phase")
	}
	if state, _ = stm.LoadPersonaBoosterState(); state.Sessions["default"].Turns != 1 || state.TurnsElapsed != 0 {
		t.Fatalf("a replayed turn was counted: %+v", state)
	}
}

func TestPersonaBoosterTurnSkipsExcludedRunsButKeepsAmplifier(t *testing.T) {
	fixedPersonaBoosterRand(t, 0)
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := personaBoosterTestConfig()
	punk := prompts.ContextFlags{CorePersonality: "punk"}
	turn := personaBoosterTurn{Now: time.Date(2026, 11, 1, 15, 0, 0, 0, time.Local)}
	for name, run := range map[string]RunConfig{
		"mission":        {Config: cfg, ShortTermMem: stm, SessionID: "default", IsMission: true, HumanInput: memory.NewHumanInput("verdammt", "default")},
		"co-agent":       {Config: cfg, ShortTermMem: stm, SessionID: "coagent-1", IsCoAgent: true, HumanInput: memory.NewHumanInput("verdammt", "coagent-1")},
		"no human input": {Config: cfg, ShortTermMem: stm, SessionID: "default"},
	} {
		if got := preparePersonaBoosterTurn(run, punk, slog.Default(), turn); got.Phase != "" || !got.Enabled || got.Amplifier == "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if state, err := stm.LoadPersonaBoosterState(); err != nil || state.Persona != "" {
		t.Fatalf("excluded runs changed the state: %+v, err = %v", state, err)
	}
	human := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", HumanInput: memory.NewHumanInput("verdammt", "default")}
	if got := preparePersonaBoosterTurn(human, prompts.ContextFlags{CorePersonality: "neutral"}, slog.Default(), turn); got != (personaBoosterPrompt{}) {
		t.Fatalf("neutral persona: %+v", got)
	}
	cfg.Personality.PersonaBooster.Enabled = false
	if got := preparePersonaBoosterTurn(human, punk, slog.Default(), turn); got != (personaBoosterPrompt{}) {
		t.Fatalf("booster off: %+v", got)
	}
}

func TestPersonaBoosterAppliesHelperCueAfterAnalysis(t *testing.T) {
	fixedPersonaBoosterRand(t, 0)
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := personaBoosterTestConfig()
	cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
	const message = "Das war ehrlich gesagt ziemlich schwach von dir"
	apply := func(cue string, confidence float64) {
		t.Helper()
		input := memory.NewHumanInput(message, "default")
		run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", DiscoveryRunID: "cue-" + cue, HumanInput: input}
		basis := prepareTurnEmotion(context.Background(), run, prompts.ContextFlags{}, message, memory.DefaultPersonalityMeta(), slog.Default())
		if basis == nil {
			t.Fatal("missing snapshot")
		}
		applyPersonalityV2AnalysisResult("default", cfg, slog.Default(), stm, nil, nil, "", "", "", 0, false, 0, 0, personalityV2AnalysisResult{
			Input: input, Basis: basis, ObservationID: basis.TurnID, Mood: memory.MoodFocused,
			Appraisal: &memory.PersonalityAppraisal{Signal: "none", Target: "unknown", Cue: cue, Confidence: confidence, Reference: "current_user_message"},
		})
	}
	apply("criticism", 0.4)
	apply("tantrum", 0.9)
	if state, _ := stm.LoadPersonaBoosterState(); state.PhaseID != "" {
		t.Fatalf("a weak or unknown cue started a phase: %+v", state)
	}
	apply("criticism", 0.9)
	state, err := stm.LoadPersonaBoosterState()
	if err != nil || state.Trigger != personabooster.CueCriticism || (state.PhaseID != "rage" && state.PhaseID != "cynic") {
		t.Fatalf("state = %+v, err = %v", state, err)
	}
	if !strings.Contains(helperTurnBatchPrompt, memory.AppraisalCueRule) || !strings.Contains(helperTurnBatchPrompt, `"cue": "none"`) {
		t.Fatal("the batched helper prompt does not request the cue")
	}
}
