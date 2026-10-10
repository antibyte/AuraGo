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

// sessionWriter writes to the service and counts bytes. Every write is bounded by
// sessionWriteTimeout: through a write deadline when the writer has one (a TCP connection),
// or through abort for writers that have none (an SSH channel blocks inside x/crypto while the
// server does not read). abort must unblock the write, normally by closing the connection.
type sessionWriter struct {
	w        io.Writer
	deadline interface{ SetWriteDeadline(time.Time) error }
	abort    func()
	stats    *sessionStats
}

func (w sessionWriter) write(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if w.deadline != nil {
		_ = w.deadline.SetWriteDeadline(time.Now().Add(sessionWriteTimeout))
	}
	if w.abort != nil {
		watchdog := time.AfterFunc(sessionWriteTimeout, w.abort)
		defer watchdog.Stop()
	}
	n, err := w.w.Write(p)
	w.stats.bytesOut += int64(n)
	return err
}

// bounded runs f, a call that writes to the service without going through write (an SSH
// window change), and calls abort when it takes longer than sessionWriteTimeout.
func (w sessionWriter) bounded(f func() error) error {
	if w.abort != nil {
		watchdog := time.AfterFunc(sessionWriteTimeout, w.abort)
		defer watchdog.Stop()
	}
	return f()
}

// pump moves bytes between src and the browser until the session ends and returns the
// reason. Only user input (Data events that are not just terminal reports) resets the idle
// timer. The reader goroutine reuses one buffer: it waits until the pump has handled a chunk
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

	idle := time.NewTimer(m.idleTimeout())
	defer idle.Stop()
	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return cancelReason(context.Cause(ctx))
		case <-idle.C:
			return ReasonIdle
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
				// A shutdown closes the browser socket too: an already cancelled ctx names
				// the real reason.
				return endReason(ctx, ReasonRemoteClosed)
			}
			// The pump is single-threaded on purpose: all protocol state lives on this goroutine
			// and needs no locks. The trade-off is that while a keystroke write to the service
			// blocks, nothing is read from the service either. That is bounded by
			// sessionWriteTimeout (10 s) for both protocols and by the size of one browser
			// message (the WebSocket read limit is 64 KiB): Telnet writes carry a write
			// deadline on the TCP connection, SSH writes and window changes are watched and
			// close the connection when they stall (sessionWriter.abort). A service that never
			// drains its socket or channel ends the session as remote_closed.
			if len(ev.Data) > 0 {
				// The browser answers some service queries on its own; those replies still go
				// to the service but are not user activity.
				if !terminalReportsOnly(ev.Data) {
					idle.Reset(m.idleTimeout())
				}
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

// terminalReportsOnly reports whether p, one browser message, consists only of replies the
// terminal emulator sends without the user: cursor position (ESC [ n ; n R), status
// (ESC [ n n), primary and secondary device attributes (ESC [ ? params c, ESC [ > params c),
// mode reports (ESC [ ? n ; n $ y, ESC [ n ; n $ y) and focus changes (ESC [ I, ESC [ O).
// Mouse reports and everything else are user input. A modified F3 key that xterm encodes like
// a cursor position report (ESC [ 1 ; 2 R) is indistinguishable and counts as a report.
func terminalReportsOnly(p []byte) bool {
	if len(p) == 0 {
		return false
	}
	for len(p) > 0 {
		n := terminalReportLen(p)
		if n == 0 {
			return false
		}
		p = p[n:]
	}
	return true
}

// terminalReportLen returns the length of the terminal report at the start of p, or 0.
func terminalReportLen(p []byte) int {
	if len(p) < 3 || p[0] != 0x1b || p[1] != '[' {
		return 0
	}
	if p[2] == 'I' || p[2] == 'O' {
		return 3
	}
	i := 2
	var marker byte
	if p[i] == '?' || p[i] == '>' {
		marker = p[i]
		i++
	}
	start := i
	for i < len(p) && (p[i] == ';' || ('0' <= p[i] && p[i] <= '9')) {
		i++
	}
	count, ok := reportParameterCount(p[start:i])
	if !ok || i >= len(p) {
		return 0
	}
	switch final := p[i]; {
	case final == 'R' && marker == 0 && count == 2, // cursor position
		final == 'n' && marker == 0 && count == 1, // status
		final == 'c' && marker != 0:               // device attributes
		return i + 1
	case final == '$' && marker != '>' && count == 2 && i+1 < len(p) && p[i+1] == 'y': // mode report
		return i + 2
	}
	return 0
}

// reportParameterCount counts the numbers in params ("12;40" -> 2). Empty input or an empty
// number is not a valid parameter list.
func reportParameterCount(params []byte) (int, bool) {
	if len(params) == 0 {
		return 0, false
	}
	count := 1
	for i, b := range params {
		if b != ';' {
			continue
		}
		if i == 0 || i == len(params)-1 || params[i-1] == ';' {
			return 0, false
		}
		count++
	}
	return count, true
}

// idleTimeout is how long a session may go without user input.
func (m *Manager) idleTimeout() time.Duration {
	if m.IdleTimeout > 0 {
		return m.IdleTimeout
	}
	return defaultIdleTimeout
}
