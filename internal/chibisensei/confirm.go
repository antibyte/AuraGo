package chibisensei

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// DeleteTokenTTL is how long a delete-preview token stays valid.
const DeleteTokenTTL = 5 * time.Minute

const maxDeleteTokens = 256

// DeleteTokens holds delete-preview tokens in memory.
type DeleteTokens struct {
	mu    sync.Mutex
	now   func() time.Time
	items map[string]deleteToken // at most 256; the oldest is evicted
}

type deleteToken struct {
	project, sel string
	dataRev      int64
	expires      time.Time
}

// NewDeleteTokens creates the token holder; now may be nil.
func NewDeleteTokens(now func() time.Time) *DeleteTokens {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &DeleteTokens{now: now, items: map[string]deleteToken{}}
}

func canonicalSelect(sel Select) string {
	ids := append([]int64(nil), sel.IDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := ids[:0]
	for i, id := range ids {
		if i == 0 || id != ids[i-1] {
			out = append(out, id)
		}
	}
	return encodeJSON(Select{IDs: out, Filter: sel.Filter})
}

// Issue binds a 32-hex random token to project, the canonical select and data_rev.
func (d *DeleteTokens) Issue(projectID string, sel Select, dataRev int64) (string, time.Duration) {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	for k, v := range d.items {
		if now.After(v.expires) {
			delete(d.items, k)
		}
	}
	for len(d.items) >= maxDeleteTokens {
		oldest, first := "", true
		var oldestAt time.Time
		for k, v := range d.items {
			if first || v.expires.Before(oldestAt) {
				oldest, oldestAt, first = k, v.expires, false
			}
		}
		delete(d.items, oldest)
	}
	d.items[token] = deleteToken{project: projectID, sel: canonicalSelect(sel), dataRev: dataRev, expires: now.Add(DeleteTokenTTL)}
	return token, DeleteTokenTTL
}

// Consume reports whether token matches all three and has not expired; it removes the token either way.
func (d *DeleteTokens) Consume(token, projectID string, sel Select, dataRev int64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.items[token]
	if !ok {
		return false
	}
	delete(d.items, token)
	return !d.now().After(t.expires) && t.project == projectID && t.dataRev == dataRev && t.sel == canonicalSelect(sel)
}
