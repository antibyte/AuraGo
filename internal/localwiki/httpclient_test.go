package localwiki

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientRefusesRedirectsToHTTP(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-http":
			http.Redirect(w, r, "http://127.0.0.1:9/never", http.StatusFound)
		case "/to-https":
			http.Redirect(w, r, server.URL+"/ok", http.StatusFound)
		case "/ok":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	client := newHTTPClient(server.Client())

	if _, err := client.Get(server.URL + "/to-http"); !errors.Is(err, errInsecureRedirect) {
		t.Fatalf("redirect to http:// = %v, want errInsecureRedirect", err)
	}
	resp, err := client.Get(server.URL + "/to-https")
	if err != nil {
		t.Fatalf("redirect to https:// failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if server.Client().CheckRedirect != nil {
		t.Fatal("newHTTPClient must not modify the caller's client")
	}
	if def := newHTTPClient(nil); def.CheckRedirect == nil || def.Timeout != 0 {
		t.Fatalf("default client must use the HTTPS-only policy and no overall timeout: %+v", def)
	}
}

func TestRequireHTTPSAndMeta4Hosts(t *testing.T) {
	for _, raw := range []string{"http://kiwix.org/x", "https://user:pw@kiwix.org/x", "ftp://kiwix.org", "https://", "/relative"} {
		if _, err := requireHTTPS(raw); err == nil {
			t.Fatalf("requireHTTPS(%q) accepted", raw)
		}
	}
	catalog := mustURL(t, "https://127.0.0.1:8443")
	for raw, want := range map[string]bool{
		"https://lb.download.kiwix.org/zim/x.zim.meta4": true,
		"https://kiwix.org/x.meta4":                     true,
		"https://127.0.0.1:8443/zim/x.zim.meta4":        true,
		"https://evilkiwix.org/x.meta4":                 false,
		"https://kiwix.org.evil.example/x.meta4":        false,
		"https://127.0.0.1:9999/x.meta4":                false,
	} {
		parsed, err := requireHTTPS(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := meta4HostAllowed(parsed, catalog); got != want {
			t.Fatalf("meta4HostAllowed(%q) = %v, want %v", raw, got, want)
		}
	}
}
