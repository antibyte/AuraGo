package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	promptsembed "aurago/prompts"
	chromem "github.com/philippgille/chromem-go"
	"gopkg.in/yaml.v3"
)

// ToolGuideMeta represents the YAML frontmatter of a tool guide.
type ToolGuideMeta struct {
	Description string `yaml:"description"`
}

const markdownIndexerFingerprint = "markdown-rag-indexer-v2"

// IndexToolGuides reads all .md files in the tool folder and indexes them in ChromaDB.
// It skips indexing if tool guides are already present, unless force is true.
// Uses parallel batch indexing for speed.
func (cv *ChromemVectorDB) IndexToolGuides(toolsDir string, force bool) error {
	doneIndex, err := cv.beginTrackedOperation(&cv.indexingWg)
	if err != nil {
		return err
	}
	defer doneIndex()

	if cv.disabled.Load() {
		cv.logger.Warn("VectorDB disabled, skipping tool guide indexing")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Get or create collection for tool guides
	cv.mu.Lock()
	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
	cv.mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to get/create tool_guides collection: %w", err)
	}

	// Skip if already indexed and content hasn't changed.
	// We compute a SHA-256 hash of every .md file's content (sorted by name) and
	// store it in <dataDir>/.tool_guides_hash. On redeploy with changed content
	// the hash differs → full re-index. Pure count-based check was insufficient
	// because it could not detect file content updates when the count stayed the same.
	if !force && collection.Count() > 0 {
		currentHash := cv.computeToolGuidesHash(toolsDir)
		hashFile := filepath.Join(cv.dataDir, ".tool_guides_hash")
		storedHashBytes, _ := os.ReadFile(hashFile)
		if strings.TrimSpace(string(storedHashBytes)) == currentHash {
			cv.logger.Info("Tool guides already indexed, skipping", "count", collection.Count())
			return nil
		}
		cv.logger.Info("Tool guides content changed, re-indexing", "old_hash", strings.TrimSpace(string(storedHashBytes)), "new_hash", currentHash)
	}

	var docs []chromem.Document
	newDocIDs := make(map[string]struct{})
	guideFiles, err := loadToolGuideFilesWithWarnings(toolsDir, cv.logger)
	if err != nil {
		return err
	}
	for _, guide := range guideFiles {
		path := guide.Path
		data := guide.Data

		raw := string(data)
		description := ""

		// Extract frontmatter and body using line-delimited split
		frontmatter, body := splitFrontmatter(raw)

		// Extract description from frontmatter
		if frontmatter != "" {
			var meta ToolGuideMeta
			if err := yaml.Unmarshal([]byte(frontmatter), &meta); err == nil {
				description = meta.Description
			}
		}

		content := buildToolGuideEmbeddingContent(description, body)

		docID := fmt.Sprintf("tool_%s", strings.TrimSuffix(guide.Name, ".md"))
		newDocIDs[docID] = struct{}{}
		docs = append(docs, chromem.Document{
			ID: docID,
			Metadata: map[string]string{
				"path":      path,
				"tool_name": strings.TrimSuffix(guide.Name, ".md"),
			},
			Content: content,
		})
	}

	if len(docs) == 0 {
		return nil
	}

	// Use parallel AddDocuments (batch size 8 or length)
	concurrency := 8
	if len(docs) < 8 {
		concurrency = len(docs)
	}

	cv.logger.Info("Indexing tool guides...", "total", len(docs), "concurrency", concurrency)
	if err := collection.AddDocuments(ctx, docs, concurrency); err != nil {
		return fmt.Errorf("failed to batch index tool guides: %w", err)
	}

	previousDocIDs := cv.readToolGuidesDocManifest()
	for _, oldID := range previousDocIDs {
		if _, ok := newDocIDs[oldID]; ok {
			continue
		}
		if err := collection.Delete(ctx, nil, nil, oldID); err != nil {
			cv.logger.Warn("Failed to delete stale tool guide document", "doc_id", oldID, "error", err)
		}
	}

	// Persist the content hash so subsequent startups can skip re-indexing.
	newHash := cv.computeToolGuidesHash(toolsDir)
	hashFile := filepath.Join(cv.dataDir, ".tool_guides_hash")
	if err := os.WriteFile(hashFile, []byte(newHash), 0o644); err != nil {
		cv.logger.Warn("Failed to write tool guides content hash", "path", hashFile, "error", err)
		return fmt.Errorf("write tool guides content hash: %w", err)
	}
	if err := cv.writeToolGuidesDocManifest(mapKeysSorted(newDocIDs)); err != nil {
		cv.logger.Warn("Failed to write tool guides doc manifest", "error", err)
		return fmt.Errorf("write tool guides doc manifest: %w", err)
	}

	cv.logger.Info("Tool guides indexing completed", "count", collection.Count())
	return nil
}

func mapKeysSorted(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func buildToolGuideEmbeddingContent(description, body string) string {
	description = strings.TrimSpace(description)
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) > 4000 {
		body = string([]rune(body)[:4000])
	}
	switch {
	case description != "" && body != "":
		return description + "\n\n" + body
	case description != "":
		return description
	case body != "":
		return body
	default:
		return ""
	}
}

// SearchToolGuides finds relevant tool guides based on a query.
// Uses the query embedding cache if the same query is reused.
func (cv *ChromemVectorDB) SearchToolGuides(query string, topK int) ([]string, error) {
	return cv.SearchToolGuidesContext(context.Background(), query, topK)
}

// SearchToolGuidesContext finds relevant tool guides and honors caller cancellation.
func (cv *ChromemVectorDB) SearchToolGuidesContext(ctx context.Context, query string, topK int) ([]string, error) {
	if query == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	doneSearch, err := cv.beginTrackedOperation(&cv.searchWg)
	if err != nil {
		return nil, err
	}
	defer doneSearch()

	if err := cv.requireReadyForSearch(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cv.mu.RLock()
	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
	cv.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get tool_guides collection: %w", err)
	}

	if collection.Count() == 0 {
		return nil, nil
	}

	queryEmbedding, err := cv.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}

	results, err := collection.QueryEmbedding(ctx, queryEmbedding, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query tool guides: %w", err)
	}

	var guidePaths []string
	for _, result := range results {
		if result.Similarity > 0.3 {
			if path, ok := result.Metadata["path"]; ok {
				guidePaths = append(guidePaths, path)
			}
		}
	}

	return guidePaths, nil
}

// IndexDirectory scans a directory for markdown files and indexes them if they've changed.
func (cv *ChromemVectorDB) IndexDirectory(dir, collectionName string, stm *SQLiteMemory, force bool) error {
	doneIndex, err := cv.beginTrackedOperation(&cv.indexingWg)
	if err != nil {
		return err
	}
	defer doneIndex()

	if cv.disabled.Load() {
		cv.logger.Warn("VectorDB disabled, skipping directory indexing", "dir", dir)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Get/Create collection
	cv.mu.Lock()
	collection, err := cv.db.GetOrCreateCollection(collectionName, nil, cv.embeddingFunc)
	cv.mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to get/create %s collection: %w", collectionName, err)
	}

	currentMarkdown := make(map[string]struct{})
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	type indexedFile struct {
		path             string
		modTime          time.Time
		contentHash      string
		indexFingerprint string
		source           string
		docs             []chromem.Document
		docIDs           []string
	}

	var indexedFiles []indexedFile
	totalDocs := 0

	for _, file := range files {
		if file.Type()&os.ModeSymlink != 0 {
			cv.logger.Debug("Skipping symlinked markdown file", "path", filepath.Join(dir, file.Name()))
			continue
		}
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		currentMarkdown[path] = struct{}{}
		info, err := file.Info()
		if err != nil {
			continue
		}

		// 3. Read before change detection so same-mtime content changes are caught.
		data, err := os.ReadFile(path)
		if err != nil {
			cv.logger.Warn("Failed to read file for indexing", "path", path, "error", err)
			continue
		}
		contentHash := hashMarkdownIndexContent(data)
		indexFingerprint := cv.markdownIndexFingerprint()

		// 3. Change detection
		if !force && stm != nil {
			indexState, err := stm.GetFileIndexState(path, collectionName)
			if err != nil {
				return fmt.Errorf("get file index for %s in %s: %w", path, collectionName, err)
			}
			if !shouldReindexMarkdownFile(info.ModTime(), contentHash, indexFingerprint, indexState) && collection.Count() > 0 {
				cv.logger.Debug("File unchanged, skipping RAG indexing", "path", path)
				continue
			}
		}
		cv.logger.Info("File new or changed, indexing for RAG", "path", path, "collection", collectionName)

		content := string(data)
		title := strings.TrimSuffix(file.Name(), ".md")
		planned := indexedFile{
			path:             path,
			modTime:          info.ModTime(),
			contentHash:      contentHash,
			indexFingerprint: indexFingerprint,
			source:           title,
		}

		// Simple chunking for indexing (if too large)
		if utf8.RuneCountInString(content) <= 4000 {
			docID := fmt.Sprintf("%s_%s", collectionName, title)
			planned.docs = append(planned.docs, chromem.Document{
				ID: docID,
				Metadata: map[string]string{
					"path":   path,
					"source": title,
				},
				Content: title + "\n\n" + content,
			})
			planned.docIDs = append(planned.docIDs, docID)
		} else {
			chunks := chunkText(content, 3500, 200)
			for i, chunk := range chunks {
				docID := fmt.Sprintf("%s_%s_chunk_%d", collectionName, title, i)
				planned.docs = append(planned.docs, chromem.Document{
					ID: docID,
					Metadata: map[string]string{
						"path":   path,
						"source": title,
						"chunk":  fmt.Sprintf("%d/%d", i+1, len(chunks)),
					},
					Content: title + " (" + fmt.Sprintf("%d/%d", i+1, len(chunks)) + ")\n\n" + chunk,
				})
				planned.docIDs = append(planned.docIDs, docID)
			}
		}

		// Track this file for SQLite timestamp update after successful indexing
		if len(planned.docs) > 0 {
			indexedFiles = append(indexedFiles, planned)
			totalDocs += len(planned.docs)
		}
	}

	if stm != nil {
		trackedPaths, listErr := stm.ListIndexedFiles(collectionName)
		if listErr != nil {
			return fmt.Errorf("list indexed files for %s: %w", collectionName, listErr)
		}
		for _, trackedPath := range trackedPaths {
			if !isPathWithinDirectory(trackedPath, dir) {
				continue
			}
			if _, ok := currentMarkdown[trackedPath]; ok {
				continue
			}
			docIDs, idsErr := stm.GetFileEmbeddingDocIDs(trackedPath, collectionName)
			if idsErr != nil {
				return fmt.Errorf("get tracked doc ids for %s in %s: %w", trackedPath, collectionName, idsErr)
			}
			source := strings.TrimSuffix(filepath.Base(trackedPath), ".md")
			for _, docID := range docIDs {
				if delErr := collection.Delete(ctx, nil, nil, docID); delErr != nil {
					cv.logger.Warn("Failed to delete stale docs for removed file by id", "path", trackedPath, "doc_id", docID, "error", delErr)
				}
			}
			if len(docIDs) == 0 {
				if delErr := collection.Delete(ctx, map[string]string{"source": source}, nil); delErr != nil {
					cv.logger.Warn("Failed to delete stale docs for removed file by source", "path", trackedPath, "source", source, "error", delErr)
				}
			}
			if delErr := stm.DeleteFileIndex(trackedPath, collectionName); delErr != nil {
				return fmt.Errorf("delete file index for removed file %s in %s: %w", trackedPath, collectionName, delErr)
			}
			cv.logger.Info("Removed stale RAG docs for deleted file", "path", trackedPath, "collection", collectionName)
		}
	}

	if len(indexedFiles) == 0 {
		cv.logger.Info("No new/changed documents to index", "dir", dir)
		return nil
	}

	concurrency := 4
	cv.logger.Info("Indexing directory...", "dir", dir, "total_docs", totalDocs)
	var updateErr error
	for _, f := range indexedFiles {
		if delErr := collection.Delete(ctx, map[string]string{"source": f.source}, nil); delErr != nil {
			cv.logger.Warn("Failed to delete stale docs for file", "source", f.source, "error", delErr)
		}
		fileConcurrency := concurrency
		if len(f.docs) < fileConcurrency {
			fileConcurrency = len(f.docs)
		}
		if fileConcurrency <= 0 {
			fileConcurrency = 1
		}
		if err := collection.AddDocuments(ctx, f.docs, fileConcurrency); err != nil {
			if stm != nil {
				if delErr := stm.DeleteFileIndex(f.path, collectionName); delErr != nil {
					updateErr = errors.Join(updateErr, fmt.Errorf("delete failed file index for %s in %s: %w", f.path, collectionName, delErr))
				}
			}
			return errors.Join(updateErr, fmt.Errorf("failed to add documents for %s: %w", f.path, err))
		}
		if stm != nil {
			if err := stm.UpdateFileIndexWithDocsAndState(f.path, collectionName, f.modTime, f.contentHash, f.indexFingerprint, f.docIDs); err != nil {
				updateErr = errors.Join(updateErr, fmt.Errorf("update file index for %s in %s: %w", f.path, collectionName, err))
			}
		}
	}
	if updateErr != nil {
		return updateErr
	}

	cv.logger.Info("Directory indexing completed", "dir", dir, "new_count", totalDocs)
	return nil
}

func (cv *ChromemVectorDB) markdownIndexFingerprint() string {
	if embeddingFingerprint := strings.TrimSpace(cv.embeddingFingerprint); embeddingFingerprint != "" {
		return markdownIndexerFingerprint + "|" + embeddingFingerprint
	}
	return markdownIndexerFingerprint
}

func shouldReindexMarkdownFile(modTime time.Time, contentHash, indexFingerprint string, state FileIndexState) bool {
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

func hashMarkdownIndexContent(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func isPathWithinDirectory(path, dir string) bool {
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

// computeToolGuidesHash computes a SHA-256 hash of all .md files in toolsDir,
// combining file names and their contents in sorted order. Returns an empty
// string on error so a missing/unreadable directory always forces re-indexing.
func (cv *ChromemVectorDB) computeToolGuidesHash(toolsDir string) string {
	files, err := loadToolGuideFiles(toolsDir)
	if err != nil {
		return ""
	}
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Name))
		h.Write(f.Data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (cv *ChromemVectorDB) toolGuidesDocManifestPath() string {
	if cv.dataDir == "" {
		return ".tool_guides_docs.json"
	}
	return filepath.Join(cv.dataDir, ".tool_guides_docs.json")
}

func (cv *ChromemVectorDB) readToolGuidesDocManifest() []string {
	data, err := os.ReadFile(cv.toolGuidesDocManifestPath())
	if err != nil {
		return nil
	}
	var docIDs []string
	if err := json.Unmarshal(data, &docIDs); err != nil {
		if cv.logger != nil {
			cv.logger.Warn("Failed to parse tool guides doc manifest", "error", err)
		}
		return nil
	}
	return docIDs
}

func (cv *ChromemVectorDB) writeToolGuidesDocManifest(docIDs []string) error {
	data, err := json.MarshalIndent(docIDs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cv.toolGuidesDocManifestPath(), data, 0o644)
}

type toolGuideFile struct {
	Name string
	Path string
	Data []byte
}

func loadToolGuideFiles(toolsDir string) ([]toolGuideFile, error) {
	return loadToolGuideFilesWithWarnings(toolsDir, nil)
}

func loadToolGuideFilesWithWarnings(toolsDir string, logger *slog.Logger) ([]toolGuideFile, error) {
	files, err := os.ReadDir(toolsDir)
	if err == nil {
		guides := make([]toolGuideFile, 0, len(files))
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
				continue
			}
			path := filepath.Join(toolsDir, file.Name())
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				if logger != nil {
					logger.Warn("Skipping unreadable tool guide", "path", path, "error", readErr)
				}
				continue
			}
			guides = append(guides, toolGuideFile{
				Name: file.Name(),
				Path: path,
				Data: data,
			})
		}
		return guides, nil
	}

	embedEntries, embedErr := fs.ReadDir(promptsembed.FS, "tools_manuals")
	if embedErr != nil {
		return nil, fmt.Errorf("failed to read tools directory: %w", err)
	}
	guides := make([]toolGuideFile, 0, len(embedEntries))
	for _, entry := range embedEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		embedPath := filepath.ToSlash(filepath.Join("tools_manuals", entry.Name()))
		data, readErr := fs.ReadFile(promptsembed.FS, embedPath)
		if readErr != nil {
			if logger != nil {
				logger.Warn("Skipping unreadable embedded tool guide", "path", embedPath, "error", readErr)
			}
			continue
		}
		guides = append(guides, toolGuideFile{
			Name: entry.Name(),
			Path: embedPath,
			Data: data,
		})
	}
	return guides, nil
}

// splitFrontmatter splits a YAML frontmatter document (---\n...\n---\n...) into
// the frontmatter YAML string and the body. Returns ("", raw) if no frontmatter.
func splitFrontmatter(raw string) (string, string) {
	if !strings.HasPrefix(raw, "---") {
		return "", raw
	}
	inner := raw[3:]
	inner = strings.TrimLeft(inner, "\r\n")
	idx := strings.Index(inner, "\n---\n")
	if idx < 0 {
		idx = strings.Index(inner, "\n---\r\n")
	}
	if idx < 0 {
		return "", raw
	}
	frontmatter := inner[:idx]
	body := strings.TrimLeft(inner[idx+4:], "\r\n") // skip "\n---"
	return frontmatter, body
}

// IndexToolGuidesAsync starts tool guide indexing in a background goroutine.
// Returns immediately. Use IsIndexing() to check progress.
func (cv *ChromemVectorDB) IndexToolGuidesAsync(toolsDir string, force bool) {
	cv.indexing.Add(1)
	go func() {
		defer cv.indexing.Add(-1)
		if err := cv.IndexToolGuides(toolsDir, force); err != nil {
			cv.logger.Error("Async tool guide indexing failed", "error", err)
		}
	}()
}

// IndexDirectoryAsync starts directory indexing in a background goroutine.
// Returns immediately. Use IsIndexing() to check progress.
func (cv *ChromemVectorDB) IndexDirectoryAsync(dir, collectionName string, stm *SQLiteMemory, force bool) {
	cv.indexing.Add(1)
	go func() {
		defer cv.indexing.Add(-1)
		if err := cv.IndexDirectory(dir, collectionName, stm, force); err != nil {
			cv.logger.Error("Async directory indexing failed", "dir", dir, "error", err)
		}
	}()
}
