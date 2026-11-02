package security

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func startStrictTestProxy(t *testing.T, allowed *countedServer, limits CaptureProxyLimits) *CaptureProxy {
	t.Helper()
	p, err := StartStrictCaptureProxy(mustParseURL(t, allowed.URL), limits)
	if err != nil {
		t.Fatalf("StartStrictCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestStrictCaptureProxyNeedsAnAllowedOrigin(t *testing.T) {
	if p, err := StartStrictCaptureProxy(nil, CaptureProxyLimits{}); err == nil {
		_ = p.Close()
		t.Fatal("a strict proxy without an allowed origin was started")
	}
}

func TestStrictCaptureProxyServesTheAllowedOriginOnly(t *testing.T) {
	allowed, other := captureProxyTestOrigins(t) // both are 127.0.0.1 servers on different ports
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{})
	client := proxyClient(t, p)

	res, err := client.Get(allowed.URL + "/page")
	if err != nil {
		t.Fatalf("allowed origin: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "allowed /page" {
		t.Fatalf("allowed origin = %d %q", res.StatusCode, body)
	}

	// A POST with a body reaches the allowed origin as well (frames travel that way).
	res, err = client.Post(allowed.URL+"/post", "application/octet-stream", strings.NewReader("frame-bytes"))
	if err != nil {
		t.Fatalf("post to the allowed origin: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("post status = %d", res.StatusCode)
	}

	// Another loopback port is refused without a single connection.
	res, err = client.Get(other.URL + "/secret")
	if err != nil {
		t.Fatalf("other loopback port: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("other loopback port status = %d, want 403", res.StatusCode)
	}
	if got := other.accepts.Load(); got != 0 {
		t.Fatalf("the other loopback server accepted %d connections", got)
	}
}

func TestStrictCaptureProxyRefusesEveryOtherDestinationForm(t *testing.T) {
	allowed, other := captureProxyTestOrigins(t)
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{})
	port := strings.Split(allowed.hostPort(), ":")[1]
	targets := []string{
		other.hostPort(),                    // another loopback port
		"localhost:" + port,                 // another name for the allowed address
		"[::1]:" + port,                     // another address family
		"127.0.0.2:" + port,                 // another loopback address
		"example.com:443", "example.com:80", // a public name
		"93.184.216.34:443", // a public address
		"10.0.0.7:80", "169.254.169.254:80",
	}
	for _, target := range targets {
		if _, _, status := rawConnect(t, p, target); status != http.StatusForbidden {
			t.Errorf("CONNECT %s status = %d, want 403", target, status)
		}
	}
	// The same destinations as plain absolute-form requests.
	client := proxyClient(t, p)
	for _, target := range []string{"http://" + other.hostPort() + "/", "http://localhost:" + port + "/", "http://example.com/"} {
		res, err := client.Get(target)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("GET %s status = %d, want 403", target, res.StatusCode)
		}
	}
	if got := other.accepts.Load(); got != 0 {
		t.Fatalf("the other loopback server accepted %d connections", got)
	}
}

func TestStrictCaptureProxyIgnoresTheLoopbackEscapeHatch(t *testing.T) {
	allowed, other := captureProxyTestOrigins(t)
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{})
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	if _, _, status := rawConnect(t, p, other.hostPort()); status != http.StatusForbidden {
		t.Fatalf("with AURAGO_SSRF_ALLOW_LOOPBACK=1 a strict proxy answered %d for another loopback port, want 403", status)
	}
	if got := other.accepts.Load(); got != 0 {
		t.Fatalf("the other loopback server accepted %d connections", got)
	}
}

func TestStrictCaptureProxyKeepsTheNonStrictBehaviourOfTheOthers(t *testing.T) {
	// The default proxy still dials a loopback destination under the escape hatch, as before.
	allowed, other := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	if _, _, status := rawConnect(t, p, other.hostPort()); status != http.StatusOK {
		t.Fatalf("non-strict CONNECT status = %d, want 200 (existing behaviour)", status)
	}
}

func TestStrictCaptureProxyHonoursTheResponseHeaderTimeout(t *testing.T) {
	slow := newCountedServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond)
		_, _ = io.WriteString(w, "late")
	})
	get := func(limits CaptureProxyLimits) int {
		p := startStrictTestProxy(t, slow, limits)
		client := proxyClient(t, p)
		res, err := client.Get(slow.URL + "/")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if status := get(CaptureProxyLimits{ResponseHeaderTimeout: 150 * time.Millisecond}); status != http.StatusBadGateway {
		t.Fatalf("short header timeout status = %d, want 502", status)
	}
	if status := get(CaptureProxyLimits{ResponseHeaderTimeout: 5 * time.Second}); status != http.StatusOK {
		t.Fatalf("long header timeout status = %d, want 200", status)
	}
}

func TestStrictCaptureProxyCountsBodiesAgainstTheByteLimit(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{MaxBytes: 4096})
	client := proxyClient(t, p)
	body := strings.NewReader(strings.Repeat("x", 64<<10))
	res, err := client.Post(allowed.URL+"/upload", "application/octet-stream", body)
	if err == nil {
		_ = res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Fatalf("a 64 KiB body passed a 4 KiB limit")
		}
	}
	// After the limit is spent nothing new is dialed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(allowed.URL + "/page")
		if err != nil {
			return
		}
		status := res.StatusCode
		_ = res.Body.Close()
		if status != http.StatusOK {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the proxy kept serving after its byte limit")
}
