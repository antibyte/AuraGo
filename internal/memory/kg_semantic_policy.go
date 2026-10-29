package memory

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"aurago/internal/security"

	chromem "github.com/philippgille/chromem-go"
)

// The knowledge-graph semantic index embeds text that kgsemantic writes as one
// "key: value" line per property, so credential words appear as ordinary
// property names ("pin: 17", "auth: OAuth2", "password: stored in vault").
// HasDurableCredential judges every "<keyword>: <word>" as a credential, which
// withheld such nodes and edges from semantic search. The KG check below keeps
// every strong signal of HasDurableCredential and only looks at the value of a
// plain keyword match before the scrubber's keyword patterns get to see it.

var (
	// kgKeywordValue finds "<keyword><sep><value>" pairs. The keyword list is the
	// one of security's apiKeyRegex; group 3 is the first token of the value
	// without its opening quote.
	kgKeywordValue = regexp.MustCompile(`(?i)\b(key|secret|password|passwd|pwd|pin|token|auth|credential|api_key|master_key|bot_token)\b(["']?\s*[:=]\s*["']?)([^\s"']*)`)
	// kgFragmentedValue recognizes a value whose characters are spread out one by
	// one ("s k - 1 2 3 4 5 6 7 8"), the obfuscation security's
	// fragmentedSecretRegex exists for.
	kgFragmentedValue = regexp.MustCompile(`^(?:[A-Za-z0-9][\s_:\-+=]{1,3}){7,}[A-Za-z0-9]`)
)

const (
	// kgMaxPlainWordRunes keeps a long letters-only string (which can be a random
	// password) from passing as prose.
	kgMaxPlainWordRunes = 24
	// kgMaxAcronymRunes is the longest all-upper-case word ("JWT", "OAUTH") that
	// passes as an acronym.
	kgMaxAcronymRunes = 6
	// kgMaxNumberDigits is the longest number ("12", "90") that passes as prose.
	kgMaxNumberDigits = 3
	// kgShortValueRunes is the length below which a key/auth/pin value is a label
	// or a number ("OAuth2", "17") instead of a credential.
	kgShortValueRunes = 8
	// kgPINDigits is the digit count from which a pin value is a door PIN.
	kgPINDigits = 4
)

// kgSemanticDocumentAllowed applies the KG credential policy to a document that
// chromem is about to embed: its content and every metadata pair.
func kgSemanticDocumentAllowed(doc chromem.Document) error {
	if kgSemanticHasCredential(doc.Content) {
		return fmt.Errorf("%w: credentials require encrypted Vault storage", ErrAutomaticMemorySkipped)
	}
	for key, value := range doc.Metadata {
		if kgSemanticHasCredential(key + "=" + value) {
			return fmt.Errorf("%w: credentials require encrypted Vault storage", ErrAutomaticMemorySkipped)
		}
	}
	return nil
}

// kgSemanticHasCredential is HasDurableCredential for knowledge-graph text: the
// same registered-secret, key-material, placeholder and scrubber checks, except
// that a plain "<keyword>: <value>" whose value is not credential-like (see
// kgKeywordValueHarmless) is neutralized first.
func kgSemanticHasCredential(text string) bool {
	if security.Scrub(text) != text || durableKeyMaterial.MatchString(text) {
		return true
	}
	// Keep in step with HasDurableCredential.
	candidate := durableObjectKey.ReplaceAllString(text, `_memory_object_key${1}`)
	candidate = durableTokenOption.ReplaceAllString(candidate, `_memory_option${1}${2}`)
	candidate = durableTemplateValue.ReplaceAllString(candidate, `${1}${2}${3}[redacted]${5}`)
	candidate = durableEnvironmentReference.ReplaceAllString(candidate, `_memory_reference${1}environment`)
	candidate = neutralizeHarmlessKGKeywords(candidate)
	return security.RedactSensitiveInfo(candidate) != candidate
}

// neutralizeHarmlessKGKeywords prefixes the keyword of every harmless
// "<keyword>: <value>" pair with "_kg_", so the scrubber's word-boundary
// patterns no longer take it for a credential field. The value stays in place:
// URL credentials, bearer tokens, key material and the other value patterns
// still apply to it.
func neutralizeHarmlessKGKeywords(text string) string {
	matches := kgKeywordValue.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	var out strings.Builder
	last := 0
	for _, m := range matches {
		if !kgKeywordValueHarmless(text[m[2]:m[3]], text[m[6]:]) {
			continue
		}
		out.WriteString(text[last:m[2]])
		out.WriteString("_kg_")
		last = m[2]
	}
	out.WriteString(text[last:])
	return out.String()
}

// kgKeywordValueHarmless judges the whole value of a keyword up to the end of
// the line (tail starts at the value). It is harmless only if every token is:
// a plain word ("stored", "none", "Google"), a number of at most three digits
// ("12", "90") or, for the ambiguous keywords key/auth/pin, a token shorter than
// kgShortValueRunes ("OAuth2", "17"). A pin with four or more digits stays a
// credential (a door PIN), a basic/bearer/digest scheme name followed by another
// token introduces a credential, and so does a value spread out character by
// character. A value made only of plain words ("correct horse battery") passes:
// it cannot be told apart from prose.
func kgKeywordValueHarmless(keyword, tail string) bool {
	if kgFragmentedValue.MatchString(tail) {
		return false
	}
	line := tail
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	var tokens []string
	for _, field := range strings.Fields(line) {
		if token := strings.TrimRight(strings.TrimLeft(field, `"'`), `.,;:!?"'`); token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return false
	}
	keyword = strings.ToLower(keyword)
	for i, token := range tokens {
		if i < len(tokens)-1 && kgAuthSchemeWord(token) {
			return false
		}
		if !kgValueTokenHarmless(keyword, token) {
			return false
		}
	}
	return true
}

// kgAuthSchemeWord reports whether token names an HTTP authentication scheme,
// which is followed by a credential.
func kgAuthSchemeWord(token string) bool {
	return strings.EqualFold(token, "basic") || strings.EqualFold(token, "bearer") || strings.EqualFold(token, "digest")
}

// kgValueTokenHarmless judges one token of a keyword's value (keyword is lower case).
func kgValueTokenHarmless(keyword, token string) bool {
	if kgWordShaped(token) || kgSmallNumber(token) {
		return true
	}
	if keyword != "key" && keyword != "auth" && keyword != "pin" {
		return false
	}
	if len([]rune(token)) >= kgShortValueRunes {
		return false
	}
	if keyword == "pin" {
		digits := 0
		for _, r := range token {
			if unicode.IsDigit(r) {
				digits++
			}
		}
		if digits >= kgPINDigits {
			return false
		}
	}
	return true
}

// kgWordShaped reports whether token looks like a word: letters only, two to
// kgMaxPlainWordRunes of them, and written in lower case, capitalized or as an
// acronym of up to kgMaxAcronymRunes upper case letters. Random mixed-case
// strings ("XkqTzRmPwLbQ") are not word-shaped.
func kgWordShaped(token string) bool {
	n, upper, lower := 0, 0, 0
	restUpper := false // an upper case letter after the first rune
	for _, r := range token {
		if !unicode.IsLetter(r) {
			return false
		}
		if unicode.IsUpper(r) {
			upper++
			if n > 0 {
				restUpper = true
			}
		} else if unicode.IsLower(r) {
			lower++
		}
		n++
	}
	if n < 2 || n > kgMaxPlainWordRunes {
		return false
	}
	switch {
	case upper == 0: // lower case
		return true
	case !restUpper: // capitalized
		return true
	default: // acronym
		return lower == 0 && n <= kgMaxAcronymRunes
	}
}

// kgSmallNumber reports whether token is a number of at most kgMaxNumberDigits
// digits.
func kgSmallNumber(token string) bool {
	n := 0
	for _, r := range token {
		if !unicode.IsDigit(r) {
			return false
		}
		n++
	}
	return n >= 1 && n <= kgMaxNumberDigits
}
