package planner

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type KnowledgeGraph interface {
	AddNode(id, name string, properties map[string]string) error
	DeleteNode(id string) error
}

// Appointment represents a calendar appointment.
type Appointment struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	DateTime         string `json:"date_time"`
	NotificationAt   string `json:"notification_at,omitempty"`
	WakeAgent        bool   `json:"wake_agent"`
	AgentInstruction string `json:"agent_instruction,omitempty"`
	Notified         bool   `json:"notified"`
	Status           string `json:"status"` // upcoming, completed, cancelled
	KGNodeID         string `json:"kg_node_id,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// Todo represents a to-do item.
type Todo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority"` // low, medium, high
	Status      string `json:"status"`   // open, in_progress, done
	DueDate     string `json:"due_date,omitempty"`
	KGNodeID    string `json:"kg_node_id,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// InitDB initializes the planner SQLite database with appointments and todos tables.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open planner database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS appointments (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		date_time TEXT NOT NULL,
		notification_at TEXT DEFAULT '',
		wake_agent INTEGER DEFAULT 0,
		agent_instruction TEXT DEFAULT '',
		notified INTEGER DEFAULT 0,
		status TEXT DEFAULT 'upcoming',
		kg_node_id TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_appointments_date ON appointments(date_time);
	CREATE INDEX IF NOT EXISTS idx_appointments_status ON appointments(status);
	CREATE INDEX IF NOT EXISTS idx_appointments_notification ON appointments(notification_at, notified);

	CREATE TABLE IF NOT EXISTS todos (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		priority TEXT DEFAULT 'medium',
		status TEXT DEFAULT 'open',
		due_date TEXT DEFAULT '',
		kg_node_id TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
	CREATE INDEX IF NOT EXISTS idx_todos_priority ON todos(priority);
	CREATE INDEX IF NOT EXISTS idx_todos_due ON todos(due_date);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create planner schema: %w", err)
	}
	return db, nil
}

// ── Appointment CRUD ──

// validAppointmentStatus returns true for allowed appointment status values.
func validAppointmentStatus(s string) bool {
	return s == "upcoming" || s == "completed" || s == "cancelled"
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
	a.ID = uuid.New().String()
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
		boolToInt(a.WakeAgent), a.AgentInstruction, boolToInt(a.Notified),
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
	if !validAppointmentStatus(a.Status) {
		return fmt.Errorf("invalid status %q: must be upcoming, completed, or cancelled", a.Status)
	}
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	res, err := db.Exec(
		`UPDATE appointments SET title=?, description=?, date_time=?, notification_at=?, wake_agent=?, agent_instruction=?, notified=?, status=?, updated_at=?
		 WHERE id=?`,
		a.Title, a.Description, a.DateTime, a.NotificationAt,
		boolToInt(a.WakeAgent), a.AgentInstruction, boolToInt(a.Notified),
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
	var conditions []string
	var args []interface{}

	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		conditions = append(conditions, "(lower(title) LIKE ? OR lower(description) LIKE ?)")
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

// ── Todo CRUD ──

// CreateTodo adds a new todo and returns its ID.
func CreateTodo(db *sql.DB, t Todo) (string, error) {
	if t.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	t.ID = uuid.New().String()
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
	if t.DueDate != "" && !validRFC3339(t.DueDate) {
		return "", fmt.Errorf("invalid due_date format %q: must be RFC3339 (e.g. 2025-03-15T00:00:00Z)", t.DueDate)
	}
	t.KGNodeID = "todo_" + t.ID

	_, err := db.Exec(
		`INSERT INTO todos (id, title, description, priority, status, due_date, kg_node_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Priority, t.Status, t.DueDate, t.KGNodeID, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert todo: %w", err)
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
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	res, err := db.Exec(
		`UPDATE todos SET title=?, description=?, priority=?, status=?, due_date=?, updated_at=?
		 WHERE id=?`,
		t.Title, t.Description, t.Priority, t.Status, t.DueDate, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", t.ID)
	}
	return nil
}

// DeleteTodo removes a todo by ID.
func DeleteTodo(db *sql.DB, id string) error {
	res, err := db.Exec("DELETE FROM todos WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("todo not found: %s", id)
	}
	return nil
}

// GetTodo returns a single todo by ID.
func GetTodo(db *sql.DB, id string) (*Todo, error) {
	row := db.QueryRow(
		`SELECT id, title, description, priority, status, due_date, kg_node_id, created_at, updated_at
		 FROM todos WHERE id=?`, id)
	t := &Todo{}
	if err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.DueDate, &t.KGNodeID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("todo not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}
	return t, nil
}

// ListTodos returns todos filtered by search query and status.
func ListTodos(db *sql.DB, query, status string) ([]Todo, error) {
	var conditions []string
	var args []interface{}

	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		conditions = append(conditions, "(lower(title) LIKE ? OR lower(description) LIKE ?)")
		args = append(args, like, like)
	}
	if status != "" && status != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	q := `SELECT id, title, description, priority, status, due_date, kg_node_id, created_at, updated_at FROM todos`
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}
	q += " ORDER BY CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, due_date ASC"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list todos: %w", err)
	}
	defer rows.Close()

	var list []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &t.DueDate, &t.KGNodeID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		list = append(list, t)
	}
	if list == nil {
		list = []Todo{}
	}
	return list, nil
}

// ── Helpers ──

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

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

// SyncAppointmentToKG syncs a single appointment to the knowledge graph.
func SyncAppointmentToKG(kg KnowledgeGraph, db *sql.DB, id string) error {
	if kg == nil || db == nil {
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
	if kg == nil || db == nil {
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
	}
	if t.DueDate != "" {
		props["due_date"] = t.DueDate
	}
	if t.Description != "" {
		props["description"] = t.Description
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
