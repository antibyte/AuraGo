package optimizer

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var defaultDB *OptimizerDB

type OptimizerDB struct {
	db *sql.DB
}

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
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                active BOOLEAN DEFAULT 1,
                shadow BOOLEAN DEFAULT 0
        );
        CREATE INDEX IF NOT EXISTS idx_prompt_overrides_tool_status ON prompt_overrides(tool_name, active, shadow);
		
		CREATE TABLE IF NOT EXISTS optimizer_metrics (
			key TEXT PRIMARY KEY,
			value INTEGER DEFAULT 0
		);
		INSERT OR IGNORE INTO optimizer_metrics (key, value) VALUES ('rejected_mutations', 0);`

	if _, err := db.Exec(schema); err != nil {
	}

	defaultDB = &OptimizerDB{db: db}
	return defaultDB, nil
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
	// Let's check for shadow prompt first as it's the intended override for tests
	var id int
	err := o.db.QueryRow(`SELECT id FROM prompt_overrides WHERE tool_name = ? AND shadow = 1 AND active = 0 ORDER BY id DESC LIMIT 1`, toolName).Scan(&id)
	if err == nil {
		return fmt.Sprintf("v2-shadow-%d", id)
	}

	// Else check if there's a normal active override
	var count int
	err = o.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE tool_name = ? AND active = 1`, toolName).Scan(&count)
	if err == nil && count > 0 {
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
