package pdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Limits apply to one reader, including every nested stream and font.
const (
	MaxInputBytes    int64 = 32 << 20
	MaxPages               = 1000
	MaxStreamBytes   int64 = 16 << 20
	MaxDecodedBytes  int64 = 128 << 20
	MaxTextBytes           = 8 << 20
	maxObjectEntries int64 = 1 << 20
)

var ErrLimit = errors.New("PDF processing limit exceeded")

type readSafety struct {
	ctx                context.Context
	deadline           time.Time
	decoded, allocated int64
	depth              int
	err                error
}

func newSafety(ctx context.Context) *readSafety {
	if ctx == nil {
		ctx = context.Background()
	}
	return &readSafety{ctx: ctx, deadline: time.Now().Add(30 * time.Second)}
}

func (s *readSafety) fail(err error) {
	if s != nil && s.err == nil {
		s.err = err
	}
	panic(err)
}

func (s *readSafety) check() {
	if s == nil {
		return
	}
	if s.err != nil {
		panic(s.err)
	}
	if err := s.ctx.Err(); err != nil {
		s.fail(err)
	}
	if time.Now().After(s.deadline) {
		s.fail(context.DeadlineExceeded)
	}
}

func (s *readSafety) reserve(n, unit int64) {
	s.check()
	if n < 0 || n > maxObjectEntries || unit < 1 || n > MaxDecodedBytes/unit {
		s.fail(fmt.Errorf("%w: allocation size", ErrLimit))
	}
	if s != nil {
		amount := n * unit
		if amount > MaxDecodedBytes-s.allocated {
			s.fail(fmt.Errorf("%w: aggregate allocations", ErrLimit))
		}
		s.allocated += amount
	}
}

func (r *Reader) safetyState() *readSafety {
	if r == nil {
		return nil
	}
	return r.safety
}

// Err retains a terminal limit or cancellation even when a malformed page was recovered.
func (r *Reader) Err() error {
	if r == nil || r.safety == nil {
		return nil
	}
	return r.safety.err
}

type boundedStream struct {
	reader io.Reader
	safety *readSafety
	read   int64
}

func (b *boundedStream) Read(p []byte) (int, error) {
	b.safety.check()
	remaining := MaxStreamBytes - b.read
	if b.safety != nil {
		remaining = min(remaining, MaxDecodedBytes-b.safety.decoded)
	}
	if remaining < 0 {
		b.safety.fail(fmt.Errorf("%w: decoded stream bytes", ErrLimit))
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := b.reader.Read(p)
	b.safety.check()
	// Nested decoders may have consumed the shared allowance during Read.
	if b.safety != nil {
		remaining = min(remaining, MaxDecodedBytes-b.safety.decoded)
	}
	if int64(n) > remaining {
		b.safety.fail(fmt.Errorf("%w: decoded stream bytes", ErrLimit))
	}
	b.read += int64(n)
	if b.safety != nil {
		b.safety.decoded += int64(n)
	}
	return n, err
}
