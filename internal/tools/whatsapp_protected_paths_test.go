package tools

import (
	"path/filepath"
	"slices"
	"testing"

	"aurago/internal/config"
)

func TestWhatsAppDeviceStoreIsProtectedSystemState(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(root, "data")
	db := config.WhatsAppDeviceDBPath(cfg.Directories.DataDir)
	for _, path := range []string{db, db + "-wal", db + "-shm", db + "-journal", filepath.Dir(db)} {
		if !IsProtectedSystemPath(path, root, cfg) {
			t.Errorf("%s is reachable by agent file tools", path)
		}
	}
	protected := protectedSystemFilesFromConfig(cfg)
	for _, path := range config.WhatsAppProtectedPaths(cfg) {
		if !slices.Contains(protected, path) {
			t.Errorf("the explicit protected list misses %s", path)
		}
	}
	if !outgoingPathIsProtected(db, outgoingProtectedFiles(cfg)) {
		t.Error("an outgoing attachment could send the WhatsApp device store")
	}
}

// A copy or hard link of the device store keeps the keys readable, so the
// whatsapp/device.db name is refused wherever it sits. The rule is narrow on
// purpose: other device.db files and other names in a whatsapp folder stay
// reachable.
func TestWhatsAppDeviceStoreNameIsProtectedOutsideTheDataDir(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(root, "data")
	elsewhere := filepath.Join(root, "workspace")
	for _, path := range []string{
		filepath.Join(elsewhere, "whatsapp", "device.db"),
		filepath.Join(elsewhere, "whatsapp", "device.db-wal"),
		filepath.Join(elsewhere, "whatsapp", "device.db-shm"),
		filepath.Join(elsewhere, "whatsapp", "device.db-journal"),
		filepath.Join(elsewhere, "deep", "WhatsApp", "Device.DB"),
		filepath.Join("whatsapp", "device.db"), // relative to the workspace
	} {
		if !IsProtectedSystemPath(path, elsewhere, cfg) {
			t.Errorf("%s is reachable by agent file tools", path)
		}
		if filepath.IsAbs(path) && !outgoingPathIsProtected(path, outgoingProtectedFiles(cfg)) {
			t.Errorf("an outgoing attachment could send %s", path)
		}
	}
	for _, path := range []string{
		filepath.Join(elsewhere, "device.db"),
		filepath.Join(elsewhere, "other", "device.db"),
		filepath.Join(elsewhere, "whatsapp", "notes.txt"),
		filepath.Join(elsewhere, "whatsapp", "device.db.bak"),
		filepath.Join(elsewhere, "whatsapp-export", "device.db"),
	} {
		if IsProtectedSystemPath(path, elsewhere, cfg) {
			t.Errorf("%s is refused although the rule is limited to whatsapp/device.db", path)
		}
	}
}
