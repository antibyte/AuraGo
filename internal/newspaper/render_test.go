package newspaper

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func par(text string, ids ...string) Paragraph {
	return Paragraph{Text: text, SourceIDs: ids}
}

func mkStory(id, section, headline, deck string, single bool, paras ...Paragraph) Story {
	s := Story{ID: id, Section: section, Headline: headline, Deck: deck, SingleSource: single, Paragraphs: paras}
	seen := map[string]bool{}
	for _, p := range paras {
		for _, sid := range p.SourceIDs {
			if !seen[sid] {
				seen[sid] = true
				s.SourceIDs = append(s.SourceIDs, sid)
			}
		}
	}
	return s
}

func TestHTMLEscapesAndKeepsStructure(t *testing.T) {
	const evil = `<script>alert(1)</script> "&`
	pub := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	e := Edition{
		Title: "T " + evil, Language: "de", LocalDate: "2026-09-25", Place: "P " + evil, Revision: 2, Partial: true,
		Corrections: []string{"Note " + evil},
		Stories: []Story{
			mkStory("a", "politics", "H1 "+evil, "D1 "+evil, false, par("One "+evil, "src-1"), par("Two second", "src-1", "src-2")),
			mkStory("b", "politics", "H2", "D2", true, par("Three", "src-2")),
			mkStory("c", "sport", "H3", "D3", false, par("Four", "src-1")),
		},
		Sources: []Source{
			{ID: "src-1", URL: "javascript:alert(1)", Publisher: "Pub " + evil, Title: "Title " + evil, RetrievedAt: pub},
			{ID: "src-2", URL: "https://example.org/a?b=1&c=2", Publisher: "Ok", Title: "Fine", RetrievedAt: pub, PublishedAt: &pub},
		},
	}
	out := HTML(e)
	if strings.Contains(out, "<script>") {
		t.Fatal("raw script tag in HTML")
	}
	for _, want := range []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "&#34;&amp;", "Deine persönliche Tageszeitung", "Freitag, 25. September 2026", "Korrektur", labels(e).Partial, "text-indent:1.4em", "[src-1] [src-2]", "Bericht aus einer Quelle", `href="https://example.org/a?b=1&amp;c=2"`} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML misses %q", want)
		}
	}
	if got := strings.Count(out, "<article "); got != len(e.Stories) {
		t.Fatalf("article count %d", got)
	}
	if strings.Contains(out, `href="javascript`) {
		t.Fatal("unsafe URL rendered as link")
	}
	if !strings.Contains(out, "javascript:alert(1)") {
		t.Fatal("unsafe URL text dropped")
	}
	e.Language = "en"
	if en := HTML(e); !strings.Contains(en, "Friday, September 25, 2026") || !strings.Contains(en, "Your personal daily") {
		t.Fatal("English labels missing")
	}
	if longDate(Edition{Language: "de", LocalDate: "unknown"}) != "unknown" {
		t.Fatal("invalid date must pass through")
	}
}

func TestPDFMultiPageLayout(t *testing.T) {
	long := strings.Repeat("Der Stadtrat hat am Donnerstag nach langer Aussprache mehrere Beschlüsse gefasst, die den Haushalt der kommenden Jahre prägen werden. ", 4)
	sections := []string{"regional", "regional", "politics", "economy", "economy", "culture", "science", "sport", "sport"}
	var stories []Story
	for i, sec := range sections {
		stories = append(stories, mkStory(fmt.Sprintf("s%d", i), sec, fmt.Sprintf("Überschrift Nummer %d mit etwas längerem Text", i), "Eine Unterzeile, die den Inhalt kurz zusammenfasst.", i == 3,
			par(long, "src-1"), par(long, "src-2", "src-3"), par(long, "src-1"), par(long, "src-3")))
	}
	pub := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	longURL := "https://example.org/" + strings.Repeat("abcdefghij", 30)
	e := Edition{
		Title: strings.Repeat("W", 70), Language: "de", LocalDate: "unknown", Revision: 1, Partial: true, Corrections: []string{"Eine Korrektur zum Vortag."},
		Stories: stories,
		Sources: []Source{
			{ID: "src-1", URL: longURL, Publisher: "Pub", Title: "Titel", RetrievedAt: pub},
			{ID: "src-2", URL: "https://example.org/2", Publisher: "Pub", Title: "Titel", RetrievedAt: pub, PublishedAt: &pub},
			{ID: "src-3", URL: "https://example.org/3", Publisher: "Pub", Title: "Titel", RetrievedAt: pub},
		},
	}
	pdf, err := renderPDF(e, os.DirFS("fonts"))
	if err != nil || !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("PDF failed: %v", err)
	}
	pages := len(regexp.MustCompile(`/Type /Page[^s]`).FindAll(pdf, -1))
	if pages < 2 {
		t.Fatalf("expected several pages, got %d", pages)
	}
	// Short title and no stories must render as well.
	if _, err := renderPDF(Edition{Title: "Kurz", Language: "en", LocalDate: "2026-09-25"}, os.DirFS("fonts")); err != nil {
		t.Fatal(err)
	}
}

func sampleEdition(lang string) Edition {
	de := lang == "de"
	pick := func(g, en string) string {
		if de {
			return g
		}
		return en
	}
	t0 := time.Date(2026, 11, 23, 4, 30, 0, 0, time.UTC)
	ret := t0.Add(time.Hour)
	pub := func(h int) *time.Time { v := t0.Add(-time.Duration(h) * time.Hour); return &v }
	src := func(n int, publisher, title, link string, published *time.Time) Source {
		return Source{ID: fmt.Sprintf("src-%d", n), URL: link, Publisher: publisher, Title: title, PublishedAt: published, RetrievedAt: ret}
	}
	sources := []Source{
		src(1, pick("Pforzheimer Tagblatt", "Bristol Courier"), pick("Gemeinderat beschließt Neubau der Enzbrücke", "Council approves new harbour footbridge"), "https://example.org/regional/bruecke", pub(9)),
		src(2, pick("Nordschwarzwald Rundschau", "West Country Review"), pick("Verkehrsbetriebe planen dichteren Takt", "Bus operators plan a denser timetable"), "https://example.org/regional/takt", nil),
		src(3, pick("Berliner Depesche", "Capital Dispatch"), pick("Koalition einigt sich auf Haushaltsrahmen", "Coalition agrees budget framework"), "https://example.com/politik/haushalt?ressort=politik&id=48213&ref=newsletter&utm_source=morgen", pub(14)),
		src(4, pick("Handelsblatt Süd", "Trade Ledger"), pick("Mittelstand investiert wieder mehr", "Mid-sized firms invest more again"), "https://example.net/wirtschaft/investitionen", pub(6)),
		src(5, pick("Wirtschaftswoche Nord", "Market Weekly"), pick("Energiepreise geben leicht nach", "Energy prices ease slightly"), "https://example.net/wirtschaft/energie", nil),
		src(6, pick("Kulturbote", "Arts Herald"), pick("Stadttheater eröffnet Spielzeit mit Uraufführung", "City theatre opens its season with a premiere"), "https://example.org/kultur/theater", pub(20)),
		src(7, pick("Technikblick", "Tech Observer"), pick("Kommunen testen offene Mobilitätsdaten", "Councils test open mobility data"), "https://example.com/technik/mobilitaet", pub(3)),
		src(8, pick("Forschung aktuell", "Science Today"), pick("Forscher kartieren Moorflächen per Satellit", "Researchers map peatland from orbit"), "https://example.org/wissenschaft/moor", pub(30)),
		src(9, pick("Sportspiegel Regional", "Regional Sport Desk"), pick("Heimsieg im Kellerduell", "Home win in a relegation battle"), "https://example.org/sport/derby", pub(2)),
		src(10, pick("Landesanzeiger", "County Gazette"), pick("Amtliche Bekanntmachung zum Bauvorhaben", "Official notice on the building project"), "https://example.org/amt/bekanntmachung", nil),
		src(11, pick("Wetterdienst Südwest", "Coastal Weather Service"), pick("Mildes Wochenende, dann Frontdurchgang", "Mild weekend, then a front moves in"), "https://example.org/wetter/wochenende", pub(1)),
	}
	stories := []Story{
		mkStory("s1", "regional", pick("Neue Brücke über die Enz soll 2028 fertig sein", "New footbridge over the harbour due in 2028"), pick("Der Gemeinderat hat den Neubau nach monatelanger Debatte beschlossen; die Kosten liegen bei knapp zwölf Millionen Euro.", "The council has approved the project after months of debate; the cost is put at just under twelve million pounds."), false,
			par(pick("Der Gemeinderat hat am Abend den Neubau der Fußgängerbrücke beschlossen. Die Abstimmung fiel mit großer Mehrheit aus. Gegenstimmen kamen vor allem aus den Reihen der Fraktion, die auf die angespannte Haushaltslage verwies.", "The council approved the new footbridge on Monday evening. The vote passed with a large majority. Objections came mainly from members who pointed to the strained budget."), "src-1"),
			par(pick("Die Verwaltung rechnet mit Gesamtkosten von knapp zwölf Millionen Euro. Etwa die Hälfte soll aus Fördermitteln des Landes stammen. Ein Antrag liegt bereits vor.", "Officials expect total costs of just under twelve million pounds. About half is to come from regional grants. An application has already been filed."), "src-1", "src-10"),
			par(pick("Der Baubeginn ist für das Frühjahr 2027 geplant. Während der Arbeiten bleibt der alte Steg für Radfahrer gesperrt. Eine Umleitung wird ausgeschildert.", "Construction is planned to start in spring 2027. The old walkway stays closed to cyclists during the works. A signed diversion will be set up."), "src-10"),
			par(pick("Anwohner hatten sich in einer Bürgerversammlung mehrheitlich für den Neubau ausgesprochen. Kritik gab es an der geplanten Höhe der Pfeiler. Die Planer wollen die Bedenken prüfen.", "Residents had backed the new structure at a public meeting. Criticism focused on the planned height of the piers. The designers say they will review the concerns."), "src-1")),
		mkStory("s2", "regional", pick("Busse sollen künftig im Zehn-Minuten-Takt fahren", "Buses to run every ten minutes"), pick("Auf drei Linien wird das Angebot ab Dezember ausgeweitet.", "Three routes will get a denser service from December."), true,
			par(pick("Die Verkehrsbetriebe wollen das Angebot auf drei Linien ausweiten. Ab Dezember soll tagsüber alle zehn Minuten ein Bus fahren. Abends bleibt es beim bisherigen Takt.", "The operators want to expand service on three routes. From December a bus should run every ten minutes during the day. Evenings keep the current timetable."), "src-2"),
			par(pick("Finanziert wird die Ausweitung zunächst für ein Jahr. Danach soll anhand der Fahrgastzahlen entschieden werden. Die Stadt beteiligt sich mit einem Zuschuss.", "The expansion is funded for one year first. Ridership figures will then decide how it continues. The city contributes a subsidy."), "src-2"),
			par(pick("Fahrgastverbände begrüßen den Schritt, fordern aber verlässliche Anschlüsse. Besonders am Hauptbahnhof gebe es häufig Wartezeiten. Dazu äußerte sich der Betreiber nicht.", "Passenger groups welcome the step but demand dependable connections. Waiting times are often long at the main station. The operator did not comment on that."), "src-2")),
		mkStory("s3", "politics", pick("Koalition einigt sich auf Haushaltsrahmen für 2027", "Coalition agrees budget framework for 2027"), pick("Nach zähen Verhandlungen steht der Rahmen; strittig bleibt die Finanzierung der Schulen.", "After tough talks the framework stands; school funding remains disputed."), false,
			par(pick("Die Regierungsparteien haben sich auf einen Rahmen für den Haushalt des kommenden Jahres verständigt. Die Neuverschuldung soll unter der erlaubten Grenze bleiben. Details sollen in der kommenden Woche vorgestellt werden.", "The governing parties have settled on a framework for next year's budget. New borrowing is to stay below the permitted limit. Details are due to be presented next week."), "src-3"),
			par(pick("Strittig bleibt die Finanzierung der Schulen. Die Opposition spricht von einer Mogelpackung. Die Koalition weist den Vorwurf zurück.", "School funding remains disputed. The opposition speaks of a sham. The coalition rejects the charge."), "src-3", "src-4"),
			par(pick("Wirtschaftsverbände äußerten sich verhalten. Sie verlangen mehr Tempo bei der Entlastung der Betriebe. Das Papier lasse dazu wenig erkennen.", "Business groups reacted cautiously. They want faster relief for companies. The paper says little on that."), "src-4")),
		mkStory("s4", "economy", pick("Mittelstand investiert wieder mehr", "Mid-sized firms invest more again"), pick("Eine Umfrage zeigt, dass jedes zweite Unternehmen Ausgaben erhöhen will.", "A survey shows that every second company plans to raise spending."), false,
			par(pick("Mittelständische Unternehmen wollen im nächsten Jahr wieder mehr investieren. Das ergab eine Umfrage unter rund tausend Betrieben. Jeder zweite will die Ausgaben erhöhen.", "Mid-sized companies plan to invest more next year. That is the result of a survey of around a thousand firms. One in two wants to raise spending."), "src-4"),
			par(pick("Vor allem in Digitalisierung und Energieeffizienz soll mehr Geld fließen. Die Befragten nannten auch den Fachkräftemangel als Bremse. Viele Stellen blieben monatelang unbesetzt.", "Most of the money is to flow into digitisation and energy efficiency. Respondents also named the skills shortage as a brake. Many posts stay vacant for months."), "src-4", "src-5"),
			par(pick("Die Autoren warnen vor zu viel Optimismus. Die Lage hänge stark von den Energiepreisen ab. Diese gaben zuletzt leicht nach.", "The authors warn against too much optimism. The outlook depends heavily on energy prices. Those eased slightly of late."), "src-5")),
		mkStory("s5", "economy", pick("Energiepreise geben leicht nach", "Energy prices ease slightly"), pick("Gas und Strom werden an den Börsen etwas günstiger gehandelt.", "Gas and electricity trade a little cheaper on the exchanges."), false,
			par(pick("An den Energiebörsen sind die Preise für Gas und Strom leicht gesunken. Händler führen das auf das milde Wetter zurück. Die Speicher sind gut gefüllt.", "Prices for gas and electricity have fallen slightly on the energy exchanges. Traders put this down to the mild weather. Storage is well stocked."), "src-5"),
			par(pick("Für Haushalte ändert sich kurzfristig wenig. Neue Tarife werden erst zum Jahreswechsel angepasst. Ein Vergleich lohnt sich dennoch.", "For households little changes in the short term. New tariffs are only adjusted at the turn of the year. A comparison is still worthwhile."), "src-5", "src-11")),
		mkStory("s6", "culture", pick("Stadttheater eröffnet Spielzeit mit Uraufführung", "City theatre opens its season with a premiere"), pick("Das neue Stück erzählt von einer Familie zwischen Stadt und Land.", "The new play tells of a family caught between town and country."), false,
			par(pick("Das Stadttheater hat seine Spielzeit mit einer Uraufführung eröffnet. Das Stück erzählt von einer Familie zwischen Stadt und Land. Das Publikum applaudierte lange.", "The city theatre opened its season with a world premiere. The play tells of a family caught between town and country. The audience applauded for a long time."), "src-6"),
			par(pick("Die Inszenierung setzt auf wenig Bühnenbild und viel Musik. Eine Band spielt live auf der Bühne. Kritiker lobten vor allem das Ensemble.", "The staging relies on a spare set and plenty of music. A band plays live on stage. Critics praised the ensemble above all."), "src-6"),
			par(pick("Bis Weihnachten sind zwölf weitere Vorstellungen angesetzt. Für einige gibt es noch Karten. Ein Zusatztermin wird erwogen.", "Twelve more performances are scheduled before Christmas. Tickets remain for some of them. An extra date is being considered."), "src-6")),
		mkStory("s7", "technology", pick("Kommunen testen offene Mobilitätsdaten", "Councils test open mobility data"), pick("Fahrpläne und Verkehrszahlen sollen frei nutzbar werden.", "Timetables and traffic counts are to become freely usable."), false,
			par(pick("Mehrere Kommunen testen die freie Bereitstellung von Mobilitätsdaten. Fahrpläne, Verkehrszahlen und Leihradstandorte sollen über eine gemeinsame Schnittstelle abrufbar sein. Entwickler können die Daten kostenlos nutzen.", "Several councils are testing the open release of mobility data. Timetables, traffic counts and bike-share locations are to be available through a shared interface. Developers can use the data free of charge."), "src-7"),
			par(pick("Die Projektleiter hoffen auf neue Apps für Pendler. Datenschutz sei von Beginn an mitgedacht worden. Personenbezogene Angaben würden nicht veröffentlicht.", "Project leads hope for new apps for commuters. Privacy was considered from the start. No personal data will be published."), "src-7", "src-2"),
			par(pick("Nach einem halben Jahr soll eine erste Bilanz gezogen werden. Dann wird entschieden, ob weitere Städte teilnehmen. Interessenten gibt es bereits.", "A first review is due after six months. Then it will be decided whether more cities join. There are already interested parties."), "src-7")),
		mkStory("s8", "science", pick("Forscher kartieren Moorflächen per Satellit", "Researchers map peatland from orbit"), pick("Die neue Karte zeigt, wo Wiedervernässung den meisten Kohlenstoff bindet.", "The new map shows where rewetting would lock away the most carbon."), false,
			par(pick("Ein Forschungsteam hat Moorflächen erstmals flächendeckend per Satellit erfasst. Die Karte zeigt, wo Wiedervernässung den meisten Kohlenstoff binden würde. Grundlage sind Radardaten aus mehreren Jahren.", "A research team has mapped peatland across a whole region from orbit for the first time. The map shows where rewetting would lock away the most carbon. It is based on several years of radar data."), "src-8"),
			par(pick("Trockengelegte Moore gelten als bedeutende Quelle von Treibhausgasen. Die Autoren sehen in der Karte ein Werkzeug für Behörden. Landwirte sollen früh eingebunden werden.", "Drained peatlands are considered a major source of greenhouse gases. The authors see the map as a tool for authorities. Farmers are to be involved early."), "src-8"),
			par(pick("Die Daten stehen öffentlich zur Verfügung. Eine Überprüfung vor Ort soll die Genauigkeit sichern. Erste Ergebnisse werden im Frühjahr erwartet.", "The data is publicly available. Checks on the ground are meant to confirm accuracy. First results are expected in spring."), "src-8", "src-11")),
		mkStory("s9", "sport", pick("Heimsieg im Kellerduell", "Home win in a relegation battle"), pick("Ein spätes Tor sichert den Gastgebern drei wichtige Punkte.", "A late goal secures three vital points for the hosts."), false,
			par(pick("Die Gastgeber haben das Kellerduell mit 2:1 gewonnen. Ein spätes Tor in der 88. Minute brachte die Entscheidung. Die Gäste hatten zuvor ausgeglichen.", "The hosts won the relegation battle 2-1. A late goal in the 88th minute decided it. The visitors had levelled earlier."), "src-9"),
			par(pick("Der Trainer sprach von einem verdienten Erfolg. Seine Mannschaft habe Moral gezeigt. Am nächsten Wochenende wartet ein schwerer Gegner.", "The coach called it a deserved win. His side had shown character. A tough opponent awaits next weekend."), "src-9"),
			par(pick("Damit klettert das Team auf den vorletzten Tabellenplatz. Der Abstand zum rettenden Ufer beträgt nur noch einen Punkt. Die Fans feierten bis in die Nacht.", "The team climbs to second from bottom. The gap to safety is now just one point. Fans celebrated into the night."), "src-9", "src-11")),
	}
	title, place := pick("Der Morgen", "The Morning Ledger"), pick("Pforzheim", "Bristol")
	e := Edition{
		ID: "sample-" + lang, LocalDate: "2026-11-23", Revision: 2, Title: title, Language: lang, Place: place,
		CreatedAt: t0.Add(2 * time.Hour), CutoffAt: t0.Add(time.Hour), Partial: true,
		Hash:        "3f9a1c07be52d84e61a0c97f2b35d8e4a61f0c73985be2d41a7c6f08e93b5d21",
		Corrections: []string{pick("In der Ausgabe vom Vortag wurde die Bauzeit der Brücke mit zwei statt drei Jahren angegeben.", "Yesterday's edition gave the footbridge construction time as two years rather than three.")},
		Stories:     stories, Sources: sources,
	}
	return e
}

func TestWriteRenderSamples(t *testing.T) {
	dir := os.Getenv("NEWSPAPER_SAMPLE_DIR")
	if dir == "" {
		t.Skip("NEWSPAPER_SAMPLE_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, lang := range []string{"de", "en"} {
		e := sampleEdition(lang)
		write("edition-"+lang+".html", []byte(HTML(e)))
		pdf, err := PDF(e)
		if err != nil {
			t.Fatal(err)
		}
		write("edition-"+lang+".pdf", pdf)
	}
	e := sampleEdition("de")
	e.Sources[1].Title = "此消息来自当地 · 東京ニュース"
	e.Stories[0].Paragraphs[0].Text += " 此。"
	pdf, err := renderPDF(e, os.DirFS("fonts"))
	if err != nil {
		t.Fatal(err)
	}
	write("edition-cjk.pdf", pdf)
	if _, err := url.Parse(e.Sources[0].URL); err != nil {
		t.Fatal(err)
	}
}

func TestWrapTextKeepsEveryRune(t *testing.T) {
	measure := func(s string) float64 { return float64(len([]rune(s))) }
	cases := []struct {
		name        string
		text        string
		first, rest float64
	}{
		{"cjk", "此消息来自当地 · 東京ニュース。 Nach der Sitzung sagte 田中さん：「是的」，然后离开了会场。Ende (ok) 완료입니다 並びに", 12, 12},
		{"kinsoku", "東京ニュース速報ぁぃっゃ》〉", 8, 8},
		{"url", "https://example.org/" + strings.Repeat("abcdefghij", 30) + "?x=1", 20, 20},
		{"german", "Der Stadtrat hat am Donnerstag nach langer Aussprache mehrere Beschlüsse gefasst.\nZweite Zeile mit Umlauten äöü.", 25, 25},
		{"indent", "Der Stadtrat hat am Donnerstag nach langer Aussprache mehrere Beschlüsse gefasst.", 17, 30},
	}
	for _, c := range cases {
		lines := wrapText(c.text, c.first, c.rest, measure)
		for i, ln := range lines {
			limit := c.rest
			if i == 0 {
				limit = c.first
			}
			if measure(ln) > limit && len([]rune(ln)) > 1 {
				t.Errorf("%s: line %d %q exceeds %v", c.name, i, ln, limit)
			}
		}
		strip := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "\n", "") }
		if strip(strings.Join(lines, "")) != strip(c.text) {
			t.Errorf("%s: runes lost or reordered: %q", c.name, lines)
		}
		for _, ln := range lines[1:] {
			if r := []rune(ln); len(r) > 0 && noBreakBefore(r[0]) && isCJK([]rune(c.text)[0]) && (c.name == "cjk" || c.name == "kinsoku") {
				t.Errorf("line starts with closing punctuation: %q", ln)
			}
		}
	}
}
