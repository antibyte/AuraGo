package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/llm"
	"aurago/internal/security"
)

func TestFlowLLMRegistersCredentialsBeforeClientAndScrubsAnswer(t *testing.T) {
	const secret = "fixture-oauth-token-A1B2C3D4"
	data, err := json.Marshal(map[string]string{"token": secret})
	if err != nil {
		t.Fatal(err)
	}
	f, cfg, _ := newTestFlowLLM(&flowFakeChatClient{})
	cfg.Providers = []config.ProviderEntry{{ID: "test", Model: "model", APIKey: secret, AuthType: "oauth2"}}
	f.newClient = func(_ *config.Config, _ config.ProviderEntry) llm.ChatClient {
		if strings.Contains(security.Scrub(secret), secret) {
			t.Fatal("credential was not registered before the client constructor")
		}
		return &flowFakeChatClient{content: string(data)}
	}
	answer, err := f.Step(context.Background(), flows.LLMRequest{Prompt: "hello", Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(answer.Text, secret) {
		t.Fatal("the model answer disclosed the credential")
	}
}
