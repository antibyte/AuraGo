package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"aurago/internal/security"
	"aurago/internal/testutil"

	"github.com/go-rod/rod/lib/defaults"
)

// TestWebCaptureGoesThroughTheEgressProxy shows that web capture uses the helper:
// the proxy log of the browser it launched holds the page's own request.
func TestWebCaptureGoesThroughTheEgressProxy(t *testing.T) {
	bin := testutil.RequireBrowserBin(t)
	defaults.Bin = bin
	t.Cleanup(func() { defaults.Bin = "" })
	fx := testutil.NewEgressFixture(t)

	workspace := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		t.Fatal(err)
	}
	ctx := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowFilesystemWrite: true})

	var launched atomic.Pointer[security.EgressBrowser]
	restore := security.SetEgressLaunchObserver(func(eb *security.EgressBrowser) { launched.Store(eb) })
	defer restore()

	out := WebCaptureWithWorkspace(ctx, "screenshot", fx.URL(), "", false, "", workspace)
	var res webCaptureResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Status != "success" {
		t.Fatalf("WebCapture = %s (decode error %v)", out, err)
	}
	eb := launched.Load()
	if eb == nil {
		t.Fatal("WebCapture did not launch an egress browser")
	}
	if dialed := eb.ProxyDialed(); !slices.Contains(dialed, fx.HostPort()) {
		t.Fatalf("the page's own request did not go through the proxy; dialed = %v", dialed)
	}
}
