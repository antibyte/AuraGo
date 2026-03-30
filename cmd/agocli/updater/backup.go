package updater

import (
	"fmt"
	"os"
	"path/filepath"
)

// protectedFiles are configuration files that must be backed up.
var protectedFiles = []string{
	".env",
	"config.yaml",
	"config_debug.yaml",
}

// protectedDirs are directories whose contents must be preserved.
var protectedDirs = []string{
	"agent_workspace/tools",
	"agent_workspace/skills",
}

// dataFiles are runtime data files that must be preserved.
var dataFiles = []string{
	"data/chat_history.json",
	"data/crontab.json",
	"data/state.json",
	"data/graph.json",
	"data/graph.json.migrated",
	"data/current_plan.md",
	"data/character_journal.md",
	"data/vault.bin",
	"data/aurago.db",
	"data/aurago_history.db",
	"data/aurago_contacts.db",
	"data/aurago_inventory.db",
	"data/aurago_webhooks.db",
	"data/aurago_media.db",
	"data/aurago_budget.db",
	"data/aurago_fritzbox.db",
	"data/knowledge.db",
	"firstpassword.txt",
}

// CreateBackup creates a temporary backup of all protected files and data.
// Returns the backup directory path.
func CreateBackup(installDir string) (string, error) {
	backupDir, err := os.MkdirTemp("", "aurago-backup-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Backup individual files
	allFiles := append(protectedFiles, dataFiles...)
	for _, rel := range allFiles {
		src := filepath.Join(installDir, rel)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(backupDir, rel)
		if err := copyFile(src, dst); err != nil {
			return backupDir, fmt.Errorf("backup %s: %w", rel, err)
		}
	}

	// Backup protected directories
	for _, rel := range protectedDirs {
		src := filepath.Join(installDir, rel)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(backupDir, rel)
		if err := copyDir(src, dst); err != nil {
			return backupDir, fmt.Errorf("backup dir %s: %w", rel, err)
		}
	}

	// Backup vectordb
	vdbSrc := filepath.Join(installDir, "data", "vectordb")
	if info, err := os.Stat(vdbSrc); err == nil && info.IsDir() {
		vdbDst := filepath.Join(backupDir, "data", "vectordb")
		copyDir(vdbSrc, vdbDst)
	}

	return backupDir, nil
}

// RestoreBackup restores backed up files to the install directory.
func RestoreBackup(backupDir, installDir string) error {
	return filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(backupDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(installDir, rel)

		if info.IsDir() {
			return os.MkdirAll(dst, 0750)
		}
		return copyFile(path, dst)
	})
}

// copyFile copies a single file, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, _ := os.Stat(src)
	perm := info.Mode().Perm()
	return os.WriteFile(dst, data, perm)
}

// copyDir copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0750)
		}
		return copyFile(path, target)
	})
}
