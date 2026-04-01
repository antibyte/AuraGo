package memory

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteMemory struct {
	db     *sql.DB
	logger *slog.Logger

	// personality hot-path cache (1-second TTL, invalidated on every write)
	personalityCacheMu sync.RWMutex
	traitsCache        PersonalityTraits
	traitsCacheAt      time.Time
	moodCache          Mood
	moodCacheAt        time.Time
}

// openSQLiteDB opens (or recovers) the SQLite database at dbPath.
// If the file is corrupted (integrity_check fails), it is renamed to .bak and
// a fresh database is created so the agent can continue operating.
func openSQLiteDB(dbPath string, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}
	var integrityResult string
	if checkErr := db.QueryRow("PRAGMA integrity_check(1)").Scan(&integrityResult); checkErr != nil || integrityResult != "ok" {
		db.Close()
		logger.Error("SQLite database is corrupted, attempting recovery",
			"path", dbPath, "result", integrityResult, "check_error", checkErr)

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
		last_event_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		extraction_confidence REAL DEFAULT 0.75,
		verification_status TEXT DEFAULT 'unverified',
		source_type TEXT DEFAULT 'system',
		source_reliability REAL DEFAULT 0.70,
		useful_count INTEGER DEFAULT 0,
		useless_count INTEGER DEFAULT 0,
		last_effectiveness_at DATETIME,
		protected BOOLEAN DEFAULT 0,
		keep_forever BOOLEAN DEFAULT 0
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

	CREATE TABLE IF NOT EXISTS file_embedding_docs (
		file_path TEXT NOT NULL,
		doc_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (file_path, doc_id)
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
	CREATE INDEX IF NOT EXISTS idx_file_embedding_docs_path ON file_embedding_docs(file_path);

	CREATE TABLE IF NOT EXISTS archived_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT DEFAULT 'default',
		role TEXT,
		content TEXT,
		original_timestamp DATETIME,
		archived_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		consolidated BOOLEAN DEFAULT 0,
		consolidation_status TEXT DEFAULT 'pending',
		consolidation_retries INTEGER DEFAULT 0,
		consolidation_last_error TEXT DEFAULT '',
		next_retry_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_archived_messages_consolidated ON archived_messages(consolidated);

	CREATE TABLE IF NOT EXISTS tool_usage_adaptive (
		tool_name TEXT PRIMARY KEY,
		total_count INTEGER DEFAULT 0,
		success_count INTEGER DEFAULT 0,
		last_used DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS memory_usage_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		memory_id TEXT NOT NULL,
		memory_type TEXT NOT NULL,
		session_id TEXT DEFAULT 'default',
		used_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		context_relevance REAL DEFAULT 0,
		was_cited BOOLEAN DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_memory_usage_log_memory ON memory_usage_log(memory_id, used_at DESC);
	CREATE INDEX IF NOT EXISTS idx_memory_usage_log_session ON memory_usage_log(session_id, used_at DESC);

	CREATE TABLE IF NOT EXISTS agent_telemetry (
		event_type TEXT NOT NULL,
		event_name TEXT NOT NULL,
		count INTEGER DEFAULT 0,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (event_type, event_name)
	);

	CREATE TABLE IF NOT EXISTS agent_telemetry_scoped (
		provider_type TEXT NOT NULL,
		model TEXT NOT NULL,
		event_type TEXT NOT NULL,
		event_name TEXT NOT NULL,
		count INTEGER DEFAULT 0,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (provider_type, model, event_type, event_name)
	);

	CREATE TABLE IF NOT EXISTS tool_transitions (
		from_tool TEXT,
		to_tool TEXT,
		count INTEGER DEFAULT 0,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (from_tool, to_tool)
	);`

	conflictSchema := `
	CREATE TABLE IF NOT EXISTS memory_conflicts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		doc_id_left TEXT NOT NULL,
		doc_id_right TEXT NOT NULL,
		conflict_key TEXT NOT NULL,
		left_value TEXT DEFAULT '',
		right_value TEXT DEFAULT '',
		reason TEXT DEFAULT '',
		status TEXT DEFAULT 'open',
		detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		resolved_at DATETIME DEFAULT '',
		UNIQUE(doc_id_left, doc_id_right, conflict_key)
	);
	CREATE INDEX IF NOT EXISTS idx_memory_conflicts_status ON memory_conflicts(status, detected_at DESC);
	CREATE INDEX IF NOT EXISTS idx_memory_conflicts_docs ON memory_conflicts(doc_id_left, doc_id_right);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create sqlite schema: %w", err)
	}
	if _, err := db.Exec(conflictSchema); err != nil {
		return nil, fmt.Errorf("failed to create conflict schema: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logger.Warn("Failed to set WAL journal mode", "error", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logger.Warn("Failed to set synchronous=NORMAL", "error", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		logger.Warn("Failed to enable foreign key constraints", "error", err)
	}
	db.SetMaxOpenConns(1)

	if err := applySQLiteMemoryMigrations(db, logger); err != nil {
		logger.Warn("SQLite memory migrations reported warnings", "error", err)
	}

	logger.Info("Initialized SQLite Short-Term Memory", "path", dbPath)
	stm := &SQLiteMemory{db: db, logger: logger}

	if err := stm.InitPersonalityTables(); err != nil {
		logger.Warn("Failed to initialize personality tables", "error", err)
	}
	if err := stm.InitActivityTables(); err != nil {
		logger.Warn("Failed to initialize activity tables", "error", err)
	}
	if err := stm.InitPlanTables(); err != nil {
		logger.Warn("Failed to initialize plan tables", "error", err)
	}

	return stm, nil
}

func applySQLiteMemoryMigrations(db *sql.DB, logger *slog.Logger) error {
	migrateAddColumn(db, logger, "messages", "is_pinned", "BOOLEAN DEFAULT 0")
	migrateAddColumn(db, logger, "messages", "is_internal", "BOOLEAN DEFAULT 0")
	migrateAddColumn(db, logger, "user_profile", "first_seen", "DATETIME DEFAULT NULL")
	migrateAddColumn(db, logger, "tool_usage_adaptive", "success_count", "INTEGER DEFAULT 0")
	migrateAddColumn(db, logger, "memory_meta", "last_event_at", "DATETIME DEFAULT CURRENT_TIMESTAMP")
	migrateAddColumn(db, logger, "memory_meta", "extraction_confidence", "REAL DEFAULT 0.75")
	migrateAddColumn(db, logger, "memory_meta", "verification_status", "TEXT DEFAULT 'unverified'")
	migrateAddColumn(db, logger, "memory_meta", "source_type", "TEXT DEFAULT 'system'")
	migrateAddColumn(db, logger, "memory_meta", "source_reliability", "REAL DEFAULT 0.70")
	migrateAddColumn(db, logger, "memory_meta", "useful_count", "INTEGER DEFAULT 0")
	migrateAddColumn(db, logger, "memory_meta", "useless_count", "INTEGER DEFAULT 0")
	migrateAddColumn(db, logger, "memory_meta", "last_effectiveness_at", "DATETIME")
	migrateAddColumn(db, logger, "archived_messages", "consolidation_status", "TEXT DEFAULT 'pending'")
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_archived_messages_retry ON archived_messages(consolidation_status, next_retry_at)"); err != nil {
		logger.Warn("Failed to create idx_archived_messages_retry", "error", err)
	}
	migrateAddColumn(db, logger, "archived_messages", "consolidation_retries", "INTEGER DEFAULT 0")
	migrateAddColumn(db, logger, "archived_messages", "consolidation_last_error", "TEXT DEFAULT ''")
	migrateAddColumn(db, logger, "archived_messages", "next_retry_at", "DATETIME DEFAULT CURRENT_TIMESTAMP")

	if _, err := db.Exec("UPDATE archived_messages SET consolidation_status = 'pending' WHERE consolidated = 0 AND (consolidation_status IS NULL OR consolidation_status = '' OR consolidation_status NOT IN ('pending', 'failed', 'done'))"); err != nil {
		logger.Warn("Failed to normalize consolidation_status pending rows", "error", err)
	}
	if _, err := db.Exec("UPDATE archived_messages SET consolidation_status = 'done' WHERE consolidated = 1 AND (consolidation_status IS NULL OR consolidation_status = '' OR consolidation_status != 'done')"); err != nil {
		logger.Warn("Failed to normalize consolidation_status done rows", "error", err)
	}

	migrateAddColumn(db, logger, "tool_transitions", "last_updated", "DATETIME DEFAULT ''")

	const shortTermSchemaVersion = 8
	var currentVer int
	if err := db.QueryRow("PRAGMA user_version").Scan(&currentVer); err != nil {
		logger.Warn("Failed to read schema version", "error", err)
	} else if currentVer != shortTermSchemaVersion {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", shortTermSchemaVersion)); err != nil {
			logger.Warn("Failed to update schema version", "error", err)
		}
	}

	return nil
}

func migrateAddColumn(db *sql.DB, logger *slog.Logger, table, column, definition string) {
	query := fmt.Sprintf("SELECT count(*) > 0 FROM pragma_table_info('%s') WHERE name=?", table)
	var hasColumn bool
	if err := db.QueryRow(query, column).Scan(&hasColumn); err != nil {
		logger.Warn("Failed to check for column", "table", table, "column", column, "error", err)
		return
	}
	if hasColumn {
		return
	}
	logger.Info("Migrating SQLite: adding column", "table", table, "column", column)
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := db.Exec(stmt); err != nil {
		logger.Error("Failed to add column", "table", table, "column", column, "error", err)
	}
}
