package render

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

// probeScript is the script of the egress regression test. It plays the part of hostile scene code: it
// assigns location, calls window.open and fetch and tries every other way out, in the page realm and
// in a worker realm, against a loopback server (probe) that counts what reaches it. report is the
// path below the base the results are POSTed to.
func probeScript(probe, report string) string {
	return `(async () => {
  const BASE = location.href.replace(/\/$/, '');
  const P = '` + probe + `';
  const out = {};
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const attempt = async (name, fn) => {
    try { out[name] = String(await Promise.race([fn(), sleep(4000).then(() => 'timeout')])); } catch (e) { out[name] = 'fail'; }
  };
  const xhr = (u) => new Promise((res, rej) => { const x = new XMLHttpRequest(); x.open('GET', u); x.onload = () => res('ok ' + x.status); x.onerror = () => rej(new Error('x')); x.send(); });
  const ws = (u) => new Promise((res, rej) => { const s = new WebSocket(u); s.onopen = () => res('open'); s.onerror = () => rej(new Error('ws')); s.onclose = () => rej(new Error('closed')); });
  await attempt('control-boot', async () => (await fetch(BASE + '/boot.json')).status);
  await attempt('open', async () => window.open(P + '/p-open'));
  await attempt('fetch', async () => (await fetch(P + '/p-fetch')).status);
  await attempt('fetch-public', async () => (await fetch('https://example.com/')).status);
  await attempt('xhr', () => xhr(P + '/p-xhr'));
  await attempt('beacon', async () => navigator.sendBeacon(P + '/p-beacon', 'x'));
  await attempt('ws', () => ws('ws://' + P.slice(7) + '/p-ws'));
  await attempt('img', () => new Promise((res, rej) => { const i = new Image(); i.onload = () => res('ok'); i.onerror = () => rej(new Error('i')); i.src = P + '/p-img.png'; }));
  await attempt('script', () => new Promise((res, rej) => { const s = document.createElement('script'); s.onload = () => res('ok'); s.onerror = () => rej(new Error('s')); s.src = P + '/p-script.js'; document.head.appendChild(s); }));
  const workerSource = "self.onmessage = async (m) => { const BASE = m.data.base, P = m.data.probe, out = {};" +
    "const sleep = (ms) => new Promise((r) => setTimeout(r, ms));" +
    "const t = async (n, f) => { try { out[n] = String(await Promise.race([f(), sleep(3000).then(() => 'timeout')])); } catch (e) { out[n] = 'fail'; } };" +
    "await t('fetch', async () => (await fetch(P + '/w-fetch')).status);" +
    "await t('fetch-public', async () => (await fetch('https://example.com/')).status);" +
    "await t('importScripts', async () => { importScripts(P + '/w-import.js'); return 'ok'; });" +
    "await t('xhr', () => new Promise((res, rej) => { const x = new XMLHttpRequest(); x.open('GET', P + '/w-xhr'); x.onload = () => res('ok'); x.onerror = () => rej(new Error('x')); x.send(); }));" +
    "await t('ws', () => new Promise((res, rej) => { const s = new WebSocket('ws://' + P.slice(7) + '/w-ws'); s.onopen = () => res('open'); s.onerror = () => rej(new Error('ws')); s.onclose = () => rej(new Error('closed')); }));" +
    "await t('location', async () => { self.location = P + '/w-loc'; return 'assigned'; });" +
    "await t('control-import', async () => { importScripts(BASE + '/js/storymaker/player.bundle.js'); return 'ok'; });" +
    "postMessage(out); };";
  const worker = new Worker(URL.createObjectURL(new Blob([workerSource], {type: 'text/javascript'})));
  out.worker = await Promise.race([new Promise((res) => { worker.onmessage = (e) => res(e.data); worker.postMessage({base: BASE, probe: P}); }), sleep(15000).then(() => 'timeout')]);
  await fetch(BASE + '` + report + `', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({code: 'egress_probe', message: JSON.stringify(out)})});
  await sleep(300);
  location.href = P + '/p-nav';
})();
`
}

type probeOutcome struct {
	results map[string]any
	hits    []string
}

// hostAndPorts lists the paths that reached the probe, deduplicated and sorted.
func (p *probeServer) paths() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	seen := map[string]bool{}
	for _, h := range p.hits {
		if _, path, ok := strings.Cut(h, " "); ok {
			seen[path] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// runEgressVariant loads the probe page in a real Chrome configured as asked and returns what the page
// reported and what reached the probe server. withCSP serves the production CSP (with the sandbox),
// proxy puts the strict proxy in front of the browser.
func runEgressVariant(t *testing.T, chrome string, withCSP bool, proxy bool, tweak func(o *sessionOptions)) probeOutcome {
	t.Helper()
	probe := newProbeServer(t)
	var csp func(base string) string
	if withCSP {
		csp = RenderCSP
	}
	ps := newPageServer(t, probeScript(probe.URL, "/result"), csp)
	opts := sessionOptions{Bin: chrome, Profile: t.TempDir(), Base: ps.base(), Width: 320, Height: 180, NoGuard: true}
	if proxy {
		u, _ := url.Parse(ps.URL)
		p, err := security.StartStrictCaptureProxy(u, security.CaptureProxyLimits{MaxBytes: 64 << 20})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Close() })
		opts.ProxyAddr = p.Addr()
	}
	if tweak != nil {
		tweak(&opts)
	}
	return driveProbePage(t, opts, ps, probe, func() string { return ps.wait(t, 60*time.Second) })
}

func driveProbePage(t *testing.T, opts sessionOptions, ps *pageServer, probe *probeServer, wait func() string) probeOutcome {
	t.Helper()
	sess, err := openRodSession(context.Background(), opts)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer sess.Close()
	if err := sess.Navigate(context.Background(), ps.base()+"/"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	raw := wait()
	out := probeOutcome{}
	var wrapper struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil || json.Unmarshal([]byte(wrapper.Message), &out.results) != nil {
		t.Fatalf("page report %q: %v", raw, err)
	}
	// Give a navigation or a late request time to arrive before counting.
	time.Sleep(2 * time.Second)
	out.hits = probe.paths()
	if g := sess.guard; g != nil && !opts.NoGuard {
		t.Logf("guard %s: paused %d, blocked %d %v, workers %d guarded/%d failed, extra targets closed %d", g.Mode(), g.Seen(), g.Blocked(), g.BlockedHosts(), g.workers.Load(), g.workerFailed.Load(), g.extra.Load())
	}
	return out
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// leakPaths are the probe paths that stand for a finished request of scene code. The WebSocket paths
// are listed separately: the Fetch domain and the block list do not see WebSocket handshakes.
var leakPaths = []string{"/p-open", "/p-fetch", "/p-xhr", "/p-beacon", "/p-img.png", "/p-script.js", "/p-nav", "/w-fetch", "/w-import.js", "/w-xhr", "/w-loc"}
var wsPaths = []string{"/p-ws", "/w-ws"}

func leaked(hits []string, paths []string) []string {
	var out []string
	for _, p := range paths {
		if contains(hits, p) {
			out = append(out, p)
		}
	}
	return out
}

func workerResults(o probeOutcome) map[string]any {
	m, _ := o.results["worker"].(map[string]any)
	return m
}

// TestEgressLayersEachStopTheProbe is the browser regression test of the egress controls (spec step 2
// and "Browser egress regression"): hostile code assigns location, calls window.open and fetch to a
// public host and to a loopback server on another port, and tries XMLHttpRequest, WebSocket, images,
// scripts, a beacon and the same from a worker. First a control run shows that the probe would see
// every request, then each layer is tried alone, then all of them together.
func TestEgressLayersEachStopTheProbe(t *testing.T) {
	chrome := browserSmoke(t)

	t.Run("control: no layer at all lets the probe see the requests", func(t *testing.T) {
		o := runEgressVariant(t, chrome, false, false, nil)
		got := leaked(o.hits, []string{"/p-fetch", "/p-xhr", "/p-img.png", "/p-script.js", "/p-nav", "/w-fetch", "/w-import.js", "/w-xhr"})
		if len(got) < 6 {
			t.Fatalf("the harness saw only %v of the probe requests: hits %v results %v", got, o.hits, o.results)
		}
	})

	t.Run("strict proxy alone keeps every request off the probe", func(t *testing.T) {
		o := runEgressVariant(t, chrome, false, true, nil)
		if all := append(leaked(o.hits, leakPaths), leaked(o.hits, wsPaths)...); len(all) != 0 {
			t.Fatalf("the proxy let %v through (hits %v)", all, o.hits)
		}
		if o.results["control-boot"] != "200" {
			t.Errorf("the proxy broke the page's own origin: %v", o.results)
		}
	})

	t.Run("CSP and sandbox alone stop everything but a navigation", func(t *testing.T) {
		o := runEgressVariant(t, chrome, true, false, nil)
		got := leaked(o.hits, leakPaths)
		if len(got) != 1 || got[0] != "/p-nav" {
			t.Fatalf("with the CSP alone %v reached the probe (want only the navigation /p-nav): results %v hits %v", got, o.results, o.hits)
		}
		if ws := leaked(o.hits, wsPaths); len(ws) != 0 {
			t.Errorf("the CSP let a WebSocket handshake through: %v", ws)
		}
		if o.results["open"] != "null" {
			t.Errorf("window.open returned %v in the sandboxed page", o.results["open"])
		}
	})

	t.Run("CDP guard alone stops requests, workers and navigation", func(t *testing.T) {
		o := runEgressVariant(t, chrome, false, false, func(opts *sessionOptions) { opts.NoGuard = false })
		got := leaked(o.hits, leakPaths)
		// Without the sandbox a popup can open; it is another target, so its first request may land.
		var real []string
		for _, p := range got {
			if p != "/p-open" {
				real = append(real, p)
			}
		}
		if len(real) != 0 {
			t.Fatalf("the CDP guard let %v through: hits %v results %v", real, o.hits, o.results)
		}
		if o.results["control-boot"] != "200" {
			t.Errorf("the guard blocked the page's own origin: %v", o.results)
		}
	})

	for _, full := range []bool{false, true} {
		name := "all layers together (hybrid guard)"
		if full {
			name = "all layers together (catch-all guard)"
		}
		t.Run(name, func(t *testing.T) {
			o := runEgressVariant(t, chrome, true, true, func(opts *sessionOptions) { opts.NoGuard, opts.GuardFull = false, full })
			if all := append(leaked(o.hits, leakPaths), leaked(o.hits, wsPaths)...); len(all) != 0 {
				t.Fatalf("%v reached the probe: hits %v results %v", all, o.hits, o.results)
			}
			if o.results["open"] != "null" {
				t.Errorf("window.open = %v", o.results["open"])
			}
			for _, k := range []string{"fetch", "fetch-public", "xhr", "ws", "img", "script"} {
				if v := o.results[k]; v == nil || strings.HasPrefix(v.(string), "ok") || v == "open" || v == "timeout" {
					t.Errorf("page attempt %q = %v, want a failure", k, v)
				}
			}
			w := workerResults(o)
			if w == nil {
				t.Fatalf("the worker never answered: %v", o.results)
			}
			for _, k := range []string{"fetch", "fetch-public", "importScripts", "xhr", "ws"} {
				if v, _ := w[k].(string); v == "" || strings.HasPrefix(v, "ok") || v == "open" || v == "timeout" {
					t.Errorf("worker attempt %q = %v, want a failure", k, w[k])
				}
			}
			if o.results["control-boot"] != "200" || w["control-import"] != "ok" {
				t.Errorf("the page's own origin must stay reachable: page %v worker %v", o.results["control-boot"], w["control-import"])
			}
		})
	}
}
