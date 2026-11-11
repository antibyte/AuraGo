package security

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func startEgressTestProxy(t *testing.T, allowed *countedServer, opts ...CaptureProxyOption) *CaptureProxy {
	t.Helper()
	var origin *url.URL
	if allowed != nil {
		origin = mustParseURL(t, allowed.URL)
	}
	p, err := StartCaptureProxy(origin, CaptureProxyLimits{}, opts...)
	if err != nil {
		t.Fatalf("StartCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func egressProxyClient(t *testing.T, p *CaptureProxy) *http.Client {
	t.Helper()
	transport := &http.Transport{Proxy: http.ProxyURL(mustParseURL(t, "http://"+p.Addr())), DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func connectStatus(t *testing.T, p *CaptureProxy, target string) int {
	t.Helper()
	conn, err := net.DialTimeout("tcp", p.Addr(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	res, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT reply: %v", err)
	}
	return res.StatusCode
}

func TestCaptureProxyDoneClosesWhenTheProxyStops(t *testing.T) {
	p := startEgressTestProxy(t, nil)
	select {
	case <-p.Done():
		t.Fatal("Done is closed on a running proxy")
	default:
	}
	_ = p.Close()
	select {
	case <-p.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done did not close after Close")
	}
	_ = p.Close() // idempotent
}

func TestCaptureProxyAnswersTheCheckHostWithoutDialling(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startEgressTestProxy(t, allowed, withEgressCheck("n0nce-1234"))

	res, err := egressProxyClient(t, p).Get("http://egress-check.aurago.invalid/n0nce-1234")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("check host = %d, want 204", res.StatusCode)
	}
	if got := res.Header.Get("X-AuraGo-Egress"); got != "n0nce-1234" {
		t.Fatalf("X-AuraGo-Egress = %q, want the nonce", got)
	}
	if res.Header.Get("Access-Control-Expose-Headers") != "X-AuraGo-Egress" || res.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("CORS headers missing: %v", res.Header)
	}
	if got := connectStatus(t, p, "egress-check.aurago.invalid:443"); got != http.StatusForbidden {
		t.Fatalf("CONNECT to the check host = %d, want 403", got)
	}
	if n := len(p.Dialed()) + len(p.Refused()); n != 0 {
		t.Fatalf("the check host reached the dial path: dialed=%v refused=%v", p.Dialed(), p.Refused())
	}
}

func TestCaptureProxyLogsDialedAndRefusedDestinations(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "")
	allowed, _ := captureProxyTestOrigins(t)
	p := startEgressTestProxy(t, allowed)
	client := egressProxyClient(t, p)

	res, err := client.Get(allowed.URL + "/x")
	if err != nil {
		t.Fatalf("allowed get: %v", err)
	}
	res.Body.Close()
	// A refused plain-HTTP destination must look like a network error to the
	// browser: a 403 page would count as a successful navigation.
	if res, err = client.Get("http://10.255.255.1/"); err == nil {
		res.Body.Close()
		t.Fatalf("private destination answered %d, want a cut connection", res.StatusCode)
	}
	if d := p.Dialed(); len(d) != 1 || d[0] != allowed.hostPort() {
		t.Fatalf("Dialed() = %v, want [%s]", d, allowed.hostPort())
	}
	if r := p.Refused(); len(r) != 1 || r[0] != "10.255.255.1:80" {
		t.Fatalf("Refused() = %v, want [10.255.255.1:80]", r)
	}
}

func TestCaptureProxyLogsAreCapped(t *testing.T) {
	p := startEgressTestProxy(t, nil)
	for i := 0; i < captureProxyLogCap+10; i++ {
		p.logAppend(&p.dialLog, "x:1")
	}
	if n := len(p.Dialed()); n != captureProxyLogCap {
		t.Fatalf("Dialed() has %d entries, want the cap %d", n, captureProxyLogCap)
	}
}
