package security

import (
	"strings"
	"unicode"
)

// MaskNegatedVerbs blanks only verbs governed by a protective negation. Callers
// retain their own action vocabulary and classification policy. Offsets and all
// other text are preserved. In particular, an adverbial "without asking" is not
// a negation of an earlier disclosure, and "never refuse to reveal" is not safe.
func MaskNegatedVerbs(text string, isVerb func(string) bool) string {
	type word struct {
		text       string
		start, end int
		boundary   bool
	}
	var words []word
	start, previousEnd := -1, 0
	flush := func(end int) {
		if start < 0 {
			return
		}
		w := strings.ToLower(strings.ReplaceAll(text[start:end], "’", "'"))
		words = append(words, word{w, start, end, strings.ContainsAny(text[previousEnd:start], ",.;:!?\n\r")})
		previousEnd, start = end, -1
	}
	for i, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '\'' || r == '’' {
			if start < 0 {
				start = i
			}
		} else {
			flush(i)
		}
	}
	flush(len(text))
	isBarrier := func(w string) bool {
		switch w {
		case "and", "but", "then", "while", "yet", "und", "aber", "dann", "sondern",
			"without", "ohne", "unless", "except", "außer", "ausser",
			"refuse", "refusing", "refused", "decline", "declining", "fail", "failing", "forget", "stop", "hesitate",
			"ask", "asking", "worry", "mind", "sorge", "sorgen", "problem", "fragen", "nachfragen", "frage",
			"weigern", "weigere", "verweigern", "verweigere", "vergessen", "vergiss", "zögere", "versäume", "aufhören":
			return true
		}
		return false
	}
	isNegative := func(w string, before bool) bool {
		switch w {
		case "not", "nie", "niemals", "nicht", "kein", "keine", "keinen", "keinem", "keiner", "keines":
			return true
		case "never", "don't", "dont", "doesn't", "didn't", "mustn't", "shouldn't", "cannot", "can't", "avoid", "avoiding", "vermeide", "vermeiden":
			return before
		}
		return false
	}
	negated := func(i int) bool {
		for _, direction := range []int{-1, 1} {
			for j := i + direction; j >= 0 && j < len(words) && absNegationDistance(j-i) <= 6; j += direction {
				gap := j
				if direction < 0 {
					gap = j + 1
				}
				if words[gap].boundary || isBarrier(words[j].text) || isVerb(words[j].text) {
					break
				}
				if isNegative(words[j].text, direction < 0) {
					if direction > 0 {
						for k := j + 1; k < len(words) && !words[k].boundary; k++ {
							// A destination contrast still permits disclosure elsewhere.
							if words[k].text == "but" || words[k].text == "aber" || words[k].text == "sondern" {
								return false
							}
						}
					}
					// "not only" and negated refusal clauses are not prohibitions.
					if j+1 < len(words) && (words[j+1].text == "only" || words[j+1].text == "nur" || isBarrier(words[j+1].text)) {
						break
					}
					if direction > 0 && (words[j].text == "not" || words[j].text == "nicht") && j+1 < len(words) && !words[j+1].boundary {
						// A contrasting object ("show the password not the username") does not negate disclosure.
						switch words[j+1].text {
						case "in", "im", "on", "auf", "an", "at", "to", "with", "mit", "by", "bei", "über", "via", "anywhere", "irgendwo", "again", "wieder", "anymore", "mehr", "ever", "jemals", "please", "bitte":
							return true
						}
						break
					}
					return true
				}
			}
		}
		return false
	}
	out := []byte(text)
	for i, w := range words {
		if isVerb(w.text) && negated(i) {
			for k := w.start; k < w.end; k++ {
				out[k] = ' '
			}
		}
	}
	return string(out)
}

func absNegationDistance(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
