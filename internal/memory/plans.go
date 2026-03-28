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
	PlanStatusBlocked   = "blocked"
	PlanStatusCompleted = "completed"
	PlanStatusCancelled = "cancelled"
)

const (
	PlanTaskPending    = "pending"
	PlanTaskInProgress = "in_progress"
	PlanTaskBlocked    = "blocked"
	PlanTaskCompleted  = "completed"
	PlanTaskFailed     = "failed"
	PlanTaskSkipped    = "skipped"
)

type Plan struct {
	ID             string      `json:"id"`
	SessionID      string      `json:"session_id"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Status         string      `json:"status"`
	Archived       bool        `json:"archived"`
	ArchivedAt     string      `json:"archived_at,omitempty"`
	BlockedReason  string      `json:"blocked_reason,omitempty"`
	Priority       int         `json:"priority"`
	UserRequest    string      `json:"user_request"`
	CreatedAt      string      `json:"created_at"`
	UpdatedAt      string      `json:"updated_at"`
	StartedAt      string      `json:"started_at,omitempty"`
	CompletedAt    string      `json:"completed_at,omitempty"`
	Tasks          []PlanTask  `json:"tasks,omitempty"`
	Events         []PlanEvent `json:"events,omitempty"`
	TaskCounts     PlanCounts  `json:"task_counts"`
	ProgressPct    int         `json:"progress_pct"`
	CurrentTaskID  string      `json:"current_task_id,omitempty"`
	CurrentTask    string      `json:"current_task,omitempty"`
	Recommendation string      `json:"recommendation,omitempty"`
}

type PlanTask struct {
	ID            string         `json:"id"`
	PlanID        string         `json:"plan_id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	TaskOrder     int            `json:"task_order"`
	ParentTaskID  string         `json:"parent_task_id,omitempty"`
	Status        string         `json:"status"`
	Kind          string         `json:"kind"`
	ToolName      string         `json:"tool_name,omitempty"`
	ToolArgsJSON  string         `json:"tool_args_json,omitempty"`
	DependsOn     []string       `json:"depends_on,omitempty"`
	Acceptance    string         `json:"acceptance_criteria,omitempty"`
	Owner         string         `json:"owner,omitempty"`
	Artifacts     []PlanArtifact `json:"artifacts,omitempty"`
	BlockerReason string         `json:"blocker_reason,omitempty"`
	ResultSummary string         `json:"result_summary,omitempty"`
	Error         string         `json:"error,omitempty"`
	StartedAt     string         `json:"started_at,omitempty"`
	CompletedAt   string         `json:"completed_at,omitempty"`
}

type PlanTaskInput struct {
	Title        string
	Description  string
	Kind         string
	ToolName     string
	ToolArgs     map[string]interface{}
	DependsOn    []string
	Acceptance   string
	Owner        string
	ParentTaskID string
}

type PlanArtifact struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Value string `json:"value"`
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
	Blocked    int `json:"blocked"`
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
		archived INTEGER NOT NULL DEFAULT 0,
		archived_at DATETIME DEFAULT '',
		blocked_reason TEXT DEFAULT '',
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
		parent_task_id TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		kind TEXT DEFAULT 'task',
		tool_name TEXT DEFAULT '',
		tool_args_json TEXT DEFAULT '',
		depends_on_json TEXT DEFAULT '[]',
		acceptance_criteria TEXT DEFAULT '',
		owner TEXT DEFAULT '',
		artifacts_json TEXT DEFAULT '[]',
		blocker_reason TEXT DEFAULT '',
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
	for _, stmt := range []string{
		`ALTER TABLE plans ADD COLUMN blocked_reason TEXT DEFAULT ''`,
		`ALTER TABLE plans ADD COLUMN archived INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE plans ADD COLUMN archived_at DATETIME DEFAULT ''`,
		`ALTER TABLE plan_tasks ADD COLUMN acceptance_criteria TEXT DEFAULT ''`,
		`ALTER TABLE plan_tasks ADD COLUMN owner TEXT DEFAULT ''`,
		`ALTER TABLE plan_tasks ADD COLUMN artifacts_json TEXT DEFAULT '[]'`,
		`ALTER TABLE plan_tasks ADD COLUMN blocker_reason TEXT DEFAULT ''`,
		`ALTER TABLE plan_tasks ADD COLUMN parent_task_id TEXT DEFAULT ''`,
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return fmt.Errorf("plan schema migrate: %w", err)
		}
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
	case PlanStatusDraft, PlanStatusActive, PlanStatusPaused, PlanStatusBlocked, PlanStatusCompleted, PlanStatusCancelled:
		return true
	default:
		return false
	}
}

func validPlanTaskStatus(status string) bool {
	switch status {
	case PlanTaskPending, PlanTaskInProgress, PlanTaskBlocked, PlanTaskCompleted, PlanTaskFailed, PlanTaskSkipped:
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
	case PlanTaskBlocked:
		return "!"
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
	err := s.db.QueryRow(`SELECT id FROM plans WHERE session_id = ? AND status IN (?, ?, ?, ?) ORDER BY updated_at DESC LIMIT 1`,
		sessionID, PlanStatusDraft, PlanStatusActive, PlanStatusPaused, PlanStatusBlocked).Scan(&existingID)
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
			`INSERT INTO plan_tasks (id, plan_id, title, description, task_order, parent_task_id, status, kind, tool_name, tool_args_json, depends_on_json, acceptance_criteria, owner)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			taskIDs[i], planID, input.Title, input.Description, i+1, input.ParentTaskID, PlanTaskPending, kind, input.ToolName, toolArgsJSON, string(depsJSON), input.Acceptance, input.Owner,
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

func (s *SQLiteMemory) AppendPlanEvent(planID, eventType, message string) error {
	planID = strings.TrimSpace(planID)
	message = strings.TrimSpace(message)
	eventType = strings.TrimSpace(eventType)
	if planID == "" {
		return fmt.Errorf("plan id is required")
	}
	if message == "" {
		return fmt.Errorf("event message is required")
	}
	if eventType == "" {
		eventType = "note"
	}
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM plans WHERE id = ?`, planID).Scan(&exists); err != nil {
		return fmt.Errorf("check plan: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("plan not found")
	}
	now := planNow()
	if _, err := s.db.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, eventType, message, now); err != nil {
		return fmt.Errorf("append plan event: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE plans SET updated_at = ? WHERE id = ?`, now, planID); err != nil {
		return fmt.Errorf("touch plan: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) AttachPlanTaskArtifact(planID, taskID string, artifact PlanArtifact) (*Plan, error) {
	planID = strings.TrimSpace(planID)
	taskID = strings.TrimSpace(taskID)
	artifact.Type = strings.TrimSpace(artifact.Type)
	artifact.Label = strings.TrimSpace(artifact.Label)
	artifact.Value = strings.TrimSpace(artifact.Value)
	if planID == "" || taskID == "" {
		return nil, fmt.Errorf("plan id and task id are required")
	}
	if artifact.Value == "" {
		return nil, fmt.Errorf("artifact value is required")
	}
	if artifact.Type == "" {
		artifact.Type = "artifact"
	}
	if artifact.Label == "" {
		artifact.Label = artifact.Type
	}

	var raw string
	err := s.db.QueryRow(`SELECT artifacts_json FROM plan_tasks WHERE plan_id = ? AND id = ?`, planID, taskID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plan task not found")
		}
		return nil, fmt.Errorf("load task artifacts: %w", err)
	}

	var artifacts []PlanArtifact
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &artifacts)
	}
	for _, existing := range artifacts {
		if existing.Type == artifact.Type && existing.Label == artifact.Label && existing.Value == artifact.Value {
			return s.GetPlan(planID)
		}
	}
	artifacts = append(artifacts, artifact)
	if len(artifacts) > 12 {
		artifacts = artifacts[len(artifacts)-12:]
	}
	encoded, _ := json.Marshal(artifacts)
	now := planNow()
	res, err := s.db.Exec(`UPDATE plan_tasks SET artifacts_json = ? WHERE plan_id = ? AND id = ?`, string(encoded), planID, taskID)
	if err != nil {
		return nil, fmt.Errorf("update task artifacts: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan task not found")
	}
	if _, err := s.db.Exec(`UPDATE plans SET updated_at = ? WHERE id = ?`, now, planID); err != nil {
		return nil, fmt.Errorf("touch plan: %w", err)
	}
	if _, err := s.db.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "artifact", fmt.Sprintf("%s: %s", artifact.Label, artifact.Value), now); err != nil {
		return nil, fmt.Errorf("insert artifact event: %w", err)
	}
	return s.GetPlan(planID)
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
	blockedReason := ""
	switch status {
	case PlanStatusActive:
		startedAt = now
	case PlanStatusBlocked:
		blockedReason = strings.TrimSpace(note)
	case PlanStatusCompleted, PlanStatusCancelled:
		completedAt = now
	}

	res, err := tx.Exec(
		`UPDATE plans
		 SET status = ?, updated_at = ?, started_at = COALESCE(NULLIF(started_at, ''), ?), completed_at = CASE WHEN ? IS NOT NULL THEN ? ELSE completed_at END,
		     blocked_reason = CASE WHEN ? = 'blocked' THEN ? WHEN ? IN ('active','paused','completed','cancelled','draft') THEN '' ELSE blocked_reason END
		 WHERE id = ?`,
		status, now, startedAt, completedAt, completedAt, status, blockedReason, status, planID,
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
	if status == PlanTaskBlocked {
		return nil, fmt.Errorf("use set_blocker to block a task")
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
		     blocker_reason = CASE WHEN ? IN ('completed','failed','skipped','pending','in_progress') THEN '' ELSE blocker_reason END,
		     started_at = CASE WHEN ? != '' THEN ? ELSE started_at END,
		     completed_at = CASE WHEN ? != '' THEN ? ELSE completed_at END
		 WHERE plan_id = ? AND id = ?`,
		status,
		resultSummary, resultSummary,
		taskError, taskError,
		status,
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

func (s *SQLiteMemory) AdvancePlan(planID, resultSummary string) (*Plan, error) {
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var planStatus string
	if err := tx.QueryRow(`SELECT status FROM plans WHERE id = ?`, planID).Scan(&planStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plan not found")
		}
		return nil, fmt.Errorf("load plan: %w", err)
	}
	if planStatus != PlanStatusActive {
		return nil, fmt.Errorf("plan must be active to advance")
	}

	var taskID string
	if err := tx.QueryRow(`SELECT id FROM plan_tasks WHERE plan_id = ? AND status = ? ORDER BY task_order ASC LIMIT 1`, planID, PlanTaskInProgress).Scan(&taskID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no in-progress task to advance")
		}
		return nil, fmt.Errorf("load active task: %w", err)
	}

	res, err := tx.Exec(
		`UPDATE plan_tasks
		 SET status = ?, result_summary = CASE WHEN ? != '' THEN ? ELSE result_summary END, blocker_reason = '', completed_at = ?
		 WHERE plan_id = ? AND id = ?`,
		PlanTaskCompleted, resultSummary, resultSummary, now, planID, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("complete active task: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan task not found")
	}

	eventMessage := "Advanced current task"
	if strings.TrimSpace(resultSummary) != "" {
		eventMessage = strings.TrimSpace(resultSummary)
	}
	if _, err := tx.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "task", eventMessage, now); err != nil {
		return nil, fmt.Errorf("insert task advance event: %w", err)
	}
	if _, err := tx.Exec(`UPDATE plans SET updated_at = ?, blocked_reason = '' WHERE id = ?`, now, planID); err != nil {
		return nil, fmt.Errorf("touch plan: %w", err)
	}
	if err := s.promoteNextPlanTaskTx(tx, planID, now); err != nil {
		return nil, err
	}
	if err := s.finalizeCompletedPlanTx(tx, planID, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit plan advance: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) SetPlanTaskBlocker(planID, taskID, reason string) (*Plan, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("blocker reason is required")
	}
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE plan_tasks
		 SET status = ?, blocker_reason = ?, completed_at = '', error = CASE WHEN error = '' THEN ? ELSE error END
		 WHERE plan_id = ? AND id = ?`,
		PlanTaskBlocked, reason, reason, planID, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("set task blocker: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan task not found")
	}

	planRes, err := tx.Exec(
		`UPDATE plans SET status = ?, blocked_reason = ?, updated_at = ? WHERE id = ?`,
		PlanStatusBlocked, reason, now, planID,
	)
	if err != nil {
		return nil, fmt.Errorf("set plan blocked: %w", err)
	}
	if rows, _ := planRes.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan not found")
	}
	if _, err := tx.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "task_blocked", reason, now); err != nil {
		return nil, fmt.Errorf("insert blocker event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit task blocker: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) ClearPlanTaskBlocker(planID, taskID, note string) (*Plan, error) {
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE plan_tasks
		 SET status = ?, blocker_reason = '', error = CASE WHEN error = blocker_reason THEN '' ELSE error END
		 WHERE plan_id = ? AND id = ? AND status = ?`,
		PlanTaskPending, planID, taskID, PlanTaskBlocked,
	)
	if err != nil {
		return nil, fmt.Errorf("clear task blocker: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("blocked plan task not found")
	}

	planRes, err := tx.Exec(`UPDATE plans SET status = ?, blocked_reason = '', updated_at = ? WHERE id = ?`, PlanStatusActive, now, planID)
	if err != nil {
		return nil, fmt.Errorf("reactivate plan: %w", err)
	}
	if rows, _ := planRes.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("plan not found")
	}

	msg := strings.TrimSpace(note)
	if msg == "" {
		msg = "Cleared task blocker"
	}
	if _, err := tx.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "task_unblocked", msg, now); err != nil {
		return nil, fmt.Errorf("insert unblock event: %w", err)
	}
	if err := s.promoteNextPlanTaskTx(tx, planID, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit clear blocker: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) SplitPlanTask(planID, taskID string, inputs []PlanTaskInput) (*Plan, error) {
	if len(inputs) < 2 {
		return nil, fmt.Errorf("at least two subtasks are required")
	}
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var planStatus, taskStatus, taskTitle, depsJSON string
	var taskOrder int
	if err := tx.QueryRow(
		`SELECT p.status, t.status, t.title, t.task_order, t.depends_on_json
		 FROM plans p
		 JOIN plan_tasks t ON t.plan_id = p.id
		 WHERE p.id = ? AND t.id = ? AND p.archived = 0`,
		planID, taskID,
	).Scan(&planStatus, &taskStatus, &taskTitle, &taskOrder, &depsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plan task not found")
		}
		return nil, fmt.Errorf("load plan task: %w", err)
	}
	if taskStatus == PlanTaskCompleted || taskStatus == PlanTaskFailed || taskStatus == PlanTaskSkipped {
		return nil, fmt.Errorf("completed or closed tasks cannot be split")
	}
	var childCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM plan_tasks WHERE plan_id = ? AND parent_task_id = ?`, planID, taskID).Scan(&childCount); err != nil {
		return nil, fmt.Errorf("count existing subtasks: %w", err)
	}
	if childCount > 0 {
		return nil, fmt.Errorf("task already has subtasks")
	}

	var originalDeps []string
	if strings.TrimSpace(depsJSON) != "" {
		_ = json.Unmarshal([]byte(depsJSON), &originalDeps)
	}

	if _, err := tx.Exec(`UPDATE plan_tasks SET task_order = task_order + ? WHERE plan_id = ? AND task_order > ?`, len(inputs), planID, taskOrder); err != nil {
		return nil, fmt.Errorf("shift task order: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE plan_tasks
		 SET status = ?, blocker_reason = '', result_summary = ?, completed_at = ?, error = CASE WHEN error = blocker_reason THEN '' ELSE error END
		 WHERE plan_id = ? AND id = ?`,
		PlanTaskSkipped, fmt.Sprintf("Split into %d subtasks", len(inputs)), now, planID, taskID,
	); err != nil {
		return nil, fmt.Errorf("mark parent task split: %w", err)
	}

	previousTaskID := ""
	for i, input := range inputs {
		title := strings.TrimSpace(input.Title)
		if title == "" {
			return nil, fmt.Errorf("subtask %d title is required", i+1)
		}
		kind := strings.TrimSpace(input.Kind)
		if kind == "" {
			kind = "task"
		}
		subtaskID := fmt.Sprintf("%s_sub_%s", taskID, uuid.NewString())
		toolArgsJSON := ""
		if len(input.ToolArgs) > 0 {
			b, _ := json.Marshal(input.ToolArgs)
			toolArgsJSON = string(b)
		}
		subDeps := make([]string, 0, 1)
		if i == 0 {
			subDeps = append(subDeps, originalDeps...)
		} else if previousTaskID != "" {
			subDeps = append(subDeps, previousTaskID)
		}
		deps, _ := json.Marshal(subDeps)
		if _, err := tx.Exec(
			`INSERT INTO plan_tasks (id, plan_id, title, description, task_order, parent_task_id, status, kind, tool_name, tool_args_json, depends_on_json, acceptance_criteria, owner)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			subtaskID, planID, title, input.Description, taskOrder+1+i, taskID, PlanTaskPending, kind, input.ToolName, toolArgsJSON, string(deps), input.Acceptance, input.Owner,
		); err != nil {
			return nil, fmt.Errorf("insert subtask %d: %w", i+1, err)
		}
		previousTaskID = subtaskID
	}

	nextPlanStatus := planStatus
	blockedReason := ""
	if planStatus == PlanStatusBlocked {
		nextPlanStatus = PlanStatusActive
	}
	if _, err := tx.Exec(`UPDATE plans SET status = ?, blocked_reason = ?, updated_at = ? WHERE id = ?`, nextPlanStatus, blockedReason, now, planID); err != nil {
		return nil, fmt.Errorf("touch split plan: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "task_split", fmt.Sprintf("Split task %s into %d subtasks", taskTitle, len(inputs)), now); err != nil {
		return nil, fmt.Errorf("insert split event: %w", err)
	}
	if nextPlanStatus == PlanStatusActive {
		if err := s.promoteNextPlanTaskTx(tx, planID, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit split task: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) ReorderPlanTasks(planID string, orderedTaskIDs []string) (*Plan, error) {
	if len(orderedTaskIDs) == 0 {
		return nil, fmt.Errorf("task order is required")
	}
	now := planNow()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id FROM plan_tasks WHERE plan_id = ? ORDER BY task_order ASC`, planID)
	if err != nil {
		return nil, fmt.Errorf("load task order: %w", err)
	}
	defer rows.Close()
	existing := make([]string, 0, len(orderedTaskIDs))
	existingSet := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan task order: %w", err)
		}
		existing = append(existing, id)
		existingSet[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(existing) != len(orderedTaskIDs) {
		return nil, fmt.Errorf("task order must include every task exactly once")
	}
	seen := map[string]bool{}
	for _, id := range orderedTaskIDs {
		if !existingSet[id] || seen[id] {
			return nil, fmt.Errorf("task order must include every task exactly once")
		}
		seen[id] = true
	}
	for i, id := range orderedTaskIDs {
		if _, err := tx.Exec(`UPDATE plan_tasks SET task_order = ? WHERE plan_id = ? AND id = ?`, i+1, planID, id); err != nil {
			return nil, fmt.Errorf("reorder task %s: %w", id, err)
		}
	}
	if _, err := tx.Exec(`UPDATE plans SET updated_at = ? WHERE id = ? AND archived = 0`, now, planID); err != nil {
		return nil, fmt.Errorf("touch reordered plan: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "reordered", "Reordered plan tasks", now); err != nil {
		return nil, fmt.Errorf("insert reorder event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reorder tasks: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) ArchivePlan(planID string) (*Plan, error) {
	now := planNow()
	var plan Plan
	if err := s.db.QueryRow(`SELECT id, session_id, title, description, status, archived, archived_at, blocked_reason, priority, user_request, created_at, updated_at, started_at, completed_at FROM plans WHERE id = ?`,
		planID).Scan(&plan.ID, &plan.SessionID, &plan.Title, &plan.Description, &plan.Status, &plan.Archived, &plan.ArchivedAt, &plan.BlockedReason, &plan.Priority, &plan.UserRequest, &plan.CreatedAt, &plan.UpdatedAt, &plan.StartedAt, &plan.CompletedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plan not found")
		}
		return nil, fmt.Errorf("load plan for archive: %w", err)
	}
	if plan.Archived {
		return s.GetPlan(planID)
	}
	if plan.Status != PlanStatusCompleted && plan.Status != PlanStatusCancelled {
		return nil, fmt.Errorf("only completed or cancelled plans can be archived")
	}
	if _, err := s.db.Exec(`UPDATE plans SET archived = 1, archived_at = ?, updated_at = ? WHERE id = ?`, now, now, planID); err != nil {
		return nil, fmt.Errorf("archive plan: %w", err)
	}
	if _, err := s.db.Exec(`INSERT INTO plan_events (plan_id, event_type, message, created_at) VALUES (?, ?, ?, ?)`, planID, "archived", "Plan archived", now); err != nil {
		return nil, fmt.Errorf("insert archive event: %w", err)
	}
	return s.GetPlan(planID)
}

func (s *SQLiteMemory) ArchiveCompletedPlans(sessionID string) (int, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	rows, err := s.db.Query(`SELECT id FROM plans WHERE session_id = ? AND archived = 0 AND status IN (?, ?)`, sessionID, PlanStatusCompleted, PlanStatusCancelled)
	if err != nil {
		return 0, fmt.Errorf("list completed plans: %w", err)
	}
	var planIDs []string
	for rows.Next() {
		var planID string
		if err := rows.Scan(&planID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan completed plan: %w", err)
		}
		planIDs = append(planIDs, planID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	var count int
	for _, planID := range planIDs {
		if _, err := s.ArchivePlan(planID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
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

func (s *SQLiteMemory) ListPlans(sessionID, status string, limit int, includeArchived bool) ([]Plan, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := `SELECT id, session_id, title, description, status, archived, archived_at, blocked_reason, priority, user_request, created_at, updated_at, started_at, completed_at
	          FROM plans WHERE session_id = ?`
	args := []interface{}{sessionID}
	if !includeArchived {
		query += ` AND archived = 0`
	}
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
	var plans []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Title, &p.Description, &p.Status, &p.Archived, &p.ArchivedAt, &p.BlockedReason, &p.Priority, &p.UserRequest, &p.CreatedAt, &p.UpdatedAt, &p.StartedAt, &p.CompletedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range plans {
		if err := s.populatePlanComputedFields(&plans[i], false); err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func (s *SQLiteMemory) GetPlan(planID string) (*Plan, error) {
	var p Plan
	err := s.db.QueryRow(
		`SELECT id, session_id, title, description, status, archived, archived_at, blocked_reason, priority, user_request, created_at, updated_at, started_at, completed_at
		 FROM plans WHERE id = ?`,
		planID,
	).Scan(&p.ID, &p.SessionID, &p.Title, &p.Description, &p.Status, &p.Archived, &p.ArchivedAt, &p.BlockedReason, &p.Priority, &p.UserRequest, &p.CreatedAt, &p.UpdatedAt, &p.StartedAt, &p.CompletedAt)
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
		 WHERE session_id = ? AND archived = 0 AND status IN (?, ?, ?, ?)
		 ORDER BY updated_at DESC LIMIT 1`,
		sessionID, PlanStatusActive, PlanStatusBlocked, PlanStatusPaused, PlanStatusDraft,
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
	if plan.Archived {
		sb.WriteString(fmt.Sprintf("Archived: %s\n", plan.ArchivedAt))
	}
	if plan.CurrentTask != "" {
		sb.WriteString(fmt.Sprintf("Current task: %s\n", plan.CurrentTask))
	}
	if strings.TrimSpace(plan.BlockedReason) != "" {
		sb.WriteString(fmt.Sprintf("Blocked: %s\n", plan.BlockedReason))
	}
	if strings.TrimSpace(plan.Recommendation) != "" {
		sb.WriteString(fmt.Sprintf("Recommended next step: %s\n", plan.Recommendation))
	}
	if len(plan.Events) > 0 {
		sb.WriteString("Recent plan context:\n")
		maxEvents := 3
		if len(plan.Events) < maxEvents {
			maxEvents = len(plan.Events)
		}
		for i := 0; i < maxEvents; i++ {
			sb.WriteString(fmt.Sprintf("- %s\n", plan.Events[i].Message))
		}
	}
	for _, task := range plan.Tasks {
		prefix := "- "
		if strings.TrimSpace(task.ParentTaskID) != "" {
			prefix = "  - "
		}
		sb.WriteString(fmt.Sprintf("%s[%s] %s\n", prefix, planTaskStatusIcon(task.Status), task.Title))
		if strings.TrimSpace(task.BlockerReason) != "" {
			sb.WriteString(fmt.Sprintf("  blocker: %s\n", task.BlockerReason))
		}
		if task.ID == plan.CurrentTaskID && len(task.Artifacts) > 0 {
			maxArtifacts := 2
			if len(task.Artifacts) < maxArtifacts {
				maxArtifacts = len(task.Artifacts)
			}
			for i := len(task.Artifacts) - maxArtifacts; i < len(task.Artifacts); i++ {
				sb.WriteString(fmt.Sprintf("  artifact: %s = %s\n", task.Artifacts[i].Label, task.Artifacts[i].Value))
			}
		}
	}
	return sb.String(), nil
}

func planRecommendation(plan *Plan) string {
	if plan == nil {
		return ""
	}
	switch plan.Status {
	case PlanStatusDraft:
		return "Review the draft and activate the plan when the tasks look right."
	case PlanStatusPaused:
		if plan.CurrentTask != "" {
			return fmt.Sprintf("Resume the paused work with: %s.", plan.CurrentTask)
		}
		if next := nextRecommendedTask(plan.Tasks); next != nil {
			return fmt.Sprintf("Resume the next pending task: %s.", next.Title)
		}
		return "Resume the plan or archive it if the work is no longer needed."
	case PlanStatusBlocked:
		if task := currentOrBlockedTask(plan.Tasks, plan.CurrentTaskID); task != nil && strings.TrimSpace(task.BlockerReason) != "" {
			return fmt.Sprintf("Resolve the blocker for %s: %s", task.Title, task.BlockerReason)
		}
		if strings.TrimSpace(plan.BlockedReason) != "" {
			return fmt.Sprintf("Resolve the plan blocker: %s", plan.BlockedReason)
		}
		return "Resolve the blocker before continuing."
	case PlanStatusCompleted:
		return "The plan is complete. Archive it if you want to keep the list clean."
	case PlanStatusCancelled:
		return "The plan is cancelled. Archive it if you do not need to revisit it."
	case PlanStatusActive:
		if task := currentOrBlockedTask(plan.Tasks, plan.CurrentTaskID); task != nil {
			if task.Status == PlanTaskInProgress {
				return fmt.Sprintf("Continue the current task: %s.", task.Title)
			}
			if task.Status == PlanTaskBlocked && strings.TrimSpace(task.BlockerReason) != "" {
				return fmt.Sprintf("Resolve the blocker for %s: %s", task.Title, task.BlockerReason)
			}
		}
		if next := nextRecommendedTask(plan.Tasks); next != nil {
			return fmt.Sprintf("Start the next ready task: %s.", next.Title)
		}
	}
	return ""
}

func currentOrBlockedTask(tasks []PlanTask, taskID string) *PlanTask {
	for i := range tasks {
		if taskID != "" && tasks[i].ID == taskID {
			return &tasks[i]
		}
	}
	for i := range tasks {
		if tasks[i].Status == PlanTaskBlocked {
			return &tasks[i]
		}
	}
	return nil
}

func nextRecommendedTask(tasks []PlanTask) *PlanTask {
	for i := range tasks {
		if tasks[i].Status == PlanTaskPending || tasks[i].Status == PlanTaskInProgress {
			return &tasks[i]
		}
	}
	return nil
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
	if err := tx.QueryRow(`SELECT COUNT(*) FROM plan_tasks WHERE plan_id = ? AND status IN (?, ?, ?, ?)`, planID, PlanTaskPending, PlanTaskInProgress, PlanTaskBlocked, PlanTaskFailed).Scan(&remaining); err != nil {
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
		`SELECT id, plan_id, title, description, task_order, parent_task_id, status, kind, tool_name, tool_args_json, depends_on_json, acceptance_criteria, owner, artifacts_json, blocker_reason, result_summary, error, started_at, completed_at
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
		var depsJSON, artifactsJSON string
		if err := rows.Scan(&task.ID, &task.PlanID, &task.Title, &task.Description, &task.TaskOrder, &task.ParentTaskID, &task.Status, &task.Kind, &task.ToolName, &task.ToolArgsJSON, &depsJSON, &task.Acceptance, &task.Owner, &artifactsJSON, &task.BlockerReason, &task.ResultSummary, &task.Error, &task.StartedAt, &task.CompletedAt); err != nil {
			return fmt.Errorf("scan plan task: %w", err)
		}
		_ = json.Unmarshal([]byte(depsJSON), &task.DependsOn)
		_ = json.Unmarshal([]byte(artifactsJSON), &task.Artifacts)
		tasks = append(tasks, task)
		counts.Total++
		switch task.Status {
		case PlanTaskPending:
			counts.Pending++
		case PlanTaskInProgress:
			counts.InProgress++
			plan.CurrentTaskID = task.ID
			plan.CurrentTask = task.Title
		case PlanTaskBlocked:
			counts.Blocked++
			if plan.CurrentTask == "" {
				plan.CurrentTaskID = task.ID
				plan.CurrentTask = task.Title
			}
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
	plan.Recommendation = planRecommendation(plan)

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
