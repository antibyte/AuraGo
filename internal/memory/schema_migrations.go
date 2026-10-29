package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"aurago/internal/dbutil"
)

const shortTermSchemaVersion = 17
const legacyMemorySchemaVersion = 12

type memorySchemaStep struct {
	version int
	apply   func(context.Context, *sql.Tx) error
}

// NewSQLiteMemory checks compatibility before any schema write. Version 12 is
// the one-time bridge from historical idempotent initializers; subsequent steps
// commit their data changes and version together.
func NewSQLiteMemory(dbPath string, logger *slog.Logger) (*SQLiteMemory, error) {
	if logger == nil {
		logger = slog.Default()
	}
	previous, err := os.Stat(dbPath)
	if err != nil && !os.IsNotExist(err) && dbPath != ":memory:" {
		return nil, fmt.Errorf("inspect memory database: %w", err)
	}
	existing := previous != nil && previous.Size() > 0
	db, err := dbutil.Open(dbPath, dbutil.WithCorruptionRecovery(logger), dbutil.WithImmediateTransactions())
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return nil, fmt.Errorf("read memory schema version: %w", err)
	}
	if version > shortTermSchemaVersion {
		return nil, fmt.Errorf("memory database schema version %d is newer than supported version %d; use a compatible AuraGo version", version, shortTermSchemaVersion)
	}
	if err := protectMemoryDatabase(db); err != nil {
		return nil, err
	}
	if existing && version < shortTermSchemaVersion {
		if _, err := backupMainDatabase(db, fmt.Sprintf("schema-v%d", shortTermSchemaVersion)); err != nil {
			return nil, fmt.Errorf("back up memory database before migration: %w", err)
		}
	}
	stm := &SQLiteMemory{db: db, logger: logger}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if version < legacyMemorySchemaVersion {
		if err := initializeLegacySQLiteMemory(stm, existing); err != nil {
			return nil, fmt.Errorf("initialize legacy memory schema: %w", err)
		}
		if err := applyMemorySchemaStep(ctx, db, memorySchemaStep{version: legacyMemorySchemaVersion}); err != nil {
			return nil, err
		}
		version = legacyMemorySchemaVersion
	}
	for _, step := range memorySchemaSteps() {
		if step.version <= version {
			continue
		}
		if step.version != version+1 {
			return nil, fmt.Errorf("missing memory schema step after version %d", version)
		}
		if err := applyMemorySchemaStep(ctx, db, step); err != nil {
			return nil, err
		}
		version = step.version
	}
	if err := validateMemorySchema(ctx, db); err != nil {
		return nil, err
	}
	// Startup recovery is intentionally not a migration. Bound the batch; normal
	// maintenance reclaims any remaining orphaned claims without spending retries.
	if _, err := db.ExecContext(ctx, `UPDATE archived_messages
		SET consolidation_status='pending',consolidation_claimed_at=NULL
		WHERE id IN (SELECT id FROM archived_messages WHERE consolidated=0
			AND consolidation_status='in_progress' ORDER BY id LIMIT 1000)`); err != nil {
		return nil, fmt.Errorf("recover memory consolidation claims: %w", err)
	}
	if err := stm.EnsureDefaultSession(); err != nil {
		return nil, fmt.Errorf("ensure default chat session: %w", err)
	}
	success = true
	logger.Info("Initialized SQLite Short-Term Memory", "path", dbPath, "schema_version", version)
	return stm, nil
}

func memorySchemaSteps() []memorySchemaStep {
	return []memorySchemaStep{
		{version: 13, apply: migratePrivacyScrub},
		{version: 14, apply: migrateMaintenanceProgress},
		{version: 15, apply: migrateMemoryArchives},
		{version: 16, apply: func(ctx context.Context, tx *sql.Tx) error {
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pragma_table_info('compressed_tool_outputs') WHERE name='tool_operation')`).Scan(&exists); err != nil || exists {
				return err
			}
			_, err := tx.ExecContext(ctx, `ALTER TABLE compressed_tool_outputs ADD COLUMN tool_operation TEXT NOT NULL DEFAULT ''`)
			return err
		}},
		{version: 17, apply: migrateMemoryExpiry},
	}
}

func applyMemorySchemaStep(ctx context.Context, db *sql.DB, step memorySchemaStep) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin memory schema version %d: %w", step.version, err)
	}
	defer tx.Rollback()
	if step.apply != nil {
		if err := step.apply(ctx, tx); err != nil {
			return fmt.Errorf("migrate memory schema version %d: %w", step.version, err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", step.version)); err != nil {
		return fmt.Errorf("set memory schema version %d: %w", step.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit memory schema version %d: %w", step.version, err)
	}
	return nil
}

func validateMemorySchema(ctx context.Context, db *sql.DB) error {
	for _, table := range []string{
		"messages", "archived_messages", "memory_meta", "core_memory",
		"memory_corrections", "memory_learnings", "memory_learning_revisions",
		"pending_memory_writes", "personality_traits", "activity_turns",
		"audit_events", "plans", "compressed_tool_outputs", "maintenance_runs",
		"agodesk_chat_attachments", "agodesk_knowledge_documents", "memory_record_origins",
	} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("validate memory table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("required memory table %s is missing from versioned database", table)
		}
	}
	return nil
}

// Only the fixed historical DDL is split here; it contains no triggers or
// quoted semicolons. Existing tables may still lack columns used by indexes.
func splitLegacyMemorySchema(schema string) (tables, indexes string) {
	for _, statement := range strings.Split(schema, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if strings.HasPrefix(statement, "CREATE INDEX") || strings.HasPrefix(statement, "CREATE UNIQUE INDEX") {
			indexes += statement + ";\n"
		} else {
			tables += statement + ";\n"
		}
	}
	return tables, indexes
}
