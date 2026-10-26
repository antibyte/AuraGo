package security

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

func TestSameOriginComparesSchemeHostAndDefaultPort(t *testing.T) {
	origin := mustParseURL(t, "http://127.0.0.1:8080")
	cases := map[string]bool{
		"http://127.0.0.1:8080/about?x=1": true,
		"http://127.0.0.1:8081/":          false,
		"https://127.0.0.1:8080/":         false,
		"http://localhost:8080/":          false,
	}
	for raw, want := range cases {
		if got := sameOrigin(mustParseURL(t, raw), origin); got != want {
			t.Errorf("sameOrigin(%q) = %v, want %v", raw, got, want)
		}
	}
	if !sameOrigin(mustParseURL(t, "https://example.com:443/x"), mustParseURL(t, "https://Example.com")) {
		t.Error("default https port and host case must not matter")
	}
	if sameOrigin(mustParseURL(t, "http://127.0.0.1:8080/"), nil) {
		t.Error("a nil origin matches nothing")
	}
}

func TestOriginPinnedClientDialsOnlyTheAllowedOrigin(t *testing.T) {
	allowed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer allowed.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer other.Close()

	client, err := newOriginPinnedHTTPClient(mustParseURL(t, allowed.URL), 5*time.Second)
	if err != nil {
		t.Fatalf("newOriginPinnedHTTPClient: %v", err)
	}
	res, err := client.Get(allowed.URL + "/page")
	if err != nil {
		t.Fatalf("allowed origin: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("allowed origin status = %d", res.StatusCode)
	}
	if res, err := client.Get(other.URL + "/"); err == nil {
		_ = res.Body.Close()
		t.Fatal("the pinned client reached another origin")
	}
}

func TestOriginPinnedClientLeavesRedirectsToTheBrowser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest", http.StatusFound)
	}))
	defer srv.Close()
	client, err := newOriginPinnedHTTPClient(mustParseURL(t, srv.URL), 5*time.Second)
	if err != nil {
		t.Fatalf("newOriginPinnedHTTPClient: %v", err)
	}
	res, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") == "" {
		t.Fatalf("redirect was followed by Go: status %d", res.StatusCode)
	}
}

func TestNewOriginPinnedClientRejectsNonHTTPOrigins(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com", "http://"} {
		if _, err := newOriginPinnedHTTPClient(mustParseURL(t, raw), time.Second); err == nil {
			t.Errorf("origin %q was accepted", raw)
		}
	}
}
