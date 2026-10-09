package localwiki

import "time"

// rateMeter averages download throughput over the last 20 seconds.
type rateMeter struct {
	samples []rateSample
	rate    int64 // result of the last accepted sample
}

type rateSample struct {
	at    time.Time
	bytes int64
}

const (
	rateWindow = 20 * time.Second
	// rateSampleInterval throttles the samples: the download reports progress
	// after every read, far more often than a rate needs.
	rateSampleInterval = 250 * time.Millisecond
)

// observe records the byte counter at now and returns bytes per second (0 until
// at least one second of samples exists). Observations less than
// rateSampleInterval after the last sample are not recorded; they return the
// last rate.
func (r *rateMeter) observe(now time.Time, bytes int64) int64 {
	if n := len(r.samples); n > 0 {
		last := r.samples[n-1]
		if bytes < last.bytes {
			r.reset()
		} else if now.Sub(last.at) < rateSampleInterval {
			return r.rate
		}
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
	r.rate = 0
	if elapsed >= 1 && bytes > first.bytes {
		r.rate = int64(float64(bytes-first.bytes) / elapsed)
	}
	return r.rate
}

func (r *rateMeter) reset() {
	r.samples = r.samples[:0]
	r.rate = 0
}
