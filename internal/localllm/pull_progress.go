package localllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"aurago/internal/dockerutil"
)

// imagePullBaseline is the time every runtime image pull gets, exactly as the
// fixed two-hour pull timeout gave it before: no stall rule applies inside it.
// After it, the pull continues for as long as the Engine reports new progress
// and fails once a whole imagePullStallWindow passes without any. The install
// context (6 h) and desired-state cancellation still end a pull at any time.
// Tests shorten both.
var (
	imagePullBaseline    = 2 * time.Hour
	imagePullStallWindow = 15 * time.Minute
)

// pullProgressStateLimit bounds the per-layer state kept for one pull.
const pullProgressStateLimit = 1024

// imagePullStalledError ends a pull that made no progress for a whole window
// after the baseline. It never wraps context.Canceled.
type imagePullStalledError struct {
	baseline, window time.Duration
}

func (e *imagePullStalledError) Error() string {
	return fmt.Sprintf("no new pull progress from Docker for %s after the first %s", e.window, e.baseline)
}

// pullProgress records when the Engine last reported new pull progress. A
// decoded message counts when its (status, progressDetail.current,
// progressDetail.total, progress) differs from the last message for the same
// layer id, or names an id not seen before. Identical repeats (an Engine that
// re-sends unchanged layer states while the registry stalls), non-JSON lines
// and raw bytes do not count.
type pullProgress struct {
	mu      sync.Mutex
	last    time.Time
	states  map[string]string
	partial []byte
	skip    bool // the current line exceeded MaxJSONMessageLine; ignore it up to its newline
}

func newPullProgress(start time.Time) *pullProgress {
	return &pullProgress{last: start, states: make(map[string]string)}
}

func (p *pullProgress) lastProgress() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

// observe feeds bytes read from the stream; complete lines are classified.
func (p *pullProgress) observe(data []byte, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(data) > 0 {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			if !p.skip {
				if len(p.partial)+len(data) > dockerutil.MaxJSONMessageLine {
					p.partial, p.skip = p.partial[:0], true
				} else {
					p.partial = append(p.partial, data...)
				}
			}
			return
		}
		if !p.skip && len(p.partial)+end <= dockerutil.MaxJSONMessageLine {
			p.partial = append(p.partial, data[:end]...)
			p.classify(p.partial, now)
		}
		p.partial, p.skip = p.partial[:0], false
		data = data[end+1:]
	}
}

func (p *pullProgress) classify(line []byte, now time.Time) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var event struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Progress string `json:"progress"`
		Detail   struct {
			Current int64 `json:"current"`
			Total   int64 `json:"total"`
		} `json:"progressDetail"`
	}
	if json.Unmarshal(line, &event) != nil {
		return
	}
	state := event.Status + "\x00" + strconv.FormatInt(event.Detail.Current, 10) + "\x00" +
		strconv.FormatInt(event.Detail.Total, 10) + "\x00" + event.Progress
	previous, known := p.states[event.ID]
	if known && previous == state {
		return
	}
	if !known && len(p.states) >= pullProgressStateLimit {
		clear(p.states)
	}
	p.states[event.ID] = state
	p.last = now
}

// pullProgressReader passes the pull stream through and records progress.
type pullProgressReader struct {
	r        io.Reader
	progress *pullProgress
}

func (r *pullProgressReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	if n > 0 {
		r.progress.observe(b[:n], time.Now())
	}
	return n, err
}

// watchPullProgress cancels the pull with an *imagePullStalledError once the
// baseline has passed and the Engine has reported no new progress for a whole
// window. It never fires inside the baseline. The returned function stops the
// watchdog and waits for it.
func watchPullProgress(ctx context.Context, cancel context.CancelCauseFunc, progress *pullProgress, baseline, window time.Duration) func() {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		timer := time.NewTimer(baseline)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			idle := time.Since(progress.lastProgress())
			if idle >= window {
				cancel(&imagePullStalledError{baseline: baseline, window: window})
				return
			}
			timer.Reset(window - idle)
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}
