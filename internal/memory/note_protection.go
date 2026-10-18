package memory

import (
	"fmt"
	"time"
)

// SetNoteProtection is the explicit administrative unlock/retention control.
func (s *SQLiteMemory) SetNoteProtection(id int64, protected, keepForever bool) error {
	result, err := s.db.Exec(`UPDATE notes SET protected=?, keep_forever=?, updated_at=? WHERE id=?`, protected, keepForever, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("set note protection: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read note protection result: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("note with id %d not found", id)
	}
	return nil
}
