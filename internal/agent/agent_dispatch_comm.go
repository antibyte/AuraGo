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
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/desktop"
	"aurago/internal/inventory"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"
)

// mergeSkillVaultKeys combines vault_keys from a skill manifest with vault_keys from the tool call.
// Duplicates are removed. Returns nil if no keys.
// resolveSkillBridgeTools returns the intersection of the skill manifest's InternalTools
// with the config's PythonToolBridge.AllowedTools. Returns nil if the bridge is disabled
// or if the skill hasn't declared any internal_tools.
func resolveSkillBridgeTools(cfg *config.Config, skillsDir, skillName string) []string {
	if !cfg.Tools.PythonToolBridge.Enabled || len(cfg.Tools.PythonToolBridge.AllowedTools) == 0 {
		return nil
	}
	skills, err := tools.ListSkills(skillsDir)
	if err != nil {
		return nil
	}
	var manifestTools []string
	for _, s := range skills {
		if s.Name == skillName {
			manifestTools = s.InternalTools
			break
		}
	}
	if len(manifestTools) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(cfg.Tools.PythonToolBridge.AllowedTools))
	for _, t := range cfg.Tools.PythonToolBridge.AllowedTools {
		allowed[t] = true
	}
	var result []string
	for _, t := range manifestTools {
		if allowed[t] {
			result = append(result, t)
		}
	}
	return result
}

// toolBridgeURL constructs the loopback URL for the tool bridge endpoint.
func toolBridgeURL(cfg *config.Config) string {
	return internalAPIBaseURL(cfg) + "/api/internal/tool-bridge"
}

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

func dispatchQuestionUser(tc ToolCall, dc *DispatchContext) string {
	req := decodeQuestionUserArgs(tc)
	if strings.TrimSpace(req.Question) == "" {
		return `Tool Output: {"status":"error","message":"question is required"}`
	}
	if len(req.Options) < 2 {
		return `Tool Output: {"status":"error","message":"at least two options are required"}`
	}
	sessionID := strings.TrimSpace(dc.SessionID)
	if sessionID == "" {
		sessionID = "default"
	}
	source := strings.ToLower(strings.TrimSpace(dc.MessageSource))
	if source == "" {
		source = "web_chat"
	}
	timeoutSecs := req.TimeoutSecs
	if timeoutSecs <= 0 {
		if source == "web_chat" {
			timeoutSecs = 120
		} else {
			timeoutSecs = 20
		}
	}
	if source != "web_chat" && req.TimeoutSecs <= 0 {
		timeoutSecs = 20
	}
	timeout := time.Duration(timeoutSecs) * time.Second
	q := &tools.PendingQuestion{
		Question:      req.Question,
		Options:       req.Options,
		AllowFreeText: req.AllowFreeText,
		Timeout:       timeout,
		TimeoutSecs:   timeoutSecs,
	}
	responseCh := tools.RegisterQuestion(sessionID, q)
	defer tools.CancelQuestion(sessionID)

	if source == "web_chat" {
		payload, _ := json.Marshal(struct {
			Type    string                 `json:"type"`
			Payload *tools.PendingQuestion `json:"payload"`
		}{"question_user", q})
		if dc.Broker != nil {
			dc.Broker.SendJSON(string(payload))
		}
	} else if dc.Broker != nil {
		dc.Broker.Send("question_user", formatQuestionUserText(q))
	}

	select {
	case response := <-responseCh:
		b, _ := json.Marshal(response)
		return "Tool Output: " + string(b)
	case <-time.After(timeout):
		tools.CancelQuestion(sessionID)
		b, _ := json.Marshal(tools.QuestionResponse{Status: "timeout"})
		return "Tool Output: " + string(b)
	}
}

func formatQuestionUserText(q *tools.PendingQuestion) string {
	var b strings.Builder
	b.WriteString("Question: ")
	b.WriteString(q.Question)
	b.WriteString("\n\n")
	for i, opt := range q.Options {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(") ")
		b.WriteString(opt.Label)
		if strings.TrimSpace(opt.Description) != "" {
			b.WriteString(" - ")
			b.WriteString(opt.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nReply with a number (1, 2, 3...) to select.")
	if q.AllowFreeText {
		b.WriteString("\nOr type your answer freely.")
	}
	return b.String()
}

// dispatchComm handles webhook, skill, notification, email, discord, mission, and notes tool calls.
func dispatchComm(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
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
	plannerDB := dc.PlannerDB
	sqlConnectionsDB := dc.SQLConnectionsDB
	sqlConnectionPool := dc.SQLConnectionPool
	remoteHub := dc.RemoteHub
	historyMgr := dc.HistoryMgr
	isMaintenance := dc.IsMaintenance
	llmGuardian := dc.LLMGuardian
	sessionID := dc.SessionID
	coAgentRegistry := dc.CoAgentRegistry
	budgetTracker := dc.BudgetTracker
	handled := true

	result := func() string {
		switch tc.Action {
		case "invoke_tool":
			return dispatchInvokeTool(ctx, tc, dc)

		case "question_user":
			return dispatchQuestionUser(tc, dc)

		case "call_webhook":
			if !cfg.Webhooks.Enabled {
				return `Tool Output: {"status":"error","message":"Webhooks are disabled in the config. Set webhooks.enabled=true."}`
			}
			req := decodeCallWebhookArgs(tc)
			logger.Info("LLM requested webhook execution", "webhook_name", req.WebhookName)

			// Find the webhook by name
			var targetHook *config.OutgoingWebhook
			for _, w := range cfg.Webhooks.Outgoing {
				if strings.EqualFold(w.Name, req.WebhookName) {
					targetHook = &w
					break
				}
			}

			if targetHook == nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Webhook '%s' not found. Check the exact name of the webhook from your System Context."}`, req.WebhookName)
			}
			out, statusCode, err := tools.ExecuteOutgoingWebhook(ctx, *targetHook, req.Parameters)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to execute webhook: %v"}`, err)
			}

			// Provide simple response
			return fmt.Sprintf(`Tool Output: {"status":"success", "http_status_code": %d, "response": %q}`, statusCode, out)

		case "manage_outgoing_webhooks":
			if !cfg.Webhooks.Enabled {
				return `Tool Output: {"status":"error","message":"Webhooks are disabled in the config. Set webhooks.enabled=true."}`
			}
			req := decodeManageOutgoingWebhooksArgs(tc)
			if cfg.Webhooks.ReadOnly && req.Operation != "list" {
				return `Tool Output: {"status":"error","message":"Webhooks tool is set to Read-Only mode. Cannot modify."}`
			}
			return tools.ManageOutgoingWebhooks(req.Operation, req.ID, req.Name, req.Description, req.Method, req.URL, req.PayloadType, req.BodyTemplate, req.Headers, req.rawParameters(), cfg)

		case "list_skill_templates":
			logger.Info("LLM requested to list skill templates")
			templates := tools.AvailableSkillTemplates()
			type templateInfo struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Parameters  map[string]interface{} `json:"parameters"`
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
			req := decodeCreateSkillFromTemplateArgs(tc)
			if req.Template == "" {
				return "Tool Output: ERROR 'template' is required. Use list_skill_templates to see available templates."
			}
			if req.Name == "" {
				return "Tool Output: ERROR 'name' is required for the new skill."
			}

			result, err := tools.CreateSkillFromTemplate(
				cfg.Directories.SkillsDir,
				req.Template,
				req.Name,
				req.Description,
				req.URL,
				req.Dependencies,
				req.VaultKeys,
			)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR creating skill from template: %v", err)
			}

			// Provision dependencies immediately
			tools.ProvisionSkillDependencies(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, logger)

			// Persist optional documentation manual so future reuses (also after reset)
			// can rely on a canonical "how-to" attached to the skill.
			docNote := ""
			if strings.TrimSpace(req.Documentation) != "" {
				if mgr := tools.DefaultSkillManager(); mgr != nil {
					_ = mgr.SyncFromDisk()
					skills, _ := mgr.ListSkillsFiltered("", "", req.Name, nil)
					var skillID string
					for _, sk := range skills {
						if sk.Name == req.Name {
							skillID = sk.ID
							break
						}
					}
					if skillID != "" {
						if docErr := mgr.SetSkillDocumentation(skillID, req.Documentation, "agent"); docErr != nil {
							docNote = fmt.Sprintf(" (documentation NOT saved: %v)", docErr)
						} else {
							docNote = " (documentation saved)"
						}
					}
				}
			} else {
				docNote = " (NOTE: no documentation supplied; call set_skill_documentation next so the agent can reuse this skill correctly later)"
			}

			logger.Info("Skill created from template",
				"template", req.Template,
				"skill", req.Name,
				"has_documentation", req.Documentation != "",
			)
			return "Tool Output: " + result + docNote

		case "get_skill_documentation":
			name := firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name"))
			if name == "" {
				return "Tool Output: ERROR 'name' is required."
			}
			mgr := tools.DefaultSkillManager()
			if mgr == nil {
				return "Tool Output: ERROR Skill Manager is not available."
			}
			skills, err := mgr.ListSkillsFiltered("", "", name, nil)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR listing skills: %v", err)
			}
			var skillID string
			for _, sk := range skills {
				if sk.Name == name {
					skillID = sk.ID
					break
				}
			}
			if skillID == "" {
				return fmt.Sprintf("Tool Output: ERROR skill %q not found.", name)
			}
			content, err := mgr.GetSkillDocumentation(skillID)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR reading documentation: %v", err)
			}
			if strings.TrimSpace(content) == "" {
				return fmt.Sprintf("Tool Output: Skill %q has no documentation manual yet. Use set_skill_documentation to add one.", name)
			}
			return fmt.Sprintf("Tool Output: Documentation for skill %q:\n\n%s", name, content)

		case "set_skill_documentation":
			if !cfg.Agent.AllowPython {
				return "Tool Output: [PERMISSION DENIED] set_skill_documentation is disabled in Danger Zone settings (agent.allow_python: false)."
			}
			name := firstNonEmptyToolString(tc.Name, toolArgString(tc.Params, "name"))
			content := toolArgString(tc.Params, "documentation")
			if content == "" {
				content = toolArgString(tc.Params, "content")
			}
			if name == "" {
				return "Tool Output: ERROR 'name' is required."
			}
			if strings.TrimSpace(content) == "" {
				return "Tool Output: ERROR 'documentation' must contain Markdown content."
			}
			mgr := tools.DefaultSkillManager()
			if mgr == nil {
				return "Tool Output: ERROR Skill Manager is not available."
			}
			skills, err := mgr.ListSkillsFiltered("", "", name, nil)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR listing skills: %v", err)
			}
			var skillID string
			for _, sk := range skills {
				if sk.Name == name {
					skillID = sk.ID
					break
				}
			}
			if skillID == "" {
				return fmt.Sprintf("Tool Output: ERROR skill %q not found.", name)
			}
			if err := mgr.SetSkillDocumentation(skillID, content, "agent"); err != nil {
				return fmt.Sprintf("Tool Output: ERROR saving documentation: %v", err)
			}
			return fmt.Sprintf("Tool Output: Documentation for skill %q saved (%d bytes).", name, len(content))

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
			// Enrich each entry with has_documentation so the agent knows which
			// skills carry a Markdown manual it should consult before calling.
			docFlags := map[string]bool{}
			if mgr := tools.DefaultSkillManager(); mgr != nil {
				if entries, listErr := mgr.ListSkillsFiltered("", "", "", nil); listErr == nil {
					for _, e := range entries {
						docFlags[e.Name] = e.HasDocumentation
					}
				}
			}
			type listed struct {
				tools.SkillManifest
				HasDocumentation bool `json:"has_documentation"`
			}
			out := make([]listed, len(availableSkills))
			for i, sk := range availableSkills {
				out[i] = listed{SkillManifest: sk, HasDocumentation: docFlags[sk.Name]}
			}
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR serializing skills list: %v", err)
			}
			return fmt.Sprintf("Tool Output: Internal Skills Configuration (use get_skill_documentation for skills with has_documentation=true):\n%s", string(b))

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

			cleanSkillName := strings.TrimSuffix(skillName, ".py")
			if catalog := GetToolCatalogState(); catalog != nil {
				if entry, ok := catalog.Get(cleanSkillName); ok && entry.Kind != ToolKindSkill {
					return wrongToolKindForExecuteSkill(entry)
				}
			}
			// Unwrap skill_args if the LLM nested the actual parameters under that key.
			// e.g. {"skill_name": "ddg_search", "skill_args": {"query": "..."}} → {"query": "..."}
			if innerArgs, ok := args["skill_args"].(map[string]interface{}); ok && len(innerArgs) > 0 {
				args = innerArgs
			} else {
				// Clean up metadata keys that aren't real skill parameters
				cleanArgs := make(map[string]interface{}, len(args))
				metaKeys := map[string]bool{"skill_name": true, "skill": true, "name": true, "tool": true, "action": true}
				if strings.EqualFold(cleanSkillName, "git_backup_restore") {
					delete(metaKeys, "action")
				}
				for k, v := range args {
					if !metaKeys[k] {
						cleanArgs[k] = v
					}
				}
				args = cleanArgs
			}
			args = filterExecuteSkillArgs(cfg.Directories.SkillsDir, cleanSkillName, args)
			if result, ok := handleExecuteSkillBuiltinAction(ctx, dc, cleanSkillName, args); ok {
				return result
			}
			switch cleanSkillName {
			case "git_backup_restore":
				reqJSON, _ := json.Marshal(args)
				var req tools.GitBackupRequest
				if err := json.Unmarshal(reqJSON, &req); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"invalid request parameters: %v"}`, err)
				}
				return tools.ExecuteGit(cfg.Directories.WorkspaceDir, req)
			case "google_workspace":
				if !cfg.GoogleWorkspace.Enabled {
					return `Tool Output: {"status": "error", "message": "Google Workspace is not enabled. Enable it in Settings > Google Workspace."}`
				}
				req := decodeGoogleWorkspaceArgsFromMap(args)
				return "Tool Output: " + tools.ExecuteGoogleWorkspace(*cfg, vault, req.Operation, req.params())
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
			// Resolve tool bridge: if the skill declares internal_tools and the bridge is enabled,
			// compute the allowed intersection so the Python process can call native tools.
			bridgeTools := resolveSkillBridgeTools(cfg, cfg.Directories.SkillsDir, cleanSkillName)
			var bridgeURL, bridgeToken string
			if len(bridgeTools) > 0 {
				bridgeURL = toolBridgeURL(cfg)
				if tok, _ := agentInternalToken.Load().(string); tok != "" {
					bridgeToken = tok
				}
			}

			var res string
			var skillErr error
			if cfg.Tools.SkillManager.RequireSandbox {
				// Sandbox enforcement: skills must run in the container sandbox
				if !tools.GetSandboxManager().IsReady() {
					return fmt.Sprintf("Tool Output: [SANDBOX REQUIRED] Skill '%s' requires sandbox execution but the sandbox is not available. Please enable the sandbox in settings (sandbox.enabled: true) or disable 'Require Sandbox' in skill manager settings.", skillName)
				}
				res, skillErr = tools.ExecuteSkillInSandbox(cfg.Directories.SkillsDir, cleanSkillName, args, secrets, creds, cfg.Tools.SkillTimeoutSeconds, logger, bridgeURL, bridgeToken, bridgeTools)
			} else if len(secrets) > 0 || len(creds) > 0 || len(bridgeTools) > 0 {
				res, skillErr = tools.ExecuteSkillWithSecrets(ctx, cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, skillName, args, secrets, creds, bridgeURL, bridgeToken, bridgeTools)
			} else {
				res, skillErr = tools.ExecuteSkill(ctx, cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, skillName, args)
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
			req := decodeFollowUpArgs(tc)
			logger.Info("LLM requested follow-up", "prompt", req.TaskPrompt)
			if req.TaskPrompt == "" {
				return "Tool Output: ERROR 'task_prompt' is required for follow_up"
			}
			if !cfg.Agent.BackgroundTasks.Enabled {
				return "Tool Output: ERROR background tasks are disabled in config (agent.background_tasks.enabled=false)."
			}

			// Guard: follow_up must describe work for the agent to do autonomously.
			// It must NEVER be used to relay a question back to the user — that causes
			// an infinite loop where each invocation re-asks the same question.
			trimmedPrompt := strings.TrimSpace(req.TaskPrompt)
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
			if req.DelaySeconds > 0 {
				delay = time.Duration(req.DelaySeconds) * time.Second
			}
			timeout := time.Duration(cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds) * time.Second
			if req.TimeoutSecs > 0 {
				timeout = time.Duration(req.TimeoutSecs) * time.Second
			}
			task, err := bgMgr.ScheduleFollowUp(req.TaskPrompt, tools.BackgroundTaskScheduleOptions{
				Source:             "follow_up",
				Description:        "Autonomous follow-up",
				Delay:              delay,
				MaxRetries:         cfg.Agent.BackgroundTasks.MaxRetries,
				RetryDelay:         time.Duration(cfg.Agent.BackgroundTasks.RetryDelaySeconds) * time.Second,
				Timeout:            timeout,
				NotifyOnCompletion: req.NotifyOnCompletion,
			})
			if err != nil {
				logger.Error("Failed to schedule follow-up", "error", err)
				return fmt.Sprintf("Tool Output: ERROR failed to schedule follow_up: %v", err)
			}

			return fmt.Sprintf("Tool Output: Follow-up scheduled as background task %s. I will continue in the background after this message.", task.ID)

		case "wait_for_event":
			req := decodeWaitForEventArgs(tc)
			logger.Info("LLM requested wait_for_event", "event_type", req.EventType, "task_prompt", req.TaskPrompt)
			if !cfg.Agent.BackgroundTasks.Enabled {
				return "Tool Output: ERROR background tasks are disabled in config (agent.background_tasks.enabled=false)."
			}
			if req.EventType == "" {
				return "Tool Output: ERROR 'event_type' is required for wait_for_event"
			}
			bgMgr := tools.DefaultBackgroundTaskManager()
			if bgMgr == nil {
				logger.Error("Background task manager is unavailable for wait_for_event")
				return "Tool Output: ERROR background task manager is unavailable."
			}
			waitTimeout := cfg.Agent.BackgroundTasks.WaitDefaultTimeoutSecs
			if req.TimeoutSecs > 0 {
				waitTimeout = req.TimeoutSecs
			}
			pollInterval := cfg.Agent.BackgroundTasks.WaitPollIntervalSecs
			if req.IntervalSecs > 0 {
				pollInterval = req.IntervalSecs
			}
			payload := tools.WaitForEventTaskPayload{
				EventType:           req.EventType,
				TaskPrompt:          req.TaskPrompt,
				URL:                 req.URL,
				Host:                req.Host,
				Port:                req.Port,
				FilePath:            req.FilePath,
				PID:                 req.PID,
				PollIntervalSeconds: pollInterval,
				TimeoutSeconds:      waitTimeout,
			}
			desc := fmt.Sprintf("Wait for %s", req.EventType)
			task, err := bgMgr.ScheduleWaitForEvent(payload, tools.BackgroundTaskScheduleOptions{
				Source:             "wait_for_event",
				Description:        desc,
				MaxRetries:         cfg.Agent.BackgroundTasks.MaxRetries,
				RetryDelay:         time.Duration(cfg.Agent.BackgroundTasks.RetryDelaySeconds) * time.Second,
				Timeout:            time.Duration(cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds) * time.Second,
				NotifyOnCompletion: req.NotifyOnCompletion,
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
				PlannerDB:          plannerDB,
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
			req := decodeToolManualArgs(tc)
			logger.Info("LLM requested tool manual", "name", req.ToolName)
			if req.ToolName == "" {
				return "Tool Output: ERROR 'tool_name' is required"
			}

			// Fallback for LLMs getting creative with the manual name
			cleanName := strings.TrimSuffix(req.ToolName, ".md")
			cleanName = strings.TrimSuffix(cleanName, "_tool_manual")
			// Reject path traversal and separators in the manual name.
			if strings.ContainsAny(cleanName, "/\\") || strings.Contains(cleanName, "..") {
				return fmt.Sprintf("Tool Output: ERROR invalid tool name for manual lookup: '%s'", req.ToolName)
			}
			manualPath := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals", cleanName+".md")
			content, ok := prompts.ReadToolGuide(manualPath)
			if !ok {
				return fmt.Sprintf("Tool Output: ERROR could not read manual for '%s'", req.ToolName)
			}
			return fmt.Sprintf("Tool Output: [MANUAL FOR %s]\n%s", req.ToolName, content)

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
			req := decodeLifeboatHandoverArgs(tc)
			if isMaintenance {
				return "Tool Output: ERROR You are already in Lifeboat mode. Maintenance is active. Use 'exit_lifeboat' to return to the supervisor or 'execute_surgery' for code changes."
			}
			logger.Info("LLM requested lifeboat handover", "plan_len", len(req.TaskPrompt))
			return tools.InitiateLifeboatHandover(req.TaskPrompt, cfg)

		case "get_system_metrics", "system_metrics":
			req := decodeSystemMetricsArgs(tc)
			logger.Info("LLM requested system metrics", "target", req.Target)
			return "Tool Output: " + tools.GetSystemMetrics(req.Target)

		case "process_analyzer":
			req := decodeProcessAnalyzerArgs(tc)
			logger.Info("LLM requested process analysis", "operation", req.Operation, "name", req.Name, "pid", req.PID)
			return "Tool Output: " + tools.AnalyzeProcesses(req.Operation, req.Name, req.PID, req.Limit)

		case "web_capture":
			req := decodeWebCaptureArgs(tc)
			logger.Info("LLM requested web capture", "operation", req.Operation, "url", req.URL)
			return "Tool Output: " + tools.WebCapture(ctx, req.Operation, req.URL, req.Selector, req.FullPage, req.OutputDir)

		case "browser_automation":
			if !cfg.BrowserAutomation.Enabled || !cfg.Tools.BrowserAutomation.Enabled {
				return `Tool Output: {"status":"error","message":"browser_automation is disabled. Enable browser_automation.enabled=true and tools.browser_automation.enabled=true in config."}`
			}
			req := decodeBrowserAutomationArgs(tc)
			logger.Info("LLM requested browser automation", "operation", req.Operation, "session_id", req.SessionID, "url", req.URL)
			return "Tool Output: " + tools.ExecuteBrowserAutomation(ctx, cfg, tools.BrowserAutomationRequest{
				Operation:    req.Operation,
				SessionID:    req.SessionID,
				URL:          req.URL,
				Selector:     req.Selector,
				Text:         req.Text,
				Value:        req.Value,
				Key:          req.Key,
				WaitFor:      req.WaitFor,
				TimeoutMs:    req.TimeoutMs,
				OutputPath:   req.OutputPath,
				FullPage:     req.FullPage,
				FilePath:     req.FilePath,
				DownloadName: req.DownloadName,
				DOMSnippet:   req.DOMSnippet,
				MaxElements:  req.MaxElements,
			}, logger)

		case "space_agent":
			if !cfg.SpaceAgent.Enabled {
				return `Tool Output: {"status":"error","message":"space_agent is disabled. Enable space_agent.enabled=true in config."}`
			}
			req := decodeSpaceAgentArgs(tc)
			logger.Info("LLM requested Space Agent instruction", "session_id", req.SessionID)
			return "Tool Output: " + tools.ExecuteSpaceAgent(ctx, cfg, tools.SpaceAgentInstruction{
				Instruction: req.Instruction,
				Information: req.Information,
				SessionID:   req.SessionID,
			})

		case "virtual_desktop":
			logger.Info("LLM requested virtual desktop operation",
				"operation", tc.Operation,
				"path", firstNonEmptyToolString(toolArgString(tc.Params, "path"), toolArgString(tc.Params, "file_path")),
				"app_id", toolArgString(tc.Params, "app_id"),
				"widget_id", firstNonEmptyToolString(toolArgString(tc.Params, "widget_id"), toolArgString(tc.Params, "id")),
				"content_bytes", len([]byte(toolArgString(tc.Params, "content"))),
			)
			exec := tools.ExecuteVirtualDesktop(ctx, cfg, tc.Params)
			if exec.Event != nil && dc.Broker != nil {
				payload, _ := json.Marshal(struct {
					Type    string         `json:"type"`
					Payload *desktop.Event `json:"payload"`
				}{"virtual_desktop_event", exec.Event})
				dc.Broker.SendJSON(string(payload))
			}
			return "Tool Output: " + exec.Output

		case "office_document":
			logger.Info("LLM requested office document operation",
				"operation", tc.Operation,
				"path", firstNonEmptyToolString(toolArgString(tc.Params, "path"), toolArgString(tc.Params, "file_path")),
				"output_path", toolArgString(tc.Params, "output_path"),
			)
			exec := tools.ExecuteOfficeDocument(ctx, cfg, tc.Params)
			if exec.Event != nil && dc.Broker != nil {
				payload, _ := json.Marshal(struct {
					Type    string         `json:"type"`
					Payload *desktop.Event `json:"payload"`
				}{"virtual_desktop_event", exec.Event})
				dc.Broker.SendJSON(string(payload))
			}
			return "Tool Output: " + exec.Output

		case "office_workbook":
			logger.Info("LLM requested office workbook operation",
				"operation", tc.Operation,
				"path", firstNonEmptyToolString(toolArgString(tc.Params, "path"), toolArgString(tc.Params, "file_path")),
				"sheet", toolArgString(tc.Params, "sheet"),
				"cell", toolArgString(tc.Params, "cell"),
			)
			exec := tools.ExecuteOfficeWorkbook(ctx, cfg, tc.Params)
			if exec.Event != nil && dc.Broker != nil {
				payload, _ := json.Marshal(struct {
					Type    string         `json:"type"`
					Payload *desktop.Event `json:"payload"`
				}{"virtual_desktop_event", exec.Event})
				dc.Broker.SendJSON(string(payload))
			}
			return "Tool Output: " + exec.Output

		case "web_performance_audit":
			req := decodeWebPerformanceAuditArgs(tc)
			logger.Info("LLM requested web performance audit", "url", req.URL, "viewport", req.Viewport)
			return "Tool Output: " + tools.WebPerformanceAudit(ctx, req.URL, req.Viewport)

		case "network_ping":
			req := decodeNetworkPingArgs(tc)
			logger.Info("LLM requested network ping", "host", req.Host)
			return "Tool Output: " + tools.NetworkPing(req.Host, req.Count, req.Timeout, cfg.Server.UILanguage)

		case "detect_file_type":
			req := decodeDetectFileTypeArgs(tc)
			logger.Info("LLM requested file type detection", "path", req.FilePath)
			return "Tool Output: " + tools.DetectFileTypeInWorkspace(req.FilePath, req.Recursive, cfg)

		case "dns_lookup":
			req := decodeDNSLookupArgs(tc)
			logger.Info("LLM requested DNS lookup", "host", req.Host, "record_type", req.RecordType)
			return "Tool Output: " + tools.DNSLookup(req.Host, req.RecordType, cfg.Server.UILanguage)

		case "port_scanner":
			req := decodePortScannerArgs(tc)
			logger.Info("LLM requested port scan", "host", req.Host, "port_range", req.PortRange)
			return "Tool Output: " + tools.ScanPorts(ctx, req.Host, req.PortRange, req.TimeoutMs)

		case "site_crawler":
			req := decodeSiteCrawlerArgs(tc)
			logger.Info("LLM requested site crawler", "url", req.URL, "max_depth", req.MaxDepth, "max_pages", req.MaxPages)
			return "Tool Output: " + tools.ExecuteCrawler(req.URL, req.MaxDepth, req.MaxPages, req.AllowedDomains, req.Selector)

		case "whois_lookup":
			req := decodeWhoisLookupArgs(tc)
			logger.Info("LLM requested WHOIS lookup", "domain", req.domain())
			return "Tool Output: " + tools.WhoisLookup(req.domain(), req.IncludeRaw)

		case "site_monitor":
			if !cfg.Tools.WebScraper.Enabled {
				return `Tool Output: {"status":"error","message":"Site monitor is disabled. Enable web_scraper in config."}`
			}
			req := decodeSiteMonitorArgs(tc)
			logger.Info("LLM requested site monitor", "op", req.Operation, "url", req.URL, "monitor_id", req.MonitorID)
			return "Tool Output: " + tools.ExecuteSiteMonitor(cfg.SQLite.SiteMonitorPath, req.Operation, req.MonitorID, req.URL, req.Selector, req.Interval, req.Limit)

		case "form_automation":
			if !cfg.Tools.FormAutomation.Enabled {
				return `Tool Output: {"status":"error","message":"form_automation is disabled. Enable tools.form_automation.enabled in config."}`
			}
			if !cfg.Tools.WebCapture.Enabled {
				return `Tool Output: {"status":"error","message":"form_automation requires web_capture (headless browser). Enable tools.web_capture.enabled in config."}`
			}
			req := decodeFormAutomationArgs(tc)
			logger.Info("LLM requested form automation", "op", req.Operation, "url", req.URL)
			return "Tool Output: " + tools.ExecuteFormAutomation(req.Operation, req.URL, req.Fields, req.Selector, req.ScreenshotDir)

		case "upnp_scan":
			if !cfg.Tools.UPnPScan.Enabled {
				return `Tool Output: {"status":"error","message":"upnp_scan is disabled. Enable tools.upnp_scan.enabled in config."}`
			}
			req := decodeUPnPScanArgs(tc)
			logger.Info("LLM requested UPnP scan", "search_target", req.SearchTarget, "timeout_secs", req.TimeoutSecs, "auto_register", req.AutoRegister)
			scanResult := tools.ExecuteUPnPScan(req.SearchTarget, req.TimeoutSecs)
			if req.AutoRegister && inventoryDB != nil {
				scanResult = upnpAutoRegister(scanResult, inventoryDB, req.RegisterType, req.RegisterTags, req.OverwriteExisting, logger)
			}
			return "Tool Output: " + scanResult

		case "send_telegram", "send_notification", "notification_center", "send_push_notification", "web_push", "send_image", "send_audio", "send_video", "send_youtube_video", "send_document":
			res, _ := dispatchMessagingCases(ctx, tc, dc)
			return res

		case "manage_processes", "process_management":
			req := decodeManageProcessesArgs(tc)
			logger.Info("LLM requested process management", "op", req.Operation)
			return "Tool Output: " + tools.ManageProcesses(req.Operation, int32(req.PID), cfg.Server.UILanguage)

		case "register_device", "register_server":
			req := decodeRegisterDeviceArgs(tc)
			logger.Info("LLM requested device registration", "name", req.Hostname)
			tags := services.ParseTags(req.Tags)
			deviceType := req.DeviceType
			if deviceType == "" {
				deviceType = "server"
			}

			// If LLM hallucinated, putting IP in Hostname and leaving IPAddress empty:
			if req.IPAddress == "" && net.ParseIP(req.Hostname) != nil {
				req.IPAddress = req.Hostname
			}

			id, err := services.RegisterDevice(inventoryDB, vault, req.Hostname, deviceType, req.IPAddress, req.Port, req.Username, req.Password, req.PrivateKeyPath, req.Description, tags, req.MACAddress)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to register device: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Device registered successfully", "id": "%s"}`, id)

		case "wake_on_lan", "wake_device", "wol":
			if !cfg.Tools.WOL.Enabled {
				return `Tool Output: {"status": "error", "message": "Wake-on-LAN is disabled. Enable it via tools.wol.enabled in config.yaml."}`
			}
			req := decodeWakeOnLANArgs(tc)
			logger.Info("LLM requested Wake-on-LAN", "server_id", req.ServerID, "mac", req.MACAddress)

			mac := req.MACAddress
			if mac == "" && req.ServerID != "" && inventoryDB != nil {
				// Look up MAC from inventory
				device, err := inventory.GetDeviceByIDOrName(inventoryDB, req.ServerID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
				}
				mac = device.MACAddress
			}
			if mac == "" {
				return `Tool Output: {"status": "error", "message": "No MAC address available. Provide 'mac_address' or a 'server_id' with a registered MAC address."}`
			}

			if err := tools.SendWakeOnLAN(mac, req.IPAddress); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send WOL packet: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Wake-on-LAN magic packet sent to %s"}`, mac)

		case "pin_message":
			req := decodePinMessageArgs(tc)
			logger.Info("LLM requested message pinning", "id", req.ID, "pinned", req.Pinned)
			if req.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for pin_message"}`
			}
			// Try to parse ID as int64
			var msgID int64
			fmt.Sscanf(req.ID, "%d", &msgID)
			if msgID == 0 {
				return `Tool Output: {"status": "error", "message": "Invalid 'id' format"}`
			}

			err := shortTermMem.SetMessagePinned(msgID, req.Pinned)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to update SQLite: %v"}`, err)
			}
			if historyMgr != nil {
				_ = historyMgr.SetPinned(msgID, req.Pinned)
			}
			status := "pinned"
			if !req.Pinned {
				status = "unpinned"
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Message %d %s successfully."}`, msgID, status)

		case "fetch_email", "check_email", "send_email", "list_email_accounts":
			res, _ := dispatchEmailCases(ctx, tc, dc)
			return res

		case "send_discord", "fetch_discord", "list_discord_channels":
			res, _ := dispatchMessagingCases(ctx, tc, dc)
			return res

		case "manage_missions":
			if !cfg.Tools.Missions.Enabled {
				return `Tool Output: {"status":"error","message":"Missions are disabled. Set tools.missions.enabled=true in config.yaml."}`
			}
			req := decodeMissionArgs(tc)
			if cfg.Tools.Missions.ReadOnly {
				switch req.Operation {
				case "create", "add", "update", "edit", "delete", "remove", "run", "run_now", "execute":
					return `Tool Output: {"status":"error","message":"Missions are in read-only mode. Disable tools.missions.read_only to allow changes."}`
				}
			}
			logger.Info("LLM requested mission management", "op", req.Operation)
			if missionManagerV2 == nil {
				return `Tool Output: {"status": "error", "message": "Mission control storage not available"}`
			}

			switch req.Operation {
			case "list":
				missions := missionManagerV2.List()
				b, _ := json.Marshal(map[string]interface{}{"status": "success", "data": missions})
				return "Tool Output: " + string(b)

			case "create", "add":
				if req.Title == "" || req.Command == "" {
					return `Tool Output: {"status": "error", "message": "'title' (name) and 'command' (prompt) are required for create"}`
				}
				priorityStr := "medium"
				if req.Priority == 1 {
					priorityStr = "low"
				} else if req.Priority == 3 {
					priorityStr = "high"
				}
				m := &tools.MissionV2{
					Name:     req.Title,
					Prompt:   req.Command,
					Schedule: req.CronExpr,
					Priority: priorityStr,
					Locked:   req.Locked,
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
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "'id' is required for update"}`
				}
				existing, ok := missionManagerV2.Get(req.ID)
				if !ok {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Mission %s not found"}`, req.ID)
				}

				if req.Title != "" {
					existing.Name = req.Title
				}
				if req.Command != "" {
					existing.Prompt = req.Command
				}
				if req.CronExpr != "" {
					existing.Schedule = req.CronExpr
				}
				if req.Priority > 0 {
					if req.Priority == 1 {
						existing.Priority = "low"
					} else if req.Priority == 3 {
						existing.Priority = "high"
					} else {
						existing.Priority = "medium"
					}
				}
				// Only apply lock state changes if the LLM explicitly provides it,
				// though typical struct fields default to false if omitted.
				// Since we want to allow keeping existing state, we should check if it was provided in raw json
				if req.LockedProvided {
					existing.Locked = req.Locked
				}

				err := missionManagerV2.Update(req.ID, existing)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Mission updated"}`

			case "delete", "remove":
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "'id' is required for delete"}`
				}
				err := missionManagerV2.Delete(req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Mission deleted"}`

			case "run", "run_now":
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "'id' is required for run"}`
				}
				err := missionManagerV2.RunNow(req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Mission scheduled for immediate execution by the background task queue"}`

			case "history":
				if missionManagerV2 == nil {
					return `Tool Output: {"status":"error","message":"Mission control storage not available"}`
				}
				historyDB := missionManagerV2.GetHistoryDB()
				if historyDB == nil {
					return `Tool Output: {"status":"error","message":"Mission history not available"}`
				}
				filter := tools.MissionHistoryFilter{
					MissionID:   req.ID,
					Result:      req.HistoryResult,
					TriggerType: req.HistoryTriggerType,
					From:        req.HistoryFrom,
					To:          req.HistoryTo,
					Limit:       req.Limit,
				}
				if filter.Limit <= 0 {
					filter.Limit = 10
				}
				page, err := tools.QueryMissionHistory(historyDB, filter)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"History query failed: %v"}`, err)
				}
				return "Tool Output: " + tools.FormatMissionHistoryJSON(page)

			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, req.Operation)
			}

		case "manage_daemon":
			if !cfg.Tools.DaemonSkills.Enabled {
				return `Tool Output: {"status":"error","message":"Daemon skills are disabled. Set tools.daemon_skills.enabled=true in config.yaml."}`
			}
			daemonSupervisor := dc.DaemonSupervisor
			if daemonSupervisor == nil {
				return `Tool Output: {"status":"error","message":"Daemon supervisor not initialized"}`
			}
			op := tc.Operation
			skillID := tc.SkillID
			logger.Info("LLM requested daemon management", "op", op)
			switch op {
			case "list":
				states := daemonSupervisor.ListDaemons()
				b, _ := json.Marshal(map[string]interface{}{"status": "success", "count": len(states), "daemons": states})
				return "Tool Output: " + string(b)
			case "status":
				if skillID == "" {
					return `Tool Output: {"status":"error","message":"'skill_id' is required for status"}`
				}
				state, ok := daemonSupervisor.GetDaemonState(skillID)
				if !ok {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Daemon %q not found"}`, skillID)
				}
				b, _ := json.Marshal(map[string]interface{}{"status": "success", "daemon": state})
				return "Tool Output: " + string(b)
			case "start":
				if skillID == "" {
					return `Tool Output: {"status":"error","message":"'skill_id' is required for start"}`
				}
				if err := daemonSupervisor.StartDaemon(skillID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Daemon %s started"}`, skillID)
			case "stop":
				if skillID == "" {
					return `Tool Output: {"status":"error","message":"'skill_id' is required for stop"}`
				}
				if err := daemonSupervisor.StopDaemon(skillID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Daemon %s stopped"}`, skillID)
			case "reenable":
				if skillID == "" {
					return `Tool Output: {"status":"error","message":"'skill_id' is required for reenable"}`
				}
				if err := daemonSupervisor.ReenableDaemon(skillID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Daemon %s re-enabled"}`, skillID)
			case "refresh":
				if err := daemonSupervisor.RefreshSkills(); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				return `Tool Output: {"status":"success","message":"Daemon skill list refreshed from disk"}`
			default:
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown daemon operation: %s"}`, op)
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
			req := decodePlanManagementArgs(tc)
			logger.Info("LLM requested plan management", "op", req.Operation, "session_id", sessionID)
			if shortTermMem == nil {
				return `Tool Output: {"status":"error","message":"Plan storage not available"}`
			}
			switch req.Operation {
			case "create":
				if req.Title == "" {
					return `Tool Output: {"status":"error","message":"'title' is required for create"}`
				}
				plan, err := shortTermMem.CreatePlan(sessionID, req.Title, req.Description, req.Content, req.Priority, planTaskInputsFromItems(req.Items))
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan created","plan":%s}`, string(payload))
			case "list":
				status := req.Status
				if status == "" {
					status = "all"
				}
				plans, err := shortTermMem.ListPlans(sessionID, status, req.Limit, req.IncludeArchived)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plans)
				return fmt.Sprintf(`Tool Output: {"status":"success","count":%d,"plans":%s}`, len(plans), string(payload))
			case "get":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for get"}`
				}
				plan, err := shortTermMem.GetPlan(req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","plan":%s}`, string(payload))
			case "set_status":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for set_status"}`
				}
				plan, err := shortTermMem.SetPlanStatus(req.ID, req.Status, req.Content)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan status updated","plan":%s}`, string(payload))
			case "update_task":
				if req.ID == "" || req.TaskID == "" {
					return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for update_task"}`
				}
				plan, err := shortTermMem.UpdatePlanTask(req.ID, req.TaskID, req.Status, req.Result, req.Error)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task updated","plan":%s}`, string(payload))
			case "advance":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for advance"}`
				}
				plan, err := shortTermMem.AdvancePlan(req.ID, req.Result)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan advanced","plan":%s}`, string(payload))
			case "set_blocker":
				if req.ID == "" || req.TaskID == "" {
					return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for set_blocker"}`
				}
				reason := strings.TrimSpace(req.Reason)
				if reason == "" {
					reason = strings.TrimSpace(req.Content)
				}
				plan, err := shortTermMem.SetPlanTaskBlocker(req.ID, req.TaskID, reason)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task blocker set","plan":%s}`, string(payload))
			case "clear_blocker":
				if req.ID == "" || req.TaskID == "" {
					return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for clear_blocker"}`
				}
				note := strings.TrimSpace(req.Content)
				if note == "" {
					note = strings.TrimSpace(req.Reason)
				}
				plan, err := shortTermMem.ClearPlanTaskBlocker(req.ID, req.TaskID, note)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task blocker cleared","plan":%s}`, string(payload))
			case "append_note":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for append_note"}`
				}
				if err := shortTermMem.AppendPlanNote(req.ID, req.Content); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				plan, err := shortTermMem.GetPlan(req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Plan note appended","plan":%s}`, string(payload))
			case "attach_artifact":
				if req.ID == "" || req.TaskID == "" {
					return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for attach_artifact"}`
				}
				value := strings.TrimSpace(req.Content)
				if value == "" {
					value = strings.TrimSpace(req.FilePath)
				}
				if value == "" {
					value = strings.TrimSpace(req.URL)
				}
				plan, err := shortTermMem.AttachPlanTaskArtifact(req.ID, req.TaskID, memory.PlanArtifact{
					Type:  strings.TrimSpace(req.ArtifactType),
					Label: strings.TrimSpace(req.Label),
					Value: value,
				})
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Artifact attached","plan":%s}`, string(payload))
			case "split_task":
				if req.ID == "" || req.TaskID == "" {
					return `Tool Output: {"status":"error","message":"'id' and 'task_id' are required for split_task"}`
				}
				plan, err := shortTermMem.SplitPlanTask(req.ID, req.TaskID, planTaskInputsFromItems(req.Items))
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task split into subtasks","plan":%s}`, string(payload))
			case "reorder_tasks":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for reorder_tasks"}`
				}
				orderedTaskIDs := planTaskIDsFromItems(req.Items)
				plan, err := shortTermMem.ReorderPlanTasks(req.ID, orderedTaskIDs)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				payload, _ := json.Marshal(plan)
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Task order updated","plan":%s}`, string(payload))
			case "archive_completed":
				if req.ID != "" {
					plan, err := shortTermMem.ArchivePlan(req.ID)
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
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for delete"}`
				}
				if err := shortTermMem.DeletePlan(req.ID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":%q}`, err.Error())
				}
				return `Tool Output: {"status":"success","message":"Plan deleted"}`
			default:
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown plan operation: %s. Use create, list, get, update_task, advance, set_status, set_blocker, clear_blocker, append_note, attach_artifact, split_task, reorder_tasks, archive_completed, or delete"}`, req.Operation)
			}

		case "manage_notes", "notes", "todo":
			req := decodeNotesManagementArgs(tc)
			if !cfg.Tools.Notes.Enabled {
				return `Tool Output: {"status":"error","message":"Notes are disabled. Set tools.notes.enabled=true in config.yaml."}`
			}
			if cfg.Tools.Notes.ReadOnly {
				switch req.Operation {
				case "add", "update", "toggle", "delete":
					return `Tool Output: {"status":"error","message":"Notes are in read-only mode. Disable tools.notes.read_only to allow changes."}`
				}
			}
			logger.Info("LLM requested notes/todo management", "op", req.Operation)
			if shortTermMem == nil {
				return `Tool Output: {"status": "error", "message": "Notes storage not available"}`
			}
			switch req.Operation {
			case "add":
				if req.Title == "" {
					return `Tool Output: {"status": "error", "message": "'title' is required for add"}`
				}
				id, err := shortTermMem.AddNote(req.Category, req.Title, req.Content, req.Priority, req.DueDate)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note created", "id": %d}`, id)
			case "list":
				notes, err := shortTermMem.ListNotes(req.Category, req.Done)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "notes": %s}`, len(notes), memory.FormatNotesJSON(notes))
			case "update":
				if req.NoteID <= 0 {
					return `Tool Output: {"status": "error", "message": "'note_id' is required for update"}`
				}
				err := shortTermMem.UpdateNote(req.NoteID, req.Title, req.Content, req.Category, req.Priority, req.DueDate)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note %d updated"}`, req.NoteID)
			case "toggle":
				if req.NoteID <= 0 {
					return `Tool Output: {"status": "error", "message": "'note_id' is required for toggle"}`
				}
				newState, err := shortTermMem.ToggleNoteDone(req.NoteID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "note_id": %d, "done": %t}`, req.NoteID, newState)
			case "delete":
				if req.NoteID <= 0 {
					return `Tool Output: {"status": "error", "message": "'note_id' is required for delete"}`
				}
				err := shortTermMem.DeleteNote(req.NoteID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note %d deleted"}`, req.NoteID)
			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown notes operation: %s. Use add, list, update, toggle, or delete"}`, req.Operation)
			}

		case "manage_journal", "journal":
			req := decodeJournalManagementArgs(tc)
			if !cfg.Tools.Journal.Enabled {
				return `Tool Output: {"status":"error","message":"Journal is disabled. Set tools.journal.enabled=true in config.yaml."}`
			}
			if cfg.Tools.Journal.ReadOnly {
				switch req.Operation {
				case "add", "delete":
					return `Tool Output: {"status":"error","message":"Journal is in read-only mode. Disable tools.journal.read_only to allow changes."}`
				}
			}
			logger.Info("LLM requested journal management", "op", req.Operation)
			if shortTermMem == nil {
				return `Tool Output: {"status": "error", "message": "Journal storage not available"}`
			}
			switch req.Operation {
			case "add":
				if req.Title == "" {
					return `Tool Output: {"status": "error", "message": "'title' is required for add"}`
				}
				entryType := req.EntryType
				if entryType == "" {
					entryType = "reflection"
				}
				importance := req.Importance
				if importance < 1 || importance > 4 {
					importance = 2
				}
				tags := req.normalizedTags()
				id, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
					EntryType:     entryType,
					Title:         req.Title,
					Content:       req.Content,
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
				limit := req.Limit
				if limit <= 0 {
					limit = 20
				}
				var types []string
				if req.EntryType != "" {
					types = []string{req.EntryType}
				}
				entries, err := shortTermMem.GetJournalEntries(req.FromDate, req.ToDate, types, limit)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "entries": %s}`, len(entries), memory.FormatJournalEntriesJSON(entries))
			case "search":
				if req.Query == "" {
					return `Tool Output: {"status": "error", "message": "'query' is required for search"}`
				}
				limit := req.Limit
				if limit <= 0 {
					limit = 20
				}
				entries, err := shortTermMem.SearchJournalEntries(req.Query, limit)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "entries": %s}`, len(entries), memory.FormatJournalEntriesJSON(entries))
			case "delete":
				if req.EntryID <= 0 {
					return `Tool Output: {"status": "error", "message": "'entry_id' is required for delete"}`
				}
				err := shortTermMem.DeleteJournalEntry(req.EntryID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Journal entry %d deleted"}`, req.EntryID)
			case "get_summary":
				date := req.FromDate
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
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown journal operation: %s. Use add, list, search, delete, or get_summary"}`, req.Operation)
			}

		case "telnyx_sms", "telnyx_call", "telnyx_manage":
			res, _ := dispatchMessagingCases(ctx, tc, dc)
			return res

		case "address_book":
			req := decodeAddressBookArgs(tc)
			if !cfg.Tools.Contacts.Enabled {
				return `Tool Output: {"status":"error","message":"Address book is disabled. Enable tools.contacts.enabled in config."}`
			}
			if contactsDB == nil {
				return `Tool Output: {"status":"error","message":"Contacts database not available."}`
			}
			logger.Info("LLM requested address book operation", "op", req.Operation)
			switch req.Operation {
			case "list":
				list, err := contacts.List(contactsDB, "")
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				return "Tool Output: " + contacts.ToJSON(map[string]interface{}{"status": "success", "contacts": list, "count": len(list)})
			case "search":
				if req.Query == "" {
					return `Tool Output: {"status":"error","message":"'query' is required for search operation"}`
				}
				list, err := contacts.List(contactsDB, req.Query)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				return "Tool Output: " + contacts.ToJSON(map[string]interface{}{"status": "success", "contacts": list, "count": len(list)})
			case "add":
				if req.Name == "" {
					return `Tool Output: {"status":"error","message":"'name' is required to add a contact"}`
				}
				c := contacts.Contact{
					Name:         req.Name,
					Email:        req.Email,
					Phone:        req.Phone,
					Mobile:       req.Mobile,
					Address:      req.ContactAddress,
					Relationship: req.Relationship,
					Notes:        req.Notes,
				}
				id, err := contacts.Create(contactsDB, c)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Contact created","id":"%s"}`, id)
			case "update":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for update operation"}`
				}
				existing, err := contacts.GetByID(contactsDB, req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				if req.Name != "" {
					existing.Name = req.Name
				}
				if req.Email != "" {
					existing.Email = req.Email
				}
				if req.Phone != "" {
					existing.Phone = req.Phone
				}
				if req.Mobile != "" {
					existing.Mobile = req.Mobile
				}
				if req.ContactAddress != "" {
					existing.Address = req.ContactAddress
				}
				if req.Relationship != "" {
					existing.Relationship = req.Relationship
				}
				if req.Notes != "" {
					existing.Notes = req.Notes
				}
				if err := contacts.Update(contactsDB, *existing); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Contact updated","id":"%s"}`, req.ID)
			case "delete":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for delete operation"}`
				}
				if err := contacts.Delete(contactsDB, req.ID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","message":"Contact deleted","id":"%s"}`, req.ID)
			default:
				return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, search, add, update, delete"}`
			}

		case "manage_appointments":
			if !cfg.Tools.Planner.Enabled {
				return `Tool Output: {"status":"error","message":"Planner is disabled. Enable tools.planner.enabled in config."}`
			}
			if plannerDB == nil {
				return `Tool Output: {"status":"error","message":"Planner database not available."}`
			}
			return dispatchManageAppointments(tc, plannerDB, kg, logger)

		case "manage_todos":
			if !cfg.Tools.Planner.Enabled {
				return `Tool Output: {"status":"error","message":"Planner is disabled. Enable tools.planner.enabled in config."}`
			}
			if plannerDB == nil {
				return `Tool Output: {"status":"error","message":"Planner database not available."}`
			}
			return dispatchManageTodos(tc, plannerDB, kg, logger)

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

		case "golangci_lint":
			if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
				return result
			}
			return unexpectedBuiltinActionError(tc.Action)

		case "brave_search":
			if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
				return result
			}
			return unexpectedBuiltinActionError(tc.Action)

		default:
			handled = false
			return ""
		}
	}()
	return result, handled
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
	if manifest == nil {
		return args
	}
	paramNames := tools.ExtractSkillParameterNames(manifest.Parameters)
	if len(paramNames) == 0 {
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

	filtered := make(map[string]interface{}, len(paramNames))
	for _, key := range paramNames {
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

func wrongToolKindForExecuteSkill(entry *ToolCatalogEntry) string {
	if entry == nil {
		return `Tool Output: {"status":"error","message":"not a skill"}`
	}
	switch entry.Kind {
	case ToolKindNative, ToolKindMCP:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s is a native AuraGo tool, not a Python skill. Use invoke_tool when it is hidden by adaptive filtering, or call it directly when active.","correct_action":"invoke_tool","tool_name":"%s","invoke_example":"{\"action\":\"invoke_tool\",\"tool_name\":\"%s\",\"arguments\":{...}}"}`, entry.Name, entry.Name, entry.Name)
	case ToolKindCustom:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s is a custom tool, not a Python skill. Use run_tool or invoke_tool.","correct_action":"run_tool","tool_name":"%s"}`, entry.Name, entry.Name)
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s is not executable via execute_skill"}`, entry.Name)
	}
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
	case "golangci_lint":
		return cfg.GolangciLint.Enabled
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
