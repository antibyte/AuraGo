package tools

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"aurago/internal/config"
)

// Startup-only registrations survive configuration replacement and removal of
// bootstrap environment variables. Both link names and resolved targets stay private.
var processProtectedPaths struct {
	sync.RWMutex
	files []string
	dirs  []string
}

func RegisterProtectedSystemFile(path string) error {
	return registerProtectedProcessPath(path, false)
}

func registerProtectedProcessPath(path string, directory bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve protected system path: %w", err)
	}
	resolved, err := secureResolveFinalPath(abs)
	if err != nil {
		return fmt.Errorf("resolve protected system target: %w", err)
	}
	processProtectedPaths.Lock()
	defer processProtectedPaths.Unlock()
	target := &processProtectedPaths.files
	if directory {
		target = &processProtectedPaths.dirs
	}
	*target = unionRuntimePaths(*target, []string{filepath.Clean(abs), filepath.Clean(resolved)})
	return nil
}

func registeredProtectedFiles() []string {
	processProtectedPaths.RLock()
	defer processProtectedPaths.RUnlock()
	return append([]string(nil), processProtectedPaths.files...)
}

func requireUnprotectedProcessPath(path string, parents bool) error {
	processProtectedPaths.RLock()
	defer processProtectedPaths.RUnlock()
	for _, candidate := range protectedPathCandidates(path) {
		for _, file := range processProtectedPaths.files {
			if pathsEqualFold(candidate, file) || parents && pathHasProtectedPrefix(candidate, file) {
				return fmt.Errorf("path refers to protected AuraGo configuration, credential or supervisor state")
			}
		}
		for _, dir := range processProtectedPaths.dirs {
			if pathHasProtectedPrefix(dir, candidate) || parents && pathHasProtectedPrefix(candidate, dir) {
				return fmt.Errorf("path refers to protected AuraGo configuration, credential or supervisor state")
			}
		}
	}
	return nil
}

// protectedSystemFilesFromConfig lists the exact files agent file tools must
// never touch: the active config, the vault and its lock, and every configured
// SQLite database with its WAL/SHM sidecars.
func protectedSystemFilesFromConfig(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	vaultBase := filepath.Join(cfg.Directories.DataDir, "vault.bin")
	files := append([]string{cfg.ConfigPath, vaultBase, vaultBase + ".lock"}, config.SQLiteProtectedPaths(cfg)...)
	files = append(files, registeredProtectedFiles()...)
	out := make([]string, 0, len(files))
	for _, file := range files {
		if strings.TrimSpace(file) != "" {
			out = append(out, file)
		}
	}
	return out
}

// IsProtectedSystemPath reports whether rawPath names AuraGo configuration,
// credential or database state: the active config file, the vault and its lock,
// everything under directories.data_dir, configured SQLite files with WAL/SHM,
// aurago_master.key and any file named .env or ending in .env. rawPath may be
// relative to workspaceDir. Matching is case-insensitive and symlink-resolved.
func IsProtectedSystemPath(rawPath, workspaceDir string, cfg *config.Config) bool {
	if rawPath == "" || cfg == nil {
		return false
	}
	var abs string
	if filepath.IsAbs(rawPath) {
		abs = filepath.Clean(rawPath)
	} else {
		abs = filepath.Clean(filepath.Join(workspaceDir, rawPath))
	}
	return requireUnprotectedProcessPath(abs, false) != nil || isProtectedSystemPathAbs(abs, cfg.Directories.DataDir, protectedSystemFilesFromConfig(cfg))
}

// requireUnprotectedSystemPath applies IsProtectedSystemPath to a path that
// secureResolve already resolved, using the runtime snapshot's protected paths.
// The file-name checks apply even when no snapshot is configured.
func requireUnprotectedSystemPath(path, userPath string) error {
	if err := requireUnprotectedProcessPath(path, false); err != nil {
		return err
	}
	perms, _ := currentRuntimePermissions()
	if isProtectedSystemPathAbs(path, perms.ProtectedDataDir, perms.ProtectedSystemFiles) {
		return fmt.Errorf("path '%s' refers to protected AuraGo configuration, credential or database state", userPath)
	}
	return nil
}

func isProtectedSystemPathAbs(abs, dataDir string, protectedFiles []string) bool {
	candidates := protectedPathCandidates(abs)
	for _, cand := range candidates {
		base := strings.ToLower(filepath.Base(cand))
		if base == ".env" || strings.HasSuffix(base, ".env") {
			return true
		}
		if base == "aurago_master.key" || base == "vault.bin" || base == "vault.bin.lock" {
			return true
		}
	}
	if dataDir = strings.TrimSpace(dataDir); dataDir != "" {
		for _, dataRoot := range protectedPathCandidates(filepath.Clean(dataDir)) {
			for _, cand := range candidates {
				if pathHasProtectedPrefix(dataRoot, cand) {
					return true
				}
			}
		}
	}
	for _, p := range protectedFiles {
		if strings.TrimSpace(p) == "" {
			continue
		}
		for _, prot := range protectedPathCandidates(p) {
			for _, cand := range candidates {
				if pathsEqualFold(cand, prot) {
					return true
				}
			}
		}
	}
	return false
}

func protectedPathCandidates(path string) []string {
	path = filepath.Clean(path)
	out := []string{path}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		resolved = filepath.Clean(resolved)
		if !pathsEqualFold(resolved, path) {
			out = append(out, resolved)
		}
	}
	return out
}

func pathsEqualFold(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func pathHasProtectedPrefix(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err == nil {
		if rel == "." {
			return true
		}
		return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	rootSlash := strings.ToLower(filepath.ToSlash(filepath.Clean(root)))
	candSlash := strings.ToLower(filepath.ToSlash(filepath.Clean(candidate)))
	return candSlash == rootSlash || strings.HasPrefix(candSlash, rootSlash+"/")
}
