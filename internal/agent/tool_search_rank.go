package agent

import (
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"aurago/internal/prompts"
)

// The lexical ranking scores a query against a tool by the stems the two
// share. A stem found in the tool's name says more than the same stem in its
// prose, so every field has a weight.
const (
	toolSearchNameWeight        = 3.0
	toolSearchTermWeight        = 2.5 // curated search terms and aliases
	toolSearchOperationWeight   = 1.5
	toolSearchCategoryWeight    = 1.0
	toolSearchDescriptionWeight = 1.0

	// A stem that only starts like a query stem counts three quarters; a
	// German compound that ends in it ("Festplattenbelegung" for "Belegung")
	// counts half, but only in names and curated terms.
	toolSearchPrefixLevel   = 0.75
	toolSearchCompoundLevel = 0.5

	// toolSearchMaxTokens bounds the content tokens of a query.
	toolSearchMaxTokens = 32
	// A lexical hit needs this score and this share of the best score in its
	// status class, which keeps a stray description match from padding the
	// list behind a clear answer.
	toolSearchMinScore    = 1.5
	toolSearchRelativeCut = 0.2
	// Coverage scales a score between these shares: a tool that matches only
	// one of four query words keeps 0.4 + 0.6/4 of its weighted sum.
	toolSearchCoverageFloor = 0.4

	toolSearchPhraseBoost = 1.5  // the query is one of the tool's curated phrases
	toolSearchRunBoost    = 1.15 // two query words follow each other in the tool's text
	toolSearchNameBoost   = 1.25 // the query names every part of the tool's name
)

// Exact tiers rank above every lexical score.
const (
	toolSearchTierLexical = iota
	toolSearchTierOperation
	toolSearchTierAlias
	toolSearchTierName
)

// toolSearchHit is one ranked search result.
type toolSearchHit struct {
	Entry *ToolCatalogEntry
	Score float64
	Tier  int
	Class int // toolSearchStatusClass
}

func hitEntries(hits []toolSearchHit) []*ToolCatalogEntry {
	out := make([]*ToolCatalogEntry, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Entry)
	}
	return out
}

// toolSearchStatusClass orders results by what the user can do with them: a
// tool that can be called now comes first, one that only needs setup second,
// a disabled or out-of-scope one last.
func toolSearchStatusClass(entry *ToolCatalogEntry) int {
	switch {
	case entry.Status == ToolStatusNeedsSetup:
		return 1
	case entry.Enabled && (entry.Status == ToolStatusActive || entry.Status == ToolStatusHidden):
		return 0
	default:
		return 2
	}
}

type toolSearchStem struct {
	text   string
	runes  int
	weight float64
}

// toolSearchDoc is the searchable text of one catalog entry.
type toolSearchDoc struct {
	entry *ToolCatalogEntry
	*toolSearchDocText
}

// toolSearchDocText is what is derived from an entry's text. Entries with the
// same text share it (see toolSearchDocCache).
type toolSearchDocText struct {
	// Folded keys of the exact tiers.
	name       string
	aliases    []string
	operations []string
	// stems holds every stem of the entry once, with the weight of its best
	// field, heaviest first.
	stems []toolSearchStem
	// nameParts are the stems of the name that a query can mention.
	nameParts []string
	// phrases are the curated terms and aliases as stem sequences;
	// sequences adds the name and the descriptions: text in which two
	// consecutive query words can follow each other.
	phrases       [][]string
	sequences     [][]string
	nonDisplacing bool
}

// toolSearchIndex is the lazily built search view of a catalog.
type toolSearchIndex struct {
	docs []*toolSearchDoc
}

// searchIndex returns the search view, building it at the first search. It
// reads the entries' text then, so the entries must be complete by that time;
// add drops the view.
func (c *ToolCatalog) searchIndex() *toolSearchIndex {
	c.searchMu.Lock()
	defer c.searchMu.Unlock()
	if c.search == nil {
		entries := c.Entries()
		index := &toolSearchIndex{docs: make([]*toolSearchDoc, 0, len(entries))}
		for _, entry := range entries {
			index.docs = append(index.docs, newToolSearchDoc(entry))
		}
		c.search = index
	}
	return c.search
}

// toolSearchNameText is the text of the entry's name that a user could
// say: the prefix of a skill, custom tool or package is dropped, and an
// external MCP tool, whose name is an encoded identifier, uses its aliases.
func toolSearchNameText(entry *ToolCatalogEntry) string {
	if strings.HasPrefix(entry.Name, "mcp__") {
		return strings.Join(entry.Aliases, " ")
	}
	for _, prefix := range []string{"skill__", "tool__", "package__"} {
		if rest, ok := strings.CutPrefix(entry.Name, prefix); ok {
			return rest
		}
	}
	return entry.Name
}

// toolSearchDocCache keeps the derived text by content. The adaptive selection
// builds a catalog for every request, while the tools behind it rarely change.
// The cache starts over when it outgrows toolSearchDocCacheLimit, which only a
// stream of ever-changing descriptions can cause.
var toolSearchDocCache struct {
	sync.Mutex
	docs map[string]*toolSearchDocText
}

const toolSearchDocCacheLimit = 4096

func newToolSearchDoc(entry *ToolCatalogEntry) *toolSearchDoc {
	operations := toolCatalogOperations(entry)
	key := toolSearchDocKey(entry, operations)
	toolSearchDocCache.Lock()
	text := toolSearchDocCache.docs[key]
	toolSearchDocCache.Unlock()
	if text == nil {
		text = newToolSearchDocText(entry, operations)
		toolSearchDocCache.Lock()
		if toolSearchDocCache.docs == nil || len(toolSearchDocCache.docs) >= toolSearchDocCacheLimit {
			toolSearchDocCache.docs = make(map[string]*toolSearchDocText)
		}
		toolSearchDocCache.docs[key] = text
		toolSearchDocCache.Unlock()
	}
	return &toolSearchDoc{entry: entry, toolSearchDocText: text}
}

// toolSearchDocKey joins every input of newToolSearchDocText.
func toolSearchDocKey(entry *ToolCatalogEntry, operations []string) string {
	var b strings.Builder
	for _, part := range [][]string{{entry.Name, entry.Category, entry.Description, schemaFunctionDescription(entry)}, entry.SearchTerms, entry.Aliases, operations} {
		for _, text := range part {
			b.WriteString(text)
			b.WriteByte(0)
		}
		b.WriteByte(1)
	}
	return b.String()
}

func schemaFunctionDescription(entry *ToolCatalogEntry) string {
	if entry.Schema.Function == nil {
		return ""
	}
	return entry.Schema.Function.Description
}

func newToolSearchDocText(entry *ToolCatalogEntry, operations []string) *toolSearchDocText {
	doc := &toolSearchDocText{
		name:          foldToolSearchText(entry.Name),
		nonDisplacing: slices.Contains(prompts.NonDisplacingTools, entry.Name),
	}
	best := make(map[string]float64, 96)
	add := func(stem string, weight float64) {
		if weight > best[stem] {
			best[stem] = weight
		}
	}

	var nameSequence []string
	for _, word := range toolSearchWords(foldToolSearchText(toolSearchNameText(entry)), false) {
		stem := stemToolSearchWord(word)
		add(stem, toolSearchNameWeight)
		nameSequence = append(nameSequence, stem)
		if _, stop := toolSearchStopwords[word]; !stop && utf8.RuneCountInString(stem) >= 2 {
			doc.nameParts = append(doc.nameParts, stem)
		}
	}
	doc.sequences = append(doc.sequences, nameSequence)

	for _, alias := range entry.Aliases {
		doc.aliases = append(doc.aliases, foldToolSearchText(alias))
	}
	for _, term := range slices.Concat(entry.SearchTerms, entry.Aliases) {
		for _, stem := range toolSearchTermsOf(term, true) {
			add(stem, toolSearchTermWeight)
		}
		if phrase := toolSearchTermsOf(term, false); len(phrase) > 0 {
			doc.phrases = append(doc.phrases, phrase)
			doc.sequences = append(doc.sequences, phrase)
		}
	}

	for _, op := range operations {
		folded := foldToolSearchText(op)
		doc.operations = append(doc.operations, folded)
		for _, word := range toolSearchWords(folded, false) {
			add(stemToolSearchWord(word), toolSearchOperationWeight)
		}
	}

	categoryText := entry.Category + " " + toolCategoryLabels[entry.Category]
	for _, word := range toolSearchWords(foldToolSearchText(categoryText), false) {
		add(stemToolSearchWord(word), toolSearchCategoryWeight)
	}

	descriptions := []string{entry.Description}
	if schema := schemaFunctionDescription(entry); schema != "" && schema != entry.Description {
		descriptions = append(descriptions, schema)
	}
	if short := toolShortDescForName(entry.Name); short != "" && !slices.Contains(descriptions, short) {
		descriptions = append(descriptions, short)
	}
	for _, text := range descriptions {
		stems := toolSearchTermsOf(text, true)
		for _, stem := range stems {
			add(stem, toolSearchDescriptionWeight)
		}
		doc.sequences = append(doc.sequences, stems)
	}

	doc.stems = make([]toolSearchStem, 0, len(best))
	for text, weight := range best {
		doc.stems = append(doc.stems, toolSearchStem{text: text, runes: utf8.RuneCountInString(text), weight: weight})
	}
	sort.Slice(doc.stems, func(i, j int) bool {
		if doc.stems[i].weight != doc.stems[j].weight {
			return doc.stems[i].weight > doc.stems[j].weight
		}
		return doc.stems[i].text < doc.stems[j].text
	})
	return doc
}

// toolSearchStats are the document frequencies of the stems over the catalog
// of all built-in tools. They are static on purpose: the weight of a word
// must not depend on which optional tools a run happens to have enabled.
type toolSearchStats struct {
	docs int
	df   map[string]int
}

var toolSearchUniverse = sync.OnceValue(func() *toolSearchStats {
	index := BuildToolCatalog(builtinToolSchemasCached(allBuiltinToolFeatureFlags()), nil, "").searchIndex()
	stats := &toolSearchStats{docs: len(index.docs), df: make(map[string]int, 4096)}
	for _, doc := range index.docs {
		for _, stem := range doc.stems {
			stats.df[stem.text]++
		}
	}
	return stats
})

// idf is the BM25 inverse document frequency of a stem. A stem the universe
// has never seen borrows the frequency of the stems it starts or ends like,
// and counts as rare when there are none.
func (s *toolSearchStats) idf(stem string) float64 {
	df := s.df[stem]
	if df == 0 {
		runes := utf8.RuneCountInString(stem)
		for other, count := range s.df {
			if toolSearchPrefixRelated(stem, runes, other) {
				df += count
			}
		}
		df = min(df, s.docs)
	}
	return math.Log(1 + (float64(s.docs)-float64(df)+0.5)/(float64(df)+0.5))
}

func toolSearchPrefixRelated(stem string, runes int, other string) bool {
	otherRunes := utf8.RuneCountInString(other)
	if otherRunes < runes {
		return otherRunes >= 4 && strings.HasPrefix(stem, other)
	}
	return runes >= 4 && strings.HasPrefix(other, stem)
}

type toolSearchToken struct {
	stem  string
	runes int
	group int // index into toolSearchQuery.groupSize, -1 outside an identifier
	idf   float64
}

type toolSearchQuery struct {
	exact     string // folded, for the exact tiers
	nameAlias string
	tokens    []toolSearchToken
	groupSize []int
	// phrase is the query as a stem sequence.
	phrase []string
}

// parseToolSearchQuery splits a query into stems. A word with underscores is
// an identifier ("disk_io"): its parts are tokens of one group, and a tool
// only counts them when it matches about two thirds of them.
// Stopwords never become tokens; the parts of an identifier do.
func parseToolSearchQuery(query, nameAlias string) toolSearchQuery {
	trimmed := strings.TrimSpace(query)
	q := toolSearchQuery{exact: foldToolSearchText(trimmed), nameAlias: nameAlias}
	if q.exact == "" {
		return q
	}
	q.phrase = toolSearchTermsOf(q.exact, false)
	seen := make(map[string]bool)
	words := strings.FieldsFunc(q.exact, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
	for _, word := range words {
		if len(q.tokens) >= toolSearchMaxTokens {
			break
		}
		if strings.Contains(word, "_") {
			parts := strings.FieldsFunc(word, func(r rune) bool { return r == '_' })
			if len(parts) == 0 {
				continue
			}
			if len(parts) >= 2 {
				group := len(q.groupSize)
				size := 0
				for _, part := range parts {
					if len(q.tokens) >= toolSearchMaxTokens {
						break
					}
					stem := stemToolSearchWord(part)
					q.tokens = append(q.tokens, toolSearchToken{stem: stem, runes: utf8.RuneCountInString(stem), group: group})
					size++
				}
				q.groupSize = append(q.groupSize, size)
				continue
			}
			word = parts[0]
		}
		if _, stop := toolSearchStopwords[word]; stop || utf8.RuneCountInString(word) < 2 {
			continue
		}
		stem := stemToolSearchWord(word)
		if seen[stem] {
			continue
		}
		seen[stem] = true
		q.tokens = append(q.tokens, toolSearchToken{stem: stem, runes: utf8.RuneCountInString(stem), group: -1})
	}
	return q
}

// toolSearchLevel is how closely a query stem matches a stem of the tool:
// 1 when equal, 0.75 when one starts with the other (the shorter one needs
// four runes), 0.5 for a compound head, 0 otherwise.
func toolSearchLevel(stem string, runes int, other toolSearchStem) float64 {
	if stem == other.text {
		return 1
	}
	short, long, shortRunes, longRunes := stem, other.text, runes, other.runes
	if other.runes < runes {
		short, long, shortRunes, longRunes = other.text, stem, other.runes, runes
	}
	if shortRunes >= 4 && strings.HasPrefix(long, short) {
		return toolSearchPrefixLevel
	}
	if other.weight >= toolSearchTermWeight && shortRunes >= 5 && longRunes >= shortRunes+3 && strings.HasSuffix(long, short) {
		return toolSearchCompoundLevel
	}
	return 0
}

// tokenWeight is the best weight times level over the stems of the tool.
func (d *toolSearchDoc) tokenWeight(token *toolSearchToken) float64 {
	best := 0.0
	for _, other := range d.stems {
		if other.weight <= best {
			break // the stems are sorted by weight and a level never exceeds 1
		}
		if v := other.weight * toolSearchLevel(token.stem, token.runes, other); v > best {
			best = v
		}
	}
	return best
}

// score is the weighted sum of the matched query words times a coverage
// factor, raised when the query reads like the tool's own phrasing.
func (d *toolSearchDoc) score(q *toolSearchQuery) float64 {
	if len(q.tokens) == 0 {
		return 0
	}
	var matched [toolSearchMaxTokens]float64
	for i := range q.tokens {
		matched[i] = d.tokenWeight(&q.tokens[i])
	}
	// An identifier counts for a tool only when about two thirds of its
	// parts match: "zzzz_nonexistent_capability" is no query for a tool that
	// merely mentions "capability".
	if len(q.groupSize) > 0 {
		var hits [toolSearchMaxTokens]int
		for i, token := range q.tokens {
			if token.group >= 0 && matched[i] > 0 {
				hits[token.group]++
			}
		}
		for i, token := range q.tokens {
			if token.group >= 0 && hits[token.group]*3 < 2*q.groupSize[token.group] {
				matched[i] = 0
			}
		}
	}
	var sum, matchedIDF, totalIDF float64
	for i, token := range q.tokens {
		totalIDF += token.idf
		if matched[i] > 0 {
			sum += token.idf * matched[i]
			matchedIDF += token.idf
		}
	}
	if sum == 0 {
		return 0
	}
	score := sum * (toolSearchCoverageFloor + (1-toolSearchCoverageFloor)*matchedIDF/totalIDF)
	switch {
	case d.hasPhrase(q.phrase):
		score *= toolSearchPhraseBoost
	case d.continuesRun(q.phrase):
		score *= toolSearchRunBoost
	}
	if d.namesAllMatched(q) {
		score *= toolSearchNameBoost
	}
	return score
}

func (d *toolSearchDoc) hasPhrase(phrase []string) bool {
	return len(phrase) > 0 && slices.ContainsFunc(d.phrases, func(p []string) bool { return slices.Equal(p, phrase) })
}

// continuesRun reports whether two consecutive query words also follow each
// other in the name, a curated phrase or a description.
func (d *toolSearchDoc) continuesRun(phrase []string) bool {
	for _, seq := range d.sequences {
		for i := 0; i+1 < len(seq); i++ {
			for j := 0; j+1 < len(phrase); j++ {
				if seq[i] == phrase[j] && seq[i+1] == phrase[j+1] {
					return true
				}
			}
		}
	}
	return false
}

// namesAllMatched reports whether the query carries every part of the name.
func (d *toolSearchDoc) namesAllMatched(q *toolSearchQuery) bool {
	return len(d.nameParts) > 0 && !slices.ContainsFunc(d.nameParts, func(part string) bool {
		return !slices.ContainsFunc(q.tokens, func(token toolSearchToken) bool { return token.stem == part })
	})
}

// exactTier reports how exactly the whole query names the tool.
func (d *toolSearchDoc) exactTier(q *toolSearchQuery) int {
	switch {
	case d.name == q.exact || (d.entry.Enabled && d.entry.Name == q.nameAlias):
		return toolSearchTierName
	case slices.Contains(d.aliases, q.exact):
		return toolSearchTierAlias
	case slices.Contains(d.operations, q.exact):
		return toolSearchTierOperation
	}
	return toolSearchTierLexical
}

// rankedSearch ranks the catalog for a query: exact name, alias and operation
// matches first (whatever their status), then the lexical hits. Within a tier
// callable tools come before tools that need setup, those before disabled
// ones; the score decides inside a status class and the name decides ties.
func (c *ToolCatalog) rankedSearch(query string) []toolSearchHit {
	if c == nil {
		return nil
	}
	nameAlias := discoverToolNameAlias(query)
	if nameAlias == "" {
		query = resolveDiscoverToolName(query)
	}
	q := parseToolSearchQuery(query, nameAlias)
	if q.exact == "" {
		return nil
	}
	stats := toolSearchUniverse()
	for i := range q.tokens {
		q.tokens[i].idf = stats.idf(q.tokens[i].stem)
	}

	index := c.searchIndex()
	hits := make([]toolSearchHit, 0, 32)
	var top [3]float64
	for _, doc := range index.docs {
		tier := doc.exactTier(&q)
		score := doc.score(&q)
		if tier == toolSearchTierLexical && score == 0 {
			continue
		}
		hit := toolSearchHit{Entry: doc.entry, Score: score, Tier: tier, Class: toolSearchStatusClass(doc.entry)}
		if tier == toolSearchTierLexical && !doc.nonDisplacing {
			top[hit.Class] = max(top[hit.Class], score)
		}
		hits = append(hits, hit)
	}
	// The cut-off compares with the best hit that is not a non-displacing
	// tool, so enabling such a tool never changes what the others return.
	kept := hits[:0]
	for _, hit := range hits {
		if hit.Tier == toolSearchTierLexical && (hit.Score < toolSearchMinScore || hit.Score < toolSearchRelativeCut*top[hit.Class]) {
			continue
		}
		kept = append(kept, hit)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		switch {
		case a.Tier != b.Tier:
			return a.Tier > b.Tier
		case a.Class != b.Class:
			return a.Class < b.Class
		case a.Score != b.Score:
			return a.Score > b.Score
		}
		return a.Entry.Name < b.Entry.Name
	})
	return kept
}
