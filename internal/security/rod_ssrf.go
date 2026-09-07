package security

import (
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

// GuardRodPageSSRF hijacks page requests and fulfills them with
// NewSSRFProtectedHTTPClientForURL so Chromium cannot follow a public URL
// onto a private host.
func GuardRodPageSSRF(page *rod.Page) error {
	router := page.HijackRequests()
	if err := router.Add("*", "", func(h *rod.Hijack) {
		reqURL := h.Request.URL()
		if reqURL == nil {
			h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}
		client, err := NewSSRFProtectedHTTPClientForURL(reqURL.String(), 20*time.Second)
		if err != nil {
			h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}
		if err := h.LoadResponse(client, true); err != nil {
			h.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
		}
	}); err != nil {
		return err
	}
	go router.Run()
	return nil
}
