package security

import (
	"fmt"
	"net/http"
	"net/url"

	"aurago/internal/httporigin"
)

// ValidateHTTPBaseURL validates an administrator-selected integration endpoint.
// It does not grant public-only egress; callers needing that use the pinned
// public transport in addition to this syntactic boundary.
func ValidateHTTPBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("integration endpoint must be an absolute HTTP(S) URL without credentials, query or fragment")
	}
	return nil
}

// SameHTTPOrigin reports whether a and b share scheme, host and effective
// port. The policy lives in the leaf package httporigin so internal/llm can
// share it without importing internal/security.
func SameHTTPOrigin(a, b *url.URL) bool {
	return httporigin.SameOrigin(a, b)
}

// SameOriginRedirect binds all credentials, including custom headers, to the
// original integration origin. It never permits an HTTPS downgrade.
func SameOriginRedirect(req *http.Request, via []*http.Request) error {
	return httporigin.SameOriginRedirect(req, via)
}
