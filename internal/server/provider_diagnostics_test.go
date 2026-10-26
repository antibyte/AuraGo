package server

import (
	"aurago/internal/budget"
	"aurago/internal/security"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderStatusIsPassiveAndProbeRequiresSavedTarget(t *testing.T) {
	var calls atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer fixture.Close()
	s, _ := newProviderTestServer(t, fmt.Sprintf("providers:\n  - id: main\n    type: openai\n    base_url: %s/v1\n    model: fixture\nllm:\n  provider: main\n", fixture.URL))
	for range 3 {
		rec := httptest.NewRecorder()
		handleProviderStatus(s).ServeHTTP(rec, httptest.NewRequest("GET", "/api/providers/status", nil))
		if rec.Code != 200 || rec.Header().Get("ETag") == "" || calls.Load() != 0 {
			t.Fatal("passive status contacted provider")
		}
	}
	for _, body := range []string{`{"provider_id":"main","base_url":"https://untrusted.invalid"}`, `{"provider_id":"main","api_key":"override"}`, `{"provider_id":"unknown"}`} {
		rec := httptest.NewRecorder()
		handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(body)))
		if rec.Code == 200 || calls.Load() != 0 {
			t.Fatal("probe accepted a target or credential override")
		}
	}
	rec := httptest.NewRecorder()
	handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(`{"provider_id":"main"}`)))
	if rec.Code != 200 || calls.Load() != 1 || !strings.Contains(rec.Body.String(), "metadata_probe_ok") {
		t.Fatalf("metadata probe failed: %s", rec.Body.String())
	}
}

func TestProviderProbeSerializesPerProviderAndFencesChangedRevision(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer fixture.Close()
	s, _ := newProviderTestServer(t, fmt.Sprintf("providers:\n  - id: main\n    type: openai\n    base_url: %s/v1\n    model: fixture\nllm:\n  provider: main\n", fixture.URL))
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(`{"provider_id":"main"}`)))
		done <- rec
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	second := httptest.NewRecorder()
	handleProviderTest(s).ServeHTTP(second, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(`{"provider_id":"main"}`)))
	if second.Code != 409 || !strings.Contains(second.Body.String(), "provider_test_busy") {
		t.Fatalf("concurrent test admitted: %d", second.Code)
	}
	s.CfgMu.Lock()
	next := s.Cfg.Clone()
	next.LLM.Model = "changed"
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	close(release)
	select {
	case result := <-done:
		var body map[string]any
		_ = json.Unmarshal(result.Body.Bytes(), &body)
		if result.Code != 200 || body["current"] != false {
			t.Fatalf("changed config was confirmed: %s", result.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("probe did not finish")
	}
}

func TestProviderGenerationProbeBooksExactlyOnePhysicalCall(t *testing.T) {
	var calls atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"fixture-model","choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`))
	}))
	defer fixture.Close()
	s, vault := newProviderTestServer(t, fmt.Sprintf("providers:\n  - id: main\n    type: openai\n    base_url: %s/v1\n    model: fixture-model\nllm:\n  provider: main\nbudget:\n  enabled: true\n  daily_limit_usd: 5\n", fixture.URL))
	_ = vault.WriteSecret("provider_main_api_key", "fixtureProbeSecret123456")
	s.Cfg.ApplyVaultSecrets(vault)
	s.Cfg.ResolveProviders()
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	rec := httptest.NewRecorder()
	handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(`{"provider_id":"main","probe_kind":"generation"}`)))
	status := s.BudgetTracker.GetStatus()
	if rec.Code != 200 || calls.Load() != 1 || status.Models["fixture-model"].Calls != 1 {
		t.Fatalf("probe was retried or not booked: calls=%d budget=%+v response=%s", calls.Load(), status.Models, rec.Body.String())
	}
}

func TestProviderAuxiliaryOAuthSecretsAreScopedAndRotate(t *testing.T) {
	s, vault := newProviderTestServer(t, `providers:
 - id: sip-only
   type: openai
   auth_type: oauth2
llm:
 provider: ""
sip:
 voice:
  agent_provider_id: sip-only
`)
	const old = "auxiliaryOldOAuthToken123456789"
	const next = "auxiliaryNewOAuthToken987654321"
	_ = vault.WriteSecret("oauth_sip-only", `{"access_token":"`+old+`"}`)
	var scope llmSecretScope
	scope.register(s.Cfg, vault)
	defer scope.replaceAccessTokens(nil)
	if security.Scrub(old) == old {
		t.Fatal("SIP-only OAuth token was not registered")
	}
	_ = vault.WriteSecret("oauth_sip-only", `{"access_token":"`+next+`"}`)
	scope.register(s.Cfg, vault)
	if security.Scrub(old) != old || security.Scrub(next) == next {
		t.Fatal("scoped registration did not rotate")
	}
}

func TestProviderManagedVaultWritesCannotBypassRevisionProtocol(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	_ = vault.WriteSecret("provider_main_api_key", "oldManagedKey123456")
	revision := s.providerETag()
	for _, method := range []string{"POST", "DELETE"} {
		rec := httptest.NewRecorder()
		if method == "POST" {
			handleSetVaultSecret(s, rec, httptest.NewRequest(method, "/api/vault/secrets", strings.NewReader(`{"key":"provider_main_api_key","value":"newManagedKey654321"}`)))
		} else {
			handleDeleteVaultSecret(s, rec, httptest.NewRequest(method, "/api/vault/secrets?key=provider_main_api_key", nil))
		}
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "provider_use_api_providers") {
			t.Fatal("managed credential bypass accepted")
		}
	}
	key, _ := vault.ReadSecret("provider_main_api_key")
	if key != "oldManagedKey123456" || s.providerETag() != revision {
		t.Fatal("rejected credential write changed state")
	}
}

type failingProviderVault struct {
	values       map[string]string
	writes       int
	failRollback bool
}

func (v *failingProviderVault) ReadSecret(key string) (string, error) {
	value, ok := v.values[key]
	if !ok {
		return "", security.ErrSecretNotFound
	}
	return value, nil
}
func (v *failingProviderVault) WriteSecret(key, value string) error {
	v.writes++
	if v.writes == 2 || v.failRollback && v.writes > 2 {
		return errors.New("simulated storage failure")
	}
	v.values[key] = value
	return nil
}
func (v *failingProviderVault) DeleteSecret(key string) error { delete(v.values, key); return nil }
func TestProviderVaultMutationsRollBackAndReportRollbackFailures(t *testing.T) {
	for _, failRollback := range []bool{false, true} {
		vault := &failingProviderVault{values: map[string]string{"first": "old-first", "second": "old-second"}, failRollback: failRollback}
		_, err := applyVaultMutations(vault, []vaultMutation{{key: "first", value: "new-first"}, {key: "second", value: "new-second"}})
		if err == nil {
			t.Fatal("storage failure not reported")
		}
		if !failRollback && !reflect.DeepEqual(vault.values, map[string]string{"first": "old-first", "second": "old-second"}) {
			t.Fatal("vault rollback lost original state")
		}
		if failRollback {
			var detail *providerWriteError
			if !errors.As(err, &detail) || detail.code != "provider_rollback_failed" {
				t.Fatal("rollback failure was hidden")
			}
		}
	}
}
