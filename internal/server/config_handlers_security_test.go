package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestHandleVaultDeleteClearsInMemoryAuthSecrets(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "vault.bin"), []byte("old vault"), 0o600); err != nil {
		t.Fatalf("WriteFile(vault.bin): %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Server.MasterKey = "master"
	cfg.Auth.PasswordHash = "hash"
	cfg.Auth.SessionSecret = "session-secret"
	cfg.Auth.TOTPSecret = "totp-secret"

	s := &Server{Cfg: cfg, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodDelete, "/api/vault", nil)
	rec := httptest.NewRecorder()

	handleVaultDelete(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if cfg.Server.MasterKey != "" || cfg.Auth.PasswordHash != "" || cfg.Auth.SessionSecret != "" || cfg.Auth.TOTPSecret != "" {
		t.Fatalf("vault delete left in-memory secrets: master=%q password_hash=%q session=%q totp=%q",
			cfg.Server.MasterKey, cfg.Auth.PasswordHash, cfg.Auth.SessionSecret, cfg.Auth.TOTPSecret)
	}
}
