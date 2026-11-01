package whatsapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func deviceFilePaths(dataDir string) []string {
	db := config.WhatsAppDeviceDBPath(dataDir)
	return []string{db, db + "-wal", db + "-shm", db + "-journal"}
}

func TestDeviceStoreOverwritesDeletedRows(t *testing.T) {
	s, err := openDeviceStore(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var secureDelete int
	if err := s.db.QueryRow("PRAGMA secure_delete").Scan(&secureDelete); err != nil || secureDelete != 1 {
		t.Fatalf("secure_delete = %d, %v; want 1", secureDelete, err)
	}
}

// A local-only removal deletes the keys, then the database file and every
// sidecar, so no key bytes stay behind in free pages or a journal.
func TestRemoveLocalDeviceDeletesTheKeyFilesFromDisk(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	s, err := openDeviceStore(ctx, dataDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A linked device row with key material of the sizes the schema demands.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO whatsmeow_device
		(jid, registration_id, noise_key, identity_key, signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
		 adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig)
		VALUES ('491701234567:1@s.whatsapp.net', 1, zeroblob(32), zeroblob(32), zeroblob(32), 1, zeroblob(64),
		 zeroblob(32), zeroblob(8), zeroblob(64), zeroblob(32), zeroblob(64))`); err != nil {
		t.Fatalf("insert a linked device: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range deviceFilePaths(dataDir)[1:] { // sidecars a crashed run could leave
		if err := os.WriteFile(path, []byte("leftover"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if linked, err := DeviceLinked(ctx, dataDir); err != nil || !linked {
		t.Fatalf("device linked = %v, %v; want true", linked, err)
	}
	if err := RemoveLocalDevice(ctx, dataDir); err != nil {
		t.Fatalf("RemoveLocalDevice: %v", err)
	}
	for _, path := range deviceFilePaths(dataDir) {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the local removal: %v", filepath.Base(path), err)
		}
	}
	// The store opens again afterwards, unlinked.
	if linked, err := DeviceLinked(ctx, dataDir); err != nil || linked {
		t.Fatalf("after removal: linked = %v, %v; want false", linked, err)
	}
	// A second removal with nothing on disk is not an error.
	if err := RemoveLocalDevice(ctx, dataDir); err != nil {
		t.Fatalf("second removal: %v", err)
	}
}

// Missing files are fine; any other failure is reported, and the remaining
// files are still removed.
func TestRemoveLocalDeviceReportsFilesItCannotRemove(t *testing.T) {
	dataDir := t.TempDir()
	paths := deviceFilePaths(dataDir)
	if err := os.MkdirAll(filepath.Dir(paths[0]), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], []byte("keys"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A non-empty directory in a sidecar's place cannot be removed as a file.
	blocker := paths[3]
	if err := os.MkdirAll(filepath.Join(blocker, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := removeLocalDevice(context.Background(), &fakeBackend{store: &fakeStore{}}, dataDir)
	if err == nil {
		t.Fatal("an unremovable sidecar was not reported")
	}
	if _, statErr := os.Stat(paths[0]); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("device.db survived: %v", statErr)
	}
}

func TestBotLocalLogoutRemovesTheKeyFiles(t *testing.T) {
	ctx := context.Background()
	cfg := testBotConfig(config.WhatsAppModeSelfChat)
	cfg.Directories.DataDir = t.TempDir()
	paths := deviceFilePaths(cfg.Directories.DataDir)
	if err := os.MkdirAll(filepath.Dir(paths[0]), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("keys"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	client := &fakeClient{own: testOwn, loggedIn: true, logoutErr: errors.New("websocket not connected")}
	st := &fakeStore{linked: true, own: testOwn, client: client}
	b := startTestBot(t, cfg, st, &recordedSleeps{})
	waitFor(t, "connect", func() bool { return connectsOf(client) == 1 })
	if result, err := b.Logout(ctx); err != nil || result.RemoteRemoved {
		t.Fatalf("local fallback logout = %+v, %v", result, err)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the local logout: %v", filepath.Base(path), err)
		}
	}
}
