package localwiki

import (
	"bytes"
	"fmt"
	"regexp"
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

const leadMaxRunes = 2000

// renderedArticle is an article as model-facing Markdown, split at its
// headings. sections[0] is the lead (level 0, no heading).
type renderedArticle struct {
	title    string
	sections []renderedSection
}

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

// articleRoot returns the element that holds the article body.
func articleRoot(doc *goquery.Document) *goquery.Selection {
	for _, sel := range []string{"#mw-content-text .mw-parser-output", "#mw-content-text", "main", "body"} {
		if root := doc.Find(sel).First(); root.Length() > 0 {
			return root
		}
	}
	return doc.Selection
}

// renderArticle converts article HTML to sections of Markdown.
func renderArticle(raw []byte, title string) (*renderedArticle, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse article html: %w", err)
	}
	root := articleRoot(doc)
	cleanArticle(root)
	art := &renderedArticle{title: title}
	for _, rs := range splitSections(root.Nodes[0]) {
		body, err := nodesToMarkdown(rs.nodes)
		if err != nil {
			return nil, err
		}
		art.sections = append(art.sections, renderedSection{heading: rs.heading, level: rs.level, body: body})
	}
	art.dropUnwantedSections()
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
				sections = append(sections, rawSection{heading: collapseSpace(nodeText(c)), level: level})
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
)

func tidyMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, " ", " ")
	s = trailingSpace.ReplaceAllString(s, "\n")
	s = extraNewlines.ReplaceAllString(s, "\n\n")
	// Commonmark escapes the bracket of the fixed image label; for the model
	// the label is plain text.
	s = strings.ReplaceAll(s, `\[Image: `, "[Image: ")
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

// sectionMarkdown renders section i with its heading and all subsections.
func (a *renderedArticle) sectionMarkdown(i int) string {
	s := a.sections[i]
	var parts []string
	if i > 0 {
		parts = append(parts, strings.Repeat("#", s.level)+" "+s.heading)
	}
	if s.body != "" {
		parts = append(parts, s.body)
	}
	if i == 0 {
		return strings.Join(parts, "\n\n")
	}
	for j := i + 1; j < len(a.sections) && a.sections[j].level > s.level; j++ {
		sub := a.sections[j]
		parts = append(parts, strings.Repeat("#", sub.level)+" "+sub.heading)
		if sub.body != "" {
			parts = append(parts, sub.body)
		}
	}
	return strings.Join(parts, "\n\n")
}

// fullMarkdown renders the whole article under its title.
func (a *renderedArticle) fullMarkdown() string {
	parts := []string{"# " + a.title}
	if lead := a.sections[0].body; lead != "" {
		parts = append(parts, lead)
	}
	for _, s := range a.sections[1:] {
		parts = append(parts, strings.Repeat("#", s.level)+" "+s.heading)
		if s.body != "" {
			parts = append(parts, s.body)
		}
	}
	return strings.Join(parts, "\n\n")
}

// sectionList describes the rendered sections for Article.Sections.
func (a *renderedArticle) sectionList() []Section {
	out := make([]Section, len(a.sections))
	for i, s := range a.sections {
		out[i] = Section{Index: i, Heading: s.heading, Level: s.level, Chars: utf8.RuneCountInString(a.sectionMarkdown(i))}
	}
	return out
}

// leadFromHTML renders the prose before the first heading. Only the HTML up
// to the first <h2 is parsed, so long articles stay cheap.
func leadFromHTML(raw []byte) (string, error) {
	if i := bytes.Index(raw, []byte("<h2")); i > 0 {
		raw = raw[:i]
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("parse article html: %w", err)
	}
	root := articleRoot(doc)
	root.Find("table, figure, .thumb, .gallery").Remove()
	cleanArticle(root)
	if len(root.Nodes) == 0 {
		return "", nil
	}
	sections := splitSections(root.Nodes[0])
	md, err := nodesToMarkdown(sections[0].nodes)
	if err != nil {
		return "", err
	}
	return truncateRunes(md, leadMaxRunes), nil
}
