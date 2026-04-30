package services

import (
	"strings"
	"time"

	"aurago/internal/config"
)

const fileIndexerKGSyncConcurrency = 2

func buildIndexingExtensionSet(exts []string) map[string]bool {
	extMap := make(map[string]bool, len(exts))
	for _, ext := range exts {
		ext = strings.TrimSpace(strings.ToLower(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[ext] = true
	}
	return extMap
}

func (fi *FileIndexer) indexingDirectoriesSnapshot() []config.IndexingDirectory {
	fi.cfgMu.RLock()
	defer fi.cfgMu.RUnlock()
	dirs := make([]config.IndexingDirectory, len(fi.cfg.Indexing.Directories))
	copy(dirs, fi.cfg.Indexing.Directories)
	return dirs
}

func indexingDirectoryPaths(dirs []config.IndexingDirectory) []string {
	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		paths[i] = dir.Path
	}
	return paths
}

func (fi *FileIndexer) pollInterval() time.Duration {
	fi.cfgMu.RLock()
	seconds := fi.cfg.Indexing.PollIntervalSeconds
	fi.cfgMu.RUnlock()
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}
