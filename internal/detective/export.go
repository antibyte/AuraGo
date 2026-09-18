package detective

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/phpdave11/gofpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

type Artifact struct {
	SHA256   string
	Data     []byte
	MIME     string
	Filename string
}

func Export(r Report, format string) (Artifact, error) {
	a := Artifact{Filename: fmt.Sprintf("detective-report-r%d.%s", r.Revision, format)}
	var err error
	switch format {
	case "md":
		a.MIME = "text/markdown; charset=utf-8"
		a.Data = []byte(Markdown(r))
	case "pdf":
		a.MIME = "application/pdf"
		a.Data, err = reportPDF(r)
	case "docx":
		a.MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		a.Data, err = reportDOCX(r)
	default:
		err = fmt.Errorf("unsupported export format %q", format)
	}
	return a, err
}

func reportSources(r Report) ([]Source, map[string]int) {
	used := map[string]bool{}
	refs := map[string]string{}
	for _, f := range r.Findings {
		refs[f.ID] = f.SourceID
	}
	for _, b := range r.Blocks {
		for _, e := range b.Evidence {
			used[refs[e]] = true
		}
	}
	sources := []Source{}
	numbers := map[string]int{}
	for _, s := range r.Sources {
		if used[s.ID] {
			sources = append(sources, s)
			numbers[s.ID] = len(sources)
		}
	}
	for ev, src := range refs {
		numbers[ev] = numbers[src]
	}
	return sources, numbers
}
func references(b Block, n map[string]int) string {
	seen := map[int]bool{}
	nums := []int{}
	for _, e := range b.Evidence {
		if v := n[e]; v > 0 && !seen[v] {
			seen[v] = true
			nums = append(nums, v)
		}
	}
	sort.Ints(nums)
	s := ""
	for _, n := range nums {
		s += fmt.Sprintf(" [%d]", n)
	}
	return s
}
func mdText(s string) string {
	return strings.NewReplacer("\\", "\\\\", "<", "&lt;", ">", "&gt;", "[", "\\[", "]", "\\]", "|", "\\|").Replace(s)
}
func Markdown(r Report) string {
	sources, refs := reportSources(r)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", mdText(r.Title), mdText(r.Summary))
	if r.Partial {
		b.WriteString("> Partial research report\n\n")
	}
	fmt.Fprintf(&b, "%s · Revision %d\n\n", r.CreatedAt.Format("2006-01-02"), r.Revision)
	for _, v := range r.Blocks {
		switch v.Type {
		case "heading":
			fmt.Fprintf(&b, "## %s\n\n", mdText(v.Text))
		case "list":
			for _, item := range v.Items {
				fmt.Fprintf(&b, "- %s\n", mdText(item))
			}
			fmt.Fprintln(&b, references(v, refs))
			b.WriteString("\n")
		case "table":
			for i, row := range v.Rows {
				b.WriteString("| ")
				for _, cell := range row {
					b.WriteString(strings.ReplaceAll(mdText(cell), "\n", " ") + " | ")
				}
				b.WriteString("\n")
				if i == 0 {
					b.WriteString("|" + strings.Repeat(" --- |", len(row)) + "\n")
				}
			}
			fmt.Fprintf(&b, "%s\n\n", references(v, refs))
		case "quote":
			fmt.Fprintf(&b, "> %s%s\n\n", strings.ReplaceAll(mdText(v.Text), "\n", "\n> "), references(v, refs))
		default:
			fmt.Fprintf(&b, "%s%s\n\n", mdText(v.Text), references(v, refs))
		}
	}
	if r.Limitations != "" {
		fmt.Fprintf(&b, "## Limitations\n\n%s\n\n", mdText(r.Limitations))
	}
	b.WriteString("## Sources\n\n")
	for i, s := range sources {
		if s.URL == "" {
			fmt.Fprintf(&b, "%d. %s — %s\n", i+1, mdText(s.Title), mdText(s.Locator))
		} else {
			fmt.Fprintf(&b, "%d. [%s](<%s>) — %s\n", i+1, mdText(s.Title), s.URL, s.RetrievedAt.Format("2006-01-02"))
		}
	}
	return b.String()
}

func reportPDF(r Report) ([]byte, error) {
	font, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	for _, char := range Markdown(r) {
		if unicode.IsSpace(char) {
			continue
		}
		glyph, err := font.GlyphIndex(nil, char)
		if err != nil || glyph == 0 || char > 0xffff || unicode.In(char, unicode.Arabic, unicode.Devanagari, unicode.Hebrew) {
			return nil, fmt.Errorf("built-in PDF font cannot render U+%04X reliably; use Word/Markdown or configure Document Creator with Gotenberg for this script", char)
		}
	}
	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 18, 18)
	p.SetAutoPageBreak(true, 18)
	p.AddUTF8FontFromBytes("Go", "", goregular.TTF)
	p.AddUTF8FontFromBytes("Go", "B", gobold.TTF)
	p.SetTitle(r.Title, true)
	p.SetAuthor("AuraGo Detective", false)
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont("Go", "", 8)
		p.CellFormat(0, 5, fmt.Sprint(p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	write := func(text string, size float64, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		p.SetFont("Go", style, size)
		p.MultiCell(0, size*.5, text, "", "L", false)
		p.Ln(3)
	}
	write(r.Title, 20, true)
	write(r.CreatedAt.Format("2006-01-02"), 9, false)
	if r.Partial {
		write("Partial research report", 11, true)
	}
	write(r.Summary, 11, false)
	sources, refs := reportSources(r)
	for _, b := range r.Blocks {
		switch b.Type {
		case "heading":
			if p.GetY() > 250 {
				p.AddPage()
			}
			write(b.Text, 14, true)
		case "list":
			for _, v := range b.Items {
				write("• "+v, 11, false)
			}
		case "table":
			if len(b.Rows) == 0 {
				continue
			}
			cols := len(b.Rows[0])
			if cols == 0 {
				continue
			}
			width := 174 / float64(cols)
			// Split tall rows across pages rather than clipping or overlapping cells.
			drawRow := func(row []string, header bool) {
				style := ""
				if header {
					style = "B"
				}
				p.SetFont("Go", style, 9)
				lines := make([][]string, cols)
				total := 1
				for i := 0; i < cols; i++ {
					if i < len(row) {
						lines[i] = p.SplitText(row[i], width-4)
					}
					total = max(total, len(lines[i]))
				}
				for offset := 0; offset < total; {
					room := int((275 - p.GetY() - 4) / 4.5)
					if room < 1 {
						p.AddPage()
						room = int((275 - p.GetY() - 4) / 4.5)
					}
					count := min(room, total-offset)
					height := float64(count)*4.5 + 4
					x, y := 18.0, p.GetY()
					for i := 0; i < cols; i++ {
						cell := ""
						if offset < len(lines[i]) {
							cell = strings.Join(lines[i][offset:min(offset+count, len(lines[i]))], "\n")
						}
						p.Rect(x+float64(i)*width, y, width, height, "D")
						p.SetXY(x+float64(i)*width+2, y+2)
						p.MultiCell(width-4, 4.5, cell, "", "L", false)
					}
					p.SetXY(x, y+height)
					offset += count
					if offset < total {
						p.AddPage()
					}
				}
			}
			for i, row := range b.Rows {
				p.SetFont("Go", "", 9)
				height := 8.0
				for _, cell := range row {
					height = max(height, float64(len(p.SplitText(cell, width-4)))*4.5+4)
				}
				if p.GetY()+min(height, 240) > 275 {
					p.AddPage()
					if i > 0 {
						drawRow(b.Rows[0], true)
					}
				}
				drawRow(row, i == 0)
			}
			p.Ln(3)
		default:
			write(b.Text, 11, false)
		}
		if text := references(b, refs); text != "" {
			write(strings.TrimSpace(text), 9, false)
		}
	}
	if r.Limitations != "" {
		write("Limitations", 14, true)
		write(r.Limitations, 11, false)
	}
	write("Sources", 14, true)
	for i, s := range sources {
		write(fmt.Sprintf("[%d] %s — %s", i+1, s.Title, s.RetrievedAt.Format("2006-01-02")), 10, true)
		if s.URL == "" {
			write(s.Locator, 9, false)
			continue
		}
		p.SetFont("Go", "", 9)
		p.SetTextColor(30, 65, 130)
		p.WriteLinkString(5, s.URL, s.URL)
		p.Ln(8)
		p.SetTextColor(0, 0, 0)
	}
	var out bytes.Buffer
	err = p.Output(&out)
	return out.Bytes(), err
}

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func paragraph(text, style string) string {
	props := ""
	if style != "" {
		props = `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
	}
	content := strings.ReplaceAll(xmlText(strings.ReplaceAll(text, "\r\n", "\n")), "&#xA;", `</w:t><w:br/><w:t xml:space="preserve">`)
	return `<w:p>` + props + `<w:r><w:t xml:space="preserve">` + content + `</w:t></w:r></w:p>`
}

func reportDOCX(r Report) ([]byte, error) {
	sources, refs := reportSources(r)
	var body strings.Builder
	body.WriteString(paragraph(r.Title, "Title"))
	body.WriteString(paragraph(r.CreatedAt.Format("2006-01-02"), ""))
	if r.Partial {
		body.WriteString(paragraph("Partial research report", "Heading2"))
	}
	body.WriteString(paragraph(r.Summary, ""))
	for _, b := range r.Blocks {
		switch b.Type {
		case "heading":
			body.WriteString(paragraph(b.Text, "Heading1"))
		case "list":
			for _, item := range b.Items {
				body.WriteString(paragraph("• "+item, ""))
			}
		case "table":
			body.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4"/><w:left w:val="single" w:sz="4"/><w:bottom w:val="single" w:sz="4"/><w:right w:val="single" w:sz="4"/><w:insideH w:val="single" w:sz="4"/><w:insideV w:val="single" w:sz="4"/></w:tblBorders></w:tblPr>`)
			for i, row := range b.Rows {
				body.WriteString(`<w:tr>`)
				if i == 0 {
					body.WriteString(`<w:trPr><w:tblHeader/></w:trPr>`)
				}
				for _, cell := range row {
					body.WriteString(`<w:tc>` + paragraph(cell, "") + `</w:tc>`)
				}
				body.WriteString(`</w:tr>`)
			}
			body.WriteString(`</w:tbl>`)
		default:
			body.WriteString(paragraph(b.Text, ""))
		}
		if text := references(b, refs); text != "" {
			body.WriteString(paragraph(strings.TrimSpace(text), ""))
		}
	}
	if r.Limitations != "" {
		body.WriteString(paragraph("Limitations", "Heading1"))
		body.WriteString(paragraph(r.Limitations, ""))
	}
	body.WriteString(paragraph("Sources", "Heading1"))
	rels := `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="styles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/><Relationship Id="footer" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>`
	for i, s := range sources {
		if s.URL == "" {
			body.WriteString(paragraph(fmt.Sprintf("[%d] %s — %s", i+1, s.Title, s.Locator), ""))
			continue
		}
		key := fmt.Sprintf("source%d", i+1)
		rels += `<Relationship Id="` + key + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="` + xmlText(s.URL) + `" TargetMode="External"/>`
		body.WriteString(`<w:p><w:hyperlink r:id="` + key + `"><w:r><w:rPr><w:color w:val="24528A"/><w:u w:val="single"/></w:rPr><w:t>` + xmlText(fmt.Sprintf("[%d] %s — %s", i+1, s.Title, s.URL)) + `</w:t></w:r></w:hyperlink></w:p>`)
	}
	rels += `</Relationships>`
	body.WriteString(`<w:sectPr><w:footerReference w:type="default" r:id="footer"/><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1020" w:right="1020" w:bottom="1020" w:left="1020"/></w:sectPr>`)
	files := map[string]string{
		"[Content_Types].xml":          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/></Types>`,
		"_rels/.rels":                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="document" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":            `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + body.String() + `</w:body></w:document>`,
		"word/_rels/document.xml.rels": rels,
		"word/styles.xml":              `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:sz w:val="22"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:rPr><w:b/><w:sz w:val="40"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:pPr><w:keepNext/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="30"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:pPr><w:keepNext/></w:pPr><w:rPr><w:b/></w:rPr></w:style></w:styles>`,
		"word/footer1.xml":             `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p></w:ftr>`,
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	names := []string{}
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	err := z.Close()
	return out.Bytes(), err
}
