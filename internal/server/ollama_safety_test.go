package server

import (
	"aurago/internal/config"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaReadOnlyRefusesConfigAndRecreate(t *testing.T) {
	for _, embeddings := range []bool{false, true} {
		var cfg config.Config
		cfg.Docker.Enabled = true
		cfg.Docker.ReadOnly = true
		cfg.Ollama.ManagedInstance.Enabled = !embeddings
		cfg.Embeddings.LocalOllama.Enabled = embeddings
		if validateManagedDockerBackends(cfg, config.Runtime{}) == nil {
			t.Fatal("accepted read-only lifecycle")
		}
		cfg.Ollama.ManagedInstance.Enabled = true
		w := httptest.NewRecorder()
		handleOllamaManagedRecreate(&Server{Cfg: &cfg})(w, httptest.NewRequest(http.MethodPost, "/api/ollama/managed/recreate", nil))
		if w.Code != 403 || !strings.Contains(w.Body.String(), "docker_write_disabled") {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
}

func TestOllamaConnectionTestUsesSavedEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"fixture"}]}`)
		case "/api/version":
			fmt.Fprint(w, `{"version":"fixture"}`)
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	var cfg config.Config
	cfg.Ollama.URL = srv.URL
	w := httptest.NewRecorder()
	handleOllamaModels(&Server{Cfg: &cfg})(w, httptest.NewRequest("GET", "/api/ollama/models?integration=ollama&url=http://169.254.169.254", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"fixture"`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestOllamaConnectionTestNormalizesSavedBasePath(t *testing.T) {
	for _, test := range []struct{ integration, prefix, suffix string }{
		{"ollama", "", "/"},
		{"ollama", "/ollama", "/"},
		{"embeddings", "", "/v1/"},
		{"embeddings", "/ollama", "/v1/"},
	} {
		t.Run(test.integration+test.prefix, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case test.prefix + "/api/tags":
					fmt.Fprint(w, `{"models":[]}`)
				case test.prefix + "/api/version":
					fmt.Fprint(w, `{"version":"fixture"}`)
				default:
					t.Errorf("unexpected backend path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer backend.Close()
			var cfg config.Config
			cfg.Ollama.URL = backend.URL + test.prefix + test.suffix
			cfg.Embeddings.BaseURL = cfg.Ollama.URL
			response := httptest.NewRecorder()
			handleOllamaModels(&Server{Cfg: &cfg})(response, httptest.NewRequest(http.MethodGet, "/api/ollama/models?integration="+test.integration, nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"version":"fixture"`) {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
		})
	}
}
