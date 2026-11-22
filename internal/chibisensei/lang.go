package chibisensei

import (
	"sort"
	"strings"
	"unicode"
)

var latinStopwords = map[string][]string{
	"en": {"the", "and", "is", "are", "you", "of", "to", "what", "how", "with"},
	"de": {"der", "die", "das", "und", "ist", "nicht", "ich", "du", "wie", "mit"},
	"fr": {"le", "la", "les", "et", "est", "vous", "je", "pas", "une", "des"},
	"es": {"el", "los", "las", "es", "y", "que", "una", "por", "para", "como"},
	"it": {"il", "lo", "gli", "che", "è", "sono", "per", "non", "come", "della"},
	"pt": {"o", "os", "é", "não", "você", "uma", "com", "do", "da", "em"},
	"nl": {"de", "het", "een", "en", "niet", "ik", "je", "van", "wat", "zijn"},
	"pl": {"i", "w", "nie", "się", "to", "jest", "na", "że", "jak", "co"},
}

// DetectLanguage returns an ISO 639-1 code or "" (unknown). Letters are counted per script. Fewer than 20
// letters, or no script with at least 50 % of the letters, gives "". Han and Kana count together and give ja
// when Kana is at least 10 % of the letters, otherwise zh. Latin text is scored against latinStopwords: the
// winner needs at least 3 hits and must beat the runner-up by at least 1.
func DetectLanguage(texts []string) string {
	var total, han, kana, hangul, cyr, greek, arabic, hebrew, deva, thai, latin int
	for _, t := range texts {
		for _, r := range t {
			if !unicode.IsLetter(r) {
				continue
			}
			total++
			switch {
			case unicode.Is(unicode.Han, r):
				han++
			case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
				kana++
			case unicode.Is(unicode.Hangul, r):
				hangul++
			case unicode.Is(unicode.Cyrillic, r):
				cyr++
			case unicode.Is(unicode.Greek, r):
				greek++
			case unicode.Is(unicode.Arabic, r):
				arabic++
			case unicode.Is(unicode.Hebrew, r):
				hebrew++
			case unicode.Is(unicode.Devanagari, r):
				deva++
			case unicode.Is(unicode.Thai, r):
				thai++
			case unicode.Is(unicode.Latin, r):
				latin++
			}
		}
	}
	if total < 20 {
		return ""
	}
	half := func(n int) bool { return n*2 >= total }
	switch {
	case half(han + kana):
		if kana*10 >= total {
			return "ja"
		}
		return "zh"
	case half(cyr):
		return "ru"
	case half(greek):
		return "el"
	case half(arabic):
		return "ar"
	case half(hebrew):
		return "he"
	case half(deva):
		return "hi"
	case half(thai):
		return "th"
	case half(hangul):
		return "ko"
	case half(latin):
		return detectLatin(texts)
	}
	return ""
}

func detectLatin(texts []string) string {
	words := map[string]int{}
	for _, t := range texts {
		for _, w := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !unicode.IsLetter(r) }) {
			words[w]++
		}
	}
	type score struct {
		lang string
		hits int
	}
	var scores []score
	for lang, list := range latinStopwords {
		h := 0
		for _, w := range list {
			h += words[w]
		}
		scores = append(scores, score{lang, h})
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].hits != scores[j].hits {
			return scores[i].hits > scores[j].hits
		}
		return scores[i].lang < scores[j].lang
	})
	if scores[0].hits >= 3 && scores[0].hits > scores[1].hits {
		return scores[0].lang
	}
	return ""
}

func cutRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// LanguageSample is the goal text plus the user and assistant contents (or the text) of the first 200 rows by
// seq, each cut to 500 runes.
func LanguageSample(goal string, rows []Row) []string {
	var out []string
	if strings.TrimSpace(goal) != "" {
		out = append(out, cutRunes(goal, 500))
	}
	sorted := append([]Row(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Seq < sorted[j].Seq })
	if len(sorted) > 200 {
		sorted = sorted[:200]
	}
	for _, r := range sorted {
		if r.Text != nil {
			out = append(out, cutRunes(*r.Text, 500))
			continue
		}
		for _, m := range r.Messages {
			if m.Role == "user" || m.Role == "assistant" {
				out = append(out, cutRunes(m.Content, 500))
			}
		}
	}
	return out
}
