package planner

import (
	"database/sql"
	"fmt"
	"strings"

	"aurago/internal/contacts"
)

// SetAppointmentContacts replaces all contact associations for a given appointment.
// It deletes existing associations and inserts the new ones in a single transaction.
func SetAppointmentContacts(db *sql.DB, appointmentID string, contactIDs []string) error {
	if appointmentID == "" {
		return fmt.Errorf("appointment id is required")
	}

	// Deduplicate incoming IDs while preserving order
	seen := make(map[string]bool)
	unique := make([]string, 0, len(contactIDs))
	for _, id := range contactIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	contactIDs = unique

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete existing associations
	_, err = tx.Exec("DELETE FROM appointment_contacts WHERE appointment_id = ?", appointmentID)
	if err != nil {
		return fmt.Errorf("delete existing contacts: %w", err)
	}

	// Insert new associations
	for _, contactID := range contactIDs {
		_, err = tx.Exec(
			"INSERT INTO appointment_contacts (appointment_id, contact_id) VALUES (?, ?)",
			appointmentID, contactID,
		)
		if err != nil {
			return fmt.Errorf("insert contact %q: %w", contactID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// GetAppointmentContactIDs returns the list of contact IDs associated with an appointment.
func GetAppointmentContactIDs(db *sql.DB, appointmentID string) ([]string, error) {
	rows, err := db.Query(
		"SELECT contact_id FROM appointment_contacts WHERE appointment_id = ? ORDER BY rowid",
		appointmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query contact ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan contact id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact ids: %w", err)
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// EnrichAppointmentWithContacts populates the ContactIDs and Participants fields
// of an Appointment by looking up associated contacts from the contacts database.
func EnrichAppointmentWithContacts(db *sql.DB, contactsDB *sql.DB, a *Appointment) error {
	ids, err := GetAppointmentContactIDs(db, a.ID)
	if err != nil {
		return err
	}
	a.ContactIDs = ids

	if len(ids) == 0 {
		a.Participants = []ParticipantSummary{}
		return nil
	}

	participants := make([]ParticipantSummary, 0, len(ids))
	for _, id := range ids {
		c, err := contacts.GetByID(contactsDB, id)
		if err != nil {
			// Contact no longer exists; include a placeholder entry
			participants = append(participants, ParticipantSummary{
				ID:   id,
				Name: "(unavailable)",
			})
			continue
		}
		participants = append(participants, ParticipantSummary{
			ID:           c.ID,
			Name:         c.Name,
			Relationship: c.Relationship,
			Email:        c.Email,
		})
	}
	a.Participants = participants
	return nil
}

// EnrichAppointmentsWithContacts applies EnrichAppointmentWithContacts to a slice
// of appointments efficiently. It batches contact lookups where possible.
func EnrichAppointmentsWithContacts(db *sql.DB, contactsDB *sql.DB, appointments []Appointment) error {
	// Collect all unique contact IDs across all appointments
	allIDs := make(map[string]bool)
	for _, a := range appointments {
		ids, err := GetAppointmentContactIDs(db, a.ID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			allIDs[id] = true
		}
	}

	// Batch-fetch all relevant contacts
	contactMap := make(map[string]ParticipantSummary, len(allIDs))
	for id := range allIDs {
		c, err := contacts.GetByID(contactsDB, id)
		if err != nil {
			contactMap[id] = ParticipantSummary{ID: id, Name: "(unavailable)"}
			continue
		}
		contactMap[id] = ParticipantSummary{
			ID:           c.ID,
			Name:         c.Name,
			Relationship: c.Relationship,
			Email:        c.Email,
		}
	}

	// Populate each appointment
	for i := range appointments {
		ids, err := GetAppointmentContactIDs(db, appointments[i].ID)
		if err != nil {
			return err
		}
		appointments[i].ContactIDs = ids

		if len(ids) == 0 {
			appointments[i].Participants = []ParticipantSummary{}
			continue
		}

		participants := make([]ParticipantSummary, 0, len(ids))
		for _, id := range ids {
			participants = append(participants, contactMap[id])
		}
		appointments[i].Participants = participants
	}
	return nil
}
