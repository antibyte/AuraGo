package memory

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	_ "modernc.org/sqlite"
)

type SQLiteMemory struct {
	db     *sql.DB
	logger *slog.Logger
}

// openSQLiteDB opens (or recovers) the SQLite database at dbPath.
// If the file is corrupted (integrity_check fails), it is renamed to .bak and
// a fresh database is created so the agent can continue operating.
func openSQLiteDB(dbPath string, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Quick integrity check — catches "malformed disk image" before any writes.
	var integrityResult string
	if checkErr := db.QueryRow("PRAGMA integrity_check(1)").Scan(&integrityResult); checkErr != nil || integrityResult != "ok" {
		db.Close()
		logger.Error("SQLite database is corrupted, attempting recovery",
			"path", dbPath, "result", integrityResult, "check_error", checkErr)

		// Rename corrupted files so we don't lose them entirely.
		for _, suffix := range []string{"", "-wal", "-shm"} {
			src := dbPath + suffix
			if _, statErr := os.Stat(src); statErr == nil {
				dst := src + ".bak"
				if renErr := os.Rename(src, dst); renErr != nil {
					logger.Warn("Could not rename corrupted DB file", "src", src, "error", renErr)
				} else {
					logger.Warn("Renamed corrupted DB file", "src", src, "dst", dst)
				}
			}
		}

		// Open a fresh database.
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create fresh sqlite db after recovery: %w", err)
		}
		logger.Info("Created fresh SQLite database after corruption recovery", "path", dbPath)
	}

	return db, nil
}

func NewSQLiteMemory(dbPath string, logger *slog.Logger) (*SQLiteMemory, error) {
	db, err := openSQLiteDB(dbPath, logger)
	if err != nil {
		return nil, err
	}

	// Create schema if not exists
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT DEFAULT 'default',
		role TEXT,
		content TEXT,
		is_pinned BOOLEAN DEFAULT 0,
		is_internal BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS archive_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT DEFAULT 'default',
		concept TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS system_notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT,
		is_read BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS memory_meta (
		doc_id TEXT PRIMARY KEY,
		access_count INTEGER DEFAULT 0,
		last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
		protected BOOLEAN DEFAULT 0,
		keep_forever BOOLEAN DEFAULT 0
	);
	
	CREATE TABLE IF NOT EXISTS tool_transitions (
		from_tool TEXT,
		to_tool TEXT,
		count INTEGER DEFAULT 0,
		PRIMARY KEY (from_tool, to_tool)
	);

	CREATE TABLE IF NOT EXISTS interaction_patterns (
		hour_of_day INTEGER,
		day_of_week INTEGER,
		topic TEXT,
		count INTEGER DEFAULT 0,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (hour_of_day, day_of_week, topic)
	);
	
	CREATE TABLE IF NOT EXISTS file_indices (
		file_path TEXT PRIMARY KEY,
		last_modified DATETIME,
		collection TEXT
	);

	CREATE TABLE IF NOT EXISTS core_memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fact TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_profile (
		category   TEXT NOT NULL,
		key        TEXT NOT NULL,
		value      TEXT NOT NULL,
		confidence INTEGER DEFAULT 1,
		source     TEXT DEFAULT 'v2',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (category, key)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session_ts ON messages(session_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_memory_meta_accessed ON memory_meta(last_accessed);
	CREATE INDEX IF NOT EXISTS idx_interaction_patterns_last_seen ON interaction_patterns(last_seen);
	CREATE INDEX IF NOT EXISTS idx_archive_events_session_ts ON archive_events(session_id, timestamp);

	CREATE TABLE IF NOT EXISTS archived_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT DEFAULT 'default',
		role TEXT,
		content TEXT,
		original_timestamp DATETIME,
		archived_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		consolidated BOOLEAN DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_archived_messages_consolidated ON archived_messages(consolidated);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create sqlite schema: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance.
	// SQLite is single-writer, so we cap open connections to 1 to prevent locking errors.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logger.Warn("Failed to set WAL journal mode", "error", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logger.Warn("Failed to set synchronous=NORMAL", "error", err)
	}
	db.SetMaxOpenConns(1)

	// Dynamic migration for is_pinned column
	var hasPinned bool
	err = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('messages') WHERE name='is_pinned'").Scan(&hasPinned)
	if err != nil {
		logger.Error("Failed to check for is_pinned column", "error", err)
	} else if !hasPinned {
		logger.Info("Migrating SQLite: adding is_pinned column to messages table")
		_, err = db.Exec("ALTER TABLE messages ADD COLUMN is_pinned BOOLEAN DEFAULT 0")
		if err != nil {
			logger.Error("Failed to add is_pinned column", "error", err)
		}
	}

	// Dynamic migration for is_internal column
	var hasInternal bool
	err = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('messages') WHERE name='is_internal'").Scan(&hasInternal)
	if err != nil {
		logger.Error("Failed to check for is_internal column", "error", err)
	} else if !hasInternal {
		logger.Info("Migrating SQLite: adding is_internal column to messages table")
		_, err = db.Exec("ALTER TABLE messages ADD COLUMN is_internal BOOLEAN DEFAULT 0")
		if err != nil {
			logger.Error("Failed to add is_internal column", "error", err)
		}
	}

	// Dynamic migration for first_seen column in user_profile
	var hasFirstSeen bool
	err = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('user_profile') WHERE name='first_seen'").Scan(&hasFirstSeen)
	if err != nil {
		logger.Error("Failed to check for first_seen column in user_profile", "error", err)
	} else if !hasFirstSeen {
		logger.Info("Migrating SQLite: adding first_seen column to user_profile table")
		// Use DEFAULT NULL so existing rows are NULL; COALESCE(first_seen, updated_at) handles those gracefully.
		_, err = db.Exec("ALTER TABLE user_profile ADD COLUMN first_seen DATETIME DEFAULT NULL")
		if err != nil {
			logger.Error("Failed to add first_seen column to user_profile", "error", err)
		}
	}

	// Set user_version so backup/restore can detect schema generation.
	// Increment this constant whenever a new column or table is added.
	const shortTermSchemaVersion = 1
	var currentVer int
	_ = db.QueryRow("PRAGMA user_version").Scan(&currentVer)
	if currentVer != shortTermSchemaVersion {
		db.Exec(fmt.Sprintf("PRAGMA user_version = %d", shortTermSchemaVersion))
	}

	logger.Info("Initialized SQLite Short-Term Memory", "path", dbPath)
	stm := &SQLiteMemory{db: db, logger: logger}

	// Always initialize personality tables so dashboard queries work even when
	// PersonalityEngine is disabled. The config option only controls whether
	// the agent actively updates mood/traits, not whether the tables exist.
	if err := stm.InitPersonalityTables(); err != nil {
		logger.Warn("Failed to initialize personality tables", "error", err)
		// Non-fatal: continue without personality features
	}

	return stm, nil
}

func (s *SQLiteMemory) Close() error {
	return s.db.Close()
}

func (s *SQLiteMemory) InsertMessage(sessionID string, role string, content string, pinned bool, isInternal bool) (int64, error) {
	stmt := `INSERT INTO messages(session_id, role, content, is_pinned, is_internal) VALUES(?, ?, ?, ?, ?)`
	res, err := s.db.Exec(stmt, sessionID, role, content, pinned, isInternal)
	if err != nil {
		s.logger.Error("Failed to insert message into memory", "error", err)
		return 0, err
	}
	id, _ := res.LastInsertId()
	s.logger.Debug("Inserted message into memory", "session_id", sessionID, "role", role, "content_len", len(content), "id", id, "pinned", pinned, "internal", isInternal)
	return id, nil
}

func (s *SQLiteMemory) SetMessagePinned(id int64, pinned bool) error {
	stmt := `UPDATE messages SET is_pinned = ? WHERE id = ?`
	_, err := s.db.Exec(stmt, pinned, id)
	if err != nil {
		s.logger.Error("Failed to update message pinning", "id", id, "pinned", pinned, "error", err)
		return err
	}
	return nil
}

func (s *SQLiteMemory) GetRecentMessages(sessionID string, limit int) ([]openai.ChatCompletionMessage, error) {
	// We want the most recent N messages, but we need them in chronological order.
	// So we order by timestamp DESC, limit N, and then reverse the result in Go,
	// or use a subquery. We'll use a subquery for simplicity.
	query := `
	SELECT role, content FROM (
		SELECT role, content, timestamp 
		FROM messages 
		WHERE session_id = ? 
		ORDER BY timestamp DESC 
		LIMIT ?
	) ORDER BY timestamp ASC;`

	rows, err := s.db.Query(query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent messages: %w", err)
	}
	defer rows.Close()

	var messages []openai.ChatCompletionMessage
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return messages, nil
}

// GetHoursSinceLastUserMessage calculates how many hours have passed since the user last sent a message in this session.
// If no user message is found, returns 0.
func (s *SQLiteMemory) GetHoursSinceLastUserMessage(sessionID string) (float64, error) {
	query := `
	SELECT timestamp 
	FROM messages 
	WHERE session_id = ? AND role = 'user' AND is_internal = 0
	ORDER BY timestamp DESC 
	LIMIT 1;`

	var timestampStr string
	err := s.db.QueryRow(query, sessionID).Scan(&timestampStr)

	if err == sql.ErrNoRows {
		// New session or no user messages yet -> no loneliness
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to query last user message: %w", err)
	}

	// timestamp is stored as 'YYYY-MM-DD HH:MM:SS' by CURRENT_TIMESTAMP
	// In SQLite CURRENT_TIMESTAMP is UTC
	lastInteraction, err := time.Parse("2006-01-02 15:04:05", timestampStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse timestamp '%s': %w", timestampStr, err)
	}

	// Assuming the server executes this, time.Now().UTC() matches CURRENT_TIMESTAMP
	hours := time.Now().UTC().Sub(lastInteraction).Hours()
	if hours < 0 {
		hours = 0
	}
	return hours, nil
}

// DeleteOldMessages archives messages to archived_messages before removing them.
// Keeps only the most recent `keepN` messages for a given session.
func (s *SQLiteMemory) DeleteOldMessages(sessionID string, keepN int) error {
	// First find the ID of the oldest message we want to KEEP
	query := `
	SELECT id FROM messages 
	WHERE session_id = ? 
	ORDER BY timestamp DESC, id DESC 
	LIMIT 1 OFFSET ?;`

	var oldestKeepID int
	err := s.db.QueryRow(query, sessionID, keepN-1).Scan(&oldestKeepID)

	if err == sql.ErrNoRows {
		// Fewer than keepN messages exist, nothing to delete
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to find cutoff ID for deletion: %w", err)
	}

	// Archive before deleting
	archiveQuery := `
	INSERT INTO archived_messages (session_id, role, content, original_timestamp)
	SELECT session_id, role, content, timestamp
	FROM messages
	WHERE session_id = ? AND id < ? AND role IN ('user', 'assistant')
	ORDER BY timestamp ASC`
	archRes, _ := s.db.Exec(archiveQuery, sessionID, oldestKeepID)
	archived, _ := archRes.RowsAffected()

	// Delete everything older than the cutoff
	delQuery := `DELETE FROM messages WHERE session_id = ? AND id < ?`
	res, err := s.db.Exec(delQuery, sessionID, oldestKeepID)
	if err != nil {
		return fmt.Errorf("failed to delete old messages: %w", err)
	}

	rows, _ := res.RowsAffected()
	s.logger.Info("Cleaned up SQLite short-term memory", "session_id", sessionID, "deleted_rows", rows, "archived", archived)
	return nil
}

// GetUnconsolidatedMessages returns archived messages that haven't been processed by consolidation yet.
func (s *SQLiteMemory) GetUnconsolidatedMessages(limit int) ([]ArchivedMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, original_timestamp
		FROM archived_messages
		WHERE consolidated = 0
		ORDER BY original_timestamp ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query unconsolidated: %w", err)
	}
	defer rows.Close()

	var msgs []ArchivedMessage
	for rows.Next() {
		var m ArchivedMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("scan unconsolidated: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkConsolidated marks a batch of archived messages as consolidated.
func (s *SQLiteMemory) MarkConsolidated(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("UPDATE archived_messages SET consolidated = 1 WHERE id IN (%s)",
		strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, args...)
	return err
}

// CleanOldArchivedMessages removes archived messages older than the given number of days.
func (s *SQLiteMemory) CleanOldArchivedMessages(days int) (int64, error) {
	res, err := s.db.Exec(
		"DELETE FROM archived_messages WHERE archived_at < datetime('now', ?)",
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLiteMemory) DeleteMessagesByID(sessionID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Build parameterized placeholders: "?, ?, ?"
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, sessionID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf("DELETE FROM messages WHERE session_id = ? AND id IN (%s)", strings.Join(placeholders, ","))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete messages by ID: %w", err)
	}
	rows, _ := res.RowsAffected()
	s.logger.Info("Deleted specific messages from SQLite", "session_id", sessionID, "deleted_rows", rows)
	return nil
}

// Clear removes all messages for a given session.
func (s *SQLiteMemory) Clear(sessionID string) error {
	delQuery := `DELETE FROM messages WHERE session_id = ?`
	res, err := s.db.Exec(delQuery, sessionID)
	if err != nil {
		return fmt.Errorf("failed to clear messages: %w", err)
	}

	rows, _ := res.RowsAffected()
	s.logger.Info("Cleared SQLite short-term memory", "session_id", sessionID, "deleted_rows", rows)
	return nil
}

// LogArchiveEvent records that a concept was saved to long-term memory.
func (s *SQLiteMemory) LogArchiveEvent(sessionID, concept string) error {
	stmt := `INSERT INTO archive_events(session_id, concept) VALUES(?, ?)`
	_, err := s.db.Exec(stmt, sessionID, concept)
	if err != nil {
		s.logger.Error("Failed to log archive event", "error", err)
		return err
	}
	return nil
}

// GetRecentArchiveEvents returns concepts archived within the last N hours.
func (s *SQLiteMemory) GetRecentArchiveEvents(hours int) ([]string, error) {
	query := `
	SELECT concept FROM archive_events 
	WHERE timestamp >= datetime('now', ?) 
	ORDER BY timestamp ASC;`

	timeMod := fmt.Sprintf("-%d hours", hours)
	rows, err := s.db.Query(query, timeMod)
	if err != nil {
		return nil, fmt.Errorf("failed to query archive events: %w", err)
	}
	defer rows.Close()

	var concepts []string
	for rows.Next() {
		var concept string
		if err := rows.Scan(&concept); err == nil {
			concepts = append(concepts, concept)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return concepts, nil
}

// AddNotification stores a new system notification (e.g. Morning Briefing).
func (s *SQLiteMemory) AddNotification(content string) error {
	stmt := `INSERT INTO system_notifications(content) VALUES(?)`
	_, err := s.db.Exec(stmt, content)
	if err != nil {
		s.logger.Error("Failed to store system notification", "error", err)
		return err
	}
	return nil
}

// GetUnreadNotifications returns all unread notifications.
func (s *SQLiteMemory) GetUnreadNotifications() ([]string, error) {
	query := `SELECT content FROM system_notifications WHERE is_read = 0 ORDER BY timestamp ASC;`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread notifications: %w", err)
	}
	defer rows.Close()

	var notes []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			notes = append(notes, content)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return notes, nil
}

// MarkNotificationsRead marks all system notifications as read.
func (s *SQLiteMemory) MarkNotificationsRead() error {
	stmt := `UPDATE system_notifications SET is_read = 1 WHERE is_read = 0`
	_, err := s.db.Exec(stmt)
	return err
}

// ArchivedMessage represents a message that was archived before deletion from STM.
type ArchivedMessage struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	Timestamp string
}

// MemoryMeta models the tracking metadata for a VectorDB chunk.
type MemoryMeta struct {
	DocID        string
	AccessCount  int
	LastAccessed string
	Protected    bool
	KeepForever  bool
}

// UpsertMemoryMeta creates or resets a VectorDB chunk's metadata.
func (s *SQLiteMemory) UpsertMemoryMeta(docID string) error {
	stmt := `
	INSERT INTO memory_meta (doc_id, access_count, last_accessed)
	VALUES (?, 0, CURRENT_TIMESTAMP)
	ON CONFLICT(doc_id) DO UPDATE SET last_accessed=CURRENT_TIMESTAMP;`

	_, err := s.db.Exec(stmt, docID)
	return err
}

// UpdateMemoryAccess increments the access count and touches the last_accessed timestamp.
func (s *SQLiteMemory) UpdateMemoryAccess(docID string) error {
	stmt := `
	UPDATE memory_meta 
	SET access_count = access_count + 1, last_accessed = CURRENT_TIMESTAMP 
	WHERE doc_id = ?;`

	_, err := s.db.Exec(stmt, docID)
	return err
}

// DeleteMemoryMeta removes tracking for a vector DB chunk.
func (s *SQLiteMemory) DeleteMemoryMeta(docID string) error {
	stmt := `DELETE FROM memory_meta WHERE doc_id = ?;`
	_, err := s.db.Exec(stmt, docID)
	return err
}

// GetAllMemoryMeta retrieves the metadata for all chunks to calculate forgetting priorities.
func (s *SQLiteMemory) GetAllMemoryMeta() ([]MemoryMeta, error) {
	query := `SELECT doc_id, access_count, last_accessed, protected, keep_forever FROM memory_meta;`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []MemoryMeta
	for rows.Next() {
		var m MemoryMeta
		if err := rows.Scan(&m.DocID, &m.AccessCount, &m.LastAccessed, &m.Protected, &m.KeepForever); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return metas, nil
}

// RecordToolTransition increments the count for a transition from one tool to another.
func (s *SQLiteMemory) RecordToolTransition(from, to string) error {
	if from == "" || to == "" {
		return nil
	}
	stmt := `
	INSERT INTO tool_transitions (from_tool, to_tool, count)
	VALUES (?, ?, 1)
	ON CONFLICT(from_tool, to_tool) DO UPDATE SET count = count + 1;`
	_, err := s.db.Exec(stmt, from, to)
	return err
}

// GetTopTransition finds the tool most likely to follow the given tool.
func (s *SQLiteMemory) GetTopTransition(from string) (string, error) {
	if from == "" {
		return "", nil
	}
	query := `SELECT to_tool FROM tool_transitions WHERE from_tool = ? ORDER BY count DESC LIMIT 1;`
	var to string
	err := s.db.QueryRow(query, from).Scan(&to)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return to, err
}

// GetToolUsageCount returns the total transition count involving a tool (both as source and target).
func (s *SQLiteMemory) GetToolUsageCount(toolName string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(count), 0) FROM tool_transitions WHERE from_tool = ? OR to_tool = ?`,
		toolName, toolName,
	).Scan(&count)
	return count, err
}

// ── Temporal Memory (Phase A) ──────────────────────────────────────────────

// RecordInteraction logs a topic at the current hour/weekday for pattern detection.
// Topics are kept short (≤120 chars) as coarse-grained keys, not full messages.
func (s *SQLiteMemory) RecordInteraction(topic string) error {
	if topic == "" {
		return nil
	}
	if len(topic) > 120 {
		topic = topic[:120]
	}
	stmt := `
	INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
	VALUES (CAST(strftime('%H','now','localtime') AS INTEGER),
	        CAST(strftime('%w','now','localtime') AS INTEGER),
	        ?, 1, CURRENT_TIMESTAMP)
	ON CONFLICT(hour_of_day, day_of_week, topic)
	DO UPDATE SET count = count + 1, last_seen = CURRENT_TIMESTAMP;`
	_, err := s.db.Exec(stmt, topic)
	if err != nil {
		s.logger.Error("Failed to record interaction pattern", "topic", topic, "error", err)
	}
	return err
}

// GetTopPatterns returns the most common topics for the given hour and weekday,
// ordered by frequency. Useful for proactive memory pre-loading.
func (s *SQLiteMemory) GetTopPatterns(hour, weekday, limit int) ([]string, error) {
	query := `
	SELECT topic FROM interaction_patterns
	WHERE hour_of_day = ? AND day_of_week = ?
	ORDER BY count DESC
	LIMIT ?;`
	rows, err := s.db.Query(query, hour, weekday, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query interaction patterns: %w", err)
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			topics = append(topics, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return topics, nil
}

// CleanOldPatterns removes interaction patterns older than the given number of days.
func (s *SQLiteMemory) CleanOldPatterns(olderThanDays int) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM interaction_patterns WHERE last_seen < datetime('now', '-' || ? || ' days');`, olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CleanOldArchiveEvents removes archive events older than the given number of days.
func (s *SQLiteMemory) CleanOldArchiveEvents(olderThanDays int) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM archive_events WHERE timestamp < datetime('now', '-' || ? || ' days');`, olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PredictNextQuery combines tool-transition history with temporal interaction patterns
// to predict what topics the user is likely interested in right now.
// Returns up to `limit` predicted topic strings.
func (s *SQLiteMemory) PredictNextQuery(lastTool string, hour, weekday, limit int) ([]string, error) {
	var predictions []string
	seen := make(map[string]bool)

	// 1. Temporal patterns: what does the user usually do at this time?
	topics, err := s.GetTopPatterns(hour, weekday, limit)
	if err == nil {
		for _, t := range topics {
			if !seen[t] {
				predictions = append(predictions, t)
				seen[t] = true
			}
		}
	}

	// 2. Tool transition: if we know which tool was used last, predict the next tool context
	if lastTool != "" {
		nextTool, err := s.GetTopTransition(lastTool)
		if err == nil && nextTool != "" && !seen[nextTool] {
			predictions = append(predictions, nextTool)
			seen[nextTool] = true
		}
	}

	if len(predictions) > limit {
		predictions = predictions[:limit]
	}
	return predictions, nil
}

// GetFileIndex returns the last modified time for a given file path.
func (s *SQLiteMemory) GetFileIndex(path string) (time.Time, error) {
	var lastMod time.Time
	err := s.db.QueryRow("SELECT last_modified FROM file_indices WHERE file_path = ?", path).Scan(&lastMod)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return lastMod, err
}

// UpdateFileIndex updates the last modified time for a given file path.
func (s *SQLiteMemory) UpdateFileIndex(path, collection string, modTime time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO file_indices (file_path, collection, last_modified)
		VALUES (?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			last_modified = excluded.last_modified,
			collection = excluded.collection
	`, path, collection, modTime)
	return err
}

// ── Core Memory (SQLite) ──────────────────────────────────────────────────────

// GetMessageCount returns the total number of chat messages.
