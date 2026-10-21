package server

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIGatewaySlugReloadsLiveClient(t *testing.T) {
	var old config.Config
	old.AIGateway.Enabled = true
	old.AIGateway.AccountID = "account"
	old.AIGateway.GatewayID = "gateway"
	old.AIGateway.HuggingFaceProviderSlug = "before"
	next := old
	next.AIGateway.HuggingFaceProviderSlug = "after"
	if !llmHotReloadChanged(old, next) {
		t.Fatal("changing only the Hugging Face custom provider slug does not reload the production client")
	}
}

func TestAIGatewayMetadataSaveReplacesAndClearsTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := "ai_gateway:\n  enabled: true\n  account_id: account\n  gateway_id: gateway\n  metadata:\n    a: one\n    b: two\n    c: three\n    d: four\n    e: five\n"
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
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	for _, metadata := range []map[string]string{
		{"renamed": "one", "b": "two", "c": "three", "d": "four", "e": "five"},
		{},
	} {
		body, err := json.Marshal(map[string]any{"ai_gateway": map[string]any{"metadata": metadata}})
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
		if w.Code != http.StatusOK {
			t.Fatalf("metadata save failed: %d %s", w.Code, w.Body.String())
		}
		saved, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(saved.AIGateway.Metadata) != len(metadata) {
			t.Fatal("removed metadata resurrected", saved.AIGateway.Metadata)
		}
		for key, value := range metadata {
			if saved.AIGateway.Metadata[key] != value {
				t.Fatal("metadata changed during saving")
			}
		}
	}
}

func TestAIGatewayPolicySaveRebuildsGuardian(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := "providers:\n  - id: main\n    type: openai\n    model: gpt-4.1\n    base_url: https://api.openai.com/v1\nllm:\n  provider: main\nllm_guardian:\n  enabled: true\n  provider: main\nai_gateway:\n  enabled: true\n  account_id: account\n  gateway_id: gateway\n  log_mode: full\n"
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
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	s.LLMGuardian = security.NewLLMGuardian(cfg, s.Logger)
	before := s.LLMGuardian
	if before == nil {
		t.Fatal("fixture Guardian missing")
	}
	w := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"ai_gateway":{"log_mode":"off"}}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("save failed: %d %s", w.Code, w.Body.String())
	}
	if s.LLMGuardian == nil || s.LLMGuardian == before {
		t.Fatal("gateway policy save retained the old Guardian transport")
	}
}

func TestAIGatewayTokenAndPolicyReload(t *testing.T) {
	for _, change := range []func(*config.Config){
		func(c *config.Config) { c.AIGateway.Token = "rotated-fixture-token" },
		func(c *config.Config) { c.AIGateway.LogMode = "off" },
		func(c *config.Config) { c.AIGateway.Metadata = map[string]string{"env": "test"} },
		func(c *config.Config) { c.AIGateway.RequestTimeoutMS = 2500 },
	} {
		var old config.Config
		next := *old.Clone()
		if llmHotReloadChanged(old, next) {
			t.Fatal("unchanged config triggered reload")
		}
		change(&next)
		if !llmHotReloadChanged(old, next) {
			t.Fatal("gateway transport change missed by reload detection")
		}
	}
}
