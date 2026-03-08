package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Mission represents a user-defined task that the agent can execute on demand or on schedule.
type Mission struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Prompt     string    `json:"prompt"`
	Schedule   string    `json:"schedule"` // cron expression or "" for manual
	Priority   string    `json:"priority"` // low | medium | high
	Enabled    bool      `json:"enabled"`
	LastRun    time.Time `json:"last_run"`
	LastResult string    `json:"last_result"` // success | error | ""
	LastOutput string    `json:"last_output"` // truncated last response
	RunCount   int       `json:"run_count"`
	CreatedAt  time.Time `json:"created_at"`
	Locked     bool      `json:"locked"` // Prevents deletion when true
}

// MissionManager provides thread-safe CRUD for missions with cron integration.
type MissionManager struct {
	mu       sync.Mutex
	file     string
	missions []Mission
	cron     *CronManager
	callback func(prompt string) // agent invocation callback
}

// NewMissionManager creates a new MissionManager. Call Start() after creation.
func NewMissionManager(dataDir string, cronMgr *CronManager) *MissionManager {
	return &MissionManager{
		file:     filepath.Join(dataDir, "missions.json"),
		missions: []Mission{},
		cron:     cronMgr,
	}
}

// Start loads missions from disk and registers scheduled missions with the CronManager.
func (m *MissionManager) Start(callback func(prompt string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callback = callback

	data, err := os.ReadFile(m.file)
	if err == nil {
		if err := json.Unmarshal(data, &m.missions); err != nil {
			return fmt.Errorf("failed to parse %s: %w", m.file, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", m.file, err)
	}

	// Register scheduled missions with the CronManager
	for _, mission := range m.missions {
		if mission.Schedule != "" && mission.Enabled {
			cronID := "mission_" + mission.ID
			m.cron.ManageSchedule("add", cronID, mission.Schedule, mission.Prompt)
		}
	}

	return nil
}

func (m *MissionManager) save() error {
	data, err := json.MarshalIndent(m.missions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.file, data, 0644)
}

// List returns a copy of all missions.
func (m *MissionManager) List() []Mission {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Mission, len(m.missions))
	copy(out, m.missions)
	return out
}

// Get returns a single mission by ID.
func (m *MissionManager) Get(id string) (Mission, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mission := range m.missions {
		if mission.ID == id {
			return mission, true
		}
	}
	return Mission{}, false
}

// Create adds a new mission and schedules it if it has a cron expression.
func (m *MissionManager) Create(mission Mission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mission.ID == "" {
		mission.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if mission.Priority == "" {
		mission.Priority = "medium"
	}
	mission.CreatedAt = time.Now()
	mission.Enabled = true

	m.missions = append(m.missions, mission)

	// Register with cron if scheduled
	if mission.Schedule != "" && mission.Enabled {
		cronID := "mission_" + mission.ID
		if _, err := m.cron.ManageSchedule("add", cronID, mission.Schedule, mission.Prompt); err != nil {
			return fmt.Errorf("failed to schedule mission: %w", err)
		}
	}

	return m.save()
}

// Update modifies an existing mission. Re-schedules if the cron expression changed.
func (m *MissionManager) Update(id string, updated Mission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, mission := range m.missions {
		if mission.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("mission %s not found", id)
	}

	old := m.missions[idx]

	// Remove old cron entry if it existed
	if old.Schedule != "" {
		cronID := "mission_" + old.ID
		m.cron.ManageSchedule("remove", cronID, "", "")
	}

	// Preserve metadata
	updated.ID = id
	updated.CreatedAt = old.CreatedAt
	updated.LastRun = old.LastRun
	updated.LastResult = old.LastResult
	updated.LastOutput = old.LastOutput
	updated.RunCount = old.RunCount

	// Only update locked state if explicitly changed, default to existing state
	// Assuming incoming updated struct preserves it if not explicitly changed by UI
	m.missions[idx] = updated

	// Re-schedule if needed
	if updated.Schedule != "" && updated.Enabled {
		cronID := "mission_" + updated.ID
		if _, err := m.cron.ManageSchedule("add", cronID, updated.Schedule, updated.Prompt); err != nil {
			return fmt.Errorf("failed to reschedule mission: %w", err)
		}
	}

	return m.save()
}

// Delete removes a mission and unschedules it. Returns an error if the mission is locked.
func (m *MissionManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filtered := []Mission{}
	found := false
	for _, mission := range m.missions {
		if mission.ID == id {
			if mission.Locked {
				return fmt.Errorf("mission %s is locked and cannot be deleted. Unlock it first", id)
			}
			found = true
			// Remove from cron
			if mission.Schedule != "" {
				cronID := "mission_" + mission.ID
				m.cron.ManageSchedule("remove", cronID, "", "")
			}
		} else {
			filtered = append(filtered, mission)
		}
	}
	if !found {
		return fmt.Errorf("mission %s not found", id)
	}

	m.missions = filtered
	return m.save()
}

// SetResult updates the last run result for a mission.
func (m *MissionManager) SetResult(id, result, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, mission := range m.missions {
		if mission.ID == id {
			m.missions[i].LastRun = time.Now()
			m.missions[i].LastResult = result
			m.missions[i].RunCount++
			// Truncate output to max 500 chars
			if len(output) > 500 {
				output = output[:500] + "..."
			}
			m.missions[i].LastOutput = output
			m.save()
			return
		}
	}
}

// RunNow triggers a mission immediately via the callback.
func (m *MissionManager) RunNow(id string) error {
	mission, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("mission %s not found", id)
	}

	if m.callback != nil {
		go m.callback(mission.Prompt)
	}

	return nil
}
