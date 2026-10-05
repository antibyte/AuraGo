// Package httporigin is the house redirect policy for credentialed HTTP
// clients: a request that carries a key, token or custom auth header never
// follows a redirect off its exact scheme/host/effective-port origin.
//
// It is a leaf package (no aurago imports) so that both internal/security,
// which imports internal/llm, and internal/llm itself can use it.
package httporigin

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SameOrigin reports whether a and b share scheme, host (case-insensitive) and
// effective port. URLs carrying userinfo never match.
func SameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil || a.User != nil || b.User != nil {
		return false
	}
	port := func(u *url.URL) string {
		if u.Port() != "" {
			return u.Port()
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// SameOriginRedirect is an http.Client CheckRedirect policy that binds all
// credentials, including custom headers, to the original request's origin.
// It never permits an HTTPS downgrade and stops after 10 redirects.
func SameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 || len(via) >= 10 || !SameOrigin(req.URL, via[0].URL) {
		return fmt.Errorf("cross-origin integration redirect rejected")
	}
	return nil
}
