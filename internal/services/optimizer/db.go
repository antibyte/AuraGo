package optimizer

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/dbutil"
	"aurago/internal/prompts"
	"aurago/internal/promptsource"
	promptsembed "aurago/prompts"

	_ "modernc.org/sqlite"
)

var defaultDB *OptimizerDB

type OptimizerDB struct {
	db         *sql.DB
	promptsDir string
}

func InitDB(dbPath string) (*OptimizerDB, error) {
	return InitDBWithPromptsDir(dbPath, "prompts")
}

// InitDBWithPromptsDir initializes the optimizer against the same prompt
// source directory used by the runtime prompt pipeline.
func InitDBWithPromptsDir(dbPath, promptsDir string) (*OptimizerDB, error) {
	if dbPath == "" {
		dbPath = "data/optimization.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create optimizer db directory: %w", err)
	}

	// Use multiple connections: the evaluation cycle reads a rows cursor while issuing
	// additional QueryRow/Exec calls on the same DB. MaxOpenConns(1) causes an immediate
	// self-deadlock. SQLite WAL mode supports concurrent readers safely.
	db, err := dbutil.Open(dbPath, dbutil.WithMaxOpenConns(5))
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS tool_traces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		tool_name TEXT NOT NULL,
		success BOOLEAN NOT NULL,
		recovery_loops INTEGER DEFAULT 0,
		prompt_version TEXT DEFAULT 'v1',
		error_message TEXT,
		execution_time_ms INTEGER
	);

	CREATE TABLE IF NOT EXISTS prompt_overrides (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
                tool_name TEXT NOT NULL,
                mutated_prompt TEXT NOT NULL,
                original_hash TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                active BOOLEAN DEFAULT 1,
                shadow BOOLEAN DEFAULT 0
        );
	CREATE INDEX IF NOT EXISTS idx_traces_tool_version ON tool_traces(tool_name, prompt_version);
	CREATE INDEX IF NOT EXISTS idx_traces_timestamp ON tool_traces(timestamp);
		
		CREATE TABLE IF NOT EXISTS optimizer_metrics (
			key TEXT PRIMARY KEY,
			value INTEGER DEFAULT 0
		);
		INSERT OR IGNORE INTO optimizer_metrics (key, value) VALUES ('rejected_mutations', 0);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Migrate: add shadow column if missing (older DBs created before shadow testing was introduced).
	var hasShadow bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('prompt_overrides') WHERE name='shadow'").Scan(&hasShadow)
	if !hasShadow {
		if _, err := db.Exec("ALTER TABLE prompt_overrides ADD COLUMN shadow BOOLEAN DEFAULT 0"); err != nil {
			return nil, fmt.Errorf("failed to add shadow column: %w", err)
		}
	}
	// Migrate: add original_hash column if missing.
	var hasOriginalHash bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('prompt_overrides') WHERE name='original_hash'").Scan(&hasOriginalHash)
	if !hasOriginalHash {
		if _, err := db.Exec("ALTER TABLE prompt_overrides ADD COLUMN original_hash TEXT"); err != nil {
			return nil, fmt.Errorf("failed to add original_hash column: %w", err)
		}
	}
	_, _ = db.Exec("DROP INDEX IF EXISTS idx_prompt_overrides_unique_tool")

	// Migrate: remove any sqlite_autoindex on prompt_overrides.
	// Older databases had `tool_name TEXT NOT NULL UNIQUE` inline, which creates a
	// sqlite_autoindex that cannot be dropped with DROP INDEX. It prevents inserting
	// multiple rows per tool (e.g. an active row + a shadow row). Rebuild the table
	// without the UNIQUE constraint when this auto-index is detected.
	var autoIndexName string
	_ = db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='prompt_overrides' AND name LIKE 'sqlite_autoindex%' LIMIT 1`,
	).Scan(&autoIndexName)
	if autoIndexName != "" {
		// Rebuild table atomically within a transaction to prevent data loss on crash.
		tx, err := db.Begin()
		if err != nil {
			return nil, fmt.Errorf("begin rebuild transaction: %w", err)
		}
		defer tx.Rollback()

		_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS prompt_overrides_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tool_name TEXT NOT NULL,
			mutated_prompt TEXT NOT NULL,
			original_hash TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			active BOOLEAN DEFAULT 1,
			shadow BOOLEAN DEFAULT 0
		)`)
		if err != nil {
			return nil, fmt.Errorf("create new table: %w", err)
		}

		_, err = tx.Exec(`INSERT OR IGNORE INTO prompt_overrides_new
			SELECT id, tool_name, mutated_prompt, original_hash, created_at, active, shadow
			FROM prompt_overrides`)
		if err != nil {
			return nil, fmt.Errorf("copy data to new table: %w", err)
		}

		_, err = tx.Exec(`DROP TABLE prompt_overrides`)
		if err != nil {
			return nil, fmt.Errorf("drop old table: %w", err)
		}

		_, err = tx.Exec(`ALTER TABLE prompt_overrides_new RENAME TO prompt_overrides`)
		if err != nil {
			return nil, fmt.Errorf("rename table: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit rebuild: %w", err)
		}
	}

	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow)")

	if strings.TrimSpace(promptsDir) == "" {
		promptsDir = "prompts"
	}
	defaultDB = &OptimizerDB{db: db, promptsDir: filepath.Clean(promptsDir)}
	if err := initializeExposureSchema(defaultDB); err != nil {
		_ = db.Close()
		defaultDB = nil
		return nil, fmt.Errorf("initialize prompt exposures: %w", err)
	}
	defaultDB.invalidateStaleOverrides()
	return defaultDB, nil
}

func (o *OptimizerDB) loadCanonicalManual(toolName string) string {
	safeToolName := filepath.Base(strings.TrimSpace(toolName))
	if safeToolName == "." || safeToolName == "" {
		return "(No existing manual found)"
	}
	data, err := os.ReadFile(filepath.Join(o.promptsDir, "tools_manuals", safeToolName+".md"))
	_, _, _, framingErr := promptsource.Split(string(data))
	if err != nil || framingErr != nil {
		data, err = promptsembed.FS.ReadFile(filepath.ToSlash(filepath.Join("tools_manuals", safeToolName+".md")))
	}
	_, _, _, framingErr = promptsource.Split(string(data))
	if err != nil || framingErr != nil {
		return "(No existing manual found)"
	}
	return string(data)
}

func (o *OptimizerDB) invalidateStaleOverrides() {
	rows, err := o.db.Query(`SELECT id, tool_name, original_hash FROM prompt_overrides WHERE active = 1 OR shadow = 1`)
	if err != nil {
		slog.Error("[Optimizer] Failed to check stale overrides", "error", err)
		return
	}
	var stale []int
	for rows.Next() {
		var id int
		var toolName string
		var originalHash sql.NullString
		if err := rows.Scan(&id, &toolName, &originalHash); err != nil {
			continue
		}

		currentManual := o.loadCanonicalManual(toolName)

		hash := sha256.Sum256([]byte(currentManual))
		currentHashStr := hex.EncodeToString(hash[:])

		if !originalHash.Valid || originalHash.String != currentHashStr {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		_, _ = o.db.Exec(`UPDATE prompt_overrides SET active=0,shadow=0 WHERE id=?`, id)
	}
}

func LogToolTrace(toolName string, success bool, recoveryLoops int, promptVersion, errMsg string, execTimeMs int64, operation ...string) error {
	if defaultDB == nil {
		return nil
	}
	return defaultDB.LogToolTrace(toolName, success, recoveryLoops, promptVersion, errMsg, execTimeMs, operation...)
}

func (o *OptimizerDB) LogToolTrace(toolName string, success bool, recoveryLoops int, promptVersion, errMsg string, execTimeMs int64, operation ...string) error {
	if strings.HasPrefix(promptVersion, "exposure:") {
		op := ""
		if len(operation) > 0 {
			op = strings.ToLower(strings.TrimSpace(operation[0]))
		}
		action := strings.ToLower(strings.TrimSpace(toolName))
		switch action {
		case "mcp_call", "execute_skill", "run_tool", "composio_call":
			action = "" // A bridge without its target cannot form a cohort.
		}
		if len(operation) > 1 {
			action = operation[1]
		}
		return o.logExposedToolTrace(toolName, success, recoveryLoops, promptVersion, errMsg, execTimeMs, op, action)
	}
	query := `INSERT INTO tool_traces (tool_name, success, recovery_loops, prompt_version, error_message, execution_time_ms) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := o.db.Exec(query, toolName, success, recoveryLoops, promptVersion, errMsg, execTimeMs)
	if err != nil {
		// Non-critical telemetry write — log as WARN so it doesn't alarm on transient lock contention.
		slog.Warn("Failed to log tool trace", "tool", toolName, "error", err)
	}
	return err
}

func GetActivePromptOverrides() map[string]string {
	if defaultDB == nil {
		return nil
	}
	return defaultDB.GetActivePromptOverrides()
}

func GetToolPromptVersion(toolName string) string {
	if defaultDB == nil {
		return "v1"
	}
	return defaultDB.GetToolPromptVersion(toolName)
}

// GetToolPromptVersion cannot prove exposure. Request construction owns version selection.
func (o *OptimizerDB) GetToolPromptVersion(toolName string) string { return "unobserved" }

func (o *OptimizerDB) GetActivePromptOverrides() map[string]string {
	query := `SELECT tool_name, mutated_prompt, coalesce(original_hash,'') FROM prompt_overrides WHERE active = 1 ORDER BY id`
	rows, err := o.db.Query(query)
	if err != nil {
		slog.Error("Failed to load prompt overrides", "error", err)
		return nil
	}
	defer rows.Close()

	overrides := make(map[string]string)
	for rows.Next() {
		var name, prompt, originalHash string
		if err := rows.Scan(&name, &prompt, &originalHash); err == nil {
			hash := sha256.Sum256([]byte(o.loadCanonicalManual(name)))
			if originalHash != hex.EncodeToString(hash[:]) {
				continue
			}
			if valid, ok := prompts.SanitizeToolGuideOverride(prompt); ok {
				overrides[name] = valid
			}
		}
	}
	return overrides
}

func (o *OptimizerDB) Close() error {
	return o.db.Close()
}

// CleanupOldTraces removes tool_traces older than maxAgeDays.
func CleanupOldTraces(maxAgeDays int) error {
	if defaultDB == nil {
		return nil
	}
	return defaultDB.CleanupOldTraces(maxAgeDays)
}

func (o *OptimizerDB) CleanupOldTraces(maxAgeDays int) error {
	if maxAgeDays <= 0 {
		return fmt.Errorf("trace retention must be positive")
	}
	_, err := o.db.Exec(
		"DELETE FROM tool_traces WHERE timestamp < datetime('now', ? || ' days')",
		fmt.Sprintf("-%d", maxAgeDays),
	)
	if err != nil {
		return fmt.Errorf("cleanup old tool traces: %w", err)
	}
	_, err = o.db.Exec(`DELETE FROM prompt_exposures WHERE timestamp < datetime('now', ? || ' days') AND NOT EXISTS (SELECT 1 FROM tool_traces WHERE exposure_id=prompt_exposures.id)`, fmt.Sprintf("-%d", maxAgeDays))
	if err != nil {
		return fmt.Errorf("cleanup old prompt exposures: %w", err)
	}
	return nil
}
