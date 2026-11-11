package scraper

import (
	"slices"
	"sync/atomic"
	"testing"

	"aurago/internal/security"
	"aurago/internal/testutil"

	"github.com/go-rod/rod/lib/defaults"
)

// TestScraperDynamicGoesThroughTheEgressProxy shows that FetchDynamic really
// uses the helper: the proxy log of the browser it launched holds the page's
// own request. (The page is created inside FetchDynamic, so there is no hook for
// reduced motion; the fixture page has no animation.)
func TestScraperDynamicGoesThroughTheEgressProxy(t *testing.T) {
	bin := testutil.RequireBrowserBin(t)
	defaults.Bin = bin // rod's browser lookup, read by launcher.New
	t.Cleanup(func() { defaults.Bin = "" })
	fx := testutil.NewEgressFixture(t)

	var launched atomic.Pointer[security.EgressBrowser]
	restore := security.SetEgressLaunchObserver(func(eb *security.EgressBrowser) { launched.Store(eb) })
	defer restore()

	if _, err := New(nil).FetchDynamic(fx.URL(), ""); err != nil {
		t.Fatalf("FetchDynamic: %v", err)
	}
	eb := launched.Load()
	if eb == nil {
		t.Fatal("FetchDynamic did not launch an egress browser")
	}
	if dialed := eb.ProxyDialed(); !slices.Contains(dialed, fx.HostPort()) {
		t.Fatalf("the page's own request did not go through the proxy; dialed = %v", dialed)
	}
}
