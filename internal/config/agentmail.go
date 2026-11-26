package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateAgentMailWebSocketURL validates the relay endpoint without dialing it. The API key
// travels in the handshake, so only an absolute wss:// URL without credentials, query or
// fragment is accepted; the documented default has no query either.
func ValidateAgentMailWebSocketURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil // The documented wss default applies.
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "wss") || u.Hostname() == "" || u.User != nil ||
		u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return fmt.Errorf("agentmail.websocket_url must be an absolute wss:// URL without credentials, query or fragment")
	}
	return nil
}

// ValidateAgentMailBaseURL validates configuration without resolving or contacting it.
func ValidateAgentMailBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil // The documented HTTPS default applies.
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return fmt.Errorf("agentmail.base_url must be an absolute HTTPS URL without credentials, query or fragment")
	}
	return nil
}
