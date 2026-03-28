package server

import (
	"log/slog"
	"os"
	"testing"

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
