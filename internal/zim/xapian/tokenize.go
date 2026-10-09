package xapian

import "unicode"

// Character classes of Xapian 1.4's TermGenerator/QueryParser.

func isWordChar(r rune) bool {
	return unicode.In(r, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo,
		unicode.Mn, unicode.Me, unicode.Mc, unicode.Nd, unicode.Nl, unicode.No, unicode.Pc)
}

func isWhitespace(r rune) bool {
	return unicode.In(r, unicode.Cc, unicode.Zs, unicode.Zl, unicode.Zp)
}

func isDigit(r rune) bool { return unicode.Is(unicode.Nd, r) }

// isUnbroken reports CJK-family code points that are split into n-grams.
func isUnbroken(r rune) bool {
	switch {
	case r < 0x2E80:
		return false
	case r <= 0x2EFF, r >= 0x3000 && r <= 0x9FFF, r >= 0xA700 && r <= 0xA71F,
		r >= 0xAC00 && r <= 0xD7AF, r >= 0xF900 && r <= 0xFAFF, r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFFEF, r >= 0x20000 && r <= 0x2A6DF, r >= 0x2F800 && r <= 0x2FA1F:
		return true
	}
	return false
}

const infixIgnore rune = -1

// checkInfix returns the character kept inside a word when r sits between two
// word characters (0 = word break, infixIgnore = drop r but keep joining).
func checkInfix(r rune) rune {
	switch r {
	case '\'', '&', 0xb7, 0x5f4, 0x2027:
		return r
	case 0x2019, 0x201b:
		return '\''
	case 0x200b, 0x200c, 0x200d, 0x2060, 0xfeff:
		return infixIgnore
	}
	return 0
}

// checkInfixDigit is checkInfix between two decimal digits.
func checkInfixDigit(r rune) rune {
	switch r {
	case ',', '.', ';', 0x037e, 0x0589, 0x060d, 0x07f8, 0x2044, 0xfe10, 0xfe13, 0xfe14:
		return r
	case 0x200b, 0x200c, 0x200d, 0x2060, 0xfeff:
		return infixIgnore
	}
	return 0
}

func isSuffix(r rune) bool { return r == '+' || r == '#' }

// isPhraseGenerator: these characters between words make a phrase in queries.
func isPhraseGenerator(r rune) bool {
	switch r {
	case '.', '-', '/', ':', '\\', '@':
		return true
	}
	return false
}

// isStemPreventer: a query word followed by one of these is not stemmed (STEM_SOME).
func isStemPreventer(r rune) bool {
	switch r {
	case '(', '/', '\\', '@', '<', '>', '=', '*', '[', '{', '"':
		return true
	}
	return false
}

// shouldStem: STEM_SOME stems a word only if it starts with a letter of
// category Ll, Lt, Lm or Lo.
func shouldStem(term string) bool {
	for _, r := range term {
		return unicode.In(r, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo)
	}
	return false
}

// indexToken is one term produced by the TermGenerator emulation.
type indexToken struct {
	text       string
	positional bool // ordinary words and CJK unigrams carry positions; CJK bigrams do not
}

// appendNgrams emits the CJK unigrams and bigrams of span in Xapian order:
// c1, c1c2, c2, c2c3, ..., cn.
func appendNgrams(out []indexToken, span []rune) []indexToken {
	for k := range span {
		out = append(out, indexToken{text: string(span[k]), positional: true})
		if k+1 < len(span) {
			out = append(out, indexToken{text: string(span[k : k+2]), positional: false})
		}
	}
	return out
}

// indexTokens splits already-normalised text the way Xapian 1.4's
// TermGenerator does with FLAG_CJK_NGRAM (index side).
func indexTokens(s string) []indexToken {
	rs := []rune(s)
	n := len(rs)
	var out []indexToken
	i := 0
	for {
		for i < n && !isWordChar(rs[i]) {
			i++
		}
		if i >= n {
			return out
		}
		var term []rune
		ch := unicode.ToLower(rs[i])
		ended := false
	word:
		for {
			if isUnbroken(rs[i]) && isWordChar(rs[i]) {
				j := i
				for j < n && isUnbroken(rs[j]) && isWordChar(rs[j]) {
					j++
				}
				out = appendNgrams(out, rs[i:j])
				i = j
				for i < n && !isWordChar(rs[i]) {
					i++
				}
				if i >= n {
					return out
				}
				ch = unicode.ToLower(rs[i])
				continue
			}
			var prev rune
			for {
				term = append(term, ch)
				prev = ch
				i++
				if i >= n || isUnbroken(rs[i]) {
					ended = true
					break word
				}
				if !isWordChar(rs[i]) {
					break
				}
				ch = unicode.ToLower(rs[i])
			}
			if i+1 >= n || !isWordChar(rs[i+1]) {
				break
			}
			next := rs[i+1]
			var infix rune
			if isDigit(prev) && isDigit(next) {
				infix = checkInfixDigit(rs[i])
			} else {
				infix = checkInfix(rs[i])
			}
			if infix == 0 {
				break
			}
			if infix != infixIgnore {
				term = append(term, infix)
			}
			ch = unicode.ToLower(next)
			i++
		}
		if !ended {
			base := len(term)
			count := 0
			for i < n && isSuffix(rs[i]) {
				count++
				if count > 3 {
					term = term[:base]
					break
				}
				term = append(term, rs[i])
				i++
				if i >= n {
					ended = true
					break
				}
			}
			if !ended && i < n && isWordChar(rs[i]) {
				term = term[:base]
			}
		}
		out = append(out, indexToken{text: string(term), positional: true})
	}
}

// positionalTerms keeps the terms that receive a position, in position order
// (slice index = position - 1). Like TermGenerator it skips terms longer than
// maxLen bytes without consuming a position; CJK bigrams carry no position.
func positionalTerms(tokens []indexToken, maxLen int) []string {
	var out []string
	for _, t := range tokens {
		if len(t.text) > maxLen || !t.positional {
			continue
		}
		out = append(out, t.text)
	}
	return out
}

// queryWord is one word of a query as Xapian 1.4's QueryParser lexes it.
type queryWord struct {
	text    string // lowercased
	cjk     bool   // unbroken-script run: expands to n-grams
	phrased bool   // joined to the previous word by phrase-generator characters
	atEnd   bool   // ends exactly at the end of the query string
	stemOK  bool   // not followed by a stem-preventer character
}

// queryWords lexes a normalised query. Quotes, brackets, love/hate markers
// and boolean operators are treated as separators (libzim lowercases queries,
// so upper-case operators never occur).
func queryWords(s string) []queryWord {
	rs := []rune(s)
	n := len(rs)
	var out []queryWord
	i := 0
	phrased := false
	for i < n {
		if isWhitespace(rs[i]) || !isWordChar(rs[i]) {
			i++
			phrased = false
			continue
		}
		w := queryWord{phrased: phrased}
		phrased = false
		if isUnbroken(rs[i]) {
			j := i
			for j < n && isUnbroken(rs[j]) && isWordChar(rs[j]) {
				j++
			}
			w.text, w.cjk = string(rs[i:j]), true
			i = j
			w.atEnd = i >= n
			w.stemOK = true
			out = append(out, w)
			continue
		}
		term := []rune{rs[i]}
		prev := rs[i]
		i++
		for i < n {
			if isUnbroken(rs[i]) {
				break
			}
			ch := rs[i]
			if !isWordChar(ch) {
				if i+1 >= n || !isWordChar(rs[i+1]) {
					break
				}
				if isDigit(prev) && isDigit(rs[i+1]) {
					ch = checkInfixDigit(ch)
				} else {
					ch = checkInfix(ch)
				}
				if ch == 0 {
					break
				}
				if ch == infixIgnore {
					i++
					continue
				}
			}
			term = append(term, ch)
			prev = ch
			i++
		}
		if i < n && isSuffix(rs[i]) {
			suff := append([]rune(nil), term...)
			p := i
			for p < n && isSuffix(rs[p]) {
				if len(suff)-len(term) == 3 {
					suff = nil
					break
				}
				suff = append(suff, rs[p])
				p++
			}
			if suff != nil && (p >= n || !isWordChar(rs[p])) {
				term, i = suff, p
			}
		}
		w.text = lowerRunes(term)
		w.atEnd = i >= n
		w.stemOK = !(i < n && isStemPreventer(rs[i]))
		out = append(out, w)
		if i < n && isPhraseGenerator(rs[i]) {
			for i < n && isPhraseGenerator(rs[i]) {
				i++
			}
			if i < n && isWordChar(rs[i]) {
				phrased = true
			}
		}
	}
	return out
}

func lowerRunes(rs []rune) string {
	out := make([]rune, len(rs))
	for i, r := range rs {
		out[i] = unicode.ToLower(r)
	}
	return string(out)
}

// ngrams returns the CJK n-gram terms of a word (unigrams and bigrams).
func ngrams(word string) []indexToken {
	return appendNgrams(nil, []rune(word))
}
