package chibisensei

import (
	"context"
	"path/filepath"
	"time"
)

// smokeTimeout bounds one smoke generation, including a cold model start.
const smokeTimeout = 3 * time.Minute

// SmokeInput names the tuned model the smoke asks for one short answer.
type SmokeInput struct {
	RunID      string
	ModelPath  string
	ModelKey   string
	ModelBytes int64
	Profile    *Profile
}

// SmokeResult is the stored outcome of the smoke generation.
type SmokeResult struct {
	State string `json:"state"`          // ok | failed | skipped
	Code  string `json:"code,omitempty"` // runtime_not_installed | runtime_busy | runtime_disabled | start_failed | generation_failed
	MS    int64  `json:"ms,omitempty"`
	Text  string `json:"text,omitempty"` // at most 200 characters, scrubbed
}

// Smoker runs the short generation. It must not download anything.
type Smoker interface {
	Smoke(ctx context.Context, in SmokeInput) SmokeResult
}

// SetSmoker sets the smoker (nil = no smoke); call it before Start.
func (o *Orchestrator) SetSmoker(s Smoker) {
	o.smokeMu.Lock()
	o.smoker = s
	o.smokeMu.Unlock()
}

// abortSmoke ends the running smoke of runID; Cancel calls it after its commit.
func (o *Orchestrator) abortSmoke(runID string) {
	o.smokeMu.Lock()
	defer o.smokeMu.Unlock()
	if o.smokeRun == runID && o.smokeCancel != nil {
		o.smokeAborted = true
		o.smokeCancel()
	}
}

// runSmoke asks the smoker once. res is nil without a smoker, aborted reports a Cancel during the smoke (nothing
// is recorded) and err is the loop context's error when the shutdown interrupted it.
func (o *Orchestrator) runSmoke(ctx context.Context, r Run, prof *Profile, rel string, bytes int64) (res *SmokeResult, aborted bool, err error) {
	o.smokeMu.Lock()
	sm := o.smoker
	if sm == nil {
		o.smokeMu.Unlock()
		return nil, false, nil
	}
	sctx, cancel := context.WithTimeout(ctx, smokeTimeout)
	defer cancel()
	o.smokeRun, o.smokeCancel, o.smokeAborted = r.ID, cancel, false
	o.smokeMu.Unlock()

	out := sm.Smoke(sctx, SmokeInput{RunID: r.ID, ModelPath: filepath.Join(ChibiDir(o.dataDir), filepath.FromSlash(rel)),
		ModelKey: "tuned:" + r.ID, ModelBytes: bytes, Profile: prof})

	o.smokeMu.Lock()
	aborted = o.smokeAborted
	o.smokeRun, o.smokeCancel, o.smokeAborted = "", nil, false
	o.smokeMu.Unlock()

	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	if aborted {
		return nil, true, nil
	}
	switch out.State {
	case "ok", "failed", "skipped":
	default:
		out.State, out.Code = "failed", "generation_failed"
	}
	out.Code = cutRunes(out.Code, 40)
	out.MS = max(out.MS, 0)
	out.Text = scrubText(out.Text, 200)
	return &out, false, nil
}
