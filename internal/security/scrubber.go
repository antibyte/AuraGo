package security

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
)

var (
	// Regex for common API keys and secrets
	apiKeyRegex = regexp.MustCompile(`(?i)(key|secret|password|token|auth|credential|api_key|master_key|bot_token)["']?\s*[:=]\s*["']?([a-zA-Z0-9\-_:]{16,})["']?`)

	sensitiveMu     sync.RWMutex
	sensitiveValues []string
)

// RegisterSensitive registers a sensitive string (e.g. the vault master key) that must
// never appear in any outgoing text. Every registered value is replaced with a
// random hex string of equal length whenever Scrub() is called.
func RegisterSensitive(value string) {
	if value == "" {
		return
	}
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	sensitiveValues = append(sensitiveValues, value)
}

// Scrub replaces every occurrence of a registered sensitive value in text with a
// random hex replacement of the same byte length, so the output length is preserved
// and no sensitive data leaks through any communication channel.
func Scrub(text string) string {
	if text == "" {
		return ""
	}
	sensitiveMu.RLock()
	vals := make([]string, len(sensitiveValues))
	copy(vals, sensitiveValues)
	sensitiveMu.RUnlock()

	for _, v := range vals {
		if v == "" || !strings.Contains(text, v) {
			continue
		}
		replacement := randomHexReplacement(len(v))
		text = strings.ReplaceAll(text, v, replacement)
	}
	return text
}

// randomHexReplacement returns a random lowercase hex string of exactly charLen characters.
// If charLen is odd the result is truncated to charLen by dropping the last character.
func randomHexReplacement(charLen int) string {
	byteLen := (charLen + 1) / 2
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		// Fallback: fill with zeros rather than panic
		for i := range b {
			b[i] = 0
		}
	}
	return hex.EncodeToString(b)[:charLen]
}

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
