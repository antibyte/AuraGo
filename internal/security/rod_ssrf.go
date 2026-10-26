package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// OpenRodPageWithSSRF creates a blank page, intercepts every request through
// the SSRF-protected HTTP client, then navigates to rawURL. Chrome itself
// never dials the destination, so redirects and subresources cannot reach
// private or link-local addresses.
func OpenRodPageWithSSRF(browser *rod.Browser, rawURL string) (*rod.Page, error) {
	if err := ValidateSSRF(rawURL); err != nil {
		return nil, err
	}
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	if err := GuardRodPageSSRF(page); err != nil {
		_ = page.Close()
		return nil, err
	}
	if err := page.Navigate(rawURL); err != nil {
		_ = page.Close()
		return nil, err
	}
	return page, nil
}

// GuardRodPageSSRF routes every request of page through the SSRF-protected
// HTTP client. Chrome itself never dials the destination.
func GuardRodPageSSRF(page *rod.Page) error {
	return GuardRodPageWithOrigin(page, nil)
}

// GuardRodPageWithOrigin routes every request of page through Go. Requests to
// allowed (scheme, host and port) go through a client that can dial only that
// address, so the owner's loopback preview stays reachable; every other request
// goes through the SSRF-protected client. A nil allowed origin guards all
// requests.
func GuardRodPageWithOrigin(page *rod.Page, allowed *url.URL) error {
	var direct *http.Client
	if allowed != nil {
		client, err := newOriginPinnedHTTPClient(allowed, 20*time.Second)
		if err != nil {
			return err
		}
		direct = client
	}
	router := page.HijackRequests()
	if err := router.Add("*", "", func(h *rod.Hijack) {
		reqURL := h.Request.URL()
		if reqURL == nil {
			h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}
		client := direct
		if client == nil || !sameOrigin(reqURL, allowed) {
			protected, err := NewSSRFProtectedHTTPClientForURLContext(h.Request.Req().Context(), reqURL.String(), 20*time.Second)
			if err != nil {
				h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
				return
			}
			client = protected
		}
		res, err := client.Do(h.Request.Req())
		if err != nil {
			h.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
			return
		}
		defer func() { _ = res.Body.Close() }()
		body, err := ReadLimitedBody(res.Body, MaxSSRFResponseBytes)
		if err != nil {
			if err.Error() == "response exceeds 5 MiB" {
				h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
				return
			}
			h.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
			return
		}
		h.Response.Payload().ResponseCode = res.StatusCode
		for k, vs := range res.Header {
			for _, v := range vs {
				h.Response.SetHeader(k, v)
			}
		}
		h.Response.SetBody(body)
	}); err != nil {
		return err
	}
	go router.Run()
	return nil
}

// originHostPort returns host:port with the scheme's default port filled in.
func originHostPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "80"
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		}
	}
	return net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

func sameOrigin(u, origin *url.URL) bool {
	if u == nil || origin == nil {
		return false
	}
	return strings.EqualFold(u.Scheme, origin.Scheme) && originHostPort(u) == originHostPort(origin)
}

// newOriginPinnedHTTPClient returns a client that dials only the origin's
// host:port and hands redirects back to the caller (the browser), so every
// redirect target passes the hijack check again.
func newOriginPinnedHTTPClient(origin *url.URL, timeout time.Duration) (*http.Client, error) {
	if origin == nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Hostname() == "" {
		return nil, fmt.Errorf("allowed origin must be an http(s) URL with a host")
	}
	pinned := originHostPort(origin)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if !strings.EqualFold(addr, pinned) {
				return nil, fmt.Errorf("dial %s is outside the allowed origin", addr)
			}
			return dialer.DialContext(ctx, network, addr)
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}
