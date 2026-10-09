package xapian

import (
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// normalizers pools the transform chain equivalent to libzim's ICU
// transliterator "Lower; NFD; [:M:] remove; NFC". Like ICU's Lower, the
// root-locale lowercaser applies full case mapping including Greek final sigma.
var normalizers = sync.Pool{New: func() any {
	return transform.Chain(
		cases.Lower(language.Und),
		norm.NFD,
		runes.Remove(runes.In(unicode.M)),
		norm.NFC,
	)
}}

// Normalize lowercases s, strips all combining marks (Mn, Mc, Me) after
// canonical decomposition and recomposes, exactly like libzim's
// removeAccents() applied to titles, article text and queries.
func Normalize(s string) string {
	s = strings.ToValidUTF8(s, "�")
	t := normalizers.Get().(transform.Transformer)
	defer normalizers.Put(t)
	t.Reset()
	out, _, err := transform.String(t, s)
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}
