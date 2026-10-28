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
// visible: every Cf rune plus the invisible characters that are not Cf (the
// fillers and the variation selectors, which render as nothing between letters).
func isScanSeparator(r rune) bool {
	if r < 0x80 {
		return false
	}
	switch {
	case r == 0x034F, // combining grapheme joiner
		r == 0x115F, r == 0x1160, r == 0x3164, r == 0xFFA0, // Hangul fillers
		r == 0x17B4, r == 0x17B5, // Khmer inherent vowels
		r == 0x2800,                  // braille blank
		r >= 0x180B && r <= 0x180D,   // Mongolian free variation selectors
		r == 0x180F,                  // Mongolian free variation selector four
		r >= 0xFE00 && r <= 0xFE0F,   // variation selectors
		r >= 0xE0100 && r <= 0xE01EF: // variation selectors supplement
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
// injection signal is at least decodeShadowSeverity (ThreatHigh), the
// sanitizer's own severity for a decoded block, while a shadow with no signal
// adds nothing.
//
// A window yields at most four shadows (base64, percent escapes, entities and
// anchored ROT13), none longer than the window. No shadow is
// ever dropped for its position or for an earlier one. The shadows of one scan
// are budgeted (maxDecodeShadowTextFactor times the scanned text plus a few
// windows), which bounds the cost at about four reads of the text: as written,
// as view and the shadows. Beyond the budget the scan is incomplete
// (ScanResult.Incomplete), never silently shorter.
const (
	decodeShadowSeverity = 0.7
	// A page dense in two encodings (each shadow spans the page with its context)
	// needs about 1.8 times its own size in shadows; only text built to be dense in
	// all three goes beyond the budget.
	maxDecodeShadowTextFactor = 2
	maxDecodeShadowWindows    = 4
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
// not produce, one shadow per encoding: unpadded and URL-safe base64 runs of 16
// or more characters whose decoded text is mostly printable (joined by
// newlines), percent-encoded clusters and HTML numeric entity clusters. Benign
// runs yield a shadow too; the caller scans it and only a finding changes the
// level.
func decodeShadows(view string) []string {
	var shadows []string
	add := func(s string) {
		if s != "" && s != view {
			shadows = append(shadows, s)
		}
	}

	var runs []string
	for _, loc := range base64Run.FindAllStringIndex(view, -1) {
		if decoded, ok := decodeBase64Run(view[loc[0]:loc[1]]); ok {
			runs = append(runs, decoded)
		}
	}
	add(strings.Join(runs, "\n"))
	add(escapeClusterShadow(view, percentEscape, percentDecode))
	add(escapeClusterShadow(view, numericEntity, html.UnescapeString))
	if rot13Anchor.MatchString(view) {
		add(strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return 'a' + (r-'a'+13)%26
			case r >= 'A' && r <= 'Z':
				return 'A' + (r-'A'+13)%26
			}
			return r
		}, view))
	}
	return shadows
}

var rot13Anchor = regexp.MustCompile(`(?i)\b(?:vtaber|vtabevrer|vtaberm|cerivbhf|vafgehpgvbaf|flfgrz|cebzcg|nffvfgnag|pungtcg|najrvfhatra)\b`)

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

// escapeClusterShadow decodes the excerpts of view around every cluster of
// matches of re and joins them with newlines (excerpts that touch are one).
// Encoded phrases stay local, so a page with many unrelated escapes (URLs)
// yields small excerpts instead of a full decoded copy, and no cluster is left
// out however many came before it.
func escapeClusterShadow(view string, re *regexp.Regexp, decode func(string) string) string {
	locs := re.FindAllStringIndex(view, -1)
	if len(locs) < minEscapeCluster {
		return ""
	}
	var spans [][2]int
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
			if n := len(spans); n > 0 && start <= spans[n-1][1] {
				spans[n-1][1] = max(spans[n-1][1], end)
			} else {
				spans = append(spans, [2]int{start, end})
			}
		}
		i = j + 1
	}
	parts := make([]string, len(spans))
	for i, span := range spans {
		parts[i] = decode(view[span[0]:span[1]])
	}
	return strings.Join(parts, "\n")
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

// liftableShadowThreat reports whether a shadow finding says something an
// attacker wants the model to do: an override, a disclosure request, a role or a
// delimiter forgery. The sanitizer's notes on what it normalized (Cyrillic or
// zero-width text, encoding_attack at 0.3) are not such a finding: ordinary
// Russian text decoded from a percent-encoded link scores them, and lifting them
// would withhold the page and refuse the chat message that carries the link.
func liftableShadowThreat(t promptsec.Threat) bool {
	switch t.Type {
	case promptsec.ThreatInstructionOverride, promptsec.ThreatSystemPromptLeak,
		promptsec.ThreatRoleManipulation, promptsec.ThreatDelimiterInjection:
		return t.Severity >= 0.3
	}
	return false
}

// liftShadowAnalysis raises every Medium-or-higher injection finding of a decode
// shadow (liftableShadowThreat) to decodeShadowSeverity and records the
// encoding, so text that was hidden on purpose reaches ThreatHigh even when its
// own wording only scores Medium. Any other finding keeps its severity, and a
// shadow without a liftable finding is returned unchanged.
func liftShadowAnalysis(analysis *promptsec.Result) *promptsec.Result {
	lifted := false
	threats := make([]promptsec.Threat, len(analysis.Threats), len(analysis.Threats)+1)
	copy(threats, analysis.Threats)
	for i := range threats {
		if liftableShadowThreat(threats[i]) {
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
