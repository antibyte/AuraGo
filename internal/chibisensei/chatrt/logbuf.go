package chatrt

import (
	"strings"
	"sync"
)

// logBuffer keeps the last limit bytes written to it; the child's stdout and stderr go here.
type logBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newLogBuffer(limit int) *logBuffer { return &logBuffer{limit: limit} }

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(p), nil
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}
