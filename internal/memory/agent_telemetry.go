package memory

import "fmt"

// AgentTelemetryRow is a persisted aggregate counter for agent parse/recovery telemetry.
type AgentTelemetryRow struct {
	EventType string
	EventName string
	Count     int
}

// ScopedAgentTelemetryRow is a persisted telemetry counter segmented by provider/model.
type ScopedAgentTelemetryRow struct {
	ProviderType string
	Model        string
	EventType    string
	EventName    string
	Count        int
}

// UpsertAgentTelemetry increments an aggregate telemetry counter.
func (s *SQLiteMemory) UpsertAgentTelemetry(eventType, eventName string) error {
	if eventType == "" || eventName == "" {
		return nil
	}
	_, err := s.db.Exec(`
		INSERT INTO agent_telemetry (event_type, event_name, count, last_updated)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(event_type, event_name)
		DO UPDATE SET
			count = count + 1,
			last_updated = CURRENT_TIMESTAMP`,
		eventType, eventName)
	if err != nil {
		return fmt.Errorf("upsert agent telemetry: %w", err)
	}
	return nil
}

// UpsertScopedAgentTelemetry increments a telemetry counter for one provider/model scope.
func (s *SQLiteMemory) UpsertScopedAgentTelemetry(providerType, model, eventType, eventName string) error {
	if eventType == "" || eventName == "" || (providerType == "" && model == "") {
		return nil
	}
	_, err := s.db.Exec(`
		INSERT INTO agent_telemetry_scoped (provider_type, model, event_type, event_name, count, last_updated)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(provider_type, model, event_type, event_name)
		DO UPDATE SET
			count = count + 1,
			last_updated = CURRENT_TIMESTAMP`,
		providerType, model, eventType, eventName)
	if err != nil {
		return fmt.Errorf("upsert scoped agent telemetry: %w", err)
	}
	return nil
}

// LoadAgentTelemetry returns all persisted agent telemetry counters.
func (s *SQLiteMemory) LoadAgentTelemetry() ([]AgentTelemetryRow, error) {
	rows, err := s.db.Query(`SELECT event_type, event_name, count FROM agent_telemetry`)
	if err != nil {
		return nil, fmt.Errorf("load agent telemetry: %w", err)
	}
	defer rows.Close()

	var out []AgentTelemetryRow
	for rows.Next() {
		var row AgentTelemetryRow
		if err := rows.Scan(&row.EventType, &row.EventName, &row.Count); err != nil {
			return nil, fmt.Errorf("scan agent telemetry: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// LoadScopedAgentTelemetry returns all persisted scope-segmented agent telemetry counters.
func (s *SQLiteMemory) LoadScopedAgentTelemetry() ([]ScopedAgentTelemetryRow, error) {
	rows, err := s.db.Query(`SELECT provider_type, model, event_type, event_name, count FROM agent_telemetry_scoped`)
	if err != nil {
		return nil, fmt.Errorf("load scoped agent telemetry: %w", err)
	}
	defer rows.Close()

	var out []ScopedAgentTelemetryRow
	for rows.Next() {
		var row ScopedAgentTelemetryRow
		if err := rows.Scan(&row.ProviderType, &row.Model, &row.EventType, &row.EventName, &row.Count); err != nil {
			return nil, fmt.Errorf("scan scoped agent telemetry: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
