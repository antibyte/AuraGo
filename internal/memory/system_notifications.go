package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SystemNotification is the typed, id-addressable notification contract.
type SystemNotification struct {
	ID        int64                  `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title,omitempty"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	SourceID  string                 `json:"source_id,omitempty"`
	CreatedAt string                 `json:"created_at"`
}

// AddSystemNotification stores a typed notification. A non-empty SourceID is
// idempotent and can be used to bind one notification to one maintenance run.
func (s *SQLiteMemory) AddSystemNotification(notification SystemNotification) (int64, bool, error) {
	if s == nil || s.db == nil {
		return 0, false, fmt.Errorf("sqlite memory is not initialized")
	}
	notification.Type = strings.TrimSpace(notification.Type)
	if notification.Type == "" {
		notification.Type = "legacy"
	}
	notification.Message = strings.TrimSpace(notification.Message)
	if notification.Message == "" {
		return 0, false, fmt.Errorf("notification message is required")
	}
	data, err := json.Marshal(notification.Data)
	if err != nil {
		return 0, false, fmt.Errorf("marshal notification data: %w", err)
	}
	result, err := s.db.Exec(`
		INSERT OR IGNORE INTO system_notifications
			(content, notification_type, title, data_json, source_id)
		VALUES (?, ?, ?, ?, ?)`,
		notification.Message, notification.Type, strings.TrimSpace(notification.Title), string(data), strings.TrimSpace(notification.SourceID))
	if err != nil {
		return 0, false, fmt.Errorf("store system notification: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("count stored system notification: %w", err)
	}
	if affected == 0 {
		var id int64
		err := s.db.QueryRow(`SELECT id FROM system_notifications WHERE source_id = ?`, strings.TrimSpace(notification.SourceID)).Scan(&id)
		return id, false, err
	}
	id, err := result.LastInsertId()
	return id, true, err
}

// AddNotification preserves the legacy string-only write contract.
func (s *SQLiteMemory) AddNotification(content string) error {
	_, _, err := s.AddSystemNotification(SystemNotification{Type: "legacy", Message: content})
	return err
}

// GetUnreadSystemNotifications returns visible unread notifications.
func (s *SQLiteMemory) GetUnreadSystemNotifications() ([]SystemNotification, error) {
	rows, err := s.db.Query(`
		SELECT id, notification_type, title, content, data_json, source_id, timestamp
		FROM system_notifications WHERE is_read = 0 ORDER BY timestamp ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("get unread system notifications: %w", err)
	}
	defer rows.Close()
	notifications := make([]SystemNotification, 0)
	internalIDs := make([]int64, 0)
	for rows.Next() {
		var n SystemNotification
		var dataJSON string
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &dataJSON, &n.SourceID, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan system notification: %w", err)
		}
		if n.Type == "internal" || isInternalSystemNotification(n.Message) {
			internalIDs = append(internalIDs, n.ID)
			continue
		}
		if strings.TrimSpace(dataJSON) != "" && dataJSON != "{}" {
			_ = json.Unmarshal([]byte(dataJSON), &n.Data)
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system notifications: %w", err)
	}
	if err := s.MarkNotificationsReadByIDs(internalIDs); err != nil {
		return nil, err
	}
	return notifications, nil
}

// GetUnreadNotifications preserves the legacy string-only read contract.
func (s *SQLiteMemory) GetUnreadNotifications() ([]string, error) {
	typed, err := s.GetUnreadSystemNotifications()
	if err != nil {
		return nil, err
	}
	notes := make([]string, 0, len(typed))
	for _, notification := range typed {
		notes = append(notes, notification.Message)
	}
	return notes, nil
}

// MarkNotificationsReadByIDs acknowledges only the requested notifications.
func (s *SQLiteMemory) MarkNotificationsReadByIDs(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin notification acknowledgement: %w", err)
	}
	defer tx.Rollback()
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("notification id must be positive")
		}
		if _, err := tx.Exec(`UPDATE system_notifications SET is_read = 1 WHERE id = ?`, id); err != nil {
			return fmt.Errorf("acknowledge notification %d: %w", id, err)
		}
	}
	return tx.Commit()
}

// MarkNotificationsRead preserves the legacy acknowledge-all contract.
func (s *SQLiteMemory) MarkNotificationsRead() error {
	_, err := s.db.Exec(`UPDATE system_notifications SET is_read = 1 WHERE is_read = 0`)
	return err
}
