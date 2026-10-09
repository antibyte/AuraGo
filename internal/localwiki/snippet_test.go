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
