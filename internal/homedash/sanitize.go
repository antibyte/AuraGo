package homedash

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxAttrStringRunes = 256
	MaxAttrListItems   = 64
	MaxAttrBytes       = 8 << 10
	MaxStateTextRunes  = 255
	MaxNameRunes       = 120
	MaxCatalogEntities = 2000
)

var attrKeyRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

func isBidiControl(r rune) bool {
	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || r == 0x200E || r == 0x200F || r == 0x061C
}

// CleanText drops invalid UTF-8, control and bidi characters, folds whitespace runs to one space,
// trims, and cuts to max runes (max <= 0: no cut).
func CleanText(s string, max int) string {
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	n, pending := 0, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			pending = b.Len() > 0
			continue
		}
		if unicode.IsControl(r) || isBidiControl(r) {
			continue
		}
		if pending {
			if max > 0 && n+1 >= max {
				break
			}
			b.WriteByte(' ')
			n++
			pending = false
		}
		if max > 0 && n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// ContainsControl reports invalid UTF-8, control or bidi characters.
func ContainsControl(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if unicode.IsControl(r) || isBidiControl(r) {
			return true
		}
	}
	return false
}

// ContainsURL reports text that carries a URL or script scheme; documents accept no URLs.
func ContainsURL(s string) bool {
	l := strings.ToLower(s)
	for _, m := range []string{"://", "javascript:", "data:", "vbscript:"} {
		if strings.Contains(l, m) {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimSpace(l), "//")
}

// BoundAttributes keeps scalar and list-of-scalar values under keys matching ^[a-z0-9_]{1,64}$,
// cleans strings (256 runes), cuts lists (64), drops maps and non-finite numbers, and keeps keys
// in sorted order while the JSON stays within 8 KiB.
func BoundAttributes(in map[string]any) map[string]any {
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out, size := map[string]any{}, 2
	for _, k := range keys {
		if !attrKeyRe.MatchString(k) {
			continue
		}
		v, ok := boundValue(in[k], true)
		if !ok {
			continue
		}
		enc, err := json.Marshal(v)
		if err != nil || size+len(k)+4+len(enc) > MaxAttrBytes {
			continue
		}
		out[k], size = v, size+len(k)+4+len(enc)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func boundValue(v any, allowList bool) (any, bool) {
	switch t := v.(type) {
	case nil, bool:
		return t, true
	case string:
		return CleanText(t, MaxAttrStringRunes), true
	case float64, int, int64, float32, json.Number:
		var f float64
		switch n := t.(type) {
		case float64:
			f = n
		case float32:
			f = float64(n)
		case int:
			f = float64(n)
		case int64:
			f = float64(n)
		case json.Number:
			var err error
			if f, err = n.Float64(); err != nil {
				return nil, false
			}
		}
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case []string:
		list := make([]any, len(t))
		for i, s := range t {
			list[i] = s
		}
		return boundValue(list, allowList)
	case []any:
		if !allowList {
			return nil, false
		}
		if len(t) > MaxAttrListItems {
			t = t[:MaxAttrListItems]
		}
		out := make([]any, 0, len(t))
		for _, e := range t {
			if b, ok := boundValue(e, false); ok {
				out = append(out, b)
			}
		}
		return out, true
	}
	return nil, false
}
