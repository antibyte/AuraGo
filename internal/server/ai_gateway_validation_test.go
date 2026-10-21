package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestAIGatewayInvalidSaveDoesNotWriteVaultOrConfig(t *testing.T) {
	for _, section := range []map[string]any{
		{"gateway_id": "bad?query"},
		{"metadata": map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6"}},
		{"metadata": map[string]string{"": "bad"}},
		{"request_timeout_ms": 300001},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		before := []byte("ai_gateway:\n  enabled: true\n  account_id: fixture-account\n  gateway_id: fixture-gateway\n")
		if err := os.WriteFile(path, before, 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConfigPath = path
		vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(dir, "vault.bin"))
		if err != nil {
			t.Fatal(err)
		}
		section["token"] = "fixture-uncommitted-token"
		body, _ := json.Marshal(map[string]any{"ai_gateway": section})
		s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
		w := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid save status=%d", w.Code)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid save changed configuration")
		}
		if secret, err := vault.ReadSecret("ai_gateway_token"); err == nil && secret != "" {
			t.Fatal("invalid save wrote a secret")
		}
	}
}
