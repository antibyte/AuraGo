package server

import (
	"os"
	"path/filepath"

	"aurago/internal/config"
)

// whatsAppStoreSkipper returns the predicate that backup directory walks use to
// skip the WhatsApp linked-device store: <data_dir>/whatsapp and everything
// below it. The keys must not leave the machine in a backup, because a restored
// copy would clone the linked device; the owner relinks after a restore. A walk
// reaches the store whenever a configured directory (vectordb_dir) is data_dir
// or one of its ancestors. A directory that is the store under another path
// (a link or junction) is recognized by identity.
func whatsAppStoreSkipper(dataDir string) func(path string, info os.FileInfo) bool {
	store := config.WhatsAppStoreDir(dataDir)
	if store == "" {
		return func(string, os.FileInfo) bool { return false }
	}
	roots := []string{store}
	if resolved, err := filepath.EvalSymlinks(store); err == nil && resolved != store {
		roots = append(roots, resolved)
	}
	storeInfo, _ := os.Stat(store)
	return func(path string, info os.FileInfo) bool {
		for _, root := range roots {
			if pathStaysWithinDir(root, path) {
				return true
			}
		}
		return storeInfo != nil && info != nil && info.IsDir() && os.SameFile(info, storeInfo)
	}
}
