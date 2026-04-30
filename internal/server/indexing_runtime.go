package server

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/config"
	"aurago/internal/services"
)

func (s *Server) restartFileIndexer(cfg *config.Config) {
	if s.FileIndexer != nil {
		s.FileIndexer.Stop()
		s.FileIndexer = nil
	}
	if cfg == nil || !cfg.Indexing.Enabled {
		return
	}
	s.FileIndexer = services.NewFileIndexer(cfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)
	s.attachFileKGSyncer()
	s.FileIndexer.Start(context.Background())
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
