package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// EgressPrivateTargets are the host:port pairs the S8 page tries to reach. A
// browser behind the egress proxy must never connect to any of them. Each probe
// has a target of its own, so a refusal proves that probe and no other.
var EgressPrivateTargets = []string{
	"172.31.255.1:80",    // popup
	"10.255.255.5:80",    // WebSocket
	"169.254.169.254:80", // sendBeacon
	"10.255.255.1:80",    // download link
	"10.255.255.1:3478",  // WebRTC TURN over TCP
	"10.255.255.2:80",    // prefetch
	"10.255.255.3:80",    // prerender
	"192.168.255.1:80",   // service worker fetch
}

// EgressDNSCanaryHost is the host of the dns-prefetch hint. A DNS-level leak is
// NOT observable in the test: Chrome behind a proxy resolves nothing itself, and
// the proxy log only sees TCP destinations. The test can only prove that no
// request for this host reached the proxy.
const EgressDNSCanaryHost = "internal-canary.invalid"

// EgressPrivateRedirectTarget is where /redirect-private sends the browser.
const EgressPrivateRedirectTarget = "http://10.255.255.1/"

// EgressPageReport is what the S8 page reports about itself. Every probe sets a
// Ran flag after its call returned without throwing, so a probe that never
// executed cannot pass for a probe that was refused.
type EgressPageReport struct {
	// CandidatePairs is the number of WebRTC candidate pairs the page saw; with
	// UDP forced through the proxy there is none. -1 means getStats failed or
	// the connection was never created.
	CandidatePairs int `json:"candidate_pairs"`
	// PCCreated is true when the STUN RTCPeerConnection was constructed.
	PCCreated bool `json:"pc_created"`
	// PCTurnCreated is true when the TURN (tcp) RTCPeerConnection was constructed.
	PCTurnCreated bool `json:"pc_turn_created"`
	// SecureContext is window.isSecureContext of the page.
	SecureContext bool `json:"secure_context"`
	// WorkerAPI is false when navigator.serviceWorker does not exist.
	WorkerAPI bool `json:"worker_api"`
	// WorkerRegistered is true when navigator.serviceWorker.register resolved.
	WorkerRegistered bool `json:"worker_registered"`
	// WorkerError is the register() rejection, empty when it registered.
	WorkerError string `json:"worker_error"`
	// WorkerState is the state of the registered worker at report time
	// (installing, installed, activating, activated), empty when none.
	WorkerState string `json:"worker_state"`

	RanPopup     bool `json:"ran_popup"`
	PopupOpened  bool `json:"popup_opened"` // window.open returned a window
	RanWebSocket bool `json:"ran_websocket"`
	RanBeacon    bool `json:"ran_beacon"`
	BeaconQueued bool `json:"beacon_queued"` // sendBeacon returned true
	RanDownload  bool `json:"ran_download"`
	// RanPrefetch: the prefetch link was inserted. PrefetchEvent is "load" or
	// "error" when Chrome issued the request and "" when it never did.
	RanPrefetch   bool   `json:"ran_prefetch"`
	PrefetchEvent string `json:"prefetch_event"`
	// RanPrerender: the speculation rules script was inserted. Chrome gives the
	// page no signal whether it acted on the rules, so there is no event field.
	RanPrerender   bool `json:"ran_prerender"`
	PrerenderAPI   bool `json:"prerender_api"` // Chrome knows speculation rules
	RanDNSPrefetch bool `json:"ran_dns_prefetch"`
}

// EgressFixture serves the S8 page on IPv4 loopback.
type EgressFixture struct {
	Server *httptest.Server

	mu       sync.Mutex
	report   EgressPageReport
	got      chan struct{}
	once     sync.Once
	requests []string
}

const egressFixturePage = `<!doctype html><html><head><meta charset="utf-8">
</head><body><h1>S8</h1>
<script>
(function () {
  var ran = {};
  function attempt(name, fn) { try { fn(); ran[name] = true; } catch (e) { ran[name] = false; } }
  var pc = null, pcTurn = null, popup = false, beacon = false, prefetchEvent = '', prerenderAPI = false;
  var sw = {api: false, registered: false, error: '', state: ''};
  attempt('popup', function () { popup = !!window.open('http://172.31.255.1/latest/meta-data/'); });
  attempt('websocket', function () { new WebSocket('ws://10.255.255.5:80/'); });
  attempt('beacon', function () { beacon = navigator.sendBeacon('http://169.254.169.254/beacon', 'x'); });
  attempt('download', function () {
    // Chrome ignores download= for a cross-origin href and navigates instead, so
    // the click happens in an iframe: the frame fails, the report page survives.
    var f = document.createElement('iframe');
    document.body.appendChild(f);
    var a = f.contentDocument.createElement('a');
    a.href = 'http://10.255.255.1/file';
    a.download = 'file';
    f.contentDocument.body.appendChild(a);
    a.click();
  });
  attempt('prefetch', function () {
    var l = document.createElement('link');
    l.rel = 'prefetch';
    l.href = 'http://10.255.255.2/prefetch';
    l.onload = function () { prefetchEvent = 'load'; };
    l.onerror = function () { prefetchEvent = 'error'; };
    document.head.appendChild(l);
  });
  attempt('prerender', function () {
    prerenderAPI = !!(window.HTMLScriptElement && HTMLScriptElement.supports && HTMLScriptElement.supports('speculationrules'));
    var s = document.createElement('script');
    s.type = 'speculationrules';
    s.textContent = JSON.stringify({prerender: [{source: 'list', urls: ['http://10.255.255.3/prerender']}]});
    document.head.appendChild(s);
  });
  attempt('dns_prefetch', function () {
    var l = document.createElement('link');
    l.rel = 'dns-prefetch';
    l.href = '//internal-canary.invalid';
    document.head.appendChild(l);
  });
  attempt('worker', function () {
    sw.api = !!navigator.serviceWorker;
    if (!sw.api) { sw.error = 'navigator.serviceWorker is undefined'; return; }
    navigator.serviceWorker.register('/sw.js').then(function (reg) {
      sw.registered = true;
      var w = reg.installing || reg.waiting || reg.active;
      if (w) {
        sw.state = w.state;
        w.addEventListener('statechange', function () { sw.state = w.state; });
      }
    }).catch(function (e) { sw.error = String(e && e.message || e); });
  });
  attempt('pc', function () {
    pc = new RTCPeerConnection({iceServers: [{urls: 'stun:10.255.255.1:3478'}]});
    pc.createDataChannel('x');
    pc.createOffer().then(function (o) { return pc.setLocalDescription(o); }).catch(function () {});
  });
  attempt('pc_turn', function () {
    pcTurn = new RTCPeerConnection({iceServers: [{urls: 'turn:10.255.255.1:3478?transport=tcp', username: 'u', credential: 'c'}]});
    pcTurn.createDataChannel('x');
    pcTurn.createOffer().then(function (o) { return pcTurn.setLocalDescription(o); }).catch(function () {});
  });
  function report(n) {
    fetch('/report', {method: 'POST', body: JSON.stringify({
      candidate_pairs: n,
      pc_created: !!pc,
      pc_turn_created: !!pcTurn,
      secure_context: !!window.isSecureContext,
      worker_api: sw.api,
      worker_registered: sw.registered,
      worker_error: sw.error,
      worker_state: sw.state,
      ran_popup: !!ran.popup,
      popup_opened: popup,
      ran_websocket: !!ran.websocket,
      ran_beacon: !!ran.beacon,
      beacon_queued: !!beacon,
      ran_download: !!ran.download,
      ran_prefetch: !!ran.prefetch,
      prefetch_event: prefetchEvent,
      ran_prerender: !!ran.prerender,
      prerender_api: prerenderAPI,
      ran_dns_prefetch: !!ran.dns_prefetch
    })}).catch(function () {});
  }
  setTimeout(function () {
    if (!pc) { report(-1); return; }
    pc.getStats().then(function (stats) {
      var n = 0;
      stats.forEach(function (s) { if (s.type === 'candidate-pair') { n++; } });
      report(n);
    }).catch(function () { report(-1); });
  }, 6000);
})();
</script></body></html>`

// The worker tells the fixture server (same origin, so it passes the proxy)
// that it ran and how its fetch of a private address ended.
const egressFixtureWorker = `self.addEventListener('install', function (e) {
  e.waitUntil(
    fetch('/sw-ping?install').then(function () {
      return fetch('http://192.168.255.1/').then(
        function () { return fetch('/sw-ping?private=answered'); },
        function (err) { return fetch('/sw-ping?private=failed'); });
    }).catch(function () {}));
});
self.addEventListener('fetch', function () {});`

// NewEgressFixture starts the S8 page and lets the proxy dial loopback
// (AURAGO_SSRF_ALLOW_LOOPBACK=1, loopback only) so the page itself loads; every
// private target above stays refused.
func NewEgressFixture(t testing.TB) *EgressFixture {
	t.Helper()
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	f := &EgressFixture{got: make(chan struct{})}
	f.Server = NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
		f.mu.Unlock()
		switch {
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, egressFixturePage)
		case r.URL.Path == "/redirect-private":
			http.Redirect(w, r, EgressPrivateRedirectTarget, http.StatusFound)
		case r.URL.Path == "/sw.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = io.WriteString(w, egressFixtureWorker)
		case r.URL.Path == "/sw-ping":
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/report" && r.Method == http.MethodPost:
			var rep EgressPageReport
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&rep); err == nil {
				f.mu.Lock()
				f.report = rep
				f.mu.Unlock()
				f.once.Do(func() { close(f.got) })
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Server.Close)
	return f
}

// URL is the page address.
func (f *EgressFixture) URL() string { return f.Server.URL + "/" }

// RedirectURL is a page on the fixture origin that answers 302 to a private
// address (EgressPrivateRedirectTarget).
func (f *EgressFixture) RedirectURL() string { return f.Server.URL + "/redirect-private" }

// HostPort is the page's own host:port, as the proxy logs it.
func (f *EgressFixture) HostPort() string { return strings.TrimPrefix(f.Server.URL, "http://") }

// Requests lists "METHOD uri" of every request the fixture server received.
func (f *EgressFixture) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

// WorkerPings returns the query strings of the /sw-ping requests the service
// worker sent (for example "install" and "private=failed").
func (f *EgressFixture) WorkerPings() []string {
	var out []string
	for _, req := range f.Requests() {
		if rest, ok := strings.CutPrefix(req, "GET /sw-ping?"); ok {
			if q, err := url.QueryUnescape(rest); err == nil {
				rest = q
			}
			out = append(out, rest)
		}
	}
	return out
}

// WaitReport blocks until the page has reported, or fails the test.
func (f *EgressFixture) WaitReport(t testing.TB, timeout time.Duration) EgressPageReport {
	t.Helper()
	select {
	case <-f.got:
	case <-time.After(timeout):
		t.Fatalf("the S8 page did not report within %s", timeout)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.report
}

// RequireBrowserBin skips the test unless AURAGO_RUN_BROWSER_SMOKE=1 and a
// Chrome or Edge can be found. AURAGO_TEST_CHROME names a binary explicitly.
func RequireBrowserBin(t testing.TB) string {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to launch Chrome/Edge")
	}
	for _, candidate := range []string{
		os.Getenv("AURAGO_TEST_CHROME"),
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
	} {
		if candidate == "" {
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved
		}
	}
	t.Skip("Chrome/Edge unavailable")
	return ""
}
