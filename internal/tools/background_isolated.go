package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/security"
)

const BackgroundTaskTypeIsolatedCode = "isolated_code"

// BackgroundCodeAction retains only the exact canonical call and its original
// role/scope. Approval leases and other execution credentials are never copied.
type BackgroundCodeAction struct {
	Call                 json.RawMessage `json:"call"`
	TransportAction      string          `json:"transport_action,omitempty"`
	SessionID            string          `json:"session_id"`
	MissionID            string          `json:"mission_id,omitempty"`
	FlowID               string          `json:"flow_id,omitempty"`
	MessageSource        string          `json:"message_source,omitempty"`
	IsMaintenance        bool            `json:"is_maintenance,omitempty"`
	IsCoAgent            bool            `json:"is_coagent,omitempty"`
	ParentIsMaintenance  bool            `json:"parent_is_maintenance,omitempty"`
	CoAgentSpecialist    string          `json:"coagent_specialist,omitempty"`
	AllowedTools         []string        `json:"allowed_tools,omitempty"`
	ToolScopeRestricted  bool            `json:"tool_scope_restricted,omitempty"`
	AllowedAgentSkills   []string        `json:"allowed_agent_skills,omitempty"`
	SkillScopeRestricted bool            `json:"skill_scope_restricted,omitempty"`
}

type backgroundCodeActionKey struct{}

// WithBackgroundCodeAction snapshots server-resolved metadata for durable code
// admission; caller mutations cannot change the queued action.
func WithBackgroundCodeAction(ctx context.Context, action BackgroundCodeAction) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	action.Call = append(json.RawMessage(nil), action.Call...)
	action.AllowedTools = append([]string(nil), action.AllowedTools...)
	action.AllowedAgentSkills = append([]string(nil), action.AllowedAgentSkills...)
	return context.WithValue(ctx, backgroundCodeActionKey{}, action)
}

func validBackgroundCodeAction(action *BackgroundCodeAction) bool {
	if action == nil || action.SessionID == "" || len(action.Call) == 0 || len(action.Call) > 1<<20 {
		return false
	}
	var call struct {
		Action string `json:"action"`
	}
	if json.Unmarshal(action.Call, &call) != nil || call.Action == "" {
		return false
	}
	metadata := *action
	metadata.Call = nil
	encoded, err := json.Marshal(metadata)
	return err == nil && len(encoded) <= 64<<10
}

type isolatedCodePayload struct {
	Code          string                `json:"code"`
	Language      string                `json:"language"`
	ExecutionKind string                `json:"execution_kind"`
	Permissions   RuntimePermissions    `json:"permissions"`
	Action        *BackgroundCodeAction `json:"action,omitempty"`
}

// SetIsolatedCodeAuthorizer installs the shared canonical policy gate. It must
// check current policy without creating or replaying an approval.
func (m *BackgroundTaskManager) SetIsolatedCodeAuthorizer(fn func(context.Context, BackgroundCodeAction) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isolatedAuthorizer = fn
}

func (m *BackgroundTaskManager) SetIsolatedCodeExecutor(fn func(context.Context, string, string, time.Duration) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isolatedExecutor = fn
}

func (m *BackgroundTaskManager) ScheduleIsolatedCodeContext(ctx context.Context, code, language string, opts BackgroundTaskScheduleOptions) (*BackgroundTask, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, fmt.Errorf("background owner is unavailable")
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("background code is required")
	}
	kind := opts.ExecutionKind
	if kind == "" {
		kind = language
	}
	if kind != "shell" && kind != "python" {
		return nil, fmt.Errorf("unsupported background execution kind")
	}
	perms, ok := EffectiveRuntimePermissions(ctx)
	if !ok || (kind == "shell" && !perms.AllowShell) || (kind == "python" && !perms.AllowPython) {
		return nil, fmt.Errorf("background execution is disabled")
	}
	m.mu.Lock()
	ready := m.isolatedExecutor != nil
	m.mu.Unlock()
	if !ready {
		return nil, fmt.Errorf("isolated background executor is unavailable")
	}
	var action *BackgroundCodeAction
	if origin, found := ctx.Value(backgroundCodeActionKey{}).(BackgroundCodeAction); found {
		if !validBackgroundCodeAction(&origin) {
			return nil, fmt.Errorf("background action identity is invalid or exceeds its bounded snapshot")
		}
		action = &origin
	}
	payload, err := json.Marshal(isolatedCodePayload{Code: code, Language: language, ExecutionKind: kind, Permissions: perms, Action: action})
	if err != nil {
		return nil, err
	}
	task := m.newTask(BackgroundTaskTypeIsolatedCode, payload, opts)
	return m.enqueue(task)
}

func (m *BackgroundTaskManager) executeIsolatedCode(task *BackgroundTask) {
	var payload isolatedCodePayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		m.failTask(task.ID, "invalid isolated background payload", false)
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.mu.Lock()
	current, ok := m.tasks[task.ID]
	if !ok || current.Status != BackgroundTaskStatusRunning {
		m.mu.Unlock()
		cancel()
		return
	}
	if m.taskCancels == nil {
		m.taskCancels = make(map[string]context.CancelFunc)
	}
	m.taskCancels[task.ID] = cancel
	runner := m.isolatedExecutor
	authorize := m.isolatedAuthorizer
	m.mu.Unlock()
	defer func() { cancel(); m.mu.Lock(); delete(m.taskCancels, task.ID); m.mu.Unlock() }()
	ctx = WithRuntimePermissions(ctx, payload.Permissions)
	perms, ok := EffectiveRuntimePermissions(ctx)
	if !ok || (payload.ExecutionKind == "shell" && !perms.AllowShell) || (payload.ExecutionKind == "python" && !perms.AllowPython) {
		m.failTask(task.ID, "background execution permission revoked", false)
		return
	}
	if runner == nil {
		m.failTask(task.ID, "isolated background executor unavailable", false)
		return
	}
	if authorize == nil || !validBackgroundCodeAction(payload.Action) {
		m.finishTask(task.ID, BackgroundTaskStatusBlocked, "", "action_rule_required: background action identity or policy gate unavailable")
		return
	}
	if err := authorize(ctx, *payload.Action); err != nil {
		reason := []rune(security.RedactSensitiveInfo(security.Scrub(err.Error())))
		if len(reason) > 1024 {
			reason = reason[:1024]
		}
		m.finishTask(task.ID, BackgroundTaskStatusBlocked, "", "action_rule_required: "+string(reason))
		return
	}
	if ctx.Err() != nil {
		m.finishTask(task.ID, BackgroundTaskStatusCanceled, "", "background execution canceled before start")
		return
	}
	if _, err := m.promptExecutionID(task.ID); err != nil {
		m.failTask(task.ID, "persist isolated execution identity failed", false)
		return
	}
	if ctx.Err() != nil {
		m.finishTask(task.ID, BackgroundTaskStatusCanceled, "", "background execution canceled before start")
		return
	}
	output, err := func() (out string, err error) {
		defer func() {
			if recover() != nil {
				err = fmt.Errorf("background execution outcome unknown; verify effects before retrying")
			}
		}()
		return runner(ctx, payload.Code, payload.Language, time.Duration(task.TimeoutSeconds)*time.Second)
	}()
	if err != nil {
		m.failTask(task.ID, err.Error(), false)
		return
	}
	m.completeTask(task.ID, output)
}
