package planner

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Notifier periodically checks for due appointment notifications and wakes the agent.
type Notifier struct {
	db                    *sql.DB
	logger                *slog.Logger
	executor              func(string)
	durableExecutor       func(string, string) error
	durableMissionTrigger func(string, Appointment) error
	missionTrigger        func(Appointment)
	todoTrigger           func(Todo)
	mu                    sync.Mutex
	cancel                context.CancelFunc
	running               bool
}

// NewNotifier creates a new appointment notifier.
func NewNotifier(db *sql.DB, logger *slog.Logger) *Notifier {
	return &Notifier{
		db:     db,
		logger: logger,
	}
}

// SetExecutor sets the agent loopback function called when an appointment is due.
func (n *Notifier) SetExecutor(fn func(string)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.executor = fn
}

// SetMissionTrigger sets the callback fired after a due appointment notification is claimed.
func (n *Notifier) SetMissionTrigger(fn func(Appointment)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.missionTrigger = fn
}

// SetTodoOverdueTrigger sets the callback fired when an open todo is detected as overdue.
func (n *Notifier) SetTodoOverdueTrigger(fn func(Todo)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.todoTrigger = fn
}

// Start begins the notification check loop. Call in a goroutine.
// Calling Start a second time is a no-op if already running.
func (n *Notifier) Start(ctx context.Context) {
	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		n.logger.Warn("[Planner] Notifier.Start called while already running — ignoring")
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	n.cancel = cancel
	n.running = true
	n.mu.Unlock()

	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		n.mu.Lock()
		n.running = false
		n.mu.Unlock()
	}()

	// Run once immediately on start
	n.checkDue()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.checkDue()
		}
	}
}

// Stop cancels the notification loop.
func (n *Notifier) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cancel != nil {
		n.cancel()
	}
}

func (n *Notifier) checkDue() {
	if n.db == nil {
		return
	}
	if err := AutoExpireAppointments(n.db); err != nil {
		n.logger.Error("[Planner] Failed to auto-expire appointments", "error", err)
	}
	n.checkOverdueTodos()
	n.mu.Lock()
	executor := n.executor
	missionTrigger := n.missionTrigger
	durableExecutor := n.durableExecutor
	durableMissionTrigger := n.durableMissionTrigger
	n.mu.Unlock()
	if executor == nil && missionTrigger == nil && durableExecutor == nil && durableMissionTrigger == nil {
		return
	}

	due, err := GetDueNotifications(n.db)
	if err != nil {
		n.logger.Error("[Planner] Failed to query due notifications", "error", err)
		return
	}

	for _, a := range due {
		var targets []string
		if durableMissionTrigger != nil || missionTrigger != nil {
			targets = append(targets, "missions")
		}
		if a.WakeAgent && (durableExecutor != nil || executor != nil) {
			targets = append(targets, "agent")
		}
		if err := prepareAppointmentDestinations(n.db, a, targets); err != nil {
			n.logger.Error("[Planner] Failed to persist appointment destinations", "error", err)
			continue
		}
		destinations := 0
		if durableMissionTrigger != nil || missionTrigger != nil {
			destinations++
			err := n.dispatchAppointment(a, "missions", func(eventID string) error {
				if durableMissionTrigger != nil {
					return durableMissionTrigger(eventID, a)
				}
				missionTrigger(a)
				return nil
			})
			if err != nil {
				n.logger.Error("[Planner] Appointment mission admission failed", "error", err, "id", a.ID)
			}
		}
		if a.WakeAgent && (durableExecutor != nil || executor != nil) {
			destinations++
			err := n.dispatchAppointment(a, "agent", func(eventID string) error {
				if durableExecutor != nil {
					return durableExecutor(eventID, buildNotificationPrompt(a))
				}
				executor(buildNotificationPrompt(a))
				return nil
			})
			if err != nil {
				n.logger.Error("[Planner] Appointment task admission failed", "error", err, "id", a.ID)
			}
		}
		if err := finishAppointmentAdmission(n.db, a, destinations); err != nil {
			n.logger.Error("[Planner] Failed to acknowledge appointment admission", "error", err)
		}
	}

}

func (n *Notifier) checkOverdueTodos() {
	n.mu.Lock()
	todoTrigger := n.todoTrigger
	n.mu.Unlock()
	if todoTrigger == nil {
		return
	}

	todos, err := ListTodos(n.db, "", "all")
	if err != nil {
		n.logger.Error("[Planner] Failed to query overdue todos", "error", err)
		return
	}
	now := time.Now()
	for _, todo := range todos {
		if !isTodoOverdue(todo, now) {
			continue
		}
		key := fmt.Sprintf("overdue_todo:%x", sha256.Sum256([]byte(todo.ID+"\x00"+todo.DueDate)))
		result, err := n.db.Exec(`INSERT INTO planner_meta(key,value)
			SELECT ?, 'claimed' WHERE EXISTS(SELECT 1 FROM todos WHERE id=? AND due_date=? AND status!='done')
			ON CONFLICT(key) DO NOTHING`, key, todo.ID, todo.DueDate)
		if err != nil {
			n.logger.Error("[Planner] Failed to claim overdue todo", "error", err)
			continue
		}
		claimed, err := result.RowsAffected()
		if err != nil || claimed != 1 {
			continue
		}
		func(t Todo) {
			defer func() {
				if r := recover(); r != nil {
					n.logger.Error("[Planner] Panic in overdue todo mission trigger", "error", r, "todo_id", t.ID)
				}
			}()
			todoTrigger(t)
			if _, err := n.db.Exec(`UPDATE planner_meta SET value='dispatched' WHERE key=? AND value='claimed'`, key); err != nil {
				n.logger.Error("[Planner] Failed to finish overdue todo claim", "error", err)
			}
		}(todo)
	}
}

func buildNotificationPrompt(a Appointment) string {
	prompt := fmt.Sprintf("⏰ APPOINTMENT REMINDER: \"%s\" scheduled for %s.", a.Title, a.DateTime)
	if a.Description != "" {
		prompt += fmt.Sprintf(" Details: %s", a.Description)
	}
	if a.AgentInstruction != "" {
		prompt += fmt.Sprintf("\n\nAgent instruction: %s", a.AgentInstruction)
	} else {
		prompt += "\n\nPlease notify the user about this upcoming appointment."
	}
	return prompt
}
