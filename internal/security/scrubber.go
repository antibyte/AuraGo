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
	apiKeyRegex = regexp.MustCompile(`(?i)\b(key|secret|password|passwd|pwd|pin|token|auth|credential|api_key|master_key|bot_token)\b["']?\s*[:=]\s*["']?([A-Za-z0-9][A-Za-z0-9\-_:+=/]*)["']?`)
	// fragmentedSecretRegex catches secrets obfuscated by inserting whitespace/punctuation
	// between each character (e.g. "s k - 1 2 3 4 5 6 7 8").
	// '/' and '.' are intentionally excluded from the separator class: they are path
	// component separators and including them caused file-paths to be incorrectly redacted.
	fragmentedSecretRegex = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)((?:[A-Za-z0-9][\s_:\-+=]{0,3}){8,})["']?`)
	hexSecretRegex        = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)((?:[A-Fa-f0-9]{2}[\s:\-]?){6,})["']?`)
	base64SecretRegex     = regexp.MustCompile(`(?i)\b(key|secret|password|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)([A-Za-z0-9+/]{12,}={0,2})["']?`)

	// bearerTokenRegex catches Authorization header values: "Bearer <token>"
	bearerTokenRegex = regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_.+/=]+|[A-Za-z0-9\-_:+/=]{20,})`)
	// urlCredentialsRegex catches credentials embedded in URLs: "scheme://user:pass@host"
	urlCredentialsRegex = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+\-.]*://)([^:@/\s]+:[^@/\s]+)(@)`)

	// Matches <thinking>…</thinking> and <think>…</think> blocks (reasoning traces from some LLMs).
	thinkingTagRe = regexp.MustCompile(`(?is)<(thinking|think)>[\s\S]*?</(thinking|think)>`)

	// Matches <external_data>…</external_data> blocks.
	// These are security wrappers injected by the supervisor around untrusted content.
	// If the LLM erroneously echoes them in its own response text they must be stripped
	// so the wrapper syntax never leaks into the chat UI or channel outputs.
	externalDataTagRe = regexp.MustCompile(`(?is)<external_data>([\s\S]*?)</external_data>`)

	// Matches hallucinated RAG placeholder lines emitted by some models.
	// Patterns like [$LEERER TRÄGER], [/SUCHERGEBNIS], [$EMPTY CARRIER] appear when a
	// model invents bracket-based markup to represent empty search result carriers.
	// These must be stripped before they reach the chat UI.
	hallucinatedRagRe = regexp.MustCompile(`(?m)^\s*\[\$[^\]]*\]\s*$|^\s*\[\/[A-ZÄÖÜ][A-ZÄÖÜ _]+\]\s*$`)

	sensitiveMu           sync.RWMutex
	sensitiveValues       = make(map[string]sensitiveEntry)
	scopedSensitiveValues = make(map[string]int)
)

const minimumGlobalSensitiveLiteralBytes = 8

// sensitiveEntry contains the immutable, precomputed redaction forms for one
// permanently registered value. Keeping these forms with the deduplicated
// value avoids recompiling regular expressions on every Scrub call.
type sensitiveEntry struct {
	value      string
	fragmented *regexp.Regexp
	hex        *regexp.Regexp
	base64     []string
}

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
	if len(value) < minimumGlobalSensitiveLiteralBytes {
		return
	}
	sensitiveMu.Lock()
	if sensitiveValues == nil {
		sensitiveValues = make(map[string]sensitiveEntry)
	}
	if _, exists := sensitiveValues[value]; !exists {
		sensitiveValues[value] = prepareSensitiveEntry(value)
	}
	sensitiveMu.Unlock()
}

// RegisterScopedSensitiveExact registers a value for short-lived exact-match
// redaction and returns an idempotent release function. Modal secrets use this
// scope while the Vault write and value-free acknowledgement are in flight, so
// they never accumulate in the permanent process-wide registry.
//
// Values shorter than eight bytes are deliberately excluded: registering a
// common PIN or one-character value as a global literal would corrupt unrelated
// application output. Those values remain protected by the value-free prompt
// data flow and contextual credential redaction.
func RegisterScopedSensitiveExact(value string) func() {
	if len(value) < minimumGlobalSensitiveLiteralBytes {
		return func() {}
	}

	sensitiveMu.Lock()
	if scopedSensitiveValues == nil {
		scopedSensitiveValues = make(map[string]int)
	}
	scopedSensitiveValues[value]++
	sensitiveMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			sensitiveMu.Lock()
			if scopedSensitiveValues[value] <= 1 {
				delete(scopedSensitiveValues, value)
			} else {
				scopedSensitiveValues[value]--
			}
			sensitiveMu.Unlock()
		})
	}
}

// Scrub replaces occurrences of registered sensitive values with a visible placeholder.
// It also catches simple fragmented, hex-encoded, and base64-encoded renderings of
// the registered values so chat-visible output does not leak secrets indirectly.
func Scrub(text string) string {
	if text == "" {
		return ""
	}
	sensitiveMu.RLock()
	entries := make([]sensitiveEntry, 0, len(sensitiveValues))
	for _, entry := range sensitiveValues {
		entries = append(entries, entry)
	}
	scoped := make([]string, 0, len(scopedSensitiveValues))
	for value := range scopedSensitiveValues {
		if _, permanent := sensitiveValues[value]; !permanent {
			scoped = append(scoped, value)
		}
	}
	sensitiveMu.RUnlock()

	for _, entry := range entries {
		text = scrubSensitiveEntry(text, entry)
	}
	for _, value := range scoped {
		text = strings.ReplaceAll(text, value, redactedPlaceholder)
	}
	return text
}

func prepareSensitiveEntry(value string) sensitiveEntry {
	entry := sensitiveEntry{value: value}
	compact := compactSensitiveValue(value)
	if len(compact) >= 8 {
		entry.fragmented = buildFragmentedSensitiveRegex(compact)
	}
	if len(value) >= 6 {
		if pattern, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(hex.EncodeToString([]byte(value)))); err == nil {
			entry.hex = pattern
		}
		entry.base64 = []string{
			base64.StdEncoding.EncodeToString([]byte(value)),
			base64.RawStdEncoding.EncodeToString([]byte(value)),
			base64.URLEncoding.EncodeToString([]byte(value)),
			base64.RawURLEncoding.EncodeToString([]byte(value)),
		}
	}
	return entry
}

func scrubSensitiveEntry(text string, entry sensitiveEntry) string {
	if entry.value != "" && strings.Contains(text, entry.value) {
		text = strings.ReplaceAll(text, entry.value, redactedPlaceholder)
	}
	if entry.fragmented != nil {
		text = entry.fragmented.ReplaceAllString(text, redactedPlaceholder)
	}
	if entry.hex != nil {
		text = entry.hex.ReplaceAllString(text, redactedPlaceholder)
	}
	for _, encoded := range entry.base64 {
		if encoded != "" && strings.Contains(text, encoded) {
			text = strings.ReplaceAll(text, encoded, redactedPlaceholder)
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
	// Redact Bearer tokens and URL-embedded credentials
	text = bearerTokenRegex.ReplaceAllString(text, `$1`+redactedPlaceholder)
	text = urlCredentialsRegex.ReplaceAllString(text, `$1`+redactedPlaceholder+`$3`)

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
	// Protect [/TOOL_CALL] closing tags from the hallucinated RAG cleanup regex.
	// The regex \[\/[A-ZÄÖÜ][A-ZÄÖÜ _]+\] matches [/TOOL_CALL] on its own line,
	// which breaks bracket-format tool call detection downstream.
	const toolCallCloseMarker = "[/TOOL_CALL]"
	const toolCallClosePlaceholder = "\x00TC_CLOSE\x00"
	hasToolCallClose := strings.Contains(stripped, toolCallCloseMarker)
	if hasToolCallClose {
		stripped = strings.ReplaceAll(stripped, toolCallCloseMarker, toolCallClosePlaceholder)
	}
	// Remove hallucinated RAG placeholder lines (e.g. [$LEERER TRÄGER], [/SUCHERGEBNIS]).
	stripped = hallucinatedRagRe.ReplaceAllString(stripped, "")
	if hasToolCallClose {
		stripped = strings.ReplaceAll(stripped, toolCallClosePlaceholder, toolCallCloseMarker)
	}
	return strings.TrimSpace(stripped)
}
