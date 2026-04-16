package webhooks

import "strings"

// SignatureSecretVaultKey returns the vault key used for a webhook's signature secret.
func SignatureSecretVaultKey(webhookID string) string {
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		return ""
	}
	return "webhook_" + webhookID + "_signature_secret"
}
