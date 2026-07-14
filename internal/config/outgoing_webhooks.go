package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// OutgoingWebhookMaskedValue is the stable mask accepted by API and tool updates.
const OutgoingWebhookMaskedValue = "••••••••"

var outgoingWebhookIDSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// OutgoingWebhookSecrets is the encrypted vault payload for one outgoing webhook.
type OutgoingWebhookSecrets struct {
	URL          string            `json:"url,omitempty"`
	BodyTemplate string            `json:"body_template,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// OutgoingWebhookSecretStore is the vault contract required for transactional persistence.
type OutgoingWebhookSecretStore interface {
	SecretReadWriter
	DeleteSecret(key string) error
}

// OutgoingWebhookSecretsVaultKey returns the stable per-hook vault bundle key.
func OutgoingWebhookSecretsVaultKey(webhookID string) string {
	id := strings.TrimSpace(webhookID)
	if id == "" {
		return ""
	}
	sanitized := strings.Trim(outgoingWebhookIDSanitizer.ReplaceAllString(strings.ToLower(id), "_"), "_")
	if sanitized == "" {
		sanitized = "hook"
	}
	if len(sanitized) > 48 {
		sanitized = sanitized[:48]
	}
	sum := sha256.Sum256([]byte(id))
	return "webhook_outgoing_" + sanitized + "_" + hex.EncodeToString(sum[:6]) + "_secrets"
}

// IsSensitiveOutgoingWebhookHeader reports whether a header value belongs in the vault.
func IsSensitiveOutgoingWebhookHeader(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	if lower == "authorization" || lower == "proxy-authorization" || lower == "cookie" || lower == "set-cookie" {
		return true
	}
	return strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "api-key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "password") || strings.Contains(lower, "credential")
}

// NormalizeOutgoingWebhookMethod validates and canonicalizes an HTTP method.
func NormalizeOutgoingWebhookMethod(method string) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodPost
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return method, nil
	default:
		return "", fmt.Errorf("unsupported outgoing webhook method %q", method)
	}
}

// MaskOutgoingWebhooks returns a display-safe copy without exposing vault-only values.
func MaskOutgoingWebhooks(outgoing []OutgoingWebhook) []OutgoingWebhook {
	masked := make([]OutgoingWebhook, len(outgoing))
	for i, hook := range outgoing {
		masked[i] = hook
		if strings.TrimSpace(hook.URL) != "" {
			masked[i].URL = OutgoingWebhookMaskedValue
		}
		if strings.TrimSpace(hook.BodyTemplate) != "" {
			masked[i].BodyTemplate = OutgoingWebhookMaskedValue
		}
		masked[i].SecretHeaders = nil
		masked[i].Headers = cloneStringMap(hook.Headers)
		for key, value := range hook.SecretHeaders {
			if strings.TrimSpace(value) != "" {
				masked[i].Headers[key] = OutgoingWebhookMaskedValue
			}
		}
		for key, value := range hook.Headers {
			if IsSensitiveOutgoingWebhookHeader(key) && strings.TrimSpace(value) != "" {
				masked[i].Headers[key] = OutgoingWebhookMaskedValue
			}
		}
	}
	return masked
}

// PrepareOutgoingWebhooks resolves masks by stable ID and validates/splits secrets.
func PrepareOutgoingWebhooks(incoming, existing []OutgoingWebhook) ([]OutgoingWebhook, error) {
	existingByID := make(map[string]OutgoingWebhook, len(existing))
	for _, hook := range existing {
		existingByID[hook.ID] = hook
	}
	seen := make(map[string]struct{}, len(incoming))
	prepared := make([]OutgoingWebhook, len(incoming))
	for i, hook := range incoming {
		hook.ID = strings.TrimSpace(hook.ID)
		if hook.ID == "" {
			return nil, fmt.Errorf("outgoing webhook at index %d requires a stable id", i)
		}
		if _, duplicate := seen[hook.ID]; duplicate {
			return nil, fmt.Errorf("duplicate outgoing webhook id %q", hook.ID)
		}
		seen[hook.ID] = struct{}{}
		old := existingByID[hook.ID]
		if hook.URL == OutgoingWebhookMaskedValue {
			hook.URL = old.URL
		}
		if hook.BodyTemplate == OutgoingWebhookMaskedValue {
			hook.BodyTemplate = old.BodyTemplate
		}
		method, err := NormalizeOutgoingWebhookMethod(hook.Method)
		if err != nil {
			return nil, err
		}
		hook.Method = method
		parsedURL, err := url.Parse(strings.TrimSpace(hook.URL))
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return nil, fmt.Errorf("outgoing webhook %q requires a valid HTTP(S) URL", hook.ID)
		}
		publicHeaders := make(map[string]string)
		secretHeaders := make(map[string]string)
		for key, value := range hook.Headers {
			if IsSensitiveOutgoingWebhookHeader(key) {
				if value == OutgoingWebhookMaskedValue {
					value = old.SecretHeaders[key]
					if value == "" {
						value = old.Headers[key]
					}
					if value == "" {
						return nil, fmt.Errorf("masked header %q for webhook %q cannot be resolved", key, hook.ID)
					}
				}
				secretHeaders[key] = value
				continue
			}
			publicHeaders[key] = value
		}
		for key, value := range hook.SecretHeaders {
			if strings.TrimSpace(value) != "" {
				secretHeaders[key] = value
			}
		}
		hook.Headers = publicHeaders
		hook.SecretHeaders = secretHeaders
		prepared[i] = hook
	}
	return prepared, nil
}

// PersistOutgoingWebhooks updates vault bundles first, patches YAML, and returns a hydrated snapshot.
func PersistOutgoingWebhooks(configPath string, current *Config, incoming []OutgoingWebhook, vault OutgoingWebhookSecretStore) (*Config, error) {
	if current == nil {
		return nil, fmt.Errorf("current config is required")
	}
	if strings.TrimSpace(configPath) == "" {
		return nil, fmt.Errorf("config path is required")
	}
	if vault == nil {
		return nil, fmt.Errorf("vault is required for outgoing webhook secrets")
	}
	prepared, err := PrepareOutgoingWebhooks(incoming, current.Webhooks.Outgoing)
	if err != nil {
		return nil, err
	}
	originalYAML, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config before outgoing webhook update: %w", err)
	}

	type snapshot struct {
		key    string
		value  string
		exists bool
	}
	snapshots := make([]snapshot, 0, len(prepared))
	rollbackVault := func() {
		for i := len(snapshots) - 1; i >= 0; i-- {
			if snapshots[i].exists {
				_ = vault.WriteSecret(snapshots[i].key, snapshots[i].value)
			} else {
				_ = vault.DeleteSecret(snapshots[i].key)
			}
		}
	}
	for _, hook := range prepared {
		key := OutgoingWebhookSecretsVaultKey(hook.ID)
		previous, readErr := vault.ReadSecret(key)
		snapshots = append(snapshots, snapshot{key: key, value: previous, exists: readErr == nil})
		bundle, marshalErr := json.Marshal(OutgoingWebhookSecrets{URL: hook.URL, BodyTemplate: hook.BodyTemplate, Headers: hook.SecretHeaders})
		if marshalErr != nil {
			rollbackVault()
			return nil, fmt.Errorf("marshal outgoing webhook secrets for %q: %w", hook.ID, marshalErr)
		}
		if err := vault.WriteSecret(key, string(bundle)); err != nil {
			rollbackVault()
			return nil, fmt.Errorf("write outgoing webhook secrets for %q: %w", hook.ID, err)
		}
	}

	candidate := *current
	candidate.Webhooks.Outgoing = prepared
	candidate.ConfigPath = configPath
	if err := candidate.Save(configPath); err != nil {
		rollbackVault()
		return nil, fmt.Errorf("save outgoing webhook config: %w", err)
	}
	loaded, err := Load(configPath)
	if err != nil {
		_ = WriteFileAtomic(configPath, originalYAML, 0o600)
		rollbackVault()
		return nil, fmt.Errorf("reload outgoing webhook config: %w", err)
	}
	loaded.ConfigPath = configPath
	loaded.ApplyVaultSecrets(vault)
	loaded.ApplyOAuthTokens(vault)

	kept := make(map[string]struct{}, len(prepared))
	for _, hook := range prepared {
		kept[hook.ID] = struct{}{}
	}
	for _, old := range current.Webhooks.Outgoing {
		if _, ok := kept[old.ID]; !ok {
			_ = vault.DeleteSecret(OutgoingWebhookSecretsVaultKey(old.ID))
		}
	}
	return loaded, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
