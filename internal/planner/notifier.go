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
	running  bool
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
		n.logger.Info("[Planner] Processing due appointment notification", "id", a.ID, "title", a.Title, "wake_agent", a.WakeAgent)

		if a.WakeAgent {
			// BUG-1 + BUG-4: Atomically claim the notification before spawning the goroutine.
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
			// Non-wake appointments: claim atomically and log that the reminder time was reached.
			claimed, err := ClaimNotification(n.db, a.ID)
			if err != nil {
				n.logger.Error("[Planner] Failed to mark non-wake appointment as notified", "error", err, "id", a.ID)
			} else if claimed {
				n.logger.Info("[Planner] Appointment reminder time reached (wake_agent=false, no agent action)", "id", a.ID, "title", a.Title)
			}
		}
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
