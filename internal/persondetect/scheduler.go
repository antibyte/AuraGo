package persondetect

import (
	"slices"
	"sync"
	"time"
)

// frameJob is a fetched frame waiting for analysis.
type frameJob struct {
	cameraID string
	jpeg     []byte
	at       time.Time
}

// frameSlots keeps the newest unanalyzed frame per camera and hands them out
// round-robin, so slow detection lowers the rate per camera instead of queueing.
// It is safe for concurrent use.
type frameSlots struct {
	mu      sync.Mutex
	order   []string
	frames  map[string]frameJob
	last    string // the camera served by the latest take; service continues after it
	hasLast bool
}

func newFrameSlots() *frameSlots {
	return &frameSlots{frames: map[string]frameJob{}}
}

// setOrder sets the cameras in service order and drops frames of removed cameras.
// The frames of the remaining cameras are kept. Service continues after the camera
// served last, wherever it is in the new order, so a reordering neither serves a
// camera twice in a row nor skips one; when that camera was removed, service starts
// from the first camera.
func (fs *frameSlots) setOrder(ids []string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.order = append([]string(nil), ids...)
	kept := make(map[string]frameJob, len(fs.frames))
	for _, id := range ids {
		if job, ok := fs.frames[id]; ok {
			kept[id] = job
		}
	}
	fs.frames = kept
	if fs.hasLast && !slices.Contains(fs.order, fs.last) {
		fs.last, fs.hasLast = "", false
	}
}

// put stores job as its camera's newest frame; false when the camera is unknown.
func (fs *frameSlots) put(job frameJob) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, id := range fs.order {
		if id == job.cameraID {
			fs.frames[id] = job
			return true
		}
	}
	return false
}

// take returns the frame of the next camera that has one, in round-robin order
// after the camera served last. A camera with a waiting frame is served after at
// most n-1 frames of other cameras.
func (fs *frameSlots) take() (frameJob, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	n := len(fs.order)
	start := 0
	if fs.hasLast {
		if i := slices.Index(fs.order, fs.last); i >= 0 {
			start = (i + 1) % n
		}
	}
	for i := 0; i < n; i++ {
		id := fs.order[(start+i)%n]
		if job, ok := fs.frames[id]; ok {
			delete(fs.frames, id)
			fs.last, fs.hasLast = id, true
			return job, true
		}
	}
	return frameJob{}, false
}

// queued is the number of cameras with a waiting frame.
func (fs *frameSlots) queued() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.frames)
}

// FetchBackoff is the delay before the next frame request after failures
// consecutive fetch errors: interval × 2^failures, at most MaxFetchBackoff. An
// interval of MaxFetchBackoff or more (config validation allows at most 30 s) is
// returned unchanged: the backoff never shortens it, and nothing is doubled, so it
// cannot overflow.
func FetchBackoff(interval time.Duration, failures int) time.Duration {
	if interval <= 0 {
		interval = DefaultCameraInterval
	}
	if interval >= MaxFetchBackoff {
		return interval
	}
	delay := interval
	for i := 0; i < failures; i++ {
		delay *= 2
		if delay >= MaxFetchBackoff {
			return MaxFetchBackoff
		}
	}
	return delay
}

// timingAverage is the moving average of the last TimingAverageWindow detections.
// It is not safe for concurrent use: the service keeps it under Service.mu.
type timingAverage struct {
	wall, cpu [TimingAverageWindow]float64
	n, next   int
}

func (a *timingAverage) add(wall, cpu float64) {
	a.wall[a.next], a.cpu[a.next] = wall, cpu
	a.next = (a.next + 1) % TimingAverageWindow
	if a.n < TimingAverageWindow {
		a.n++
	}
}

func (a *timingAverage) average() (float64, float64) {
	if a.n == 0 {
		return 0, 0
	}
	var wall, cpu float64
	for i := 0; i < a.n; i++ {
		wall += a.wall[i]
		cpu += a.cpu[i]
	}
	return wall / float64(a.n), cpu / float64(a.n)
}

// analysisRate counts the analyses of one camera within the last minute. It is not
// safe for concurrent use, and perMinute modifies it as well (it prunes): the
// service holds Service.mu for every call, including reads.
type analysisRate struct {
	times []time.Time
}

func (r *analysisRate) prune(now time.Time) {
	cutoff := now.Add(-time.Minute)
	kept := r.times[:0]
	for _, t := range r.times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	r.times = kept
}

// add records an analysis at the given time. The analyses of one camera arrive in
// time order, so an analysis that precedes the latest one means the wall clock
// stepped back: the earlier entries belong to the time before the step and are
// dropped instead of being counted as recent until the clock catches up.
func (r *analysisRate) add(at time.Time) {
	if n := len(r.times); n > 0 && at.Before(r.times[n-1]) {
		r.times = r.times[:0]
	}
	r.prune(at)
	r.times = append(r.times, at)
}

func (r *analysisRate) perMinute(now time.Time) float64 {
	r.prune(now)
	return float64(len(r.times))
}
