package detective

import (
	"fmt"
	"html"
	"strings"
)

// ReportHTML deliberately contains no scripts, remote resources or raw markup.
func ReportHTML(r Report) string {
	e := html.EscapeString
	var out strings.Builder
	out.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'"><style>@page{size:A4;margin:20mm}body{font:11pt sans-serif;line-height:1.5;color:#16202c}h1{font-size:24pt}h2{break-after:avoid;font-size:16pt}p,li,td{white-space:pre-wrap;overflow-wrap:anywhere}table{border-collapse:collapse;width:100%;font-size:9pt}td,th{border:1px solid #aaa;padding:7px;text-align:left}thead{display:table-header-group}a{color:#24528a}small{font-size:9pt}</style></head><body>`)
	fmt.Fprintf(&out, "<h1>%s</h1><small>%s · Revision %d</small>", e(r.Title), r.CreatedAt.Format("2006-01-02"), r.Revision)
	if r.Partial {
		out.WriteString("<p><b>Partial research report</b></p>")
	}
	fmt.Fprintf(&out, "<p>%s</p>", e(r.Summary))
	sources, refs := reportSources(r)
	for _, b := range r.Blocks {
		switch b.Type {
		case "heading":
			fmt.Fprintf(&out, "<h2>%s</h2>", e(b.Text))
		case "list":
			out.WriteString("<ul>")
			for _, v := range b.Items {
				fmt.Fprintf(&out, "<li>%s</li>", e(v))
			}
			out.WriteString("</ul>")
		case "table":
			out.WriteString("<table>")
			for i, row := range b.Rows {
				if i == 0 {
					out.WriteString("<thead>")
				} else if i == 1 {
					out.WriteString("<tbody>")
				}
				out.WriteString("<tr>")
				for _, v := range row {
					fmt.Fprintf(&out, "<td>%s</td>", e(v))
				}
				out.WriteString("</tr>")
				if i == 0 {
					out.WriteString("</thead>")
				}
			}
			if len(b.Rows) > 1 {
				out.WriteString("</tbody>")
			}
			out.WriteString("</table>")
		default:
			fmt.Fprintf(&out, "<p>%s</p>", e(b.Text))
		}
		if refs := references(b, refs); refs != "" {
			fmt.Fprintf(&out, "<small>%s</small>", e(refs))
		}
	}
	if r.Limitations != "" {
		fmt.Fprintf(&out, "<h2>Limitations</h2><p>%s</p>", e(r.Limitations))
	}
	out.WriteString("<h2>Sources</h2><ol>")
	for _, s := range sources {
		if s.URL != "" {
			fmt.Fprintf(&out, `<li><a href="%s">%s</a> — %s</li>`, e(s.URL), e(s.Title), s.RetrievedAt.Format("2006-01-02"))
		} else {
			fmt.Fprintf(&out, "<li>%s — %s</li>", e(s.Title), e(s.Locator))
		}
	}
	out.WriteString("</ol></body></html>")
	return out.String()
}
