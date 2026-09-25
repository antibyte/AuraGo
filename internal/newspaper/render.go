package newspaper

import (
	"bytes"
	"fmt"
	"html"
	"sort"
	"strings"
	"unicode"

	"github.com/phpdave11/gofpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

type editionLabels struct {
	Edition, Partial, Correction, SingleSource, Sources, Retrieved, Published, DateUnknown string
}

func labels(e Edition) editionLabels {
	if strings.HasPrefix(strings.ToLower(e.Language), "de") {
		return editionLabels{
			Edition: "Ausgabe", Partial: "Teilausgabe: Für einige gewünschte Themen gab es keine ausreichend belegten Meldungen.",
			Correction: "Korrektur", SingleSource: "Bericht aus einer Quelle; unabhängig nicht bestätigt.",
			Sources: "Quellen und Kontext", Retrieved: "Abgerufen", Published: "Veröffentlicht", DateUnknown: "Veröffentlichungsdatum unbekannt",
		}
	}
	return editionLabels{
		Edition: "Edition", Partial: "Partial edition: some requested coverage could not be verified.",
		Correction: "Correction", SingleSource: "Single-source report; independently unconfirmed.",
		Sources: "Sources and context", Retrieved: "Retrieved", Published: "Published", DateUnknown: "Publication date unknown",
	}
}

func sectionLabel(language, section string) string {
	if strings.HasPrefix(strings.ToLower(language), "de") {
		if label := (map[string]string{"regional": "Regional", "national": "Deutschland", "international": "International", "politics": "Politik", "economy": "Wirtschaft", "culture": "Kultur", "technology": "Technik", "science": "Wissenschaft", "environment": "Umwelt", "health": "Gesundheit", "sport": "Sport", "interests": "Interessen"})[section]; label != "" {
			return label
		}
	}
	return strings.Title(section)
}

func Text(e Edition) string {
	var b strings.Builder
	l := labels(e)
	fmt.Fprintf(&b, "%s\n%s · %s · %s %d\n\n", e.Title, e.LocalDate, e.Place, l.Edition, e.Revision)
	if e.Partial {
		b.WriteString(l.Partial + "\n\n")
	}
	for _, note := range e.Corrections {
		fmt.Fprintf(&b, "%s: %s\n\n", l.Correction, note)
	}
	previous := ""
	for _, story := range e.Stories {
		if story.Section != previous {
			label := strings.ToUpper(sectionLabel(e.Language, story.Section))
			fmt.Fprintf(&b, "%s\n%s\n\n", label, strings.Repeat("-", len([]rune(label))))
			previous = story.Section
		}
		fmt.Fprintf(&b, "%s\n%s\n", story.Headline, story.Deck)
		if story.SingleSource {
			b.WriteString(l.SingleSource + "\n")
		}
		for _, p := range story.Paragraphs {
			fmt.Fprintf(&b, "\n%s %s\n", p.Text, refs(p.SourceIDs))
		}
		b.WriteString("\n")
	}
	b.WriteString(strings.ToUpper(l.Sources) + "\n")
	for _, s := range e.Sources {
		fmt.Fprintf(&b, "[%s] %s — %s\n%s\n%s: %s", s.ID, s.Publisher, s.Title, s.URL, l.Retrieved, s.RetrievedAt.Format("2006-01-02 15:04 UTC"))
		if s.PublishedAt != nil {
			fmt.Fprintf(&b, " · %s: %s", l.Published, s.PublishedAt.Format("2006-01-02 15:04 UTC"))
		} else {
			b.WriteString(" · " + l.DateUnknown)
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func refs(ids []string) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, "["+id+"]")
	}
	return strings.Join(out, " ")
}

func HTML(e Edition) string {
	var b strings.Builder
	l := labels(e)
	lang := html.EscapeString(e.Language)
	dir := "ltr"
	if strings.HasPrefix(lang, "ar") || strings.HasPrefix(lang, "he") || strings.HasPrefix(lang, "fa") {
		dir = "rtl"
	}
	fmt.Fprintf(&b, "<!doctype html><html lang=\"%s\" dir=\"%s\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>%s</title></head><body style=\"margin:0;background:#eeece5;color:#1d211f;font-family:Arial,sans-serif\"><div style=\"max-width:760px;margin:auto;padding:32px;background:#f7f4ec\"><div style=\"border-top:5px double #1d211f;border-bottom:5px double #1d211f;padding:16px 0;text-align:center\"><small style=\"letter-spacing:.16em;text-transform:uppercase\">%s · %s · %s %d</small><h1 style=\"font-family:Georgia,serif;font-size:56px;line-height:1;margin:10px 0\">%s</h1></div>", lang, dir, html.EscapeString(e.Title), html.EscapeString(e.LocalDate), html.EscapeString(e.Place), html.EscapeString(l.Edition), e.Revision, html.EscapeString(e.Title))
	if e.Partial {
		fmt.Fprintf(&b, "<p style=\"border-bottom:1px solid #8b3338;padding:14px 0;color:#8b3338\">%s</p>", html.EscapeString(l.Partial))
	}
	for _, note := range e.Corrections {
		fmt.Fprintf(&b, "<p style=\"color:#8b3338\"><b>%s:</b> %s</p>", html.EscapeString(l.Correction), html.EscapeString(note))
	}
	previous := ""
	for _, story := range e.Stories {
		if story.Section != previous {
			fmt.Fprintf(&b, "<h2 style=\"font:700 14px Arial,sans-serif;letter-spacing:.14em;text-transform:uppercase;border-top:2px solid #1d211f;padding-top:14px;margin-top:38px\">%s</h2>", html.EscapeString(sectionLabel(e.Language, story.Section)))
			previous = story.Section
		}
		fmt.Fprintf(&b, "<article style=\"border-bottom:1px solid #c8c3b8;padding:8px 0 24px\"><h3 style=\"font:700 32px/1.12 Georgia,serif;margin:12px 0\">%s</h3><p style=\"font:18px/1.5 Georgia,serif;color:#555\">%s</p>", html.EscapeString(story.Headline), html.EscapeString(story.Deck))
		if story.SingleSource {
			fmt.Fprintf(&b, "<p style=\"font-size:12px;color:#8b3338\">%s</p>", html.EscapeString(l.SingleSource))
		}
		for _, p := range story.Paragraphs {
			fmt.Fprintf(&b, "<p style=\"font:17px/1.65 Georgia,serif\">%s <small style=\"color:#8b3338\">%s</small></p>", html.EscapeString(p.Text), html.EscapeString(refs(p.SourceIDs)))
		}
		b.WriteString("</article>")
	}
	fmt.Fprintf(&b, "<h2 style=\"font:700 16px Arial,sans-serif;border-top:2px solid #1d211f;padding-top:14px;margin-top:40px\">%s</h2><ol style=\"padding-left:22px;font:13px/1.6 Arial,sans-serif\">", html.EscapeString(l.Sources))
	for _, s := range e.Sources {
		fmt.Fprintf(&b, "<li style=\"margin-bottom:15px;overflow-wrap:anywhere\"><b>[%s] %s</b> — %s<br><a href=\"%s\" style=\"color:#8b3338\">%s</a><br>%s %s", html.EscapeString(s.ID), html.EscapeString(s.Publisher), html.EscapeString(s.Title), html.EscapeString(s.URL), html.EscapeString(s.URL), html.EscapeString(l.Retrieved), s.RetrievedAt.Format("2006-01-02 15:04 UTC"))
		if s.PublishedAt != nil {
			fmt.Fprintf(&b, " · %s %s", html.EscapeString(l.Published), s.PublishedAt.Format("2006-01-02 15:04 UTC"))
		} else {
			b.WriteString(" · " + html.EscapeString(l.DateUnknown))
		}
		b.WriteString("</li>")
	}
	b.WriteString("</ol></div></body></html>")
	return b.String()
}

// PDF renders the saved revision with local Unicode fonts. Unsupported glyphs
// fail explicitly so Telegram can deliver the complete plain-text alternative.
func PDF(e Edition) ([]byte, error) {
	l := labels(e)
	font, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	for _, char := range Text(e) {
		if unicode.IsSpace(char) {
			continue
		}
		glyph, e := font.GlyphIndex(nil, char)
		if e != nil || glyph == 0 || char > 0xffff || unicode.In(char, unicode.Arabic, unicode.Devanagari, unicode.Hebrew) {
			return nil, fmt.Errorf("PDF font cannot render U+%04X", char)
		}
	}
	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 18, 18)
	p.SetAutoPageBreak(true, 19)
	p.AddUTF8FontFromBytes("News", "", goregular.TTF)
	p.AddUTF8FontFromBytes("News", "B", gobold.TTF)
	p.SetTitle(e.Title, true)
	p.SetAuthor("AuraGo Newspaper", false)
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont("News", "", 8)
		p.SetTextColor(90, 90, 90)
		p.CellFormat(0, 5, fmt.Sprintf("%s · %s · %d", e.Title, e.LocalDate, p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	p.SetDrawColor(29, 33, 31)
	p.SetLineWidth(.5)
	p.Line(18, 16, 192, 16)
	p.SetFont("News", "", 9)
	p.CellFormat(0, 7, fmt.Sprintf("%s · %s · %s %d", e.LocalDate, e.Place, l.Edition, e.Revision), "", 1, "C", false, 0, "")
	p.SetFont("News", "B", 28)
	p.CellFormat(0, 20, e.Title, "", 1, "C", false, 0, "")
	p.Line(18, p.GetY(), 192, p.GetY())
	p.Ln(8)
	write := func(s string, size float64, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		p.SetFont("News", style, size)
		p.MultiCell(0, size*.55, s, "", "L", false)
		p.Ln(2)
	}
	if e.Partial {
		write(l.Partial, 10, true)
	}
	for _, note := range e.Corrections {
		write(l.Correction+": "+note, 10, true)
	}
	previous := ""
	for _, story := range e.Stories {
		if story.Section != previous {
			if p.GetY() > 248 {
				p.AddPage()
			}
			p.Ln(5)
			p.SetDrawColor(139, 51, 56)
			p.Line(18, p.GetY(), 192, p.GetY())
			p.Ln(3)
			write(strings.ToUpper(sectionLabel(e.Language, story.Section)), 11, true)
			previous = story.Section
		}
		if p.GetY() > 247 {
			p.AddPage()
		}
		write(story.Headline, 17, true)
		write(story.Deck, 11, false)
		if story.SingleSource {
			write(l.SingleSource, 8, false)
		}
		for _, para := range story.Paragraphs {
			write(para.Text+" "+refs(para.SourceIDs), 10, false)
		}
		p.Ln(4)
	}
	p.AddPage()
	write(l.Sources, 17, true)
	sources := append([]Source(nil), e.Sources...)
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
	for _, source := range sources {
		write("["+source.ID+"] "+source.Publisher+" — "+source.Title, 10, true)
		write(source.URL, 8, false)
		write(l.Retrieved+" "+source.RetrievedAt.Format("2006-01-02 15:04 UTC"), 8, false)
	}
	var out bytes.Buffer
	if err = p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
