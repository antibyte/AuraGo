package llm

import (
	"fmt"
	"net/http"
)

type providerURLTransport struct{ base http.RoundTripper }

func (t *providerURLTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil || req.URL.User != nil {
		return nil, fmt.Errorf("provider URLs must not contain embedded credentials")
	}
	return t.base.RoundTrip(req)
}
