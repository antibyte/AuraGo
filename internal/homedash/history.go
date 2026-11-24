package homedash

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

type ring struct {
	buf     []Sample
	head, n int
	lastUse time.Time
}

func (r *ring) last() *Sample {
	if r.n == 0 {
		return nil
	}
	return &r.buf[(r.head+r.n-1)%len(r.buf)]
}

func (r *ring) push(s Sample) {
	if r.n < len(r.buf) {
		r.buf[(r.head+r.n)%len(r.buf)] = s
		r.n++
		return
	}
	r.buf[r.head] = s
	r.head = (r.head + 1) % len(r.buf)
}

// historyStore keeps in-memory numeric rings for subscribed entities: one point per bucket (last
// value wins), points per ring, at most maxEntities rings (least recently used evicted). Nothing
// persists. mu guards every field; the store never calls out while holding it and never takes
// Hub.mu (the hub calls it under Hub.mu: lock order Hub.mu then historyStore.mu).
type historyStore struct {
	mu          sync.Mutex
	points      int
	maxEntities int
	bucket      time.Duration
	rings       map[EntityID]*ring
}

func newHistoryStore(points, entities int, bucket time.Duration) *historyStore {
	return &historyStore{points: points, maxEntities: entities, bucket: bucket, rings: map[EntityID]*ring{}}
}

func (s *historyStore) touch(id EntityID, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.ringLocked(id, now); r != nil {
		r.lastUse = now
	}
}

// ringLocked returns the ring of id, creating it (and evicting the least recently used ring when the
// store is full). It returns nil for a degenerate configuration.
func (s *historyStore) ringLocked(id EntityID, now time.Time) *ring {
	if s.points <= 0 || s.maxEntities <= 0 {
		return nil
	}
	if r := s.rings[id]; r != nil {
		return r
	}
	for len(s.rings) >= s.maxEntities {
		var oldest EntityID
		var at time.Time
		found := false
		for k, r := range s.rings {
			if !found || r.lastUse.Before(at) {
				oldest, at, found = k, r.lastUse, true
			}
		}
		if !found {
			break
		}
		delete(s.rings, oldest)
	}
	r := &ring{buf: make([]Sample, s.points), lastUse: now}
	s.rings[id] = r
	return r
}

// observe stores a finite sample. The hub calls it only for subscribed entities, so a missing ring
// (evicted while the entity stayed subscribed) is created again, with the same LRU eviction as touch.
func (s *historyStore) observe(id EntityID, v float64, at time.Time) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.ringLocked(id, at)
	if r == nil || len(r.buf) == 0 {
		return
	}
	r.lastUse = at
	if l := r.last(); l != nil && s.bucket > 0 && l.T.Truncate(s.bucket).Equal(at.Truncate(s.bucket)) {
		*l = Sample{T: at, V: v}
		return
	}
	r.push(Sample{T: at, V: v})
}

func (s *historyStore) resize(points, entities int, bucket time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if points == s.points && entities == s.maxEntities && bucket == s.bucket {
		return
	}
	s.points, s.maxEntities, s.bucket, s.rings = points, entities, bucket, map[EntityID]*ring{}
}

func (s *historyStore) series(id EntityID, from, to time.Time, points int) []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rings[id]
	if r == nil {
		return nil
	}
	out := []Sample{}
	for i := 0; i < r.n; i++ {
		smp := r.buf[(r.head+i)%len(r.buf)]
		if !smp.T.Before(from) && !smp.T.After(to) {
			out = append(out, smp)
		}
	}
	return downsample(out, points)
}

// downsample averages consecutive groups so at most points samples remain (T = last of the group).
func downsample(in []Sample, points int) []Sample {
	if points <= 0 || len(in) <= points {
		return in
	}
	stride := (len(in) + points - 1) / points
	out := make([]Sample, 0, points)
	for i := 0; i < len(in); i += stride {
		end := min(i+stride, len(in))
		sum := 0.0
		for _, s := range in[i:end] {
			sum += s.V
		}
		out = append(out, Sample{T: in[end-1].T, V: sum / float64(end-i)})
	}
	return out
}

// Series is a numeric history answer.
type Series struct {
	Origin string    `json:"origin"` // "source" | "memory"
	Since  time.Time `json:"since,omitempty"`
	Points []Sample  `json:"points"`
}

const (
	MaxHistoryRange  = 7 * 24 * time.Hour
	MaxHistoryPoints = 500
	MaxSnapshotBytes = 8 << 20
)

func historyGuarded(ctx context.Context, src HistorySource, native string, from, to time.Time, points int) (out []Sample, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("history panic: %v", r)
		}
	}()
	return src.History(ctx, native, from, to, points)
}

func snapshotGuarded(ctx context.Context, src SnapshotSource, native string, w, h int) (img []byte, ct string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("snapshot panic: %v", r)
		}
	}()
	return src.Snapshot(ctx, native, w, h)
}

// activeSource returns the live source of an entity's source id, or nil. No source call is made.
func (h *Hub) activeSource(id EntityID) (Source, string) {
	srcID, native, err := ParseEntityID(string(id))
	if err != nil {
		return nil, ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if rt := h.rts[srcID]; rt != nil && rt.active && !h.closed {
		return rt.src, native
	}
	return nil, ""
}

// History returns a numeric series: from the source when it can serve one, otherwise from the
// in-memory ring (samples since the process started).
func (h *Hub) History(ctx context.Context, id EntityID, from, to time.Time, points int) (Series, error) {
	if !to.After(from) || to.Sub(from) > MaxHistoryRange || points < 1 || points > MaxHistoryPoints {
		return Series{}, newError(CodeBadRequest, "invalid history range or point count")
	}
	if _, err := h.Entity(ctx, id); err != nil {
		return Series{}, err
	}
	src, native := h.activeSource(id)
	if hs, ok := src.(HistorySource); ok && src != nil {
		cctx, cancel := context.WithTimeout(ctx, h.opts.CatalogTimeout)
		defer cancel()
		pts, err := historyGuarded(cctx, hs, native, from, to, points)
		if err != nil {
			h.log.Warn("homedash: history failed", "entity", id, "error", err)
			return Series{}, newError(CodeSourceError, "the source could not serve the history")
		}
		if pts == nil {
			pts = []Sample{}
		}
		return Series{Origin: "source", Points: downsample(pts, points)}, nil
	}
	pts := []Sample{}
	if h.rec != nil {
		if got := h.rec.series(id, from, to, points); got != nil {
			pts = got
		}
	}
	return Series{Origin: "memory", Since: h.startedAt, Points: pts}, nil
}

// Snapshot fetches one camera image from a source that can serve it. It is never stored.
func (h *Hub) Snapshot(ctx context.Context, id EntityID, w, hgt int) ([]byte, string, error) {
	e, err := h.Entity(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if _, ok := findCap(e.Capabilities, CapSnapshot); !ok {
		return nil, "", newError(CodeUnsupportedCommand, "the entity has no snapshot")
	}
	src, native := h.activeSource(id)
	ss, ok := src.(SnapshotSource)
	if !ok || src == nil {
		return nil, "", newError(CodeUnsupportedCommand, "the source serves no snapshots")
	}
	if w < 1 || w > 1920 {
		w = 640
	}
	if hgt < 1 || hgt > 1080 {
		hgt = 360
	}
	cctx, cancel := context.WithTimeout(ctx, h.opts.CommandTimeout)
	defer cancel()
	img, ct, err := snapshotGuarded(cctx, ss, native, w, hgt)
	if err != nil || len(img) > MaxSnapshotBytes {
		if err != nil {
			h.log.Warn("homedash: snapshot failed", "entity", id, "error", err)
		}
		return nil, "", newError(CodeSourceError, "the source could not serve the snapshot")
	}
	return img, ct, nil
}
