package services

import "aurago/internal/config"

// CleanupDirectory removes VectorDB, STM, and optional KG tracking for indexed
// files that belong to a directory which is no longer configured for indexing.
func (fi *FileIndexer) CleanupDirectory(indexingDir config.IndexingDirectory) []string {
	if fi == nil || fi.stm == nil || fi.vectorDB == nil {
		return nil
	}
	fi.scanMu.Lock()
	defer fi.scanMu.Unlock()

	collection := getDirCollection(indexingDir)
	trackedPaths, err := fi.stm.ListIndexedFiles(collection)
	if err != nil {
		return []string{err.Error()}
	}
	return fi.cleanupDeletedTrackedFiles(indexingDir.Path, collection, trackedPaths, map[string]struct{}{})
}
