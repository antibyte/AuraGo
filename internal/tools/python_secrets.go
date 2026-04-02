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
	"aurago/internal/security"
)

// blockedSecretPrefixes lists vault key prefixes that are exclusively managed by
// system integrations and must NEVER be accessible to Python tools.
var blockedSecretPrefixes = []string{
	"email_",
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
	"ntfy_token":                  {},
	"home_assistant_access_token": {},
	"webdav_password":             {},
	"webdav_token":                {},
	"koofr_password":              {},
	"proxmox_secret":              {},
	"github_token":                {},
	"rocketchat_auth_token":       {},
	"mqtt_password":               {},
	"adguard_password":            {},
	"netlify_token":               {},
	"pushover_user_key":           {},
	"pushover_app_token":          {},
	"paperless_ngx_api_token":     {},
	"jellyfin_api_key":            {},
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
	// Start from parent env if not already set
	if cmd.Env == nil {
		cmd.Env = os.Environ()
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
	if cmd.Env == nil {
		cmd.Env = os.Environ()
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
