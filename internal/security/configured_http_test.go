package security

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestConfiguredHTTPTransportPinsDNSAndBypassesPrivateProxy(t *testing.T) {
	for _, private := range []bool{true, false} {
		t.Run(map[bool]string{true: "private", false: "public"}[private], func(t *testing.T) {
			originHits, proxyHits, lookups := 0, 0, 0
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				originHits++
				if r.Header.Get("Authorization") != "Bearer fixture-secret" {
					t.Error("credential missing")
				}
				io.WriteString(w, "ok")
			}))
			defer origin.Close()
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { proxyHits++; io.WriteString(w, "proxied") }))
			defer proxy.Close()
			proxyURL, _ := url.Parse(proxy.URL)
			originURL, _ := url.Parse(origin.URL)
			dialed := ""
			source := http.DefaultTransport.(*http.Transport).Clone()
			source.Proxy = http.ProxyURL(proxyURL)
			source.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				dialed = address
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}
			raw := "http://integration.test:" + originURL.Port()
			transport := NewConfiguredHTTPTransport(raw, source).(*configuredHTTPTransport)
			defer transport.CloseIdleConnections()
			transport.lookup = func(context.Context, string) ([]net.IPAddr, error) {
				lookups++
				if lookups > 1 {
					t.Error("second resolution could rebind the target")
				}
				ip := "127.0.0.1"
				if !private {
					ip = "93.184.216.34"
				}
				return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
			}
			req, _ := http.NewRequest(http.MethodGet, raw, nil)
			req.Header.Set("Authorization", "Bearer fixture-secret")
			resp, err := (&http.Client{Transport: transport}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if lookups != 1 {
				t.Fatalf("lookups=%d", lookups)
			}
			if private && (originHits != 1 || proxyHits != 0 || !strings.HasPrefix(dialed, "127.0.0.1:")) {
				t.Fatalf("private request: origin=%d proxy=%d dial=%s", originHits, proxyHits, dialed)
			}
			if !private && (proxyHits != 1 || originHits != 0) {
				t.Fatalf("public request: origin=%d proxy=%d", originHits, proxyHits)
			}
		})
	}
}

func TestConfiguredHTTPTransportRejectsForeignOriginBeforeResolution(t *testing.T) {
	transport := NewConfiguredHTTPTransport("https://configured.test", http.DefaultTransport.(*http.Transport)).(*configuredHTTPTransport)
	transport.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("resolved foreign target")
		return nil, nil
	}
	for _, target := range []string{"https://other.test/", "http://configured.test/", "https://configured.test:444/", "https://user@configured.test/"} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		if _, err := transport.RoundTrip(req); err == nil {
			t.Errorf("accepted %s", target)
		}
	}
}
