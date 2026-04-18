package tools

import (
	"bytes"
	"sync"
)

// BoundedBuffer is a thread-safe bytes.Buffer that prevents
// Out-Of-Memory errors by silently dropping bytes once it exceeds maxBytes.
// It is intended as a drop-in replacement for bytes.Buffer in os/exec scenarios.
type BoundedBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	maxBytes int
	dropped  bool
}

// NewBoundedBuffer creates a new buffer that accepts up to maxBytes.
func NewBoundedBuffer(maxBytes int) *BoundedBuffer {
	return &BoundedBuffer{
		maxBytes: maxBytes,
	}
}

// Write appends the contents of p to the buffer.
// If appending would exceed maxBytes, it drops the remainder
// and marks the buffer as having dropped data.
func (b *BoundedBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	currentLen := b.buf.Len()
	if currentLen >= b.maxBytes {
		b.dropped = true
		return len(p), nil // pretend we wrote it all to satisfy io.Writer
	}

	allowed := b.maxBytes - currentLen
	if len(p) > allowed {
		n, err = b.buf.Write(p[:allowed])
		b.dropped = true
		return len(p), err // pretend we wrote it all
	}

	return b.buf.Write(p)
}

// String returns the contents of the unread portion of the buffer
// as a string. If data was dropped, it appends a truncation warning.
func (b *BoundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := b.buf.String()
	if b.dropped {
		s += "\n...[OUTPUT TRUNCATED: exceeded limit]..."
	}
	return s
}

// Reset clears the buffer contents and dropped flag.
func (b *BoundedBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
	b.dropped = false
}
