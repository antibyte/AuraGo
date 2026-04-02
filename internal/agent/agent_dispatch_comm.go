package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/inventory"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/telnyx"
	"aurago/internal/tools"
)

type emailContentEvaluator interface {
	EvaluateContent(ctx context.Context, contentType string, content string) security.GuardianResult
}

const emailGuardianWorkerLimit = 4

func sanitizeFetchedEmails(ctx context.Context, logger *slog.Logger, guardian *security.Guardian, llmGuardian emailContentEvaluator, scanEmails bool, messages []tools.EmailMessage) []tools.EmailMessage {
	if guardian == nil || len(messages) == 0 {
		return messages
	}

	sanitized := make([]tools.EmailMessage, len(messages))
	workerCount := emailGuardianWorkerLimit
	if len(messages) < workerCount {
		workerCount = len(messages)
	}

	indexCh := make(chan int, len(messages))
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				msg := messages[idx]
				combined := msg.From + " " + msg.Subject + " " + msg.Body
				scanRes := guardian.ScanForInjection(combined)
				if scanRes.Level >= security.ThreatHigh {
					if logger != nil {
						logger.Warn("[Email] Guardian HIGH threat in message", "uid", msg.UID, "from", msg.From, "threat", scanRes.Level.String())
					}
					msg.Body = security.RedactedText("guardian blocked content after injection detection")
					msg.Subject = security.SanitizedText("guardian scan flagged this message")
					msg.Snippet = security.RedactedText("")
					sanitized[idx] = msg
					continue
				}

				if llmGuardian != nil && scanEmails {
					llmResult := llmGuardian.EvaluateContent(ctx, "email", combined)
					if llmResult.Decision == security.DecisionBlock {
						if logger != nil {
							logger.Warn("[Email] LLM Guardian blocked email content", "uid", msg.UID, "from", msg.From, "reason", llmResult.Reason)
						}
						msg.Body = security.RedactedText("llm guardian blocked content: " + llmResult.Reason)
						msg.Subject = security.SanitizedText("llm guardian blocked this message")
						msg.Snippet = security.RedactedText("")
						sanitized[idx] = msg
						continue
					}
				}

				msg.Body = guardian.SanitizeToolOutput("email", msg.Body)
				sanitized[idx] = msg
			}
		}()
	}

	for idx := range messages {
		indexCh <- idx
	}
	close(indexCh)
	wg.Wait()

	return sanitized
}

// mergeSkillVaultKeys combines vault_keys from a skill manifest with vault_keys from the tool call.
// Duplicates are removed. Returns nil if no keys.
func mergeSkillVaultKeys(skillsDir, skillName string, tcKeys []string) []string {
	seen := make(map[string]bool, len(tcKeys))
	var merged []string
	for _, k := range tcKeys {
		k = strings.TrimSpace(k)
		if k != "" && !seen[k] {
			seen[k] = true
			merged = append(merged, k)
		}
	}
	// Try to load manifest vault_keys
	skills, err := tools.ListSkills(skillsDir)
	if err == nil {
		for _, s := range skills {
			if s.Name == skillName {
				for _, k := range s.VaultKeys {
					k = strings.TrimSpace(k)
					if k != "" && !seen[k] {
						seen[k] = true
						merged = append(merged, k)
					}
				}
				break
			}
		}
	}
	return merged
}

func planTaskInputsFromItems(items []map[string]interface{}) []memory.PlanTaskInput {
	inputs := make([]memory.PlanTaskInput, 0, len(items))
	for _, item := range items {
		input := memory.PlanTaskInput{}
		if v, ok := item["title"].(string); ok {
			input.Title = strings.TrimSpace(v)
		}
		if v, ok := item["description"].(string); ok {
			input.Description = strings.TrimSpace(v)
		}
		if v, ok := item["kind"].(string); ok {
			input.Kind = strings.TrimSpace(v)
		}
		if v, ok := item["tool_name"].(string); ok {
			input.ToolName = strings.TrimSpace(v)
		}
		if v, ok := item["acceptance_criteria"].(string); ok {
			input.Acceptance = strings.TrimSpace(v)
		}
		if v, ok := item["owner"].(string); ok {
			input.Owner = strings.TrimSpace(v)
		}
		if v, ok := item["parent_task_id"].(string); ok {
			input.ParentTaskID = strings.TrimSpace(v)
		}
		if v, ok := item["tool_args"].(map[string]interface{}); ok {
			input.ToolArgs = v
		}
		switch deps := item["depends_on"].(type) {
		case []interface{}:
			for _, dep := range deps {
				input.DependsOn = append(input.DependsOn, fmt.Sprint(dep))
			}
		case []string:
			input.DependsOn = append(input.DependsOn, deps...)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func planTaskIDsFromItems(items []map[string]interface{}) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if v, ok := item["task_id"].(string); ok && strings.TrimSpace(v) != "" {
			ids = append(ids, strings.TrimSpace(v))
		}
	}
	return ids
}

// dispatchComm handles webhook, skill, notification, email, discord, mission, and notes tool calls.
func dispatchComm(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	cfg := dc.Cfg
	logger := dc.Logger
	llmClient := dc.LLMClient
	vault := dc.Vault
	registry := dc.Registry
	manifest := dc.Manifest
	cronManager := dc.CronManager
	missionManagerV2 := dc.MissionManagerV2
	longTermMem := dc.LongTermMem
	shortTermMem := dc.ShortTermMem
	kg := dc.KG
	inventoryDB := dc.InventoryDB
	invasionDB := dc.InvasionDB
	cheatsheetDB := dc.CheatsheetDB
	imageGalleryDB := dc.ImageGalleryDB
	mediaRegistryDB := dc.MediaRegistryDB
	homepageRegistryDB := dc.HomepageRegistryDB
	contactsDB := dc.ContactsDB
	sqlConnectionsDB := dc.SQLConnectionsDB
	sqlConnectionPool := dc.SQLConnectionPool
	remoteHub := dc.RemoteHub
	historyMgr := dc.HistoryMgr
	isMaintenance := dc.IsMaintenance
	guardian := dc.Guardian
	llmGuardian := dc.LLMGuardian
	sessionID := dc.SessionID
	coAgentRegistry := dc.CoAgentRegistry
	budgetTracker := dc.BudgetTracker

	switch tc.Action {
	case "call_webhook":
		if !cfg.Webhooks.Enabled {
			return `Tool Output: {"status":"error","message":"Webhooks are disabled in the config. Set webhooks.enabled=true."}`
		}
		logger.Info("LLM requested webhook execution", "webhook_name", tc.WebhookName)

		// Find the webhook by name
		var targetHook *config.OutgoingWebhook
		for _, w := range cfg.Webhooks.Outgoing {
			if strings.EqualFold(w.Name, tc.WebhookName) {
				targetHook = &w
				break
			}
		}

		if targetHook == nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Webhook '%s' not found. Check the exact name of the webhook from your System Context."}`, tc.WebhookName)
		}

		// Map parameters
		paramMap := make(map[string]interface{})
		if pm, ok := tc.Parameters.(map[string]interface{}); ok {
			for k, v := range pm {
				paramMap[k] = v
			}
		}

		out, statusCode, err := tools.ExecuteOutgoingWebhook(ctx, *targetHook, paramMap)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to execute webhook: %v"}`, err)
		}

		// Provide simple response
		return fmt.Sprintf(`Tool Output: {"status":"success", "http_status_code": %d, "response": %q}`, statusCode, out)

	case "manage_outgoing_webhooks":
		if !cfg.Webhooks.Enabled {
			return `Tool Output: {"status":"error","message":"Webhooks are disabled in the config. Set webhooks.enabled=true."}`
		}
		if cfg.Webhooks.ReadOnly && tc.Operation != "list" {
			return `Tool Output: {"status":"error","message":"Webhooks tool is set to Read-Only mode. Cannot modify."}`
		}

		var rawParams []interface{}
		if rp, ok := tc.Parameters.([]interface{}); ok {
			rawParams = rp
		}

		return tools.ManageOutgoingWebhooks(tc.Operation, tc.ID, tc.Name, tc.Description, tc.Method, tc.URL, tc.PayloadType, tc.BodyTemplate, tc.Headers, rawParams, cfg)

	case "list_skill_templates":
		logger.Info("LLM requested to list skill templates")
		templates := tools.AvailableSkillTemplates()
		type templateInfo struct {
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Parameters  map[string]string `json:"parameters"`
		}
		infos := make([]templateInfo, len(templates))
		for i, t := range templates {
			infos[i] = templateInfo{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
		}
		b, err := json.MarshalIndent(infos, "", "  ")
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR serializing templates: %v", err)
		}
		return fmt.Sprintf("Tool Output: Available Skill Templates:\n%s\n\nUse create_skill_from_template with template=<name> and name=<skill_name> to create a skill.", string(b))

	case "create_skill_from_template":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] create_skill_from_template is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		templateName := tc.Template
		skillName := tc.Name
		if templateName == "" {
			return "Tool Output: ERROR 'template' is required. Use list_skill_templates to see available templates."
		}
		if skillName == "" {
			return "Tool Output: ERROR 'name' is required for the new skill."
		}

		// Extract optional array fields from Params or SkillArgs
		var deps []string
		if rawDeps, ok := tc.Params["dependencies"]; ok {
			if arr, ok := rawDeps.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						deps = append(deps, s)
					}
				}
			}
		}

		result, err := tools.CreateSkillFromTemplate(
			cfg.Directories.SkillsDir,
			templateName,
			skillName,
			tc.Description,
			tc.URL,
			deps,
			tc.VaultKeys,
		)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR creating skill from template: %v", err)
		}

		// Provision dependencies immediately
		tools.ProvisionSkillDependencies(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, logger)

		logger.Info("Skill created from template",
			"template", templateName,
			"skill", skillName,
		)
		return "Tool Output: " + result

	case "list_skills":
		logger.Info("LLM requested to list skills")
		skills, err := tools.ListSkills(cfg.Directories.SkillsDir)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR listing skills: %v", err)
		}
		// Filter out built-in skills whose backing integration is disabled in config.
		// This prevents the agent from trying (and failing) to use skills that are
		// not configured, and then reporting them as "not working".
		var availableSkills []tools.SkillManifest
		for _, s := range skills {
			if s.Executable == "__builtin__" && !isBuiltinSkillEnabled(s.Name, cfg) {
				continue
			}
			availableSkills = append(availableSkills, s)
		}
		if len(availableSkills) == 0 {
			return "Tool Output: No internal skills found."
		}
		b, err := json.MarshalIndent(availableSkills, "", "  ")
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR serializing skills list: %v", err)
		}
		return fmt.Sprintf("Tool Output: Internal Skills Configuration:\n%s", string(b))

	case "execute_skill":
		logger.Info("LLM requested skill execution", "skill", tc.Skill, "args", tc.SkillArgs, "params", tc.Params)
		// Robust argument lookup: handle both 'skill_args' and 'params'
		args := tc.SkillArgs
		if args == nil {
			args = tc.Params
		}
		if len(args) == 0 {
			args = synthesizeExecuteSkillArgs(tc)
		}

		skillName := tc.Skill
		if skillName == "" && args != nil {
			// Aggressive recovery: Check if LLM nested the skill name inside arguments
			for _, key := range []string{"skill", "skill_name", "name", "tool"} {
				if s, ok := args[key].(string); ok && s != "" {
					skillName = s
					logger.Info("[Recovery] Found nested skill name in arguments", "key", key, "skill", skillName)
					break
				}
			}
		}

		if skillName == "" {
			return "Tool Output: ERROR 'skill' name is required. Use {\"action\": \"execute_skill\", \"skill\": \"name\", \"params\": {...}}"
		}

		// Unwrap skill_args if the LLM nested the actual parameters under that key.
		// e.g. {"skill_name": "ddg_search", "skill_args": {"query": "..."}} → {"query": "..."}
		if innerArgs, ok := args["skill_args"].(map[string]interface{}); ok && len(innerArgs) > 0 {
			args = innerArgs
		} else {
			// Clean up metadata keys that aren't real skill parameters
			cleanArgs := make(map[string]interface{}, len(args))
			metaKeys := map[string]bool{"skill_name": true, "skill": true, "name": true, "tool": true, "action": true}
			for k, v := range args {
				if !metaKeys[k] {
					cleanArgs[k] = v
				}
			}
			args = cleanArgs
		}
		cleanSkillName := strings.TrimSuffix(skillName, ".py")
		if nativeAction, ok := mistakenNativeToolSkillName(cleanSkillName); ok {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s is a native AuraGo tool, not a Python skill. Call it directly as {\"action\":\"%s\"} instead of wrapping it in execute_skill.","redirect_action":"%s","redirect_example":"{\"action\":\"%s\"}"}`, cleanSkillName, nativeAction, nativeAction, nativeAction)
		}
		args = filterExecuteSkillArgs(cfg.Directories.SkillsDir, cleanSkillName, args)
		if result, ok := handleExecuteSkillBuiltinAction(ctx, dc, cleanSkillName, args); ok {
			return result
		}
		switch cleanSkillName {
		case "git_backup_restore":
			reqJSON, _ := json.Marshal(args)
			var req tools.GitBackupRequest
			json.Unmarshal(reqJSON, &req)
			return tools.ExecuteGit(cfg.Directories.WorkspaceDir, req)
		case "google_workspace":
			if !cfg.GoogleWorkspace.Enabled {
				return `Tool Output: {"status": "error", "message": "Google Workspace is not enabled. Enable it in Settings > Google Workspace."}`
			}
			op, _ := args["operation"].(string)
			return "Tool Output: " + tools.ExecuteGoogleWorkspace(*cfg, vault, op, args)
		case "pdf_extractor":
			if !cfg.Tools.PDFExtractor.Enabled {
				return "Tool Output: [PERMISSION DENIED] pdf_extractor is disabled in settings (tools.pdf_extractor.enabled: false)."
			}
			filePath, _ := args["filepath"].(string)
			if ext := strings.ToLower(filepath.Ext(filePath)); filePath != "" && ext != "" && ext != ".pdf" {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s is not a PDF. Use analyze_image for PNG/JPG/WebP screenshots and images."}`, filePath)
			}
			result := tools.ExecutePDFExtract(cfg.Directories.WorkspaceDir, filePath)
			if cfg.Tools.PDFExtractor.SummaryMode {
				searchQuery, _ := args["search_query"].(string)
				if searchQuery == "" {
					searchQuery = "summarise the key content of this document"
				}
				summary, err := tools.SummariseContent(ctx, tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
					APIKey:  cfg.Tools.PDFExtractor.SummaryAPIKey,
					BaseURL: cfg.Tools.PDFExtractor.SummaryBaseURL,
					Model:   cfg.Tools.PDFExtractor.SummaryModel,
				}), logger, result, searchQuery, "PDF document")
				if err != nil {
					logger.Warn("pdf_extractor summary failed, returning raw content", "error", err)
				} else {
					result = summary
				}
			}
			return result
		}

		// If the skill manifest doesn't exist in SkillsDir but a matching .py is in ToolsDir,
		// redirect the agent to run_tool (mirror of the run_tool → execute_skill redirect).
		skillManifestPath := filepath.Join(cfg.Directories.SkillsDir, cleanSkillName+".json")
		if _, statErr := os.Stat(skillManifestPath); os.IsNotExist(statErr) {
			toolPyName := cleanSkillName + ".py"
			if _, toolErr := os.Stat(filepath.Join(cfg.Directories.ToolsDir, toolPyName)); toolErr == nil {
				return fmt.Sprintf("Tool Output: ERROR '%s' is a custom tool, not a skill. Use {\"action\": \"run_tool\", \"name\": \"%s\"} instead.", cleanSkillName, toolPyName)
			}
		}

		// Generic Python skill fallback — gate on AllowPython
		if !cfg.Agent.AllowPython {
			return fmt.Sprintf("Tool Output: [PERMISSION DENIED] Skill '%s' requires Python execution which is disabled (agent.allow_python: false).", skillName)
		}
		// Resolve vault secrets: merge skill manifest vault_keys with tool call vault_keys.
		// Split out cred:<id> entries — those are credential IDs, not vault keys.
		allVaultKeys := mergeSkillVaultKeys(cfg.Directories.SkillsDir, skillName, tc.VaultKeys)
		var plainVaultKeys []string
		var credIDsFromManifest []string
		for _, k := range allVaultKeys {
			if strings.HasPrefix(k, "cred:") {
				credIDsFromManifest = append(credIDsFromManifest, strings.TrimPrefix(k, "cred:"))
			} else {
				plainVaultKeys = append(plainVaultKeys, k)
			}
		}
		secrets, rejectedInfo := resolveVaultKeys(cfg, vault, plainVaultKeys, logger)
		// Merge credential IDs from manifest with those from the tool call
		mergedCredIDs := append(credIDsFromManifest, tc.CredentialIDs...)
		// Resolve credential secrets
		creds, credRejInfo := resolveCredentials(cfg, vault, inventoryDB, mergedCredIDs, logger)
		if credRejInfo != "" {
			if rejectedInfo != "" {
				rejectedInfo += "\n" + credRejInfo
			} else {
				rejectedInfo = credRejInfo
			}
		}
		var res string
		var skillErr error
		if cfg.Tools.SkillManager.RequireSandbox {
			// Sandbox enforcement: skills must run in the container sandbox
			if !tools.GetSandboxManager().IsReady() {
				return fmt.Sprintf("Tool Output: [SANDBOX REQUIRED] Skill '%s' requires sandbox execution but the sandbox is not available. Please enable the sandbox in settings (sandbox.enabled: true) or disable 'Require Sandbox' in skill manager settings.", skillName)
			}
			res, skillErr = tools.ExecuteSkillInSandbox(cfg.Directories.SkillsDir, cleanSkillName, args, secrets, creds, cfg.Tools.SkillTimeoutSeconds, logger)
		} else if len(secrets) > 0 || len(creds) > 0 {
			res, skillErr = tools.ExecuteSkillWithSecrets(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, skillName, args, secrets, creds)
		} else {
			res, skillErr = tools.ExecuteSkill(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, skillName, args)
		}
		if skillErr != nil {
			msg := fmt.Sprintf("Tool Output: ERROR executing skill: %s\nOutput: %s", security.Scrub(skillErr.Error()), security.Scrub(res))
			if rejectedInfo != "" {
				msg = rejectedInfo + "\n" + msg
			}
			return msg
		}
		if rejectedInfo != "" {
			return rejectedInfo + "\nTool Output: " + res
		}
		return fmt.Sprintf("Tool Output: %s", res)

	case "follow_up":
		logger.Info("LLM requested follow-up", "prompt", tc.TaskPrompt)
		if tc.TaskPrompt == "" {
			return "Tool Output: ERROR 'task_prompt' is required for follow_up"
		}
		if !cfg.Agent.BackgroundTasks.Enabled {
			return "Tool Output: ERROR background tasks are disabled in config (agent.background_tasks.enabled=false)."
		}

		// Guard: follow_up must describe work for the agent to do autonomously.
		// It must NEVER be used to relay a question back to the user — that causes
		// an infinite loop where each invocation re-asks the same question.
		trimmedPrompt := strings.TrimSpace(tc.TaskPrompt)
		if isFollowUpQuestion(trimmedPrompt) {
			logger.Warn("[follow_up] Blocked: task_prompt looks like a question directed at the user", "prompt", trimmedPrompt)
			return `Tool Output: [ERROR] follow_up must not be used to ask the user for information. ` +
				`If you need input from the user, respond directly with your question in plain text. ` +
				`follow_up is only for scheduling autonomous background work you will perform yourself.`
		}

		bgMgr := tools.DefaultBackgroundTaskManager()
		if bgMgr == nil {
			logger.Error("Background task manager is unavailable for follow_up")
			return "Tool Output: ERROR background task manager is unavailable."
		}
		delay := time.Duration(cfg.Agent.BackgroundTasks.FollowUpDelaySeconds) * time.Second
		if tc.DelaySeconds > 0 {
			delay = time.Duration(tc.DelaySeconds) * time.Second
		}
		timeout := time.Duration(cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds) * time.Second
		if tc.TimeoutSecs > 0 {
			timeout = time.Duration(tc.TimeoutSecs) * time.Second
		}
		task, err := bgMgr.ScheduleFollowUp(tc.TaskPrompt, tools.BackgroundTaskScheduleOptions{
			Source:             "follow_up",
			Description:        "Autonomous follow-up",
			Delay:              delay,
			MaxRetries:         cfg.Agent.BackgroundTasks.MaxRetries,
			RetryDelay:         time.Duration(cfg.Agent.BackgroundTasks.RetryDelaySeconds) * time.Second,
			Timeout:            timeout,
			NotifyOnCompletion: tc.NotifyOnCompletion,
		})
		if err != nil {
			logger.Error("Failed to schedule follow-up", "error", err)
			return fmt.Sprintf("Tool Output: ERROR failed to schedule follow_up: %v", err)
		}

		return fmt.Sprintf("Tool Output: Follow-up scheduled as background task %s. I will continue in the background after this message.", task.ID)

	case "wait_for_event":
		logger.Info("LLM requested wait_for_event", "event_type", tc.EventType, "task_prompt", tc.TaskPrompt)
		if !cfg.Agent.BackgroundTasks.Enabled {
			return "Tool Output: ERROR background tasks are disabled in config (agent.background_tasks.enabled=false)."
		}
		if tc.EventType == "" {
			return "Tool Output: ERROR 'event_type' is required for wait_for_event"
		}
		if tc.TaskPrompt == "" {
			return "Tool Output: ERROR 'task_prompt' is required for wait_for_event"
		}
		bgMgr := tools.DefaultBackgroundTaskManager()
		if bgMgr == nil {
			logger.Error("Background task manager is unavailable for wait_for_event")
			return "Tool Output: ERROR background task manager is unavailable."
		}
		waitTimeout := cfg.Agent.BackgroundTasks.WaitDefaultTimeoutSecs
		if tc.TimeoutSecs > 0 {
			waitTimeout = tc.TimeoutSecs
		}
		pollInterval := cfg.Agent.BackgroundTasks.WaitPollIntervalSecs
		if tc.IntervalSecs > 0 {
			pollInterval = tc.IntervalSecs
		}
		payload := tools.WaitForEventTaskPayload{
			EventType:           tc.EventType,
			TaskPrompt:          tc.TaskPrompt,
			URL:                 tc.URL,
			Host:                tc.Host,
			Port:                tc.Port,
			FilePath:            tc.FilePath,
			PID:                 tc.PID,
			PollIntervalSeconds: pollInterval,
			TimeoutSeconds:      waitTimeout,
		}
		desc := fmt.Sprintf("Wait for %s", tc.EventType)
		task, err := bgMgr.ScheduleWaitForEvent(payload, tools.BackgroundTaskScheduleOptions{
			Source:             "wait_for_event",
			Description:        desc,
			MaxRetries:         cfg.Agent.BackgroundTasks.MaxRetries,
			RetryDelay:         time.Duration(cfg.Agent.BackgroundTasks.RetryDelaySeconds) * time.Second,
			Timeout:            time.Duration(cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds) * time.Second,
			NotifyOnCompletion: tc.NotifyOnCompletion,
		})
		if err != nil {
			logger.Error("Failed to schedule wait_for_event", "error", err)
			return fmt.Sprintf("Tool Output: ERROR failed to schedule wait_for_event: %v", err)
		}
		return fmt.Sprintf("Tool Output: wait_for_event scheduled as background task %s. I will continue automatically once the event occurs.", task.ID)

	case "ask_aurago":
		message := strings.TrimSpace(tc.Message)
		if message == "" {
			message = strings.TrimSpace(tc.Content)
		}
		if message == "" {
			message = strings.TrimSpace(tc.Query)
		}
		if message == "" {
			return `Tool Output: {"status":"error","message":"message is required"}`
		}

		answer, err := AskAuraGoBridge(ctx, RunConfig{
			Config:             cfg,
			Logger:             logger,
			LLMClient:          llmClient,
			ShortTermMem:       shortTermMem,
			LongTermMem:        longTermMem,
			KG:                 kg,
			InventoryDB:        inventoryDB,
			InvasionDB:         invasionDB,
			CheatsheetDB:       cheatsheetDB,
			ImageGalleryDB:     imageGalleryDB,
			MediaRegistryDB:    mediaRegistryDB,
			HomepageRegistryDB: homepageRegistryDB,
			ContactsDB:         contactsDB,
			SQLConnectionsDB:   sqlConnectionsDB,
			SQLConnectionPool:  sqlConnectionPool,
			RemoteHub:          remoteHub,
			Vault:              vault,
			Registry:           registry,
			Manifest:           manifest,
			CronManager:        cronManager,
			MissionManagerV2:   missionManagerV2,
			CoAgentRegistry:    coAgentRegistry,
			BudgetTracker:      budgetTracker,
			LLMGuardian:        llmGuardian,
			SessionID:          vscodeDebugBridgeSessionID,
			MessageSource:      "mcp-vscode-bridge",
		}, message)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
		}
		return fmt.Sprintf("Tool Output: %s", answer)

	case "get_tool_manual":
		logger.Info("LLM requested tool manual", "name", tc.ToolName)
		if tc.ToolName == "" {
			return "Tool Output: ERROR 'tool_name' is required"
		}

		// Fallback for LLMs getting creative with the manual name
		cleanName := strings.TrimSuffix(tc.ToolName, ".md")
		cleanName = strings.TrimSuffix(cleanName, "_tool_manual")
		manualPath := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals", cleanName+".md")
		data, err := os.ReadFile(manualPath)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR could not read manual for '%s': %v", tc.ToolName, err)
		}
		return fmt.Sprintf("Tool Output: [MANUAL FOR %s]\n%s", tc.ToolName, string(data))

	case "execute_surgery":
		if !isMaintenance {
			return "Tool Output: ERROR 'execute_surgery' can ONLY be used when in Maintenance mode (Lifeboat). You are currently in Supervisor mode. You MUST use 'initiate_handover' first to propose a plan and switch to Maintenance mode for complex code changes."
		}

		// Robustness: handle both 'task_prompt' and 'content' for the plan
		plan := tc.TaskPrompt
		if plan == "" {
			plan = tc.Content
		}

		logger.Info("LLM requested surgery via Gemini CLI", "plan_len", len(plan), "prompt_preview", Truncate(plan, 100))
		if plan == "" {
			return "Tool Output: ERROR surgery plan is required (via 'task_prompt' or 'content')"
		}
		// Using external Gemini CLI via the surgery tool
		res, err := tools.ExecuteSurgery(plan, cfg.Directories.WorkspaceDir, logger)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR surgery failed: %v\nOutput: %s", err, res)
		}
		return fmt.Sprintf("Tool Output: Surgery successful.\nDetails:\n%s", res)

	case "exit_lifeboat":
		if !isMaintenance {
			return "Tool Output: ERROR 'exit_lifeboat' can only be used when already in maintenance mode. You are currently in the standard Supervisor mode."
		}
		logger.Info("LLM requested to exit lifeboat")
		tools.SetBusy(false)
		return "Tool Output: [LIFEBOAT_EXIT_SIGNAL] Maintenance complete. Attempting to return to main supervisor."

	case "initiate_handover":
		if isMaintenance {
			return "Tool Output: ERROR You are already in Lifeboat mode. Maintenance is active. Use 'exit_lifeboat' to return to the supervisor or 'execute_surgery' for code changes."
		}
		logger.Info("LLM requested lifeboat handover", "plan_len", len(tc.TaskPrompt))
		return tools.InitiateLifeboatHandover(tc.TaskPrompt, cfg)

	case "get_system_metrics", "system_metrics":
		logger.Info("LLM requested system metrics", "target", tc.Target)
		return "Tool Output: " + tools.GetSystemMetrics(tc.Target)

	case "process_analyzer":
		logger.Info("LLM requested process analysis", "operation", tc.Operation, "name", tc.Name, "pid", tc.PID)
		return "Tool Output: " + tools.AnalyzeProcesses(tc.Operation, tc.Name, int(tc.PID), tc.Limit)

	case "web_capture":
		logger.Info("LLM requested web capture", "operation", tc.Operation, "url", tc.URL)
		return "Tool Output: " + tools.WebCapture(tc.Operation, tc.URL, tc.Selector, tc.FullPage, tc.OutputDir)

	case "web_performance_audit":
		logger.Info("LLM requested web performance audit", "url", tc.URL, "viewport", tc.Viewport)
		return "Tool Output: " + tools.WebPerformanceAudit(tc.URL, tc.Viewport)

	case "network_ping":
		logger.Info("LLM requested network ping", "host", tc.Host)
		return "Tool Output: " + tools.NetworkPing(tc.Host, tc.Count, tc.Timeout)

	case "detect_file_type":
		logger.Info("LLM requested file type detection", "path", tc.FilePath)
		return "Tool Output: " + tools.DetectFileType(tc.FilePath, tc.Recursive)

	case "dns_lookup":
		logger.Info("LLM requested DNS lookup", "host", tc.Host, "record_type", tc.RecordType)
		return "Tool Output: " + tools.DNSLookup(tc.Host, tc.RecordType)

	case "port_scanner":
		logger.Info("LLM requested port scan", "host", tc.Host, "port_range", tc.PortRange)
		return "Tool Output: " + tools.ScanPorts(tc.Host, tc.PortRange, tc.TimeoutMs)

	case "site_crawler":
		logger.Info("LLM requested site crawler", "url", tc.URL, "max_depth", tc.MaxDepth, "max_pages", tc.MaxPages)
		return "Tool Output: " + tools.ExecuteCrawler(tc.URL, tc.MaxDepth, tc.MaxPages, tc.AllowedDomains, tc.Selector)

	case "whois_lookup":
		logger.Info("LLM requested WHOIS lookup", "domain", tc.Host)
		domain := tc.Host
		if domain == "" {
			domain = tc.URL
		}
		return "Tool Output: " + tools.WhoisLookup(domain, tc.IncludeRaw)

	case "site_monitor":
		if !cfg.Tools.WebScraper.Enabled {
			return `Tool Output: {"status":"error","message":"Site monitor is disabled. Enable web_scraper in config."}`
		}
		logger.Info("LLM requested site monitor", "op", tc.Operation, "url", tc.URL, "monitor_id", tc.MonitorID)
		return "Tool Output: " + tools.ExecuteSiteMonitor(cfg.SQLite.SiteMonitorPath, tc.Operation, tc.MonitorID, tc.URL, tc.Selector, tc.Interval, tc.Limit)

	case "form_automation":
		if !cfg.Tools.FormAutomation.Enabled {
			return `Tool Output: {"status":"error","message":"form_automation is disabled. Enable tools.form_automation.enabled in config."}`
		}
		if !cfg.Tools.WebCapture.Enabled {
			return `Tool Output: {"status":"error","message":"form_automation requires web_capture (headless browser). Enable tools.web_capture.enabled in config."}`
		}
		logger.Info("LLM requested form automation", "op", tc.Operation, "url", tc.URL)
		return "Tool Output: " + tools.ExecuteFormAutomation(tc.Operation, tc.URL, tc.Fields, tc.Selector, tc.ScreenshotDir)

	case "upnp_scan":
		if !cfg.Tools.UPnPScan.Enabled {
			return `Tool Output: {"status":"error","message":"upnp_scan is disabled. Enable tools.upnp_scan.enabled in config."}`
		}
		logger.Info("LLM requested UPnP scan", "search_target", tc.SearchTarget, "timeout_secs", tc.TimeoutSecs, "auto_register", tc.AutoRegister)
		scanResult := tools.ExecuteUPnPScan(tc.SearchTarget, tc.TimeoutSecs)
		if tc.AutoRegister && inventoryDB != nil {
			scanResult = upnpAutoRegister(scanResult, inventoryDB, tc.RegisterType, tc.RegisterTags, tc.OverwriteExisting, logger)
		}
		return "Tool Output: " + scanResult

	case "send_notification", "notification_center", "send_push_notification", "web_push":
		if tc.ToolName == "send_push_notification" || tc.ToolName == "web_push" {
			tc.Channel = "push"
		}
		logger.Info("LLM requested notification", "channel", tc.Channel, "title", tc.Title)
		// Use discord bridge (tools.DiscordSend) to avoid import cycle
		var discordSend tools.DiscordSendFunc
		if cfg.Discord.Enabled {
			discordSend = func(channelID, content string) error {
				return tools.DiscordSend(channelID, content, logger)
			}
		}
		var telnyxSend tools.TelnyxSendFunc
		if cfg.Telnyx.Enabled && cfg.Telnyx.PhoneNumber != "" {
			telnyxSend = func(to, message string) error {
				client := telnyx.NewClient(cfg.Telnyx.APIKey, logger)
				_, err := client.SendSMS(ctx, cfg.Telnyx.PhoneNumber, to, message, cfg.Telnyx.MessagingProfileID)
				return err
			}
		}
		priority := tc.Tag // reuse existing Tag field for priority
		return "Tool Output: " + tools.SendNotification(cfg, logger, tc.Channel, tc.Title, tc.Message, priority, discordSend, telnyxSend)

	case "send_image":
		logger.Info("LLM requested image send", "path", tc.Path, "caption", tc.Caption)
		return handleSendImage(tc, cfg, logger)

	case "send_audio":
		logger.Info("LLM requested audio send", "path", tc.Path, "title", tc.Title)
		return handleSendAudio(tc, cfg, logger, mediaRegistryDB)

	case "send_document":
		logger.Info("LLM requested document send", "path", tc.Path, "title", tc.Title)
		return handleSendDocument(tc, cfg, logger, mediaRegistryDB)

	case "manage_processes", "process_management":
		logger.Info("LLM requested process management", "op", tc.Operation)
		return "Tool Output: " + tools.ManageProcesses(tc.Operation, int32(tc.PID))

	case "register_device", "register_server":
		logger.Info("LLM requested device registration", "name", tc.Hostname)
		tags := services.ParseTags(tc.Tags)
		deviceType := tc.DeviceType
		if deviceType == "" {
			deviceType = "server"
		}

		// If LLM hallucinated, putting IP in Hostname and leaving IPAddress empty:
		if tc.IPAddress == "" && net.ParseIP(tc.Hostname) != nil {
			tc.IPAddress = tc.Hostname
		}

		id, err := services.RegisterDevice(inventoryDB, vault, tc.Hostname, deviceType, tc.IPAddress, tc.Port, tc.Username, tc.Password, tc.PrivateKeyPath, tc.Description, tags, tc.MACAddress)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to register device: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Device registered successfully", "id": "%s"}`, id)

	case "wake_on_lan", "wake_device", "wol":
		if !cfg.Tools.WOL.Enabled {
			return `Tool Output: {"status": "error", "message": "Wake-on-LAN is disabled. Enable it via tools.wol.enabled in config.yaml."}`
		}
		logger.Info("LLM requested Wake-on-LAN", "server_id", tc.ServerID, "mac", tc.MACAddress)

		mac := tc.MACAddress
		if mac == "" && tc.ServerID != "" && inventoryDB != nil {
			// Look up MAC from inventory
			device, err := inventory.GetDeviceByIDOrName(inventoryDB, tc.ServerID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
			}
			mac = device.MACAddress
		}
		if mac == "" {
			return `Tool Output: {"status": "error", "message": "No MAC address available. Provide 'mac_address' or a 'server_id' with a registered MAC address."}`
		}

		if err := tools.SendWakeOnLAN(mac, tc.IPAddress); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send WOL packet: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Wake-on-LAN magic packet sent to %s"}`, mac)

	case "pin_message":
		logger.Info("LLM requested message pinning", "id", tc.ID, "pinned", tc.Pinned)
		if tc.ID == "" {
			return `Tool Output: {"status": "error", "message": "'id' is required for pin_message"}`
		}
		// Try to parse ID as int64
		var msgID int64
		fmt.Sscanf(tc.ID, "%d", &msgID)
		if msgID == 0 {
			return `Tool Output: {"status": "error", "message": "Invalid 'id' format"}`
		}

		err := shortTermMem.SetMessagePinned(msgID, tc.Pinned)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to update SQLite: %v"}`, err)
		}
		if historyMgr != nil {
			_ = historyMgr.SetPinned(msgID, tc.Pinned)
		}
		status := "pinned"
		if !tc.Pinned {
			status = "unpinned"
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Message %d %s successfully."}`, msgID, status)

	case "fetch_email", "check_email":
		if !cfg.Email.Enabled && len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`
		}
		// Resolve email account
		var acct *config.EmailAccount
		if tc.Account != "" {
			acct = cfg.FindEmailAccount(tc.Account)
			if acct == nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' not found. Use list_email_accounts to see available accounts."}`, tc.Account)
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`
		}
		if acct.Disabled {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is disabled. Enable it in Settings > Email."}`, acct.ID)
		}
		logger.Info("LLM requested email fetch", "account", acct.ID, "folder", tc.Folder)
		folder := tc.Folder
		if folder == "" {
			folder = acct.WatchFolder
		}
		limit := tc.Limit
		if limit <= 0 {
			limit = 10
		}
		messages, err := tools.FetchEmails(
			acct.IMAPHost, acct.IMAPPort,
			acct.Username, acct.Password,
			folder, limit, logger,
		)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "IMAP fetch failed (%s): %v"}`, acct.ID, err)
		}
		messages = sanitizeFetchedEmails(ctx, logger, guardian, llmGuardian, cfg.LLMGuardian.ScanEmails, messages)
		result := tools.EmailResult{Status: "success", Count: len(messages), Data: messages, Message: fmt.Sprintf("Account: %s", acct.ID)}
		return "Tool Output: " + tools.EncodeEmailResult(result)

	case "send_email":
		if !cfg.Email.Enabled && len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`
		}
		if cfg.Email.ReadOnly {
			return `Tool Output: {"status":"error","message":"Email is in read-only mode. Disable email.read_only to allow sending."}`
		}
		// Resolve email account
		var acct *config.EmailAccount
		if tc.Account != "" {
			acct = cfg.FindEmailAccount(tc.Account)
			if acct == nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' not found. Use list_email_accounts to see available accounts."}`, tc.Account)
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`
		}
		if acct.Disabled {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is disabled. Enable it in Settings > Email."}`, acct.ID)
		}
		if acct.ReadOnly {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is read-only. Enable sending in Settings > Email."}`, acct.ID)
		}
		to := tc.To
		if to == "" {
			return `Tool Output: {"status": "error", "message": "'to' (recipient address) is required"}`
		}
		subject := tc.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		body := tc.Body
		if body == "" {
			body = tc.Content
		}
		logger.Info("LLM requested email send", "account", acct.ID, "to", to, "subject", subject)
		var sendErr error
		if acct.SMTPPort == 465 {
			sendErr = tools.SendEmailTLS(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		} else {
			sendErr = tools.SendEmail(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		}
		if sendErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "SMTP send failed (%s): %v"}`, acct.ID, sendErr)
		}
		result := tools.EmailResult{Status: "success", Message: fmt.Sprintf("Email sent to %s via account %s", to, acct.ID)}
		return "Tool Output: " + tools.EncodeEmailResult(result)

	case "list_email_accounts":
		if len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "success", "count": 0, "data": [], "message": "No email accounts configured."}`
		}
		type acctInfo struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			IMAP      string `json:"imap"`
			SMTP      string `json:"smtp"`
			Watcher   bool   `json:"watcher"`
			Enabled   bool   `json:"enabled"`
			AllowSend bool   `json:"allow_sending"`
		}
		var accts []acctInfo
		for _, a := range cfg.EmailAccounts {
			accts = append(accts, acctInfo{
				ID:        a.ID,
				Name:      a.Name,
				Email:     a.FromAddress,
				IMAP:      fmt.Sprintf("%s:%d", a.IMAPHost, a.IMAPPort),
				SMTP:      fmt.Sprintf("%s:%d", a.SMTPHost, a.SMTPPort),
				Watcher:   a.WatchEnabled,
				Enabled:   !a.Disabled,
				AllowSend: !a.ReadOnly,
			})
		}
		result := tools.EmailResult{Status: "success", Count: len(accts), Data: accts}
		return "Tool Output: " + tools.EncodeEmailResult(result)

	case "send_discord":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`
		}
		if cfg.Discord.ReadOnly {
			return `Tool Output: {"status":"error","message":"Discord is in read-only mode. Disable discord.read_only to allow changes."}`
		}
		channelID := tc.ChannelID
		if channelID == "" {
			channelID = cfg.Discord.DefaultChannelID
		}
		if channelID == "" {
			return `Tool Output: {"status": "error", "message": "'channel_id' is required (or set default_channel_id in config)"}`
		}
		message := tc.Message
		if message == "" {
			message = tc.Content
		}
		if message == "" {
			message = tc.Body
		}
		if message == "" {
			return `Tool Output: {"status": "error", "message": "'message' (or 'content') is required"}`
		}
		logger.Info("LLM requested Discord send", "channel", channelID)
		if err := tools.DiscordSend(channelID, message, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Discord send failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Message sent to Discord channel %s"}`, channelID)

	case "fetch_discord":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`
		}
		channelID := tc.ChannelID
		if channelID == "" {
			channelID = cfg.Discord.DefaultChannelID
		}
		if channelID == "" {
			return `Tool Output: {"status": "error", "message": "'channel_id' is required (or set default_channel_id in config)"}`
		}
		limit := tc.Limit
		if limit <= 0 {
			limit = 10
		}
		logger.Info("LLM requested Discord message fetch", "channel", channelID, "limit", limit)
		msgs, err := tools.DiscordFetch(channelID, limit, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Discord fetch failed: %v"}`, err)
		}
		// Guardian-sanitize external content
		if guardian != nil {
			for i := range msgs {
				scanRes := guardian.ScanForInjection(msgs[i].Author + " " + msgs[i].Content)
				if scanRes.Level >= security.ThreatHigh {
					logger.Warn("[Discord] Guardian HIGH threat in message", "author", msgs[i].Author, "threat", scanRes.Level.String())
					msgs[i].Content = security.RedactedText("guardian blocked content after injection detection")
				} else {
					msgs[i].Content = guardian.SanitizeToolOutput("discord", msgs[i].Content)
				}
			}
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(msgs),
			"data":   msgs,
		})
		return "Tool Output: " + string(data)

	case "list_discord_channels":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled."}`
		}
		guildID := cfg.Discord.GuildID
		if guildID == "" {
			return `Tool Output: {"status": "error", "message": "'guild_id' must be set in config.yaml"}`
		}
		logger.Info("LLM requested Discord channel list", "guild", guildID)
		channels, err := tools.DiscordListChannels(guildID, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Channel list failed: %v"}`, err)
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(channels),
			"data":   channels,
		})
		return "Tool Output: " + string(data)

	case "manage_missions":
		if !cfg.Tools.Missions.Enabled {
			return `Tool Output: {"status":"error","message":"Missions are disabled. Set tools.missions.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Missions.ReadOnly {
			switch tc.Operation {
			case "create", "add", "update", "edit", "delete", "remove", "run", "run_now", "execute":
				return `Tool Output: {"status":"error","message":"Missions are in read-only mode. Disable tools.missions.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested mission management", "op", tc.Operation)
		if missionManagerV2 == nil {
			return `Tool Output: {"status": "error", "message": "Mission control storage not available"}`
		}

		switch tc.Operation {
		case "list":
			missions := missionManagerV2.List()
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "data": missions})
			return "Tool Output: " + string(b)

		case "create", "add":
			if tc.Title == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'title' (name) and 'command' (prompt) are required for create"}`
			}
			priorityStr := "medium"
			if tc.Priority == 1 {
				priorityStr = "low"
			} else if tc.Priority == 3 {
				priorityStr = "high"
			}
			m := &tools.MissionV2{
				Name:     tc.Title,
				Prompt:   tc.Command,
				Schedule: tc.CronExpr,
				Priority: priorityStr,
				Locked:   tc.Locked,
				Enabled:  true,
			}
			if m.Schedule != "" {
				m.ExecutionType = tools.ExecutionScheduled
			} else {
				m.ExecutionType = tools.ExecutionManual
			}
			err := missionManagerV2.Create(m)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "message": "Mission created"})
			return "Tool Output: " + string(b)

		case "update", "edit":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for update"}`
			}
			existing, ok := missionManagerV2.Get(tc.ID)
			if !ok {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Mission %s not found"}`, tc.ID)
			}

			if tc.Title != "" {
				existing.Name = tc.Title
			}
			if tc.Command != "" {
				existing.Prompt = tc.Command
			}
			if tc.CronExpr != "" {
				existing.Schedule = tc.CronExpr
			}
			if tc.Priority > 0 {
				if tc.Priority == 1 {
					existing.Priority = "low"
				} else if tc.Priority == 3 {
					existing.Priority = "high"
				} else {
					existing.Priority = "medium"
				}
			}
			// Only apply lock state changes if the LLM explicitly provides it,
			// though typical struct fields default to false if omitted.
			// Since we want to allow keeping existing state, we should check if it was provided in raw json
			if strings.Contains(tc.RawJSON, `"locked"`) {
				existing.Locked = tc.Locked
			}

			err := missionManagerV2.Update(tc.ID, existing)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission updated"}`

		case "delete", "remove":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for delete"}`
			}
			err := missionManagerV2.Delete(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission deleted"}`

		case "run", "run_now":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for run"}`
			}
			err := missionManagerV2.RunNow(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission scheduled for immediate execution by the background task queue"}`

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, tc.Operation)
		}

	case "remember":
		if !cfg.Tools.Memory.Enabled {
			return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Memory.ReadOnly {
			return `Tool Output: {"status":"error","message":"Memory is in read-only mode."}`
		}
		return handleRemember(tc, cfg, logger, shortTermMem, kg, sessionID)

	case "manage_plan":
		logger.Info("LLM requested plan management", "op", tc.Operation, "session_id", sessionID)
		if shortTermMem == nil {
			return `Tool Output: {"status":"error","message":"Plan storage not available"}`
		}
		switch tc.Operation {
		case "create":
			if tc.Title == "" {
				return `Tool Output: {"status":"error","message":"'title' is required for create"}`
			}
			plan, err := shortTermMem.CreatePlan(sessionID, tc.Title, tc.Description, tc.Content, tc.Priority, planTaskInputsFromItems(tc.Items))
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan created","plan":%s}`, string(payload))
		case "list":
			status := tc.Status
			if status == "" {
				status = "all"
			}
			plans, err := shortTermMem.ListPlans(sessionID, status, tc.Limit, tc.IncludeArchived)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plans)
			return fmt.Sprintf(`Tool Output: {"status":"success","count":%d,"plans":%s}`, len(plans), string(payload))
		case "get":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for get"}`
			}
			plan, err := shortTermMem.GetPlan(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","plan":%s}`, string(payload))
		case "set_status":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for set_status"}`
			}
			plan, err := shortTermMem.SetPlanStatus(tc.ID, tc.Status, tc.Content)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan status updated","plan":%s}`, string(payload))
		case "update_task":
			if tc.ID == "" || tc.TaskID == "" {
				return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for update_task"}`
			}
			plan, err := shortTermMem.UpdatePlanTask(tc.ID, tc.TaskID, tc.Status, tc.Result, tc.Error)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task updated","plan":%s}`, string(payload))
		case "advance":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for advance"}`
			}
			plan, err := shortTermMem.AdvancePlan(tc.ID, tc.Result)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan advanced","plan":%s}`, string(payload))
		case "set_blocker":
			if tc.ID == "" || tc.TaskID == "" {
				return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for set_blocker"}`
			}
			reason := strings.TrimSpace(tc.Reason)
			if reason == "" {
				reason = strings.TrimSpace(tc.Content)
			}
			plan, err := shortTermMem.SetPlanTaskBlocker(tc.ID, tc.TaskID, reason)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task blocker set","plan":%s}`, string(payload))
		case "clear_blocker":
			if tc.ID == "" || tc.TaskID == "" {
				return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for clear_blocker"}`
			}
			note := strings.TrimSpace(tc.Content)
			if note == "" {
				note = strings.TrimSpace(tc.Reason)
			}
			plan, err := shortTermMem.ClearPlanTaskBlocker(tc.ID, tc.TaskID, note)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task blocker cleared","plan":%s}`, string(payload))
		case "append_note":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for append_note"}`
			}
			if err := shortTermMem.AppendPlanNote(tc.ID, tc.Content); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			plan, err := shortTermMem.GetPlan(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan note appended","plan":%s}`, string(payload))
		case "attach_artifact":
			if tc.ID == "" || tc.TaskID == "" {
				return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for attach_artifact"}`
			}
			value := strings.TrimSpace(tc.Content)
			if value == "" {
				value = strings.TrimSpace(tc.FilePath)
			}
			if value == "" {
				value = strings.TrimSpace(tc.URL)
			}
			plan, err := shortTermMem.AttachPlanTaskArtifact(tc.ID, tc.TaskID, memory.PlanArtifact{
				Type:  strings.TrimSpace(tc.ArtifactType),
				Label: strings.TrimSpace(tc.Label),
				Value: value,
			})
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Artifact attached","plan":%s}`, string(payload))
		case "split_task":
			if tc.ID == "" || tc.TaskID == "" {
				return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for split_task"}`
			}
			plan, err := shortTermMem.SplitPlanTask(tc.ID, tc.TaskID, planTaskInputsFromItems(tc.Items))
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task split into subtasks","plan":%s}`, string(payload))
		case "reorder_tasks":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for reorder_tasks"}`
			}
			orderedTaskIDs := planTaskIDsFromItems(tc.Items)
			plan, err := shortTermMem.ReorderPlanTasks(tc.ID, orderedTaskIDs)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			payload, _ := json.Marshal(plan)
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task order updated","plan":%s}`, string(payload))
		case "archive_completed":
			if tc.ID != "" {
				plan, err := shortTermMem.ArchivePlan(tc.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan archived","plan":%s}`, string(payload))
			}
			count, err := shortTermMem.ArchiveCompletedPlans(sessionID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Archived completed plans","count":%d}`, count)
		case "delete":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for delete"}`
			}
			if err := shortTermMem.DeletePlan(tc.ID); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
			}
			return `Tool Output: {"status":"success","message":"Plan deleted"}`
		default:
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown plan operation: %s. Use create, list, get, update_task, advance, set_status, set_blocker, clear_blocker, append_note, attach_artifact, split_task, reorder_tasks, archive_completed, or delete"}`, tc.Operation)
		}

	case "manage_notes", "notes", "todo":
		if !cfg.Tools.Notes.Enabled {
			return `Tool Output: {"status":"error","message":"Notes are disabled. Set tools.notes.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Notes.ReadOnly {
			switch tc.Operation {
			case "add", "update", "toggle", "delete":
				return `Tool Output: {"status":"error","message":"Notes are in read-only mode. Disable tools.notes.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested notes/todo management", "op", tc.Operation)
		if shortTermMem == nil {
			return `Tool Output: {"status": "error", "message": "Notes storage not available"}`
		}
		switch tc.Operation {
		case "add":
			if tc.Title == "" {
				return `Tool Output: {"status": "error", "message": "'title' is required for add"}`
			}
			id, err := shortTermMem.AddNote(tc.Category, tc.Title, tc.Content, tc.Priority, tc.DueDate)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note created", "id": %d}`, id)
		case "list":
			notes, err := shortTermMem.ListNotes(tc.Category, tc.Done)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "notes": %s}`, len(notes), memory.FormatNotesJSON(notes))
		case "update":
			if tc.NoteID <= 0 {
				return `Tool Output: {"status": "error", "message": "'note_id' is required for update"}`
			}
			err := shortTermMem.UpdateNote(tc.NoteID, tc.Title, tc.Content, tc.Category, tc.Priority, tc.DueDate)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note %d updated"}`, tc.NoteID)
		case "toggle":
			if tc.NoteID <= 0 {
				return `Tool Output: {"status": "error", "message": "'note_id' is required for toggle"}`
			}
			newState, err := shortTermMem.ToggleNoteDone(tc.NoteID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "note_id": %d, "done": %t}`, tc.NoteID, newState)
		case "delete":
			if tc.NoteID <= 0 {
				return `Tool Output: {"status": "error", "message": "'note_id' is required for delete"}`
			}
			err := shortTermMem.DeleteNote(tc.NoteID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note %d deleted"}`, tc.NoteID)
		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown notes operation: %s. Use add, list, update, toggle, or delete"}`, tc.Operation)
		}

	case "manage_journal", "journal":
		if !cfg.Tools.Journal.Enabled {
			return `Tool Output: {"status":"error","message":"Journal is disabled. Set tools.journal.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Journal.ReadOnly {
			switch tc.Operation {
			case "add", "delete":
				return `Tool Output: {"status":"error","message":"Journal is in read-only mode. Disable tools.journal.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested journal management", "op", tc.Operation)
		if shortTermMem == nil {
			return `Tool Output: {"status": "error", "message": "Journal storage not available"}`
		}
		switch tc.Operation {
		case "add":
			if tc.Title == "" {
				return `Tool Output: {"status": "error", "message": "'title' is required for add"}`
			}
			entryType := tc.EntryType
			if entryType == "" {
				entryType = "reflection"
			}
			importance := tc.Importance
			if importance < 1 || importance > 4 {
				importance = 2
			}
			var tags []string
			if tc.Tags != "" {
				for _, t := range strings.Split(tc.Tags, ",") {
					if s := strings.TrimSpace(t); s != "" {
						tags = append(tags, s)
					}
				}
			}
			id, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
				EntryType:     entryType,
				Title:         tc.Title,
				Content:       tc.Content,
				Tags:          tags,
				Importance:    importance,
				SessionID:     sessionID,
				AutoGenerated: false,
			})
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Journal entry created", "id": %d}`, id)
		case "list":
			limit := tc.Limit
			if limit <= 0 {
				limit = 20
			}
			var types []string
			if tc.EntryType != "" {
				types = []string{tc.EntryType}
			}
			entries, err := shortTermMem.GetJournalEntries(tc.FromDate, tc.ToDate, types, limit)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "entries": %s}`, len(entries), memory.FormatJournalEntriesJSON(entries))
		case "search":
			if tc.Query == "" {
				return `Tool Output: {"status": "error", "message": "'query' is required for search"}`
			}
			limit := tc.Limit
			if limit <= 0 {
				limit = 20
			}
			entries, err := shortTermMem.SearchJournalEntries(tc.Query, limit)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "entries": %s}`, len(entries), memory.FormatJournalEntriesJSON(entries))
		case "delete":
			if tc.EntryID <= 0 {
				return `Tool Output: {"status": "error", "message": "'entry_id' is required for delete"}`
			}
			err := shortTermMem.DeleteJournalEntry(tc.EntryID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Journal entry %d deleted"}`, tc.EntryID)
		case "get_summary":
			date := tc.FromDate
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			summary, err := shortTermMem.GetDailySummary(date)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			if summary == nil {
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "No summary available for %s"}`, date)
			}
			topicsJSON, _ := json.Marshal(summary.KeyTopics)
			return fmt.Sprintf(`Tool Output: {"status": "success", "date": "%s", "summary": %q, "key_topics": %s, "sentiment": %q}`,
				summary.Date, summary.Summary, string(topicsJSON), summary.Sentiment)
		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown journal operation: %s. Use add, list, search, delete, or get_summary"}`, tc.Operation)
		}

	case "telnyx_sms":
		if !cfg.Telnyx.Enabled {
			return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`
		}
		if cfg.Telnyx.ReadOnly {
			return `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`
		}
		if cfg.Telnyx.PhoneNumber == "" {
			return `Tool Output: {"status":"error","message":"Telnyx phone number not configured"}`
		}
		msgID := tc.ID
		if msgID == "" {
			msgID = tc.MessageID
		}
		return "Tool Output: " + telnyx.DispatchSMS(ctx, tc.Operation, tc.To, tc.Message, msgID, tc.MediaURLs, cfg, logger)

	case "telnyx_call":
		if !cfg.Telnyx.Enabled {
			return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`
		}
		if cfg.Telnyx.ReadOnly && tc.Operation != "list_active" {
			return `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`
		}
		return "Tool Output: " + telnyx.DispatchCall(ctx, tc.Operation, tc.To, tc.CallControlID, tc.Text, tc.AudioURL, tc.MaxDigits, tc.TimeoutSecs, cfg, logger)

	case "telnyx_manage":
		if !cfg.Telnyx.Enabled {
			return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`
		}
		return "Tool Output: " + telnyx.DispatchManage(ctx, tc.Operation, tc.Limit, tc.Port, cfg, logger)

	case "address_book":
		if !cfg.Tools.Contacts.Enabled {
			return `Tool Output: {"status":"error","message":"Address book is disabled. Enable tools.contacts.enabled in config."}`
		}
		if contactsDB == nil {
			return `Tool Output: {"status":"error","message":"Contacts database not available."}`
		}
		logger.Info("LLM requested address book operation", "op", tc.Operation)
		switch tc.Operation {
		case "list":
			list, err := contacts.List(contactsDB, "")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			return "Tool Output: " + contacts.ToJSON(map[string]interface{}{"status": "success", "contacts": list, "count": len(list)})
		case "search":
			if tc.Query == "" {
				return `Tool Output: {"status":"error","message":"'query' is required for search operation"}`
			}
			list, err := contacts.List(contactsDB, tc.Query)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			return "Tool Output: " + contacts.ToJSON(map[string]interface{}{"status": "success", "contacts": list, "count": len(list)})
		case "add":
			if tc.Name == "" {
				return `Tool Output: {"status":"error","message":"'name' is required to add a contact"}`
			}
			c := contacts.Contact{
				Name:         tc.Name,
				Email:        tc.Email,
				Phone:        tc.Phone,
				Mobile:       tc.Mobile,
				Address:      tc.ContactAddress,
				Relationship: tc.Relationship,
				Notes:        tc.Notes,
			}
			id, err := contacts.Create(contactsDB, c)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Contact created","id":"%s"}`, id)
		case "update":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for update operation"}`
			}
			existing, err := contacts.GetByID(contactsDB, tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			if tc.Name != "" {
				existing.Name = tc.Name
			}
			if tc.Email != "" {
				existing.Email = tc.Email
			}
			if tc.Phone != "" {
				existing.Phone = tc.Phone
			}
			if tc.Mobile != "" {
				existing.Mobile = tc.Mobile
			}
			if tc.ContactAddress != "" {
				existing.Address = tc.ContactAddress
			}
			if tc.Relationship != "" {
				existing.Relationship = tc.Relationship
			}
			if tc.Notes != "" {
				existing.Notes = tc.Notes
			}
			if err := contacts.Update(contactsDB, *existing); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Contact updated","id":"%s"}`, tc.ID)
		case "delete":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for delete operation"}`
			}
			if err := contacts.Delete(contactsDB, tc.ID); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Contact deleted","id":"%s"}`, tc.ID)
		default:
			return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, search, add, update, delete"}`
		}

	// Built-in search/scrape tools — also callable as direct actions (no execute_skill wrapper needed)
	case "ddg_search":
		if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
			return result
		}
		return unexpectedBuiltinActionError(tc.Action)

	case "web_scraper":
		if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
			return result
		}
		return unexpectedBuiltinActionError(tc.Action)

	case "wikipedia_search":
		if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
			return result
		}
		return unexpectedBuiltinActionError(tc.Action)

	case "virustotal_scan":
		if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
			return result
		}
		return unexpectedBuiltinActionError(tc.Action)

	case "brave_search":
		if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
			return result
		}
		return unexpectedBuiltinActionError(tc.Action)
		if !cfg.BraveSearch.Enabled {
			return `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Enable it in Settings › Brave Search."}`
		}
		queryStr := tc.Query
		if queryStr == "" {
			queryStr, _ = tc.Params["query"].(string)
		}
		count, ok := tc.Params["count"].(float64)
		if !ok {
			count = 10
		}
		country, _ := tc.Params["country"].(string)
		if country == "" {
			country = cfg.BraveSearch.Country
		}
		lang, _ := tc.Params["lang"].(string)
		if lang == "" {
			lang = cfg.BraveSearch.Lang
		}
		return tools.ExecuteBraveSearch(cfg.BraveSearch.APIKey, queryStr, int(count), country, lang)

	default:
		return dispatchNotHandled
	}
}

func synthesizeExecuteSkillArgs(tc ToolCall) map[string]interface{} {
	raw, err := json.Marshal(tc)
	if err != nil {
		return map[string]interface{}{}
	}

	var args map[string]interface{}
	if err := json.Unmarshal(raw, &args); err != nil {
		return map[string]interface{}{}
	}

	for _, key := range []string{
		"action", "skill", "skill_args", "params", "_todo",
		"raw_json", "native_call_id",
	} {
		delete(args, key)
	}

	cleaned := make(map[string]interface{}, len(args))
	for k, v := range args {
		if isEmptySkillArgValue(v) {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

func isEmptySkillArgValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	default:
		return false
	}
}

func filterExecuteSkillArgs(skillsDir, skillName string, args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 || strings.TrimSpace(skillName) == "" {
		return args
	}

	skills, err := tools.ListSkills(skillsDir)
	if err != nil {
		return args
	}

	var manifest *tools.SkillManifest
	for i := range skills {
		if strings.EqualFold(skills[i].Name, skillName) {
			manifest = &skills[i]
			break
		}
	}
	if manifest == nil || len(manifest.Parameters) == 0 {
		return args
	}

	aliasArgs := make(map[string]interface{}, len(args)+2)
	for k, v := range args {
		aliasArgs[k] = v
	}
	if v, ok := aliasArgs["file_path"]; ok {
		if _, exists := aliasArgs["filepath"]; !exists {
			aliasArgs["filepath"] = v
		}
	}
	if v, ok := aliasArgs["filepath"]; ok {
		if _, exists := aliasArgs["file_path"]; !exists {
			aliasArgs["file_path"] = v
		}
		if _, exists := aliasArgs["path"]; !exists {
			aliasArgs["path"] = v
		}
	}
	if v, ok := aliasArgs["path"]; ok {
		if _, exists := aliasArgs["filepath"]; !exists {
			aliasArgs["filepath"] = v
		}
		if _, exists := aliasArgs["file_path"]; !exists {
			aliasArgs["file_path"] = v
		}
	}

	filtered := make(map[string]interface{}, len(manifest.Parameters))
	for key := range manifest.Parameters {
		if v, ok := aliasArgs[key]; ok && !isEmptySkillArgValue(v) {
			filtered[key] = v
		}
	}
	return filtered
}

// upnpAutoRegister parses the JSON result from ExecuteUPnPScan, bulk-inserts every
// discovered device into the inventory, and appends a registration summary.
func upnpAutoRegister(scanJSON string, db *sql.DB, deviceType string, tags []string, overwrite bool, logger *slog.Logger) string {
	var result struct {
		Status  string `json:"status"`
		Count   int    `json:"count"`
		Devices []struct {
			USN          string `json:"usn"`
			Location     string `json:"location"`
			FriendlyName string `json:"friendly_name"`
			DeviceType   string `json:"device_type"`
			Manufacturer string `json:"manufacturer"`
			ModelName    string `json:"model_name"`
		} `json:"devices"`
		Message string `json:"message,omitempty"`
	}
	if err := json.Unmarshal([]byte(scanJSON), &result); err != nil || result.Status != "success" || len(result.Devices) == 0 {
		return scanJSON
	}

	var created, updated, skipped int
	for _, dev := range result.Devices {
		name := dev.FriendlyName
		if name == "" {
			name = dev.ModelName
		}
		if name == "" {
			name = dev.USN
		}

		ip, port := upnpParseLocation(dev.Location)

		dt := deviceType
		if dt == "" {
			dt = dev.DeviceType
			if dt == "" {
				dt = "upnp-device"
			}
		}

		desc := strings.TrimSpace(dev.Manufacturer + " " + dev.ModelName)

		record := inventory.DeviceRecord{
			Name:        name,
			Type:        dt,
			IPAddress:   ip,
			Port:        port,
			Description: desc,
			Tags:        tags,
		}
		c, u, err := inventory.UpsertDeviceByName(db, record, overwrite)
		if err != nil {
			logger.Warn("upnp auto-register: failed to upsert device", "name", name, "error", err)
			skipped++
			continue
		}
		switch {
		case c:
			created++
		case u:
			updated++
		default:
			skipped++
		}
	}

	type regSummary struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Skipped int `json:"skipped"`
	}
	out := map[string]interface{}{
		"status":        result.Status,
		"count":         result.Count,
		"devices":       result.Devices,
		"auto_register": regSummary{Created: created, Updated: updated, Skipped: skipped},
	}
	if result.Message != "" {
		out["message"] = result.Message
	}
	b, _ := json.Marshal(out)
	return string(b)
}

var nativeToolSkillConfusions = map[string]string{
	"upnp_scan":        "upnp_scan",
	"mdns_scan":        "mdns_scan",
	"network_ping":     "network_ping",
	"port_scanner":     "port_scanner",
	"dns_lookup":       "dns_lookup",
	"whois_lookup":     "whois_lookup",
	"detect_file_type": "detect_file_type",
	"site_crawler":     "site_crawler",
	"web_capture":      "web_capture",
	"web_performance":  "web_performance",
}

func mistakenNativeToolSkillName(skillName string) (string, bool) {
	clean := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(skillName, ".py")))
	action, ok := nativeToolSkillConfusions[clean]
	return action, ok
}

// isBuiltinSkillEnabled returns false for built-in skills whose backing
// integration is disabled in config. This lets list_skills filter them out so
// the agent never attempts to call an integration that is not available.
func isBuiltinSkillEnabled(skillName string, cfg *config.Config) bool {
	switch skillName {
	case "brave_search":
		return cfg.BraveSearch.Enabled
	case "virustotal_scan":
		return cfg.VirusTotal.Enabled
	case "web_scraper":
		return cfg.Tools.WebScraper.Enabled
	case "pdf_extractor":
		return cfg.Tools.PDFExtractor.Enabled
	case "paperless":
		return cfg.PaperlessNGX.Enabled
	// ddg_search, wikipedia_search, git_backup_restore have no dedicated enabled flag
	// and are always available.
	default:
		return true
	}
}

// upnpParseLocation extracts the IP address and port from a UPnP location URL.
// e.g. "http://192.168.1.1:49152/desc.xml" → ("192.168.1.1", 49152)
func upnpParseLocation(location string) (ip string, port int) {
	if location == "" {
		return "", 0
	}
	u, err := url.Parse(location)
	if err != nil {
		return "", 0
	}
	ip = u.Hostname()
	if p := u.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	return ip, port
}
