package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenDeviceStoreCreatesPrivateDatabaseWithForeignKeys(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	s, err := openDeviceStore(ctx, dataDir, nil)
	if err != nil {
		t.Fatalf("openDeviceStore: %v", err)
	}
	var foreignKeys int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, %v; want 1", foreignKeys, err)
	}
	var tables int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'whatsmeow_device'").Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("whatsmeow_device tables = %d, %v; want 1", tables, err)
	}
	if s.Linked() || !s.OwnJID().IsEmpty() {
		t.Fatal("a fresh store reports a linked device")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	reopened, err := openDeviceStore(ctx, dataDir, nil)
	if err != nil {
		t.Fatalf("reopen migrated store: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened: %v", err)
	}
	dir := filepath.Join(dataDir, "whatsapp")
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("store dir: %v", err)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, "device.db"))
	if err != nil {
		t.Fatalf("store file: %v", err)
	}
	if runtime.GOOS != "windows" && (dirInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600) {
		t.Fatalf("modes dir=%v file=%v, want 0700/0600", dirInfo.Mode().Perm(), fileInfo.Mode().Perm())
	}
}

func TestOpenDeviceStoreRequiresDataDir(t *testing.T) {
	if _, err := openDeviceStore(context.Background(), " ", nil); err == nil {
		t.Fatal("an empty data dir opened a device store")
	}
}
