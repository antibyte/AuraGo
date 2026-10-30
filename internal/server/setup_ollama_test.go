package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const ollamaTagsFixture = `{"models":[
 {"name":"qwen3:8b","size":5200000000,"details":{"family":"qwen3","parameter_size":"8B"}},
 {"name":"nomic-embed-text:latest","size":274000000,"details":{"family":"nomic-bert","parameter_size":"137M"}},
 {"name":"llama3.1:latest","size":4900000000,"details":{"family":"llama","parameter_size":"8.0B"}},
 {"name":"all-minilm","size":46000000,"details":{"family":"bert","parameter_size":"23M"}}
]}`

// useSetupOllamaTags points the detection at a test server and clears the probe cache.
func useSetupOllamaTags(t *testing.T, target string) {
	t.Helper()
	previous := setupOllamaTagsURL
	setupOllamaTagsURL = func(bool) string { return target }
	resetSetupOllamaProbeCache()
	t.Cleanup(func() {
		setupOllamaTagsURL = previous
		resetSetupOllamaProbeCache()
	})
}

func getSetupOllama(s *Server, remoteAddr, csrf string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/setup/ollama", nil)
	req.RemoteAddr = remoteAddr
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handleSetupOllamaDetect(s).ServeHTTP(rec, req)
	return rec
}

func TestSetupOllamaListsChatModelsOnly(t *testing.T) {
	var queries []string
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		_, _ = w.Write([]byte(ollamaTagsFixture))
	}))
	defer ollama.Close()
	useSetupOllamaTags(t, ollama.URL+"/api/tags")

	s := newBootstrapTestServer(t, "127.0.0.1", false)
	addSetupCSRFTokenForTest(s, "csrf-ollama")
	// A caller-supplied target must never be read.
	req := httptest.NewRequest(http.MethodGet, "/api/setup/ollama?base_url=http://169.254.169.254/", nil)
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("X-CSRF-Token", "csrf-ollama")
	rec := httptest.NewRecorder()
	handleSetupOllamaDetect(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got setupOllamaResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Reachable || len(got.Models) != 2 || got.Models[0].Name != "llama3.1:latest" || got.Models[1].Name != "qwen3:8b" {
		t.Fatalf("result = %+v, want the two chat models sorted by name", got)
	}
	if got.Models[0].ParameterSize != "8.0B" || got.Models[0].Family != "llama" {
		t.Fatalf("model details lost: %+v", got.Models[0])
	}
	if len(queries) != 1 || queries[0] != "" {
		t.Fatalf("Ollama queries = %q, want exactly one without a query string", queries)
	}
}

func TestSetupOllamaReportsUnreachable(t *testing.T) {
	hang := make(chan struct{})
	ollama := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-hang }))
	defer ollama.Close()
	defer close(hang)
	useSetupOllamaTags(t, ollama.URL+"/api/tags")

	s := newBootstrapTestServer(t, "127.0.0.1", false)
	addSetupCSRFTokenForTest(s, "csrf-ollama")
	started := time.Now()
	rec := getSetupOllama(s, "127.0.0.1:50000", "csrf-ollama", nil)
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("detection took %s, want it bounded by the probe timeout", elapsed)
	}
	var got setupOllamaResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || got.Reachable || got.Code != "unreachable" || got.Models == nil {
		t.Fatalf("status=%d result=%+v, want reachable:false code unreachable with an empty list", rec.Code, got)
	}
}

func TestSetupOllamaRejectsForeignHostsAndBadResponses(t *testing.T) {
	useSetupOllamaTags(t, "http://192.0.2.10:11434/api/tags")
	if got := probeSetupOllama(t.Context(), "http://192.0.2.10:11434/api/tags", "http://localhost:11434/v1"); got.Reachable || got.Code != "unreachable" {
		t.Fatalf("foreign host probe = %+v, want unreachable without a request", got)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 10)))
	}))
	defer broken.Close()
	if got := probeSetupOllama(t.Context(), broken.URL+"/api/tags", "http://localhost:11434/v1"); got.Reachable || got.Code != "bad_response" {
		t.Fatalf("garbled body probe = %+v, want bad_response", got)
	}
}

func TestSetupOllamaIsGatedLikeTheOtherSetupEndpoints(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(ollamaTagsFixture))
	}))
	defer ollama.Close()
	useSetupOllamaTags(t, ollama.URL+"/api/tags")

	s := newBootstrapTestServer(t, "0.0.0.0", false)
	addSetupCSRFTokenForTest(s, "csrf-ollama")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	if rec := getSetupOllama(s, bootstrapTestRemotePeer, "", map[string]string{setupBootstrapHeader: "bootstrap-secret"}); rec.Code != http.StatusForbidden || responseCode(t, rec) != "csrf_invalid" {
		t.Fatalf("without CSRF: status=%d body=%s, want 403 csrf_invalid", rec.Code, rec.Body.String())
	}
	if rec := getSetupOllama(s, bootstrapTestRemotePeer, "csrf-ollama", nil); rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_bootstrap_token_required" {
		t.Fatalf("remote without token: status=%d body=%s, want 403 setup_bootstrap_token_required", rec.Code, rec.Body.String())
	}
	if rec := getSetupOllama(s, bootstrapTestRemotePeer, "csrf-ollama", map[string]string{setupBootstrapHeader: "bootstrap-secret"}); rec.Code != http.StatusOK {
		t.Fatalf("remote with token: status=%d body=%s, want 200", rec.Code, rec.Body.String())
	}
	if !validateSetupCSRFToken(s, "csrf-ollama", false) {
		t.Fatal("detection consumed the CSRF token the final save still needs")
	}

	s.Cfg.Auth.PasswordHash = "configured"
	if rec := getSetupOllama(s, bootstrapTestRemotePeer, "csrf-ollama", map[string]string{setupBootstrapHeader: "bootstrap-secret"}); rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_already_completed" {
		t.Fatalf("after setup: status=%d body=%s, want 403 setup_already_completed", rec.Code, rec.Body.String())
	}

	post := httptest.NewRecorder()
	handleSetupOllamaDetect(s).ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/setup/ollama", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", post.Code)
	}
}
