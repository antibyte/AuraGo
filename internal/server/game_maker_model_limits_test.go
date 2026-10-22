package server

import (
	"testing"

	"aurago/internal/config"
)

func TestGameMakerModelLimitsBlockKnownSmallModel(t *testing.T) {
	cfg := &config.Config{}
	contextWindow, blocked := gameMakerModelLimitBlocked(cfg, "", "gpt-4", false)
	if !blocked {
		t.Fatalf("gpt-4 was not blocked (context=%d, floor=%d)", contextWindow, gameMakerMinContextWindow)
	}
	if contextWindow <= 0 || contextWindow >= gameMakerMinContextWindow {
		t.Fatalf("blocked model context = %d", contextWindow)
	}
	if _, blocked := gameMakerModelLimitBlocked(cfg, "", "gpt-4", true); blocked {
		t.Fatal("explicit opt-in did not bypass the context floor")
	}
}

func TestGameMakerModelLimitsKeepUnknownModelsStartable(t *testing.T) {
	cfg := &config.Config{}
	limits := gameMakerModelLimits(cfg, "", "totally-unknown-model-xyz")
	if !limits.Unknown {
		t.Fatalf("unknown model reported as known: %+v", limits)
	}
	if _, blocked := gameMakerModelLimitBlocked(cfg, "", "totally-unknown-model-xyz", false); blocked {
		t.Fatal("unknown model was blocked")
	}
}

func TestGameMakerModelLimitsRecognizeLargeModel(t *testing.T) {
	cfg := &config.Config{}
	limits := gameMakerModelLimits(cfg, "", "gpt-4o")
	if limits.Unknown || limits.ContextWindow < gameMakerMinContextWindow {
		t.Fatalf("gpt-4o limits = %+v", limits)
	}
	if _, blocked := gameMakerModelLimitBlocked(cfg, "", "gpt-4o", false); blocked {
		t.Fatal("large-context model was blocked")
	}
}
