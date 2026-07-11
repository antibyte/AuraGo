package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRepositoryConfigTemplatePerformanceAndDebugDefaults(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse config_template.yaml: %v", err)
	}

	agent, ok := raw["agent"].(map[string]any)
	if !ok || agent["max_concurrent_loops"] != 8 {
		t.Fatalf("agent.max_concurrent_loops = %#v, want 8", agent["max_concurrent_loops"])
	}
	circuitBreaker, ok := raw["circuit_breaker"].(map[string]any)
	if !ok {
		t.Fatal("config_template.yaml missing circuit_breaker section")
	}
	if circuitBreaker["llm_per_attempt_timeout_seconds"] != 120 {
		t.Fatalf("circuit_breaker.llm_per_attempt_timeout_seconds = %#v, want 120", circuitBreaker["llm_per_attempt_timeout_seconds"])
	}
	if circuitBreaker["final_retry_interval"] != "30s" {
		t.Fatalf("circuit_breaker.final_retry_interval = %#v, want 30s", circuitBreaker["final_retry_interval"])
	}
	server, ok := raw["server"].(map[string]any)
	if !ok || server["debug_pprof"] != false {
		t.Fatalf("server.debug_pprof = %#v, want false", server["debug_pprof"])
	}
}
