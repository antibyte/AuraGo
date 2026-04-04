package optimizer

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	promptsembed "aurago/prompts"

	_ "modernc.org/sqlite"
)

var defaultDB *OptimizerDB

type OptimizerDB struct {
	db *sql.DB
}

type versionCacheEntry struct {
	hasShadow bool
	shadowID  int
	hasActive bool
	expireAt  time.Time
}

var versionCache sync.Map

func InitDB(dbPath string) (*OptimizerDB, error) {
	if dbPath == "" {
		dbPath = "data/optimization.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create optimizer db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
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
        CREATE INDEX IF NOT EXISTS idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow);
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
		// Now create the index that requires the shadow column.
		_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow)")
	}

	defaultDB = &OptimizerDB{db: db}
	defaultDB.invalidateStaleOverrides()
	return defaultDB, nil
}

func (o *OptimizerDB) invalidateStaleOverrides() {
	rows, err := o.db.Query(`SELECT id, tool_name, original_hash FROM prompt_overrides WHERE active = 1 OR shadow = 1`)
	if err != nil {
		slog.Error("[Optimizer] Failed to check stale overrides", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var toolName string
		var originalHash sql.NullString
		if err := rows.Scan(&id, &toolName, &originalHash); err != nil {
			continue
		}

		// Load current manual
		var currentManual string
		safeToolName := filepath.Base(toolName)
		data, err := os.ReadFile("prompts/tools_manuals/" + safeToolName + ".md")
		if err != nil {
			data, err = promptsembed.FS.ReadFile("tools_manuals/" + safeToolName + ".md")
		}
		if err == nil {
			currentManual = string(data)
		} else {
			currentManual = "(No existing manual found)"
		}

		hash := sha256.Sum256([]byte(currentManual))
		currentHashStr := hex.EncodeToString(hash[:])

		if !originalHash.Valid || originalHash.String != currentHashStr {
			_, err := o.db.Exec(`UPDATE prompt_overrides SET active = 0, shadow = 0 WHERE id = ?`, id)
			if err == nil {
				slog.Info("[Optimizer] Invalidated stale prompt override", "tool", toolName, "id", id)
			}
		}
	}
}

func LogToolTrace(toolName string, success bool, recoveryLoops int, promptVersion, errMsg string, execTimeMs int64) error {
	if defaultDB == nil {
		return nil
	}
	return defaultDB.LogToolTrace(toolName, success, recoveryLoops, promptVersion, errMsg, execTimeMs)
}

func (o *OptimizerDB) LogToolTrace(toolName string, success bool, recoveryLoops int, promptVersion, errMsg string, execTimeMs int64) error {
	query := `INSERT INTO tool_traces (tool_name, success, recovery_loops, prompt_version, error_message, execution_time_ms) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := o.db.Exec(query, toolName, success, recoveryLoops, promptVersion, errMsg, execTimeMs)
	if err != nil {
		slog.Error("Failed to log tool trace", "tool", toolName, "error", err)
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

func (o *OptimizerDB) GetToolPromptVersion(toolName string) string {
	now := time.Now()
	var entry versionCacheEntry

	if val, ok := versionCache.Load(toolName); ok {
		cached := val.(versionCacheEntry)
		if now.Before(cached.expireAt) {
			entry = cached
		}
	}

	if entry.expireAt.IsZero() || now.After(entry.expireAt) {
		// Fetch from DB
		var id int
		err := o.db.QueryRow(`SELECT id FROM prompt_overrides WHERE tool_name = ? AND shadow = 1 AND active = 0 ORDER BY id DESC LIMIT 1`, toolName).Scan(&id)
		if err == nil {
			entry.hasShadow = true
			entry.shadowID = id
		} else {
			var count int
			err = o.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE tool_name = ? AND active = 1`, toolName).Scan(&count)
			if err == nil && count > 0 {
				entry.hasActive = true
			}
		}

		entry.expireAt = now.Add(60 * time.Second)
		versionCache.Store(toolName, entry)
	}

	if entry.hasShadow {
		// ~30% chance for shadow test (v2), ~70% for standard (v1)
		if rand.Float64() < 0.3 {
			return fmt.Sprintf("v2-shadow-%d", entry.shadowID)
		}
		return "v1"
	}

	if entry.hasActive {
		return "optim-db"
	}

	return "v1"
}

func (o *OptimizerDB) GetActivePromptOverrides() map[string]string {
	query := `SELECT tool_name, mutated_prompt FROM prompt_overrides WHERE active = 1`
	rows, err := o.db.Query(query)
	if err != nil {
		slog.Error("Failed to load prompt overrides", "error", err)
		return nil
	}
	defer rows.Close()

	overrides := make(map[string]string)
	for rows.Next() {
		var name, prompt string
		if err := rows.Scan(&name, &prompt); err == nil {
			overrides[name] = prompt
		}
	}
	return overrides
}

func (o *OptimizerDB) Close() error {
	return o.db.Close()
}
