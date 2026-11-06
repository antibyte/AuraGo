package storymaker

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// CodeWordsTell marks a stock phrase of machine copy, or a dash that reads as machine text.
const CodeWordsTell = "words_tell"

// wordsTellEN and wordsTellDE are phrases a film does not need. A match ignores case and needs a
// word boundary on both sides.
var wordsTellEN = []string{
	"in a world where", "little did", "more than just", "every moment counts",
	"against all odds", "it's not about", "nothing would ever be the same", "tapestry",
	"vibrant", "breathtaking", "game changer", "game-changer", "seamless", "delve",
}

var wordsTellDE = []string{
	"in einer welt, in der", "mehr als nur", "jeder moment zählt",
	"allen widrigkeiten zum trotz", "es geht nicht um", "nichts würde mehr so sein", "atemberaubend",
	"bahnbrechend", "nahtlos", "auf eine reise", "auf einer reise", "tauche ein",
}

var (
	wordsTellENRe = wordsTellPattern(wordsTellEN)
	wordsTellDERe = wordsTellPattern(wordsTellDE)
)

// wordsTellPattern matches any phrase of the list, case-insensitively, on word boundaries.
func wordsTellPattern(phrases []string) *regexp.Regexp {
	quoted := make([]string, len(phrases))
	for i, phrase := range phrases {
		quoted[i] = regexp.QuoteMeta(phrase)
	}
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(quoted, "|") + `)\b`)
}

// LintWords finds stock phrases and dashes in the film text: the narration of each scene and the
// text of each show and say beat. Cue markers are removed before matching. Every language gets the
// dash check. Only English and German also get their phrase list.
func LintWords(st *Story, language string) []Issue {
	if st == nil {
		return nil
	}
	var phrases *regexp.Regexp
	switch LanguageBase(language) {
	case "en":
		phrases = wordsTellENRe
	case "de":
		phrases = wordsTellDERe
	}
	var out []Issue
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		out = lintWordsText(out, sc.ID, jp("scenes", i)+"/narration", sc.Narration, phrases)
		for j := range sc.Beats {
			if b := &sc.Beats[j]; b.Do == "show" || b.Do == "say" {
				out = lintWordsText(out, sc.ID, fmt.Sprintf("%s/beats/%d/text", jp("scenes", i), j), b.Text, phrases)
			}
		}
	}
	return out
}

// lintWordsText appends one issue per phrase hit, and one issue when the text has a dash.
func lintWordsText(out []Issue, sceneID, path, text string, phrases *regexp.Regexp) []Issue {
	text = StripMarkers(text)
	if phrases != nil {
		for _, hit := range phrases.FindAllString(text, -1) {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sceneID, Path: path, Code: CodeWordsTell,
				Message: fmt.Sprintf("Scene %s uses the phrase %q, which reads as stock copy.", sceneID, hit),
				Fix:     "Say it concretely, or cut the line.",
			})
		}
	}
	if hasWordsDash(text) {
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: sceneID, Path: path, Code: CodeWordsTell,
			Message: fmt.Sprintf("Scene %s uses a dash. Machine text uses dashes where a comma, a full stop or a colon reads better.", sceneID),
			Fix:     "Replace the dash with a comma, a full stop or a colon.",
		})
	}
	return out
}

// hasWordsDash says whether text has an em dash, a double hyphen, or an en dash that does not join
// two numbers.
func hasWordsDash(text string) bool {
	if strings.ContainsRune(text, '—') || strings.Contains(text, "--") {
		return true
	}
	runes := []rune(text)
	for i, r := range runes {
		if r != '–' {
			continue
		}
		if i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
			continue
		}
		return true
	}
	return false
}
