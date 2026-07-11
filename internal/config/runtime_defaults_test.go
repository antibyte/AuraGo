package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeDefaultsUseBoundedConcurrencyAndFinalRetryInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Agent.MaxConcurrentLoops != 8 {
		t.Fatalf("agent.max_concurrent_loops = %d, want 8", cfg.Agent.MaxConcurrentLoops)
	}
	if cfg.CircuitBreaker.FinalRetryInterval != "30s" {
		t.Fatalf("circuit_breaker.final_retry_interval = %q, want 30s", cfg.CircuitBreaker.FinalRetryInterval)
	}
}
