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
