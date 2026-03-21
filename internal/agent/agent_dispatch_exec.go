package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// dispatchExec handles execution, memory, security, filesystem, API, remote, and scheduling tool calls.
func dispatchExec(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManagerV2 *tools.MissionManagerV2, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, contactsDB *sql.DB, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
	switch tc.Action {
	case "execute_sandbox":
		if !cfg.Sandbox.Enabled {
			return "Tool Output: [PERMISSION DENIED] execute_sandbox is disabled (sandbox.enabled: false)."
		}
		if tc.Code == "" {
			return "Tool Output: [EXECUTION ERROR] 'code' field is empty. Provide the source code to execute."
		}
		lang := tc.SandboxLang
		if lang == "" {
			lang = tc.Language
		}
		if lang == "" {
			lang = "python"
		}
		logger.Info("LLM requested sandbox execution", "language", lang, "code_len", len(tc.Code), "libraries", len(tc.Libraries))
		result, err := tools.SandboxExecuteCode(tc.Code, lang, tc.Libraries, cfg.Sandbox.TimeoutSeconds, logger)
		if err != nil {
			// Fall back to execute_python if sandbox not ready and Python is allowed
			if cfg.Agent.AllowPython && lang == "python" {
				logger.Warn("Sandbox execution failed, falling back to execute_python", "error", err)
				stdout, stderr, pyErr := tools.ExecutePython(tc.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
				var sb strings.Builder
				sb.WriteString("Tool Output (sandbox unavailable — ran via local Python):\n")
				if stdout != "" {
					sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
				}
				if stderr != "" {
					sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
				}
				if pyErr != nil {
					sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", pyErr))
				}
				return sb.String()
			}
			return fmt.Sprintf("Tool Output: [EXECUTION ERROR] sandbox: %v", err)
		}
		return "Tool Output:\n" + result

	case "execute_python":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] execute_python is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		logger.Info("LLM requested python execution", "code_len", len(tc.Code), "background", tc.Background)
		if tc.Code == "" {
			return "Tool Output: [EXECUTION ERROR] 'code' field is empty. You MUST provide Python source code in the 'code' field. Do NOT use execute_python for SSH or remote tasks — use query_inventory / execute_remote_shell instead."
		}
		if tc.Background {
			logger.Info("LLM requested background Python execution", "code_len", len(tc.Code))
			pid, err := tools.ExecutePythonBackground(tc.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: [EXECUTION ERROR] starting background process: %v", err)
			}
			return fmt.Sprintf("Tool Output: Process started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		logger.Debug("Executing Python (foreground)", "code_preview", Truncate(tc.Code, 300))
		logger.Info("LLM requested python execution", "code_len", len(tc.Code))
		stdout, stderr, err := tools.ExecutePython(tc.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", err))
		}
		return sb.String()

	case "execute_shell":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] execute_shell is disabled in Danger Zone settings (agent.allow_shell: false)."
		}
		// Block commands that attempt to read AURAGO_* environment variables (contains vault master key etc.)
		if isBlockedEnvRead(tc.Command) {
			logger.Warn("[Security] Blocked attempt to read sensitive environment variable", "command", tc.Command)
			return "Tool Output: [PERMISSION DENIED] Reading AURAGO_ environment variables via shell is not permitted."
		}
		logger.Info("LLM requested shell execution", "command", tc.Command, "background", tc.Background)
		if tc.Background {
			pid, err := tools.ExecuteShellBackground(tc.Command, cfg.Directories.WorkspaceDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: [EXECUTION ERROR] starting background shell process: %v", err)
			}
			return fmt.Sprintf("Tool Output: Shell process started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		stdout, stderr, err := tools.ExecuteShell(tc.Command, cfg.Directories.WorkspaceDir)
		stdout = security.Scrub(stdout)
		stderr = security.Scrub(stderr)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", err))
		}
		return sb.String()

	case "execute_sudo":
		if !cfg.Agent.SudoEnabled {
			return "Tool Output: [PERMISSION DENIED] execute_sudo is not enabled in config. Set agent.sudo_enabled: true and store the sudo password in the vault as 'sudo_password'."
		}
		if tc.Command == "" {
			return "Tool Output: [EXECUTION ERROR] 'command' is required for execute_sudo"
		}
		sudoPass, vaultErr := vault.ReadSecret("sudo_password")
		if vaultErr != nil || sudoPass == "" {
			return "Tool Output: [PERMISSION DENIED] sudo password not found in vault. Store it first: {\"action\": \"secrets_vault\", \"operation\": \"store\", \"key\": \"sudo_password\", \"value\": \"<password>\"}"
		}
		logger.Info("LLM requested sudo execution", "command", tc.Command)
		stdoutS, stderrS, errS := tools.ExecuteSudo(tc.Command, cfg.Directories.WorkspaceDir, sudoPass)
		stdoutS = security.Scrub(stdoutS)
		stderrS = security.Scrub(stderrS)

		var sbSudo strings.Builder
		sbSudo.WriteString("Tool Output:\n")
		if stdoutS != "" {
			sbSudo.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdoutS))
		}
		if stderrS != "" {
			sbSudo.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderrS))
		}
		if errS != nil {
			sbSudo.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", errS))
		}
		return sbSudo.String()

	case "install_package":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] install_package is disabled in Danger Zone settings (agent.allow_shell: false)."
		}
		logger.Info("LLM requested package installation", "package", tc.Package)
		if tc.Package == "" {
			return "Tool Output: [EXECUTION ERROR] 'package' is required for install_package"
		}
		stdout, stderr, err := tools.InstallPackage(tc.Package, cfg.Directories.WorkspaceDir)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", err))
		}
		return sb.String()

	case "save_tool":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] save_tool is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		logger.Info("LLM requested tool persistence", "name", tc.Name)
		if tc.Name == "" || tc.Code == "" {
			return "Tool Output: ERROR 'name' and 'code' are required for save_tool"
		}
		if err := manifest.SaveTool(cfg.Directories.ToolsDir, tc.Name, tc.Description, tc.Code); err != nil {
			return fmt.Sprintf("Tool Output: ERROR saving tool: %v", err)
		}
		return fmt.Sprintf("Tool Output: Tool '%s' saved and registered successfully.", tc.Name)

	case "list_tools":
		logger.Info("LLM requested to list tools")
		loaded, err := manifest.Load()
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR loading tool manifest: %v", err)
		}
		var sb strings.Builder
		if len(loaded) == 0 {
			sb.WriteString("Tool Output: No custom Python tools saved yet. Use 'save_tool' to create them.\n")
		} else {
			sb.WriteString("Tool Output: Saved Reusable Tools (Python):\n")
			for k, v := range loaded {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
			}
		}

		sb.WriteString("\n[NOTE] Core capabilities like 'filesystem', 'execute_python', 'core_memory', 'query_memory', 'execute_surgery' (Maintenance only) are built-in and always available. See your system prompt and 'get_tool_manual' for details.")
		return sb.String()

	case "run_tool":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] run_tool is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		// Intercept LLM confusing Skills for Tools
		toolPath := filepath.Join(cfg.Directories.ToolsDir, tc.Name)
		if _, err := os.Stat(toolPath); os.IsNotExist(err) {
			skillCheckName := tc.Name
			if !strings.HasSuffix(skillCheckName, ".py") {
				skillCheckName += ".py"
			}
			skillPath := filepath.Join(cfg.Directories.SkillsDir, skillCheckName)
			if _, err2 := os.Stat(skillPath); err2 == nil {
				skillBase := strings.TrimSuffix(skillCheckName, ".py")
				return fmt.Sprintf("Tool Output: ERROR '%s' is a registered SKILL, not a generic tool. You MUST use {\"action\": \"execute_skill\", \"skill\": \"%s\", \"skill_args\": {\"arg1\": \"val1\"}} (JSON object) instead.", tc.Name, skillBase)
			}
		}

		if tc.Background {
			logger.Info("LLM requested background tool execution", "name", tc.Name)
			pid, err := tools.RunToolBackground(tc.Name, tc.GetArgs(), cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR starting background tool: %v", err)
			}
			return fmt.Sprintf("Tool Output: Tool started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		logger.Info("LLM requested tool execution", "name", tc.Name)
		stdout, stderr, err := tools.RunTool(tc.Name, tc.GetArgs(), cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		return fmt.Sprintf("Tool Output:\nSTDOUT:\n%s\nSTDERR:\n%s\nERROR:\n%s\n", stdout, stderr, errStr)

	case "list_processes":
		logger.Info("LLM requested process list")
		list := registry.List()
		if len(list) == 0 {
			return "Tool Output: No active background processes."
		}
		var sb strings.Builder
		sb.WriteString("Tool Output: Active processes:\n")
		for _, p := range list {
			pid, _ := p["pid"].(int)
			started, _ := p["started"].(string)
			sb.WriteString(fmt.Sprintf("- PID: %d, Started: %s\n", pid, started))
		}
		return sb.String()

	case "stop_process":
		if !cfg.Tools.StopProcess.Enabled {
			return `Tool Output: {"status":"error","message":"stop_process is disabled. Set tools.stop_process.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested process stop", "pid", tc.PID)
		if err := registry.Terminate(tc.PID); err != nil {
			return fmt.Sprintf("Tool Output: ERROR stopping process %d: %v", tc.PID, err)
		}
		return fmt.Sprintf("Tool Output: Process %d stopped.", tc.PID)

	case "read_process_logs":
		logger.Info("LLM requested process logs", "pid", tc.PID)
		proc, ok := registry.Get(tc.PID)
		if !ok {
			return fmt.Sprintf("Tool Output: ERROR process %d not found", tc.PID)
		}
		return fmt.Sprintf("Tool Output: [LOGS for PID %d]\n%s", tc.PID, proc.ReadOutput())

	case "query_memory":
		if !cfg.Tools.Memory.Enabled {
			return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
		}
		searchContent := tc.Content
		if searchContent == "" {
			searchContent = tc.Query
		}
		if searchContent == "" {
			return `Tool Output: {"status": "error", "message": "'content' or 'query' (search query) is required"}`
		}
		perSourceLimit := tc.Limit
		if perSourceLimit <= 0 || perSourceLimit > 20 {
			perSourceLimit = 5
		}

		// Determine which sources to search
		sourceMap := map[string]bool{"vector_db": true, "knowledge_graph": true, "journal": true, "notes": true, "core_memory": true, "error_patterns": true}
		if len(tc.Sources) > 0 {
			sourceMap = map[string]bool{}
			for _, s := range tc.Sources {
				sourceMap[s] = true
			}
		}
		logger.Info("LLM requested multi-source memory search", "query", searchContent, "sources", tc.Sources, "limit", perSourceLimit)

		type sourceResult struct {
			Source string      `json:"source"`
			Count  int         `json:"count"`
			Data   interface{} `json:"data"`
		}
		var combined []sourceResult
		var errors []string

		// ── Vector DB (long-term memory) ──
		if sourceMap["vector_db"] && longTermMem != nil {
			results, _, err := longTermMem.SearchSimilar(searchContent, perSourceLimit)
			if err != nil {
				errors = append(errors, fmt.Sprintf("vector_db: %v", err))
			} else if len(results) > 0 {
				combined = append(combined, sourceResult{Source: "vector_db", Count: len(results), Data: results})
			}
		}

		// ── Knowledge Graph ──
		if sourceMap["knowledge_graph"] && kg != nil {
			kgResult := kg.SearchForContext(searchContent, perSourceLimit, 2000)
			if kgResult != "" && kgResult != "No matching entities found." {
				combined = append(combined, sourceResult{Source: "knowledge_graph", Count: 1, Data: kgResult})
			}
		}

		// ── Journal ──
		if sourceMap["journal"] && shortTermMem != nil {
			entries, err := shortTermMem.SearchJournalEntries(searchContent, perSourceLimit)
			if err != nil {
				errors = append(errors, fmt.Sprintf("journal: %v", err))
			} else if len(entries) > 0 {
				combined = append(combined, sourceResult{Source: "journal", Count: len(entries), Data: entries})
			}
		}

		// ── Notes ──
		if sourceMap["notes"] && shortTermMem != nil {
			notes, err := shortTermMem.SearchNotes(searchContent, perSourceLimit)
			if err != nil {
				errors = append(errors, fmt.Sprintf("notes: %v", err))
			} else if len(notes) > 0 {
				combined = append(combined, sourceResult{Source: "notes", Count: len(notes), Data: notes})
			}
		}

		// ── Core Memory ──
		if sourceMap["core_memory"] && shortTermMem != nil {
			facts, err := shortTermMem.GetCoreMemoryFacts()
			if err != nil {
				errors = append(errors, fmt.Sprintf("core_memory: %v", err))
			} else {
				// Filter core memory facts by query relevance (case-insensitive substring match)
				lowerQ := strings.ToLower(searchContent)
				var matched []memory.CoreMemoryFact
				for _, f := range facts {
					if strings.Contains(strings.ToLower(f.Fact), lowerQ) {
						matched = append(matched, f)
						if len(matched) >= perSourceLimit {
							break
						}
					}
				}
				if len(matched) > 0 {
					combined = append(combined, sourceResult{Source: "core_memory", Count: len(matched), Data: matched})
				}
			}
		}

		// ── Error Patterns ──
		if sourceMap["error_patterns"] && shortTermMem != nil {
			errPatterns, err := shortTermMem.GetFrequentErrors("", perSourceLimit)
			if err != nil {
				errors = append(errors, fmt.Sprintf("error_patterns: %v", err))
			} else {
				// Filter by query relevance
				lowerQ := strings.ToLower(searchContent)
				var matched []memory.ErrorPattern
				for _, ep := range errPatterns {
					if strings.Contains(strings.ToLower(ep.ToolName), lowerQ) || strings.Contains(strings.ToLower(ep.ErrorMessage), lowerQ) || strings.Contains(strings.ToLower(ep.Resolution), lowerQ) {
						matched = append(matched, ep)
						if len(matched) >= perSourceLimit {
							break
						}
					}
				}
				if len(matched) > 0 {
					combined = append(combined, sourceResult{Source: "error_patterns", Count: len(matched), Data: matched})
				}
			}
		}

		if len(combined) == 0 && len(errors) == 0 {
			return `Tool Output: {"status": "success", "message": "No matching memories found across any source."}`
		}

		response := map[string]interface{}{
			"status":  "success",
			"results": combined,
		}
		if len(errors) > 0 {
			response["errors"] = errors
		}
		b, err := json.Marshal(response)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize results: %v"}`, err)
		}
		return "Tool Output: " + string(b)

	case "memory_reflect":
		if !cfg.MemoryAnalysis.Enabled {
			return `Tool Output: {"status":"error","message":"Memory analysis is disabled. Enable memory_analysis.enabled in config."}`
		}
		scope := tc.Scope
		if scope == "" {
			scope = "recent"
		}
		logger.Info("LLM requested memory reflection", "scope", scope)
		result, err := generateMemoryReflection(ctx, cfg, logger, shortTermMem, kg, longTermMem, llmClient, scope)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Reflection failed: %v"}`, err)
		}
		resultJSON, _ := json.Marshal(result)
		return fmt.Sprintf(`Tool Output: {"status":"success","reflection":%s}`, string(resultJSON))

	case "manage_updates":
		if !cfg.Agent.AllowSelfUpdate {
			return "Tool Output: [PERMISSION DENIED] manage_updates is disabled in Danger Zone settings (agent.allow_self_update: false)."
		}
		logger.Info("LLM requested update management", "operation", tc.Operation)
		switch tc.Operation {
		case "check":
			installDir := filepath.Dir(cfg.ConfigPath)

			// Binary-only install: no .git directory → use GitHub Releases API
			if _, gitErr := os.Stat(filepath.Join(installDir, ".git")); os.IsNotExist(gitErr) {
				// Read installed version from .version file
				currentVer := "unknown"
				if vb, err := os.ReadFile(filepath.Join(installDir, ".version")); err == nil {
					currentVer = strings.TrimSpace(string(vb))
				}
				// Fetch latest release from GitHub
				type ghRelease struct {
					TagName string `json:"tag_name"`
				}
				httpClient := &http.Client{Timeout: 10 * time.Second}
				req, reqErr := http.NewRequest("GET", "https://api.github.com/repos/antibyte/AuraGo/releases/latest", nil)
				if reqErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to build request: %v"}`, reqErr)
				}
				req.Header.Set("User-Agent", "AuraGo-Agent/1.0")
				resp, fetchErr := httpClient.Do(req)
				if fetchErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to reach GitHub: %v"}`, fetchErr)
				}
				defer resp.Body.Close()
				var rel ghRelease
				if decErr := json.NewDecoder(resp.Body).Decode(&rel); decErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to parse GitHub response: %v"}`, decErr)
				}
				if currentVer != "unknown" && currentVer == rel.TagName {
					return fmt.Sprintf(`Tool Output: {"status":"success","update_available":false,"current_version":%q,"latest_version":%q,"message":"AuraGo is up to date."}`, currentVer, rel.TagName)
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","update_available":true,"current_version":%q,"latest_version":%q,"message":"Update available."}`, currentVer, rel.TagName)
			}

			// Git-based install
			_, err := runGitCommand(filepath.Dir(cfg.ConfigPath), "fetch", "origin", "main", "--quiet")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch updates: %v"}`, err)
			}

			countOut, err := runGitCommand(filepath.Dir(cfg.ConfigPath), "rev-list", "HEAD..origin/main", "--count")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to check update count: %v"}`, err)
			}
			countStr := strings.TrimSpace(string(countOut))
			count, _ := strconv.Atoi(countStr)

			if count == 0 {
				return `Tool Output: {"status": "success", "update_available": false, "message": "AuraGo is up to date."}`
			}

			logOut, _ := runGitCommand(filepath.Dir(cfg.ConfigPath), "log", "HEAD..origin/main", "--oneline", "-n", "10")

			return fmt.Sprintf(`Tool Output: {"status": "success", "update_available": true, "count": %d, "changelog": %q}`, count, string(logOut))

		case "install":
			logger.Warn("LLM requested update installation")
			updateScript := filepath.Join(filepath.Dir(cfg.ConfigPath), "update.sh")
			if _, err := os.Stat(updateScript); err != nil {
				return `Tool Output: {"status": "error", "message": "update.sh not found in application directory"}`
			}

			// Run ./update.sh --yes
			updateCmd := exec.Command("/bin/bash", "./update.sh", "--yes")
			updateCmd.Dir = filepath.Dir(cfg.ConfigPath)
			// Ensure environment is passed for update script too
			home, _ := os.UserHomeDir()
			if home != "" {
				updateCmd.Env = append(os.Environ(), "HOME="+home)
			}
			// Start update script. It will handle the rest, potentially killing this process.
			if err := updateCmd.Start(); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to start update script: %v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Update initiated. The system will restart and apply changes shortly."}`

		default:
			return `Tool Output: {"status": "error", "message": "Invalid operation. Use 'check' or 'install'."}`
		}

	case "archive_memory":
		if !cfg.Tools.MemoryMaintenance.Enabled {
			return `Tool Output: {"status":"error","message":"Memory maintenance is disabled. Set tools.memory_maintenance.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested memory archival", "id", tc.ID)
		return "Tool Output: " + runMemoryOrchestrator(tc, cfg, logger, llmClient, longTermMem, shortTermMem, kg)

	case "optimize_memory":
		if !cfg.Tools.MemoryMaintenance.Enabled {
			return `Tool Output: {"status":"error","message":"Memory maintenance is disabled. Set tools.memory_maintenance.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested memory optimization")
		return "Tool Output: " + runMemoryOrchestrator(tc, cfg, logger, llmClient, longTermMem, shortTermMem, kg)

	case "manage_knowledge", "knowledge_graph":
		if !cfg.Tools.KnowledgeGraph.Enabled {
			return `Tool Output: {"status":"error","message":"Knowledge graph is disabled. Set tools.knowledge_graph.enabled=true in config.yaml."}`
		}
		if cfg.Tools.KnowledgeGraph.ReadOnly {
			switch tc.Operation {
			case "add_node", "add_edge", "delete_node", "delete_edge", "optimize":
				return `Tool Output: {"status":"error","message":"Knowledge graph is in read-only mode. Disable tools.knowledge_graph.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested knowledge graph operation", "op", tc.Operation)
		// Phase 69: Route to actual KnowledgeGraph implementation
		switch tc.Operation {
		case "add_node":
			err := kg.AddNode(tc.ID, tc.Label, tc.Properties)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Node added to graph"}`
		case "add_edge":
			err := kg.AddEdge(tc.Source, tc.Target, tc.Relation, tc.Properties)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Edge added to graph"}`
		case "delete_node":
			err := kg.DeleteNode(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Node deleted"}`
		case "delete_edge":
			err := kg.DeleteEdge(tc.Source, tc.Target, tc.Relation)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Edge deleted"}`
		case "search":
			res := kg.Search(tc.Content)
			return fmt.Sprintf("Tool Output: %s", res)
		case "optimize":
			res := runMemoryOrchestrator(tc, cfg, logger, llmClient, longTermMem, shortTermMem, kg)
			return fmt.Sprintf("Tool Output: %s", res)

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown graph operation: %s"}`, tc.Operation)
		}

	case "manage_memory", "core_memory":
		if !cfg.Tools.Memory.Enabled {
			return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Memory.ReadOnly {
			switch tc.Operation {
			case "add", "store", "save", "set", "update", "delete", "remove", "reset_profile", "delete_profile_entry":
				return `Tool Output: {"status":"error","message":"Memory is in read-only mode. Disable tools.memory.read_only to allow changes."}`
			}
		}
		// Handle synonyms for 'fact'
		fact := tc.Fact
		if fact == "" {
			if tc.MemoryValue != "" {
				fact = tc.MemoryValue
			} else if tc.MemoryKey != "" {
				fact = tc.MemoryKey
			} else if tc.Value != "" {
				fact = tc.Value
			} else if tc.Content != "" {
				fact = tc.Content
			}
		}
		// When LLM uses separate key+value fields, combine into a meaningful fact (e.g. "agent_name: Nova")
		// Only for add/update, and only when key is a descriptive word (not a numeric ID)
		{
			op := strings.ToLower(tc.Operation)
			keyField := tc.Key
			if keyField == "" {
				keyField = tc.MemoryKey
			}
			if (op == "add" || op == "update") && keyField != "" && fact != "" && fact != keyField {
				if _, parseErr := strconv.ParseInt(keyField, 10, 64); parseErr != nil {
					// Key is not a numeric ID — prefix fact with key for context
					if !strings.HasPrefix(strings.ToLower(fact), strings.ToLower(keyField)+":") &&
						!strings.HasPrefix(strings.ToLower(fact), strings.ToLower(keyField)+" ") {
						fact = keyField + ": " + fact
					}
				}
			}
		}

		logger.Info("LLM requested core memory management", "op", tc.Operation, "fact", fact)
		if tc.Operation == "" {
			return `Tool Output: {"status": "error", "message": "'operation' is required for manage_memory"}`
		}

		// User Profile operations (sub-ops of manage_memory)
		switch tc.Operation {
		case "view_profile":
			entries, err := shortTermMem.GetProfileEntries("")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			if len(entries) == 0 {
				return `Tool Output: {"status": "success", "message": "No user profile data collected yet.", "entries": []}`
			}
			var sb strings.Builder
			sb.WriteString(`{"status":"success","entries":[`)
			for i, e := range entries {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf(`{"category":%q,"key":%q,"value":%q,"confidence":%d}`, e.Category, e.Key, e.Value, e.Confidence))
			}
			sb.WriteString(`]}`)
			return fmt.Sprintf("Tool Output: %s", sb.String())
		case "reset_profile":
			if err := shortTermMem.ResetUserProfile(); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "User profile has been completely reset."}`
		case "delete_profile_entry":
			cat := tc.Key
			key := tc.Value
			if cat == "" || key == "" {
				return `Tool Output: {"status": "error", "message": "'key' (category) and 'value' (key name) are required for delete_profile_entry"}`
			}
			if err := shortTermMem.DeleteProfileEntry(cat, key); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Profile entry %s/%s deleted."}`, cat, key)
		}

		var memID int64
		fmt.Sscanf(tc.ID, "%d", &memID)
		result, err := tools.ManageCoreMemory(tc.Operation, fact, memID, shortTermMem, cfg.Agent.CoreMemoryMaxEntries, cfg.Agent.CoreMemoryCapMode)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		return fmt.Sprintf("Tool Output: %s", result)

	case "cheatsheet":
		if cheatsheetDB == nil {
			return `Tool Output: {"status":"error","message":"Cheat sheet database is not available."}`
		}
		op := strings.ToLower(strings.TrimSpace(tc.Operation))
		if op == "" {
			return `Tool Output: {"status":"error","message":"'operation' is required (list, get, create, update, delete)."}`
		}
		switch op {
		case "list":
			sheets, err := tools.CheatsheetList(cheatsheetDB, true)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			type entry struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			list := make([]entry, len(sheets))
			for i, s := range sheets {
				list[i] = entry{ID: s.ID, Name: s.Name}
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(list), "cheatsheets": list})
			return fmt.Sprintf("Tool Output: %s", string(data))
		case "get":
			var sheet *tools.CheatSheet
			var err error
			if tc.ID != "" {
				sheet, err = tools.CheatsheetGet(cheatsheetDB, tc.ID)
			} else if tc.Name != "" {
				sheet, err = tools.CheatsheetGetByName(cheatsheetDB, tc.Name)
			} else {
				return `Tool Output: {"status":"error","message":"'id' or 'name' is required for get."}`
			}
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"cheat sheet not found: %v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "ok", "cheatsheet": sheet})
			return fmt.Sprintf("Tool Output: %s", string(data))
		case "create":
			if tc.Name == "" {
				return `Tool Output: {"status":"error","message":"'name' is required for create."}`
			}
			sheet, err := tools.CheatsheetCreate(cheatsheetDB, tc.Name, tc.Content, "agent")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Cheat sheet created.", "cheatsheet": sheet})
			return fmt.Sprintf("Tool Output: %s", string(data))
		case "update":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for update."}`
			}
			var namePtr, contentPtr *string
			var activePtr *bool
			if tc.Name != "" {
				namePtr = &tc.Name
			}
			if tc.Content != "" {
				contentPtr = &tc.Content
			}
			if tc.Active != nil {
				activePtr = tc.Active
			}
			sheet, err := tools.CheatsheetUpdate(cheatsheetDB, tc.ID, namePtr, contentPtr, activePtr)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Cheat sheet updated.", "cheatsheet": sheet})
			return fmt.Sprintf("Tool Output: %s", string(data))
		case "delete":
			if tc.ID == "" {
				return `Tool Output: {"status":"error","message":"'id' is required for delete."}`
			}
			if err := tools.CheatsheetDelete(cheatsheetDB, tc.ID); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
			}
			return `Tool Output: {"status":"ok","message":"Cheat sheet deleted."}`
		default:
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown cheatsheet operation: %s. Use list, get, create, update, or delete."}`, op)
		}

	case "get_secret", "secrets_vault":
		if !cfg.Tools.SecretsVault.Enabled {
			return `Tool Output: {"status":"error","message":"Secrets vault is disabled. Set tools.secrets_vault.enabled=true in config.yaml."}`
		}
		op := strings.TrimSpace(strings.ToLower(tc.Operation))
		if cfg.Tools.SecretsVault.ReadOnly && (op == "store" || op == "set" || tc.Action == "set_secret") {
			return `Tool Output: {"status":"error","message":"Secrets vault is in read-only mode. Disable tools.secrets_vault.read_only to allow changes."}`
		}
		if op == "store" || op == "set" || (tc.Action == "set_secret") {
			logger.Info("LLM requested secret storage", "key", tc.Key)
			if tc.Key == "" || tc.Value == "" {
				return `Tool Output: {"status": "error", "message": "'key' and 'value' are required for set_secret/store"}`
			}
			if isSystemSecret(tc.Key) {
				logger.Warn("LLM attempted to overwrite system-managed secret — access denied", "key", tc.Key)
				return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be overwritten via secrets_vault."}`
			}
			err := vault.WriteSecret(tc.Key, tc.Value)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Secret '%s' stored safely."}`, tc.Key)
		}

		// Default: read/list
		logger.Info("LLM requested secret retrieval", "key", tc.Key)
		if tc.Key == "" {
			// List available secret keys when no key is specified
			// Filter out system-managed keys — the agent must not know they exist
			keys, err := vault.ListKeys()
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			visibleKeys := keys[:0]
			for _, k := range keys {
				if !isSystemSecret(k) {
					visibleKeys = append(visibleKeys, k)
				}
			}
			b, mErr := json.Marshal(visibleKeys)
			if mErr != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize keys: %v"}`, mErr)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Stored secret keys (use get_secret with 'key' to retrieve a value)", "keys": %s}`, string(b))
		}
		// Block access to system-managed secrets
		if isSystemSecret(tc.Key) {
			logger.Warn("LLM attempted to read system-managed secret — access denied", "key", tc.Key)
			return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be retrieved via secrets_vault."}`
		}
		secret, err := vault.ReadSecret(tc.Key)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		// JSON-encode the secret value to prevent injection from special characters
		safeVal, _ := json.Marshal(secret)
		return fmt.Sprintf(`Tool Output: {"status": "success", "key": "%s", "value": %s}`, tc.Key, string(safeVal))

	case "set_secret":
		if !cfg.Tools.SecretsVault.Enabled {
			return `Tool Output: {"status":"error","message":"Secrets vault is disabled. Set tools.secrets_vault.enabled=true in config.yaml."}`
		}
		if cfg.Tools.SecretsVault.ReadOnly {
			return `Tool Output: {"status":"error","message":"Secrets vault is in read-only mode. Disable tools.secrets_vault.read_only to allow changes."}`
		}
		logger.Info("LLM requested secret storage", "key", tc.Key)
		if tc.Key == "" || tc.Value == "" {
			return `Tool Output: {"status": "error", "message": "'key' and 'value' are required for set_secret"}`
		}
		if isSystemSecret(tc.Key) {
			logger.Warn("LLM attempted to overwrite system-managed secret — access denied", "key", tc.Key)
			return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be overwritten via secrets_vault."}`
		}
		err := vault.WriteSecret(tc.Key, tc.Value)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Secret '%s' stored safely."}`, tc.Key)

	case "archive":
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		if !cfg.Agent.AllowFilesystemWrite && (strings.EqualFold(tc.Operation, "create") || strings.EqualFold(tc.Operation, "extract")) {
			return "Tool Output: [PERMISSION DENIED] archive create/extract operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		logger.Info("LLM requested archive operation", "op", tc.Operation, "path", fpath, "target_dir", tc.Destination)
		dest := tc.Destination
		if dest == "" {
			dest = tc.Dest
		}
		return "Tool Output: " + tools.ExecuteArchive(tc.Operation, fpath, dest, tc.SourceFiles, tc.Format)

	case "pdf_operations":
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		op := strings.ToLower(tc.Operation)
		if !cfg.Agent.AllowFilesystemWrite && (op == "merge" || op == "split" || op == "watermark" || op == "compress" || op == "encrypt" || op == "decrypt") {
			return "Tool Output: [PERMISSION DENIED] pdf_operations write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		output := tc.OutputFile
		if output == "" {
			output = tc.Destination
		}
		logger.Info("LLM requested PDF operation", "op", tc.Operation, "path", fpath)
		return "Tool Output: " + tools.ExecutePDFOperations(tc.Operation, fpath, output, tc.Pages, tc.Password, tc.WatermarkText, tc.SourceFiles)

	case "image_processing":
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		op := strings.ToLower(tc.Operation)
		if !cfg.Agent.AllowFilesystemWrite && op != "info" {
			return "Tool Output: [PERMISSION DENIED] image_processing write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
		}
		output := tc.OutputFile
		if output == "" {
			output = tc.Destination
		}
		logger.Info("LLM requested image processing", "op", tc.Operation, "path", fpath)
		return "Tool Output: " + tools.ExecuteImageProcessing(tc.Operation, fpath, output, tc.OutputFormat, tc.Width, tc.Height, tc.QualityPct, tc.CropX, tc.CropY, tc.CropWidth, tc.CropHeight, tc.Angle)

	case "filesystem", "filesystem_op":
		// Parameter robustness: handle 'path' and 'dest' aliases frequently hallucinated by LLMs
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		fdest := tc.Destination
		if fdest == "" {
			fdest = tc.Dest
		}

		op := strings.TrimSpace(strings.ToLower(tc.Operation))
		if op == "list" || op == "ls" {
			op = "list_dir"
		}

		// Block access to system-sensitive files (config, vault, databases, .env)
		wsDir := cfg.Directories.WorkspaceDir
		for _, checkPath := range []string{fpath, fdest} {
			if isProtectedSystemPath(checkPath, wsDir, cfg) {
				logger.Warn("LLM attempted filesystem access to protected system file — blocked",
					"op", op, "path", checkPath)
				return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
			}
		}

		if !cfg.Agent.AllowFilesystemWrite {
			writeOps := map[string]bool{"write": true, "write_file": true, "append": true, "delete": true, "remove": true, "move": true, "rename": true, "mkdir": true, "create_dir": true, "create": true}
			if writeOps[op] {
				return "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
			}
		}
		logger.Info("LLM requested filesystem operation", "op", op, "path", fpath, "dest", fdest)
		return tools.ExecuteFilesystem(op, fpath, fdest, tc.Content, cfg.Directories.WorkspaceDir)

	case "api_request":
		if !cfg.Agent.AllowNetworkRequests {
			return "Tool Output: [PERMISSION DENIED] api_request is disabled in Danger Zone settings (agent.allow_network_requests: false)."
		}
		logger.Info("LLM requested generic API request", "url", tc.URL)
		return tools.ExecuteAPIRequest(tc.Method, tc.URL, tc.Body, tc.Headers)

	case "koofr", "koofr_api", "koofr_op":
		if !cfg.Koofr.Enabled {
			return `Tool Output: {"status": "error", "message": "Koofr integration is not enabled. Set koofr.enabled=true in config.yaml."}`
		}
		if cfg.Koofr.ReadOnly {
			switch tc.Operation {
			case "write", "put", "upload", "mkdir", "delete", "rm", "move", "rename", "mv":
				return `Tool Output: {"status":"error","message":"Koofr is in read-only mode. Disable koofr.read_only to allow changes."}`
			}
		}
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		fdest := tc.Destination
		if fdest == "" {
			fdest = tc.Dest
		}
		logger.Info("LLM requested koofr operation", "op", tc.Operation, "path", fpath, "dest", fdest)
		koofrCfg := tools.KoofrConfig{
			BaseURL:     cfg.Koofr.BaseURL,
			Username:    cfg.Koofr.Username,
			AppPassword: cfg.Koofr.AppPassword,
		}
		return tools.ExecuteKoofr(koofrCfg, tc.Operation, fpath, fdest, tc.Content)

	case "google_workspace", "gworkspace":
		if !cfg.GoogleWorkspace.Enabled {
			return `Tool Output: {"status": "error", "message": "Google Workspace is not enabled. Enable it in Settings > Google Workspace."}`
		}
		op := tc.Operation
		if op == "" {
			op = tc.Action
		}
		logger.Info("LLM requested google_workspace operation", "op", op)
		params := make(map[string]interface{})
		if tc.Params != nil {
			params = tc.Params
		}
		// Map ToolCall fields into params for the dispatch function
		if tc.DocumentID != "" {
			params["document_id"] = tc.DocumentID
		}
		if tc.MessageID != "" {
			params["message_id"] = tc.MessageID
		}
		if tc.FileID != "" {
			params["file_id"] = tc.FileID
		}
		if tc.EventID != "" {
			params["event_id"] = tc.EventID
		}
		if tc.MaxResults > 0 {
			params["max_results"] = tc.MaxResults
		}
		if tc.Query != "" {
			params["query"] = tc.Query
		}
		if tc.To != "" {
			params["to"] = tc.To
		}
		if tc.Subject != "" {
			params["subject"] = tc.Subject
		}
		if tc.Body != "" {
			params["body"] = tc.Body
		}
		if tc.Title != "" {
			params["title"] = tc.Title
		}
		if tc.Description != "" {
			params["description"] = tc.Description
		}
		if tc.StartTime != "" {
			params["start_time"] = tc.StartTime
		}
		if tc.EndTime != "" {
			params["end_time"] = tc.EndTime
		}
		if tc.Range != "" {
			params["range"] = tc.Range
		}
		if len(tc.Values) > 0 {
			params["values"] = tc.Values
		}
		if len(tc.AddLabels) > 0 {
			params["add_labels"] = tc.AddLabels
		}
		if len(tc.RemoveLabels) > 0 {
			params["remove_labels"] = tc.RemoveLabels
		}
		return "Tool Output: " + tools.ExecuteGoogleWorkspace(*cfg, vault, op, params)

	case "onedrive", "onedrive_op":
		if !cfg.OneDrive.Enabled {
			return `Tool Output: {"status": "error", "message": "OneDrive integration is not enabled. Set onedrive.enabled=true in config.yaml."}`
		}
		op := tc.Operation
		if op == "" {
			op = tc.Action
		}
		if cfg.OneDrive.ReadOnly {
			switch op {
			case "upload", "write", "mkdir", "delete", "move", "copy", "share":
				return `Tool Output: {"status":"error","message":"OneDrive is in read-only mode. Disable onedrive.readonly to allow changes."}`
			}
		}
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		fdest := tc.Destination
		if fdest == "" {
			fdest = tc.Dest
		}
		logger.Info("LLM requested onedrive operation", "op", op, "path", fpath, "dest", fdest)
		client, err := tools.NewOneDriveClient(*cfg, vault)
		if err != nil {
			return "Tool Output: " + tools.ODErrJSON("OneDrive client error: %v", err)
		}
		return "Tool Output: " + client.ExecuteOneDrive(op, fpath, fdest, tc.Content, tc.MaxResults)

	case "generate_image":
		if !cfg.ImageGeneration.Enabled {
			return `Tool Output: {"status": "error", "message": "Image generation is not enabled. Enable it in Settings > Image Generation."}`
		}
		if cfg.ImageGeneration.APIKey == "" {
			return `Tool Output: {"status": "error", "message": "Image generation provider not configured. Set a provider in Settings > Image Generation."}`
		}
		prompt := tc.Prompt
		if prompt == "" {
			prompt = tc.Content
		}
		if prompt == "" {
			return `Tool Output: {"status": "error", "message": "'prompt' is required for image generation."}`
		}
		logger.Info("LLM requested image generation", "prompt_len", len(prompt), "provider", cfg.ImageGeneration.ProviderType)

		// Check budget
		if budgetTracker != nil && budgetTracker.IsBlocked("image_generation") {
			return `Tool Output: {"status": "error", "message": "Image generation blocked: daily budget exceeded."}`
		}

		// Check monthly limit
		if cfg.ImageGeneration.MaxMonthly > 0 {
			count, err := tools.ImageGalleryMonthlyCount(imageGalleryDB)
			if err == nil && count >= cfg.ImageGeneration.MaxMonthly {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Monthly image generation limit reached (%d/%d). Try again next month or increase the limit in settings."}`, count, cfg.ImageGeneration.MaxMonthly)
			}
		}

		// Prompt enhancement
		enhancedPrompt := ""
		doEnhance := cfg.ImageGeneration.PromptEnhancement
		if tc.EnhancePrompt != nil {
			doEnhance = *tc.EnhancePrompt
		}
		effectivePrompt := prompt
		if doEnhance {
			enhanced, err := tools.EnhanceImagePrompt(llmClient, cfg.LLM.Model, prompt)
			if err != nil {
				logger.Warn("Image prompt enhancement failed, using original", "error", err)
			} else {
				enhancedPrompt = enhanced
				effectivePrompt = enhanced
			}
		}

		// Build config
		genCfg := tools.ImageGenConfig{
			ProviderType: cfg.ImageGeneration.ProviderType,
			BaseURL:      cfg.ImageGeneration.BaseURL,
			APIKey:       cfg.ImageGeneration.APIKey,
			Model:        cfg.ImageGeneration.ResolvedModel,
			DataDir:      cfg.Directories.DataDir,
		}
		if tc.Model != "" {
			genCfg.Model = tc.Model
		}

		opts := tools.ImageGenOptions{
			Size:    tc.Size,
			Quality: tc.Quality,
			Style:   tc.Style,
		}
		if opts.Size == "" {
			opts.Size = cfg.ImageGeneration.DefaultSize
		}
		if opts.Quality == "" {
			opts.Quality = cfg.ImageGeneration.DefaultQuality
		}
		if opts.Style == "" {
			opts.Style = cfg.ImageGeneration.DefaultStyle
		}
		if tc.SourceImage != "" {
			opts.SourceImage = tools.ResolveSourceImagePath(tc.SourceImage, cfg.Directories.WorkspaceDir, cfg.Directories.DataDir)
		}

		result, err := tools.GenerateImage(genCfg, effectivePrompt, opts)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Image generation failed: %s"}`, err.Error())
		}

		result.Prompt = prompt
		result.EnhancedPrompt = enhancedPrompt

		// Save to gallery DB
		tools.SaveGeneratedImage(imageGalleryDB, result)

		// Auto-register in media registry
		if mediaRegistryDB != nil {
			tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
				MediaType:        "image",
				SourceTool:       "generate_image",
				Filename:         result.Filename,
				FilePath:         result.Filename,
				WebPath:          result.WebPath,
				Format:           "png",
				Provider:         result.Provider,
				Model:            result.Model,
				Prompt:           result.Prompt,
				Quality:          result.Quality,
				Style:            result.Style,
				Size:             result.Size,
				SourceImage:      result.SourceImage,
				GenerationTimeMs: int64(result.DurationMs),
				CostEstimate:     result.CostEstimate,
				Tags:             []string{"auto-generated"},
			})
		}

		// Record cost in budget tracker
		if budgetTracker != nil && result.CostEstimate > 0 {
			budgetTracker.RecordCost(result.CostEstimate)
		}

		resultJSON, _ := json.Marshal(map[string]interface{}{
			"status":          "success",
			"web_path":        result.WebPath,
			"markdown":        result.Markdown,
			"prompt":          result.Prompt,
			"enhanced_prompt": result.EnhancedPrompt,
			"model":           result.Model,
			"provider":        result.Provider,
			"size":            result.Size,
			"duration_ms":     result.DurationMs,
		})
		return "Tool Output: " + string(resultJSON)

	case "query_inventory":
		queryTag := tc.Tag
		if queryTag == "" {
			queryTag = tc.Tags
		}
		logger.Info("LLM requested inventory query", "tag", queryTag, "name", tc.Hostname)
		devices, err := inventory.QueryDevices(inventoryDB, queryTag, tc.DeviceType, tc.Hostname)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to query inventory: %v"}`, err)
		}
		b, mErr := json.Marshal(devices)
		if mErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize devices: %v"}`, mErr)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "tag": "%s", "device_type": "%s", "name_match": "%s", "devices": %s}`, tc.Tag, tc.DeviceType, tc.Hostname, string(b))

	case "execute_remote_shell", "remote_execution":
		if !cfg.Agent.AllowRemoteShell {
			return "Tool Output: [PERMISSION DENIED] execute_remote_shell is disabled in Danger Zone settings (agent.allow_remote_shell: false)."
		}
		logger.Info("LLM requested remote shell execution", "server_id", tc.ServerID, "command", tc.Command)
		if tc.ServerID == "" || tc.Command == "" {
			return `Tool Output: {"status": "error", "message": "'server_id' and 'command' are required"}`
		}
		device, err := inventory.GetDeviceByIDOrName(inventoryDB, tc.ServerID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
		}
		secret, err := vault.ReadSecret(device.VaultSecretID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch secret: %v"}`, err)
		}
		rCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		output, err := remote.ExecuteRemoteCommand(rCtx, device.Name, device.Port, device.Username, []byte(secret), tc.Command)
		if err != nil {
			safeOutput, mErr := json.Marshal(output)
			if mErr != nil {
				safeOutput = []byte(`""`)
			}
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Remote execution failed", "output": %s, "error": "%v"}`, string(safeOutput), err)
		}
		safeOutput, mErr := json.Marshal(output)
		if mErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize output: %v"}`, mErr)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "output": %s}`, string(safeOutput))

	case "transfer_remote_file":
		if !cfg.Agent.AllowRemoteShell {
			return "Tool Output: [PERMISSION DENIED] transfer_remote_file is disabled in Danger Zone settings (agent.allow_remote_shell: false)."
		}
		logger.Info("LLM requested remote file transfer", "server_id", tc.ServerID, "direction", tc.Direction)
		if tc.ServerID == "" || tc.Direction == "" || tc.LocalPath == "" || tc.RemotePath == "" {
			return `Tool Output: {"status": "error", "message": "'server_id', 'direction', 'local_path', and 'remote_path' are required"}`
		}
		// Sanitize and restrict local path
		absLocal, err := filepath.Abs(tc.LocalPath)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Invalid local path: %v"}`, err)
		}
		workspaceWorkdir := filepath.Join(cfg.Directories.WorkspaceDir, "workdir")
		if !strings.HasPrefix(strings.ToLower(absLocal), strings.ToLower(workspaceWorkdir)) {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Permission denied: local_path must be within %s"}`, workspaceWorkdir)
		}

		device, err := inventory.GetDeviceByIDOrName(inventoryDB, tc.ServerID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
		}
		secret, err := vault.ReadSecret(device.VaultSecretID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch secret: %v"}`, err)
		}
		rCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		err = remote.TransferFile(rCtx, device.Name, device.Port, device.Username, []byte(secret), absLocal, tc.RemotePath, tc.Direction)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "File transfer failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "File %s successfully"}`, tc.Direction)

	case "manage_schedule", "cron_scheduler":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Scheduler.ReadOnly {
			switch tc.Operation {
			case "add", "remove":
				return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested cron management", "operation", tc.Operation)
		result, err := cronManager.ManageSchedule(tc.Operation, tc.ID, tc.CronExpr, tc.TaskPrompt)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR in manage_schedule: %v", err)
		}
		return result

	case "schedule_cron":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Scheduler.ReadOnly {
			return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
		}
		logger.Info("LLM requested cron scheduling", "expr", tc.CronExpr)
		result, err := cronManager.ManageSchedule("add", "", tc.CronExpr, tc.TaskPrompt)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR scheduling cron: %v", err)
		}
		return result

	case "list_cron_jobs":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested cron job list")
		result, _ := cronManager.ManageSchedule("list", "", "", "")
		return result

	case "remove_cron_job":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Scheduler.ReadOnly {
			return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
		}
		logger.Info("LLM requested cron job removal", "id", tc.ID)
		result, _ := cronManager.ManageSchedule("remove", tc.ID, "", "")
		return result

	case "document_creator":
		if !cfg.Tools.DocumentCreator.Enabled {
			return `Tool Output: {"status":"error","message":"Document Creator is disabled. Set tools.document_creator.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested document creation", "operation", tc.Operation, "backend", cfg.Tools.DocumentCreator.Backend)
		docResult := tools.ExecuteDocumentCreator(ctx, &cfg.Tools.DocumentCreator, tc.Operation, tc.Title, tc.Content, tc.URL, tc.Filename, tc.PaperSize, tc.Landscape, tc.Sections, tc.SourceFiles)
		// Auto-register every successfully created document in the media registry
		if mediaRegistryDB != nil {
			var parsed struct {
				Status   string `json:"status"`
				FilePath string `json:"file_path"`
				WebPath  string `json:"web_path"`
				Filename string `json:"filename"`
			}
			if jsonErr := json.Unmarshal([]byte(docResult), &parsed); jsonErr == nil && parsed.Status == "success" {
				mediaType := "document"
				if tc.Operation == "screenshot_url" || tc.Operation == "screenshot_html" {
					mediaType = "image"
				}
				tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
					MediaType:   mediaType,
					SourceTool:  "document_creator",
					Filename:    parsed.Filename,
					FilePath:    parsed.FilePath,
					WebPath:     parsed.WebPath,
					Description: tc.Title,
					Tags:        []string{"auto-generated"},
				})
			}
		}
		return docResult

	default:
		return dispatchNotHandled
	}
}

// isBlockedEnvRead returns true if the shell command appears to read an AURAGO_*
// environment variable. These variables include the master vault key and must never
// be accessible through the shell tool.
func isBlockedEnvRead(command string) bool {
	upper := strings.ToUpper(command)
	if !strings.Contains(upper, "AURAGO_") {
		return false
	}
	lower := strings.ToLower(command)
	// Match common env-reading patterns across sh/bash/zsh/PowerShell
	return strings.Contains(lower, "printenv") ||
		strings.Contains(lower, "echo") ||
		strings.Contains(lower, "$env:") ||
		strings.Contains(lower, "get-item") ||
		strings.Contains(lower, "get-childitem") ||
		strings.Contains(lower, "getenvironmentvariable") ||
		strings.Contains(lower, "[system.environment]") ||
		strings.Contains(lower, "environ") ||
		strings.Contains(lower, "export")
}
