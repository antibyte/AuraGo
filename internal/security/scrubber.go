package security

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
)

var (
	redactedPlaceholder  = "[redacted]"
	sanitizedPlaceholder = "[sanitized]"

	// Regex for common API keys and secrets.
	// \b word boundaries prevent matching keywords embedded inside longer identifiers
	// (e.g. "auth" inside "auth_token", "key" inside "local_key_path").
	apiKeyRegex = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b["']?\s*[:=]\s*["']?([A-Za-z0-9][A-Za-z0-9\-_:+=/]{7,})["']?`)
	// fragmentedSecretRegex catches secrets obfuscated by inserting whitespace/punctuation
	// between each character (e.g. "s k - 1 2 3 4 5 6 7 8").
	// '/' and '.' are intentionally excluded from the separator class: they are path
	// component separators and including them caused file-paths to be incorrectly redacted.
	fragmentedSecretRegex = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)((?:[A-Za-z0-9][\s_:\-+=]{0,3}){8,})["']?`)
	hexSecretRegex        = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)((?:[A-Fa-f0-9]{2}[\s:\-]?){6,})["']?`)
	base64SecretRegex     = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)([A-Za-z0-9+/]{12,}={0,2})["']?`)

	// Matches <thinking>…</thinking> and <think>…</think> blocks (reasoning traces from some LLMs).
	thinkingTagRe = regexp.MustCompile(`(?is)<(thinking|think)>[\s\S]*?</(thinking|think)>`)

	// Matches <external_data>…</external_data> blocks.
	// These are security wrappers injected by the supervisor around untrusted content.
	// If the LLM erroneously echoes them in its own response text they must be stripped
	// so the wrapper syntax never leaks into the chat UI or channel outputs.
	externalDataTagRe = regexp.MustCompile(`(?is)<external_data>([\s\S]*?)</external_data>`)

	sensitiveMu     sync.RWMutex
	sensitiveValues []string
)

// RedactedText returns a user-visible placeholder for hidden content.
func RedactedText(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return redactedPlaceholder
	}
	return redactedPlaceholder + " " + reason
}

// SanitizedText returns a user-visible placeholder for content that was sanitized.
func SanitizedText(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return sanitizedPlaceholder
	}
	return sanitizedPlaceholder + ": " + reason
}

// RegisterSensitive registers a sensitive string (e.g. the vault master key) that must
// never appear in any outgoing text. Every registered value is replaced with a
// visible placeholder whenever Scrub() is called.
func RegisterSensitive(value string) {
	if value == "" {
		return
	}
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	sensitiveValues = append(sensitiveValues, value)
}

// Scrub replaces occurrences of registered sensitive values with a visible placeholder.
// It also catches simple fragmented, hex-encoded, and base64-encoded renderings of
// the registered values so chat-visible output does not leak secrets indirectly.
func Scrub(text string) string {
	if text == "" {
		return ""
	}
	sensitiveMu.RLock()
	vals := make([]string, len(sensitiveValues))
	copy(vals, sensitiveValues)
	sensitiveMu.RUnlock()

	for _, v := range vals {
		if v == "" {
			continue
		}
		text = scrubRegisteredSensitive(text, v)
	}
	return text
}

func scrubRegisteredSensitive(text, value string) string {
	if strings.Contains(text, value) {
		text = strings.ReplaceAll(text, value, redactedPlaceholder)
	}

	compact := compactSensitiveValue(value)
	if len(compact) >= 8 {
		if fragmented := buildFragmentedSensitiveRegex(compact); fragmented != nil {
			text = fragmented.ReplaceAllString(text, redactedPlaceholder)
		}
	}

	if len(value) >= 6 {
		text = replaceEncodedLiteral(text, hex.EncodeToString([]byte(value)), true)
		for _, encoded := range []string{
			base64.StdEncoding.EncodeToString([]byte(value)),
			base64.RawStdEncoding.EncodeToString([]byte(value)),
			base64.URLEncoding.EncodeToString([]byte(value)),
			base64.RawURLEncoding.EncodeToString([]byte(value)),
		} {
			text = replaceEncodedLiteral(text, encoded, false)
		}
	}

	return text
}

func compactSensitiveValue(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func buildFragmentedSensitiveRegex(value string) *regexp.Regexp {
	if value == "" {
		return nil
	}
	parts := make([]string, 0, len(value))
	for _, r := range value {
		parts = append(parts, regexp.QuoteMeta(string(r)))
	}
	pattern := strings.Join(parts, `(?:[\s_:\-./+='"\\]{0,3})`)
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return compiled
}

func replaceEncodedLiteral(text, encoded string, insensitive bool) string {
	if encoded == "" || len(encoded) < 8 {
		return text
	}
	if !insensitive && !strings.Contains(text, encoded) {
		return text
	}
	pattern := regexp.QuoteMeta(encoded)
	if insensitive {
		pattern = `(?i)` + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return text
	}
	return re.ReplaceAllString(text, redactedPlaceholder)
}

// RedactSensitiveInfo replaces sensitive patterns with a visible placeholder.
func RedactSensitiveInfo(text string) string {
	if text == "" {
		return ""
	}

	// Redact specific key-value patterns
	text = apiKeyRegex.ReplaceAllStringFunc(text, redactKeyValueMatch)
	text = fragmentedSecretRegex.ReplaceAllString(text, `$1$2`+redactedPlaceholder)
	text = hexSecretRegex.ReplaceAllString(text, `$1$2`+redactedPlaceholder)
	text = base64SecretRegex.ReplaceAllString(text, `$1$2`+redactedPlaceholder)

	// Note: We avoid aggressive generic redaction to prevent breaking valid code/data.
	// But we can add specific known keys here if identified.

	return text
}

func redactKeyValueMatch(match string) string {
	parts := strings.SplitN(match, ":", 2)
	separator := ":"
	if len(parts) < 2 {
		parts = strings.SplitN(match, "=", 2)
		separator = "="
	}
	if len(parts) == 2 {
		key := strings.TrimRight(parts[0], `"' `)
		return key + separator + " " + redactedPlaceholder
	}
	return redactedPlaceholder
}

// StripThinkingTags removes <thinking>…</thinking> (and <think>…</think>) blocks from text.
// These reasoning traces are emitted by some LLMs and must be removed before sending
// responses through channels that cannot render collapsible UI (Telegram, Discord, etc.).
// It also strips any <external_data>…</external_data> wrappers the LLM may erroneously
// include in its own output — their content is kept, only the wrapper tags are removed.
func StripThinkingTags(text string) string {
	stripped := thinkingTagRe.ReplaceAllString(text, "")
	// Unwrap <external_data> blocks: keep inner content, remove the wrapper tags.
	stripped = externalDataTagRe.ReplaceAllString(stripped, "$1")
	return strings.TrimSpace(stripped)
}
