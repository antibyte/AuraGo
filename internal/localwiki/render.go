package localwiki

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	leadMaxRunes = 2000
	// leadScanBytes bounds the HTML that leadFromHTML parses.
	leadScanBytes = 1 << 20
)

// renderedArticle is an article as model-facing Markdown, split at its
// headings. sections[0] is the lead (level 0, no heading). A rendered
// article is shared through the render cache and does not change after
// renderArticle returns: the full Markdown, the place of every section in
// it and the section list are built once (layout).
type renderedArticle struct {
	title    string
	sections []renderedSection

	layoutOnce sync.Once
	full       string     // fullMarkdown()
	spans      []textSpan // sectionMarkdown(i) is full[spans[i].start:spans[i].end]
	list       []Section
	// headingFolded and headingKeys are the folded heading (punctuation and
	// qualifier kept) and its titleKey, per section, for findSection.
	headingFolded []string
	headingKeys   []string
	size          int // approximate memory use in bytes, for the render cache
}

// textSpan is a byte range of renderedArticle.full.
type textSpan struct{ start, end int }

// sectionOverhead approximates the memory of one section besides its text
// (renderedSection, Section, textSpan and slice headers).
const sectionOverhead = 160

type renderedSection struct {
	heading string
	level   int
	body    string
}

// rawSection collects the nodes between two headings before conversion.
type rawSection struct {
	heading string
	level   int
	nodes   []*html.Node
}

var markdownConverter = sync.OnceValue(func() *converter.Converter {
	return converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(
			table.WithNewlineBehavior(table.NewlineBehaviorPreserve),
			table.WithCellPaddingBehavior(table.CellPaddingBehaviorMinimal),
			table.WithSkipEmptyRows(true),
			table.WithHeaderPromotion(true),
		),
	))
})

// externalLinkHeadings are the folded headings of external-link sections in
// the 16 supported languages.
var externalLinkHeadings = func() map[string]bool {
	m := map[string]bool{}
	for _, h := range []string{
		"External links", "Weblinks", "Liens externes", "Enlaces externos", "Collegamenti esterni",
		"Externe links", "Linki zewnętrzne", "Ligações externas", "Ligações exteriores", "Externa länkar",
		"Eksterne lenker", "Eksterne henvisninger", "Eksterne links", "Externí odkazy",
		"Εξωτερικοί σύνδεσμοι", "外部リンク", "外部链接", "外部連結", "बाहरी कड़ियाँ",
	} {
		m[titleKey(h)] = true
	}
	return m
}()

// articleRoot returns the element that holds the article body. Only a
// .mw-parser-output directly inside #mw-content-text (mwoffliner 2.x) is the
// body; a nested one in a 1.13 page is merely a template wrapper.
func articleRoot(doc *goquery.Document) *goquery.Selection {
	for _, sel := range []string{"#mw-content-text > .mw-parser-output", "#mw-content-text", "main", "body"} {
		if root := doc.Find(sel).First(); root.Length() > 0 {
			return root
		}
	}
	return doc.Selection
}

// renderArticle converts article HTML to sections of Markdown.
func renderArticle(raw []byte, title string) (*renderedArticle, error) {
	return renderArticleContext(context.Background(), raw, title)
}

// renderArticleContext is renderArticle that stops with ctx.Err() between
// the conversion of two sections.
func renderArticleContext(ctx context.Context, raw []byte, title string) (*renderedArticle, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse article html: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := articleRoot(doc)
	cleanArticle(root)
	art := &renderedArticle{title: title}
	for _, rs := range splitSections(root.Nodes[0]) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, err := nodesToMarkdown(rs.nodes)
		if err != nil {
			return nil, err
		}
		art.sections = append(art.sections, renderedSection{heading: rs.heading, level: rs.level, body: body})
	}
	art.dropUnwantedSections()
	art.layout()
	return art, nil
}

// splitSections walks the article in document order and starts a new
// section at every h2–h6 heading, descending into wrappers (section, details,
// summary, div.mw-heading) that contain headings.
func splitSections(root *html.Node) []rawSection {
	sections := []rawSection{{}}
	var walk func(parent *html.Node)
	walk = func(parent *html.Node) {
		for c := parent.FirstChild; c != nil; c = c.NextSibling {
			if level := headingLevel(c); level > 0 {
				sections = append(sections, rawSection{heading: collapseSpace(restoreAmp(nodeText(c))), level: level})
				continue
			}
			if c.Type == html.ElementNode && containsHeading(c) {
				walk(c)
				continue
			}
			cur := &sections[len(sections)-1]
			cur.nodes = append(cur.nodes, c)
		}
	}
	walk(root)
	return sections
}

func headingLevel(n *html.Node) int {
	if n.Type != html.ElementNode {
		return 0
	}
	switch n.DataAtom {
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 0
}

func containsHeading(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if headingLevel(c) > 0 || (c.Type == html.ElementNode && containsHeading(c)) {
			return true
		}
	}
	return false
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// nodesToMarkdown moves nodes into a fresh container and converts it.
func nodesToMarkdown(nodes []*html.Node) (string, error) {
	container := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
		container.AppendChild(n)
	}
	out, err := markdownConverter().ConvertNode(container)
	if err != nil {
		return "", fmt.Errorf("convert article to markdown: %w", err)
	}
	return tidyMarkdown(string(out)), nil
}

var (
	trailingSpace = regexp.MustCompile(`[ \t]+\n`)
	extraNewlines = regexp.MustCompile(`\n{3,}`)
	// markdownFinish turns the cleanup's sentinels into brackets and
	// decodes the entities the converter writes for a "<" or ">" of the text
	// (and "&amp;", should it ever write one). The article's own ampersands
	// were set aside as markerAmp before conversion, so every entity left in
	// the converter's output is the converter's: a literal "&lt;" of the
	// article (in prose, code or pre) arrives here as markerAmp + "lt;" and
	// comes out as "&lt;". One pass: a restored "&" is never decoded again.
	// Nothing else is decoded: the tool isolates the Markdown afterwards.
	markdownFinish = strings.NewReplacer(
		string(markerOpen), "[", string(markerClose), "]",
		string(markerLeftBracket), `\[`, string(markerRightBracket), `\]`,
		"&lt;", "<", "&gt;", ">", "&amp;", "&",
		string(markerAmp), "&",
	)
)

func tidyMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, " ", " ")
	s = trailingSpace.ReplaceAllString(s, "\n")
	s = extraNewlines.ReplaceAllString(s, "\n\n")
	s = markdownFinish.Replace(s)
	return strings.TrimSpace(s)
}

// dropUnwantedSections removes external-link sections with their
// subsections, and sections left empty by the cleanup (reference lists)
// unless one of their subsections has content. The lead always stays.
func (a *renderedArticle) dropUnwantedSections() {
	n := len(a.sections)
	drop := make([]bool, n)
	for i := 1; i < n; i++ {
		if !externalLinkHeadings[titleKey(a.sections[i].heading)] {
			continue
		}
		drop[i] = true
		for j := i + 1; j < n && a.sections[j].level > a.sections[i].level; j++ {
			drop[j] = true
		}
	}
	for i := n - 1; i >= 1; i-- {
		if drop[i] {
			continue
		}
		empty := a.sections[i].body == ""
		for j := i + 1; empty && j < n && a.sections[j].level > a.sections[i].level; j++ {
			empty = drop[j]
		}
		drop[i] = empty
	}
	kept := a.sections[:0]
	for i, s := range a.sections {
		if !drop[i] {
			kept = append(kept, s)
		}
	}
	a.sections = kept
}

// layout builds, once, the full Markdown (the title, then every section's
// heading and body, joined by blank lines), the byte span of every section's
// Markdown in it (heading, body and all subsections; the lead is its body
// alone) and the section list. Section bodies become substrings of the full
// Markdown, so the article holds its text only once.
func (a *renderedArticle) layout() {
	a.layoutOnce.Do(func() {
		n := len(a.sections)
		var b strings.Builder
		b.WriteString("# " + a.title)
		a.spans = make([]textSpan, n)
		bodyAt := make([]int, n)
		contentEnd := make([]int, n)
		for i, s := range a.sections {
			if i > 0 {
				b.WriteString("\n\n")
				a.spans[i].start = b.Len()
				b.WriteString(strings.Repeat("#", s.level) + " " + s.heading)
			}
			bodyAt[i] = -1
			if s.body != "" {
				b.WriteString("\n\n")
				bodyAt[i] = b.Len()
				b.WriteString(s.body)
			}
			contentEnd[i] = b.Len()
		}
		a.full = b.String()
		a.size = len(a.full)
		if n > 0 {
			if bodyAt[0] >= 0 {
				a.spans[0] = textSpan{bodyAt[0], contentEnd[0]}
			} else {
				a.spans[0] = textSpan{contentEnd[0], contentEnd[0]}
			}
		}
		for i := 1; i < n; i++ {
			j := i
			for j+1 < n && a.sections[j+1].level > a.sections[i].level {
				j++
			}
			a.spans[i].end = contentEnd[j]
		}
		a.list = make([]Section, n)
		a.headingFolded = make([]string, n)
		a.headingKeys = make([]string, n)
		for i, s := range a.sections {
			if bodyAt[i] >= 0 {
				a.sections[i].body = a.full[bodyAt[i] : bodyAt[i]+len(s.body)]
			}
			span := a.spans[i]
			a.list[i] = Section{Index: i, Heading: s.heading, Level: s.level, Chars: utf8.RuneCountInString(a.full[span.start:span.end])}
			a.headingFolded[i] = foldText(collapseSpace(s.heading))
			a.headingKeys[i] = titleKey(s.heading)
			a.size += len(s.heading) + len(a.headingFolded[i]) + len(a.headingKeys[i]) + sectionOverhead
		}
	})
}

// sectionMarkdown returns section i with its heading and all subsections.
func (a *renderedArticle) sectionMarkdown(i int) string {
	a.layout()
	span := a.spans[i]
	return a.full[span.start:span.end]
}

// fullMarkdown returns the whole article under its title.
func (a *renderedArticle) fullMarkdown() string {
	a.layout()
	return a.full
}

// sectionList describes the rendered sections for Article.Sections, as a
// copy because cached articles are shared between readers.
func (a *renderedArticle) sectionList() []Section {
	a.layout()
	return slices.Clone(a.list)
}

// memSize approximates the memory the rendered article holds.
func (a *renderedArticle) memSize() int {
	a.layout()
	return a.size
}

// leadFromHTML renders the prose before the first heading. It parses at most
// leadScanBytes of HTML and drops the body from its first h2-h6 on before
// the cleanup, so long articles stay cheap.
func leadFromHTML(raw []byte) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(cutAtRune(raw, leadScanBytes)))
	if err != nil {
		return "", fmt.Errorf("parse article html: %w", err)
	}
	root := articleRoot(doc)
	if len(root.Nodes) == 0 {
		return "", nil
	}
	cutAtFirstHeading(root.Nodes[0])
	root.Find("table, figure, .thumb, .gallery").Remove()
	cleanArticle(root)
	sections := splitSections(root.Nodes[0])
	md, err := nodesToMarkdown(sections[0].nodes)
	if err != nil {
		return "", err
	}
	return truncateRunes(md, leadMaxRunes), nil
}

// cutAtRune returns at most max bytes of raw, cut on a rune boundary.
func cutAtRune(raw []byte, max int) []byte {
	if len(raw) <= max {
		return raw
	}
	cut := max
	for cut > 0 && cut > max-utf8.UTFMax && !utf8.RuneStart(raw[cut]) {
		cut--
	}
	return raw[:cut]
}

// cutAtFirstHeading removes the first h2-h6 below root and everything that
// follows it in document order.
func cutAtFirstHeading(root *html.Node) {
	first := firstHeading(root)
	if first == nil {
		return
	}
	for n := first; n != root && n.Parent != nil; n = n.Parent {
		for s := n.NextSibling; s != nil; {
			next := s.NextSibling
			n.Parent.RemoveChild(s)
			s = next
		}
	}
	first.Parent.RemoveChild(first)
}

// firstHeading returns the first h2-h6 below n in document order.
func firstHeading(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if headingLevel(c) > 0 {
			return c
		}
		if h := firstHeading(c); h != nil {
			return h
		}
	}
	return nil
}
