package planner

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

type KnowledgeGraph interface {
	AddNode(id, name string, properties map[string]string) error
	DeleteNode(id string) error
}

// ParticipantSummary is a compact contact reference embedded in appointments.
type ParticipantSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Relationship string `json:"relationship,omitempty"`
	Email        string `json:"email,omitempty"`
}

// Appointment represents a calendar appointment.
type Appointment struct {
	ID               string               `json:"id"`
	Title            string               `json:"title"`
	Description      string               `json:"description,omitempty"`
	DateTime         string               `json:"date_time"`
	NotificationAt   string               `json:"notification_at,omitempty"`
	WakeAgent        bool                 `json:"wake_agent"`
	AgentInstruction string               `json:"agent_instruction,omitempty"`
	Notified         bool                 `json:"notified"`
	Status           string               `json:"status"` // upcoming, completed, cancelled
	KGNodeID         string               `json:"kg_node_id,omitempty"`
	ContactIDs       []string             `json:"contact_ids"`
	Participants     []ParticipantSummary `json:"participants"`
	CreatedAt        string               `json:"created_at"`
	UpdatedAt        string               `json:"updated_at"`
}

// Todo represents a to-do item.
type Todo struct {
	ID                  string     `json:"id"`
	Title               string     `json:"title"`
	Description         string     `json:"description,omitempty"`
	Priority            string     `json:"priority"` // low, medium, high
	Status              string     `json:"status"`   // open, in_progress, done
	DueDate             string     `json:"due_date,omitempty"`
	RemindDaily         bool       `json:"remind_daily"`
	CompletedAt         string     `json:"completed_at,omitempty"`
	LastDailyReminderAt string     `json:"last_daily_reminder_at,omitempty"`
	Items               []TodoItem `json:"items,omitempty"`
	ItemCount           int        `json:"item_count"`
	DoneItemCount       int        `json:"done_item_count"`
	ProgressPercent     int        `json:"progress_percent"`
	KGNodeID            string     `json:"kg_node_id,omitempty"`
	CreatedAt           string     `json:"created_at"`
	UpdatedAt           string     `json:"updated_at"`
}

// TodoItem represents an optional checklist item inside a todo.
type TodoItem struct {
	ID          string `json:"id"`
	TodoID      string `json:"todo_id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Position    int    `json:"position"`
	IsDone      bool   `json:"is_done"`
	CompletedAt string `json:"completed_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// PromptSnapshotOptions controls how much planner data is included in prompt injections.
type PromptSnapshotOptions struct {
	TodoLimit         int
	AppointmentLimit  int
	AppointmentWindow time.Duration
}

// PromptSnapshot is a compact planner summary intended for prompt injection.
type PromptSnapshot struct {
	GeneratedAt       time.Time     `json:"generated_at"`
	OpenTodoCount     int           `json:"open_todo_count"`
	OverdueTodoCount  int           `json:"overdue_todo_count"`
	Todos             []Todo        `json:"todos"`
	Appointments      []Appointment `json:"appointments"`
	AppointmentWindow time.Duration `json:"appointment_window"`
}

// DailyReminderSnapshot is the planner summary used for the once-per-day proactive reminder.
type DailyReminderSnapshot struct {
	GeneratedAt      time.Time    `json:"generated_at"`
	OpenTodoCount    int          `json:"open_todo_count"`
	OverdueTodoCount int          `json:"overdue_todo_count"`
	Todos            []Todo       `json:"todos"`
	NextAppointment  *Appointment `json:"next_appointment,omitempty"`
}

const dailyTodoReminderMetaKey = "daily_todo_reminder_last_seen"

const (
	defaultPromptTodoLimit        = 10
	defaultPromptAppointmentLimit = 5
	defaultReminderTodoLimit      = 4
)

// InitDB initializes the planner SQLite database with appointments and todos tables.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open planner database: %w", err)
	}

	if err := initPlannerSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// ── Appointment CRUD ──

// validAppointmentStatus returns true for allowed appointment status values.
func validAppointmentStatus(s string) bool {
	return s == "upcoming" || s == "completed" || s == "cancelled" || s == "overdue"
}

func validTodoPriority(p string) bool {
	return p == "low" || p == "medium" || p == "high"
}

func validTodoStatus(s string) bool {
	return s == "open" || s == "in_progress" || s == "done"
}

func validRFC3339(s string) bool {
	if s == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

// normalizeDateInput converts a date-only string (YYYY-MM-DD) to RFC3339 by appending T00:00:00Z.
// If the input is already RFC3339, it is returned unchanged.
func normalizeDateInput(s string) string {
	if s == "" {
		return s
	}
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return s // already valid RFC3339
	}
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return s + "T00:00:00Z"
	}
	return s // return unchanged; validation will catch the error
}

func priorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}

func parsePlannerTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, true
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func isTodoOpenStatus(status string) bool {
	return status == "open" || status == "in_progress"
}

func isTodoOverdue(todo Todo, now time.Time) bool {
	if !isTodoOpenStatus(todo.Status) {
		return false
	}
	due, ok := parsePlannerTime(todo.DueDate)
	if !ok {
		return false
	}
	return due.Before(now)
}

func sortTodosForSnapshot(todos []Todo, now time.Time) {
	sort.SliceStable(todos, func(i, j int) bool {
		leftOverdue := isTodoOverdue(todos[i], now)
		rightOverdue := isTodoOverdue(todos[j], now)
		if leftOverdue != rightOverdue {
			return leftOverdue
		}

		leftPriority := priorityRank(todos[i].Priority)
		rightPriority := priorityRank(todos[j].Priority)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}

		leftDue, leftHasDue := parsePlannerTime(todos[i].DueDate)
		rightDue, rightHasDue := parsePlannerTime(todos[j].DueDate)
		if leftHasDue != rightHasDue {
			return leftHasDue
		}
		if leftHasDue && !leftDue.Equal(rightDue) {
			return leftDue.Before(rightDue)
		}

		leftCreated, leftHasCreated := parsePlannerTime(todos[i].CreatedAt)
		rightCreated, rightHasCreated := parsePlannerTime(todos[j].CreatedAt)
		if leftHasCreated && rightHasCreated && !leftCreated.Equal(rightCreated) {
			return leftCreated.Before(rightCreated)
		}

		return todos[i].Title < todos[j].Title
	})
}

func normalizePromptSnapshotOptions(opts PromptSnapshotOptions) PromptSnapshotOptions {
	if opts.TodoLimit <= 0 {
		opts.TodoLimit = defaultPromptTodoLimit
	}
	if opts.AppointmentLimit <= 0 {
		opts.AppointmentLimit = defaultPromptAppointmentLimit
	}
	if opts.AppointmentWindow <= 0 {
		opts.AppointmentWindow = 48 * time.Hour
	}
	return opts
}

func collectOpenTodos(list []Todo, now time.Time, limit int) ([]Todo, int, int) {
	filtered := make([]Todo, 0, len(list))
	overdueCount := 0
	for _, todo := range list {
		if !isTodoOpenStatus(todo.Status) {
			continue
		}
		if isTodoOverdue(todo, now) {
			overdueCount++
		}
		filtered = append(filtered, todo)
	}
	sortTodosForSnapshot(filtered, now)
	openCount := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, openCount, overdueCount
}

func collectUpcomingAppointments(list []Appointment, start, end time.Time, limit int) []Appointment {
	filtered := make([]Appointment, 0, len(list))
	for _, appointment := range list {
		if appointment.Status != "upcoming" {
			continue
		}
		when, ok := parsePlannerTime(appointment.DateTime)
		if !ok {
			continue
		}
		if when.Before(start) {
			continue
		}
		if !end.IsZero() && when.After(end) {
			continue
		}
		filtered = append(filtered, appointment)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, _ := parsePlannerTime(filtered[i].DateTime)
		right, _ := parsePlannerTime(filtered[j].DateTime)
		if !left.Equal(right) {
			return left.Before(right)
		}
		return filtered[i].Title < filtered[j].Title
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// BuildPromptSnapshot returns a compact planner snapshot for prompt injection.
func BuildPromptSnapshot(db *sql.DB, now time.Time, opts PromptSnapshotOptions) (PromptSnapshot, error) {
	snapshot := PromptSnapshot{GeneratedAt: now.UTC()}
	if db == nil {
		return snapshot, nil
	}
	opts = normalizePromptSnapshotOptions(opts)

	todos, err := ListTodos(db, "", "")
	if err != nil {
		return snapshot, err
	}
	snapshot.Todos, snapshot.OpenTodoCount, snapshot.OverdueTodoCount = collectOpenTodos(todos, now, opts.TodoLimit)

	appointments, err := ListAppointments(db, "", "upcoming")
	if err != nil {
		return snapshot, err
	}
	snapshot.AppointmentWindow = opts.AppointmentWindow
	snapshot.Appointments = collectUpcomingAppointments(appointments, now, now.Add(opts.AppointmentWindow), opts.AppointmentLimit)
	return snapshot, nil
}

// BuildPromptContextText formats a planner snapshot for system-prompt injection.
func BuildPromptContextText(snapshot PromptSnapshot) string {
	if snapshot.OpenTodoCount == 0 && len(snapshot.Appointments) == 0 {
		return ""
	}

	var builder strings.Builder
	if snapshot.OpenTodoCount > 0 {
		builder.WriteString(fmt.Sprintf("Open todos: %d", snapshot.OpenTodoCount))
		if snapshot.OverdueTodoCount > 0 {
			builder.WriteString(fmt.Sprintf(" (%d overdue)", snapshot.OverdueTodoCount))
		}
		builder.WriteString("\n")
		for _, todo := range snapshot.Todos {
			builder.WriteString("- [")
			builder.WriteString(strings.ToUpper(strings.TrimSpace(todo.Priority)))
			builder.WriteString("] ")
			builder.WriteString(strings.TrimSpace(todo.Title))
			if todo.Status == "in_progress" {
				builder.WriteString(" (in progress)")
			}
			if todo.DueDate != "" {
				builder.WriteString(" (due: ")
				builder.WriteString(todo.DueDate)
				builder.WriteString(")")
			}
			builder.WriteString("\n")
		}
	}

	if len(snapshot.Appointments) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		hours := int(snapshot.AppointmentWindow.Hours())
		if hours <= 0 {
			hours = 48
		}
		builder.WriteString(fmt.Sprintf("Upcoming appointments (next %dh):\n", hours))
		for _, appointment := range snapshot.Appointments {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(appointment.DateTime))
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(appointment.Title))
			builder.WriteString("\n")
		}
	}

	return strings.TrimSpace(builder.String())
}

// CreateAppointment adds a new appointment and returns its ID.
func CreateAppointment(db *sql.DB, a Appointment) (string, error) {
	if a.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	if a.DateTime == "" {
		return "", fmt.Errorf("date_time is required")
	}
	if !validRFC3339(a.DateTime) {
		return "", fmt.Errorf("invalid date_time format %q: must be RFC3339 (e.g. 2025-03-15T14:00:00Z)", a.DateTime)
	}
	if a.NotificationAt != "" && !validRFC3339(a.NotificationAt) {
		return "", fmt.Errorf("invalid notification_at format %q: must be RFC3339", a.NotificationAt)
	}
	a.ID = uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "upcoming"
	}
	if !validAppointmentStatus(a.Status) {
		return "", fmt.Errorf("invalid status %q: must be upcoming, completed, or cancelled", a.Status)
	}
	a.KGNodeID = "appointment_" + a.ID

	_, err := db.Exec(
		`INSERT INTO appointments (id, title, description, date_time, notification_at, wake_agent, agent_instruction, notified, status, kg_node_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Title, a.Description, a.DateTime, a.NotificationAt,
		dbutil.BoolToInt(a.WakeAgent), a.AgentInstruction, dbutil.BoolToInt(a.Notified),
		a.Status, a.KGNodeID, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert appointment: %w", err)
	}
	return a.ID, nil
}

// UpdateAppointment modifies an existing appointment.
func UpdateAppointment(db *sql.DB, a Appointment) error {
	if a.ID == "" {
		return fmt.Errorf("id is required")
	}
	if a.DateTime != "" && !validRFC3339(a.DateTime) {
		return fmt.Errorf("invalid date_time format %q: must be RFC3339 (e.g. 2025-03-15T14:00:00Z)", a.DateTime)
	}
	if a.NotificationAt != "" && !validRFC3339(a.NotificationAt) {
		return fmt.Errorf("invalid notification_at format %q: must be RFC3339", a.NotificationAt)
	}
	if !validAppointmentStatus(a.Status) {
		return fmt.Errorf("invalid status %q: must be upcoming, completed, or cancelled", a.Status)
	}
	// BUG-3: Reset notified when notification_at changes so the new time triggers a notification.
	var currentNotificationAt string
	_ = db.QueryRow("SELECT notification_at FROM appointments WHERE id=?", a.ID).Scan(&currentNotificationAt)
	if a.NotificationAt != currentNotificationAt {
		a.Notified = false
	}
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	res, err := db.Exec(
		`UPDATE appointments SET title=?, description=?, date_time=?, notification_at=?, wake_agent=?, agent_instruction=?, notified=?, status=?, updated_at=?
		 WHERE id=?`,
		a.Title, a.Description, a.DateTime, a.NotificationAt,
		dbutil.BoolToInt(a.WakeAgent), a.AgentInstruction, dbutil.BoolToInt(a.Notified),
		a.Status, a.UpdatedAt, a.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update appointment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("appointment not found: %s", a.ID)
	}
	return nil
}

// DeleteAppointment removes an appointment by ID.
func DeleteAppointment(db *sql.DB, id string) error {
	res, err := db.Exec("DELETE FROM appointments WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("failed to delete appointment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("appointment not found: %s", id)
	}
	return nil
}

// GetAppointment returns a single appointment by ID.
func GetAppointment(db *sql.DB, id string) (*Appointment, error) {
	row := db.QueryRow(
		`SELECT id, title, description, date_time, notification_at, wake_agent, agent_instruction, notified, status, kg_node_id, created_at, updated_at
		 FROM appointments WHERE id=?`, id)
	return scanAppointment(row)
}

// ListAppointments returns appointments filtered by search query and status.
func ListAppointments(db *sql.DB, query, status string) ([]Appointment, error) {
	if db != nil {
		_ = AutoExpireAppointments(db)
	}
	var conditions []string
	var args []interface{}

	if query != "" {
		escapedQuery := dbutil.EscapeLike(strings.ToLower(query))
		like := "%" + escapedQuery + "%"
		conditions = append(conditions, "(lower(title) LIKE ? ESCAPE '\\' OR lower(description) LIKE ? ESCAPE '\\')")
		args = append(args, like, like)
	}
	if status != "" && status != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	q := `SELECT id, title, description, date_time, notification_at, wake_agent, agent_instruction, notified, status, kg_node_id, created_at, updated_at FROM appointments`
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}
	q += " ORDER BY date_time ASC"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list appointments: %w", err)
	}
	defer rows.Close()

	var list []Appointment
	for rows.Next() {
		var a Appointment
		var wakeAgent, notified int
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.DateTime, &a.NotificationAt,
			&wakeAgent, &a.AgentInstruction, &notified, &a.Status, &a.KGNodeID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan appointment: %w", err)
		}
		a.WakeAgent = wakeAgent != 0
		a.Notified = notified != 0
		list = append(list, a)
	}
	if list == nil {
		list = []Appointment{}
	}
	return list, nil
}

// AutoExpireAppointments updates the status of past appointments from 'upcoming' to 'overdue'.
func AutoExpireAppointments(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE appointments SET status = 'overdue', updated_at = ? WHERE status = 'upcoming' AND date_time < ?`,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to auto-expire appointments: %w", err)
	}
	return nil
}

// GetDueNotifications returns appointments due for notification that have not been notified yet.
// Limited to 50 per tick to avoid burst load after downtime.
// Includes all appointments with a due notification_at, regardless of wake_agent setting.
func GetDueNotifications(db *sql.DB) ([]Appointment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := db.Query(
		`SELECT id, title, description, date_time, notification_at, wake_agent, agent_instruction, notified, status, kg_node_id, created_at, updated_at
		 FROM appointments
		 WHERE notification_at != '' AND notification_at <= ? AND notified = 0 AND status = 'upcoming'
		 ORDER BY notification_at ASC
		 LIMIT 50`, now)
	if err != nil {
		return nil, fmt.Errorf("failed to query due notifications: %w", err)
	}
	defer rows.Close()

	var list []Appointment
	for rows.Next() {
		var a Appointment
		var wakeAgent, notified int
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.DateTime, &a.NotificationAt,
			&wakeAgent, &a.AgentInstruction, &notified, &a.Status, &a.KGNodeID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		a.WakeAgent = wakeAgent != 0
		a.Notified = notified != 0
		list = append(list, a)
	}
	return list, nil
}

// MarkNotified marks an appointment as notified.
func MarkNotified(db *sql.DB, id string) error {
	res, err := db.Exec("UPDATE appointments SET notified = 1 WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("appointment not found: %s", id)
	}
	return nil
}

// ClaimNotification atomically marks an appointment as notified using compare-and-swap.
// Returns true if this call was the first to claim (rows affected = 1), false if already claimed.
// This prevents duplicate notifications when multiple goroutines race on the same appointment.
func ClaimNotification(db *sql.DB, id string) (bool, error) {
	res, err := db.Exec("UPDATE appointments SET notified = 1 WHERE id = ? AND notified = 0", id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ── Todo CRUD ──

// CreateTodo adds a new todo and returns its ID.
func CreateTodo(db *sql.DB, t Todo) (string, error) {
	if t.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	t.ID = uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = "open"
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	if !validTodoStatus(t.Status) {
		return "", fmt.Errorf("invalid status %q: must be open, in_progress, or done", t.Status)
	}
	if !validTodoPriority(t.Priority) {
		return "", fmt.Errorf("invalid priority %q: must be low, medium, or high", t.Priority)
	}
	if t.DueDate != "" {
		t.DueDate = normalizeDateInput(t.DueDate)
		if !validRFC3339(t.DueDate) {
			return "", fmt.Errorf("invalid due_date format %q: must be RFC3339 (e.g. 2025-03-15T00:00:00Z) or date-only (e.g. 2025-03-15)", t.DueDate)
		}
	}
	if err := prepareTodoForPersistence(&t, now); err != nil {
		return "", err
	}
	t.KGNodeID = "todo_" + t.ID

	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin todo insert: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO todos (id, title, description, priority, status, due_date, remind_daily, completed_at, last_daily_reminder_at, kg_node_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Priority, t.Status, t.DueDate, dbutil.BoolToInt(t.RemindDaily), t.CompletedAt, t.LastDailyReminderAt, t.KGNodeID, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert todo: %w", err)
	}
	if err := replaceTodoItemsTx(tx, t.ID, t.Items, now); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit todo insert: %w", err)
	}
	return t.ID, nil
}

// UpdateTodo modifies an existing todo.
func UpdateTodo(db *sql.DB, t Todo) error {
	if t.ID == "" {
		return fmt.Errorf("id is required")
	}
	if !validTodoStatus(t.Status) {
		return fmt.Errorf("invalid status %q: must be open, in_progress, or done", t.Status)
	}
	if !validTodoPriority(t.Priority) {
		return fmt.Errorf("invalid priority %q: must be low, medium, or high", t.Priority)
	}
	if t.DueDate != "" {
		t.DueDate = normalizeDateInput(t.DueDate)
		if !validRFC3339(t.DueDate) {
			return fmt.Errorf("invalid due_date format %q: must be RFC3339 (e.g. 2025-03-15T00:00:00Z) or date-only (e.g. 2025-03-15)", t.DueDate)
		}
	}
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := prepareTodoForPersistence(&t, t.UpdatedAt); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin todo update: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE todos SET title=?, description=?, priority=?, status=?, due_date=?, remind_daily=?, completed_at=?, last_daily_reminder_at=?, updated_at=?
		 WHERE id=?`,
		t.Title, t.Description, t.Priority, t.Status, t.DueDate, dbutil.BoolToInt(t.RemindDaily), t.CompletedAt, t.LastDailyReminderAt, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", t.ID)
	}
	if t.Items != nil {
		if err := replaceTodoItemsTx(tx, t.ID, t.Items, t.UpdatedAt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit todo update: %w", err)
	}
	return nil
}

// DeleteTodo removes a todo by ID.
func DeleteTodo(db *sql.DB, id string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin todo delete: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM todo_items WHERE todo_id=?", id); err != nil {
		return fmt.Errorf("failed to delete todo items: %w", err)
	}
	res, err := tx.Exec("DELETE FROM todos WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", id)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit todo delete: %w", err)
	}
	return nil
}

// GetTodo returns a single todo by ID.
func GetTodo(db *sql.DB, id string) (*Todo, error) {
	row := db.QueryRow(
		`SELECT id, title, description, priority, status, due_date, remind_daily, completed_at, last_daily_reminder_at, kg_node_id, created_at, updated_at
		 FROM todos WHERE id=?`, id)
	t := &Todo{}
	var remindDaily int
	if err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.DueDate, &remindDaily, &t.CompletedAt, &t.LastDailyReminderAt, &t.KGNodeID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("todo not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}
	t.RemindDaily = remindDaily != 0
	items, err := listTodoItems(db, t.ID)
	if err != nil {
		return nil, err
	}
	t.Items = items
	ComputeTodoProgress(t)
	return t, nil
}

// ListTodos returns todos filtered by search query and status.
func ListTodos(db *sql.DB, query, status string) ([]Todo, error) {
	var conditions []string
	var args []interface{}

	if query != "" {
		escapedQuery := dbutil.EscapeLike(strings.ToLower(query))
		like := "%" + escapedQuery + "%"
		conditions = append(conditions, "(lower(title) LIKE ? ESCAPE '\\' OR lower(description) LIKE ? ESCAPE '\\')")
		args = append(args, like, like)
	}
	if status != "" && status != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	q := `SELECT id, title, description, priority, status, due_date, remind_daily, completed_at, last_daily_reminder_at, kg_node_id, created_at, updated_at FROM todos`
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}
	q += " ORDER BY CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, due_date ASC"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}
	list := []Todo{}
	for rows.Next() {
		var t Todo
		var remindDaily int
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.DueDate, &remindDaily, &t.CompletedAt, &t.LastDailyReminderAt, &t.KGNodeID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		t.RemindDaily = remindDaily != 0
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate todos: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close todo rows: %w", err)
	}

	for index := range list {
		items, err := listTodoItems(db, list[index].ID)
		if err != nil {
			return nil, err
		}
		list[index].Items = items
		ComputeTodoProgress(&list[index])
	}
	if list == nil {
		list = []Todo{}
	}
	return list, nil
}

// AddTodoItem adds a subtask to an existing todo.
func AddTodoItem(db *sql.DB, todoID string, item TodoItem) (string, error) {
	if strings.TrimSpace(todoID) == "" {
		return "", fmt.Errorf("todo_id is required")
	}
	if _, err := GetTodo(db, todoID); err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items, err := listTodoItems(db, todoID)
	if err != nil {
		return "", err
	}
	item.ID = uid.New()
	item.TodoID = todoID
	if err := prepareTodoItem(&item, len(items), now); err != nil {
		return "", err
	}
	_, err = db.Exec(
		`INSERT INTO todo_items (id, todo_id, title, description, position, is_done, completed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.TodoID, item.Title, item.Description, item.Position, dbutil.BoolToInt(item.IsDone), item.CompletedAt, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert todo item: %w", err)
	}
	if err := syncTodoDerivedState(db, todoID, now); err != nil {
		return "", err
	}
	return item.ID, nil
}

// UpdateTodoItem updates a subtask within a todo.
func UpdateTodoItem(db *sql.DB, item TodoItem) error {
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("item id is required")
	}
	if strings.TrimSpace(item.TodoID) == "" {
		return fmt.Errorf("todo_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := prepareTodoItem(&item, item.Position, now); err != nil {
		return err
	}
	res, err := db.Exec(
		`UPDATE todo_items SET title=?, description=?, position=?, is_done=?, completed_at=?, updated_at=?
		 WHERE id=? AND todo_id=?`,
		item.Title, item.Description, item.Position, dbutil.BoolToInt(item.IsDone), item.CompletedAt, item.UpdatedAt, item.ID, item.TodoID,
	)
	if err != nil {
		return fmt.Errorf("failed to update todo item: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo item not found: %s", item.ID)
	}
	return syncTodoDerivedState(db, item.TodoID, now)
}

// DeleteTodoItem removes a subtask from a todo.
func DeleteTodoItem(db *sql.DB, todoID, itemID string) error {
	res, err := db.Exec("DELETE FROM todo_items WHERE id=? AND todo_id=?", itemID, todoID)
	if err != nil {
		return fmt.Errorf("failed to delete todo item: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo item not found: %s", itemID)
	}
	return syncTodoDerivedState(db, todoID, time.Now().UTC().Format(time.RFC3339))
}

// ReorderTodoItems updates item positions inside a todo.
func ReorderTodoItems(db *sql.DB, todoID string, itemIDs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin todo item reorder: %w", err)
	}
	defer tx.Rollback()

	for position, itemID := range itemIDs {
		res, err := tx.Exec("UPDATE todo_items SET position=?, updated_at=? WHERE id=? AND todo_id=?", position, time.Now().UTC().Format(time.RFC3339), itemID, todoID)
		if err != nil {
			return fmt.Errorf("failed to reorder todo item: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("todo item not found: %s", itemID)
		}
	}
	if err := syncTodoDerivedStateTx(tx, todoID, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit todo item reorder: %w", err)
	}
	return nil
}

// CompleteTodo marks a todo as done and optionally auto-completes all remaining items.
func CompleteTodo(db *sql.DB, todoID string, completeItems bool) error {
	todo, err := GetTodo(db, todoID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	todo.Status = "done"
	todo.UpdatedAt = now
	if completeItems {
		for index := range todo.Items {
			todo.Items[index].IsDone = true
			todo.Items[index].CompletedAt = now
			todo.Items[index].UpdatedAt = now
		}
	}
	return UpdateTodo(db, *todo)
}

// ClaimDailyReminderSnapshot returns the planner summary for the once-per-day proactive reminder.
func ClaimDailyReminderSnapshot(db *sql.DB, now time.Time) (DailyReminderSnapshot, error) {
	snapshot := DailyReminderSnapshot{GeneratedAt: now.UTC()}
	if db == nil {
		return snapshot, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return snapshot, fmt.Errorf("begin daily planner reminder claim: %w", err)
	}
	defer tx.Rollback()

	dayKey := reminderDayKey(now)
	lastSeen, err := getPlannerMetaTx(tx, dailyTodoReminderMetaKey)
	if err != nil {
		return snapshot, err
	}
	if reminderMatchesDay(lastSeen, dayKey) {
		if err := tx.Commit(); err != nil {
			return snapshot, fmt.Errorf("commit daily planner reminder no-op: %w", err)
		}
		return snapshot, nil
	}

	rows, err := tx.Query(`
		SELECT id, title, description, priority, status, due_date, remind_daily, completed_at, last_daily_reminder_at, kg_node_id, created_at, updated_at
		FROM todos
		WHERE status IN ('open', 'in_progress')
		ORDER BY CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, due_date ASC, created_at ASC`)
	if err != nil {
		return snapshot, fmt.Errorf("list daily reminder todos: %w", err)
	}
	allTodos := []Todo{}
	for rows.Next() {
		var todo Todo
		var remindDaily int
		if err := rows.Scan(&todo.ID, &todo.Title, &todo.Description, &todo.Priority, &todo.Status, &todo.DueDate, &remindDaily, &todo.CompletedAt, &todo.LastDailyReminderAt, &todo.KGNodeID, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			rows.Close()
			return snapshot, fmt.Errorf("scan daily reminder todo: %w", err)
		}
		todo.RemindDaily = remindDaily != 0
		allTodos = append(allTodos, todo)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, fmt.Errorf("iterate daily reminder todos: %w", err)
	}
	if err := rows.Close(); err != nil {
		return snapshot, fmt.Errorf("close daily reminder rows: %w", err)
	}

	for index := range allTodos {
		items, err := listTodoItemsTx(tx, allTodos[index].ID)
		if err != nil {
			return snapshot, err
		}
		allTodos[index].Items = items
		ComputeTodoProgress(&allTodos[index])
	}
	snapshot.Todos, snapshot.OpenTodoCount, snapshot.OverdueTodoCount = collectOpenTodos(allTodos, now, defaultReminderTodoLimit)

	appointmentRows, err := tx.Query(`
		SELECT id, title, description, date_time, notification_at, wake_agent, agent_instruction, notified, status, kg_node_id, created_at, updated_at
		FROM appointments
		WHERE status = 'upcoming'
		ORDER BY date_time ASC`)
	if err != nil {
		return snapshot, fmt.Errorf("list daily reminder appointments: %w", err)
	}
	appointments := make([]Appointment, 0, 8)
	for appointmentRows.Next() {
		var appointment Appointment
		var wakeAgent, notified int
		if err := appointmentRows.Scan(&appointment.ID, &appointment.Title, &appointment.Description, &appointment.DateTime, &appointment.NotificationAt,
			&wakeAgent, &appointment.AgentInstruction, &notified, &appointment.Status, &appointment.KGNodeID, &appointment.CreatedAt, &appointment.UpdatedAt); err != nil {
			appointmentRows.Close()
			return snapshot, fmt.Errorf("scan daily reminder appointment: %w", err)
		}
		appointment.WakeAgent = wakeAgent != 0
		appointment.Notified = notified != 0
		appointments = append(appointments, appointment)
	}
	if err := appointmentRows.Err(); err != nil {
		appointmentRows.Close()
		return snapshot, fmt.Errorf("iterate daily reminder appointments: %w", err)
	}
	if err := appointmentRows.Close(); err != nil {
		return snapshot, fmt.Errorf("close daily reminder appointment rows: %w", err)
	}
	upcomingAppointments := collectUpcomingAppointments(appointments, now, now.Add(24*time.Hour), 1)
	if len(upcomingAppointments) > 0 {
		nextAppointment := upcomingAppointments[0]
		snapshot.NextAppointment = &nextAppointment
	}

	if err := upsertPlannerMetaTx(tx, dailyTodoReminderMetaKey, dayKey); err != nil {
		return snapshot, err
	}

	if len(allTodos) > 0 {
		remindedAt := now.UTC().Format(time.RFC3339)
		for _, todo := range allTodos {
			if _, err := tx.Exec(`UPDATE todos SET last_daily_reminder_at=? WHERE id=?`, remindedAt, todo.ID); err != nil {
				return snapshot, fmt.Errorf("mark todo reminder timestamp: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return snapshot, fmt.Errorf("commit daily planner reminder claim: %w", err)
	}
	return snapshot, nil
}

// ClaimDailyTodoReminderTodos keeps the legacy todo-only reminder API for compatibility.
func ClaimDailyTodoReminderTodos(db *sql.DB, now time.Time) ([]Todo, error) {
	snapshot, err := ClaimDailyReminderSnapshot(db, now)
	if err != nil {
		return nil, err
	}
	return snapshot.Todos, nil
}

// BuildDailyPlannerReminderText formats the once-per-day planner reminder for prompt injection.
func BuildDailyPlannerReminderText(snapshot DailyReminderSnapshot) string {
	if snapshot.OpenTodoCount == 0 && snapshot.NextAppointment == nil {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("You currently have %d open todos", snapshot.OpenTodoCount))
	if snapshot.OverdueTodoCount > 0 {
		builder.WriteString(fmt.Sprintf(", %d of them overdue", snapshot.OverdueTodoCount))
	}
	builder.WriteString(".\n")

	if len(snapshot.Todos) > 0 {
		builder.WriteString("Top open todos:\n")
		for _, todo := range snapshot.Todos {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(todo.Title))
			if todo.DueDate != "" {
				builder.WriteString(" (due: ")
				builder.WriteString(todo.DueDate)
				builder.WriteString(")")
			}
			if todo.Status == "in_progress" {
				builder.WriteString(" (in progress)")
			}
			builder.WriteString("\n")
		}
	}

	if snapshot.NextAppointment != nil {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("Next appointment within 24h:\n- ")
		builder.WriteString(strings.TrimSpace(snapshot.NextAppointment.DateTime))
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(snapshot.NextAppointment.Title))
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

// ── Helpers ──

func scanAppointment(row *sql.Row) (*Appointment, error) {
	a := &Appointment{}
	var wakeAgent, notified int
	if err := row.Scan(&a.ID, &a.Title, &a.Description, &a.DateTime, &a.NotificationAt,
		&wakeAgent, &a.AgentInstruction, &notified, &a.Status, &a.KGNodeID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("appointment not found")
		}
		return nil, fmt.Errorf("failed to get appointment: %w", err)
	}
	a.WakeAgent = wakeAgent != 0
	a.Notified = notified != 0
	return a, nil
}

// ComputeTodoProgress derives item counters and progress from the current todo state.
func ComputeTodoProgress(t *Todo) {
	if t == nil {
		return
	}
	t.ItemCount = len(t.Items)
	t.DoneItemCount = 0
	for index := range t.Items {
		if t.Items[index].IsDone {
			t.DoneItemCount++
		}
	}
	switch {
	case t.ItemCount > 0:
		if t.Status == "done" {
			t.ProgressPercent = 100
		} else {
			t.ProgressPercent = int(float64(t.DoneItemCount) / float64(t.ItemCount) * 100)
		}
	case t.Status == "done":
		t.ProgressPercent = 100
	case t.Status == "in_progress":
		t.ProgressPercent = 50
	default:
		t.ProgressPercent = 0
	}
}

func prepareTodoForPersistence(t *Todo, now string) error {
	if t == nil {
		return nil
	}
	if t.Items != nil {
		for index := range t.Items {
			item := &t.Items[index]
			if item.ID == "" {
				item.ID = uid.New()
			}
			item.TodoID = t.ID
			if err := prepareTodoItem(item, index, now); err != nil {
				return err
			}
		}
	}
	sort.SliceStable(t.Items, func(i, j int) bool { return t.Items[i].Position < t.Items[j].Position })
	if len(t.Items) > 0 {
		doneCount := 0
		for index := range t.Items {
			if t.Items[index].IsDone {
				doneCount++
			}
		}
		switch {
		case t.Status == "done":
			for index := range t.Items {
				t.Items[index].IsDone = true
				if t.Items[index].CompletedAt == "" {
					t.Items[index].CompletedAt = now
				}
				t.Items[index].UpdatedAt = now
			}
		case doneCount == len(t.Items):
			t.Status = "done"
		case doneCount > 0 && t.Status == "open":
			t.Status = "in_progress"
		case doneCount == 0 && t.Status == "done":
			t.Status = "open"
		}
	}
	if t.Status == "done" {
		if t.CompletedAt == "" {
			t.CompletedAt = now
		}
	} else {
		t.CompletedAt = ""
	}
	ComputeTodoProgress(t)
	return nil
}

func prepareTodoItem(item *TodoItem, defaultPosition int, now string) error {
	if item == nil {
		return nil
	}
	item.Title = strings.TrimSpace(item.Title)
	if item.Title == "" {
		return fmt.Errorf("todo item title is required")
	}
	if item.Position < 0 {
		item.Position = defaultPosition
	}
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	if item.IsDone {
		if item.CompletedAt == "" {
			item.CompletedAt = now
		}
	} else {
		item.CompletedAt = ""
	}
	return nil
}

func listTodoItems(db *sql.DB, todoID string) ([]TodoItem, error) {
	rows, err := db.Query(
		`SELECT id, todo_id, title, description, position, is_done, completed_at, created_at, updated_at
		 FROM todo_items WHERE todo_id=? ORDER BY position ASC, created_at ASC`, todoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list todo items: %w", err)
	}
	defer rows.Close()

	items := []TodoItem{}
	for rows.Next() {
		var item TodoItem
		var isDone int
		if err := rows.Scan(&item.ID, &item.TodoID, &item.Title, &item.Description, &item.Position, &isDone, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan todo item: %w", err)
		}
		item.IsDone = isDone != 0
		items = append(items, item)
	}
	return items, nil
}

func replaceTodoItemsTx(tx *sql.Tx, todoID string, items []TodoItem, now string) error {
	if _, err := tx.Exec("DELETE FROM todo_items WHERE todo_id=?", todoID); err != nil {
		return fmt.Errorf("failed to clear todo items: %w", err)
	}
	for index := range items {
		item := items[index]
		item.TodoID = todoID
		if err := prepareTodoItem(&item, index, now); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO todo_items (id, todo_id, title, description, position, is_done, completed_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.TodoID, item.Title, item.Description, item.Position, dbutil.BoolToInt(item.IsDone), item.CompletedAt, item.CreatedAt, item.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert todo item: %w", err)
		}
	}
	return nil
}

func touchTodoUpdatedAt(db *sql.DB, todoID, updatedAt string) error {
	res, err := db.Exec("UPDATE todos SET updated_at=? WHERE id=?", updatedAt, todoID)
	if err != nil {
		return fmt.Errorf("failed to update todo timestamp: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", todoID)
	}
	return nil
}

func touchTodoUpdatedAtTx(tx *sql.Tx, todoID, updatedAt string) error {
	res, err := tx.Exec("UPDATE todos SET updated_at=? WHERE id=?", updatedAt, todoID)
	if err != nil {
		return fmt.Errorf("failed to update todo timestamp: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", todoID)
	}
	return nil
}

func reminderDayKey(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.In(now.Location()).Format("2006-01-02")
}

func reminderMatchesDay(stored, dayKey string) bool {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return false
	}
	if stored == dayKey {
		return true
	}
	if parsed, err := time.Parse(time.RFC3339, stored); err == nil {
		return parsed.Format("2006-01-02") == dayKey
	}
	return false
}

func syncTodoDerivedState(db *sql.DB, todoID, updatedAt string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin todo state sync: %w", err)
	}
	defer tx.Rollback()

	if err := syncTodoDerivedStateTx(tx, todoID, updatedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit todo state sync: %w", err)
	}
	return nil
}

func syncTodoDerivedStateTx(tx *sql.Tx, todoID, updatedAt string) error {
	todo, err := getTodoTx(tx, todoID)
	if err != nil {
		return err
	}
	items, err := listTodoItemsTx(tx, todoID)
	if err != nil {
		return err
	}
	todo.Items = items
	todo.UpdatedAt = updatedAt
	deriveTodoStateFromChecklist(todo, updatedAt)
	res, err := tx.Exec(
		`UPDATE todos SET status=?, completed_at=?, updated_at=? WHERE id=?`,
		todo.Status, todo.CompletedAt, updatedAt, todoID,
	)
	if err != nil {
		return fmt.Errorf("failed to sync todo state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", todoID)
	}
	for _, item := range todo.Items {
		res, err := tx.Exec(
			`UPDATE todo_items SET position=?, is_done=?, completed_at=?, updated_at=? WHERE id=? AND todo_id=?`,
			item.Position, dbutil.BoolToInt(item.IsDone), item.CompletedAt, item.UpdatedAt, item.ID, todoID,
		)
		if err != nil {
			return fmt.Errorf("failed to sync todo item state: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("todo item not found: %s", item.ID)
		}
	}
	return nil
}

func deriveTodoStateFromChecklist(todo *Todo, updatedAt string) {
	if todo == nil {
		return
	}
	sort.SliceStable(todo.Items, func(i, j int) bool { return todo.Items[i].Position < todo.Items[j].Position })
	if len(todo.Items) > 0 {
		doneCount := 0
		for index := range todo.Items {
			if todo.Items[index].IsDone {
				doneCount++
			}
		}
		switch {
		case doneCount == len(todo.Items):
			todo.Status = "done"
		case doneCount > 0:
			if todo.Status == "done" || todo.Status == "open" {
				todo.Status = "in_progress"
			}
		case todo.Status == "done" || todo.Status == "in_progress":
			todo.Status = "open"
		}
	}
	if todo.Status == "done" {
		if todo.CompletedAt == "" {
			todo.CompletedAt = updatedAt
		}
	} else {
		todo.CompletedAt = ""
	}
	ComputeTodoProgress(todo)
}

func getTodoTx(tx *sql.Tx, id string) (*Todo, error) {
	row := tx.QueryRow(
		`SELECT id, title, description, priority, status, due_date, remind_daily, completed_at, last_daily_reminder_at, kg_node_id, created_at, updated_at
		 FROM todos WHERE id=?`, id,
	)
	t := &Todo{}
	var remindDaily int
	if err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.DueDate, &remindDaily, &t.CompletedAt, &t.LastDailyReminderAt, &t.KGNodeID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("todo not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}
	t.RemindDaily = remindDaily != 0
	return t, nil
}

func listTodoItemsTx(tx *sql.Tx, todoID string) ([]TodoItem, error) {
	rows, err := tx.Query(
		`SELECT id, todo_id, title, description, position, is_done, completed_at, created_at, updated_at
		 FROM todo_items WHERE todo_id=? ORDER BY position ASC, created_at ASC`, todoID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list todo items: %w", err)
	}
	defer rows.Close()

	items := []TodoItem{}
	for rows.Next() {
		var item TodoItem
		var isDone int
		if err := rows.Scan(&item.ID, &item.TodoID, &item.Title, &item.Description, &item.Position, &isDone, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan todo item: %w", err)
		}
		item.IsDone = isDone != 0
		items = append(items, item)
	}
	return items, nil
}

func getPlannerMetaTx(tx *sql.Tx, key string) (string, error) {
	var value string
	err := tx.QueryRow(`SELECT value FROM planner_meta WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get planner meta %s: %w", key, err)
	}
	return value, nil
}

func upsertPlannerMetaTx(tx *sql.Tx, key, value string) error {
	if _, err := tx.Exec(`
		INSERT INTO planner_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value); err != nil {
		return fmt.Errorf("upsert planner meta %s: %w", key, err)
	}
	return nil
}

// SyncAppointmentToKG syncs a single appointment to the knowledge graph.
func SyncAppointmentToKG(kg KnowledgeGraph, db *sql.DB, id string) error {
	if isNilKnowledgeGraph(kg) || db == nil {
		return nil
	}
	a, err := GetAppointment(db, id)
	if err != nil {
		return fmt.Errorf("failed to get appointment for KG sync: %w", err)
	}
	props := map[string]string{
		"type":   "event",
		"source": "planner",
		"date":   a.DateTime,
		"status": a.Status,
	}
	if a.Description != "" {
		props["description"] = a.Description
	}
	if err := kg.AddNode(a.KGNodeID, a.Title, props); err != nil {
		return fmt.Errorf("failed to sync appointment to KG: %w", err)
	}
	return nil
}

// SyncTodoToKG syncs a single todo to the knowledge graph.
func SyncTodoToKG(kg KnowledgeGraph, db *sql.DB, id string) error {
	if isNilKnowledgeGraph(kg) || db == nil {
		return nil
	}
	t, err := GetTodo(db, id)
	if err != nil {
		return fmt.Errorf("failed to get todo for KG sync: %w", err)
	}
	props := map[string]string{
		"type":     "task",
		"source":   "planner",
		"priority": t.Priority,
		"status":   t.Status,
		"progress": fmt.Sprintf("%d", t.ProgressPercent),
	}
	if t.DueDate != "" {
		props["due_date"] = t.DueDate
	}
	if t.Description != "" {
		props["description"] = t.Description
	}
	if t.RemindDaily {
		props["remind_daily"] = "true"
	}
	if t.ItemCount > 0 {
		props["item_count"] = fmt.Sprintf("%d", t.ItemCount)
		props["done_item_count"] = fmt.Sprintf("%d", t.DoneItemCount)
	}
	if err := kg.AddNode(t.KGNodeID, t.Title, props); err != nil {
		return fmt.Errorf("failed to sync todo to KG: %w", err)
	}
	return nil
}

// ToJSON marshals a value to JSON string (for tool output).
func ToJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Use json.Marshal for the error message itself to ensure safe escaping.
		msg, _ := json.Marshal(err.Error())
		return `{"error":` + string(msg) + `}`
	}
	return string(b)
}

func isNilKnowledgeGraph(kg KnowledgeGraph) bool {
	if kg == nil {
		return true
	}
	value := reflect.ValueOf(kg)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
