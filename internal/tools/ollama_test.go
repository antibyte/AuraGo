package tools

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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
			_, _ = w.Write([]byte(`{"model_info":{"general.architecture":"llama"},"details":{"family":"llama"}}`))
		case "/api/pull":
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/delete":
			_, _ = w.Write([]byte(`{"status":"success"}`))
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
	} {
		if got := call(); !strings.Contains(got, `"status":"ok"`) {
			t.Fatalf("%s response = %s, want ok", name, got)
		}
	}

	for _, path := range []string{"/api/show", "/api/pull", "/api/delete"} {
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

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split server host: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	pullModelIfNeeded(port, "nomic-embed-text", ollamaTestLogger{})

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
