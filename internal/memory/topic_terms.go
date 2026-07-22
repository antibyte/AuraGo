package memory

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxTopicTerms = 8

var topicStopTerms = map[string]struct{}{
	"aber": {}, "again": {}, "aktuell": {}, "aktuelle": {}, "aktuellen": {}, "aktueller": {},
	"anything": {}, "auch": {}, "aufgabe": {}, "bitte": {}, "check": {}, "could": {}, "current": {},
	"dann": {}, "error": {}, "erneut": {}, "fehler": {}, "follow": {}, "gibt": {}, "gibts": {}, "guten": {}, "hallo": {},
	"hello": {}, "heute": {}, "hi": {}, "latest": {}, "machen": {}, "mach": {}, "morgen": {},
	"neues": {}, "noch": {}, "nochmal": {}, "nochmals": {}, "okay": {}, "open": {}, "pending": {},
	"please": {}, "problem": {}, "probleme": {}, "prüfe": {}, "remind": {}, "retry": {}, "servus": {},
	"status": {}, "there": {}, "this": {}, "time": {}, "todo": {}, "try": {}, "versuch": {}, "versuche": {},
	"was": {}, "what": {}, "wieder": {}, "wiederhole": {}, "with": {}, "would": {}, "zeit": {},
	"gestern": {}, "jetzt": {}, "kannst": {}, "könntest": {}, "moin": {}, "the": {}, "yo": {},
}

var shortTopicTerms = map[string]struct{}{
	"api": {}, "gpu": {}, "nas": {}, "pkw": {}, "ssl": {},
}

// TopicTerms returns at most eight unique, Unicode-normalized subject terms in
// encounter order. Generic retry, status, time, error, greeting, and filler
// words are intentionally excluded so they cannot reactivate unrelated memory.
func TopicTerms(text string) []string {
	normalized := norm.NFKC.String(strings.ToLower(strings.TrimSpace(text)))
	seen := make(map[string]struct{}, maxTopicTerms)
	terms := make([]string, 0, maxTopicTerms)
	for _, term := range strings.FieldsFunc(normalized, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if _, blocked := topicStopTerms[term]; blocked {
			continue
		}
		if len([]rune(term)) < 4 {
			if _, allowed := shortTopicTerms[term]; !allowed {
				continue
			}
		}
		if _, duplicate := seen[term]; duplicate {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) == maxTopicTerms {
			break
		}
	}
	return terms
}

// TopicTermSet exposes the shared classifier as a set for exact overlap checks.
func TopicTermSet(text string) map[string]struct{} {
	terms := TopicTerms(text)
	result := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		result[term] = struct{}{}
	}
	return result
}

// HasTopicOverlap reports whether two texts share an exact classified term.
func HasTopicOverlap(left, right string) bool {
	leftTerms := TopicTermSet(left)
	if len(leftTerms) == 0 {
		return false
	}
	for _, term := range TopicTerms(right) {
		if _, ok := leftTerms[term]; ok {
			return true
		}
	}
	return false
}
