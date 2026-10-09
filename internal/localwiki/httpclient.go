package localwiki

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errInsecureRedirect = errors.New("localwiki: redirect to a non-HTTPS URL refused")

// httpsOnlyRedirect follows at most ten redirects and only to HTTPS URLs
// without credentials, so no mirror can downgrade a download to HTTP.
func httpsOnlyRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("localwiki: too many redirects")
	}
	if req.URL.Scheme != "https" || req.URL.User != nil {
		return errInsecureRedirect
	}
	return nil
}

// newHTTPClient returns a copy of base with the HTTPS-only redirect policy, or
// a default client without an overall timeout: downloads take hours, so every
// request carries its own deadline or stall watchdog instead.
func newHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSHandshakeTimeout = 15 * time.Second
		transport.ResponseHeaderTimeout = 60 * time.Second
		return &http.Client{Transport: transport, CheckRedirect: httpsOnlyRedirect}
	}
	client := *base
	client.CheckRedirect = httpsOnlyRedirect
	return &client
}

// requireHTTPS accepts absolute https URLs without user information.
func requireHTTPS(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("localwiki: %q is not an https URL", raw)
	}
	return parsed, nil
}

// meta4HostAllowed limits edition metadata to Kiwix hosts or the configured
// catalog host (tests run a local catalog).
func meta4HostAllowed(target, catalog *url.URL) bool {
	if catalog != nil && strings.EqualFold(target.Host, catalog.Host) {
		return true
	}
	host := strings.ToLower(target.Hostname())
	return host == "kiwix.org" || strings.HasSuffix(host, ".kiwix.org")
}

func hostOf(raw string) string {
	if parsed, err := url.Parse(raw); err == nil {
		return parsed.Host
	}
	return ""
}
