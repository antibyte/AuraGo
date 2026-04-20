package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/security"
	"aurago/internal/tools"
)

func dispatchPython(tc ToolCall, dc *DispatchContext) string {
	cfg := dc.Cfg
	logger := dc.Logger
	vault := dc.Vault
	registry := dc.Registry
	manifest := dc.Manifest
	inventoryDB := dc.InventoryDB

	switch tc.Action {
	case "execute_python":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] execute_python is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		req := decodePythonExecutionArgs(tc)
		logger.Info("LLM requested python execution", "code_len", len(req.Code), "background", req.Background)
		if req.Code == "" {
			return "Tool Output: [EXECUTION ERROR] 'code' field is empty. You MUST provide Python source code in the 'code' field. Do NOT use execute_python for SSH or remote tasks — use query_inventory / execute_remote_shell instead."
		}
		// Resolve vault secrets
		secrets, rejectedInfo := resolveVaultKeys(cfg, vault, req.VaultKeys, logger)
		// Resolve credential secrets
		creds, credRejInfo := resolveCredentials(cfg, vault, inventoryDB, req.CredentialIDs, logger)
		if credRejInfo != "" {
			if rejectedInfo != "" {
				rejectedInfo += "\n" + credRejInfo
			} else {
				rejectedInfo = credRejInfo
			}
		}
		hasSecrets := len(secrets) > 0 || len(creds) > 0
		if req.Background {
			logger.Info("LLM requested background Python execution", "code_len", len(req.Code))
			var pid int
			var bgErr error
			if hasSecrets {
				pid, bgErr = tools.ExecutePythonBackgroundWithSecrets(req.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry, secrets, creds)
			} else {
				pid, bgErr = tools.ExecutePythonBackground(req.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry)
			}
			if bgErr != nil {
				return fmt.Sprintf("Tool Output: [EXECUTION ERROR] starting background process: %v", bgErr)
			}
			msg := fmt.Sprintf("Tool Output: Process started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
			if rejectedInfo != "" {
				msg = rejectedInfo + "\n" + msg
			}
			return msg
		}
		logger.Debug("Executing Python (foreground)", "code_preview", Truncate(req.Code, 300))
		logger.Info("LLM requested python execution", "code_len", len(req.Code))
		var stdout, stderr string
		var pyErr error
		if hasSecrets {
			stdout, stderr, pyErr = tools.ExecutePythonWithSecrets(req.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, secrets, creds)
		} else {
			stdout, stderr, pyErr = tools.ExecutePython(req.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
		}

		// Always scrub sensitive values from Python output, regardless of whether
		// vault keys were explicitly requested (defence-in-depth: the LLM may have
		// recalled sensitive values from its context window and embedded them in code)
		stdout = security.Scrub(stdout)
		stderr = security.Scrub(stderr)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if rejectedInfo != "" {
			sb.WriteString(rejectedInfo + "\n")
		}
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if pyErr != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %s\n", security.Scrub(pyErr.Error())))
		}
		return sb.String()

	case "save_tool":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] save_tool is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		req := decodeSaveToolArgs(tc)
		if req.Name == "" || req.Code == "" {
			return "Tool Output: ERROR 'name' and 'code' are required for save_tool"
		}
		if collisionName, ok := customToolBuiltinCollisionName(req.Name, allBuiltinToolNameSet()); ok {
			return fmt.Sprintf("Tool Output: ERROR custom tool name %q collides with built-in tool %q. Choose a different name.", req.Name, collisionName)
		}
		codeHash := sha256.Sum256([]byte(req.Code))
		logger.Info("LLM requested tool persistence",
			"name", req.Name,
			"description", req.Description,
			"code_size", len(req.Code),
			"code_sha256", hex.EncodeToString(codeHash[:]),
		)
		if err := manifest.SaveTool(cfg.Directories.ToolsDir, req.Name, req.Description, req.Code); err != nil {
			return fmt.Sprintf("Tool Output: ERROR saving tool: %v", err)
		}
		return fmt.Sprintf("Tool Output: Tool '%s' saved and registered successfully.", req.Name)

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

		sb.WriteString("\n[NOTE] 'list_tools' ONLY lists custom reusable Python tools saved in tools/manifest.json. It does NOT list built-in AuraGo tools or pre-built skills.\n")
		sb.WriteString("[NOTE] For built-in skills/integrations such as 'virustotal_scan', 'brave_search', 'web_scraper', 'wikipedia_search', or 'pdf_extractor', use the direct built-in action if available in your prompt/tool list, or use {\"action\":\"list_skills\"} followed by {\"action\":\"execute_skill\", ...}. Do NOT assume an integration is unavailable just because it does not appear in 'list_tools'.\n")
		sb.WriteString("[NOTE] Core capabilities like 'filesystem', 'execute_python', 'core_memory', 'query_memory', and other built-in tools are separate from custom tools. See your system prompt and tool manuals for details.")
		return sb.String()

	case "discover_tools":
		return handleDiscoverTools(tc, cfg, logger, dc.SessionID)

	case "run_tool":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] run_tool is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		req := decodeRunToolArgs(tc)
		if req.Name == "" {
			return "Tool Output: ERROR 'name' is required for run_tool"
		}
		// Intercept LLM confusing Skills for Tools
		toolPath := filepath.Join(cfg.Directories.ToolsDir, req.Name)
		// Check for symlinks to prevent path traversal
		fi, err := os.Lstat(toolPath)
		if err != nil {
			if os.IsNotExist(err) {
				skillCheckName := req.Name
				if !strings.HasSuffix(skillCheckName, ".py") {
					skillCheckName += ".py"
				}
				skillPath := filepath.Join(cfg.Directories.SkillsDir, skillCheckName)
				if _, err2 := os.Stat(skillPath); err2 == nil {
					skillBase := strings.TrimSuffix(skillCheckName, ".py")
					return fmt.Sprintf("Tool Output: ERROR '%s' is a registered SKILL, not a generic tool. You MUST use {\"action\": \"execute_skill\", \"skill\": \"%s\", \"skill_args\": {\"arg1\": \"val1\"}} (JSON object) instead.", req.Name, skillBase)
				}
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Tool '%s' not found"}`, req.Name)
			}
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Tool '%s' not accessible: %s"}`, req.Name, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"symlinks are not allowed in tools directory"}`)
		}

		if req.Background {
			logger.Info("LLM requested background tool execution", "name", req.Name)
			// Resolve vault secrets
			secrets, rejInfo := resolveVaultKeys(cfg, vault, req.VaultKeys, logger)
			// Resolve credential secrets
			creds, credRejInfo := resolveCredentials(cfg, vault, inventoryDB, req.CredentialIDs, logger)
			if credRejInfo != "" {
				if rejInfo != "" {
					rejInfo += "\n" + credRejInfo
				} else {
					rejInfo = credRejInfo
				}
			}
			var pid int
			var bgErr error
			if len(secrets) > 0 || len(creds) > 0 {
				pid, bgErr = tools.RunToolBackgroundWithSecrets(req.Name, req.Args, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry, secrets, creds)
			} else {
				pid, bgErr = tools.RunToolBackground(req.Name, req.Args, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry)
			}
			if bgErr != nil {
				return fmt.Sprintf("Tool Output: ERROR starting background tool: %v", bgErr)
			}
			msg := fmt.Sprintf("Tool Output: Tool started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
			if rejInfo != "" {
				msg = rejInfo + "\n" + msg
			}
			return msg
		}
		logger.Info("LLM requested tool execution", "name", req.Name)
		// Resolve vault secrets
		secrets, rejectedInfo := resolveVaultKeys(cfg, vault, req.VaultKeys, logger)
		// Resolve credential secrets
		creds, credRejInfo := resolveCredentials(cfg, vault, inventoryDB, req.CredentialIDs, logger)
		if credRejInfo != "" {
			if rejectedInfo != "" {
				rejectedInfo += "\n" + credRejInfo
			} else {
				rejectedInfo = credRejInfo
			}
		}
		var stdout, stderr string
		var runErr error
		if len(secrets) > 0 || len(creds) > 0 {
			stdout, stderr, runErr = tools.RunToolWithSecrets(req.Name, req.Args, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, secrets, creds)
		} else {
			stdout, stderr, runErr = tools.RunTool(req.Name, req.Args, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
		}
		errStr := ""
		if runErr != nil {
			errStr = security.Scrub(runErr.Error())
		}
		result := fmt.Sprintf("Tool Output:\nSTDOUT:\n%s\nSTDERR:\n%s\nERROR:\n%s\n", security.Scrub(stdout), security.Scrub(stderr), errStr)
		if rejectedInfo != "" {
			result = rejectedInfo + "\n" + result
		}
		return result

	default:
		return ""
	}
}
