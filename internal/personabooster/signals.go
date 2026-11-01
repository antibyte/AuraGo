// Package personabooster decides temporary persona phases. It is pure logic:
// no database, no prompt files and no clock reads.
package personabooster

import (
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Trigger names. Catalogs, inputs and logs use the same strings.
const (
	SignalPrefix = "signal:"
	CuePrefix    = "cue:"

	// TriggerSpontaneous weights a phase for random starts.
	TriggerSpontaneous = "spontaneous"

	SignalMoodFrustratedStreak = "signal:mood_frustrated_streak"
	SignalMoodPlayfulHigh      = "signal:mood_playful_high"
	SignalMoodConcernedLoad    = "signal:mood_concerned_load"
	SignalMoodCuriousCalm      = "signal:mood_curious_calm"
	SignalNight                = "signal:night"
	SignalLongAbsence          = "signal:long_absence"
	SignalLongSession          = "signal:long_session"
	SignalTextShouting         = "signal:text_shouting"
	SignalTextSwearing         = "signal:text_swearing"
	SignalTextStop             = "signal:text_stop"

	CueProvocation    = "cue:provocation"
	CueCriticism      = "cue:criticism"
	CuePraise         = "cue:praise"
	CueDistress       = "cue:distress"
	CueWantsCalm      = "cue:wants_calm"
	CueDeepTopic      = "cue:deep_topic"
	CueConspiracyBait = "cue:conspiracy_bait"
	CueRulesPressure  = "cue:rules_pressure"
	CueBanter         = "cue:banter"
)

// Priorities from the design.
const (
	PriorityCalm        = 100
	PriorityConfront    = 70
	PriorityMood        = 50
	PriorityContext     = 40
	PrioritySpontaneous = 20
)

// CuePromptOptions is the helper cue enum as written into analysis prompts.
const CuePromptOptions = "provocation|criticism|praise|distress|wants_calm|deep_topic|conspiracy_bait|rules_pressure|banter|none"

// MinCueConfidence is the lowest helper confidence that makes a cue count.
const MinCueConfidence = 0.5

var triggerPriority = map[string]int{
	CueDistress:    PriorityCalm,
	CueWantsCalm:   PriorityCalm,
	SignalTextStop: PriorityCalm,

	CueProvocation:     PriorityConfront,
	CueCriticism:       PriorityConfront,
	SignalTextSwearing: PriorityConfront,
	SignalTextShouting: PriorityConfront,

	SignalMoodFrustratedStreak: PriorityMood,
	SignalMoodPlayfulHigh:      PriorityMood,
	SignalMoodConcernedLoad:    PriorityMood,
	SignalMoodCuriousCalm:      PriorityMood,

	CuePraise:         PriorityContext,
	CueDeepTopic:      PriorityContext,
	CueConspiracyBait: PriorityContext,
	CueRulesPressure:  PriorityContext,
	CueBanter:         PriorityContext,
	SignalNight:       PriorityContext,
	SignalLongAbsence: PriorityContext,
	SignalLongSession: PriorityContext,

	TriggerSpontaneous: PrioritySpontaneous,
}

// Priority returns the design priority of a trigger, or 0 for unknown names.
func Priority(trigger string) int {
	return triggerPriority[trigger]
}

// KnownTrigger reports whether a catalog may reference the trigger.
func KnownTrigger(trigger string) bool {
	_, ok := triggerPriority[trigger]
	return ok
}

// NormalizeCue maps a helper cue ("criticism" or "cue:criticism") to its
// trigger name. Unknown values, "none" and low confidence are rejected.
func NormalizeCue(raw string, confidence float64) (string, bool) {
	if math.IsNaN(confidence) || math.IsInf(confidence, 0) || confidence < MinCueConfidence {
		return "", false
	}
	name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), CuePrefix)
	if name == "" || name == "none" {
		return "", false
	}
	trigger := CuePrefix + name
	if !KnownTrigger(trigger) {
		return "", false
	}
	return trigger, true
}

// MoodInput is the engine snapshot read at turn start.
type MoodInput struct {
	Mood                  string
	Arousal               float64
	Load                  float64
	Friction              float64
	ConsecutiveToolErrors int
}

// MoodSignals derives the engine-backed signals.
func MoodSignals(in MoodInput) []string {
	var out []string
	switch in.Mood {
	case "frustrated":
		if in.Friction >= 0.5 || in.ConsecutiveToolErrors >= 2 {
			out = append(out, SignalMoodFrustratedStreak)
		}
	case "playful", "creative":
		if in.Arousal >= 0.6 {
			out = append(out, SignalMoodPlayfulHigh)
		}
	case "concerned", "cautious":
		if in.Load >= 0.5 {
			out = append(out, SignalMoodConcernedLoad)
		}
	case "curious", "analytical":
		if in.Arousal <= 0.4 {
			out = append(out, SignalMoodCuriousCalm)
		}
	}
	return out
}

// TimeSignals derives the clock, absence and session signals. now must be
// local time.
func TimeSignals(now time.Time, inactivityHours float64, longSession bool) []string {
	var out []string
	if hour := now.Hour(); hour >= 0 && hour <= 4 {
		out = append(out, SignalNight)
	}
	if inactivityHours >= 24 {
		out = append(out, SignalLongAbsence)
	}
	if longSession {
		out = append(out, SignalLongSession)
	}
	return out
}

var swearWords = map[string]bool{
	"shit": true, "shitty": true, "bullshit": true, "damn": true, "dammit": true, "goddamn": true,
	"crap": true, "crappy": true, "wtf": true, "asshole": true, "bastard": true,
	"mist": true, "kacke": true, "kackmist": true, "arsch": true, "arschloch": true, "dreck": true,
}

var swearPrefixes = []string{"fuck", "scheiß", "scheiss", "verdammt", "verfickt", "drecks"}

// stopPhrases maps each stop phrase to the words that may follow it. A nil
// list accepts any continuation; otherwise the phrase must end a clause or be
// followed by one of the listed words ("hör auf mich" is not a stop request).
var stopPhrases = map[string][]string{
	"beruhig dich":  nil,
	"beruhige dich": nil,
	"sei netter":    nil,
	"calm down":     nil,
	"hör auf":       {"damit", "bitte", "jetzt", "zu"},
	"hoer auf":      {"damit", "bitte", "jetzt", "zu"},
	"lass das":      {"bitte", "jetzt", "sein"},
	"stop it":       {"please", "now", "already"},
	"be nice":       {"please", "now"},
}

// DetectTextSignals returns shouting, swearing and stop signals for the
// human message, in that order.
func DetectTextSignals(message string) []string {
	text := strings.TrimSpace(message)
	if text == "" {
		return nil
	}
	var out []string
	if isShouting(text) {
		out = append(out, SignalTextShouting)
	}
	lower := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	if hasSwearing(lower) {
		out = append(out, SignalTextSwearing)
	}
	if isStopRequest(lower) {
		out = append(out, SignalTextStop)
	}
	return out
}

func isShouting(text string) bool {
	if strings.Contains(text, "!!!") {
		return true
	}
	letters, upper := 0, 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	return letters >= 12 && float64(upper) >= 0.6*float64(letters)
}

func hasSwearing(lower string) bool {
	for _, word := range strings.FieldsFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) }) {
		if swearWords[word] {
			return true
		}
		for _, prefix := range swearPrefixes {
			if strings.HasPrefix(word, prefix) {
				return true
			}
		}
	}
	return false
}

func isStopRequest(lower string) bool {
	for phrase, continuations := range stopPhrases {
		if containsStopPhrase(lower, phrase, continuations) {
			return true
		}
	}
	return false
}

func containsStopPhrase(text, phrase string, continuations []string) bool {
	for offset := 0; offset < len(text); {
		idx := strings.Index(text[offset:], phrase)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(phrase)
		offset = end
		if !letterBoundary(text, start, end) {
			continue
		}
		if continuations == nil {
			return true
		}
		rest := strings.TrimLeft(text[end:], " ")
		if rest == "" {
			return true
		}
		if r, _ := utf8.DecodeRuneInString(rest); strings.ContainsRune(".!?,;", r) {
			return true
		}
		next := strings.FieldsFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) })
		if len(next) > 0 && slices.Contains(continuations, next[0]) {
			return true
		}
	}
	return false
}

func letterBoundary(text string, start, end int) bool {
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(text[:start]); unicode.IsLetter(r) {
			return false
		}
	}
	if end < len(text) {
		if r, _ := utf8.DecodeRuneInString(text[end:]); unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
