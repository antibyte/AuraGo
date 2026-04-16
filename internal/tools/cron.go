package tools

import (
	"aurago/internal/i18n"
	"encoding/json"
	"fmt"
	"log/slog"
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
	Source     string `json:"source,omitempty"`
}

type CronManager struct {
	mu           sync.Mutex
	engine       *cron.Cron
	file         string
	store        *systemTaskStore
	jobs         []CronJob
	cronEntryIDs map[string]cron.EntryID
	callback     func(prompt string)
	runners      map[string]func(jobID, prompt string)
}

func NewCronManager(dataDir string) *CronManager {
	store, _ := newSystemTaskStore(dataDir)
	return &CronManager{
		engine:       cron.New(cron.WithParser(newCronParser())),
		file:         filepath.Join(dataDir, "crontab.json"),
		store:        store,
		jobs:         []CronJob{},
		cronEntryIDs: make(map[string]cron.EntryID),
		runners:      make(map[string]func(jobID, prompt string)),
	}
}

func newCronParser() cron.Parser {
	return cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
}

func (m *CronManager) Start(callback func(prompt string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callback = callback

	loaded, err := m.loadLocked()
	if err != nil {
		slog.Warn("[CronManager] Failed to load persisted jobs; starting with empty job list", "error", err)
		m.jobs = []CronJob{}
	} else if !loaded {
		m.jobs = []CronJob{}
	}

	for _, job := range m.jobs {
		m.scheduleInternal(job)
	}

	m.engine.Start()
	return nil
}

// RegisterRunner registers a source-specific runner for cron jobs.
// When a job with a matching Source fires, the runner receives the job ID
// and prompt instead of the global fallback callback.
func (m *CronManager) RegisterRunner(source string, runner func(jobID, prompt string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runners[source] = runner
}

// Unlocked scheduling logic — must be called with m.mu held.
func (m *CronManager) scheduleInternal(job CronJob) error {
	if job.Disabled {
		return nil
	}
	entryID, err := m.engine.AddFunc(job.CronExpr, func() {
		m.mu.Lock()
		runner := m.runners[job.Source]
		if runner == nil && m.callback != nil {
			cb := m.callback
			m.mu.Unlock()
			cb(job.TaskPrompt)
			return
		}
		j := job
		m.mu.Unlock()
		if runner != nil {
			runner(j.ID, j.TaskPrompt)
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

func (m *CronManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store == nil {
		return nil
	}
	return m.store.close()
}

func (m *CronManager) save() error {
	if m.store != nil {
		return m.store.save(systemTaskNamespaceCron, m.jobs)
	}
	if err := os.MkdirAll(filepath.Dir(m.file), 0o750); err != nil {
		return fmt.Errorf("ensure cron dir: %w", err)
	}
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

func (m *CronManager) loadLocked() (bool, error) {
	if m.store != nil {
		loaded, err := m.store.load(systemTaskNamespaceCron, &m.jobs)
		if err != nil {
			return false, err
		}
		if loaded {
			return true, nil
		}
	}

	data, err := os.ReadFile(m.file)
	if err == nil {
		if err := json.Unmarshal(data, &m.jobs); err != nil {
			return false, fmt.Errorf("failed to parse %s: %w", m.file, err)
		}
		if m.store != nil {
			if err := m.store.save(systemTaskNamespaceCron, m.jobs); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to read %s: %w", m.file, err)
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
	return m.ManageScheduleWithSource(operation, id, expr, prompt, lang, "")
}

// ManageScheduleWithSource is like ManageSchedule but allows setting a job source.
func (m *CronManager) ManageScheduleWithSource(operation, id, expr, prompt string, lang string, source string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch operation {
	case "add":
		if expr == "" || prompt == "" {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_add_required")), nil
		}

		// Parse check
		parser := newCronParser()
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
			Source:     source,
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
