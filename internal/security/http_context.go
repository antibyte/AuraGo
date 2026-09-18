package security

import (
	"context"
	"io"
	"net/http"
)

// HTTPClientWithContext keeps the client's redirect and SSRF policy while
// binding libraries that create their own requests to their caller's lifetime.
func HTTPClientWithContext(client *http.Client, ctx context.Context) *http.Client {
	copy := *client
	transport := copy.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	copy.Transport = contextTransport{ctx: ctx, next: transport}
	return &copy
}

type contextBody struct {
	io.ReadCloser
	close func()
}

func (b *contextBody) Close() error { defer b.close(); return b.ReadCloser.Close() }

type contextTransport struct {
	ctx  context.Context
	next http.RoundTripper
}

func (t contextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	if t.ctx.Err() != nil {
		cancel()
	}
	response, err := t.next.RoundTrip(r.Clone(ctx))
	if err != nil {
		stop()
		cancel()
		return nil, err
	}
	response.Body = &contextBody{ReadCloser: response.Body, close: func() { stop(); cancel() }}
	return response, nil
}
