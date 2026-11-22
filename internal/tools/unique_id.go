package tools

import (
	"strconv"
	"sync/atomic"
	"time"
)

// freeNanoID returns prefix plus the first decimal value at or after
// now.UnixNano() for which taken reports false. Generated IDs keep their
// timestamp shape, but a clock that repeats between back-to-back calls (common
// on Windows) can no longer hand out an ID that is still in use. The caller
// must hold the lock that guards the collection taken inspects and insert the
// returned ID before releasing it.
func freeNanoID(prefix string, now time.Time, taken func(id string) bool) string {
	for n := now.UnixNano(); ; n++ {
		id := prefix + strconv.FormatInt(n, 10)
		if !taken(id) {
			return id
		}
	}
}

var lastProcessUniqueNano atomic.Int64

// processUniqueNano returns now.UnixNano(), raised above every value it has
// returned before in this process. Use it for IDs written to stores that
// upsert by ID and offer no in-memory index for freeNanoID to check.
func processUniqueNano(now time.Time) int64 {
	for {
		last := lastProcessUniqueNano.Load()
		next := now.UnixNano()
		if next <= last {
			next = last + 1
		}
		if lastProcessUniqueNano.CompareAndSwap(last, next) {
			return next
		}
	}
}
