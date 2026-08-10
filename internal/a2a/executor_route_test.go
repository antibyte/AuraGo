package a2a

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func TestBuildLLMClientUsesA2AProviderRoute(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = []config.ProviderEntry{
		{ID: "main", Type: "openai", BaseURL: "https://main.example/v1", Model: "main-model", ContextWindow: 128000},
		{ID: "a2a-local", Type: "ollama", BaseURL: "http://localhost:11434/v1", Model: "a2a-model", ContextWindow: 8192, MaxOutputTokens: 2048},
	}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.BaseURL = "https://main.example/v1"
	cfg.LLM.Model = "main-model"
	cfg.A2A.LLM.Provider = "a2a-local"
	cfg.A2A.LLM.ProviderType = "ollama"
	cfg.A2A.LLM.BaseURL = "http://localhost:11434/v1"
	cfg.A2A.LLM.Model = "a2a-model"

	executor := NewExecutor(&ExecutorDeps{Config: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	client := executor.buildLLMClient()
	manager, ok := client.(*llm.FailoverManager)
	if !ok {
		t.Fatalf("client type = %T, want *llm.FailoverManager", client)
	}
	defer manager.Stop()
	routes := manager.CandidateRoutes(openai.ChatCompletionRequest{Model: "a2a-model"})
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	if routes[0].ProviderID != "a2a-local" || routes[0].ProviderType != "ollama" || routes[0].ContextWindowOverride != 8192 {
		t.Fatalf("A2A route = %+v", routes[0])
	}
}

func TestBuildLLMClientRetainsInheritedMainProviderIdentity(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = []config.ProviderEntry{{
		ID: "main", Type: "openai", BaseURL: "https://main.example/v1", Model: "main-model", ContextWindow: 64000,
	}}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.BaseURL = "https://main.example/v1"
	cfg.LLM.Model = "main-model"
	cfg.A2A.LLM.ProviderType = cfg.LLM.ProviderType
	cfg.A2A.LLM.BaseURL = cfg.LLM.BaseURL
	cfg.A2A.LLM.Model = cfg.LLM.Model

	executor := NewExecutor(&ExecutorDeps{Config: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	manager := executor.buildLLMClient().(*llm.FailoverManager)
	defer manager.Stop()
	routes := manager.CandidateRoutes(openai.ChatCompletionRequest{Model: cfg.A2A.LLM.Model})
	if len(routes) != 1 || routes[0].ProviderID != "main" || routes[0].ContextWindowOverride != 64000 {
		t.Fatalf("inherited A2A route = %+v", routes)
	}
}
