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

func TestGameMakerModelLimitsNameCompactRoute(t *testing.T) {
	cfg := &config.Config{}
	belowFloor := gameMakerModelLimits(cfg, "", "gpt-4")
	if !gameMakerRouteIsSmall(belowFloor) {
		t.Fatalf("gpt-4 is below the compact boundary: %+v", belowFloor)
	}
	shortOutput := gameMakerModelLimits(cfg, "", "gpt-4o-2024-05-13")
	if shortOutput.Unknown || shortOutput.ContextWindow < gameMakerSmallContextWindow || shortOutput.MaxOutputTokens >= gameMakerSmallOutputTokens {
		t.Fatalf("fixture model is not a large-context short-output route: %+v", shortOutput)
	}
	if !gameMakerRouteIsSmall(shortOutput) {
		t.Fatal("short output was not marked compact")
	}
	if _, blocked := gameMakerModelLimitBlocked(cfg, "", "gpt-4o-2024-05-13", false); blocked {
		t.Fatal("compact output limit was treated as the context floor")
	}
	large := gameMakerModelLimits(cfg, "", "gpt-4o")
	if gameMakerRouteIsSmall(large) {
		t.Fatalf("gpt-4o was marked compact: %+v", large)
	}
	unknown := gameMakerModelLimits(cfg, "", "totally-unknown-model-xyz")
	if gameMakerRouteIsSmall(unknown) {
		t.Fatal("unknown model was marked compact")
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
