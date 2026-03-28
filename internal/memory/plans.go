package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	PlanStatusDraft     = "draft"
	PlanStatusActive    = "active"
	PlanStatusPaused    = "paused"
	PlanStatusCompleted = "completed"
	PlanStatusCancelled = "cancelled"
)

const (
	PlanTaskPending    = "pending"
	PlanTaskInProgress = "in_progress"
	PlanTaskCompleted  = "completed"
	PlanTaskFailed     = "failed"
	PlanTaskSkipped    = "skipped"
)

type Plan struct {
	ID            string      `json:"id"`
	SessionID     string      `json:"session_id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Status        string      `json:"status"`
	Priority      int         `json:"priority"`
	UserRequest   string      `json:"user_request"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	StartedAt     string      `json:"started_at,omitempty"`
	CompletedAt   string      `json:"completed_at,omitempty"`
	Tasks         []PlanTask  `json:"tasks,omitempty"`
	Events        []PlanEvent `json:"events,omitempty"`
	TaskCounts    PlanCounts  `json:"task_counts"`
	ProgressPct   int         `json:"progress_pct"`
	CurrentTaskID string      `json:"current_task_id,omitempty"`
	CurrentTask   string      `json:"current_task,omitempty"`
}

type PlanTask struct {
	ID            string   `json:"id"`
	PlanID        string   `json:"plan_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	TaskOrder     int      `json:"task_order"`
	Status        string   `json:"status"`
	Kind          string   `json:"kind"`
	ToolName      string   `json:"tool_name,omitempty"`
	ToolArgsJSON  string   `json:"tool_args_json,omitempty"`
	DependsOn     []string `json:"depends_on,omitempty"`
	ResultSummary string   `json:"result_summary,omitempty"`
	Error         string   `json:"error,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	CompletedAt   string   `json:"completed_at,omitempty"`
}

type PlanTaskInput struct {
	Title       string
	Description string
	Kind        string
	ToolName    string
	ToolArgs    map[string]interface{}
	DependsOn   []string
}

type PlanEvent struct {
	ID        int64  `json:"id"`
	PlanID    string `json:"plan_id"`
	EventType string `json:"event_type"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type PlanCounts struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}

func (s *SQLiteMemory) InitPlanTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS plans (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT 'draft',
		priority INTEGER DEFAULT 2,
		user_request TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME DEFAULT '',
		completed_at DATETIME DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS plan_tasks (
		id TEXT PRIMARY KEY,
		plan_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		task_order INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		kind TEXT DEFAULT 'task',
		tool_name TEXT DEFAULT '',
		tool_args_json TEXT DEFAULT '',
		depends_on_json TEXT DEFAULT '[]',
		result_summary TEXT DEFAULT '',
		error TEXT DEFAULT '',
		started_at DATETIME DEFAULT '',
		completed_at DATETIME DEFAULT '',
		FOREIGN KEY(plan_id) REFERENCES plans(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS plan_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(plan_id) REFERENCES plans(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_plans_session_status ON plans(session_id, status, updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_plan_tasks_plan_order ON plan_tasks(plan_id, task_order);
	CREATE INDEX IF NOT EXISTS idx_plan_events_plan_created ON plan_events(plan_id, created_at DESC);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("plan schema: %w", err)
	}
	return nil
}

func normalizePlanPriority(priority int) int {
	if priority < 1 || priority > 3 {
		return 2
	}
	return priority
}

func validPlanStatus(status string) bool {
	switch status {
	case PlanStatusDraft, PlanStatusActive, PlanStatusPaused, PlanStatusCompleted, PlanStatusCancelled:
		return true
	default:
		return false
	}
}

func validPlanTaskStatus(status string) bool {
	switch status {
	case PlanTaskPending, PlanTaskInProgress, PlanTaskCompleted, PlanTaskFailed, PlanTaskSkipped:
		return true
	default:
		return false
	}
}

func planNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func planTaskStatusIcon(status string) string {
	switch status {
	case PlanTaskCompleted:
		return "x"
	case PlanTaskInProgress:
		return "~"
	case PlanTaskFailed:
		return "!"
	case PlanTaskSkipped:
		return "-"
	default:
		return " "
	}
}

func (s *SQLiteMemory) CreatePlan(sessionID, title, description, userRequest string, priority int, inputs []PlanTaskInput) (*Plan, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("at least one task is required")
	}

	var existingID string
	err := s.db.QueryRow(`SELECT id FROM plans WHERE session_id = ? AND status IN (?, ?, ?) ORDER BY updated_at DESC LIMIT 1`,
		sessionID, PlanStatusDraft, PlanStatusActive, PlanStatusPaused).Scan(&existingID)
	if err == nil && existingID != "" {
		return nil, fmt.Errorf("session already has an unfinished plan")
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("check existing plan: %w", err)
	}

	now := planNow()
	planID := "plan_" + uuid.NewString()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO plans (id, session_id, title, description, status, priority, user_request, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		planID, sessionID, title, description, PlanStatusDraft, normalizePlanPriority(priority), userRequest, now, now,
	); err != nil {
		return nil, fmt.Errorf("insert plan: %w", err)
	}

	taskIDs := make([]string, len(inputs))
	for i := range inputs {
		taskIDs[i] = fmt.Sprintf("%s_task_%02d", planID, i+1)
	}

	for i, input := range inputs {
		if strings.TrimSpace(input.Title) == "" {
			return nil, fmt.Errorf("task %d title is required", i+1)
		}
		kind := strings.TrimSpace(input.Kind)
		if kind == "" {
			kind = "task"
		}
		toolArgsJSON := ""
		if len(input.ToolArgs) > 0 {
			b, _ := json.Marshal(input.ToolArgs)
			toolArgsJSON = string(b)
		}

		resolvedDeps := make([]string, 0, len(input.DependsOn))
		for _, dep := range input.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if strings.HasPrefix(dep, "task_") || strings.HasPrefix(dep, planID+"_task_") {
				resolvedDeps = append(resolvedDeps, dep)
				continue
			}
			var depIndex int
			if _, convErr := fmt.Sscanf(dep, "%d", &depIndex); convErr == nil && depIndex >= 1 && depIndex <= len(taskIDs) {
				resolvedDeps = append(resolvedDeps, taskIDs[depIndex-1])
			}
		}
		depsJSON, _ := json.Marshal(resolvedDeps)

		if _, err := tx.Exec(
			`INSERT INTO plan_tasks (id, plan_id, title, description, task_order, status, kind, tool_name, tool_args_json, depends_on_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			taskIDs[i], planID, input.Title, input.Description, i+1, PlanTaskPending, kind, input.ToolName, toolArgsJSON, string(depsJSON),
		); err != nil {
			return nil, fmt.Errorf("insert task %d: %w", i+1, err)
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
		planID, "created", "Plan created", now,
	); err != nil {
		return nil, fmt.Errorf("insert plan event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit plan create: %w", err)
	}

	return s.GetPlan(planID)
}

func (s *SQLiteMemory) AppendPlanNote(planID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("note is required")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM plans WHERE id = ?`, planID).Scan(&exists); err != nil {
		return fmt.Errorf("check plan: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("plan not found")
	}
	if _, err := s.db.Exec(
		`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
		planID, "note", note, planNow(),
	); err != nil {
		return fmt.Errorf("append plan note: %w", err)
	}
	updateRes, err := s.db.Exec(`UPDATE plans SET updated_at = ? WHERE id = ?`, planNow(), planID)
	if err != nil {
		return fmt.Errorf("touch plan: %w", err)
	}
	if rows, _ := updateRes.RowsAffected(); rows == 0 {
		return fmt.Errorf("plan not found")
	}
	return nil
}

func (s *SQLiteMemory) SetPlanStatus(planID, status, note string) (*Plan, error) {
	if !validPlanStatus(status) {
		return nil, fmt.Errorf("invalid plan status: %s", status)
	}
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var startedAt, completedAt interface{} = nil, nil
	switch status {
	case PlanStatusActive:
		startedAt = now
	case PlanStatusCompleted, PlanStatusCancelled:
		completedAt = now
	}

	res, err := tx.Exec(
		`UPDATE plans
		 SET status = ?, updated_at = ?, started_at = COALESCE(NULLIF(started_at, ''), ?), completed_at = CASE WHEN ? IS NOT NULL THEN ? ELSE completed_at END
		 WHERE id = ?`,
		status, now, startedAt, completedAt, completedAt, planID,
	)
	if err != nil {
		return nil, fmt.Errorf("update plan status: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan not found")
	}

	eventMessage := fmt.Sprintf("Plan status changed to %s", status)
	if strings.TrimSpace(note) != "" {
		eventMessage = strings.TrimSpace(note)
	}
	if _, err := tx.Exec(
		`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
		planID, "status", eventMessage, now,
	); err != nil {
		return nil, fmt.Errorf("insert status event: %w", err)
	}

	if status == PlanStatusActive {
		if err := s.promoteNextPlanTaskTx(tx, planID, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit plan status: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) UpdatePlanTask(planID, taskID, status, resultSummary, taskError string) (*Plan, error) {
	if !validPlanTaskStatus(status) {
		return nil, fmt.Errorf("invalid task status: %s", status)
	}
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	startedAt := ""
	completedAt := ""
	switch status {
	case PlanTaskInProgress:
		startedAt = now
	case PlanTaskCompleted, PlanTaskFailed, PlanTaskSkipped:
		completedAt = now
	}

	res, err := tx.Exec(
		`UPDATE plan_tasks
		 SET status = ?,
		     result_summary = CASE WHEN ? != '' THEN ? ELSE result_summary END,
		     error = CASE WHEN ? != '' THEN ? ELSE error END,
		     started_at = CASE WHEN ? != '' THEN ? ELSE started_at END,
		     completed_at = CASE WHEN ? != '' THEN ? ELSE completed_at END
		 WHERE plan_id = ? AND id = ?`,
		status,
		resultSummary, resultSummary,
		taskError, taskError,
		startedAt, startedAt,
		completedAt, completedAt,
		planID, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan task not found")
	}

	message := fmt.Sprintf("Task %s -> %s", taskID, status)
	if resultSummary != "" {
		message = resultSummary
	}
	if taskError != "" {
		message = taskError
	}
	if _, err := tx.Exec(
		`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
		planID, "task", message, now,
	); err != nil {
		return nil, fmt.Errorf("insert task event: %w", err)
	}
	if _, err := tx.Exec(`UPDATE plans SET updated_at = ? WHERE id = ?`, now, planID); err != nil {
		return nil, fmt.Errorf("touch plan: %w", err)
	}

	if status == PlanTaskCompleted {
		if err := s.promoteNextPlanTaskTx(tx, planID, now); err != nil {
			return nil, err
		}
		if err := s.finalizeCompletedPlanTx(tx, planID, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit task update: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) DeletePlan(planID string) error {
	res, err := s.db.Exec(`DELETE FROM plans WHERE id = ?`, planID)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("plan not found")
	}
	return nil
}

func (s *SQLiteMemory) ListPlans(sessionID, status string, limit int) ([]Plan, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := `SELECT id, session_id, title, description, status, priority, user_request, created_at, updated_at, started_at, completed_at
	          FROM plans WHERE session_id = ?`
	args := []interface{}{sessionID}
	if status != "" && status != "all" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	var plans []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Title, &p.Description, &p.Status, &p.Priority, &p.UserRequest, &p.CreatedAt, &p.UpdatedAt, &p.StartedAt, &p.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		if err := s.populatePlanComputedFields(&p, false); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (s *SQLiteMemory) GetPlan(planID string) (*Plan, error) {
	var p Plan
	err := s.db.QueryRow(
		`SELECT id, session_id, title, description, status, priority, user_request, created_at, updated_at, started_at, completed_at
		 FROM plans WHERE id = ?`,
		planID,
	).Scan(&p.ID, &p.SessionID, &p.Title, &p.Description, &p.Status, &p.Priority, &p.UserRequest, &p.CreatedAt, &p.UpdatedAt, &p.StartedAt, &p.CompletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plan not found")
		}
		return nil, fmt.Errorf("get plan: %w", err)
	}
	if err := s.populatePlanComputedFields(&p, true); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *SQLiteMemory) GetSessionPlan(sessionID string) (*Plan, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	var planID string
	err := s.db.QueryRow(
		`SELECT id FROM plans
		 WHERE session_id = ? AND status IN (?, ?, ?)
		 ORDER BY updated_at DESC LIMIT 1`,
		sessionID, PlanStatusActive, PlanStatusPaused, PlanStatusDraft,
	).Scan(&planID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session plan: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) BuildSessionPlanPrompt(sessionID string) (string, error) {
	plan, err := s.GetSessionPlan(sessionID)
	if err != nil || plan == nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plan: %s [%s] (%d%% complete)\n", plan.Title, plan.Status, plan.ProgressPct))
	for _, task := range plan.Tasks {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", planTaskStatusIcon(task.Status), task.Title))
	}
	return sb.String(), nil
}

func (s *SQLiteMemory) promoteNextPlanTaskTx(tx *sql.Tx, planID, now string) error {
	var activeStatus string
	err := tx.QueryRow(`SELECT status FROM plans WHERE id = ?`, planID).Scan(&activeStatus)
	if err != nil {
		return fmt.Errorf("read plan status: %w", err)
	}
	if activeStatus != PlanStatusActive {
		return nil
	}

	var inProgressCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM plan_tasks WHERE plan_id = ? AND status = ?`, planID, PlanTaskInProgress).Scan(&inProgressCount); err != nil {
		return fmt.Errorf("count in_progress tasks: %w", err)
	}
	if inProgressCount > 0 {
		return nil
	}

	rows, err := tx.Query(`SELECT id, depends_on_json FROM plan_tasks WHERE plan_id = ? AND status = ? ORDER BY task_order ASC`, planID, PlanTaskPending)
	if err != nil {
		return fmt.Errorf("select pending tasks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskID, depsJSON string
		if err := rows.Scan(&taskID, &depsJSON); err != nil {
			return fmt.Errorf("scan pending task: %w", err)
		}
		var deps []string
		if depsJSON != "" {
			_ = json.Unmarshal([]byte(depsJSON), &deps)
		}
		ready := true
		for _, depID := range deps {
			var depStatus string
			if err := tx.QueryRow(`SELECT status FROM plan_tasks WHERE plan_id = ? AND id = ?`, planID, depID).Scan(&depStatus); err != nil {
				ready = false
				break
			}
			if depStatus != PlanTaskCompleted && depStatus != PlanTaskSkipped {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE plan_tasks SET status = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END WHERE id = ?`,
			PlanTaskInProgress, now, taskID,
		); err != nil {
			return fmt.Errorf("promote next task: %w", err)
		}
		_, _ = tx.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
			planID, "task", fmt.Sprintf("Started next task: %s", taskID), now)
		return nil
	}
	return rows.Err()
}

func (s *SQLiteMemory) finalizeCompletedPlanTx(tx *sql.Tx, planID, now string) error {
	var remaining int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM plan_tasks WHERE plan_id = ? AND status IN (?, ?)`, planID, PlanTaskPending, PlanTaskInProgress).Scan(&remaining); err != nil {
		return fmt.Errorf("count remaining tasks: %w", err)
	}
	if remaining != 0 {
		return nil
	}
	if _, err := tx.Exec(
		`UPDATE plans SET status = ?, updated_at = ?, completed_at = CASE WHEN completed_at = '' THEN ? ELSE completed_at END WHERE id = ? AND status = ?`,
		PlanStatusCompleted, now, now, planID, PlanStatusActive,
	); err != nil {
		return fmt.Errorf("complete plan: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`,
		planID, "status", "Plan completed", now,
	); err != nil {
		return fmt.Errorf("insert completion event: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) populatePlanComputedFields(plan *Plan, withDetails bool) error {
	rows, err := s.db.Query(
		`SELECT id, plan_id, title, description, task_order, status, kind, tool_name, tool_args_json, depends_on_json, result_summary, error, started_at, completed_at
		 FROM plan_tasks WHERE plan_id = ? ORDER BY task_order ASC`,
		plan.ID,
	)
	if err != nil {
		return fmt.Errorf("load plan tasks: %w", err)
	}
	defer rows.Close()

	var tasks []PlanTask
	counts := PlanCounts{}
	for rows.Next() {
		var task PlanTask
		var depsJSON string
		if err := rows.Scan(&task.ID, &task.PlanID, &task.Title, &task.Description, &task.TaskOrder, &task.Status, &task.Kind, &task.ToolName, &task.ToolArgsJSON, &depsJSON, &task.ResultSummary, &task.Error, &task.StartedAt, &task.CompletedAt); err != nil {
			return fmt.Errorf("scan plan task: %w", err)
		}
		_ = json.Unmarshal([]byte(depsJSON), &task.DependsOn)
		tasks = append(tasks, task)
		counts.Total++
		switch task.Status {
		case PlanTaskPending:
			counts.Pending++
		case PlanTaskInProgress:
			counts.InProgress++
			plan.CurrentTaskID = task.ID
			plan.CurrentTask = task.Title
		case PlanTaskCompleted:
			counts.Completed++
		case PlanTaskFailed:
			counts.Failed++
		case PlanTaskSkipped:
			counts.Skipped++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	plan.Tasks = tasks
	plan.TaskCounts = counts
	if counts.Total > 0 {
		plan.ProgressPct = int(float64(counts.Completed+counts.Skipped) / float64(counts.Total) * 100)
	}

	if withDetails {
		eventRows, err := s.db.Query(
			`SELECT id, plan_id, event_type, message, created_at FROM plan_events WHERE plan_id = ? ORDER BY created_at DESC LIMIT 8`,
			plan.ID,
		)
		if err != nil {
			return fmt.Errorf("load plan events: %w", err)
		}
		defer eventRows.Close()
		var events []PlanEvent
		for eventRows.Next() {
			var evt PlanEvent
			if err := eventRows.Scan(&evt.ID, &evt.PlanID, &evt.EventType, &evt.Message, &evt.CreatedAt); err != nil {
				return fmt.Errorf("scan plan event: %w", err)
			}
			events = append(events, evt)
		}
		if err := eventRows.Err(); err != nil {
			return err
		}
		plan.Events = events
	}
	return nil
}
