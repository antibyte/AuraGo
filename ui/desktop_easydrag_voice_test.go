package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The voice lints of EasyDrag (W1b part B, documentation/easydrag/voice.md). They read the shipped
// locale files; a text that breaks a rule fails the test with the key and the offending word.

// easyDragVoicePlaceholder matches {name} and {{name}}. The lints remove placeholders before they read a text.
var easyDragVoicePlaceholder = regexp.MustCompile(`\{+[^{}]*\}+`)

// easyDragCapitalDu matches the capital forms of German "du" as whole words: Du, Dich, Dir and Dein
// with an ending. Whole words, so "Deinstallieren" does not match.
var easyDragCapitalDu = regexp.MustCompile(`\b(?:Du|Dich|Dir|Dein(?:e|em|en|er|es)?)\b`)

// easyDragAtSentenceStart reports whether before ends where a sentence begins: nothing before, or a
// sentence end, a colon or a line break.
func easyDragAtSentenceStart(before string) bool {
	if before == "" {
		return true
	}
	for _, mark := range []string{". ", "! ", "? ", ": ", "\n"} {
		if strings.HasSuffix(before, mark) {
			return true
		}
	}
	return false
}

// easyDragCapitalDuForms returns the capital "du" forms inside a German text (voice guide rule 3).
func easyDragCapitalDuForms(text string) []string {
	text = easyDragVoicePlaceholder.ReplaceAllString(text, "")
	var found []string
	for _, loc := range easyDragCapitalDu.FindAllStringIndex(text, -1) {
		if !easyDragAtSentenceStart(text[:loc[0]]) {
			found = append(found, text[loc[0]:loc[1]])
		}
	}
	return found
}

func TestEasyDragGermanUsesLowercaseDu(t *testing.T) {
	// The lint itself: a capital form inside a sentence is found, one at a sentence start is not.
	for text, want := range map[string]int{
		"Prüfe das, bevor Du startest.":     1,
		"Du startest.":                      0,
		"Fertig. Du kannst weiter.":         0,
		"Wähle: Dein Entwurf bleibt.":       0,
		"Zeile eins\nDir gehört das.":       0,
		"Deinstallieren ist nicht möglich.": 0,
		"{name}: bei Dir und Deinen Daten":  2,
		"bevor du deine Daten prüfst":       0,
		"Es hängt von Dir ab":               1,
	} {
		if got := len(easyDragCapitalDuForms(text)); got != want {
			t.Errorf("lint self-check %q: %d capital forms, want %d", text, got, want)
		}
	}
	var bad []string
	for key, text := range readEasyDragLang(t, "de") {
		if forms := easyDragCapitalDuForms(text); len(forms) > 0 {
			bad = append(bad, key+": "+strings.Join(forms, ", "))
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("%d German text(s) write du with a capital inside a sentence (voice guide, rule 3):\n%s", len(bad), strings.Join(bad, "\n"))
	}
}
