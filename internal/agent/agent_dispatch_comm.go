package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/sqlconnections"
	"aurago/internal/telnyx"
	"aurago/internal/tools"
)

// dispatchComm handles webhook, skill, notification, email, discord, mission, and notes tool calls.
func dispatchComm(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManagerV2 *tools.MissionManagerV2, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, contactsDB *sql.DB, sqlConnectionsDB *sql.DB, sqlConnectionPool *sqlconnections.ConnectionPool, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, llmGuardian *security.LLMGuardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
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
		args = filterExecuteSkillArgs(cfg.Directories.SkillsDir, cleanSkillName, args)
		switch cleanSkillName {
		case "web_scraper":
			if !cfg.Tools.WebScraper.Enabled {
				return "Tool Output: [PERMISSION DENIED] web_scraper is disabled in settings (tools.web_scraper.enabled: false)."
			}
			urlStr, _ := args["url"].(string)
			scraped := tools.ExecuteWebScraper(urlStr)

			// Summary mode: send scraped content to a separate LLM for
			// summarisation so the agent only receives a concise summary.
			// This saves tokens in the main model and prevents prompt
			// injection from external web content.
			if cfg.Tools.WebScraper.SummaryMode {
				searchQuery, _ := args["search_query"].(string)
				if searchQuery == "" {
					searchQuery = "general summary of the page content"
				}
				summary, err := tools.SummariseScrapedContent(ctx, cfg, logger, scraped, searchQuery)
				if err != nil {
					logger.Warn("web_scraper summary failed, returning raw content", "error", err)
				} else {
					scraped = summary
				}
			}
			return scraped
		case "wikipedia_search":
			queryStr, _ := args["query"].(string)
			langStr, _ := args["language"].(string)
			result := tools.ExecuteWikipediaSearch(queryStr, langStr)
			if cfg.Tools.Wikipedia.SummaryMode {
				searchQuery, _ := args["search_query"].(string)
				if searchQuery == "" {
					searchQuery = "summarise the key facts about: " + queryStr
				}
				summary, err := tools.SummariseContent(ctx, tools.SummaryLLMConfig{
					APIKey:  cfg.Tools.Wikipedia.SummaryAPIKey,
					BaseURL: cfg.Tools.Wikipedia.SummaryBaseURL,
					Model:   cfg.Tools.Wikipedia.SummaryModel,
				}, logger, result, searchQuery, "Wikipedia article")
				if err != nil {
					logger.Warn("wikipedia summary failed, returning raw content", "error", err)
				} else {
					result = summary
				}
			}
			return result
		case "ddg_search":
			queryStr, _ := args["query"].(string)
			maxRes, ok := args["max_results"].(float64)
			if !ok {
				maxRes = 5
			}
			result := tools.ExecuteDDGSearch(queryStr, int(maxRes))
			if cfg.Tools.DDGSearch.SummaryMode {
				searchQuery, _ := args["search_query"].(string)
				if searchQuery == "" {
					searchQuery = "synthesise the most relevant findings for: " + queryStr
				}
				summary, err := tools.SummariseContent(ctx, tools.SummaryLLMConfig{
					APIKey:  cfg.Tools.DDGSearch.SummaryAPIKey,
					BaseURL: cfg.Tools.DDGSearch.SummaryBaseURL,
					Model:   cfg.Tools.DDGSearch.SummaryModel,
				}, logger, result, searchQuery, "search results")
				if err != nil {
					logger.Warn("ddg_search summary failed, returning raw content", "error", err)
				} else {
					result = summary
				}
			}
			return result
		case "virustotal_scan":
			if !cfg.VirusTotal.Enabled {
				return `Tool Output: {"status": "error", "message": "VirusTotal integration is not enabled. Set virustotal.enabled=true in config.yaml."}`
			}
			resource, _ := args["resource"].(string)
			filePath, _ := args["file_path"].(string)
			if filePath == "" {
				filePath, _ = args["path"].(string)
			}
			mode, _ := args["mode"].(string)
			return tools.ExecuteVirusTotalScanWithOptions(cfg.VirusTotal.APIKey, tools.VirusTotalOptions{
				Resource: resource,
				FilePath: filePath,
				Mode:     mode,
			})
		case "brave_search":
			if !cfg.BraveSearch.Enabled {
				return `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Enable it in Settings \u203a Brave Search."}`
			}
			queryStr, _ := args["query"].(string)
			count, ok := args["count"].(float64)
			if !ok {
				count = 10
			}
			country, _ := args["country"].(string)
			if country == "" {
				country = cfg.BraveSearch.Country
			}
			lang, _ := args["lang"].(string)
			if lang == "" {
				lang = cfg.BraveSearch.Lang
			}
			return tools.ExecuteBraveSearch(cfg.BraveSearch.APIKey, queryStr, int(count), country, lang)
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
			result := tools.ExecutePDFExtract(cfg.Directories.WorkspaceDir, filePath)
			if cfg.Tools.PDFExtractor.SummaryMode {
				searchQuery, _ := args["search_query"].(string)
				if searchQuery == "" {
					searchQuery = "summarise the key content of this document"
				}
				summary, err := tools.SummariseContent(ctx, tools.SummaryLLMConfig{
					APIKey:  cfg.Tools.PDFExtractor.SummaryAPIKey,
					BaseURL: cfg.Tools.PDFExtractor.SummaryBaseURL,
					Model:   cfg.Tools.PDFExtractor.SummaryModel,
				}, logger, result, searchQuery, "PDF document")
				if err != nil {
					logger.Warn("pdf_extractor summary failed, returning raw content", "error", err)
				} else {
					result = summary
				}
			}
			return result
		case "paperless", "paperless_ngx":
			if !cfg.PaperlessNGX.Enabled {
				return `Tool Output: {"status": "error", "message": "Paperless-ngx integration is not enabled. Set paperless_ngx.enabled=true in config.yaml."}`
			}
			op, _ := args["operation"].(string)
			if cfg.PaperlessNGX.ReadOnly {
				switch op {
				case "upload", "post", "update", "patch", "delete", "rm":
					return `Tool Output: {"status":"error","message":"Paperless-ngx is in read-only mode. Disable paperless_ngx.readonly to allow changes."}`
				}
			}
			plCfg := tools.PaperlessConfig{
				URL:      cfg.PaperlessNGX.URL,
				APIToken: cfg.PaperlessNGX.APIToken,
			}
			docID, _ := args["document_id"].(string)
			query, _ := args["query"].(string)
			content, _ := args["content"].(string)
			title, _ := args["title"].(string)
			tagsStr, _ := args["tags"].(string)
			corrName, _ := args["name"].(string)
			category, _ := args["category"].(string)
			limitF, _ := args["limit"].(float64)
			switch op {
			case "search", "find", "query":
				logger.Info("LLM requested Paperless search (via skill)", "query", query)
				return "Tool Output: " + tools.PaperlessSearch(plCfg, query, tagsStr, corrName, category, int(limitF))
			case "get", "info":
				logger.Info("LLM requested Paperless get (via skill)", "document_id", docID)
				return "Tool Output: " + tools.PaperlessGet(plCfg, docID)
			case "download", "read", "content":
				logger.Info("LLM requested Paperless download (via skill)", "document_id", docID)
				return "Tool Output: " + tools.PaperlessDownload(plCfg, docID)
			case "upload", "post":
				logger.Info("LLM requested Paperless upload (via skill)", "title", title)
				return "Tool Output: " + tools.PaperlessUpload(plCfg, title, content, tagsStr, corrName, category)
			case "update", "patch":
				logger.Info("LLM requested Paperless update (via skill)", "document_id", docID)
				return "Tool Output: " + tools.PaperlessUpdate(plCfg, docID, title, tagsStr, corrName, category)
			case "delete", "rm":
				logger.Info("LLM requested Paperless delete (via skill)", "document_id", docID)
				return "Tool Output: " + tools.PaperlessDelete(plCfg, docID)
			case "list_tags", "tags":
				logger.Info("LLM requested Paperless list tags (via skill)")
				return "Tool Output: " + tools.PaperlessListTags(plCfg)
			case "list_correspondents", "correspondents":
				logger.Info("LLM requested Paperless list correspondents (via skill)")
				return "Tool Output: " + tools.PaperlessListCorrespondents(plCfg)
			case "list_document_types", "document_types":
				logger.Info("LLM requested Paperless list document types (via skill)")
				return "Tool Output: " + tools.PaperlessListDocumentTypes(plCfg)
			default:
				return `Tool Output: {"status": "error", "message": "Unknown paperless operation. Use: search, get, download, upload, update, delete, list_tags, list_correspondents, list_document_types"}`
			}
		}

		// Generic Python skill fallback — gate on AllowPython
		if !cfg.Agent.AllowPython {
			return fmt.Sprintf("Tool Output: [PERMISSION DENIED] Skill '%s' requires Python execution which is disabled (agent.allow_python: false).", skillName)
		}
		res, err := tools.ExecuteSkill(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, skillName, args)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR executing skill: %v\nOutput: %s", err, res)
		}
		return fmt.Sprintf("Tool Output: %s", res)

	case "follow_up":
		logger.Info("LLM requested follow-up", "prompt", tc.TaskPrompt)
		if tc.TaskPrompt == "" {
			return "Tool Output: ERROR 'task_prompt' is required for follow_up"
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

		// Trigger background follow-up request
		go func(prompt string, port int) {
			time.Sleep(2 * time.Second) // Let current response finish
			url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", port)

			payload := map[string]interface{}{
				"model":  "aurago",
				"stream": false,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
			}

			body, _ := json.Marshal(payload)
			req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
			if err != nil {
				logger.Error("Failed to create follow-up request", "error", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-FollowUp", "true")

			client := &http.Client{Timeout: 10 * time.Minute}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Follow-up request failed", "error", err)
				return
			}
			defer resp.Body.Close()
			logger.Info("Follow-up triggered successfully", "status", resp.Status)
		}(tc.TaskPrompt, cfg.Server.Port)

		return "Tool Output: Follow-up scheduled. I will continue in the background immediately after this message."

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
		// Guardian: scan each message body for injection attempts
		if guardian != nil {
			for i := range messages {
				combined := messages[i].From + " " + messages[i].Subject + " " + messages[i].Body
				scanRes := guardian.ScanForInjection(combined)
				if scanRes.Level >= security.ThreatHigh {
					logger.Warn("[Email] Guardian HIGH threat in message", "uid", messages[i].UID, "from", messages[i].From, "threat", scanRes.Level.String())
					messages[i].Body = "[REDACTED by Guardian — injection attempt detected]"
					messages[i].Subject = "[SANITIZED] " + messages[i].Subject
					messages[i].Snippet = "[REDACTED]"
				} else {
					// LLM Guardian: deeper content scan if regex didn't flag HIGH
					if llmGuardian != nil && cfg.LLMGuardian.ScanEmails {
						llmResult := llmGuardian.EvaluateContent(ctx, "email", combined)
						if llmResult.Decision == security.DecisionBlock {
							logger.Warn("[Email] LLM Guardian blocked email content", "uid", messages[i].UID, "from", messages[i].From, "reason", llmResult.Reason)
							messages[i].Body = "[REDACTED by LLM Guardian — " + llmResult.Reason + "]"
							messages[i].Subject = "[SANITIZED] " + messages[i].Subject
							messages[i].Snippet = "[REDACTED]"
						} else {
							messages[i].Body = guardian.SanitizeToolOutput("email", messages[i].Body)
						}
					} else {
						messages[i].Body = guardian.SanitizeToolOutput("email", messages[i].Body)
					}
				}
			}
		}
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
					msgs[i].Content = "[REDACTED by Guardian — injection attempt detected]"
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
		queryStr := tc.Query
		if queryStr == "" {
			queryStr, _ = tc.Params["query"].(string)
		}
		maxRes, ok := tc.Params["max_results"].(float64)
		if !ok {
			maxRes = 5
		}
		result := tools.ExecuteDDGSearch(queryStr, int(maxRes))
		if cfg.Tools.DDGSearch.SummaryMode {
			searchQuery, _ := tc.Params["search_query"].(string)
			if searchQuery == "" {
				searchQuery = "synthesise the most relevant findings for: " + queryStr
			}
			summary, err := tools.SummariseContent(ctx, tools.SummaryLLMConfig{
				APIKey:  cfg.Tools.DDGSearch.SummaryAPIKey,
				BaseURL: cfg.Tools.DDGSearch.SummaryBaseURL,
				Model:   cfg.Tools.DDGSearch.SummaryModel,
			}, logger, result, searchQuery, "search results")
			if err != nil {
				logger.Warn("ddg_search summary failed, returning raw content", "error", err)
			} else {
				result = summary
			}
		}
		return result

	case "web_scraper":
		if !cfg.Tools.WebScraper.Enabled {
			return "Tool Output: [PERMISSION DENIED] web_scraper is disabled in settings (tools.web_scraper.enabled: false)."
		}
		urlStr := tc.URL
		if urlStr == "" {
			urlStr, _ = tc.Params["url"].(string)
		}
		scraped := tools.ExecuteWebScraper(urlStr)
		if cfg.Tools.WebScraper.SummaryMode {
			searchQuery, _ := tc.Params["search_query"].(string)
			if searchQuery == "" {
				searchQuery = "general summary of the page content"
			}
			summary, err := tools.SummariseScrapedContent(ctx, cfg, logger, scraped, searchQuery)
			if err != nil {
				logger.Warn("web_scraper summary failed, returning raw content", "error", err)
			} else {
				scraped = summary
			}
		}
		return scraped

	case "wikipedia_search":
		queryStr := tc.Query
		if queryStr == "" {
			queryStr, _ = tc.Params["query"].(string)
		}
		langStr, _ := tc.Params["language"].(string)
		result := tools.ExecuteWikipediaSearch(queryStr, langStr)
		if cfg.Tools.Wikipedia.SummaryMode {
			searchQuery, _ := tc.Params["search_query"].(string)
			if searchQuery == "" {
				searchQuery = "summarise the key facts about: " + queryStr
			}
			summary, err := tools.SummariseContent(ctx, tools.SummaryLLMConfig{
				APIKey:  cfg.Tools.Wikipedia.SummaryAPIKey,
				BaseURL: cfg.Tools.Wikipedia.SummaryBaseURL,
				Model:   cfg.Tools.Wikipedia.SummaryModel,
			}, logger, result, searchQuery, "Wikipedia article")
			if err != nil {
				logger.Warn("wikipedia summary failed, returning raw content", "error", err)
			} else {
				result = summary
			}
		}
		return result

	case "virustotal_scan":
		if !cfg.VirusTotal.Enabled {
			return `Tool Output: {"status": "error", "message": "VirusTotal integration is not enabled. Set virustotal.enabled=true in config.yaml."}`
		}
		resource := tc.Resource
		if resource == "" {
			resource, _ = tc.Params["resource"].(string)
		}
		filePath := tc.FilePath
		if filePath == "" {
			filePath, _ = tc.Params["file_path"].(string)
		}
		if filePath == "" {
			filePath, _ = tc.Params["path"].(string)
		}
		if filePath == "" {
			filePath = tc.Path
		}
		mode := tc.Mode
		if mode == "" {
			mode, _ = tc.Params["mode"].(string)
		}
		return tools.ExecuteVirusTotalScanWithOptions(cfg.VirusTotal.APIKey, tools.VirusTotalOptions{
			Resource: resource,
			FilePath: filePath,
			Mode:     mode,
		})

	case "brave_search":
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

	filtered := make(map[string]interface{}, len(manifest.Parameters))
	for key := range manifest.Parameters {
		if v, ok := args[key]; ok && !isEmptySkillArgValue(v) {
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
