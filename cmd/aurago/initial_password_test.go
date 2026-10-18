package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/server"
)

func TestResolveInitialPasswordFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-password")
	if err := os.WriteFile(path, []byte("correct horse battery staple\n"), 0600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	got, err := resolveInitialPassword("", path)
	if err != nil {
		t.Fatalf("resolveInitialPassword returned error: %v", err)
	}
	if got != "correct horse battery staple" {
		t.Fatalf("password = %q", got)
	}
}

func TestResolveInitialPasswordRejectsFlagAndFileTogether(t *testing.T) {
	_, err := resolveInitialPassword("secret", "secret-file")
	if err == nil {
		t.Fatal("expected error when both password flag and password file are set")
	}
}

func loadFreshInstallConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  host: 127.0.0.1\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// config.Load leaves DataDir empty without a data_dir entry; a real
	// config.yaml always sets it.
	cfg.Directories.DataDir = filepath.Join(dir, "data")
	if _, err := os.Stat(cfg.Directories.DataDir); !os.IsNotExist(err) {
		t.Fatalf("fresh install data dir should not exist yet, stat err = %v", err)
	}
	return cfg, path
}

func TestApplyInitialPasswordCreatesMissingDataDir(t *testing.T) {
	cfg, path := loadFreshInstallConfig(t)
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(raw)
	const password = "correct horse battery staple"

	if err := applyInitialPassword(cfg, path, password, key); err != nil {
		t.Fatalf("applyInitialPassword: %v", err)
	}

	if info, err := os.Stat(cfg.Directories.DataDir); err != nil || !info.IsDir() {
		t.Fatalf("data dir not created: info=%v err=%v", info, err)
	}
	v, err := security.NewVault(key, filepath.Join(cfg.Directories.DataDir, "vault.bin"))
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	hash, err := v.ReadSecret("auth_password_hash")
	if err != nil {
		t.Fatalf("read password hash: %v", err)
	}
	if !server.CheckPassword(password, hash) {
		t.Fatal("stored password hash does not match the password")
	}
	if sec, err := v.ReadSecret("auth_session_secret"); err != nil || sec == "" {
		t.Fatalf("session secret missing: %q, %v", sec, err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !loaded.Auth.Enabled {
		t.Fatal("auth was not enabled in config.yaml")
	}
}

func TestApplyInitialPasswordRejectsInvalidMasterKey(t *testing.T) {
	cfg, path := loadFreshInstallConfig(t)

	err := applyInitialPassword(cfg, path, "secret", "too-short")
	if !errors.Is(err, errInitialPasswordMasterKey) {
		t.Fatalf("error = %v, want errInitialPasswordMasterKey", err)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.Directories.DataDir, "vault.bin")); !os.IsNotExist(statErr) {
		t.Fatalf("vault must not be written, stat err = %v", statErr)
	}
}
