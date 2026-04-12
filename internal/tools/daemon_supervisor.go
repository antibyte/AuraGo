package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/budget"
)

// DaemonSSEEventType identifies daemon-related SSE events.
// All daemon events use the single canonical type "daemon_update"
// so the frontend can listen with AuraSSE.on('daemon_update', ...).
const (
	DaemonSSEStatus       = "daemon_update"
	DaemonSSEWakeUp       = "daemon_update"
	DaemonSSEAutoDisabled = "daemon_update"
)

// DaemonEventBroadcaster abstracts SSE broadcasting to avoid an import cycle with server.
type DaemonEventBroadcaster interface {
	BroadcastDaemonEvent(eventType string, payload any)
}

// DaemonSupervisorConfig holds configuration for the DaemonSupervisor.
type DaemonSupervisorConfig struct {
	Enabled              bool
	MaxConcurrentDaemons int
	WakeUpGate           WakeUpGateConfig
	WorkspaceDir         string
	SkillsDir            string
	LogDir               string // defaults to data/daemon_logs
}

// DaemonSupervisor orchestrates all daemon skill processes.
// It discovers daemon skills, starts/stops DaemonRunners, and dispatches wake-up events.
type DaemonSupervisor struct {
	mu sync.RWMutex

	config  DaemonSupervisorConfig
	runners map[string]*DaemonRunner // skill ID → runner
	gate    *WakeUpGate
	wakeCh  chan daemonWakeEvent
	stopCh  chan struct{}
	stopped bool

	// Dependencies
	registry     *ProcessRegistry
	taskManager  *BackgroundTaskManager
	broadcaster  DaemonEventBroadcaster
	logger       *slog.Logger
	missionMgr   *MissionManagerV2
	cheatsheetDB *sql.DB
}

// NewDaemonSupervisor creates a new DaemonSupervisor.
func NewDaemonSupervisor(
	cfg DaemonSupervisorConfig,
	budgetTracker *budget.Tracker,
	registry *ProcessRegistry,
	taskManager *BackgroundTaskManager,
	broadcaster DaemonEventBroadcaster,
	logger *slog.Logger,
) *DaemonSupervisor {
	if cfg.MaxConcurrentDaemons <= 0 {
		cfg.MaxConcurrentDaemons = 5
	}
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join("data", "daemon_logs")
	}

	gate := NewWakeUpGate(cfg.WakeUpGate, budgetTracker, logger)

	return &DaemonSupervisor{
		config:      cfg,
		runners:     make(map[string]*DaemonRunner),
		gate:        gate,
		wakeCh:      make(chan daemonWakeEvent, 64),
		stopCh:      make(chan struct{}),
		registry:    registry,
		taskManager: taskManager,
		broadcaster: broadcaster,
		logger:      logger.With("component", "daemon_supervisor"),
	}
}

// Gate returns the WakeUpGate for external configuration (e.g., REST API toggle).
func (s *DaemonSupervisor) Gate() *WakeUpGate {
	return s.gate
}

// SetMissionManager injects the MissionManagerV2 for daemon-triggered mission execution.
func (s *DaemonSupervisor) SetMissionManager(mgr *MissionManagerV2) {
	s.missionMgr = mgr
}

// SetCheatsheetDB injects the cheatsheet database for daemon working instruction injection.
func (s *DaemonSupervisor) SetCheatsheetDB(db *sql.DB) {
	s.cheatsheetDB = db
}

// Start discovers daemon skills and starts enabled ones, then begins the wake-up dispatcher.
func (s *DaemonSupervisor) Start() error {
	if !s.config.Enabled {
		s.logger.Info("Daemon supervisor disabled")
		return nil
	}

	// Ensure log directory exists
	if err := os.MkdirAll(s.config.LogDir, 0755); err != nil {
		return fmt.Errorf("create daemon log dir: %w", err)
	}

	// Discover daemon skills
	skills, err := ListSkills(s.config.SkillsDir)
	if err != nil {
		return fmt.Errorf("list skills for daemon discovery: %w", err)
	}

	running := 0
	for _, manifest := range skills {
		if manifest.Daemon == nil || !manifest.Daemon.Enabled {
			continue
		}
		if running >= s.config.MaxConcurrentDaemons {
			s.logger.Warn("Max concurrent daemons reached, skipping remaining",
				"max", s.config.MaxConcurrentDaemons, "skill", manifest.Name)
			break
		}
		if err := s.startRunner(manifest); err != nil {
			s.logger.Error("Failed to start daemon", "skill", manifest.Name, "error", err)
			continue
		}
		running++
	}

	// Start the wake-up dispatcher goroutine
	go s.wakeUpDispatcher()

	s.logger.Info("Daemon supervisor started", "active_daemons", running)
	return nil
}

// Stop gracefully shuts down all daemons and the wake-up dispatcher.
func (s *DaemonSupervisor) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stopCh)
	runners := make([]*DaemonRunner, 0, len(s.runners))
	for _, r := range s.runners {
		runners = append(runners, r)
	}
	s.mu.Unlock()

	for _, r := range runners {
		if err := r.Stop(); err != nil {
			s.logger.Warn("Error stopping daemon", "skill_id", r.skillID, "error", err)
		}
	}
	s.logger.Info("Daemon supervisor stopped", "daemons_stopped", len(runners))
}

// startRunner creates and starts a DaemonRunner for the given manifest.
// Caller must NOT hold s.mu for write operations on s.runners — this method acquires it.
func (s *DaemonSupervisor) startRunner(manifest SkillManifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	skillID := manifest.Name // use skill name as ID (same as skill system convention)

	if _, exists := s.runners[skillID]; exists {
		return fmt.Errorf("daemon runner already exists for %q", skillID)
	}

	daemon := *manifest.Daemon
	daemon.ApplyDefaults()

	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:      skillID,
		SkillName:    manifest.Name,
		Config:       daemon,
		Manifest:     manifest,
		WorkspaceDir: s.config.WorkspaceDir,
		SkillsDir:    s.config.SkillsDir,
		Registry:     s.registry,
		LogDir:       s.config.LogDir,
		Logger:       s.logger,
		WakeCh:       s.wakeCh,
	})

	// Register in the wake-up gate
	s.gate.RegisterSkill(skillID, daemon.WakeAgent, daemon.WakeRateLimitSeconds)

	// Start the runner (lock is released internally by runner.Start())
	if err := runner.Start(); err != nil {
		s.gate.UnregisterSkill(skillID)
		return fmt.Errorf("start daemon runner: %w", err)
	}

	s.runners[skillID] = runner
	s.broadcastStatus(skillID, runner)
	return nil
}

// StopDaemon stops a specific daemon by skill ID.
func (s *DaemonSupervisor) StopDaemon(skillID string) error {
	s.mu.RLock()
	runner, ok := s.runners[skillID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no daemon runner for skill %q", skillID)
	}
	if err := runner.Stop(); err != nil {
		return err
	}
	s.broadcastStatus(skillID, runner)
	return nil
}

// StartDaemon starts (or restarts) a specific daemon by skill ID.
// If no runner exists yet (daemon had enabled=false in manifest), the manifest is
// loaded from disk, enabled=true is persisted, and a new runner is created.
func (s *DaemonSupervisor) StartDaemon(skillID string) error {
	s.mu.RLock()
	runner, ok := s.runners[skillID]
	s.mu.RUnlock()
	if !ok {
		// No runner yet — create one on demand from the manifest on disk.
		return s.startDaemonOnDemand(skillID)
	}
	if err := runner.Start(); err != nil {
		return err
	}
	s.broadcastStatus(skillID, runner)
	return nil
}

// startDaemonOnDemand scans the skills directory for a manifest whose Name matches
// skillID, sets enabled=true (persisted to disk), and starts a new runner.
func (s *DaemonSupervisor) startDaemonOnDemand(skillID string) error {
	entries, err := os.ReadDir(s.config.SkillsDir)
	if err != nil {
		return fmt.Errorf("cannot read skills directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(s.config.SkillsDir, entry.Name())
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			s.logger.Warn("Cannot read skill manifest, skipping", "file", entry.Name(), "error", readErr)
			continue
		}
		var manifest SkillManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			s.logger.Warn("Cannot parse skill manifest, skipping", "file", entry.Name(), "error", err)
			continue
		}
		if manifest.Name != skillID {
			continue
		}
		if manifest.Daemon == nil {
			return fmt.Errorf("skill %q is not a daemon skill", skillID)
		}
		// Persist enabled=true so the daemon survives restarts / RefreshSkills calls.
		manifest.Daemon.Enabled = true
		if updated, marshalErr := json.MarshalIndent(manifest, "", "  "); marshalErr == nil {
			if writeErr := os.WriteFile(filePath, updated, 0644); writeErr != nil {
				s.logger.Warn("Could not persist daemon enabled flag", "skill_id", skillID, "error", writeErr)
			} else {
				InvalidateSkillsCache(s.config.SkillsDir)
			}
		}
		s.logger.Info("Starting daemon on demand (was disabled)", "skill_id", skillID)
		if startErr := s.startRunner(manifest); startErr != nil {
			// Benign: another goroutine might have created the runner between our check
			// and startRunner's internal lock acquisition.
			if strings.Contains(startErr.Error(), "already exists") {
				return nil
			}
			return startErr
		}
		return nil
	}
	return fmt.Errorf("daemon skill %q not found — check skills directory", skillID)
}

// ReenableDaemon clears the auto-disabled flag and allows restart.
func (s *DaemonSupervisor) ReenableDaemon(skillID string) error {
	s.mu.RLock()
	runner, ok := s.runners[skillID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no daemon runner for skill %q", skillID)
	}
	runner.Reenable()
	s.gate.ResetEscalation(skillID)
	s.broadcastStatus(skillID, runner)
	return nil
}

// ListDaemons returns the state of all registered daemons.
func (s *DaemonSupervisor) ListDaemons() []DaemonState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	states := make([]DaemonState, 0, len(s.runners))
	for _, r := range s.runners {
		states = append(states, r.State())
	}
	return states
}

// GetDaemonState returns the state for a specific daemon.
func (s *DaemonSupervisor) GetDaemonState(skillID string) (DaemonState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runner, ok := s.runners[skillID]
	if !ok {
		return DaemonState{}, false
	}
	return runner.State(), true
}

// RunnerCount returns the number of registered daemons.
func (s *DaemonSupervisor) RunnerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runners)
}

// wakeUpDispatcher reads wake-up events from the shared channel and dispatches them
// through the WakeUpGate to the BackgroundTaskManager.
func (s *DaemonSupervisor) wakeUpDispatcher() {
	for {
		select {
		case <-s.stopCh:
			// Drain pending events so senders don't block indefinitely after shutdown.
			for {
				select {
				case <-s.wakeCh:
				default:
					return
				}
			}
		case event := <-s.wakeCh:
			s.handleWakeUp(event)
		}
	}
}

// handleWakeUp processes a single wake-up event from a daemon.
func (s *DaemonSupervisor) handleWakeUp(event daemonWakeEvent) {
	skillID := event.SkillID

	allowed, denial := s.gate.Allow(skillID)
	if !allowed {
		s.logger.Info("Wake-up denied",
			"skill_id", skillID,
			"layer", denial.Layer,
			"reason", denial.Reason,
		)
		s.gate.RecordSuppressed(skillID)

		s.mu.RLock()
		if runner, ok := s.runners[skillID]; ok {
			runner.IncrementSuppressed()
		}
		s.mu.RUnlock()

		if s.gate.ShouldAutoDisable(skillID) {
			s.autoDisableDaemon(skillID, "circuit breaker: too many wake-ups per hour")
		}
		return
	}

	s.gate.RecordWakeUp(skillID, 0)

	s.mu.RLock()
	if runner, ok := s.runners[skillID]; ok {
		runner.IncrementWakeUp()
	}
	s.mu.RUnlock()

	// Look up the daemon manifest for trigger_mission_id / cheatsheet_id
	var daemonCfg *DaemonManifest
	s.mu.RLock()
	if r, ok := s.runners[skillID]; ok {
		daemonCfg = &r.config
	}
	s.mu.RUnlock()

	triggerMissionID := ""
	cheatsheetID := ""
	if daemonCfg != nil {
		triggerMissionID = daemonCfg.TriggerMissionID
		cheatsheetID = daemonCfg.CheatsheetID
	}

	// Path A: wake-agent (existing background prompt)
	if daemonCfg == nil || daemonCfg.WakeAgent {
		s.dispatchAgentWakeUp(event)
	}

	// Path B: trigger a mission
	if triggerMissionID != "" && s.missionMgr != nil {
		s.dispatchMissionTrigger(event, triggerMissionID, cheatsheetID)
	}

	// Broadcast SSE event
	if s.broadcaster != nil {
		s.broadcaster.BroadcastDaemonEvent(DaemonSSEWakeUp, map[string]any{
			"skill_id":   skillID,
			"skill_name": event.SkillName,
			"severity":   event.Message.Severity,
			"message":    truncateString(event.Message.Message, 200),
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	}
}

// dispatchAgentWakeUp schedules a background prompt via the task manager (existing behavior).
func (s *DaemonSupervisor) dispatchAgentWakeUp(event daemonWakeEvent) {
	prompt := s.buildWakeUpPrompt(event)
	_, err := s.taskManager.ScheduleCronPrompt(prompt, BackgroundTaskScheduleOptions{
		Source:      fmt.Sprintf("daemon:%s", event.SkillID),
		Description: fmt.Sprintf("Daemon wake-up from %s", event.SkillName),
	})
	if err != nil {
		s.logger.Error("Failed to schedule daemon wake-up",
			"skill_id", event.SkillID,
			"error", err,
		)
		return
	}
	s.logger.Info("Daemon wake-up dispatched",
		"skill_id", event.SkillID,
		"severity", event.Message.Severity,
	)
}

// dispatchMissionTrigger queues a mission via MissionManagerV2, optionally injecting
// the daemon-assigned cheatsheet as working instructions.
func (s *DaemonSupervisor) dispatchMissionTrigger(event daemonWakeEvent, missionID, cheatsheetID string) {
	triggerType := "daemon_wake"
	triggerData := s.buildTriggerData(event, cheatsheetID)

	var extraCSIDs []string
	var extraPromptSuffix string
	if cheatsheetID != "" && s.cheatsheetDB != nil {
		sheet, err := CheatsheetGet(s.cheatsheetDB, cheatsheetID)
		if err != nil || !sheet.Active {
			s.logger.Warn("Daemon cheatsheet not found or inactive, skipping injection",
				"cheatsheet_id", cheatsheetID,
				"error", err,
			)
		} else {
			extraPromptSuffix = fmt.Sprintf("\n\n[Daemon Working Instructions from Cheat Sheet: %q]\n%s", sheet.Name, sheet.Content)
			for _, a := range sheet.Attachments {
				extraPromptSuffix += fmt.Sprintf("\n\n[Cheat Sheet Attachment: %q]\n%s", a.Filename, a.Content)
			}
		}
	}

	if err := s.missionMgr.TriggerMissionWithOptions(missionID, triggerType, triggerData, extraCSIDs, extraPromptSuffix); err != nil {
		s.logger.Error("Failed to trigger daemon mission",
			"skill_id", event.SkillID,
			"mission_id", missionID,
			"error", err,
		)
		return
	}
	s.logger.Info("Daemon mission triggered",
		"skill_id", event.SkillID,
		"mission_id", missionID,
		"cheatsheet_id", cheatsheetID,
	)
}

// buildTriggerData produces a JSON string with daemon event metadata for the mission trigger.
func (s *DaemonSupervisor) buildTriggerData(event daemonWakeEvent, cheatsheetID string) string {
	data := map[string]string{
		"source":     "daemon",
		"skill_id":   event.SkillID,
		"skill_name": event.SkillName,
		"severity":   event.Message.Severity,
		"message":    truncateString(event.Message.Message, 500),
		"timestamp":  event.Timestamp.Format(time.RFC3339),
	}
	if cheatsheetID != "" {
		data["cheatsheet_id"] = cheatsheetID
	}
	if event.Message.Data != nil {
		const maxD = 2048
		d := string(event.Message.Data)
		if len(d) > maxD {
			d = d[:maxD] + "... [truncated]"
		}
		data["data"] = d
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// buildWakeUpPrompt constructs the prompt that the agent loop will receive.
func (s *DaemonSupervisor) buildWakeUpPrompt(event daemonWakeEvent) string {
	severity := event.Message.Severity
	if severity == "" {
		severity = "info"
	}

	prompt := fmt.Sprintf("[DAEMON EVENT — %s] (severity: %s)\n%s",
		event.SkillName,
		severity,
		event.Message.Message,
	)

	if event.Message.Data != nil {
		const maxDataBytes = 4096
		data := string(event.Message.Data)
		if len(data) > maxDataBytes {
			data = data[:maxDataBytes] + "... [truncated]"
		}
		prompt += fmt.Sprintf("\n\nAdditional data: %s", data)
	}

	return prompt
}

// autoDisableDaemon stops a daemon and marks it as auto-disabled.
func (s *DaemonSupervisor) autoDisableDaemon(skillID, reason string) {
	s.mu.RLock()
	runner, ok := s.runners[skillID]
	s.mu.RUnlock()
	if !ok {
		return
	}

	runner.Disable(reason)

	s.logger.Warn("Daemon auto-disabled by circuit breaker",
		"skill_id", skillID,
		"reason", reason,
	)

	// Broadcast SSE event
	if s.broadcaster != nil {
		s.broadcaster.BroadcastDaemonEvent(DaemonSSEAutoDisabled, map[string]any{
			"skill_id":   skillID,
			"skill_name": runner.skillName,
			"reason":     reason,
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	}
}

// broadcastStatus sends the current daemon state via SSE.
func (s *DaemonSupervisor) broadcastStatus(skillID string, runner *DaemonRunner) {
	if s.broadcaster == nil {
		return
	}
	state := runner.State()
	s.broadcaster.BroadcastDaemonEvent(DaemonSSEStatus, state)
}

// RefreshSkills re-scans the skills directory and starts/stops daemons as needed.
func (s *DaemonSupervisor) RefreshSkills() error {
	if !s.config.Enabled {
		return nil
	}

	skills, err := ListSkills(s.config.SkillsDir)
	if err != nil {
		return fmt.Errorf("list skills for refresh: %w", err)
	}

	// Build a set of daemon skills that should be running
	desired := make(map[string]SkillManifest)
	for _, m := range skills {
		if m.Daemon != nil && m.Daemon.Enabled {
			desired[m.Name] = m
		}
	}

	s.mu.RLock()
	currentIDs := make([]string, 0, len(s.runners))
	for id := range s.runners {
		currentIDs = append(currentIDs, id)
	}
	s.mu.RUnlock()

	// Stop runners that are no longer desired
	for _, id := range currentIDs {
		if _, wanted := desired[id]; !wanted {
			s.logger.Info("Stopping removed/disabled daemon", "skill_id", id)
			_ = s.StopDaemon(id)
			s.gate.UnregisterSkill(id)
			s.mu.Lock()
			delete(s.runners, id)
			s.mu.Unlock()
		}
	}

	// Start new daemons. For auto-disabled runners that are still desired,
	// clear the auto-disabled flag so they can restart.
	s.mu.RLock()
	for id := range desired {
		if runner, ok := s.runners[id]; ok && runner.IsAutoDisabled() {
			s.mu.RUnlock()
			runner.Reenable()
			s.gate.ResetEscalation(id)
			s.logger.Info("Re-enabling auto-disabled daemon during refresh", "skill_id", id)
			s.mu.RLock()
		}
	}
	s.mu.RUnlock()

	// Start new daemons.
	// Re-read the count under lock before each attempt so concurrent start/stop
	// operations don't cause us to exceed MaxConcurrentDaemons or skip slots.
	for id, manifest := range desired {
		s.mu.RLock()
		running := len(s.runners)
		s.mu.RUnlock()

		if running >= s.config.MaxConcurrentDaemons {
			s.logger.Warn("Max concurrent daemons reached during refresh", "max", s.config.MaxConcurrentDaemons)
			break
		}
		if err := s.startRunner(manifest); err != nil {
			// startRunner itself checks for existence under a write lock; an
			// "already exists" error is benign (another goroutine beat us to it).
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			s.logger.Error("Failed to start daemon during refresh", "skill", id, "error", err)
			continue
		}
	}

	return nil
}
