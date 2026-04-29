package agent

import (
	"fmt"
	"strings"

	"aurago/internal/security"
	"aurago/internal/tools"
)

func dispatchShell(tc ToolCall, dc *DispatchContext) string {
	cfg := dc.Cfg
	configureToolRuntimePermissions(cfg)
	logger := dc.Logger
	vault := dc.Vault
	registry := dc.Registry
	inventoryDB := dc.InventoryDB

	switch tc.Action {
	case "execute_sandbox":
		if !cfg.Sandbox.Enabled {
			return "Tool Output: [PERMISSION DENIED] execute_sandbox is disabled (sandbox.enabled: false)."
		}
		req := decodeSandboxExecutionArgs(tc)
		if req.Code == "" {
			return "Tool Output: [EXECUTION ERROR] 'code' field is empty. Provide the source code to execute."
		}
		lang := req.Language
		if lang == "" {
			lang = "python"
		}
		// Resolve vault secrets for injection
		secrets, rejectedInfo := resolveVaultKeys(cfg, vault, req.VaultKeys, logger)
		// Resolve credential secrets for injection
		creds, credRejInfo := resolveCredentials(cfg, vault, inventoryDB, req.CredentialIDs, logger)
		if credRejInfo != "" {
			if rejectedInfo != "" {
				rejectedInfo += "\n" + credRejInfo
			} else {
				rejectedInfo = credRejInfo
			}
		}
		codeToRun := req.Code
		if len(secrets) > 0 {
			codeToRun = tools.BuildSecretPrelude(secrets) + codeToRun
		}
		if len(creds) > 0 {
			codeToRun = tools.BuildCredentialPrelude(creds) + codeToRun
		}
		logger.Info("LLM requested sandbox execution", "language", lang, "code_len", len(req.Code), "libraries", len(req.Libraries))
		result, err := tools.SandboxExecuteCode(codeToRun, lang, req.Libraries, cfg.Sandbox.TimeoutSeconds, logger)
		if err != nil {
			// Fall back to execute_python if sandbox not ready and Python is allowed
			if cfg.Agent.AllowPython && lang == "python" {
				logger.Warn("Sandbox execution failed, falling back to execute_python", "error", err)
				var stdout, stderr string
				var pyErr error
				if len(secrets) > 0 || len(creds) > 0 {
					stdout, stderr, pyErr = tools.ExecutePythonWithSecrets(req.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, secrets, creds)
				} else {
					stdout, stderr, pyErr = tools.ExecutePython(req.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
				}
				// Scrub all outputs to prevent secret leakage via error messages/tracebacks
				stdout = security.Scrub(stdout)
				stderr = security.Scrub(stderr)
				var sb strings.Builder
				sb.WriteString("Tool Output (sandbox unavailable — ran via local Python):\n")
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
			}
			return fmt.Sprintf("Tool Output: [EXECUTION ERROR] sandbox: %s", security.Scrub(err.Error()))
		}
		scrubbedResult := security.Scrub(result)
		if rejectedInfo != "" {
			return "Tool Output:\n" + rejectedInfo + "\n" + scrubbedResult
		}
		return "Tool Output:\n" + scrubbedResult

	case "execute_shell":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] execute_shell is disabled in Danger Zone settings (agent.allow_shell: false)."
		}
		req := decodeShellExecutionArgs(tc)
		// Block commands that attempt to read AURAGO_* environment variables (contains vault master key etc.)
		if isBlockedEnvRead(req.Command) {
			logger.Warn("[Security] Blocked attempt to read sensitive environment variable", "command", Truncate(req.Command, 200))
			return "Tool Output: [PERMISSION DENIED] Reading AURAGO_ environment variables via shell is not permitted."
		}
		logger.Info("LLM requested shell execution", "command", Truncate(req.Command, 200), "background", req.Background)
		if req.Background {
			pid, err := tools.ExecuteShellBackground(req.Command, cfg.Directories.WorkspaceDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: [EXECUTION ERROR] starting background shell process: %v", err)
			}
			return fmt.Sprintf("Tool Output: Shell process started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		stdout, stderr, err := tools.ExecuteShell(req.Command, cfg.Directories.WorkspaceDir)
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
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %s\n", security.Scrub(err.Error())))
			// Hint: shell is /bin/sh (POSIX), not bash — process substitution <(...) is not available.
			sb.WriteString("[Shell: /bin/sh (POSIX sh). Bash-specific syntax (e.g. process substitution <(...), [[ ]], arrays) is NOT available. Use POSIX-compatible alternatives.]\n")
		}
		return sb.String()

	case "service_manager":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] service_manager requires shell access (agent.allow_shell: false)."
		}
		operation := stringValueFromMap(tc.Params, "operation")
		service := stringValueFromMap(tc.Params, "service")
		if operation == "" || service == "" {
			return "Tool Output: [ERROR] 'operation' and 'service' are required for service_manager."
		}
		sm := tools.NewServiceManager()
		out, err := sm.ManageService(operation, service)
		if err != nil {
			return fmt.Sprintf("Tool Output: [ERROR] %v", err)
		}
		return fmt.Sprintf("Tool Output:\n%s", out)

	case "execute_sudo":
		if !cfg.Agent.SudoEnabled {
			return "Tool Output: [PERMISSION DENIED] execute_sudo is not enabled in config. Set agent.sudo_enabled: true and store the sudo password in the vault as 'sudo_password'."
		}
		if cfg.Runtime.NoNewPrivileges {
			return `Tool Output: [PERMISSION DENIED] sudo is not available: the "no new privileges" flag is set on this system. Remove no-new-privileges from your container or systemd configuration to use sudo.`
		}
		if cfg.Agent.SudoUnrestricted && cfg.Runtime.ProtectSystemStrict {
			return `Tool Output: [PERMISSION DENIED] sudo_unrestricted is enabled but ProtectSystem=strict is still active in the systemd unit. System-wide writes are blocked until you update the unit and restart AuraGo. Run: sudo systemctl edit --full aurago, comment out or remove ProtectSystem=strict, then run: sudo systemctl daemon-reload && sudo systemctl restart aurago`
		}
		req := decodeSudoExecutionArgs(tc)
		if req.Command == "" {
			return "Tool Output: [EXECUTION ERROR] 'command' is required for execute_sudo"
		}
		sudoPass, vaultErr := vault.ReadSecret("sudo_password")
		if vaultErr != nil || sudoPass == "" {
			return "Tool Output: [PERMISSION DENIED] sudo password not found in vault. Store it first: {\"action\": \"secrets_vault\", \"operation\": \"store\", \"key\": \"sudo_password\", \"value\": \"<password>\"}"
		}
		logger.Info("LLM requested sudo execution", "command", Truncate(req.Command, 200))
		stdoutS, stderrS, errS := tools.ExecuteSudo(req.Command, cfg.Directories.WorkspaceDir, sudoPass)
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
			sbSudo.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %s\n", security.Scrub(errS.Error())))
		}
		return sbSudo.String()

	case "install_package":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] install_package is disabled in Danger Zone settings (agent.allow_shell: false)."
		}
		req := decodeInstallPackageArgs(tc)
		logger.Info("LLM requested package installation", "package", req.Package)
		if req.Package == "" {
			return "Tool Output: [EXECUTION ERROR] 'package' is required for install_package"
		}
		stdout, stderr, err := tools.InstallPackage(req.Package, cfg.Directories.WorkspaceDir)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %s\n", security.Scrub(err.Error())))
		}
		return sb.String()

	default:
		return ""
	}
}
