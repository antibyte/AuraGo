package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

func TestCaptureHomepageAnnotationSnapshotRejectsConcurrentCapture(t *testing.T) {
	homepageAnnotationSlot <- struct{}{}
	defer func() { <-homepageAnnotationSlot }()
	target, _ := url.Parse("http://127.0.0.1:1/")
	_, err := CaptureHomepageAnnotationSnapshot(context.Background(), HomepageAnnotationCaptureOptions{URL: target, AllowedOrigin: target, Width: 1280, Height: 800})
	if !errors.Is(err, ErrHomepageAnnotationBusy) {
		t.Fatalf("err = %v, want ErrHomepageAnnotationBusy", err)
	}
}

func TestCaptureHomepageAnnotationSnapshotBlocksPrivateDeployTargets(t *testing.T) {
	target, _ := url.Parse("http://10.0.0.7/")
	_, err := CaptureHomepageAnnotationSnapshot(context.Background(), HomepageAnnotationCaptureOptions{URL: target, Width: 1280, Height: 800})
	if !errors.Is(err, ErrHomepageAnnotationBlocked) {
		t.Fatalf("err = %v, want ErrHomepageAnnotationBlocked", err)
	}
}

func TestClipHomepageAnnotationElementsDropsAndReindexes(t *testing.T) {
	elements := []HomepageAnnotationElement{
		{I: 0, Selector: "body", Box: [4]float64{0, 0, 1280, 20000}, Parent: -1},
		{I: 1, Selector: "footer", Box: [4]float64{0, 15000, 1280, 200}, Parent: 0},
		{I: 2, Selector: "#cta", Box: [4]float64{10, 11950, 100, 100}, Parent: 0},
		{I: 3, Selector: "#cta > span", Box: [4]float64{12, 11960, 20, 20}, Parent: 2},
		{I: 4, Selector: ".offscreen", Box: [4]float64{0, -300, 50, 50}, Parent: 0},
	}
	got := clipHomepageAnnotationElements(elements, 12000)
	if len(got) != 3 {
		t.Fatalf("kept %d elements, want 3: %+v", len(got), got)
	}
	if got[0].Box[3] != 12000 || got[1].Box[3] != 50 {
		t.Fatalf("boxes were not clipped to the page height: %+v", got)
	}
	if got[1].I != 1 || got[1].Parent != 0 || got[2].I != 2 || got[2].Parent != 1 {
		t.Fatalf("indexes not rewritten: %+v", got)
	}
}

func homepageAnnotationTestBrowser(t *testing.T) string {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to launch Chrome/Edge")
	}
	for _, path := range []string{`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "chromium", "google-chrome"} {
		if resolved, err := exec.LookPath(path); err == nil {
			return resolved
		}
	}
	t.Skip("Chrome/Edge unavailable")
	return ""
}

func TestCaptureHomepageAnnotationSnapshotBrowser(t *testing.T) {
	bin := homepageAnnotationTestBrowser(t)
	var probed atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probed.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer other.Close()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><body style="margin:0">
<header><nav><a href="/a">Alpha</a><a href="/b">Beta</a></nav></header>
<main><h1>Welcome</h1><a id="cta" class="btn primary" href="/signup">Get started</a>
<p class="lead">%s</p><div style="display:none" id="ghost">Hidden</div>
<img src="%s/probe.png" width="10" height="10" alt="probe"></main></body></html>`, strings.Repeat("long text ", 40), other.URL)
	}))
	defer site.Close()

	target, _ := url.Parse(site.URL + "/")
	origin, _ := url.Parse(site.URL)
	snap, err := CaptureHomepageAnnotationSnapshot(context.Background(), HomepageAnnotationCaptureOptions{
		URL: target, AllowedOrigin: origin, Width: 1280, Height: 800, BrowserBin: bin,
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if !strings.HasPrefix(snap.Image, "data:image/jpeg;base64,") {
		t.Fatalf("image is not a JPEG data URI: %.40q", snap.Image)
	}
	if snap.Page.Width != 1280 || snap.Page.Height < 800 || snap.Page.Truncated {
		t.Fatalf("page = %+v", snap.Page)
	}
	if probed.Load() {
		t.Fatal("a subresource on another loopback origin was fetched")
	}
	seen := map[string]bool{}
	var cta *HomepageAnnotationElement
	for i := range snap.Elements {
		el := &snap.Elements[i]
		if seen[el.Selector] {
			t.Fatalf("duplicate selector %q", el.Selector)
		}
		seen[el.Selector] = true
		if el.Selector == "#ghost" {
			t.Fatal("a hidden element was mapped")
		}
		if utf8.RuneCountInString(el.Text) > 120 {
			t.Fatalf("text not truncated: %d runes", utf8.RuneCountInString(el.Text))
		}
		if el.Selector == "#cta" {
			cta = el
		}
	}
	if cta == nil || cta.Tag != "a" || cta.Text != "Get started" || cta.Attrs["href"] != "/signup" || !cta.Tab {
		t.Fatalf("cta = %+v", cta)
	}
}
