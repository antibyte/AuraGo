package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/chunking"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// IndexerCollection is the default collection name for file indexing.
const IndexerCollection = "file_index"

const fileIndexerFingerprint = "file-indexer-v3"

var ErrNoIndexableContent = stderrors.New("file contains no indexable content")

// Retry constants for indexing operations (aligned with KnowledgeGraph retry pattern).
const (
	indexingRetryMaxAttempts = 3
	indexingRetryBackoffBase = 250 * time.Millisecond
	fileIndexerStopWait      = 10 * time.Second
)

// IndexerStatus holds runtime statistics for the file indexer.
type IndexerStatus struct {
	Running          bool      `json:"running"`
	Directories      []string  `json:"directories"`
	TotalFiles       int       `json:"total_files"`
	IndexedFiles     int       `json:"indexed_files"`
	IndexedDocuments int       `json:"indexed_documents"`
	ChunkingStrategy string    `json:"chunking_strategy"`
	LastScanAt       time.Time `json:"last_scan_at,omitempty"`
	LastScanDuration string    `json:"last_scan_duration,omitempty"`
	Errors           []string  `json:"errors,omitempty"`
}

// FileIngestResult describes the persisted VectorDB output for one indexed file.
type FileIngestResult struct {
	DocumentIDs []string
	ChunkCount  int
	Indexed     bool
}

// FileIndexer watches configured directories and indexes files into VectorDB.
type FileIndexer struct {
	cfg               *config.Config
	cfgMu             *sync.RWMutex
	vectorDB          memory.VectorDB
	stm               *memory.SQLiteMemory
	logger            *slog.Logger
	cancel            context.CancelFunc
	runCtx            context.Context
	runToken          *struct{}
	lifecycleMu       sync.Mutex
	runWg             sync.WaitGroup
	scanGate          chan struct{}
	mu                sync.RWMutex
	status            IndexerStatus
	extensions        map[string]bool
	embedderMu        sync.Mutex
	cachedEmbedder    *memory.MultimodalEmbedder
	cachedEmbedderKey string
	kgSyncer          *FileKGSyncer
	kgSyncSem         chan struct{}
}

// NewFileIndexer creates a new file indexer service.
func NewFileIndexer(cfg *config.Config, cfgMu *sync.RWMutex, vectorDB memory.VectorDB, stm *memory.SQLiteMemory, logger *slog.Logger) *FileIndexer {
	return &FileIndexer{
		cfg:        cfg,
		cfgMu:      cfgMu,
		vectorDB:   vectorDB,
		stm:        stm,
		logger:     logger,
		extensions: buildIndexingExtensionSet(cfg.Indexing.Extensions),
		scanGate:   make(chan struct{}, 1),
		kgSyncSem:  make(chan struct{}, fileIndexerKGSyncConcurrency),
	}
}

// Start begins the indexing loop. It performs an initial scan and then polls
// at the configured interval.
func (fi *FileIndexer) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	fi.lifecycleMu.Lock()
	if fi.cancel != nil {
		fi.lifecycleMu.Unlock()
		fi.logger.Info("[Indexer] Start requested while file indexer is already running")
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	runToken := &struct{}{}
	fi.runCtx = runCtx
	fi.cancel = cancel
	fi.runToken = runToken
	fi.lifecycleMu.Unlock()

	fi.logger.Info("[Indexer] Starting file indexer",
		"directories", fi.cfg.Indexing.Directories,
		"poll_interval", fi.cfg.Indexing.PollIntervalSeconds,
		"extensions", fi.cfg.Indexing.Extensions)

	dirs := fi.indexingDirectoriesSnapshot()
	fi.mu.Lock()
	fi.status.Running = true
	fi.status.Directories = indexingDirectoryPaths(dirs)
	fi.mu.Unlock()

	// Ensure all configured directories exist
	fi.ensureDirectories()

	fi.runWg.Add(1)
	go func() {
		defer fi.runWg.Done()
		defer func() {
			fi.lifecycleMu.Lock()
			if fi.runToken == runToken {
				fi.cancel = nil
				fi.runCtx = nil
				fi.runToken = nil
			}
			fi.lifecycleMu.Unlock()
		}()
		fi.scan(runCtx)

		ticker := time.NewTicker(fi.pollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				fi.mu.Lock()
				fi.status.Running = false
				fi.mu.Unlock()
				fi.logger.Info("[Indexer] File indexer stopped")
				return
			case <-ticker.C:
				fi.scan(runCtx)
			}
		}
	}()
}

// Stop gracefully stops the indexer.
func (fi *FileIndexer) Stop() bool {
	fi.lifecycleMu.Lock()
	cancel := fi.cancel
	fi.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	} else {
		return true
	}

	done := make(chan struct{})
	go func() {
		fi.runWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(fileIndexerStopWait):
		fi.logger.Warn("[Indexer] Timed out waiting for file indexer to stop", "timeout", fileIndexerStopWait)
		return false
	}
}

// SetKGSyncer attaches an optional FileKGSyncer so that file deletions can
// trigger cleanup of the corresponding knowledge-graph entities.
func (fi *FileIndexer) SetKGSyncer(syncer *FileKGSyncer) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.kgSyncer = syncer
}

// Status returns the current indexer status.
func (fi *FileIndexer) Status() IndexerStatus {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	status := fi.status
	status.ChunkingStrategy = fi.chunkingOptionsSnapshot().Strategy
	return status
}

// Rescan triggers an immediate full rescan of all directories.
func (fi *FileIndexer) Rescan() {
	ctx := context.Background()
	fi.lifecycleMu.Lock()
	if fi.runCtx != nil {
		ctx = fi.runCtx
	}
	fi.lifecycleMu.Unlock()
	go fi.scan(ctx)
}

// IndexFile indexes one file through the same extraction, chunking, embedding,
// fingerprinting, and persistence path used by background directory scans.
func (fi *FileIndexer) IndexFile(ctx context.Context, directory config.IndexingDirectory, path string, metadata map[string]string) (FileIngestResult, error) {
	if fi == nil || fi.stm == nil || fi.vectorDB == nil {
		return FileIngestResult{}, fmt.Errorf("file indexer is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return FileIngestResult{}, err
	}
	root, err := filepath.Abs(strings.TrimSpace(directory.Path))
	if err != nil || root == "" {
		return FileIngestResult{}, fmt.Errorf("resolve indexing directory: %w", err)
	}
	target, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || target == "" {
		return FileIngestResult{}, fmt.Errorf("resolve indexed file: %w", err)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return FileIngestResult{}, fmt.Errorf("indexed file must stay within configured directory")
	}
	info, err := os.Lstat(target)
	if err != nil {
		return FileIngestResult{}, fmt.Errorf("stat indexed file: %w", err)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return FileIngestResult{}, fmt.Errorf("indexed path must be a regular file")
	}
	collection := getDirCollection(directory)
	if err := fi.stm.UpsertFileIndexMetadata(target, collection, metadata); err != nil {
		return FileIngestResult{}, err
	}

	if err := fi.acquireScanGate(ctx); err != nil {
		return FileIngestResult{}, err
	}
	defer fi.releaseScanGate()
	info, err = os.Lstat(target)
	if err != nil {
		return FileIngestResult{}, fmt.Errorf("restat indexed file: %w", err)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return FileIngestResult{}, fmt.Errorf("indexed path must remain a regular file")
	}

	outcome := fi.indexFileCore(ctx, root, collection, target, info)
	if outcome.err != nil {
		return FileIngestResult{}, fmt.Errorf("index file: %w", outcome.err)
	}
	if !outcome.eligible || outcome.noContent {
		return FileIngestResult{}, ErrNoIndexableContent
	}
	documentIDs, err := fi.stm.GetFileEmbeddingDocIDs(target, collection)
	if err != nil {
		return FileIngestResult{}, fmt.Errorf("load indexed document IDs: %w", err)
	}
	if len(documentIDs) == 0 {
		return FileIngestResult{}, ErrNoIndexableContent
	}
	return FileIngestResult{
		DocumentIDs: documentIDs,
		ChunkCount:  len(documentIDs),
		Indexed:     len(documentIDs) > 0,
	}, nil
}

// ensureDirectories creates any missing configured directories.
func (fi *FileIndexer) ensureDirectories() {
	for _, dir := range fi.indexingDirectoriesSnapshot() {
		if err := os.MkdirAll(dir.Path, 0755); err != nil {
			fi.logger.Warn("[Indexer] Failed to create directory", "dir", dir.Path, "error", err)
		}
	}
}

// scan walks all configured directories and indexes new/changed files.
func (fi *FileIndexer) scan(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return
	}
	if !fi.tryAcquireScanGate() {
		fi.logger.Info("[Indexer] Scan already running, skipping overlapping request")
		return
	}
	defer fi.releaseScanGate()

	start := time.Now()
	fi.logger.Debug("[Indexer] Starting scan...")

	dirs := fi.indexingDirectoriesSnapshot()
	chunkingOptions := fi.chunkingOptionsSnapshot()

	fi.mu.Lock()
	fi.status.IndexedDocuments = 0
	fi.status.ChunkingStrategy = chunkingOptions.Strategy
	fi.mu.Unlock()

	var totalFiles, indexedFiles int
	var scanErrors []string
	var dirPaths []string

	if fi.vectorDB.IsDisabled() {
		for _, indexingDir := range dirs {
			if err := ctx.Err(); err != nil {
				break
			}
			dirPaths = append(dirPaths, indexingDir.Path)
			nTotal, errs := fi.countIndexableFiles(ctx, indexingDir.Path)
			totalFiles += nTotal
			scanErrors = append(scanErrors, errs...)
		}
		scanErrors = append(scanErrors, "Embedding pipeline unavailable; file indexing is disabled until an embedding provider is configured.")
		duration := time.Since(start)

		fi.mu.Lock()
		fi.status.TotalFiles = totalFiles
		fi.status.IndexedFiles = 0
		fi.status.IndexedDocuments = 0
		fi.status.ChunkingStrategy = chunkingOptions.Strategy
		fi.status.LastScanAt = start
		fi.status.LastScanDuration = duration.Round(time.Millisecond).String()
		fi.status.Errors = scanErrors
		fi.status.Directories = dirPaths
		fi.mu.Unlock()

		fi.logger.Debug("[Indexer] VectorDB is disabled, scan status updated", "total_files", totalFiles, "duration", duration.Round(time.Millisecond))
		return
	}

	for _, indexingDir := range dirs {
		if err := ctx.Err(); err != nil {
			break
		}
		dirPaths = append(dirPaths, indexingDir.Path)
		collection := getDirCollection(indexingDir)
		nTotal, nIndexed, errs := fi.scanDirectory(ctx, indexingDir.Path, collection)
		totalFiles += nTotal
		indexedFiles += nIndexed
		scanErrors = append(scanErrors, errs...)
	}

	duration := time.Since(start)

	fi.mu.Lock()
	fi.status.TotalFiles = totalFiles
	fi.status.IndexedFiles = indexedFiles
	fi.status.ChunkingStrategy = chunkingOptions.Strategy
	fi.status.LastScanAt = start
	fi.status.LastScanDuration = duration.Round(time.Millisecond).String()
	fi.status.Errors = scanErrors
	fi.status.Directories = dirPaths
	fi.mu.Unlock()

	if indexedFiles > 0 {
		fi.logger.Info("[Indexer] Scan completed",
			"total_files", totalFiles,
			"indexed", indexedFiles,
			"duration", duration.Round(time.Millisecond))
	} else {
		fi.logger.Debug("[Indexer] Scan completed, no changes",
			"total_files", totalFiles,
			"duration", duration.Round(time.Millisecond))
	}
}

func (fi *FileIndexer) countIndexableFiles(ctx context.Context, dir string) (int, []string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	var total int
	var errors []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("walk error %s: %v", path, err))
			return nil
		}
		if isSymlink(info) {
			fi.logger.Debug("[Indexer] Skipping symlink", "path", path)
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		isImage := IsImageFile(ext)
		isAudio := IsAudioFile(ext)

		fi.cfgMu.RLock()
		indexImages := fi.cfg.Indexing.IndexImages
		multimodal := fi.cfg.Embeddings.Multimodal
		fi.cfgMu.RUnlock()

		if fi.extensions[ext] || (isImage && (indexImages || multimodal)) || (isAudio && multimodal) {
			if skip, reason := shouldSkipIndexingFile(info); skip {
				errors = append(errors, fmt.Sprintf("skip %s: %s", path, reason))
				return nil
			}
			total++
		}
		return nil
	})
	if err != nil && !stderrors.Is(err, context.Canceled) {
		errors = append(errors, fmt.Sprintf("walk error %s: %v", dir, err))
	}
	return total, errors
}

// getDirCollection returns the collection name for an indexing directory.
// Returns the configured collection, or the default "file_index" if none is set.
func getDirCollection(dir config.IndexingDirectory) string {
	if dir.Collection != "" {
		return dir.Collection
	}
	return IndexerCollection
}

// scanDirectory walks a single directory (recursively) and indexes supported files.
func (fi *FileIndexer) scanDirectory(ctx context.Context, dir, collection string) (totalFiles, indexedFiles int, errors []string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, 0, nil
	}
	trackedPaths, trackedErr := fi.stm.ListIndexedFiles(collection)
	if trackedErr != nil {
		errors = append(errors, fmt.Sprintf("list indexed files %s: %v", dir, trackedErr))
	}
	seenPaths := make(map[string]struct{})

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fi.logger.Debug("[Indexer] Directory does not exist, skipping", "dir", dir)
		errors = append(errors, fi.cleanupDeletedTrackedFiles(dir, collection, trackedPaths, seenPaths)...)
		return 0, 0, errors
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("walk error %s: %v", path, err))
			return nil // continue walking
		}
		if isSymlink(info) {
			fi.logger.Debug("[Indexer] Skipping symlink", "path", path)
			return nil
		}

		// Skip directories and hidden files/folders
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		outcome := fi.indexFileCore(ctx, dir, collection, path, info)
		if outcome.eligible {
			totalFiles++
			seenPaths[path] = struct{}{}
		}
		if outcome.err != nil {
			errors = append(errors, outcome.err.Error())
			return nil
		}
		if outcome.indexed {
			indexedFiles++
		}
		return nil
	})
	if err != nil {
		if stderrors.Is(err, context.Canceled) {
			return totalFiles, indexedFiles, errors
		}
		errors = append(errors, fmt.Sprintf("walk error %s: %v", dir, err))
	}

	errors = append(errors, fi.cleanupDeletedTrackedFiles(dir, collection, trackedPaths, seenPaths)...)
	return totalFiles, indexedFiles, errors
}

type fileIndexOutcome struct {
	eligible  bool
	indexed   bool
	noContent bool
	err       error
}

func (fi *FileIndexer) indexFileCore(ctx context.Context, dir, collection, path string, info os.FileInfo) fileIndexOutcome {
	ext := strings.ToLower(filepath.Ext(info.Name()))
	isImage := IsImageFile(ext)
	isAudio := IsAudioFile(ext)

	fi.cfgMu.RLock()
	indexImages := fi.cfg.Indexing.IndexImages
	multimodal := fi.cfg.Embeddings.Multimodal
	fi.cfgMu.RUnlock()

	accepted := fi.extensions[ext] ||
		(isImage && (indexImages || multimodal)) ||
		(isAudio && multimodal)
	if !accepted {
		return fileIndexOutcome{noContent: true}
	}
	if skip, reason := shouldSkipIndexingFile(info); skip {
		fi.logger.Warn("[Indexer] Skipping oversized file", "path", path, "reason", reason)
		return fileIndexOutcome{err: fmt.Errorf("skip %s: %s", path, reason)}
	}
	if IsBinaryFile(path) {
		fi.logger.Debug("[Indexer] Skipping binary file", "path", path)
		return fileIndexOutcome{noContent: true}
	}

	outcome := fileIndexOutcome{eligible: true}
	indexMode := fi.indexModeForFile(ext, isImage, isAudio, multimodal)
	rawHash, hashErr := hashIndexedFileBytes(path)
	if hashErr != nil {
		outcome.err = fmt.Errorf("hash error %s: %v", path, hashErr)
		return outcome
	}
	indexFingerprint := fi.indexFingerprintForMode(indexMode)
	indexState, stateErr := fi.stm.GetFileIndexState(path, collection)
	if stateErr != nil {
		outcome.err = fmt.Errorf("get file index %s: %v", path, stateErr)
		return outcome
	}
	if !shouldReindexFile(rawHash, indexFingerprint, indexState) {
		return outcome
	}

	relPath, _ := filepath.Rel(dir, path)
	if relPath == "" {
		relPath = info.Name()
	}
	fileMetadata, metadataErr := fi.stm.GetFileIndexMetadata(path, collection)
	if metadataErr != nil {
		outcome.err = fmt.Errorf("metadata error %s: %v", path, metadataErr)
		return outcome
	}

	var content string
	var precomputedEmbedding []float32
	if multimodal && (isImage || isAudio) {
		embedder := fi.getMultimodalEmbedder()
		if embedder == nil {
			outcome.err = fmt.Errorf("multimodal embedder unavailable for %s", path)
			return outcome
		}
		vec, embedErr := fi.indexEmbedWithRetry(ctx, func(ctx context.Context) ([]float32, error) {
			return embedder.EmbedFile(ctx, path)
		}, path, "multimodal")
		if embedErr != nil {
			fi.logger.Warn("[Indexer] Multimodal embedding failed", "path", path, "error", scrubIndexingError(embedErr))
			outcome.err = fmt.Errorf("multimodal embed error %s: %v", path, embedErr)
			return outcome
		}
		precomputedEmbedding = vec
		kind := "Bild"
		if isAudio {
			kind = "Audio"
		}
		content = fmt.Sprintf("%s-Datei: %s (Pfad: %s)", kind, info.Name(), relPath)
	} else if isImage {
		prompt := fmt.Sprintf(
			"Analysiere dieses Bild detailliert. Beschreibe den Inhalt, erkennbare Texte, Objekte und relevante Details. "+
				"Dateiname: %s, Pfad: %s", info.Name(), relPath,
		)
		var analysis string
		var visionErr error
		for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
			var width, height int
			analysis, width, height, visionErr = tools.AnalyzeImageWithPromptContext(ctx, path, prompt, fi.cfg)
			_ = width
			_ = height
			if visionErr == nil {
				if attempt > 1 {
					fi.logger.Info("[Indexer] Vision retry successful", "path", path, "attempt", attempt)
				}
				break
			}
			if attempt == indexingRetryMaxAttempts || !shouldRetryIndexingErr(visionErr) {
				break
			}
			backoff := time.Duration(attempt*attempt) * indexingRetryBackoffBase
			fi.logger.Warn("[Indexer] Vision analysis failed, retrying", "path", path, "attempt", attempt, "backoff", backoff, "error", scrubIndexingError(visionErr))
			if sleepErr := waitIndexingBackoff(ctx, backoff); sleepErr != nil {
				visionErr = sleepErr
				break
			}
		}
		if visionErr != nil {
			fi.logger.Warn("[Indexer] Vision analysis failed", "path", path, "error", scrubIndexingError(visionErr))
			outcome.err = fmt.Errorf("vision error %s: %v", path, visionErr)
			return outcome
		}
		content = fmt.Sprintf("Bildanalyse von %s (Pfad: %s):\n%s", info.Name(), relPath, analysis)
	} else if IsDocumentFile(ext) {
		extracted, extractErr := ExtractText(path)
		if extractErr != nil {
			if multimodal && ext == ".pdf" {
				fallbackContent, vec, fallbackErr := fi.indexPDFWithMultimodalFallback(ctx, path, relPath, info.Name())
				if fallbackErr == nil {
					content = fallbackContent
					precomputedEmbedding = vec
					fi.logger.Info("[Indexer] PDF auto-fallback to multimodal embedding", "path", relPath)
				} else {
					fi.logger.Warn("[Indexer] PDF multimodal fallback failed", "path", path, "error", scrubIndexingError(fallbackErr))
				}
			}
			if precomputedEmbedding == nil {
				fi.logger.Warn("[Indexer] Text extraction failed, skipping document", "path", path, "error", scrubIndexingError(extractErr))
				outcome.err = fmt.Errorf("text extraction error %s: %v", path, extractErr)
				return outcome
			}
		} else {
			content = extracted
		}
		if multimodal && ext == ".pdf" && precomputedEmbedding == nil && len(strings.TrimSpace(content)) < 10 {
			fallbackContent, vec, fallbackErr := fi.indexPDFWithMultimodalFallback(ctx, path, relPath, info.Name())
			if fallbackErr == nil {
				content = fallbackContent
				precomputedEmbedding = vec
				fi.logger.Info("[Indexer] PDF auto-fallback to multimodal embedding", "path", relPath)
			} else {
				fi.logger.Warn("[Indexer] PDF multimodal fallback failed", "path", path, "error", scrubIndexingError(fallbackErr))
			}
		}
	} else {
		text, readErr := readIndexedTextFile(path)
		if readErr != nil {
			outcome.err = fmt.Errorf("read error %s: %v", path, readErr)
			return outcome
		}
		content = text
	}

	if strings.TrimSpace(content) == "" && precomputedEmbedding == nil {
		if err := fi.removeTrackedFile(path, collection); err != nil {
			fi.logger.Warn("[Indexer] Failed to remove stale embeddings for empty file", "path", path, "error", scrubIndexingError(err))
			outcome.err = fmt.Errorf("cleanup empty file %s: %v", path, err)
			return outcome
		}
		outcome.noContent = true
		return outcome
	}
	if err := fi.removeTrackedFile(path, collection); err != nil {
		fi.logger.Warn("[Indexer] Failed to remove stale embeddings before reindex", "path", path, "error", scrubIndexingError(err))
		outcome.err = fmt.Errorf("cleanup error %s: %v", path, err)
		return outcome
	}

	concept := fmt.Sprintf("Datei: %s (Pfad: %s, Geändert: %s)",
		info.Name(), relPath, info.ModTime().Format("2006-01-02 15:04"))
	if title := strings.TrimSpace(fileMetadata["archive_title"]); title != "" {
		concept = fmt.Sprintf("Dokument: %s (Datei: %s, Pfad: %s, Geändert: %s)",
			title, info.Name(), relPath, info.ModTime().Format("2006-01-02 15:04"))
	}
	if tags := strings.TrimSpace(fileMetadata["archive_tags"]); tags != "" {
		concept += " Tags: " + tags
	}

	var docIDs []string
	var storeErr error
	if precomputedEmbedding != nil {
		var docID string
		docID, storeErr = fi.indexStoreDocWithRetry(ctx, func() (string, error) {
			return fi.vectorDB.StoreDocumentWithEmbeddingInCollection(concept, content, precomputedEmbedding, collection)
		}, path)
		if storeErr == nil && docID != "" {
			docIDs = []string{docID}
		}
	} else if store, ok := fi.vectorDB.(chunkingCollectionStore); ok {
		chunkingOptions := fi.chunkingOptionsSnapshot()
		extraMetadata := map[string]string{
			"source_path":    path,
			"relative_path":  relPath,
			"source_type":    "file_indexer",
			"file_extension": ext,
			"index_mode":     indexMode,
		}
		for key, value := range fileMetadata {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
				extraMetadata[key] = value
			}
		}
		docIDs, storeErr = fi.indexStoreWithRetry(ctx, func() ([]string, error) {
			return store.StoreDocumentInCollectionWithChunking(concept, content, collection, chunkingOptions, extraMetadata)
		}, path)
	} else {
		docIDs, storeErr = fi.indexStoreWithRetry(ctx, func() ([]string, error) {
			return fi.vectorDB.StoreDocumentInCollection(concept, content, collection)
		}, path)
	}
	if storeErr != nil {
		fi.logger.Warn("[Indexer] Failed to index file", "path", path, "error", scrubIndexingError(storeErr))
		outcome.err = fmt.Errorf("index error %s: %v", path, storeErr)
		return outcome
	}
	if err := fi.stm.UpdateFileIndexWithDocsAndState(path, collection, info.ModTime(), rawHash, indexFingerprint, docIDs); err != nil {
		fi.logger.Warn("[Indexer] Failed to persist file index tracking", "path", path, "error", scrubIndexingError(err))
		if rollbackErr := fi.rollbackUntrackedDocuments(docIDs, collection); rollbackErr != nil {
			fi.logger.Warn("[Indexer] Failed to roll back untracked embeddings", "path", path, "error", scrubIndexingError(rollbackErr))
			outcome.err = fmt.Errorf("tracking error %s: %v; tracking rollback error %s: %v", path, err, path, rollbackErr)
			return outcome
		}
		outcome.err = fmt.Errorf("tracking error %s: %v", path, err)
		return outcome
	}
	if len(docIDs) == 0 {
		outcome.noContent = true
		return outcome
	}

	outcome.indexed = true
	fi.recordIndexedDocuments(len(docIDs))
	fi.logger.Info("[Indexer] Indexed file", "path", relPath, "size", info.Size(), "doc_ids", len(docIDs))
	fi.syncIndexedFileToKG(path, collection)
	return outcome
}

func scrubIndexingError(err error) string {
	if err == nil {
		return ""
	}
	return security.RedactSensitiveInfo(security.Scrub(err.Error()))
}

func (fi *FileIndexer) acquireScanGate(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case fi.scanGate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (fi *FileIndexer) tryAcquireScanGate() bool {
	select {
	case fi.scanGate <- struct{}{}:
		return true
	default:
		return false
	}
}

func (fi *FileIndexer) releaseScanGate() {
	<-fi.scanGate
}

func (fi *FileIndexer) rollbackUntrackedDocuments(docIDs []string, collection string) error {
	if len(docIDs) == 0 {
		return nil
	}
	if batcher, ok := fi.vectorDB.(interface {
		DeleteDocumentsFromCollection([]string, string) error
	}); ok {
		if err := batcher.DeleteDocumentsFromCollection(docIDs, collection); err != nil {
			return fmt.Errorf("batch delete vector docs from collection %s: %w", collection, err)
		}
	} else {
		for _, docID := range docIDs {
			if err := fi.vectorDB.DeleteDocumentFromCollection(docID, collection); err != nil {
				return fmt.Errorf("delete vector doc %s from collection %s: %w", docID, collection, err)
			}
		}
	}
	if err := fi.stm.DeleteMemoryMetaBatch(docIDs); err != nil {
		return fmt.Errorf("delete memory meta batch: %w", err)
	}
	return nil
}

type embeddingFingerprinter interface {
	EmbeddingFingerprint() string
}

type chunkingCollectionStore interface {
	StoreDocumentInCollectionWithChunking(concept, content, collection string, options chunking.Options, extraMetadata map[string]string) ([]string, error)
}

const (
	fileIndexModeText       = "text"
	fileIndexModeDocument   = "document"
	fileIndexModeVision     = "vision"
	fileIndexModeMultimodal = "multimodal"
	fileIndexModePDF        = "pdf"
)

func (fi *FileIndexer) indexModeForFile(ext string, isImage, isAudio, multimodal bool) string {
	switch {
	case multimodal && (isImage || isAudio):
		return fileIndexModeMultimodal
	case isImage:
		return fileIndexModeVision
	case ext == ".pdf":
		return fileIndexModePDF
	case IsDocumentFile(ext):
		return fileIndexModeDocument
	default:
		return fileIndexModeText
	}
}

func (fi *FileIndexer) indexFingerprintForMode(mode string) string {
	parts := []string{fileIndexerFingerprint, "mode=" + mode}
	if fp, ok := fi.vectorDB.(embeddingFingerprinter); ok {
		if embeddingFingerprint := strings.TrimSpace(fp.EmbeddingFingerprint()); embeddingFingerprint != "" {
			parts = append(parts, "embedding="+embeddingFingerprint)
		}
	}
	parts = append(parts, "chunking="+chunking.Fingerprint(fi.chunkingOptionsSnapshot()))

	fi.cfgMu.RLock()
	defer fi.cfgMu.RUnlock()
	switch mode {
	case fileIndexModeVision:
		parts = append(parts,
			fingerprintProvider("vision", fi.cfg.Vision.Provider, fi.cfg.Vision.ProviderType, fi.cfg.Vision.BaseURL, fi.cfg.Vision.Model, fi.cfg.Vision.APIKey),
			fingerprintProvider("llm", fi.cfg.LLM.Provider, fi.cfg.LLM.ProviderType, fi.cfg.LLM.BaseURL, fi.cfg.LLM.Model, fi.cfg.LLM.APIKey),
		)
	case fileIndexModeMultimodal, fileIndexModePDF:
		parts = append(parts,
			fingerprintProvider("embeddings", fi.cfg.Embeddings.Provider, fi.cfg.Embeddings.ProviderType, fi.cfg.Embeddings.BaseURL, fi.cfg.Embeddings.Model, fi.cfg.Embeddings.APIKey),
			"multimodal_format="+strings.TrimSpace(fi.cfg.Embeddings.MultimodalFormat),
		)
	}
	return strings.Join(parts, "|")
}

func (fi *FileIndexer) chunkingOptionsSnapshot() chunking.Options {
	if fi == nil || fi.cfg == nil {
		return chunking.NormalizeOptions(chunking.Options{})
	}
	if fi.cfgMu != nil {
		fi.cfgMu.RLock()
		defer fi.cfgMu.RUnlock()
	}
	cfg := fi.cfg.Indexing.Chunking
	return chunking.NormalizeOptionsWithDefaults(chunking.Options{
		Strategy:     cfg.Strategy,
		MaxChars:     cfg.MaxChars,
		OverlapChars: cfg.OverlapChars,
		MaxChunks:    cfg.MaxChunksPerFile,
	})
}

func (fi *FileIndexer) recordIndexedDocuments(count int) {
	if count <= 0 {
		return
	}
	fi.mu.Lock()
	fi.status.IndexedDocuments += count
	fi.mu.Unlock()
}

func fingerprintProvider(name, provider, providerType, baseURL, model, apiKey string) string {
	return strings.Join([]string{
		name,
		"provider=" + strings.TrimSpace(provider),
		"type=" + strings.TrimSpace(providerType),
		"base=" + strings.TrimSpace(baseURL),
		"model=" + strings.TrimSpace(model),
		"key_sha256=" + hashSecretForFingerprint(apiKey),
	}, ";")
}

func hashSecretForFingerprint(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func shouldReindexFile(contentHash, indexFingerprint string, state memory.FileIndexState) bool {
	if state.LastModified.IsZero() {
		return true
	}
	if state.ContentHash == "" || state.IndexFingerprint == "" {
		return true
	}
	if state.ContentHash != contentHash {
		return true
	}
	if state.IndexFingerprint != indexFingerprint {
		return true
	}
	return false
}

func hashIndexedFileBytes(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (fi *FileIndexer) syncIndexedFileToKG(path, collection string) {
	fi.mu.RLock()
	syncer := fi.kgSyncer
	fi.mu.RUnlock()
	if syncer == nil {
		return
	}

	fi.cfgMu.RLock()
	kgEnabled := fi.cfg.Tools.KnowledgeGraph.Enabled
	autoExtraction := fi.cfg.Tools.KnowledgeGraph.AutoExtraction
	fi.cfgMu.RUnlock()
	if !kgEnabled || !autoExtraction {
		return
	}
	if fi.kgSyncSem != nil {
		select {
		case fi.kgSyncSem <- struct{}{}:
		default:
			fi.logger.Warn("[Indexer] KG sync concurrency limit reached, skipping live sync", "path", path)
			return
		}
	}

	go func() {
		if fi.kgSyncSem != nil {
			defer func() { <-fi.kgSyncSem }()
		}
		res := syncer.runSyncFile(path, collection, FileKGSyncOptions{})
		if len(res.Errors) > 0 {
			fi.logger.Warn("[Indexer] KG sync produced errors", "path", path, "errors", res.Errors)
		}
	}()
}

func (fi *FileIndexer) cleanupDeletedTrackedFiles(dir, collection string, trackedPaths []string, seenPaths map[string]struct{}) []string {
	var errors []string
	for _, trackedPath := range trackedPaths {
		if !isPathWithinDir(trackedPath, dir) {
			continue
		}
		if _, ok := seenPaths[trackedPath]; ok {
			continue
		}
		if err := fi.removeTrackedFile(trackedPath, collection); err != nil {
			errors = append(errors, fmt.Sprintf("deleted file cleanup %s: %v", trackedPath, err))
			fi.logger.Warn("[Indexer] Failed to remove deleted file embeddings", "path", trackedPath, "error", err)
			continue
		}
		if err := fi.stm.DeleteFileIndexMetadata(trackedPath, collection); err != nil {
			errors = append(errors, fmt.Sprintf("deleted file metadata cleanup %s: %v", trackedPath, err))
			fi.logger.Warn("[Indexer] Failed to remove deleted file metadata", "path", trackedPath, "error", err)
		}
		fi.logger.Info("[Indexer] Removed embeddings for deleted file", "path", trackedPath)
	}
	return errors
}

func (fi *FileIndexer) removeTrackedFile(path, collection string) error {
	docIDs, err := fi.stm.GetFileEmbeddingDocIDs(path, collection)
	if err != nil {
		return fmt.Errorf("load tracked doc ids: %w", err)
	}
	if len(docIDs) > 0 {
		if batcher, ok := fi.vectorDB.(interface{ DeleteDocumentsFromCollection([]string, string) error }); ok {
			if err := batcher.DeleteDocumentsFromCollection(docIDs, collection); err != nil {
				return fmt.Errorf("batch delete vector docs from collection %s: %w", collection, err)
			}
		} else {
			for _, docID := range docIDs {
				if err := fi.vectorDB.DeleteDocumentFromCollection(docID, collection); err != nil {
					return fmt.Errorf("delete vector doc %s from collection %s: %w", docID, collection, err)
				}
			}
		}
		if err := fi.stm.DeleteMemoryMetaBatch(docIDs); err != nil {
			return fmt.Errorf("delete memory meta batch: %w", err)
		}
	}
	if err := fi.stm.DeleteFileIndex(path, collection); err != nil {
		return fmt.Errorf("delete file index: %w", err)
	}
	// Clean up corresponding KG entities if a syncer is attached.
	fi.mu.RLock()
	syncer := fi.kgSyncer
	fi.mu.RUnlock()
	if syncer != nil {
		if res := syncer.CleanupFile(path, collection, false); len(res.Errors) > 0 {
			fi.logger.Warn("[Indexer] KG cleanup produced errors", "path", path, "errors", res.Errors)
		}
	}
	return nil
}

func isPathWithinDir(path, dir string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = filepath.Clean(path)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = filepath.Clean(dir)
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

// shouldRetryIndexingErr determines if an indexing error is transient and worth retrying.
// Mirrors the logic used in KnowledgeGraph retrySemanticEmbedding.
func shouldRetryIndexingErr(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if stderrors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	// Best-effort handling for provider-side throttling / transient upstream issues.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return true
	}
	if strings.Contains(msg, " 429 ") || strings.Contains(msg, "429") {
		return true
	}
	if strings.Contains(msg, " 5") && strings.Contains(msg, "http") {
		return true
	}
	return false
}

// getMultimodalEmbedder creates a MultimodalEmbedder from the current config.
// Returns nil if the embedding provider is not configured.
func (fi *FileIndexer) getMultimodalEmbedder() *memory.MultimodalEmbedder {
	fi.cfgMu.RLock()
	baseURL := fi.cfg.Embeddings.BaseURL
	apiKey := fi.cfg.Embeddings.APIKey
	model := fi.cfg.Embeddings.Model
	format := fi.cfg.Embeddings.MultimodalFormat
	provType := fi.cfg.Embeddings.ProviderType
	fi.cfgMu.RUnlock()

	if baseURL == "" || model == "" {
		return nil
	}

	keyHash := sha256.Sum256([]byte(apiKey))
	cacheKey := baseURL + "|" + model + "|" + format + "|" + provType + "|" + hex.EncodeToString(keyHash[:])
	fi.embedderMu.Lock()
	defer fi.embedderMu.Unlock()
	if fi.cachedEmbedder != nil && fi.cachedEmbedderKey == cacheKey {
		return fi.cachedEmbedder
	}
	embedder := memory.NewMultimodalEmbedder(baseURL, apiKey, model, format, provType, fi.logger)
	fi.cachedEmbedder = embedder
	fi.cachedEmbedderKey = cacheKey
	return embedder
}

// indexEmbedWithRetry calls the provided embedding function with exponential backoff retry.
// It retries up to indexingRetryMaxAttempts times on transient errors (network timeouts,
// rate limits, 5xx HTTP errors). Returns the embedding vector on success.
func (fi *FileIndexer) indexEmbedWithRetry(ctx context.Context, fn func(context.Context) ([]float32, error), path, op string) ([]float32, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		vec, err := fn(ctx)
		if err == nil {
			if attempt > 1 {
				fi.logger.Info("[Indexer] Retry successful", "op", op, "path", path, "attempt", attempt)
			}
			return vec, nil
		}
		lastErr = err

		if attempt == indexingRetryMaxAttempts || !shouldRetryIndexingErr(err) {
			return nil, err
		}

		backoff := time.Duration(attempt*attempt) * indexingRetryBackoffBase
		fi.logger.Warn("[Indexer] Embedding failed, retrying", "op", op, "path", path, "attempt", attempt, "backoff", backoff, "error", err)
		if sleepErr := waitIndexingBackoff(ctx, backoff); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, lastErr
}

// indexStoreWithRetry calls the provided VectorDB store function with exponential backoff retry.
// It retries up to indexingRetryMaxAttempts times on transient errors. Returns the doc IDs on success.
func (fi *FileIndexer) indexStoreWithRetry(ctx context.Context, fn func() ([]string, error), path string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		docIDs, err := fn()
		if err == nil {
			if attempt > 1 {
				fi.logger.Info("[Indexer] Store retry successful", "path", path, "attempt", attempt)
			}
			return docIDs, nil
		}
		lastErr = err

		if attempt == indexingRetryMaxAttempts || !shouldRetryIndexingErr(err) {
			return nil, err
		}

		backoff := time.Duration(attempt*attempt) * indexingRetryBackoffBase
		fi.logger.Warn("[Indexer] Store failed, retrying", "path", path, "attempt", attempt, "backoff", backoff, "error", err)
		if sleepErr := waitIndexingBackoff(ctx, backoff); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, lastErr
}

// indexStoreDocWithRetry is like indexStoreWithRetry but for single document ID return.
func (fi *FileIndexer) indexStoreDocWithRetry(ctx context.Context, fn func() (string, error), path string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		docID, err := fn()
		if err == nil {
			if attempt > 1 {
				fi.logger.Info("[Indexer] Store retry successful", "path", path, "attempt", attempt)
			}
			return docID, nil
		}
		lastErr = err

		if attempt == indexingRetryMaxAttempts || !shouldRetryIndexingErr(err) {
			return "", err
		}

		backoff := time.Duration(attempt*attempt) * indexingRetryBackoffBase
		fi.logger.Warn("[Indexer] Store failed, retrying", "path", path, "attempt", attempt, "backoff", backoff, "error", err)
		if sleepErr := waitIndexingBackoff(ctx, backoff); sleepErr != nil {
			return "", sleepErr
		}
	}
	return "", lastErr
}

func waitIndexingBackoff(ctx context.Context, backoff time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
