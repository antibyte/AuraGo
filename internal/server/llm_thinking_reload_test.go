package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"github.com/sashabaranov/go-openai"
)

func TestLLMHotReloadChangedDetectsThinkingOnlyChanges(t *testing.T) {
	base := config.Config{}
	base.LLM.AnthropicThinking.BudgetTokens = 10000
	base.LLM.AnthropicThinking.ModelAllowlist = []string{"claude-sonnet-4-6"}
	for _, tc := range []struct {
		name   string
		change func(*config.Config)
	}{
		{"enable", func(c *config.Config) { c.LLM.AnthropicThinking.Enabled = true }},
		{"budget", func(c *config.Config) { c.LLM.AnthropicThinking.BudgetTokens = 2048 }},
		{"allowlist", func(c *config.Config) { c.LLM.AnthropicThinking.ModelAllowlist = []string{"claude-opus-5"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated := base
			tc.change(&updated)
			if !llmHotReloadChanged(base, updated) {
				t.Fatal("thinking-only change did not trigger reconfiguration")
			}
		})
	}
	if llmHotReloadChanged(base, base) {
		t.Fatal("unchanged thinking configuration triggered reconfiguration")
	}
}

func TestThinkingOnlySaveChangesTheNextProviderRequest(t *testing.T) {
	var thinking []bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Thinking *struct {
				Type   string `json:"type"`
				Budget int    `json:"budget_tokens"`
			} `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		enabled := body.Thinking != nil && body.Thinking.Type == "enabled" && body.Thinking.Budget == 2048
		thinking = append(thinking, enabled)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"m","model":"claude-3-7-sonnet","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":1}}`)
	}))
	defer provider.Close()
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := fmt.Sprintf("providers:\n  - id: main\n    type: anthropic\n    model: claude-3-7-sonnet\n    base_url: %s/v1\nllm:\n  provider: main\n  anthropic_thinking:\n    enabled: false\n    budget_tokens: 2048\n", provider.URL)
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("1", 64), filepath.Join(filepath.Dir(path), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	client := llm.NewFailoverManager(cfg, slog.Default())
	defer client.Stop()
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default(), LLMClient: client}
	request := openai.ChatCompletionRequest{MaxTokens: 4096, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}}
	if _, err := client.CreateChatCompletion(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"llm":{"anthropic_thinking":{"enabled":true}}}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("settings save failed: %d %s", w.Code, w.Body.String())
	}
	if _, err := client.CreateChatCompletion(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if len(thinking) != 2 || thinking[0] || !thinking[1] {
		t.Fatalf("provider requests retained stale thinking: %v", thinking)
	}
}
