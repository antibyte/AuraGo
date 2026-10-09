package retronet

import (
	"context"
	"io"
	"sync"
	"time"
)

const sessionReadBufferSize = 32 << 10

// sessionWriteTimeout bounds every write to the service, negotiation replies included, so a
// service that stops reading cannot block the pump. It is a variable only so tests can
// shorten it.
var sessionWriteTimeout = 10 * time.Second

// pumpHandlers are the protocol-specific steps of the byte pump. They run on the Run
// goroutine only, so protocol state needs no extra locking. remote must not keep p.
type pumpHandlers struct {
	remote func(p []byte) error // service output: answer negotiation, decode, forward to the browser
	input  func(p []byte) error // browser keystrokes: encode and write to the service
	resize func(s Size) error   // browser window size, already clamped
}

// serviceRead is one chunk read from the service, or the error that ended reading. data
// aliases the reader's buffer until the pump hands it back.
type serviceRead struct {
	data []byte
	err  error
}

// sessionWriter writes to the service under a write deadline (when available) and counts bytes.
type sessionWriter struct {
	w        io.Writer
	deadline interface{ SetWriteDeadline(time.Time) error }
	stats    *sessionStats
}

func (w sessionWriter) write(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if w.deadline != nil {
		_ = w.deadline.SetWriteDeadline(time.Now().Add(sessionWriteTimeout))
	}
	n, err := w.w.Write(p)
	w.stats.bytesOut += int64(n)
	return err
}

// pump moves bytes between src and the browser until the session ends and returns the
// reason. The reader goroutine reuses one buffer: it waits until the pump has handled a chunk
// before reading the next one. src is closed and the reader goroutine has exited when pump
// returns.
func (m *Manager) pump(ctx context.Context, client Client, src io.ReadCloser, stats *sessionStats, h pumpHandlers) string {
	readCtx, stopRead := context.WithCancel(ctx)
	reads := make(chan serviceRead)
	release := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Go(func() { readService(readCtx, src, reads, release) })
	defer func() {
		stopRead()
		_ = src.Close()
		wg.Wait()
	}()

	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return cancelReason(context.Cause(ctx))
		case r := <-reads:
			if r.err != nil {
				return endReason(ctx, ReasonRemoteClosed)
			}
			stats.bytesIn += int64(len(r.data))
			err := h.remote(r.data)
			release <- struct{}{}
			if err != nil {
				return endReason(ctx, ReasonRemoteClosed)
			}
		case ev, ok := <-events:
			if !ok {
				return ReasonRemoteClosed
			}
			if len(ev.Data) > 0 {
				if err := h.input(ev.Data); err != nil {
					return endReason(ctx, ReasonRemoteClosed)
				}
			}
			if ev.Resize != nil {
				if err := h.resize(clampSessionSize(*ev.Resize)); err != nil {
					return endReason(ctx, ReasonRemoteClosed)
				}
			}
		}
	}
}

// readService reads src into one reused buffer and hands every chunk to out; it reads again
// only after the pump signalled release. It returns when reading fails or ctx is done.
func readService(ctx context.Context, src io.Reader, out chan<- serviceRead, release <-chan struct{}) {
	buf := make([]byte, sessionReadBufferSize)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			select {
			case out <- serviceRead{data: buf[:n]}:
			case <-ctx.Done():
				return
			}
			select {
			case <-release:
			case <-ctx.Done():
				return
			}
		}
		if err != nil {
			select {
			case out <- serviceRead{err: err}:
			case <-ctx.Done():
			}
			return
		}
	}
}
