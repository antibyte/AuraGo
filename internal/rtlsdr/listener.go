package rtlsdr

import (
	"context"
	"io"
	"sync"
	"time"
)

type listenerLease struct {
	lastHeartbeat time.Time
	ctx           context.Context
	cancel        context.CancelFunc
}

// Each body retains its original lease, even if the client later tunes again.
type leasedStream struct {
	reader    io.ReadCloser
	ctx       context.Context
	cancel    context.CancelFunc
	stopLease func() bool
	once      sync.Once
	err       error
}

func (s *leasedStream) Read(p []byte) (int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	return s.reader.Read(p)
}
func (s *leasedStream) Close() error {
	s.once.Do(func() { s.stopLease(); s.cancel(); s.err = s.reader.Close() })
	return s.err
}
