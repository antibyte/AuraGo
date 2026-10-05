package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// c13HeaderRecorder is a server that records the headers of every request it gets.
type c13HeaderRecorder struct {
	mu      sync.Mutex
	headers []http.Header
}

func (r *c13HeaderRecorder) record(h http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.headers = append(r.headers, h.Clone())
}

func (r *c13HeaderRecorder) last(t *testing.T) http.Header {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.headers) == 0 {
		t.Fatal("the target got no request")
	}
	return r.headers[len(r.headers)-1]
}

func c13APIRequest(t *testing.T, url string, headers map[string]string) APIResult {
	t.Helper()
	var out APIResult
	got := ExecuteAPIRequestWithOptions(http.MethodPost, url, `{"a":1}`, headers, APIRequestOptions{})
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("decode %s: %v", got, err)
	}
	return out
}

// 1c-13 B8: a redirect to another origin must not carry the caller's credentials: neither
// Authorization (net/http keeps it for the same host on another port) nor custom headers
// such as X-API-Key (net/http keeps those everywhere).
func TestAPIRequestRedirectToAnotherOriginDropsCallerHeaders(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	other := &c13HeaderRecorder{}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		other.record(r.Header)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	origin := &c13HeaderRecorder{}
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin.record(r.Header)
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, target.URL+"/landing", http.StatusTemporaryRedirect)
		case "/here":
			http.Redirect(w, r, "/landing", http.StatusTemporaryRedirect)
		case "/away-and-back":
			http.Redirect(w, r, target.URL+"/bounce", http.StatusFound)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer start.Close()
	headers := map[string]string{"Authorization": "Bearer c13-secret", "X-API-Key": "c13-key", "x-custom-auth": "c13-custom",
		"Accept": "application/json", "Content-Type": "application/json"}

	if out := c13APIRequest(t, start.URL+"/away", headers); out.Status != "success" {
		t.Fatalf("redirected request = %+v", out)
	}
	if first := origin.last(t); first.Get("Authorization") != "Bearer c13-secret" || first.Get("X-Api-Key") != "c13-key" {
		t.Fatalf("the first request lost its headers: %v", first)
	}
	got := other.last(t)
	for _, name := range []string{"Authorization", "X-Api-Key", "X-Custom-Auth", "Cookie"} {
		if v := got.Get(name); v != "" {
			t.Errorf("the other origin got %s: %q", name, v)
		}
	}
	if got.Get("Accept") != "application/json" || got.Get("Content-Type") != "application/json" || got.Get("User-Agent") != "AuraGo-Agent/1.0" {
		t.Errorf("the other origin lost a kept header: %v", got)
	}

	// The same origin keeps them.
	if out := c13APIRequest(t, start.URL+"/here", headers); out.Status != "success" {
		t.Fatalf("same-origin redirect = %+v", out)
	}
	if same := origin.last(t); same.Get("Authorization") != "Bearer c13-secret" || same.Get("X-Api-Key") != "c13-key" || same.Get("X-Custom-Auth") != "c13-custom" {
		t.Errorf("a same-origin redirect lost the caller's headers: %v", same)
	}
}

// 1c-13 review I3: which redirects keep the caller's headers.
func TestAPIRedirectKeepsHeadersRule(t *testing.T) {
	cases := []struct {
		from, to string
		keep     bool
	}{
		{"https://api.example.com/a", "https://api.example.com/b", true},
		{"https://API.example.com/a", "https://api.EXAMPLE.com:443/b", true},
		{"http://api.example.com/a", "http://api.example.com:80/b", true},
		{"http://api.example.com:8080/a", "http://api.example.com:8080/b", true},
		{"http://api.example.com/a", "https://api.example.com/b", true},        // upgrade 80 → 443
		{"http://api.example.com:80/a", "https://api.example.com:443/b", true}, // upgrade, ports written out
		{"https://api.example.com/a", "http://api.example.com/b", false},       // downgrade
		{"http://api.example.com:8080/a", "https://api.example.com/b", false},  // upgrade from another port
		{"http://api.example.com/a", "https://api.example.com:8443/b", false},  // upgrade to another port
		{"https://api.example.com/a", "https://api.example.com:8443/b", false},
		{"http://127.0.0.1:1000/a", "http://127.0.0.1:2000/b", false},
		{"https://example.com/a", "https://api.example.com/b", false}, // subdomain
		{"https://api.example.com/a", "https://example.com/b", false}, // parent domain
		{"https://api.example.com/a", "https://evil.example/b", false},
		{"https://api.example.com/a", "ftp://api.example.com/b", false},
	}
	for _, c := range cases {
		from, err := url.Parse(c.from)
		if err != nil {
			t.Fatal(err)
		}
		to, err := url.Parse(c.to)
		if err != nil {
			t.Fatal(err)
		}
		if got := apiRedirectKeepsHeaders(from, to); got != c.keep {
			t.Errorf("%s → %s: keep = %v, want %v", c.from, c.to, got, c.keep)
		}
	}
	if apiRedirectKeepsHeaders(nil, &url.URL{}) || apiRedirectKeepsHeaders(&url.URL{}, nil) {
		t.Error("a missing URL keeps the headers")
	}
}

// Once a chain has left the origin, a redirect back does not restore the credentials.
func TestAPIRequestRedirectBackToTheOriginStaysStripped(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	origin := &c13HeaderRecorder{}
	var startURL string
	away := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, startURL+"/landing", http.StatusFound)
	}))
	defer away.Close()
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin.record(r.Header)
		if r.URL.Path == "/go" {
			http.Redirect(w, r, away.URL+"/x", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer start.Close()
	startURL = start.URL

	if out := c13APIRequest(t, start.URL+"/go", map[string]string{"X-API-Key": "c13-key"}); out.Status != "success" {
		t.Fatalf("chain = %+v", out)
	}
	if back := origin.last(t); back.Get("X-Api-Key") != "" {
		t.Fatalf("the chain returned to the origin with the key: %v", back)
	}
}
