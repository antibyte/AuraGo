package localwiki

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSnippetPrefersFirstSentenceWithQueryWord(t *testing.T) {
	raw := []byte(htmlPage("Ulm", `<table class="infobox"><tr><th>Einwohner</th><td>130.288 Hauptstadt</td></tr></table>`+
		`<p><b>Ulm</b> ist eine Großstadt in Baden-Württemberg.<sup class="reference">[1]</sup> Das Ulmer Münster hat den höchsten Kirchturm.</p>`+
		`<p>Ulm liegt an der Donau.</p>`))
	if got := snippetFromHTML(raw, matchWords("münster")); got != "Das Ulmer Münster hat den höchsten Kirchturm." {
		t.Fatalf("snippet = %q", got)
	}
	if got := snippetFromHTML(raw, matchWords("Donau")); got != "Ulm liegt an der Donau." {
		t.Fatalf("snippet = %q", got)
	}
}

func TestSnippetFallsBackToLeadStart(t *testing.T) {
	raw := []byte(htmlPage("Ulm", `<p>`+strings.Repeat("Ulm ist alt. ", 30)+`</p>`))
	got := snippetFromHTML(raw, matchWords("Raumfahrt"))
	if !strings.HasPrefix(got, "Ulm ist alt. Ulm ist alt.") || !strings.HasSuffix(got, "…") || utf8.RuneCountInString(got) > snippetMaxRunes {
		t.Fatalf("snippet = %q", got)
	}
}

func TestSnippetIgnoresNavigationReferencesAndTables(t *testing.T) {
	raw := []byte(htmlPage("X", `<div class="navbox"><p>Navigation Hauptstadt</p></div>`+
		`<table><tr><td><p>Tabelle Hauptstadt</p></td></tr></table>`+
		`<div class="hatnote"><p>Hinweis Hauptstadt</p></div>`+
		`<p>Text ohne Treffer.<br/>Zweite Zeile.</p>`))
	if got := snippetFromHTML(raw, matchWords("Hauptstadt")); got != "Text ohne Treffer. Zweite Zeile." {
		t.Fatalf("snippet = %q", got)
	}
}

func TestArticleParagraphsSurvivesVoidElementsWithSkipClasses(t *testing.T) {
	raw := []byte(htmlPage("X", `<p>Eins <img class="noprint" src="a.png"> zwei.</p><p>Drei.</p>`))
	if got := articleParagraphs(raw, 10); !reflect.DeepEqual(got, []string{"Eins zwei.", "Drei."}) {
		t.Fatalf("paragraphs = %q", got)
	}
}

func TestSplitSentences(t *testing.T) {
	got := splitSentences("Am 3. Oktober ca. 12 Uhr begann es. Wirklich? Ja! 東京は首都です。次の文")
	want := []string{"Am 3. Oktober ca. 12 Uhr begann es.", "Wirklich?", "Ja!", "東京は首都です。", "次の文"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sentences = %q", got)
	}
}

func TestSnippetSplitsHindiSentencesAtDanda(t *testing.T) {
	raw := []byte(htmlPage("भारत", `<p>दिल्ली भारत की राजधानी है। मुंबई महाराष्ट्र की राजधानी है॥ चेन्नई तमिलनाडु में है।</p>`))
	if got := snippetFromHTML(raw, matchWords("मुंबई")); got != "मुंबई महाराष्ट्र की राजधानी है॥" {
		t.Fatalf("snippet = %q", got)
	}
	if got := snippetFromHTML(raw, matchWords("चेन्नई")); got != "चेन्नई तमिलनाडु में है।" {
		t.Fatalf("snippet = %q", got)
	}
	// The danda ends a sentence without following white space, like 。！？.
	got := splitSentences("पहला वाक्य।दूसरा वाक्य॥तीसरा वाक्य")
	if want := []string{"पहला वाक्य।", "दूसरा वाक्य॥", "तीसरा वाक्य"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sentences = %q", got)
	}
}

func TestArticleParagraphsFlushesTheOpenParagraphAtTheEndOfInput(t *testing.T) {
	if got := articleParagraphs([]byte(`<p>Eins</p><p>Zwei ohne Ende`), 10); !reflect.DeepEqual(got, []string{"Eins", "Zwei ohne Ende"}) {
		t.Fatalf("paragraphs = %q", got)
	}
	// A paragraph that runs over the end of the scan window is kept up to the
	// window, cut on a rune boundary.
	raw := []byte("<p>" + strings.Repeat("ä", snippetScanBytes) + "</p>")
	got := articleParagraphs(raw, 3)
	if len(got) != 1 || !utf8.ValidString(got[0]) || strings.Trim(got[0], "ä") != "" || got[0] == "" {
		t.Fatalf("paragraphs = %d, valid=%v", len(got), len(got) == 1 && utf8.ValidString(got[0]))
	}
	if n := utf8.RuneCountInString(got[0]); n < snippetScanBytes/2-4 || n > snippetScanBytes/2 {
		t.Fatalf("kept %d runes of the window", n)
	}
}

func TestArticleParagraphsHandlesImplicitParagraphEnds(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"unclosed paragraphs":             {`<p>eins<p>zwei<p>drei`, []string{"eins", "zwei", "drei"}},
		"hatnote closed by next p":        {`<p class="hatnote">Hinweis Hauptstadt<p>Echter Text.</p><p>Mehr.</p>`, []string{"Echter Text.", "Mehr."}},
		"hatnote closed by parent":        {`<div><p class="hatnote">Hinweis</div><p>Echter Text.</p>`, []string{"Echter Text."}},
		"hatnote closed by block":         {`<p class="hatnote">Hinweis <b>fett</b><h2>Titel</h2><p>Echter Text.</p>`, []string{"Echter Text."}},
		"paragraph closed by block start": {`<p>Vorher<div>Nachher</div><p>Text.</p>`, []string{"Vorher", "Text."}},
	} {
		if got := articleParagraphs([]byte(htmlPage("X", tc.body)), 10); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: paragraphs = %q, want %q", name, got, tc.want)
		}
	}
}

func TestArticleParagraphsVoidTagsWithSkipClassesDoNotSwallowThePage(t *testing.T) {
	for _, void := range []string{
		`<param class="noprint" name="a" value="b">`,
		`<keygen class="noprint" name="k">`,
		`<frame class="noprint" src="a.html">`,
		`<img class="noprint" src="a.png">`,
	} {
		raw := []byte(htmlPage("X", `<div>`+void+`</div><p>Text bleibt.</p>`))
		if got := articleParagraphs(raw, 10); !reflect.DeepEqual(got, []string{"Text bleibt."}) {
			t.Fatalf("%s: paragraphs = %q", void, got)
		}
	}
}

func TestArticleParagraphsMatchesWholeClassTokens(t *testing.T) {
	raw := []byte(htmlPage("X", `<div class="my-reference-list noprint-extra"><p>Bleibt.</p></div>`+
		`<div class="seite noprint"><p>Weg.</p></div>`+
		`<div class="x hatnote y"><p>Auch weg.</p></div>`+
		`<div class="references"><p>Weg wie vorher.</p></div>`+
		`<p>Ende.</p>`))
	if got := articleParagraphs(raw, 10); !reflect.DeepEqual(got, []string{"Bleibt.", "Ende."}) {
		t.Fatalf("paragraphs = %q", got)
	}
}

func TestArticleParagraphsIgnoresSVGText(t *testing.T) {
	raw := []byte(htmlPage("X", `<p>Vorher <svg width="10"><title>Diagramm Hauptstadt</title><text>Beschriftung</text><path d="M0 0"/></svg>nachher.</p>`))
	if got := articleParagraphs(raw, 10); !reflect.DeepEqual(got, []string{"Vorher nachher."}) {
		t.Fatalf("paragraphs = %q", got)
	}
	if got := snippetFromHTML(raw, matchWords("Hauptstadt")); got != "Vorher nachher." {
		t.Fatalf("snippet = %q", got)
	}
}
