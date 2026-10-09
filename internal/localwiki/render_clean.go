package localwiki

import (
	"fmt"
	"html"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	xhtml "golang.org/x/net/html"
)

const (
	maxInfoboxRows  = 80
	maxInfoboxDepth = 4 // infobox plus nested subboxes
	maxTableRows    = 50
)

// The cleanup writes its own bracketed labels ("[Image: …]", "[Table
// truncated: …]") with private-use sentinels instead of brackets: the
// converter would escape a "[" as "\[", and tidyMarkdown must never
// un-escape brackets that come from the article (that could turn article
// text into a live link). cleanArticle first strips these runes from the
// article, so after conversion only the cleanup's labels carry them.
const (
	markerOpen         = '' // "[" opening a label
	markerClose        = '' // "]" closing it
	markerLeftBracket  = '' // a "[" from the article inside a label, written as `\[`
	markerRightBracket = '' // a "]" from the article inside a label, written as `\]`
)

// markerLabel returns text as a bracketed label; brackets inside text stay
// escaped so article text can neither close the label nor start a link.
func markerLabel(text string) string {
	text = strings.NewReplacer("[", string(markerLeftBracket), "]", string(markerRightBracket)).Replace(text)
	return string(markerOpen) + text + string(markerClose)
}

func isMarkerRune(r rune) bool { return r >= markerOpen && r <= markerRightBracket }

// stripMarkerRunes removes the label sentinels from the text, comments and
// attribute values of the article.
func stripMarkerRunes(n *xhtml.Node) {
	strip := func(s string) string {
		if !strings.ContainsFunc(s, isMarkerRune) {
			return s
		}
		return strings.Map(func(r rune) rune {
			if isMarkerRune(r) {
				return -1
			}
			return r
		}, s)
	}
	if n.Type == xhtml.TextNode || n.Type == xhtml.CommentNode {
		n.Data = strip(n.Data)
	}
	for i := range n.Attr {
		n.Attr[i].Val = strip(n.Attr[i].Val)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		stripMarkerRunes(c)
	}
}

// removeSelectors match subtrees that never reach the model: page chrome
// and the article title, edit links and navbars, reference markers and
// reference lists, navigation boxes and sidebars, maintenance templates,
// hatnotes, tables of contents, coordinates, categories, the licence footer,
// raw-text and fallback elements and hidden elements (inline display:none is
// removed by removeHiddenStyles). The class names cover mwoffliner 1.13
// (mobile sections) and 2.x (Parsoid read views) output and the English and
// German template families; mwoffliner itself already drops noprint,
// metadata, ambox and navbar.
var removeSelectors = strings.Join([]string{
	"script", "style", "link", "meta", "noscript", "template", "h1", "input", "map",
	"xmp", "noembed", "noframes", "plaintext", "[hidden]",
	".mw-editsection", ".navbox-navbar", ".mw-cite-backlink",
	"sup.reference", "sup.mw-ref", ".mw-ref", ".mw-reflink-text",
	"ol.references", ".mw-references-wrap", ".reflist", ".refbegin", ".references",
	".navbox", ".navbox-styles", ".vertical-navbox", ".navbox-container", ".navigation-not-searchable",
	`[role="navigation"]`, ".NavFrame", ".klappleiste", ".navileiste", ".BoxenVerschmelzen",
	".sidebar", ".sistersitebox", ".schwesterbox", ".portal-bar",
	".ambox", ".mbox-small", ".ombox", ".tmbox", ".cmbox", ".imbox", ".metadata", ".noprint",
	".hatnote", ".dablink", ".rellink", ".hauptartikel", ".sieheauch", ".shortdescription",
	"#toc", ".toc", ".mw-empty-elt", "#coordinates", ".geo-nondefault", "#normdaten",
	".Z3988", ".ext-phonos", ".ext-phonos-attribution", ".mw-kartographer-container",
	"#catlinks", ".catlinks", ".zim-footer",
}, ", ")

// cleanArticle prepares the content root of a parsed article for conversion.
// The order matters: math and figures read nodes that the removal step drops,
// and the label sentinels are stripped before any label is written.
func cleanArticle(root *goquery.Selection) {
	for _, n := range root.Nodes {
		stripMarkerRunes(n)
	}
	removeFooter(root)
	convertMath(root)
	root.Find(removeSelectors).Remove()
	removeHiddenStyles(root)
	convertFigures(root)
	convertInfoboxes(root)
	root.Find("img, picture, video, audio, svg").Remove()
	flattenNestedTables(root)
	truncateTables(root)
	unwrapLinks(root)
}

// removeFooter drops mwoffliner's licence footer, which sits between
// <!--htdig_noindex--> and <!--/htdig_noindex--> comments.
func removeFooter(root *goquery.Selection) {
	for _, n := range root.Nodes {
		removeBetweenNoIndex(n)
	}
}

func removeBetweenNoIndex(n *xhtml.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == xhtml.CommentNode && strings.TrimSpace(c.Data) == "htdig_noindex" {
			for s := c.NextSibling; s != nil; {
				after := s.NextSibling
				done := s.Type == xhtml.CommentNode && strings.TrimSpace(s.Data) == "/htdig_noindex"
				n.RemoveChild(s)
				s = after
				if done {
					break
				}
			}
			next = c.NextSibling
			n.RemoveChild(c)
		} else if c.Type == xhtml.ElementNode {
			removeBetweenNoIndex(c)
		}
		c = next
	}
}

// removeHiddenStyles drops elements hidden by an inline style, whatever the
// case and spacing ("display:none", "DISPLAY : None !important").
func removeHiddenStyles(root *goquery.Selection) {
	root.Find("[style]").Each(func(_ int, el *goquery.Selection) {
		style, _ := el.Attr("style")
		compact := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return unicode.ToLower(r)
		}, style)
		if strings.Contains(compact, "display:none") {
			el.Remove()
		}
	})
}

// convertMath replaces MediaWiki math with its TeX source as inline code.
func convertMath(root *goquery.Selection) {
	root.Find(".mwe-math-element").Each(func(_ int, m *goquery.Selection) {
		tex := strings.TrimSpace(m.Find(`annotation[encoding="application/x-tex"]`).First().Text())
		if tex == "" {
			tex, _ = m.Find("img").First().Attr("alt")
			tex = strings.TrimSpace(tex)
		}
		if strings.HasPrefix(tex, `{\displaystyle `) && strings.HasSuffix(tex, "}") {
			tex = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tex, `{\displaystyle `), "}"))
		}
		if tex == "" {
			m.Remove()
			return
		}
		m.ReplaceWithHtml("<code>" + html.EscapeString(tex) + "</code>")
	})
}

// convertFigures replaces images that carry a caption with "[Image: caption]".
// Figures without an image (nopic editions) or without any caption are dropped.
func convertFigures(root *goquery.Selection) {
	root.Find("figure, div.thumb, .gallerybox").Each(func(_ int, fig *goquery.Selection) {
		if len(fig.Nodes) == 0 || fig.Nodes[0].Parent == nil {
			return
		}
		media := fig.Find("img, video, audio, picture")
		caption := collapseSpace(fig.Find("figcaption, .thumbcaption, .gallerytext").First().Text())
		if caption == "" {
			caption, _ = media.First().Attr("alt")
			caption = collapseSpace(caption)
		}
		if media.Length() == 0 || caption == "" {
			fig.Remove()
			return
		}
		tag := "p"
		if goquery.NodeName(fig) == "li" {
			tag = "li"
		}
		fig.ReplaceWithHtml("<" + tag + ">" + html.EscapeString(markerLabel("Image: "+caption)) + "</" + tag + ">")
	})
}

// convertInfoboxes turns infobox tables into a key/value list
// ("- **Einwohner:** 3.878.100"); header rows become bold paragraphs.
// Embedded subboxes (a nested table.infobox-subbox or table.infobox in a
// full-width row) continue the same list: their header rows become bold
// paragraphs and their rows key/value lines, as if they were rows of the
// outer infobox. Other nested tables (charts, layout) are dropped.
func convertInfoboxes(root *goquery.Selection) {
	root.Find("table.infobox").Each(func(_ int, table *goquery.Selection) {
		if len(table.Nodes) == 0 || table.Nodes[0].Parent == nil {
			return
		}
		w := &infoboxWriter{}
		w.table(table, 1)
		w.closeList()
		if w.b.Len() == 0 {
			table.Remove()
			return
		}
		table.ReplaceWithHtml(w.b.String())
	})
}

// infoboxWriter collects the HTML that replaces one infobox.
type infoboxWriter struct {
	b    strings.Builder
	open bool
	rows int
}

func (w *infoboxWriter) closeList() {
	if w.open {
		w.b.WriteString("</ul>")
		w.open = false
	}
}

// item adds a list item; itemHTML is already escaped.
func (w *infoboxWriter) item(itemHTML string) {
	if !w.open {
		w.b.WriteString("<ul>")
		w.open = true
	}
	w.b.WriteString("<li>" + itemHTML + "</li>")
}

func (w *infoboxWriter) heading(text string) {
	w.closeList()
	w.b.WriteString("<p><strong>" + html.EscapeString(text) + "</strong></p>")
}

// table writes the caption and the own rows of an infobox or subbox.
func (w *infoboxWriter) table(table *goquery.Selection, depth int) {
	if caption := collapseSpace(table.ChildrenFiltered("caption").Text()); caption != "" {
		w.heading(caption)
	}
	table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		if w.rows >= maxInfoboxRows || !sameNode(tr.Closest("table"), table) {
			return
		}
		w.row(table, tr, depth)
	})
}

func (w *infoboxWriter) row(table, tr *goquery.Selection, depth int) {
	cells := tr.ChildrenFiltered("th, td")
	if cells.Length() > 1 && cells.Length() == cells.Filter("th").Length() {
		return // a row of column labels, e.g. above coat of arms and map
	}
	switch cells.Length() {
	case 0:
		return
	case 1:
		cell := cells.First()
		if cell.Find("table").Length() > 0 {
			if depth < maxInfoboxDepth {
				cell.Find("table.infobox-subbox, table.infobox").Each(func(_ int, sub *goquery.Selection) {
					if sameNode(sub.Parent().Closest("table"), table) {
						w.table(sub, depth+1)
					}
				})
			}
			return // charts and nested layout tables carry no key/value data
		}
		if cell.HasClass("infobox-image") || cell.Find("img").Length() > 0 {
			if cell.Find("img").Length() == 0 {
				return
			}
			caption := cellText(cell)
			if caption == "" {
				caption, _ = cell.Find("img").First().Attr("alt")
				caption = collapseSpace(caption)
			}
			if caption != "" {
				w.rows++
				w.item(html.EscapeString(markerLabel("Image: " + caption)))
			}
			return
		}
		text := cellText(cell)
		if text == "" {
			return
		}
		w.rows++
		if goquery.NodeName(cell) == "th" || cell.HasClass("infobox-header") || cell.HasClass("infobox-above") || cell.HasClass("infobox-title") {
			w.heading(text)
			return
		}
		w.item(html.EscapeString(text))
	default:
		key := strings.TrimSpace(strings.TrimSuffix(cellText(cells.First()), ":"))
		var values []string
		cells.Slice(1, cells.Length()).Each(func(_ int, c *goquery.Selection) {
			if v := cellText(c); v != "" {
				values = append(values, v)
			}
		})
		value := strings.Join(values, " ")
		switch {
		case key != "" && value != "":
			w.rows++
			w.item("<strong>" + html.EscapeString(key) + ":</strong> " + html.EscapeString(value))
		case value != "":
			w.rows++
			w.item(html.EscapeString(value))
		case key != "":
			w.rows++
			w.heading(key)
		}
	}
}

// cellText returns the visible text of a table cell, with line breaks and
// list items separated by "; ".
func cellText(cell *goquery.Selection) string {
	cell.Find("br").ReplaceWithHtml(" ; ")
	cell.Find("li").Each(func(_ int, li *goquery.Selection) { li.AppendHtml(" ; ") })
	parts := strings.Split(collapseSpace(cell.Text()), " ; ")
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(strings.Trim(p, ";")); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}

func sameNode(a, b *goquery.Selection) bool {
	return len(a.Nodes) > 0 && len(b.Nodes) > 0 && a.Nodes[0] == b.Nodes[0]
}

// flattenNestedTables replaces tables inside table cells with their text,
// because Markdown tables cannot nest.
func flattenNestedTables(root *goquery.Selection) {
	root.Find("td table, th table").Each(func(_ int, t *goquery.Selection) {
		if len(t.Nodes) == 0 || t.Nodes[0].Parent == nil {
			return
		}
		t.ReplaceWithHtml("<span>" + html.EscapeString(collapseSpace(t.Text())) + "</span>")
	})
}

// truncateTables keeps the first 50 rows of every table and notes the cut.
func truncateTables(root *goquery.Selection) {
	root.Find("table").Each(func(_ int, table *goquery.Selection) {
		var rows []*goquery.Selection
		table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			if sameNode(tr.Closest("table"), table) {
				rows = append(rows, tr)
			}
		})
		if len(rows) <= maxTableRows {
			return
		}
		for _, tr := range rows[maxTableRows:] {
			tr.Remove()
		}
		table.AfterHtml("<p>" + markerLabel(fmt.Sprintf("Table truncated: showing %d of %d rows", maxTableRows, len(rows))) + "</p>")
	})
}

// unwrapLinks replaces every link with its content, so the Markdown carries
// plain text instead of ZIM-internal or external URLs.
func unwrapLinks(root *goquery.Selection) {
	root.Find("a").Each(func(_ int, a *goquery.Selection) {
		for _, n := range a.Nodes {
			if n.Parent == nil {
				continue
			}
			for c := n.FirstChild; c != nil; c = n.FirstChild {
				n.RemoveChild(c)
				n.Parent.InsertBefore(c, n)
			}
			n.Parent.RemoveChild(n)
		}
	})
}
