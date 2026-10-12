package tools

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

// PersonDetectedEvent is the start of a person detection event as the server reports it to
// Mission Control (overview contract C7). It fires at every event start, armed or not;
// missions filter with their trigger config.
type PersonDetectedEvent struct {
	EventID    int64
	CameraID   string
	CameraName string
	Zones      []string
	MaxScore   float64
	Persons    int
	Armed      bool
	StartedAt  time.Time
	ImagePath  string
}

// personDetectedMatches applies the person_detected filters of cfg: only armed events
// unless PersonIncludeDisarmed, the camera (case-insensitive, empty = any) and a zone the
// event hit (case-insensitive, empty = any; an event without zones covers the whole
// picture and matches no zone filter).
func personDetectedMatches(cfg *TriggerConfig, cameraID string, zones []string, armed bool) bool {
	if cfg == nil {
		cfg = &TriggerConfig{}
	}
	if !armed && !cfg.PersonIncludeDisarmed {
		return false
	}
	if camera := strings.TrimSpace(cfg.PersonCameraID); camera != "" && !strings.EqualFold(camera, strings.TrimSpace(cameraID)) {
		return false
	}
	zone := strings.TrimSpace(cfg.PersonZone)
	if zone == "" {
		return true
	}
	for _, z := range zones {
		if strings.EqualFold(strings.TrimSpace(z), zone) {
			return true
		}
	}
	return false
}

// personDetectedTriggerData is the trigger data of a person_detected event, with exactly the
// C7 keys. started_at has the form of the detection API's event times (C4): UTC, RFC 3339,
// millisecond precision, so a mission can match it against list_detection_events.
func personDetectedTriggerData(ev PersonDetectedEvent) map[string]any {
	zones := ev.Zones
	if zones == nil {
		zones = []string{}
	}
	started := ev.StartedAt
	if started.IsZero() {
		started = time.Now()
	}
	return map[string]any{
		"event_id":    ev.EventID,
		"camera_id":   ev.CameraID,
		"camera_name": ev.CameraName,
		"zones":       zones,
		"max_score":   ev.MaxScore,
		"persons":     ev.Persons,
		"armed":       ev.Armed,
		"started_at":  started.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano),
		"image_path":  ev.ImagePath,
	}
}

// NotifyPersonDetected fires the person_detected mission and flow triggers for an event
// start. Like NotifyHomeAssistantEvent it queues matching enabled local missions, applies
// each mission's min_interval_seconds (shouldFireTriggerLocked) and starts matching flows.
func (m *MissionManagerV2) NotifyPersonDetected(ev PersonDetectedEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	data := personDetectedTriggerData(ev)
	raw, err := json.Marshal(data)
	if err != nil {
		slog.Error("[MissionV2] Person detection trigger data could not be encoded", "error", err)
		return
	}
	queued := false
	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerPersonDetected {
			continue
		}
		if !personDetectedMatches(mission.TriggerConfig, ev.CameraID, ev.Zones, ev.Armed) {
			continue
		}
		if !m.shouldFireTriggerLocked(mission, string(TriggerPersonDetected), now) {
			continue
		}
		m.queue.Enqueue(mission.ID, mission.Priority, string(TriggerPersonDetected), string(raw))
		mission.Status = MissionStatusQueued
		mission.LastResult = ""
		queued = true
	}
	m.notifyFlowsLocked(TriggerPersonDetected, flowEvent{CameraID: ev.CameraID, Zones: ev.Zones, Armed: ev.Armed}, data)
	if queued {
		if err := m.save(); err != nil {
			slog.Error("[MissionV2] Failed to persist missions after person detection event", "error", err)
		}
		if err := m.saveQueueLocked(); err != nil {
			slog.Error("[MissionV2] Failed to persist queue after person detection event", "error", err)
		}
	}
}
