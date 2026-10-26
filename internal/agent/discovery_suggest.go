package agent

import (
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// discoverySuggestionThreshold is the lowest Dice coefficient that still counts
// as a plausible misspelling or partial name. A one-letter swap in a six-letter
// name ("dokcer" for "docker") scores 0.33.
const discoverySuggestionThreshold = 0.3

const (
	// discoverySuggestionTokens bounds the words of a query that are compared
	// on their own; discoveryToolNameLimit bounds the query.
	discoverySuggestionTokens = 8
	// discoveryLabelLimit bounds the bytes of a label that are compared (the
	// longest built-in one has 36), discoveryLabelCacheLimit the labels kept.
	discoveryLabelLimit      = 128
	discoveryLabelCacheLimit = 8192
)

// discoveryLabelCache keeps the trigrams of the labels of tools (names,
// aliases and search terms) by text, so that a suggestion does not take
// thousands of labels apart again. It starts over when it is full.
var discoveryLabelCache struct {
	sync.Mutex
	grams map[string][]uint64
}

func discoveryLabelGrams(label string) []uint64 {
	label = truncateUTF8ToLimit(label, discoveryLabelLimit, "")
	discoveryLabelCache.Lock()
	grams, ok := discoveryLabelCache.grams[label]
	discoveryLabelCache.Unlock()
	if ok {
		return grams
	}
	grams = discoveryTrigrams(label)
	discoveryLabelCache.Lock()
	if discoveryLabelCache.grams == nil || len(discoveryLabelCache.grams) >= discoveryLabelCacheLimit {
		discoveryLabelCache.grams = make(map[string][]uint64)
	}
	discoveryLabelCache.grams[label] = grams
	discoveryLabelCache.Unlock()
	return grams
}

// discoverySuggestions names up to limit catalog entries whose name or alias
// resembles the query, best first, for answers that would otherwise be a dead
// end (zero search hits, unknown tool name). keep filters the candidates, for
// example to enabled tools the run may call; nil keeps all.
func discoverySuggestions(catalog *ToolCatalog, query string, limit int, keep func(*ToolCatalogEntry) bool) []string {
	query = truncateUTF8ToLimit(strings.TrimSpace(query), discoveryToolNameLimit, "")
	if catalog == nil || query == "" || limit <= 0 {
		return nil
	}
	// The whole query plus the first tokens of three or more characters, so
	// that "send an emial" still finds send_email through "emial".
	var probes [][]uint64
	if grams := discoveryTrigrams(query); grams != nil {
		probes = append(probes, grams)
	}
	tokens := 0
	for _, token := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if tokens == discoverySuggestionTokens {
			break
		}
		if utf8.RuneCountInString(token) < 3 {
			continue
		}
		tokens++
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
		for _, label := range slices.Concat([]string{entry.Name}, entry.Aliases, entry.SearchTerms) {
			labelGrams := discoveryLabelGrams(label)
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

// echoedToolName makes a model-supplied tool name safe to repeat in an answer:
// control characters (CR, LF, TAB and the rest) and Unicode line and paragraph
// separators are removed, so a name can never start a line of its own and pose
// as a "Tool Output:" or "SYSTEM:" line, and the rest is cut to
// toolNameEchoLimit bytes on a character boundary.
func echoedToolName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, name)
	return truncateUTF8ToLimit(strings.TrimSpace(name), toolNameEchoLimit, "")
}

// isDiscoveryMetaTool reports the tools that only route to other tools. A
// suggestion for a missing name never proposes them: invoke_tool needs a target
// of its own and discover_tools is already named by the guidance.
func isDiscoveryMetaTool(name string) bool {
	return name == "invoke_tool" || name == "discover_tools"
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
				return entry.Enabled && entry.Name != name && !isDiscoveryMetaTool(entry.Name) && catalogEntryAllowed(dc, entry)
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

// discoveryTrigrams returns the sorted, distinct character trigrams of s,
// lower-cased, with every non-letter and non-digit mapped to '_' and padded
// with '_' on both sides so that word starts and ends weigh in. A trigram is
// its three runes packed into 21 bits each. Nil when s has no letter or digit.
func discoveryTrigrams(s string) []uint64 {
	runes := make([]rune, 1, len(s)+2)
	runes[0] = '_'
	alnum := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			alnum = true
			runes = append(runes, r)
		} else {
			runes = append(runes, '_')
		}
	}
	runes = append(runes, '_')
	if !alnum || len(runes) < 3 {
		return nil
	}
	grams := make([]uint64, 0, len(runes)-2)
	for i := 0; i+3 <= len(runes); i++ {
		grams = append(grams, uint64(runes[i])<<42|uint64(runes[i+1])<<21|uint64(runes[i+2]))
	}
	slices.Sort(grams)
	return slices.Compact(grams)
}

// discoveryDice is the Dice coefficient of two trigram sets: 2|A∩B| / (|A|+|B|).
func discoveryDice(a, b []uint64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] < b[j]:
			i++
		case a[i] > b[j]:
			j++
		default:
			shared++
			i++
			j++
		}
	}
	return 2 * float64(shared) / float64(len(a)+len(b))
}
