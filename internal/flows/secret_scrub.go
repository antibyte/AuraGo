package flows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"slices"
	"strings"
	"unicode/utf16"
)

// redactedText replaces a secret in text a node returns.
const redactedText = "[redacted]"

// secretScrubber removes one secret value from what a node returns. A server can
// echo the request headers back (a debug endpoint, an error page) and the tool puts
// its own error text, URL and server answers into its output, so a secret a node
// sent could otherwise end up in the run record, in a later AI prompt or in a
// message.
//
// It removes the value as it is and in the encodings an echo usually has:
//   - JSON string escapes, with and without HTML escaping (Go writes < for "<"),
//     with and without PHP's "\/" for "/", and with non-ASCII characters written as
//     \uXXXX (the default of Python's json.dumps and PHP's json_encode), in lower
//     and upper case hex;
//   - HTML entities, as html.EscapeString and as PHP's htmlspecialchars write them;
//   - URL escapes (query form with "+" or "%20", and path form), in upper and lower
//     case hex.
//
// Other transformations are not recognised: base64 of "user:password", a hash,
// JSON inside JSON, an escape of every character. The invoker registers the vault
// value with the output scrubber as well (see SecretReader).
type secretScrubber struct {
	forms []string
	// replacer removes all forms in one pass, so the replacement text is never
	// searched again (a short secret can be part of "[redacted]" itself).
	replacer *strings.Replacer
}

// newSecretScrubber returns a scrubber for secret; for an empty secret it removes nothing.
func newSecretScrubber(secret string) *secretScrubber {
	s := &secretScrubber{}
	if secret == "" {
		return s
	}
	add := func(forms ...string) {
		for _, form := range forms {
			if form != "" && !slices.Contains(s.forms, form) {
				s.forms = append(s.forms, form)
			}
		}
	}
	add(secret)
	add(jsonEscapedForms(secret)...)
	add(htmlEscapedForms(secret)...)
	add(urlEscapedForms(secret)...)
	// Longest first, so that an escaped form is not broken up by its shorter relative.
	slices.SortStableFunc(s.forms, func(a, b string) int { return len(b) - len(a) })
	oldnew := make([]string, 0, 2*len(s.forms))
	for _, form := range s.forms {
		oldnew = append(oldnew, form, redactedText)
	}
	s.replacer = strings.NewReplacer(oldnew...)
	return s
}

// jsonEscapedForms returns how a JSON encoder can write secret inside a string:
// every combination of HTML escaping, PHP's escaped slash and \uXXXX for non-ASCII.
func jsonEscapedForms(secret string) []string {
	var forms []string
	for _, escapeHTML := range []bool{true, false} {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(escapeHTML)
		if enc.Encode(secret) != nil {
			continue
		}
		quoted := strings.TrimSuffix(buf.String(), "\n")
		if len(quoted) < 2 {
			continue
		}
		inner := quoted[1 : len(quoted)-1]
		forms = append(forms, inner, strings.ReplaceAll(inner, "/", `\/`))
	}
	for _, form := range slices.Clone(forms) {
		forms = append(forms, asciiJSON(form, false), asciiJSON(form, true))
	}
	return forms
}

// asciiJSON writes every non-ASCII character of s as \uXXXX (a surrogate pair above
// U+FFFF), with lower or upper case hex digits.
func asciiJSON(s string, upper bool) string {
	digits := `\u%04x`
	if upper {
		digits = `\u%04X`
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r >= 0x10000:
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&b, digits+digits, hi, lo)
		default:
			fmt.Fprintf(&b, digits, r)
		}
	}
	return b.String()
}

func htmlEscapedForms(secret string) []string {
	php := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#039;")
	return []string{html.EscapeString(secret), php.Replace(secret)}
}

func urlEscapedForms(secret string) []string {
	query := url.QueryEscape(secret)
	forms := []string{query, strings.ReplaceAll(query, "+", "%20"), url.PathEscape(secret)}
	for _, form := range slices.Clone(forms) {
		forms = append(forms, lowerPercent(form))
	}
	return forms
}

// lowerPercent writes the hex digits of every %XX escape in lower case.
func lowerPercent(s string) string {
	b := []byte(s)
	for i := 0; i+2 < len(b); i++ {
		if b[i] != '%' {
			continue
		}
		for j := i + 1; j <= i+2; j++ {
			if b[j] >= 'A' && b[j] <= 'F' {
				b[j] += 'a' - 'A'
			}
		}
		i += 2
	}
	return string(b)
}

// text replaces every occurrence of the secret in v. cut says that v was cut at an
// arbitrary place (a message cut to its rune limit, a response body cut at the
// tool's size limit): then the start of the secret at its end, which no full match
// covers, is dropped as well, like the rest of the secret the cut removed. A cut in
// the middle of a multi-byte character leaves U+FFFD behind once the text went
// through a JSON encoder or ToValidUTF8, so those are dropped first. The ellipsis
// that ends a cut message stays. Only pass cut for a text that really was cut: the
// end of an intact text may start like the secret by chance, and it would lose those
// characters.
func (s *secretScrubber) text(v string, cut bool) string {
	if s == nil || len(s.forms) == 0 || v == "" {
		return v
	}
	v = s.replacer.Replace(v)
	if !cut {
		return v
	}
	base := strings.TrimSuffix(v, "…")
	ellipsis := v[len(base):]
	kept := strings.TrimRight(base, "�")
	longest := 0
	for _, form := range s.forms {
		for k := min(len(form)-1, len(kept)); k > longest; k-- {
			if strings.HasSuffix(kept, form[:k]) {
				longest = k
				break
			}
		}
	}
	if longest == 0 && len(kept) == len(base) {
		return v
	}
	return kept[:len(kept)-longest] + ellipsis
}

// value removes the secret from every text inside v, in place: v must be data this
// node owns (a freshly parsed tool answer), never a parameter or an input. Object
// keys are left alone, because the keys of a tool answer are the tool's own and a
// short secret must not rename them; valueKeys scrubs them too.
func (s *secretScrubber) value(v any) any { return s.deep(v, false) }

// valueKeys is value for data that came from the server, where an object key can
// carry the secret as well. Keys that change are renamed; two keys that end up
// with the same name merge into one entry.
func (s *secretScrubber) valueKeys(v any) any { return s.deep(v, true) }

func (s *secretScrubber) deep(v any, keys bool) any {
	if s == nil || len(s.forms) == 0 {
		return v
	}
	switch x := v.(type) {
	case string:
		return s.text(x, false)
	case map[string]any:
		var renamed []string
		for k, item := range x {
			x[k] = s.deep(item, keys)
			if keys && s.text(k, false) != k {
				renamed = append(renamed, k)
			}
		}
		for _, k := range renamed {
			item := x[k]
			delete(x, k)
			x[s.text(k, false)] = item
		}
	case []any:
		for i, item := range x {
			x[i] = s.deep(item, keys)
		}
	}
	return v
}

// err returns err with the secret removed from its message. The message of a tool
// error is cut to maxToolMessageRunes before the node sees it (truncateRunes ends a
// cut text with an ellipsis), so a message with an ellipsis is scrubbed as a cut
// text. A nil error stays nil; the code is kept.
func (s *secretScrubber) err(err error) error {
	if err == nil || s == nil || len(s.forms) == 0 {
		return err
	}
	ne := asNodeError(err)
	return &NodeError{Code: ne.Code, Message: s.text(ne.Message, strings.HasSuffix(ne.Message, "…"))}
}
