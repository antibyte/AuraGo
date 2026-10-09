package xapian

import (
	"strings"

	"github.com/blevesearch/snowballstem"
	"github.com/blevesearch/snowballstem/danish"
	"github.com/blevesearch/snowballstem/dutch"
	"github.com/blevesearch/snowballstem/english"
	"github.com/blevesearch/snowballstem/french"
	"github.com/blevesearch/snowballstem/german"
	"github.com/blevesearch/snowballstem/italian"
	"github.com/blevesearch/snowballstem/norwegian"
	"github.com/blevesearch/snowballstem/portuguese"
	"github.com/blevesearch/snowballstem/spanish"
	"github.com/blevesearch/snowballstem/swedish"
)

// iso639Part2 maps ISO 639-2/T codes to the two-letter codes ICU's
// Locale::getLanguage() returns for them (libzim derives the stemmer
// language that way). Codes not listed pass through unchanged.
var iso639Part2 = map[string]string{
	"ara": "ar", "cat": "ca", "ces": "cs", "dan": "da", "deu": "de", "ell": "el", "eng": "en",
	"eus": "eu", "fin": "fi", "fra": "fr", "gle": "ga", "hin": "hi", "hun": "hu", "hye": "hy",
	"ind": "id", "ita": "it", "jpn": "ja", "kor": "ko", "lit": "lt", "nep": "ne", "nld": "nl", "nno": "nn",
	"nob": "nb", "nor": "no", "pol": "pl", "por": "pt", "ron": "ro", "rus": "ru", "spa": "es",
	"swe": "sv", "tam": "ta", "tur": "tr", "zho": "zh",
}

// xapianStemmed lists the codes for which Xapian 1.4.23 has a stemmer.
var xapianStemmed = map[string]bool{
	"ar": true, "hy": true, "eu": true, "ca": true, "da": true, "nl": true, "en": true, "fi": true,
	"fr": true, "de": true, "hu": true, "id": true, "ga": true, "it": true, "lt": true, "ne": true,
	"nb": true, "nn": true, "no": true, "pt": true, "ro": true, "ru": true, "es": true, "sv": true,
	"ta": true, "tr": true,
}

// goStemmers are the Snowball stemmers verified to match Xapian 1.4 on the
// accent-stripped Snowball vocabularies (see the plan's Format notes).
var goStemmers = map[string]func(*snowballstem.Env) bool{
	"da": danish.Stem, "de": german.Stem, "en": english.Stem, "es": spanish.Stem,
	"fr": french.Stem, "it": italian.Stem, "nl": dutch.Stem, "nb": norwegian.Stem,
	"nn": norwegian.Stem, "no": norwegian.Stem, "pt": portuguese.Stem, "sv": swedish.Stem,
}

// cjkFulltext records the outcome of TestCJKFulltextDecision: libzim's CJK
// n-gram indexing is reproduced, so Japanese, Chinese and Korean keep
// full-text search. Setting it to false degrades them to title search.
var cjkFulltext = true

// Analyzer reproduces libzim's query analysis for one ZIM language.
type Analyzer struct {
	language  string
	stem      func(*snowballstem.Env) bool
	supported bool
}

// NewAnalyzer takes the raw language string (M/Language of the ZIM or the
// "language" metadata of the index, e.g. "deu" or "deu,eng"; the first code
// wins, like libzim's indexer). Like ICU's Locale::getLanguage(), only the
// language subtag counts: region, script, charset and keywords are cut off
// ("en_US", "nb-NO", "zh-Hans", "de.UTF-8", "de@collation=phonebook").
func NewAnalyzer(zimLanguage string) Analyzer {
	code := strings.TrimSpace(strings.SplitN(zimLanguage, ",", 2)[0])
	if i := strings.IndexAny(code, "-_.@"); i >= 0 {
		code = code[:i]
	}
	code = strings.ToLower(code)
	if two, ok := iso639Part2[code]; ok {
		code = two
	}
	a := Analyzer{language: code, supported: true}
	if !cjkFulltext && (code == "ja" || code == "zh" || code == "ko") {
		a.supported = false
		return a
	}
	if xapianStemmed[code] {
		fn, ok := goStemmers[code]
		if !ok {
			a.supported = false // Xapian stems this language; we cannot reproduce it
			return a
		}
		a.stem = fn
	}
	return a
}

// Language is the two-letter (or unmapped) language code in use.
func (a Analyzer) Language() string { return a.language }

// FulltextSupported reports whether QueryTerms reproduces libzim's index
// terms for this language. False means: use title search only.
func (a Analyzer) FulltextSupported() bool { return a.supported }

// Stem applies the language's Snowball stemmer (identity without one).
func (a Analyzer) Stem(word string) string {
	if a.stem == nil || word == "" {
		return word
	}
	env := snowballstem.NewEnv(word)
	a.stem(env)
	return env.Current()
}

// QueryTerms turns a user query into the full-text index terms libzim's
// Searcher would look up: Normalize, QueryParser lexing with CJK n-grams,
// every non-CJK word stemmed (STEM_ALL, no "Z" prefix). Duplicates are kept
// because Xapian weights repeated query terms once per occurrence.
func (a Analyzer) QueryTerms(query string) []string {
	var out []string
	for _, w := range queryWords(Normalize(query)) {
		if w.cjk {
			for _, g := range ngrams(w.text) {
				out = append(out, g.text)
			}
			continue
		}
		out = append(out, a.Stem(w.text))
	}
	return out
}
