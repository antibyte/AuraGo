package fritzbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"aurago/internal/security"
)

// boundTransport keeps every request, including response reads, within the
// client's lifetime and its two administrator-configured router origins.
type boundTransport struct {
	parent  context.Context
	base    http.RoundTripper
	origins []*url.URL
}

func (t *boundTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	allowed := false
	for _, origin := range t.origins {
		allowed = allowed || security.SameHTTPOrigin(req.URL, origin)
	}
	if !allowed {
		return nil, fmt.Errorf("Fritz!Box request outside configured router origins")
	}
	security.RegisterSensitive(req.URL.Query().Get("sid"))
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(t.parent, cancel)
	if t.parent.Err() != nil {
		cancel()
	}
	cleanup := func() { stop(); cancel() }
	resp, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		cleanup()
		return nil, scrubFritzError(err)
	}
	resp.Body = &lifetimeBody{ReadCloser: resp.Body, cleanup: cleanup}
	return resp, nil
}

func (t *boundTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type lifetimeBody struct {
	io.ReadCloser
	cleanup func()
}

func (b *lifetimeBody) Close() error { defer b.cleanup(); return b.ReadCloser.Close() }

type scrubbedFritzError struct{ err error }

func (e scrubbedFritzError) Error() string {
	var network *url.Error
	if errors.As(e.err, &network) {
		target, err := url.Parse(network.URL)
		if err != nil {
			return security.Scrub(network.Op + ": " + network.Err.Error())
		}
		target.User = nil
		target.RawQuery = ""
		target.Fragment = ""
		return security.Scrub(network.Op + " " + target.String() + ": " + network.Err.Error())
	}
	return security.Scrub(e.err.Error())
}
func (e scrubbedFritzError) Unwrap() error { return e.err }
func scrubFritzError(err error) error {
	if err == nil {
		return nil
	}
	return scrubbedFritzError{err}
}
