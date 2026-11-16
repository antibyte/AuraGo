package tools

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTailscaleSetRoutesRespectsDirectReadOnly(t *testing.T) {
	cfg := TailscaleConfig{ReadOnly: true}

	got := TailscaleSetRoutes(cfg, "node-1234567890", []string{"192.168.1.0/24"}, true)
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(strings.ToLower(got), "read-only") {
		t.Fatalf("expected read-only error, got %s", got)
	}
}

func TestTailscaleDeviceIdentifiersAreSingleSegments(t *testing.T) {
	for _, valid := range []string{"11055", "nABC123CNTRL"} {
		if got, err := tailscaleFindDeviceID(TailscaleConfig{}, valid); err != nil || got != valid {
			t.Fatalf("valid ID %q: %q, %v", valid, got, err)
		}
	}
	for _, bad := range []string{"../tailnet/devices", "nABC123CNTRL/routes", "nABC123CNTRL?x=1", "nABC123CNTRL#x", "nABC123%2fCNTRL", "nABC123%252fCNTRL", "nABC123\nCNTRL", "nABC123\\CNTRL"} {
		if _, err := tailscaleFindDeviceID(TailscaleConfig{}, bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	previous := tailscaleHTTPClient
	t.Cleanup(func() { tailscaleHTTPClient = previous })
	for _, id := range []string{"11055", "../../tailnet/other"} {
		tailscaleHTTPClient = &http.Client{Transport: tailscaleFixtureTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"devices":[{"id":"` + id + `","hostname":"nas","addresses":["100.64.0.1"]}]}`)), Header: make(http.Header)}, nil
		})}
		for _, query := range []string{"nas", "100.64.0.1"} {
			_, err := tailscaleFindDeviceID(TailscaleConfig{}, query)
			if (err == nil) != (id == "11055") {
				t.Errorf("query=%q id=%q err=%v", query, id, err)
			}
		}
	}
}

type tailscaleFixtureTransport func(*http.Request) (*http.Response, error)

func (f tailscaleFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
