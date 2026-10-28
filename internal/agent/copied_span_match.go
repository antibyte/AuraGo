package agent

import (
	"encoding/json"
	"html"
	"math/bits"
	"math/rand/v2"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

// The copied-span test compares normalised text on both sides. A model that copies
// a paragraph into a call changes whitespace (a hard-wrapped 40-column paragraph
// becomes one line), HTML escaping (&#34; back to a quote) and JSON escaping, so a
// byte-for-byte window would miss a verbatim copy.

// normalizeCopiedText decodes HTML entities, drops format characters and collapses
// every whitespace run, newlines included, to one space.
func normalizeCopiedText(text string) string {
	if strings.Contains(text, "&") {
		text = html.UnescapeString(text)
	}
	var out strings.Builder
	out.Grow(len(text))
	pending := false
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Cf, r):
			continue
		case unicode.IsSpace(r):
			pending = out.Len() > 0
		default:
			if pending {
				out.WriteByte(' ')
				pending = false
			}
			out.WriteRune(r)
		}
	}
	return out.String()
}

// decodeJSONEscapes resolves the string escapes of JSON text (\n, \", \\, \/, \uXXXX
// with surrogate pairs) without requiring the whole text to parse. A tool result
// that is JSON carries line breaks and quotes in this form, while a model that
// copies the content writes the decoded characters.
func decodeJSONEscapes(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	hex4 := func(at int) (rune, bool) {
		if at+4 > len(text) {
			return 0, false
		}
		value, err := strconv.ParseUint(text[at:at+4], 16, 32)
		return rune(value), err == nil
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '\\' || i+1 >= len(text) {
			out.WriteByte(c)
			continue
		}
		switch next := text[i+1]; next {
		case 'n', 'r', 't', 'b', 'f':
			out.WriteByte(' ')
			i++
		case '"', '\\', '/':
			out.WriteByte(next)
			i++
		case 'u':
			r, ok := hex4(i + 2)
			if !ok {
				out.WriteByte(c)
				continue
			}
			consumed := 5
			if utf16.IsSurrogate(r) && i+7 < len(text) && text[i+6] == '\\' && text[i+7] == 'u' {
				if low, ok := hex4(i + 8); ok {
					if pair := utf16.DecodeRune(r, low); pair != unicode.ReplacementChar {
						r, consumed = pair, 11
					}
				}
			}
			out.WriteRune(r)
			i += consumed
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// percentDecode resolves %XX escapes and form-encoded plus signs. An invalid escape
// stays as written, so a stray percent sign cannot keep the rest from being decoded
// (url.QueryUnescape gives up on the whole text at the first one).
func percentDecode(text string) string {
	if !strings.ContainsAny(text, "%+") {
		return text
	}
	hexValue := func(c byte) (byte, bool) {
		switch {
		case c >= '0' && c <= '9':
			return c - '0', true
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10, true
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10, true
		}
		return 0, false
	}
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '+' {
			out.WriteByte(' ')
			continue
		}
		if c == '%' && i+2 < len(text) {
			hi, okHi := hexValue(text[i+1])
			lo, okLo := hexValue(text[i+2])
			if okHi && okLo {
				out.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.String()
}

// percentDecodedRounds returns what text becomes after one, two and three rounds of
// percent decoding, stopping when a round changes nothing. A model that wants to
// hide a copy can encode it more than once.
func percentDecodedRounds(text string) []string {
	var rounds []string
	for range 3 {
		next := percentDecode(text)
		if next == text {
			break
		}
		rounds = append(rounds, next)
		text = next
	}
	return rounds
}

// copiedSpanArgumentText is the text of a call that would be executed or sent:
// every string value and object key of its JSON form, decoded, in one normalised
// line. A value that holds percent-encoding (a form-encoded body, a query string)
// also contributes its decoded forms, because the receiver reads those. The session
// todo and the guardian notes ride along on the call without being part of it.
func copiedSpanArgumentText(tc ToolCall) string {
	return copiedSpanArguments(tc, true)
}

func copiedSpanArguments(tc ToolCall, decodePercent bool) string {
	tc.Todo = ""
	tc.GuardianJustification = ""
	tc.GuardianBlocked = false
	tc.GuardianBlockReason = ""
	raw, err := json.Marshal(tc)
	if err != nil {
		return ""
	}
	var node interface{}
	if json.Unmarshal(raw, &node) != nil {
		return ""
	}
	var text strings.Builder
	var walk func(node interface{}, depth int)
	walk = func(node interface{}, depth int) {
		if depth > 64 {
			return
		}
		switch value := node.(type) {
		case string:
			text.WriteString(value)
			text.WriteByte('\n')
			if decodePercent {
				for _, decoded := range percentDecodedRounds(value) {
					text.WriteString(decoded)
					text.WriteByte('\n')
				}
			}
		case []interface{}:
			for _, item := range value {
				walk(item, depth+1)
			}
		case map[string]interface{}:
			for key, item := range value {
				text.WriteString(key)
				text.WriteByte('\n')
				walk(item, depth+1)
			}
		}
	}
	walk(node, 0)
	if decodePercent {
		return normalizeCopiedText(text.String())
	}
	return text.String()
}

// copiedSpanViews are the normalised texts an isolated output is indexed by: the
// text as shown, and the same text with JSON string escapes resolved when it has any.
func copiedSpanViews(payload string) []string {
	plain := normalizeCopiedText(payload)
	views := []string{plain}
	if decoded := normalizeCopiedText(decodeJSONEscapes(payload)); decoded != plain {
		views = append(views, decoded)
	}
	return views
}

// sharesWindow reports whether the two texts have a common run of copiedSpanRunes
// runes. It indexes the shorter text by a rolling hash of every window and probes
// the longer one, so the cost is linear in both. A hash hit is verified rune by
// rune, so a collision can never produce a false match.
func sharesWindow(a, b []rune) bool {
	const w = copiedSpanRunes
	if len(a) < w || len(b) < w {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	top := uint64(1) // spanHashBase^(w-1)
	for i := 1; i < w; i++ {
		top = spanHashMul(top, spanHashBase)
	}
	hash := func(text []rune) uint64 {
		var h uint64
		for _, r := range text {
			h = spanHashAdd(spanHashMul(h, spanHashBase), uint64(r))
		}
		return h
	}
	roll := func(h uint64, out, in rune) uint64 {
		h = spanHashSub(h, spanHashMul(uint64(out), top))
		return spanHashAdd(spanHashMul(h, spanHashBase), uint64(in))
	}
	windows := len(a) - w + 1
	head := make(map[uint64]int32, windows) // hash -> latest window start + 1
	chain := make([]int32, windows)         // previous window with the same hash + 1
	h := hash(a[:w])
	for i := 0; ; i++ {
		chain[i], head[h] = head[h], int32(i+1)
		if i+1 == windows {
			break
		}
		h = roll(h, a[i], a[i+w])
	}
	same := func(start, at int) bool {
		for k := 0; k < w; k++ {
			if a[start+k] != b[at+k] {
				return false
			}
		}
		return true
	}
	h = hash(b[:w])
	for j := 0; ; j++ {
		for k := head[h]; k != 0; k = chain[k-1] {
			if same(int(k-1), j) {
				return true
			}
		}
		if j+w == len(b) {
			return false
		}
		h = roll(h, b[j], b[j+w])
	}
}

// Rolling hash over the Mersenne prime 2^61-1 with a base chosen at start, so the
// model cannot aim a call at a collision.
const spanHashMod = (1 << 61) - 1

var spanHashBase = uint64(1<<21) + rand.Uint64N(spanHashMod-(1<<21))

func spanHashMul(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	r := (hi<<3 | lo>>61) + (lo & spanHashMod)
	if r >= spanHashMod {
		r -= spanHashMod
	}
	return r
}

func spanHashAdd(a, b uint64) uint64 {
	r := a + b
	if r >= spanHashMod {
		r -= spanHashMod
	}
	return r
}

func spanHashSub(a, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return a + spanHashMod - b
}
