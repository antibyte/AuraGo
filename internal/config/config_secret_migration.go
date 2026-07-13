package config

import (
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var plaintextSecretVaultPaths = map[string]string{
	"ai_gateway.token":                 "ai_gateway_token",
	"telegram.bot_token":               "telegram_bot_token",
	"discord.bot_token":                "discord_bot_token",
	"meshcentral.password":             "meshcentral_password",
	"meshcentral.login_token":          "meshcentral_token",
	"tailscale.api_key":                "tailscale_api_key",
	"tailscale.tsnet.auth_key":         "tailscale_tsnet_authkey",
	"ansible.token":                    "ansible_token",
	"virustotal.api_key":               "virustotal_api_key",
	"brave_search.api_key":             "brave_search_api_key",
	"tts.elevenlabs.api_key":           "tts_elevenlabs_api_key",
	"tts.minimax.api_key":              "tts_minimax_api_key",
	"agentmail.api_key":                "agentmail_api_key",
	"notifications.ntfy.token":         "ntfy_token",
	"notifications.pushover.user_key":  "pushover_user_key",
	"notifications.pushover.app_token": "pushover_app_token",
	"auth.password_hash":               "auth_password_hash",
	"auth.session_secret":              "auth_session_secret",
	"auth.totp_secret":                 "auth_totp_secret",
	"home_assistant.access_token":      "home_assistant_access_token",
	"webdav.password":                  "webdav_password",
	"webdav.token":                     "webdav_token",
	"koofr.app_password":               "koofr_password",
	"s3.access_key":                    "s3_access_key",
	"s3.secret_key":                    "s3_secret_key",
	"paperless_ngx.api_token":          "paperless_ngx_api_token",
	"proxmox.secret":                   "proxmox_secret",
	"frigate.api_token":                "frigate_api_token",
	"github.token":                     "github_token",
	"rocketchat.auth_token":            "rocketchat_auth_token",
	"mqtt.password":                    "mqtt_password",
	"adguard.password":                 "adguard_password",
	"uptime_kuma.api_key":              "uptime_kuma_api_key",
	"space_agent.admin_password":       "space_agent_admin_password",
	"space_agent.bridge_token":         "space_agent_bridge_token",
	"fritzbox.password":                "fritzbox_password",
	"homepage.deploy_password":         "homepage_deploy_password",
	"homepage.deploy_key":              "homepage_deploy_key",
	"netlify.token":                    "netlify_token",
	"vercel.token":                     "vercel_token",
	"egg_mode.shared_key":              "egg_shared_key",
	"google_workspace.client_secret":   "google_workspace_client_secret",
	"onedrive.client_secret":           "onedrive_client_secret",
	"a2a.auth.api_key":                 "a2a_api_key",
	"a2a.auth.bearer_secret":           "a2a_bearer_secret",
	"email.password":                   "email_password",
	"telnyx.api_key":                   "telnyx_api_key",
	"ldap.bind_password":               "ldap_bind_password",
	"truenas.api_key":                  "truenas_api_key",
	"jellyfin.api_key":                 "jellyfin_api_key",
	"obsidian.api_key":                 "obsidian_api_key",
	"llm.api_key":                      "provider_main_api_key",
	"fallback_llm.api_key":             "provider_fallback_api_key",
	"co_agents.llm.api_key":            "provider_coagent_api_key",
	"a2a.llm.api_key":                  "provider_a2a_api_key",
	"personality.v2_api_key":           "provider_helper_api_key",
	"manifest.api_key":                 "manifest_api_key",
	"manifest.postgres_password":       "manifest_postgres_password",
	"manifest.better_auth_secret":      "manifest_better_auth_secret",
	"omniroute.api_key":                "omniroute_api_key",
	"omniroute.initial_password":       "omniroute_initial_password",
	"omniroute.jwt_secret":             "omniroute_jwt_secret",
	"omniroute.api_key_secret":         "omniroute_api_key_secret",
	"omniroute.ws_bridge_secret":       "omniroute_ws_bridge_secret",
	"composio.api_key":                 "composio_api_key",
	"manus.api_key":                    "manus_api_key",
	"evomap.node_secret":               "evomap_node_secret",
	"evomap.api_key":                   "evomap_api_key",
	"dograh.api_key":                   "dograh_api_key",
	"dograh.oss_jwt_secret":            "dograh_oss_jwt_secret",
	"dograh.postgres_password":         "dograh_postgres_password",
	"dograh.redis_password":            "dograh_redis_password",
	"dograh.minio_root_password":       "dograh_minio_root_password",
	"dograh.aurago_mcp_token":          "dograh_aurago_mcp_token",
}

func migrateNestedStringSecret(root map[string]interface{}, path []string, vaultKey string, vault SecretReadWriter, log *slog.Logger) bool {
	return migrateNestedStringSecretWithOverwrite(root, path, vaultKey, vault, log, false)
}

func migrateNestedStringSecretWithOverwrite(root map[string]interface{}, path []string, vaultKey string, vault SecretReadWriter, log *slog.Logger, overwrite bool) bool {
	if len(path) == 0 {
		return false
	}

	current := root
	for _, segment := range path[:len(path)-1] {
		next, ok := current[segment].(map[string]interface{})
		if !ok {
			return false
		}
		current = next
	}

	return migrateMapStringSecretWithOverwrite(current, path[len(path)-1], vaultKey, vault, log, overwrite)
}

func migrateMapStringSecret(section map[string]interface{}, yamlKey, vaultKey string, vault SecretReadWriter, log *slog.Logger) bool {
	return migrateMapStringSecretWithOverwrite(section, yamlKey, vaultKey, vault, log, false)
}

func migrateMapStringSecretWithOverwrite(section map[string]interface{}, yamlKey, vaultKey string, vault SecretReadWriter, log *slog.Logger, overwrite bool) bool {
	val, exists := section[yamlKey]
	if !exists {
		return false
	}

	strVal, ok := val.(string)
	if !ok || strings.TrimSpace(strVal) == "" {
		delete(section, yamlKey)
		return true
	}

	existing, _ := vault.ReadSecret(vaultKey)
	if overwrite || existing == "" {
		if err := vault.WriteSecret(vaultKey, strVal); err != nil {
			log.Error("[Config] Failed to migrate secret to vault", "key", vaultKey, "error", err)
			return false
		}
		log.Info("[Config] Migrated plaintext secret from config.yaml to vault", "key", vaultKey)
	} else {
		log.Debug("[Config] Vault already has value for key, skipping YAML migration", "key", vaultKey)
	}

	delete(section, yamlKey)
	return true
}

func migrateProviderSecrets(rawCfg map[string]interface{}, vault SecretReadWriter, log *slog.Logger) bool {
	items, ok := rawCfg["providers"].([]interface{})
	if !ok {
		return false
	}

	migrated := false
	for _, item := range items {
		provider, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := provider["id"].(string)
		if strings.TrimSpace(id) == "" {
			continue
		}
		if migrateMapStringSecret(provider, "api_key", "provider_"+id+"_api_key", vault, log) {
			migrated = true
		}
		if migrateMapStringSecret(provider, "oauth_client_secret", "provider_"+id+"_oauth_client_secret", vault, log) {
			migrated = true
		}
	}
	return migrated
}

func migrateEmailAccountSecrets(rawCfg map[string]interface{}, vault SecretReadWriter, log *slog.Logger) bool {
	items, ok := rawCfg["email_accounts"].([]interface{})
	if !ok {
		return false
	}

	migrated := false
	for _, item := range items {
		account, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := account["id"].(string)
		if strings.TrimSpace(id) == "" {
			continue
		}
		if migrateMapStringSecret(account, "password", "email_"+id+"_password", vault, log) {
			migrated = true
		}
	}
	return migrated
}

func migrateA2ARemoteAgentSecrets(rawCfg map[string]interface{}, vault SecretReadWriter, log *slog.Logger) bool {
	a2a, ok := rawCfg["a2a"].(map[string]interface{})
	if !ok {
		return false
	}
	client, ok := a2a["client"].(map[string]interface{})
	if !ok {
		return false
	}
	items, ok := client["remote_agents"].([]interface{})
	if !ok {
		return false
	}

	migrated := false
	for _, item := range items {
		agent, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := agent["id"].(string)
		if strings.TrimSpace(id) == "" {
			continue
		}
		if migrateMapStringSecret(agent, "api_key", "a2a_remote_"+id+"_api_key", vault, log) {
			migrated = true
		}
		if migrateMapStringSecret(agent, "bearer_token", "a2a_remote_"+id+"_bearer_token", vault, log) {
			migrated = true
		}
	}
	return migrated
}

func migrateKlipperPrinterSecrets(rawCfg map[string]interface{}, vault SecretReadWriter, log *slog.Logger) bool {
	threeD, ok := rawCfg["three_d_printers"].(map[string]interface{})
	if !ok {
		return false
	}
	klipper, ok := threeD["klipper"].(map[string]interface{})
	if !ok {
		return false
	}
	items, ok := klipper["printers"].([]interface{})
	if !ok {
		return false
	}

	migrated := false
	for _, item := range items {
		printer, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := printer["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			if _, exists := printer["api_key"]; exists {
				delete(printer, "api_key")
				migrated = true
			}
			continue
		}
		key := ThreeDPrinterKlipperAPIKeyVaultKey(id)
		if key == "" {
			if _, exists := printer["api_key"]; exists {
				delete(printer, "api_key")
				migrated = true
			}
			continue
		}
		if migrateMapStringSecret(printer, "api_key", key, vault, log) {
			migrated = true
		}
	}
	return migrated
}

// MigratePlaintextSecretsToVault moves plaintext secrets from config.yaml into the vault.
// It covers both static top-level paths and dynamic collections such as providers and email accounts.
func MigratePlaintextSecretsToVault(configPath string, vault SecretReadWriter, log *slog.Logger) {
	if vault == nil {
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return
	}

	migrated := false
	for path, vaultKey := range plaintextSecretVaultPaths {
		// Generated egg configs intentionally include a fresh per-deploy shared
		// key. Egg containers keep a persistent vault volume, so this one secret
		// must replace any stale value from a previous hatch.
		overwrite := path == "egg_mode.shared_key"
		if migrateNestedStringSecretWithOverwrite(rawCfg, strings.Split(path, "."), vaultKey, vault, log, overwrite) {
			migrated = true
		}
	}
	if migrateProviderSecrets(rawCfg, vault, log) {
		migrated = true
	}
	if migrateEmailAccountSecrets(rawCfg, vault, log) {
		migrated = true
	}
	if migrateA2ARemoteAgentSecrets(rawCfg, vault, log) {
		migrated = true
	}
	if migrateKlipperPrinterSecrets(rawCfg, vault, log) {
		migrated = true
	}

	if !migrated {
		return
	}

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		log.Error("[Config] Failed to marshal cleaned config after secret migration", "error", err)
		return
	}
	if err := WriteFileAtomic(configPath, out, 0o600); err != nil {
		log.Error("[Config] Failed to write cleaned config after secret migration", "error", err)
	}
}

// MigrateAuthSecretsToVault is a one-time startup migration for deployments that
// were originally configured before auth secrets moved to the encrypted vault.
// It reads the raw config.yaml, extracts any auth secrets stored in plaintext
// (password_hash, session_secret, totp_secret), writes them to the vault, then
// removes them from config.yaml so they are no longer stored in plaintext.
// Must be called after the vault is initialised and before ApplyVaultSecrets.
func MigrateAuthSecretsToVault(configPath string, vault SecretReadWriter, log *slog.Logger) {
	MigratePlaintextSecretsToVault(configPath, vault, log)
}

// MigrateEggModeSharedKeyToVault moves egg_mode.shared_key from plaintext YAML
// into the vault and removes the YAML field afterwards.
func MigrateEggModeSharedKeyToVault(configPath string, vault SecretReadWriter, log *slog.Logger) {
	MigratePlaintextSecretsToVault(configPath, vault, log)
}
