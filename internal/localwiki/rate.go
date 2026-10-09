package localwiki

import "time"

// rateMeter averages download throughput over the last 20 seconds.
type rateMeter struct {
	samples []rateSample
}

type rateSample struct {
	at    time.Time
	bytes int64
}

const rateWindow = 20 * time.Second

// observe records the byte counter at now and returns bytes per second (0 until
// at least one second of samples exists).
func (r *rateMeter) observe(now time.Time, bytes int64) int64 {
	if n := len(r.samples); n > 0 && bytes < r.samples[n-1].bytes {
		r.reset()
	}
	r.samples = append(r.samples, rateSample{at: now, bytes: bytes})
	// Keep the newest sample older than the window as the anchor, so slow
	// downloads (one sample per MiB) still report a rate.
	cutoff := now.Add(-rateWindow)
	drop := 0
	for drop < len(r.samples)-1 && r.samples[drop+1].at.Before(cutoff) {
		drop++
	}
	r.samples = r.samples[drop:]
	first := r.samples[0]
	elapsed := now.Sub(first.at).Seconds()
	if elapsed < 1 || bytes <= first.bytes {
		return 0
	}
	return int64(float64(bytes-first.bytes) / elapsed)
}

func (r *rateMeter) reset() {
	r.samples = r.samples[:0]
}
