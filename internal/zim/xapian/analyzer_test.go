package xapian

import (
	"reflect"
	"sort"
	"testing"
)

func TestQueryTermsGolden(t *testing.T) {
	var g goldenAnalysis
	loadJSON(t, "analysis.json", &g)
	for _, c := range g.QueryParse {
		got := NewAnalyzer(c.Language).QueryTerms(c.Query)
		uniq := map[string]bool{}
		for _, term := range got {
			uniq[term] = true
		}
		var sorted []string
		for term := range uniq {
			sorted = append(sorted, term)
		}
		sort.Strings(sorted)
		want := c.Terms
		if len(want) == 0 {
			want = nil
		}
		if !reflect.DeepEqual(sorted, want) {
			t.Errorf("QueryTerms(%s, %q) = %q, want %q", c.Language, c.Query, sorted, want)
		}
	}
}

// ICU's Locale::getLanguage() keeps only the language subtag; libzim derives
// the stemmer from it.
func TestAnalyzerLanguageSubtag(t *testing.T) {
	words := []string{"running", "connections", "hauser", "kjærligheten", "maisons"}
	for raw, want := range map[string]string{
		"en_US": "en", "en-GB": "en", "nb-NO": "nb", "nb_NO": "nb", "zh-Hans": "zh", "zh_Hant_TW": "zh",
		"pt_BR": "pt", "deu-DE": "de", "DE_at": "de", "de.UTF-8": "de", "de@collation=phonebook": "de",
		" fr-CA ,eng": "fr", "cat-ES": "ca", "jpn_JP": "ja",
	} {
		a, ref := NewAnalyzer(raw), NewAnalyzer(want)
		if a.Language() != want {
			t.Errorf("NewAnalyzer(%q).Language() = %q, want %q", raw, a.Language(), want)
			continue
		}
		if a.FulltextSupported() != ref.FulltextSupported() || (a.stem == nil) != (ref.stem == nil) {
			t.Errorf("NewAnalyzer(%q) differs from NewAnalyzer(%q)", raw, want)
		}
		for _, w := range words {
			if a.Stem(w) != ref.Stem(w) {
				t.Errorf("NewAnalyzer(%q).Stem(%q) = %q, want %q", raw, w, a.Stem(w), ref.Stem(w))
			}
		}
	}
}

func TestFulltextSupportedTable(t *testing.T) {
	cases := map[string]bool{
		"deu": true, "eng": true, "fra": true, "spa": true, "ita": true, "nld": true, "nor": true,
		"por": true, "swe": true, "dan": true, "ces": true, "ell": true, "hin": true, "pol": true,
		"jpn": true, "zho": true, "deu,eng": true, "": true, "nob": true, "nno": true,
		"cat": false, "eus": false, "hye": false, "ind": false, "lit": false, "nep": false,
		"ara": false, "fin": false, "hun": false, "gle": false, "ron": false, "rus": false, "tam": false, "tur": false,
	}
	for lang, want := range cases {
		if got := NewAnalyzer(lang).FulltextSupported(); got != want {
			t.Errorf("NewAnalyzer(%q).FulltextSupported() = %v, want %v", lang, got, want)
		}
	}
	if got := NewAnalyzer("deu,eng").Language(); got != "de" {
		t.Errorf("first language code must win, got %q", got)
	}
	// Kiwix ships Norwegian Bokmål as "nob": ICU maps it to "nb", which Xapian
	// stems with its Norwegian stemmer (aliases nb, nn, no).
	for _, code := range []string{"nob", "nno", "nor", "nb", "no"} {
		a := NewAnalyzer(code)
		if got := a.Stem("kjærligheten"); got != NewAnalyzer("nor").Stem("kjærligheten") || a.stem == nil {
			t.Errorf("NewAnalyzer(%q) does not use the Norwegian stemmer (got %q)", code, got)
		}
	}
}
