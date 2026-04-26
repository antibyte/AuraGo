package invasion

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/uid"
)

// EggMessageRecord is a rate-limited notification emitted by an egg.
type EggMessageRecord struct {
	ID              string   `json:"id"`
	NestID          string   `json:"nest_id"`
	EggID           string   `json:"egg_id"`
	MissionID       string   `json:"mission_id,omitempty"`
	TaskID          string   `json:"task_id,omitempty"`
	Severity        string   `json:"severity"`
	Title           string   `json:"title"`
	Body            string   `json:"body"`
	ArtifactIDs     []string `json:"artifact_ids,omitempty"`
	DedupKey        string   `json:"dedup_key,omitempty"`
	WakeupRequested bool     `json:"wakeup_requested"`
	WakeupAllowed   bool     `json:"wakeup_allowed"`
	CreatedAt       string   `json:"created_at"`
	AcknowledgedAt  string   `json:"acknowledged_at,omitempty"`
}

type EggMessageRatePolicy struct {
	Burst  int
	Window time.Duration
}

type EggMessageFilter struct {
	NestID string
	EggID  string
	Limit  int
}

func RecordEggMessage(db *sql.DB, msg EggMessageRecord, policy EggMessageRatePolicy, now time.Time) (EggMessageRecord, error) {
	if db == nil {
		return EggMessageRecord{}, fmt.Errorf("invasion database is unavailable")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	msg.NestID = strings.TrimSpace(msg.NestID)
	msg.EggID = strings.TrimSpace(msg.EggID)
	if msg.NestID == "" {
		return EggMessageRecord{}, fmt.Errorf("nest_id is required")
	}
	if msg.Severity == "" {
		msg.Severity = "info"
	}
	if msg.Title == "" {
		msg.Title = "Egg message"
	}
	if msg.DedupKey != "" {
		existing, err := findEggMessageByDedupKey(db, msg.NestID, msg.EggID, msg.DedupKey)
		if err == nil {
			return existing, nil
		}
	}
	if policy.Burst <= 0 {
		policy.Burst = 3
	}
	if policy.Window <= 0 {
		policy.Window = time.Minute
	}
	msg.WakeupAllowed = false
	if msg.WakeupRequested {
		allowed, err := eggMessageWakeupAllowed(db, msg.NestID, msg.EggID, policy, now)
		if err != nil {
			return EggMessageRecord{}, err
		}
		msg.WakeupAllowed = allowed
	}
	msg.ID = uid.New()
	msg.CreatedAt = now.UTC().Format(time.RFC3339)
	artifactJSON, _ := json.Marshal(msg.ArtifactIDs)
	_, err := db.Exec(`INSERT INTO invasion_egg_messages
		(id, nest_id, egg_id, mission_id, task_id, severity, title, body, artifact_ids_json, dedup_key,
		 wakeup_requested, wakeup_allowed, created_at, acknowledged_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		msg.ID, msg.NestID, msg.EggID, msg.MissionID, msg.TaskID, msg.Severity, msg.Title, msg.Body, string(artifactJSON),
		msg.DedupKey, dbutil.BoolToInt(msg.WakeupRequested), dbutil.BoolToInt(msg.WakeupAllowed), msg.CreatedAt)
	if err != nil {
		return EggMessageRecord{}, fmt.Errorf("failed to record egg message: %w", err)
	}
	return msg, nil
}

func ListEggMessages(db *sql.DB, filter EggMessageFilter) ([]EggMessageRecord, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	clauses := []string{"1=1"}
	args := []interface{}{}
	if strings.TrimSpace(filter.NestID) != "" {
		clauses = append(clauses, "nest_id = ?")
		args = append(args, strings.TrimSpace(filter.NestID))
	}
	if strings.TrimSpace(filter.EggID) != "" {
		clauses = append(clauses, "egg_id = ?")
		args = append(args, strings.TrimSpace(filter.EggID))
	}
	args = append(args, limit)
	rows, err := db.Query(`SELECT id, nest_id, egg_id, mission_id, task_id, severity, title, body,
		artifact_ids_json, dedup_key, wakeup_requested, wakeup_allowed, created_at, acknowledged_at
		FROM invasion_egg_messages WHERE `+strings.Join(clauses, " AND ")+` ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list egg messages: %w", err)
	}
	defer rows.Close()
	var out []EggMessageRecord
	for rows.Next() {
		msg, err := scanEggMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

func AcknowledgeEggMessage(db *sql.DB, id string, when time.Time) error {
	if when.IsZero() {
		when = time.Now().UTC()
	}
	res, err := db.Exec(`UPDATE invasion_egg_messages SET acknowledged_at=? WHERE id=?`, when.UTC().Format(time.RFC3339), strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("failed to acknowledge egg message: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("egg message not found: %s", id)
	}
	return nil
}

func findEggMessageByDedupKey(db *sql.DB, nestID, eggID, dedupKey string) (EggMessageRecord, error) {
	row := db.QueryRow(`SELECT id, nest_id, egg_id, mission_id, task_id, severity, title, body,
		artifact_ids_json, dedup_key, wakeup_requested, wakeup_allowed, created_at, acknowledged_at
		FROM invasion_egg_messages WHERE nest_id=? AND egg_id=? AND dedup_key=? ORDER BY created_at DESC LIMIT 1`,
		nestID, eggID, dedupKey)
	return scanEggMessage(row)
}

func eggMessageWakeupAllowed(db *sql.DB, nestID, eggID string, policy EggMessageRatePolicy, now time.Time) (bool, error) {
	windowStart := now.Add(-policy.Window).UTC().Format(time.RFC3339)
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM invasion_egg_messages
		WHERE nest_id=? AND egg_id=? AND wakeup_allowed=1 AND created_at >= ?`, nestID, eggID, windowStart).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count recent egg wakeups: %w", err)
	}
	return count < policy.Burst, nil
}

type eggMessageScanner interface {
	Scan(dest ...interface{}) error
}

func scanEggMessage(scanner eggMessageScanner) (EggMessageRecord, error) {
	var msg EggMessageRecord
	var artifactJSON string
	var wakeReq, wakeAllowed int
	var ack sql.NullString
	err := scanner.Scan(&msg.ID, &msg.NestID, &msg.EggID, &msg.MissionID, &msg.TaskID, &msg.Severity, &msg.Title,
		&msg.Body, &artifactJSON, &msg.DedupKey, &wakeReq, &wakeAllowed, &msg.CreatedAt, &ack)
	if err != nil {
		return EggMessageRecord{}, err
	}
	_ = json.Unmarshal([]byte(artifactJSON), &msg.ArtifactIDs)
	msg.WakeupRequested = wakeReq != 0
	msg.WakeupAllowed = wakeAllowed != 0
	msg.AcknowledgedAt = nullStr(ack)
	return msg, nil
}
