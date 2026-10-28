package agent

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/memory"
)

// Automatic memory extraction reads the assistant's final text, which often
// relays web pages, mails and tool output. The analyzer prompt says only human
// messages supply facts, but a prompt is no control: nothing stopped a model
// that followed a planted line from storing it as a user fact or a follow-up.
// Every item therefore has to carry a quote that the human message of the same
// turn really contains (the grounding learnings.go already applies), the item
// has to be derived from that quote (a real quote must not vouch for an
// unrelated sentence), and an item that reads like a standing order is dropped
// whatever it quotes.

const (
	// A quote has to be a sentence fragment, not a word: "German" or "Please"
	// occurs in nearly every message and grounds nothing.
	minMemoryQuoteRunes = 16
	minMemoryQuoteWords = 3
	// Han and Kana text has no spaces; a quote of mostly such characters is
	// judged by length alone.
	minMemoryQuoteCJKRunes = 8
	// A trigger or title that stands in for the match against later messages
	// has to be a word, not a letter.
	minGroundedFieldRunes = 3
)

// normalizeGroundingText folds text for the quote check (memory.NormalizeGroundingText).
func normalizeGroundingText(text string) string {
	return memory.NormalizeGroundingText(text)
}

// humanGrounding answers whether a text occurs in the human message the
// analyzer was shown. It holds the message already filtered like learnings do:
// credentials scrubbed, fenced code, quoted lines and isolated external data
// removed, because pasted or relayed text is not what the human wrote.
type humanGrounding struct{ text string }

func newHumanGrounding(human string) humanGrounding {
	return humanGrounding{text: normalizeGroundingText(learningHumanText(human))}
}

// find returns the normalised form of text when it is a substring of the human
// message and at least minRunes long. The analyzer sees the message HTML
// escaped (IsolateExternalData), so its quote is also tried unescaped.
func (g humanGrounding) find(text string, minRunes int) (string, bool) {
	text = strings.TrimSpace(text)
	if g.text == "" || text == "" {
		return "", false
	}
	for _, candidate := range []string{text, html.UnescapeString(text)} {
		normalized := normalizeGroundingText(candidate)
		if normalized != "" && utf8.RuneCountInString(normalized) >= minRunes && strings.Contains(g.text, normalized) {
			return normalized, true
		}
	}
	return "", false
}

// contains reports whether text, normalised, is a substring of the human
// message and at least minRunes long.
func (g humanGrounding) contains(text string, minRunes int) bool {
	_, ok := g.find(text, minRunes)
	return ok
}

// quote returns the normalised quote when it is a usable verbatim quote of the
// human message: three words and sixteen runes (eight for Han and Kana text).
func (g humanGrounding) quote(quote string) (string, bool) {
	normalized, ok := g.find(quote, minMemoryQuoteCJKRunes)
	if !ok {
		return "", false
	}
	runes, cjk := 0, 0
	for _, r := range normalized {
		runes++
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
			cjk++
		}
	}
	if cjk*2 >= runes {
		return normalized, true
	}
	if runes < minMemoryQuoteRunes || len(strings.Fields(normalized)) < minMemoryQuoteWords {
		return "", false
	}
	return normalized, true
}

// directiveShaped reports whether any of the texts reads like a standing order
// for a tool, a secret, an outbound channel or without restrictions
// (memory.StandingOrder).
func directiveShaped(texts ...string) bool {
	for _, text := range texts {
		if _, hit := memory.StandingOrder(text); hit {
			return true
		}
	}
	return false
}

// groundedPendingTrigger returns the trigger a pending action may be stored
// with. The action needs a usable human quote, no directive shape, and a title
// and summary that say what the quote says. The trigger is matched against
// later user messages and echoed into the prompt, so it must come from the
// human message too: a non-empty trigger that is not found there drops the
// action, an empty one falls back to the title only when the title is found
// there.
func groundedPendingTrigger(g humanGrounding, action pendingAction) (string, bool) {
	quote, ok := g.quote(action.Quote)
	if !ok || directiveShaped(action.Title, action.Summary, action.Trigger) ||
		!memory.DerivedFrom(action.Title, quote) || !memory.DerivedFrom(action.Summary, quote) {
		return "", false
	}
	if trigger := strings.TrimSpace(action.Trigger); trigger != "" {
		return trigger, g.contains(trigger, minGroundedFieldRunes)
	}
	if title := strings.TrimSpace(action.Title); g.contains(title, minGroundedFieldRunes) {
		return title, true
	}
	return "", false
}

// groundMemoryAnalysisResult keeps the facts, preferences, corrections and
// pending actions that the human message of the turn supports and returns how
// many items it dropped. Learnings carry their own quote check and are left
// alone. A result without a human message keeps nothing.
func groundMemoryAnalysisResult(result memoryAnalysisResult) (memoryAnalysisResult, int) {
	grounding := newHumanGrounding(result.human)
	dropped := 0
	keep := func(items []extractedFact) []extractedFact {
		var kept []extractedFact
		for _, item := range items {
			quote, ok := grounding.quote(item.Quote)
			if !ok || directiveShaped(item.Content, item.Category) || !memory.DerivedFrom(item.Content, quote) {
				dropped++
				continue
			}
			kept = append(kept, item)
		}
		return kept
	}
	result.Facts, result.Preferences, result.Corrections = keep(result.Facts), keep(result.Preferences), keep(result.Corrections)
	var actions []pendingAction
	for _, action := range result.PendingActions {
		trigger, ok := groundedPendingTrigger(grounding, action)
		if !ok {
			dropped++
			continue
		}
		action.Trigger = trigger
		actions = append(actions, action)
	}
	result.PendingActions = actions
	return result, dropped
}
