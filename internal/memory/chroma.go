package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"
	"gopkg.in/yaml.v3"
)

// ToolGuideMeta represents the YAML frontmatter of a tool guide.
type ToolGuideMeta struct {
	Description string `yaml:"description"`
}

// IndexToolGuides reads all .md files in the tool folder and indexes them in ChromaDB.
// It skips indexing if tool guides are already present, unless force is true.
// Uses parallel batch indexing for speed.
func (cv *ChromemVectorDB) IndexToolGuides(toolsDir string, force bool) error {
	if cv.disabled.Load() {
		cv.logger.Warn("VectorDB disabled, skipping tool guide indexing")
		return nil
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Get or create collection for tool guides
	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
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

	files, err := os.ReadDir(toolsDir)
	if err != nil {
		return fmt.Errorf("failed to read tools directory: %w", err)
	}

	var docs []chromem.Document
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		path := filepath.Join(toolsDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			cv.logger.Warn("Failed to read tool guide", "path", path, "error", err)
			continue
		}

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

		// Fallback to first 200 chars if no description
		if description == "" {
			contentOnly := strings.TrimSpace(body)
			if len(contentOnly) > 200 {
				description = contentOnly[:200]
			} else {
				description = contentOnly
			}
		}

		docID := fmt.Sprintf("tool_%s", strings.TrimSuffix(file.Name(), ".md"))
		docs = append(docs, chromem.Document{
			ID: docID,
			Metadata: map[string]string{
				"path":      path,
				"tool_name": strings.TrimSuffix(file.Name(), ".md"),
			},
			Content: description,
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

	// Persist the content hash so subsequent startups can skip re-indexing.
	newHash := cv.computeToolGuidesHash(toolsDir)
	hashFile := filepath.Join(cv.dataDir, ".tool_guides_hash")
	_ = os.WriteFile(hashFile, []byte(newHash), 0o644)

	cv.logger.Info("Tool guides indexing completed", "count", collection.Count())
	return nil
}

// SearchToolGuides finds relevant tool guides based on a query.
// Uses the query embedding cache if the same query is reused.
func (cv *ChromemVectorDB) SearchToolGuides(query string, topK int) ([]string, error) {
	if cv.disabled.Load() || query == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
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
	if cv.disabled.Load() {
		cv.logger.Warn("VectorDB disabled, skipping directory indexing", "dir", dir)
		return nil
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Get/Create collection
	collection, err := cv.db.GetOrCreateCollection(collectionName, nil, cv.embeddingFunc)
	if err != nil {
		return fmt.Errorf("failed to get/create %s collection: %w", collectionName, err)
	}

	// 2. Scan directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	type indexedFile struct {
		path    string
		modTime time.Time
	}

	var newDocs []chromem.Document
	var indexedFiles []indexedFile

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		info, err := file.Info()
		if err != nil {
			continue
		}

		// 3. Change detection
		if !force && stm != nil {
			lastIndexed, _ := stm.GetFileIndex(path)
			if !info.ModTime().After(lastIndexed) && collection.Count() > 0 {
				cv.logger.Debug("File unchanged, skipping RAG indexing", "path", path)
				continue
			}
		}
		cv.logger.Info("File new or changed, indexing for RAG", "path", path, "collection", collectionName)

		// 4. Read and Chunk
		data, err := os.ReadFile(path)
		if err != nil {
			cv.logger.Warn("Failed to read file for indexing", "path", path, "error", err)
			continue
		}

		content := string(data)
		title := strings.TrimSuffix(file.Name(), ".md")

		// Simple chunking for indexing (if too large)
		if len(content) <= 4000 {
			newDocs = append(newDocs, chromem.Document{
				ID: fmt.Sprintf("%s_%s", collectionName, title),
				Metadata: map[string]string{
					"path":   path,
					"source": title,
				},
				Content: title + "\n\n" + content,
			})
		} else {
			chunks := chunkText(content, 3500, 200)
			for i, chunk := range chunks {
				newDocs = append(newDocs, chromem.Document{
					ID: fmt.Sprintf("%s_%s_chunk_%d", collectionName, title, i),
					Metadata: map[string]string{
						"path":   path,
						"source": title,
						"chunk":  fmt.Sprintf("%d/%d", i+1, len(chunks)),
					},
					Content: title + " (" + fmt.Sprintf("%d/%d", i+1, len(chunks)) + ")\n\n" + chunk,
				})
			}
		}

		// Track this file for SQLite timestamp update after successful indexing
		indexedFiles = append(indexedFiles, indexedFile{path: path, modTime: info.ModTime()})
	}

	if len(newDocs) == 0 {
		cv.logger.Info("No new/changed documents to index", "dir", dir)
		return nil
	}

	// 6. Batch Add
	concurrency := 4
	cv.logger.Info("Indexing directory...", "dir", dir, "total_docs", len(newDocs))
	if err := collection.AddDocuments(ctx, newDocs, concurrency); err != nil {
		return fmt.Errorf("failed to add documents: %w", err)
	}

	// 7. Success! Update SQLite only for files that were actually indexed
	if stm != nil {
		for _, f := range indexedFiles {
			_ = stm.UpdateFileIndex(f.path, collectionName, f.modTime)
		}
	}

	cv.logger.Info("Directory indexing completed", "dir", dir, "new_count", len(newDocs))
	return nil
}

// computeToolGuidesHash computes a SHA-256 hash of all .md files in toolsDir,
// combining file names and their contents in sorted order. Returns an empty
// string on error so a missing/unreadable directory always forces re-indexing.
func (cv *ChromemVectorDB) computeToolGuidesHash(toolsDir string) string {
	files, err := os.ReadDir(toolsDir)
	if err != nil {
		return ""
	}
	h := sha256.New()
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(toolsDir, f.Name()))
		if err != nil {
			continue
		}
		h.Write([]byte(f.Name()))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
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
