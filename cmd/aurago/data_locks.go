package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/restorepending"
	"github.com/gofrs/flock"
)

type dataDirectoryLock struct {
	lock *flock.Flock
	info os.FileInfo
}

type dataDirectoryLocks map[string]dataDirectoryLock

func (locks dataDirectoryLocks) acquire(dirs ...string) error {
	sort.Strings(dirs)
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		dir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		dir, err = filepath.EvalSymlinks(dir)
		if err != nil {
			return err
		}
		key := dir
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, ok := locks[key]; ok {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		alreadyHeld := false
		for _, held := range locks {
			if os.SameFile(info, held.info) {
				alreadyHeld = true
				break
			}
		}
		if alreadyHeld {
			continue
		}
		lock := flock.New(filepath.Join(dir, "aurago-data.lock"))
		if ok, err := lock.TryLock(); err != nil || !ok {
			lock.Close()
			return fmt.Errorf("data directory %q is locked or unavailable: %v", dir, err)
		}
		locks[key] = dataDirectoryLock{lock: lock, info: info}
	}
	return nil
}

func (locks dataDirectoryLocks) close() {
	for key, lock := range locks {
		_ = lock.lock.Close()
		delete(locks, key)
	}
}

func configuredDataDirectories(cfg *config.Config) []string {
	var dirs []string
	for _, file := range config.SQLiteDatabasePaths(cfg) {
		dirs = append(dirs, filepath.Dir(file))
	}
	return dirs
}

// Called under the installation lock; only lock failures stop startup here.
func prepareStartupRestore(configFile string, apply bool, locks dataDirectoryLocks, log *slog.Logger) (string, error) {
	root, err := restorepending.ConfigRoot(configFile)
	if err != nil {
		return "", err
	}
	cfg, _ := config.LoadReadOnly(configFile)
	opts := restorepending.Options{
		Logger:   log,
		Validate: func() error { _, err := config.LoadReadOnly(configFile); return err },
		ResolveDirs: func() (restorepending.Dirs, error) {
			if cfg == nil {
				return restorepending.Dirs{}, nil
			}
			return restorepending.Dirs{VectorDB: cfg.Directories.VectorDBDir, Prompts: cfg.Directories.PromptsDir, Skills: cfg.Directories.SkillsDir, AgentSkills: cfg.Directories.AgentSkillsDir, Tools: cfg.Directories.ToolsDir, Workspace: cfg.Directories.WorkspaceDir}, nil
		},
	}
	dirs := append(configuredDataDirectories(cfg), filepath.Join(root, "data"))
	if apply {
		affected, resolveErr := restorepending.AffectedDataDirectories(root, opts)
		dirs = append(dirs, affected...)
		if resolveErr != nil {
			log.Warn("Pending restore validation failed; acquiring state locks before rejection", "error", resolveErr)
		}
	}
	if err := locks.acquire(dirs...); err != nil {
		return "", err
	}
	if !apply {
		return "", nil
	}
	if res, err := restorepending.Apply(root, opts); err != nil {
		status := ""
		if res != nil {
			status = res.Status
		}
		log.Error("System restore failed; continuing with the previous state", "status", status, "error", err)
	}
	// A restored configuration may point at additional databases. Lock those
	// directories while all pre-restore locks are still held, before opening any DB.
	restored, _ := config.LoadReadOnly(configFile)
	if err := locks.acquire(configuredDataDirectories(restored)...); err != nil {
		return root, err
	}
	return root, nil
}
