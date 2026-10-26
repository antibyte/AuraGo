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
	if err := vault.WriteAgentSecret("user_defined_api_key", "agent-secret"); err != nil {
		t.Fatalf("WriteAgentSecret: %v", err)
	}
	if err := vault.WriteSecret("modal_api_key", "modal-secret"); err != nil {
		t.Fatalf("WriteSecret(modal): %v", err)
	}

	s := &Server{
		Cfg:   &config.Config{},
		Vault: vault,
	}
	s.Cfg.Tools.PythonSecretInjection.Enabled = true
	skill := &tools.SkillRegistryEntry{
		VaultKeys: []string{"github_token", "user_defined_api_key", "modal_api_key"},
	}

	got := loadPlainSkillSecrets(s, skill)
	if _, ok := got["github_token"]; ok {
		t.Fatalf("loadPlainSkillSecrets exposed system-managed github_token")
	}
	if got["user_defined_api_key"] != "agent-secret" {
		t.Fatalf("loadPlainSkillSecrets agent secret = %q, want agent-secret", got["user_defined_api_key"])
	}
	if _, ok := got["modal_api_key"]; ok {
		t.Fatalf("loadPlainSkillSecrets exposed modal/user-supplied secret")
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
	used, err := mgr.GetSkill(entry.ID)
	if err != nil || used.Usage.Attempts != 1 || used.Usage.Failures != 1 {
		t.Fatalf("usage=%+v err=%v", used, err)
	}
}

func TestApprovePythonSkillWarningRoute(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   tools.SecurityStatus
		readOnly bool
		method   string
		want     int
	}{
		{"warning", tools.SecurityWarning, false, http.MethodPost, http.StatusOK},
		{"read only", tools.SecurityWarning, true, http.MethodPost, http.StatusForbidden},
		{"clean", tools.SecurityClean, false, http.MethodPost, http.StatusBadRequest},
		{"dangerous", tools.SecurityDangerous, false, http.MethodPost, http.StatusBadRequest},
		{"pending", tools.SecurityPending, false, http.MethodPost, http.StatusBadRequest},
		{"wrong method", tools.SecurityWarning, false, http.MethodGet, http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			db, err := tools.InitSkillsDB(filepath.Join(tmp, "skills.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			mgr := tools.NewSkillManager(db, filepath.Join(tmp, "skills"), slog.Default())
			entry, err := mgr.CreateSkillEntry("approval", "test", `def run(): return "ok"`, tools.SkillTypeUser, "test", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := mgr.UpdateSkillSecurity(entry.ID, tc.status, &tools.SecurityReport{}); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{}
			cfg.WebConfig.Enabled = true
			cfg.Tools.SkillManager.Enabled = true
			cfg.Tools.SkillManager.ReadOnly = tc.readOnly
			s := &Server{Cfg: cfg, SkillManager: mgr, Logger: slog.Default()}
			mux := http.NewServeMux()
			s.registerToolAPIRoutes(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, "/api/skills/"+entry.ID+"/approve-warning", nil))
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
			after, err := mgr.GetSkill(entry.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.WarningApproved != (tc.want == http.StatusOK) {
				t.Fatalf("approval changed unexpectedly: %+v", after)
			}
		})
	}
}
