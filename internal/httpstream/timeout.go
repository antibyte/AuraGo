// Package httpstream keeps active HTTP streams alive without unbounded writes.
package httpstream

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// WithWriteTimeout renews the write deadline for successful SSE and MJPEG
// responses before each write or flush. Other responses keep the server's
// absolute deadline. A handler deadline set through http.ResponseController can
// only shorten a stream's budget, never extend or clear it. Install outside
// response-writer wrappers so the controller can reach the transport; timeout
// must be positive.
func WithWriteTimeout(next http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		panic("httpstream: write timeout must be positive")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		next.ServeHTTP(&responseWriter{
			ResponseWriter: w,
			controller:     http.NewResponseController(w),
			ctx:            ctx,
			timeout:        timeout,
			cancel:         cancel,
		}, r.WithContext(ctx))
	})
}

type responseWriter struct {
	http.ResponseWriter
	controller  *http.ResponseController
	ctx         context.Context
	timeout     time.Duration
	cancel      context.CancelFunc
	wroteHeader bool
	streaming   bool
	err         error
	// handlerDeadline is the last deadline the handler requested; zero if none.
	handlerDeadline time.Time
}

func (w *responseWriter) WriteHeader(status int) {
	if !w.wroteHeader && status >= 200 {
		w.wroteHeader = true
		contentType, _, _ := strings.Cut(w.Header().Get("Content-Type"), ";")
		contentType = strings.ToLower(strings.TrimSpace(contentType))
		w.streaming = status < 300 && (contentType == "text/event-stream" || contentType == "multipart/x-mixed-replace" || strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/"))
	}
	if w.renewDeadline() == nil {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if err := w.renewDeadline(); err != nil {
		return 0, err
	}
	n, err := w.ResponseWriter.Write(p)
	return n, w.recordError(err)
}

func (w *responseWriter) FlushError() error {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if err := w.renewDeadline(); err != nil {
		return err
	}
	return w.recordError(w.controller.Flush())
}

func (w *responseWriter) Flush() {
	_ = w.FlushError()
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.controller.Hijack()
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) renewDeadline() error {
	if w.err != nil {
		return w.err
	}
	if !w.streaming {
		return nil
	}
	return w.applyDeadline(w.streamDeadline())
}

// SetWriteDeadline lets handlers tighten a stream's budget. ResponseController
// finds this method before Unwrap.
func (w *responseWriter) SetWriteDeadline(deadline time.Time) error {
	if w.err != nil {
		return w.err
	}
	w.handlerDeadline = deadline
	if w.streaming {
		return w.applyDeadline(w.streamDeadline())
	}
	if deadline.IsZero() {
		// Zero must never clear the bound on an ordinary response either.
		deadline = time.Now().Add(w.timeout)
	}
	return w.applyDeadline(deadline)
}

// streamDeadline returns the stricter of the stream budget and the handler deadline.
func (w *responseWriter) streamDeadline() time.Time {
	deadline := time.Now().Add(w.timeout)
	if !w.handlerDeadline.IsZero() && w.handlerDeadline.Before(deadline) {
		return w.handlerDeadline
	}
	return deadline
}

func (w *responseWriter) applyDeadline(deadline time.Time) error {
	if err := w.ctx.Err(); err != nil {
		return w.recordError(err)
	}
	err := w.controller.SetWriteDeadline(deadline)
	if ctxErr := w.ctx.Err(); ctxErr != nil {
		// The call above may have overwritten the shutdown drain's expired deadline.
		_ = w.controller.SetWriteDeadline(time.Now())
		return w.recordError(ctxErr)
	}
	if errors.Is(err, http.ErrNotSupported) {
		// Custom writers (e.g. httptest.ResponseRecorder) own their transport.
		return nil
	}
	return w.recordError(err)
}

func (w *responseWriter) recordError(err error) error {
	if err != nil {
		w.err = err
		// Also stop handlers that use http.Flusher and cannot see flush errors.
		w.cancel()
	}
	return err
}
