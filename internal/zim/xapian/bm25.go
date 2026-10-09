package xapian

import (
	"container/heap"
	"math"
)

// bm25 reproduces Xapian 1.4's BM25Weight for a single database without a
// relevance set (k2 = 0, k3 = 1, query term wqf = 1).
type bm25 struct {
	k1, b, minNormLen float64
	docCount          float64
	lenFactor         float64 // 1 / average document length (0 for an empty db)
}

// Xapian's default BM25Weight(): k1=1, k2=0, k3=1, b=0.5, min_normlen=0.5.
func newBM25(db *Database) bm25 { return newBM25Params(db, 1, 0.5) }

// newBM25Params is used by suggestions with libzim's BM25Weight(0.001,0,1,1,0.5).
func newBM25Params(db *Database, k1, b float64) bm25 {
	w := bm25{k1: k1, b: b, minNormLen: 0.5, docCount: float64(db.DocCount())}
	if db.AverageLength() > 0 {
		w.lenFactor = 1 / db.AverageLength()
	}
	return w
}

// termWeight is the per-term factor: log of the (smoothed) inverse document
// frequency times (k1 + 1). The k3 factor (k3+1)*wqf/(k3+wqf) is 1 for wqf = 1.
func (w bm25) termWeight(tf uint32) float64 {
	if tf == 0 {
		return 0
	}
	f := float64(tf)
	tw := (w.docCount - f + 0.5) / (f + 0.5)
	if tw < 2 {
		tw = tw*0.5 + 1
	}
	return math.Log(tw) * (w.k1 + 1)
}

// sumPart is one term's contribution for a document with wdf and length.
func (w bm25) sumPart(termWeight float64, wdf, doclen uint32) float64 {
	if wdf == 0 {
		return 0
	}
	normlen := float64(doclen) * w.lenFactor // never NaN: same as math.Max, without the call
	if normlen < w.minNormLen {
		normlen = w.minNormLen
	}
	d := float64(wdf)
	return termWeight * (d / (w.k1*(normlen*w.b+(1-w.b)) + d))
}

// orTermFreqEstimate is Xapian's term frequency estimate for an OR of terms
// (used for synonym weights): repeatedly combine the two smallest estimates
// l, r into round(l + r - l*r/N) until one remains. Each combined estimate is
// clamped to [0, N]: valid term frequencies never leave that range, but on
// damaged data (tf > N, or N = 0) the formula could go negative or past
// 2^32, and converting such a float to uint32 is implementation-defined.
func orTermFreqEstimate(tfs []uint32, docCount uint32) uint32 {
	if len(tfs) == 0 {
		return 0
	}
	h := make(uintHeap, len(tfs))
	copy(h, tfs)
	heap.Init(&h)
	n := float64(docCount)
	for h.Len() > 1 {
		l := float64(heap.Pop(&h).(uint32))
		r := float64(heap.Pop(&h).(uint32))
		est := l + r
		if n > 0 {
			est -= l * r / n
		}
		est = math.Min(math.Max(est, 0), n)
		heap.Push(&h, uint32(est+0.5))
	}
	return h[0]
}

type uintHeap []uint32

func (h uintHeap) Len() int           { return len(h) }
func (h uintHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h uintHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *uintHeap) Push(x any)        { *h = append(*h, x.(uint32)) }
func (h *uintHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
