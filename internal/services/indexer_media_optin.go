package services

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"aurago/internal/config"
	"aurago/internal/memory"
)

// mediaSkipLogged remembers the directories whose skipped media was already
// reported, so a folder without permission produces one Info line per process
// instead of one per poll.
var mediaSkipLogged sync.Map

// ReplaceConfigLocked points the indexer at a newly published config snapshot, so
// a later scan reads the current folder permissions, capability switches and
// extension list instead of the snapshot the indexer was created with. It also
// rebuilds the extension set, the only state derived from the config at
// construction. The caller must hold fi.cfgMu for writing (the server passes its
// CfgMu, which replaceConfigSnapshot and the directory handlers hold while they
// publish); the method does not lock again.
func (fi *FileIndexer) ReplaceConfigLocked(cfg *config.Config) {
	if fi == nil || cfg == nil {
		return
	}
	fi.cfg = cfg
	fi.extensions = buildIndexingExtensionSet(cfg.Indexing.Extensions)
}

// mediaAnalysisAllowedForFile reports whether the file may be sent to the Vision
// LLM or the multimodal embedding provider (images, audio, scanned PDFs). The
// per-folder opt-in (allow_media_analysis) of the deepest configured directory
// that contains the file decides, because a parent entry also scans its
// configured subfolders. A file outside every configured directory is denied.
func (fi *FileIndexer) mediaAnalysisAllowedForFile(path string) bool {
	allowed, _ := fi.mediaAnalysisDecisionForFile(path)
	return allowed
}

// mediaAnalysisDecisionForFile is mediaAnalysisAllowedForFile that also names the
// directory that decided (empty when no configured directory contains the file).
// When several entries resolve to the deepest path, the file is allowed only if
// every one of them allows it, so a duplicate entry can never widen the permission.
func (fi *FileIndexer) mediaAnalysisDecisionForFile(path string) (allowed bool, decidingDir string) {
	fileKey := indexingPathKey(path)
	if fileKey == "" {
		return false, ""
	}
	deepest := -1
	for _, configured := range fi.indexingDirectoriesSnapshot() {
		rootKey := indexingPathKey(configured.Path)
		if rootKey == "" || !indexingPathKeyWithin(fileKey, rootKey) {
			continue
		}
		switch {
		case len(rootKey) > deepest:
			deepest, allowed, decidingDir = len(rootKey), configured.AllowMediaAnalysis, configured.Path
		case len(rootKey) == deepest:
			// Both roots are prefixes of the same file path with the same length: the same path.
			allowed = allowed && configured.AllowMediaAnalysis
		}
	}
	if deepest < 0 {
		return false, ""
	}
	return allowed, decidingDir
}

// indexingPathKey is the comparable form of a path: absolute, cleaned and, on
// Windows, case-folded. An empty path has no key.
func indexingPathKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

// indexingPathKeyWithin reports whether key is root itself or lies below it. The
// boundary is a path separator, so "/data/media-private" is not within "/data/media".
func indexingPathKeyWithin(key, root string) bool {
	if key == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(key, prefix)
}

// acceptsExtension decides whether a file with this extension is indexed at all.
// Images and audio are media: they are only accepted for a file with the
// opt-in, and only while the matching global capability switch is on, no matter
// whether the extension is also listed in indexing.extensions. It takes fi.cfgMu
// itself for the extension set, so the caller must not hold it.
func (fi *FileIndexer) acceptsExtension(ext string, allowMedia, indexImages, multimodal bool) bool {
	switch {
	case IsImageFile(ext):
		return allowMedia && (indexImages || multimodal)
	case IsAudioFile(ext):
		return allowMedia && multimodal
	default:
		fi.cfgMu.RLock()
		known := fi.extensions[ext]
		fi.cfgMu.RUnlock()
		return known
	}
}

// fileIndexStateTracked reports whether the index remembers the file at all.
func fileIndexStateTracked(state memory.FileIndexState) bool {
	return !state.LastModified.IsZero() || state.ContentHash != "" || state.IndexFingerprint != ""
}

// skipUnpermittedMedia handles an image or audio file in a folder without the
// media opt-in. Nothing is sent anywhere; a file that an earlier run (before the
// opt-in existed, or before it was revoked) left in the index is removed
// locally. An untracked file costs one index-state lookup. wouldSend says that
// a provider capability is enabled and would have used the file, which is what
// the once-per-directory notice is about. dir is the directory that denied the
// permission (the deepest configured one containing the file), not the scan root.
func (fi *FileIndexer) skipUnpermittedMedia(dir, collection, path string, wouldSend bool) fileIndexOutcome {
	if wouldSend {
		if _, loggedBefore := mediaSkipLogged.LoadOrStore(indexingPathKey(dir), struct{}{}); !loggedBefore {
			fi.logger.Info("[Indexer] Skipping images/audio of a directory without media permission; set allow_media_analysis for it to send them to the Vision/multimodal provider",
				"dir", dir)
		}
	}
	state, err := fi.stm.GetFileIndexState(path, collection)
	if err != nil {
		return fileIndexOutcome{err: fmt.Errorf("get file index %s: %v", path, err)}
	}
	if fileIndexStateTracked(state) {
		if err := fi.removeTrackedFile(path, collection); err != nil {
			fi.logger.Warn("[Indexer] Failed to remove media of a directory without media permission", "path", path, "error", scrubIndexingError(err))
			return fileIndexOutcome{err: fmt.Errorf("cleanup unpermitted media %s: %v", path, err)}
		}
	}
	return fileIndexOutcome{noContent: true}
}
