package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type ollamaTestLogger struct{}

func (ollamaTestLogger) Info(string, ...any)  {}
func (ollamaTestLogger) Warn(string, ...any)  {}
func (ollamaTestLogger) Error(string, ...any) {}

func TestOllamaReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := OllamaConfig{ReadOnly: true}

	for name, got := range map[string]string{
		"pull":   OllamaPullModel(cfg, "llama3:latest"),
		"delete": OllamaDeleteModel(cfg, "llama3:latest"),
		"copy":   OllamaCopyModel(cfg, "llama3:latest", "llama3-copy:latest"),
		"load":   OllamaLoadModel(cfg, "llama3:latest"),
		"unload": OllamaUnloadModel(cfg, "llama3:latest"),
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, "read-only mode") {
				t.Fatalf("response = %s, want read-only denial", got)
			}
		})
	}
}

func TestOllamaModelManagementUsesCurrentModelField(t *testing.T) {
	var mu sync.Mutex
	bodies := map[string]map[string]interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode %s body: %v", r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodies[r.URL.Path] = body
		mu.Unlock()

		switch r.URL.Path {
		case "/api/show":
			_, _ = w.Write([]byte(`{"model_info":{"general.architecture":"llama","llama.context_length":8192,"tokenizer.ggml.tokens":"huge"},"details":{"family":"llama"}}`))
		case "/api/pull":
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/delete":
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/generate":
			_, _ = w.Write([]byte(`{"done":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := OllamaConfig{URL: server.URL}
	for name, call := range map[string]func() string{
		"show":   func() string { return OllamaShowModel(cfg, "llama3:latest") },
		"pull":   func() string { return OllamaPullModel(cfg, "llama3:latest") },
		"delete": func() string { return OllamaDeleteModel(cfg, "llama3:latest") },
		"load":   func() string { return OllamaLoadModel(cfg, "llama3:latest") },
	} {
		got := call()
		if !strings.Contains(got, `"status":"ok"`) {
			t.Fatalf("%s response = %s, want ok", name, got)
		}
		if name == "show" {
			if strings.Contains(got, "tokenizer.ggml.tokens") {
				t.Fatalf("show kept an unrelated model_info key: %s", got)
			}
			if !strings.Contains(got, "llama.context_length") || !strings.Contains(got, "general.architecture") {
				t.Fatalf("show dropped required model_info: %s", got)
			}
		}
	}

	for _, path := range []string{"/api/show", "/api/pull", "/api/delete", "/api/generate"} {
		mu.Lock()
		body := bodies[path]
		mu.Unlock()
		if body == nil {
			t.Fatalf("missing request body for %s", path)
		}
		if _, ok := body["name"]; ok {
			t.Fatalf("%s body used deprecated name field: %#v", path, body)
		}
		if body["model"] != "llama3:latest" {
			t.Fatalf("%s model = %#v, want llama3:latest; body=%#v", path, body["model"], body)
		}
	}
	mu.Lock()
	load := bodies["/api/generate"]
	mu.Unlock()
	if load["prompt"] != "" {
		t.Fatalf("load prompt = %#v, want empty string", load["prompt"])
	}
	keep, ok := load["keep_alive"].(float64)
	if !ok || keep != -1 {
		t.Fatalf("load keep_alive = %#v, want -1", load["keep_alive"])
	}
}

func TestPullModelIfNeededUsesCurrentModelField(t *testing.T) {
	var mu sync.Mutex
	bodies := map[string]map[string]interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode %s body: %v", r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodies[r.URL.Path] = body
		mu.Unlock()

		switch r.URL.Path {
		case "/api/show":
			w.WriteHeader(http.StatusNotFound)
		case "/api/pull":
			_, _ = w.Write([]byte(`{"status":"success"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := ollamaEnsureModel(OllamaConfig{URL: server.URL}, "nomic-embed-text"); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/show", "/api/pull"} {
		mu.Lock()
		body := bodies[path]
		mu.Unlock()
		if body == nil {
			t.Fatalf("missing request body for %s", path)
		}
		if _, ok := body["name"]; ok {
			t.Fatalf("%s body used deprecated name field: %#v", path, body)
		}
		if body["model"] != "nomic-embed-text" {
			t.Fatalf("%s model = %#v, want nomic-embed-text; body=%#v", path, body["model"], body)
		}
	}
}
