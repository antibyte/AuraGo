package scraper

import (
	"strings"
	"testing"
)

func TestCleanHiddenHTML(t *testing.T) {
	for _, attr := range []string{`hidden`, `aria-hidden="true"`, `style="display:none"`, `style="visibility:collapse"`, `style="opacity: 0"`, `style="font-size:0px"`, `style="color:transparent"`, `style="position:absolute;left:-9999px"`, `style="text-indent:-999px"`, `style="clip:rect(0, 0, 0, 0)"`, `style="clip-path:inset(50%)"`, `style="transform:scale(0)"`, `style="height:0;overflow:hidden"`, `class="sr-only"`} {
		raw := `<article><p>Visible useful article content.</p><span ` + attr + `>UNIQUE_HIDDEN_SENTENCE with a long instruction for an automated reader.</span></article>`
		clean := CleanHiddenHTML(raw)
		if strings.Contains(clean, "UNIQUE_HIDDEN_SENTENCE") || !strings.Contains(clean, "Visible useful") {
			t.Errorf("%s: %s", attr, clean)
		}
	}
	for _, raw := range []string{`<span class="sr-only">Label</span>`, `<div hidden role="region">Accordion information</div>`, `<div style="display:none" role="tabpanel">Tab information</div>`, `<div hidden aria-labelledby="heading">Labelled information</div>`, `<noscript>Fallback information</noscript>`} {
		if !strings.Contains(CleanHiddenHTML(raw), "information") && !strings.Contains(CleanHiddenHTML(raw), "Label") {
			t.Error(raw)
		}
	}
	for _, attr := range []string{`style="opacity:0.0"`, `style="color:rgba(1,2,3,0)"`, `style="color:rgb(1 2 3 / 0)"`, `role="tabpanel" style="opacity:0"`, `style="width:0;overflow:clip"`, `style="position:absolute;left:99999px"`, `style="clip:rect(0 0 0 0)"`} {
		if strings.Contains(CleanHiddenHTML(`<p `+attr+`>hidden prose</p>`), "hidden prose") {
			t.Error(attr)
		}
	}
	if !strings.Contains(CleanHiddenHTML(`<span class="sr-only" style="clip:rect(0,0,0,0)">Label</span>`), "Label") {
		t.Fatal("short accessible label removed")
	}
}
