package security

import (
	"context"
	"slices"
	"testing"
	"time"

	"aurago/internal/testutil"

	"github.com/go-rod/rod/lib/proto"
)

// TestEgressBrowserRefusesTheS8Fixture drives the helper directly. The page
// tries every route to a private address that a page-level hook cannot see
// (popup, WebSocket, service worker, prefetch and prerender, WebRTC STUN). The
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

	// Popup (169.254.169.254) and WebSocket (10.255.255.1) are required.
	for _, target := range []string{"169.254.169.254:80", "10.255.255.1:80"} {
		if !slices.Contains(refused, target) {
			t.Errorf("the proxy never refused %s; refused = %v", target, refused)
		}
	}
	// The service worker probe must say plainly which of two things is true.
	assertServiceWorkerOutcome(t, fx, report, refused)

	for _, target := range testutil.EgressPrivateTargets {
		if slices.Contains(dialed, target) {
			t.Errorf("the proxy dialled %s", target)
		}
	}
	if !slices.Contains(dialed, fx.HostPort()) {
		t.Errorf("the page's own request did not go through the proxy; dialed = %v", dialed)
	}
	if report.CandidatePairs != 0 {
		t.Errorf("the page formed %d WebRTC candidate pairs, want 0 (UDP must not leave the proxy)", report.CandidatePairs)
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
		// (b): the reason is the register() rejection, not a guess.
		t.Logf("service worker (b): cannot register behind the proxy, so it has no egress. secure_context=%v worker_api=%v reason=%q",
			report.SecureContext, report.WorkerAPI, report.WorkerError)
		if slices.Contains(pings, "install") {
			t.Errorf("service worker (b): register() failed but the worker ran anyway; pings = %v", pings)
		}
	default:
		t.Errorf("service worker: neither registered nor refused with an error; report = %+v, pings = %v", report, pings)
	}
}
