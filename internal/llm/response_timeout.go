package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SSE needs an inactivity deadline; http.Client.Timeout instead cuts off even
// healthy long answers. Non-streaming requests retain their total deadline.
type responseTimeoutTransport struct {
	base    http.RoundTripper
	timeout time.Duration
}

func (t *responseTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	stream := req.Header.Get("Accept") == "text/event-stream"
	ctx, cancel := context.WithCancelCause(req.Context())
	kind := "request"
	if stream {
		kind = "stream inactivity"
	}
	timer := time.AfterFunc(t.timeout, func() {
		cancel(fmt.Errorf("LLM %s timeout after %s: %w", kind, t.timeout, context.DeadlineExceeded))
	})
	resp, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		timer.Stop()
		if ctx.Err() != nil {
			err = context.Cause(ctx)
		}
		cancel(context.Canceled)
		return nil, err
	}
	body := &responseTimeoutBody{ReadCloser: resp.Body, ctx: ctx, cancel: cancel, timer: timer}
	if stream && strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		body.idle = t.timeout
		timer.Reset(body.idle)
	}
	resp.Body = body
	return resp, nil
}

type responseTimeoutBody struct {
	io.ReadCloser
	ctx    context.Context
	cancel context.CancelCauseFunc
	timer  *time.Timer
	idle   time.Duration
	mu     sync.Mutex
	closed bool
}

func (b *responseTimeoutBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.mu.Lock()
	if err != nil {
		if b.ctx.Err() != nil {
			err = context.Cause(b.ctx)
		}
		b.timer.Stop()
		b.cancel(context.Canceled)
		b.closed = true
	} else if n > 0 && b.idle > 0 && !b.closed {
		b.timer.Reset(b.idle)
	}
	b.mu.Unlock()
	return n, err
}

func (b *responseTimeoutBody) Close() error {
	b.mu.Lock()
	b.closed = true
	b.timer.Stop()
	b.cancel(context.Canceled)
	b.mu.Unlock()
	return b.ReadCloser.Close()
}
