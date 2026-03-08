package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

const indexerCollection = "file_index"

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
	cfg        *config.Config
	cfgMu      *sync.RWMutex
	vectorDB   memory.VectorDB
	stm        *memory.SQLiteMemory
	logger     *slog.Logger
	cancel     context.CancelFunc
	mu         sync.RWMutex
	status     IndexerStatus
	extensions map[string]bool
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
	fi.status.Directories = fi.cfg.Indexing.Directories
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
		if err := os.MkdirAll(dir, 0755); err != nil {
			fi.logger.Warn("[Indexer] Failed to create directory", "dir", dir, "error", err)
		}
	}
}

// scan walks all configured directories and indexes new/changed files.
func (fi *FileIndexer) scan() {
	// Skip silently when the embedding pipeline failed at startup — the
	// disabled state is already reported once during VectorDB initialisation.
	if fi.vectorDB.IsDisabled() {
		fi.logger.Debug("[Indexer] VectorDB is disabled, skipping scan")
		return
	}

	start := time.Now()
	fi.logger.Debug("[Indexer] Starting scan...")

	fi.cfgMu.RLock()
	dirs := fi.cfg.Indexing.Directories
	fi.cfgMu.RUnlock()

	var totalFiles, indexedFiles int
	var scanErrors []string

	for _, dir := range dirs {
		nTotal, nIndexed, errs := fi.scanDirectory(dir)
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
	fi.status.Directories = dirs
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

// scanDirectory walks a single directory (recursively) and indexes supported files.
func (fi *FileIndexer) scanDirectory(dir string) (totalFiles, indexedFiles int, errors []string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fi.logger.Debug("[Indexer] Directory does not exist, skipping", "dir", dir)
		return 0, 0, nil
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

		// Determine if IndexImages is on (read under lock since cfg can change)
		fi.cfgMu.RLock()
		indexImages := fi.cfg.Indexing.IndexImages
		fi.cfgMu.RUnlock()

		// Accept: configured text/document extension, OR image when IndexImages is enabled
		if !fi.extensions[ext] && !(isImage && indexImages) {
			return nil
		}

		// ── Binary safety: skip executables and binary files ──
		if IsBinaryFile(path) {
			fi.logger.Debug("[Indexer] Skipping binary file", "path", path)
			return nil
		}

		totalFiles++

		// Check if file needs re-indexing (change detection via SQLite)
		lastIndexed, _ := fi.stm.GetFileIndex(path)
		if !info.ModTime().After(lastIndexed) {
			return nil
		}

		// Build relative path for concept
		relPath, _ := filepath.Rel(dir, path)
		if relPath == "" {
			relPath = info.Name()
		}

		var content string

		// ── Image indexing via Vision LLM ──
		if isImage {
			// indexImages is guaranteed true here (extension filter above ensures this)
			prompt := fmt.Sprintf(
				"Analysiere dieses Bild detailliert. Beschreibe den Inhalt, erkennbare Texte, Objekte und relevante Details. "+
					"Dateiname: %s, Pfad: %s", info.Name(), relPath,
			)

			analysis, visionErr := tools.AnalyzeImageWithPrompt(path, prompt, fi.cfg)
			if visionErr != nil {
				errors = append(errors, fmt.Sprintf("vision error %s: %v", path, visionErr))
				fi.logger.Warn("[Indexer] Vision analysis failed", "path", path, "error", visionErr)
				return nil
			}

			content = fmt.Sprintf("Bildanalyse von %s (Pfad: %s):\n%s", info.Name(), relPath, analysis)

		} else if IsDocumentFile(ext) {
			// ── Document text extraction (PDF, DOCX, XLSX, PPTX, ODT, RTF) ──
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

		} else {
			// ── Plain text files ──
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				errors = append(errors, fmt.Sprintf("read error %s: %v", path, readErr))
				return nil
			}
			content = strings.TrimSpace(string(data))
		}

		if len(content) == 0 {
			return nil
		}

		// Build metadata-rich concept for the embedding
		concept := fmt.Sprintf("Datei: %s (Pfad: %s, Geändert: %s)",
			info.Name(), relPath, info.ModTime().Format("2006-01-02 15:04"))

		// Store in VectorDB — embed file path in concept for retrieval
		_, storeErr := fi.vectorDB.StoreDocument(concept, content)
		if storeErr != nil {
			errors = append(errors, fmt.Sprintf("index error %s: %v", path, storeErr))
			fi.logger.Warn("[Indexer] Failed to index file", "path", path, "error", storeErr)
			return nil
		}

		// Update SQLite timestamp
		_ = fi.stm.UpdateFileIndex(path, indexerCollection, info.ModTime())
		indexedFiles++
		fi.logger.Info("[Indexer] Indexed file", "path", relPath, "size", info.Size())

		return nil
	})
	if err != nil {
		errors = append(errors, fmt.Sprintf("walk error %s: %v", dir, err))
	}

	return totalFiles, indexedFiles, errors
}
