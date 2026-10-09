package localwiki

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	snippetMaxRunes      = 200
	snippetScanBytes     = 512 << 10
	snippetMaxParagraphs = 40
)

// snippetSkipTags are subtrees whose text never belongs to a snippet.
var snippetSkipTags = map[string]bool{
	"table": true, "figure": true, "style": true, "script": true, "sup": true,
	"math": true, "noscript": true, "template": true, "figcaption": true,
}

// voidTags never have an end tag, so they cannot open a skipped subtree.
var voidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

// snippetSkipClasses mark navigation, reference and maintenance subtrees.
var snippetSkipClasses = []string{
	"infobox", "navbox", "navigation-not-searchable", "reference", "noprint",
	"metadata", "hatnote", "mw-editsection", "ambox", "mwe-math",
}

// articleParagraphs returns the visible text of the first max <p> elements of
// an article, scanning at most snippetScanBytes of HTML.
func articleParagraphs(raw []byte, max int) []string {
	if len(raw) > snippetScanBytes {
		raw = raw[:snippetScanBytes]
	}
	z := html.NewTokenizer(bytes.NewReader(raw))
	var (
		out       []string
		buf       strings.Builder
		inP       bool
		skipTag   string
		skipDepth int
	)
	for len(out) < max {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return out
		case html.TextToken:
			if inP && skipDepth == 0 {
				buf.Write(z.Text())
			}
		case html.SelfClosingTagToken:
			if name, _ := z.TagName(); inP && skipDepth == 0 && string(name) == "br" {
				buf.WriteByte(' ')
			}
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)
			if skipDepth > 0 {
				if tag == skipTag {
					skipDepth++
				}
				continue
			}
			if !voidTags[tag] && (snippetSkipTags[tag] || (hasAttr && hasSkipClass(z))) {
				skipTag, skipDepth = tag, 1
				continue
			}
			switch tag {
			case "p":
				inP = true
				buf.Reset()
			case "br":
				buf.WriteByte(' ')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if skipDepth > 0 {
				if tag == skipTag {
					skipDepth--
				}
				continue
			}
			if tag == "p" && inP {
				inP = false
				if text := collapseSpace(buf.String()); text != "" {
					out = append(out, text)
				}
			}
		}
	}
	return out
}

// hasSkipClass reads the remaining attributes of the current tag and reports
// whether its class names a skipped subtree.
func hasSkipClass(z *html.Tokenizer) bool {
	for {
		key, val, more := z.TagAttr()
		if string(key) == "class" {
			class := string(val)
			for _, c := range snippetSkipClasses {
				if strings.Contains(class, c) {
					return true
				}
			}
		}
		if !more {
			return false
		}
	}
}

// abbreviations end with a period that does not end a sentence.
var abbreviations = map[string]bool{
	"ca": true, "bzw": true, "usw": true, "etc": true, "vgl": true, "evtl": true, "dr": true, "nr": true,
	"st": true, "mr": true, "mrs": true, "ms": true, "vs": true, "jh": true, "chr": true, "geb": true,
	"gest": true, "inkl": true, "sog": true, "bspw": true, "mio": true, "mrd": true, "no": true, "approx": true,
}

// splitSentences splits text after ".", "!" or "?" followed by white space
// and after the CJK full stops. A period after a single letter, a number or
// a common abbreviation ("z.", "3.", "ca.") does not end a sentence.
func splitSentences(text string) []string {
	var out []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		end := false
		switch r {
		case '。', '！', '？':
			end = true
		case '.', '!', '?':
			if i+1 < len(runes) && unicode.IsSpace(runes[i+1]) {
				end = r != '.' || !abbreviationBefore(runes, i)
			}
		}
		if end {
			if s := strings.TrimSpace(string(runes[start : i+1])); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	if s := strings.TrimSpace(string(runes[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

// abbreviationBefore reports whether the word ending at runes[i-1] is a
// single letter, a number or a known abbreviation.
func abbreviationBefore(runes []rune, i int) bool {
	j := i
	for j > 0 && isWordRune(runes[j-1]) {
		j--
	}
	word := string(runes[j:i])
	if utf8.RuneCountInString(word) <= 1 {
		return true
	}
	if strings.IndexFunc(word, func(r rune) bool { return !unicode.IsDigit(r) }) < 0 {
		return true
	}
	return abbreviations[foldText(word)]
}

// snippetFromHTML returns the first sentence of the article that contains a
// query word, at most 200 characters; without such a sentence it returns the
// start of the lead.
func snippetFromHTML(raw []byte, words []string) string {
	paragraphs := articleParagraphs(raw, snippetMaxParagraphs)
	for _, p := range paragraphs {
		for _, sentence := range splitSentences(p) {
			if containsQueryWord(sentence, words) {
				return truncateRunes(sentence, snippetMaxRunes)
			}
		}
	}
	if len(paragraphs) == 0 {
		return ""
	}
	return truncateRunes(paragraphs[0], snippetMaxRunes)
}
