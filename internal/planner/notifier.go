package planner

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Notifier periodically checks for due appointment notifications and wakes the agent.
type Notifier struct {
	db               *sql.DB
	logger           *slog.Logger
	executor         func(string)
	missionTrigger   func(Appointment)
	todoTrigger      func(Todo)
	seenOverdueTodos map[string]struct{}
	mu               sync.Mutex
	cancel           context.CancelFunc
	running          bool
}

// NewNotifier creates a new appointment notifier.
func NewNotifier(db *sql.DB, logger *slog.Logger) *Notifier {
	return &Notifier{
		db:               db,
		logger:           logger,
		seenOverdueTodos: make(map[string]struct{}),
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
	n.mu.Unlock()
	if executor == nil && missionTrigger == nil {
		return
	}

	due, err := GetDueNotifications(n.db)
	if err != nil {
		n.logger.Error("[Planner] Failed to query due notifications", "error", err)
		return
	}

	for _, a := range due {
		n.logger.Info("[Planner] Processing due appointment notification", "id", a.ID, "title", a.Title, "wake_agent", a.WakeAgent)

		// BUG-1 + BUG-4: Atomically claim the notification before spawning callbacks.
		// ClaimNotification uses compare-and-swap (WHERE notified=0) so that if the 30s ticker
		// fires again before the executor finishes, the second tick finds notified=1 and skips.
		// This also means a panic in the executor no longer causes an infinite retry loop,
		// because the claim was already committed to the DB.
		claimed, err := ClaimNotification(n.db, a.ID)
		if err != nil {
			n.logger.Error("[Planner] Failed to claim appointment notification", "error", err, "id", a.ID)
			continue
		}
		if !claimed {
			// Another goroutine already claimed this notification; skip.
			continue
		}

		if missionTrigger != nil {
			func(apt Appointment) {
				defer func() {
					if r := recover(); r != nil {
						n.logger.Error("[Planner] Panic in appointment mission trigger", "error", r, "appointment_id", apt.ID)
					}
				}()
				missionTrigger(apt)
			}(a)
		}

		if a.WakeAgent && executor != nil {
			prompt := buildNotificationPrompt(a)
			go func(apt Appointment, p string) {
				defer func() {
					if r := recover(); r != nil {
						n.logger.Error("[Planner] Panic in appointment notification executor", "error", r, "appointment_id", apt.ID)
					}
				}()
				executor(p)
			}(a, prompt)
		} else {
			n.logger.Info("[Planner] Appointment reminder time reached (wake_agent=false, no agent action)", "id", a.ID, "title", a.Title)
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
		if todo.Status == "done" || todo.DueDate == "" {
			continue
		}
		dueAt, err := time.Parse(time.RFC3339, todo.DueDate)
		if err != nil || dueAt.After(now) {
			continue
		}
		key := todo.ID + "|" + todo.DueDate
		n.mu.Lock()
		_, seen := n.seenOverdueTodos[key]
		if !seen {
			n.seenOverdueTodos[key] = struct{}{}
		}
		n.mu.Unlock()
		if seen {
			continue
		}
		func(t Todo) {
			defer func() {
				if r := recover(); r != nil {
					n.logger.Error("[Planner] Panic in overdue todo mission trigger", "error", r, "todo_id", t.ID)
				}
			}()
			todoTrigger(t)
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
