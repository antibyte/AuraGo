package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/security"
	"aurago/internal/storymaker"
)

const (
	storymakerChatMaxRows   = 20
	storymakerChatMaxChecks = 8
)

// storymakerChatArgs is a decoded storymaker chat tool call.
type storymakerChatArgs struct {
	Operation     string
	Topic         string
	Audience      string
	LengthSeconds int
	Format        string
	Language      string
	Voice         *storymaker.Voice
	ProjectID     string
	Instruction   string
	Silent        bool
}

// decodeStorymakerArgs reads params only: ToolCall.UnmarshalJSON copies every top-level key of a
// text-mode call into Params as well.
func decodeStorymakerArgs(tc ToolCall) storymakerChatArgs {
	args := storymakerChatArgs{
		Operation:     strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")))),
		Topic:         toolArgString(tc.Params, "topic"),
		Audience:      toolArgString(tc.Params, "audience"),
		LengthSeconds: toolArgInt(tc.Params, 0, "length_seconds"),
		Format:        strings.TrimSpace(toolArgString(tc.Params, "format", "aspect")),
		Language:      strings.TrimSpace(toolArgString(tc.Params, "language")),
		ProjectID:     strings.TrimSpace(toolArgString(tc.Params, "project_id")),
		Instruction:   toolArgString(tc.Params, "instruction"),
	}
	args.Silent, _ = toolArgBool(tc.Params, "silent")
	if voice := toolArgInterfaceMap(tc.Params, "voice"); voice != nil {
		if id := strings.TrimSpace(toolArgString(voice, "voice")); id != "" {
			args.Voice = &storymaker.Voice{Provider: strings.TrimSpace(toolArgString(voice, "provider")), Voice: id, Language: args.Language}
		}
	}
	return args
}

// storymakerChatToolAvailable is the runtime gate of the schema.
func storymakerChatToolAvailable() bool {
	facade := storymaker.CurrentAgentFacade()
	return facade != nil && facade.Available()
}

// storymakerToolAnswer encodes without HTML escaping, so the isolation tags stay readable.
func storymakerToolAnswer(fields map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(fields); err != nil {
		return `Tool Output: {"status":"error","code":"storymaker_failed","message":"result could not be encoded"}`
	}
	return "Tool Output: " + strings.TrimRight(buf.String(), "\n")
}

func storymakerToolRefusal(status, code, message, next string) string {
	return storymakerToolAnswer(map[string]any{"status": status, "code": code, "message": message, "next_action": next})
}

func storymakerToolFailure(err error) string {
	var refused *storymaker.AgentError
	if errors.As(err, &refused) {
		return storymakerToolRefusal("error", refused.Code, refused.Message, refused.NextAction)
	}
	return storymakerToolRefusal("error", "storymaker_failed", "Storymaker could not finish this call.", "Try once more; if it fails again, tell the user.")
}

// dispatchStorymaker runs the chat tool. The topic and the instruction are never logged.
func dispatchStorymaker(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	args := decodeStorymakerArgs(tc)
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested Storymaker", "operation", args.Operation)
	}
	facade := storymaker.CurrentAgentFacade()
	if facade == nil || !facade.Available() {
		return storymakerToolRefusal("error", "agent_disabled", "The Storymaker agent is switched off or not available.", "Tell the user to enable the Storymaker agent in the settings.")
	}
	switch args.Operation {
	case "create":
		return storymakerToolCreate(ctx, facade, args)
	case "list":
		return storymakerToolList(ctx, facade)
	case "status", "revise", "render", "open":
		if !storymaker.ValidProjectID(args.ProjectID) {
			return storymakerToolRefusal("error", "project_not_found", "project_id is not a known project.", "Call list and use a returned project_id.")
		}
	default:
		return storymakerToolRefusal("error", "invalid_operation", "operation must be create, status, revise, render, open or list.", "Send one of these operations.")
	}
	switch args.Operation {
	case "status":
		return storymakerToolStatus(ctx, facade, args.ProjectID)
	case "revise":
		return storymakerToolRevise(ctx, facade, args)
	case "render":
		return storymakerToolRender(ctx, facade, args)
	}
	return storymakerToolOpen(ctx, facade, args.ProjectID, dc)
}

func storymakerToolCreate(ctx context.Context, facade storymaker.AgentFacade, args storymakerChatArgs) string {
	ref, err := facade.Create(ctx, storymaker.AgentCreateRequest{
		Topic:         args.Topic,
		Audience:      args.Audience,
		Aspect:        args.Format,
		Language:      args.Language,
		LengthSeconds: args.LengthSeconds,
		Voice:         args.Voice,
	})
	if err != nil {
		return storymakerToolFailure(err)
	}
	return storymakerToolAnswer(map[string]any{
		"status": "success", "project_id": ref.ProjectID, "job_id": ref.JobID, "state": ref.State, "eta_hint_seconds": ref.ETAHintSeconds,
	})
}

func storymakerToolStatus(ctx context.Context, facade storymaker.AgentFacade, projectID string) string {
	status, err := facade.Status(ctx, projectID)
	if err != nil {
		return storymakerToolFailure(err)
	}
	fields := map[string]any{
		"status": "success", "project_id": projectID, "state": status.State,
		"scenes_done": status.ScenesDone, "scenes_total": status.ScenesTotal, "next_action": status.NextAction,
	}
	if status.Phase != "" {
		fields["phase"] = status.Phase
	}
	if len(status.OpenChecks) > 0 {
		checks := status.OpenChecks
		if len(checks) > storymakerChatMaxChecks {
			checks = checks[:storymakerChatMaxChecks]
		}
		fields["open_checks"] = checks
	}
	if status.ErrorCode != "" {
		fields["error_code"] = status.ErrorCode
	}
	if status.PollAfterSeconds > 0 {
		fields["poll_after_seconds"] = status.PollAfterSeconds
	}
	if status.OutputPath != "" {
		fields["output_path"] = status.OutputPath
	}
	if len(status.ChangedScenes) > 0 {
		fields["changed_scenes"] = status.ChangedScenes
	}
	if len(status.UnchangedScenes) > 0 {
		fields["unchanged_scenes"] = status.UnchangedScenes
	}
	if len(status.RemovedScenes) > 0 {
		fields["removed_scenes"] = status.RemovedScenes
	}
	return storymakerToolAnswer(fields)
}

func storymakerToolRevise(ctx context.Context, facade storymaker.AgentFacade, args storymakerChatArgs) string {
	ref, err := facade.Revise(ctx, args.ProjectID, args.Instruction)
	if err != nil {
		return storymakerToolFailure(err)
	}
	return storymakerToolAnswer(map[string]any{"status": "success", "project_id": args.ProjectID, "job_id": ref.JobID, "state": ref.State})
}

func storymakerToolRender(ctx context.Context, facade storymaker.AgentFacade, args storymakerChatArgs) string {
	if !args.Silent {
		id, err := facade.Render(ctx, args.ProjectID)
		if err != nil {
			return storymakerToolFailure(err)
		}
		return storymakerToolAnswer(map[string]any{"status": "success", "render_job_id": id})
	}
	id, changed, err := facade.RenderSilent(ctx, args.ProjectID)
	if err != nil {
		return storymakerToolFailure(err)
	}
	return storymakerToolAnswer(map[string]any{
		"status": "success", "render_job_id": id, "speech_removed": changed > 0, "scenes_changed": changed,
		"note": "Speech was removed from the changed scenes for this export. Tell the user; the Storymaker app can restore it with Restore speech.",
	})
}

func storymakerToolList(ctx context.Context, facade storymaker.AgentFacade) string {
	rows, err := facade.List(ctx)
	if err != nil {
		return storymakerToolFailure(err)
	}
	if len(rows) > storymakerChatMaxRows {
		rows = rows[:storymakerChatMaxRows]
	}
	projects := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, map[string]any{
			"project_id": row.ProjectID, "title": security.IsolateExternalData(row.Title), "updated_at": row.UpdatedAt,
			"state": row.State, "duration_seconds": row.DurationSeconds,
		})
	}
	return storymakerToolAnswer(map[string]any{"status": "success", "projects": projects})
}

// storymakerToolOpen asks the desktop to open the project, with the same gate and event as retro_games.
func storymakerToolOpen(ctx context.Context, facade storymaker.AgentFacade, projectID string, dc *DispatchContext) string {
	cfg := dc.Cfg
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		return storymakerToolRefusal("error", "desktop_unavailable", "The virtual desktop is switched off, so nothing can be opened.", "Tell the user.")
	}
	if !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled {
		return storymakerToolRefusal("policy_denied", "desktop_agent_control_disabled", "The user has not allowed the agent to control the desktop.", "Tell the user the project can be opened in the Storymaker app.")
	}
	if _, err := facade.Status(ctx, projectID); err != nil {
		return storymakerToolFailure(err)
	}
	if dc.Broker == nil {
		return storymakerToolRefusal("error", "desktop_unavailable", "This run has no desktop to open the project on.", "Tell the user the project can be opened in the Storymaker app.")
	}
	event := &desktop.Event{
		ID:        rand.Text(),
		Type:      "open_app",
		Payload:   map[string]any{"app_id": "storymaker", "project_id": projectID},
		CreatedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(struct {
		Type    string         `json:"type"`
		Payload *desktop.Event `json:"payload"`
	}{"virtual_desktop_event", event})
	if err != nil {
		return storymakerToolFailure(err)
	}
	dc.Broker.SendJSON(string(payload))
	return storymakerToolAnswer(map[string]any{"status": "success", "project_id": projectID, "opened": true})
}
