package security

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func SameHTTPOrigin(a, b *url.URL) bool {
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

// SameOriginRedirect binds all credentials, including custom headers, to the
// original integration origin. It never permits an HTTPS downgrade.
func SameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 || len(via) >= 10 || !SameHTTPOrigin(req.URL, via[0].URL) {
		return fmt.Errorf("cross-origin integration redirect rejected")
	}
	return nil
}
