package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

// IndexerCollection is the default collection name for file indexing.
const IndexerCollection = "file_index"

const fileIndexerFingerprint = "file-indexer-v2"

// Retry constants for indexing operations (aligned with KnowledgeGraph retry pattern).
const (
	indexingRetryMaxAttempts = 3
	indexingRetryBackoffBase = 250 * time.Millisecond
)

// IndexerStatus holds runtime statistics for the file indexer.
type IndexerStatus struct {
	Running          bool      `json:"running"`
	Directories      []string  `json:"directories"`
	TotalFiles       int       `json:"total_files"`
	IndexedFiles     int       `json:"indexed_files"`
	LastScanAt       time.Time `json:"last_scan_at,omitempty"`
	LastScanDuration string    `json:"last_scan_duration,omitempty"`
	Errors           []string  `json:"errors,omitempty"`
}

// FileIndexer watches configured directories and indexes files into VectorDB.
type FileIndexer struct {
	cfg               *config.Config
	cfgMu             *sync.RWMutex
	vectorDB          memory.VectorDB
	stm               *memory.SQLiteMemory
	logger            *slog.Logger
	cancel            context.CancelFunc
	mu                sync.RWMutex
	status            IndexerStatus
	extensions        map[string]bool
	embedderMu        sync.Mutex
	cachedEmbedder    *memory.MultimodalEmbedder
	cachedEmbedderKey string
	kgSyncer          *FileKGSyncer
}

// NewFileIndexer creates a new file indexer service.
func NewFileIndexer(cfg *config.Config, cfgMu *sync.RWMutex, vectorDB memory.VectorDB, stm *memory.SQLiteMemory, logger *slog.Logger) *FileIndexer {
	extMap := make(map[string]bool, len(cfg.Indexing.Extensions))
	for _, ext := range cfg.Indexing.Extensions {
		extMap[strings.ToLower(ext)] = true
	}

	return &FileIndexer{
		cfg:        cfg,
		cfgMu:      cfgMu,
		vectorDB:   vectorDB,
		stm:        stm,
		logger:     logger,
		extensions: extMap,
	}
}

// Start begins the indexing loop. It performs an initial scan and then polls
// at the configured interval.
func (fi *FileIndexer) Start(ctx context.Context) {
	ctx, fi.cancel = context.WithCancel(ctx)

	fi.logger.Info("[Indexer] Starting file indexer",
		"directories", fi.cfg.Indexing.Directories,
		"poll_interval", fi.cfg.Indexing.PollIntervalSeconds,
		"extensions", fi.cfg.Indexing.Extensions)

	fi.mu.Lock()
	fi.status.Running = true
	dirPaths := make([]string, len(fi.cfg.Indexing.Directories))
	for i, d := range fi.cfg.Indexing.Directories {
		dirPaths[i] = d.Path
	}
	fi.status.Directories = dirPaths
	fi.mu.Unlock()

	// Ensure all configured directories exist
	fi.ensureDirectories()

	// Initial scan
	fi.scan()

	// Poll loop
	go func() {
		ticker := time.NewTicker(time.Duration(fi.cfg.Indexing.PollIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fi.mu.Lock()
				fi.status.Running = false
				fi.mu.Unlock()
				fi.logger.Info("[Indexer] File indexer stopped")
				return
			case <-ticker.C:
				fi.scan()
			}
		}
	}()
}

// Stop gracefully stops the indexer.
func (fi *FileIndexer) Stop() {
	if fi.cancel != nil {
		fi.cancel()
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
	return fi.status
}

// Rescan triggers an immediate full rescan of all directories.
func (fi *FileIndexer) Rescan() {
	go fi.scan()
}

// ensureDirectories creates any missing configured directories.
func (fi *FileIndexer) ensureDirectories() {
	for _, dir := range fi.cfg.Indexing.Directories {
		if err := os.MkdirAll(dir.Path, 0755); err != nil {
			fi.logger.Warn("[Indexer] Failed to create directory", "dir", dir.Path, "error", err)
		}
	}
}

// scan walks all configured directories and indexes new/changed files.
func (fi *FileIndexer) scan() {
	start := time.Now()
	fi.logger.Debug("[Indexer] Starting scan...")

	fi.cfgMu.RLock()
	dirs := fi.cfg.Indexing.Directories
	fi.cfgMu.RUnlock()

	var totalFiles, indexedFiles int
	var scanErrors []string
	var dirPaths []string

	if fi.vectorDB.IsDisabled() {
		for _, indexingDir := range dirs {
			dirPaths = append(dirPaths, indexingDir.Path)
			nTotal, errs := fi.countIndexableFiles(indexingDir.Path)
			totalFiles += nTotal
			scanErrors = append(scanErrors, errs...)
		}
		scanErrors = append(scanErrors, "Embedding pipeline unavailable; file indexing is disabled until an embedding provider is configured.")
		duration := time.Since(start)

		fi.mu.Lock()
		fi.status.TotalFiles = totalFiles
		fi.status.IndexedFiles = 0
		fi.status.LastScanAt = start
		fi.status.LastScanDuration = duration.Round(time.Millisecond).String()
		fi.status.Errors = scanErrors
		fi.status.Directories = dirPaths
		fi.mu.Unlock()

		fi.logger.Debug("[Indexer] VectorDB is disabled, scan status updated", "total_files", totalFiles, "duration", duration.Round(time.Millisecond))
		return
	}

	for _, indexingDir := range dirs {
		dirPaths = append(dirPaths, indexingDir.Path)
		collection := getDirCollection(indexingDir)
		nTotal, nIndexed, errs := fi.scanDirectory(indexingDir.Path, collection)
		totalFiles += nTotal
		indexedFiles += nIndexed
		scanErrors = append(scanErrors, errs...)
	}

	duration := time.Since(start)

	fi.mu.Lock()
	fi.status.TotalFiles = totalFiles
	fi.status.IndexedFiles = indexedFiles
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

func (fi *FileIndexer) countIndexableFiles(dir string) (int, []string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	var total int
	var errors []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errors = append(errors, fmt.Sprintf("walk error %s: %v", path, err))
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
			total++
		}
		return nil
	})
	if err != nil {
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
func (fi *FileIndexer) scanDirectory(dir, collection string) (totalFiles, indexedFiles int, errors []string) {
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
		if err != nil {
			errors = append(errors, fmt.Sprintf("walk error %s: %v", path, err))
			return nil // continue walking
		}

		// Skip directories and hidden files/folders
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(info.Name()))
		isImage := IsImageFile(ext)
		isAudio := IsAudioFile(ext)

		// Determine config flags (read under lock since cfg can change)
		fi.cfgMu.RLock()
		indexImages := fi.cfg.Indexing.IndexImages
		multimodal := fi.cfg.Embeddings.Multimodal
		fi.cfgMu.RUnlock()

		// Accept: configured text/document extension, OR image when IndexImages or multimodal is enabled,
		// OR audio when multimodal is enabled
		accepted := fi.extensions[ext] ||
			(isImage && (indexImages || multimodal)) ||
			(isAudio && multimodal)
		if !accepted {
			return nil
		}

		// Binary safety: skip executables and binary files.
		if IsBinaryFile(path) {
			fi.logger.Debug("[Indexer] Skipping binary file", "path", path)
			return nil
		}

		totalFiles++
		seenPaths[path] = struct{}{}

		// Build relative path for concept
		relPath, _ := filepath.Rel(dir, path)
		if relPath == "" {
			relPath = info.Name()
		}

		var content string
		var precomputedEmbedding []float32

		// Multimodal path: images and audio via multimodal embedding API.
		if multimodal && (isImage || isAudio) {
			embedder := fi.getMultimodalEmbedder()
			if embedder == nil {
				errors = append(errors, fmt.Sprintf("multimodal embedder unavailable for %s", path))
				return nil
			}
			vec, embedErr := fi.indexEmbedWithRetry(func() ([]float32, error) {
				return embedder.EmbedFile(context.Background(), path)
			}, path, "multimodal")
			if embedErr != nil {
				errors = append(errors, fmt.Sprintf("multimodal embed error %s: %v", path, embedErr))
				fi.logger.Warn("[Indexer] Multimodal embedding failed", "path", path, "error", embedErr)
				return nil
			}
			precomputedEmbedding = vec
			kind := "Bild"
			if isAudio {
				kind = "Audio"
			}
			content = fmt.Sprintf("%s-Datei: %s (Pfad: %s)", kind, info.Name(), relPath)

		} else if isImage {
			// Image indexing via Vision LLM (non-multimodal fallback).
			prompt := fmt.Sprintf(
				"Analysiere dieses Bild detailliert. Beschreibe den Inhalt, erkennbare Texte, Objekte und relevante Details. "+
					"Dateiname: %s, Pfad: %s", info.Name(), relPath,
			)

			var analysis string
			var visionErr error
			for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
				var a string
				var w, h int
				a, w, h, visionErr = tools.AnalyzeImageWithPrompt(path, prompt, fi.cfg)
				if visionErr == nil {
					if attempt > 1 {
						fi.logger.Info("[Indexer] Vision retry successful", "path", path, "attempt", attempt)
					}
					analysis = a
					_ = w
					_ = h
					break
				}

				if attempt == indexingRetryMaxAttempts || !shouldRetryIndexingErr(visionErr) {
					break
				}

				backoff := time.Duration(attempt*attempt) * indexingRetryBackoffBase
				fi.logger.Warn("[Indexer] Vision analysis failed, retrying", "path", path, "attempt", attempt, "backoff", backoff, "error", visionErr)
				time.Sleep(backoff)
			}
			if visionErr != nil {
				errors = append(errors, fmt.Sprintf("vision error %s: %v", path, visionErr))
				fi.logger.Warn("[Indexer] Vision analysis failed", "path", path, "error", visionErr)
				return nil
			}

			content = fmt.Sprintf("Bildanalyse von %s (Pfad: %s):\n%s", info.Name(), relPath, analysis)

		} else if IsDocumentFile(ext) {
			// Document text extraction (PDF, DOCX, XLSX, PPTX, ODT, RTF).
			extracted, extractErr := ExtractText(path)
			if extractErr != nil {
				fi.logger.Warn("[Indexer] Text extraction failed, trying raw read", "path", path, "error", extractErr)
				// Fallback: try reading as raw text
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					errors = append(errors, fmt.Sprintf("read error %s: %v", path, readErr))
					return nil
				}
				content = strings.TrimSpace(string(data))
			} else {
				content = extracted
			}

			// PDF auto-fallback: if text extraction yielded almost nothing and
			// multimodal is enabled, treat the PDF as an image (scanned document)
			if multimodal && ext == ".pdf" && len(strings.TrimSpace(content)) < 10 {
				embedder := fi.getMultimodalEmbedder()
				if embedder != nil {
					vec, embedErr := fi.indexEmbedWithRetry(func() ([]float32, error) {
						return embedder.EmbedFile(context.Background(), path)
					}, path, "pdf-fallback")
					if embedErr == nil {
						precomputedEmbedding = vec
						content = fmt.Sprintf("PDF (gescannt): %s (Pfad: %s)", info.Name(), relPath)
						fi.logger.Info("[Indexer] PDF auto-fallback to multimodal embedding", "path", relPath)
					} else {
						fi.logger.Warn("[Indexer] PDF multimodal fallback failed", "path", path, "error", embedErr)
					}
				}
			}

		} else {
			// Plain text files.
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				errors = append(errors, fmt.Sprintf("read error %s: %v", path, readErr))
				return nil
			}
			content = strings.TrimSpace(string(data))
		}

		if len(content) == 0 && precomputedEmbedding == nil {
			return nil
		}

		contentHash := hashIndexedFileContent(content, precomputedEmbedding, path)
		indexFingerprint := fi.indexFingerprint()
		indexState, _ := fi.stm.GetFileIndexState(path, collection)
		if !shouldReindexFile(info.ModTime(), contentHash, indexFingerprint, indexState) {
			return nil
		}

		if err := fi.removeTrackedFile(path, collection); err != nil {
			errors = append(errors, fmt.Sprintf("cleanup error %s: %v", path, err))
			fi.logger.Warn("[Indexer] Failed to remove stale embeddings before reindex", "path", path, "error", err)
			return nil
		}

		// Build metadata-rich concept for the embedding
		concept := fmt.Sprintf("Datei: %s (Pfad: %s, Geändert: %s)",
			info.Name(), relPath, info.ModTime().Format("2006-01-02 15:04"))

		// Store in VectorDB - use pre-computed embedding when available.
		// Use collection-aware methods to route documents to the per-directory collection.
		var docIDs []string
		var storeErr error
		if precomputedEmbedding != nil {
			var docID string
			docID, storeErr = fi.indexStoreDocWithRetry(func() (string, error) {
				return fi.vectorDB.StoreDocumentWithEmbeddingInCollection(concept, content, precomputedEmbedding, collection)
			}, path)
			if storeErr == nil && docID != "" {
				docIDs = []string{docID}
			}
		} else {
			docIDs, storeErr = fi.indexStoreWithRetry(func() ([]string, error) {
				return fi.vectorDB.StoreDocumentInCollection(concept, content, collection)
			}, path)
		}
		if storeErr != nil {
			errors = append(errors, fmt.Sprintf("index error %s: %v", path, storeErr))
			fi.logger.Warn("[Indexer] Failed to index file", "path", path, "error", storeErr)
			return nil
		}

		if err := fi.stm.UpdateFileIndexWithDocsAndState(path, collection, info.ModTime(), contentHash, indexFingerprint, docIDs); err != nil {
			errors = append(errors, fmt.Sprintf("tracking error %s: %v", path, err))
			fi.logger.Warn("[Indexer] Failed to persist file index tracking", "path", path, "error", err)
			return nil
		}

		indexedFiles++
		fi.logger.Info("[Indexer] Indexed file", "path", relPath, "size", info.Size(), "doc_ids", len(docIDs))
		fi.syncIndexedFileToKG(path, collection)

		return nil
	})
	if err != nil {
		errors = append(errors, fmt.Sprintf("walk error %s: %v", dir, err))
	}

	errors = append(errors, fi.cleanupDeletedTrackedFiles(dir, collection, trackedPaths, seenPaths)...)
	return totalFiles, indexedFiles, errors
}

type embeddingFingerprinter interface {
	EmbeddingFingerprint() string
}

func (fi *FileIndexer) indexFingerprint() string {
	if fp, ok := fi.vectorDB.(embeddingFingerprinter); ok {
		if embeddingFingerprint := strings.TrimSpace(fp.EmbeddingFingerprint()); embeddingFingerprint != "" {
			return fileIndexerFingerprint + "|" + embeddingFingerprint
		}
	}
	return fileIndexerFingerprint
}

func shouldReindexFile(modTime time.Time, contentHash, indexFingerprint string, state memory.FileIndexState) bool {
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
	return modTime.After(state.LastModified)
}

func hashIndexedFileContent(content string, precomputedEmbedding []float32, path string) string {
	h := sha256.New()
	if precomputedEmbedding != nil {
		if data, err := os.ReadFile(path); err == nil {
			h.Write(data)
			return hex.EncodeToString(h.Sum(nil))
		}
	}
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
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

	go func() {
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
		fi.logger.Info("[Indexer] Removed embeddings for deleted file", "path", trackedPath)
	}
	return errors
}

func (fi *FileIndexer) removeTrackedFile(path, collection string) error {
	docIDs, err := fi.stm.GetFileEmbeddingDocIDs(path, collection)
	if err != nil {
		return fmt.Errorf("load tracked doc ids: %w", err)
	}
	for _, docID := range docIDs {
		if err := fi.vectorDB.DeleteDocumentFromCollection(docID, collection); err != nil {
			return fmt.Errorf("delete vector doc %s from collection %s: %w", docID, collection, err)
		}
		if err := fi.stm.DeleteMemoryMeta(docID); err != nil {
			return fmt.Errorf("delete memory meta %s: %w", docID, err)
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
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
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

	cacheKey := baseURL + "|" + model + "|" + format + "|" + provType
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
func (fi *FileIndexer) indexEmbedWithRetry(fn func() ([]float32, error), path, op string) ([]float32, error) {
	var lastErr error
	for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
		vec, err := fn()
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
		time.Sleep(backoff)
	}
	return nil, lastErr
}

// indexStoreWithRetry calls the provided VectorDB store function with exponential backoff retry.
// It retries up to indexingRetryMaxAttempts times on transient errors. Returns the doc IDs on success.
func (fi *FileIndexer) indexStoreWithRetry(fn func() ([]string, error), path string) ([]string, error) {
	var lastErr error
	for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
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
		time.Sleep(backoff)
	}
	return nil, lastErr
}

// indexStoreDocWithRetry is like indexStoreWithRetry but for single document ID return.
func (fi *FileIndexer) indexStoreDocWithRetry(fn func() (string, error), path string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= indexingRetryMaxAttempts; attempt++ {
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
		time.Sleep(backoff)
	}
	return "", lastErr
}
