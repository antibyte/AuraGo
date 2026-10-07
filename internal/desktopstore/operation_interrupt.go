package desktopstore

import (
	"context"
	"time"

	"aurago/internal/security"
)

// InterruptOperation records cleanup after a cancelled owner. It never changes
// an already terminal operation or starts another external action.
func (s *Service) InterruptOperation(operationID string, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message := "operation interrupted"
	if cause != nil {
		message = security.Scrub(cause.Error())
	}
	now := formatTime(time.Now().UTC())
	_, err := s.db.ExecContext(ctx, `UPDATE desktop_store_operations SET status = ?, error = ?, updated_at = ?, completed_at = ? WHERE id = ? AND status IN (?, ?)`, OperationFailed, message, now, now, operationID, OperationPending, OperationRunning)
	return err
}
