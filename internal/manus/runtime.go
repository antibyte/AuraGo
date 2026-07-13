package manus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RuntimeConfig binds local policy and filesystem limits to the API client.
type RuntimeConfig struct {
	Policy       Policy
	WorkspaceDir string
	DownloadRoot string
	MaxFileBytes int64
	PollInterval time.Duration
	MaxWait      time.Duration
}

// Runtime safely coordinates the remote API and local task ledger.
type Runtime struct {
	client       *Client
	ledger       TaskStore
	policy       Policy
	workspaceDir string
	downloadRoot string
	maxFileBytes int64
	pollInterval time.Duration
	maxWait      time.Duration
}

// TaskState is the normalized lifecycle state returned to the AuraGo agent.
type TaskState struct {
	State               string      `json:"state"`
	Task                Task        `json:"task"`
	TaskURL             string      `json:"task_url,omitempty"`
	WaitingType         string      `json:"waiting_type,omitempty"`
	WaitingDescription  string      `json:"waiting_description,omitempty"`
	Messages            []TaskEvent `json:"messages,omitempty"`
	PollAfterSeconds    int         `json:"poll_after_seconds,omitempty"`
	ConfirmationEventID string      `json:"-"`
}

// NewRuntime constructs the local policy boundary around an authenticated client.
func NewRuntime(client *Client, ledger TaskStore, cfg RuntimeConfig) *Runtime {
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	maxWait := cfg.MaxWait
	if maxWait <= 0 || maxWait > 60*time.Second {
		maxWait = 60 * time.Second
	}
	return &Runtime{
		client: client, ledger: ledger, policy: cfg.Policy,
		workspaceDir: cfg.WorkspaceDir, downloadRoot: cfg.DownloadRoot,
		maxFileBytes: cfg.MaxFileBytes, pollInterval: pollInterval, maxWait: maxWait,
	}
}

// CreateTask validates policy, uploads approved files, and records a successful task.
func (r *Runtime) CreateTask(ctx context.Context, request CreateTaskRequest, localPaths []string) (CreateTaskResult, error) {
	if err := r.policy.Authorize("create_task"); err != nil {
		return CreateTaskResult{}, err
	}
	if err := r.policy.ValidateResources(request.ProjectID, request.Connectors, request.EnableSkills, request.ForceSkills); err != nil {
		return CreateTaskResult{}, err
	}
	if err := ValidateAgentProfile(request.AgentProfile); err != nil {
		return CreateTaskResult{}, err
	}
	if err := ValidateStructuredOutputSchema(request.StructuredOutputSchema); err != nil {
		return CreateTaskResult{}, err
	}
	if err := r.ledger.PreflightWrite(ctx); err != nil {
		return CreateTaskResult{}, fmt.Errorf("preflight Manus task persistence: %w", err)
	}
	content, err := r.contentWithUploads(ctx, request.Content, localPaths)
	if err != nil {
		return CreateTaskResult{}, err
	}
	request.Content = content
	result, err := r.client.CreateTask(ctx, request)
	if err != nil {
		return CreateTaskResult{}, err
	}
	if strings.TrimSpace(result.TaskID) == "" {
		return result, &OutcomeUnknownError{Operation: "/v2/task.create", Err: fmt.Errorf("Manus task creation returned no task ID")}
	}
	if err := r.persistMutation(ctx, TaskRecord{
		TaskID: result.TaskID, Title: result.TaskTitle, TaskURL: result.TaskURL,
		Status: "running", AgentProfile: request.AgentProfile,
	}); err != nil {
		return result, &RemoteAppliedError{Operation: "create_task", TaskID: result.TaskID, TaskURL: result.TaskURL, Err: err}
	}
	return result, nil
}

// GetTask refreshes one AuraGo-tracked task.
func (r *Runtime) GetTask(ctx context.Context, taskID string) (Task, error) {
	record, err := r.requireTracked(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	task, err := r.client.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	if task.TaskURL == "" {
		task.TaskURL = record.TaskURL
	}
	if err := r.updateRecord(ctx, record, task, record.LastCursor); err != nil {
		return Task{}, err
	}
	return task, nil
}

// ListMessages retrieves safe, non-verbose events for a tracked task.
func (r *Runtime) ListMessages(ctx context.Context, options ListMessagesOptions) (MessagePage, error) {
	record, err := r.requireTracked(ctx, options.TaskID)
	if err != nil {
		return MessagePage{}, err
	}
	page, err := r.client.ListMessages(ctx, options)
	if err != nil {
		return MessagePage{}, err
	}
	sanitizeTaskEvents(page.Messages)
	if page.NextCursor != "" {
		record.LastCursor = page.NextCursor
		record.UpdatedAt = time.Now().UTC()
		if err := r.ledger.Upsert(ctx, record); err != nil {
			return MessagePage{}, err
		}
	}
	return page, nil
}

func sanitizeTaskEvents(events []TaskEvent) {
	for index := range events {
		event := &events[index]
		event.StatusUpdate.Brief = ""
		event.StatusUpdate.Description = ""
		event.StatusUpdate.StatusDetail.WaitingForEventID = ""
		event.StatusUpdate.StatusDetail.ConfirmInputSchema = nil
		if event.StructuredOutputResult != nil && !event.StructuredOutputResult.Success {
			event.StructuredOutputResult.Value = nil
		}
	}
}

// WaitForTask performs bounded polling and normalizes waiting states.
func (r *Runtime) WaitForTask(ctx context.Context, taskID string, requested time.Duration) (TaskState, error) {
	if _, err := r.requireTracked(ctx, taskID); err != nil {
		return TaskState{}, err
	}
	if requested <= 0 || requested > r.maxWait {
		requested = r.maxWait
	}
	deadline := time.Now().Add(requested)
	for {
		task, err := r.GetTask(ctx, taskID)
		if err != nil {
			return TaskState{}, err
		}
		switch task.Status {
		case "waiting":
			page, err := r.ListMessages(ctx, ListMessagesOptions{TaskID: taskID, Limit: 50, Order: "desc"})
			if err != nil {
				return TaskState{}, err
			}
			return normalizeTaskState(task, page.Messages), nil
		case "completed", "success", "finished":
			return TaskState{State: "completed", Task: task, TaskURL: task.TaskURL}, nil
		case "stopped":
			page, err := r.ListMessages(ctx, ListMessagesOptions{TaskID: taskID, Limit: 50, Order: "desc"})
			if err != nil {
				return TaskState{}, err
			}
			state := "completed"
			for _, event := range page.Messages {
				if event.Type == "user_stop" {
					state = "stopped"
					break
				}
			}
			return TaskState{State: state, Task: task, TaskURL: task.TaskURL, Messages: page.Messages}, nil
		case "error", "failed":
			page, err := r.ListMessages(ctx, ListMessagesOptions{TaskID: taskID, Limit: 50, Order: "desc"})
			if err != nil {
				return TaskState{}, err
			}
			return TaskState{State: "error", Task: task, TaskURL: task.TaskURL, Messages: page.Messages}, nil
		}
		if time.Now().Add(r.pollInterval).After(deadline) {
			return TaskState{State: "running", Task: task, TaskURL: task.TaskURL, PollAfterSeconds: max(1, int(r.pollInterval.Seconds()))}, nil
		}
		timer := time.NewTimer(r.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return TaskState{}, fmt.Errorf("wait for Manus task: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// SendMessage continues a tracked task after local authorization.
func (r *Runtime) SendMessage(ctx context.Context, request SendMessageRequest, localPaths []string) (SendMessageResult, error) {
	if err := r.policy.Authorize("send_message"); err != nil {
		return SendMessageResult{}, err
	}
	record, err := r.requireTracked(ctx, request.TaskID)
	if err != nil {
		return SendMessageResult{}, err
	}
	if err := r.policy.ValidateResources("", request.Connectors, request.EnableSkills, request.ForceSkills); err != nil {
		return SendMessageResult{}, err
	}
	if request.AgentProfile != "" {
		if err := ValidateAgentProfile(request.AgentProfile); err != nil {
			return SendMessageResult{}, err
		}
	}
	if err := ValidateStructuredOutputSchema(request.StructuredOutputSchema); err != nil {
		return SendMessageResult{}, err
	}
	if err := r.ledger.PreflightWrite(ctx); err != nil {
		return SendMessageResult{}, fmt.Errorf("preflight Manus task persistence: %w", err)
	}
	content, err := r.contentWithUploads(ctx, request.Content, localPaths)
	if err != nil {
		return SendMessageResult{}, err
	}
	request.Content = content
	result, err := r.client.SendMessage(ctx, request)
	if err != nil {
		return SendMessageResult{}, err
	}
	record.Status = "running"
	record.UpdatedAt = time.Now().UTC()
	if request.AgentProfile != "" {
		record.AgentProfile = request.AgentProfile
	}
	if err := r.persistMutation(ctx, record); err != nil {
		return result, &RemoteAppliedError{Operation: "send_message", TaskID: record.TaskID, TaskURL: record.TaskURL, Err: err}
	}
	return result, nil
}

func (r *Runtime) contentWithUploads(ctx context.Context, content any, localPaths []string) (any, error) {
	if len(localPaths) == 0 {
		return content, nil
	}
	if err := r.policy.AuthorizeUpload(); err != nil {
		return nil, err
	}
	text, ok := content.(string)
	if !ok {
		return nil, fmt.Errorf("Manus file uploads require text message content")
	}
	parts := make([]ContentPart, 0, len(localPaths)+1)
	if strings.TrimSpace(text) != "" {
		parts = append(parts, ContentPart{Type: "text", Text: text})
	}
	for _, path := range localPaths {
		local, err := ResolveUploadPath(r.workspaceDir, path, r.maxFileBytes)
		if err != nil {
			return nil, err
		}
		file, err := r.client.UploadLocalFile(ctx, local)
		if err != nil {
			return nil, err
		}
		parts = append(parts, ContentPart{Type: "file", FileID: file.ID, Filename: file.Filename})
	}
	return parts, nil
}

// StopTask stops a tracked task after local authorization.
func (r *Runtime) StopTask(ctx context.Context, taskID string) error {
	if err := r.policy.Authorize("stop_task"); err != nil {
		return err
	}
	record, err := r.requireTracked(ctx, taskID)
	if err != nil {
		return err
	}
	if err := r.ledger.PreflightWrite(ctx); err != nil {
		return fmt.Errorf("preflight Manus task persistence: %w", err)
	}
	if err := r.client.StopTask(ctx, taskID); err != nil {
		return err
	}
	record.Status = "stopped"
	record.UpdatedAt = time.Now().UTC()
	if err := r.persistMutation(ctx, record); err != nil {
		return &RemoteAppliedError{Operation: "stop_task", TaskID: record.TaskID, TaskURL: record.TaskURL, Err: err}
	}
	return nil
}

func (r *Runtime) persistMutation(ctx context.Context, record TaskRecord) error {
	const attempts = 3
	for attempt := 0; attempt < attempts; attempt++ {
		err := r.ledger.Upsert(ctx, record)
		if err == nil {
			return nil
		}
		if !isSQLiteBusy(err) || attempt+1 == attempts {
			return err
		}
		delay := 50 * time.Millisecond * time.Duration(1<<attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("retry Manus ledger persistence: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("persist Manus task metadata: retry attempts exhausted")
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToUpper(err.Error())
	return strings.Contains(message, "SQLITE_BUSY") || strings.Contains(message, "SQLITE_LOCKED") || strings.Contains(message, "DATABASE IS LOCKED")
}

// ListTrackedTasks returns only the local AuraGo task inventory.
func (r *Runtime) ListTrackedTasks(ctx context.Context, limit int) ([]TaskRecord, error) {
	return r.ledger.List(ctx, limit)
}

// DownloadAttachments downloads attachments from one assistant event of a tracked task.
func (r *Runtime) DownloadAttachments(ctx context.Context, taskID, eventID string) ([]string, error) {
	if err := r.policy.Authorize("download_attachments"); err != nil {
		return nil, err
	}
	if _, err := r.requireTracked(ctx, taskID); err != nil {
		return nil, err
	}
	page, err := r.ListMessages(ctx, ListMessagesOptions{TaskID: taskID, Limit: 200, Order: "desc"})
	if err != nil {
		return nil, err
	}
	var attachments []TaskAttachment
	for _, event := range page.Messages {
		if event.Type != "assistant_message" || (eventID != "" && event.ID != eventID) {
			continue
		}
		if len(event.AssistantMessage.Attachments) > 0 {
			attachments = event.AssistantMessage.Attachments
			break
		}
	}
	if len(attachments) == 0 {
		return nil, fmt.Errorf("no downloadable Manus attachments found for the selected event")
	}
	paths := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		payload, err := r.client.DownloadAttachment(ctx, attachment, r.maxFileBytes)
		if err != nil {
			return nil, err
		}
		path, err := r.writeAttachment(taskID, attachment.Filename, payload)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (r *Runtime) writeAttachment(taskID, filename string, payload []byte) (string, error) {
	workspaceAbs, err := filepath.Abs(strings.TrimSpace(r.workspaceDir))
	if err != nil || strings.TrimSpace(r.workspaceDir) == "" {
		return "", fmt.Errorf("resolve Manus workspace root")
	}
	workspaceInfo, err := os.Lstat(workspaceAbs)
	if err != nil {
		return "", fmt.Errorf("inspect Manus workspace root: %w", err)
	}
	if workspaceInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Manus download blocks a symlinked workspace root")
	}
	downloadRoot := strings.TrimSpace(r.downloadRoot)
	if downloadRoot == "" {
		downloadRoot = filepath.Join(workspaceAbs, "workdir", "manus")
	}
	downloadAbs, err := filepath.Abs(downloadRoot)
	if err != nil || !pathWithin(workspaceAbs, downloadAbs) {
		return "", fmt.Errorf("Manus download root leaves the agent workspace")
	}
	downloadRelative, err := filepath.Rel(workspaceAbs, downloadAbs)
	if err != nil {
		return "", fmt.Errorf("resolve Manus download root: %w", err)
	}
	root, err := os.OpenRoot(workspaceAbs)
	if err != nil {
		return "", fmt.Errorf("open Manus workspace root: %w", err)
	}
	defer root.Close()
	if err := root.MkdirAll(downloadRelative, 0o750); err != nil {
		return "", fmt.Errorf("create Manus download root: %w", err)
	}
	if info, err := root.Lstat(downloadRelative); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Manus download root must be a real workspace directory")
	}
	taskDir := filepath.Join(downloadRelative, SafeAttachmentFilename(taskID))
	if err := root.MkdirAll(taskDir, 0o750); err != nil {
		return "", fmt.Errorf("create Manus task download directory: %w", err)
	}
	if info, err := root.Lstat(taskDir); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Manus task download directory must not be a symlink")
	}
	relativePath, file, err := createUniqueAttachment(root, taskDir, SafeAttachmentFilename(filename))
	if err != nil {
		return "", err
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = root.Remove(relativePath)
		return "", fmt.Errorf("write Manus attachment: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = root.Remove(relativePath)
		return "", fmt.Errorf("close Manus attachment: %w", err)
	}
	return filepath.Join(workspaceAbs, relativePath), nil
}

func (r *Runtime) requireTracked(ctx context.Context, taskID string) (TaskRecord, error) {
	record, ok, err := r.ledger.Get(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return TaskRecord{}, err
	}
	if !ok {
		return TaskRecord{}, fmt.Errorf("Manus task %q is not tracked by AuraGo", taskID)
	}
	return record, nil
}

func (r *Runtime) updateRecord(ctx context.Context, record TaskRecord, task Task, cursor string) error {
	record.Title = task.Title
	record.Status = task.Status
	record.AgentProfile = task.AgentProfile
	record.CreditUsage = task.CreditUsage
	record.LastCursor = cursor
	if task.TaskURL != "" {
		record.TaskURL = task.TaskURL
	}
	record.UpdatedAt = time.Now().UTC()
	return r.ledger.Upsert(ctx, record)
}

func normalizeTaskState(task Task, messages []TaskEvent) TaskState {
	state := TaskState{State: "needs_human_approval", Task: task, TaskURL: task.TaskURL, Messages: messages}
	for _, event := range messages {
		if event.Type != "status_update" || event.StatusUpdate.AgentStatus != "waiting" {
			continue
		}
		detail := event.StatusUpdate.StatusDetail
		state.WaitingType = detail.WaitingForEventType
		state.WaitingDescription = detail.WaitingDescription
		if detail.WaitingForEventType == "messageAskUser" {
			state.State = "needs_user_input"
		}
		break
	}
	return state
}

// ValidateAgentProfile restricts tasks to the documented Manus v2 profiles.
func ValidateAgentProfile(profile string) error {
	switch strings.TrimSpace(profile) {
	case "manus-1.6", "manus-1.6-lite", "manus-1.6-max":
		return nil
	default:
		return fmt.Errorf("unsupported Manus agent profile %q", profile)
	}
}

// DefaultLedgerPath returns the standard data/manus.db location.
func DefaultLedgerPath(dataDir string) string {
	return filepath.Join(dataDir, "manus.db")
}
