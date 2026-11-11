package security

import (
	"context"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/testutil"

	"github.com/go-rod/rod/lib/proto"
)

// TestEgressBrowserRefusesTheS8Fixture drives the helper directly. The page
// tries every route to a private address that a page-level hook cannot see
// (popup, WebSocket, sendBeacon, download link, service worker, prefetch and
// prerender, WebRTC STUN and TURN, dns-prefetch). The
// proxy must refuse each TCP target and never dial it, and no WebRTC candidate
// pair may form.
func TestEgressBrowserRefusesTheS8Fixture(t *testing.T) {
	bin := testutil.RequireBrowserBin(t)
	fx := testutil.NewEgressFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	eb, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{Bin: bin})
	if err != nil {
		t.Fatalf("LaunchEgressBrowser: %v", err)
	}
	defer eb.Close()

	page, err := eb.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	page = page.Context(ctx)
	// Every browser test sets reduced motion explicitly.
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "prefers-reduced-motion", Value: "reduce"},
	}}).Call(page); err != nil {
		t.Fatalf("reduced motion: %v", err)
	}
	if err := page.Navigate(fx.URL()); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	report := fx.WaitReport(t, 45*time.Second)
	time.Sleep(1 * time.Second) // let late probes reach the proxy

	if err := eb.Err(); err != nil {
		t.Fatalf("browser ended early: %v", err)
	}
	refused, dialed := eb.ProxyRefused(), eb.ProxyDialed()
	t.Logf("proxy refused = %v", refused)
	t.Logf("proxy dialed  = %v", dialed)
	t.Logf("page report   = %+v", report)
	t.Logf("worker pings  = %v", fx.WorkerPings())

	// Every probe must have run, or its refusal proves nothing.
	for name, ran := range map[string]bool{
		"popup": report.RanPopup, "websocket": report.RanWebSocket, "beacon": report.RanBeacon,
		"download": report.RanDownload, "prefetch": report.RanPrefetch, "prerender": report.RanPrerender,
		"dns-prefetch": report.RanDNSPrefetch, "webrtc stun connection": report.PCCreated,
		"webrtc turn connection": report.PCTurnCreated,
	} {
		if !ran {
			t.Errorf("the %s probe did not run; report = %+v", name, report)
		}
	}
	// These probes are required: each has its own private target, and the proxy
	// must have refused it. The TURN target is a different port of the STUN host.
	for probe, target := range map[string]string{
		"popup": "172.31.255.1:80", "websocket": "10.255.255.5:80", "beacon": "169.254.169.254:80",
		"download": "10.255.255.1:80", "webrtc turn over tcp": "10.255.255.1:3478",
	} {
		if !slices.Contains(refused, target) {
			t.Errorf("the %s probe: the proxy never refused %s; refused = %v", probe, target, refused)
		}
	}
	// Prefetch: Chrome may skip it by heuristics. The page can tell (the link
	// fires load or error when Chrome issued the request), so a skipped prefetch
	// is logged as not covered, and an issued one must have been refused.
	if report.PrefetchEvent != "" {
		if !slices.Contains(refused, "10.255.255.2:80") {
			t.Errorf("prefetch: Chrome issued it (link event %q) but the proxy never refused 10.255.255.2:80; refused = %v", report.PrefetchEvent, refused)
		}
	} else {
		t.Logf("prefetch is NOT covered by this run: Chrome fired no load or error event, so it never issued the request")
	}
	// Prerender: Chrome gives the page no signal that it acted on speculation
	// rules, so a missing refusal cannot be told from a skipped prerender. The
	// claim is dropped: it is logged, and only a dial is a failure (below).
	if slices.Contains(refused, "10.255.255.3:80") {
		t.Logf("prerender: Chrome issued it and the proxy refused 10.255.255.3:80")
	} else {
		t.Logf("prerender is NOT covered by this run: no request reached the proxy and the page cannot see whether Chrome tried (speculation rules API present: %v)", report.PrerenderAPI)
	}
	// dns-prefetch: a DNS-level leak is NOT observable here. Chrome behind a proxy
	// resolves no names itself and the proxy log only holds TCP destinations. All
	// this can prove is that no request for the canary host reached the proxy.
	for _, entry := range slices.Concat(refused, dialed) {
		if strings.Contains(entry, testutil.EgressDNSCanaryHost) {
			t.Errorf("a request for the dns-prefetch canary host reached the proxy: %s", entry)
		}
	}
	// The service worker probe must say plainly which of two things is true.
	assertServiceWorkerOutcome(t, fx, report, refused)

	for _, target := range testutil.EgressPrivateTargets {
		if slices.Contains(dialed, target) {
			t.Errorf("the proxy dialled %s", target)
		}
	}
	// Chrome keeps a few startup requests of its own (connectivity check,
	// sign-in and static hosts) even with background networking, sync and
	// component updates off. They are public Google hosts, so the proxy may dial
	// them; anything else besides the fixture's own host is a finding.
	for _, d := range dialed {
		if d != fx.HostPort() && !chromeOwnHost(d) {
			t.Errorf("the proxy dialled %s, which is neither the fixture's own host %s nor a Chrome-internal Google host; dialed = %v", d, fx.HostPort(), dialed)
		}
	}
	if !slices.Contains(dialed, fx.HostPort()) {
		t.Errorf("the page's own request did not go through the proxy; dialed = %v", dialed)
	}
	if !report.PCCreated {
		t.Errorf("RTCPeerConnection could not be created, so the WebRTC probe proves nothing; report = %+v", report)
	} else if report.CandidatePairs != 0 {
		t.Errorf("the page formed %d WebRTC candidate pairs, want 0 (UDP must not leave the proxy; -1 means getStats failed)", report.CandidatePairs)
	}
}

// assertServiceWorkerOutcome proves one of two things and fails otherwise:
//
//	(a) the worker registered and ran behind the proxy, and its fetch to
//	    192.168.255.1 was refused by the proxy (and reported as failed by the
//	    worker);
//	(b) the worker could not register behind the proxy, so it has no egress at
//	    all; the page's register() error is logged as the reason.
//
// A silent pass (no registration, no error) is a failure.
func assertServiceWorkerOutcome(t *testing.T, fx *testutil.EgressFixture, report testutil.EgressPageReport, refused []string) {
	t.Helper()
	pings := fx.WorkerPings()
	switch {
	case report.WorkerRegistered:
		if !slices.Contains(pings, "install") {
			t.Errorf("service worker (a): registered (state %q) but never ran; pings = %v", report.WorkerState, pings)
			return
		}
		if !slices.Contains(refused, "192.168.255.1:80") {
			t.Errorf("service worker (a): registered and ran, but the proxy never refused 192.168.255.1:80; refused = %v", refused)
		}
		if !slices.Contains(pings, "private=failed") {
			t.Errorf("service worker (a): its fetch to 192.168.255.1 did not fail; pings = %v", pings)
		}
		t.Logf("service worker (a): registered, ran, and its fetch to 192.168.255.1 was refused by the proxy")
	case report.WorkerError != "":
		// (b): only an expected failure text counts: no service worker API at all,
		// or the "operation is insecure" rejection of an untrusted origin. Any other
		// register() error is a failure of the probe, not an outcome.
		if !strings.Contains(report.WorkerError, "navigator.serviceWorker is undefined") &&
			!strings.Contains(strings.ToLower(report.WorkerError), "operation is insecure") {
			t.Errorf("service worker (b): register() failed with an unexpected error %q", report.WorkerError)
			return
		}
		t.Logf("service worker (b): cannot register behind the proxy, so it has no egress. secure_context=%v worker_api=%v reason=%q",
			report.SecureContext, report.WorkerAPI, report.WorkerError)
		if slices.Contains(pings, "install") {
			t.Errorf("service worker (b): register() failed but the worker ran anyway; pings = %v", pings)
		}
	default:
		t.Errorf("service worker: neither registered nor refused with an error; report = %+v, pings = %v", report, pings)
	}
}

// chromeOwnHost reports whether a dialled host:port is one of Chrome's own
// public Google endpoints.
func chromeOwnHost(hostPort string) bool {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false
	}
	for _, suffix := range []string{"google.com", "gstatic.com", "googleapis.com", "gvt1.com", "gvt2.com"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// A public plain-HTTP page that redirects to a private address must fail the
// navigation. The proxy used to answer such a request with a 403 text body,
// which Chrome renders as a successful page.
func TestEgressBrowserFailsANavigationRedirectedToAPrivateAddress(t *testing.T) {
	bin := testutil.RequireBrowserBin(t)
	fx := testutil.NewEgressFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	eb, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{Bin: bin})
	if err != nil {
		t.Fatalf("LaunchEgressBrowser: %v", err)
	}
	defer eb.Close()

	page, err := eb.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	err = page.Context(ctx).Navigate(fx.RedirectURL())
	if err == nil {
		t.Fatalf("navigating to %s succeeded; the redirect to %s must fail", fx.RedirectURL(), testutil.EgressPrivateRedirectTarget)
	}
	t.Logf("navigate error = %v", err)
	if !slices.Contains(eb.ProxyRefused(), "10.255.255.1:80") {
		t.Errorf("the proxy never refused 10.255.255.1:80; refused = %v", eb.ProxyRefused())
	}
	if slices.Contains(eb.ProxyDialed(), "10.255.255.1:80") {
		t.Errorf("the proxy dialled 10.255.255.1:80")
	}
}
