package chibisensei

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// PlanTokenTTL is how long a prepared start stays valid.
const PlanTokenTTL = 15 * time.Minute

const maxPlanTokens = 64

// PlanTicket binds a prepared start: project, the run ID shown in the dialog, the preset and the plan hash.
type PlanTicket struct {
	ProjectID, RunID, Preset, Hash string
	Expires                        time.Time
}

// PlanTokens holds plan tokens in memory (never persisted); at most 64, the earliest-expiring is evicted.
type PlanTokens struct {
	mu    sync.Mutex
	now   func() time.Time
	items map[string]PlanTicket
}

// NewPlanTokens creates the holder; now may be nil.
func NewPlanTokens(now func() time.Time) *PlanTokens {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &PlanTokens{now: now, items: map[string]PlanTicket{}}
}

// Issue stores t with Expires = now + PlanTokenTTL and returns a random 32-hex token and the TTL.
func (p *PlanTokens) Issue(t PlanTicket) (string, time.Duration) {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	for k, v := range p.items {
		if now.After(v.Expires) {
			delete(p.items, k)
		}
	}
	for len(p.items) >= maxPlanTokens {
		oldest, first := "", true
		var oldestAt time.Time
		for k, v := range p.items {
			if first || v.Expires.Before(oldestAt) {
				oldest, oldestAt, first = k, v.Expires, false
			}
		}
		delete(p.items, oldest)
	}
	t.Expires = now.Add(PlanTokenTTL)
	p.items[token] = t
	return token, PlanTokenTTL
}

// Consume returns the ticket when the token exists, has not expired and belongs to projectID. It always removes the
// token. The caller still compares Hash with a freshly computed PlanHash.
func (p *PlanTokens) Consume(token, projectID string) (PlanTicket, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ok := p.items[token]
	if !ok {
		return PlanTicket{}, false
	}
	delete(p.items, token)
	if p.now().After(t.Expires) || t.ProjectID != projectID {
		return PlanTicket{}, false
	}
	return t, true
}

// PlanHash is the sha256 (hex) over the canonical JSON of the plan, its data_rev, the profile SHA, the script SHA,
// the owner, the machine shape and the delete flag. Struct order makes the encoding deterministic.
func PlanHash(plan Plan, profileSHA, scriptSHA, owner, machineShape string, deleteAfterRun bool) string {
	payload := encodeJSON(struct {
		V              int    `json:"v"`
		Plan           Plan   `json:"plan"`
		DataRev        int64  `json:"data_rev"`
		ProfileSHA     string `json:"profile_sha"`
		ScriptSHA      string `json:"script_sha"`
		Owner          string `json:"owner"`
		MachineShape   string `json:"machine_shape"`
		DeleteAfterRun bool   `json:"delete_remote_after_run"`
	}{1, plan, plan.DataRev, profileSHA, scriptSHA, owner, machineShape, deleteAfterRun})
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
