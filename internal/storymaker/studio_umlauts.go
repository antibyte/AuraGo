package storymaker

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
)

//go:embed data/de_umlauts.json
var deUmlautsJSON []byte

// UmlautFix is one repaired word: how it was written, what it became, and how often.
type UmlautFix struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

// umlautNote is the sentence that travels with a repair in a tool answer.
const umlautNote = "Missing umlauts were restored in the text you sent. Write ä, ö, ü and ß yourself next time."

var (
	umlautOnce sync.Once
	umlautMap  map[string]string
)

// germanUmlautMap is the curated map of ASCII spellings that lost an umlaut to the word with it. The file is embedded and a
// test checks every entry, so a decode error means a broken build and leaves the map empty (no repair).
func germanUmlautMap() map[string]string {
	umlautOnce.Do(func() {
		umlautMap = map[string]string{}
		_ = json.Unmarshal(deUmlautsJSON, &umlautMap)
	})
	return umlautMap
}

// RepairUmlauts restores the umlauts that a weak model left out of German text. It replaces whole words only, keeps the case
// (pruft, Pruft, PRUFT) and leaves every other character, cue markers included, as it is. Only language de (any regional
// tag with base de) is touched, and only words of the curated map, whose ASCII spelling is not a German word. It returns the
// text and the distinct repairs; with nothing to repair it returns the text as given and nil.
func RepairUmlauts(text, language string) (string, []UmlautFix) {
	if text == "" || LanguageBase(language) != "de" {
		return text, nil
	}
	words := germanUmlautMap()
	if len(words) == 0 {
		return text, nil
	}
	runes := []rune(text)
	var out strings.Builder
	var fixes []UmlautFix
	for i := 0; i < len(runes); {
		if !unicode.IsLetter(runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) && unicode.IsLetter(runes[j]) {
			j++
		}
		word := string(runes[i:j])
		if fixed, ok := repairUmlautWord(words, word); ok {
			fixes = addUmlautFix(fixes, UmlautFix{From: word, To: fixed, Count: 1})
			word = fixed
		}
		out.WriteString(word)
		i = j
	}
	if len(fixes) == 0 {
		return text, nil
	}
	return out.String(), fixes
}

// repairUmlautWord looks one word up and gives the repaired word in the case of the original.
func repairUmlautWord(words map[string]string, word string) (string, bool) {
	to, ok := words[strings.ToLower(word)]
	if !ok {
		return "", false
	}
	letters := []rune(word)
	switch {
	case len(letters) > 1 && word == strings.ToUpper(word):
		return strings.ToUpper(to), true
	case unicode.IsUpper(letters[0]):
		r := []rune(to)
		r[0] = unicode.ToUpper(r[0])
		return string(r), true
	}
	return to, true
}

// addUmlautFix adds a repair to a list, merging the counts of the same pair.
func addUmlautFix(list []UmlautFix, fix UmlautFix) []UmlautFix {
	for i := range list {
		if list[i].From == fix.From && list[i].To == fix.To {
			list[i].Count += fix.Count
			return list
		}
	}
	return append(list, fix)
}
