package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	LooperSchemaV2Key          = "looper_schema_v2"
	LooperDefaultMaxRounds     = 10
	LooperMaxRoundsLimit       = 50
	LooperDefaultTargetScore   = 85
	LooperMinTargetScore       = 50
	LooperMaxTargetScore       = 100
	LooperDefaultStallRounds   = 3
	LooperMaxStallRounds       = 10
	LooperRunRetention         = 20
	LooperMaxLogEntries        = 200
	LooperMaxLogResponseRunes  = 8000
	LooperGoalExcerptLimit     = 200
)

// LooperPreset describes a saved Looper configuration (model v2).
type LooperPreset struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	IsBuiltin   bool      `json:"is_builtin"`
	BuiltinKey  string    `json:"builtin_key,omitempty"`
	Goal        string    `json:"goal"`
	Work        string    `json:"work"`
	Evaluate    string    `json:"evaluate"`
	Finish      string    `json:"finish"`
	MaxRounds   int       `json:"max_rounds"`
	TargetScore int       `json:"target_score"`
	StallRounds int       `json:"stall_rounds"`
	ProviderID  string    `json:"provider_id"`
	Model       string    `json:"model"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LooperLogEntry is one step inside a loop run.
type LooperLogEntry struct {
	Round    int    `json:"round"`
	Step     string `json:"step"`
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
	Duration int64  `json:"duration_ms"`
	Score    int    `json:"score,omitempty"`
	Done     bool   `json:"done,omitempty"`
	Feedback string `json:"feedback,omitempty"`
}

// LooperRunState is the live status of a running or finished loop.
type LooperRunState struct {
	Status        string           `json:"status"`
	Running       bool             `json:"running"`
	CurrentStep   string           `json:"current_step"`
	Round         int              `json:"round"`
	MaxRounds     int              `json:"max_rounds"`
	ScoreHistory  []int            `json:"score_history,omitempty"`
	BestScore     int              `json:"best_score"`
	LastFeedback  string           `json:"last_feedback,omitempty"`
	LastSummary   string           `json:"last_summary,omitempty"`
	LastResult    string           `json:"last_result,omitempty"`
	Logs          []LooperLogEntry `json:"logs"`
	Error         string           `json:"error,omitempty"`
	Stopped       bool             `json:"stopped,omitempty"`
	InputTokens   int64            `json:"input_tokens"`
	OutputTokens  int64            `json:"output_tokens"`
	EstimatedCostUSD float64       `json:"estimated_cost_usd"`
	Paused        bool             `json:"paused"`
	ResumeFrom    int              `json:"resume_from,omitempty"`
	ResumeSnapshot *LooperResumeState `json:"resume_snapshot,omitempty"`
	StartedAt     time.Time        `json:"started_at,omitempty"`
}

// LooperResumeState captures the minimal state required to resume a loop.
type LooperResumeState struct {
	Round           int    `json:"round"`
	BestScore       int    `json:"best_score"`
	ScoreHistory    []int  `json:"score_history,omitempty"`
	LastFeedback    string `json:"last_feedback,omitempty"`
	LastWorkSummary string `json:"last_work_summary,omitempty"`
}

// LooperRunRecord is a persisted finished run (history).
type LooperRunRecord struct {
	ID           int64            `json:"id"`
	PresetName   string           `json:"preset_name"`
	GoalExcerpt  string           `json:"goal_excerpt"`
	Status       string           `json:"status"`
	Rounds       int              `json:"rounds"`
	MaxRounds    int              `json:"max_rounds"`
	BestScore    int              `json:"best_score"`
	FinalScore   int              `json:"final_score"`
	TargetScore  int              `json:"target_score"`
	InputTokens  int64            `json:"input_tokens"`
	OutputTokens int64            `json:"output_tokens"`
	CostUSD      float64          `json:"cost_usd"`
	Error        string           `json:"error,omitempty"`
	StartedAt    time.Time        `json:"started_at"`
	FinishedAt   time.Time        `json:"finished_at"`
	Logs         []LooperLogEntry `json:"logs,omitempty"`
}

// LooperRunConfig holds everything needed to execute one loop.
type LooperRunConfig struct {
	Goal        string
	Work        string
	Evaluate    string
	Finish      string
	MaxRounds   int
	TargetScore int
	StallRounds int
	ProviderID  string
	Model       string
	PresetName  string
}

// LooperPresetStore handles CRUD for looper presets and run history.
type LooperPresetStore struct {
	db     *sql.DB
	dbPath string
}

// NewLooperPresetStore creates a preset store.
func NewLooperPresetStore(db *sql.DB) *LooperPresetStore {
	return &LooperPresetStore{db: db}
}

// SetDBPath records the on-disk database path used for the v2 backup.
func (ps *LooperPresetStore) SetDBPath(path string) {
	if ps == nil {
		return
	}
	ps.dbPath = strings.TrimSpace(path)
}

// NormalizeLooperRunConfig clamps run settings to supported ranges.
func NormalizeLooperRunConfig(cfg *LooperRunConfig) {
	if cfg == nil {
		return
	}
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = LooperDefaultMaxRounds
	}
	if cfg.MaxRounds > LooperMaxRoundsLimit {
		cfg.MaxRounds = LooperMaxRoundsLimit
	}
	if cfg.TargetScore <= 0 {
		cfg.TargetScore = LooperDefaultTargetScore
	}
	if cfg.TargetScore < LooperMinTargetScore {
		cfg.TargetScore = LooperMinTargetScore
	}
	if cfg.TargetScore > LooperMaxTargetScore {
		cfg.TargetScore = LooperMaxTargetScore
	}
	if cfg.StallRounds < 0 {
		cfg.StallRounds = LooperDefaultStallRounds
	}
	if cfg.StallRounds > LooperMaxStallRounds {
		cfg.StallRounds = LooperMaxStallRounds
	}
}

// NormalizeLooperPreset applies the same clamps used for a live run.
func NormalizeLooperPreset(p *LooperPreset) {
	if p == nil {
		return
	}
	cfg := LooperRunConfig{
		MaxRounds:   p.MaxRounds,
		TargetScore: p.TargetScore,
		StallRounds: p.StallRounds,
	}
	NormalizeLooperRunConfig(&cfg)
	p.MaxRounds = cfg.MaxRounds
	p.TargetScore = cfg.TargetScore
	p.StallRounds = cfg.StallRounds
}

// Init ensures the looper tables exist, migrates to model v2, and seeds builtins.
func (ps *LooperPresetStore) Init(ctx context.Context) error {
	if _, err := ps.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS desktop_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("migrate looper meta: %w", err)
	}
	if _, err := ps.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS desktop_looper_presets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			is_builtin INTEGER DEFAULT 0,
			prepare TEXT NOT NULL,
			plan TEXT NOT NULL,
			action TEXT NOT NULL,
			test TEXT NOT NULL,
			exit_cond TEXT NOT NULL,
			finish TEXT DEFAULT '',
			provider_id TEXT DEFAULT '',
			model TEXT DEFAULT '',
			max_iter INTEGER DEFAULT 20,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("migrate looper table: %w", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE desktop_looper_presets ADD COLUMN context_mode TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN finish_context TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN prepare_truncation INTEGER DEFAULT 0`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN summarize_iterations INTEGER DEFAULT 0`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN goal TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN work TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN evaluate TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN target_score INTEGER DEFAULT 85`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN stall_rounds INTEGER DEFAULT 3`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN builtin_key TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN max_rounds INTEGER DEFAULT 0`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN created_at TEXT DEFAULT ''`,
		`ALTER TABLE desktop_looper_presets ADD COLUMN updated_at TEXT DEFAULT ''`,
	} {
		_, _ = ps.db.ExecContext(ctx, stmt)
	}
	if err := ps.ensureRunsTable(ctx); err != nil {
		return err
	}
	if err := ps.migrateToV2(ctx); err != nil {
		return err
	}
	return ps.seedBuiltinPresets(ctx)
}

func (ps *LooperPresetStore) ensureRunsTable(ctx context.Context) error {
	if _, err := ps.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS desktop_looper_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			preset_name TEXT DEFAULT '',
			goal_excerpt TEXT DEFAULT '',
			status TEXT NOT NULL,
			rounds INTEGER DEFAULT 0,
			max_rounds INTEGER DEFAULT 0,
			best_score INTEGER DEFAULT 0,
			final_score INTEGER DEFAULT 0,
			target_score INTEGER DEFAULT 0,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			cost_usd REAL DEFAULT 0,
			error TEXT DEFAULT '',
			started_at DATETIME,
			finished_at DATETIME,
			logs_json TEXT DEFAULT '[]'
		)`); err != nil {
		return fmt.Errorf("migrate looper runs table: %w", err)
	}
	return nil
}

func (ps *LooperPresetStore) migrateToV2(ctx context.Context) error {
	var marker string
	err := ps.db.QueryRowContext(ctx, `SELECT value FROM desktop_meta WHERE key = ?`, LooperSchemaV2Key).Scan(&marker)
	if err == nil && strings.TrimSpace(marker) == "true" {
		return ps.removeObsoleteBuiltins(ctx)
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read looper schema marker: %w", err)
	}

	if backupErr := ps.backupBeforeV2(ctx); backupErr != nil {
		slog.Default().Warn("looper v2 backup failed; continuing additive migration", "error", backupErr)
	}

	if _, err := ps.db.ExecContext(ctx, `
		UPDATE desktop_looper_presets
		SET
			goal = CASE WHEN TRIM(COALESCE(goal, '')) = '' THEN prepare ELSE goal END,
			work = CASE WHEN TRIM(COALESCE(work, '')) = '' THEN TRIM(plan || char(10) || char(10) || action) ELSE work END,
			evaluate = CASE WHEN TRIM(COALESCE(evaluate, '')) = '' THEN TRIM(test || char(10) || char(10) || 'Done when: ' || exit_cond) ELSE evaluate END,
			max_rounds = CASE
				WHEN COALESCE(max_rounds, 0) > 0 THEN max_rounds
				WHEN COALESCE(max_iter, 0) > 0 THEN max_iter
				ELSE 10
			END,
			target_score = CASE WHEN COALESCE(target_score, 0) < 50 THEN 85 ELSE target_score END,
			stall_rounds = CASE WHEN stall_rounds IS NULL THEN 3 ELSE stall_rounds END
		WHERE is_builtin = 0
	`); err != nil {
		return fmt.Errorf("migrate user looper presets to v2: %w", err)
	}

	if err := ps.removeObsoleteBuiltins(ctx); err != nil {
		return err
	}

	if _, err := ps.db.ExecContext(ctx,
		`INSERT INTO desktop_meta(key, value) VALUES(?, 'true')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, LooperSchemaV2Key); err != nil {
		return fmt.Errorf("mark looper schema v2: %w", err)
	}
	return nil
}

func (ps *LooperPresetStore) backupBeforeV2(ctx context.Context) error {
	if ps.dbPath == "" || ps.dbPath == ":memory:" {
		return nil
	}
	dest := ps.dbPath + ".looper-v2.bak"
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	escaped := strings.ReplaceAll(filepath.ToSlash(dest), "'", "''")
	if _, err := ps.db.ExecContext(ctx, `VACUUM INTO '`+escaped+`'`); err != nil {
		return fmt.Errorf("vacuum into %s: %w", dest, err)
	}
	return nil
}

func builtinPresetNames() map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range DefaultLooperPresets() {
		out[p.Name] = struct{}{}
	}
	return out
}

func (ps *LooperPresetStore) removeObsoleteBuiltins(ctx context.Context) error {
	keep := builtinPresetNames()
	rows, err := ps.db.QueryContext(ctx, `SELECT name FROM desktop_looper_presets WHERE is_builtin = 1`)
	if err != nil {
		return fmt.Errorf("list builtin looper presets: %w", err)
	}
	defer rows.Close()
	var obsolete []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan builtin looper name: %w", err)
		}
		if _, ok := keep[name]; !ok {
			obsolete = append(obsolete, name)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range obsolete {
		if _, err := ps.db.ExecContext(ctx, `DELETE FROM desktop_looper_presets WHERE name = ? AND is_builtin = 1`, name); err != nil {
			return fmt.Errorf("delete obsolete builtin %s: %w", name, err)
		}
	}
	return nil
}

func (ps *LooperPresetStore) seedBuiltinPresets(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := ps.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin looper seed: %w", err)
	}
	defer tx.Rollback()

	for _, p := range DefaultLooperPresets() {
		NormalizeLooperPreset(&p)
		_, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_looper_presets(
				name, is_builtin, builtin_key, prepare, plan, action, test, exit_cond,
				goal, work, evaluate, finish, target_score, stall_rounds,
				provider_id, model, max_iter, max_rounds, created_at, updated_at)
			VALUES(?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				is_builtin=excluded.is_builtin,
				builtin_key=excluded.builtin_key,
				prepare=excluded.prepare,
				plan=excluded.plan,
				action=excluded.action,
				test=excluded.test,
				exit_cond=excluded.exit_cond,
				goal=excluded.goal,
				work=excluded.work,
				evaluate=excluded.evaluate,
				finish=excluded.finish,
				target_score=excluded.target_score,
				stall_rounds=excluded.stall_rounds,
				provider_id=excluded.provider_id,
				model=excluded.model,
				max_iter=excluded.max_iter,
				max_rounds=excluded.max_rounds,
				updated_at=excluded.updated_at
			WHERE desktop_looper_presets.is_builtin = 1`,
			p.Name, p.BuiltinKey, p.Goal, p.Work, p.Work, p.Evaluate, p.Evaluate,
			p.Goal, p.Work, p.Evaluate, p.Finish, p.TargetScore, p.StallRounds,
			p.ProviderID, p.Model, p.MaxRounds, p.MaxRounds, now, now)
		if err != nil {
			return fmt.Errorf("seed looper preset %s: %w", p.Name, err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO desktop_meta(key, value) VALUES('looper_presets_seeded', 'true')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		return fmt.Errorf("mark looper presets seeded: %w", err)
	}
	return tx.Commit()
}

const looperPresetSelectCols = `id, name, is_builtin, COALESCE(builtin_key, ''), COALESCE(goal, ''), COALESCE(work, ''), COALESCE(evaluate, ''), finish, COALESCE(max_rounds, 0), COALESCE(max_iter, 0), COALESCE(target_score, 0), COALESCE(stall_rounds, 0), provider_id, model, created_at, updated_at`

func parseLooperTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func scanLooperPreset(scan func(dest ...any) error) (LooperPreset, error) {
	var p LooperPreset
	var isBuiltin, maxRounds, maxIter int
	var createdAt, updatedAt string
	if err := scan(&p.ID, &p.Name, &isBuiltin, &p.BuiltinKey, &p.Goal, &p.Work, &p.Evaluate, &p.Finish, &maxRounds, &maxIter, &p.TargetScore, &p.StallRounds, &p.ProviderID, &p.Model, &createdAt, &updatedAt); err != nil {
		return p, err
	}
	p.IsBuiltin = isBuiltin == 1
	p.MaxRounds = maxRounds
	if p.MaxRounds <= 0 {
		p.MaxRounds = maxIter
	}
	p.CreatedAt = parseLooperTime(createdAt)
	p.UpdatedAt = parseLooperTime(updatedAt)
	NormalizeLooperPreset(&p)
	return p, nil
}

// ListPresets returns all presets (builtin + user-saved).
func (ps *LooperPresetStore) ListPresets(ctx context.Context) ([]LooperPreset, error) {
	rows, err := ps.db.QueryContext(ctx,
		`SELECT `+looperPresetSelectCols+`
		FROM desktop_looper_presets ORDER BY is_builtin DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list looper presets: %w", err)
	}
	defer rows.Close()

	var out []LooperPreset
	for rows.Next() {
		p, err := scanLooperPreset(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan looper preset: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPreset loads a single preset by ID.
func (ps *LooperPresetStore) GetPreset(ctx context.Context, id int64) (LooperPreset, error) {
	row := ps.db.QueryRowContext(ctx,
		`SELECT `+looperPresetSelectCols+`
		FROM desktop_looper_presets WHERE id = ?`, id)
	p, err := scanLooperPreset(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return p, err
		}
		return p, fmt.Errorf("get looper preset: %w", err)
	}
	return p, nil
}

// SavePreset inserts or updates a user preset.
func (ps *LooperPresetStore) SavePreset(ctx context.Context, p LooperPreset) (int64, error) {
	if strings.TrimSpace(p.Name) == "" {
		return 0, fmt.Errorf("preset name is required")
	}
	NormalizeLooperPreset(&p)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if p.ID == 0 {
		res, err := ps.db.ExecContext(ctx,
			`INSERT INTO desktop_looper_presets(
				name, is_builtin, builtin_key, prepare, plan, action, test, exit_cond,
				goal, work, evaluate, finish, target_score, stall_rounds,
				provider_id, model, max_iter, max_rounds, created_at, updated_at)
			VALUES(?, 0, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Goal, p.Work, p.Work, p.Evaluate, p.Evaluate,
			p.Goal, p.Work, p.Evaluate, p.Finish, p.TargetScore, p.StallRounds,
			p.ProviderID, p.Model, p.MaxRounds, p.MaxRounds, now, now)
		if err != nil {
			return 0, fmt.Errorf("insert looper preset: %w", err)
		}
		return res.LastInsertId()
	}
	_, err := ps.db.ExecContext(ctx,
		`UPDATE desktop_looper_presets SET
			name=?, prepare=?, plan=?, action=?, test=?, exit_cond=?,
			goal=?, work=?, evaluate=?, finish=?, target_score=?, stall_rounds=?,
			provider_id=?, model=?, max_iter=?, max_rounds=?, updated_at=?
		WHERE id=? AND is_builtin=0`,
		p.Name, p.Goal, p.Work, p.Work, p.Evaluate, p.Evaluate,
		p.Goal, p.Work, p.Evaluate, p.Finish, p.TargetScore, p.StallRounds,
		p.ProviderID, p.Model, p.MaxRounds, p.MaxRounds, now, p.ID)
	if err != nil {
		return 0, fmt.Errorf("update looper preset: %w", err)
	}
	return p.ID, nil
}

// DeletePreset removes a user preset.
func (ps *LooperPresetStore) DeletePreset(ctx context.Context, id int64) error {
	res, err := ps.db.ExecContext(ctx, `DELETE FROM desktop_looper_presets WHERE id=? AND is_builtin=0`, id)
	if err != nil {
		return fmt.Errorf("delete looper preset: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("preset not found or is builtin")
	}
	return nil
}

func clampLooperLogs(logs []LooperLogEntry) []LooperLogEntry {
	if len(logs) > LooperMaxLogEntries {
		logs = logs[len(logs)-LooperMaxLogEntries:]
	}
	out := make([]LooperLogEntry, len(logs))
	for i, entry := range logs {
		entry.Response = truncateRunes(entry.Response, LooperMaxLogResponseRunes)
		entry.Prompt = truncateRunes(entry.Prompt, LooperMaxLogResponseRunes)
		entry.Feedback = truncateRunes(entry.Feedback, 2000)
		out[i] = entry
	}
	return out
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + fmt.Sprintf("... (%d more chars)", len(runes)-max)
}

func excerptGoal(goal string) string {
	goal = strings.TrimSpace(strings.Join(strings.Fields(goal), " "))
	runes := []rune(goal)
	if len(runes) <= LooperGoalExcerptLimit {
		return goal
	}
	return string(runes[:LooperGoalExcerptLimit])
}

// SaveRun persists a finished run and evicts older rows beyond retention.
func (ps *LooperPresetStore) SaveRun(ctx context.Context, rec LooperRunRecord) (int64, error) {
	rec.Logs = clampLooperLogs(rec.Logs)
	raw, err := json.Marshal(rec.Logs)
	if err != nil {
		return 0, fmt.Errorf("marshal looper run logs: %w", err)
	}
	if rec.FinishedAt.IsZero() {
		rec.FinishedAt = time.Now().UTC()
	}
	if rec.StartedAt.IsZero() {
		rec.StartedAt = rec.FinishedAt
	}
	res, err := ps.db.ExecContext(ctx,
		`INSERT INTO desktop_looper_runs(
			preset_name, goal_excerpt, status, rounds, max_rounds, best_score, final_score,
			target_score, input_tokens, output_tokens, cost_usd, error, started_at, finished_at, logs_json)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.PresetName, excerptGoal(rec.GoalExcerpt), rec.Status, rec.Rounds, rec.MaxRounds,
		rec.BestScore, rec.FinalScore, rec.TargetScore, rec.InputTokens, rec.OutputTokens,
		rec.CostUSD, rec.Error, rec.StartedAt.UTC().Format(time.RFC3339Nano),
		rec.FinishedAt.UTC().Format(time.RFC3339Nano), string(raw))
	if err != nil {
		return 0, fmt.Errorf("insert looper run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := ps.db.ExecContext(ctx,
		`DELETE FROM desktop_looper_runs WHERE id NOT IN (
			SELECT id FROM desktop_looper_runs ORDER BY id DESC LIMIT ?
		)`, LooperRunRetention); err != nil {
		return id, fmt.Errorf("retain looper runs: %w", err)
	}
	return id, nil
}

func scanLooperRun(scan func(dest ...any) error, includeLogs bool) (LooperRunRecord, error) {
	var rec LooperRunRecord
	var started, finished, logsJSON string
	dest := []any{
		&rec.ID, &rec.PresetName, &rec.GoalExcerpt, &rec.Status, &rec.Rounds, &rec.MaxRounds,
		&rec.BestScore, &rec.FinalScore, &rec.TargetScore, &rec.InputTokens, &rec.OutputTokens,
		&rec.CostUSD, &rec.Error, &started, &finished,
	}
	if includeLogs {
		dest = append(dest, &logsJSON)
	}
	if err := scan(dest...); err != nil {
		return rec, err
	}
	if started != "" {
		if t, err := time.Parse(time.RFC3339Nano, started); err == nil {
			rec.StartedAt = t
		} else if t, err := time.Parse(time.RFC3339, started); err == nil {
			rec.StartedAt = t
		}
	}
	if finished != "" {
		if t, err := time.Parse(time.RFC3339Nano, finished); err == nil {
			rec.FinishedAt = t
		} else if t, err := time.Parse(time.RFC3339, finished); err == nil {
			rec.FinishedAt = t
		}
	}
	if includeLogs && strings.TrimSpace(logsJSON) != "" {
		if err := json.Unmarshal([]byte(logsJSON), &rec.Logs); err != nil {
			return rec, fmt.Errorf("decode looper run logs: %w", err)
		}
	}
	return rec, nil
}

const looperRunListCols = `id, preset_name, goal_excerpt, status, rounds, max_rounds, best_score, final_score, target_score, input_tokens, output_tokens, cost_usd, error, started_at, finished_at`

// ListRuns returns recent runs newest first, without log bodies.
func (ps *LooperPresetStore) ListRuns(ctx context.Context) ([]LooperRunRecord, error) {
	rows, err := ps.db.QueryContext(ctx,
		`SELECT `+looperRunListCols+` FROM desktop_looper_runs ORDER BY id DESC LIMIT ?`, LooperRunRetention)
	if err != nil {
		return nil, fmt.Errorf("list looper runs: %w", err)
	}
	defer rows.Close()
	var out []LooperRunRecord
	for rows.Next() {
		rec, err := scanLooperRun(rows.Scan, false)
		if err != nil {
			return nil, fmt.Errorf("scan looper run: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// GetRun loads one run including logs.
func (ps *LooperPresetStore) GetRun(ctx context.Context, id int64) (LooperRunRecord, error) {
	row := ps.db.QueryRowContext(ctx,
		`SELECT `+looperRunListCols+`, logs_json FROM desktop_looper_runs WHERE id = ?`, id)
	rec, err := scanLooperRun(row.Scan, true)
	if err != nil {
		if err == sql.ErrNoRows {
			return rec, err
		}
		return rec, fmt.Errorf("get looper run: %w", err)
	}
	return rec, nil
}

// DeleteRun removes one history row.
func (ps *LooperPresetStore) DeleteRun(ctx context.Context, id int64) error {
	res, err := ps.db.ExecContext(ctx, `DELETE FROM desktop_looper_runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete looper run: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("run not found")
	}
	return nil
}

// ClearRuns removes the entire run history.
func (ps *LooperPresetStore) ClearRuns(ctx context.Context) error {
	if _, err := ps.db.ExecContext(ctx, `DELETE FROM desktop_looper_runs`); err != nil {
		return fmt.Errorf("clear looper runs: %w", err)
	}
	return nil
}

// LooperRunStateHolder holds mutable run state safely.
type LooperRunStateHolder struct {
	mu          sync.Mutex
	state       LooperRunState
	cancelFn    context.CancelFunc
	paused      bool
	resumeState *LooperResumeState
}

// NewLooperRunStateHolder creates a state holder.
func NewLooperRunStateHolder() *LooperRunStateHolder {
	return &LooperRunStateHolder{
		state: LooperRunState{Status: "idle", CurrentStep: "idle"},
	}
}

func copyLooperState(src LooperRunState) LooperRunState {
	s := src
	if src.Logs != nil {
		s.Logs = make([]LooperLogEntry, len(src.Logs))
		copy(s.Logs, src.Logs)
	}
	if src.ScoreHistory != nil {
		s.ScoreHistory = make([]int, len(src.ScoreHistory))
		copy(s.ScoreHistory, src.ScoreHistory)
	}
	if src.ResumeSnapshot != nil {
		cp := *src.ResumeSnapshot
		if src.ResumeSnapshot.ScoreHistory != nil {
			cp.ScoreHistory = make([]int, len(src.ResumeSnapshot.ScoreHistory))
			copy(cp.ScoreHistory, src.ResumeSnapshot.ScoreHistory)
		}
		s.ResumeSnapshot = &cp
	}
	return s
}

// State returns a deep copy of the current state.
func (h *LooperRunStateHolder) State() LooperRunState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return copyLooperState(h.state)
}

func emptyLooperState(maxRounds int) LooperRunState {
	return LooperRunState{
		Status:       "running",
		Running:      true,
		CurrentStep:  "work",
		MaxRounds:    maxRounds,
		Logs:         make([]LooperLogEntry, 0),
		ScoreHistory: make([]int, 0),
		StartedAt:    time.Now().UTC(),
	}
}

// TryStart atomically reserves a new run and stores its cancel function.
func (h *LooperRunStateHolder) TryStart(maxRounds int, cancel context.CancelFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.Running {
		return fmt.Errorf("a loop is already running")
	}
	h.paused = false
	h.resumeState = nil
	h.cancelFn = cancel
	h.state = emptyLooperState(maxRounds)
	return nil
}

// TryStartResume re-enters running state while preserving logs and usage counters.
func (h *LooperRunStateHolder) TryStartResume(maxRounds, resumeFrom int, cancel context.CancelFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.Running {
		return fmt.Errorf("a loop is already running")
	}
	if h.resumeState == nil {
		return fmt.Errorf("no paused run to resume")
	}
	h.paused = false
	h.cancelFn = cancel
	h.state.Running = true
	h.state.Status = "running"
	h.state.CurrentStep = "work"
	h.state.MaxRounds = maxRounds
	h.state.Round = resumeFrom
	h.state.Error = ""
	h.state.Stopped = false
	h.state.Paused = false
	if h.state.Logs == nil {
		h.state.Logs = make([]LooperLogEntry, 0)
	}
	return nil
}

// SetIdle marks the run as finished (normal completion, stop, or error).
func (h *LooperRunStateHolder) SetIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.resumeState != nil || h.state.Paused {
		h.state.Running = false
		h.state.Status = "paused"
		h.state.CurrentStep = "paused"
		h.cancelFn = nil
		h.paused = false
		return
	}
	h.paused = false
	h.resumeState = nil
	h.state.Running = false
	h.state.Paused = false
	h.state.ResumeFrom = 0
	h.state.ResumeSnapshot = nil
	h.cancelFn = nil
	switch {
	case h.state.Stopped:
		h.state.Status = "stopped"
		h.state.CurrentStep = "stopped"
	case h.state.Status == "completed" || h.state.Status == "max_rounds" || h.state.Status == "stalled" || h.state.Status == "failed":
		// keep terminal status
	case h.state.Error != "":
		h.state.Status = "failed"
		if h.state.CurrentStep == "" || h.state.CurrentStep == "paused" {
			h.state.CurrentStep = "idle"
		}
	default:
		h.state.Status = "idle"
		h.state.CurrentStep = "idle"
	}
}

// SetStep updates the current step.
func (h *LooperRunStateHolder) SetStep(step string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.CurrentStep = step
}

// SetRound updates the current round.
func (h *LooperRunStateHolder) SetRound(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Round = n
}

// SetStatus records a terminal or live status without clearing the run flag.
func (h *LooperRunStateHolder) SetStatus(status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Status = status
}

// SetLastResult updates the last result.
func (h *LooperRunStateHolder) SetLastResult(res string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.LastResult = res
}

// RecordEvaluation stores the latest score, feedback, and summary.
func (h *LooperRunStateHolder) RecordEvaluation(score int, feedback, summary string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.ScoreHistory = append(h.state.ScoreHistory, score)
	if score > h.state.BestScore {
		h.state.BestScore = score
	}
	h.state.LastFeedback = feedback
	h.state.LastSummary = summary
	h.state.LastResult = summary
}

// AppendLog adds a log entry. Keeps at most 200 entries.
func (h *LooperRunStateHolder) AppendLog(entry LooperLogEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Logs = append(h.state.Logs, entry)
	if len(h.state.Logs) > LooperMaxLogEntries {
		h.state.Logs = h.state.Logs[len(h.state.Logs)-LooperMaxLogEntries:]
	}
}

// SetError sets the error field (clears Stopped so UI shows error, not stop).
func (h *LooperRunStateHolder) SetError(err string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Error = err
	h.state.Stopped = false
	h.state.Status = "failed"
}

// SetStopped marks a user-initiated stop (not an error).
func (h *LooperRunStateHolder) SetStopped() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Stopped = true
	h.state.Error = ""
	h.state.Status = "stopped"
	h.state.CurrentStep = "stopped"
}

// AddUsage accumulates token usage and estimated USD cost for the run.
func (h *LooperRunStateHolder) AddUsage(inputTokens, outputTokens int, costUSD float64) {
	if inputTokens <= 0 && outputTokens <= 0 && costUSD <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if inputTokens > 0 {
		h.state.InputTokens += int64(inputTokens)
	}
	if outputTokens > 0 {
		h.state.OutputTokens += int64(outputTokens)
	}
	if costUSD > 0 {
		h.state.EstimatedCostUSD += costUSD
	}
}

// SetCancelFn stores the cancel function for the current run.
func (h *LooperRunStateHolder) SetCancelFn(fn context.CancelFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cancelFn = fn
}

// CancelRun cancels the current run if any.
func (h *LooperRunStateHolder) CancelRun() {
	h.mu.Lock()
	fn := h.cancelFn
	h.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// RequestPause signals that the currently executing loop should pause
// at the next safe checkpoint.
func (h *LooperRunStateHolder) RequestPause() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.Running && !h.state.Paused {
		h.paused = true
	}
}

// IsPauseRequested returns whether a pause request is pending.
func (h *LooperRunStateHolder) IsPauseRequested() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.paused
}

// SaveResumeState stores the snapshot that enables resuming the loop later.
func (h *LooperRunStateHolder) SaveResumeState(rs LooperResumeState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := rs
	if rs.ScoreHistory != nil {
		cp.ScoreHistory = make([]int, len(rs.ScoreHistory))
		copy(cp.ScoreHistory, rs.ScoreHistory)
	}
	h.resumeState = &cp
	h.state.Paused = true
	h.state.Running = false
	h.state.Status = "paused"
	h.state.CurrentStep = "paused"
	h.state.ResumeFrom = rs.Round
	snap := cp
	h.state.ResumeSnapshot = &snap
	h.paused = false
	h.cancelFn = nil
}

// GetResumeState returns the saved resume snapshot (if one exists).
func (h *LooperRunStateHolder) GetResumeState() (LooperResumeState, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.resumeState == nil {
		return LooperResumeState{}, false
	}
	cp := *h.resumeState
	if h.resumeState.ScoreHistory != nil {
		cp.ScoreHistory = make([]int, len(h.resumeState.ScoreHistory))
		copy(cp.ScoreHistory, h.resumeState.ScoreHistory)
	}
	return cp, true
}

// ClearResumeState discards any saved resume information.
func (h *LooperRunStateHolder) ClearResumeState() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.resumeState = nil
	h.paused = false
	h.state.Paused = false
	h.state.ResumeFrom = 0
	h.state.ResumeSnapshot = nil
}

// StallWithoutImprovement reports whether the last stallN scores failed to
// beat the best score from earlier rounds.
func StallWithoutImprovement(history []int, stallN int) bool {
	if stallN <= 0 || len(history) < stallN {
		return false
	}
	bestBefore := 0
	prefix := history[:len(history)-stallN]
	for _, score := range prefix {
		if score > bestBefore {
			bestBefore = score
		}
	}
	for _, score := range history[len(history)-stallN:] {
		if score > bestBefore {
			return false
		}
	}
	return true
}
