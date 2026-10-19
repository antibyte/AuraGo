package planner

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotificationOutcomeUnknown means admission may have succeeded. Such an
// event is retained for review and is never automatically replayed.
var ErrNotificationOutcomeUnknown = errors.New("appointment notification admission outcome unknown")

// SetDurableExecutor acknowledges only durable background-task admission.
func (n *Notifier) SetDurableExecutor(fn func(eventID, prompt string) error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.durableExecutor = fn
}

// SetDurableMissionTrigger acknowledges only durable mission-queue admission.
func (n *Notifier) SetDurableMissionTrigger(fn func(eventID string, appointment Appointment) error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.durableMissionTrigger = fn
}

// Register every destination together so a process loss between two admissions
// cannot strand the second destination behind an unknown first outcome.
func prepareAppointmentDestinations(db *sql.DB, a Appointment, destinations []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var revision int64
	if err := tx.QueryRow(`SELECT notification_revision FROM appointments WHERE id=? AND notification_at=? AND notified=0 AND status IN ('upcoming','overdue')`, a.ID, a.NotificationAt).Scan(&revision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	eventID := fmt.Sprintf("appointment_%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", a.ID, revision, a.NotificationAt))))
	for _, destination := range destinations {
		if _, err := tx.Exec(`INSERT INTO appointment_notification_outbox(event_id,destination,appointment_id,revision,state,updated_at) VALUES(?,?,?,?,'pending',?) ON CONFLICT DO NOTHING`, eventID, destination, a.ID, revision, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (n *Notifier) dispatchAppointment(a Appointment, destination string, admit func(string) error) error {
	var revision int64
	// Recheck the revision and due instant after querying the due batch.
	if err := n.db.QueryRow(`SELECT notification_revision FROM appointments WHERE id=? AND notification_at=? AND notified=0 AND status IN ('upcoming','overdue') AND notification_epoch<=?`, a.ID, a.NotificationAt, time.Now().UnixMilli()).Scan(&revision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	eventID := fmt.Sprintf("appointment_%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", a.ID, revision, a.NotificationAt))))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := n.db.Exec(`INSERT INTO appointment_notification_outbox(event_id,destination,appointment_id,revision,state,updated_at) VALUES(?,?,?,?,'pending',?) ON CONFLICT DO NOTHING`, eventID, destination, a.ID, revision, now); err != nil {
		return err
	}
	result, err := n.db.Exec(`UPDATE appointment_notification_outbox SET state='dispatching',updated_at=? WHERE event_id=? AND destination=? AND state='pending' AND EXISTS(SELECT 1 FROM appointments WHERE id=? AND notification_revision=? AND notification_at=? AND notified=0 AND status IN ('upcoming','overdue') AND notification_epoch<=?)`, now, eventID, destination, a.ID, revision, a.NotificationAt, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if claimed != 1 {
		return nil
	}
	admissionErr := func() (err error) {
		defer func() {
			if recover() != nil {
				err = ErrNotificationOutcomeUnknown
			}
		}()
		return admit(eventID)
	}()
	state, code := "accepted", ""
	if errors.Is(admissionErr, ErrNotificationOutcomeUnknown) {
		state, code = "unknown", "admission_outcome_unknown"
	} else if admissionErr != nil {
		state, code = "pending", "admission_failed"
	}
	// A failed acknowledgement write leaves 'dispatching', which prevents replay.
	if _, err := n.db.Exec(`UPDATE appointment_notification_outbox SET state=?,last_error=?,updated_at=? WHERE event_id=? AND destination=? AND state='dispatching'`, state, code, now, eventID, destination); err != nil {
		return fmt.Errorf("persist appointment admission result: %w", err)
	}
	return admissionErr
}

func finishAppointmentAdmission(db *sql.DB, a Appointment, destinations int) error {
	_, err := db.Exec(`UPDATE appointments SET notified=1 WHERE id=? AND notification_at=? AND notified=0
 AND (SELECT COUNT(*) FROM appointment_notification_outbox o WHERE o.appointment_id=appointments.id AND o.revision=appointments.notification_revision AND o.state='accepted')>=?
 AND NOT EXISTS(SELECT 1 FROM appointment_notification_outbox o WHERE o.appointment_id=appointments.id AND o.revision=appointments.notification_revision AND o.state!='accepted')`, a.ID, a.NotificationAt, destinations)
	return err
}

// ListAppointmentNotificationOutbox exposes sanitized delivery states for
// diagnostics. Payloads and session identifiers are never persisted here.
func ListAppointmentNotificationOutbox(db *sql.DB) ([]AppointmentNotificationState, error) {
	rows, err := db.Query(`SELECT event_id,destination,state,last_error FROM appointment_notification_outbox ORDER BY updated_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var states []AppointmentNotificationState
	for rows.Next() {
		var state AppointmentNotificationState
		if err := rows.Scan(&state.EventID, &state.Destination, &state.State, &state.LastError); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

type AppointmentNotificationState struct {
	EventID     string `json:"event_id"`
	Destination string `json:"destination"`
	State       string `json:"state"`
	LastError   string `json:"last_error,omitempty"`
}
