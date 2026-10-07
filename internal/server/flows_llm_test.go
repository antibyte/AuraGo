package server

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/llm"
	"aurago/internal/security"
)

type flowFakeChatClient struct {
	requests []openai.ChatCompletionRequest
	content  string
}

func (f *flowFakeChatClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	f.requests = append(f.requests, req)
	return openai.ChatCompletionResponse{Model: req.Model + "-served",
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: f.content}, FinishReason: openai.FinishReasonStop}},
		Usage:   openai.Usage{PromptTokens: 11, CompletionTokens: 7}}, nil
}

func (f *flowFakeChatClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, nil
}

func newTestFlowLLM(main *flowFakeChatClient) (*flowLLM, *config.Config, map[string]*flowFakeChatClient) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	s := &Server{Cfg: cfg, LLMClient: main}
	providers := map[string]*flowFakeChatClient{}
	f := &flowLLM{s: s, blocked: func() bool { return false },
		newClient: func(_ *config.Config, p config.ProviderEntry) llm.ChatClient {
			c := &flowFakeChatClient{content: "from " + p.ID}
			providers[p.ID] = c
			return c
		}}
	return f, cfg, providers
}

func TestFlowLLMTextStep(t *testing.T) {
	main := &flowFakeChatClient{content: "  <think>hm</think>Hallo Welt  "}
	f, _, _ := newTestFlowLLM(main)
	resp, err := f.Step(context.Background(), flows.LLMRequest{System: "Sei kurz.", Prompt: "Sag hallo"})
	if err != nil || resp.Text != "Hallo Welt" || resp.Model != "main-model-served" || resp.InputTokens != 11 || resp.OutputTokens != 7 {
		t.Fatalf("response = %+v, %v", resp, err)
	}
	req := main.requests[0]
	if req.Model != "main-model" || len(req.Messages) != 2 || req.Messages[0].Role != openai.ChatMessageRoleSystem || req.MaxTokens != flowAIMaxTokens {
		t.Fatalf("request = %+v", req)
	}
}

func TestFlowLLMStructuredStepAndProviders(t *testing.T) {
	main := &flowFakeChatClient{content: "```json\n{\"title\":\"Bericht\"}\n```"}
	f, cfg, providers := newTestFlowLLM(main)
	resp, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", JSONSchema: map[string]any{"type": "object"}})
	if err != nil || resp.JSON["title"] != "Bericht" {
		t.Fatalf("structured response = %+v, %v", resp, err)
	}
	cfg.Providers = []config.ProviderEntry{{ID: "fast", Model: "fast-model"}}
	resp, err = f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "fast"})
	if err != nil || resp.Text != "from fast" || providers["fast"].requests[0].Model != "fast-model" {
		t.Fatalf("provider response = %+v, %v", resp, err)
	}
	cfg.Flows.AIProvider = "fast"
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); err != nil || len(providers["fast"].requests) != 1 {
		t.Fatalf("flows.ai_provider must select the provider: %v", err)
	}
	var ne *flows.NodeError
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x", Model: "missing"}); !errors.As(err, &ne) || ne.Code != "FLOW_AI_UNAVAILABLE" {
		t.Fatalf("unknown provider = %v", err)
	}
	f.blocked = func() bool { return true }
	if _, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "x"}); !errors.As(err, &ne) || ne.Code != "FLOW_BUDGET_EXCEEDED" {
		t.Fatalf("budget = %v", err)
	}
}

func TestFlowSecretsReadOnlyTheirNamespace(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteUserSecret("easydrag_api_token", "tok-flow-1234567890", true); err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("provider_api_key", "do-not-leak-1234567890"); err != nil {
		t.Fatal(err)
	}
	secrets := flowSecrets{s: &Server{Vault: vault}}
	if v, err := secrets.ReadSecret("api_token"); err != nil || v != "tok-flow-1234567890" {
		t.Fatalf("ReadSecret = %q, %v", v, err)
	}
	for _, name := range []string{"provider_api_key", "../provider_api_key", "API_TOKEN", ""} {
		if _, err := secrets.ReadSecret(name); err == nil {
			t.Errorf("%q must not be readable", name)
		}
	}
	if got := security.Scrub("value tok-flow-1234567890"); strings.Contains(got, "tok-flow-1234567890") {
		t.Fatal("flow secret values must be registered with the scrubber")
	}
}
