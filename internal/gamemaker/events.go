package gamemaker

import (
	"context"
	"fmt"
	"time"
)

func (s *Service) emit(ctx context.Context, projectID, jobID, eventType string, payload map[string]any) (Event, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `INSERT INTO gm_events(project_id,job_id,event_type,payload,created_at)
		VALUES(?,?,?,?,?)`, projectID, jobID, eventType, eventJSON(payload), now)
	if err != nil {
		return Event{}, fmt.Errorf("append game maker event: %w", err)
	}
	id, _ := result.LastInsertId()
	event := Event{ID: id, ProjectID: projectID, JobID: jobID, Type: eventType, Payload: payload, CreatedAt: now}
	s.mu.RLock()
	var targets []chan Event
	for ch := range s.subscribers[projectID] {
		targets = append(targets, ch)
	}
	s.mu.RUnlock()
	for _, ch := range targets {
		select {
		case ch <- event:
		default:
		}
	}
	return event, nil
}

func (s *Service) EventsAfter(ctx context.Context, projectID string, afterID int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,job_id,event_type,payload,created_at
		FROM gm_events WHERE project_id=? AND id>? ORDER BY id LIMIT ?`, projectID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list game maker events: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var event Event
		var raw string
		if err := rows.Scan(&event.ID, &event.ProjectID, &event.JobID, &event.Type, &raw, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Payload = decodeEventPayload(raw)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Service) Subscribe(projectID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	if s.subscribers[projectID] == nil {
		s.subscribers[projectID] = map[chan Event]struct{}{}
	}
	s.subscribers[projectID][ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if subscribers := s.subscribers[projectID]; subscribers != nil {
			delete(subscribers, ch)
			if len(subscribers) == 0 {
				delete(s.subscribers, projectID)
			}
		}
		s.mu.Unlock()
	}
}

// EmitAgentEvent records bounded agent progress without giving callers direct
// access to the database.
func (s *Service) EmitAgentEvent(ctx context.Context, projectID, jobID, eventType string, payload map[string]any) error {
	switch eventType {
	case "text_delta", "skill_activation", "file_changed", "asset_changed", "diagnostic", "phase":
	default:
		return fmt.Errorf("unsupported game maker agent event %q", eventType)
	}
	_, err := s.emit(ctx, projectID, jobID, eventType, payload)
	return err
}
