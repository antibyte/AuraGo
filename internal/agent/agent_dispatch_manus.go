package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/manus"
	"aurago/internal/security"
)

func dispatchManusCall(ctx context.Context, req manusCallArgs, cfg *config.Config) string {
	if cfg == nil || !cfg.Manus.Enabled {
		return manusErrorOutput("Manus is disabled. Enable manus.enabled and configure the API key in the vault.")
	}
	if strings.TrimSpace(cfg.Manus.APIKey) == "" {
		return manusErrorOutput("Manus API key is not configured in the vault.")
	}
	client, err := manus.NewClient(cfg.Manus.APIKey, manus.ClientConfig{
		Timeout:        time.Duration(cfg.Manus.RequestTimeoutSeconds) * time.Second,
		MaxResultBytes: int64(cfg.Manus.MaxResultBytes),
	})
	if err != nil {
		return manusErrorOutput(err.Error())
	}
	return dispatchManusCallWithClient(ctx, req, cfg, client)
}

func dispatchManusCallWithClient(ctx context.Context, req manusCallArgs, cfg *config.Config, client *manus.Client) string {
	if cfg == nil || !cfg.Manus.Enabled {
		return manusErrorOutput("Manus is disabled.")
	}
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	policy := manusPolicyFromConfig(cfg.Manus)
	if op == "capabilities" {
		return manusJSONOutput(map[string]interface{}{
			"status":                 "success",
			"operation":              op,
			"read_only":              policy.ReadOnly,
			"allow_create_tasks":     policy.AllowCreateTasks,
			"allow_send_messages":    policy.AllowSendMessages,
			"allow_stop_tasks":       policy.AllowStopTasks,
			"allow_file_uploads":     policy.AllowFileUploads,
			"allow_file_downloads":   policy.AllowFileDownloads,
			"allowed_project_ids":    policy.AllowedProjectIDs,
			"allowed_connector_ids":  policy.AllowedConnectorIDs,
			"allowed_skill_ids":      policy.AllowedSkillIDs,
			"confirm_action_exposed": false,
			"recommended_workflow":   []string{"create_task", "wait_for_task", "list_messages", "send_message"},
		})
	}
	if client == nil {
		return manusErrorOutput("Manus client is unavailable.")
	}
	if req.SchemaError != "" {
		return manusErrorOutput("structured_output_schema is not valid JSON: " + req.SchemaError)
	}

	ledger, err := manus.OpenLedger(manus.DefaultLedgerPath(cfg.Directories.DataDir))
	if err != nil {
		return manusErrorOutput(err.Error())
	}
	defer ledger.Close()
	runtime := manus.NewRuntime(client, ledger, manus.RuntimeConfig{
		Policy:       policy,
		WorkspaceDir: cfg.Directories.WorkspaceDir,
		DownloadRoot: filepath.Join(cfg.Directories.WorkspaceDir, "workdir", "manus"),
		MaxFileBytes: int64(cfg.Manus.MaxFileSizeMB) * 1024 * 1024,
		PollInterval: time.Duration(cfg.Manus.PollIntervalSeconds) * time.Second,
		MaxWait:      time.Duration(cfg.Manus.MaxWaitSeconds) * time.Second,
	})

	switch op {
	case "get_credits":
		credits, err := client.AvailableCredits(ctx)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "credits": credits.Data})
	case "list_projects":
		items, err := client.ListProjects(ctx)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "projects": annotateManusProjects(items, policy.AllowedProjectIDs)})
	case "list_connectors":
		items, err := client.ListConnectors(ctx)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "connectors": annotateManusConnectors(items, policy.AllowedConnectorIDs)})
	case "list_skills":
		if req.ProjectID != "" && !manusIDAllowed(policy.AllowedProjectIDs, req.ProjectID) {
			return manusErrorOutput(fmt.Sprintf("Manus project %q is not allowlisted.", req.ProjectID))
		}
		items, err := client.ListSkills(ctx, req.ProjectID)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "skills": annotateManusSkills(items, policy.AllowedSkillIDs)})
	case "create_task":
		if strings.TrimSpace(req.Message) == "" {
			return manusErrorOutput("message is required for create_task")
		}
		profile := strings.TrimSpace(req.AgentProfile)
		if profile == "" {
			profile = cfg.Manus.DefaultAgentProfile
		}
		locale := strings.TrimSpace(req.Locale)
		if locale == "" {
			locale = cfg.Manus.DefaultLocale
		}
		result, err := runtime.CreateTask(ctx, manus.CreateTaskRequest{
			Content: req.Message, ProjectID: req.ProjectID, Locale: locale,
			InteractiveMode: req.InteractiveMode, AgentProfile: profile, Title: req.Title,
			Connectors: req.ConnectorIDs, EnableSkills: req.EnableSkillIDs, ForceSkills: req.ForceSkillIDs,
			StructuredOutputSchema: req.StructuredOutputSchema,
		}, req.LocalFilePaths)
		if err != nil {
			return manusOperationErrorOutput(op, err, map[string]interface{}{"task": result})
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "task": result})
	case "list_tracked_tasks":
		items, err := runtime.ListTrackedTasks(ctx, req.Limit)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "tasks": items})
	case "get_task":
		task, err := runtime.GetTask(ctx, req.TaskID)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "task": task})
	case "list_messages":
		page, err := runtime.ListMessages(ctx, manus.ListMessagesOptions{TaskID: req.TaskID, Cursor: req.Cursor, Limit: req.Limit})
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "page": page})
	case "wait_for_task":
		state, err := runtime.WaitForTask(ctx, req.TaskID, time.Duration(req.WaitSeconds)*time.Second)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "task_state": state})
	case "send_message":
		if strings.TrimSpace(req.Message) == "" {
			return manusErrorOutput("message is required for send_message")
		}
		result, err := runtime.SendMessage(ctx, manus.SendMessageRequest{
			TaskID: req.TaskID, Content: req.Message, Connectors: req.ConnectorIDs,
			EnableSkills: req.EnableSkillIDs, ForceSkills: req.ForceSkillIDs,
			AgentProfile: req.AgentProfile, StructuredOutputSchema: req.StructuredOutputSchema,
		}, req.LocalFilePaths)
		if err != nil {
			return manusOperationErrorOutput(op, err, map[string]interface{}{"task": result})
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "task": result})
	case "stop_task":
		if err := runtime.StopTask(ctx, req.TaskID); err != nil {
			return manusOperationErrorOutput(op, err, map[string]interface{}{"task_id": req.TaskID})
		}
		return manusJSONOutput(map[string]interface{}{"status": "success", "operation": op, "task_id": req.TaskID})
	case "download_attachments":
		paths, err := runtime.DownloadAttachments(ctx, req.TaskID, req.EventID)
		if err != nil {
			return manusErrorOutput(err.Error())
		}
		return manusExternalOutput(map[string]interface{}{"status": "success", "operation": op, "paths": paths})
	default:
		return manusErrorOutput(fmt.Sprintf("unknown Manus operation %q", op))
	}
}

func manusPolicyFromConfig(cfg config.ManusConfig) manus.Policy {
	return manus.Policy{
		ReadOnly: cfg.ReadOnly, AllowCreateTasks: cfg.AllowCreateTasks,
		AllowSendMessages: cfg.AllowSendMessages, AllowStopTasks: cfg.AllowStopTasks,
		AllowFileUploads: cfg.AllowFileUploads, AllowFileDownloads: cfg.AllowFileDownloads,
		AllowedProjectIDs: cfg.AllowedProjectIDs, AllowedConnectorIDs: cfg.AllowedConnectorIDs,
		AllowedSkillIDs: cfg.AllowedSkillIDs,
	}
}

func annotateManusProjects(items []manus.Project, allowed []string) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{"id": item.ID, "name": item.Name, "instruction": item.Instruction, "allowed": manusIDAllowed(allowed, item.ID)})
	}
	return result
}

func annotateManusConnectors(items []manus.Connector, allowed []string) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{"id": item.ID, "name": item.Name, "type": item.Type, "description": item.Description, "category": item.Category, "allowed": manusIDAllowed(allowed, item.ID)})
	}
	return result
}

func annotateManusSkills(items []manus.Skill, allowed []string) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{"id": item.ID, "name": item.Name, "description": item.Description, "owner_type": item.OwnerType, "allowed": manusIDAllowed(allowed, item.ID)})
	}
	return result
}

func manusIDAllowed(allowed []string, id string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == strings.TrimSpace(id) && strings.TrimSpace(id) != "" {
			return true
		}
	}
	return false
}

func manusJSONOutput(payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return "Tool Output: " + string(raw)
}

func manusExternalOutput(payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return "Tool Output: " + security.IsolateExternalData(security.Scrub(string(raw)))
}

func manusOperationErrorOutput(operation string, err error, payload map[string]interface{}) string {
	var applied *manus.RemoteAppliedError
	if errors.As(err, &applied) {
		result := make(map[string]interface{}, len(payload)+8)
		for key, value := range payload {
			result[key] = value
		}
		result["status"] = "partial_success"
		result["operation"] = operation
		result["remote_applied"] = true
		result["retry_safe"] = false
		result["task_id"] = applied.TaskID
		if applied.TaskURL != "" {
			result["task_url"] = applied.TaskURL
		}
		result["message"] = security.Scrub(applied.Error())
		return manusExternalOutput(result)
	}
	var unknown *manus.OutcomeUnknownError
	if errors.As(err, &unknown) {
		return manusExternalOutput(map[string]interface{}{
			"status": "error", "operation": operation, "outcome": "unknown",
			"retry_safe": false, "message": security.Scrub(unknown.Error()),
		})
	}
	return manusErrorOutput(err.Error())
}

func manusErrorOutput(message string) string {
	return manusExternalOutput(map[string]interface{}{"status": "error", "message": security.Scrub(message)})
}
