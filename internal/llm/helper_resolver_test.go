package llm

import (
	"context"
	"testing"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

type mockChatClient struct{}

func (m *mockChatClient) CreateChatCompletion(_ context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{}, nil
}

func (m *mockChatClient) CreateChatCompletionStream(_ context.Context, _ openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, nil
}

func TestResolveHelperLLMReturnsResolvedFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openrouter"
	cfg.LLM.HelperBaseURL = "https://openrouter.ai/api/v1"
	cfg.LLM.HelperAPIKey = "secret"
	cfg.LLM.HelperResolvedModel = "google/gemini-2.0-flash-001"

	got := ResolveHelperLLM(cfg)

	if !got.Enabled {
		t.Fatal("expected helper LLM to be enabled")
	}
	if got.ProviderID != "helper" {
		t.Fatalf("ProviderID = %q, want helper", got.ProviderID)
	}
	if got.ProviderType != "openrouter" {
		t.Fatalf("ProviderType = %q, want openrouter", got.ProviderType)
	}
	if got.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("BaseURL = %q", got.BaseURL)
	}
	if got.APIKey != "secret" {
		t.Fatalf("APIKey = %q", got.APIKey)
	}
	if got.Model != "google/gemini-2.0-flash-001" {
		t.Fatalf("Model = %q", got.Model)
	}
}

func TestIsHelperLLMAvailableRequiresExplicitResolution(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperResolvedModel = "cheap-model"

	if IsHelperLLMAvailable(cfg) {
		t.Fatal("expected helper LLM to be unavailable without provider type")
	}

	cfg.LLM.HelperProviderType = "openai"
	if !IsHelperLLMAvailable(cfg) {
		t.Fatal("expected helper LLM to become available once explicitly resolved")
	}
}

func TestResolveHelperBackedClientFallsBackWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = false
	cfg.LLM.Model = "main-model"

	fallbackClient := &mockChatClient{}
	client, model := ResolveHelperBackedClient(cfg, fallbackClient, cfg.LLM.Model)

	if client != fallbackClient {
		t.Fatal("expected fallback client when helper disabled")
	}
	if model != "main-model" {
		t.Fatalf("model = %q, want main-model", model)
	}
}

func TestResolveHelperBackedClientFallsBackWhenModelEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperBaseURL = "https://api.openai.com/v1"
	cfg.LLM.HelperAPIKey = "test-key"
	cfg.LLM.HelperResolvedModel = ""
	cfg.LLM.Model = "main-model"

	fallbackClient := &mockChatClient{}
	client, model := ResolveHelperBackedClient(cfg, fallbackClient, cfg.LLM.Model)

	if client != fallbackClient {
		t.Fatal("expected fallback client when helper model is empty")
	}
	if model != "main-model" {
		t.Fatalf("model = %q, want main-model", model)
	}
}

func TestResolveHelperBackedClientFallsBackOnClientCreationFailure(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "unknown_provider"
	cfg.LLM.HelperBaseURL = "https://example.com"
	cfg.LLM.HelperAPIKey = "key"
	cfg.LLM.HelperResolvedModel = "cheap-model"
	cfg.LLM.Model = "main-model"

	fallbackClient := &mockChatClient{}
	client, model := ResolveHelperBackedClient(cfg, fallbackClient, cfg.LLM.Model)

	if client == fallbackClient {
		t.Fatal("expected new client, NewClientFromProvider always returns non-nil")
	}
	if model != "cheap-model" {
		t.Fatalf("model = %q, want cheap-model", model)
	}
}

func TestResolveHelperBackedClientReturnsHelperWhenAvailable(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperBaseURL = "https://api.openai.com/v1"
	cfg.LLM.HelperAPIKey = "test-key"
	cfg.LLM.HelperResolvedModel = "gpt-4o-mini"
	cfg.LLM.Model = "expensive-model"

	fallbackClient := &mockChatClient{}
	client, model := ResolveHelperBackedClient(cfg, fallbackClient, cfg.LLM.Model)

	if client == fallbackClient {
		t.Fatal("expected a new helper client, not the fallback")
	}
	if model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want gpt-4o-mini", model)
	}
}

func TestResolveHelperBackedClientNilConfig(t *testing.T) {
	fallbackClient := &mockChatClient{}
	client, model := ResolveHelperBackedClient(nil, fallbackClient, "fallback-model")

	if client != fallbackClient {
		t.Fatal("expected fallback client for nil config")
	}
	if model != "fallback-model" {
		t.Fatalf("model = %q, want fallback-model", model)
	}
}
