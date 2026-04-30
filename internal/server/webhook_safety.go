package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/webhooks"
)

func webhooksReadOnly(s *Server) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg.Webhooks.ReadOnly
}

func rejectWebhookMutationIfReadOnly(w http.ResponseWriter, s *Server) bool {
	if !webhooksReadOnly(s) {
		return false
	}
	jsonError(w, "Webhooks are read-only", http.StatusForbidden)
	return true
}

func webhookUpdateOptions(body []byte) webhooks.UpdateOptions {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return webhooks.UpdateOptions{}
	}
	opts := webhooks.UpdateOptions{}
	if _, ok := raw["enabled"]; ok {
		opts.EnabledSet = true
	}
	formatRaw, ok := raw["format"]
	if !ok {
		return opts
	}
	var format map[string]json.RawMessage
	if err := json.Unmarshal(formatRaw, &format); err != nil {
		return opts
	}
	if _, ok := format["signature_header"]; ok {
		opts.SignatureHeaderSet = true
	}
	if _, ok := format["signature_algo"]; ok {
		opts.SignatureAlgoSet = true
	}
	if _, ok := format["signature_secret"]; ok {
		opts.SignatureSecretSet = true
	}
	return opts
}

func maskOutgoingWebhooksForDisplay(outgoing []config.OutgoingWebhook) []config.OutgoingWebhook {
	masked := make([]config.OutgoingWebhook, len(outgoing))
	for i, hook := range outgoing {
		masked[i] = hook
		if hook.Headers != nil {
			masked[i].Headers = make(map[string]string, len(hook.Headers))
			for key, value := range hook.Headers {
				if value != "" && isSensitiveOutgoingHeader(key) {
					masked[i].Headers[key] = maskedKey
					continue
				}
				masked[i].Headers[key] = value
			}
		}
		if strings.TrimSpace(hook.BodyTemplate) != "" {
			masked[i].BodyTemplate = maskedKey
		}
	}
	return masked
}

func restoreMaskedOutgoingWebhooks(incoming, existing []config.OutgoingWebhook) []config.OutgoingWebhook {
	existingByID := make(map[string]config.OutgoingWebhook, len(existing))
	for _, hook := range existing {
		existingByID[hook.ID] = hook
	}
	result := make([]config.OutgoingWebhook, len(incoming))
	copy(result, incoming)
	for i := range result {
		oldHook, ok := existingByID[result[i].ID]
		if !ok {
			continue
		}
		if result[i].BodyTemplate == maskedKey {
			result[i].BodyTemplate = oldHook.BodyTemplate
		}
		if result[i].Headers == nil {
			continue
		}
		for key, value := range result[i].Headers {
			if value == maskedKey {
				result[i].Headers[key] = oldHook.Headers[key]
			}
		}
	}
	return result
}

func isSensitiveOutgoingHeader(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	if lower == "authorization" || lower == "proxy-authorization" || lower == "cookie" || lower == "set-cookie" {
		return true
	}
	return strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "api-key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "password") || strings.Contains(lower, "credential")
}
