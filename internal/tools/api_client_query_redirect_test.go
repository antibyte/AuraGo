package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// A secret that http.request puts into the first address (auth_in=query) never reaches a later
// hop: the next address is the server's Location, to another host and to the same host alike.
func TestAPIRequestRedirectDoesNotCarryTheQuerySecret(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	var mu sync.Mutex
	var seen []string
	record := func(who string, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, who+" "+r.URL.Path+"?"+r.URL.RawQuery)
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("target", r)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("start", r)
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, target.URL+"/landing", http.StatusTemporaryRedirect)
		case "/here":
			http.Redirect(w, r, "/landing", http.StatusTemporaryRedirect)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer start.Close()

	for _, path := range []string{"/away", "/here"} {
		if out := c13APIRequest(t, start.URL+path+"?api_key=qry-secret-value", nil); out.Status != "success" {
			t.Fatalf("%s = %+v", path, out)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for i, line := range seen {
		first := strings.HasPrefix(line, "start /away?") || strings.HasPrefix(line, "start /here?")
		if first && !strings.Contains(line, "api_key=qry-secret-value") {
			t.Errorf("request %d lost the first-hop query: %s", i, line)
		}
		if !first && strings.Contains(line, "qry-secret-value") {
			t.Errorf("request %d carries the secret after a redirect: %s", i, line)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("requests = %v, want the two first hops and the two landings", seen)
	}
}
