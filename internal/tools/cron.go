package tools

import (
	"aurago/internal/i18n"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	ID         string `json:"id"`
	CronExpr   string `json:"cron_expr"`
	TaskPrompt string `json:"task_prompt"`
	Disabled   bool   `json:"disabled,omitempty"`
}

type CronManager struct {
	mu           sync.Mutex
	engine       *cron.Cron
	file         string
	jobs         []CronJob
	cronEntryIDs map[string]cron.EntryID
	callback     func(prompt string)
}

func NewCronManager(dataDir string) *CronManager {
	return &CronManager{
		engine:       cron.New(), // Standard 5-field cron (minute hour dom month dow)
		file:         filepath.Join(dataDir, "crontab.json"),
		jobs:         []CronJob{},
		cronEntryIDs: make(map[string]cron.EntryID),
	}
}

func (m *CronManager) Start(callback func(prompt string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callback = callback

	// Load existing from JSON
	data, err := os.ReadFile(m.file)
	if err == nil {
		if err := json.Unmarshal(data, &m.jobs); err != nil {
			return fmt.Errorf("failed to parse %s: %w", m.file, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", m.file, err)
	}

	for _, job := range m.jobs {
		m.scheduleInternal(job)
	}

	m.engine.Start()
	return nil
}

// Unlocked scheduling logic — must be called with m.mu held.
func (m *CronManager) scheduleInternal(job CronJob) error {
	if job.Disabled {
		return nil
	} // Rebind job.TaskPrompt so the closure captures the correct string
	prompt := job.TaskPrompt
	// Caller already holds m.mu, so read m.callback directly (no nested lock).
	callback := m.callback
	entryID, err := m.engine.AddFunc(job.CronExpr, func() {
		if callback != nil {
			callback(prompt)
		}
	})
	if err != nil {
		return err
	}
	m.cronEntryIDs[job.ID] = entryID
	return nil
}

// Stop shuts down the cron engine. Must be called during graceful shutdown
// to ensure running jobs complete and internal goroutines exit.
func (m *CronManager) Stop() {
	ctx := m.engine.Stop()
	<-ctx.Done()
}

func (m *CronManager) save() error {
	data, err := json.MarshalIndent(m.jobs, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp cron file: %w", err)
	}
	if err := os.Rename(tmp, m.file); err != nil {
		return fmt.Errorf("rename temp cron file: %w", err)
	}
	return nil
}

// GetJobs returns a copy of the current cron jobs slice.
func (m *CronManager) GetJobs() []CronJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CronJob, len(m.jobs))
	copy(out, m.jobs)
	return out
}

// ManageSchedule handles cron job operations with i18n support.
// The lang parameter is used for i18n of user-facing messages. If empty, English is used.
func (m *CronManager) ManageSchedule(operation, id, expr, prompt string, lang string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch operation {
	case "add":
		if expr == "" || prompt == "" {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_add_required")), nil
		}

		// Parse check
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		_, err := parser.Parse(expr)
		if err != nil {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_invalid_expr", err)), nil
		}

		jobID := id
		if jobID == "" {
			jobID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		// Idempotent add: remove existing job with same ID to avoid duplicates
		if existingEntryID, exists := m.cronEntryIDs[jobID]; exists && jobID != "" {
			m.engine.Remove(existingEntryID)
			delete(m.cronEntryIDs, jobID)
			filtered := []CronJob{}
			for _, j := range m.jobs {
				if j.ID != jobID {
					filtered = append(filtered, j)
				}
			}
			m.jobs = filtered
		}

		job := CronJob{
			ID:         jobID,
			CronExpr:   expr,
			TaskPrompt: prompt,
		}

		if err := m.scheduleInternal(job); err != nil {
			return "", err
		}

		m.jobs = append(m.jobs, job)
		if err := m.save(); err != nil {
			return "", err
		}

		return fmt.Sprintf(`{"status": "success", "message": "%s", "id": "%s"}`, i18n.T(lang, "tools.cron_scheduled"), jobID), nil

	case "remove":
		if id == "" {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_remove_id_required")), nil
		}

		entryID, exists := m.cronEntryIDs[id]
		if !exists {
			return fmt.Sprintf(`{"status": "warning", "message": "%s"}`, i18n.T(lang, "tools.cron_job_not_found")), nil
		}

		m.engine.Remove(entryID)
		delete(m.cronEntryIDs, id)

		filtered := []CronJob{}
		for _, j := range m.jobs {
			if j.ID != id {
				filtered = append(filtered, j)
			}
		}
		m.jobs = filtered
		if err := m.save(); err != nil {
			return "", err
		}

		return fmt.Sprintf(`{"status": "success", "message": "%s"}`, i18n.T(lang, "tools.cron_removed")), nil

	case "enable":
		if id == "" {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_enable_id_required")), nil
		}
		for i, job := range m.jobs {
			if job.ID == id {
				if job.Disabled {
					m.jobs[i].Disabled = false
					if err := m.scheduleInternal(m.jobs[i]); err != nil {
						return "", err
					}
					if err := m.save(); err != nil {
						return "", err
					}
					return fmt.Sprintf(`{"status": "success", "message": "%s"}`, i18n.T(lang, "tools.cron_enabled")), nil
				}
				return fmt.Sprintf(`{"status": "success", "message": "%s"}`, i18n.T(lang, "tools.cron_already_enabled")), nil
			}
		}
		return fmt.Sprintf(`{"status": "warning", "message": "%s"}`, i18n.T(lang, "tools.cron_job_not_found")), nil

	case "disable":
		if id == "" {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_disable_id_required")), nil
		}
		for i, job := range m.jobs {
			if job.ID == id {
				if !job.Disabled {
					m.jobs[i].Disabled = true
					if entryID, exists := m.cronEntryIDs[id]; exists {
						m.engine.Remove(entryID)
						delete(m.cronEntryIDs, id)
					}
					if err := m.save(); err != nil {
						return "", err
					}
					return fmt.Sprintf(`{"status": "success", "message": "%s"}`, i18n.T(lang, "tools.cron_disabled")), nil
				}
				return fmt.Sprintf(`{"status": "success", "message": "%s"}`, i18n.T(lang, "tools.cron_already_disabled")), nil
			}
		}
		return fmt.Sprintf(`{"status": "warning", "message": "%s"}`, i18n.T(lang, "tools.cron_job_not_found")), nil

	case "list":
		if len(m.jobs) == 0 {
			return `{"status": "success", "jobs": []}`, nil
		}
		data, _ := json.Marshal(m.jobs)
		return fmt.Sprintf(`{"status": "success", "jobs": %s}`, string(data)), nil

	default:
		return "", fmt.Errorf("unsupported manage_schedule operation: %s", operation)
	}
}
