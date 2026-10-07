package tools

import (
	"aurago/internal/i18n"
	"encoding/json"
	"errors"
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

const schedulerDisabledByConfiguration = "scheduler disabled by configuration"

type CronJobRuntimeStatus struct {
	CronJob
	Registered bool   `json:"registered"`
	LastError  string `json:"last_error,omitempty"`
}

type CronManager struct {
	mu                 sync.Mutex
	engine             *cron.Cron
	file               string
	store              *systemTaskStore
	jobs               []CronJob
	cronEntryIDs       map[string]cron.EntryID
	registrationErrors map[string]string
	callback           func(prompt string)
	runners            map[string]func(jobID, prompt string)
	started            bool
	// runtimeJobs holds the ids of jobs that live in memory only (AddRuntimeJob,
	// MakeRuntimeOnly); saveLocked leaves them out of the persisted job list.
	runtimeJobs map[string]bool
}

func NewCronManager(dataDir string) *CronManager {
	store, _ := newSystemTaskStore(dataDir)
	return &CronManager{
		engine:             cron.New(cron.WithParser(newCronParser())),
		file:               filepath.Join(dataDir, "crontab.json"),
		store:              store,
		jobs:               []CronJob{},
		cronEntryIDs:       make(map[string]cron.EntryID),
		registrationErrors: make(map[string]string),
		runners:            make(map[string]func(jobID, prompt string)),
		runtimeJobs:        make(map[string]bool),
	}
}

// NextRun reports the next scheduled execution of a registered job. The
// boolean is false when the job is unknown or the engine has not been started
// yet: robfig/cron only computes Next for running engines.
func (m *CronManager) NextRun(jobID string) (time.Time, bool) {
	m.mu.Lock()
	entryID, ok := m.cronEntryIDs[jobID]
	m.mu.Unlock()
	if !ok {
		return time.Time{}, false
	}
	entry := m.engine.Entry(entryID)
	if !entry.Valid() || entry.Next.IsZero() {
		return time.Time{}, false
	}
	return entry.Next, true
}

func newCronParser() cron.Parser {
	return cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
}

func (m *CronManager) Start(callback func(prompt string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callback = callback

	// Runtime-only jobs added before Start are not in the persisted list; keep them.
	var runtimeJobs []CronJob
	for _, job := range m.jobs {
		if m.runtimeJobs[job.ID] {
			runtimeJobs = append(runtimeJobs, job)
		}
	}
	loaded, err := m.loadLocked()
	if err != nil {
		slog.Warn("[CronManager] Failed to load persisted jobs; starting with empty job list", "error", err)
		m.jobs = []CronJob{}
	} else if !loaded {
		m.jobs = []CronJob{}
	}
	m.keepRuntimeJobsLocked(runtimeJobs)

	err = m.refreshRuntimeRegistrationsLocked()

	m.engine.Start()
	m.started = true
	return err
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
	j := job
	entryID, err := m.engine.AddFunc(j.CronExpr, func() {
		m.mu.Lock()
		runner := m.runners[j.Source]
		if runner == nil && j.Source == flowCronSource {
			// Flow schedules run only through the mission manager's flow runner, never as an
			// agent prompt; before that runner exists (startup) the run is skipped.
			m.mu.Unlock()
			slog.Info("[CronManager] Flow cron job fired before the flow runner was ready; skipped", "id", j.ID)
			return
		}
		if runner == nil && m.callback != nil {
			cb := m.callback
			m.mu.Unlock()
			slog.Info("[CronManager] Cron job fired",
				"id", j.ID,
				"source", j.Source,
				"dispatch", "fallback_callback")
			cb(j.TaskPrompt)
			return
		}
		m.mu.Unlock()
		if runner != nil {
			slog.Info("[CronManager] Cron job fired",
				"id", j.ID,
				"source", j.Source,
				"dispatch", "source_runner")
			runner(j.ID, j.TaskPrompt)
			return
		}
		slog.Warn("[CronManager] Cron job fired without runner or fallback callback",
			"id", j.ID,
			"source", j.Source)
	})
	if err != nil {
		return err
	}
	m.cronEntryIDs[j.ID] = entryID
	delete(m.registrationErrors, j.ID)
	return nil
}

func schedulerRuntimeEnabled() bool {
	perms, configured := currentRuntimePermissions()
	return configured && perms.SchedulerEnabled
}

func (m *CronManager) clearRuntimeEntriesLocked() {
	for jobID, entryID := range m.cronEntryIDs {
		m.engine.Remove(entryID)
		delete(m.cronEntryIDs, jobID)
	}
}

func (m *CronManager) refreshRuntimeRegistrationsLocked() error {
	m.clearRuntimeEntriesLocked()

	if !schedulerRuntimeEnabled() {
		for _, job := range m.jobs {
			if job.Disabled {
				delete(m.registrationErrors, job.ID)
				continue
			}
			m.registrationErrors[job.ID] = schedulerDisabledByConfiguration
		}
		return nil
	}

	var scheduleErrs []error
	for _, job := range m.jobs {
		if job.Disabled {
			delete(m.registrationErrors, job.ID)
			continue
		}
		if err := m.scheduleInternal(job); err != nil {
			slog.Warn("[CronManager] Failed to register persisted cron job",
				"id", job.ID,
				"source", job.Source,
				"cron_expr", job.CronExpr,
				"error", err)
			m.registrationErrors[job.ID] = err.Error()
			scheduleErrs = append(scheduleErrs, fmt.Errorf("register cron job %q (%s): %w", job.ID, job.CronExpr, err))
		}
	}
	return errors.Join(scheduleErrs...)
}

func (m *CronManager) RefreshRuntimePermissions() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshRuntimeRegistrationsLocked()
}

// Stop shuts down the cron engine. Must be called during graceful shutdown
// to ensure running jobs complete and internal goroutines exit.
func (m *CronManager) Stop() {
	ctx := m.engine.Stop()
	<-ctx.Done()
}

func (m *CronManager) Close() error {
	m.mu.Lock()
	wasStarted := m.started
	m.started = false
	store := m.store
	m.store = nil
	m.mu.Unlock()

	if wasStarted {
		m.Stop()
	}
	if store != nil {
		return store.release()
	}
	return nil
}

func (m *CronManager) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

// keepRuntimeJobsLocked puts the runtime-only jobs back after Start loaded the persisted
// list; a loaded job under the id of one of them gives way to it. Caller holds m.mu.
func (m *CronManager) keepRuntimeJobsLocked(runtimeJobs []CronJob) {
	if len(runtimeJobs) == 0 {
		return
	}
	jobs := make([]CronJob, 0, len(m.jobs)+len(runtimeJobs))
	replaced := false
	for _, job := range m.jobs {
		if m.runtimeJobs[job.ID] {
			replaced = true
			continue
		}
		jobs = append(jobs, job)
	}
	m.jobs = append(jobs, runtimeJobs...)
	if replaced {
		if err := m.saveLocked(); err != nil {
			slog.Warn("[CronManager] Failed to save cron jobs", "error", err)
		}
	}
}

// persistedJobsLocked returns the jobs that are saved: all but the runtime-only ones.
// Caller holds m.mu.
func (m *CronManager) persistedJobsLocked() []CronJob {
	if len(m.runtimeJobs) == 0 {
		return m.jobs
	}
	jobs := make([]CronJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		if !m.runtimeJobs[job.ID] {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// saveLocked must be called with m.mu held. Runtime-only jobs are never written.
func (m *CronManager) saveLocked() error {
	jobs := m.persistedJobsLocked()
	if m.store != nil {
		return m.store.save(systemTaskNamespaceCron, jobs)
	}
	if err := os.MkdirAll(filepath.Dir(m.file), 0o750); err != nil {
		return fmt.Errorf("ensure cron dir: %w", err)
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
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
			if err := m.saveLocked(); err != nil {
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

func (m *CronManager) GetJobsWithRuntimeStatus() []CronJobRuntimeStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobsWithRuntimeStatusLocked()
}

func (m *CronManager) jobsWithRuntimeStatusLocked() []CronJobRuntimeStatus {
	out := make([]CronJobRuntimeStatus, 0, len(m.jobs))
	for _, job := range m.jobs {
		_, registered := m.cronEntryIDs[job.ID]
		if job.Disabled {
			registered = false
		}
		out = append(out, CronJobRuntimeStatus{
			CronJob:    job,
			Registered: registered,
			LastError:  m.registrationErrors[job.ID],
		})
	}
	return out
}

func (m *CronManager) removeJobLocked(id string) bool {
	found := false
	if entryID, hasEntry := m.cronEntryIDs[id]; hasEntry {
		m.engine.Remove(entryID)
		delete(m.cronEntryIDs, id)
		found = true
	}
	delete(m.registrationErrors, id)
	delete(m.runtimeJobs, id)

	filtered := []CronJob{}
	for _, j := range m.jobs {
		if j.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, j)
	}
	m.jobs = filtered
	return found
}

// ManageSchedule handles cron job operations with i18n support.
// The lang parameter is used for i18n of user-facing messages. If empty, English is used.
func (m *CronManager) ManageSchedule(operation, id, expr, prompt string, lang string) (string, error) {
	return m.ManageScheduleWithSource(operation, id, expr, prompt, lang, "")
}

// ManageScheduleWithSource is like ManageSchedule but allows setting a job source.
func (m *CronManager) ManageScheduleWithSource(operation, id, expr, prompt string, lang string, source string) (string, error) {
	if err := requireSchedulerPermission(operation); err != nil {
		return fmt.Sprintf(`{"status": "error", "message": "%s"}`, err.Error()), nil
	}

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

		// Idempotent add: remove any existing persisted/runtime job with the same ID.
		if jobID != "" {
			m.removeJobLocked(jobID)
		}

		job := CronJob{
			ID:         jobID,
			CronExpr:   expr,
			TaskPrompt: prompt,
			Source:     source,
		}

		if err := m.scheduleInternal(job); err != nil {
			m.registrationErrors[jobID] = err.Error()
			return "", err
		}

		m.jobs = append(m.jobs, job)
		if err := m.saveLocked(); err != nil {
			return "", err
		}

		return fmt.Sprintf(`{"status": "success", "message": "%s", "id": "%s"}`, i18n.T(lang, "tools.cron_scheduled"), jobID), nil

	case "remove":
		if id == "" {
			return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T(lang, "tools.cron_remove_id_required")), nil
		}

		found := m.removeJobLocked(id)
		if !found {
			return fmt.Sprintf(`{"status": "warning", "message": "%s"}`, i18n.T(lang, "tools.cron_job_not_found")), nil
		}
		if err := m.saveLocked(); err != nil {
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
						m.registrationErrors[id] = err.Error()
						return "", err
					}
					if err := m.saveLocked(); err != nil {
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
					delete(m.registrationErrors, id)
					if err := m.saveLocked(); err != nil {
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
		data, _ := json.Marshal(m.jobsWithRuntimeStatusLocked())
		return fmt.Sprintf(`{"status": "success", "jobs": %s}`, string(data)), nil

	default:
		return "", fmt.Errorf("unsupported manage_schedule operation: %s", operation)
	}
}

// AddRuntimeJob schedules a job like ManageScheduleWithSource "add" (the same scheduler
// permission, expression check and JSON answer), but keeps it in memory only: GetJobs,
// "list", the dashboard and the agent's cron tools see it, no save ever writes it to the
// persisted job list, and it ends with the process, so its owner registers it again on
// every start. A persisted job under the same id is replaced and leaves the store. id is
// required. EasyDrag's flow schedules use it, so an older AuraGo, which hands every job
// it loads without a runner to its agent, never runs them.
func (m *CronManager) AddRuntimeJob(id, expr, prompt, source string) (string, error) {
	if err := requireSchedulerPermission("add"); err != nil {
		return fmt.Sprintf(`{"status": "error", "message": "%s"}`, err.Error()), nil
	}
	if id == "" || expr == "" || prompt == "" {
		return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T("", "tools.cron_add_required")), nil
	}
	if _, err := newCronParser().Parse(expr); err != nil {
		return fmt.Sprintf(`{"status": "error", "message": "%s"}`, i18n.T("", "tools.cron_invalid_expr", err)), nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	wasPersisted := false
	for _, job := range m.jobs {
		if job.ID == id && !m.runtimeJobs[id] {
			wasPersisted = true
		}
	}
	m.removeJobLocked(id)
	job := CronJob{ID: id, CronExpr: expr, TaskPrompt: prompt, Source: source}
	if err := m.scheduleInternal(job); err != nil {
		m.registrationErrors[id] = err.Error()
		return "", err
	}
	if m.runtimeJobs == nil {
		m.runtimeJobs = make(map[string]bool)
	}
	m.runtimeJobs[id] = true
	m.jobs = append(m.jobs, job)
	if wasPersisted {
		if err := m.saveLocked(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf(`{"status": "success", "message": "%s", "id": "%s"}`, i18n.T("", "tools.cron_scheduled"), id), nil
}

// MakeRuntimeOnly turns every job of source into a runtime-only job (see AddRuntimeJob)
// and rewrites the persisted job list without them. The jobs keep their schedule and
// keep running; only their storage changes, so it needs no scheduler permission. It
// reports how many jobs left the persisted list; an empty source changes nothing.
// MissionManagerV2.Start uses it for flow schedule jobs that an earlier build persisted.
func (m *CronManager) MakeRuntimeOnly(source string) (int, error) {
	if source == "" {
		return 0, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	moved := 0
	for _, job := range m.jobs {
		if job.Source != source || m.runtimeJobs[job.ID] {
			continue
		}
		if m.runtimeJobs == nil {
			m.runtimeJobs = make(map[string]bool)
		}
		m.runtimeJobs[job.ID] = true
		moved++
	}
	if moved == 0 {
		return 0, nil
	}
	return moved, m.saveLocked()
}
