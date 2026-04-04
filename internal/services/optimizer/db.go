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
		tool_name TEXT UNIQUE NOT NULL,
		mutated_prompt TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		active BOOLEAN DEFAULT 1
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init optimizer schema: %w", err)
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
