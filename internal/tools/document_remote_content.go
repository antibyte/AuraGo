package tools

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// remoteContentCSPMeta is the policy that restrictRemoteContent puts first into <head>.
// It blocks scripts, frames, objects, fetches and every remote subresource; inline styles
// and data: images and fonts keep working.
const remoteContentCSPMeta = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">`

const (
	// metaScanWindow bounds the bytes read for one <meta> tag. A longer tag cannot be
	// shown to be harmless and is neutralized.
	metaScanWindow = 4096
	// metaScanLimit bounds the <meta> tags examined one by one. Every later one is
	// neutralized unexamined, so hostile input cannot make the pass quadratic.
	metaScanLimit = 256
	// utf8ByteOrderMark is U+FEFF encoded as UTF-8.
	utf8ByteOrderMark = "\xef\xbb\xbf"
)

// renderTextCleaner drops NUL and ESC before any scan, so that dropping them cannot join a
// tag afterwards. ESC is what switches ISO-2022-JP away from ASCII; without it, every
// encoding Chromium supports reads the ASCII bytes of the inserted policy as ASCII.
var renderTextCleaner = strings.NewReplacer("\x00", "", "\x1b", "")

// restrictRemoteContent prepares caller-supplied HTML for a Chromium render that must not
// reach the network (document_creator's block_remote_content):
//   - the text becomes valid UTF-8 without NUL and ESC, so no byte order mark or
//     ISO-2022-JP escape can make Chromium decode the inserted policy differently;
//   - every <meta> tag that could be a refresh becomes a bogus comment (see
//     neutralizeMetaRefresh), because a CSP does not govern navigation;
//   - remoteContentCSPMeta becomes the first element of <head>. A meta CSP only covers what
//     follows it in the document, so it goes directly after an explicit <head> tag, or else
//     in front of the first token that makes the parser create the head implicitly (see
//     cspInsertionOffset). A doctype in front stays in front.
//
// Known limits: the policy does not cover DNS prefetch or preconnect hints, which resolve a
// name or open a connection but fetch nothing. Markup that only appears after entity
// decoding, such as an <iframe srcdoc> document, inherits the policy, and its navigation is
// blocked by frame-src (default-src 'none').
func restrictRemoteContent(doc string) string {
	doc = sanitizeRenderText(doc)
	bom := ""
	if strings.HasPrefix(doc, utf8ByteOrderMark) {
		// Chromium consumes a leading byte order mark before parsing, so it stays first.
		bom, doc = utf8ByteOrderMark, doc[len(utf8ByteOrderMark):]
	}
	doc = neutralizeMetaRefresh(doc)
	at := cspInsertionOffset(doc)
	return bom + doc[:at] + remoteContentCSPMeta + doc[at:]
}

// restrictRemoteMarkdown prepares Markdown for Gotenberg's markdown route. Raw HTML in the
// Markdown ends up in the rendered body, so refresh tags are neutralized here too; the
// policy itself goes into the head of the wrapper page.
func restrictRemoteMarkdown(markdown string) string {
	return neutralizeMetaRefresh(sanitizeRenderText(markdown))
}

func sanitizeRenderText(s string) string {
	return renderTextCleaner.Replace(strings.ToValidUTF8(s, string(utf8.RuneError)))
}

// neutralizeMetaRefresh turns every <meta> tag that could carry http-equiv="refresh" into a
// bogus comment by inserting "!" after its "<" ("<!meta ...>").
//
// Each "<meta" occurrence is examined on its own, as if the tokenizer were in the data state
// there. A start tag only begins in the data state, and from there its extent and attributes
// do not depend on what came before, so a refresh tag that Chromium sees in any context
// (inside SVG, after a CDATA section, behind a confusing comment) is found. Look-alikes in
// comments, scripts or attribute values are neutralized too, which is harmless. The
// inserted "!" adds no "<" and changes no quote, ">" or whitespace, so it can neither
// create a new tag nor change the attributes of a tag that was kept.
func neutralizeMetaRefresh(doc string) string {
	lower := asciiLower(doc)
	var out strings.Builder
	last, examined := 0, 0
	for from := 0; ; {
		i := strings.Index(lower[from:], "<meta")
		if i < 0 {
			break
		}
		i += from
		from = i + len("<meta")
		if from == len(lower) || !strings.ContainsRune("\t\n\f\r />", rune(lower[from])) {
			continue // a longer tag name such as <metadata>, or "<meta" at the very end
		}
		examined++
		if examined <= metaScanLimit && metaTagIsHarmless(doc[i:min(len(doc), i+metaScanWindow)]) {
			continue
		}
		out.WriteString(doc[last : i+1])
		out.WriteByte('!')
		last = i + 1
	}
	if last == 0 {
		return doc
	}
	out.WriteString(doc[last:])
	return out.String()
}

// metaTagIsHarmless reports whether fragment, which starts with a "<meta" tag, holds that
// whole tag and no http-equiv attribute that mentions refresh. Chromium compares the value
// with "refresh" ASCII case-insensitively; any value that contains it counts here.
func metaTagIsHarmless(fragment string) bool {
	z := html.NewTokenizer(strings.NewReader(fragment))
	if tt := z.Next(); tt != html.StartTagToken && tt != html.SelfClosingTagToken {
		return false // cut off by the scan window or by the end of the document
	}
	name, more := z.TagName()
	if string(name) != "meta" {
		return false
	}
	for more {
		var key, val []byte
		key, val, more = z.TagAttr()
		if string(key) == "http-equiv" && bytes.Contains(bytes.ToLower(val), []byte("refresh")) {
			return false
		}
	}
	return true
}

// cspInsertionOffset returns where the policy goes so that it becomes the first element of
// <head>: directly after an explicit <head> tag, or else in front of the first token that
// would make the parser create the head implicitly. Only a doctype, comments, whitespace and
// the <html> tag are skipped; none of them loads anything or leaves the head. A skipped
// token must end where Chromium ends it too (see endsAtFirstTerminator); otherwise the
// policy goes in front of it.
func cspInsertionOffset(doc string) int {
	z := html.NewTokenizer(strings.NewReader(doc))
	offset := 0
	for {
		tt := z.Next()
		raw := z.Raw()
		size := len(raw)
		switch tt {
		case html.TextToken:
			if !isHTMLWhitespace(raw) {
				return offset
			}
		case html.DoctypeToken, html.CommentToken:
			if !endsAtFirstTerminator(raw) {
				return offset
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			if !endsAtFirstTerminator(raw) {
				return offset
			}
			switch name, _ := z.TagName(); string(name) {
			case "head":
				return offset + size
			case "html":
			default:
				return offset
			}
		default: // an end tag, or the end of the document
			return offset
		}
		offset += size
	}
}

// endsAtFirstTerminator reports whether raw, a doctype, comment or start tag, ends at the
// first place where Chromium may end it: a "<!--" comment at its first "-->" or "--!>" (or
// as "<!-->" / "<!--->"), anything else at its first ">". A tag with ">" inside a quoted
// value fails this check, and the policy then goes in front of it, which is still safe.
func endsAtFirstTerminator(raw []byte) bool {
	if !bytes.HasPrefix(raw, []byte("<!--")) {
		return bytes.IndexByte(raw, '>') == len(raw)-1
	}
	if s := string(raw); s == "<!-->" || s == "<!--->" {
		return true
	}
	body := raw[len("<!--"):]
	end := -1
	for _, terminator := range []string{"-->", "--!>"} {
		if i := bytes.Index(body, []byte(terminator)); i >= 0 && (end < 0 || i+len(terminator) < end) {
			end = i + len(terminator)
		}
	}
	return end == len(body)
}

func isHTMLWhitespace(b []byte) bool {
	for _, c := range b {
		switch c {
		case ' ', '\t', '\n', '\f', '\r':
		default:
			return false
		}
	}
	return true
}

// asciiLower lowercases ASCII letters only, so byte offsets stay valid for the original.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
