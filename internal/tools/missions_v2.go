package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
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
	TriggerMissionCompleted TriggerType = "mission_completed" // Another mission finished
	TriggerEmailReceived    TriggerType = "email_received"    // Email received
	TriggerWebhook          TriggerType = "webhook"           // Webhook fired
	TriggerEggHatched       TriggerType = "egg_hatched"       // Egg deployed to a nest
	TriggerNestCleared      TriggerType = "nest_cleared"      // Nest removed
	TriggerMQTTMessage      TriggerType = "mqtt_message"      // MQTT message received
	TriggerSystemStartup    TriggerType = "system_startup"    // AuraGo Startup
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
	CheatsheetIDs []string      `json:"cheatsheet_ids,omitempty"` // Linked cheat sheet IDs for prompt expansion
}

// QueueItem represents a mission in the execution queue
type QueueItem struct {
	MissionID   string    `json:"mission_id"`
	Priority    int       `json:"priority"` // 3=high, 2=medium, 1=low
	EnqueuedAt  time.Time `json:"enqueued_at"`
	TriggerType string    `json:"trigger_type,omitempty"`
	TriggerData string    `json:"trigger_data,omitempty"` // JSON data from trigger
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
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if already queued
	for _, item := range q.items {
		if item.MissionID == missionID {
			return // Already queued
		}
	}

	prio := 2 // medium
	switch priority {
	case "high":
		prio = 3
	case "low":
		prio = 1
	}

	item := QueueItem{
		MissionID:   missionID,
		Priority:    prio,
		EnqueuedAt:  time.Now(),
		TriggerType: triggerType,
		TriggerData: triggerData,
	}

	q.items = append(q.items, item)
	q.sort()
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
	onMissionComplete func(completedID, result, output string) // callback for mission completion
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
		file:     filepath.Join(dataDir, "missions_v2.json"),
		missions: make(map[string]*MissionV2),
		queue:    NewMissionQueue(),
		cron:     cronMgr,
		ctx:      ctx,
		cancel:   cancel,
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
			m.missions[mission.ID] = mission
			if mission.Status == "running" {
				mission.Status = "idle" // Reset on startup
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", m.file, err)
	}

	// Setup triggers
	m.setupTriggers()

	// Start queue processor
	go m.processQueue()

	return nil
}

// Stop shuts down the mission manager
func (m *MissionManagerV2) Stop() {
	m.cancel()
}

func (m *MissionManagerV2) save() error {
	// NOTE: must be called while m.mu is already held (Lock or RLock).
	// Do NOT acquire m.mu here — all callers hold the write lock.
	missions := make([]*MissionV2, 0, len(m.missions))
	for _, mission := range m.missions {
		missions = append(missions, mission)
	}

	data, err := json.MarshalIndent(missions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.file, data, 0644)
}

// setupTriggers initializes all active triggers
func (m *MissionManagerV2) setupTriggers() {
	for _, mission := range m.missions {
		if !mission.Enabled || mission.ExecutionType != ExecutionTriggered {
			continue
		}
		m.registerTrigger(mission)
	}
}

// registerTrigger sets up a single trigger
func (m *MissionManagerV2) registerTrigger(mission *MissionV2) {
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
	m.mu.Lock()
	if m.queue.GetRunning() != "" {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	item, ok := m.queue.Dequeue()
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

	mission.Status = "running"
	mission.LastRun = time.Now()
	m.save()

	callback := m.callback
	prompt := mission.Prompt
	missionID := mission.ID
	cheatsheetIDs := mission.CheatsheetIDs
	cheatsheetDB := m.cheatsheetDB
	m.mu.Unlock()

	if callback == nil {
		// No callback set — mark mission as error and release queue
		m.mu.Lock()
		if ms, ok := m.missions[missionID]; ok {
			ms.Status = "idle"
			ms.LastResult = "error"
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

	// Enhance prompt with trigger data
	if item.TriggerData != "" {
		prompt = fmt.Sprintf("%s\n\n[Trigger Context: %s]", prompt, item.TriggerData)
	}
	go callback(prompt, missionID)
}

// OnMissionComplete handles mission completion and triggers dependent missions
func (m *MissionManagerV2) OnMissionComplete(missionID, result, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update mission status
	if mission, ok := m.missions[missionID]; ok {
		mission.Status = "idle"
		mission.LastResult = result
		mission.LastOutput = truncateString(output, 500)
		mission.RunCount++
		m.save()
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
		if cfg.RequireSuccess && result != "success" {
			continue
		}

		// Queue the triggered mission
		m.queue.Enqueue(mission.ID, mission.Priority, "mission_completed",
			fmt.Sprintf(`{"source_mission":"%s","result":"%s"}`, missionID, result))
		mission.Status = "queued"
	}
	m.save()
}

// TriggerMission manually triggers a mission (for webhooks, emails, etc.)
func (m *MissionManagerV2) TriggerMission(missionID, triggerType, triggerData string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mission, ok := m.missions[missionID]
	if !ok {
		return fmt.Errorf("mission not found")
	}
	if !mission.Enabled {
		return fmt.Errorf("mission is disabled")
	}

	m.queue.Enqueue(missionID, mission.Priority, triggerType, triggerData)
	mission.Status = "queued"
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
		mission.Status = "queued"
	}
	m.save()
}

// NotifySystemStartup fires mission triggers meant to run when AuraGo starts.
func (m *MissionManagerV2) NotifySystemStartup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mission := range m.missions {
		if !mission.Enabled ||
			mission.ExecutionType != ExecutionTriggered ||
			mission.TriggerType != TriggerSystemStartup {
			continue
		}

		triggerData, _ := json.Marshal(map[string]string{
			"event": "system_startup",
			"time":  time.Now().Format(time.RFC3339),
		})
		m.queue.Enqueue(mission.ID, mission.Priority, string(TriggerSystemStartup), string(triggerData))
		mission.Status = "queued"
	}
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

	// For manual execution, we still queue but with high priority
	m.queue.Enqueue(id, "high", "manual", "")
	mission.Status = "queued"
	m.save()
	return nil
}

// Create adds a new mission
func (m *MissionManagerV2) Create(mission *MissionV2) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mission.ID == "" {
		mission.ID = fmt.Sprintf("mission_%d", time.Now().UnixNano())
	}
	if mission.Priority == "" {
		mission.Priority = "medium"
	}
	if mission.ExecutionType == "" {
		mission.ExecutionType = ExecutionManual
	}
	mission.Status = "idle"
	mission.CreatedAt = time.Now()
	mission.Enabled = true

	m.missions[mission.ID] = mission

	// Register trigger if needed
	if mission.ExecutionType == ExecutionTriggered {
		m.registerTrigger(mission)
	}

	// Register with cron if scheduled
	if mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" {
		cronID := "mission_" + mission.ID
		m.cron.ManageSchedule("add", cronID, mission.Schedule, mission.Prompt)
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

	// Unregister old triggers
	if mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" {
		cronID := "mission_" + id
		m.cron.ManageSchedule("remove", cronID, "", "")
	}

	// Preserve metadata
	updated.ID = id
	updated.CreatedAt = mission.CreatedAt
	updated.LastRun = mission.LastRun
	updated.LastResult = mission.LastResult
	updated.LastOutput = mission.LastOutput
	updated.RunCount = mission.RunCount
	updated.Status = mission.Status

	m.missions[id] = updated

	// Register new triggers
	if updated.Enabled {
		if updated.ExecutionType == ExecutionTriggered {
			m.registerTrigger(updated)
		} else if updated.ExecutionType == ExecutionScheduled && updated.Schedule != "" {
			cronID := "mission_" + id
			m.cron.ManageSchedule("add", cronID, updated.Schedule, updated.Prompt)
		}
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
	if mission.ExecutionType == ExecutionScheduled && mission.Schedule != "" {
		cronID := "mission_" + id
		m.cron.ManageSchedule("remove", cronID, "", "")
	}

	delete(m.missions, id)
	m.queue.Remove(id)
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
	// Return a copy
	copy := *mission
	return &copy, true
}

// List returns all missions
func (m *MissionManagerV2) List() []*MissionV2 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	missions := make([]*MissionV2, 0, len(m.missions))
	for _, m := range m.missions {
		copy := *m
		missions = append(missions, &copy)
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
