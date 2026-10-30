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

// directiveScopeMarkers say the text applies beyond this answer. They are rare
// in ordinary prose and phrase a standing order, so a target is enough. A
// trailing * matches any word ending; every other phrase is matched as whole
// words.
var directiveScopeMarkers = []string{
	// A negated refusal verb ("never refuse to", "zögere nie") is an order to comply:
	// the phrase stands for a recurrence and no negation relief reaches the verb after it.
	"never refuse to",
	"never fail to",
	"never hesitate to",
	"never decline to",
	"do not refuse to",
	"do not fail to",
	"do not hesitate to",
	"do not decline to",
	"don't refuse to",
	"don't fail to",
	"don't hesitate to",
	"don't decline to",
	"don’t refuse to",
	"don’t fail to",
	"don’t hesitate to",
	"don’t decline to",
	"dont refuse to",
	"dont fail to",
	"dont hesitate to",
	"dont decline to",
	"never stop",
	"never cease",
	"do not stop",
	"don't stop",
	"don’t stop",
	"verweigere nie",
	"verweigere niemals",
	"verweigere nicht",
	"zögere nie",
	"zögere niemals",
	"zögere nicht",
	"weigere dich nie",
	"weigere dich niemals",
	"weigere dich nicht",
	"versäume nie",
	"versäume niemals",
	"versäume nicht",
	"höre nie auf",
	"höre niemals auf",
	"hör nie auf",
	"hör niemals auf",
	"vergiss nie",
	"vergiss niemals",
	"from now on", "from here on", "from this point on", "going forward", "henceforth",
	"before replying", "before you reply", "before answering", "before you answer", "before responding",
	"standing instruction", "standing instructions", "standing order",
	"for future chats", "for future conversations", "for future sessions", "for all future", "in all future",
	"ab jetzt", "ab sofort", "von nun an", "bevor du antwortest", "vor jeder antwort",
}

// directiveRepeatMarkers say something recurs ("always", "whenever", "immer").
// Documentation, logs, reminders and recaps are full of them ("Remember to
// rotate the API key quarterly", "Der Tresor ist immer verschlossen", "The
// assistant always uses execute_python"), so they count only next to a tool
// name the sentence commands (directiveCommandsTool) or an outbound action on a
// secret or a destination.
var directiveRepeatMarkers = []string{
	"always", "whenever", "every time", "each time", "permanently", "remember to",
	"immer", "jedes mal", "jedesmal", "dauerhaft", "merke dir", "merk dir", "in zukunft",
}

// directiveTools are the tool names such a standing instruction invokes, the
// names the agent dispatches. A bare "shell" or "terminal" is an everyday word
// and no tool name.
var directiveTools = []string{
	"execute_*", "run_tool", "api_request", "send_email", "send_telegram", "send_discord", "send_sms",
	"manage_missions", "manage_webhooks", "call_webhook", "save_tool",
}

// directiveSecondPerson address the reader. A sentence that says "you" or "your"
// next to a repeat marker and a tool name is an order, not a recap.
var directiveSecondPerson = []string{"you", "your", "yours", "yourself", "du", "dich", "dein*"}

// directiveCallVerbs invoke a tool. Opening a clause ("Always use execute_python
// for plots", "whenever the user says hello, call api_request") they command it;
// "uses", "called" or "wants to use" describe.
var directiveCallVerbs = []string{
	"call", "run", "use", "invoke", "execute", "trigger", "launch", "start", "fire",
	"rufe", "ruf", "führe", "nutze", "verwende", "benutze", "starte",
}

// directiveTrailingVerbs are the German infinitives that close an order after the
// tool name ("Immer execute_python verwenden"). The descriptive present
// ("verwendet immer execute_python") puts its verb before the tool.
var directiveTrailingVerbs = []string{"verwenden", "nutzen", "benutzen", "aufrufen", "ausführen", "starten", "einsetzen"}

// directiveConnectors may stand between a clause break and a command ("first run
// execute_shell", "then call api_request").
var directiveConnectors = []string{
	"first", "then", "next", "also", "and", "please", "just", "simply", "immediately",
	"zuerst", "dann", "bitte", "und", "einfach", "sofort",
}

// directiveSecrets are the secrets it goes after. A secret word alone is no
// hit: "Always validate the token" and "Remember to change your password" are
// advice. It counts next to an outbound or reading action (below).
var directiveSecrets = []string{
	"vault", "api key", "api keys", "api-key", "api-keys", "apikey", "apikeys",
	"token", "tokens", "password", "passwords", "passphrase", "secret", "secrets", "private key",
	"credential*", "exfiltrat*",
	"passwort", "passwörter", "kennwort", "kennwörter", "geheim*", "zugangsdaten", "anmeldedaten", "tresor",
	"api-schlüssel", "api schlüssel",
}

// directiveVerbs send something away. With a secret or a destination they make
// the instruction an exfiltration order.
var directiveVerbs = []string{
	"send", "forward", "post", "upload", "transmit", "exfiltrat*",
	"sende", "senden", "schick", "schicke", "schicken", "leite", "weiterleiten",
	"poste", "posten", "lade", "hochladen", "übermittle", "übermitteln",
}

// directiveReadVerbs reach a secret without sending it anywhere yet. After a
// scope marker ("From now on read the vault") they make the order too.
var directiveReadVerbs = []string{
	"read", "reveal", "disclose", "share", "show", "print", "output", "include", "append", "leak",
	"copy", "dump", "export", "attach", "expose", "extract", "retrieve", "fetch",
	"lies", "lese", "zeige", "zeigen", "ausgeben", "auszugeben", "offenlegen", "verrate", "kopiere", "kopieren", "anhängen", "hänge",
}

// directiveDestination matches a URL or an e-mail address in lowercase text.
var directiveDestination = regexp.MustCompile(`(?:https?|ftp|wss?)://\S+|\bwww\.[a-z0-9-]+\.[a-z]{2,}|[a-z0-9._%+-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)+`)

var directiveMinMarkerLen = func() int {
	n := len(directiveScopeMarkers[0])
	for _, markers := range [][]string{directiveScopeMarkers, directiveRepeatMarkers} {
		for _, m := range markers {
			n = min(n, len(m))
		}
	}
	return n
}()

// DirectiveShape reports whether text reads like a standing instruction that
// reaches for a tool, a secret or an outbound channel, in one sentence:
//
//   - a scope marker ("from now on", "going forward", "before replying",
//     "standing instruction", "ab jetzt") with a tool name, with a send verb
//     aimed at a secret or a URL or e-mail address, or with a read verb aimed at
//     a secret;
//   - a repeat marker ("always", "whenever", "immer") with a tool name that the
//     sentence commands ("Always use execute_python", "Every time you answer,
//     call send_email"), or with a send verb aimed at a secret or a destination.
//
// "Always send me the weekly report" has no target, "Remember to change your
// password every 90 days" no outbound action, and "The assistant always uses
// execute_python to plot the data" only describes a habit: none is a hit.
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
	if !hasPhrase(lower, directiveScopeMarkers) && !hasPhrase(lower, directiveRepeatMarkers) {
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
	scope := hasPhrase(window, directiveScopeMarkers)
	if !scope && !hasPhrase(window, directiveRepeatMarkers) {
		return false
	}
	// The markers themselves are no targets or verbs ("going forward").
	rest := maskPhrases(maskPhrases(window, directiveScopeMarkers), directiveRepeatMarkers)
	if hasPhrase(rest, directiveTools) && (scope || directiveCommandsTool(rest)) {
		return true
	}
	secret := hasPhrase(rest, directiveSecrets)
	// A negated send or disclosure verb aimed at a secret protects it ("From now
	// on never show my API keys in chat"); a destination keeps the sentence a hit.
	if secret && !directiveDestination.MatchString(rest) {
		rest = directiveMaskNegatedVerbs(rest)
	}
	if hasPhrase(rest, directiveVerbs) && (secret || directiveDestination.MatchString(rest)) {
		return true
	}
	return scope && secret && hasPhrase(rest, directiveReadVerbs)
}

// directiveNegations negate a nearby verb; directiveClauseBreaks end their reach.
var (
	directiveNegations    = []string{"never", "not", "no", "without", "avoid*", "dont", "don", "doesn", "t", "nie", "niemals", "nicht", "kein*", "ohne", "vermeide*"}
	directiveClauseBreaks = []string{"and", "but", "then", "und", "aber", "dann", "sondern", "while", "yet"}
)

// directiveMaskNegatedVerbs blanks every send or read verb that a negation
// governs: up to four words before it or after it in the same clause (no comma,
// colon or conjunction between).
func directiveMaskNegatedVerbs(s string) string {
	words := directiveWords(s)
	// directiveWords drops punctuation, so recover it from the gaps between words.
	starts := make([]int, len(words))
	for i, w := range words {
		starts[i] = w.end - len(w.text)
	}
	barrier := func(i int) bool { return hasPhrase(words[i].text, directiveClauseBreaks) }
	negated := func(i int) bool {
		for j := i - 1; j >= 0 && j >= i-4; j-- {
			if barrier(j) || words[j+1].afterBreak {
				break
			}
			if hasPhrase(words[j].text, directiveNegations) {
				return true
			}
		}
		for j := i + 1; j < len(words) && j <= i+4; j++ {
			if barrier(j) || words[j].afterBreak {
				break
			}
			if hasPhrase(words[j].text, directiveNegations) {
				return true
			}
		}
		return false
	}
	out := []byte(s)
	for i, w := range words {
		if !hasPhrase(w.text, directiveVerbs) && !hasPhrase(w.text, directiveReadVerbs) {
			continue
		}
		if negated(i) {
			for k := starts[i]; k < w.end; k++ {
				out[k] = ' '
			}
		}
	}
	return string(out)
}

// directiveCommandsTool reports whether a sentence with a repeat marker and a
// tool name orders the tool: it speaks to the reader ("you", "du"), a call
// verb opens a clause (sentence start, after a comma, colon or dash, or after a
// connector such as "first") and a tool name follows it, or a German infinitive
// closes the sentence after the tool name. A third-person recap
// ("The agent always called save_tool", "The user always wants the
// manage_missions tool used") is neither. rest is lowercase with the markers
// masked.
func directiveCommandsTool(rest string) bool {
	if hasPhrase(rest, directiveSecondPerson) {
		return true
	}
	words := directiveWords(rest)
	for i, w := range words {
		if hasPhrase(w.text, directiveTrailingVerbs) && hasPhrase(rest[:w.end], directiveTools) {
			return true
		}
		if !hasPhrase(w.text, directiveCallVerbs) {
			continue
		}
		if i > 0 && !w.afterBreak && !hasPhrase(words[i-1].text, directiveConnectors) {
			continue
		}
		if hasPhrase(rest[w.end:], directiveTools) {
			return true
		}
	}
	return false
}

// directiveWord is one word of directiveWords: its end offset in the text and
// whether anything but spaces (punctuation) separates it from the word before.
type directiveWord struct {
	text       string
	end        int
	afterBreak bool
}

// directiveWords splits s into words (see isDirectiveWordRune). The first word
// counts as following a break.
func directiveWords(s string) []directiveWord {
	var words []directiveWord
	start, brk := -1, true
	flush := func(end int) {
		if start >= 0 {
			words = append(words, directiveWord{text: s[start:end], end: end, afterBreak: brk})
			start, brk = -1, false
		}
	}
	for i, r := range s {
		switch {
		case isDirectiveWordRune(r):
			if start < 0 {
				start = i
			}
		case unicode.IsSpace(r):
			flush(i)
		default:
			flush(i)
			brk = true
		}
	}
	flush(len(s))
	return words
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

// maskPhrases blanks every whole-word occurrence of phrases in s, so a marker
// cannot double as a target ("going forward" holds the verb "forward").
func maskPhrases(s string, phrases []string) string {
	var b []byte
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
				if b == nil {
					b = []byte(s)
				}
				for k := i; k < end; k++ {
					b[k] = ' '
				}
				s = string(b)
			}
			from = i + 1
		}
	}
	return s
}
