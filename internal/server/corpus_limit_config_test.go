package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestCorpusLimitConfigRejectsBeforeWritingAndRequiresRestart(t *testing.T) {
	for _, value := range []any{0, -1, 1048577, 1.5, 2048} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		initial := "server:\n  host: 127.0.0.1\n  port: 8080\nembeddings:\n  provider: disabled\n  corpus_max_size_mb: 1024\n"
		if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		s := &Server{Cfg: cfg, Logger: slog.Default()}
		body, _ := json.Marshal(map[string]any{"embeddings": map[string]any{"corpus_max_size_mb": value}})
		rec := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
		valid := value == 2048
		if !valid {
			after, _ := os.ReadFile(path)
			if rec.Code != 400 || string(after) != initial {
				t.Fatalf("invalid limit changed config: %d", rec.Code)
			}
			continue
		}
		if rec.Code != 200 {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
		var response map[string]any
		json.Unmarshal(rec.Body.Bytes(), &response)
		if response["needs_restart"] != true && response["restart_required"] != true {
			t.Fatalf("missing restart indication: %+v", response)
		}
		if embeddingsConfigChanged(*cfg, *s.ConfigSnapshot()) {
			t.Fatal("capacity-only change reset embeddings")
		}
	}
}
