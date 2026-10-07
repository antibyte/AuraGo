package security

import (
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"aurago/internal/httporigin"
)

// The fixed list plus headers caught by name: Google's API-key header (Veo,
// Lyria), a webhook-configured token header and an arbitrary secret header.
var ssrfRedirectCredentialHeaders = []string{
	"Authorization", "Proxy-Authorization", "Cookie", "X-Api-Key", "X-Auth-Token",
	"X-Goog-Api-Key", "X-Webhook-Token", "My-Secret",
}

var ssrfRedirectPlainHeaders = []string{"Accept", "Content-Type"}

func ssrfRedirectWithCredentials(t *testing.T, target string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range ssrfRedirectCredentialHeaders {
		req.Header.Set(name, "fixture-credential")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return req
}

// The SSRF clients follow redirects to other public origins. A hop that leaves
// the original scheme/host/port origin must not carry the caller's
// credentials, including custom API-key headers net/http keeps on its own.
// Loopback origins on different ports keep the fixture free of DNS.
func TestSSRFClientStripsCredentialHeadersAcrossOrigins(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	forURL, err := NewSSRFProtectedHTTPClientForURL("http://localhost:18081/start", time.Second)
	if err != nil {
		t.Fatalf("NewSSRFProtectedHTTPClientForURL: %v", err)
	}
	for _, tc := range []struct {
		name, origin, cross, same string
		client                    *http.Client
	}{
		{"NewSSRFProtectedHTTPClient", "http://127.0.0.1:18081/start", "http://127.0.0.1:18082/next", "http://127.0.0.1:18081/next", NewSSRFProtectedHTTPClient(time.Second)},
		{"NewSSRFProtectedHTTPClientForURL", "http://localhost:18081/start", "http://localhost:18082/next", "http://localhost:18081/next", forURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original, err := http.NewRequest(http.MethodGet, tc.origin, nil)
			if err != nil {
				t.Fatal(err)
			}

			cross := ssrfRedirectWithCredentials(t, tc.cross)
			if err := tc.client.CheckRedirect(cross, []*http.Request{original}); err != nil {
				t.Fatalf("cross-origin redirect rejected: %v", err)
			}
			for _, name := range ssrfRedirectCredentialHeaders {
				if got := cross.Header.Get(name); got != "" {
					t.Errorf("cross-origin redirect kept %s = %q", name, got)
				}
			}
			for _, name := range ssrfRedirectPlainHeaders {
				if cross.Header.Get(name) != "application/json" {
					t.Errorf("cross-origin redirect dropped the non-credential header %s", name)
				}
			}

			same := ssrfRedirectWithCredentials(t, tc.same)
			if err := tc.client.CheckRedirect(same, []*http.Request{original}); err != nil {
				t.Fatalf("same-origin redirect rejected: %v", err)
			}
			for _, name := range slices.Concat(ssrfRedirectCredentialHeaders, ssrfRedirectPlainHeaders) {
				if same.Header.Get(name) == "" {
					t.Errorf("same-origin redirect dropped %s", name)
				}
			}
		})
	}

	// The credentialed variant still refuses the cross-origin hop outright.
	original, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:18081/start", nil)
	cross := ssrfRedirectWithCredentials(t, "http://127.0.0.1:18082/next")
	if err := NewSSRFProtectedHTTPClientSameOrigin(time.Second).CheckRedirect(cross, []*http.Request{original}); !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("same-origin client: error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
}
