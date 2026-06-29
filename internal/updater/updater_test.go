package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestCheckUpdatesBinaryReturnsErrorOnNonOKRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	got := CheckUpdates(context.Background(), CheckOptions{
		InstallDir:    t.TempDir(),
		ReleaseAPIURL: srv.URL,
		HTTPClient:    srv.Client(),
	})

	if got.Mode != "binary" {
		t.Fatalf("Mode = %q, want binary", got.Mode)
	}
	if got.Error == "" {
		t.Fatalf("expected non-200 release response to be reported as an error: %+v", got)
	}
}

func TestCheckUpdatesBinaryUnknownVersionAllowsInstallation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	t.Cleanup(srv.Close)

	got := CheckUpdates(context.Background(), CheckOptions{
		InstallDir:    t.TempDir(),
		ReleaseAPIURL: srv.URL,
		HTTPClient:    srv.Client(),
	})

	if got.CurrentVersion != "unknown" {
		t.Fatalf("CurrentVersion = %q, want unknown", got.CurrentVersion)
	}
	if !got.UpdateAvailable {
		t.Fatalf("unknown installed version with known latest release must allow install: %+v", got)
	}
	if got.LatestVersion != "v9.9.9" {
		t.Fatalf("LatestVersion = %q", got.LatestVersion)
	}
}

func TestCheckUpdatesGitCountsPendingCommits(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0"}`))
	}))
	t.Cleanup(srv.Close)

	runner := func(dir, name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "describe"):
			return []byte("v1.0.0\n"), nil
		case strings.Contains(joined, "fetch"):
			return nil, nil
		case strings.Contains(joined, "rev-list"):
			return []byte("3\n"), nil
		case strings.Contains(joined, "log"):
			return []byte("abc123 fix updater\n"), nil
		default:
			return nil, errors.New("unexpected command: " + name + " " + joined)
		}
	}

	got := CheckUpdates(context.Background(), CheckOptions{
		InstallDir:    dir,
		ReleaseAPIURL: srv.URL,
		HTTPClient:    srv.Client(),
		RunCommand:    runner,
	})

	if got.Mode != "git" || !got.UpdateAvailable || got.CommitCount != 3 {
		t.Fatalf("unexpected git check result: %+v", got)
	}
	if !strings.Contains(got.Changelog, "fix updater") {
		t.Fatalf("missing changelog: %+v", got)
	}
}

func TestValidateInstallRuntimeGates(t *testing.T) {
	installDir := t.TempDir()
	scriptPath := filepath.Join(installDir, "update.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write update.sh: %v", err)
	}
	okLookPath := func(name string) (string, error) { return "/bin/bash", nil }

	t.Run("self update disabled", func(t *testing.T) {
		cfg := &config.Config{}
		err := ValidateInstall(cfg, installDir, "linux", okLookPath)
		if err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("expected disabled error, got %v", err)
		}
	})

	t.Run("docker blocked", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Agent.AllowSelfUpdate = true
		cfg.Runtime.IsDocker = true
		err := ValidateInstall(cfg, installDir, "linux", okLookPath)
		if err == nil || !strings.Contains(err.Error(), "Docker") {
			t.Fatalf("expected Docker error, got %v", err)
		}
	})

	t.Run("non linux blocked", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Agent.AllowSelfUpdate = true
		err := ValidateInstall(cfg, installDir, "windows", okLookPath)
		if err == nil || !strings.Contains(err.Error(), "Linux") {
			t.Fatalf("expected Linux-only error, got %v", err)
		}
	})

	t.Run("missing bash blocked", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Agent.AllowSelfUpdate = true
		err := ValidateInstall(cfg, installDir, "linux", func(name string) (string, error) {
			return "", errors.New("not found")
		})
		if err == nil || !strings.Contains(err.Error(), "bash") {
			t.Fatalf("expected bash error, got %v", err)
		}
	})

	t.Run("missing script blocked", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Agent.AllowSelfUpdate = true
		err := ValidateInstall(cfg, filepath.Join(installDir, "missing"), "linux", okLookPath)
		if err == nil || !strings.Contains(err.Error(), "update.sh") {
			t.Fatalf("expected update.sh error, got %v", err)
		}
	})
}

func TestStartInstallPreservesMasterKeyOnlyForTrustedUpdateScript(t *testing.T) {
	installDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(installDir, "update.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write update.sh: %v", err)
	}
	t.Setenv("AURAGO_MASTER_KEY", strings.Repeat("a", 64))
	t.Setenv("OPENAI_API_KEY", "must-not-leak")

	cfg := &config.Config{}
	cfg.Agent.AllowSelfUpdate = true
	var gotEnv []string
	_, err := StartInstall(StartInstallOptions{
		Cfg:        cfg,
		InstallDir: installDir,
		GOOS:       "linux",
		LookPath:   func(name string) (string, error) { return "/bin/bash", nil },
		StartScript: func(launch ScriptLaunch) error {
			gotEnv = append([]string(nil), launch.Env...)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("StartInstall returned error: %v", err)
	}

	if !envContains(gotEnv, "AURAGO_MASTER_KEY="+strings.Repeat("a", 64)) {
		t.Fatalf("trusted update script env is missing AURAGO_MASTER_KEY: %#v", gotEnv)
	}
	if envContainsPrefix(gotEnv, "OPENAI_API_KEY=") {
		t.Fatalf("trusted update script env leaked OPENAI_API_KEY: %#v", gotEnv)
	}
}

func envContains(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

func envContainsPrefix(env []string, prefix string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}
