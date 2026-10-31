package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/tools"
)

// Config UX wave 1, batch 5: provider test, Ollama models check and treg test run with the
// values typed into the form. Nothing is saved.

type authRecorder struct {
	mu    sync.Mutex
	auths []string
}

func (a *authRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.auths = append(a.auths, r.Header.Get("Authorization"))
	a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"data":[]}`))
}

func (a *authRecorder) saw(value string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, got := range a.auths {
		if got == value {
			return true
		}
	}
	return false
}

func (a *authRecorder) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.auths)
}

func providerDraftServer(t *testing.T, savedURL string) *Server {
	t.Helper()
	s, vault := newProviderTestServer(t, fmt.Sprintf("providers:\n  - id: main\n    type: openai\n    base_url: %s/v1\n    model: fixture\nllm:\n  provider: main\n", savedURL))
	_ = vault.WriteSecret("provider_main_api_key", "saved-provider-key-1234567890")
	s.Cfg.ApplyVaultSecrets(vault)
	s.Cfg.ResolveProviders()
	return s
}

func TestProviderTestUsesDraftProvider(t *testing.T) {
	saved, other := &authRecorder{}, &authRecorder{}
	savedSrv, otherSrv := httptest.NewServer(saved), httptest.NewServer(other)
	defer savedSrv.Close()
	defer otherSrv.Close()
	s := providerDraftServer(t, savedSrv.URL)
	post := func(provider string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		body := `{"provider_id":"main","probe_kind":"metadata","provider":` + provider + `}`
		handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(body)))
		return rec
	}

	// A typed key is used as is.
	rec := post(fmt.Sprintf(`{"type":"openai","base_url":"%s/v1","api_key":"typed-draft-key-123456"}`, savedSrv.URL))
	if rec.Code != 200 || !saved.saw("Bearer typed-draft-key-123456") {
		t.Fatalf("typed key not used: %d %s auth=%v", rec.Code, rec.Body.String(), saved.auths)
	}
	// A masked key falls back to the saved key while the target is unchanged.
	rec = post(fmt.Sprintf(`{"type":"openai","base_url":"%s/v1","api_key":"••••••••"}`, savedSrv.URL))
	if rec.Code != 200 || !saved.saw("Bearer saved-provider-key-1234567890") {
		t.Fatalf("saved key not used for the same target: %d %s auth=%v", rec.Code, rec.Body.String(), saved.auths)
	}
	// The typed base URL is probed, and the saved key never travels there.
	rec = post(fmt.Sprintf(`{"type":"openai","base_url":"%s/v1","api_key":"••••••••"}`, otherSrv.URL))
	if other.count() != 0 && other.saw("Bearer saved-provider-key-1234567890") {
		t.Fatalf("saved key was sent to a changed target: %v", other.auths)
	}
	rec = post(fmt.Sprintf(`{"type":"openai","base_url":"%s/v1","api_key":"typed-other-key-123456"}`, otherSrv.URL))
	if rec.Code != 200 || !other.saw("Bearer typed-other-key-123456") {
		t.Fatalf("draft URL was not probed: %d %s auth=%v", rec.Code, rec.Body.String(), other.auths)
	}
	// A draft never becomes the provider's last test result.
	if _, ok := s.providerProbeHistory.Load("main"); ok {
		t.Fatal("draft probe was recorded as the saved provider's last test")
	}
	if got := s.Cfg.FindProvider("main").BaseURL; !strings.HasPrefix(got, savedSrv.URL) {
		t.Fatalf("saved provider changed: %q", got)
	}
}

func TestProviderTestDraftWorksForNewProvider(t *testing.T) {
	fixture := &authRecorder{}
	srv := httptest.NewServer(fixture)
	defer srv.Close()
	s, _ := newProviderTestServer(t, "providers: []\n")
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"provider_id":"fresh","provider":{"type":"openai","base_url":"%s/v1","api_key":"fresh-key-12345678901","model":"m"}}`, srv.URL)
	handleProviderTest(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/providers/test", strings.NewReader(body)))
	if rec.Code != 200 || !fixture.saw("Bearer fresh-key-12345678901") {
		t.Fatalf("new provider draft was not probed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOllamaModelsCheckUsesDraftURL(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/api/version" {
			_, _ = w.Write([]byte(`{"version":"0.9.9"}`))
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3"}]}`))
	}))
	defer srv.Close()
	s := newDraftTestServer(t)
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"draft":{"ollama":{"url":%q}}}`, srv.URL)
	handleOllamaModels(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/ollama/models?integration=ollama", strings.NewReader(body)))
	if rec.Code != 200 || hits == 0 || !strings.Contains(rec.Body.String(), "llama3") {
		t.Fatalf("draft URL was not checked: %d %s", rec.Code, rec.Body.String())
	}
	if got := s.ConfigSnapshot().Ollama.URL; got == srv.URL {
		t.Fatalf("saved snapshot was changed: %q", got)
	}
	// POST is only for the integration mode.
	rec = httptest.NewRecorder()
	handleOllamaModels(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/ollama/models", strings.NewReader(body)))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("plain POST allowed: %d", rec.Code)
	}
}

func TestTregTestUsesDraftToken(t *testing.T) {
	s := newDraftTestServer(t)
	s.Cfg.Agent.AllowNetworkRequests = true
	s.initConfigSnapshot()
	var gotToken string
	prev := newTregServerClient
	newTregServerClient = func(token string) (*tools.TregClient, error) {
		gotToken = token
		return nil, fmt.Errorf("stop after token capture")
	}
	t.Cleanup(func() { newTregServerClient = prev })
	rec := postDraft(handleTreg(s), "/api/treg/test-connection", `{"draft":{"treg":{"token":"typed-treg-token-1234567"}}}`)
	if gotToken != "typed-treg-token-1234567" {
		t.Fatalf("draft token not used: %q (%d %s)", gotToken, rec.Code, rec.Body.String())
	}
	// Without a draft and without a saved token the test asks for a token.
	gotToken = ""
	rec = postDraft(handleTreg(s), "/api/treg/test-connection", "")
	if gotToken != "" || rec.Code != http.StatusBadRequest {
		t.Fatalf("empty token reached the client: %q (%d %s)", gotToken, rec.Code, rec.Body.String())
	}
}
