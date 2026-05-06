package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LooperPreset describes a saved Looper configuration.
type LooperPreset struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	IsBuiltin  bool      `json:"is_builtin"`
	Prepare    string    `json:"prepare"`
	Plan       string    `json:"plan"`
	Action     string    `json:"action"`
	Test       string    `json:"test"`
	ExitCond   string    `json:"exit_cond"`
	Finish     string    `json:"finish"`
	ProviderID string    `json:"provider_id"`
	Model      string    `json:"model"`
	MaxIter    int       `json:"max_iter"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LooperLogEntry is one step inside a loop run.
type LooperLogEntry struct {
	Iteration int    `json:"iteration"`
	Step      string `json:"step"`
	Prompt    string `json:"prompt"`
	Response  string `json:"response"`
	Duration  int64  `json:"duration"`
}

// LooperRunState is the live status of a running or finished loop.
type LooperRunState struct {
	Running       bool             `json:"running"`
	CurrentStep   string           `json:"current_step"`
	Iteration     int              `json:"iteration"`
	MaxIterations int              `json:"max_iterations"`
	LastResult    string           `json:"last_result"`
	Logs          []LooperLogEntry `json:"logs"`
	Error         string           `json:"error,omitempty"`
}

// LooperRunConfig holds everything needed to execute one loop.
type LooperRunConfig struct {
	Prepare    string
	Plan       string
	Action     string
	Test       string
	ExitCond   string
	Finish     string
	ProviderID string
	Model      string
	MaxIter    int
}

// LooperPresetStore handles CRUD for looper presets.
type LooperPresetStore struct {
	db *sql.DB
}

// NewLooperPresetStore creates a preset store.
func NewLooperPresetStore(db *sql.DB) *LooperPresetStore {
	return &LooperPresetStore{db: db}
}

// Init ensures the looper presets table exists and seeds builtins.
func (ps *LooperPresetStore) Init(ctx context.Context) error {
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
	return ps.seedBuiltinPresets(ctx)
}

func (ps *LooperPresetStore) seedBuiltinPresets(ctx context.Context) error {
	var seeded string
	err := ps.db.QueryRowContext(ctx, `SELECT value FROM desktop_meta WHERE key = 'looper_presets_seeded'`).Scan(&seeded)
	if err == nil && seeded == "true" {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read looper seed state: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := ps.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin looper seed: %w", err)
	}
	defer tx.Rollback()

	for _, p := range DefaultLooperPresets() {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO desktop_looper_presets(name, is_builtin, prepare, plan, action, test, exit_cond, finish, provider_id, model, max_iter, created_at, updated_at)
			VALUES(?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish, p.ProviderID, p.Model, p.MaxIter, now, now)
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

// ListPresets returns all presets (builtin + user-saved).
func (ps *LooperPresetStore) ListPresets(ctx context.Context) ([]LooperPreset, error) {
	rows, err := ps.db.QueryContext(ctx,
		`SELECT id, name, is_builtin, prepare, plan, action, test, exit_cond, finish, provider_id, model, max_iter, created_at, updated_at
		FROM desktop_looper_presets ORDER BY is_builtin DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list looper presets: %w", err)
	}
	defer rows.Close()

	var out []LooperPreset
	for rows.Next() {
		var p LooperPreset
		var isBuiltin int
		if err := rows.Scan(&p.ID, &p.Name, &isBuiltin, &p.Prepare, &p.Plan, &p.Action, &p.Test, &p.ExitCond, &p.Finish, &p.ProviderID, &p.Model, &p.MaxIter, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan looper preset: %w", err)
		}
		p.IsBuiltin = isBuiltin == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListExamples returns only builtin presets.
func (ps *LooperPresetStore) ListExamples(ctx context.Context) ([]LooperPreset, error) {
	all, err := ps.ListPresets(ctx)
	if err != nil {
		return nil, err
	}
	var out []LooperPreset
	for _, p := range all {
		if p.IsBuiltin {
			out = append(out, p)
		}
	}
	return out, nil
}

// GetPreset loads a single preset by ID.
func (ps *LooperPresetStore) GetPreset(ctx context.Context, id int64) (LooperPreset, error) {
	var p LooperPreset
	var isBuiltin int
	err := ps.db.QueryRowContext(ctx,
		`SELECT id, name, is_builtin, prepare, plan, action, test, exit_cond, finish, provider_id, model, max_iter, created_at, updated_at
		FROM desktop_looper_presets WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &isBuiltin, &p.Prepare, &p.Plan, &p.Action, &p.Test, &p.ExitCond, &p.Finish, &p.ProviderID, &p.Model, &p.MaxIter, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, fmt.Errorf("get looper preset: %w", err)
	}
	p.IsBuiltin = isBuiltin == 1
	return p, nil
}

// SavePreset inserts or updates a user preset.
func (ps *LooperPresetStore) SavePreset(ctx context.Context, p LooperPreset) (int64, error) {
	if strings.TrimSpace(p.Name) == "" {
		return 0, fmt.Errorf("preset name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if p.ID == 0 {
		res, err := ps.db.ExecContext(ctx,
			`INSERT INTO desktop_looper_presets(name, is_builtin, prepare, plan, action, test, exit_cond, finish, provider_id, model, max_iter, created_at, updated_at)
			VALUES(?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish, p.ProviderID, p.Model, p.MaxIter, now, now)
		if err != nil {
			return 0, fmt.Errorf("insert looper preset: %w", err)
		}
		return res.LastInsertId()
	}
	_, err := ps.db.ExecContext(ctx,
		`UPDATE desktop_looper_presets SET name=?, prepare=?, plan=?, action=?, test=?, exit_cond=?, finish=?, provider_id=?, model=?, max_iter=?, updated_at=?
		WHERE id=? AND is_builtin=0`,
		p.Name, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish, p.ProviderID, p.Model, p.MaxIter, now, p.ID)
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

// LooperRunStateHolder holds mutable run state safely.
type LooperRunStateHolder struct {
	mu       sync.Mutex
	state    LooperRunState
	cancelFn context.CancelFunc
}

// NewLooperRunStateHolder creates a state holder.
func NewLooperRunStateHolder() *LooperRunStateHolder {
	return &LooperRunStateHolder{state: LooperRunState{CurrentStep: "idle"}}
}

// State returns a deep copy of the current state.
func (h *LooperRunStateHolder) State() LooperRunState {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.state
	s.Logs = make([]LooperLogEntry, len(h.state.Logs))
	copy(s.Logs, h.state.Logs)
	return s
}

// SetRunning initializes state for a new run.
func (h *LooperRunStateHolder) SetRunning(maxIter int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = LooperRunState{
		Running:       true,
		CurrentStep:   "prepare",
		MaxIterations: maxIter,
		Logs:          make([]LooperLogEntry, 0),
		Error:         "",
	}
}

// SetIdle marks the run as finished.
func (h *LooperRunStateHolder) SetIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Running = false
	h.state.CurrentStep = "idle"
}

// SetStep updates the current step.
func (h *LooperRunStateHolder) SetStep(step string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.CurrentStep = step
}

// SetIteration updates the current iteration.
func (h *LooperRunStateHolder) SetIteration(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Iteration = n
}

// SetLastResult updates the last result.
func (h *LooperRunStateHolder) SetLastResult(res string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.LastResult = res
}

// AppendLog adds a log entry.
func (h *LooperRunStateHolder) AppendLog(iteration int, step, prompt, response string, duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Logs = append(h.state.Logs, LooperLogEntry{
		Iteration: iteration,
		Step:      step,
		Prompt:    prompt,
		Response:  response,
		Duration:  duration.Milliseconds(),
	})
}

// SetError sets the error field.
func (h *LooperRunStateHolder) SetError(err string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Error = err
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
