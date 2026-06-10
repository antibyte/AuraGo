package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MaintenancePhaseResults captures deterministic nightly maintenance outcomes.
type MaintenancePhaseResults struct {
	JournalRemoved     int      `json:"journal_removed"`
	NotesArchived      int      `json:"notes_archived"`
	ConsolidationFacts int      `json:"consolidation_facts"`
	CompressedDeleted  int      `json:"compressed_deleted"`
	KGFilesProcessed   int      `json:"kg_files_processed"`
	KGNodesExtracted   int      `json:"kg_nodes_extracted"`
	Errors             []string `json:"errors,omitempty"`
}

// MaintenanceRunRecord is a persisted nightly maintenance run entry.
type MaintenanceRunRecord struct {
	ID           int64                   `json:"id"`
	StartedAt    string                  `json:"started_at"`
	FinishedAt   string                  `json:"finished_at"`
	Status       string                  `json:"status"`
	PhaseResults MaintenancePhaseResults `json:"phase_results"`
}

// InitMaintenanceRunsTable creates the maintenance run ledger table.
func (s *SQLiteMemory) InitMaintenanceRunsTable() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite memory is not initialized")
	}
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS maintenance_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at DATETIME NOT NULL,
			finished_at DATETIME NOT NULL,
			status TEXT NOT NULL,
			phase_results TEXT NOT NULL DEFAULT '{}'
		);
		CREATE INDEX IF NOT EXISTS idx_maintenance_runs_finished ON maintenance_runs(finished_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("create maintenance_runs table: %w", err)
	}
	return nil
}

// InsertMaintenanceRun stores a completed nightly maintenance run.
func (s *SQLiteMemory) InsertMaintenanceRun(startedAt, finishedAt time.Time, status string, results MaintenancePhaseResults) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite memory is not initialized")
	}
	if err := s.InitMaintenanceRunsTable(); err != nil {
		return err
	}
	payload, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("marshal maintenance phase results: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO maintenance_runs (started_at, finished_at, status, phase_results) VALUES (?, ?, ?, ?)`,
		startedAt.UTC().Format(time.RFC3339),
		finishedAt.UTC().Format(time.RFC3339),
		status,
		string(payload),
	)
	if err != nil {
		return fmt.Errorf("insert maintenance run: %w", err)
	}
	return nil
}

// GetLatestMaintenanceRun returns the most recent maintenance run, if any.
func (s *SQLiteMemory) GetLatestMaintenanceRun() (*MaintenanceRunRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite memory is not initialized")
	}
	if err := s.InitMaintenanceRunsTable(); err != nil {
		return nil, err
	}
	row := s.db.QueryRow(`
		SELECT id, started_at, finished_at, status, phase_results
		FROM maintenance_runs
		ORDER BY finished_at DESC, id DESC
		LIMIT 1`)
	var record MaintenanceRunRecord
	var payload string
	if err := row.Scan(&record.ID, &record.StartedAt, &record.FinishedAt, &record.Status, &payload); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest maintenance run: %w", err)
	}
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &record.PhaseResults); err != nil {
			return nil, fmt.Errorf("decode maintenance phase results: %w", err)
		}
	}
	return &record, nil
}