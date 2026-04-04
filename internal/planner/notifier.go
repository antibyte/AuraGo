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
	db       *sql.DB
	logger   *slog.Logger
	executor func(string)
	mu       sync.Mutex
	cancel   context.CancelFunc
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

// Start begins the notification check loop. Call in a goroutine.
func (n *Notifier) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	n.mu.Lock()
	n.cancel = cancel
	n.mu.Unlock()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

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
	n.mu.Lock()
	executor := n.executor
	n.mu.Unlock()
	if executor == nil {
		return
	}

	due, err := GetDueNotifications(n.db)
	if err != nil {
		n.logger.Error("[Planner] Failed to query due notifications", "error", err)
		return
	}

	for _, a := range due {
		prompt := buildNotificationPrompt(a)
		n.logger.Info("[Planner] Triggering appointment notification", "id", a.ID, "title", a.Title)

		go func(apt Appointment, p string) {
			defer func() {
				if r := recover(); r != nil {
					n.logger.Error("[Planner] Panic in appointment notification executor", "error", r, "appointment_id", apt.ID)
					return
				}
			}()
			executor(p)
			if err := MarkNotified(n.db, apt.ID); err != nil {
				n.logger.Error("[Planner] Failed to mark appointment as notified", "error", err, "id", apt.ID)
			}
		}(a, prompt)
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
