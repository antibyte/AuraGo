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

// easyDragBannedDe and easyDragBannedEn are the words the concept ledger replaced (voice guide rule 4).
// Whole words (a letter, digit or underscore next to the word means it is part of a longer word),
// placeholders removed first. "Baustein" and "block" are ledger words and not banned.
var (
	easyDragBannedDe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])(Workflows?|Knoten|Nodes?|Blocks?|Blöcke[n]?|Revisionen?|Pakete?n?)(?:[^\p{L}\p{N}_]|$)`)
	easyDragBannedEn = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])(workflows?|nodes?|building blocks?|revisions?|packages?)(?:[^\p{L}\p{N}_]|$)`)
	// easyDragBannedDeCompound catches the German compounds of "Block" that mean a building block.
	easyDragBannedDeCompound = regexp.MustCompile(`\bBlock(?:version|kopie|inhalt|eingabe|ausgabe)\w*`)
)

// easyDragStepKeys are the texts that talk about a step or a step type. They must not call it a
// building block: "Baustein" and "block" belong to the reusable sub-flow (decision 5 of W1b).
var easyDragStepKeys = []string{
	"easydrag.ui.palette_title", "easydrag.ui.palette_search", "easydrag.ui.palette_hide", "easydrag.ui.palette_no_results",
	"easydrag.ui.keys_search", "easydrag.ui.badge_unknown", "easydrag.ui.error_node_type_unknown", "easydrag.ui.error_flow_node_unavailable",
	"easydrag.ui.issue_node_type_unknown", "easydrag.ui.issue_node_unavailable", "easydrag.ui.home_empty_text", "easydrag.ui.panel_resize_palette",
}

func TestEasyDragLocalesUseLedgerWords(t *testing.T) {
	// The lint itself.
	for _, c := range []struct {
		de   bool
		text string
		want bool
	}{
		{true, "Lokale Blöcke ({count})", true},
		{true, "Node-Editor", true},
		{true, "Die Blockversionen", true},
		{true, "Lokale Bausteine ({count})", false},
		{true, "Blockiert", false},
		{true, "Version v{revision} ({nodes} Knoten)", true},
		{true, "Version v{revision} mit {nodes} Schritten", false},
		{false, "This building block is not available.", true},
		{false, "Search building blocks", true},
		{false, "The node catalog", true},
		{false, "Local blocks ({count})", false},
		{false, "Version v{revision} and {nodes} steps", false},
		{false, "The agent permissions block this action.", false},
	} {
		text := easyDragVoicePlaceholder.ReplaceAllString(c.text, "")
		hit := easyDragBannedEn.MatchString(text)
		if c.de {
			hit = easyDragBannedDe.MatchString(text) || easyDragBannedDeCompound.MatchString(text)
		}
		if hit != c.want {
			t.Errorf("lint self-check %q (de %v): banned %v, want %v", c.text, c.de, hit, c.want)
		}
	}
	var bad []string
	for lang, banned := range map[string]*regexp.Regexp{"de": easyDragBannedDe, "en": easyDragBannedEn} {
		for key, text := range readEasyDragLang(t, lang) {
			text = easyDragVoicePlaceholder.ReplaceAllString(text, "")
			if m := banned.FindStringSubmatch(text); m != nil {
				bad = append(bad, lang+" "+key+": "+m[1])
			} else if m := easyDragBannedDeCompound.FindString(text); lang == "de" && m != "" {
				bad = append(bad, lang+" "+key+": "+m)
			}
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d text(s) use a word the concept ledger replaced (voice guide, rule 4):\n%s", len(bad), strings.Join(bad, "\n"))
	}
	// A step text must not call the step a building block.
	de, en := readEasyDragLang(t, "de"), readEasyDragLang(t, "en")
	for _, key := range easyDragStepKeys {
		if strings.Contains(strings.ToLower(de[key]), "baustein") {
			t.Errorf("de %s calls a step a building block: %q", key, de[key])
		}
		if strings.Contains(strings.ToLower(en[key]), "block") {
			t.Errorf("en %s calls a step a block: %q", key, en[key])
		}
	}
}
