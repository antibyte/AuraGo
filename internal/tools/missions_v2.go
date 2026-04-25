package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

// ExecutionType defines how a mission is executed
type ExecutionType string

const (
	ExecutionManual    ExecutionType = "manual"    // Run on demand
	ExecutionScheduled ExecutionType = "scheduled" // Run on cron schedule
	ExecutionTriggered ExecutionType = "triggered" // Run by trigger event
)

// TriggerType defines what can trigger a mission
type TriggerType string

const (
	TriggerMissionCompleted   TriggerType = "mission_completed"    // Another mission finished
	TriggerEmailReceived      TriggerType = "email_received"       // Email received
	TriggerWebhook            TriggerType = "webhook"              // Webhook fired
	TriggerEggHatched         TriggerType = "egg_hatched"          // Egg deployed to a nest
	TriggerNestCleared        TriggerType = "nest_cleared"         // Nest removed
	TriggerMQTTMessage        TriggerType = "mqtt_message"         // MQTT message received
	TriggerSystemStartup      TriggerType = "system_startup"       // AuraGo Startup
	TriggerDeviceConnected    TriggerType = "device_connected"     // Remote device connected
	TriggerDeviceDisconnected TriggerType = "device_disconnected"  // Remote device disconnected or stale
	TriggerFritzBoxCall       TriggerType = "fritzbox_call"        // Fritz!Box call or voicemail event
	TriggerBudgetWarning      TriggerType = "budget_warning"       // Budget warning threshold crossed
	TriggerBudgetExceeded     TriggerType = "budget_exceeded"      // Budget limit exceeded
	TriggerHomeAssistantState TriggerType = "home_assistant_state" // HA entity state change
)

// MissionStatus represents the runtime status of a mission
const (
	MissionStatusIdle    = "idle"
	MissionStatusQueued  = "queued"
	MissionStatusRunning = "running"
	MissionStatusWaiting = "waiting"
)

// MissionResult represents the outcome of a mission run
const (
	MissionResultSuccess = "success"
	MissionResultError   = "error"
)

// TriggerConfig holds configuration for mission triggers
type TriggerConfig struct {
	// For TriggerMissionCompleted
	SourceMissionID   string `json:"source_mission_id,omitempty"`   // ID of mission that triggers this one
	SourceMissionName string `json:"source_mission_name,omitempty"` // Name for display purposes
	RequireSuccess    bool   `json:"require_success,omitempty"`     // Only trigger if source succeeded

	// For TriggerEmailReceived
	EmailSubjectContains string `json:"email_subject_contains,omitempty"` // Filter by subject
	EmailFromContains    string `json:"email_from_contains,omitempty"`    // Filter by sender
	EmailFolder          string `json:"email_folder,omitempty"`           // Folder to watch

	// For TriggerWebhook
	WebhookID   string `json:"webhook_id,omitempty"`   // ID of linked webhook
	WebhookSlug string `json:"webhook_slug,omitempty"` // Slug for display

	// For TriggerEggHatched and TriggerNestCleared
	NestID   string `json:"nest_id,omitempty"`   // Filter by nest ID (empty = any)
	NestName string `json:"nest_name,omitempty"` // Nest name for display
	EggID    string `json:"egg_id,omitempty"`    // Filter by egg ID (empty = any, egg_hatched only)
	EggName  string `json:"egg_name,omitempty"`  // Egg name for display

	// For TriggerMQTTMessage
	MQTTTopic           string `json:"mqtt_topic,omitempty"`            // Topic filter (supports MQTT wildcards + and #)
	MQTTPayloadContains string `json:"mqtt_payload_contains,omitempty"` // Optional: only trigger if payload contains this string

	// For TriggerDeviceConnected / TriggerDeviceDisconnected
	DeviceID   string `json:"device_id,omitempty"`   // Filter by device ID (empty = any)
	DeviceName string `json:"device_name,omitempty"` // Filter by device name (empty = any)

	// For TriggerFritzBoxCall
	CallType string `json:"call_type,omitempty"` // "call", "tam_message", or empty for any

	// For TriggerHomeAssistantState
	HAEntityID    string `json:"ha_entity_id,omitempty"`    // Entity to watch
	HAStateEquals string `json:"ha_state_equals,omitempty"` // Trigger when state matches this
}

// MissionV2 represents an enhanced mission with trigger support
type MissionV2 struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Prompt        string         `json:"prompt"`
	ExecutionType ExecutionType  `json:"execution_type"` // manual, scheduled, triggered
	Schedule      string         `json:"schedule"`       // cron expression (for scheduled)
	TriggerType   TriggerType    `json:"trigger_type"`   // mission_completed, email_received, webhook
	TriggerConfig *TriggerConfig `json:"trigger_config,omitempty"`
	Priority      string         `json:"priority"` // low | medium | high
	Enabled       bool           `json:"enabled"`
	Status        string         `json:"status"` // idle | queued | running | waiting
	LastRun       time.Time      `json:"last_run"`
	LastResult    string         `json:"last_result"` // success | error | ""
	LastOutput    string         `json:"last_output"` // truncated last response
	RunCount      int            `json:"run_count"`
	CreatedAt     time.Time      `json:"created_at"`
	Locked        bool           `json:"locked"`                   // Prevents deletion
	WaitingForID  string         `json:"waiting_for_id,omitempty"` // ID of mission this is waiting for
	CheatsheetIDs []string       `json:"cheatsheet_ids,omitempty"` // Linked cheat sheet IDs for prompt expansion

	// Remote execution fields
	RunnerType       MissionRunner `json:"runner_type,omitempty"`        // local | remote
	RemoteNestID     string        `json:"remote_nest_id,omitempty"`     // Invasion nest connection target
	RemoteNestName   string        `json:"remote_nest_name,omitempty"`   // Display cache
	RemoteEggID      string        `json:"remote_egg_id,omitempty"`      // Assigned egg template ID
	RemoteEggName    string        `json:"remote_egg_name,omitempty"`    // Display cache
	RemoteSyncStatus string        `json:"remote_sync_status,omitempty"` // synced | pending | error
	RemoteSyncError  string        `json:"remote_sync_error,omitempty"`
	RemoteRevision   string        `json:"remote_revision,omitempty"`

	// Mission Preparation fields
	PreparationStatus string     `json:"preparation_status,omitempty"` // none|preparing|prepared|stale|error
	LastPreparedAt    *time.Time `json:"last_prepared_at,omitempty"`
	AutoPrepare       bool       `json:"auto_prepare,omitempty"`
}

// QueueItem represents a mission in the execution queue
type QueueItem struct {
	MissionID          string    `json:"mission_id"`
	Priority           int       `json:"priority"` // 3=high, 2=medium, 1=low
	EnqueuedAt         time.Time `json:"enqueued_at"`
	TriggerType        string    `json:"trigger_type,omitempty"`
	TriggerData        string    `json:"trigger_data,omitempty"`         // JSON data from trigger
	ExtraCheatsheetIDs []string  `json:"extra_cheatsheet_ids,omitempty"` // transient cheatsheet IDs injected by daemon
	ExtraPromptSuffix  string    `json:"extra_prompt_suffix,omitempty"`  // transient prompt augmentation from daemon
}

// MissionQueue manages the execution queue with priority sorting
type MissionQueue struct {
	mu      sync.Mutex
	items   []QueueItem
	running string // ID of currently running mission
}

// NewMissionQueue creates a new mission queue
func NewMissionQueue() *MissionQueue {
	return &MissionQueue{
		items: []QueueItem{},
	}
}

// Enqueue adds a mission to the queue
func (q *MissionQueue) Enqueue(missionID string, priority string, triggerType, triggerData string) {
	q.enqueueItem(QueueItem{
		MissionID:   missionID,
		Priority:    prioFromString(priority),
		EnqueuedAt:  time.Now(),
		TriggerType: triggerType,
		TriggerData: triggerData,
	})
}

// EnqueueWithExtras adds a mission to the queue with transient daemon extras.
func (q *MissionQueue) EnqueueWithExtras(missionID string, priority int, triggerType, triggerData string, extraCheatsheetIDs []string, extraPromptSuffix string) {
	q.enqueueItem(QueueItem{
		MissionID:          missionID,
		Priority:           priority,
		EnqueuedAt:         time.Now(),
		TriggerType:        triggerType,
		TriggerData:        triggerData,
		ExtraCheatsheetIDs: extraCheatsheetIDs,
		ExtraPromptSuffix:  extraPromptSuffix,
	})
}

func (q *MissionQueue) enqueueItem(item QueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, existing := range q.items {
		if existing.MissionID == item.MissionID {
			return
		}
	}

	q.items = append(q.items, item)
	q.sort()
}

func prioFromString(p string) int {
	switch p {
	case "high":
		return 3
	case "low":
		return 1
	default:
		return 2
	}
}

// Dequeue removes and returns the next mission to run
func (q *MissionQueue) Dequeue() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return QueueItem{}, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.running = item.MissionID
	return item, true
}

// TryStartNext atomically claims the next queued mission if no mission is
// currently running. This avoids a race between GetRunning and Dequeue.
func (q *MissionQueue) TryStartNext() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running != "" || len(q.items) == 0 {
		return QueueItem{}, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.running = item.MissionID
	return item, true
}

// sort orders items by priority (high first), then by enqueue time
func (q *MissionQueue) sort() {
	sort.Slice(q.items, func(i, j int) bool {
		if q.items[i].Priority != q.items[j].Priority {
			return q.items[i].Priority > q.items[j].Priority
		}
		return q.items[i].EnqueuedAt.Before(q.items[j].EnqueuedAt)
	})
}

// Done marks the current mission as finished
func (q *MissionQueue) Done() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.running = ""
}

// GetRunning returns the ID of the currently running mission
func (q *MissionQueue) GetRunning() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running
}

// List returns all queued items
func (q *MissionQueue) List() []QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]QueueItem, len(q.items))
	copy(out, q.items)
	return out
}

// Remove removes a specific mission from the queue
func (q *MissionQueue) Remove(missionID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.items {
		if item.MissionID == missionID {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return true
		}
	}
	return false
}

// MissionManagerV2 provides enhanced mission management with triggers and queue
type MissionManagerV2 struct {
	mu                sync.RWMutex
	saveMu            sync.Mutex // serialises file writes in save()
	file              string
	missions          map[string]*MissionV2
	queue             *MissionQueue
	cron              *CronManager
	callback          func(prompt string, missionID string) // agent invocation callback with mission ID
	ctx               context.Context
	cancel            context.CancelFunc
	emailWatcher      EmailWatcherInterface
	webhookMgr        WebhookManagerInterface
	mqttMgr           MQTTManagerInterface
	cheatsheetDB      *sql.DB
	preparedDB        *sql.DB                                  // prepared missions database
	historyDB         *sql.DB                                  // mission execution history database
	activeRunID       map[string]string                        // missionID → history run ID for in-progress tracking
	onMissionComplete func(completedID, result, output string) // callback for mission completion
	missionGuards     map[string]context.CancelFunc            // per-mission timeout guardian cancel functions
	remoteClient      RemoteMissionClient
}

// EmailWatcherInterface for email trigger integration
type EmailWatcherInterface interface {
	RegisterMissionTrigger(folder, subjectContains, fromContains string, callback func(subject, from, body string))
}

// WebhookManagerInterface for webhook trigger integration
type WebhookManagerInterface interface {
	RegisterMissionTrigger(webhookID string, callback func(payload []byte))
}

// MQTTManagerInterface for MQTT trigger integration
type MQTTManagerInterface interface {
	RegisterMissionTrigger(topicFilter string, payloadContains string, callback func(topic, payload string))
}

// NewMissionManagerV2 creates a new enhanced MissionManager
func NewMissionManagerV2(dataDir string, cronMgr *CronManager) *MissionManagerV2 {
	ctx, cancel := context.WithCancel(context.Background())
	return &MissionManagerV2{
		file:          filepath.Join(dataDir, "missions_v2.json"),
		missions:      make(map[string]*MissionV2),
		queue:         NewMissionQueue(),
		cron:          cronMgr,
		ctx:           ctx,
		cancel:        cancel,
		activeRunID:   make(map[string]string),
		missionGuards: make(map[string]context.CancelFunc),
	}
}

// SetCallback sets the agent invocation callback
func (m *MissionManagerV2) SetCallback(callback func(prompt string, missionID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callback = callback
}

// SetEmailWatcher sets the email watcher for email triggers
func (m *MissionManagerV2) SetEmailWatcher(watcher EmailWatcherInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emailWatcher = watcher
}

// SetWebhookManager sets the webhook manager for webhook triggers
func (m *MissionManagerV2) SetWebhookManager(mgr WebhookManagerInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhookMgr = mgr
}

// SetMQTTManager sets the MQTT manager for MQTT message triggers
func (m *MissionManagerV2) SetMQTTManager(mgr MQTTManagerInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mqttMgr = mgr
}

// SetCheatsheetDB sets the cheatsheet database for prompt expansion
func (m *MissionManagerV2) SetCheatsheetDB(db *sql.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cheatsheetDB = db
}

// SetPreparedDB sets the prepared missions database
func (m *MissionManagerV2) SetPreparedDB(db *sql.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preparedDB = db
}

// SetHistoryDB sets the mission execution history database
func (m *MissionManagerV2) SetHistoryDB(db *sql.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.historyDB = db
}

// SetRemoteMissionClient sets the client used to synchronize missions to eggs.
func (m *MissionManagerV2) SetRemoteMissionClient(client RemoteMissionClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.remoteClient = client
}

// SetCompletionCallback registers a callback fired after any mission completes.
func (m *MissionManagerV2) SetCompletionCallback(callback func(completedID, result, output string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMissionComplete = callback
}

// GetHistoryDB returns the mission execution history database reference.
func (m *MissionManagerV2) GetHistoryDB() *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.historyDB
}

// GetCheatsheetDB returns the cheatsheet database reference.
func (m *MissionManagerV2) GetCheatsheetDB() *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cheatsheetDB
}

// GetPreparedDB returns the prepared missions database reference.
func (m *MissionManagerV2) GetPreparedDB() *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.preparedDB
}

// SetPreparationStatus updates the preparation status on a mission in memory and saves.
func (m *MissionManagerV2) SetPreparationStatus(missionID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, ok := m.missions[missionID]
	if !ok {
		return
	}
	mission.PreparationStatus = status
	if status == string(PrepStatusPrepared) {
		now := time.Now()
		mission.LastPreparedAt = &now
	}
	m.save()
}

// Start loads missions and initializes triggers
func (m *MissionManagerV2) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load missions
	data, err := os.ReadFile(m.file)
	if err == nil {
		var missions []*MissionV2
		if err := json.Unmarshal(data, &missions); err != nil {
			return fmt.Errorf("failed to parse %s: %w", m.file, err)
		}
		for _, mission := range missions {
			mission.RunnerType = normalizeMissionRunner(mission.RunnerType)
			m.missions[mission.ID] = mission
			if mission.Status == MissionStatusRunning || mission.Status == MissionStatusQueued {
				mission.Status = MissionStatusIdle // Reset on startup
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", m.file, err)
	}

	// Setup triggers
	m.setupTriggers()

	// Setup cron schedules for enabled scheduled missions (ensures they survive restarts)
	if m.cron != nil {
		m.cron.RegisterRunner("mission", func(jobID, prompt string) {
			missionID := strings.TrimPrefix(jobID, "mission_")
			if missionID != "" {
				m.TriggerMission(missionID, "cron", "")
			}
		})
		for _, mission := range m.missions {
			if !mission.Enabled || mission.ExecutionType != ExecutionScheduled {
				continue
			}
			if isRemoteMission(mission) {
				continue
			}
			if mission.Schedule != "" {
				cronID := "mission_" + mission.ID
				if _, err := m.cron.ManageScheduleWithSource("add", cronID, mission.Schedule, mission.Prompt, "", "mission"); err != nil {
					slog.Warn("[MissionV2] Failed to register scheduled mission with cron", "mission_id", mission.ID, "schedule", mission.Schedule, "error", err)
				}
			}
		}
	}

	// Start queue processor
	go m.processQueue()

	return nil
}

// Stop shuts down the mission manager
func (m *MissionManagerV2) Stop() {
	m.cancel()
}

func (m *MissionManagerV2) save() error {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	// NOTE: callers should hold m.mu (Lock or RLock) when possible, but some
	// call paths (e.g. processNext) invoke save after releasing the lock.
	// Do NOT acquire m.mu here — to avoid double locking.
	missions := make([]*MissionV2, 0, len(m.missions))
	for _, mission := range m.missions {
		missions = append(missions, mission)
	}

	data, err := json.MarshalIndent(missions, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename to prevent data loss on crash
	tmp := m.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		slog.Error("[MissionV2] Failed to persist mission state", "error", err)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, m.file); err != nil {
		slog.Error("[MissionV2] Failed to persist mission state", "error", err)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// setupTriggers initializes all active triggers
func (m *MissionManagerV2) setupTriggers() {
	for _, mission := range m.missions {
		if !mission.Enabled || mission.ExecutionType != ExecutionTriggered {
			continue
		}
		if isRemoteMission(mission) {
			continue
		}
		m.registerTrigger(mission)
	}
}

// registerTrigger sets up a single trigger
func (m *MissionManagerV2) registerTrigger(mission *MissionV2) {
	if isRemoteMission(mission) {
		return
	}
	if mission.TriggerConfig == nil {
		return
	}

	switch mission.TriggerType {
	case TriggerEmailReceived:
		if m.emailWatcher != nil {
			cfg := mission.TriggerConfig
			m.emailWatcher.RegisterMissionTrigger(
				cfg.EmailFolder,
				cfg.EmailSubjectContains,
				cfg.EmailFromContains,
				func(subject, from, body string) {
					triggerData, _ := json.Marshal(map[string]string{
						"subject": subject,
						"from":    from,
						"body":    body,
					})
					m.TriggerMission(mission.ID, "email", string(triggerData))
				},
			)
		}

	case TriggerWebhook:
		if m.webhookMgr != nil && mission.TriggerConfig.WebhookID != "" {
			m.webhookMgr.RegisterMissionTrigger(
				mission.TriggerConfig.WebhookID,
				func(payload []byte) {
					m.TriggerMission(mission.ID, "webhook", string(payload))
				},
			)
		}

	case TriggerMQTTMessage:
		if m.mqttMgr != nil && mission.TriggerConfig.MQTTTopic != "" {
			cfg := mission.TriggerConfig
			m.mqttMgr.RegisterMissionTrigger(
				cfg.MQTTTopic,
				cfg.MQTTPayloadContains,
				func(topic, payload string) {
					triggerData, _ := json.Marshal(map[string]string{
						"topic":   topic,
						"payload": payload,
					})
					m.TriggerMission(mission.ID, "mqtt", string(triggerData))
				},
			)
		}
	}
	// TriggerMissionCompleted is handled via OnMissionComplete callback
}

// processQueue runs the main queue processing loop
func (m *MissionManagerV2) processQueue() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("[MissionManagerV2] recovered from panic in processNext: %v\n", r)
					}
				}()
				m.processNext()
			}()
		}
	}
}

// processNext executes the next mission in queue if none is running
func (m *MissionManagerV2) processNext() {
	item, ok := m.queue.TryStartNext()
	if !ok {
		return
	}

	m.mu.Lock()
	mission, exists := m.missions[item.MissionID]
	if !exists || !mission.Enabled {
		m.queue.Done()
		m.mu.Unlock()
		return
	}

	mission.Status = MissionStatusRunning
	mission.LastRun = time.Now()
	m.save()

	callback := m.callback
	prompt := mission.Prompt
	missionID := mission.ID
	missionName := mission.Name
	cheatsheetIDs := mission.CheatsheetIDs
	cheatsheetDB := m.cheatsheetDB
	preparedDB := m.preparedDB
	historyDB := m.historyDB
	m.mu.Unlock()

	// Record mission start in history
	if historyDB != nil {
		triggerType := item.TriggerType
		if triggerType == "" {
			triggerType = "manual"
		}
		if runID, err := RecordMissionStart(historyDB, missionID, missionName, triggerType, item.TriggerData); err == nil {
			m.mu.Lock()
			m.activeRunID[missionID] = runID
			m.mu.Unlock()
		}
	}

	if callback == nil {
		// No callback set — mark mission as error and release queue
		m.mu.Lock()
		if ms, ok := m.missions[missionID]; ok {
			ms.Status = MissionStatusIdle
			ms.LastResult = MissionResultError
			ms.LastOutput = "no callback registered"
			m.save()
		}
		m.mu.Unlock()
		m.queue.Done()
		return
	}

	// Enhance prompt with linked cheat sheets
	if len(cheatsheetIDs) > 0 && cheatsheetDB != nil {
		if extra := CheatsheetGetMultiple(cheatsheetDB, cheatsheetIDs); extra != "" {
			prompt += extra
		}
	}

	// Enhance prompt with transient daemon cheatsheet IDs
	if len(item.ExtraCheatsheetIDs) > 0 && cheatsheetDB != nil {
		if extra := CheatsheetGetMultiple(cheatsheetDB, item.ExtraCheatsheetIDs); extra != "" {
			prompt += extra
		}
	}

	// Enhance prompt with transient daemon prompt suffix
	if item.ExtraPromptSuffix != "" {
		prompt += item.ExtraPromptSuffix
	}

	// Enhance prompt with prepared context (advisory)
	if preparedDB != nil {
		if pm, err := GetPreparedMission(preparedDB, missionID); err == nil && pm != nil {
			if advisory := pm.RenderPreparedContext(); advisory != "" {
				prompt += advisory
			}
		}
	}

	prompt = appendIsolatedTriggerContext(prompt, item.TriggerType, item.TriggerData)
	// Start timeout guardian to prevent permanent queue blocking if callback hangs
	guardCtx, guardCancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.missionGuards[missionID] = guardCancel
	m.mu.Unlock()

	go func() {
		timer := time.NewTimer(40 * time.Minute)
		defer timer.Stop()
		select {
		case <-timer.C:
			slog.Warn("[MissionV2] Mission execution timeout, releasing queue", "mission_id", missionID, "timeout", "40m")
			m.OnMissionComplete(missionID, MissionResultError, "mission execution timeout exceeded (40m)")
		case <-guardCtx.Done():
			// Normal completion, guardian cancelled
		case <-m.ctx.Done():
			// System shutdown
		}
	}()
	go callback(prompt, missionID)
}

func appendIsolatedTriggerContext(prompt, triggerType, triggerData string) string {
	if triggerData == "" {
		return prompt
	}
	if triggerType == "" {
		triggerType = "event"
	}
	return fmt.Sprintf("%s\n\n[Trigger Context: %s]\n%s", prompt, triggerType, security.IsolateExternalData(triggerData))
}

// OnMissionComplete handles mission completion and triggers dependent missions
func (m *MissionManagerV2) OnMissionComplete(missionID, result, output string) {
	m.mu.Lock()

	// Cancel timeout guardian if active
	if cancel, ok := m.missionGuards[missionID]; ok {
		cancel()
		delete(m.missionGuards, missionID)
	}

	// Guard against double completion (e.g. timeout + normal completion race)
	if mission, ok := m.missions[missionID]; ok && mission.Status != MissionStatusRunning {
		m.mu.Unlock()
		return
	}

	// Record mission completion in history
	if runID, ok := m.activeRunID[missionID]; ok && m.historyDB != nil {
		hdb := m.historyDB
		delete(m.activeRunID, missionID)
		// Write history outside the lock to avoid contention
		go func() {
			var histErr error
			if result == MissionResultSuccess || result == "success" {
				histErr = RecordMissionCompletion(hdb, runID, "success", output)
			} else {
				histErr = RecordMissionError(hdb, runID, output)
			}
			if histErr != nil {
				slog.Error("[MissionV2] Failed to record mission history", "run_id", runID, "error", histErr)
			}
		}()
	}
	defer m.mu.Unlock()

	// Update mission status
	if mission, ok := m.missions[missionID]; ok {
		mission.Status = MissionStatusIdle
		mission.LastResult = result
		mission.LastOutput = truncateString(output, 500)
		mission.RunCount++
		// (save deferred to end of method to avoid double write)
	}

	m.queue.Done()

	// Check for missions triggered by this completion
	for _, mission := range m.missions {
		if !mission.Enabled ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerMissionCompleted {
			continue
		}

		cfg := mission.TriggerConfig
		if cfg == nil || cfg.SourceMissionID != missionID {
			continue
		}

		// Check if success is required
		if cfg.RequireSuccess && result != MissionResultSuccess {
			continue
		}

		// Queue the triggered mission
		m.queue.Enqueue(mission.ID, mission.Priority, "mission_completed",
			fmt.Sprintf(`{"source_mission":"%s","result":"%s"}`, missionID, result))
		mission.Status = MissionStatusQueued
	}
	completeCB := m.onMissionComplete
	m.save() // Second save: persist queued status of triggered dependents
	if completeCB != nil {
		go completeCB(missionID, result, output)
	}
}

// TriggerMission manually triggers a mission (for webhooks, emails, etc.)
func (m *MissionManagerV2) TriggerMission(missionID, triggerType, triggerData string) error {
	return m.TriggerMissionWithOptions(missionID, triggerType, triggerData, nil, "")
}

// TriggerMissionWithOptions triggers a mission with optional transient daemon extras.
// extraCheatsheetIDs are appended to the mission prompt in addition to the mission's own cheatsheets.
// extraPromptSuffix is appended verbatim after cheatsheet expansion.
func (m *MissionManagerV2) TriggerMissionWithOptions(missionID, triggerType, triggerData string, extraCheatsheetIDs []string, extraPromptSuffix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission, ok := m.missions[missionID]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if !mission.Enabled {
		return fmt.Errorf("mission is disabled")
	}
	if isRemoteMission(mission) {
		if len(extraCheatsheetIDs) > 0 || extraPromptSuffix != "" {
			return fmt.Errorf("remote missions do not support transient prompt extras")
		}
		if m.remoteClient == nil {
			return fmt.Errorf("remote mission client is not configured")
		}
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		defer cancel()
		if err := m.remoteClient.RunMission(ctx, *mission, triggerType, triggerData); err != nil {
			mission.RemoteSyncStatus = RemoteSyncError
			mission.RemoteSyncError = err.Error()
			m.save()
			return err
		}
		mission.Status = MissionStatusQueued
		mission.RemoteSyncStatus = RemoteSyncSynced
		mission.RemoteSyncError = ""
		m.save()
		return nil
	}

	m.queue.EnqueueWithExtras(missionID, prioFromString(mission.Priority), triggerType, triggerData, extraCheatsheetIDs, extraPromptSuffix)
	mission.Status = MissionStatusQueued
	m.save()
	return nil
}

// NotifyInvasionEvent fires mission triggers for invasion events (egg_hatched, nest_cleared).
// eventType must be "egg_hatched" or "nest_cleared".
func (m *MissionManagerV2) NotifyInvasionEvent(eventType, nestID, nestName, eggID, eggName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			string(mission.TriggerType) != eventType {
			continue
		}

		cfg := mission.TriggerConfig
		if cfg == nil {
			cfg = &TriggerConfig{}
		}

		// Apply optional filters
		if cfg.NestID != "" && cfg.NestID != nestID {
			continue
		}
		if eventType == string(TriggerEggHatched) && cfg.EggID != "" && cfg.EggID != eggID {
			continue
		}

		triggerData, _ := json.Marshal(map[string]string{
			"event":     eventType,
			"nest_id":   nestID,
			"nest_name": nestName,
			"egg_id":    eggID,
			"egg_name":  eggName,
		})
		m.queue.Enqueue(mission.ID, mission.Priority, eventType, string(triggerData))
		mission.Status = MissionStatusQueued
	}
	m.save()
}

// NotifySystemStartup fires mission triggers meant to run when AuraGo starts.
func (m *MissionManagerV2) NotifySystemStartup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerSystemStartup {
			continue
		}

		triggerData, _ := json.Marshal(map[string]string{
			"event": "system_startup",
			"time":  time.Now().Format(time.RFC3339),
		})
		m.queue.Enqueue(mission.ID, mission.Priority, string(TriggerSystemStartup), string(triggerData))
		mission.Status = MissionStatusQueued
	}
	m.save()
}

// NotifyDeviceEvent fires mission triggers for remote device events (device_connected, device_disconnected).
func (m *MissionManagerV2) NotifyDeviceEvent(eventType, deviceID, deviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigType := TriggerType(eventType)
	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != trigType {
			continue
		}

		cfg := mission.TriggerConfig
		if cfg == nil {
			cfg = &TriggerConfig{}
		}

		if cfg.DeviceID != "" && cfg.DeviceID != deviceID {
			continue
		}
		if cfg.DeviceName != "" && cfg.DeviceName != deviceName {
			continue
		}

		triggerData, _ := json.Marshal(map[string]string{
			"event":       eventType,
			"device_id":   deviceID,
			"device_name": deviceName,
			"time":        time.Now().Format(time.RFC3339),
		})
		m.queue.Enqueue(mission.ID, mission.Priority, eventType, string(triggerData))
		mission.Status = MissionStatusQueued
	}
	m.save()
}

// NotifyFritzBoxEvent fires mission triggers for Fritz!Box telephony events.
// callType is "call" or "tam_message", summary is the event description.
func (m *MissionManagerV2) NotifyFritzBoxEvent(callType, summary string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerFritzBoxCall {
			continue
		}

		cfg := mission.TriggerConfig
		if cfg == nil {
			cfg = &TriggerConfig{}
		}

		if cfg.CallType != "" && cfg.CallType != callType {
			continue
		}

		triggerData, _ := json.Marshal(map[string]string{
			"call_type": callType,
			"summary":   summary,
			"time":      time.Now().Format(time.RFC3339),
		})
		m.queue.Enqueue(mission.ID, mission.Priority, "fritzbox_call", string(triggerData))
		mission.Status = MissionStatusQueued
	}
	m.save()
}

// NotifyBudgetEvent fires mission triggers for budget threshold events.
// eventType is "budget_warning" or "budget_exceeded".
func (m *MissionManagerV2) NotifyBudgetEvent(eventType string, spentUSD, limitUSD, percentage float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigType := TriggerType(eventType)
	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != trigType {
			continue
		}

		triggerData, _ := json.Marshal(map[string]interface{}{
			"event":      eventType,
			"spent_usd":  spentUSD,
			"limit_usd":  limitUSD,
			"percentage": percentage,
			"time":       time.Now().Format(time.RFC3339),
		})
		m.queue.Enqueue(mission.ID, mission.Priority, eventType, string(triggerData))
		mission.Status = MissionStatusQueued
	}
	m.save()
}

// NotifyHomeAssistantEvent fires mission triggers for HA state changes.
func (m *MissionManagerV2) NotifyHomeAssistantEvent(entityID, newState, oldState string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mission := range m.missions {
		if !mission.Enabled ||
			isRemoteMission(mission) ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerHomeAssistantState {
			continue
		}

		cfg := mission.TriggerConfig
		if cfg == nil {
			cfg = &TriggerConfig{}
		}

		if cfg.HAEntityID != "" && cfg.HAEntityID != entityID {
			continue
		}
		if cfg.HAStateEquals != "" && cfg.HAStateEquals != newState {
			continue
		}

		triggerData, _ := json.Marshal(map[string]string{
			"entity_id": entityID,
			"new_state": newState,
			"old_state": oldState,
			"time":      time.Now().Format(time.RFC3339),
		})
		m.queue.Enqueue(mission.ID, mission.Priority, "home_assistant_state", string(triggerData))
		mission.Status = MissionStatusQueued
	}
	m.save()
}

func (m *MissionManagerV2) buildRemotePromptSnapshotLocked(mission *MissionV2) string {
	if mission == nil {
		return ""
	}
	prompt := mission.Prompt
	if len(mission.CheatsheetIDs) > 0 && m.cheatsheetDB != nil {
		if extra := CheatsheetGetMultiple(m.cheatsheetDB, mission.CheatsheetIDs); extra != "" {
			prompt += extra
		}
	}
	return prompt
}

func (m *MissionManagerV2) syncRemoteMissionLocked(mission *MissionV2) error {
	if !isRemoteMission(mission) {
		return nil
	}
	if m.remoteClient == nil {
		mission.RemoteSyncStatus = RemoteSyncError
		mission.RemoteSyncError = "remote mission client is not configured"
		return fmt.Errorf("remote mission client is not configured")
	}
	mission.RemoteRevision = newRemoteRevision()
	mission.RemoteSyncStatus = RemoteSyncPending
	mission.RemoteSyncError = ""
	promptSnapshot := m.buildRemotePromptSnapshotLocked(mission)
	ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
	defer cancel()
	if err := m.remoteClient.SyncMission(ctx, *mission, promptSnapshot); err != nil {
		mission.RemoteSyncStatus = RemoteSyncError
		mission.RemoteSyncError = err.Error()
		return err
	}
	mission.RemoteSyncStatus = RemoteSyncSynced
	mission.RemoteSyncError = ""
	return nil
}

// SetRemoteResult records a result reported by an egg for a master-side mission.
func (m *MissionManagerV2) SetRemoteResult(id, result, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, ok := m.missions[id]
	if !ok {
		return
	}
	mission.Status = MissionStatusIdle
	mission.LastRun = time.Now()
	mission.LastResult = result
	mission.LastOutput = truncateString(output, 500)
	mission.RunCount++
	mission.RemoteSyncStatus = RemoteSyncSynced
	mission.RemoteSyncError = ""
	m.save()
}

// RunNow triggers a mission immediately (bypasses queue for manual execution)
func (m *MissionManagerV2) RunNow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission, ok := m.missions[id]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if !mission.Enabled {
		return fmt.Errorf("mission is disabled")
	}
	if isRemoteMission(mission) {
		if m.remoteClient == nil {
			return fmt.Errorf("remote mission client is not configured")
		}
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		defer cancel()
		if err := m.remoteClient.RunMission(ctx, *mission, "manual", ""); err != nil {
			mission.RemoteSyncStatus = RemoteSyncError
			mission.RemoteSyncError = err.Error()
			m.save()
			return err
		}
		mission.Status = MissionStatusQueued
		mission.RemoteSyncStatus = RemoteSyncSynced
		mission.RemoteSyncError = ""
		m.save()
		return nil
	}

	// For manual execution, we still queue but with high priority
	m.queue.Enqueue(id, "high", "manual", "")
	mission.Status = MissionStatusQueued
	m.save()
	return nil
}

// Create adds a new mission
func (m *MissionManagerV2) Create(mission *MissionV2) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission.RunnerType = normalizeMissionRunner(mission.RunnerType)
	if mission.ID == "" {
		mission.ID = fmt.Sprintf("mission_%d", time.Now().UnixNano())
	}
	if mission.Priority == "" {
		mission.Priority = "medium"
	}
	if mission.ExecutionType == "" {
		mission.ExecutionType = ExecutionManual
	}
	mission.Status = MissionStatusIdle
	mission.CreatedAt = time.Now()
	mission.Enabled = true

	if err := validateRemoteMission(*mission); err != nil {
		return err
	}
	if isRemoteMission(mission) {
		if err := m.syncRemoteMissionLocked(mission); err != nil {
			return err
		}
	}

	m.missions[mission.ID] = mission

	// Register trigger if needed
	if !isRemoteMission(mission) && mission.ExecutionType == ExecutionTriggered {
		m.registerTrigger(mission)
	}

	// Register with cron if scheduled
	if !isRemoteMission(mission) && mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" {
		cronID := "mission_" + mission.ID
		if m.cron == nil {
			return fmt.Errorf("cron manager is not configured")
		}
		if _, err := m.cron.ManageScheduleWithSource("add", cronID, mission.Schedule, mission.Prompt, "", "mission"); err != nil {
			return fmt.Errorf("failed to register mission with cron: %w", err)
		}
	}

	return m.save()
}

// Update modifies a mission
func (m *MissionManagerV2) Update(id string, updated *MissionV2) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission, ok := m.missions[id]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	updated.RunnerType = normalizeMissionRunner(updated.RunnerType)
	if err := validateRemoteMission(*updated); err != nil {
		return err
	}

	// Unregister old triggers
	if !isRemoteMission(mission) && mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" && m.cron != nil {
		cronID := "mission_" + id
		m.cron.ManageSchedule("remove", cronID, "", "", "")
	}
	if isRemoteMission(mission) && (!isRemoteMission(updated) || mission.RemoteNestID != updated.RemoteNestID) && m.remoteClient != nil {
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		if err := m.remoteClient.DeleteMission(ctx, *mission); err != nil {
			cancel()
			return err
		}
		cancel()
	}

	// Preserve metadata
	updated.ID = id
	updated.CreatedAt = mission.CreatedAt
	updated.LastRun = mission.LastRun
	updated.LastResult = mission.LastResult
	updated.LastOutput = mission.LastOutput
	updated.RunCount = mission.RunCount
	updated.Status = mission.Status
	updated.PreparationStatus = mission.PreparationStatus
	updated.LastPreparedAt = mission.LastPreparedAt
	if isRemoteMission(updated) {
		updated.RemoteRevision = newRemoteRevision()
		if err := m.syncRemoteMissionLocked(updated); err != nil {
			return err
		}
	}

	m.missions[id] = updated

	// Register new triggers
	if updated.Enabled {
		if isRemoteMission(updated) {
			// Remote eggs register their own schedule/trigger handlers.
		} else if updated.ExecutionType == ExecutionTriggered {
			m.registerTrigger(updated)
		} else if updated.ExecutionType == ExecutionScheduled && updated.Schedule != "" {
			cronID := "mission_" + id
			if m.cron == nil {
				return fmt.Errorf("cron manager is not configured")
			}
			if _, err := m.cron.ManageScheduleWithSource("add", cronID, updated.Schedule, updated.Prompt, "", "mission"); err != nil {
				return fmt.Errorf("failed to register mission with cron: %w", err)
			}
		}
	}

	// Invalidate prepared mission cache when mission content changes
	if m.preparedDB != nil {
		InvalidatePreparedMission(m.preparedDB, id)
	}

	return m.save()
}

// Delete removes a mission
func (m *MissionManagerV2) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission, ok := m.missions[id]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if mission.Locked {
		return fmt.Errorf("mission is locked")
	}

	// Unregister triggers
	if !isRemoteMission(mission) && mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" && m.cron != nil {
		cronID := "mission_" + id
		m.cron.ManageSchedule("remove", cronID, "", "", "")
	}
	if isRemoteMission(mission) && m.remoteClient != nil {
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		if err := m.remoteClient.DeleteMission(ctx, *mission); err != nil {
			cancel()
			return err
		}
		cancel()
	}

	delete(m.missions, id)
	m.queue.Remove(id)

	// Clean up prepared mission data
	if m.preparedDB != nil {
		DeletePreparedMission(m.preparedDB, id)
	}

	return m.save()
}

// Get returns a single mission
func (m *MissionManagerV2) Get(id string) (*MissionV2, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mission, ok := m.missions[id]
	if !ok {
		return nil, false
	}
	// Return a deep copy
	cp := *mission
	if mission.TriggerConfig != nil {
		tc := *mission.TriggerConfig
		cp.TriggerConfig = &tc
	}
	if mission.CheatsheetIDs != nil {
		cp.CheatsheetIDs = make([]string, len(mission.CheatsheetIDs))
		copy(cp.CheatsheetIDs, mission.CheatsheetIDs)
	}
	return &cp, true
}

// List returns all missions
func (m *MissionManagerV2) List() []*MissionV2 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	missions := make([]*MissionV2, 0, len(m.missions))
	for _, ms := range m.missions {
		cp := *ms
		if ms.TriggerConfig != nil {
			tc := *ms.TriggerConfig
			cp.TriggerConfig = &tc
		}
		if ms.CheatsheetIDs != nil {
			cp.CheatsheetIDs = make([]string, len(ms.CheatsheetIDs))
			copy(cp.CheatsheetIDs, ms.CheatsheetIDs)
		}
		missions = append(missions, &cp)
	}

	// Sort by created_at desc
	sort.Slice(missions, func(i, j int) bool {
		return missions[i].CreatedAt.After(missions[j].CreatedAt)
	})

	return missions
}

// GetQueue returns the current queue state
func (m *MissionManagerV2) GetQueue() (*MissionQueue, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queue, m.queue.GetRunning()
}

// SetResult updates mission result (called by agent when done)
func (m *MissionManagerV2) SetResult(id, result, output string) {
	m.OnMissionComplete(id, result, output)
}

// truncateString truncates a string to max length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
