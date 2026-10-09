package xapian

import "testing"

func TestStemmerParityGolden(t *testing.T) {
	var g goldenAnalysis
	loadJSON(t, "analysis.json", &g)
	for lang, pairs := range g.Stems {
		a := NewAnalyzer(lang)
		if a.stem == nil {
			t.Errorf("language %s has no Go stemmer", lang)
			continue
		}
		for _, p := range pairs {
			if got := a.Stem(p[0]); got != p[1] {
				t.Errorf("%s: Stem(%q) = %q, Xapian %q", lang, p[0], got, p[1])
			}
		}
	}
}

// Every word of the fixture articles, tokenised like libzim's indexer and
// stemmed by the Go analyzer, must exist as a term of the libzim-built
// full-text index (spec: stemmer parity test).
func TestStemmerParityAgainstIndex(t *testing.T) {
	var g goldenAnalysis
	loadJSON(t, "analysis.json", &g)
	for name, words := range g.IndexWords {
		db := openFixture(t, name, "fulltext")
		a := NewAnalyzer(fixtureLanguage[name])
		for _, w := range words {
			term := w
			if r := []rune(w); !isUnbroken(r[0]) {
				term = a.Stem(w)
			}
			tf, err := db.TermFreq(term)
			if err != nil || tf == 0 {
				t.Errorf("%s: word %q -> term %q missing from the full-text index (%v)", name, w, term, err)
			}
		}
	}
}
