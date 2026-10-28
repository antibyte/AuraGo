package server

import (
	"fmt"
	"testing"

	"aurago/internal/config"
)

// A Studio stream may stay silent for gameMakerStreamIdleSeconds once it has
// progressed; a larger configured window is kept.
func TestGameMakerStreamIdleRaisesOnlyAShorterWindow(t *testing.T) {
	for _, test := range []struct{ configured, want int }{{0, 120}, {30, 120}, {120, 120}, {300, 300}} {
		t.Run(fmt.Sprintf("%d", test.configured), func(t *testing.T) {
			var cfg config.Config
			cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = test.configured
			applyGameMakerStreamIdle(&cfg)
			if got := cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds; got != test.want {
				t.Fatalf("idle window = %d, want %d", got, test.want)
			}
		})
	}
}
