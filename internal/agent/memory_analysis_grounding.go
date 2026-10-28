package agent

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/security"
)

// Automatic memory extraction reads the assistant's final text, which often
// relays web pages, mails and tool output. The analyzer prompt says only human
// messages supply facts, but a prompt is no control: nothing stopped a model
// that followed a planted line from storing it as a user fact or a follow-up.
// Every item therefore has to carry a quote that the human message of the same
// turn really contains (the grounding learnings.go already applies), and an item
// that reads like a standing order is dropped whatever it quotes.

// minMemoryQuoteRunes keeps a one-word quote ("the") from grounding anything.
const minMemoryQuoteRunes = 6

// normalizeGroundingText folds text for the quote check: format characters
// dropped (the analyzer saw the isolated view that strips them), case folded,
// whitespace runs collapsed to one space.
func normalizeGroundingText(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, text)
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

// humanGrounding answers whether a text occurs in the human message the
// analyzer was shown. It holds the message already filtered like learnings do:
// credentials scrubbed, fenced code, quoted lines and isolated external data
// removed, because pasted or relayed text is not what the human wrote.
type humanGrounding struct{ text string }

func newHumanGrounding(human string) humanGrounding {
	return humanGrounding{text: normalizeGroundingText(learningHumanText(human))}
}

// contains reports whether text, normalised, is a substring of the human
// message and at least minRunes long. The analyzer sees the message HTML
// escaped (IsolateExternalData), so its quote is also tried unescaped.
func (g humanGrounding) contains(text string, minRunes int) bool {
	text = strings.TrimSpace(text)
	if g.text == "" || text == "" {
		return false
	}
	for _, candidate := range []string{text, html.UnescapeString(text)} {
		normalized := normalizeGroundingText(candidate)
		if normalized != "" && utf8.RuneCountInString(normalized) >= minRunes && strings.Contains(g.text, normalized) {
			return true
		}
	}
	return false
}

// quotes reports whether quote is a usable verbatim quote of the human message.
func (g humanGrounding) quotes(quote string) bool {
	return g.contains(quote, minMemoryQuoteRunes)
}

// directiveShaped reports whether any of the texts reads like a standing order
// for a tool, a secret or an outbound channel (security.DirectiveShape).
func directiveShaped(texts ...string) bool {
	for _, text := range texts {
		if _, hit := security.DirectiveShape(text); hit {
			return true
		}
	}
	return false
}

// groundedPendingTrigger returns the trigger a pending action may be stored
// with. The action needs a verbatim human quote and no directive shape. The
// trigger is matched against later user messages and echoed into the prompt,
// so it must come from the human message too: a non-empty trigger that is not
// found there drops the action, an empty one falls back to the title only when
// the title is found there.
func groundedPendingTrigger(g humanGrounding, action pendingAction) (string, bool) {
	if !g.quotes(action.Quote) || directiveShaped(action.Title, action.Summary, action.Trigger) {
		return "", false
	}
	if trigger := strings.TrimSpace(action.Trigger); trigger != "" {
		return trigger, g.contains(trigger, 1)
	}
	if title := strings.TrimSpace(action.Title); g.contains(title, 1) {
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
			if !grounding.quotes(item.Quote) || directiveShaped(item.Content, item.Category) {
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
