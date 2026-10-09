package localwiki

import (
	"fmt"
	"html"
	"strings"

	"github.com/PuerkitoBio/goquery"
	xhtml "golang.org/x/net/html"
)

const (
	maxInfoboxRows = 80
	maxTableRows   = 50
)

// removeSelectors match subtrees that never reach the model: page chrome
// and the article title, edit links and navbars, reference markers and
// reference lists, navigation boxes and sidebars, maintenance templates,
// hatnotes, tables of contents, coordinates, categories, the licence footer
// and hidden elements. The class names cover mwoffliner 1.13 (mobile
// sections) and 2.x (Parsoid read views) output and the English and German
// template families; mwoffliner itself already drops noprint, metadata,
// ambox and navbar.
var removeSelectors = strings.Join([]string{
	"script", "style", "link", "meta", "noscript", "template", "h1", "input", "map",
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
	`[style*="display:none"]`, `[style*="display: none"]`,
}, ", ")

// cleanArticle prepares the content root of a parsed article for conversion.
// The order matters: math and figures read nodes that the removal step drops.
func cleanArticle(root *goquery.Selection) {
	removeFooter(root)
	convertMath(root)
	root.Find(removeSelectors).Remove()
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
		fig.ReplaceWithHtml("<" + tag + ">" + html.EscapeString("[Image: "+caption+"]") + "</" + tag + ">")
	})
}

// convertInfoboxes turns infobox tables into a key/value list
// ("- **Einwohner:** 3.878.100"); header rows become bold paragraphs.
func convertInfoboxes(root *goquery.Selection) {
	root.Find("table.infobox").Each(func(_ int, table *goquery.Selection) {
		if len(table.Nodes) == 0 || table.Nodes[0].Parent == nil {
			return
		}
		var b strings.Builder
		open := false
		closeList := func() {
			if open {
				b.WriteString("</ul>")
				open = false
			}
		}
		item := func(text string) {
			if !open {
				b.WriteString("<ul>")
				open = true
			}
			b.WriteString("<li>" + text + "</li>")
		}
		if caption := collapseSpace(table.ChildrenFiltered("caption").Text()); caption != "" {
			b.WriteString("<p><strong>" + html.EscapeString(caption) + "</strong></p>")
		}
		rows := 0
		table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			if rows >= maxInfoboxRows || !sameNode(tr.Closest("table"), table) {
				return
			}
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
						rows++
						item(html.EscapeString("[Image: " + caption + "]"))
					}
					return
				}
				text := cellText(cell)
				if text == "" {
					return
				}
				rows++
				if goquery.NodeName(cell) == "th" || cell.HasClass("infobox-header") || cell.HasClass("infobox-above") || cell.HasClass("infobox-title") {
					closeList()
					b.WriteString("<p><strong>" + html.EscapeString(text) + "</strong></p>")
					return
				}
				item(html.EscapeString(text))
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
					rows++
					item("<strong>" + html.EscapeString(key) + ":</strong> " + html.EscapeString(value))
				case value != "":
					rows++
					item(html.EscapeString(value))
				case key != "":
					rows++
					closeList()
					b.WriteString("<p><strong>" + html.EscapeString(key) + "</strong></p>")
				}
			}
		})
		closeList()
		if b.Len() == 0 {
			table.Remove()
			return
		}
		table.ReplaceWithHtml(b.String())
	})
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
		table.AfterHtml(fmt.Sprintf("<p>[Table truncated: showing %d of %d rows]</p>", maxTableRows, len(rows)))
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
