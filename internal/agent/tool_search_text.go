package agent

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// toolSearchUmlauts spell the German umlauts and the sharp s the way people
// type them without the keys, so "Enzyklopädie" and "enzyklopaedie" meet.
var toolSearchUmlauts = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")

// foldToolSearchText lower-cases text, spells umlauts as ae/oe/ue/ss and drops
// every other combining mark ("é" becomes "e").
func foldToolSearchText(text string) string {
	text = strings.ToLower(text)
	ascii := true
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return text
	}
	text = toolSearchUmlauts.Replace(norm.NFC.String(text))
	text = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.M, r) {
			return -1
		}
		return r
	}, norm.NFD.String(text))
	return norm.NFC.String(text)
}

// toolSearchWords splits folded text into words of letters and digits; every
// other character, "_" and "-" included, separates. With index set, two words
// joined by a single hyphen ("retro-spiele") also yield the joined word, so a
// query that writes the compound without the hyphen still finds it.
func toolSearchWords(folded string, index bool) []string {
	var words []string
	prev := ""
	start := -1
	gapRunes, gapHyphen, join := 1, false, false
	emit := func(end int) {
		word := folded[start:end]
		words = append(words, word)
		if join {
			words = append(words, prev+word)
		}
		prev, start = word, -1
		gapRunes, gapHyphen = 0, false
	}
	for i, r := range folded {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
				join = index && prev != "" && gapRunes == 1 && gapHyphen
			}
			continue
		}
		if start >= 0 {
			emit(i)
		}
		gapRunes++
		gapHyphen = r == '-'
	}
	if start >= 0 {
		emit(len(folded))
	}
	return words
}

// stemToolSearchWord reduces a folded word to the stem that query and index
// share: plural, verb and German noun endings go, so "Notizen" meets "notiz"
// and "scanning" meets "scan". It is a light suffix stripper, not a
// linguistic stemmer: words under four runes stay as they are, at most three
// endings come off, and a stem keeps at least three runes. There is no bare
// "-er" or "-n" rule; both cut too many unrelated words together.
func stemToolSearchWord(word string) string {
	for round := 0; round < 3 && utf8.RuneCountInString(word) >= 4; round++ {
		next, ok := stemToolSearchStep(word)
		if !ok {
			break
		}
		word = next
	}
	return word
}

// toolStemRule strips one ending. The rules are tried in order and the first
// that fits and leaves a stem of three runes wins.
type toolStemRule struct {
	suffix, replacement string
	undouble            bool     // drop a doubled final consonant ("runn" -> "run")
	exceptTails         []string // words ending like this keep the ending
}

var toolStemRules = []toolStemRule{
	{suffix: "ungen"},
	{suffix: "ung"},
	{suffix: "ies", replacement: "y"},
	{suffix: "ing", undouble: true},
	{suffix: "ed", undouble: true},
	{suffix: "en"},
	{suffix: "es"},
	{suffix: "ern", replacement: "er"},
	{suffix: "eln", replacement: "el"},
	{suffix: "e"},
	// "-s" stays on "class", "status" and "analysis".
	{suffix: "s", exceptTails: []string{"ss", "us", "is"}},
}

// stemToolSearchStep applies the first ending rule that fits.
func stemToolSearchStep(word string) (string, bool) {
	for _, rule := range toolStemRules {
		if !strings.HasSuffix(word, rule.suffix) {
			continue
		}
		if slices.ContainsFunc(rule.exceptTails, func(tail string) bool { return strings.HasSuffix(word, tail) }) {
			continue
		}
		stem := word[:len(word)-len(rule.suffix)]
		if rule.undouble {
			stem = undoubleToolSearchStem(stem)
		}
		stem += rule.replacement
		if utf8.RuneCountInString(stem) >= 3 {
			return stem, true
		}
	}
	return word, false
}

// undoubleToolSearchStem turns "runn" into "run" and "stopp" into "stop",
// except for l, s and z ("pull", "pass", "buzz" keep both).
func undoubleToolSearchStem(stem string) string {
	last, size := utf8.DecodeLastRuneInString(stem)
	if prev, _ := utf8.DecodeLastRuneInString(stem[:len(stem)-size]); prev != last || !unicode.IsLetter(last) || strings.ContainsRune("aeioulsz", last) {
		return stem
	}
	return stem[:len(stem)-size]
}

// toolSearchStopwords are the words no query is about: articles, pronouns,
// auxiliaries, particles, polite fillers and generic verbs, in folded ASCII
// spelling. Domain verbs and nouns (send, read, search, list, server, file,
// datei, ...) are deliberately absent.
var toolSearchStopwords = func() map[string]struct{} {
	set := make(map[string]struct{}, 320)
	for _, word := range strings.Fields(`
		a an the and or but nor if then else of to in on at by for with from into onto over under about as
		is are was were be been being am do does did doing have has had having can could will would shall
		should may might must i me my mine we us our ours you your yours he him his she her hers it its
		they them their theirs this that these those there here what which who whom whose when where why
		how not no yes so than too very just also only some any all each every both few more most other
		such own same up down out off again once via per etc please kindly thanks thank hello hi hey ok
		okay want wants wanted need needs needed like likes let lets get gets got make makes made use
		uses used using give gives take takes try tries`) {
		set[word] = struct{}{}
	}
	for _, word := range strings.Fields(`
		der die das den dem des ein eine einen einem einer eines und oder aber sondern denn weil dass
		ob als wie wo wer was wen wem wessen wann warum wieso weshalb wozu wohin woher ich du er sie es
		wir ihr mich mir dich dir ihn ihm uns euch ihnen mein meine meinen meinem meiner meines dein
		deine deinen deinem deiner unser unsere unseren sein seine seinen seinem seiner ihre ihren
		ihrem ihrer ist sind bin bist war waren wird werden wurde wurden wirst hat haben habe hast
		hatte hatten kann kannst koennen koennte koenntest muss musst muessen soll sollst sollen
		sollte will willst wollen wollte moechte moechtest moechten darf darfst duerfen mit von vom zu
		zum zur bei beim nach aus auf an am im ins um fuer ueber unter durch gegen ohne bis seit vor
		hinter neben zwischen nicht kein keine keinen keinem nichts auch noch schon nur sehr mal doch
		ja nein bitte danke hallo hier dort da dies diese diesen diesem dieser dieses jede jeden jedem
		jeder alle allen alles mehr ganz nun dann also eben halt eigentlich nutze nutzen verwende
		verwenden mach mache machen gib gebe geben brauche brauchst brauchen braucht haette wuerde
		wuerdest`) {
		set[word] = struct{}{}
	}
	return set
}()

// toolSearchTermsOf turns free text (a query, a description, a curated
// phrase) into the stems that are compared: folded, split into words, without
// stopwords and one-letter words, stemmed. With index set the hyphen-joined
// words are kept (see toolSearchWords).
func toolSearchTermsOf(text string, index bool) []string {
	words := toolSearchWords(foldToolSearchText(text), index)
	stems := words[:0]
	for _, word := range words {
		if _, stop := toolSearchStopwords[word]; stop || utf8.RuneCountInString(word) < 2 {
			continue
		}
		stems = append(stems, stemToolSearchWord(word))
	}
	return stems
}
