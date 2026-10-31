package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/restorepending"
	"aurago/internal/security"
)

// assertAwaitingConfirmationThenConfirm checks the import response of a restore
// with security changes, proves an unconfirmed restore is not applied, and
// confirms it through the status endpoint.
func assertAwaitingConfirmationThenConfirm(t *testing.T, s *Server, importRec *httptest.ResponseRecorder, wantNewTokens int) {
	t.Helper()
	var resp struct {
		Awaiting        bool                            `json:"awaiting_confirmation"`
		RestartRequired bool                            `json:"restart_required"`
		RestoreID       string                          `json:"restore_id"`
		Changes         *restorepending.SecurityChanges `json:"security_changes"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Awaiting || resp.RestartRequired || resp.RestoreID == "" || resp.Changes == nil || resp.Changes.NewTokenCount != wantNewTokens {
		t.Fatalf("import response = %s", importRec.Body.String())
	}
	root, err := restorepending.ConfigRoot(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := restorepending.Apply(root, restorepending.Options{}); res != nil || err != nil {
		t.Fatalf("unconfirmed restore must not be applied: res=%+v err=%v", res, err)
	}
	confirmRestoreForTest(t, s, resp.RestoreID)
}

func confirmRestoreForTest(t *testing.T, s *Server, id string) {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"action":"confirm","id":"` + id + `"}`)
	handleBackupRestoreStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backup/restore-status", body))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"confirmed":true`) {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBackupCreateExcludesMatchRelativePathOnly(t *testing.T) {
	s := newBackupCreateTestServer(t)
	// The install path itself contains /images/, which must not hide the workdir.
	base := filepath.Join(t.TempDir(), "images", "AuraGo")
	workdir := filepath.Join(base, "workdir")
	for rel, content := range map[string]string{"keep.txt": "k", "sub/also.txt": "a", "images/skip.png": "s", "attachments/skip.bin": "s"} {
		p := filepath.Join(workdir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []*string{&s.Cfg.Directories.PromptsDir, &s.Cfg.Directories.SkillsDir, &s.Cfg.Directories.ToolsDir} {
		*d = filepath.Join(base, "empty", filepath.Base(*d)+"x")
	}
	s.Cfg.Directories.WorkspaceDir = workdir
	rec := httptest.NewRecorder()
	handleBackupCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{"include_workdir":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	if !got["agent_workspace/workdir/keep.txt"] || !got["agent_workspace/workdir/sub/also.txt"] {
		t.Fatalf("workdir under an /images/ install path was dropped: %v", got)
	}
	if got["agent_workspace/workdir/images/skip.png"] || got["agent_workspace/workdir/attachments/skip.bin"] {
		t.Fatalf("images/attachments below the workdir must still be excluded: %v", got)
	}
}

func TestBackupCreateReportsSkippedDatabaseOutsideConfigRoot(t *testing.T) {
	s := newBackupCreateTestServer(t)
	outside := filepath.Join(t.TempDir(), "elsewhere.db")
	writeTestSQLiteDB(t, outside)
	s.Cfg.SQLite.ShortTermPath = outside
	for _, d := range []*string{&s.Cfg.Directories.PromptsDir, &s.Cfg.Directories.SkillsDir, &s.Cfg.Directories.ToolsDir, &s.Cfg.Directories.WorkspaceDir} {
		*d = filepath.Join(t.TempDir(), "none")
	}
	rec := httptest.NewRecorder()
	handleBackupCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-AuraGo-Backup-Skipped") == "" {
		t.Fatal("skipped header missing")
	}
	zr, _ := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, _ := f.Open()
		var m agoManifest
		if err := json.NewDecoder(rc).Decode(&m); err != nil {
			t.Fatal(err)
		}
		rc.Close()
		if len(m.Skipped) == 0 || !strings.Contains(m.Skipped[0], "elsewhere.db") {
			t.Fatalf("manifest.skipped = %v", m.Skipped)
		}
		return
	}
	t.Fatal("manifest missing")
}

func TestBackupImportMapsConfiguredDirectoriesFromActiveConfigOnly(t *testing.T) {
	s, root := newPendingTestServer(t)
	toolsDir := filepath.Join(t.TempDir(), "mytools")
	s.Cfg.Directories.ToolsDir = toolsDir
	// The archive's config.yaml tries to redirect writes; it must be ignored.
	evil := t.TempDir()
	rec := importArchiveForTest(t, s, map[string][]byte{
		"config.yaml":                  []byte("server:\n  port: 1234\ndirectories:\n  tools_dir: " + filepath.ToSlash(evil) + "\n"),
		"agent_workspace/tools/new.py": []byte("print(1)"),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	// New tool file => security confirmation required.
	var resp struct {
		RestoreID string `json:"restore_id"`
		Awaiting  bool   `json:"awaiting_confirmation"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Awaiting {
		t.Fatalf("a new tool must require confirmation: %s", rec.Body.String())
	}
	confirmRestoreForTest(t, s, resp.RestoreID)
	rootAbs, _ := restorepending.ConfigRoot(s.Cfg.ConfigPath)
	res, err := restorepending.Apply(rootAbs, restorepending.Options{ResolveDirs: func() (restorepending.Dirs, error) {
		return restorepending.Dirs{Tools: toolsDir}, nil
	}})
	if err != nil || res.Status != restorepending.StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if b, err := os.ReadFile(filepath.Join(toolsDir, "new.py")); err != nil || string(b) != "print(1)" {
		t.Fatalf("tool not written to the configured dir: %v", err)
	}
	for _, p := range []string{filepath.Join(root, "agent_workspace", "tools", "new.py"), filepath.Join(evil, "new.py")} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("file leaked to %s", p)
		}
	}
}

func TestBackupImportSecurityChangesSummaryAndConfirmFlow(t *testing.T) {
	s, root := newPendingTestServer(t)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Server.Host = "127.0.0.1"
	rec := importArchiveForTest(t, s, map[string][]byte{
		"config.yaml": []byte("auth:\n  enabled: false\nserver:\n  host: 0.0.0.0\n"),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Awaiting bool                            `json:"awaiting_confirmation"`
		Restart  bool                            `json:"restart_required"`
		Changes  *restorepending.SecurityChanges `json:"security_changes"`
		ID       string                          `json:"restore_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Awaiting || resp.Restart || resp.Changes == nil || !resp.Changes.AuthDisabled || !resp.Changes.HostChanged || resp.Changes.HostTo != "0.0.0.0" {
		t.Fatalf("response = %s", rec.Body.String())
	}

	statusOf := func() map[string]interface{} {
		r := httptest.NewRecorder()
		handleBackupRestoreStatus(s).ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/backup/restore-status", nil))
		var m map[string]interface{}
		_ = json.Unmarshal(r.Body.Bytes(), &m)
		return m
	}
	st := statusOf()
	if st["awaiting_confirmation"] != true || st["restart_required"] != false || st["id"] != resp.ID || st["security_changes"] == nil {
		t.Fatalf("status = %v", st)
	}

	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		handleBackupRestoreStatus(s).ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/backup/restore-status", strings.NewReader(body)))
		return r
	}
	if r := post(`{"action":"confirm","id":"nope"}`); r.Code != http.StatusConflict {
		t.Fatalf("wrong id = %d", r.Code)
	}
	if r := post(`{"action":"explode","id":"x"}`); r.Code != http.StatusBadRequest {
		t.Fatalf("bad action = %d", r.Code)
	}
	if res, err := restorepending.Apply(root, restorepending.Options{}); res != nil || err != nil {
		t.Fatalf("unconfirmed restore applied: %+v %v", res, err)
	}
	if r := post(`{"action":"confirm","id":"` + resp.ID + `"}`); r.Code != http.StatusOK {
		t.Fatalf("confirm = %d %s", r.Code, r.Body.String())
	}
	if st := statusOf(); st["awaiting_confirmation"] != false || st["restart_required"] != true {
		t.Fatalf("status after confirm = %v", st)
	}
	if res, err := restorepending.Apply(root, restorepending.Options{}); err != nil || res == nil || res.Status != restorepending.StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if r := post(`{"action":"confirm","id":"` + resp.ID + `"}`); r.Code != http.StatusNotFound {
		t.Fatalf("confirm without pending = %d", r.Code)
	}
}

func TestBackupImportRejectsUnknownTokenScopes(t *testing.T) {
	s, root := newPendingTestServer(t)
	s.Vault = newTestVault(t, filepath.Join(root, "data", "vault.bin"), "07")
	plain, _ := json.Marshal([]security.Token{{ID: "t1", Name: "evil", TokenHash: "h", Scopes: []string{"admin", "root-everything"}, Enabled: true}})
	blob, err := encryptBackupPasswordBlob(tokenStoreMagic, plain, "pw")
	if err != nil {
		t.Fatal(err)
	}
	rec := importArchiveWithPassword(t, s, map[string][]byte{"data/state.json": []byte("x"), "token_store.enc": blob}, "pw")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "root-everything") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if restorepending.ReadState(root).Pending {
		t.Fatal("rejected import must not stage anything")
	}
}

func TestBackupEndpointsRequireSafeHostWhenLoginIsDisabled(t *testing.T) {
	newServer := func(authEnabled bool) *Server {
		cfg := &config.Config{}
		cfg.Auth.Enabled = authEnabled
		cfg.Server.HTTPS.Domain = "aurago.example.com"
		return &Server{Cfg: cfg, Logger: slog.Default()}
	}
	call := func(s *Server, host string) int {
		h := requireRebindingSafeHostWithoutLogin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
		req := httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	off := newServer(false)
	for host, want := range map[string]int{
		"evil.attacker.net:8088": http.StatusForbidden,
		"rebind.localhost.evil":  http.StatusForbidden,
		"192.168.1.20:8088":      http.StatusOK,
		"[::1]:8088":             http.StatusOK,
		"localhost:8088":         http.StatusOK,
		"app.localhost":          http.StatusOK,
		"aurago.example.com":     http.StatusOK,
	} {
		if got := call(off, host); got != want {
			t.Errorf("auth off, host %q = %d, want %d", host, got, want)
		}
	}
	if got := call(newServer(true), "evil.attacker.net"); got != http.StatusOK {
		t.Errorf("auth on must be unchanged, got %d", got)
	}
}

func importChangesForTest(t *testing.T, s *Server, files map[string][]byte, password string) restorepending.SecurityChanges {
	t.Helper()
	rec := importArchiveWithPassword(t, s, files, password)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Awaiting bool                            `json:"awaiting_confirmation"`
		Changes  *restorepending.SecurityChanges `json:"security_changes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Awaiting || resp.Changes == nil {
		t.Fatalf("expected confirmation: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "hashA") || strings.Contains(rec.Body.String(), "secretA") {
		t.Fatal("secret values must never be returned")
	}
	return *resp.Changes
}

func TestBackupImportVaultAuthValuesNeedConfirmation(t *testing.T) {
	s, root := newPendingTestServer(t)
	s.Vault = newTestVault(t, filepath.Join(root, "data", "vault.bin"), "08")
	s.Cfg.Auth.PasswordHash = "hashB"
	s.Cfg.Auth.TOTPEnabled = true
	s.Cfg.Auth.RequireOriginHeader = true
	src := newTestVault(t, filepath.Join(t.TempDir(), "vault.bin"), "09")
	for k, v := range map[string]string{"auth_password_hash": "hashA", "auth_totp_secret": "secretA", "auth_webauthn_state": "secretA"} {
		if err := src.WriteSecret(k, v); err != nil {
			t.Fatal(err)
		}
	}
	blob, err := exportVaultSecrets(src, "pw")
	if err != nil {
		t.Fatal(err)
	}
	ch := importChangesForTest(t, s, map[string][]byte{
		"vault_secrets.enc": blob,
		"config.yaml":       []byte("auth:\n  totp_enabled: false\n  require_origin_header: false\n"),
	}, "pw")
	if !ch.PasswordHashChanged || !ch.TOTPSecretChanged || !ch.WebAuthnChanged || !ch.TOTPDisabled || !ch.RequireOriginOff {
		t.Fatalf("changes = %+v", ch)
	}
}

func TestBackupImportTokenScopeChangeCrontabAndCredentialDBNeedConfirmation(t *testing.T) {
	s, root := newPendingTestServer(t)
	s.Vault = newTestVault(t, filepath.Join(root, "data", "vault.bin"), "0a")
	cur, _ := json.Marshal([]security.Token{{ID: "t1", TokenHash: "h", Scopes: []string{"read"}, Enabled: true}})
	enc, err := s.Vault.EncryptBytes(cur)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "tokens.json"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "crontab.json"), []byte(`[{"id":"j1","cron_expr":"* * * * *","task_prompt":"old"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	staged, _ := json.Marshal([]security.Token{{ID: "t1", TokenHash: "h", Scopes: []string{"desktop:read"}, Enabled: true}})
	blob, err := encryptBackupPasswordBlob(tokenStoreMagic, staged, "pw")
	if err != nil {
		t.Fatal(err)
	}
	ch := importChangesForTest(t, s, map[string][]byte{
		"token_store.enc":        blob,
		"data/crontab.json":      []byte(`[{"id":"j1","cron_expr":"* * * * *","task_prompt":"new"},{"id":"j2","cron_expr":"* * * * *","task_prompt":"x"}]`),
		"data/remote_control.db": makeUserVersionDB(t, 1),
		"data/short_term.db":     makeUserVersionDB(t, 1),
	}, "pw")
	if ch.NewTokenCount != 1 {
		t.Fatalf("a scope change must count as a changed token: %+v", ch)
	}
	cats := map[string]restorepending.FileChange{}
	for _, f := range ch.Files {
		cats[f.Category] = f
	}
	if c := cats["crontab"]; c.New != 1 || c.Changed != 1 {
		t.Fatalf("crontab = %+v", c)
	}
	if d := cats["databases"]; d.New != 1 || len(d.Examples) != 1 || d.Examples[0] != "remote_control.db" {
		t.Fatalf("databases = %+v (only credential databases are reported)", d)
	}
}

func TestRebindingGuardTsnetNameRules(t *testing.T) {
	mk := func(enabled bool) *Server {
		cfg := &config.Config{}
		cfg.Tailscale.TsNet.Enabled = enabled
		cfg.Tailscale.TsNet.Hostname = "aurago"
		return &Server{Cfg: cfg, Logger: slog.Default()}
	}
	for _, tc := range []struct {
		enabled bool
		host    string
		want    bool
	}{
		{true, "aurago:8088", true},
		{true, "aurago.attacker.test", false},
		{false, "aurago", false},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
		req.Host = tc.host
		if got := mk(tc.enabled).rebindingSafeHost(req); got != tc.want {
			t.Errorf("tsnet enabled=%v host %q = %v, want %v", tc.enabled, tc.host, got, tc.want)
		}
	}
}
