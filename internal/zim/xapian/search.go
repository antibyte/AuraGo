package xapian

import (
	"container/heap"
	"context"
	"sort"
)

// Op combines query terms.
type Op int

const (
	OpAnd Op = iota
	OpOr
)

// Hit is one ranked document.
type Hit struct {
	DocID uint32
	Score float64
	Path  string // from document data, without the "C/" prefix
	Title string // value slot 0
}

// ctxCheckEvery bounds how many documents are scored between ctx checks.
const ctxCheckEvery = 256

type searchTerm struct {
	pl     *postingList
	weight float64 // termWeight * multiplicity of the term in the query
}

// Search ranks the documents matching terms with Xapian's default BM25
// weighting, exactly like libzim's full-text Searcher (OpAnd) or an OR of the
// same terms (OpOr): highest score first, ties by ascending docid. A term
// without postings makes an AND match nothing and is skipped by OR. Repeated
// terms count once per occurrence, as in Xapian. estimatedTotal is the exact
// number of matching documents.
func Search(ctx context.Context, db *Database, terms []string, op Op, offset, limit int) (hits []Hit, estimatedTotal int, err error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || len(terms) == 0 {
		return nil, 0, nil
	}
	mult := map[string]int{}
	var order []string
	for _, t := range terms {
		if t == "" {
			continue
		}
		if mult[t] == 0 {
			order = append(order, t)
		}
		mult[t]++
	}
	w := newBM25(db)
	var srcs []*searchTerm
	tfs := map[*searchTerm]uint32{}
	for _, t := range order {
		pl, err := db.openPostings(t)
		if err != nil {
			return nil, 0, err
		}
		if pl.done {
			if op == OpAnd {
				return nil, 0, nil
			}
			continue
		}
		st := &searchTerm{pl: pl, weight: w.termWeight(pl.termFreq) * float64(mult[t])}
		srcs = append(srcs, st)
		tfs[st] = pl.termFreq
	}
	if len(srcs) == 0 {
		return nil, 0, nil
	}
	doclens, err := db.openPostings("")
	if err != nil {
		return nil, 0, err
	}
	top := &topK{k: offset + limit}
	score := func(did uint32, matched []*searchTerm) error {
		ok, err := doclens.skipTo(did)
		if err != nil {
			return err
		}
		if !ok || doclens.did != did {
			return corruptf("document %d has postings but no length", did)
		}
		s := 0.0
		for _, st := range matched {
			s += w.sumPart(st.weight, st.pl.wdf, doclens.wdf)
		}
		top.offer(did, s)
		estimatedTotal++
		return nil
	}
	if op == OpAnd {
		sort.Slice(srcs, func(i, j int) bool { return tfs[srcs[i]] < tfs[srcs[j]] })
		err = andMatches(ctx, srcs, score)
	} else {
		err = orMatches(ctx, srcs, score)
	}
	if err != nil {
		return nil, 0, err
	}
	ranked := top.sorted()
	if offset >= len(ranked) {
		return nil, estimatedTotal, nil
	}
	ranked = ranked[offset:]
	titles := db.newValueReader(0)
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].DocID < ranked[j].DocID })
	for i := range ranked {
		data, err := db.Data(ranked[i].DocID)
		if err != nil {
			return nil, 0, err
		}
		ranked[i].Path = stripNamespace(data)
		if ranked[i].Title, err = titles.get(ranked[i].DocID); err != nil {
			return nil, 0, err
		}
	}
	sortHits(ranked)
	return ranked, estimatedTotal, nil
}

// ExistingTerms returns the terms that index at least one document, in their
// original order (duplicates kept). The localwiki facade uses it to drop
// query words the index does not know before an AND search (spec: "terms
// with zero postings are dropped"); Search itself stays a faithful Xapian AND.
func ExistingTerms(db *Database, terms []string) ([]string, error) {
	known := map[string]bool{}
	var out []string
	for _, t := range terms {
		ok, seen := known[t]
		if !seen {
			tf, err := db.TermFreq(t)
			if err != nil {
				return nil, err
			}
			ok = tf > 0
			known[t] = ok
		}
		if ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// andMatches calls fn for every docid present in all lists (leapfrog join;
// srcs[0] should be the rarest term).
func andMatches(ctx context.Context, srcs []*searchTerm, fn func(uint32, []*searchTerm) error) error {
	target := uint32(1)
	n := 0
	for {
		agreed := 0
		for agreed < len(srcs) {
			for _, st := range srcs {
				ok, err := st.pl.skipTo(target)
				if err != nil || !ok {
					return err
				}
				if st.pl.did > target {
					target = st.pl.did
					agreed = 0
					break
				}
				agreed++
			}
		}
		if err := fn(target, srcs); err != nil {
			return err
		}
		if n++; n%ctxCheckEvery == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		if target == 0xffffffff {
			return nil
		}
		target++
	}
}

// orMatches calls fn for every docid present in at least one list, with the
// lists positioned on that docid.
func orMatches(ctx context.Context, srcs []*searchTerm, fn func(uint32, []*searchTerm) error) error {
	live := make([]*searchTerm, 0, len(srcs))
	for _, st := range srcs {
		if st.pl.Next() {
			live = append(live, st)
		} else if err := st.pl.Err(); err != nil {
			return err
		}
	}
	n := 0
	matched := make([]*searchTerm, 0, len(srcs))
	for len(live) > 0 {
		did := live[0].pl.did
		for _, st := range live[1:] {
			if st.pl.did < did {
				did = st.pl.did
			}
		}
		matched = matched[:0]
		for _, st := range live {
			if st.pl.did == did {
				matched = append(matched, st)
			}
		}
		if err := fn(did, matched); err != nil {
			return err
		}
		next := live[:0]
		for _, st := range live {
			if st.pl.did == did && !st.pl.Next() {
				if err := st.pl.Err(); err != nil {
					return err
				}
				continue
			}
			next = append(next, st)
		}
		live = next
		if n++; n%ctxCheckEvery == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// topK keeps the k best (score desc, docid asc) hits.
type topK struct {
	k int
	h hitHeap
}

func (t *topK) offer(did uint32, score float64) {
	if t.k <= 0 {
		return
	}
	h := Hit{DocID: did, Score: score}
	if len(t.h) < t.k {
		heap.Push(&t.h, h)
		return
	}
	if better(h, t.h[0]) {
		t.h[0] = h
		heap.Fix(&t.h, 0)
	}
}

func (t *topK) sorted() []Hit {
	out := append([]Hit(nil), t.h...)
	sortHits(out)
	return out
}

func better(a, b Hit) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	return a.DocID < b.DocID
}

func sortHits(h []Hit) { sort.Slice(h, func(i, j int) bool { return better(h[i], h[j]) }) }

// hitHeap is a min-heap with the worst hit on top.
type hitHeap []Hit

func (h hitHeap) Len() int           { return len(h) }
func (h hitHeap) Less(i, j int) bool { return better(h[j], h[i]) }
func (h hitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hitHeap) Push(x any)        { *h = append(*h, x.(Hit)) }
func (h *hitHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
