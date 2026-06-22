package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestLoadPlainSkillSecretsRejectsSystemManagedKeys(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", vaultPath)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("github_token", "system-secret"); err != nil {
		t.Fatalf("WriteSecret(system): %v", err)
	}
	if err := vault.WriteSecret("user_defined_api_key", "user-secret"); err != nil {
		t.Fatalf("WriteSecret(user): %v", err)
	}

	s := &Server{
		Cfg:   &config.Config{},
		Vault: vault,
	}
	s.Cfg.Tools.PythonSecretInjection.Enabled = true
	skill := &tools.SkillRegistryEntry{
		VaultKeys: []string{"github_token", "user_defined_api_key"},
	}

	got := loadPlainSkillSecrets(s, skill)
	if _, ok := got["github_token"]; ok {
		t.Fatalf("loadPlainSkillSecrets exposed system-managed github_token")
	}
	if got["user_defined_api_key"] != "user-secret" {
		t.Fatalf("loadPlainSkillSecrets user secret = %q, want user-secret", got["user_defined_api_key"])
	}
}

func TestLoadPlainSkillSecretsRequiresPythonSecretInjectionEnabled(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", vaultPath)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("user_defined_api_key", "user-secret"); err != nil {
		t.Fatalf("WriteSecret(user): %v", err)
	}

	s := &Server{
		Cfg:   &config.Config{},
		Vault: vault,
	}
	skill := &tools.SkillRegistryEntry{
		VaultKeys: []string{"user_defined_api_key"},
	}

	if got := loadPlainSkillSecrets(s, skill); len(got) != 0 {
		t.Fatalf("loadPlainSkillSecrets() = %#v, want no secrets when injection disabled", got)
	}
}

func TestHandleTestSkillForbiddenWhenDisabled(t *testing.T) {
	tmp := t.TempDir()
	db, err := tools.InitSkillsDB(filepath.Join(tmp, "skills.db"))
	if err != nil {
		t.Fatalf("InitSkillsDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	skillsDir := filepath.Join(tmp, "skills")
	mgr := tools.NewSkillManager(db, skillsDir, slog.Default())
	entry, err := mgr.CreateSkillEntry("api_blocked", "test", `def run(): return "x"`, tools.SkillTypeUser, "test", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}
	if err := mgr.EnableSkill(entry.ID, false, "test"); err != nil {
		t.Fatalf("EnableSkill: %v", err)
	}
	cfg := &config.Config{}
	cfg.Directories.SkillsDir = skillsDir
	cfg.Directories.WorkspaceDir = filepath.Join(tmp, "workspace")
	s := &Server{Cfg: cfg, SkillManager: mgr, Logger: slog.Default()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/skills/"+entry.ID+"/test", strings.NewReader(`{"args":{}}`))
	handleTestSkill(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "error" {
		t.Fatalf("body=%v", body)
	}
}