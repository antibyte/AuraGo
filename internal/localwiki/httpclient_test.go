package localwiki

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHTTPClientRefusesRedirectsToHTTP(t *testing.T) {
	var server *httptest.Server
	var sameServerByName string
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-http":
			http.Redirect(w, r, "http://127.0.0.1:9/never", http.StatusFound)
		case "/to-https":
			http.Redirect(w, r, server.URL+"/ok", http.StatusFound)
		case "/to-local":
			// Same server, but under a name that is not the trusted host.
			http.Redirect(w, r, sameServerByName+"/ok", http.StatusFound)
		case "/to-private":
			http.Redirect(w, r, "https://192.168.1.1/x", http.StatusFound)
		case "/ok":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	sameServerByName = "https://localhost:" + mustURL(t, server.URL).Port()
	client := newHTTPClient(server.Client(), mustURL(t, server.URL))

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
	// Redirects to local hosts are refused before any connection, also on a
	// caller's client that has no dial guard.
	for _, path := range []string{"/to-local", "/to-private"} {
		if _, err := client.Get(server.URL + path); !errors.Is(err, errLocalAddress) {
			t.Fatalf("redirect %s = %v, want errLocalAddress", path, err)
		}
	}
	if _, err := newHTTPClient(server.Client()).Get(server.URL + "/to-https"); !errors.Is(err, errLocalAddress) {
		t.Fatalf("redirect to an untrusted local host = %v, want errLocalAddress", err)
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

// trustTestServer makes a default client accept the httptest certificate.
func trustTestServer(t *testing.T, client *http.Client, server *httptest.Server) *http.Client {
	t.Helper()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("default client transport is %T", client.Transport)
	}
	transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	return client
}

func TestDefaultHTTPClientRefusesLocalAddresses(t *testing.T) {
	var localhost string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hop" {
			http.Redirect(w, r, localhost+"/ok", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	base := mustURL(t, server.URL)
	localhost = "https://localhost:" + base.Port()

	for _, target := range []string{server.URL + "/ok", localhost + "/ok"} {
		client := trustTestServer(t, newHTTPClient(nil), server)
		if _, err := client.Get(target); !errors.Is(err, errLocalAddress) {
			t.Fatalf("GET %s = %v, want errLocalAddress", target, err)
		}
	}

	trusted := trustTestServer(t, newHTTPClient(nil, base), server)
	resp, err := trusted.Get(server.URL + "/ok")
	if err != nil {
		t.Fatalf("GET from the trusted host: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if _, err := trusted.Get(server.URL + "/hop"); !errors.Is(err, errLocalAddress) {
		t.Fatalf("redirect from the trusted host to another local host = %v, want errLocalAddress", err)
	}
	if custom := newHTTPClient(server.Client()); custom.Transport != server.Client().Transport {
		t.Fatal("a caller's client keeps its own transport")
	}
}

func TestDialGuardExemptsTrustedHostsAndConfiguredProxies(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://download.kiwix.org/zim/wikipedia/x.zim", nil)
	for raw, address := range map[string]string{
		"http://10.0.0.5:3128": "10.0.0.5:3128",
		"http://Proxy.LAN":     "proxy.lan:80",
		"https://[fd00::1]":    "[fd00::1]:443",
		"socks5://192.168.1.2": "192.168.1.2:1080",
	} {
		proxyURL := mustURL(t, raw)
		guard := newDialGuard(func(*http.Request) (*url.URL, error) { return proxyURL, nil }, nil)
		if guard.isExempt(address) {
			t.Fatalf("%s is exempt before it was used as a proxy", address)
		}
		if got, err := guard.proxy(req); err != nil || got != proxyURL {
			t.Fatalf("proxy(%s) = %v, %v", raw, got, err)
		}
		if !guard.isExempt(address) {
			t.Fatalf("proxy %s (%s) is not exempt", raw, address)
		}
	}
	guard := newDialGuard(nil, []*url.URL{mustURL(t, "https://127.0.0.1:8443"), nil, mustURL(t, "https://Catalog.Test")})
	for address, want := range map[string]bool{
		"127.0.0.1:8443":   true,
		"catalog.test:443": true,
		"127.0.0.1:9443":   false,
		"catalog.test:80":  false,
	} {
		if got := guard.isExempt(address); got != want {
			t.Fatalf("isExempt(%s) = %v, want %v", address, got, want)
		}
	}
	if got, err := guard.proxy(req); got != nil || err != nil {
		t.Fatalf("a guard without a proxy function = %v, %v", got, err)
	}
}

func TestRefuseLocalAddress(t *testing.T) {
	for address, refused := range map[string]bool{
		"127.0.0.1:443":          true,
		"[::1]:443":              true,
		"10.1.2.3:443":           true,
		"172.16.0.1:443":         true,
		"192.168.1.1:443":        true,
		"169.254.169.254:80":     true,
		"[fe80::1%1]:443":        true,
		"100.64.0.1:443":         true,
		"0.0.0.0:443":            true,
		"[::ffff:127.0.0.1]:443": true,
		"[fd00::1]:443":          true,
		"[::]:443":               true,
		"garbage":                true,
		"example.org:443":        true,
		"93.184.215.14:443":      false,
		"[2a01:4f8::1]:443":      false,
	} {
		err := refuseLocalAddress("tcp", address, nil)
		if refused != errors.Is(err, errLocalAddress) {
			t.Fatalf("refuseLocalAddress(%s) = %v, refused want %v", address, err, refused)
		}
	}
}
