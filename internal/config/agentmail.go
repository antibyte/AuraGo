package config

import (
	"fmt"
	"net/url"
	"strings"
)

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
