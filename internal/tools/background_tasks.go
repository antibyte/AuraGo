package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	BackgroundTaskTypeFollowUp     = "follow_up"
	BackgroundTaskTypeCronPrompt   = "cron_prompt"
	BackgroundTaskTypeWaitForEvent = "wait_for_event"
)

const (
	BackgroundTaskStatusQueued    = "queued"
	BackgroundTaskStatusWaiting   = "waiting"
	BackgroundTaskStatusRunning   = "running"
	BackgroundTaskStatusCompleted = "completed"
	BackgroundTaskStatusFailed    = "failed"
	BackgroundTaskStatusCanceled  = "canceled"
)

type BackgroundTask struct {
	ID                 string          `json:"id"`
	Type               string          `json:"type"`
	Status             string          `json:"status"`
	Source             string          `json:"source,omitempty"`
	Description        string          `json:"description,omitempty"`
	Payload            json.RawMessage `json:"payload"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	StartedAt          *time.Time      `json:"started_at,omitempty"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	NextAttemptAt      time.Time       `json:"next_attempt_at"`
	RetryCount         int             `json:"retry_count"`
	MaxRetries         int             `json:"max_retries"`
	RetryDelaySeconds  int             `json:"retry_delay_seconds,omitempty"`
	TimeoutSeconds     int             `json:"timeout_seconds,omitempty"`
	NotifyOnCompletion bool            `json:"notify_on_completion"`
	LastError          string          `json:"last_error,omitempty"`
	Result             string          `json:"result,omitempty"`
}

type FollowUpTaskPayload struct {
	Prompt string `json:"prompt"`
}

type WaitForEventTaskPayload struct {
	EventType           string    `json:"event_type"`
	TaskPrompt          string    `json:"task_prompt,omitempty"`
	URL                 string    `json:"url,omitempty"`
	Host                string    `json:"host,omitempty"`
	Port                int       `json:"port,omitempty"`
	FilePath            string    `json:"file_path,omitempty"`
	PID                 int       `json:"pid,omitempty"`
	PollIntervalSeconds int       `json:"poll_interval_seconds,omitempty"`
	TimeoutSeconds      int       `json:"timeout_seconds,omitempty"`
	ScheduledAt         time.Time `json:"scheduled_at"`
	InitialExists       bool      `json:"initial_exists,omitempty"`
	InitialModUnixNano  int64     `json:"initial_mod_unix_nano,omitempty"`
	InitialSize         int64     `json:"initial_size,omitempty"`
}

type BackgroundTaskScheduleOptions struct {
	Source             string
	Description        string
	Delay              time.Duration
	MaxRetries         int
	RetryDelay         time.Duration
	Timeout            time.Duration
	NotifyOnCompletion bool
}

type BackgroundTaskSummary struct {
	Queued    int `json:"queued"`
	Waiting   int `json:"waiting"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
	Total     int `json:"total"`
}

type BackgroundTaskManager struct {
	mu           sync.Mutex
	file         string
	tasks        map[string]*BackgroundTask
	logger       *slog.Logger
	httpClient   *http.Client
	registry     *ProcessRegistry
	executor     func(prompt string, timeout time.Duration) error
	notifier     func(title, body string)
	ctx          context.Context
	cancel       context.CancelFunc
	trigger      chan struct{}
	workerOnce   sync.Once
	shutdownOnce sync.Once
}

var (
	defaultBackgroundTaskManagerMu sync.RWMutex
	defaultBackgroundTaskManager   *BackgroundTaskManager
)

func SetDefaultBackgroundTaskManager(mgr *BackgroundTaskManager) {
	defaultBackgroundTaskManagerMu.Lock()
	defer defaultBackgroundTaskManagerMu.Unlock()
	defaultBackgroundTaskManager = mgr
}

func DefaultBackgroundTaskManager() *BackgroundTaskManager {
	defaultBackgroundTaskManagerMu.RLock()
	defer defaultBackgroundTaskManagerMu.RUnlock()
	return defaultBackgroundTaskManager
}

func NewBackgroundTaskManager(dataDir string, logger *slog.Logger) *BackgroundTaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &BackgroundTaskManager{
		file:   filepath.Join(dataDir, "background_tasks.json"),
		tasks:  make(map[string]*BackgroundTask),
		logger: logger,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		ctx:     ctx,
		cancel:  cancel,
		trigger: make(chan struct{}, 1),
	}
	if err := mgr.load(); err != nil && logger != nil {
		logger.Warn("Failed to load background tasks", "error", err)
	}
	return mgr
}

func (m *BackgroundTaskManager) SetLoopbackExecutor(fn func(prompt string, timeout time.Duration) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executor = fn
}

func (m *BackgroundTaskManager) SetProcessRegistry(registry *ProcessRegistry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry = registry
}

func (m *BackgroundTaskManager) SetNotifier(fn func(title, body string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifier = fn
}

func (m *BackgroundTaskManager) Start() {
	m.workerOnce.Do(func() {
		go m.run()
	})
}

func (m *BackgroundTaskManager) Stop() {
	m.shutdownOnce.Do(func() {
		m.cancel()
	})
}

func (m *BackgroundTaskManager) ScheduleFollowUp(prompt string, opts BackgroundTaskScheduleOptions) (*BackgroundTask, error) {
	payload, err := json.Marshal(FollowUpTaskPayload{Prompt: prompt})
	if err != nil {
		return nil, fmt.Errorf("marshal follow-up payload: %w", err)
	}
	task := m.newTask(BackgroundTaskTypeFollowUp, payload, opts)
	return m.enqueue(task)
}

func (m *BackgroundTaskManager) ScheduleCronPrompt(prompt string, opts BackgroundTaskScheduleOptions) (*BackgroundTask, error) {
	payload, err := json.Marshal(FollowUpTaskPayload{Prompt: prompt})
	if err != nil {
		return nil, fmt.Errorf("marshal cron payload: %w", err)
	}
	task := m.newTask(BackgroundTaskTypeCronPrompt, payload, opts)
	return m.enqueue(task)
}

func (m *BackgroundTaskManager) ScheduleWaitForEvent(payload WaitForEventTaskPayload, opts BackgroundTaskScheduleOptions) (*BackgroundTask, error) {
	if payload.ScheduledAt.IsZero() {
		payload.ScheduledAt = time.Now().UTC()
	}
	if payload.FilePath != "" {
		if info, err := os.Stat(payload.FilePath); err == nil {
			payload.InitialExists = true
			payload.InitialModUnixNano = info.ModTime().UnixNano()
			payload.InitialSize = info.Size()
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wait payload: %w", err)
	}
	task := m.newTask(BackgroundTaskTypeWaitForEvent, raw, opts)
	return m.enqueue(task)
}

func (m *BackgroundTaskManager) CancelTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return false
	}
	if task.Status == BackgroundTaskStatusCompleted || task.Status == BackgroundTaskStatusFailed || task.Status == BackgroundTaskStatusCanceled {
		return false
	}
	now := time.Now().UTC()
	task.Status = BackgroundTaskStatusCanceled
	task.UpdatedAt = now
	task.CompletedAt = &now
	task.LastError = ""
	task.Result = "task canceled"
	_ = m.saveLocked()
	return true
}

func (m *BackgroundTaskManager) RetryTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return false
	}
	if task.Status != BackgroundTaskStatusFailed {
		return false
	}
	now := time.Now().UTC()
	task.Status = BackgroundTaskStatusQueued
	task.UpdatedAt = now
	task.StartedAt = nil
	task.CompletedAt = nil
	task.LastError = ""
	task.Result = ""
	task.NextAttemptAt = now
	task.RetryCount = 0
	_ = m.saveLocked()
	m.signal()
	return true
}

func (m *BackgroundTaskManager) ListTasks(limit int) []*BackgroundTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*BackgroundTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		cp := *task
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (m *BackgroundTaskManager) GetTask(id string) (*BackgroundTask, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, false
	}
	cp := *task
	return &cp, true
}

func (m *BackgroundTaskManager) Summary() BackgroundTaskSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	var summary BackgroundTaskSummary
	for _, task := range m.tasks {
		summary.Total++
		switch task.Status {
		case BackgroundTaskStatusQueued:
			summary.Queued++
		case BackgroundTaskStatusWaiting:
			summary.Waiting++
		case BackgroundTaskStatusRunning:
			summary.Running++
		case BackgroundTaskStatusCompleted:
			summary.Completed++
		case BackgroundTaskStatusFailed:
			summary.Failed++
		case BackgroundTaskStatusCanceled:
			summary.Canceled++
		}
	}
	return summary
}

func (m *BackgroundTaskManager) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.processDueTasks()
		case <-m.trigger:
			m.processDueTasks()
		}
	}
}

func (m *BackgroundTaskManager) processDueTasks() {
	now := time.Now().UTC()
	due := m.claimDueTasks(now)
	for _, task := range due {
		m.executeTask(task)
	}
}

func (m *BackgroundTaskManager) claimDueTasks(now time.Time) []*BackgroundTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for id, task := range m.tasks {
		if task.Status == BackgroundTaskStatusQueued || task.Status == BackgroundTaskStatusWaiting {
			if !task.NextAttemptAt.After(now) {
				ids = append(ids, id)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		return m.tasks[ids[i]].NextAttemptAt.Before(m.tasks[ids[j]].NextAttemptAt)
	})
	out := make([]*BackgroundTask, 0, len(ids))
	for _, id := range ids {
		task := m.tasks[id]
		if task.Status == BackgroundTaskStatusCanceled || task.Status == BackgroundTaskStatusCompleted || task.Status == BackgroundTaskStatusFailed {
			continue
		}
		started := now
		task.Status = BackgroundTaskStatusRunning
		task.UpdatedAt = now
		task.StartedAt = &started
		cp := *task
		out = append(out, &cp)
	}
	_ = m.saveLocked()
	return out
}

func (m *BackgroundTaskManager) executeTask(task *BackgroundTask) {
	switch task.Type {
	case BackgroundTaskTypeFollowUp, BackgroundTaskTypeCronPrompt:
		m.executePromptTask(task)
	case BackgroundTaskTypeWaitForEvent:
		m.executeWaitTask(task)
	default:
		m.failTask(task.ID, fmt.Sprintf("unsupported background task type %q", task.Type), true)
	}
}

func (m *BackgroundTaskManager) executePromptTask(task *BackgroundTask) {
	var payload FollowUpTaskPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		m.failTask(task.ID, fmt.Sprintf("invalid prompt payload: %v", err), true)
		return
	}
	if strings.TrimSpace(payload.Prompt) == "" {
		m.failTask(task.ID, "background prompt is empty", true)
		return
	}

	timeout := 2 * time.Minute
	if task.TimeoutSeconds > 0 {
		timeout = time.Duration(task.TimeoutSeconds) * time.Second
	} else if task.Type == BackgroundTaskTypeCronPrompt {
		timeout = 3 * time.Minute
	}

	m.mu.Lock()
	executor := m.executor
	m.mu.Unlock()
	if executor == nil {
		m.failTask(task.ID, "no loopback executor configured", true)
		return
	}

	if err := executor(payload.Prompt, timeout); err != nil {
		m.retryOrFail(task.ID, fmt.Sprintf("loopback execution failed: %v", err))
		return
	}

	m.completeTask(task.ID, fmt.Sprintf("%s executed successfully", task.Type))
}

func (m *BackgroundTaskManager) executeWaitTask(task *BackgroundTask) {
	var payload WaitForEventTaskPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		m.failTask(task.ID, fmt.Sprintf("invalid wait payload: %v", err), true)
		return
	}
	pollInterval := time.Duration(maxInt(payload.PollIntervalSeconds, 5)) * time.Second
	timeout := time.Duration(maxInt(payload.TimeoutSeconds, 600)) * time.Second
	if payload.ScheduledAt.IsZero() {
		payload.ScheduledAt = time.Now().UTC()
	}

	met, details, err := m.checkWaitCondition(payload)
	if err != nil {
		m.failTask(task.ID, err.Error(), true)
		return
	}
	if !met {
		if time.Since(payload.ScheduledAt) >= timeout {
			m.failTask(task.ID, "wait_for_event timed out", true)
			return
		}
		m.rescheduleWaiting(task.ID, pollInterval, details)
		return
	}

	if strings.TrimSpace(payload.TaskPrompt) == "" {
		m.completeTask(task.ID, details)
		return
	}

	eventPrompt := payload.TaskPrompt
	if details != "" {
		eventPrompt = fmt.Sprintf("%s\n\n[Wait event completed: %s]", payload.TaskPrompt, details)
	}
	promptTask := *task
	promptTask.Type = BackgroundTaskTypeFollowUp
	promptTask.Payload, _ = json.Marshal(FollowUpTaskPayload{Prompt: eventPrompt})
	m.executePromptTask(&promptTask)
}

func (m *BackgroundTaskManager) checkWaitCondition(payload WaitForEventTaskPayload) (bool, string, error) {
	switch strings.ToLower(strings.TrimSpace(payload.EventType)) {
	case "process_exited":
		if payload.PID <= 0 {
			return false, "", fmt.Errorf("wait_for_event process_exited requires pid")
		}
		m.mu.Lock()
		registry := m.registry
		m.mu.Unlock()
		if registry == nil {
			return false, "", fmt.Errorf("process registry is unavailable")
		}
		if info, ok := registry.Get(payload.PID); ok && info != nil && info.Alive {
			return false, fmt.Sprintf("process %d is still running", payload.PID), nil
		}
		return true, fmt.Sprintf("process %d exited", payload.PID), nil
	case "http_available":
		urlStr := strings.TrimSpace(payload.URL)
		if urlStr == "" && payload.Host != "" {
			hostPort := payload.Host
			if payload.Port > 0 {
				hostPort = net.JoinHostPort(payload.Host, strconv.Itoa(payload.Port))
			}
			urlStr = "http://" + hostPort
		}
		if urlStr == "" {
			return false, "", fmt.Errorf("wait_for_event http_available requires url or host")
		}
		req, err := http.NewRequest(http.MethodGet, urlStr, nil)
		if err != nil {
			return false, "", fmt.Errorf("invalid wait url: %w", err)
		}
		resp, err := m.httpClient.Do(req)
		if err != nil {
			return false, fmt.Sprintf("endpoint not available yet: %v", err), nil
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return false, fmt.Sprintf("endpoint returned %d", resp.StatusCode), nil
		}
		return true, fmt.Sprintf("endpoint %s is available (%d)", urlStr, resp.StatusCode), nil
	case "file_changed":
		if payload.FilePath == "" {
			return false, "", fmt.Errorf("wait_for_event file_changed requires file_path")
		}
		info, err := os.Stat(payload.FilePath)
		if err != nil {
			if os.IsNotExist(err) {
				return false, fmt.Sprintf("file %s does not exist yet", payload.FilePath), nil
			}
			return false, "", fmt.Errorf("stat file: %w", err)
		}
		if !payload.InitialExists {
			return true, fmt.Sprintf("file %s was created", payload.FilePath), nil
		}
		if info.ModTime().UnixNano() != payload.InitialModUnixNano || info.Size() != payload.InitialSize {
			return true, fmt.Sprintf("file %s changed", payload.FilePath), nil
		}
		return false, fmt.Sprintf("file %s is unchanged", payload.FilePath), nil
	default:
		return false, "", fmt.Errorf("unsupported wait_for_event type %q", payload.EventType)
	}
}

func (m *BackgroundTaskManager) completeTask(id, result string) {
	m.finishTask(id, BackgroundTaskStatusCompleted, result, "")
}

func (m *BackgroundTaskManager) failTask(id, errMsg string, notify bool) {
	m.finishTask(id, BackgroundTaskStatusFailed, "", errMsg)
	if !notify {
		return
	}
}

func (m *BackgroundTaskManager) finishTask(id, status, result, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	task.Status = status
	task.UpdatedAt = now
	task.CompletedAt = &now
	task.LastError = errMsg
	task.Result = result
	_ = m.saveLocked()
	maybeNotify := task.NotifyOnCompletion
	notify := m.notifier
	if maybeNotify && notify != nil {
		title := "Background task completed"
		body := fmt.Sprintf("%s finished successfully.", task.DescriptionOrType())
		if status == BackgroundTaskStatusFailed {
			title = "Background task failed"
			body = fmt.Sprintf("%s failed: %s", task.DescriptionOrType(), errMsg)
		}
		go notify(title, body)
	}
}

func (m *BackgroundTaskManager) retryOrFail(id, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	task.UpdatedAt = now
	task.LastError = errMsg
	task.RetryCount++
	if task.RetryCount > task.MaxRetries {
		task.Status = BackgroundTaskStatusFailed
		task.CompletedAt = &now
		_ = m.saveLocked()
		if task.NotifyOnCompletion && m.notifier != nil {
			go m.notifier("Background task failed", fmt.Sprintf("%s failed: %s", task.DescriptionOrType(), errMsg))
		}
		return
	}
	task.Status = BackgroundTaskStatusQueued
	retryDelay := time.Duration(maxInt(task.RetryDelaySeconds, 60)) * time.Second
	task.NextAttemptAt = now.Add(retryDelay)
	_ = m.saveLocked()
	m.signal()
}

func (m *BackgroundTaskManager) rescheduleWaiting(id string, delay time.Duration, details string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	task.Status = BackgroundTaskStatusWaiting
	task.UpdatedAt = now
	task.NextAttemptAt = now.Add(delay)
	task.Result = details
	_ = m.saveLocked()
}

func (m *BackgroundTaskManager) newTask(taskType string, payload json.RawMessage, opts BackgroundTaskScheduleOptions) *BackgroundTask {
	now := time.Now().UTC()
	delay := opts.Delay
	if delay < 0 {
		delay = 0
	}
	task := &BackgroundTask{
		ID:                 fmt.Sprintf("bt_%d", now.UnixNano()),
		Type:               taskType,
		Status:             BackgroundTaskStatusQueued,
		Source:             opts.Source,
		Description:        opts.Description,
		Payload:            payload,
		CreatedAt:          now,
		UpdatedAt:          now,
		NextAttemptAt:      now.Add(delay),
		MaxRetries:         maxInt(opts.MaxRetries, 0),
		RetryDelaySeconds:  int(maxDuration(opts.RetryDelay, time.Minute).Seconds()),
		TimeoutSeconds:     int(maxDuration(opts.Timeout, 0).Seconds()),
		NotifyOnCompletion: opts.NotifyOnCompletion,
	}
	return task
}

func (m *BackgroundTaskManager) enqueue(task *BackgroundTask) (*BackgroundTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[task.ID] = task
	if err := m.saveLocked(); err != nil {
		delete(m.tasks, task.ID)
		return nil, err
	}
	cp := *task
	m.signal()
	return &cp, nil
}

func (m *BackgroundTaskManager) load() error {
	if _, err := os.Stat(m.file); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat background tasks file: %w", err)
	}
	data, err := os.ReadFile(m.file)
	if err != nil {
		return fmt.Errorf("read background tasks file: %w", err)
	}
	var tasks []*BackgroundTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("parse background tasks file: %w", err)
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.Status == BackgroundTaskStatusRunning {
			task.Status = BackgroundTaskStatusQueued
			task.StartedAt = nil
		}
		if task.NextAttemptAt.IsZero() {
			task.NextAttemptAt = time.Now().UTC()
		}
		m.tasks[task.ID] = task
	}
	return nil
}

func (m *BackgroundTaskManager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.file), 0o750); err != nil {
		return fmt.Errorf("ensure background task dir: %w", err)
	}
	tasks := make([]*BackgroundTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal background tasks: %w", err)
	}
	tmp := m.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp background tasks: %w", err)
	}
	if err := os.Rename(tmp, m.file); err != nil {
		return fmt.Errorf("rename temp background tasks: %w", err)
	}
	return nil
}

func (m *BackgroundTaskManager) signal() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

func (t *BackgroundTask) DescriptionOrType() string {
	if strings.TrimSpace(t.Description) != "" {
		return t.Description
	}
	return strings.ReplaceAll(t.Type, "_", " ")
}

func maxInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

func maxDuration(v, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	return v
}
