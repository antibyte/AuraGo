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
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsBuiltin bool   `json:"is_builtin"`
	Prepare   string `json:"prepare"`
	Plan      string `json:"plan"`
	Action    string `json:"action"`
	Test      string `json:"test"`
	ExitCond  string `json:"exit_cond"`
	Finish    string `json:"finish"`

	// FinishContext controls how much of the final iteration result
	// is made available to the Finish prompt.
	// Valid values: "none", "last_test", "last_action_test", "full"
	// Default / empty = "last_test" (good balance for most creative loops)
	FinishContext string `json:"finish_context"`

	// PrepareTruncation controls how many characters of the Prepare step result
	// are kept for the iteration seed (iterSeed). Higher values are useful for
	// creative loops (style references, music descriptions, long briefs).
	// 0 = use default (2000 characters).
	PrepareTruncation int `json:"prepare_truncation"`

	// SummarizeIterations, when true, adds an explicit "Summarize this iteration"
	// step after Test. The resulting summary is fed into the next iteration.
	// This greatly improves coherence for long creative loops (e.g. Ralph Loop).
	SummarizeIterations bool `json:"summarize_iterations"`

	ProviderID  string    `json:"provider_id"`
	Model       string    `json:"model"`
	MaxIter     int       `json:"max_iter"`
	ContextMode string    `json:"context_mode"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LooperLogEntry is one step inside a loop run.
type LooperLogEntry struct {
	Iteration int    `json:"iteration"`
	Step      string `json:"step"`
	Prompt    string `json:"prompt"`
	Response  string `json:"response"`
	Duration  int64  `json:"duration"`
	Reason    string `json:"reason,omitempty"` // populated when exit condition returns structured output with "reason"
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
	// Pause / Resume support (E8)
	Paused         bool               `json:"paused"`
	ResumeFrom     int                `json:"resume_from,omitempty"`
	ResumeSnapshot *LooperResumeState `json:"resume_snapshot,omitempty"`
}

// LooperResumeState captures the minimal state required to resume a loop
// from a given iteration. It stores the key carry-over variables that the
// iteration loop uses to maintain continuity (especially important for
// "every_iteration" and "never" context modes and for the optional summarizer).
type LooperResumeState struct {
	Iteration                int    `json:"iteration"`
	LastTestResult           string `json:"last_test_result,omitempty"`
	PreviousIterationSummary string `json:"previous_iteration_summary,omitempty"`
	LastIterationSummary     string `json:"last_iteration_summary,omitempty"`
}

// LooperRunConfig holds everything needed to execute one loop.
type LooperRunConfig struct {
	Prepare  string
	Plan     string
	Action   string
	Test     string
	ExitCond string
	Finish   string

	// FinishContext is passed through from the preset.
	// See LooperPreset.FinishContext for possible values.
	FinishContext string

	// PrepareTruncation comes from the preset (0 = default 2000 chars).
	PrepareTruncation int

	// SummarizeIterations comes from the preset.
	SummarizeIterations bool

	ProviderID  string
	Model       string
	MaxIter     int
	ContextMode string
}

// LooperPresetStore handles CRUD for looper presets.
type LooperPresetStore struct {
	db *sql.DB
}

// NewLooperPresetStore creates a preset store.
func NewLooperPresetStore(db *sql.DB) *LooperPresetStore {
	return &LooperPresetStore{db: db}
}

// NormalizeContextMode returns a valid context mode, defaulting to "every_iteration".
func NormalizeContextMode(mode string) string {
	switch mode {
	case "never", "every_iteration", "every_step":
		return mode
	default:
		return "every_iteration"
	}
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
	ps.db.ExecContext(ctx, `ALTER TABLE desktop_looper_presets ADD COLUMN context_mode TEXT DEFAULT ''`)
	ps.db.ExecContext(ctx, `ALTER TABLE desktop_looper_presets ADD COLUMN finish_context TEXT DEFAULT ''`)
	ps.db.ExecContext(ctx, `ALTER TABLE desktop_looper_presets ADD COLUMN prepare_truncation INTEGER DEFAULT 0`)
	return ps.seedBuiltinPresets(ctx)
}

func (ps *LooperPresetStore) seedBuiltinPresets(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := ps.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin looper seed: %w", err)
	}
	defer tx.Rollback()

	for _, p := range DefaultLooperPresets() {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_looper_presets(name, is_builtin, prepare, plan, action, test, exit_cond, finish, finish_context, prepare_truncation, provider_id, model, max_iter, context_mode, created_at, updated_at)
			VALUES(?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				is_builtin=excluded.is_builtin,
				prepare=excluded.prepare,
				plan=excluded.plan,
				action=excluded.action,
				test=excluded.test,
				exit_cond=excluded.exit_cond,
				finish=excluded.finish,
				finish_context=excluded.finish_context,
				prepare_truncation=excluded.prepare_truncation,
				provider_id=excluded.provider_id,
				model=excluded.model,
				max_iter=excluded.max_iter,
				context_mode=excluded.context_mode,
				updated_at=excluded.updated_at
			WHERE desktop_looper_presets.is_builtin = 1`,
			p.Name, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish, p.FinishContext, p.PrepareTruncation, p.ProviderID, p.Model, p.MaxIter, p.ContextMode, now, now)
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
		`SELECT id, name, is_builtin, prepare, plan, action, test, exit_cond, finish, finish_context, prepare_truncation, provider_id, model, max_iter, context_mode, created_at, updated_at
		FROM desktop_looper_presets ORDER BY is_builtin DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list looper presets: %w", err)
	}
	defer rows.Close()

	var out []LooperPreset
	for rows.Next() {
		var p LooperPreset
		var isBuiltin int
		if err := rows.Scan(&p.ID, &p.Name, &isBuiltin, &p.Prepare, &p.Plan, &p.Action, &p.Test, &p.ExitCond, &p.Finish, &p.FinishContext, &p.PrepareTruncation, &p.ProviderID, &p.Model, &p.MaxIter, &p.ContextMode, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan looper preset: %w", err)
		}
		p.IsBuiltin = isBuiltin == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListExamples returns only builtin presets.
func (ps *LooperPresetStore) ListExamples(ctx context.Context) ([]LooperPreset, error) {
	rows, err := ps.db.QueryContext(ctx,
		`SELECT id, name, is_builtin, prepare, plan, action, test, exit_cond, finish, finish_context, prepare_truncation, provider_id, model, max_iter, context_mode, created_at, updated_at
		FROM desktop_looper_presets WHERE is_builtin = 1 ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list looper examples: %w", err)
	}
	defer rows.Close()

	var out []LooperPreset
	for rows.Next() {
		var p LooperPreset
		var isBuiltin int
		if err := rows.Scan(&p.ID, &p.Name, &isBuiltin, &p.Prepare, &p.Plan, &p.Action, &p.Test, &p.ExitCond, &p.Finish, &p.FinishContext, &p.PrepareTruncation, &p.ProviderID, &p.Model, &p.MaxIter, &p.ContextMode, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan looper example: %w", err)
		}
		p.IsBuiltin = isBuiltin == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPreset loads a single preset by ID.
func (ps *LooperPresetStore) GetPreset(ctx context.Context, id int64) (LooperPreset, error) {
	var p LooperPreset
	var isBuiltin int
	err := ps.db.QueryRowContext(ctx,
		`SELECT id, name, is_builtin, prepare, plan, action, test, exit_cond, finish, finish_context, prepare_truncation, provider_id, model, max_iter, context_mode, created_at, updated_at
		FROM desktop_looper_presets WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &isBuiltin, &p.Prepare, &p.Plan, &p.Action, &p.Test, &p.ExitCond, &p.Finish, &p.FinishContext, &p.PrepareTruncation, &p.ProviderID, &p.Model, &p.MaxIter, &p.ContextMode, &p.CreatedAt, &p.UpdatedAt)
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
			`INSERT INTO desktop_looper_presets(name, is_builtin, prepare, plan, action, test, exit_cond, finish, finish_context, prepare_truncation, provider_id, model, max_iter, context_mode, created_at, updated_at)
			VALUES(?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Name, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish, p.FinishContext, p.PrepareTruncation, p.ProviderID, p.Model, p.MaxIter, p.ContextMode, now, now)
		if err != nil {
			return 0, fmt.Errorf("insert looper preset: %w", err)
		}
		return res.LastInsertId()
	}
	_, err := ps.db.ExecContext(ctx,
		`UPDATE desktop_looper_presets SET name=?, prepare=?, plan=?, action=?, test=?, exit_cond=?, finish=?, finish_context=?, prepare_truncation=?, provider_id=?, model=?, max_iter=?, context_mode=?, updated_at=?
		WHERE id=? AND is_builtin=0`,
		p.Name, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish, p.FinishContext, p.PrepareTruncation, p.ProviderID, p.Model, p.MaxIter, p.ContextMode, now, p.ID)
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
	mu          sync.Mutex
	state       LooperRunState
	cancelFn    context.CancelFunc
	paused      bool
	resumeState *LooperResumeState
}

// NewLooperRunStateHolder creates a state holder.
func NewLooperRunStateHolder() *LooperRunStateHolder {
	return &LooperRunStateHolder{
		state: LooperRunState{CurrentStep: "idle"},
	}
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
	h.paused = false
	h.resumeState = nil
	h.state = LooperRunState{
		Running:        true,
		CurrentStep:    "prepare",
		MaxIterations:  maxIter,
		Logs:           make([]LooperLogEntry, 0),
		Error:          "",
		Paused:         false,
		ResumeFrom:     0,
		ResumeSnapshot: nil,
	}
}

// TryStart atomically reserves a new run and stores its cancel function.
func (h *LooperRunStateHolder) TryStart(maxIter int, cancel context.CancelFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.Running {
		return fmt.Errorf("a loop is already running")
	}
	h.paused = false
	h.resumeState = nil
	h.cancelFn = cancel
	h.state = LooperRunState{
		Running:        true,
		CurrentStep:    "prepare",
		MaxIterations:  maxIter,
		Logs:           make([]LooperLogEntry, 0),
		Error:          "",
		Paused:         false,
		ResumeFrom:     0,
		ResumeSnapshot: nil,
	}
	return nil
}

// SetIdle marks the run as finished (normal completion or error).
// If a resume snapshot exists we deliberately keep the paused/resumable state
// so that a later resume call can continue from where we left off.
func (h *LooperRunStateHolder) SetIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.resumeState != nil || h.state.Paused {
		// We intentionally paused with a valid resume point.
		// Keep the snapshot and only ensure the run flag is off.
		h.state.Running = false
		h.state.CurrentStep = "paused"
		h.cancelFn = nil
		h.paused = false // the request flag is no longer relevant
		return
	}
	// Normal terminal state (finished, error, or user stopped without resume intent)
	h.paused = false
	h.resumeState = nil
	h.state.Running = false
	h.state.CurrentStep = "idle"
	h.state.Paused = false
	h.state.ResumeFrom = 0
	h.state.ResumeSnapshot = nil
	h.cancelFn = nil
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

// AppendLog adds a log entry. Keeps at most 200 entries to prevent unbounded growth.
func (h *LooperRunStateHolder) AppendLog(iteration int, step, prompt, response string, duration time.Duration) {
	h.AppendLogWithReason(iteration, step, prompt, response, duration, "")
}

// AppendLogWithReason is like AppendLog but also records an optional human-readable reason
// (typically coming from structured exit output).
func (h *LooperRunStateHolder) AppendLogWithReason(iteration int, step, prompt, response string, duration time.Duration, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Logs = append(h.state.Logs, LooperLogEntry{
		Iteration: iteration,
		Step:      step,
		Prompt:    prompt,
		Response:  response,
		Duration:  duration.Milliseconds(),
		Reason:    reason,
	})
	const maxLogs = 200
	if len(h.state.Logs) > maxLogs {
		h.state.Logs = h.state.Logs[len(h.state.Logs)-maxLogs:]
	}
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

// ─────────────────────────────────────────────────────────────────────────────
// Pause / Resume support (E8 – clean foundation)
//
// RequestPause + checkpoint logic inside the runner allow expensive creative
// loops (e.g. the 18-iteration Ralph Loop) to be paused at safe iteration
// boundaries and later resumed without losing the accumulated context and
// summaries.
// ─────────────────────────────────────────────────────────────────────────────

// RequestPause signals that the currently executing loop should pause
// at the next safe checkpoint (after completing the current iteration's
// Exit decision and before starting the next iteration).
// The check is performed inside executeStarted.
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
// It transitions the holder into a "paused, resumable" state that the UI
// can observe via State() (Paused=true + ResumeFrom + ResumeSnapshot).
func (h *LooperRunStateHolder) SaveResumeState(rs LooperResumeState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := rs // copy
	h.resumeState = &cp
	h.state.Paused = true
	h.state.Running = false
	h.state.CurrentStep = "paused"
	h.state.ResumeFrom = rs.Iteration
	h.state.ResumeSnapshot = &cp
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
	return cp, true
}

// ClearResumeState discards any saved resume information (used after a
// successful resume or when the user explicitly discards a paused run).
func (h *LooperRunStateHolder) ClearResumeState() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.resumeState = nil
	h.paused = false
	h.state.Paused = false
	h.state.ResumeFrom = 0
	h.state.ResumeSnapshot = nil
}
