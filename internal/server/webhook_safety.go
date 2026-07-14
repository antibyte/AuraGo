package server

import (
	"encoding/json"
	"net/http"

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
	return config.MaskOutgoingWebhooks(outgoing)
}

func restoreMaskedOutgoingWebhooks(incoming, existing []config.OutgoingWebhook) []config.OutgoingWebhook {
	prepared, err := config.PrepareOutgoingWebhooks(incoming, existing)
	if err != nil {
		return incoming
	}
	return prepared
}

func isSensitiveOutgoingHeader(name string) bool {
	return config.IsSensitiveOutgoingWebhookHeader(name)
}
