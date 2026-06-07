package tools

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/sandbox"
	"aurago/internal/security"
)

// blockedSecretPrefixes lists vault key prefixes that are exclusively managed by
// system integrations and must NEVER be accessible to Python tools.
var blockedSecretPrefixes = []string{
	"email_",
	"agentmail_",
	"google_workspace_",
	"vapid_",
	"homepage_",
	"nest_",
	"auth_",
	"provider_",
	"credential_password_",
	"credential_certificate_",
	"credential_token_",
	"credential_api_key_",
	"credential_session_",
	"credential_bearer_",
	"sqlconn_",
	"cloudflare_tunnel_",
	"s3_",
	"telnyx_",
	"fritzbox_",
	"mqtt_",
	"ollama_managed_",
	"jellyfin_",
	"obsidian_",
	"webhook_",
	"space_agent_",
	"manifest_",
	"dograh_",
	"desktop_store_",
	"oauth_",
	"mcp_secret_",
	"a2a_remote_",
	"music_minimax_",
	"music_google_lyria_",
}

// blockedSecretExact is the set of exact vault keys that are exclusively
// managed by system integrations and must NEVER be accessible to Python tools.
var blockedSecretExact = map[string]struct{}{
	"ai_gateway_token":            {},
	"telegram_bot_token":          {},
	"discord_bot_token":           {},
	"meshcentral_password":        {},
	"meshcentral_token":           {},
	"tailscale_api_key":           {},
	"tailscale_tsnet_authkey":     {},
	"ansible_token":               {},
	"virustotal_api_key":          {},
	"brave_search_api_key":        {},
	"tts_elevenlabs_api_key":      {},
	"tts_minimax_api_key":         {},
	"ntfy_token":                  {},
	"home_assistant_access_token": {},
	"webdav_password":             {},
	"webdav_token":                {},
	"koofr_password":              {},
	"proxmox_secret":              {},
	"frigate_api_token":           {},
	"github_token":                {},
	"rocketchat_auth_token":       {},
	"mqtt_password":               {},
	"ldap_bind_password":          {},
	"adguard_password":            {},
	"netlify_token":               {},
	"vercel_token":                {},
	"pushover_user_key":           {},
	"pushover_app_token":          {},
	"paperless_ngx_api_token":     {},
	"jellyfin_api_key":            {},
	"obsidian_api_key":            {},
	"uptime_kuma_api_key":         {},
	"grafana_api_key":             {},
	"yepapi_api_key":              {},
	"n8n_api_token":               {},
	"mcp_server_token":            {},
	"copilot_github_token":        {},
	"onedrive_client_secret":      {},
	"onedrive_device_code":        {},
	"sudo_password":               {},
	"a2a_api_key":                 {},
	"a2a_bearer_secret":           {},
	"egg_shared_key":              {},
	"truenas_api_key":             {},
	"composio_api_key":            {},
}

// IsPythonAccessibleSecret returns true only if the vault key is a user/agent-created
// secret and NOT a system/integration-managed secret.
func IsPythonAccessibleSecret(key string) bool {
	lower := strings.ToLower(key)
	if _, ok := blockedSecretExact[lower]; ok {
		return false
	}
	for _, pfx := range blockedSecretPrefixes {
		if strings.HasPrefix(lower, pfx) {
			return false
		}
	}
	return true
}

// ResolveVaultSecrets reads the requested vault keys via the SecretReader interface.
// It returns the successfully resolved secrets, the list of rejected (blocked) key names,
// and an error only if the vault itself fails.
func ResolveVaultSecrets(vault config.SecretReader, keys []string) (resolved map[string]string, rejected []string, err error) {
	resolved = make(map[string]string, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !IsPythonAccessibleSecret(key) {
			rejected = append(rejected, key)
			continue
		}
		val, readErr := vault.ReadSecret(key)
		if readErr != nil {
			slog.Warn("Vault secret not found for Python injection", "key", key)
			continue
		}
		if val == "" {
			continue
		}
		resolved[key] = val
	}
	return resolved, rejected, nil
}

// InjectSecretsEnv adds the resolved secrets as AURAGO_SECRET_<KEY> environment variables
// to the given exec.Cmd. Each secret value is registered with the scrubber so it never
// appears in any output. The parent process environment is never modified.
func InjectSecretsEnv(cmd *exec.Cmd, secrets map[string]string) {
	if len(secrets) == 0 {
		return
	}
	// Start from filtered parent env if not already set — strips sensitive
	// vars (AURAGO_MASTER_KEY, API keys, etc.) before passing to subprocess.
	if cmd.Env == nil {
		cmd.Env = sandbox.FilterEnv(os.Environ())
	}
	for key, val := range secrets {
		envKey := "AURAGO_SECRET_" + sanitizeEnvKey(key)
		cmd.Env = append(cmd.Env, envKey+"="+val)
		security.RegisterSensitive(val)
	}
}

// BuildSecretPrelude generates a Python code snippet that sets secrets as environment
// variables. This is used for sandbox execution where env vars cannot be passed directly
// to the container. The LLM never sees the secret values — the prelude is injected
// server-side before the code is sent to the sandbox.
func BuildSecretPrelude(secrets map[string]string) string {
	if len(secrets) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("import os as _aurago_os\n")
	for key, val := range secrets {
		envKey := "AURAGO_SECRET_" + sanitizeEnvKey(key)
		// Escape backslashes and single quotes for safe Python string literal
		escaped := strings.ReplaceAll(val, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `'`, `\'`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		escaped = strings.ReplaceAll(escaped, "\r", `\r`)
		escaped = strings.ReplaceAll(escaped, "\t", `\t`)
		escaped = strings.ReplaceAll(escaped, "\x00", `\x00`)
		sb.WriteString(fmt.Sprintf("_aurago_os.environ['%s'] = '%s'\n", envKey, escaped))
		security.RegisterSensitive(val)
	}
	sb.WriteString("del _aurago_os\n")
	return sb.String()
}

// ScrubSecretOutput applies the security scrubber to stdout and stderr strings.
func ScrubSecretOutput(stdout, stderr string) (string, string) {
	return security.Scrub(stdout), security.Scrub(stderr)
}

// sanitizeEnvKey converts a vault key to a valid environment variable suffix:
// uppercase, non-alphanumeric characters replaced with underscore.
func sanitizeEnvKey(key string) string {
	upper := strings.ToUpper(key)
	var sb strings.Builder
	sb.Grow(len(upper))
	for _, r := range upper {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// CredentialFields holds the non-secret metadata and secret values for a single
// credential, keyed by field name (username, password, token).
type CredentialFields struct {
	Name   string            // sanitized credential name for env key
	Fields map[string]string // key→value: "username", "password", "token"
}

// ResolveCredentialSecrets looks up credentials by ID, checks that each has
// allow_python enabled, and reads the associated vault secrets.
// Returns resolved credential maps, list of rejected IDs, and an error.
func ResolveCredentialSecrets(db *sql.DB, vault config.SecretReader, credentialIDs []string) ([]CredentialFields, []string, error) {
	var resolved []CredentialFields
	var rejected []string

	for _, id := range credentialIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		rec, err := credentials.GetByID(db, id)
		if err != nil {
			slog.Warn("Credential not found for Python injection", "id", id)
			rejected = append(rejected, id)
			continue
		}
		if !rec.AllowPython {
			slog.Warn("Credential not allowed for Python (allow_python=false)", "id", id, "name", rec.Name)
			rejected = append(rejected, id)
			continue
		}

		cf := CredentialFields{
			Name:   sanitizeEnvKey(rec.Name),
			Fields: make(map[string]string),
		}
		if rec.Username != "" {
			cf.Fields["username"] = rec.Username
		}
		if rec.PasswordVaultID != "" {
			val, readErr := vault.ReadSecret(rec.PasswordVaultID)
			if readErr == nil && val != "" {
				cf.Fields["password"] = val
			}
		}
		if rec.TokenVaultID != "" {
			val, readErr := vault.ReadSecret(rec.TokenVaultID)
			if readErr == nil && val != "" {
				cf.Fields["token"] = val
			}
		}
		resolved = append(resolved, cf)
	}
	return resolved, rejected, nil
}

// InjectCredentialEnv adds credential fields as AURAGO_CRED_<NAME>_<FIELD>
// environment variables to the given exec.Cmd. Secret values are registered
// with the scrubber.
func InjectCredentialEnv(cmd *exec.Cmd, creds []CredentialFields) {
	if len(creds) == 0 {
		return
	}
	// Start from filtered parent env if not already set — strips sensitive vars.
	if cmd.Env == nil {
		cmd.Env = sandbox.FilterEnv(os.Environ())
	}
	for _, cf := range creds {
		prefix := "AURAGO_CRED_" + cf.Name + "_"
		for field, val := range cf.Fields {
			envKey := prefix + strings.ToUpper(field)
			cmd.Env = append(cmd.Env, envKey+"="+val)
			security.RegisterSensitive(val)
		}
	}
}

// BuildCredentialPrelude generates a Python code snippet that sets credential
// fields as environment variables. Used for sandbox execution.
func BuildCredentialPrelude(creds []CredentialFields) string {
	if len(creds) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("import os as _aurago_os\n")
	for _, cf := range creds {
		prefix := "AURAGO_CRED_" + cf.Name + "_"
		for field, val := range cf.Fields {
			envKey := prefix + strings.ToUpper(field)
			escaped := strings.ReplaceAll(val, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `'`, `\'`)
			escaped = strings.ReplaceAll(escaped, "\n", `\n`)
			escaped = strings.ReplaceAll(escaped, "\r", `\r`)
			escaped = strings.ReplaceAll(escaped, "\t", `\t`)
			escaped = strings.ReplaceAll(escaped, "\x00", `\x00`)
			sb.WriteString(fmt.Sprintf("_aurago_os.environ['%s'] = '%s'\n", envKey, escaped))
			security.RegisterSensitive(val)
		}
	}
	sb.WriteString("del _aurago_os\n")
	return sb.String()
}

// InjectToolBridgeEnv adds the tool bridge URL, token, and allowed tools as environment variables
// to the given exec.Cmd so Python skills can call native AuraGo tools.
// The token is registered with the scrubber to prevent leaking.
func InjectToolBridgeEnv(cmd *exec.Cmd, bridgeURL, bridgeToken string, allowedTools []string) {
	if bridgeURL == "" || bridgeToken == "" {
		return
	}
	if cmd.Env == nil {
		cmd.Env = sandbox.FilterEnv(os.Environ())
	}
	cmd.Env = append(cmd.Env,
		"AURAGO_TOOL_BRIDGE_URL="+bridgeURL,
		"AURAGO_TOOL_BRIDGE_TOKEN="+bridgeToken,
		"AURAGO_TOOL_BRIDGE_TOOLS="+strings.Join(allowedTools, ","),
	)
	security.RegisterSensitive(bridgeToken)
}

// BuildToolBridgePrelude generates a Python code snippet that sets the tool bridge
// URL, token, and allowed tools as environment variables. Used for sandbox execution.
func BuildToolBridgePrelude(bridgeURL, bridgeToken string, allowedTools []string) string {
	if bridgeURL == "" || bridgeToken == "" {
		return ""
	}
	security.RegisterSensitive(bridgeToken)
	escapedToken := strings.ReplaceAll(bridgeToken, `\`, `\\`)
	escapedToken = strings.ReplaceAll(escapedToken, `'`, `\'`)
	return fmt.Sprintf("import os as _aurago_tb\n_aurago_tb.environ['AURAGO_TOOL_BRIDGE_URL'] = '%s'\n_aurago_tb.environ['AURAGO_TOOL_BRIDGE_TOKEN'] = '%s'\n_aurago_tb.environ['AURAGO_TOOL_BRIDGE_TOOLS'] = '%s'\ndel _aurago_tb\n",
		bridgeURL, escapedToken, strings.Join(allowedTools, ","))
}

const (
	defaultToolBridgeCallLimit = 10
	maxToolBridgeCallLimit     = 50
)

func normalizeToolBridgeCallLimit(limit int) int {
	if limit <= 0 {
		return defaultToolBridgeCallLimit
	}
	if limit > maxToolBridgeCallLimit {
		return maxToolBridgeCallLimit
	}
	return limit
}

// BuildToolBridgeSDKPrelude registers a small in-memory aurago module for
// foreground execute_python runs that explicitly opt in to tool reentry.
func BuildToolBridgeSDKPrelude(callLimit int) string {
	callLimit = normalizeToolBridgeCallLimit(callLimit)
	return fmt.Sprintf(`
import json as _aurago_json
import os as _aurago_os
import sys as _aurago_sys
import types as _aurago_types
import urllib.error as _aurago_urlerror
import urllib.request as _aurago_urlrequest

class _AuraGoModule(_aurago_types.ModuleType):
    def __init__(self):
        super().__init__("aurago")
        self._call_count = 0
        self._call_limit = %d

    def call_tool(self, tool_name, parameters=None, timeout=60):
        if not isinstance(tool_name, str) or not tool_name.strip():
            raise ValueError("tool_name must be a non-empty string")
        tool_name = tool_name.strip()
        if parameters is None:
            parameters = {}
        if not isinstance(parameters, dict):
            raise TypeError("parameters must be a dict")
        if self._call_count >= self._call_limit:
            raise RuntimeError("AuraGo tool bridge call limit exceeded")
        allowed = [name.strip() for name in _aurago_os.environ.get("AURAGO_TOOL_BRIDGE_TOOLS", "").split(",") if name.strip()]
        if allowed and tool_name not in allowed:
            raise PermissionError("tool is not allowed via AuraGo tool bridge: " + tool_name)
        bridge_url = _aurago_os.environ.get("AURAGO_TOOL_BRIDGE_URL", "").rstrip("/")
        bridge_token = _aurago_os.environ.get("AURAGO_TOOL_BRIDGE_TOKEN", "")
        if not bridge_url or not bridge_token:
            raise RuntimeError("AuraGo tool bridge is not configured for this Python run")
        timeout = int(timeout or 60)
        body = _aurago_json.dumps({"parameters": parameters, "timeout": timeout}).encode("utf-8")
        request = _aurago_urlrequest.Request(
            bridge_url + "/" + tool_name,
            data=body,
            headers={"Content-Type": "application/json", "X-Internal-Token": bridge_token},
            method="POST",
        )
        self._call_count += 1
        try:
            with _aurago_urlrequest.urlopen(request, timeout=timeout + 5) as response:
                payload = _aurago_json.loads(response.read().decode("utf-8"))
        except _aurago_urlerror.HTTPError as exc:
            try:
                payload = _aurago_json.loads(exc.read().decode("utf-8"))
            except Exception:
                payload = {"status": "error", "result": str(exc)}
        if payload.get("status") == "error":
            raise RuntimeError(str(payload.get("result", "AuraGo tool bridge call failed")))
        return payload

_aurago_sys.modules["aurago"] = _AuraGoModule()
del _AuraGoModule
`, callLimit)
}
