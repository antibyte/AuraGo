package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/agodesk"
	"aurago/internal/budget"
	"aurago/internal/config"
)

func TestProviderPricesRoundTripThroughHTTPAndVaultNoop(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	_ = vault.WriteSecret("provider_main_api_key", "priceRoundtripKey123456")
	s.Cfg.ApplyVaultSecrets(vault)
	s.Cfg.ResolveProviders()
	beforeVault, _ := os.ReadFile(filepath.Join(filepath.Dir(s.Cfg.ConfigPath), "vault.bin"))
	body := `[{"id":"main","name":"Priced","type":"openai","base_url":"https://api.openai.com/v1","model":"gpt-4o-mini","api_key":"••••••••","models":[{"name":"gpt-4o-mini","input_per_million":1.25,"output_per_million":2.5,"source":"openrouter","estimated":true}]}]`
	rec := providerTestWrite(s, body, s.providerETag())
	if rec.Code != 200 {
		t.Fatalf("save: %s", rec.Body.String())
	}
	price := s.Cfg.Providers[0].Models[0]
	if price.InputPerMillion != 1.25 || price.OutputPerMillion != 2.5 || price.Source != "openrouter" || !price.Estimated {
		t.Fatalf("price roundtrip lost data: %+v", price)
	}
	raw, _ := json.Marshal(s.Cfg.Providers[0].Models)
	if !bytes.Contains(raw, []byte(`"input_per_million":1.25`)) {
		t.Fatal("price DTO uses incompatible JSON names")
	}
	afterVault, _ := os.ReadFile(filepath.Join(filepath.Dir(s.Cfg.ConfigPath), "vault.bin"))
	if !bytes.Equal(beforeVault, afterVault) {
		t.Fatal("unchanged credentials were rewritten")
	}
}

func TestProviderNewDanglingReferencesAreRejectedBeforeSecrets(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	before, _ := os.ReadFile(s.Cfg.ConfigPath)
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"co_agents":{"llm":{"provider":"deleted"}},"tts":{"api_key":"rejectedReferenceSecret123456"}}`)))
	after, _ := os.ReadFile(s.Cfg.ConfigPath)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "provider_reference_conflict") || !bytes.Equal(before, after) {
		t.Fatalf("dangling reference accepted: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := vault.ReadSecret("tts_api_key"); err == nil {
		t.Fatal("rejected reference changed Vault")
	}
}

func TestProviderSetupPreviewOnlyChecksMetadata(t *testing.T) {
	var generations atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/tags" {
			generations.Add(1)
		}
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer fixture.Close()
	s, _ := newProviderTestServer(t, providerSafetyConfig)
	addSetupCSRFTokenForTest(s, "setup-metadata-fixture")
	request := httptest.NewRequest("POST", "/api/setup/test", strings.NewReader(fmt.Sprintf(`{"provider_type":"ollama","base_url":%q,"model":"fixture"}`, fixture.URL+"/v1")))
	request.Header.Set("X-CSRF-Token", "setup-metadata-fixture")
	request.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handleSetupTestConnection(s).ServeHTTP(rec, request)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 200 || body["ok"] != true || body["probe_kind"] != "metadata" || generations.Load() != 0 {
		t.Fatalf("setup test generated content or failed: %s", rec.Body.String())
	}
}

const providerSafetyConfig = `
providers:
  - id: main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: main
`

func providerTestWrite(s *Server, body, revision string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/providers", strings.NewReader(body))
	if revision != "" {
		req.Header.Set("If-Match", revision)
	}
	rec := httptest.NewRecorder()
	handlePutProviders(s, rec, req)
	return rec
}

func TestProviderRevisionSerializesIndependentEditors(t *testing.T) {
	s, _ := newProviderTestServer(t, providerSafetyConfig)
	revision := s.providerETag()
	body := `[{"id":"main","name":"Edited","type":"openai","base_url":"https://api.openai.com/v1","model":"gpt-4o-mini"}]`
	if got := providerTestWrite(s, body, "").Code; got != 428 {
		t.Fatalf("missing revision: %d", got)
	}
	start := make(chan struct{})
	codes := make(chan int, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() { <-start; codes <- providerTestWrite(s, body, revision).Code })
	}
	close(start)
	group.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[412] != 1 {
		t.Fatalf("concurrent results: %v", counts)
	}
	if s.providerETag() == revision {
		t.Fatal("successful save did not rotate revision")
	}
}

func TestProviderPricingAndEditorCannotOverwriteEachOther(t *testing.T) {
	s, _ := newProviderTestServer(t, providerSafetyConfig)
	revision := s.providerETag()
	results := make(chan int, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Go(func() {
		<-start
		results <- providerTestWrite(s, `[{"id":"main","type":"openai","model":"gpt-4o-mini","name":"Editor draft"}]`, revision).Code
	})
	group.Go(func() {
		<-start
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/providers/pricing?id=main", strings.NewReader(`[{"model_id":"gpt-4o-mini","input_per_million":42}]`))
		req.Header.Set("If-Match", revision)
		handleApplyPricing(s, rec, req, "main")
		results <- rec.Code
	})
	close(start)
	group.Wait()
	close(results)
	codes := map[int]int{}
	for result := range results {
		codes[result]++
	}
	if codes[200] != 1 || codes[412] != 1 {
		t.Fatalf("lost update between pricing and editor: %v", codes)
	}
}

func TestProviderRejectionsHaveNoConfigOrVaultSideEffects(t *testing.T) {
	for _, providers := range []string{
		`[{"id":"main","type":"openai"},{"id":"new","api_key":"••••••••"}]`,
		`[{"id":"main","auth_type":"api_key","oauth_client_secret":"••••••••"}]`,
		`[{"id":"main","auth_type":"oauth2","api_key":"••••••••"}]`,
		`[{"id":"main"},{"id":"main","api_key":"syntheticSecret123456"}]`,
		`[{"id":"main"},{"id":"invalid ID","api_key":"syntheticSecret123456"}]`,
		`[{"id":"main","context_window":-1,"api_key":"syntheticSecret123456"}]`,
		`[{"id":"main","usage":"unknown","api_key":"syntheticSecret123456"}]`,
		`[{"id":"main","base_url":"javascript:invalid","api_key":"syntheticSecret123456"}]`,
	} {
		t.Run(providers, func(t *testing.T) {
			s, vault := newProviderTestServer(t, providerSafetyConfig)
			before, _ := os.ReadFile(s.Cfg.ConfigPath)
			snapshot := s.Cfg
			revision := s.providerETag()
			rec := providerTestWrite(s, providers, revision)
			if rec.Code != 400 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			after, _ := os.ReadFile(s.Cfg.ConfigPath)
			if !bytes.Equal(before, after) || s.Cfg != snapshot || s.providerETag() != revision {
				t.Fatal("rejected save changed configuration")
			}
			for _, id := range []string{"main", "new", "invalid ID"} {
				if _, err := vault.ReadSecret("provider_" + id + "_api_key"); err == nil {
					t.Fatal("rejected save changed vault")
				}
			}
		})
	}
}

func TestProviderInvalidStoredMaskRequiresExplicitRepair(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	if err := vault.WriteSecret("provider_main_api_key", maskedKey); err != nil {
		t.Fatal(err)
	}
	s.Cfg.ApplyVaultSecrets(vault)
	revision := s.providerETag()
	rec := providerTestWrite(s, `[{"id":"main","type":"openai","api_key":"••••••••"}]`, revision)
	if rec.Code != 400 {
		t.Fatal("invalid stored mask was silently kept")
	}
	if value, err := vault.ReadSecret("provider_main_api_key"); err != nil || value != maskedKey {
		t.Fatal("invalid credential was automatically removed")
	}
	rec = providerTestWrite(s, `[{"id":"main","type":"openai","api_key":""}]`, revision)
	if rec.Code != 200 {
		t.Fatalf("explicit repair failed: %s", rec.Body.String())
	}
}

func TestProviderGenericConfigRejectsAnyProviderFieldBeforeSecretExtraction(t *testing.T) {
	for _, value := range []string{`null`, `[]`, `[{"id":"Invalid ID!","api_key":"syntheticSecret123456"}]`} {
		s, vault := newProviderTestServer(t, providerSafetyConfig)
		before, _ := os.ReadFile(s.Cfg.ConfigPath)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"providers":`+value+`,"tts":{"api_key":"syntheticOtherKey123456"}}`))
		handleUpdateConfig(s).ServeHTTP(rec, req)
		after, _ := os.ReadFile(s.Cfg.ConfigPath)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "provider_use_api_providers") || !bytes.Equal(before, after) {
			t.Fatalf("generic provider patch accepted: %d", rec.Code)
		}
		if _, err := vault.ReadSecret("tts_api_key"); err == nil {
			t.Fatal("other secret mutated before provider rejection")
		}
	}
}

func TestProviderDeletionProtectsDormantReferencesAndIgnoresForce(t *testing.T) {
	s, _ := newProviderTestServer(t, providerSafetyConfig)
	s.Cfg.FallbackLLM.Provider = "main"
	s.Cfg.CoAgents.LLM.Provider = "main"
	s.Cfg.SIP.Voice.AgentProviderID = "main"
	_, err := deleteAgodeskProvider(s, agodesk.ConfigProviderDeletePayload{ProviderID: "main", Revision: s.providerETag(), Force: true})
	detail, ok := err.(*providerWriteError)
	if !ok || detail.code != "provider_in_use" || len(detail.references) != 4 {
		t.Fatalf("incomplete delete guard: %v", err)
	}
}

func TestProviderFailedPricingSaveDoesNotMutatePublishedSnapshot(t *testing.T) {
	s, _ := newProviderTestServer(t, providerSafetyConfig)
	s.Cfg.Budget.Enabled = true
	s.Cfg.Providers[0].Models = []config.ModelCost{{Name: "gpt-4o-mini", InputPerMillion: 1, OutputPerMillion: 2}}
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	beforeCost, known := s.BudgetTracker.EstimateCost("gpt-4o-mini", 1000000, 1000000)
	if !known {
		t.Fatal("fixture budget price unavailable")
	}
	s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "missing.yaml")
	snapshot := s.Cfg
	s.cfgSnapshot.Store(snapshot)
	revision := s.providerETag()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/providers/pricing?id=main", strings.NewReader(`[{"model_id":"gpt-4o-mini","input_per_million":42,"output_per_million":84}]`))
	req.Header.Set("If-Match", revision)
	handleApplyPricing(s, rec, req, "main")
	if rec.Code != 500 || len(snapshot.Providers[0].Models) != 1 || s.ConfigSnapshot() != snapshot || s.providerETag() != revision {
		t.Fatal("failed pricing save mutated published state")
	}
	afterCost, known := s.BudgetTracker.EstimateCost("gpt-4o-mini", 1000000, 1000000)
	if !known || afterCost != beforeCost {
		t.Fatal("failed save changed budget pricing")
	}
}

func TestProviderSetupSharesValidationAndSecretAllowlist(t *testing.T) {
	for _, id := range []string{"Invalid ID!", config.LocalLLMProviderID} {
		s, vault := newProviderTestServer(t, "providers: []")
		before, _ := os.ReadFile(s.Cfg.ConfigPath)
		patch := map[string]interface{}{"providers": []interface{}{map[string]interface{}{"id": id, "api_key": "syntheticSetupSecret123456"}}}
		if _, err := applyConfigPatch(s, patch); err == nil {
			t.Fatal("invalid setup provider accepted")
		}
		after, _ := os.ReadFile(s.Cfg.ConfigPath)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid setup changed config")
		}
		if _, err := vault.ReadSecret("provider_" + id + "_api_key"); err == nil {
			t.Fatal("invalid setup wrote secret")
		}
	}
	s, vault := newProviderTestServer(t, "providers: []")
	patch := map[string]interface{}{"llm": map[string]interface{}{"provider": "setup"}, "providers": []interface{}{map[string]interface{}{"id": "main", "type": "openai"}, map[string]interface{}{"id": "setup", "api_key": "syntheticSetupSecret123456", "unexpected_secret": "must-never-persist"}}}
	cfg, err := applyConfigPatch(s, patch)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.Cfg.ConfigPath)
	if bytes.Contains(after, []byte("syntheticSetupSecret")) || bytes.Contains(after, []byte("must-never-persist")) || cfg.FindProvider("setup") == nil {
		t.Fatal("setup allowlist failed")
	}
	if value, err := vault.ReadSecret("provider_setup_api_key"); err != nil || value != "syntheticSetupSecret123456" {
		t.Fatal("setup did not store vault credential")
	}
}

func TestProviderReferencePublicationInvalidatesStaleEditor(t *testing.T) {
	s, _ := newProviderTestServer(t, providerSafetyConfig)
	revision := s.providerETag()
	next := s.Cfg.Clone()
	next.CoAgents.LLM.Provider = "main"
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	rec := providerTestWrite(s, "[]", revision)
	if rec.Code != 412 {
		t.Fatalf("stale reference save: %d", rec.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "provider_revision_conflict" {
		t.Fatalf("unexpected conflict: %v", body)
	}
}
