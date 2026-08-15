package server

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func newTestServerWithPersonalityState(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitPersonalityTables(); err != nil {
		t.Fatalf("InitPersonalityTables: %v", err)
	}
	if err := stm.InitEmotionTables(); err != nil {
		t.Fatalf("InitEmotionTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.Personality.Engine = true
	cfg.Personality.EmotionSynthesizer.Enabled = true

	return &Server{
		Cfg:          cfg,
		Logger:       logger,
		ShortTermMem: stm,
	}
}

func TestBuildPersonalityStatePayloadIncludesCurrentEmotion(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if err := s.ShortTermMem.LogMood(memory.MoodFocused, "user asked a direct question"); err != nil {
		t.Fatalf("LogMood: %v", err)
	}
	if err := s.ShortTermMem.InsertEmotionHistory("I feel calm and attentive.", "focused", "recent successful interaction"); err != nil {
		t.Fatalf("InsertEmotionHistory: %v", err)
	}

	payload := s.buildPersonalityStatePayload()
	if enabled, _ := payload["enabled"].(bool); !enabled {
		t.Fatalf("expected enabled payload, got %#v", payload)
	}
	if got, _ := payload["current_emotion"].(string); got != "I feel calm and attentive." {
		t.Fatalf("current_emotion = %q, want latest synthesized emotion", got)
	}
	if state, ok := payload["current_emotion_state"].(*memory.EmotionHistoryEntry); !ok || state == nil {
		t.Fatalf("expected structured current_emotion_state, got %#v", payload["current_emotion_state"])
	}
	if got, _ := payload["mood"].(string); got != "focused" {
		t.Fatalf("mood = %q, want focused", got)
	}
}

func TestBuildPersonalityStatePayloadSanitizesReasoningFromEmotion(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if err := s.ShortTermMem.InsertEmotionHistory("<think>hidden reasoning</think> I feel calm and ready to help with this.", "focused", "recent successful interaction"); err != nil {
		t.Fatalf("InsertEmotionHistory: %v", err)
	}

	payload := s.buildPersonalityStatePayload()
	got, _ := payload["current_emotion"].(string)
	if strings.Contains(got, "<think>") || strings.Contains(strings.ToLower(got), "hidden reasoning") {
		t.Fatalf("current_emotion still contains reasoning: %q", got)
	}
	if got != "I feel calm and ready to help with this." {
		t.Fatalf("current_emotion = %q", got)
	}
}

func TestBuildPersonalityStatePayloadFallsBackWhenEmotionIsOnlyReasoning(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if err := s.ShortTermMem.InsertEmotionStateHistory(memory.EmotionState{
		Description:              "<think>very long hidden reasoning only</think>",
		PrimaryMood:              memory.MoodFocused,
		Cause:                    "the last fix completed successfully",
		Source:                   "test",
		RecommendedResponseStyle: "calm_and_precise",
	}, "plan_completed"); err != nil {
		t.Fatalf("InsertEmotionStateHistory: %v", err)
	}

	payload := s.buildPersonalityStatePayload()
	got, _ := payload["current_emotion"].(string)
	if got == "" {
		t.Fatal("expected fallback current_emotion, got empty string")
	}
	if strings.Contains(got, "<think>") {
		t.Fatalf("fallback current_emotion still contains think tag: %q", got)
	}
}

func TestBuildPersonalityStatePayloadIncludesCharacterNotes(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if _, err := s.ShortTermMem.InsertCharacterNote(memory.CharacterNote{
		Category: memory.CharacterNoteCategoryHabit,
		Text:     "I verify changes with one concrete check before calling the work done.",
		Source:   memory.CharacterNoteSourceReflection,
	}); err != nil {
		t.Fatalf("InsertCharacterNote: %v", err)
	}
	payload := s.buildPersonalityStatePayload()
	notes, ok := payload["character_notes"].([]memory.CharacterNote)
	if !ok || len(notes) != 1 {
		t.Fatalf("character_notes = %#v", payload["character_notes"])
	}
	if notes[0].Text == "" {
		t.Fatal("expected note text")
	}
	if visible, _ := payload["narrative_visible"].(bool); visible {
		t.Fatal("narrative should default off")
	}
	if _, ok := payload["affect_events"].([]memory.AffectEventRecord); !ok {
		t.Fatalf("affect_events missing: %#v", payload["affect_events"])
	}
}

func TestBuildPersonalityStatePayloadStripsChatAffectDetails(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if _, err := s.ShortTermMem.ApplyAffectEvent(memory.AffectEvent{
		CauseCode: memory.AffectCausePositiveFeedback,
		Valence:   0.4,
		Arousal:   0.4,
		Weight:    0.3,
		Source:    "chat",
		Detail:    "Danke, hier ist mein API-Schlüssel sk-test-secret",
	}, time.Now()); err != nil {
		t.Fatalf("ApplyAffectEvent: %v", err)
	}
	payload := s.buildPersonalityStatePayload()
	events, ok := payload["affect_events"].([]memory.AffectEventRecord)
	if !ok || len(events) == 0 {
		t.Fatalf("affect_events = %#v", payload["affect_events"])
	}
	for _, event := range events {
		if strings.Contains(event.Detail, "API-Schlüssel") || strings.Contains(event.Detail, "sk-test") || strings.Contains(event.Detail, "Danke") {
			t.Fatalf("chat affect detail leaked user text: %#v", event)
		}
	}

	if _, err := s.ShortTermMem.ApplyAffectEvent(memory.AffectEvent{
		CauseCode: memory.AffectCauseOpsIssueOpened,
		Valence:   -0.4,
		Arousal:   0.6,
		Weight:    0.3,
		Source:    "ops",
		Detail:    "open high-severity operational issue",
	}, time.Now()); err != nil {
		t.Fatalf("ApplyAffectEvent ops: %v", err)
	}
	payload = s.buildPersonalityStatePayload()
	events, ok = payload["affect_events"].([]memory.AffectEventRecord)
	if !ok {
		t.Fatalf("affect_events after ops = %#v", payload["affect_events"])
	}
	foundOps := false
	for _, event := range events {
		if event.CauseCode == memory.AffectCauseOpsIssueOpened {
			foundOps = true
			if event.Detail == "" {
				t.Fatal("ops affect detail should remain visible")
			}
		}
	}
	if !foundOps {
		t.Fatal("expected ops affect event in payload")
	}
}

func TestBuildPersonalityStatePayloadKeepsEnabledWhenTraitsUnavailable(t *testing.T) {
	s := newTestServerWithPersonalityState(t)
	if err := s.ShortTermMem.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	payload := s.buildPersonalityStatePayload()
	if enabled, _ := payload["enabled"].(bool); !enabled {
		t.Fatalf("enabled personality engine should not be reported as disabled when traits are unavailable: %#v", payload)
	}
	if traits, ok := payload["traits"].(memory.PersonalityTraits); !ok || traits[memory.TraitCuriosity] == 0 {
		t.Fatalf("expected fallback traits in payload, got %#v", payload["traits"])
	}
}
