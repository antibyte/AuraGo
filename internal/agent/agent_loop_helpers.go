package agent

import (
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

// splitCSV splits a comma-separated value string into a trimmed, non-empty slice.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// systemSecretPrefixes lists key prefixes that are exclusively managed by system/tool handlers.
// The agent may NOT read or list these via the secrets_vault tool — only their respective
// tool handlers are allowed to access them directly.
var systemSecretPrefixes = []string{
	"email_",            // email account passwords  (email_<id>_password, email_password)
	"google_workspace_", // Google Workspace OAuth credentials
	"vapid_",            // Web Push VAPID keys
	"homepage_",         // Homepage deploy credentials
	"nest_",             // Invasion nest secrets
	"auth_",             // Auth hashes / session secrets / TOTP
}

// systemSecretExact is the set of exact vault keys managed by system handlers.
var systemSecretExact = map[string]struct{}{
	"telegram_bot_token":          {},
	"discord_bot_token":           {},
	"meshcentral_password":        {},
	"meshcentral_token":           {},
	"tailscale_api_key":           {},
	"ansible_token":               {},
	"virustotal_api_key":          {},
	"brave_search_api_key":        {},
	"tts_elevenlabs_api_key":      {},
	"ntfy_token":                  {},
	"home_assistant_access_token": {},
	"webdav_password":             {},
	"koofr_password":              {},
	"proxmox_secret":              {},
	"github_token":                {},
	"rocketchat_auth_token":       {},
	"mqtt_password":               {},
	"adguard_password":            {},
	"netlify_token":               {},
	"pushover_user_key":           {},
	"pushover_app_token":          {},
}

// isSystemSecret returns true if the given vault key belongs to a system/tool handler
// and therefore must not be readable by the agent via the secrets_vault tool.
func isSystemSecret(key string) bool {
	if _, ok := systemSecretExact[key]; ok {
		return true
	}
	for _, pfx := range systemSecretPrefixes {
		if strings.HasPrefix(key, pfx) {
			return true
		}
	}
	return false
}

// isProtectedSystemPath returns true when the given path refers to a system-sensitive
// file that the agent must never read or write via the filesystem tool:
//   - The active config.yaml
//   - The vault file (vault.bin) and its lock
//   - All SQLite database files (short-term, long-term, inventory, invasion) + WAL/SHM journals
//   - Any file named .env or ending in .env
//
// rawPath may be absolute or relative; relative paths are resolved against workspaceDir.
func isProtectedSystemPath(rawPath, workspaceDir string, cfg *config.Config) bool {
	if rawPath == "" {
		return false
	}

	// Resolve to absolute path
	var abs string
	if filepath.IsAbs(rawPath) {
		abs = filepath.Clean(rawPath)
	} else {
		abs = filepath.Clean(filepath.Join(workspaceDir, rawPath))
	}

	// Block .env files by name regardless of location
	base := strings.ToLower(filepath.Base(abs))
	if base == ".env" || strings.HasSuffix(base, ".env") {
		return true
	}

	// Build list of protected absolute paths from config
	vaultBase := filepath.Join(cfg.Directories.DataDir, "vault.bin")
	protected := []string{
		cfg.ConfigPath,
		vaultBase,
		vaultBase + ".lock",
		cfg.SQLite.ShortTermPath,
		cfg.SQLite.ShortTermPath + "-wal",
		cfg.SQLite.ShortTermPath + "-shm",
		cfg.SQLite.LongTermPath,
		cfg.SQLite.LongTermPath + "-wal",
		cfg.SQLite.LongTermPath + "-shm",
		cfg.SQLite.InventoryPath,
		cfg.SQLite.InventoryPath + "-wal",
		cfg.SQLite.InventoryPath + "-shm",
		cfg.SQLite.InvasionPath,
		cfg.SQLite.InvasionPath + "-wal",
		cfg.SQLite.InvasionPath + "-shm",
	}

	for _, p := range protected {
		if p == "" {
			continue
		}
		cleanP := filepath.Clean(p)
		if abs == cleanP {
			return true
		}
		// Resolve symlinks on the stored path (covers Linux /proc/ or mount aliases)
		if resolved, err := filepath.EvalSymlinks(cleanP); err == nil {
			if abs == filepath.Clean(resolved) {
				return true
			}
		}
	}
	return false
}

// isToolError returns true if the tool result content indicates an error.
// Used for tool usage tracking to distinguish successes from failures.
func isToolError(resultContent string) bool {
	if strings.Contains(resultContent, `"status": "error"`) ||
		strings.Contains(resultContent, `"status":"error"`) ||
		strings.Contains(resultContent, `[EXECUTION ERROR]`) {
		return true
	}
	// Sandbox/shell failures with non-zero exit code
	if strings.Contains(resultContent, `"exit_code":`) &&
		!strings.Contains(resultContent, `"exit_code": 0`) &&
		!strings.Contains(resultContent, `"exit_code":0`) {
		return true
	}
	return false
}
