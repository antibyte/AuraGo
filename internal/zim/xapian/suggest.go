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
// ctx stops the search with ctx.Err(). At most MaxSearchWindow hits are
// returned, and memory grows with the limit, not with the number of matching
// titles: candidates stream in docid order through a bounded top-k.
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
	var cands candStream
	var err error
	if words := queryWords(uq); len(words) == 0 {
		cands, err = s.wildcardOnly(uq)
	} else {
		cands, err = s.evaluate(words)
	}
	if err != nil {
		return nil, err
	}
	top := newSuggestTop(db, min(limit, MaxSearchWindow))
	for {
		c, ok, err := cands.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return top.hits(&s.p)
		}
		if err := top.offer(c); err != nil {
			return nil, err
		}
	}
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
// Its live lists (members and full term) form a min-heap on their current
// docid, so a skip touches only the lists behind the target instead of all
// of up to 101 lists.
type partialGroup struct {
	members   []*leaf // expansions; weights unused, the synonym weight applies
	synWeight float64
	full      *leaf // nil for a word-less wildcard query
	cur       uint32
	live      leafHeap
	started   bool
}

func (g *partialGroup) skipTo(t uint32) (bool, error) {
	if !g.started {
		g.started = true
		lists := g.members[:len(g.members):len(g.members)]
		if g.full != nil {
			lists = append(lists, g.full)
		}
		for _, l := range lists {
			if l.pl.done {
				continue
			}
			ok, err := l.pl.skipTo(t)
			if err != nil {
				return false, err
			}
			if ok {
				g.live = append(g.live, l)
			}
		}
		heap.Init(&g.live)
	}
	for len(g.live) > 0 && g.live[0].pl.did < t {
		ok, err := g.live[0].pl.skipTo(t)
		if err != nil {
			return false, err
		}
		if ok {
			heap.Fix(&g.live, 0)
		} else {
			heap.Pop(&g.live)
		}
	}
	if len(g.live) == 0 {
		g.cur = 0
		return false, nil
	}
	g.cur = g.live[0].pl.did
	return true, nil
}
func (g *partialGroup) doc() uint32 { return g.cur }

// score returns the group weight at did, the docid the group is on.
func (g *partialGroup) score(w bm25, did, doclen uint32) float64 {
	synWdf := g.live.wdfAt(0, did, g.full)
	if synWdf > doclen {
		synWdf = doclen
	}
	total := w.sumPart(g.synWeight, synWdf, doclen)
	if g.full != nil && !g.full.pl.done && g.full.pl.did == did {
		total += w.sumPart(g.full.weight, g.full.pl.wdf, doclen)
	}
	return total
}

// leafHeap orders posting lists by their current docid.
type leafHeap []*leaf

func (h leafHeap) Len() int           { return len(h) }
func (h leafHeap) Less(i, j int) bool { return h[i].pl.did < h[j].pl.did }
func (h leafHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *leafHeap) Push(x any)        { *h = append(*h, x.(*leaf)) }
func (h *leafHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// wdfAt sums the wdf of the lists on did in the subtree at i, leaving out
// skip. No list sorts before its parent, so the walk stops at lists past did.
func (h leafHeap) wdfAt(i int, did uint32, skip *leaf) uint32 {
	if i >= len(h) || h[i].pl.did != did {
		return 0
	}
	sum := h.wdfAt(2*i+1, did, skip) + h.wdfAt(2*i+2, did, skip)
	if h[i] != skip {
		sum += h[i].pl.wdf
	}
	return sum
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

// candStream yields scored candidate documents in ascending docid order.
type candStream interface {
	next() (scored, bool, error)
}

// emptyStream yields nothing.
type emptyStream struct{}

func (emptyStream) next() (scored, bool, error) { return scored{}, false, nil }

// evaluate prepares libzim's OR(AND part, phrase part, anchored phrase part)
// as one stream of scored documents.
func (s *suggester) evaluate(words []queryWord) (candStream, error) {
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
	and, err := s.andPart(streams, plain, partials, phraseGroups)
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
	phrase, err := s.phrasePart(raw)
	if err != nil {
		return nil, err
	}
	return &mergeStream{a: and, b: phrase}, nil
}

// joiner is a leapfrog join: next returns each docid every stream contains,
// in ascending order. ctx is polled once per skip.
type joiner struct {
	p       *poller
	streams []stream
	target  uint32 // the next docid to look at
	done    bool
}

func (j *joiner) next() (uint32, bool, error) {
	if j.done {
		return 0, false, nil
	}
	agreed := 0
	for agreed < len(j.streams) {
		for _, st := range j.streams {
			if err := j.p.poll(); err != nil {
				return 0, false, err
			}
			ok, err := st.skipTo(j.target)
			if err != nil {
				return 0, false, err
			}
			if !ok {
				j.done = true
				return 0, false, nil
			}
			if st.doc() > j.target {
				j.target, agreed = st.doc(), 0
				break
			}
			agreed++
		}
	}
	did := j.target
	if did == 0xffffffff {
		j.done = true
	} else {
		j.target = did + 1
	}
	return did, true, nil
}

// docLength moves the document length list to did, which must be in it.
func docLength(doclens *postingList, did uint32) (uint32, error) {
	ok, err := doclens.skipTo(did)
	if err != nil {
		return 0, err
	}
	if !ok || doclens.did != did {
		return 0, corruptf("document %d has postings but no length", did)
	}
	return doclens.wdf, nil
}

// andPart streams the documents matching every word, with the AND part's
// phrases checked against the stored title.
func (s *suggester) andPart(streams []stream, plain []*leaf, partials []*partialGroup, phrases [][]string) (candStream, error) {
	if len(streams) == 0 {
		return emptyStream{}, nil
	}
	doclens, err := s.db.openPostings("")
	if err != nil {
		return nil, err
	}
	return &andStream{
		s: s, j: joiner{p: &s.p, streams: streams, target: 1}, plain: plain, partials: partials,
		phrases: phrases, doclens: doclens, titles: s.db.newValueReader(0),
	}, nil
}

type andStream struct {
	s        *suggester
	j        joiner
	plain    []*leaf
	partials []*partialGroup
	phrases  [][]string
	doclens  *postingList
	titles   *valueReader
}

func (a *andStream) next() (scored, bool, error) {
	for {
		did, ok, err := a.j.next()
		if err != nil || !ok {
			return scored{}, false, err
		}
		dl, err := docLength(a.doclens, did)
		if err != nil {
			return scored{}, false, err
		}
		if len(a.phrases) > 0 {
			title, err := a.titles.get(did)
			if err != nil {
				return scored{}, false, err
			}
			toks := titleTokens(title)
			match := true
			for _, ph := range a.phrases {
				if !containsPhrase(toks, ph) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		score := 0.0
		for _, l := range a.plain {
			score += a.s.w.sumPart(l.weight, l.pl.wdf, dl)
		}
		for _, p := range a.partials {
			score += p.score(a.s.w, did, dl)
		}
		return scored{did, score}, true, nil
	}
}

// phrasePart streams libzim's two phrase subqueries: the raw words as a phrase
// (a single word needs no adjacency) and "0posanchor" + raw words (title
// starts with them). Both need every raw term in the document.
func (s *suggester) phrasePart(raw []rawTerm) (candStream, error) {
	if len(raw) == 0 {
		return emptyStream{}, nil
	}
	ps := &phraseStream{s: s, raw: raw, uniq: map[string]*leaf{}, allPositional: true}
	for _, r := range raw {
		if !r.positional {
			ps.allPositional = false
		}
		if ps.uniq[r.text] == nil {
			l, err := s.leaf(r.text)
			if err != nil {
				return nil, err
			}
			if l.pl.done {
				return emptyStream{}, nil // a raw term is absent: neither phrase can match
			}
			ps.uniq[r.text] = l
			ps.j.streams = append(ps.j.streams, leafStream{l})
		}
	}
	if len(raw) > 1 && !ps.allPositional {
		return emptyStream{}, nil // a phrase over an unpositioned bigram never matches
	}
	var err error
	if ps.anchor, err = s.leaf(anchorTerm); err != nil {
		return nil, err
	}
	if ps.doclens, err = s.db.openPostings(""); err != nil {
		return nil, err
	}
	ps.titles = s.db.newValueReader(0)
	ps.j.p, ps.j.target = &s.p, 1
	ps.words = make([]string, len(raw))
	for i, r := range raw {
		ps.words[i] = r.text
	}
	return ps, nil
}

type phraseStream struct {
	s             *suggester
	j             joiner
	raw           []rawTerm
	words         []string
	uniq          map[string]*leaf
	allPositional bool
	anchor        *leaf
	doclens       *postingList
	titles        *valueReader
}

func (ps *phraseStream) next() (scored, bool, error) {
	for {
		did, ok, err := ps.j.next()
		if err != nil || !ok {
			return scored{}, false, err
		}
		dl, err := docLength(ps.doclens, did)
		if err != nil {
			return scored{}, false, err
		}
		wordsScore := 0.0
		for _, r := range ps.raw {
			l := ps.uniq[r.text]
			wordsScore += ps.s.w.sumPart(l.weight, l.pl.wdf, dl)
		}
		score := 0.0
		var toks []string
		if ps.allPositional {
			title, err := ps.titles.get(did)
			if err != nil {
				return scored{}, false, err
			}
			toks = titleTokens(title)
		}
		if len(ps.raw) == 1 || (ps.allPositional && containsPhrase(toks, ps.words)) {
			score += wordsScore
		}
		if ps.allPositional && hasPrefixWords(toks, ps.words) {
			if at, err := ps.anchor.at(did); err != nil {
				return scored{}, false, err
			} else if at {
				score += wordsScore + ps.s.w.sumPart(ps.anchor.weight, ps.anchor.pl.wdf, dl)
			}
		}
		if score > 0 {
			return scored{did, score}, true, nil
		}
	}
}

// wildcardOnly handles a query without words: libzim then expands the whole
// normalised query as a prefix (synonym of all expansions).
func (s *suggester) wildcardOnly(uq string) (candStream, error) {
	g, err := s.partial(uq, "")
	if err != nil {
		return nil, err
	}
	doclens, err := s.db.openPostings("")
	if err != nil {
		return nil, err
	}
	return &wildcardStream{s: s, g: g, doclens: doclens, target: 1}, nil
}

type wildcardStream struct {
	s       *suggester
	g       *partialGroup
	doclens *postingList
	target  uint32
	done    bool
}

func (w *wildcardStream) next() (scored, bool, error) {
	if w.done {
		return scored{}, false, nil
	}
	if err := w.s.p.poll(); err != nil {
		return scored{}, false, err
	}
	ok, err := w.g.skipTo(w.target)
	if err != nil || !ok {
		w.done = true
		return scored{}, false, err
	}
	did := w.g.doc()
	dl, err := docLength(w.doclens, did)
	if err != nil {
		return scored{}, false, err
	}
	if did == 0xffffffff {
		w.done = true
	} else {
		w.target = did + 1
	}
	return scored{did, w.g.score(w.s.w, did, dl)}, true, nil
}

// mergeStream ORs two candidate streams; a document in both scores the sum.
type mergeStream struct {
	a, b     candStream
	ha, hb   scored
	oka, okb bool
	primed   bool
}

func (m *mergeStream) next() (scored, bool, error) {
	var err error
	if !m.primed {
		m.primed = true
		if m.ha, m.oka, err = m.a.next(); err != nil {
			return scored{}, false, err
		}
		if m.hb, m.okb, err = m.b.next(); err != nil {
			return scored{}, false, err
		}
	}
	var out scored
	switch {
	case !m.oka && !m.okb:
		return scored{}, false, nil
	case !m.okb || (m.oka && m.ha.did < m.hb.did):
		out = m.ha
		m.ha, m.oka, err = m.a.next()
	case !m.oka || m.hb.did < m.ha.did:
		out = m.hb
		m.hb, m.okb, err = m.b.next()
	default:
		out = scored{m.ha.did, m.ha.score + m.hb.score}
		if m.ha, m.oka, err = m.a.next(); err == nil {
			m.hb, m.okb, err = m.b.next()
		}
	}
	if err != nil {
		return scored{}, false, err
	}
	return out, true, nil
}

// suggestTop keeps the best limit suggestions in libzim's order (score
// descending, title ascending, docid ascending) with at most one document per
// non-empty collapse key (value slot 1), like Xapian's collapse during the
// match: a candidate replaces the kept document of its key only when it ranks
// higher, and a document pushed out of the top never comes back. Candidates
// arrive in docid order, so the value slots are read with forward readers,
// and only for candidates that can still enter the top.
type suggestTop struct {
	db     *Database
	limit  int
	h      suggestHeap
	titles *valueReader
	keys   *valueReader
}

func newSuggestTop(db *Database, limit int) *suggestTop {
	return &suggestTop{db: db, limit: limit, h: suggestHeap{byKey: map[string]int{}},
		titles: db.newValueReader(0), keys: db.newValueReader(1)}
}

type suggestion struct {
	Hit
	key string // collapse key, "" = none
}

func (a suggestion) better(b suggestion) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Title != b.Title {
		return a.Title < b.Title
	}
	return a.DocID < b.DocID
}

func (t *suggestTop) offer(c scored) error {
	full := t.h.Len() == t.limit
	if full && c.score < t.h.items[0].Score {
		return nil // below every kept suggestion whatever its title
	}
	title, err := t.titles.get(c.did)
	if err != nil {
		return err
	}
	cand := suggestion{Hit: Hit{DocID: c.did, Score: c.score, Title: title}}
	if full && !cand.better(t.h.items[0]) {
		return nil
	}
	if cand.key, err = t.keys.get(c.did); err != nil {
		return err
	}
	if i, ok := t.h.byKey[cand.key]; ok && cand.key != "" {
		if cand.better(t.h.items[i]) {
			t.h.items[i] = cand
			heap.Fix(&t.h, i)
		}
		return nil
	}
	heap.Push(&t.h, cand)
	if t.h.Len() > t.limit {
		heap.Pop(&t.h)
	}
	return nil
}

// hits returns the kept suggestions in order, with their paths.
func (t *suggestTop) hits(p *poller) ([]Hit, error) {
	items := t.h.items
	if len(items) == 0 {
		return nil, nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].better(items[j]) })
	out := make([]Hit, len(items))
	for i, it := range items {
		if err := p.poll(); err != nil {
			return nil, err
		}
		data, err := t.db.Data(it.DocID)
		if err != nil {
			return nil, err
		}
		out[i] = it.Hit
		out[i].Path = stripNamespace(data)
	}
	return out, nil
}

// suggestHeap holds the kept suggestions with the worst on top; byKey maps
// each non-empty collapse key to its suggestion's index.
type suggestHeap struct {
	items []suggestion
	byKey map[string]int
}

func (h *suggestHeap) Len() int           { return len(h.items) }
func (h *suggestHeap) Less(i, j int) bool { return h.items[j].better(h.items[i]) }
func (h *suggestHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.index(i)
	h.index(j)
}
func (h *suggestHeap) index(i int) {
	if k := h.items[i].key; k != "" {
		h.byKey[k] = i
	}
}
func (h *suggestHeap) Push(x any) {
	h.items = append(h.items, x.(suggestion))
	h.index(len(h.items) - 1)
}
func (h *suggestHeap) Pop() any {
	x := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	if x.key != "" {
		delete(h.byKey, x.key)
	}
	return x
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
