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
	"math": true, "svg": true, "noscript": true, "template": true, "figcaption": true,
}

// voidTags never have an end tag, so they cannot open a skipped subtree.
var voidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "frame": true, "hr": true,
	"img": true, "input": true, "keygen": true, "link": true, "meta": true, "param": true,
	"source": true, "track": true, "wbr": true,
}

// snippetSkipClasses mark navigation, reference and maintenance subtrees.
// They match whole class tokens, so "my-reference-list" is not "reference".
var snippetSkipClasses = map[string]bool{
	"infobox": true, "navbox": true, "vertical-navbox": true, "navigation-not-searchable": true,
	"reference": true, "references": true, "reflist": true, "mw-references-wrap": true,
	"noprint": true, "metadata": true, "hatnote": true, "mw-editsection": true, "ambox": true,
	"mwe-math-element": true, "mwe-math-mathml-inline": true, "mwe-math-mathml-display": true,
}

// closesParagraph lists the start tags that implicitly end an open <p>
// (its end tag is optional in HTML).
var closesParagraph = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "center": true,
	"details": true, "dialog": true, "dir": true, "div": true, "dl": true, "dd": true, "dt": true,
	"fieldset": true, "figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "header": true,
	"hgroup": true, "hr": true, "li": true, "listing": true, "main": true, "menu": true, "nav": true,
	"ol": true, "p": true, "plaintext": true, "pre": true, "search": true, "section": true,
	"summary": true, "table": true, "ul": true, "xmp": true,
}

// endsParagraph reports whether the end tag </tag> closes an open <p>: its
// own, the end of a block it could not be part of, or of its table cell.
func endsParagraph(tag string) bool {
	switch tag {
	case "td", "th", "tr", "tbody", "thead", "tfoot", "caption", "body", "html":
		return true
	}
	return closesParagraph[tag]
}

// scanWindow returns at most snippetScanBytes of raw, cut on a rune boundary.
func scanWindow(raw []byte) []byte {
	if len(raw) <= snippetScanBytes {
		return raw
	}
	cut := snippetScanBytes
	for cut > snippetScanBytes-utf8.UTFMax && !utf8.RuneStart(raw[cut]) {
		cut--
	}
	return raw[:cut]
}

// articleParagraphs returns the visible text of the first max <p> elements of
// an article, scanning at most snippetScanBytes of HTML. A paragraph without
// end tag (closed by the next block element or by the end of the input) still
// counts.
func articleParagraphs(raw []byte, max int) []string {
	z := html.NewTokenizer(bytes.NewReader(scanWindow(raw)))
	var (
		out       []string
		buf       strings.Builder
		inP       bool
		skipTag   string
		skipDepth int
	)
	flush := func() {
		if inP {
			if text := collapseSpace(buf.String()); text != "" {
				out = append(out, text)
			}
		}
		inP = false
		buf.Reset()
	}
	for len(out) < max {
		switch z.Next() {
		case html.ErrorToken:
			flush()
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
				if skipTag != "p" {
					if tag == skipTag {
						skipDepth++
					}
					continue
				}
				// A skipped paragraph may omit its end tag: the next block
				// element ends it, and is handled like any other tag.
				if !closesParagraph[tag] {
					continue
				}
				skipTag, skipDepth = "", 0
			}
			if inP && closesParagraph[tag] {
				flush()
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
				if skipTag == "p" {
					if endsParagraph(tag) {
						skipTag, skipDepth = "", 0
					}
				} else if tag == skipTag {
					skipDepth--
				}
				continue
			}
			if inP && endsParagraph(tag) {
				flush()
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
			for _, c := range bytes.Fields(val) {
				if snippetSkipClasses[string(c)] {
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
// and after the CJK full stops and the Devanagari danda, which end a sentence
// wherever they stand. A period after a single letter, a number or a common
// abbreviation ("z.", "3.", "ca.") does not end a sentence.
func splitSentences(text string) []string {
	var out []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		end := false
		switch r {
		case '。', '！', '？', '।', '॥':
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
	return abbreviations[strings.ToLower(word)]
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
