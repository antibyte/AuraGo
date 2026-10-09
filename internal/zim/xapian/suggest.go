package xapian

import (
	"container/heap"
	"context"
	"sort"
	"strings"
)

const (
	anchorTerm          = "0posanchor" // libzim prefixes every indexed title with "0posanchor "
	titleMaxWordLength  = 240          // libzim's MAX_INDEXABLE_TITLE_WORD_SIZE
	maxPartialExpansion = 100          // Xapian 1.4 QueryParser default for FLAG_PARTIAL (WILDCARD_LIMIT_MOST_FREQUENT)
)

// maxPartialScan caps the terms one prefix expansion reads. libzim (Xapian)
// reads every term with the prefix and keeps the 100 most frequent; so does
// Suggest up to this many terms, far more than a word prefix has even in the
// largest Wikipedia title index. Past the cap the expansion keeps the most
// frequent of the terms read so far. A variable so tests can lower it.
var maxPartialScan = 1 << 22

// Suggest queries a title index (X/title/xapian) the way libzim's
// SuggestionSearcher does: all words but the last must match (stemmed with a
// "Z" prefix where the title index has one), the last word also matches as a
// prefix (up to 100 most frequent expansions), titles containing the words
// as a phrase and titles starting with them score extra. Ranking uses
// BM25(k1=0.001, b=1), ties are broken by title then docid, and redirects to
// the same target collapse to the best-ranked one (value slot 1). A cancelled
// ctx stops the search with ctx.Err().
func Suggest(ctx context.Context, db *Database, a Analyzer, query string, limit int) ([]Hit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	uq := Normalize(query)
	if strings.TrimSpace(uq) == "" {
		return nil, nil
	}
	s := &suggester{p: poller{ctx: ctx}, db: db, a: a, w: newBM25Params(db, 0.001, 1)}
	words := queryWords(uq)
	var cands []scored
	var err error
	if len(words) == 0 {
		cands, err = s.wildcardOnly(uq)
	} else {
		cands, err = s.evaluate(words)
	}
	if err != nil {
		return nil, err
	}
	return s.rank(cands, limit)
}

type suggester struct {
	p  poller // ctx polling shared by every loop of one Suggest call
	db *Database
	a  Analyzer
	w  bm25
}

type scored struct {
	did   uint32
	score float64
}

// leaf is one weighted query term.
type leaf struct {
	term   string
	weight float64
	pl     *postingList
}

func (s *suggester) leaf(term string) (*leaf, error) {
	pl, err := s.db.openPostings(term)
	if err != nil {
		return nil, err
	}
	return &leaf{term: term, weight: s.w.termWeight(pl.termFreq), pl: pl}, nil
}

// at reports whether the leaf's postings are positioned on did.
func (l *leaf) at(did uint32) (bool, error) {
	if l.pl.done {
		return false, nil
	}
	ok, err := l.pl.skipTo(did)
	return ok && l.pl.did == did, err
}

// stream is a docid source for the leapfrog join of the AND part.
type stream interface {
	skipTo(target uint32) (bool, error)
	doc() uint32
}

type leafStream struct{ l *leaf }

func (s leafStream) skipTo(t uint32) (bool, error) {
	if s.l.pl.done {
		return false, nil
	}
	return s.l.pl.skipTo(t)
}
func (s leafStream) doc() uint32 { return s.l.pl.did }

// partialGroup is OR(SYNONYM(prefix expansions), full term) for the last word.
type partialGroup struct {
	members   []*leaf // expansions; weights unused, the synonym weight applies
	synWeight float64
	full      *leaf // nil for a word-less wildcard query
	cur       uint32
}

func (g *partialGroup) all() []*leaf {
	out := append([]*leaf(nil), g.members...)
	if g.full != nil {
		out = append(out, g.full)
	}
	return out
}

func (g *partialGroup) skipTo(t uint32) (bool, error) {
	found := false
	var best uint32
	for _, l := range g.all() {
		if l.pl.done {
			continue
		}
		ok, err := l.pl.skipTo(t)
		if err != nil {
			return false, err
		}
		if ok && (!found || l.pl.did < best) {
			best, found = l.pl.did, true
		}
	}
	g.cur = best
	return found, nil
}
func (g *partialGroup) doc() uint32 { return g.cur }

// score returns the group weight at did (all members already skipped to did).
func (g *partialGroup) score(w bm25, did, doclen uint32) float64 {
	var synWdf uint32
	for _, l := range g.members {
		if !l.pl.done && l.pl.did == did {
			synWdf += l.pl.wdf
		}
	}
	if synWdf > doclen {
		synWdf = doclen
	}
	total := w.sumPart(g.synWeight, synWdf, doclen)
	if g.full != nil && !g.full.pl.done && g.full.pl.did == did {
		total += w.sumPart(g.full.weight, g.full.pl.wdf, doclen)
	}
	return total
}

// makeTerm is the STEM_SOME term for a non-phrase query word.
func (s *suggester) makeTerm(w queryWord) string {
	if s.a.stem != nil && w.stemOK && shouldStem(w.text) {
		return "Z" + s.a.Stem(w.text)
	}
	return w.text
}

// partial builds the last-word group: the up to 100 most frequent terms
// starting with word as one synonym, plus the full term ("" = none). Like
// Xapian's WILDCARD_LIMIT_MOST_FREQUENT the selection runs over every term
// with the prefix (up to maxPartialScan); ties at the cut-off keep the terms
// that sort first.
func (s *suggester) partial(word string, full string) (*partialGroup, error) {
	top := &expansionHeap{}
	scanned := 0
	err := s.db.walkTerms(word, func(term string, c *cursor) (bool, error) {
		if err := s.p.poll(); err != nil {
			return false, err
		}
		tag, err := c.tagView()
		if err != nil {
			return false, err
		}
		tf, _, err := unpackUint32(tag)
		if err != nil {
			return false, err
		}
		e := expansion{term, tf}
		switch {
		case top.Len() < maxPartialExpansion:
			heap.Push(top, e)
		case e.better((*top)[0]):
			(*top)[0] = e
			heap.Fix(top, 0)
		}
		scanned++
		return scanned < maxPartialScan, nil
	})
	if err != nil {
		return nil, err
	}
	exps := []expansion(*top)
	sort.Slice(exps, func(i, j int) bool { return exps[i].term < exps[j].term })
	g := &partialGroup{}
	var tfs []uint32
	for _, e := range exps {
		l, err := s.leaf(e.term)
		if err != nil {
			return nil, err
		}
		g.members = append(g.members, l)
		tfs = append(tfs, e.tf)
	}
	g.synWeight = s.w.termWeight(orTermFreqEstimate(tfs, s.db.DocCount()))
	if full != "" {
		var err error
		if g.full, err = s.leaf(full); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// expansion is one candidate term of a prefix expansion.
type expansion struct {
	term string
	tf   uint32
}

// better orders expansions by term frequency, then term (earlier wins).
func (e expansion) better(o expansion) bool {
	if e.tf != o.tf {
		return e.tf > o.tf
	}
	return e.term < o.term
}

// expansionHeap keeps the best expansions with the worst on top.
type expansionHeap []expansion

func (h expansionHeap) Len() int           { return len(h) }
func (h expansionHeap) Less(i, j int) bool { return h[j].better(h[i]) }
func (h expansionHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *expansionHeap) Push(x any)        { *h = append(*h, x.(expansion)) }
func (h *expansionHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// rawTerm is a STEM_NONE query term with its positional flag.
type rawTerm struct {
	text       string
	positional bool
}

// evaluate scores every document matched by libzim's
// OR(AND part, phrase part, anchored phrase part).
func (s *suggester) evaluate(words []queryWord) ([]scored, error) {
	// Group phrased words (joined by - . / : \ @); CJK words never join.
	var groups [][]queryWord
	for _, w := range words {
		if w.phrased && !w.cjk && len(groups) > 0 && !groups[len(groups)-1][0].cjk {
			groups[len(groups)-1] = append(groups[len(groups)-1], w)
			continue
		}
		groups = append(groups, []queryWord{w})
	}
	var streams []stream
	var plain []*leaf           // plain AND leaves (stemmed words, CJK n-grams, phrase words)
	var phraseGroups [][]string // AND-part phrases needing adjacency
	var partials []*partialGroup
	for gi, g := range groups {
		last := gi == len(groups)-1
		switch {
		case len(g) == 1 && g[0].cjk:
			for _, ng := range ngrams(g[0].text) {
				l, err := s.leaf(ng.text)
				if err != nil {
					return nil, err
				}
				plain = append(plain, l)
			}
		case len(g) == 1 && last && g[0].atEnd:
			p, err := s.partial(g[0].text, s.makeTerm(g[0]))
			if err != nil {
				return nil, err
			}
			partials = append(partials, p)
		case len(g) == 1:
			l, err := s.leaf(s.makeTerm(g[0]))
			if err != nil {
				return nil, err
			}
			plain = append(plain, l)
		default:
			var phrase []string
			for _, w := range g {
				l, err := s.leaf(w.text)
				if err != nil {
					return nil, err
				}
				plain = append(plain, l)
				phrase = append(phrase, w.text)
			}
			phraseGroups = append(phraseGroups, phrase)
		}
	}
	for _, l := range plain {
		streams = append(streams, leafStream{l})
	}
	for _, p := range partials {
		streams = append(streams, p)
	}
	andDocs, err := s.andPart(streams, plain, partials, phraseGroups)
	if err != nil {
		return nil, err
	}
	var raw []rawTerm
	for _, w := range words {
		if w.cjk {
			for _, ng := range ngrams(w.text) {
				raw = append(raw, rawTerm{ng.text, ng.positional})
			}
			continue
		}
		raw = append(raw, rawTerm{w.text, true})
	}
	phraseDocs, err := s.phrasePart(raw)
	if err != nil {
		return nil, err
	}
	return mergeScored(andDocs, phraseDocs), nil
}

func (s *suggester) andPart(streams []stream, plain []*leaf, partials []*partialGroup, phrases [][]string) ([]scored, error) {
	if len(streams) == 0 {
		return nil, nil
	}
	doclens, err := s.db.openPostings("")
	if err != nil {
		return nil, err
	}
	titles := s.db.newValueReader(0)
	var out []scored
	target := uint32(1)
	for {
		agreed := 0
		for agreed < len(streams) {
			for _, st := range streams {
				if err := s.p.poll(); err != nil {
					return nil, err
				}
				ok, err := st.skipTo(target)
				if err != nil || !ok {
					return out, err
				}
				if st.doc() > target {
					target, agreed = st.doc(), 0
					break
				}
				agreed++
			}
		}
		did := target
		ok, err := doclens.skipTo(did)
		if err != nil {
			return nil, err
		}
		if !ok || doclens.did != did {
			return nil, corruptf("document %d has postings but no length", did)
		}
		dl := doclens.wdf
		match := true
		if len(phrases) > 0 {
			title, err := titles.get(did)
			if err != nil {
				return nil, err
			}
			toks := titleTokens(title)
			for _, ph := range phrases {
				if !containsPhrase(toks, ph) {
					match = false
					break
				}
			}
		}
		if match {
			score := 0.0
			for _, l := range plain {
				score += s.w.sumPart(l.weight, l.pl.wdf, dl)
			}
			for _, p := range partials {
				score += p.score(s.w, did, dl)
			}
			out = append(out, scored{did, score})
		}
		if did == 0xffffffff {
			return out, nil
		}
		target = did + 1
	}
}

// phrasePart scores libzim's two phrase subqueries: the raw words as a phrase
// (a single word needs no adjacency) and "0posanchor" + raw words (title
// starts with them). Both need every raw term in the document.
func (s *suggester) phrasePart(raw []rawTerm) ([]scored, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	uniq := map[string]*leaf{}
	var leaves []*leaf
	allPositional := true
	for _, r := range raw {
		if !r.positional {
			allPositional = false
		}
		if uniq[r.text] == nil {
			l, err := s.leaf(r.text)
			if err != nil {
				return nil, err
			}
			if l.pl.done {
				return nil, nil // a raw term is absent: neither phrase can match
			}
			uniq[r.text] = l
			leaves = append(leaves, l)
		}
	}
	anchor, err := s.leaf(anchorTerm)
	if err != nil {
		return nil, err
	}
	doclens, err := s.db.openPostings("")
	if err != nil {
		return nil, err
	}
	titles := s.db.newValueReader(0)
	var streams []stream
	for _, l := range leaves {
		streams = append(streams, leafStream{l})
	}
	var out []scored
	target := uint32(1)
	words := make([]string, len(raw))
	for i, r := range raw {
		words[i] = r.text
	}
	for {
		agreed := 0
		for agreed < len(streams) {
			for _, st := range streams {
				if err := s.p.poll(); err != nil {
					return nil, err
				}
				ok, err := st.skipTo(target)
				if err != nil || !ok {
					return out, err
				}
				if st.doc() > target {
					target, agreed = st.doc(), 0
					break
				}
				agreed++
			}
		}
		did := target
		ok, err := doclens.skipTo(did)
		if err != nil {
			return nil, err
		}
		if !ok || doclens.did != did {
			return nil, corruptf("document %d has postings but no length", did)
		}
		dl := doclens.wdf
		wordsScore := 0.0
		for _, r := range raw {
			l := uniq[r.text]
			wordsScore += s.w.sumPart(l.weight, l.pl.wdf, dl)
		}
		score := 0.0
		var toks []string
		if allPositional {
			title, err := titles.get(did)
			if err != nil {
				return nil, err
			}
			toks = titleTokens(title)
		}
		if len(raw) == 1 || (allPositional && containsPhrase(toks, words)) {
			score += wordsScore
		}
		if allPositional && hasPrefixWords(toks, words) {
			if at, err := anchor.at(did); err != nil {
				return nil, err
			} else if at {
				score += wordsScore + s.w.sumPart(anchor.weight, anchor.pl.wdf, dl)
			}
		}
		if score > 0 {
			out = append(out, scored{did, score})
		}
		if did == 0xffffffff {
			return out, nil
		}
		target = did + 1
	}
}

// wildcardOnly handles a query without words: libzim then expands the whole
// normalised query as a prefix (synonym of all expansions).
func (s *suggester) wildcardOnly(uq string) ([]scored, error) {
	g, err := s.partial(uq, "")
	if err != nil {
		return nil, err
	}
	doclens, err := s.db.openPostings("")
	if err != nil {
		return nil, err
	}
	var out []scored
	target := uint32(1)
	for {
		if err := s.p.poll(); err != nil {
			return nil, err
		}
		ok, err := g.skipTo(target)
		if err != nil || !ok {
			return out, err
		}
		did := g.doc()
		if ok, err := doclens.skipTo(did); err != nil || !ok || doclens.did != did {
			if err == nil {
				err = corruptf("document %d has postings but no length", did)
			}
			return nil, err
		}
		out = append(out, scored{did, g.score(s.w, did, doclens.wdf)})
		if did == 0xffffffff {
			return out, nil
		}
		target = did + 1
	}
}

// rank orders candidates by score (desc), title (asc), docid (asc) and keeps
// the best document per collapse key (value slot 1), up to limit hits.
func (s *suggester) rank(cands []scored, limit int) ([]Hit, error) {
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].did < cands[j].did
	})
	seen := map[string]bool{}
	var hits []Hit
	for i := 0; i < len(cands) && len(hits) < limit; {
		j := i
		for j < len(cands) && cands[j].score == cands[i].score {
			j++
		}
		group := make([]Hit, 0, j-i)
		for _, c := range cands[i:j] {
			if err := s.p.poll(); err != nil {
				return nil, err
			}
			title, err := s.db.Value(c.did, 0)
			if err != nil {
				return nil, err
			}
			group = append(group, Hit{DocID: c.did, Score: c.score, Title: title})
		}
		sort.SliceStable(group, func(x, y int) bool {
			if group[x].Title != group[y].Title {
				return group[x].Title < group[y].Title
			}
			return group[x].DocID < group[y].DocID
		})
		for _, h := range group {
			key, err := s.db.Value(h.DocID, 1)
			if err != nil {
				return nil, err
			}
			if key != "" {
				if seen[key] {
					continue
				}
				seen[key] = true
			}
			data, err := s.db.Data(h.DocID)
			if err != nil {
				return nil, err
			}
			h.Path = stripNamespace(data)
			hits = append(hits, h)
			if len(hits) == limit {
				break
			}
		}
		i = j
	}
	return hits, nil
}

// mergeScored sums the scores of two ascending candidate lists.
func mergeScored(a, b []scored) []scored {
	out := make([]scored, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case j >= len(b) || (i < len(a) && a[i].did < b[j].did):
			out = append(out, a[i])
			i++
		case i >= len(a) || b[j].did < a[i].did:
			out = append(out, b[j])
			j++
		default:
			out = append(out, scored{a[i].did, a[i].score + b[j].score})
			i++
			j++
		}
	}
	return out
}

// titleTokens are the positional terms of an indexed title, in position order
// (position = index + 2; the anchor term holds position 1).
func titleTokens(title string) []string {
	return positionalTerms(indexTokens(Normalize(title)), titleMaxWordLength)
}

func containsPhrase(toks, words []string) bool {
	if len(words) == 0 || len(words) > len(toks) {
		return false
	}
	for i := 0; i+len(words) <= len(toks); i++ {
		if hasPrefixWords(toks[i:], words) {
			return true
		}
	}
	return false
}

func hasPrefixWords(toks, words []string) bool {
	if len(words) == 0 || len(words) > len(toks) {
		return false
	}
	for i, w := range words {
		if toks[i] != w {
			return false
		}
	}
	return true
}
