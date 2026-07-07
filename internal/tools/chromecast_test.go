package tools

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDiscoverChromecastDevicesParsesFriendlyName(t *testing.T) {
	orig := chromecastMDNSQuery
	chromecastMDNSQuery = func(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
		return []*mdnsEntry{
			{
				Name: "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437._googlecast._tcp.local.",
				IPs:  []string{"192.168.6.130"},
				Port: 8009,
				TXTs: []string{"fn=Arbeitszimmer", "md=Google Home Mini"},
			},
		}, nil
	}
	defer func() {
		chromecastMDNSQuery = orig
	}()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	devices, err := DiscoverChromecastDevices(logger)
	if err != nil {
		t.Fatalf("DiscoverChromecastDevices returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].FriendlyName != "Arbeitszimmer" {
		t.Fatalf("FriendlyName = %q, want %q", devices[0].FriendlyName, "Arbeitszimmer")
	}
	if devices[0].Name != "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437" {
		t.Fatalf("Name = %q", devices[0].Name)
	}
}

func TestFindChromecastDeviceByNameMatchesFriendlyName(t *testing.T) {
	devices := []ChromecastDevice{
		{
			Name:         "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437",
			FriendlyName: "Arbeitszimmer",
			Addr:         "192.168.6.130",
			Port:         8009,
		},
	}

	device, ok := FindChromecastDeviceByName(devices, "Arbeitszimmer")
	if !ok {
		t.Fatal("expected friendly-name match")
	}
	if device.Addr != "192.168.6.130" {
		t.Fatalf("Addr = %q, want 192.168.6.130", device.Addr)
	}
}

func TestValidateChromecastMediaURLRejectsHTTP404(t *testing.T) {
	origFactory := chromecastHTTPClientFactory
	chromecastHTTPClientFactory = func(cfg ChromecastConfig) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})}
	}
	defer func() {
		chromecastHTTPClientFactory = origFactory
	}()

	if err := validateChromecastMediaURL("https://media.example.test/missing.mp3", ChromecastConfig{}); err == nil {
		t.Fatal("expected 404 media URL to be rejected")
	}
}

func TestValidateChromecastMediaURLPolicyRejectsPrivateURLByDefault(t *testing.T) {
	err := validateChromecastMediaURLPolicy("http://192.168.1.10/movie.mp4", ChromecastConfig{})
	if err == nil || !strings.Contains(err.Error(), "SSRF protection") {
		t.Fatalf("error = %v, want SSRF protection rejection", err)
	}
}

func TestValidateChromecastMediaURLPolicyAllowsConfiguredPrivateHosts(t *testing.T) {
	cfg := ChromecastConfig{
		MediaHostAllowlist: []string{
			"192.168.1.10",
			"media.lan:8096",
			"192.168.2.0/24",
		},
	}
	tests := []string{
		"http://192.168.1.10/movie.mp4",
		"http://media.lan:8096/movie.mp4",
		"http://192.168.2.42/movie.mp4",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if err := validateChromecastMediaURLPolicy(rawURL, cfg); err != nil {
				t.Fatalf("validateChromecastMediaURLPolicy() error = %v", err)
			}
		})
	}
}

func TestValidateChromecastMediaURLPolicyRejectsForbiddenInternalTargetsEvenWhenAllowlisted(t *testing.T) {
	cfg := ChromecastConfig{
		MediaHostAllowlist: []string{
			"127.0.0.1",
			"169.254.169.254",
			"0.0.0.0/0",
		},
	}
	tests := []string{
		"http://127.0.0.1:8090/cast-media/movie.mp4",
		"http://169.254.169.254/latest/meta-data",
		"http://0.0.0.0:8090/cast-media/movie.mp4",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			err := validateChromecastMediaURLPolicy(rawURL, cfg)
			if err == nil || !strings.Contains(err.Error(), "SSRF protection") {
				t.Fatalf("error = %v, want SSRF protection rejection", err)
			}
		})
	}
}

func TestValidateChromecastMediaURLPolicyAllowsAuraGoOwnedMediaURLs(t *testing.T) {
	cfg := ChromecastConfig{ServerHost: "192.168.1.20", ServerPort: 8090}
	tests := []string{
		"http://192.168.1.20:8090/tts/speech.mp3",
		"http://192.168.1.20:8090/cast-media/movie.mp4",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if err := validateChromecastMediaURLPolicy(rawURL, cfg); err != nil {
				t.Fatalf("validateChromecastMediaURLPolicy() error = %v", err)
			}
		})
	}

	if err := validateChromecastMediaURLPolicy("http://192.168.1.20:8090/admin", cfg); err == nil {
		t.Fatal("expected non-media path on private AuraGo host to be rejected")
	}
}

func TestChromecastNeedsLANReachableHost(t *testing.T) {
	tests := map[string]bool{
		"":             true,
		"0.0.0.0":      true,
		"127.0.0.1":    true,
		"127.0.0.2":    true,
		"localhost":    true,
		"::1":          true,
		"[::1]":        true,
		"192.168.1.20": false,
		"aurago.lan":   false,
	}
	for host, want := range tests {
		t.Run(host, func(t *testing.T) {
			if got := chromecastNeedsLANReachableHost(host); got != want {
				t.Fatalf("chromecastNeedsLANReachableHost(%q) = %v, want %v", host, got, want)
			}
		})
	}
}

func TestChromecastMediaHTTPClientRevalidatesRedirectTargets(t *testing.T) {
	client := newChromecastMediaHTTPClient(ChromecastConfig{}, time.Second)
	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	err = client.CheckRedirect(req, nil)
	if err == nil || !strings.Contains(err.Error(), "SSRF protection") {
		t.Fatalf("redirect error = %v, want SSRF protection rejection", err)
	}
}
