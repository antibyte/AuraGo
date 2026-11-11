package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/dbutil"
	"aurago/internal/desktopstore"
)

func deepTutorPolicyServer(t *testing.T, pending bool) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Auth.Enabled, cfg.Auth.PasswordHash = true, "fixture-hash"
	cfg.Directories.DataDir = t.TempDir()
	cfg.ConfigPath = filepath.Join(cfg.Directories.DataDir, "config.yaml")
	if err := os.WriteFile(cfg.ConfigPath, []byte("auth:\n  enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := dbutil.Open(filepath.Join(cfg.Directories.DataDir, config.DesktopStoreDBFilename))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	query := `CREATE TABLE desktop_store_apps (app_id TEXT); INSERT INTO desktop_store_apps VALUES ('deeptutor')`
	if pending {
		query = `CREATE TABLE desktop_store_operations (app_id TEXT, status TEXT); INSERT INTO desktop_store_operations VALUES ('deeptutor', 'pending')`
	}
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
	return &Server{Cfg: cfg, Logger: slog.Default()}
}

func TestDeepTutorConfigChangesFailBeforeMutation(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "installed", true: "installing"}[pending], func(t *testing.T) {
			s := deepTutorPolicyServer(t, pending)
			before, _ := os.ReadFile(s.Cfg.ConfigPath)
			patch := map[string]interface{}{"auth": map[string]interface{}{"enabled": false, "password_hash": ""}, "directories": map[string]interface{}{"data_dir": t.TempDir()}}
			if _, err := applyConfigPatch(s, patch); !errors.Is(err, desktopstore.ErrDeepTutorRequiresLogin) {
				t.Fatalf("internal save: %v", err)
			}
			if err := patchAuthConfig(s, map[string]interface{}{"enabled": false, "password_hash": ""}); !errors.Is(err, desktopstore.ErrDeepTutorRequiresLogin) {
				t.Fatalf("auth save: %v", err)
			}
			w := httptest.NewRecorder()
			handleUpdateConfig(s)(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"auth":{"enabled":false,"password_hash":""}}`)))
			if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"deeptutor_requires_login"`) {
				t.Fatalf("save: %d %s", w.Code, w.Body)
			}
			after, _ := os.ReadFile(s.Cfg.ConfigPath)
			if string(after) != string(before) || !s.ConfigSnapshot().Auth.Enabled || s.DesktopStore != nil {
				t.Fatal("rejected save mutated config or initialized Store")
			}
		})
	}
}

func TestDeepTutorInvalidStartupAndReload(t *testing.T) {
	s := deepTutorPolicyServer(t, false)
	valid := s.ConfigSnapshot()
	invalid := valid.Clone()
	invalid.Auth.Enabled = false
	if err := Start(StartOptions{Cfg: invalid}); !errors.Is(err, desktopstore.ErrDeepTutorRequiresLogin) {
		t.Fatalf("start: %v", err)
	}
	s.replaceConfigSnapshot(invalid)
	if s.ConfigSnapshot() != valid {
		t.Fatal("invalid reload replaced last valid snapshot")
	}
	invalid = valid.Clone()
	invalid.Auth.PasswordHash = ""
	if err := validateDeepTutorLogin(context.Background(), invalid, valid); !errors.Is(err, desktopstore.ErrDeepTutorRequiresLogin) {
		t.Fatalf("missing password: %v", err)
	}
}

func TestDeepTutorAdmissionWaitsForConfigSave(t *testing.T) {
	s := deepTutorPolicyServer(t, true)
	s.CfgSaveMu.Lock()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		release, err := (storeLLMRelayProvider{s: s}).AdmitDeepTutor(context.Background())
		if err == nil {
			release()
		}
		done <- err
	}()
	<-started
	select {
	case <-done:
		t.Fatal("admission bypassed save lock")
	default:
	}
	invalid := s.Cfg.Clone()
	invalid.Auth.Enabled = false
	// Simulate the completed opposite ordering: admission must see the saved policy.
	s.Cfg = invalid
	s.cfgSnapshot.Store(invalid)
	s.CfgSaveMu.Unlock()
	if err := <-done; !errors.Is(err, desktopstore.ErrDeepTutorRequiresLogin) {
		t.Fatalf("admission: %v", err)
	}
}
