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
