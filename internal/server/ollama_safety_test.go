package server

import (
	"aurago/internal/config"
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
