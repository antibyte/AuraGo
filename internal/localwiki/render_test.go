package localwiki

import (
	"fmt"
	"strings"
	"testing"
)

func renderTestBody(t *testing.T, body string) *renderedArticle {
	t.Helper()
	art, err := renderArticle([]byte(htmlPage("Test", body)), "Test")
	if err != nil {
		t.Fatal(err)
	}
	return art
}

func TestRenderInfoboxAsKeyValueList(t *testing.T) {
	art := renderTestBody(t, `<table class="infobox"><caption class="infobox-title">Berlin</caption>`+
		`<tr><td colspan="2" class="infobox-image"><img src="a.webp" alt="Flagge"><div class="infobox-caption">Flagge Berlins</div></td></tr>`+
		`<tr><th colspan="2" class="infobox-header">Basisdaten</th></tr>`+
		`<tr><th class="infobox-label">Einwohner</th><td class="infobox-data">3.878.100<sup class="reference">[1]</sup></td></tr>`+
		`<tr><td>Bundesland:</td><td><a href="Berlin">Berlin</a></td></tr>`+
		`<tr><td colspan="2"><table><tr><td>Diagramm</td></tr></table></td></tr>`+
		`<tr><th class="infobox-label">Bezirke</th><td><ul><li>Mitte</li><li>Pankow</li></ul></td></tr>`+
		`</table><p>Text.</p>`)
	want := "**Berlin**\n\n- [Image: Flagge Berlins]\n\n**Basisdaten**\n\n- **Einwohner:** 3.878.100\n- **Bundesland:** Berlin\n- **Bezirke:** Mitte; Pankow\n\nText."
	if got := art.sections[0].body; got != want {
		t.Fatalf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderTruncatesLongTables(t *testing.T) {
	var rows strings.Builder
	rows.WriteString("<tr><th>Jahr</th><th>Wert</th></tr>")
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&rows, "<tr><td>%d</td><td>%d</td></tr>", 1900+i, i)
	}
	art := renderTestBody(t, `<table class="wikitable">`+rows.String()+`</table>`)
	body := art.sections[0].body
	if !strings.HasPrefix(body, "| Jahr | Wert |\n|---|---|\n| 1901 | 1 |") {
		t.Fatalf("table = %q", body[:60])
	}
	if strings.Contains(body, "| 1950 |") || !strings.Contains(body, "| 1949 | 49 |") {
		t.Fatalf("expected the header plus 49 data rows: %s", body)
	}
	if !strings.HasSuffix(body, "[Table truncated: showing 50 of 61 rows]") {
		t.Fatalf("missing truncation note: %q", body[len(body)-60:])
	}
}

func TestRenderRemovesChromeReferencesAndNavigation(t *testing.T) {
	art := renderTestBody(t, `<div class="hatnote">Siehe auch Berlin (Begriffsklärung)</div>`+
		`<table class="ambox"><tr><td>Belege fehlen</td></tr></table>`+
		`<div class="shortdescription">Hauptstadt</div>`+
		`<p>Erster<sup class="mw-ref reference"><a href="#cite_note-1"><span class="mw-reflink-text">[1]</span></a></sup> Satz mit <a href="Spree">Spree</a> und <a class="external" href="https://example.org">Web</a>.</p>`+
		`<span id="coordinates">52° N</span>`+
		`<div style="display:none">versteckt</div>`+
		`<div role="navigation" class="navbox"><p>Navi</p></div>`+
		`<div class="klappleiste navigation-not-searchable">Klappleiste</div>`+
		`<h2 class="section-heading"><span class="mw-headline">Geschichte</span><span class="mw-editsection">[Bearbeiten]</span></h2>`+
		`<p>Alt.</p>`+
		`<h2>Einzelnachweise</h2><div class="mw-references-wrap"><ol class="references"><li>Quelle</li></ol></div>`+
		`<h2>Weblinks</h2><ul><li><a class="external" href="https://berlin.de">berlin.de</a></li></ul>`+
		`<!--htdig_noindex--><div><div>Dieser Artikel stammt aus Wikipedia.</div></div><!--/htdig_noindex-->`)
	got := art.fullMarkdown()
	want := "# Test\n\nErster Satz mit Spree und Web.\n\n## Geschichte\n\nAlt."
	if got != want {
		t.Fatalf("markdown =\n%s\nwant\n%s", got, want)
	}
	if len(art.sections) != 2 {
		t.Fatalf("sections = %+v", art.sections)
	}
}

func TestRenderImagesAsCaptionPlaceholders(t *testing.T) {
	art := renderTestBody(t, `<figure typeof="mw:File/Thumb"><a href="F"><img src="a.webp" alt="alt"></a><figcaption>Das <a href="M">Münster</a> im Winter</figcaption></figure>`+
		`<figure><figcaption>Bild ohne Datei (nopic)</figcaption></figure>`+
		`<div class="thumb"><div class="thumbinner"><img src="b.webp" alt="Altes Rathaus"><div class="thumbcaption"></div></div></div>`+
		`<ul class="gallery"><li class="gallerybox"><img src="c.webp"><div class="gallerytext">Donau</div></li></ul>`+
		`<p>Text mit <img class="flagicon" src="f.png" alt="Flagge"> Symbol.</p>`)
	want := "[Image: Das Münster im Winter]\n\n[Image: Altes Rathaus]\n\n- [Image: Donau]\n\nText mit Symbol."
	if got := art.sections[0].body; got != want {
		t.Fatalf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderMathAsTeX(t *testing.T) {
	art := renderTestBody(t, `<p>Formel <span class="mwe-math-element"><span class="mwe-math-mathml-inline" style="display: none;"><math><semantics><mi>E</mi>`+
		`<annotation encoding="application/x-tex">{\displaystyle E=mc^{2}}</annotation></semantics></math></span><img class="mwe-math-fallback-image-inline" alt="{\displaystyle E=mc^{2}}"></span> gilt.</p>`)
	if got := art.sections[0].body; got != "Formel `E=mc^{2}` gilt." {
		t.Fatalf("body = %q", got)
	}
}

func TestRenderSplitsNestedSectionWrappers(t *testing.T) {
	art := renderTestBody(t, `<section data-mw-section-id="0"><p>Lead.</p></section>`+
		`<section data-mw-section-id="1"><div class="mw-heading mw-heading2"><h2 id="A">A</h2></div><p>Eins.</p>`+
		`<section data-mw-section-id="2"><h3 id="B">B</h3><p>Zwei.</p></section></section>`+
		`<details><summary><h2>C</h2></summary><p>Drei.</p></details>`)
	got := art.sectionList()
	want := []Section{{0, "", 0, 5}, {1, "A", 2, 25}, {2, "B", 3, 12}, {3, "C", 2, 11}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("sections = %+v, want %+v", got, want)
	}
	if art.sectionMarkdown(1) != "## A\n\nEins.\n\n### B\n\nZwei." {
		t.Fatalf("section A = %q", art.sectionMarkdown(1))
	}
}

func TestRenderKeepsEmptyParentWithContentfulSubsection(t *testing.T) {
	art := renderTestBody(t, `<p>Lead.</p><h2>Politik</h2><h3>Rat</h3><p>Sitze.</p><h2>Anmerkungen</h2><div class="reflist">x</div>`)
	if fmt.Sprint(art.sectionList()) != fmt.Sprint([]Section{{0, "", 0, 5}, {1, "Politik", 2, 27}, {2, "Rat", 3, 15}}) {
		t.Fatalf("sections = %+v", art.sectionList())
	}
}

func TestLeadFromHTMLKeepsOnlyLeadProse(t *testing.T) {
	raw := []byte(htmlPage("Ulm", `<table class="infobox"><tr><th>Einwohner</th><td>130.288</td></tr></table>`+
		`<figure><img src="a.webp"><figcaption>Münster</figcaption></figure>`+
		`<p><b>Ulm</b> ist eine Großstadt.</p><ul><li>Donau</li></ul>`+
		`<h2>Geschichte</h2><p>Alt.</p>`))
	lead, err := leadFromHTML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if lead != "**Ulm** ist eine Großstadt.\n\n- Donau" {
		t.Fatalf("lead = %q", lead)
	}
	long := []byte(htmlPage("X", "<p>"+strings.Repeat("Wort ", 600)+"</p>"))
	if lead, _ := leadFromHTML(long); len([]rune(lead)) > leadMaxRunes || !strings.HasSuffix(lead, "…") {
		t.Fatalf("long lead has %d runes", len([]rune(lead)))
	}
}
