package agent

import (
	"sort"
	"strings"
	"unicode"
)

// discoverySuggestionThreshold is the lowest Dice coefficient that still counts
// as a plausible misspelling or partial name. A one-letter swap in a six-letter
// name ("dokcer" for "docker") scores 0.33.
const discoverySuggestionThreshold = 0.3

// discoverySuggestions names up to limit catalog entries whose name or alias
// resembles the query, best first, for answers that would otherwise be a dead
// end (zero search hits, unknown tool name). keep filters the candidates, for
// example to enabled tools the run may call; nil keeps all.
func discoverySuggestions(catalog *ToolCatalog, query string, limit int, keep func(*ToolCatalogEntry) bool) []string {
	query = strings.TrimSpace(query)
	if catalog == nil || query == "" || limit <= 0 {
		return nil
	}
	// The whole query plus every token of three or more characters, so that
	// "send an emial" still finds send_email through "emial".
	var probes []map[string]struct{}
	if grams := discoveryTrigrams(query); grams != nil {
		probes = append(probes, grams)
	}
	for _, token := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(token)) < 3 {
			continue
		}
		if grams := discoveryTrigrams(token); grams != nil {
			probes = append(probes, grams)
		}
	}
	if len(probes) == 0 {
		return nil
	}

	type scored struct {
		name  string
		score float64
	}
	var matches []scored
	for _, entry := range catalog.Entries() {
		if entry == nil || (keep != nil && !keep(entry)) {
			continue
		}
		best := 0.0
		for _, label := range append([]string{entry.Name}, entry.Aliases...) {
			labelGrams := discoveryTrigrams(label)
			for _, probe := range probes {
				best = max(best, discoveryDice(probe, labelGrams))
			}
		}
		if best >= discoverySuggestionThreshold {
			matches = append(matches, scored{entry.Name, best})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].name < matches[j].name
	})
	names := make([]string, 0, min(limit, len(matches)))
	for _, match := range matches[:min(limit, len(matches))] {
		names = append(names, match.name)
	}
	return names
}

// toolNameEchoLimit bounds a model-supplied tool name that an answer repeats.
const toolNameEchoLimit = 80

// echoedToolName shortens a tool name before an answer repeats it.
func echoedToolName(name string) string {
	return truncateUTF8ToLimit(strings.TrimSpace(name), toolNameEchoLimit, "")
}

// unknownToolGuidance tells a model that called a name this run does not know
// how to find the right tool. suggestions are up to three enabled tools of the
// run's catalog that resemble the name; the guidance repeats them. It reads the
// published run catalog only and never builds one. The pointer to discover_tools
// appears only when the run's scope allows that tool.
func unknownToolGuidance(dc *DispatchContext, name string) (guidance string, suggestions []string) {
	name = echoedToolName(name)
	if dc != nil {
		if catalog := GetToolCatalogState(dc.discoveryKey()); catalog != nil {
			suggestions = discoverySuggestions(catalog, name, 3, func(entry *ToolCatalogEntry) bool {
				return entry.Enabled && entry.Name != name && catalogEntryAllowed(dc, entry)
			})
		}
	}
	var parts []string
	if dispatchToolAllowed(dc, "discover_tools") {
		parts = append(parts, "Use discover_tools with operation=search to find the right tool, then follow its call_method.")
	}
	if len(suggestions) > 0 {
		parts = append(parts, "Closest enabled tools: "+strings.Join(suggestions, ", ")+".")
	}
	return strings.Join(parts, " "), suggestions
}

// discoveryTrigrams returns the character trigrams of s, lower-cased, with
// every non-letter and non-digit mapped to '_' and padded with '_' on both
// sides so that word starts and ends weigh in. Nil when s has no letter or
// digit.
func discoveryTrigrams(s string) map[string]struct{} {
	var b strings.Builder
	b.WriteByte('_')
	alnum := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			alnum = true
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	b.WriteByte('_')
	runes := []rune(b.String())
	if !alnum || len(runes) < 3 {
		return nil
	}
	grams := make(map[string]struct{}, len(runes))
	for i := 0; i+3 <= len(runes); i++ {
		grams[string(runes[i:i+3])] = struct{}{}
	}
	return grams
}

// discoveryDice is the Dice coefficient of two trigram sets: 2|A∩B| / (|A|+|B|).
func discoveryDice(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for gram := range a {
		if _, ok := b[gram]; ok {
			shared++
		}
	}
	return 2 * float64(shared) / float64(len(a)+len(b))
}
