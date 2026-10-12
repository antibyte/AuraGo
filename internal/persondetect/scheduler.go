package persondetect

import (
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
type frameSlots struct {
	mu     sync.Mutex
	order  []string
	frames map[string]frameJob
	next   int
}

func newFrameSlots() *frameSlots {
	return &frameSlots{frames: map[string]frameJob{}}
}

// setOrder sets the cameras in service order and drops frames of removed cameras.
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
	if fs.next >= len(fs.order) {
		fs.next = 0
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

// take returns the next camera's frame in round-robin order.
func (fs *frameSlots) take() (frameJob, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	n := len(fs.order)
	for i := 0; i < n; i++ {
		idx := (fs.next + i) % n
		id := fs.order[idx]
		if job, ok := fs.frames[id]; ok {
			delete(fs.frames, id)
			fs.next = (idx + 1) % n
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
// consecutive fetch errors: interval × 2^failures, at most MaxFetchBackoff.
func FetchBackoff(interval time.Duration, failures int) time.Duration {
	if interval <= 0 {
		interval = DefaultCameraInterval
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

// analysisRate counts the analyses of one camera within the last minute.
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

func (r *analysisRate) add(at time.Time) {
	r.prune(at)
	r.times = append(r.times, at)
}

func (r *analysisRate) perMinute(now time.Time) float64 {
	r.prune(now)
	return float64(len(r.times))
}
