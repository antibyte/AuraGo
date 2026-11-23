package newspaper

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"aurago/internal/webassets"
)

type editionLabels struct {
	Edition, Partial, Correction, SingleSource, Sources, Retrieved, Published, DateUnknown string
	Motto, YourEdition, Checked, About, Page, Checksum                                     string
}

func isGerman(e Edition) bool {
	return strings.HasPrefix(strings.ToLower(e.Language), "de")
}

func labels(e Edition) editionLabels {
	if isGerman(e) {
		return editionLabels{
			Edition: "Ausgabe", Partial: "Teilausgabe: Für einige gewünschte Themen gab es keine ausreichend belegten Meldungen.",
			Correction: "Korrektur", SingleSource: "Bericht aus einer Quelle; unabhängig nicht bestätigt.",
			Sources: "Quellen und Kontext", Retrieved: "Abgerufen", Published: "Veröffentlicht", DateUnknown: "Veröffentlichungsdatum unbekannt",
			Motto: "Deine persönliche Tageszeitung", YourEdition: "Deine Ausgabe", Checked: "Mit geprüften Quellen",
			About: "Über diese Ausgabe", Page: "Seite", Checksum: "Prüfsumme",
		}
	}
	return editionLabels{
		Edition: "Edition", Partial: "Partial edition: some requested coverage could not be verified.",
		Correction: "Correction", SingleSource: "Single-source report; independently unconfirmed.",
		Sources: "Sources and context", Retrieved: "Retrieved", Published: "Published", DateUnknown: "Publication date unknown",
		Motto: "Your personal daily", YourEdition: "Your edition", Checked: "Sources checked",
		About: "About this edition", Page: "Page", Checksum: "Checksum",
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

// articleCount and sourceCount return the singular/plural counts shown in the
// ears and the colophon.
func articleCount(e Edition, n int) string {
	if isGerman(e) {
		return fmt.Sprintf("%d Artikel", n)
	}
	if n == 1 {
		return "1 article"
	}
	return fmt.Sprintf("%d articles", n)
}

func sourceCount(e Edition, n int) string {
	if isGerman(e) {
		if n == 1 {
			return "1 Quelle"
		}
		return fmt.Sprintf("%d Quellen", n)
	}
	if n == 1 {
		return "1 source"
	}
	return fmt.Sprintf("%d sources", n)
}

// longDate renders LocalDate as a spelled-out weekday date; an unparsable
// value is returned unchanged.
func longDate(e Edition) string {
	t, err := time.Parse("2006-01-02", e.LocalDate)
	if err != nil {
		return e.LocalDate
	}
	if isGerman(e) {
		days := []string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}
		months := []string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}
		return fmt.Sprintf("%s, %d. %s %d", days[t.Weekday()], t.Day(), months[t.Month()-1], t.Year())
	}
	return fmt.Sprintf("%s, %s %d, %d", t.Weekday().String(), t.Month().String(), t.Day(), t.Year())
}

func editionTag(e Edition) string {
	return fmt.Sprintf("%s %d", labels(e).Edition, e.Revision)
}

func folioPlace(e Edition) string {
	if e.Place != "" {
		return e.Place
	}
	return labels(e).YourEdition
}

func colophon(e Edition) string {
	l := labels(e)
	n, m := articleCount(e, len(e.Stories)), sourceCount(e, len(e.Sources))
	var b strings.Builder
	cutoff := ""
	if !e.CutoffAt.IsZero() {
		cutoff = e.CutoffAt.UTC().Format("2006-01-02 15:04 UTC")
	}
	if isGerman(e) {
		fmt.Fprintf(&b, "%s aus %s, zusammengestellt von AuraGo.", n, m)
		if cutoff != "" {
			fmt.Fprintf(&b, " Redaktionsschluss %s.", cutoff)
		}
		fmt.Fprintf(&b, " Jeder Absatz nennt in eckigen Klammern die Quellen, auf denen er beruht; sie stehen unter %s.", l.Sources)
	} else {
		fmt.Fprintf(&b, "%s from %s, compiled by AuraGo.", n, m)
		if cutoff != "" {
			fmt.Fprintf(&b, " Editorial cutoff %s.", cutoff)
		}
		fmt.Fprintf(&b, " Each paragraph names the sources it rests on in square brackets; they are listed under %s.", l.Sources)
	}
	b.WriteString(" " + editionTag(e))
	if e.Hash != "" {
		hash := e.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}
		fmt.Fprintf(&b, " · %s %s", l.Checksum, hash)
	}
	return b.String()
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

// sourceDates is the shared "Retrieved … · Published …" line of HTML and PDF.
func sourceDatePhrases(l editionLabels, s Source) (string, string) {
	retrieved := l.Retrieved + " " + s.RetrievedAt.Format("2006-01-02 15:04 UTC")
	if s.PublishedAt != nil {
		return retrieved, l.Published + " " + s.PublishedAt.Format("2006-01-02 15:04 UTC")
	}
	return retrieved, l.DateUnknown
}

func sourceDates(l editionLabels, s Source) string {
	a, b := sourceDatePhrases(l, s)
	return a + " · " + b
}

// safeLinkURL reports whether raw is a plain http(s) URL with a host.
func safeLinkURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

const (
	htmlInk    = "#1b1b19"
	htmlMuted  = "#55554e"
	htmlRule   = "#bcb7aa"
	htmlAccent = "#8b3338"
	htmlSerif  = "font-family:Georgia,'Times New Roman',Times,serif"
	htmlSans   = "font-family:Arial,Helvetica,sans-serif"
)

func htmlEar(top, bottom string) string {
	return fmt.Sprintf("<table role=\"presentation\" width=\"100%%\" height=\"62\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\" style=\"height:62px;border:1px solid %s\"><tr><td valign=\"middle\" align=\"center\" style=\"padding:6px 8px;text-align:center\"><div style=\"%s;font-size:14px;line-height:1.2;font-weight:700;color:%s\">%s</div><div style=\"%s;font-size:10px;line-height:1.3;font-weight:400;text-transform:uppercase;letter-spacing:.1em;color:%s\">%s</div></td></tr></table>",
		htmlInk, htmlSerif, htmlInk, html.EscapeString(top), htmlSans, htmlMuted, html.EscapeString(strings.ToUpper(bottom)))
}

func HTML(e Edition) string {
	var b strings.Builder
	l := labels(e)
	lang := html.EscapeString(e.Language)
	dir, first, last := "ltr", "left", "right"
	if strings.HasPrefix(lang, "ar") || strings.HasPrefix(lang, "he") || strings.HasPrefix(lang, "fa") {
		dir, first, last = "rtl", "right", "left"
	}
	esc := html.EscapeString
	tag := editionTag(e)
	fmt.Fprintf(&b, "<!doctype html><html lang=\"%s\" dir=\"%s\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><meta name=\"color-scheme\" content=\"light only\"><meta name=\"supported-color-schemes\" content=\"light only\"><title>%s</title>", lang, dir, esc(e.Title))
	b.WriteString("<style>@media only screen and (max-width:620px){.np-ear{display:none!important;width:0!important}.np-pad{padding:20px 16px 26px!important}.np-title{font-size:40px!important}.np-folio td{display:block!important;width:auto!important;text-align:center!important;padding:2px 0!important}}</style></head>")
	fmt.Fprintf(&b, "<body style=\"margin:0;padding:0;background:#e7e3d8;color:%s\">", htmlInk)
	if len(e.Stories) > 0 {
		fmt.Fprintf(&b, "<div style=\"display:none;max-height:0;overflow:hidden;mso-hide:all;font-size:1px;line-height:1px;color:#e7e3d8;opacity:0\">%s — %s</div>", esc(e.Stories[0].Headline), esc(e.Stories[0].Deck))
	}
	b.WriteString("<table role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\" style=\"background:#e7e3d8\"><tr><td align=\"center\" style=\"padding:24px 12px\">")
	b.WriteString("<table role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\" style=\"max-width:720px;background:#f5f2e9;border:1px solid #d6d1c4\"><tr><td class=\"np-pad\" style=\"padding:30px 34px 34px;text-align:" + first + "\">")

	// Masthead: ears around the nameplate.
	b.WriteString("<table role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\"><tr>")
	fmt.Fprintf(&b, "<td class=\"np-ear\" width=\"126\" valign=\"middle\" style=\"width:126px\">%s</td>", htmlEar(articleCount(e, len(e.Stories)), sourceCount(e, len(e.Sources))))
	fmt.Fprintf(&b, "<td align=\"center\" valign=\"middle\" style=\"padding:0 8px;text-align:center\"><h1 class=\"np-title\" style=\"%s;font-size:54px;line-height:1;font-weight:700;letter-spacing:-1px;text-align:center;color:%s;margin:0\">%s</h1></td>", htmlSerif, htmlInk, esc(e.Title))
	fmt.Fprintf(&b, "<td class=\"np-ear\" width=\"126\" valign=\"middle\" style=\"width:126px\">%s</td>", htmlEar(l.Checked, tag))
	b.WriteString("</tr></table>")
	fmt.Fprintf(&b, "<p style=\"%s;font-size:15px;line-height:1.4;font-style:italic;text-align:center;color:%s;margin:8px 0 10px\">%s</p>", htmlSerif, htmlMuted, esc(l.Motto))
	folio := fmt.Sprintf("padding:6px 2px;%s;font-size:10px;line-height:1.4;font-weight:700;text-transform:uppercase;letter-spacing:.12em;color:%s", htmlSans, htmlMuted)
	fmt.Fprintf(&b, "<table class=\"np-folio\" role=\"presentation\" width=\"100%%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\" style=\"border-top:1px solid %s;border-bottom:1px solid %s\"><tr>", htmlInk, htmlInk)
	fmt.Fprintf(&b, "<td align=\"%s\" style=\"%s;text-align:%s\">%s</td>", first, folio, first, esc(strings.ToUpper(folioPlace(e))))
	fmt.Fprintf(&b, "<td align=\"center\" style=\"%s;text-align:center\">%s</td>", folio, esc(strings.ToUpper(longDate(e))))
	fmt.Fprintf(&b, "<td align=\"%s\" style=\"%s;text-align:%s\">%s</td>", last, folio, last, esc(strings.ToUpper(tag)))
	b.WriteString("</tr></table>")
	fmt.Fprintf(&b, "<div style=\"height:3px;background:%s;margin-top:3px;font-size:0;line-height:3px\">&nbsp;</div>", htmlInk)

	box := fmt.Sprintf("border:1px solid %s;border-left:4px solid %s;padding:10px 14px;margin:18px 0 0;%s;font-size:15px;line-height:1.45;color:%s", htmlInk, htmlInk, htmlSerif, htmlInk)
	if e.Partial {
		fmt.Fprintf(&b, "<p style=\"%s\">%s</p>", box, esc(l.Partial))
	}
	for _, note := range e.Corrections {
		fmt.Fprintf(&b, "<p style=\"%s\"><b style=\"%s;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:%s\">%s:</b> %s</p>", box, htmlSans, htmlAccent, esc(l.Correction), esc(note))
	}

	previous := ""
	for i, story := range e.Stories {
		if story.Section != previous {
			fmt.Fprintf(&b, "<h2 style=\"margin:24px 0 0;padding:0 0 5px;border-bottom:2px solid %s;%s;font-size:11px;line-height:1.4;font-weight:700;text-transform:uppercase;letter-spacing:.14em;color:%s\">%s</h2>", htmlInk, htmlSans, htmlAccent, esc(sectionLabel(e.Language, story.Section)))
			previous = story.Section
		}
		border := "border-bottom:1px solid " + htmlRule
		if i+1 >= len(e.Stories) || e.Stories[i+1].Section != story.Section {
			border = "border-bottom:0"
		}
		fmt.Fprintf(&b, "<article style=\"padding:16px 0 22px;%s\">", border)
		size, lead := "26px", "1.15"
		if i == 0 {
			size, lead = "34px", "1.08"
		}
		fmt.Fprintf(&b, "<h3 style=\"%s;font-size:%s;line-height:%s;font-weight:700;letter-spacing:-.01em;color:%s;margin:0 0 8px\">%s</h3>", htmlSerif, size, lead, htmlInk, esc(story.Headline))
		if story.Deck != "" {
			fmt.Fprintf(&b, "<p style=\"%s;font-size:17px;line-height:1.45;font-weight:700;color:%s;margin:0 0 12px\">%s</p>", htmlSerif, htmlInk, esc(story.Deck))
		}
		if story.SingleSource {
			fmt.Fprintf(&b, "<p style=\"margin:0 0 12px;padding:7px 10px;border:1px solid %s;%s;font-size:13px;line-height:1.45;color:%s\">%s</p>", htmlAccent, htmlSerif, htmlInk, esc(l.SingleSource))
		}
		for j, p := range story.Paragraphs {
			indent := "1.4em"
			if j == 0 {
				indent = "0"
			}
			fmt.Fprintf(&b, "<p style=\"margin:0;text-indent:%s;%s;font-size:16px;line-height:1.6;color:%s\">%s&#160;<sup style=\"%s;font-size:10px;line-height:0;font-weight:700;white-space:nowrap;color:%s\">%s</sup></p>", indent, htmlSerif, htmlInk, esc(p.Text), htmlSans, htmlAccent, esc(refs(p.SourceIDs)))
		}
		b.WriteString("</article>")
	}

	fmt.Fprintf(&b, "<h2 style=\"margin:36px 0 12px;padding:6px 0;border-top:3px solid %s;border-bottom:1px solid %s;%s;font-size:22px;line-height:1.3;font-weight:700;color:%s\">%s</h2>", htmlInk, htmlInk, htmlSerif, htmlInk, esc(l.Sources))
	for _, s := range e.Sources {
		link := esc(s.URL)
		if safeLinkURL(s.URL) {
			link = fmt.Sprintf("<a href=\"%s\" style=\"color:%s\">%s</a>", esc(s.URL), htmlAccent, esc(s.URL))
		} else {
			link = fmt.Sprintf("<span style=\"color:%s\">%s</span>", htmlAccent, link)
		}
		ra, pb := sourceDatePhrases(l, s)
		dates := "<span style=\"white-space:nowrap\">" + esc(ra) + "</span> · <span style=\"white-space:nowrap\">" + esc(pb) + "</span>"
		fmt.Fprintf(&b, "<p style=\"margin:0 0 12px;%s;font-size:12px;line-height:1.5;color:%s;overflow-wrap:anywhere;word-break:break-word\"><b style=\"color:%s\">[%s] %s</b> — %s<br>%s<br>%s</p>", htmlSans, htmlMuted, htmlInk, esc(s.ID), esc(s.Publisher), esc(s.Title), link, dates)
	}

	fmt.Fprintf(&b, "<div style=\"margin-top:30px;border-top:3px double %s;padding-top:9px\">", htmlInk)
	foot := fmt.Sprintf("%s;font-size:10px;line-height:1.4;text-transform:uppercase;letter-spacing:.1em;color:%s", htmlSans, htmlMuted)
	fmt.Fprintf(&b, "<table class=\"np-folio\" role=\"presentation\" width=\"100%%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\"><tr><td align=\"%s\" style=\"%s;text-align:%s\">%s</td><td align=\"%s\" style=\"%s;text-align:%s\">%s · %s</td></tr></table>", first, foot, first, esc(e.Title), last, foot, last, esc(longDate(e)), esc(tag))
	fmt.Fprintf(&b, "<p style=\"margin:10px 0 0;%s;font-size:11px;line-height:1.5;color:%s\"><b style=\"%s;font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:.1em;color:%s\">%s</b> %s</p>", htmlSerif, htmlMuted, htmlSans, htmlInk, esc(l.About), esc(colophon(e)))
	b.WriteString("</div></td></tr></table></td></tr></table></body></html>")
	return b.String()
}

// PDF renders the saved revision with local Unicode fonts. Unsupported glyphs
// fail explicitly so Telegram can deliver the complete plain-text alternative.
func PDF(e Edition) ([]byte, error) {
	return renderPDF(e, webassets.Default.Namespace("newspaper/fonts"))
}
