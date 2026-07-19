package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/security"
	"aurago/internal/tools"
)

func dispatchPackageManager(tc ToolCall, dc *DispatchContext) string {
	cfg := dc.Cfg
	vault := dc.Vault
	if cfg == nil {
		return packageManagerJSON(map[string]interface{}{"status": "error", "message": "config is not available"})
	}
	if !cfg.Agent.AllowPackageManager || !cfg.PackageManager.Enabled {
		return packageManagerJSON(map[string]interface{}{"status": "error", "message": "package_manager is disabled. Set agent.allow_package_manager=true and package_manager.enabled=true in config.yaml."})
	}

	operation := strings.ToLower(strings.TrimSpace(stringValueFromMap(tc.Params, "operation")))
	pkg := strings.TrimSpace(stringValueFromMap(tc.Params, "package", "pkg", "query"))
	manager, err := resolvePackageManager(stringValueFromMap(tc.Params, "manager"), cfg.PackageManager.Override, cfg.PackageManager.AutoDetect)
	if operation == "detect" && err != nil {
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "message": security.Scrub(err.Error())})
	}
	if err != nil {
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "message": security.Scrub(err.Error())})
	}

	if operation == "detect" {
		return packageManagerJSON(map[string]interface{}{"status": "success", "operation": operation, "manager": manager})
	}

	if cfg.PackageManager.ReadOnly && packageManagerMutation(operation) {
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "package_manager is in read-only mode"})
	}
	if !cfg.PackageManager.AllowInstall && operation == "install" {
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "package installs are disabled by package_manager.allow_install=false"})
	}
	if !cfg.PackageManager.AllowRemove && operation == "remove" {
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "package removals are disabled by package_manager.allow_remove=false"})
	}
	if !cfg.PackageManager.AllowUpgrade && (operation == "update" || operation == "upgrade") {
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "package updates/upgrades are disabled by package_manager.allow_upgrade=false"})
	}

	var sudoPassword string
	useSudo := tools.PackageManagerRequiresSudo(manager, operation)
	if useSudo {
		if !cfg.Agent.SudoEnabled {
			return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "Linux package mutations require agent.sudo_enabled=true and sudo_password in the vault"})
		}
		if cfg.Runtime.NoNewPrivileges {
			return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "sudo is not available because no-new-privileges is active"})
		}
		if !cfg.Agent.SudoUnrestricted {
			return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "Linux package mutations require agent.sudo_unrestricted=true because they write outside AuraGo's install directory"})
		}
		if cfg.Runtime.ProtectSystemStrict {
			return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "Linux package mutations are blocked because ProtectSystem=strict makes system paths read-only. Set ProtectSystem=false for aurago.service, reload systemd, and restart AuraGo."})
		}
		if vault == nil {
			return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "vault is not available for sudo_password lookup"})
		}
		var vaultErr error
		sudoPassword, vaultErr = vault.ReadSecret("sudo_password")
		if vaultErr != nil || sudoPassword == "" {
			return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "sudo_password not found in vault. Store it via secrets_vault before Linux package mutations."})
		}
	}

	if dc.Logger != nil {
		dc.Logger.Info("LLM requested package manager operation", "operation", operation, "manager", manager, "package", Truncate(pkg, 120), "sudo", useSudo)
	}

	switch operation {
	case "install":
		stdout, stderr, err := tools.PackageManagerInstall(manager, pkg, useSudo, sudoPassword)
		return packageManagerExecutionResult(operation, manager, pkg, stdout, stderr, err)
	case "remove":
		stdout, stderr, err := tools.PackageManagerRemove(manager, pkg, useSudo, sudoPassword)
		return packageManagerExecutionResult(operation, manager, pkg, stdout, stderr, err)
	case "update":
		stdout, stderr, err := tools.PackageManagerUpdate(manager, useSudo, sudoPassword)
		return packageManagerExecutionResult(operation, manager, pkg, stdout, stderr, err)
	case "upgrade":
		stdout, stderr, err := tools.PackageManagerUpgrade(manager, pkg, useSudo, sudoPassword)
		return packageManagerExecutionResult(operation, manager, pkg, stdout, stderr, err)
	case "search":
		out, err := tools.PackageManagerSearch(manager, pkg)
		return packageManagerReadResult(operation, manager, pkg, out, err)
	case "list_installed":
		out, err := tools.PackageManagerListInstalled(manager)
		return packageManagerReadResult(operation, manager, pkg, out, err)
	case "info":
		out, err := tools.PackageManagerInfo(manager, pkg)
		return packageManagerReadResult(operation, manager, pkg, out, err)
	default:
		return packageManagerJSON(map[string]interface{}{"status": "error", "operation": operation, "manager": manager, "message": "unknown package_manager operation"})
	}
}

func resolvePackageManager(requested, configured string, autoDetect bool) (string, error) {
	manager := strings.ToLower(strings.TrimSpace(requested))
	if manager == "" {
		manager = strings.ToLower(strings.TrimSpace(configured))
	}
	if manager != "" {
		if err := validatePackageManagerName(manager); err != nil {
			return "", err
		}
		return manager, nil
	}
	if !autoDetect {
		return "", fmt.Errorf("package_manager.auto_detect=false and no manager override was provided")
	}
	return tools.DetectPackageManager()
}

func validatePackageManagerName(manager string) error {
	switch manager {
	case "apt", "dnf", "yum", "pacman", "zypper", "apk", "brew", "winget", "choco", "scoop":
		return nil
	default:
		return fmt.Errorf("unsupported package manager override %q", manager)
	}
}

func packageManagerMutation(operation string) bool {
	switch operation {
	case "install", "remove", "update", "upgrade":
		return true
	default:
		return false
	}
}

func packageManagerExecutionResult(operation, manager, pkg, stdout, stderr string, err error) string {
	result := map[string]interface{}{
		"status":    "success",
		"operation": operation,
		"manager":   manager,
		"package":   pkg,
		"stdout":    security.Scrub(stdout),
		"stderr":    security.Scrub(stderr),
	}
	if err != nil {
		result["status"] = "error"
		result["message"] = security.Scrub(err.Error())
	}
	return packageManagerJSON(result)
}

func packageManagerReadResult(operation, manager, pkg, output string, err error) string {
	result := map[string]interface{}{
		"status":    "success",
		"operation": operation,
		"manager":   manager,
		"package":   pkg,
		"output":    security.Scrub(output),
	}
	if err != nil {
		result["status"] = "error"
		result["message"] = security.Scrub(err.Error())
	}
	return packageManagerJSON(result)
}

func packageManagerJSON(payload map[string]interface{}) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"encode package_manager result: %s"}`, security.Scrub(err.Error()))
	}
	return "Tool Output: " + string(data)
}
