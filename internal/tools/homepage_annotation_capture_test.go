package tools

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

// homepageAnnotationProbe records every connection and request an "internal"
// server receives. Any entry means the capture browser reached an address the
// preview must not be able to reach.
type homepageAnnotationProbe struct {
	mu      sync.Mutex
	seen    []string
	accepts atomic.Int64
	udp     atomic.Int64
}

func (p *homepageAnnotationProbe) record(entry string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, entry)
}

func (p *homepageAnnotationProbe) report() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("tcp accepts=%d, udp datagrams=%d, requests=%v", p.accepts.Load(), p.udp.Load(), p.seen)
}

func (p *homepageAnnotationProbe) total() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.accepts.Load() + p.udp.Load() + int64(len(p.seen))
}

type homepageAnnotationCountingListener struct {
	net.Listener
	probe *homepageAnnotationProbe
}

func (l *homepageAnnotationCountingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.probe.accepts.Add(1)
	}
	return conn, err
}

// newHomepageAnnotationProbeServer starts a loopback HTTP server that counts
// TCP connections as well as requests, plus a UDP socket on loopback.
func newHomepageAnnotationProbeServer(t *testing.T) (srv *httptest.Server, probe *homepageAnnotationProbe, udpPort int) {
	t.Helper()
	probe = &homepageAnnotationProbe{}
	srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.record(r.Method + " " + r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	srv.Listener = &homepageAnnotationCountingListener{Listener: srv.Listener, probe: probe}
	srv.Start()
	t.Cleanup(srv.Close)

	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp probe: %v", err)
	}
	t.Cleanup(func() { _ = udp.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, err := udp.ReadFrom(buf); err != nil {
				return
			}
			probe.udp.Add(1)
		}
	}()
	return srv, probe, udp.LocalAddr().(*net.UDPAddr).Port
}

// TestCaptureHomepageAnnotationSnapshotReachesNothingBeyondThePreviewBrowser
// pins the egress boundary of the capture browser. The preview page tries every
// route to a second loopback server that a CDP request hook alone cannot see:
// service-worker install and activate fetches, a WebSocket handshake, speculation
// rules (prerender and prefetch), a dedicated worker, an image, WebTransport
// and a WebRTC STUN probe. The slow script keeps the load event open so the
// service worker runs during the capture.
func TestCaptureHomepageAnnotationSnapshotReachesNothingBeyondThePreviewBrowser(t *testing.T) {
	bin := homepageAnnotationTestBrowser(t)
	internal, probe, udpPort := newHomepageAnnotationProbeServer(t)
	internalHost := strings.TrimPrefix(internal.URL, "http://")
	fill := strings.NewReplacer("{{B}}", internal.URL, "{{BHOST}}", internalHost, "{{UDP}}", fmt.Sprint(udpPort))

	const pageHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<script type="speculationrules">{"prerender":[{"source":"list","urls":["{{B}}/prerender"]}],"prefetch":[{"source":"list","urls":["{{B}}/prefetch"]}]}</script>
</head><body style="margin:0"><h1>Probe</h1><a id="cta" href="/signup">Get started</a>
<img src="{{B}}/x.png" width="10" height="10" alt="probe">
<script>
try { navigator.serviceWorker.register('/sw.js').catch(function () {}); } catch (e) {}
try { new WebSocket('ws://{{BHOST}}/ws'); } catch (e) {}
try { new Worker('/w.js'); } catch (e) {}
try { fetch('{{B}}/page-fetch', {mode: 'no-cors'}).catch(function () {}); } catch (e) {}
try { var wt = new WebTransport('https://127.0.0.1:{{UDP}}/wt'); wt.ready.catch(function () {}); wt.closed.catch(function () {}); } catch (e) {}
try { var pc = new RTCPeerConnection({iceServers: [{urls: 'stun:127.0.0.1:{{UDP}}'}]}); pc.createDataChannel('x'); pc.createOffer().then(function (o) { return pc.setLocalDescription(o); }).catch(function () {}); } catch (e) {}
</script>
<script src="/slow.js"></script></body></html>`
	const swJS = `self.addEventListener('install', function (e) { e.waitUntil(fetch('{{B}}/sw-install', {mode: 'no-cors'}).catch(function () {})); });
self.addEventListener('activate', function (e) { e.waitUntil(fetch('{{B}}/sw-activate', {mode: 'no-cors'}).catch(function () {})); });
self.addEventListener('fetch', function () {});`
	const workerJS = `fetch('{{B}}/worker-fetch', {mode: 'no-cors'}).catch(function () {});`

	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(fill.Replace(pageHTML)))
		case "/sw.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(fill.Replace(swJS)))
		case "/w.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(fill.Replace(workerJS)))
		case "/slow.js":
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte("window.__slowDone = true;"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()

	target, _ := url.Parse(site.URL + "/")
	origin, _ := url.Parse(site.URL)
	snap, err := CaptureHomepageAnnotationSnapshot(context.Background(), HomepageAnnotationCaptureOptions{
		URL: target, AllowedOrigin: origin, Width: 1280, Height: 800, BrowserBin: bin,
	})
	if err != nil {
		t.Fatalf("capture: %v (probe: %s)", err, probe.report())
	}
	time.Sleep(700 * time.Millisecond)
	if probe.total() != 0 {
		t.Fatalf("the capture browser reached the internal server: %s", probe.report())
	}
	var cta bool
	for _, el := range snap.Elements {
		if el.Selector == "#cta" {
			cta = true
		}
	}
	if !cta {
		t.Fatalf("#cta missing from the element map: %+v", snap.Elements)
	}
}

// A page without a viewport meta tag must lay out at the emulated width. With
// mobile emulation Chrome lays such a page out at 980 px while the screenshot
// is clipped to the 390 px of the studio's phone preview, so boxes and image
// would disagree.
func TestCaptureHomepageAnnotationSnapshotNarrowViewportBrowser(t *testing.T) {
	bin := homepageAnnotationTestBrowser(t)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body style="margin:0"><a id="wide" href="/x" style="display:block;height:60px">Wide</a></body></html>`)
	}))
	defer site.Close()
	target, _ := url.Parse(site.URL + "/")
	origin, _ := url.Parse(site.URL)
	snap, err := CaptureHomepageAnnotationSnapshot(context.Background(), HomepageAnnotationCaptureOptions{
		URL: target, AllowedOrigin: origin, Width: 390, Height: 800, BrowserBin: bin,
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	for _, el := range snap.Elements {
		if el.Selector == "#wide" {
			if el.Box[2] != 390 {
				t.Fatalf("#wide box = %v, want a 390 px wide block", el.Box)
			}
			return
		}
	}
	t.Fatalf("#wide missing from the element map: %+v", snap.Elements)
}

func TestHomepageAnnotationNavigateErrorMapping(t *testing.T) {
	cases := []struct {
		err  string
		want error
	}{
		{"navigate failed: net::ERR_TIMED_OUT", ErrHomepageAnnotationTimeout},
		{"navigate failed: net::ERR_CONNECTION_TIMED_OUT", ErrHomepageAnnotationTimeout},
		{"navigate failed: net::ERR_BLOCKED_BY_CLIENT", ErrHomepageAnnotationBlocked},
		{"navigate failed: net::ERR_TUNNEL_CONNECTION_FAILED", ErrHomepageAnnotationBlocked},
		{"navigate failed: net::ERR_NAME_NOT_RESOLVED", ErrHomepageAnnotationNavigation},
	}
	for _, tc := range cases {
		got := homepageAnnotationNavigateError(context.Background(), errors.New(tc.err))
		if !errors.Is(got, tc.want) {
			t.Errorf("%s mapped to %v, want %v", tc.err, got, tc.want)
		}
	}
}
