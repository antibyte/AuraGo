package security

import (
	"encoding/base64"
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danielthedm/promptsec"
	"golang.org/x/text/unicode/norm"
)

// The scan view is the text the heuristics read next to the raw text. The
// promptsec sanitizer deletes zero-width characters, so a phrase whose spaces
// were replaced by U+200B reads as one word and misses every `\s+` rule. The
// view never deletes: format characters and the invisible fillers become one
// ASCII space (runs collapsed), other Unicode whitespace becomes a plain
// space, and NFKC folds mathematical, circled, squared, superscript, fullwidth
// and ligature forms to the letters they imitate. The isolated body is never
// rewritten from the view.

// isScanSeparator reports whether r hides inside a word without being
// visible: every Cf rune plus the invisible fillers that are not Cf.
func isScanSeparator(r rune) bool {
	if r < 0x80 {
		return false
	}
	switch r {
	case 0x034F, // combining grapheme joiner
		0x115F, 0x1160, 0x3164, 0xFFA0, // Hangul fillers
		0x17B4, 0x17B5, // Khmer inherent vowels
		0x2800: // braille blank
		return true
	}
	return unicode.Is(unicode.Cf, r)
}

// scanView returns text with separators turned into spaces and NFKC applied.
// Plain ASCII is its own view and costs no allocation.
func scanView(text string) string {
	plain := true
	for i := 0; i < len(text); i++ {
		if c := text[i]; c >= 0x80 || c == '\v' {
			plain = false
			break
		}
	}
	if plain {
		return text
	}

	var b strings.Builder
	b.Grow(len(text))
	inRun := false
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case isScanSeparator(r):
			if !inRun {
				b.WriteByte(' ')
				inRun = true
			}
		case r == '\v' || (r >= 0x80 && unicode.IsSpace(r)):
			b.WriteByte(' ')
			inRun = false
		default:
			b.WriteString(text[i : i+size])
			inRun = false
		}
		i += size
	}
	return norm.NFKC.String(b.String())
}

// Decode shadows are extra windows built from text that only exists after
// decoding. Their findings count as encoded payloads: a shadow with a real
// signal is at least decodeShadowSeverity (ThreatHigh), the sanitizer's own
// severity for a decoded block, while a shadow with no signal adds nothing.
const (
	decodeShadowSeverity = 0.7
	// A scan decodes at most maxDecodeShadows shadows, together at most
	// maxDecodeShadowWindows scan windows of text; maxShadowsPerKind bounds one
	// window per encoding.
	maxDecodeShadows       = 64
	maxDecodeShadowWindows = 4
	maxShadowsPerKind      = 16
	// An escape cluster is at least minEscapeCluster escapes whose gaps are at
	// most escapeClusterGap bytes; the shadow adds escapeClusterContext bytes
	// on each side.
	minEscapeCluster     = 2
	escapeClusterGap     = 128
	escapeClusterContext = 128
	minShadowBase64Bytes = 8
)

var (
	base64Run     = regexp.MustCompile(`[A-Za-z0-9+/_-]{16,}={0,2}`)
	percentEscape = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)
	numericEntity = regexp.MustCompile(`&#(?:[0-9]{1,7}|[xX][0-9A-Fa-f]{1,6});?`)
)

// decodeShadows returns the decoded forms of the view that the sanitizer does
// not produce: unpadded and URL-safe base64 runs of 16 or more characters whose
// decoded text is mostly printable, percent-encoded clusters and HTML numeric
// entity clusters. Benign runs yield a shadow too; the caller scans it and
// only a finding changes the level.
func decodeShadows(view string) []string {
	var shadows []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && s != view && !seen[s] {
			seen[s] = true
			shadows = append(shadows, s)
		}
	}

	count := 0
	for _, loc := range base64Run.FindAllStringIndex(view, -1) {
		if decoded, ok := decodeBase64Run(view[loc[0]:loc[1]]); ok {
			add(decoded)
			if count++; count >= maxShadowsPerKind {
				break
			}
		}
	}
	for _, s := range escapeClusterShadows(view, percentEscape, percentDecode) {
		add(s)
	}
	for _, s := range escapeClusterShadows(view, numericEntity, html.UnescapeString) {
		add(s)
	}
	return shadows
}

// decodeBase64Run decodes a run with the raw alphabet it uses (URL-safe when it
// holds '-' or '_', standard otherwise); padding is optional. A run one
// character longer than a whole number of groups loses that character.
func decodeBase64Run(run string) (string, bool) {
	run = strings.TrimRight(run, "=")
	if len(run)%4 == 1 {
		run = run[:len(run)-1]
	}
	enc := base64.RawStdEncoding
	if strings.ContainsAny(run, "-_") {
		enc = base64.RawURLEncoding
	}
	raw, err := enc.DecodeString(run)
	if err != nil || len(raw) < minShadowBase64Bytes || !mostlyPrintable(raw) {
		return "", false
	}
	return string(raw), true
}

// mostlyPrintable reports whether at least 90% of the runes are printable or
// ordinary whitespace; undecodable bytes count against the text.
func mostlyPrintable(raw []byte) bool {
	total, printable := 0, 0
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		total++
		if (r != utf8.RuneError || size > 1) && (unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t') {
			printable++
		}
		i += size
	}
	return total > 0 && printable*10 >= total*9
}

// escapeClusterShadows decodes the excerpts of view around clusters of
// matches of re. Encoded phrases stay local, so a page with many unrelated
// escapes (URLs) yields small excerpts instead of a full decoded copy.
func escapeClusterShadows(view string, re *regexp.Regexp, decode func(string) string) []string {
	locs := re.FindAllStringIndex(view, -1)
	if len(locs) < minEscapeCluster {
		return nil
	}
	var out []string
	for i := 0; i < len(locs); {
		j := i
		for j+1 < len(locs) && locs[j+1][0]-locs[j][1] <= escapeClusterGap {
			j++
		}
		if j-i+1 >= minEscapeCluster {
			start := max(0, locs[i][0]-escapeClusterContext)
			end := min(len(view), locs[j][1]+escapeClusterContext)
			for start > 0 && !utf8.RuneStart(view[start]) {
				start--
			}
			for end < len(view) && !utf8.RuneStart(view[end]) {
				end++
			}
			out = append(out, decode(view[start:end]))
			if len(out) >= maxShadowsPerKind {
				break
			}
		}
		i = j + 1
	}
	return out
}

// percentDecode decodes valid %XX escapes and form-encoded '+' spaces and
// leaves malformed escapes as they are.
func percentDecode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '%' && i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2]):
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
		case c == '+':
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

// liftShadowAnalysis raises every Medium-or-higher finding of a decode shadow
// to decodeShadowSeverity and records the encoding, so text that was hidden on
// purpose reaches ThreatHigh even when its own wording only scores Medium.
// A shadow without such a finding is returned unchanged.
func liftShadowAnalysis(analysis *promptsec.Result) *promptsec.Result {
	lifted := false
	threats := make([]promptsec.Threat, len(analysis.Threats), len(analysis.Threats)+1)
	copy(threats, analysis.Threats)
	for i := range threats {
		if threats[i].Severity >= 0.3 {
			lifted = true
			if threats[i].Severity < decodeShadowSeverity {
				threats[i].Severity = decodeShadowSeverity
			}
		}
	}
	if !lifted {
		return analysis
	}
	threats = append(threats, promptsec.Threat{
		Type:     promptsec.ThreatEncodingAttack,
		Severity: decodeShadowSeverity,
		Message:  "encoded payload decodes to text with injection signals",
		Guard:    "guardian",
	})
	out := *analysis
	out.Safe = false
	out.Threats = threats
	return &out
}
