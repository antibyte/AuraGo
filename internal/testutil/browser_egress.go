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
// browser behind the egress proxy must never connect to any of them.
var EgressPrivateTargets = []string{"169.254.169.254:80", "10.255.255.1:80", "192.168.255.1:80"}

// EgressPageReport is what the S8 page reports about itself.
type EgressPageReport struct {
	// CandidatePairs is the number of WebRTC candidate pairs the page saw; with
	// UDP forced through the proxy there is none. -1 means getStats failed.
	CandidatePairs int `json:"candidate_pairs"`
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
<script type="speculationrules">{"prerender":[{"source":"list","urls":["http://169.254.169.254/prerender"]}]}</script>
<link rel="prefetch" href="http://169.254.169.254/prefetch">
</head><body><h1>S8</h1>
<script>
(function () {
  function attempt(fn) { try { fn(); } catch (e) {} }
  var pc = null;
  var sw = {api: false, registered: false, error: '', state: ''};
  attempt(function () { window.open('http://169.254.169.254/latest/meta-data/'); });
  attempt(function () { new WebSocket('ws://10.255.255.1:80/'); });
  attempt(function () {
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
  attempt(function () {
    pc = new RTCPeerConnection({iceServers: [{urls: 'stun:10.255.255.1:3478'}]});
    pc.createDataChannel('x');
    pc.createOffer().then(function (o) { return pc.setLocalDescription(o); }).catch(function () {});
  });
  function report(n) {
    fetch('/report', {method: 'POST', body: JSON.stringify({
      candidate_pairs: n,
      secure_context: !!window.isSecureContext,
      worker_api: sw.api,
      worker_registered: sw.registered,
      worker_error: sw.error,
      worker_state: sw.state
    })}).catch(function () {});
  }
  setTimeout(function () {
    if (!pc) { report(0); return; }
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
