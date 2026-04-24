package tools

import (
	"aurago/internal/dbutil"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// MediaItem represents a single entry in the media registry.
type MediaItem struct {
	ID               int64    `json:"id"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
	MediaType        string   `json:"media_type"`  // image, video, tts, audio, music
	SourceTool       string   `json:"source_tool"` // generate_image, generate_video, send_video, tts, transcribe, manual
	Filename         string   `json:"filename"`
	FilePath         string   `json:"file_path"`
	WebPath          string   `json:"web_path"`
	FileSize         int64    `json:"file_size"`
	Format           string   `json:"format"` // png, jpg, mp3, wav, etc.
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
	GenerationTimeMs int64    `json:"generation_time_ms,omitempty"`
	CostEstimate     float64  `json:"cost_estimate,omitempty"`
	SourceImage      string   `json:"source_image,omitempty"`
	Quality          string   `json:"quality,omitempty"`
	Style            string   `json:"style,omitempty"`
	Size             string   `json:"size,omitempty"`
	Language         string   `json:"language,omitempty"`
	VoiceID          string   `json:"voice_id,omitempty"`
	Hash             string   `json:"hash,omitempty"`
	Deleted          bool     `json:"deleted"`
}

// InitMediaRegistryDB initializes the media registry SQLite database.
func InitMediaRegistryDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open media registry database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS media_items (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		media_type         TEXT NOT NULL DEFAULT 'image',
		source_tool        TEXT NOT NULL DEFAULT 'manual',
		filename           TEXT NOT NULL,
		file_path          TEXT NOT NULL DEFAULT '',
		web_path           TEXT NOT NULL DEFAULT '',
		file_size          INTEGER DEFAULT 0,
		format             TEXT DEFAULT '',
		provider           TEXT DEFAULT '',
		model              TEXT DEFAULT '',
		prompt             TEXT DEFAULT '',
		description        TEXT DEFAULT '',
		tags               TEXT DEFAULT '[]',
		duration_ms        INTEGER DEFAULT 0,
		generation_time_ms INTEGER DEFAULT 0,
		cost_estimate      REAL DEFAULT 0,
		source_image       TEXT DEFAULT '',
		quality            TEXT DEFAULT '',
		style              TEXT DEFAULT '',
		size               TEXT DEFAULT '',
		language           TEXT DEFAULT '',
		voice_id           TEXT DEFAULT '',
		hash               TEXT DEFAULT '',
		deleted            INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_media_items_media_type ON media_items(media_type);
	CREATE INDEX IF NOT EXISTS idx_media_items_created_at ON media_items(created_at);
	CREATE INDEX IF NOT EXISTS idx_media_items_hash ON media_items(hash);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_media_items_hash_unique ON media_items(hash) WHERE hash != '';
	CREATE INDEX IF NOT EXISTS idx_media_items_deleted ON media_items(deleted);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create media registry schema: %w", err)
	}
	if err := repairLegacyMediaTypes(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to repair legacy media types: %w", err)
	}
	if err := cleanupTTSMediaEntries(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to clean up TTS media entries: %w", err)
	}

	return db, nil
}

func repairLegacyMediaTypes(db *sql.DB) error {
	// Always run: fix media_type for items where the file extension doesn't match the
	// stored media_type (e.g. a .pdf stored as "image").  The UPDATE is idempotent
	// (WHERE clause skips items that already have the correct type).

	repairs := []struct {
		mediaType string
		patterns  []string
	}{
		{mediaType: "document", patterns: []string{".pdf", ".doc", ".docx", ".txt", ".md", ".rtf", ".odt", ".csv", ".json", ".yaml", ".yml", ".xls", ".xlsx", ".ppt", ".pptx"}},
		{mediaType: "audio", patterns: []string{".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac"}},
		{mediaType: "video", patterns: []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".ts", ".m2ts"}},
	}

	for _, repair := range repairs {
		var clauses []string
		var args []interface{}
		for _, ext := range repair.patterns {
			clauses = append(clauses, "LOWER(filename) LIKE ?")
			args = append(args, "%"+ext)
			clauses = append(clauses, "LOWER(file_path) LIKE ?")
			args = append(args, "%"+ext)
		}
		query := "UPDATE media_items SET media_type = ?, updated_at = CURRENT_TIMESTAMP WHERE deleted = 0 AND media_type = 'image' AND (" + strings.Join(clauses, " OR ") + ")"
		args = append([]interface{}{repair.mediaType}, args...)
		if _, err := db.Exec(query, args...); err != nil {
			return err
		}
	}

	return nil
}

// cleanupTTSMediaEntries removes TTS entries from the media registry since TTS files
// are ephemeral and managed by the TTS cache cleanup, not the media view.
func cleanupTTSMediaEntries(db *sql.DB) error {
	res, err := db.Exec("UPDATE media_items SET deleted = 1, updated_at = CURRENT_TIMESTAMP WHERE media_type = 'tts' AND deleted = 0")
	if err != nil {
		return fmt.Errorf("failed to soft-delete TTS media entries: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		_, _ = db.Exec("DELETE FROM media_items WHERE media_type = 'tts' AND deleted = 1")
	}
	return nil
}

// RegisterMedia inserts a new media item. Returns the ID. Uses ON CONFLICT for hash deduplication.
func RegisterMedia(db *sql.DB, item MediaItem) (int64, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("media registry DB not initialized")
	}

	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return 0, false, fmt.Errorf("failed to marshal tags: %w", err)
	}
	if item.Tags == nil {
		tagsJSON = []byte("[]")
	}

	// Use ON CONFLICT for hash-based deduplication - more efficient than transaction-based check.
	// Only applies when hash is non-empty (partial deduplication for items without hash).
	var res sql.Result
	if item.Hash != "" {
		// SQLite's ON CONFLICT(col) upsert syntax does not support partial unique indexes
		// (WHERE clause), so we use an explicit check-then-insert instead.
		var existingID int64
		checkErr := db.QueryRow("SELECT id FROM media_items WHERE hash = ? AND deleted = 0 LIMIT 1", item.Hash).Scan(&existingID)
		if checkErr == nil {
			// Already registered — update file_path if it changed and return as skipped.
			_, _ = db.Exec("UPDATE media_items SET updated_at = datetime('now'), file_path = COALESCE(NULLIF(?, ''), file_path) WHERE id = ?", item.FilePath, existingID)
			return existingID, true, nil
		}
		// Not found — fall through to the plain INSERT below.
		res, err = db.Exec(`INSERT INTO media_items
			(media_type, source_tool, filename, file_path, web_path, file_size, format, provider, model,
			 prompt, description, tags, duration_ms, generation_time_ms, cost_estimate, source_image,
			 quality, style, size, language, voice_id, hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.MediaType, item.SourceTool, item.Filename, item.FilePath, item.WebPath, item.FileSize,
			item.Format, item.Provider, item.Model, item.Prompt, item.Description, string(tagsJSON),
			item.DurationMs, item.GenerationTimeMs, item.CostEstimate, item.SourceImage,
			item.Quality, item.Style, item.Size, item.Language, item.VoiceID, item.Hash,
		)
	} else {
		// Fall back to regular insert for items without hash (no deduplication)
		res, err = db.Exec(`INSERT INTO media_items
			(media_type, source_tool, filename, file_path, web_path, file_size, format, provider, model,
			 prompt, description, tags, duration_ms, generation_time_ms, cost_estimate, source_image,
			 quality, style, size, language, voice_id, hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.MediaType, item.SourceTool, item.Filename, item.FilePath, item.WebPath, item.FileSize,
			item.Format, item.Provider, item.Model, item.Prompt, item.Description, string(tagsJSON),
			item.DurationMs, item.GenerationTimeMs, item.CostEstimate, item.SourceImage,
			item.Quality, item.Style, item.Size, item.Language, item.VoiceID, item.Hash,
		)
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to insert media item: %w", err)
	}
	id, _ := res.LastInsertId()
	// For the hash path the early return handles the "already exists" case, so
	// we never reach this point with a duplicate — id is always the new row ID.
	return id, false, nil
}

func inferMediaType(filename, filePath string) string {
	name := strings.TrimSpace(filename)
	if name == "" {
		name = strings.TrimSpace(filePath)
	}
	if name == "" {
		return "image"
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf", ".doc", ".docx", ".txt", ".md", ".rtf", ".odt", ".csv", ".json", ".yaml", ".yml", ".xls", ".xlsx", ".ppt", ".pptx":
		return "document"
	case ".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac":
		return "audio"
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".ts", ".m2ts":
		return "video"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	default:
		return "image"
	}
}

// SearchMedia searches media items by query string across description, prompt, tags, and filename.
// Note: Dynamic WHERE clause building using string concatenation. While safe from SQL injection
// (all values are parameterized via args slice), this pattern requires careful maintenance:
// conditions and args must stay in sync when adding new filter criteria.
func SearchMedia(db *sql.DB, query, mediaType string, tags []string, limit, offset int) ([]MediaItem, int, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("media registry DB not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}
	conditions = append(conditions, "deleted = 0")

	if query != "" {
		// Split multi-word queries so each word is matched independently (OR per field, AND across words)
		words := strings.Fields(query)
		for _, word := range words {
			w := "%" + word + "%"
			conditions = append(conditions, "(description LIKE ? OR prompt LIKE ? OR tags LIKE ? OR filename LIKE ?)")
			args = append(args, w, w, w, w)
		}
	}
	if mediaType != "" {
		// Map logical UI type groups to the actual media_type values stored in the DB.
		// "audio"    covers manual audio (send_audio), TTS output, and generated music.
		// "image"    covers generated images — kept as single type for compatibility.
		// "document" covers manually sent documents.
		switch mediaType {
		case "audio":
			conditions = append(conditions, "media_type IN ('audio', 'tts', 'music')")
		default:
			conditions = append(conditions, "media_type = ?")
			args = append(args, mediaType)
		}
	}
	for _, t := range tags {
		// Exact JSON array element matching to avoid partial matches
		// (e.g. searching for tag "sun" should not match "sunset").
		// Tags are stored as JSON arrays like ["tag1", "tag2"].
		patterns := []string{
			`["` + t + `"]`, // single element
			`["` + t + `",`, // first element
			`,"` + t + `"]`, // last element
			`,"` + t + `",`, // middle element
		}
		var clauses []string
		for _, p := range patterns {
			clauses = append(clauses, "tags LIKE ?")
			args = append(args, "%"+p+"%")
		}
		conditions = append(conditions, "("+strings.Join(clauses, " OR ")+")")
	}

	where := strings.Join(conditions, " AND ")

	// Count total
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := db.QueryRow("SELECT COUNT(*) FROM media_items WHERE "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count media items: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := db.Query("SELECT id, created_at, updated_at, media_type, source_tool, filename, file_path, web_path, file_size, format, provider, model, prompt, description, tags, duration_ms, generation_time_ms, cost_estimate, source_image, quality, style, size, language, voice_id, hash FROM media_items WHERE "+where+" ORDER BY created_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search media items: %w", err)
	}
	defer rows.Close()

	var items []MediaItem
	for rows.Next() {
		var m MediaItem
		var tagsStr string
		if err := rows.Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.MediaType, &m.SourceTool, &m.Filename, &m.FilePath, &m.WebPath, &m.FileSize, &m.Format, &m.Provider, &m.Model, &m.Prompt, &m.Description, &tagsStr, &m.DurationMs, &m.GenerationTimeMs, &m.CostEstimate, &m.SourceImage, &m.Quality, &m.Style, &m.Size, &m.Language, &m.VoiceID, &m.Hash); err != nil {
			return nil, 0, fmt.Errorf("failed to scan media item: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsStr), &m.Tags); err != nil {
			m.Tags = []string{}
		}
		if m.Tags == nil {
			m.Tags = []string{}
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating media items: %w", err)
	}
	return items, total, nil
}

// GetMedia retrieves a single media item by ID.
func GetMedia(db *sql.DB, id int64) (*MediaItem, error) {
	if db == nil {
		return nil, fmt.Errorf("media registry DB not initialized")
	}
	var m MediaItem
	var tagsStr string
	err := db.QueryRow("SELECT id, created_at, updated_at, media_type, source_tool, filename, file_path, web_path, file_size, format, provider, model, prompt, description, tags, duration_ms, generation_time_ms, cost_estimate, source_image, quality, style, size, language, voice_id, hash FROM media_items WHERE id = ? AND deleted = 0", id).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.MediaType, &m.SourceTool, &m.Filename, &m.FilePath, &m.WebPath, &m.FileSize, &m.Format, &m.Provider, &m.Model, &m.Prompt, &m.Description, &tagsStr, &m.DurationMs, &m.GenerationTimeMs, &m.CostEstimate, &m.SourceImage, &m.Quality, &m.Style, &m.Size, &m.Language, &m.VoiceID, &m.Hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("media item %d not found", id)
		}
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsStr), &m.Tags); err != nil {
		m.Tags = []string{}
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
	return &m, nil
}

// ListMedia returns media items filtered by type.
func ListMedia(db *sql.DB, mediaType string, limit, offset int) ([]MediaItem, int, error) {
	return SearchMedia(db, "", mediaType, nil, limit, offset)
}

// UpdateMedia updates description and/or tags for a media item.
func UpdateMedia(db *sql.DB, id int64, description string, tags []string) error {
	if db == nil {
		return fmt.Errorf("media registry DB not initialized")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	if tags != nil {
		tagsJSON, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		_, err = db.Exec("UPDATE media_items SET description = ?, tags = ?, updated_at = ? WHERE id = ? AND deleted = 0", description, string(tagsJSON), now, id)
		return err
	}
	_, err := db.Exec("UPDATE media_items SET description = ?, updated_at = ? WHERE id = ? AND deleted = 0", description, now, id)
	return err
}

// TagMedia modifies tags for a media item. mode: "add", "remove", or "set".
func TagMedia(db *sql.DB, id int64, newTags []string, mode string) error {
	if db == nil {
		return fmt.Errorf("media registry DB not initialized")
	}

	item, err := GetMedia(db, id)
	if err != nil {
		return err
	}

	var resultTags []string
	switch mode {
	case "set":
		resultTags = newTags
	case "remove":
		removeSet := make(map[string]bool)
		for _, t := range newTags {
			removeSet[t] = true
		}
		for _, t := range item.Tags {
			if !removeSet[t] {
				resultTags = append(resultTags, t)
			}
		}
	default: // "add"
		existing := make(map[string]bool)
		for _, t := range item.Tags {
			existing[t] = true
		}
		resultTags = append(resultTags, item.Tags...)
		for _, t := range newTags {
			if !existing[t] {
				resultTags = append(resultTags, t)
			}
		}
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	tagsJSON, err := json.Marshal(resultTags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}
	_, err = db.Exec("UPDATE media_items SET tags = ?, updated_at = ? WHERE id = ? AND deleted = 0", string(tagsJSON), now, id)
	return err
}

// DeleteMedia performs a soft-delete on a media item.
func DeleteMedia(db *sql.DB, id int64) error {
	if db == nil {
		return fmt.Errorf("media registry DB not initialized")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := db.Exec("UPDATE media_items SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0", now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("media item %d not found or already deleted", id)
	}
	return nil
}

// MediaStats returns aggregate statistics about the media registry.
func MediaStats(db *sql.DB) (map[string]interface{}, error) {
	if db == nil {
		return nil, fmt.Errorf("media registry DB not initialized")
	}

	stats := map[string]interface{}{}

	// Counts by type
	rows, err := db.Query("SELECT media_type, COUNT(*), COALESCE(SUM(file_size), 0) FROM media_items WHERE deleted = 0 GROUP BY media_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	typeCounts := map[string]interface{}{}
	var totalCount int64
	var totalSize int64
	for rows.Next() {
		var mt string
		var cnt, sz int64
		if err := rows.Scan(&mt, &cnt, &sz); err != nil {
			return nil, fmt.Errorf("failed to scan media stats: %w", err)
		}
		typeCounts[mt] = map[string]interface{}{"count": cnt, "size_bytes": sz}
		totalCount += cnt
		totalSize += sz
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media stats: %w", err)
	}
	stats["by_type"] = typeCounts
	stats["total_count"] = totalCount
	stats["total_size_bytes"] = totalSize

	return stats, nil
}

// DispatchMediaRegistry handles tool calls for the media_registry action.
// workspaceDir is used to validate that file_path stays inside the workspace.
func DispatchMediaRegistry(db *sql.DB, workspaceDir, operation, query, mediaType, description string, tags []string, tagMode string, id int64, limit, offset int, filename, filePath, webPath string) string {
	switch operation {
	case "register":
		// Security: validate file_path stays inside the workspace
		if filePath != "" && workspaceDir != "" {
			resolved, err := secureResolve(workspaceDir, filePath)
			if err != nil {
				return fmt.Sprintf(`{"status":"error","message":%q}`, "invalid file_path: "+err.Error())
			}
			filePath = resolved
		}
		item := MediaItem{
			MediaType:   mediaType,
			SourceTool:  "manual",
			Description: description,
			Filename:    filename,
			FilePath:    filePath,
			WebPath:     webPath,
		}
		if item.MediaType == "" {
			item.MediaType = inferMediaType(item.Filename, item.FilePath)
		}
		if len(tags) > 0 {
			item.Tags = tags
		}
		newID, dup, err := RegisterMedia(db, item)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		if dup {
			return fmt.Sprintf(`{"status":"duplicate","id":%d,"message":"Entry already exists."}`, newID)
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"Media item registered."}`, newID)

	case "search":
		items, total, err := SearchMedia(db, query, mediaType, tags, limit, offset)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(items)
		return fmt.Sprintf(`{"status":"success","total":%d,"items":%s}`, total, string(b))

	case "get":
		if id <= 0 {
			return `{"status":"error","message":"'id' is required for get operation."}`
		}
		item, err := GetMedia(db, id)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(item)
		return fmt.Sprintf(`{"status":"success","item":%s}`, string(b))

	case "list":
		items, total, err := ListMedia(db, mediaType, limit, offset)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(items)
		return fmt.Sprintf(`{"status":"success","total":%d,"items":%s}`, total, string(b))

	case "update":
		if id <= 0 {
			return `{"status":"error","message":"'id' is required for update operation."}`
		}
		err := UpdateMedia(db, id, description, tags)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Media item updated."}`

	case "tag":
		if id <= 0 {
			return `{"status":"error","message":"'id' is required for tag operation."}`
		}
		if tagMode == "" {
			tagMode = "add"
		}
		err := TagMedia(db, id, tags, tagMode)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return fmt.Sprintf(`{"status":"success","message":"Tags updated (mode: %s)."}`, tagMode)

	case "delete":
		if id <= 0 {
			return `{"status":"error","message":"'id' is required for delete operation."}`
		}
		err := DeleteMedia(db, id)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"Media item deleted (soft-delete)."}`

	case "stats":
		s, err := MediaStats(db)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(s)
		return fmt.Sprintf(`{"status":"success","stats":%s}`, string(b))

	default:
		return fmt.Sprintf(`{"status":"error","message":"Unknown media_registry operation '%s'. Use: register, search, get, list, update, tag, delete, stats."}`, operation)
	}
}
