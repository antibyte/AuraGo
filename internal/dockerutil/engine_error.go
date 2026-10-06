package dockerutil

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// EngineErrorMessage returns the text of a Docker Engine error body: the
// "message" field of the Engine's JSON error object when it has a printable
// character, otherwise the whole body. Socket proxies and reverse proxies answer with
// HTML or plain text, which is returned unchanged. The result is neither
// shortened nor cleaned; pass it through SanitizeOneLine before it reaches a
// log, an API answer or the agent.
func EngineErrorMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil && SanitizeOneLine(payload.Message, 0) != "" {
		return payload.Message
	}
	return string(body)
}

// SanitizeOneLine returns text as one printable line of at most maxBytes
// bytes. Every rune that unicode.IsPrint rejects (control characters, line
// and paragraph separators, bidi overrides, zero-width characters) becomes a
// space, the result is trimmed, and a longer result is cut at the last rune
// boundary at or before maxBytes. maxBytes <= 0 keeps the whole line. Invalid
// UTF-8 bytes become U+FFFD.
func SanitizeOneLine(text string, maxBytes int) string {
	text = strings.TrimSpace(strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return ' '
		}
		return r
	}, text))
	if maxBytes > 0 && len(text) > maxBytes {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut]
	}
	return text
}
