package security

import (
	"regexp"
	"strings"
)

var (
	// Regex for common API keys and secrets
	apiKeyRegex = regexp.MustCompile(`(?i)(key|secret|password|token|auth|credential|api_key|master_key|bot_token)["']?\s*[:=]\s*["']?([a-zA-Z0-9\-_:]{16,})["']?`)
)

// RedactSensitiveInfo replaces sensitive patterns with [REDACTED].
func RedactSensitiveInfo(text string) string {
	if text == "" {
		return ""
	}

	// Redact specific key-value patterns
	text = apiKeyRegex.ReplaceAllStringFunc(text, func(match string) string {
		parts := strings.SplitN(match, ":", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(match, "=", 2)
		}
		if len(parts) == 2 {
			key := parts[0]
			return key + ": [REDACTED]"
		}
		return "[REDACTED]"
	})

	// Note: We avoid aggressive generic redaction to prevent breaking valid code/data.
	// But we can add specific known keys here if identified.

	return text
}
