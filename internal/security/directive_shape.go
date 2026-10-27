package security

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DirectiveShapeCategory is the sanitized category DirectiveShape reports. The
// SECURITY WARNING line quotes it as is.
const DirectiveShapeCategory = "directive_shape"

const (
	// A durability marker and its target must share a sentence or, in a longer
	// run without punctuation, a window of directiveWindow bytes.
	directiveWindow = 512
	directiveStep   = 384
	// DirectiveShape reads at most this much of its input, which keeps the
	// worst case cheap for callers that pass whole documents.
	directiveMaxBytes = 1 << 20
)

// directiveMarkers say the text should outlive this answer (durability) or
// apply beyond it (scope). A trailing * matches any word ending; every other
// phrase is matched as whole words.
var directiveMarkers = []string{
	"from now on", "from here on", "from this point on", "going forward", "henceforth",
	"always", "whenever", "every time", "each time",
	"before replying", "before you reply", "before answering", "before you answer", "before responding",
	"permanently", "standing instruction", "standing instructions", "standing order",
	"for future chats", "for future conversations", "for future sessions", "for all future", "in all future",
	"remember to",
	"ab jetzt", "ab sofort", "von nun an", "immer", "jedes mal", "jedesmal",
	"bevor du antwortest", "vor jeder antwort", "dauerhaft", "merke dir", "merk dir", "in zukunft",
}

// directiveTargets are the tools and secrets such a standing instruction goes
// after. Tool names are the names the agent dispatches.
var directiveTargets = []string{
	"execute_*", "run_tool", "api_request", "send_email", "send_telegram", "send_discord", "send_sms",
	"manage_missions", "manage_webhooks", "call_webhook", "save_tool",
	"shell", "terminal", "vault", "api key", "api keys", "api-key", "api-keys", "apikey", "apikeys",
	"token", "tokens", "password", "passwords", "passphrase", "secret", "secrets", "private key",
	"credential*", "exfiltrat*",
	"passwort", "passwörter", "kennwort", "kennwörter", "geheim*", "zugangsdaten", "anmeldedaten", "tresor",
	"api-schlüssel", "api schlüssel",
}

// directiveVerbs send something away. They count only next to a destination.
var directiveVerbs = []string{
	"send", "forward", "post", "upload", "transmit",
	"sende", "senden", "schick", "schicke", "schicken", "leite", "weiterleiten",
	"poste", "posten", "lade", "hochladen", "übermittle", "übermitteln",
}

// directiveDestination matches a URL or an e-mail address in lowercase text.
var directiveDestination = regexp.MustCompile(`(?:https?|ftp|wss?)://\S+|\bwww\.[a-z0-9-]+\.[a-z]{2,}|[a-z0-9._%+-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)+`)

var directiveMinMarkerLen = func() int {
	n := len(directiveMarkers[0])
	for _, m := range directiveMarkers {
		n = min(n, len(m))
	}
	return n
}()

// DirectiveShape reports whether text reads like a standing instruction that
// reaches for a tool, a secret or an outbound channel: a durability or scope
// marker ("from now on", "whenever", "immer") together with a tool name, a
// secret word or a send verb aimed at a URL or e-mail address, in one sentence.
// "Always send me the weekly report" has no target and is no hit.
//
// It is a local, deterministic heuristic over the text as a reader sees it
// (invisible characters and compatibility letters folded, case and spacing
// ignored). It reads at most the first MiB, holds no state and is safe for
// concurrent use. The category is DirectiveShapeCategory on a hit, "" otherwise.
func DirectiveShape(text string) (category string, hit bool) {
	if len(text) > directiveMaxBytes {
		cut := directiveMaxBytes
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut]
	}
	lower := strings.ToLower(directiveNormalize(text))
	if !hasPhrase(lower, directiveMarkers) {
		return "", false
	}
	start := 0
	for i := 0; i <= len(lower); i++ {
		if i < len(lower) && !directiveBoundary(lower, i) {
			continue
		}
		if directiveSegmentHit(lower[start:i]) {
			return DirectiveShapeCategory, true
		}
		start = i + 1
	}
	return "", false
}

// directiveNormalize applies the scan view and then reduces whitespace to what
// matters here: a blank line ends a paragraph, every other run of spaces, tabs
// and line breaks is one space.
func directiveNormalize(text string) string {
	view := scanView(text)
	var b strings.Builder
	b.Grow(len(view))
	newlines, gap := 0, false
	for i := 0; i < len(view); {
		r, size := utf8.DecodeRuneInString(view[i:])
		i += size
		switch r {
		case '\n':
			newlines++
		case ' ', '\t', '\r':
			gap = true
		default:
			if b.Len() > 0 {
				switch {
				case newlines > 1:
					b.WriteString("\n\n")
				case newlines == 1 || gap:
					b.WriteByte(' ')
				}
			}
			newlines, gap = 0, false
			b.WriteRune(r)
		}
	}
	return b.String()
}

// directiveBoundary reports whether s[i] ends a sentence: terminal punctuation
// before a space (so URLs and addresses stay whole) or a paragraph break.
func directiveBoundary(s string, i int) bool {
	switch s[i] {
	case '\n':
		return true
	case '.', '!', '?', ';':
		return i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\n'
	}
	return false
}

// directiveSegmentHit tests one sentence, or the overlapping windows of a run
// longer than directiveWindow.
func directiveSegmentHit(seg string) bool {
	if len(seg) < directiveMinMarkerLen {
		return false
	}
	if len(seg) <= directiveWindow {
		return directiveWindowHit(seg)
	}
	for from := 0; from < len(seg); from += directiveStep {
		for from < len(seg) && !utf8.RuneStart(seg[from]) {
			from++
		}
		end := min(from+directiveWindow, len(seg))
		for end < len(seg) && !utf8.RuneStart(seg[end]) {
			end++
		}
		if directiveWindowHit(seg[from:end]) {
			return true
		}
		if end == len(seg) {
			break
		}
	}
	return false
}

func directiveWindowHit(window string) bool {
	if !hasPhrase(window, directiveMarkers) {
		return false
	}
	if hasPhrase(window, directiveTargets) {
		return true
	}
	return hasPhrase(window, directiveVerbs) && directiveDestination.MatchString(window)
}

// hasPhrase reports whether s holds any phrase as whole words (see
// directiveMarkers for the trailing *).
func hasPhrase(s string, phrases []string) bool {
	for _, phrase := range phrases {
		prefix := strings.HasSuffix(phrase, "*")
		phrase = strings.TrimSuffix(phrase, "*")
		for from := 0; from < len(s); {
			i := strings.Index(s[from:], phrase)
			if i < 0 {
				break
			}
			i += from
			end := i + len(phrase)
			if wordEdgeBefore(s, i) && (prefix || wordEdgeAfter(s, end)) {
				return true
			}
			from = i + 1
		}
	}
	return false
}

func wordEdgeBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !isDirectiveWordRune(r)
}

func wordEdgeAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return !isDirectiveWordRune(r)
}

// isDirectiveWordRune treats letters, digits and the underscore as part of a
// word, so "execute_shell" is one word and "powershell" does not contain "shell".
func isDirectiveWordRune(r rune) bool {
	if r < 0x80 {
		return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
