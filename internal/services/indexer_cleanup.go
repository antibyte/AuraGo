package services

import (
	"context"

	"aurago/internal/config"
)

// CleanupDirectory removes VectorDB, STM, and optional KG tracking for indexed
// files that belong to a directory which is no longer configured for indexing.
func (fi *FileIndexer) CleanupDirectory(indexingDir config.IndexingDirectory) []string {
	if fi == nil || fi.stm == nil || fi.vectorDB == nil {
		return nil
	}
	if err := fi.acquireScanGate(context.Background()); err != nil {
		return []string{err.Error()}
	}
	defer fi.releaseScanGate()

	collection := getDirCollection(indexingDir)
	trackedPaths, err := fi.stm.ListIndexedFiles(collection)
	if err != nil {
		return []string{err.Error()}
	}
	return fi.cleanupDeletedTrackedFiles(indexingDir.Path, collection, trackedPaths, map[string]struct{}{})
}
