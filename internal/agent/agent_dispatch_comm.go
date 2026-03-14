package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"
)

// dispatchComm handles webhook, skill, notification, email, discord, mission, and notes tool calls.
func dispatchComm(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManager *tools.MissionManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
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
		if len(skills) == 0 {
			return "Tool Output: No internal skills found."
		}
		b, err := json.MarshalIndent(skills, "", "  ")
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
			return tools.ExecuteWikipediaSearch(queryStr, langStr)
		case "ddg_search":
			queryStr, _ := args["query"].(string)
			maxRes, ok := args["max_results"].(float64)
			if !ok {
				maxRes = 5
			}
			return tools.ExecuteDDGSearch(queryStr, int(maxRes))
		case "virustotal_scan":
			if !cfg.VirusTotal.Enabled {
				return `Tool Output: {"status": "error", "message": "VirusTotal integration is not enabled. Set virustotal.enabled=true in config.yaml."}`
			}
			resource, _ := args["resource"].(string)
			return tools.ExecuteVirusTotalScan(cfg.VirusTotal.APIKey, resource)
		case "brave_search":
			if !cfg.BraveSearch.Enabled {
				return `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Set brave_search.enabled=true in config.yaml."}`
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
		logger.Info("LLM requested system metrics")
		return "Tool Output: " + tools.GetSystemMetrics()

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
		priority := tc.Tag // reuse existing Tag field for priority
		return "Tool Output: " + tools.SendNotification(cfg, logger, tc.Channel, tc.Title, tc.Message, priority, discordSend)

	case "send_image":
		logger.Info("LLM requested image send", "path", tc.Path, "caption", tc.Caption)
		return handleSendImage(tc, cfg, logger)

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
					messages[i].Body = guardian.SanitizeToolOutput("email", messages[i].Body)
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
		if missionManager == nil {
			return `Tool Output: {"status": "error", "message": "Mission control storage not available"}`
		}

		switch tc.Operation {
		case "list":
			missions := missionManager.List()
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
			m := tools.Mission{
				Name:     tc.Title,
				Prompt:   tc.Command,
				Schedule: tc.CronExpr,
				Priority: priorityStr,
				Locked:   tc.Locked,
			}
			err := missionManager.Create(m)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "message": "Mission created"})
			return "Tool Output: " + string(b)

		case "update", "edit":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for update"}`
			}
			existing, ok := missionManager.Get(tc.ID)
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

			err := missionManager.Update(tc.ID, existing)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission updated"}`

		case "delete", "remove":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for delete"}`
			}
			err := missionManager.Delete(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission deleted"}`

		case "run", "run_now":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for run"}`
			}
			err := missionManager.RunNow(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission scheduled for immediate execution by the background task queue"}`

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, tc.Operation)
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

	default:
		return dispatchNotHandled
	}
}
