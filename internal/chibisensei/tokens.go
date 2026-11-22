package chibisensei

import (
	"math"
	"sort"
	"unicode"
	"unicode/utf8"
)

// EstimateTokens = ceil(UTF-8 bytes of non-CJK runes / 3.6) + 1 per CJK rune + 4 per message. The bytes are
// summed over the whole row and rounded up once. A text row has no message overhead.
func EstimateTokens(msgs []Message, text string) int {
	var bytes, cjk int
	add := func(s string) {
		for _, r := range s {
			if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
				cjk++
			} else {
				bytes += utf8.RuneLen(r)
			}
		}
	}
	for _, m := range msgs {
		add(m.Content)
	}
	add(text)
	return int(math.Ceil(float64(bytes)/3.6)) + cjk + 4*len(msgs)
}

// Percentile is nearest-rank on a sorted copy: v[ceil(q*n)-1]; 0 for an empty slice.
func Percentile(values []int, q float64) int {
	if len(values) == 0 {
		return 0
	}
	v := append([]int(nil), values...)
	sort.Ints(v)
	idx := int(math.Ceil(q*float64(len(v)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(v) {
		idx = len(v) - 1
	}
	return v[idx]
}
