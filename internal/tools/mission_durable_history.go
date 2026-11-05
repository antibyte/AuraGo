package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"aurago/internal/security"
)

// DurableHistoryPrefix separates recoverable flow projections from ordinary
// mission runs, whose history IDs use the run_ prefix.
const DurableHistoryPrefix = "flowdurable_"

var durableHistoryIDPattern = regexp.MustCompile(`^run_[A-Za-z0-9_-]{1,124}$`)
var durableHistoryCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// SetDurableHistorySource installs the read-only journal observation used for
// user-visible history. The callback is never called under the manager lock.
func (m *MissionManagerV2) SetDurableHistorySource(source func(context.Context, string, string) (bool, error)) {
	m.mu.Lock()
	m.durableHistorySource = source
	m.mu.Unlock()
}

// QueryMissionHistory returns the ordinary history page and marks nonterminal
// durable entries unknown when their authoritative source cannot be checked.
func (m *MissionManagerV2) QueryMissionHistory(ctx context.Context, filter MissionHistoryFilter) (*MissionHistoryPage, error) {
	m.mu.RLock()
	db, source := m.historyDB, m.durableHistorySource
	m.mu.RUnlock()
	page, err := QueryMissionHistory(db, filter)
	if err != nil {
		return nil, err
	}
	AnnotateDurableHistory(ctx, page.Entries, source)
	return page, nil
}

// AnnotateDurableHistory is safe to call without a source; unavailable
// nonterminal durable entries are then reported as unknown.
func AnnotateDurableHistory(ctx context.Context, entries []*MissionRun, source func(context.Context, string, string) (bool, error)) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for _, run := range entries {
		if run == nil || !strings.HasPrefix(run.ID, DurableHistoryPrefix) ||
			(run.Status != "queued" && run.Status != "running" && run.Status != "waiting") {
			continue
		}
		runID := strings.TrimPrefix(run.ID, DurableHistoryPrefix)
		known := false
		if source != nil && ctx.Err() == nil && durableHistoryIDPattern.MatchString(runID) {
			present, err := source(ctx, runID, run.MissionID)
			known = err == nil && present
		}
		if !known {
			run.LastKnownStatus = run.Status
			run.Status = "unknown"
		}
	}
}

// DurableMissionHistory contains only trusted flow-journal metadata. It never
// includes trigger data, resolved values, checkpoint contents or error text.
type DurableMissionHistory struct {
	RunID       string
	FlowID      string
	MissionID   string
	MissionName string
	Status      string
	StartedAt   time.Time
	FinishedAt  *time.Time
	ErrorCode   string
}

// ProjectDurableMissionHistory upserts a committed durable run without invoking
// mission completion hooks. The caller serializes updates for a run and rereads
// its source after each commit. A terminal projection cannot regress.
func ProjectDurableMissionHistory(ctx context.Context, db *sql.DB, record DurableMissionHistory) error {
	if db == nil {
		return errors.New("mission history database unavailable")
	}
	record.MissionName = security.ScrubSecrets(record.MissionName)
	if len(record.MissionName) > 1024 {
		record.MissionName = cutAtRuneBoundary(record.MissionName, 1024)
	}
	if !durableHistoryIDPattern.MatchString(record.RunID) || record.MissionID == "" || len(record.MissionID) > 128 || (record.ErrorCode != "" && !durableHistoryCodePattern.MatchString(record.ErrorCode)) {
		return errors.New("invalid durable mission history metadata")
	}
	terminal := false
	switch record.Status {
	case "queued", "running", "waiting":
	case "success", "error", "cancelled":
		terminal = true
	default:
		return errors.New("invalid durable mission history status")
	}
	if terminal != (record.FinishedAt != nil) {
		return errors.New("inconsistent durable mission history timestamp")
	}
	var finished any
	var duration int64
	if terminal {
		finished = record.FinishedAt.UTC().Format(time.RFC3339Nano)
		if !record.StartedAt.IsZero() && record.FinishedAt.After(record.StartedAt) {
			duration = record.FinishedAt.Sub(record.StartedAt).Milliseconds()
		}
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO mission_history
			(id, mission_id, mission_name, trigger_type, trigger_data, status, output, error_msg, started_at, completed_at, duration_ms)
		VALUES (?, ?, ?, 'manual', '', ?, '', ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status, error_msg = excluded.error_msg,
			completed_at = excluded.completed_at, duration_ms = excluded.duration_ms
		WHERE mission_history.mission_id = excluded.mission_id
		  AND mission_history.status NOT IN ('success', 'error', 'cancelled')`,
		DurableHistoryPrefix+record.RunID, record.MissionID, record.MissionName,
		record.Status, record.ErrorCode, record.StartedAt.UTC().Format(time.RFC3339Nano), finished, duration)
	if err != nil {
		return fmt.Errorf("project durable mission history: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect durable mission projection: %w", err)
	}
	if changed == 0 {
		var owner, status string
		if err := db.QueryRowContext(ctx, `SELECT mission_id, status FROM mission_history WHERE id = ?`, DurableHistoryPrefix+record.RunID).Scan(&owner, &status); err != nil {
			return fmt.Errorf("read durable mission projection: %w", err)
		}
		if owner != record.MissionID || (status != "success" && status != "error" && status != "cancelled") {
			return errors.New("durable mission history identity conflict")
		}
	}
	return nil
}

// ProjectDurableFlowRun projects the committed source without dispatching any
// completion, notification or accounting hooks. Callers serialize by run ID.
func (m *MissionManagerV2) ProjectDurableFlowRun(ctx context.Context, record DurableMissionHistory) error {
	if record.FlowID == "" || len(record.FlowID) > 128 {
		return errors.New("invalid durable flow identity")
	}
	m.mu.Lock()
	mission, exists := m.missions[record.MissionID]
	if exists && (!isFlowMission(mission) || mission.FlowID != record.FlowID) {
		m.mu.Unlock()
		return errors.New("durable mission owner mismatch")
	}
	if exists && isFlowMission(mission) && mission.FlowID == record.FlowID {
		record.MissionName = mission.Name
	} else {
		record.MissionName = ""
	}
	db := m.historyDB
	m.mu.Unlock()
	if err := ProjectDurableMissionHistory(ctx, db, record); err != nil {
		return err
	}
	// The database may already contain a terminal observation, in which case
	// the upsert deliberately ignored an older nonterminal observation.
	if err := db.QueryRowContext(ctx, `SELECT status FROM mission_history WHERE id = ?`, DurableHistoryPrefix+record.RunID).Scan(&record.Status); err != nil {
		return fmt.Errorf("read projected durable status: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	mission, exists = m.missions[record.MissionID]
	if !exists || !isFlowMission(mission) || mission.FlowID != record.FlowID {
		return nil
	}
	if m.durableFlowActive == nil {
		m.durableFlowActive = make(map[string]string)
	}
	owner, active := m.durableFlowActive[record.RunID]
	if active && owner != record.MissionID {
		return errors.New("durable mission active identity conflict")
	}
	wantsActive := record.Status == "running" || record.Status == "waiting"
	if wantsActive && !active {
		if m.flowActive == nil {
			m.flowActive = make(map[string]int)
		}
		m.durableFlowActive[record.RunID] = record.MissionID
		m.flowActive[record.MissionID]++
		mission.Status = MissionStatusRunning
	} else if !wantsActive && active {
		delete(m.durableFlowActive, record.RunID)
		if m.flowActive[record.MissionID] > 1 {
			m.flowActive[record.MissionID]--
		} else {
			delete(m.flowActive, record.MissionID)
			mission.Status = MissionStatusIdle
		}
	}
	return nil
}

func (m *MissionManagerV2) forgetDurableFlowRunsLocked(missionID string) {
	for runID, owner := range m.durableFlowActive {
		if owner == missionID {
			delete(m.durableFlowActive, runID)
		}
	}
}
