[← Plan index](README.md) · Spec: [plans/2026-11-11-easydrag-s0b-spec.md](../2026-11-11-easydrag-s0b-spec.md)

## Part A: Browser egress (FD-18)

Spec: `plans/2026-11-11-easydrag-s0b-spec.md` §3 (design), §7 (tests), §8 (A1-A6), §9 (acceptance). The tasks run in the order A.1 to A.6. A.1 can stop the whole part.

**Shell.** Git Bash. Every command starts with `cd <worktree> && ...` because the working directory resets between calls. Set `MSYS_NO_PATHCONV=1` for any argument that starts with `//`.

**Worktree.** `WT` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress`.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0b-egress -b feat/easydrag-s0b-egress main
mkdir -p C:/disposable/easydrag-s0b D:/tmp/go-build D:/tmp/easydrag-s0b-shots
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git log -1 --format=%h && git status --short
```

Expected: the worktree is created from `main` (805c775f7 or later); `git status --short` prints nothing.

**Baseline.** Run it once before A.1. Any failure that is already on main is reported to the coordinator, not fixed here.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ ./internal/scraper/ ./internal/tools/ -count=1 -timeout 15m 2>&1 | tail -30
```

Expected: three `ok` lines (the tools package takes several minutes). Copy any `FAIL` line into your report.

**Conventions of this part.**
- Go temp dir: `GOTMPDIR='D:\tmp\go-build'` for `internal/security`, `internal/scraper`, `internal/tools`.
- Chrome tests are gated by `AURAGO_RUN_BROWSER_SMOKE=1`. Chrome cannot start from a binary under the repo here (crashpad), so run them with the Bash tool's sandbox disabled and with `GOTMPDIR='D:\tmp\go-build'`. The test binary then lives in `D:\tmp\go-build`, which is the working recipe (`reference_chrome_launch_from_agent`).
- Escapes: the Write and Edit tools turn backslash-u sequences into characters. This part has none; keep it that way.
- Commit: one per task. `git add <exact paths>`, then `git commit -F C:/disposable/easydrag-s0b/<message file>`. Write each message file with the Write tool. Every message ends with the GitNexus line and the Co-Authored-By line; the implementer replaces `<your model line>` with the model it runs as.
- Gates before every Go commit: `go vet` on the touched packages and `gofmt -l` on the touched files (it must print nothing).
- GitNexus cannot index a worktree on this machine. Run IMPACT from the main checkout path (command in each task); HIGH or CRITICAL is reported, UNKNOWN is confirmed with `grep -rn`.

**Deviations from the spec that this part needs (each is also in the final report to the coordinator).**
1. `EgressBrowserOptions` gets two additive fields beyond spec §3.2: `Bin string` (the browser binary; the homepage capture already takes one for tests) and `Sandbox bool` (true removes `--no-sandbox` even in a container, which is today's `NoSandbox(false)` of web capture, web performance and form automation; spec §3.2 says "the existing sandbox flags of each site, kept as they are", and the sites differ).
2. `EgressBrowser` gets `Wrap(err) error`, `ProxyDialed()` and `ProxyRefused()`; the proxy gets `Dialed()` and `Refused()` logs. Spec §3.6 says "the proxy logs each private target" and "reads the proxy's refusal log", but main's proxy has no log. The two site tests of A.5 need the log of a proxy that the site creates internally, so `SetEgressLaunchObserver` (tests only) hands them the launched `EgressBrowser`.
3. The proxy's check answer carries CORS headers so the helper can read the nonce with `fetch` from a blank page (a 204 navigation does not commit, so a navigation cannot show the header).
4. The slot wait is capped at 2 minutes when the caller's context has no deadline (`ErrEgressBusy`), so a fifth caller cannot hang for ever.

---

### Task A.1: Spike, does Chrome answer the proxy challenge over DevTools?

**Files:**
- Create (untracked, deleted before the commit): `WT/internal/security/egress_auth_spike_test.go`
- Create (outputs, outside the repo): `C:/disposable/easydrag-s0b/a1-spike-result.txt`, `C:/disposable/easydrag-s0b/a1-amendment.md`
- Modify: `WT/plans/2026-11-11-easydrag-s0b-spec.md` (an amendment after the "Spike first" paragraph of §3.3)

**Where the spike lives and why.** It is a Go test in the worktree's `internal/security`, not under `C:/disposable/easydrag-s0b/`: it needs the module's `go-rod/rod` dependency and a `go.mod`, and a file in the disposable folder has neither. A build tag `spike` keeps it out of every normal `go test` and `go vet`. The file stays untracked and is deleted before the commit. The spike carries its own 40-line authenticating proxy, because A.2 has not been written yet.

**What it measures.** For each request kind it starts a fresh proxy and a fresh Chrome, so no cached proxy credential can hide a failure. Its proxy answers the first request to every key with 407 even if credentials are present (this forces a real challenge per kind, because Chrome caches proxy credentials per profile), then lets the authenticated retry pass. A kind passes when the target server was reached after a challenge and an authenticated retry. Two variants are tried:
- `browser-wide`: `Fetch.enable{handleAuthRequests:true}` on the browser target, then an `EachEvent` loop that answers every `Fetch.authRequired` and continues every `Fetch.requestPaused`. This is what rod's `HandleAuth` does, but `HandleAuth` answers one challenge per call (it waits once), so the spike loops.
- `browser-wide+attach`: the same, plus `Target.setAutoAttach` (flatten, wait for debugger) so every page, worker and service worker session gets its own `Fetch.enable{handleAuthRequests:true}`.

Kinds: `nav` (top-level navigation), `subresource` (img), `fetch`, `xhr`, `websocket`, `worker` (dedicated worker fetch), `serviceworker` (fetch in the service worker install event), `prefetch` (speculation-rules prefetch).

- [ ] **Step 1: Confirm the rod API used below.**

```bash
cd C:/Users/Andi/go/pkg/mod/github.com/go-rod/rod@v0.116.2 && grep -n "func (b \*Browser) HandleAuth\|func (b \*Browser) EachEvent\|func (b \*Browser) Call" hijack.go browser.go && grep -n "type FetchEnable struct\|type FetchAuthRequired struct\|type FetchContinueWithAuth struct\|type TargetSetAutoAttach struct\|type TargetAttachedToTarget struct" lib/proto/fetch.go lib/proto/target.go
```

Expected: five type lines and three method lines (`hijack.go:390` for `HandleAuth`). If a name differs, stop and fix the spike code below to the real name before running it.

- [ ] **Step 2: Write the spike.** Create `WT/internal/security/egress_auth_spike_test.go`:

```go
//go:build spike

package security

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var spikeKinds = []string{"nav", "subresource", "fetch", "xhr", "websocket", "worker", "serviceworker", "prefetch"}
var spikeVariants = []string{"browser-wide", "browser-wide+attach"}

// spikeProxy forces a 407 on the first request to every key, so each kind of
// request has to answer a real challenge even when Chrome cached credentials.
type spikeProxy struct {
	ln         net.Listener
	want       string
	mu         sync.Mutex
	challenged map[string]bool
	authed     map[string]bool
}

func newSpikeProxy(t *testing.T, user, pass string) *spikeProxy {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	p := &spikeProxy{
		ln:         ln,
		want:       "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass)),
		challenged: map[string]bool{},
		authed:     map[string]bool{},
	}
	srv := &http.Server{Handler: p}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return p
}

func (p *spikeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Host
	if r.Method != http.MethodConnect {
		key = r.URL.Host + r.URL.Path
	}
	ok := r.Header.Get("Proxy-Authorization") == p.want
	p.mu.Lock()
	first := !p.challenged[key]
	p.challenged[key] = true
	pass := ok && !first
	if pass {
		p.authed[key] = true
	}
	p.mu.Unlock()
	if !pass {
		w.Header().Set("Proxy-Authenticate", `Basic realm="spike"`)
		w.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	if r.Method == http.MethodConnect {
		spikeTunnel(w, r)
		return
	}
	out := r.Clone(r.Context())
	out.RequestURI = ""
	out.Header.Del("Proxy-Authorization")
	res, err := (&http.Transport{Proxy: nil}).RoundTrip(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	for k, v := range res.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func spikeTunnel(w http.ResponseWriter, r *http.Request) {
	up, err := net.DialTimeout("tcp", r.Host, 5*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = up.Close()
		return
	}
	c, rw, err := hj.Hijack()
	if err != nil {
		_ = up.Close()
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	_ = rw.Flush()
	go func() { _, _ = io.Copy(up, rw); _ = up.Close() }()
	_, _ = io.Copy(c, up)
	_ = c.Close()
}

func (p *spikeProxy) state(match func(key string) bool) (challenged, authed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k := range p.challenged {
		if match(k) {
			challenged = true
			if p.authed[k] {
				authed = true
			}
		}
	}
	return
}

func spikePage(kind, wsAddr string) string {
	body := map[string]string{
		"nav":           "",
		"subresource":   `<img src="/k/subresource">`,
		"fetch":         `<script>fetch('/k/fetch').catch(function(){})</script>`,
		"xhr":           `<script>var x=new XMLHttpRequest();x.open('GET','/k/xhr');x.send()</script>`,
		"websocket":     `<script>try{new WebSocket('ws://` + wsAddr + `/k/websocket')}catch(e){}</script>`,
		"worker":        `<script>new Worker('/w.js')</script>`,
		"serviceworker": `<script>navigator.serviceWorker.register('/sw.js').catch(function(){})</script>`,
		"prefetch":      `<script type="speculationrules">{"prefetch":[{"source":"list","urls":["/k/prefetch"],"eagerness":"immediate"}]}</script>`,
	}[kind]
	return "<!doctype html><html><body>" + body + "</body></html>"
}

type spikeSession struct {
	b   *rod.Browser
	sid proto.TargetSessionID
}

func (c spikeSession) Call(ctx context.Context, _, method string, params interface{}) ([]byte, error) {
	return c.b.Call(ctx, string(c.sid), method, params)
}

func spikeAuthenticate(t *testing.T, b *rod.Browser, variant, user, pass string, answered *atomic.Int64, info *atomic.Value) {
	t.Helper()
	handlers := []interface{}{
		func(e *proto.FetchRequestPaused, sid proto.TargetSessionID) {
			go func() { _ = proto.FetchContinueRequest{RequestID: e.RequestID}.Call(spikeSession{b, sid}) }()
		},
		func(e *proto.FetchAuthRequired, sid proto.TargetSessionID) {
			if c := e.AuthChallenge; c != nil {
				info.Store(fmt.Sprintf("source=%s origin=%s scheme=%s realm=%s", c.Source, c.Origin, c.Scheme, c.Realm))
			}
			go func() {
				err := proto.FetchContinueWithAuth{
					RequestID: e.RequestID,
					AuthChallengeResponse: &proto.FetchAuthChallengeResponse{
						Response: proto.FetchAuthChallengeResponseResponseProvideCredentials,
						Username: user,
						Password: pass,
					},
				}.Call(spikeSession{b, sid})
				if err == nil {
					answered.Add(1)
				}
			}()
		},
	}
	attach := variant == "browser-wide+attach"
	if attach {
		handlers = append(handlers, func(e *proto.TargetAttachedToTarget, _ proto.TargetSessionID) {
			go func() {
				s := spikeSession{b, e.SessionID}
				_ = proto.FetchEnable{HandleAuthRequests: true}.Call(s)
				_ = proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}.Call(s)
				if e.WaitingForDebugger {
					_ = proto.RuntimeRunIfWaitingForDebugger{}.Call(s)
				}
			}()
		})
	}
	if err := (proto.FetchEnable{HandleAuthRequests: true}).Call(b); err != nil {
		t.Fatalf("Fetch.enable: %v", err)
	}
	go b.EachEvent(handlers...)()
	if attach {
		if err := (proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}).Call(b); err != nil {
			t.Fatalf("Target.setAutoAttach: %v", err)
		}
	}
}

func spikeChromeBin(t *testing.T) string {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	for _, c := range []string{os.Getenv("AURAGO_TEST_CHROME"), `C:\Program Files\Google\Chrome\Application\chrome.exe`, "google-chrome", "chromium"} {
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	t.Skip("no Chrome found")
	return ""
}

var spikeChromeVersion atomic.Value

// runSpikeCase returns a status word and a detail text.
func runSpikeCase(t *testing.T, bin, variant, kind string) (string, string) {
	hits := &sync.Map{}
	var wsHits atomic.Int64

	wsLn, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ws listen: %v", err)
	}
	t.Cleanup(func() { _ = wsLn.Close() })
	go func() {
		for {
			c, err := wsLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				req, err := http.ReadRequest(bufio.NewReader(c))
				if err != nil || !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
					return
				}
				wsHits.Add(1)
				sum := sha1.Sum([]byte(req.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
				fmt.Fprintf(c, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
				time.Sleep(500 * time.Millisecond)
			}(c)
		}
	}()
	wsAddr := wsLn.Addr().String()

	targetLn, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("target listen: %v", err)
	}
	target := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/k/nav":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, spikePage(kind, wsAddr))
		case r.URL.Path == "/w.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = io.WriteString(w, "fetch('/k/worker').catch(function(){});")
		case r.URL.Path == "/sw.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = io.WriteString(w, "self.addEventListener('install',function(e){e.waitUntil(fetch('/k/sw').catch(function(){}))});")
		default:
			hits.Store(r.URL.Path, true)
			_, _ = io.WriteString(w, "ok")
		}
	})}
	go func() { _ = target.Serve(targetLn) }()
	t.Cleanup(func() { _ = target.Close() })

	user, pass := "u"+strconv.FormatInt(time.Now().UnixNano(), 16), "p"+strconv.FormatInt(time.Now().UnixNano()/7, 16)
	proxy := newSpikeProxy(t, user, pass)
	l := launcher.New().Bin(bin).Headless(true).
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Set("webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Proxy("http://" + proxy.ln.Addr().String()).
		Set("proxy-bypass-list", "<-loopback>")
	controlURL, err := l.Launch()
	if err != nil {
		return "FAIL", "launch: " + err.Error()
	}
	t.Cleanup(func() { l.Kill(); l.Cleanup() })
	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return "FAIL", "connect: " + err.Error()
	}
	t.Cleanup(func() { _ = b.Close() })
	if v, err := (proto.BrowserGetVersion{}).Call(b); err == nil {
		spikeChromeVersion.Store(v.Product)
	}

	var answered atomic.Int64
	var info atomic.Value
	spikeAuthenticate(t, b, variant, user, pass, &answered, &info)

	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return "FAIL", "page: " + err.Error()
	}
	navURL := "http://" + targetLn.Addr().String() + "/k/nav"
	navErr := page.Timeout(20 * time.Second).Navigate(navURL)
	settle := 7 * time.Second
	if s, err := strconv.Atoi(os.Getenv("AURAGO_SPIKE_SETTLE")); err == nil {
		settle = time.Duration(s) * time.Second
	}
	time.Sleep(settle)

	_, navAuthed := proxy.state(func(k string) bool { return strings.HasSuffix(k, "/k/nav") })
	if !navAuthed {
		return "FAIL", fmt.Sprintf("navigation was not authenticated (navigate error: %v) answered=%d", navErr, answered.Load())
	}
	match := func(k string) bool { return strings.HasSuffix(k, "/k/"+kind) }
	hit := false
	switch kind {
	case "websocket":
		match = func(k string) bool { return k == wsAddr || strings.HasSuffix(k, "/k/websocket") }
		hit = wsHits.Load() > 0
	case "serviceworker":
		match = func(k string) bool { return strings.HasSuffix(k, "/k/sw") }
		_, hit = hits.Load("/k/sw")
	case "nav":
		hit = true
	default:
		_, hit = hits.Load("/k/" + kind)
	}
	ch, au := proxy.state(match)
	detail := fmt.Sprintf("answered=%d challenge[%v]", answered.Load(), info.Load())
	switch {
	case ch && au && hit:
		return "PASS", detail
	case ch && !au:
		return "FAIL", "challenge not answered; " + detail
	case !ch && !au:
		return "NOT-OBSERVED", detail
	}
	return "FAIL", "authenticated but target not reached; " + detail
}

func TestSpikeProxyAuth(t *testing.T) {
	bin := spikeChromeBin(t)
	results := map[string]string{}
	var lines []string
	for _, variant := range spikeVariants {
		for _, kind := range spikeKinds {
			variant, kind := variant, kind
			t.Run(variant+"/"+kind, func(t *testing.T) {
				status, detail := runSpikeCase(t, bin, variant, kind)
				results[variant+"/"+kind] = status
				lines = append(lines, fmt.Sprintf("%-20s %-14s %-13s %s", variant, kind, status, detail))
				if status == "FAIL" {
					t.Errorf("%s", detail)
				}
			})
		}
	}
	text := strings.Join(lines, "\n") + "\n"
	if out := os.Getenv("AURAGO_SPIKE_OUT"); out != "" {
		_ = os.WriteFile(out, []byte(text), 0o644)
	}
	t.Log("\n" + text)

	// Verdict: a variant wins when every kind passes (prefetch may stay unobserved:
	// a request Chrome never issues cannot leak, and A.5 re-checks it).
	winner := ""
	for _, variant := range spikeVariants {
		all := true
		for _, kind := range spikeKinds {
			s := results[variant+"/"+kind]
			if s != "PASS" && !(kind == "prefetch" && s == "NOT-OBSERVED") {
				all = false
			}
		}
		if all && winner == "" {
			winner = variant
		}
	}
	var md strings.Builder
	version, _ := spikeChromeVersion.Load().(string)
	fmt.Fprintf(&md, "\n**A.1 spike result (%s, %s, go-rod v0.116.2).** Each cell is one fresh Chrome and one fresh proxy; the proxy forces a 407 on the first request to every key.\n\n", time.Now().Format("2006-01-02"), version)
	md.WriteString("| kind | browser-wide | browser-wide+attach |\n|---|---|---|\n")
	for _, kind := range spikeKinds {
		fmt.Fprintf(&md, "| %s | %s | %s |\n", kind, results["browser-wide/"+kind], results["browser-wide+attach/"+kind])
	}
	if winner != "" {
		fmt.Fprintf(&md, "\nVerdict: every kind authenticates with variant `%s`. Plan task A.3 sets `egressPerTargetAuth = %v`. Section 3.3 stands as written.\n", winner, winner == "browser-wide+attach")
	} else {
		md.WriteString("\nVerdict: STOP. No variant authenticates every kind (see the table). The plan waits for the owner before A.2.\n")
	}
	if out := os.Getenv("AURAGO_SPIKE_AMENDMENT"); out != "" {
		_ = os.WriteFile(out, []byte(md.String()), 0o644)
	}
}
```

- [ ] **Step 3: Run the spike.** Bash tool with the sandbox disabled (Chrome recipe). It takes about 2 to 3 minutes (16 cases of about 9 s).

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SPIKE_OUT='C:/disposable/easydrag-s0b/a1-spike-result.txt' AURAGO_SPIKE_AMENDMENT='C:/disposable/easydrag-s0b/a1-amendment.md' GOTMPDIR='D:\tmp\go-build' go test -tags spike ./internal/security/ -run TestSpikeProxyAuth -count=1 -v -timeout 10m 2>&1 | tail -60
```

Expected output: 16 `--- PASS/FAIL: TestSpikeProxyAuth/<variant>/<kind>` lines and a table such as `browser-wide  nav  PASS  answered=1 challenge[source=Proxy origin=http://127.0.0.1:NNNNN scheme=basic realm=spike]`. A compile error means a name from Step 1 differs; fix it and rerun. If every case says `FAIL launch:`, Chrome cannot start from here: re-read the Chrome recipe above (sandbox off, test binary in `D:\tmp\go-build`).

My expectation, to be checked and not assumed: `nav`, `subresource`, `fetch`, `xhr`, `websocket` pass with `browser-wide`; `worker`, `serviceworker` and possibly `prefetch` fail there and pass only with `browser-wide+attach`.

- [ ] **Step 4: Read the result and apply the decision rule.**

```bash
cat C:/disposable/easydrag-s0b/a1-amendment.md
```

The rule:
1. **A variant has all kinds PASS** (prefetch may be NOT-OBSERVED): continue. Record which variant won. A.3 sets `egressPerTargetAuth` to `true` only for `browser-wide+attach`. The challenge line shows the real `source` and `origin` of a proxy challenge; A.3 checks `source=Proxy` and that the origin contains the proxy address, so confirm both are visible in the output (if `source` is empty, A.3's check must drop the source test; report this).
2. **A kind is NOT-OBSERVED** (other than prefetch): rerun only that cell with a longer settle time, once: `... go test -tags spike ./internal/security/ -run 'TestSpikeProxyAuth/browser-wide/serviceworker' ...` with `AURAGO_SPIKE_SETTLE=15`. Still not observed: treat it as FAIL.
3. **No variant passes every kind (any FAIL left):** do not start A.2. Still do Steps 5 to 7 (record the findings in the spec), then stop and send NEEDS_CONTEXT to the coordinator listing the failed kinds per variant, with the line from `a1-spike-result.txt` for each. A request that cannot authenticate still fails closed; this is a functional question for the owner, not a security hole.

- [ ] **Step 5: Delete the spike file.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && rm internal/security/egress_auth_spike_test.go && git status --short
```

Expected: nothing printed.

- [ ] **Step 6: Insert the amendment after the "Spike first" paragraph of §3.3.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && sed -i '/^  If any of them cannot authenticate, the plan stops and asks the owner\./r C:/disposable/easydrag-s0b/a1-amendment.md' plans/2026-11-11-easydrag-s0b-spec.md && git diff --stat && sed -n 99,125p plans/2026-11-11-easydrag-s0b-spec.md
```

Expected: one file changed with about 15 insertions; the table follows the "If any of them cannot authenticate" line. (The spec writes that sentence indented by two spaces inside the bullet; the `sed` address matches it. If `git diff --stat` shows no change, print `grep -n "cannot authenticate" plans/2026-11-11-easydrag-s0b-spec.md` and adjust the address to the real text.)

- [ ] **Step 7: Commit.** Write `C:/disposable/easydrag-s0b/a1-commit.txt` with the Write tool:

```
docs(plans): S0b A.1 spike result, proxy authentication over DevTools

Records which request kinds (navigation, subresource, fetch, XHR, WebSocket,
dedicated worker, service worker, speculation-rules prefetch) answer a proxy
407 challenge through Fetch.authRequired in Chrome via go-rod v0.116.2, for
a browser-wide Fetch.enable and for a per-target auto-attach variant. The
spike code is a throwaway and is not part of this commit.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git add plans/2026-11-11-easydrag-s0b-spec.md && git commit -F C:/disposable/easydrag-s0b/a1-commit.txt && git log -1 --stat --format=%s
```

Expected: one file changed. If the verdict was STOP, report NEEDS_CONTEXT now and do not continue.

---

### Task A.2: Capture proxy credentials, `Done()`, the check host and the dial log

**Files:**
- Modify: `internal/security/capture_proxy.go` (imports; consts at `:20-33`; struct at `:77-100`; start functions at `:102-196`; `Addr` at `:199`; `ServeHTTP` at `:217`; `dialUpstream` at `:312`)
- Create: `internal/security/capture_proxy_auth_test.go`

**IMPACT first** (main checkout path; the signatures stay source compatible because the new parameter is variadic):

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "StartCaptureProxy" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "StartStrictCaptureProxy" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "ServeHTTP" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Known callers (read from the code): `homepage_annotation_capture.go:116` (StartCaptureProxy), `internal/storymaker/render/render.go:500`, `compilecheck.go:164`, `egress_smoke_test.go:96` (StartStrictCaptureProxy). Warn on HIGH or CRITICAL; the storymaker callers keep compiling and behave the same without options.

- [ ] **Step 1: Write the failing tests.** Create `internal/security/capture_proxy_auth_test.go`:

```go
package security

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

const (
	testProxyUser = "egress-user"
	testProxyPass = "egress-pass"
)

func basicProxyAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func startAuthProxy(t *testing.T, allowed *countedServer, opts ...CaptureProxyOption) *CaptureProxy {
	t.Helper()
	var origin *url.URL
	if allowed != nil {
		origin = mustParseURL(t, allowed.URL)
	}
	opts = append([]CaptureProxyOption{WithProxyCredentials(testProxyUser, testProxyPass)}, opts...)
	p, err := StartCaptureProxy(origin, CaptureProxyLimits{}, opts...)
	if err != nil {
		t.Fatalf("StartCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// authProxyClient talks through p; with creds it sends the proxy credentials
// (net/http adds Proxy-Authorization for a proxy URL with user info).
func authProxyClient(t *testing.T, p *CaptureProxy, creds bool) *http.Client {
	t.Helper()
	proxyURL := mustParseURL(t, "http://"+p.Addr())
	if creds {
		proxyURL.User = url.UserPassword(testProxyUser, testProxyPass)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func connectStatus(t *testing.T, p *CaptureProxy, target, auth string) int {
	t.Helper()
	conn, err := net.DialTimeout("tcp", p.Addr(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
	if auth != "" {
		req += "Proxy-Authorization: " + auth + "\r\n"
	}
	fmt.Fprint(conn, req+"\r\n")
	res, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT reply: %v", err)
	}
	return res.StatusCode
}

func TestCaptureProxyWithCredentialsAnswers407WithoutThem(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startAuthProxy(t, allowed)

	res, err := authProxyClient(t, p, false).Get(allowed.URL + "/x")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("status = %d, want 407", res.StatusCode)
	}
	if got := res.Header.Get("Proxy-Authenticate"); got != `Basic realm="aurago-egress"` {
		t.Fatalf("Proxy-Authenticate = %q", got)
	}
	if allowed.accepts.Load() != 0 {
		t.Fatalf("the origin was dialled %d times without credentials", allowed.accepts.Load())
	}
}

func TestCaptureProxyWithCredentialsPassesWithThem(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startAuthProxy(t, allowed)

	res, err := authProxyClient(t, p, true).Get(allowed.URL + "/x")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "allowed /x" {
		t.Fatalf("status = %d body = %q, want 200 \"allowed /x\"", res.StatusCode, body)
	}
}

func TestCaptureProxyConnectNeedsTheCredentialsToo(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startAuthProxy(t, allowed)
	target := allowed.hostPort()

	if got := connectStatus(t, p, target, ""); got != http.StatusProxyAuthRequired {
		t.Fatalf("CONNECT without credentials = %d, want 407", got)
	}
	if got := connectStatus(t, p, target, basicProxyAuth(testProxyUser, "wrong")); got != http.StatusProxyAuthRequired {
		t.Fatalf("CONNECT with a wrong password = %d, want 407", got)
	}
	if got := connectStatus(t, p, target, basicProxyAuth(testProxyUser, testProxyPass)); got != http.StatusOK {
		t.Fatalf("CONNECT with credentials = %d, want 200", got)
	}
}

func TestStrictCaptureProxyAlsoTakesCredentials(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p, err := StartStrictCaptureProxy(mustParseURL(t, allowed.URL), CaptureProxyLimits{}, WithProxyCredentials(testProxyUser, testProxyPass))
	if err != nil {
		t.Fatalf("StartStrictCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if got := connectStatus(t, p, allowed.hostPort(), ""); got != http.StatusProxyAuthRequired {
		t.Fatalf("strict CONNECT without credentials = %d, want 407", got)
	}
	if got := connectStatus(t, p, allowed.hostPort(), basicProxyAuth(testProxyUser, testProxyPass)); got != http.StatusOK {
		t.Fatalf("strict CONNECT with credentials = %d, want 200", got)
	}
}

func TestCaptureProxyDoneClosesWhenTheProxyStops(t *testing.T) {
	p := startAuthProxy(t, nil)
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
	p := startAuthProxy(t, allowed, withEgressCheck("n0nce-1234"))
	const checkURL = "http://egress-check.aurago.invalid/n0nce-1234"

	res, err := authProxyClient(t, p, false).Get(checkURL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("check host without credentials = %d, want 407", res.StatusCode)
	}

	res, err = authProxyClient(t, p, true).Get(checkURL)
	if err != nil {
		t.Fatalf("get with credentials: %v", err)
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
	if got := connectStatus(t, p, "egress-check.aurago.invalid:443", basicProxyAuth(testProxyUser, testProxyPass)); got != http.StatusForbidden {
		t.Fatalf("CONNECT to the check host = %d, want 403", got)
	}
	if n := len(p.Dialed()) + len(p.Refused()); n != 0 {
		t.Fatalf("the check host reached the dial path: dialed=%v refused=%v", p.Dialed(), p.Refused())
	}
}

func TestCaptureProxyLogsDialedAndRefusedDestinations(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "")
	allowed, _ := captureProxyTestOrigins(t)
	p := startAuthProxy(t, allowed)
	client := authProxyClient(t, p, true)

	res, err := client.Get(allowed.URL + "/x")
	if err != nil {
		t.Fatalf("allowed get: %v", err)
	}
	res.Body.Close()
	res, err = client.Get("http://10.255.255.1/")
	if err != nil {
		t.Fatalf("private get: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("private destination = %d, want 403", res.StatusCode)
	}
	if d := p.Dialed(); len(d) != 1 || d[0] != allowed.hostPort() {
		t.Fatalf("Dialed() = %v, want [%s]", d, allowed.hostPort())
	}
	if r := p.Refused(); len(r) != 1 || r[0] != "10.255.255.1:80" {
		t.Fatalf("Refused() = %v, want [10.255.255.1:80]", r)
	}
}
```

- [ ] **Step 2: Run them and see them fail for the right reason.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ -run 'CaptureProxyWithCredentials|CaptureProxyConnectNeeds|StrictCaptureProxyAlso|CaptureProxyDone|CaptureProxyAnswersTheCheck|CaptureProxyLogsDialed' -count=1 2>&1 | head -20
```

Expected: a compile failure, `undefined: CaptureProxyOption`, `WithProxyCredentials`, `withEgressCheck`, and `p.Done undefined`, `p.Dialed undefined`.

- [ ] **Step 3: Implement, part 1: imports, constants, struct fields.** In `internal/security/capture_proxy.go`:

Replace
```go
import (
	"context"
	"errors"
```
with
```go
import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
```

Replace
```go
	captureProxyBufferBytes     = 32 << 10
)
```
with
```go
	captureProxyBufferBytes     = 32 << 10

	// captureProxyRealm is the Basic realm of the 407 answer.
	captureProxyRealm = "aurago-egress"
	// captureProxyLogCap bounds each of the dial and refusal logs.
	captureProxyLogCap = 256
	// captureEgressCheckHost never resolves (.invalid). When a nonce is set
	// with withEgressCheck, the proxy answers it itself and never dials it.
	captureEgressCheckHost = "egress-check.aurago.invalid"
)
```

Replace
```go
	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool

	closeOnce sync.Once
}
```
with
```go
	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool

	authWant   string // "Basic <base64 user:password>" that clients must send; "" means no authentication
	checkNonce string // non-empty: answer captureEgressCheckHost with this nonce

	done chan struct{} // closed when the listener has stopped serving, for any reason

	logMu      sync.Mutex
	dialLog    []string // host:port of every destination the proxy really connected to
	refusalLog []string // host:port of every destination the policy refused

	closeOnce sync.Once
}

// CaptureProxyOption tunes a capture proxy at start.
type CaptureProxyOption func(*CaptureProxy)

// WithProxyCredentials makes the proxy answer every request that does not carry
// the matching Basic Proxy-Authorization with 407, plain requests and CONNECT
// alike. The browser receives the credentials over DevTools, never on a command line.
func WithProxyCredentials(user, password string) CaptureProxyOption {
	return func(p *CaptureProxy) {
		p.authWant = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
	}
}

// withEgressCheck makes the proxy answer http://egress-check.aurago.invalid/...
// with 204 and X-AuraGo-Egress: nonce, without resolving or dialling the host.
func withEgressCheck(nonce string) CaptureProxyOption {
	return func(p *CaptureProxy) { p.checkNonce = nonce }
}
```

- [ ] **Step 4: Implement, part 2: the start functions.** Replace the four start functions (from `func StartCaptureProxy` down to the `go func() { _ = p.server.Serve(listener) }()` line) as follows.

Replace
```go
func StartCaptureProxy(allowed *url.URL, limits CaptureProxyLimits) (*CaptureProxy, error) {
	return startCaptureProxy(allowed, limits, captureProxyIdleTimeout)
}
```
with
```go
func StartCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, opts ...CaptureProxyOption) (*CaptureProxy, error) {
	return startCaptureProxy(allowed, limits, captureProxyIdleTimeout, opts...)
}
```

Replace
```go
func StartStrictCaptureProxy(allowed *url.URL, limits CaptureProxyLimits) (*CaptureProxy, error) {
	if allowed == nil {
		return nil, fmt.Errorf("strict capture proxy needs an allowed origin")
	}
	return startCaptureProxyMode(allowed, limits, captureProxyIdleTimeout, true)
}

func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration) (*CaptureProxy, error) {
	return startCaptureProxyMode(allowed, limits, idle, false)
}

func startCaptureProxyMode(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration, strict bool) (*CaptureProxy, error) {
```
with
```go
func StartStrictCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, opts ...CaptureProxyOption) (*CaptureProxy, error) {
	if allowed == nil {
		return nil, fmt.Errorf("strict capture proxy needs an allowed origin")
	}
	return startCaptureProxyMode(allowed, limits, captureProxyIdleTimeout, true, opts...)
}

func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration, opts ...CaptureProxyOption) (*CaptureProxy, error) {
	return startCaptureProxyMode(allowed, limits, idle, false, opts...)
}

func startCaptureProxyMode(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration, strict bool, opts ...CaptureProxyOption) (*CaptureProxy, error) {
```

Replace
```go
		ctx:      ctx,
		cancel:   cancel,
		conns:    make(map[net.Conn]struct{}),
	}
	p.transport = &http.Transport{
```
with
```go
		ctx:      ctx,
		cancel:   cancel,
		conns:    make(map[net.Conn]struct{}),
		done:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.transport = &http.Transport{
```

Replace
```go
	go func() { _ = p.server.Serve(listener) }()
	return p, nil
}

// Addr returns the proxy's listen address, for example 127.0.0.1:53211.
func (p *CaptureProxy) Addr() string { return p.listener.Addr().String() }
```
with
```go
	go func() {
		_ = p.server.Serve(listener)
		close(p.done)
	}()
	return p, nil
}

// Addr returns the proxy's listen address, for example 127.0.0.1:53211.
func (p *CaptureProxy) Addr() string { return p.listener.Addr().String() }

// Done is closed when the proxy's listener has stopped, whether by Close or
// because it failed. A browser behind the proxy must not keep running after that.
func (p *CaptureProxy) Done() <-chan struct{} { return p.done }

// Dialed returns the host:port of every destination the proxy connected to
// (at most captureProxyLogCap entries). Tests use it to prove traffic took this path.
func (p *CaptureProxy) Dialed() []string { return p.logCopy(&p.dialLog) }

// Refused returns the host:port of every destination the policy refused.
func (p *CaptureProxy) Refused() []string { return p.logCopy(&p.refusalLog) }

func (p *CaptureProxy) logCopy(list *[]string) []string {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	return append([]string(nil), *list...)
}

func (p *CaptureProxy) logAppend(list *[]string, entry string) {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	if len(*list) < captureProxyLogCap {
		*list = append(*list, entry)
	}
}
```

- [ ] **Step 5: Implement, part 3: authentication, the check host and the log calls.**

Replace
```go
func (p *CaptureProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
```
with
```go
func (p *CaptureProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(r) {
		p.challenge(w)
		return
	}
	if strings.EqualFold(r.URL.Hostname(), captureEgressCheckHost) {
		p.serveEgressCheck(w, r)
		return
	}
	if r.Method == http.MethodConnect {
```

Add after the `ServeHTTP` function (before `// track registers c`):
```go
// authorized reports whether r carries the credentials the proxy was started
// with; a proxy without credentials accepts everything.
func (p *CaptureProxy) authorized(r *http.Request) bool {
	if p.authWant == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Proxy-Authorization")), []byte(p.authWant)) == 1
}

func (p *CaptureProxy) challenge(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="`+captureProxyRealm+`"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusProxyAuthRequired)
	_, _ = io.WriteString(w, "capture proxy: proxy authentication required\n")
}

// serveEgressCheck answers the path-check host itself. The CORS headers let the
// helper read the nonce with fetch() from a blank page: a 204 navigation does not
// commit, so a navigation could not show the header. CONNECT, or a proxy without a
// nonce, is refused; either way the host is never resolved or dialled.
func (p *CaptureProxy) serveEgressCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || p.checkNonce == "" {
		http.Error(w, "capture proxy: destination is not allowed", http.StatusForbidden)
		return
	}
	h := w.Header()
	h.Set("X-AuraGo-Egress", p.checkNonce)
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Expose-Headers", "X-AuraGo-Egress")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
```

In `dialUpstream`, replace
```go
	target, err := p.destination(dialCtx, hostPort)
	if err != nil {
		return nil, err
	}
	raw, err := p.dialer.DialContext(dialCtx, "tcp", target)
```
with
```go
	target, err := p.destination(dialCtx, hostPort)
	if err != nil {
		if errors.Is(err, errCaptureDestinationBlocked) {
			p.logAppend(&p.refusalLog, hostPort)
		}
		return nil, err
	}
	p.logAppend(&p.dialLog, hostPort)
	raw, err := p.dialer.DialContext(dialCtx, "tcp", target)
```

- [ ] **Step 6: Format, vet, run the new tests and the old proxy tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && gofmt -w internal/security/capture_proxy.go internal/security/capture_proxy_auth_test.go && gofmt -l internal/security/ && go vet ./internal/security/ && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ -run 'CaptureProxy|OriginHostPort' -count=1 -v 2>&1 | grep -E "^(--- |FAIL|ok|PASS)" | sort | uniq -c | sort -rn | head -40
```

Expected: no `gofmt` output, `go vet` silent, every test line `--- PASS`, final `ok  	aurago/internal/security`. The 7 new tests are among them (count the `--- PASS: TestCaptureProxy...` lines for `WithCredentials`, `ConnectNeeds`, `Done`, `AnswersTheCheckHost`, `LogsDialed`, and `TestStrictCaptureProxyAlsoTakesCredentials`). Then check the storymaker callers still build and pass their proxy tests:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && go build ./... && GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render/ -run 'Egress|Proxy' -count=1 2>&1 | tail -3
```

Expected: `go build` silent; `ok` (or `no tests to run` is acceptable if the names differ; then run `go vet ./internal/storymaker/render/`).

- [ ] **Step 7: Commit.** Message file `C:/disposable/easydrag-s0b/a2-commit.txt`:

```
feat(security): capture proxy credentials, Done, check host and dial log

CaptureProxy takes an optional Basic credential (WithProxyCredentials) on both
start functions: a request without it, plain or CONNECT, gets 407 with the
aurago-egress realm and never reaches the destination policy. Done() closes when
the listener stops for any reason. With withEgressCheck the proxy answers
egress-check.aurago.invalid itself (204, X-AuraGo-Egress, CORS-readable) without
resolving or dialling it. Dialed() and Refused() log the destinations of a
capture, capped at 256 entries, so browser tests can prove the path.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git add internal/security/capture_proxy.go internal/security/capture_proxy_auth_test.go && git commit -F C:/disposable/easydrag-s0b/a2-commit.txt && git log -1 --stat --format=%s
```

---

### Task A.3: `LaunchEgressBrowser`

**Files:**
- Create: `internal/security/egress_browser.go`
- Create: `internal/security/egress_browser_test.go`

Facts used (read from the repo): `homepage_annotation_capture.go:124-150` builds the launcher with both WebRTC flags, `Proxy`, and `proxy-bypass-list=<-loopback>`, then `Launch`, `rod.New().Context(ctx).ControlURL(u)`, `Connect`; `scraper.go:130-190` and `web_capture`/`web_performance`/`form_automation` use `launcher.New()` (binary lookup is rod's own: `defaults.Bin`, then `LookPath`, then download). The helper keeps that lookup, and uses `opts.Bin` when set.

Design (all in the code below):
- Unexported function variables `egressStart`, `egressAuth`, `egressProbe` are the seams for the unit tests, so no test needs Chrome.
- `shutdown` is the one teardown: it stops the browser, cancels the context, closes the proxy, removes the profile, releases the slot and closes `Done`, once. Every failure path inside `LaunchEgressBrowser` calls it, so the slot is always released (the lesson of c1d427ffc).
- A watcher ends the browser when the proxy dies (`ErrEgressProxyDied`), the lifetime runs out (`ErrEgressLifetime`), or the caller's context ends.
- Credentials: 16 random bytes hex-encoded for the user, 16 for the password, per call. They reach Chrome only through `Fetch.continueWithAuth`, and only for a challenge with `source=Proxy` and the proxy's address.
- `egressPerTargetAuth` is set from the A.1 verdict.

- [ ] **Step 1: Write the failing tests.** Create `internal/security/egress_browser_test.go`:

```go
package security

import (
	"context"
	"errors"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// egressStub replaces Chrome: egressStart returns an unconnected rod.Browser and
// a no-op stop function, egressAuth records the credentials, egressProbe is the
// path check. No test below needs a real browser.
type egressStub struct {
	launchers []*launcher.Launcher
	users     []string
	passes    []string
}

func stubEgressChrome(t *testing.T, probe func(checkURL string) (string, error)) *egressStub {
	t.Helper()
	s := &egressStub{}
	oldStart, oldAuth, oldProbe := egressStart, egressAuth, egressProbe
	egressStart = func(_ context.Context, l *launcher.Launcher) (*rod.Browser, func(), error) {
		s.launchers = append(s.launchers, l)
		return rod.New(), func() {}, nil
	}
	egressAuth = func(_ *rod.Browser, _, user, pass string) error {
		s.users = append(s.users, user)
		s.passes = append(s.passes, pass)
		return nil
	}
	egressProbe = func(_ context.Context, _ *rod.Browser, checkURL string) (string, error) {
		return probe(checkURL)
	}
	t.Cleanup(func() { egressStart, egressAuth, egressProbe = oldStart, oldAuth, oldProbe })
	return s
}

// echoNonce is a path check that sees the nonce, as a real proxy path would.
func echoNonce(checkURL string) (string, error) { return path.Base(checkURL), nil }

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s did not happen within 3 s", what)
	}
}

func TestLaunchEgressBrowserReleasesTheSlotOnEveryFailure(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"launch fails": func(t *testing.T) {
			stubEgressChrome(t, echoNonce)
			egressStart = func(context.Context, *launcher.Launcher) (*rod.Browser, func(), error) {
				return nil, nil, errors.New("forced launch failure")
			}
		},
		"credentials fail": func(t *testing.T) {
			stubEgressChrome(t, echoNonce)
			egressAuth = func(*rod.Browser, string, string, string) error { return errors.New("forced auth failure") }
		},
		"path check fails": func(t *testing.T) {
			stubEgressChrome(t, func(string) (string, error) { return "", errors.New("forced check failure") })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			setup(t)
			// More attempts than slots: a leaked slot would block or return ErrEgressBusy.
			for i := 0; i < egressMaxBrowsers+2; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				eb, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{})
				cancel()
				if err == nil {
					eb.Close()
					t.Fatalf("attempt %d: want an error", i)
				}
				if errors.Is(err, ErrEgressBusy) {
					t.Fatalf("attempt %d: slot leaked: %v", i, err)
				}
			}
			if n := len(egressSlots); n != 0 {
				t.Fatalf("%d slots still held", n)
			}
		})
	}
}

func TestLaunchEgressBrowserWaitsForASlotAndGivesUpWithItsContext(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	held := make([]*EgressBrowser, 0, egressMaxBrowsers)
	for i := 0; i < egressMaxBrowsers; i++ {
		eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
		if err != nil {
			t.Fatalf("launch %d: %v", i, err)
		}
		held = append(held, eb)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{}); !errors.Is(err, ErrEgressBusy) {
		t.Fatalf("fifth launch = %v, want ErrEgressBusy", err)
	}
	held[0].Close()
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
	if err != nil {
		t.Fatalf("launch after a Close: %v", err)
	}
	held = append(held[1:], eb)
	for _, h := range held {
		h.Close()
	}
	if n := len(egressSlots); n != 0 {
		t.Fatalf("%d slots still held", n)
	}
}

func TestEgressBrowserReportsProxyDeath(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer eb.Close()
	_ = eb.proxy.Close()
	waitClosed(t, eb.Done, "Done after the proxy was closed")
	if err := eb.Err(); !errors.Is(err, ErrEgressProxyDied) {
		t.Fatalf("Err() = %v, want ErrEgressProxyDied", err)
	}
	if n := len(egressSlots); n != 0 {
		t.Fatalf("%d slots still held after the proxy died", n)
	}
	if got := eb.Wrap(errors.New("page failed")); !errors.Is(got, ErrEgressProxyDied) {
		t.Fatalf("Wrap = %v, want it to carry ErrEgressProxyDied", got)
	}
}

func TestLaunchEgressBrowserFailsClosedWhenTheCheckAnswersWithoutTheNonce(t *testing.T) {
	for name, answer := range map[string]string{"empty": "", "wrong": "not-the-nonce"} {
		t.Run(name, func(t *testing.T) {
			stubEgressChrome(t, func(string) (string, error) { return answer, nil })
			eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
			if eb != nil {
				eb.Close()
			}
			if !errors.Is(err, ErrEgressBypassed) {
				t.Fatalf("err = %v, want ErrEgressBypassed", err)
			}
		})
	}
}

func TestEgressBrowserEndsAfterItsLifetime(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{Lifetime: 60 * time.Millisecond})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer eb.Close()
	waitClosed(t, eb.Done, "Done after the lifetime")
	if err := eb.Err(); !errors.Is(err, ErrEgressLifetime) {
		t.Fatalf("Err() = %v, want ErrEgressLifetime", err)
	}
}

func TestEgressBrowserCloseIsCleanAndIdempotent(t *testing.T) {
	stubEgressChrome(t, echoNonce)
	eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	profile := eb.profile
	if _, err := os.Stat(profile); err != nil {
		t.Fatalf("profile directory missing before Close: %v", err)
	}
	if err := eb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_ = eb.Close()
	waitClosed(t, eb.Done, "Done after Close")
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("profile directory not removed: %v", err)
	}
	if err := eb.Err(); err != nil {
		t.Fatalf("Err() after a plain Close = %v, want nil", err)
	}
}

func TestLaunchEgressBrowserSetsTheProxyFlagsAndKeepsCredentialsOffTheCommandLine(t *testing.T) {
	stub := stubEgressChrome(t, echoNonce)
	var addrs []string
	for i := 0; i < 2; i++ {
		eb, err := LaunchEgressBrowser(context.Background(), EgressBrowserOptions{Sandbox: true})
		if err != nil {
			t.Fatalf("launch %d: %v", i, err)
		}
		addrs = append(addrs, eb.proxy.Addr())
		eb.Close()
	}
	l := stub.launchers[0]
	if got, want := l.Get("proxy-server"), "http://"+addrs[0]; got != want {
		t.Fatalf("proxy-server = %q, want %q", got, want)
	}
	if got := l.Get("proxy-bypass-list"); got != "<-loopback>" {
		t.Fatalf("proxy-bypass-list = %q", got)
	}
	for _, flag := range []string{"force-webrtc-ip-handling-policy", "webrtc-ip-handling-policy"} {
		if got := l.Get(launcherFlag(flag)); got != "disable_non_proxied_udp" {
			t.Fatalf("%s = %q", flag, got)
		}
	}
	if l.Has("no-sandbox") {
		t.Fatal("Sandbox: true must remove --no-sandbox")
	}
	cmdline := strings.Join(l.FormatArgs(), " ")
	for i := range stub.users {
		if len(stub.users[i]) != 32 || len(stub.passes[i]) != 32 {
			t.Fatalf("credentials are not 16 hex bytes: %q %q", stub.users[i], stub.passes[i])
		}
		if strings.Contains(cmdline, stub.users[i]) || strings.Contains(cmdline, stub.passes[i]) {
			t.Fatal("credentials appear on the Chrome command line")
		}
	}
	if stub.users[0] == stub.users[1] || stub.passes[0] == stub.passes[1] {
		t.Fatal("two calls got the same credentials")
	}
}
```

The last test calls `launcherFlag`, a thin conversion because `launcher.Launcher.Get` takes a `flags.Flag`. Add it at the end of the same test file, and add `"github.com/go-rod/rod/lib/launcher/flags"` to the file's imports:

```go
func launcherFlag(name string) flags.Flag { return flags.Flag(name) }
```

- [ ] **Step 2: Run them and see them fail for the right reason.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ -run 'EgressBrowser' -count=1 2>&1 | head -12
```

Expected: compile errors, `undefined: egressStart`, `egressAuth`, `egressProbe`, `LaunchEgressBrowser`, `EgressBrowserOptions`, `ErrEgressBusy` and so on.

- [ ] **Step 3: Implement.** Create `internal/security/egress_browser.go`:

```go
package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	egressMaxBrowsers     = 4
	egressDefaultLifetime = 2 * time.Minute
	egressCheckTimeout    = 5 * time.Second
	// egressSlotWaitCap bounds the wait for a free slot when the caller's context
	// has no deadline of its own.
	egressSlotWaitCap = 2 * time.Minute

	// egressPerTargetAuth is the A.1 verdict. False: one browser-wide
	// Fetch.enable answers every proxy challenge. True: every attached target
	// (page, worker, service worker) also gets its own Fetch.enable.
	egressPerTargetAuth = false
)

var (
	// ErrEgressProxyDied is the reason a browser ended when its egress proxy stopped.
	ErrEgressProxyDied = errors.New("egress proxy stopped; the page was cut off")
	// ErrEgressBypassed means the path check did not come back through the proxy.
	ErrEgressBypassed = errors.New("browser traffic does not go through the egress proxy")
	// ErrEgressLifetime is the reason a browser ended when its lifetime ran out.
	ErrEgressLifetime = errors.New("egress browser lifetime ended")
	// ErrEgressBusy means no browser slot became free in time.
	ErrEgressBusy = errors.New("too many egress browsers are running")
)

// egressSlots allows at most egressMaxBrowsers egress browsers at once.
var egressSlots = make(chan struct{}, egressMaxBrowsers)

// The three seams below let unit tests run the helper without Chrome.
var (
	// egressStart launches Chrome and connects to it. stop kills it.
	egressStart = startEgressChrome
	// egressAuth makes the browser answer the proxy's 407 challenge.
	egressAuth = authenticateEgressBrowser
	// egressProbe opens checkURL from a blank page and returns the value of the
	// X-AuraGo-Egress response header.
	egressProbe = probeEgressViaPage
)

// EgressBrowserOptions configures LaunchEgressBrowser.
type EgressBrowserOptions struct {
	// AllowedOrigin is the one origin (a loopback preview) the proxy may dial
	// without the SSRF check. Nil: public addresses only.
	AllowedOrigin *url.URL
	// Lifetime bounds the browser; zero means 2 minutes.
	Lifetime time.Duration
	// Bin is the browser binary; empty uses rod's lookup.
	Bin string
	// Sandbox true removes --no-sandbox even inside a container.
	Sandbox bool
}

// EgressBrowser is a headless Chrome whose whole network traffic goes through
// its own authenticating capture proxy.
type EgressBrowser struct {
	Browser *rod.Browser
	// Done is closed when the browser, the proxy or the lifetime ends.
	Done <-chan struct{}

	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	proxy     *CaptureProxy
	profile   string
	stop      func()
	closeOnce sync.Once

	mu      sync.Mutex
	err     error
	closing bool
}

// LaunchEgressBrowser starts a capture proxy with fresh credentials and a fresh
// Chrome with a fresh profile behind it, hands Chrome the credentials over
// DevTools and proves with a path check that its traffic really uses the proxy.
// The caller must Close the result.
func LaunchEgressBrowser(ctx context.Context, opts EgressBrowserOptions) (*EgressBrowser, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	lifetime := opts.Lifetime
	if lifetime <= 0 {
		lifetime = egressDefaultLifetime
	}
	if err := acquireEgressSlot(ctx); err != nil {
		return nil, err
	}

	bctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	eb := &EgressBrowser{Done: done, done: done, ctx: bctx, cancel: cancel}
	// From here on every failure goes through shutdown, which releases the slot.
	fail := func(err error) (*EgressBrowser, error) {
		eb.shutdown()
		return nil, err
	}

	user, err1 := randomHex(16)
	pass, err2 := randomHex(16)
	nonce, err3 := randomHex(16)
	if err := errors.Join(err1, err2, err3); err != nil {
		return fail(fmt.Errorf("egress credentials: %w", err))
	}
	proxy, err := StartCaptureProxy(opts.AllowedOrigin, CaptureProxyLimits{
		MaxConcurrent: 16, MaxConnections: 600, MaxBytes: 64 << 20,
	}, WithProxyCredentials(user, pass), withEgressCheck(nonce))
	if err != nil {
		return fail(fmt.Errorf("egress proxy: %w", err))
	}
	eb.proxy = proxy

	profile, err := os.MkdirTemp("", "aurago-egress-*")
	if err != nil {
		return fail(fmt.Errorf("egress profile directory: %w", err))
	}
	eb.profile = profile

	browser, stop, err := egressStart(bctx, newEgressLauncher(bctx, proxy.Addr(), profile, opts))
	if err != nil {
		return fail(err)
	}
	eb.Browser, eb.stop = browser, stop

	if err := egressAuth(browser, proxy.Addr(), user, pass); err != nil {
		return fail(fmt.Errorf("egress proxy credentials: %w", err))
	}
	if err := verifyEgressPath(bctx, browser, nonce); err != nil {
		select {
		case <-proxy.Done():
			err = ErrEgressProxyDied
		default:
		}
		return fail(err)
	}

	go eb.watch(lifetime)
	if observe := egressObserver(); observe != nil {
		observe(eb)
	}
	return eb, nil
}

// Close stops the browser and the proxy and removes the profile. It is safe to
// call more than once.
func (b *EgressBrowser) Close() error {
	b.shutdown()
	return nil
}

// Err says why the browser ended: nil after a plain Close, ErrEgressProxyDied or
// ErrEgressLifetime otherwise.
func (b *EgressBrowser) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// Wrap adds the reason the browser ended, if there is one, to a failure the
// caller saw. A caller that gets a non-nil cause must return it and never
// partial content.
func (b *EgressBrowser) Wrap(err error) error {
	if err == nil {
		return nil
	}
	if cause := b.Err(); cause != nil && !errors.Is(err, cause) {
		return fmt.Errorf("%w: %v", cause, err)
	}
	return err
}

// ProxyDialed lists the destinations (host:port) the proxy connected to.
func (b *EgressBrowser) ProxyDialed() []string { return b.proxy.Dialed() }

// ProxyRefused lists the destinations (host:port) the proxy refused.
func (b *EgressBrowser) ProxyRefused() []string { return b.proxy.Refused() }

func (b *EgressBrowser) end(err error) {
	b.mu.Lock()
	if !b.closing && b.err == nil {
		b.err = err
	}
	b.mu.Unlock()
	b.shutdown()
}

func (b *EgressBrowser) watch(lifetime time.Duration) {
	timer := time.NewTimer(lifetime)
	defer timer.Stop()
	select {
	case <-b.proxy.Done():
		b.end(ErrEgressProxyDied)
	case <-timer.C:
		b.end(ErrEgressLifetime)
	case <-b.ctx.Done():
		b.shutdown() // the caller's context ended
	case <-b.done:
	}
}

// shutdown is the one teardown. It runs once, whatever started it.
func (b *EgressBrowser) shutdown() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closing = true
		b.mu.Unlock()
		if b.stop != nil {
			b.stop()
		}
		b.cancel()
		if b.proxy != nil {
			_ = b.proxy.Close()
		}
		if b.profile != "" {
			_ = os.RemoveAll(b.profile)
		}
		<-egressSlots
		close(b.done)
	})
}

func acquireEgressSlot(ctx context.Context) error {
	wait := time.NewTimer(egressSlotWaitCap)
	defer wait.Stop()
	select {
	case egressSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrEgressBusy, ctx.Err())
	case <-wait.C:
		return ErrEgressBusy
	}
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newEgressLauncher(ctx context.Context, proxyAddr, profile string, opts EgressBrowserOptions) *launcher.Launcher {
	l := launcher.New().Context(ctx).Headless(true).UserDataDir(profile).
		// Chrome 156 ignores the force- spelling for STUN, so set both.
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Set("webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Proxy("http://" + proxyAddr).
		// Chrome bypasses a proxy for loopback unless told otherwise.
		Set("proxy-bypass-list", "<-loopback>")
	if opts.Bin != "" {
		l = l.Bin(opts.Bin)
	}
	if opts.Sandbox {
		l = l.NoSandbox(false)
	}
	return l
}

func startEgressChrome(ctx context.Context, l *launcher.Launcher) (*rod.Browser, func(), error) {
	controlURL, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}
	kill := func() { l.Kill(); l.Cleanup() }
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		kill()
		return nil, nil, fmt.Errorf("connect browser: %w", err)
	}
	stop := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = browser.Context(closeCtx).Close()
		kill()
	}
	return browser, stop, nil
}

// egressSession sends a DevTools command on one session of the browser.
type egressSession struct {
	b   *rod.Browser
	sid proto.TargetSessionID
}

func (c egressSession) Call(ctx context.Context, _, method string, params interface{}) ([]byte, error) {
	return c.b.Call(ctx, string(c.sid), method, params)
}

// egressAuthResponse provides the credentials only to a challenge from this
// proxy; a site that asks for a login (a 401) or any other proxy gets nothing.
func egressAuthResponse(c *proto.FetchAuthChallenge, proxyAddr, user, pass string) *proto.FetchAuthChallengeResponse {
	if c != nil && c.Source == proto.FetchAuthChallengeSourceProxy && strings.Contains(c.Origin, proxyAddr) {
		return &proto.FetchAuthChallengeResponse{
			Response: proto.FetchAuthChallengeResponseResponseProvideCredentials,
			Username: user,
			Password: pass,
		}
	}
	return &proto.FetchAuthChallengeResponse{Response: proto.FetchAuthChallengeResponseResponseCancelAuth}
}

// authenticateEgressBrowser answers every proxy challenge over DevTools. rod's
// HandleAuth answers only one challenge per call, so this runs an event loop:
// Fetch.enable with handleAuthRequests pauses every request, so each paused
// request is continued and each Fetch.authRequired is answered.
func authenticateEgressBrowser(b *rod.Browser, proxyAddr, user, pass string) error {
	handlers := []interface{}{
		func(e *proto.FetchRequestPaused, sid proto.TargetSessionID) {
			go func() { _ = proto.FetchContinueRequest{RequestID: e.RequestID}.Call(egressSession{b, sid}) }()
		},
		func(e *proto.FetchAuthRequired, sid proto.TargetSessionID) {
			go func() {
				_ = proto.FetchContinueWithAuth{
					RequestID:             e.RequestID,
					AuthChallengeResponse: egressAuthResponse(e.AuthChallenge, proxyAddr, user, pass),
				}.Call(egressSession{b, sid})
			}()
		},
	}
	if egressPerTargetAuth {
		handlers = append(handlers, func(e *proto.TargetAttachedToTarget, _ proto.TargetSessionID) {
			go func() {
				s := egressSession{b, e.SessionID}
				_ = proto.FetchEnable{HandleAuthRequests: true}.Call(s)
				_ = proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}.Call(s)
				if e.WaitingForDebugger {
					_ = proto.RuntimeRunIfWaitingForDebugger{}.Call(s)
				}
			}()
		})
	}
	// Fetch.enable first, so rod sees the domain as enabled and EachEvent does
	// not re-enable it without handleAuthRequests. No page exists yet, so no
	// event is missed between the two calls.
	if err := (proto.FetchEnable{HandleAuthRequests: true}).Call(b); err != nil {
		return err
	}
	go b.EachEvent(handlers...)()
	if egressPerTargetAuth {
		return proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}.Call(b)
	}
	return nil
}

// verifyEgressPath loads the proxy's own check host from a blank page. The
// proxy answers it without dialling; a browser that reaches anything else, or
// nothing, did not go through the proxy.
func verifyEgressPath(ctx context.Context, b *rod.Browser, nonce string) error {
	checkCtx, cancel := context.WithTimeout(ctx, egressCheckTimeout)
	defer cancel()
	got, err := egressProbe(checkCtx, b, "http://"+captureEgressCheckHost+"/"+nonce)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrEgressBypassed, err)
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(nonce)) != 1 {
		return fmt.Errorf("%w: the check answered without the nonce", ErrEgressBypassed)
	}
	return nil
}

func probeEgressViaPage(ctx context.Context, b *rod.Browser, checkURL string) (string, error) {
	page, err := b.Context(ctx).Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return "", err
	}
	defer func() { _ = page.Close() }()
	res, err := page.Eval(`(u) => fetch(u, {cache: 'no-store'}).then((r) => r.headers.get('X-AuraGo-Egress') || '')`, checkURL)
	if err != nil {
		return "", err
	}
	return res.Value.Str(), nil
}

var (
	egressObserverMu sync.Mutex
	egressObserverFn func(*EgressBrowser)
)

func egressObserver() func(*EgressBrowser) {
	egressObserverMu.Lock()
	defer egressObserverMu.Unlock()
	return egressObserverFn
}

// SetEgressLaunchObserver makes fn see every EgressBrowser that
// LaunchEgressBrowser returns, so a test can read the proxy log of a browser
// that a tool launched internally. Tests only. The returned function restores
// the previous observer.
func SetEgressLaunchObserver(fn func(*EgressBrowser)) (restore func()) {
	egressObserverMu.Lock()
	previous := egressObserverFn
	egressObserverFn = fn
	egressObserverMu.Unlock()
	return func() {
		egressObserverMu.Lock()
		egressObserverFn = previous
		egressObserverMu.Unlock()
	}
}
```

Set `egressPerTargetAuth` now from the A.1 verdict: leave `false` when the verdict said `browser-wide`; change it to `true` when it said `browser-wide+attach`.

- [ ] **Step 4: Format, vet and run the tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && gofmt -w internal/security/egress_browser.go internal/security/egress_browser_test.go && gofmt -l internal/security/ && go vet ./internal/security/ && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ -run 'EgressBrowser' -count=1 -v 2>&1 | grep -E "^(=== RUN|--- |FAIL|ok|PASS)" | grep -v "=== RUN"
```

Expected: no `gofmt` output; `go vet` silent; `--- PASS` for `TestLaunchEgressBrowserReleasesTheSlotOnEveryFailure` (three subtests), `...WaitsForASlotAndGivesUpWithItsContext`, `TestEgressBrowserReportsProxyDeath`, `...FailsClosedWhenTheCheckAnswersWithoutTheNonce` (two subtests), `TestEgressBrowserEndsAfterItsLifetime`, `TestEgressBrowserCloseIsCleanAndIdempotent`, `...SetsTheProxyFlagsAndKeepsCredentialsOffTheCommandLine`, then `ok`. If a `-race` build is available, also run once with `-race` (the shutdown and watcher paths are concurrent): `go test -race ./internal/security/ -run EgressBrowser -count=1`; skip it if cgo is unavailable.

If `l.Get("proxy-server")` is empty in the flags test, the real flag name differs: print `l.FormatArgs()` once and use the printed name (rod's `Proxy` sets the `flags.ProxyServer` constant).

- [ ] **Step 5: A real-Chrome check of the helper, run once and not committed.** This proves that the default seams work with Chrome. Create `internal/security/egress_real_chrome_check_test.go` (a temporary file, deleted in Step 6):

```go
package security

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestTempEgressRealChrome(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("gated")
	}
	bin, err := exec.LookPath(`C:\Program Files\Google\Chrome\Application\chrome.exe`)
	if err != nil {
		t.Skip("no Chrome")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	eb, err := LaunchEgressBrowser(ctx, EgressBrowserOptions{Bin: bin})
	if err != nil {
		t.Fatalf("LaunchEgressBrowser: %v", err)
	}
	defer eb.Close()
	if eb.Err() != nil {
		t.Fatalf("Err = %v", eb.Err())
	}
}
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && AURAGO_RUN_BROWSER_SMOKE=1 GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ -run TestTempEgressRealChrome -count=1 -v 2>&1 | tail -8
```

Expected: `--- PASS: TestTempEgressRealChrome`. A `FAIL` with `ErrEgressBypassed` and a note about the check means the path check or the credential answer does not work with this Chrome: print the error text and compare with the A.1 result (a challenge with another `source`/`origin` spelling is the likely cause; `egressAuthResponse` is the only place that depends on it). Fix the code, not the test.

- [ ] **Step 6: Remove the temporary file and commit.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && rm internal/security/egress_real_chrome_check_test.go && git status --short
```

Expected: only `egress_browser.go` and `egress_browser_test.go` untracked. Message file `C:/disposable/easydrag-s0b/a3-commit.txt`:

```
feat(security): LaunchEgressBrowser, one browser per call behind its own proxy

A headless Chrome with a fresh profile behind a fresh authenticating capture
proxy. The proxy credentials (16 random bytes each) reach Chrome only over
DevTools, and only for a challenge from this proxy. A path check against the
proxy's own check host fails the launch with ErrEgressBypassed when traffic does
not go through the proxy; a stopped proxy ends the browser with
ErrEgressProxyDied, and the 2 minute lifetime with ErrEgressLifetime. At most 4
browsers run at once and the slot is released on every failure path.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git add internal/security/egress_browser.go internal/security/egress_browser_test.go && git commit -F C:/disposable/easydrag-s0b/a3-commit.txt && git log -1 --stat --format=%s
```

---

### Task A.4: Move the five call sites, delete the shared browser and the hook

**Files:**
- Modify: `internal/scraper/scraper.go:17-21,127-212` and `internal/scraper/scraper_test.go:126-135`
- Modify: `internal/tools/web_capture.go:4-17,73-92,105-139`
- Modify: `internal/tools/web_performance.go:3-14,76-103,170-173,250-256`
- Modify: `internal/tools/form_automation.go:14-19,96-131`
- Modify: `internal/tools/homepage_annotation_capture.go:3-19,109-150,197-203`
- Modify: `internal/tools/utils.go` (delete `getSharedBrowser` and its vars, fix the header comment and imports)
- Delete: `internal/security/rod_ssrf.go`
- Modify: `internal/security/AGENTS.md:64`, `internal/tools/AGENTS.md:67`, `ui/js/desktop/apps/AGENTS.md:5405`

**IMPACT first** (main checkout path). Each symbol has a small caller set:

```bash
for s in getSharedBrowser OpenRodPageWithSSRF GuardRodPageSSRF FetchDynamic CaptureHomepageAnnotationSnapshot WebCaptureWithWorkspace WebPerformanceAudit ExecuteFormAutomationWithWorkspace; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -v '^{"level"' | head -12; done
```

Report HIGH or CRITICAL. The signatures of the five tool entry points do not change, so their callers (flows `tool.*` nodes, agent dispatch, the Homepage Studio endpoint) are unaffected; what changes is the browser they use.

- [ ] **Step 1: Prove no other caller remains for the symbols to delete.** Run this before and after the edits.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && grep -rn "getSharedBrowser\|sharedBrowser\|OpenRodPageWithSSRF\|GuardRodPageSSRF\|newScraperLauncher" --include=*.go --include=*.md --include=*.js . | grep -v "^./plans/\|^./graft/"
```

Expected before the edits (callers): `scraper.go` (`newScraperLauncher`, `OpenRodPageWithSSRF`), `scraper_test.go` (`newScraperLauncher`), `web_capture.go`, `web_performance.go`, `form_automation.go`, `utils.go`, `rod_ssrf.go`, and the three AGENTS.md lines. Expected after Step 9: no output at all. (`graft/` is a git-ignored generated index; `plans/` holds history and stays as written.)

- [ ] **Step 2: Baseline of the existing tests of these sites, before any edit.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && GOTMPDIR='D:\tmp\go-build' go test ./internal/scraper/ ./internal/tools/ -run 'Scrap|DynamicScrape|WebPerformance|HomepageAnnotation|HomepageScreenshot|WebCapture|FormAutomation' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL)" | head -60
```

Expected: only `PASS` and `SKIP` (the gated Chrome tests skip). Save this list: it is the set that must still pass. It includes `TestScraperLauncherKeepsRodSandboxDefault`, which Step 3 deletes together with the function it tests.

- [ ] **Step 3: Scraper.** In `internal/scraper/scraper.go`:

Imports: replace
```go
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)
```
with
```go
	"github.com/go-rod/rod/lib/proto"
)
```

Delete `newScraperLauncher` and its comment (lines 130-135):
```go
// newScraperLauncher builds the headless Rod launcher without forcing --no-sandbox.
// Rod's New already sets that flag when running in a container.
func newScraperLauncher(ctx context.Context) *launcher.Launcher {
	return launcher.New().Context(ctx).Headless(true).
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp")
}
```

In `FetchDynamic`, replace (the part from the launcher to the `OpenRodPageWithSSRF` call)
```go
	l := newScraperLauncher(ctx)
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("rod launcher: %w", err)
	}
	defer func() { l.Kill(); l.Cleanup() }()

	browser := rod.New().Context(ctx).ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("rod connect: %w", err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = browser.Context(closeCtx).Close()
	}()

	page, err := security.OpenRodPageWithSSRF(browser, rawURL)
	if err != nil {
		return nil, fmt.Errorf("rod open page %s: %w", rawURL, err)
	}

	// Wait for the page to reach a network-idle state.
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("rod wait load: %w", err)
	}

	if waitForSelector != "" {
		if err := page.WaitElementsMoreThan(waitForSelector, 0); err != nil {
			return nil, fmt.Errorf("rod wait selector %q: %w", waitForSelector, err)
		}
	}

	// Capture the fully-rendered outer HTML.
	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("rod get HTML: %w", err)
	}
	return a.ScrapeHTML(rawURL, []byte(html))
```
with
```go
	// The browser and its proxy are made for this call; every byte Chrome sends,
	// service workers and prefetch included, goes through the egress proxy.
	eb, err := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{})
	if err != nil {
		return nil, fmt.Errorf("rod launcher: %w", err)
	}
	defer func() { _ = eb.Close() }()

	page, err := eb.Browser.Page(proto.TargetCreateTarget{})
	if err == nil {
		err = page.Navigate(rawURL)
	}
	if err != nil {
		return nil, fmt.Errorf("rod open page %s: %w", rawURL, eb.Wrap(err))
	}

	// Wait for the page to reach a network-idle state.
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("rod wait load: %w", eb.Wrap(err))
	}

	if waitForSelector != "" {
		if err := page.WaitElementsMoreThan(waitForSelector, 0); err != nil {
			return nil, fmt.Errorf("rod wait selector %q: %w", waitForSelector, eb.Wrap(err))
		}
	}

	// Capture the fully-rendered outer HTML.
	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("rod get HTML: %w", eb.Wrap(err))
	}
	if cause := eb.Err(); cause != nil {
		return nil, fmt.Errorf("rod fetch %s: %w", rawURL, cause)
	}
	return a.ScrapeHTML(rawURL, []byte(html))
```
Kept as they were: `ValidateSSRF` before the slot, the one-at-a-time `dynamicScrapeSlot`, the 30 s `context.WithTimeout`, and the error texts `rod launcher:`, `rod open page`, `rod wait load`, `rod wait selector`, `rod get HTML`. Check that `time` is still used in the file (yes: `30*time.Second`).

In `internal/scraper/scraper_test.go`, delete `TestScraperLauncherKeepsRodSandboxDefault` (lines 126-135) and the `launcher` import (`"github.com/go-rod/rod/lib/launcher"`, line 12) if nothing else in the file uses it (`grep -n launcher internal/scraper/scraper_test.go` after the edit must print nothing). The WebRTC flag assertion it carried now lives in `TestLaunchEgressBrowserSetsTheProxyFlagsAndKeepsCredentialsOffTheCommandLine` (A.3).

- [ ] **Step 4: Web capture.** In `internal/tools/web_capture.go`, replace
```go
	browser, err := getSharedBrowser()
	if err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", err)})
	}

	page, err := security.OpenRodPageWithSSRF(browser, rawURL)
	if err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("open page failed: %v", err)})
	}
	page = page.Context(ctx)
	defer page.Close()

	if err := page.WaitLoad(); err != nil {
		// non-fatal; continue with possibly partial page
		_ = err
	}
```
with
```go
	// A browser and a proxy per call: no shared browser, no cached launch failure.
	eb, err := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{Sandbox: true})
	if err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", err)})
	}
	defer func() { _ = eb.Close() }()

	page, err := eb.Browser.Page(proto.TargetCreateTarget{})
	if err == nil {
		page = page.Context(ctx)
		err = page.Navigate(rawURL)
	}
	if err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("open page failed: %v", eb.Wrap(err))})
	}

	if err := page.WaitLoad(); err != nil {
		// non-fatal while the proxy is alive; with it gone the page is cut off,
		// and the caller never gets partial content.
		if cause := eb.Err(); cause != nil {
			return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("page load failed: %v", cause)})
		}
	}
```
`proto` is already imported in this file. Then wrap the three later errors with `eb.Wrap`: `fmt.Sprintf("screenshot failed: %v", err)` becomes `fmt.Sprintf("screenshot failed: %v", eb.Wrap(err))`, `fmt.Sprintf("pdf render failed: %v", err)` becomes `...eb.Wrap(err)`, and `fmt.Sprintf("read pdf stream failed: %v", err)` becomes `...eb.Wrap(err)`. Immediately before `filename = filepath.Join(root.Name(), filename)` add:
```go
	if cause := eb.Err(); cause != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("capture cut off: %v", cause)})
	}
```
Kept: `ValidateSSRF` at `:62`, every message, the output root and the atomic write.

- [ ] **Step 5: Web performance.** In `internal/tools/web_performance.go`, replace
```go
	// Get shared headless browser
	browser, err := getSharedBrowser()
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", err)})
	}

	// Set viewport
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("create page failed: %v", err)})
	}
	defer page.Close()

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  vpWidth,
		Height: vpHeight,
	})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("set viewport failed: %v", err)})
	}
	if err := security.GuardRodPageSSRF(page); err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("SSRF guard failed: %v", err)})
	}

	// Navigate and wait
	if err := page.Navigate(rawURL); err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("navigation failed: %v", err)})
	}
```
with
```go
	// A headless browser and its egress proxy for this call only.
	eb, err := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{Sandbox: true})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", err)})
	}
	defer func() { _ = eb.Close() }()

	// Set viewport
	page, err := eb.Browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("create page failed: %v", eb.Wrap(err))})
	}

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  vpWidth,
		Height: vpHeight,
	})
	if err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("set viewport failed: %v", eb.Wrap(err))})
	}

	// Navigate and wait
	if err := page.Navigate(rawURL); err != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("navigation failed: %v", eb.Wrap(err))})
	}
```
Then `fmt.Sprintf("failed to collect metrics: %v", err)` becomes `...eb.Wrap(err)`, and before the final `return webPerfJSON(webPerfResult{Status: "success", ...` add
```go
	if cause := eb.Err(); cause != nil {
		return webPerfJSON(webPerfResult{Status: "error", Message: fmt.Sprintf("audit cut off: %v", cause)})
	}
```
The context `ctx` of `WebPerformanceAudit` now bounds the browser (the shared browser ignored it); `ctx == nil` is handled by the helper. Kept: `ValidateSSRF` at `:72`, the 30 s load wait, the 500 ms settle, the metrics script.

- [ ] **Step 6: Form automation.** In `internal/tools/form_automation.go`, delete the import `"github.com/go-rod/rod/lib/launcher"` (`rod` stays: the helpers take `*rod.Page`), then replace
```go
	// Launch headless browser
	u, launchErr := launcher.New().
		Headless(true).
		NoSandbox(false).
		Launch()
	if launchErr != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", launchErr)})
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("browser connect failed: %v", err)})
	}
	defer func() {
		_ = browser.Close()
	}()

	page, err := security.OpenRodPageWithSSRF(browser, rawURL)
	if err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("open page failed: %v", err)})
	}
	page = page.Context(ctx)
	if err := page.WaitLoad(); err != nil {
		_ = err // non-fatal
	}

	switch operation {
	case "get_fields":
		return formGetFields(page, rawURL)
	case "click":
		return formClick(page, rawURL, selector, screenshotDir)
	case "fill_submit":
		return formFillSubmit(page, rawURL, fieldsJSON, selector, screenshotDir)
	}
	return formJSON(formResult{Status: "error", Message: "unhandled operation"})
```
with
```go
	// Launch a headless browser behind its own egress proxy (this site had no
	// proxy and no WebRTC flags before).
	eb, launchErr := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{Sandbox: true})
	if launchErr != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", launchErr)})
	}
	defer func() { _ = eb.Close() }()

	page, err := eb.Browser.Page(proto.TargetCreateTarget{})
	if err == nil {
		page = page.Context(ctx)
		err = page.Navigate(rawURL)
	}
	if err != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("open page failed: %v", eb.Wrap(err))})
	}
	if err := page.WaitLoad(); err != nil {
		if cause := eb.Err(); cause != nil {
			return formJSON(formResult{Status: "error", Message: fmt.Sprintf("page load failed: %v", cause)})
		}
		_ = err // non-fatal while the proxy is alive
	}

	var out string
	switch operation {
	case "get_fields":
		out = formGetFields(page, rawURL)
	case "click":
		out = formClick(page, rawURL, selector, screenshotDir)
	case "fill_submit":
		out = formFillSubmit(page, rawURL, fieldsJSON, selector, screenshotDir)
	default:
		return formJSON(formResult{Status: "error", Message: "unhandled operation"})
	}
	if cause := eb.Err(); cause != nil {
		return formJSON(formResult{Status: "error", Message: fmt.Sprintf("form run cut off: %v", cause)})
	}
	return out
```
Kept: `ValidateSSRF` at `:85`, the operation check, the screenshot root and its context key, `NoSandbox(false)` as `Sandbox: true`. `proto` is already imported.

- [ ] **Step 7: Homepage annotation capture.** In `internal/tools/homepage_annotation_capture.go`, delete the import `"github.com/go-rod/rod/lib/launcher"` (`rod` stays for `evaluateHomepageAnnotationMap`), then replace the block from the proxy to the browser connect:
```go
	// Every byte the browser sends goes through this proxy, which refuses any
	// destination but the allowed origin and public addresses. A CDP request
	// hook sees only the page target; service workers, WebSockets and
	// prerenders would slip past it.
	proxy, err := security.StartCaptureProxy(opts.AllowedOrigin, security.CaptureProxyLimits{
		MaxConcurrent: 16, MaxConnections: 600, MaxBytes: 64 << 20,
	})
	if err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	defer func() { _ = proxy.Close() }()

	l := launcher.New().Context(ctx).Headless(true).
		// Chrome 156 ignores the force- spelling for STUN (verified: a page
		// still sent UDP binding requests to loopback), so set both.
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Set("webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Proxy(proxy.Addr()).
		// Chrome bypasses a proxy for loopback unless told otherwise; the
		// preview itself lives on loopback.
		Set("proxy-bypass-list", "<-loopback>")
	if opts.BrowserBin != "" {
		l = l.Bin(opts.BrowserBin)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, homepageAnnotationLaunchError(ctx, err)
	}
	defer func() { l.Kill(); l.Cleanup() }()

	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = browser.Context(closeCtx).Close()
	}()

	page, err := browser.Page(proto.TargetCreateTarget{})
```
with
```go
	// Every byte the browser sends goes through the helper's proxy, which refuses
	// any destination but the allowed origin and public addresses (loopback is
	// reached through the proxy too). The loopback preview is the allowed origin.
	eb, err := security.LaunchEgressBrowser(ctx, security.EgressBrowserOptions{
		AllowedOrigin: opts.AllowedOrigin,
		Bin:           opts.BrowserBin,
	})
	if err != nil {
		return nil, homepageAnnotationLaunchError(ctx, err)
	}
	defer func() { _ = eb.Close() }()

	page, err := eb.Browser.Page(proto.TargetCreateTarget{})
```
Immediately before `return &HomepageAnnotationSnapshot{` add
```go
	if cause := eb.Err(); cause != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, cause)
	}
```
Kept: the `ValidateSSRF` call when `AllowedOrigin == nil` (`:97-101`), the one-at-a-time `homepageAnnotationSlot`, the 30 s timeout, the viewport, `Mobile: false`, the error mapping. `homepageAnnotationLaunchError` still names a blocked sandbox because the helper wraps the launcher error with `%w` and its text (`launch browser: ... No usable sandbox ...`) is unchanged. The `time` import stays (`homepageAnnotationTimeout`).

- [ ] **Step 8: Delete the shared browser and the hook.** In `internal/tools/utils.go` delete the `var (sharedBrowser ...)` block and `getSharedBrowser`, drop the imports `"sync"`, `"github.com/go-rod/rod"` and `".../launcher"` (keep `"unicode/utf8"`), and rewrite the header comment so it no longer names the function:
```go
// Package tools provides utility functions for the AuraGo agent tool system.
//
// This file (utils.go) serves as a collection point for helper functions that are used
// across multiple tool implementations (truncateUTF8Safe: used by ansible.go and email.go
// for safe UTF-8 string truncation without breaking multi-byte characters).
//
// Headless browsers that open untrusted URLs are never built here or with
// launcher.New(): use security.LaunchEgressBrowser (see AGENTS.md).
package tools
```
Then:
```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git rm -q internal/security/rod_ssrf.go && gofmt -w internal/tools/utils.go internal/tools/web_capture.go internal/tools/web_performance.go internal/tools/form_automation.go internal/tools/homepage_annotation_capture.go internal/scraper/scraper.go internal/scraper/scraper_test.go && go build ./... 2>&1 | head -20
```
Expected: no output. A leftover unused import is reported here; remove it. A `go vet` complaint about an unused variable `browser` means a line from Step 4-7 was missed.

- [ ] **Step 9: AGENTS.md.** Three edits.

`internal/security/AGENTS.md`: replace the line
```
- `GuardRodPageSSRF` fulfills through the SSRF client and fails a body larger than `MaxSSRFResponseBytes` instead of calling `LoadResponse` with an unlimited read.
```
with
```
- A headless browser that opens an untrusted URL comes from `LaunchEgressBrowser` (`egress_browser.go`) and nothing else: one Chrome with a fresh profile and one authenticating `CaptureProxy` per call, at most 4 at once, a 2 minute lifetime. The proxy credentials (16 random bytes each, per call) reach Chrome only over DevTools (`Fetch.authRequired`, answered only for a challenge from that proxy) and never on a command line, in the environment or in a file. A path check against `egress-check.aurago.invalid` must come back through the proxy or the launch fails with `ErrEgressBypassed`; a stopped proxy ends the browser with `ErrEgressProxyDied` and the caller returns that error, never partial content. Never use `launcher.New()` with a page-level request hook for untrusted URLs: a hook covers one page target and misses service workers, prefetch, prerender and WebRTC. The old `OpenRodPageWithSSRF` and `GuardRodPageSSRF` are gone. Verify `TestEgressBrowser*`, `TestLaunchEgressBrowser*`, `TestCaptureProxy*` and the gated `TestEgressBrowserRefusesTheS8Fixture`.
```
`internal/tools/AGENTS.md`: replace
```
Dynamic mode uses `newScraperLauncher` and one process-wide slot.
```
with
```
Dynamic mode uses `security.LaunchEgressBrowser` and one process-wide slot.
```
and, directly before the line `## Desktop invocation ownership`, add
```
- Web capture, web performance, form automation, the dynamic scraper and the Homepage annotation capture each launch their browser with `security.LaunchEgressBrowser` (a browser and an authenticating proxy per call, no shared browser). New headless-browser code that opens untrusted URLs does the same, never `launcher.New()` with a request hook. Keep `security.ValidateSSRF` in front of each launch for the clear error, and on a failure call `eb.Wrap(err)` and return `eb.Err()` when it is non-nil, never partial content. The Homepage capture passes its loopback preview as `AllowedOrigin`. Verify `TestCaptureHomepageAnnotationSnapshot*`, `TestWebPerformanceAudit_*`, `TestDynamicScrapeSlotRejectsASecondHolder` and the gated `TestScraperDynamicGoesThroughTheEgressProxy` and `TestWebCaptureGoesThroughTheEgressProxy`.
```
Use the Edit tool with the whole old line as `old_string` for the first two replacements and `## Desktop invocation ownership` as the anchor for the insert.

`ui/js/desktop/apps/AGENTS.md`: replace
```
64 MiB per capture. `GuardRodPageSSRF` is unchanged. Captures use a
```
with
```
64 MiB per capture. The browser and the proxy come from `security.LaunchEgressBrowser`, which adds per-call proxy credentials over DevTools and a path check. Captures use a
```

- [ ] **Step 10: Prove no caller remains, vet, and run the existing tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && grep -rn "getSharedBrowser\|sharedBrowser\|OpenRodPageWithSSRF\|GuardRodPageSSRF\|newScraperLauncher" --include=*.go --include=*.md --include=*.js . | grep -v "^./plans/\|^./graft/"; echo "grep-exit-done"; gofmt -l internal/security internal/scraper internal/tools; go vet ./internal/security/ ./internal/scraper/ ./internal/tools/ && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ ./internal/scraper/ -count=1 2>&1 | tail -5 && GOTMPDIR='D:\tmp\go-build' go test ./internal/tools/ -run 'DynamicScrape|WebPerformance|HomepageAnnotation|HomepageScreenshot|Scraper' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL)" | head -40
```

Expected: the grep prints nothing before `grep-exit-done`; no `gofmt` output; `go vet` silent; `ok` for `internal/security` and `internal/scraper`; in tools the same PASS/SKIP list as Step 2, minus `TestScraperLauncherKeepsRodSandboxDefault`.

The existing tests that must keep passing, and what each pins:
- `internal/scraper`: `TestDynamicScrapeSlotRejectsASecondHolder` (the one-at-a-time slot stays) and the whole package.
- `internal/tools`: `TestCaptureHomepageAnnotationSnapshotRejectsConcurrentCapture`, `...BlocksPrivateDeployTargets` (the `ValidateSSRF` call before launch), `TestClipHomepageAnnotationElementsDropsAndReindexes`, `TestHomepageAnnotationNavigateErrorMapping`, `TestWebPerformanceAudit_*` (input validation order), `TestHomepageScreenshotFallsBackToWebCaptureWhenPlaywrightIsMissing`.
- Gated, run with Chrome: `TestCaptureHomepageAnnotationSnapshotBrowser`, `...ReachesNothingBeyondThePreviewBrowser`, `...NarrowViewportBrowser`. These prove the homepage capture still keeps its `AllowedOrigin` and reaches nothing else through the new path.

- [ ] **Step 11: Run the gated homepage browser tests with Chrome.** Bash tool, sandbox disabled.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0b-shots GOTMPDIR='D:\tmp\go-build' go test ./internal/tools/ -run 'CaptureHomepageAnnotationSnapshot' -count=1 -v -timeout 10m 2>&1 | grep -E "^(--- |ok|FAIL|\s+homepage)" 
```

Expected: five `--- PASS` lines (`Browser`, `ReachesNothingBeyondThePreviewBrowser`, `NarrowViewportBrowser`, `RejectsConcurrentCapture`, `BlocksPrivateDeployTargets`) and `ok`. If `ReachesNothingBeyond...` fails with the internal server reached, stop: the egress boundary regressed; do not weaken the test. If `...Browser` fails with a bypass error, the path check cannot read the nonce on this Chrome: print the error text and fix `probeEgressViaPage` (A.3).

- [ ] **Step 12: Commit.** Message file `C:/disposable/easydrag-s0b/a4-commit.txt`:

```
refactor(egress): move the five browser sites to LaunchEgressBrowser

The dynamic scraper, web capture, web performance, form automation and the
Homepage annotation capture now each launch a browser and an authenticating
egress proxy per call. getSharedBrowser (a sync.Once singleton that cached a
failed launch for the process) and OpenRodPageWithSSRF/GuardRodPageSSRF (a page
hook that missed service workers, prefetch and WebRTC) are deleted. ValidateSSRF
still runs before each launch, timeouts and messages are kept, and a dead proxy
now fails the call instead of returning partial content. AGENTS.md records the
rule for new headless-browser code.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git add internal/security/rod_ssrf.go internal/scraper/scraper.go internal/scraper/scraper_test.go internal/tools/utils.go internal/tools/web_capture.go internal/tools/web_performance.go internal/tools/form_automation.go internal/tools/homepage_annotation_capture.go internal/security/AGENTS.md internal/tools/AGENTS.md ui/js/desktop/apps/AGENTS.md && git commit -F C:/disposable/easydrag-s0b/a4-commit.txt && git log -1 --stat --format=%s
```

Expected: 11 files changed, `rod_ssrf.go` deleted.

---

### Task A.5: The S8 fixture and the gated browser tests

**Files:**
- Create: `internal/testutil/browser_egress.go`
- Create: `internal/security/egress_s8_browser_test.go`
- Create: `internal/scraper/egress_browser_test.go`
- Create: `internal/tools/egress_browser_test.go`

Facts used: `AURAGO_SSRF_ALLOW_LOOPBACK` is read per call by `allowSSRFLoopback()` (`internal/security/ssrf.go:90-93`: `1` or `true`) and makes `isAllowedPrivateIP` accept loopback only (`:98-104`; RFC 1918 and link-local stay blocked), so the fixture page on `127.0.0.1` is reachable through the proxy while `169.254.169.254`, `10.255.255.1` and `192.168.255.1` are refused. The approach is the one of `homepage_annotation_capture_test.go:206-221`: one loopback `httptest` server serves the page, a service worker script, and the probes run in the page. `internal/testutil` imports no `security` package (checked with `go list -deps`), so a test inside package `security` may import it.

The fixture page tries: `window.open` of the metadata address, a WebSocket to `10.255.255.1:80`, a service worker (`/sw.js`) that fetches `http://192.168.255.1/`, a `<link rel="prefetch">` and a speculation-rules prerender of `http://169.254.169.254/`, and an `RTCPeerConnection` with `stun:10.255.255.1:3478`. After 4 seconds it reports the number of WebRTC candidate pairs to `/report`. The prefetch and prerender probes are best effort (Chrome may not issue a cross-origin prerender); the test requires only the three TCP targets in the proxy's refusal log.

- [ ] **Step 1: Write the fixture.** Create `internal/testutil/browser_egress.go`:

```go
package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
}

// EgressFixture serves the S8 page on IPv4 loopback.
type EgressFixture struct {
	Server *httptest.Server

	mu     sync.Mutex
	report EgressPageReport
	got    chan struct{}
	once   sync.Once
}

const egressFixturePage = `<!doctype html><html><head><meta charset="utf-8">
<script type="speculationrules">{"prerender":[{"source":"list","urls":["http://169.254.169.254/prerender"]}]}</script>
<link rel="prefetch" href="http://169.254.169.254/prefetch">
</head><body><h1>S8</h1>
<script>
(function () {
  function attempt(fn) { try { fn(); } catch (e) {} }
  var pc = null;
  attempt(function () { window.open('http://169.254.169.254/latest/meta-data/'); });
  attempt(function () { new WebSocket('ws://10.255.255.1:80/'); });
  attempt(function () { navigator.serviceWorker.register('/sw.js').catch(function () {}); });
  attempt(function () {
    pc = new RTCPeerConnection({iceServers: [{urls: 'stun:10.255.255.1:3478'}]});
    pc.createDataChannel('x');
    pc.createOffer().then(function (o) { return pc.setLocalDescription(o); }).catch(function () {});
  });
  function report(n) {
    fetch('/report', {method: 'POST', body: JSON.stringify({candidate_pairs: n})}).catch(function () {});
  }
  setTimeout(function () {
    if (!pc) { report(0); return; }
    pc.getStats().then(function (stats) {
      var n = 0;
      stats.forEach(function (s) { if (s.type === 'candidate-pair') { n++; } });
      report(n);
    }).catch(function () { report(-1); });
  }, 4000);
})();
</script></body></html>`

const egressFixtureWorker = `self.addEventListener('install', function (e) {
  e.waitUntil(fetch('http://192.168.255.1/', {mode: 'no-cors'}).catch(function () {}));
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
		switch {
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, egressFixturePage)
		case r.URL.Path == "/sw.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = io.WriteString(w, egressFixtureWorker)
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
```

- [ ] **Step 2: Write the security test.** Create `internal/security/egress_s8_browser_test.go`:

```go
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
	time.Sleep(500 * time.Millisecond) // let late probes reach the proxy

	if err := eb.Err(); err != nil {
		t.Fatalf("browser ended early: %v", err)
	}
	refused, dialed := eb.ProxyRefused(), eb.ProxyDialed()
	for _, target := range testutil.EgressPrivateTargets {
		if !slices.Contains(refused, target) {
			t.Errorf("the proxy never refused %s; refused = %v", target, refused)
		}
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
```

- [ ] **Step 3: Write the scraper test.** Create `internal/scraper/egress_browser_test.go`:

```go
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
```

- [ ] **Step 4: Write the web capture test.** Create `internal/tools/egress_browser_test.go`:

```go
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
```

- [ ] **Step 5: Vet, and run the three tests without Chrome (they must skip, not fail).**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && gofmt -w internal/testutil/browser_egress.go internal/security/egress_s8_browser_test.go internal/scraper/egress_browser_test.go internal/tools/egress_browser_test.go && gofmt -l internal/ ; go vet ./internal/testutil/ ./internal/security/ ./internal/scraper/ ./internal/tools/ && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ ./internal/scraper/ ./internal/tools/ -run 'TestEgressBrowserRefusesTheS8Fixture|TestScraperDynamicGoesThroughTheEgressProxy|TestWebCaptureGoesThroughTheEgressProxy' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```

Expected: no `gofmt` output; `go vet` silent; three `--- SKIP` lines and three `ok`. A `RuntimePermissions has no field AllowFilesystemWrite` error means the field has another name: `grep -n "AllowFilesystemWrite" internal/tools/permissions.go` and use the real one.

- [ ] **Step 6: Run the three tests with Chrome.** Bash tool, sandbox disabled.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0b-shots GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ ./internal/scraper/ ./internal/tools/ -run 'TestEgressBrowserRefusesTheS8Fixture|TestScraperDynamicGoesThroughTheEgressProxy|TestWebCaptureGoesThroughTheEgressProxy' -count=1 -v -timeout 10m 2>&1 | grep -E "^(--- |ok|FAIL|\s+\S+_test.go)"
```

Expected: three `--- PASS` and three `ok`. Failure meanings:
- `the proxy never refused 169.254.169.254:80`: Chrome did not issue the popup (check that `disable-popup-blocking` is in rod's default flags: `grep -n "popup-blocking" C:/Users/Andi/go/pkg/mod/github.com/go-rod/rod@v0.116.2/lib/launcher/launcher.go`, it is) or the page script threw: print the page's console with a one-off `page.EachEvent`. Do not drop the target from the required list; the popup and the WebSocket are the core of S8.
- `192.168.255.1:80` missing: the service worker did not install in time. The page reports after 4 s, and the test waits 500 ms more; raise only the page's report delay (`4000`) and the sleep, not the assertion.
- `candidate pairs, want 0`: `-1` means getStats failed (the page code, not the proxy); a positive number means UDP left the proxy: stop, that is a real finding, report it.
- the prefetch and prerender probes are not asserted: a `169.254.169.254:80` entry may come from the popup alone.

- [ ] **Step 7: Commit.** Message file `C:/disposable/easydrag-s0b/a5-commit.txt`:

```
test(egress): S8 fixture and the gated egress browser tests

testutil.EgressFixture serves a loopback page that tries every route to a private
address a page hook cannot see: window.open of the metadata address, a WebSocket,
a service worker fetch, prefetch and prerender, and a WebRTC STUN probe. The
security test drives LaunchEgressBrowser against it and requires the proxy to
refuse and never dial each private target, with no WebRTC candidate pair. The
scraper and web capture tests read the proxy log of the browser those tools
launch and require the page's own request in it. All three are gated by
AURAGO_RUN_BROWSER_SMOKE=1.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git add internal/testutil/browser_egress.go internal/security/egress_s8_browser_test.go internal/scraper/egress_browser_test.go internal/tools/egress_browser_test.go && git commit -F C:/disposable/easydrag-s0b/a5-commit.txt && git log -1 --stat --format=%s
```

---

### Task A.6: The live run on aurago-test

**Files:**
- Modify: `plans/2026-11-11-easydrag-s0b-spec.md` (an "A.6 result" paragraph appended to §3.6)
- Create (outside the repo, deleted after): `C:/disposable/easydrag-s0b/security.test`, `/tmp/security.test` on aurago-test

**Standing rule (aurago-test).** The run is read-only on that server: it creates no container, app or data and changes none. The test binary starts its own Chrome and its own loopback listeners, uses a profile directory under `/tmp` that it removes itself, and touches neither the live AuraGo, its config, its vault, Docker, nor the RTL-SDR dongle. Do not stop, restart or deploy anything there.

- [ ] **Step 1: Check that a Chrome exists on the server.**

```bash
ssh aurago-test 'which chromium chromium-browser google-chrome google-chrome-stable 2>/dev/null; echo "exit=$?"'
```

Expected: at least one path printed. If nothing is printed, stop and report: the live run cannot happen, and installing a browser there is not allowed by the standing rule. Record that in the "A.6 result" paragraph of Step 6 and ask the coordinator.

- [ ] **Step 2: Build the Linux test binary.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTMPDIR='D:\tmp\go-build' go test -c ./internal/security -o C:/disposable/easydrag-s0b/security.test && ls -l C:/disposable/easydrag-s0b/security.test
```

Expected: a file of tens of MB. If the build fails because the package needs cgo, rerun without `CGO_ENABLED=0` and, if that fails too, report the error.

- [ ] **Step 3: Copy it over.**

```bash
scp C:/disposable/easydrag-s0b/security.test aurago-test:/tmp/security.test && ssh aurago-test 'chmod +x /tmp/security.test && ls -l /tmp/security.test'
```

- [ ] **Step 4: Run the S8 test once, against the server's Chrome.**

```bash
ssh aurago-test 'AURAGO_RUN_BROWSER_SMOKE=1 /tmp/security.test -test.run TestEgressBrowserRefusesTheS8Fixture -test.v -test.timeout 3m' 2>&1 | tail -30
```

Expected: `--- PASS: TestEgressBrowserRefusesTheS8Fixture` and `PASS`. Read the result as follows:
- `--- SKIP ... Chrome/Edge unavailable`: the test did not run (the lookup list has `google-chrome`, `chromium`, `chromium-browser` but the binary has another name): rerun with `AURAGO_TEST_CHROME=<path printed in Step 1>`. A skip is not an acceptance.
- `No usable sandbox` in the launch error: Chrome refuses to start inside this host's user-namespace limits. That is the host's policy (see `homepageAnnotationLaunchError`); do not change the server. Record it in the result paragraph and report it to the coordinator.
- any `the proxy dialled ...` or `candidate pairs, want 0` failure is a real egress finding: stop and report it verbatim.

- [ ] **Step 5: Remove the binary on the server and locally.**

```bash
ssh aurago-test 'rm -f /tmp/security.test; ls /tmp/security.test 2>&1 | head -1'; rm -f C:/disposable/easydrag-s0b/security.test
```

Expected: `ls: cannot access '/tmp/security.test': No such file or directory`.

- [ ] **Step 6: Record the result in the spec.** Write `C:/disposable/easydrag-s0b/a6-result.md` with the Write tool, filling the four facts from the run (date, the Chrome version line, the pass line, and the three refused targets). The text for a passing run:

```
**A.6 result (2026-11-11).** Ran `TestEgressBrowserRefusesTheS8Fixture` once on aurago-test as a Linux test binary (`go test -c`, copied to `/tmp`, removed after the run) against the server's Chrome. Result: PASS. The proxy refused and never dialled 169.254.169.254:80, 10.255.255.1:80 and 192.168.255.1:80, the page's own request went through the proxy, and the page formed no WebRTC candidate pair. The run created and changed nothing on the server (no container, app or data).
```

For a failing or skipped run, replace the second and third sentences with what Step 4 printed (the exact failing assertion, or the sandbox message) and keep the last sentence. Then:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && sed -i '/^\*\*Live on aurago-test\.\*\*/r C:/disposable/easydrag-s0b/a6-result.md' plans/2026-11-11-easydrag-s0b-spec.md && git diff --stat && grep -n "A.6 result" plans/2026-11-11-easydrag-s0b-spec.md
```

Expected: one file changed, the paragraph follows the "Live on aurago-test" paragraph in §3.6. If the file `a6-result.md` does not start with a blank line, add one so the paragraph is separated from the previous one.

- [ ] **Step 7: Final gates for the part, then commit.** Run the part's whole Go surface once more, so the branch is ready for the coordinator.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && gofmt -l internal/ ; go vet ./internal/security/ ./internal/scraper/ ./internal/tools/ ./internal/testutil/ && GOTMPDIR='D:\tmp\go-build' go test ./internal/security/ ./internal/scraper/ ./internal/tools/ -count=1 -timeout 15m 2>&1 | tail -5 && git status --short
```

Expected: no `gofmt` output; `go vet` silent; three `ok` lines (the same three that were `ok` at the baseline; a failure that was already in the baseline is reported, not fixed); `git status --short` prints nothing but the spec file. Message file `C:/disposable/easydrag-s0b/a6-commit.txt`:

```
docs(plans): S0b A.6 result, S8 fixture run on aurago-test

Records the one live run of TestEgressBrowserRefusesTheS8Fixture on aurago-test
(Linux test binary against the server's Chrome). The run was read-only: no
container, app or data was created or changed, and the binary was removed.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0b-egress && git add plans/2026-11-11-easydrag-s0b-spec.md && git commit -F C:/disposable/easydrag-s0b/a6-commit.txt && git log --oneline main..HEAD
```

Expected: six commits, A.1 to A.6, on `feat/easydrag-s0b-egress`. Do not push and do not merge; the coordinator lands Part A (it is the smaller part and lands first).

---

### Part A coverage

| Spec item | Task |
|---|---|
| §3.3 spike on proxy authentication (all kinds), spec amendment | A.1 |
| §3.3 per-call proxy credentials, 407 for plain requests and CONNECT | A.2 |
| §3.4 `Done()` channel on the proxy | A.2 |
| §3.4 check host answered by the proxy without dialling | A.2 |
| §3.6 "proxy logs each private target" (refusal log) | A.2 |
| §3.2 helper API, semaphore of 4, 2 minute lifetime, flags, fresh profile | A.3 |
| §3.3 credentials over DevTools only (never command line, env, file) | A.3 |
| §3.4 `ErrEgressProxyDied`, `ErrEgressBypassed`, `ErrEgressLifetime`, fail closed | A.3 |
| §3.2 slot released on every failure path | A.3 |
| §3.5 five call sites moved, `ValidateSSRF` kept in front | A.4 |
| §3.5 `getSharedBrowser`, `OpenRodPageWithSSRF`, `GuardRodPageSSRF` deleted | A.4 |
| §3.5 AGENTS.md rule in `internal/security` and `internal/tools` | A.4 |
| §3.6 S8 fixture and `TestEgressBrowserRefusesTheS8Fixture` | A.5 |
| §3.6 `TestScraperDynamicGoesThroughTheEgressProxy`, `TestWebCaptureGoesThroughTheEgressProxy` | A.5 |
| §3.6 live run on aurago-test, recorded in the spec | A.6 |
| §7 Part A unit tests (407, slot release, proxy death, path check) | A.2, A.3 |
| §9 first four acceptance boxes | A.3, A.4, A.5, A.6 |
