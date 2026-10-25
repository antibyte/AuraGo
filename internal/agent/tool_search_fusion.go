package agent

import (
	"path/filepath"
	"sort"
	"strings"
)

// toolFusionConfig tunes how the lexical ranking and the semantic manual
// ranking merge (fuseToolRanking).
type toolFusionConfig struct {
	// ManualWindow is how many manuals, counted from the best, take part. A
	// manual that documents no available tool still uses up a position.
	ManualWindow int
	// MaxSemanticTools caps the tools the manuals add that the lexical
	// ranking did not find.
	MaxSemanticTools int
	// FamilyLimit is the size above which a manual is too broad to add tools
	// of its own: one manual documents several tools (email, mqtt, yepapi_*),
	// and a larger family would crowd the cap.
	FamilyLimit                   int
	LexicalWeight, SemanticWeight float64
	// K flattens the reciprocal rank 1/(K+rank): the larger it is, the less
	// the very first ranks outweigh the following ones.
	K float64
}

// adaptiveToolFusion serves the first-turn tool selection, discoveryToolFusion
// the discover_tools search, whose result page is five tools long.
var (
	adaptiveToolFusion  = toolFusionConfig{ManualWindow: 8, MaxSemanticTools: 4, FamilyLimit: 3, LexicalWeight: 1, SemanticWeight: 0.8, K: 20}
	discoveryToolFusion = toolFusionConfig{ManualWindow: 5, MaxSemanticTools: 5, FamilyLimit: 3, LexicalWeight: 1, SemanticWeight: 0.8, K: 20}
)

// manualIDFromPath returns the manual ID behind a tool guide path
// ("tools_manuals/email.md" is "email").
func manualIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(filepath.Clean(path)), filepath.Ext(path))
}

// fuseToolRanking merges the lexical ranking with the semantic ranking of the
// tool manuals by reciprocal rank fusion and returns tool names, best first.
//
//   - pinned names (exact matches) come first, in the given order.
//   - lexical is the lexical ranking; a tool scores LexicalWeight/(K+rank).
//   - manuals is the semantic ranking. A manual for which skip reports true
//     takes no window position and no rank: a non-displacing tool that cannot
//     be used must not cost another tool its place. members maps a manual to
//     the tools it documents, in the order they are offered; they share the
//     manual's rank and score SemanticWeight/(K+rank).
//   - a tool that is also ranked lexically gains the semantic score without
//     using up MaxSemanticTools; only tools the lexical ranking missed use it.
//     A manual with more than FamilyLimit tools adds none of those: its
//     lexical members only gain the score.
//
// Ties go to the better of the two ranks, then to the tool seen first.
func fuseToolRanking(pinned, lexical, manuals []string, members map[string][]string, skip func(manual string) bool, cfg toolFusionConfig) []string {
	out := make([]string, 0, len(pinned)+len(lexical)+cfg.MaxSemanticTools)
	taken := make(map[string]bool, len(pinned)+len(lexical))
	for _, name := range pinned {
		if !taken[name] {
			taken[name] = true
			out = append(out, name)
		}
	}

	type candidate struct {
		name             string
		lexRank, semRank int
	}
	var candidates []*candidate
	byName := make(map[string]*candidate, len(lexical)+cfg.MaxSemanticTools)
	for _, name := range lexical {
		if taken[name] || byName[name] != nil {
			continue
		}
		c := &candidate{name: name, lexRank: len(candidates) + 1}
		candidates = append(candidates, c)
		byName[name] = c
	}

	seenManual := make(map[string]bool, len(manuals))
	considered, added := 0, 0
	for _, manual := range manuals {
		if seenManual[manual] || (skip != nil && skip(manual)) {
			continue
		}
		seenManual[manual] = true
		if considered == cfg.ManualWindow {
			break
		}
		considered++
		family := members[manual]
		for _, name := range family {
			if taken[name] {
				continue
			}
			if c := byName[name]; c != nil {
				if c.semRank == 0 {
					c.semRank = considered
				}
				continue
			}
			if len(family) > cfg.FamilyLimit || added == cfg.MaxSemanticTools {
				continue
			}
			c := &candidate{name: name, semRank: considered}
			candidates = append(candidates, c)
			byName[name] = c
			added++
		}
	}

	score := func(c *candidate) float64 {
		s := 0.0
		if c.lexRank > 0 {
			s += cfg.LexicalWeight / (cfg.K + float64(c.lexRank))
		}
		if c.semRank > 0 {
			s += cfg.SemanticWeight / (cfg.K + float64(c.semRank))
		}
		return s
	}
	bestRank := func(c *candidate) int {
		switch {
		case c.lexRank == 0:
			return c.semRank
		case c.semRank == 0:
			return c.lexRank
		}
		return min(c.lexRank, c.semRank)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if sa, sb := score(a), score(b); sa != sb {
			return sa > sb
		}
		return bestRank(a) < bestRank(b)
	})
	for _, c := range candidates {
		out = append(out, c.name)
	}
	return out
}
