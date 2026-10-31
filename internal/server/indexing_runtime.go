package server

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/services"
)

// fileIndexer returns the current file indexer, or nil while indexing is off
// or being restarted. Callers must use the returned value, not call it again.
func (s *Server) fileIndexer() *services.FileIndexer {
	return s.fileIndexerRef.Load()
}

func (s *Server) setFileIndexer(fi *services.FileIndexer) {
	s.fileIndexerRef.Store(fi)
}

func (s *Server) restartFileIndexer(cfg *config.Config) {
	if current := s.fileIndexer(); current != nil {
		if !current.Stop() {
			s.Logger.Warn("[Indexer] Existing file indexer did not stop; aborting restart")
			return
		}
		s.setFileIndexer(nil)
	}
	if cfg == nil || !cfg.Indexing.Enabled {
		return
	}
	next := services.NewFileIndexer(cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)
	next.SetDecisionService(taskDecisionService{s})
	s.setFileIndexer(next)
	s.attachFileKGSyncer()
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	next.Start(budget.WithTracking(parent, s.budgetTrackerSnapshot(), "vision"))
}

func resolveIndexingRequestPath(configDir, rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func sameIndexingPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func isRootIndexingPath(path string) bool {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	root := volume + string(filepath.Separator)
	return clean == root || clean == string(filepath.Separator)
}
