## Part B: Server API, job manager and silent render pipeline

Part B turns a validated story into an MP4 and serves the Storymaker API. It builds on part A (`internal/storymaker`: story types, `DecodeStory`, `ApplyDefaults`, `Validate`, `Catalog`, `ProjectStore`, `Store`), part C (the player bundles `ui/js/storymaker/host.bundle.js` and `player.bundle.js`, the render page protocol) and part D (the starter catalog under `ui/img/storymaker/`, four Geist font faces). Everything of B except the HTTP handlers lives in the new subpackage `internal/storymaker/render/` (package `render`); the handlers are `internal/server/storymaker_*.go`.

**What is built, in task order**

- B1 `security.StartStrictCaptureProxy`: a capture proxy that dials only the render origin.
- B2 `videostudio` exports (`Encoder.GlobalArgs/FilterTail/VideoArgs`, `ResolveFFmpeg`) so the frame-input graph uses the same encoder pieces as Video Studio.
- B3 job manager: `jobs.json`, queue cap 16 (`ErrBusy`), one render at a time, cancel, restart-safe, change notifications for SSE.
- B4 browser resolution (`render.browser_path` -> system Chrome -> pinned Chromium revision 1321438 with SHA-256 per platform) and the cached launch probe.
- B5 FFmpeg frame-input pipeline (`frame.go`, `ffmpeg.go`): BT.709 conversion, silent AAC, `+faststart`, whitelists, hardware settle.
- B6 strict frame ingest (`ingest.go`): order, count, size, header, running state.
- B7 per-job loopback listener (`listener.go`): routes of contract section 7, CORS, exact CSP, 404 outside the base.
- B8 compile-report validation, the CDP egress guard and the Chrome session (`report.go`, `egress.go`, `session.go`).
- B9 the render job (`render.go`): snapshot, strict proxy, Chrome, report, FFmpeg, watchdogs, restart, outputs, cleanup.
- B10 browser tests: real end-to-end render, determinism, egress regression by layer, pinned-Chromium acceptance, spike harnesses.
- B11 server: `Store` over `*desktop.Service`, runtime, the API of contract section 5, the player host page, route and gate wiring, tests.
- B12 parity test between the app player host (sandboxed iframe) and the render page.
- B13 the frame-transport spike: measurements on this box, the decision, and the repeatable step for aurago-test (Linux).
- B14 documentation (`internal/storymaker/render/AGENTS.md`, section for `internal/storymaker/AGENTS.md`).
- B15 two small player changes B asks of part C (frames as CORS-simple requests, per-frame deadline from `boot.json`); optional, measured gain 14 percent in rendering speed and no deadline failures on a slow host.

**Inputs.** A, C and D landed (tasks of those parts). B1 to B9 do not need the player; B10 and B12 need the built bundles (`npm run build:ui`) and the starter catalog. B11 needs A's `ProjectStore` and `NormalizeStorymakerConfig`.

**Test environment.** On the Windows development box set `GOCACHE=D:\dev-cache\go-build` and `GOTMPDIR=D:\tmp\go-build` (backslashes). FFmpeg is not on the Git Bash PATH: set `AURAGO_TEST_FFMPEG_DIR` to the folder that holds `ffmpeg` and `ffprobe` (WinGet: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\Gyan.FFmpeg_*\ffmpeg-*\bin`); tests that need it skip when it is missing. Browser tests need `AURAGO_RUN_BROWSER_SMOKE=1` and an installed Chrome (`AURAGO_TEST_CHROME` overrides the lookup); run them through `go test` (test binaries live in `GOTMPDIR`; a repo-built executable cannot start Chrome on this box). The transport spike also needs `AURAGO_STORYMAKER_SPIKE=1`.

**Decisions that deviate from, or complete, the contract** (details and measurements are in `part-B-notes.md`, committed next to this file):

- The default transport is **png**; `storymaker.render.transport` selects another. Measured on the production path (strict proxy and CDP guard active): png 10 to 16 fps, lossless WebP 4.5, screenshot 4.1, raw RGBA 1.5 at 1080p; every transport gives bit-identical pixels and is deterministic.
- CDP: `Fetch.enable` does not exist on a worker's own session in Chrome 156 and pausing a frame upload costs about 35 ms. The guard therefore pauses only navigations and blocks everything else natively with `Network.setBlockedURLs` patterns on the page and on every auto-attached worker; the catch-all interception is the fallback. Three independent layers (CSP and sandbox, strict proxy, CDP guard) are each proven to stop a probe by `TestEgressLayersEachStopTheProbe`.
- The browser status has two extra fields, `can_download` and `version`: the pinned Chromium is downloaded by the first export, never by a status request.
- The render request is snapshotted when it is queued (story, code bundles), not when it starts; catalog, fonts and player bundles are snapshotted at start.
- `GET /projects/{id}/jobs` is an addition to the contract (the app needs it to find a running export after a reload).
- The silent AAC track (`anullsrc`, 48 kHz stereo, 64 kbit/s) is part of every export so the stream layout is the same as in later phases.

### Task B1: Strict capture proxy

**Files:**
- Modify: `internal/security/capture_proxy.go:60,73,102,103,119,127,144,267` (see the diff)
- Test: `internal/security/capture_proxy_strict_test.go`

The render page must reach its own listener and nothing else: other loopback ports, other hosts, `localhost` and the `AURAGO_SSRF_ALLOW_LOOPBACK` escape hatch are all refused, and a refusal costs no DNS lookup. `StartCaptureProxy` and its callers (`internal/tools/homepage_annotation_capture.go:116`) keep their behaviour; the response header timeout becomes a limit (`CaptureProxyLimits.ResponseHeaderTimeout`, default 20 s) because the render listener answers `/report` only after FFmpeg started.

- [ ] **Step 1: Write the failing test**

```go
package security

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func startStrictTestProxy(t *testing.T, allowed *countedServer, limits CaptureProxyLimits) *CaptureProxy {
	t.Helper()
	p, err := StartStrictCaptureProxy(mustParseURL(t, allowed.URL), limits)
	if err != nil {
		t.Fatalf("StartStrictCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestStrictCaptureProxyNeedsAnAllowedOrigin(t *testing.T) {
	if p, err := StartStrictCaptureProxy(nil, CaptureProxyLimits{}); err == nil {
		_ = p.Close()
		t.Fatal("a strict proxy without an allowed origin was started")
	}
}

func TestStrictCaptureProxyServesTheAllowedOriginOnly(t *testing.T) {
	allowed, other := captureProxyTestOrigins(t) // both are 127.0.0.1 servers on different ports
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{})
	client := proxyClient(t, p)

	res, err := client.Get(allowed.URL + "/page")
	if err != nil {
		t.Fatalf("allowed origin: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "allowed /page" {
		t.Fatalf("allowed origin = %d %q", res.StatusCode, body)
	}

	// A POST with a body reaches the allowed origin as well (frames travel that way).
	res, err = client.Post(allowed.URL+"/post", "application/octet-stream", strings.NewReader("frame-bytes"))
	if err != nil {
		t.Fatalf("post to the allowed origin: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("post status = %d", res.StatusCode)
	}

	// Another loopback port is refused without a single connection.
	res, err = client.Get(other.URL + "/secret")
	if err != nil {
		t.Fatalf("other loopback port: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("other loopback port status = %d, want 403", res.StatusCode)
	}
	if got := other.accepts.Load(); got != 0 {
		t.Fatalf("the other loopback server accepted %d connections", got)
	}
}

func TestStrictCaptureProxyRefusesEveryOtherDestinationForm(t *testing.T) {
	allowed, other := captureProxyTestOrigins(t)
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{})
	port := strings.Split(allowed.hostPort(), ":")[1]
	targets := []string{
		other.hostPort(),                    // another loopback port
		"localhost:" + port,                 // another name for the allowed address
		"[::1]:" + port,                     // another address family
		"127.0.0.2:" + port,                 // another loopback address
		"example.com:443", "example.com:80", // a public name
		"93.184.216.34:443", // a public address
		"10.0.0.7:80", "169.254.169.254:80",
	}
	for _, target := range targets {
		if _, _, status := rawConnect(t, p, target); status != http.StatusForbidden {
			t.Errorf("CONNECT %s status = %d, want 403", target, status)
		}
	}
	// The same destinations as plain absolute-form requests.
	client := proxyClient(t, p)
	for _, target := range []string{"http://" + other.hostPort() + "/", "http://localhost:" + port + "/", "http://example.com/"} {
		res, err := client.Get(target)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("GET %s status = %d, want 403", target, res.StatusCode)
		}
	}
	if got := other.accepts.Load(); got != 0 {
		t.Fatalf("the other loopback server accepted %d connections", got)
	}
}

func TestStrictCaptureProxyIgnoresTheLoopbackEscapeHatch(t *testing.T) {
	allowed, other := captureProxyTestOrigins(t)
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{})
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	if _, _, status := rawConnect(t, p, other.hostPort()); status != http.StatusForbidden {
		t.Fatalf("with AURAGO_SSRF_ALLOW_LOOPBACK=1 a strict proxy answered %d for another loopback port, want 403", status)
	}
	if got := other.accepts.Load(); got != 0 {
		t.Fatalf("the other loopback server accepted %d connections", got)
	}
}

func TestStrictCaptureProxyKeepsTheNonStrictBehaviourOfTheOthers(t *testing.T) {
	// The default proxy still dials a loopback destination under the escape hatch, as before.
	allowed, other := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	if _, _, status := rawConnect(t, p, other.hostPort()); status != http.StatusOK {
		t.Fatalf("non-strict CONNECT status = %d, want 200 (existing behaviour)", status)
	}
}

func TestStrictCaptureProxyHonoursTheResponseHeaderTimeout(t *testing.T) {
	slow := newCountedServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond)
		_, _ = io.WriteString(w, "late")
	})
	get := func(limits CaptureProxyLimits) int {
		p := startStrictTestProxy(t, slow, limits)
		client := proxyClient(t, p)
		res, err := client.Get(slow.URL + "/")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if status := get(CaptureProxyLimits{ResponseHeaderTimeout: 150 * time.Millisecond}); status != http.StatusBadGateway {
		t.Fatalf("short header timeout status = %d, want 502", status)
	}
	if status := get(CaptureProxyLimits{ResponseHeaderTimeout: 5 * time.Second}); status != http.StatusOK {
		t.Fatalf("long header timeout status = %d, want 200", status)
	}
}

func TestStrictCaptureProxyCountsBodiesAgainstTheByteLimit(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startStrictTestProxy(t, allowed, CaptureProxyLimits{MaxBytes: 4096})
	client := proxyClient(t, p)
	body := strings.NewReader(strings.Repeat("x", 64<<10))
	res, err := client.Post(allowed.URL+"/upload", "application/octet-stream", body)
	if err == nil {
		_ = res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Fatalf("a 64 KiB body passed a 4 KiB limit")
		}
	}
	// After the limit is spent nothing new is dialed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(allowed.URL + "/page")
		if err != nil {
			return
		}
		status := res.StatusCode
		_ = res.Body.Close()
		if status != http.StatusOK {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the proxy kept serving after its byte limit")
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/security/ -run "StrictCaptureProxy" -count=1`

Expected: FAIL (build failed: `undefined: StartStrictCaptureProxy`, `unknown field ResponseHeaderTimeout in struct literal of type CaptureProxyLimits`)

- [ ] **Step 3: Implement**

Apply this change to `internal/security/capture_proxy.go` (the file is the same at the drafting base `0248bc133` and at main HEAD `561685cde`; hunk headers give the lines):

```diff
diff --git a/internal/security/capture_proxy.go b/internal/security/capture_proxy.go
index 910585a21..2023ec6fb 100644
--- a/internal/security/capture_proxy.go
+++ b/internal/security/capture_proxy.go
@@ -58,6 +58,11 @@ type CaptureProxyLimits struct {
 	// MaxBytes is the total number of bytes relayed in both directions; the
 	// connections are cut when it is exceeded.
 	MaxBytes int64
+	// ResponseHeaderTimeout is how long the proxy waits for an upstream to start
+	// answering a plain HTTP request. Zero selects 20 seconds; a strict proxy in
+	// front of a server that answers late (a render listener that starts FFmpeg
+	// before it replies) raises it.
+	ResponseHeaderTimeout time.Duration
 }
 
 // CaptureProxy is an in-process forward proxy for one headless-browser capture.
@@ -71,6 +76,7 @@ type CaptureProxyLimits struct {
 // validated IP itself is dialed, so there is no second lookup to rebind.
 type CaptureProxy struct {
 	allowed string // canonical host:port of the trusted origin, "" for none
+	strict  bool   // dial the allowed origin only; no SSRF-checked public destinations
 	limits  CaptureProxyLimits
 	idle    time.Duration // tunnel idle timeout
 
@@ -100,7 +106,24 @@ func StartCaptureProxy(allowed *url.URL, limits CaptureProxyLimits) (*CapturePro
 	return startCaptureProxy(allowed, limits, captureProxyIdleTimeout)
 }
 
+// StartStrictCaptureProxy is StartCaptureProxy for a page that must talk to its own
+// origin and nothing else (the Storymaker render page). The proxy dials the allowed
+// origin's host:port exactly as given and refuses every other destination: other
+// loopback ports, other hosts, public addresses, name forms of the same address
+// (localhost) and the AURAGO_SSRF_ALLOW_LOOPBACK escape hatch included. A destination
+// is never resolved, so a refusal costs no DNS lookup. allowed is required.
+func StartStrictCaptureProxy(allowed *url.URL, limits CaptureProxyLimits) (*CaptureProxy, error) {
+	if allowed == nil {
+		return nil, fmt.Errorf("strict capture proxy needs an allowed origin")
+	}
+	return startCaptureProxyMode(allowed, limits, captureProxyIdleTimeout, true)
+}
+
 func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration) (*CaptureProxy, error) {
+	return startCaptureProxyMode(allowed, limits, idle, false)
+}
+
+func startCaptureProxyMode(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration, strict bool) (*CaptureProxy, error) {
 	allowedHostPort := ""
 	if allowed != nil {
 		if (allowed.Scheme != "http" && allowed.Scheme != "https") || allowed.Hostname() == "" {
@@ -117,6 +140,9 @@ func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Du
 	if limits.MaxBytes <= 0 {
 		limits.MaxBytes = defaultCaptureProxyBytes
 	}
+	if limits.ResponseHeaderTimeout <= 0 {
+		limits.ResponseHeaderTimeout = captureProxyResponseTimeout
+	}
 	listener, err := net.Listen("tcp", "127.0.0.1:0")
 	if err != nil {
 		return nil, fmt.Errorf("capture proxy listen: %w", err)
@@ -125,6 +151,7 @@ func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Du
 	ctx, cancel := context.WithCancel(context.Background())
 	p := &CaptureProxy{
 		allowed:  allowedHostPort,
+		strict:   strict,
 		limits:   limits,
 		idle:     idle,
 		listener: listener,
@@ -141,7 +168,7 @@ func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Du
 		MaxIdleConns:           limits.MaxConcurrent,
 		MaxIdleConnsPerHost:    2,
 		IdleConnTimeout:        idle,
-		ResponseHeaderTimeout:  captureProxyResponseTimeout,
+		ResponseHeaderTimeout:  limits.ResponseHeaderTimeout,
 		MaxResponseHeaderBytes: 256 << 10,
 	}
 	p.server = &http.Server{
@@ -265,6 +292,9 @@ func (p *CaptureProxy) destination(ctx context.Context, hostPort string) (string
 	if p.allowed != "" && captureHostPort(host, port) == p.allowed {
 		return p.allowed, nil
 	}
+	if p.strict {
+		return "", fmt.Errorf("%w: only the render origin is reachable", errCaptureDestinationBlocked)
+	}
 	target, _, err := validatedSSRFDialTarget(ctx, hostPort)
 	if err != nil {
 		var dnsErr *net.DNSError
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/security/ -run "CaptureProxy" -count=1`

Expected: PASS (the existing `CaptureProxy` tests and the 7 strict tests)

- [ ] **Step 5: Commit**

```bash
git add internal/security/capture_proxy.go internal/security/capture_proxy_strict_test.go
git commit -m "feat(security): strict capture proxy mode that dials only the allowed origin" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B2: Encoder exports for frame-input pipelines

**Files:**
- Create: `internal/videostudio/encoder_export.go`
- Test: `internal/videostudio/encoder_export_test.go`

`Encoder.globalArgs`, `filterTail` and `videoArgs` are unexported; the Storymaker render job needs the same pieces (hardware device arguments, upload filter, codec arguments with the BT.709 tags) so one `SelectEncoder` choice produces the same stream settings everywhere. These exports are thin wrappers; no existing function is edited, so GitNexus `impact` has nothing to measure.

- [ ] **Step 1: Write the failing test**

```go
package videostudio

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEncoderExportsMatchTheInternalViews(t *testing.T) {
	rate := Rate{Num: 30, Den: 1}
	for _, e := range []Encoder{{}, {Codec: CodecVAAPI, Device: "/dev/dri/renderD129"}, {Codec: CodecQSV}, {Codec: CodecNVENC, Quality: 20}, {Codec: CodecAMF}} {
		if !reflect.DeepEqual(e.GlobalArgs(), e.globalArgs()) {
			t.Errorf("%+v: GlobalArgs differs", e)
		}
		if e.FilterTail() != e.filterTail() {
			t.Errorf("%+v: FilterTail differs", e)
		}
		if !reflect.DeepEqual(e.VideoArgs(rate), e.videoArgs(rate)) {
			t.Errorf("%+v: VideoArgs differs", e)
		}
	}
	// The pieces a frame-input graph relies on.
	if got := (Encoder{Codec: CodecVAAPI}).FilterTail(); got != ",format=nv12,hwupload" {
		t.Errorf("VAAPI filter tail = %q", got)
	}
	if got := strings.Join((Encoder{}).VideoArgs(rate), " "); !strings.Contains(got, "-c:v libx264") || !strings.Contains(got, "-colorspace bt709") {
		t.Errorf("software video args = %q", got)
	}
}

func TestResolveFFmpegUsesTheSharedPolicy(t *testing.T) {
	if _, err := ResolveFFmpeg(filepath.Join(t.TempDir(), "no-ffmpeg")); err == nil {
		t.Error("a configured path that does not exist was accepted")
	}
	if _, err := ResolveFFmpeg(t.TempDir()); err == nil {
		t.Error("a directory was accepted as the FFmpeg binary")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/videostudio/ -run "TestEncoderExports|TestResolveFFmpegUses" -count=1`

Expected: FAIL (`e.GlobalArgs undefined (type Encoder has no field or method GlobalArgs, but does have method globalArgs)`, `undefined: ResolveFFmpeg`)

- [ ] **Step 3: Implement**

```go
package videostudio

// Exported views of the encoder pieces that frame-input pipelines outside this package (Storymaker's
// render job) need. They return exactly what the Video Studio segment and preview graphs use, so one
// encoder choice (SelectEncoder) produces the same stream settings everywhere.

// GlobalArgs returns the arguments that initialise the encoder's hardware device; they precede all inputs.
func (e Encoder) GlobalArgs() []string { return e.globalArgs() }

// FilterTail returns the filter text appended after the format=yuv420p conversion ("" for software encoders,
// ",format=nv12,hwupload" for VAAPI and QSV).
func (e Encoder) FilterTail() string { return e.filterTail() }

// VideoArgs returns the video codec arguments for the output (codec, quality, GOP and the BT.709 tags).
func (e Encoder) VideoArgs(rate Rate) []string { return e.videoArgs(rate) }

// ResolveFFmpeg resolves the FFmpeg binary with the shared policy: a configured path must be an existing
// regular file, a configured bare name is looked up on PATH, an empty path searches PATH.
func ResolveFFmpeg(configuredPath string) (string, error) {
	return resolveBinary(configuredPath, "ffmpeg")
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/videostudio/ -run "TestEncoderExports|TestResolveFFmpegUses" -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/videostudio/encoder_export.go internal/videostudio/encoder_export_test.go
git commit -m "feat(videostudio): export the encoder pieces that frame-input pipelines need" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B3: Job manager

**Files:**
- Create: `internal/storymaker/render/jobs.go`
- Test: `internal/storymaker/render/jobs_test.go`

`jobs.json` holds the public job fields and the SHA-256 of the render prefix (`HashPrefix`), never the prefix. A job that was queued or running at start becomes `interrupted` (code `interrupted`) and the staging folder is emptied. At most 16 jobs wait (`ErrBusy`), one runs at a time, cancel works for queued and running jobs, progress notifications are throttled (150 ms) and persisted at most every 3 s, subscribers get a coalesced signal and read the changed jobs with `ChangedSince` (the SSE handler does). An `Idempotency-Key` returns the earlier job for the same project and request fingerprint. The request payload (the render snapshot) travels in memory.

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/jobs_test.go`:

```go
package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	projA = "11111111-1111-4111-8111-111111111111"
	projB = "22222222-2222-4222-8222-222222222222"
)

func newTestManager(t *testing.T, dir string, run RunFunc) *Manager {
	t.Helper()
	m, err := NewManager(ManagerOptions{Dir: dir, Run: run})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(m.Close)
	return m
}

func waitJob(t *testing.T, m *Manager, id string, pred func(Job) bool) Job {
	t.Helper()
	return waitJobFor(t, m, id, pred, 15*time.Second)
}

func waitJobFor(t *testing.T, m *Manager, id string, pred func(Job) bool, limit time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if j, ok := m.Get(id); ok && pred(j) {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	j, _ := m.Get(id)
	t.Fatalf("job %s did not reach the expected state, last: %+v", id, j)
	return Job{}
}

func terminal(j Job) bool { return j.Terminal() }

func TestJobLifecycleSuccessAndPersistence(t *testing.T) {
	dir := t.TempDir()
	m := newTestManager(t, dir, func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		h.SetPrefixHash(HashPrefix("secret-prefix-123"))
		h.SetPhase(PhaseRendering)
		h.SetFrames(5, 10)
		return Output{Path: "Documents/Storymaker/p/exports/x.mp4", Size: 42}, nil
	})
	sub, unsub := m.Subscribe(projA)
	defer unsub()
	job, replayed, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
	if err != nil || replayed {
		t.Fatalf("Enqueue = %v, replayed %v", err, replayed)
	}
	if job.Status != StatusQueued || job.Kind != KindRender || !validJobID(job.ID) {
		t.Fatalf("queued job = %+v", job)
	}
	done := waitJob(t, m, job.ID, terminal)
	if done.Status != StatusSucceeded || done.Progress != 1 || done.Output == nil || done.Output.Size != 42 || done.Error != nil {
		t.Fatalf("finished job = %+v", done)
	}
	select {
	case <-sub:
	default:
		t.Fatal("the subscriber was never signalled")
	}
	changed, rev := m.ChangedSince(projA, 0)
	if len(changed) != 1 || changed[0].ID != job.ID || rev == 0 {
		t.Fatalf("ChangedSince = %+v, %d", changed, rev)
	}
	if again, _ := m.ChangedSince(projA, rev); len(again) != 0 {
		t.Fatalf("a second ChangedSince returned %+v", again)
	}
	if other, _ := m.ChangedSince(projB, 0); len(other) != 0 {
		t.Fatalf("another project saw %+v", other)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-prefix-123") {
		t.Fatal("jobs.json holds the render prefix")
	}
	if !strings.Contains(string(raw), HashPrefix("secret-prefix-123")) {
		t.Fatal("jobs.json does not hold the prefix hash")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("jobs.json is not JSON: %v", err)
	}

	m.Close()
	m2 := newTestManager(t, dir, func(context.Context, Job, any, *Handle) (Output, error) { return Output{}, nil })
	reloaded, ok := m2.Get(job.ID)
	if !ok || reloaded.Status != StatusSucceeded || reloaded.Output == nil || reloaded.Output.Path != done.Output.Path {
		t.Fatalf("reloaded job = %+v, %v", reloaded, ok)
	}
	if m2.PrefixHash(job.ID) != HashPrefix("secret-prefix-123") {
		t.Fatal("the prefix hash did not survive a restart")
	}
}

func TestJobFailureCodes(t *testing.T) {
	var n atomic.Int32
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		switch n.Add(1) {
		case 1:
			return Output{}, Fail(CodeCodeError, "scene s3 is broken")
		case 2:
			return Output{}, errors.New("disk exploded")
		default:
			panic("boom")
		}
	})
	want := []string{CodeCodeError, CodeRenderFailed, CodeRenderFailed}
	for i := range want {
		job, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
		if err != nil {
			t.Fatal(err)
		}
		done := waitJob(t, m, job.ID, terminal)
		if done.Status != StatusFailed || done.Error == nil || done.Error.Code != want[i] {
			t.Fatalf("job %d = %+v", i, done)
		}
	}
}

func TestJobsRunOneAtATimeAndTheQueueHasACap(t *testing.T) {
	release := make(chan struct{})
	var running, peak atomic.Int32
	started := make(chan string, 64)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		cur := running.Add(1)
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		started <- job.ID
		select {
		case <-release:
		case <-ctx.Done():
		}
		running.Add(-1)
		return Output{}, nil
	})
	first, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
	if err != nil {
		t.Fatal(err)
	}
	<-started // the first job runs and blocks
	var queued []string
	for i := 0; i < DefaultMaxQueued; i++ {
		j, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
		if err != nil {
			t.Fatalf("queued job %d: %v", i, err)
		}
		queued = append(queued, j.ID)
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA}); !errors.Is(err, ErrBusy) {
		t.Fatalf("job 17 = %v, want ErrBusy", err)
	}
	// Cancelling a queued job frees a place.
	if j, ok := m.Cancel(queued[0]); !ok || j.Status != StatusCancelled {
		t.Fatalf("Cancel(queued) = %+v, %v", j, ok)
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projB}); err != nil {
		t.Fatalf("enqueue after a cancel: %v", err)
	}
	close(release)
	waitJob(t, m, first.ID, terminal)
	for _, id := range queued[1:] {
		waitJob(t, m, id, terminal)
	}
	if peak.Load() != 1 {
		t.Fatalf("%d jobs ran at once", peak.Load())
	}
}

func TestCancelRunningJob(t *testing.T) {
	started := make(chan struct{})
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		close(started)
		<-ctx.Done()
		return Output{}, ctx.Err()
	})
	job, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	<-started
	if j, ok := m.Cancel(job.ID); !ok || j.Status != StatusRunning {
		t.Fatalf("Cancel(running) = %+v, %v", j, ok)
	}
	done := waitJob(t, m, job.ID, terminal)
	if done.Status != StatusCancelled || done.Error == nil || done.Error.Code != CodeCancelled {
		t.Fatalf("cancelled job = %+v", done)
	}
	// Cancelling again changes nothing.
	if j, ok := m.Cancel(job.ID); !ok || j.Status != StatusCancelled {
		t.Fatalf("second Cancel = %+v", j)
	}
	if _, ok := m.Cancel("00000000-0000-4000-8000-000000000000"); ok {
		t.Fatal("Cancel of an unknown job succeeded")
	}
}

func TestRestartMarksUnfinishedJobsInterruptedAndClearsStaging(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(StagingRoot(dir), "leftover")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	// Write a jobs.json by hand, as a crashed server would have left it.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	doc := `{"version":1,"jobs":[` +
		`{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","project_id":"` + projA + `","kind":"render","status":"running","progress":0.4,"phase":"rendering","frames_done":4,"frames_total":10,"created_at":"` + now + `","updated_at":"` + now + `","rev":3},` +
		`{"id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","project_id":"` + projA + `","kind":"render","status":"queued","progress":0,"frames_done":0,"frames_total":0,"created_at":"` + now + `","updated_at":"` + now + `","rev":4},` +
		`{"id":"not-a-uuid","project_id":"` + projA + `","kind":"render","status":"queued","created_at":"` + now + `","updated_at":"` + now + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newTestManager(t, dir, func(context.Context, Job, any, *Handle) (Output, error) { return Output{}, nil })
	for _, id := range []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"} {
		j, ok := m.Get(id)
		if !ok || j.Status != StatusInterrupted || j.Error == nil || j.Error.Code != CodeInterrupted || j.FinishedAt == nil {
			t.Fatalf("job %s = %+v, %v", id, j, ok)
		}
	}
	if _, ok := m.Get("not-a-uuid"); ok {
		t.Fatal("a job with an invalid id was loaded")
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("the staging folder survived the restart: %v", err)
	}
	// New revisions continue after the loaded ones.
	j, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	if changed, _ := m.ChangedSince(projA, 4); len(changed) == 0 || changed[len(changed)-1].ID != j.ID {
		t.Fatalf("revisions did not continue: %+v", changed)
	}
}

func TestCloseInterruptsTheRunningJob(t *testing.T) {
	started := make(chan struct{})
	m, err := NewManager(ManagerOptions{Dir: t.TempDir(), Run: func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		close(started)
		<-ctx.Done()
		return Output{}, ctx.Err()
	}})
	if err != nil {
		t.Fatal(err)
	}
	running, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	<-started
	waiting, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	m.Close()
	for _, id := range []string{running.ID, waiting.ID} {
		if j, _ := m.Get(id); j.Status != StatusInterrupted {
			t.Fatalf("job %s after Close = %+v", id, j)
		}
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Enqueue after Close = %v", err)
	}
}

func TestIdempotencyKey(t *testing.T) {
	gate := make(chan struct{})
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		<-gate
		return Output{}, nil
	})
	defer close(gate)
	a, replay, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Key: "k1", Fingerprint: "story-v1"})
	if err != nil || replay {
		t.Fatalf("first = %v %v", err, replay)
	}
	b, replay, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Key: "k1", Fingerprint: "story-v1"})
	if err != nil || !replay || b.ID != a.ID {
		t.Fatalf("replay = %+v %v %v", b, replay, err)
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Key: "k1", Fingerprint: "story-v2"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("same key, other story = %v", err)
	}
	// The same key in another project is a different request.
	c, replay, err := m.Enqueue(EnqueueRequest{ProjectID: projB, Key: "k1", Fingerprint: "story-v1"})
	if err != nil || replay || c.ID == a.ID {
		t.Fatalf("other project = %+v %v %v", c, replay, err)
	}
}

func TestCancelProjectCancelsRunningAndQueued(t *testing.T) {
	started := make(chan struct{}, 4)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		started <- struct{}{}
		<-ctx.Done()
		return Output{}, ctx.Err()
	})
	a, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	<-started
	b, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	c, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projB})
	m.CancelProject(projA)
	for _, id := range []string{a.ID, b.ID} {
		if j := waitJob(t, m, id, terminal); j.Status != StatusCancelled {
			t.Fatalf("job %s = %+v", id, j)
		}
	}
	// The other project's job gets its turn and keeps running.
	<-started
	if j, _ := m.Get(c.ID); j.Status != StatusRunning {
		t.Fatalf("other project's job = %+v", j)
	}
}

func TestProgressUpdatesAreThrottledButTheFinalStateIsAlwaysAnnounced(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		defer wg.Done()
		for i := 1; i <= 1000; i++ {
			h.SetFrames(i, 1000)
		}
		return Output{Path: "p", Size: 1}, nil
	})
	sub, unsub := m.Subscribe("")
	defer unsub()
	job, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	wg.Wait()
	done := waitJob(t, m, job.ID, terminal)
	if done.FramesDone != 1000 || done.FramesTotal != 1000 || done.Status != StatusSucceeded {
		t.Fatalf("done = %+v", done)
	}
	if rev := m.Revision(); rev > 40 {
		t.Fatalf("1000 progress ticks produced %d revisions", rev)
	}
	select {
	case <-sub:
	case <-time.After(time.Second):
		t.Fatal("no signal")
	}
}

func TestPayloadReachesTheRunFuncAndIsReleased(t *testing.T) {
	got := make(chan any, 1)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, payload any, h *Handle) (Output, error) {
		got <- payload
		return Output{}, nil
	})
	job, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Payload: "snapshot-1"})
	if err != nil {
		t.Fatal(err)
	}
	if v := <-got; v != "snapshot-1" {
		t.Fatalf("payload = %v", v)
	}
	waitJob(t, m, job.ID, terminal)
	m.mu.Lock()
	left := len(m.payload)
	m.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d payloads kept after the job ended", left)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestJob|TestRestart|TestClose|TestIdempotency|TestCancel|TestProgress|TestPayload" -count=1`

Expected: FAIL (build failed: `undefined: RunFunc`, `undefined: Manager`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/jobs.go`:

```go
// Package render turns a Storymaker story into an MP4: the job manager, the per-job loopback
// listener the headless browser talks to, frame ingest, browser provisioning and the FFmpeg
// frame-input pipeline. The HTTP handlers live in internal/server; nothing here depends on it.
package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"github.com/google/uuid"
)

// Job kinds, statuses and phases (contract section 5).
const (
	KindRender = "render"

	StatusQueued      = "queued"
	StatusRunning     = "running"
	StatusSucceeded   = "succeeded"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
	StatusInterrupted = "interrupted"

	PhaseStarting  = "starting"
	PhaseCompiling = "compiling"
	PhaseMixing    = "mixing"
	PhaseRendering = "rendering"
	PhaseEncoding  = "encoding"
	PhaseWriting   = "writing"
)

// Stable job error codes (spec "Error handling"); interrupted is the code of a job that a restart cut off.
const (
	CodeInvalidStory       = "invalid_story"
	CodeBrowserUnavailable = "browser_unavailable"
	CodeFFmpegUnavailable  = "ffmpeg_unavailable"
	CodeRenderFailed       = "render_failed"
	CodeRenderTimeout      = "render_timeout"
	CodeCancelled          = "cancelled"
	CodeDisabled           = "storymaker_disabled"
	CodeBusy               = "busy"
	CodeCodeError          = "code_error"
	CodeInterrupted        = "interrupted"
)

const (
	// DefaultMaxQueued is the number of waiting render jobs; the next one is answered with ErrBusy.
	DefaultMaxQueued = 16
	// DefaultRetention is the number of finished jobs kept in jobs.json.
	DefaultRetention = 200

	progressNotifyEvery  = 150 * time.Millisecond
	progressPersistEvery = 3 * time.Second
	jobsFileVersion      = 1
)

// ErrBusy means the render queue already holds DefaultMaxQueued waiting jobs.
var ErrBusy = errors.New("storymaker: the render queue is full")

// ErrIdempotencyConflict means an Idempotency-Key was reused for a different request.
var ErrIdempotencyConflict = errors.New("storymaker: idempotency key reused with a different request")

// ErrClosed means the manager has been closed.
var ErrClosed = errors.New("storymaker: the job manager is closed")

// JobError is the error of a failed job and the error type RunFunc returns.
type JobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *JobError) Error() string { return e.Code + ": " + e.Message }

// Fail builds a *JobError.
func Fail(code, format string, args ...any) *JobError {
	return &JobError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Output is the file a successful render wrote: its desktop path and size.
type Output struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Job is the public job document (contract section 5).
type Job struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Progress    float64    `json:"progress"`
	Phase       string     `json:"phase,omitempty"`
	FramesDone  int        `json:"frames_done"`
	FramesTotal int        `json:"frames_total"`
	Output      *Output    `json:"output,omitempty"`
	Error       *JobError  `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Terminal reports whether the job has ended.
func (j Job) Terminal() bool {
	switch j.Status {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusInterrupted:
		return true
	}
	return false
}

// record is a job as the manager keeps and persists it. PrefixHash is the SHA-256 of the render
// prefix of the listener that served the job (never the prefix itself); KeyHash and Fingerprint
// implement Idempotency-Key.
type record struct {
	Job
	PrefixHash  string `json:"prefix_hash,omitempty"`
	KeyHash     string `json:"key_hash,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Rev         uint64 `json:"rev"`

	cancelRequested bool
	notifiedAt      time.Time
	persistedAt     time.Time
}

type diskState struct {
	Version int       `json:"version"`
	Jobs    []*record `json:"jobs"`
}

// RunFunc executes one job. It returns the output on success and a *JobError (or any error, which
// becomes render_failed) on failure. A cancelled ctx means the job was cancelled or the manager is
// closing; the function stops its processes and returns.
type RunFunc func(ctx context.Context, job Job, payload any, h *Handle) (Output, error)

// EnqueueRequest describes a job to queue.
type EnqueueRequest struct {
	ProjectID string
	// Key is the Idempotency-Key header (optional); Fingerprint identifies the request (project id and
	// story etag): the same key with another fingerprint is an ErrIdempotencyConflict.
	Key         string
	Fingerprint string
	// Payload travels to the RunFunc unchanged. It is kept in memory only: a job that was queued
	// when the server stopped is interrupted, so it never needs to be persisted.
	Payload any
}

// ManagerOptions configures NewManager.
type ManagerOptions struct {
	// Dir is the storymaker data folder (<DataDir>/storymaker); jobs.json lives in it and the
	// staging/ folder below it is emptied at start.
	Dir       string
	Run       RunFunc
	MaxQueued int
	Retention int
	Logger    *slog.Logger
	// NewID and Now are for tests.
	NewID func() string
	Now   func() time.Time
}

// Manager runs render jobs one at a time from a bounded queue and persists them in jobs.json.
type Manager struct {
	opts   ManagerOptions
	path   string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	jobs    map[string]*record
	payload map[string]any
	queue   []string
	wake    chan struct{}
	active  string
	cancels map[string]context.CancelFunc
	subs    map[int]*subscriber
	nextSub int
	rev     uint64
	closed  bool
}

type subscriber struct {
	project string // "" = every project
	ch      chan struct{}
}

// NewManager loads jobs.json (jobs that were queued or running become interrupted), empties the
// staging folder and starts the worker.
func NewManager(opts ManagerOptions) (*Manager, error) {
	if opts.Dir == "" || opts.Run == nil {
		return nil, errors.New("storymaker: job manager needs a folder and a run function")
	}
	if opts.MaxQueued <= 0 {
		opts.MaxQueued = DefaultMaxQueued
	}
	if opts.Retention <= 0 {
		opts.Retention = DefaultRetention
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.NewID == nil {
		opts.NewID = uuid.NewString
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return nil, fmt.Errorf("storymaker: create job folder: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		opts: opts, path: filepath.Join(opts.Dir, "jobs.json"), ctx: ctx, cancel: cancel,
		jobs: map[string]*record{}, payload: map[string]any{}, wake: make(chan struct{}, 1), cancels: map[string]context.CancelFunc{}, subs: map[int]*subscriber{},
	}
	if err := m.load(); err != nil {
		cancel()
		return nil, err
	}
	clearStaging(StagingRoot(opts.Dir))
	m.wg.Add(1)
	go m.worker()
	return m, nil
}

// StagingRoot is the folder under dir that holds the per-job temporary files (Chrome profile and
// the MP4 before it is published).
func StagingRoot(dir string) string { return filepath.Join(dir, "staging") }

func clearStaging(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(root, e.Name()))
	}
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storymaker: read jobs.json: %w", err)
	}
	if err == nil {
		var st diskState
		if err := json.Unmarshal(data, &st); err != nil || st.Version != jobsFileVersion {
			// A damaged file must not lock the feature out: keep it aside and start empty.
			m.opts.Logger.Warn("[Storymaker] jobs.json is unreadable and was set aside")
			_ = os.Rename(m.path, m.path+".bad")
		} else {
			for _, r := range st.Jobs {
				if r == nil || !validJobID(r.ID) || r.Kind != KindRender {
					continue
				}
				if r.Status == StatusQueued || r.Status == StatusRunning {
					now := m.opts.Now()
					r.Status = StatusInterrupted
					r.Error = &JobError{Code: CodeInterrupted, Message: "The server restarted before this render finished. Start the export again."}
					r.FinishedAt = &now
					r.UpdatedAt = now
				}
				if r.Rev > m.rev {
					m.rev = r.Rev
				}
				m.jobs[r.ID] = r
			}
		}
	}
	return m.persistLocked()
}

func validJobID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == id
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HashPrefix returns the hash a job stores in place of its render prefix.
func HashPrefix(prefix string) string { return hashHex("storymaker-render-prefix\x00" + prefix) }

func (m *Manager) persistLocked() error {
	st := diskState{Version: jobsFileVersion}
	for _, r := range m.jobs {
		st.Jobs = append(st.Jobs, r)
	}
	sort.Slice(st.Jobs, func(i, j int) bool {
		if !st.Jobs[i].CreatedAt.Equal(st.Jobs[j].CreatedAt) {
			return st.Jobs[i].CreatedAt.Before(st.Jobs[j].CreatedAt)
		}
		return st.Jobs[i].ID < st.Jobs[j].ID
	})
	data, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return fmt.Errorf("storymaker: encode jobs: %w", err)
	}
	if err := writeAtomic(m.path, data, 0o600); err != nil {
		return fmt.Errorf("storymaker: persist jobs: %w", err)
	}
	now := m.opts.Now()
	for _, r := range m.jobs {
		r.persistedAt = now
	}
	return nil
}

// writeAtomic writes data to path through a temp file in the same folder.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if err := tmp.Chmod(perm); err != nil && !errors.Is(err, os.ErrPermission) {
		// Windows ignores most mode bits; other failures are not fatal for a state file.
		_ = err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := fileutil.Rename(name, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// Enqueue queues a render. With an Idempotency-Key that was seen for this project and request it
// returns the earlier job and replayed true. ErrBusy when DefaultMaxQueued jobs are waiting.
func (m *Manager) Enqueue(req EnqueueRequest) (job Job, replayed bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Job{}, false, ErrClosed
	}
	keyHash := ""
	if req.Key != "" {
		keyHash = hashHex(req.ProjectID + "\x00" + req.Key)
		for _, r := range m.jobs {
			if r.KeyHash == keyHash {
				if r.Fingerprint != req.Fingerprint {
					return Job{}, false, ErrIdempotencyConflict
				}
				return r.Job, true, nil
			}
		}
	}
	if len(m.queue) >= m.opts.MaxQueued {
		return Job{}, false, ErrBusy
	}
	now := m.opts.Now()
	r := &record{
		Job: Job{
			ID: m.opts.NewID(), ProjectID: req.ProjectID, Kind: KindRender, Status: StatusQueued,
			CreatedAt: now, UpdatedAt: now,
		},
		KeyHash: keyHash, Fingerprint: req.Fingerprint,
	}
	m.jobs[r.ID] = r
	m.payload[r.ID] = req.Payload
	m.queue = append(m.queue, r.ID)
	m.touchLocked(r)
	m.pruneLocked()
	if err := m.persistLocked(); err != nil {
		delete(m.jobs, r.ID)
		delete(m.payload, r.ID)
		m.queue = m.queue[:len(m.queue)-1]
		return Job{}, false, err
	}
	m.notifyLocked(r)
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return r.Job, false, nil
}

// touchLocked stamps UpdatedAt and the revision of r.
func (m *Manager) touchLocked(r *record) {
	m.rev++
	r.Rev = m.rev
	r.UpdatedAt = m.opts.Now()
}

func (m *Manager) pruneLocked() {
	var finished []*record
	for _, r := range m.jobs {
		if r.Terminal() {
			finished = append(finished, r)
		}
	}
	if len(finished) <= m.opts.Retention {
		return
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].CreatedAt.Before(finished[j].CreatedAt) })
	for _, r := range finished[:len(finished)-m.opts.Retention] {
		delete(m.jobs, r.ID)
	}
}

func (m *Manager) notifyLocked(r *record) {
	r.notifiedAt = m.opts.Now()
	for _, s := range m.subs {
		if s.project != "" && s.project != r.ProjectID {
			continue
		}
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
}

// Get returns a copy of the job.
func (m *Manager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return r.Job, true
}

// List returns the jobs of a project (all jobs for ""), newest first.
func (m *Manager) List(projectID string) []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Job
	for _, r := range m.jobs {
		if projectID == "" || r.ProjectID == projectID {
			out = append(out, r.Job)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

// PrefixHash returns the stored hash of the render prefix of a job ("" until the listener started).
func (m *Manager) PrefixHash(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.jobs[id]; ok {
		return r.PrefixHash
	}
	return ""
}

// Subscribe returns a channel that receives a signal after every job change of the project ("" for
// all projects) and a function that ends the subscription. Signals coalesce; call ChangedSince for
// the jobs themselves.
func (m *Manager) Subscribe(projectID string) (<-chan struct{}, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextSub
	m.nextSub++
	s := &subscriber{project: projectID, ch: make(chan struct{}, 1)}
	m.subs[id] = s
	return s.ch, func() {
		m.mu.Lock()
		delete(m.subs, id)
		m.mu.Unlock()
	}
}

// Revision returns the current change counter.
func (m *Manager) Revision() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rev
}

// ChangedSince returns the jobs of the project that changed after revision rev and the highest
// revision among them (rev itself when nothing changed).
func (m *Manager) ChangedSince(projectID string, rev uint64) ([]Job, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var changed []*record
	for _, r := range m.jobs {
		if (projectID == "" || r.ProjectID == projectID) && r.Rev > rev {
			changed = append(changed, r)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Rev < changed[j].Rev })
	out := make([]Job, 0, len(changed))
	for _, r := range changed {
		out = append(out, r.Job)
		if r.Rev > rev {
			rev = r.Rev
		}
	}
	return out, rev
}

// Cancel cancels a queued or running job and returns its state. A finished job is returned as is.
func (m *Manager) Cancel(id string) (Job, bool) {
	m.mu.Lock()
	r, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return Job{}, false
	}
	switch r.Status {
	case StatusQueued:
		m.removeFromQueueLocked(id)
		delete(m.payload, id)
		m.finishLocked(r, StatusCancelled, &JobError{Code: CodeCancelled, Message: "The export was cancelled."}, nil)
		_ = m.persistLocked()
	case StatusRunning:
		r.cancelRequested = true
		if cancel := m.cancels[id]; cancel != nil {
			cancel()
		}
	}
	job := r.Job
	m.mu.Unlock()
	return job, true
}

// CancelProject cancels every unfinished job of a project (a deleted project).
func (m *Manager) CancelProject(projectID string) {
	m.mu.Lock()
	var ids []string
	for id, r := range m.jobs {
		if r.ProjectID == projectID && !r.Terminal() {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Cancel(id)
	}
}

func (m *Manager) removeFromQueueLocked(id string) {
	for i, q := range m.queue {
		if q == id {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

func (m *Manager) finishLocked(r *record, status string, jobErr *JobError, out *Output) {
	now := m.opts.Now()
	r.Status = status
	r.Error = jobErr
	r.Output = out
	r.FinishedAt = &now
	if status == StatusSucceeded {
		r.Progress = 1
	}
	m.touchLocked(r)
	m.notifyLocked(r)
}

// Close cancels the running job (it ends as interrupted), marks the queued ones interrupted and
// waits for the worker.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.cancel()
	for _, id := range m.queue {
		if r := m.jobs[id]; r != nil {
			m.finishLocked(r, StatusInterrupted, &JobError{Code: CodeInterrupted, Message: "The server stopped before this render started."}, nil)
		}
	}
	m.queue = nil
	m.payload = map[string]any{}
	_ = m.persistLocked()
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return
		}
		var id string
		if len(m.queue) > 0 {
			id = m.queue[0]
			m.queue = m.queue[1:]
		}
		m.mu.Unlock()
		if id == "" {
			select {
			case <-m.ctx.Done():
				return
			case <-m.wake:
			}
			continue
		}
		m.runJob(id)
	}
}

func (m *Manager) runJob(id string) {
	m.mu.Lock()
	r := m.jobs[id]
	if r == nil || r.Status != StatusQueued {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancels[id] = cancel
	m.active = id
	now := m.opts.Now()
	r.Status = StatusRunning
	r.StartedAt = &now
	r.Phase = PhaseStarting
	m.touchLocked(r)
	_ = m.persistLocked()
	m.notifyLocked(r)
	job := r.Job
	payload := m.payload[id]
	m.mu.Unlock()

	out, err := m.safeRun(ctx, job, payload, &Handle{m: m, id: id})
	cancel()

	m.mu.Lock()
	delete(m.cancels, id)
	delete(m.payload, id)
	m.active = ""
	switch {
	case err == nil:
		o := out
		m.finishLocked(r, StatusSucceeded, nil, &o)
	case r.cancelRequested && !m.closed:
		m.finishLocked(r, StatusCancelled, &JobError{Code: CodeCancelled, Message: "The export was cancelled."}, nil)
	case m.closed && ctx.Err() != nil:
		m.finishLocked(r, StatusInterrupted, &JobError{Code: CodeInterrupted, Message: "The server stopped before this render finished. Start the export again."}, nil)
	default:
		var je *JobError
		if !errors.As(err, &je) {
			je = &JobError{Code: CodeRenderFailed, Message: clip(err.Error(), 400)}
		}
		m.finishLocked(r, StatusFailed, je, nil)
	}
	m.pruneLocked()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *Manager) safeRun(ctx context.Context, job Job, payload any, h *Handle) (out Output, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			m.opts.Logger.Error("[Storymaker] Recovered from a panic in a render job", "panic", rec)
			err = &JobError{Code: CodeRenderFailed, Message: "The render stopped unexpectedly."}
		}
	}()
	return m.opts.Run(ctx, job, payload, h)
}

// Handle lets a running RunFunc report its progress.
type Handle struct {
	m  *Manager
	id string
}

func (h *Handle) update(fn func(r *record), state bool) {
	m := h.m
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.jobs[h.id]
	if r == nil || r.Status != StatusRunning {
		return
	}
	fn(r)
	now := m.opts.Now()
	if !state && now.Sub(r.notifiedAt) < progressNotifyEvery {
		return
	}
	m.touchLocked(r)
	m.notifyLocked(r)
	if state || now.Sub(r.persistedAt) >= progressPersistEvery {
		_ = m.persistLocked()
	}
}

// SetPhase records the phase (a state change: it is persisted and announced at once).
func (h *Handle) SetPhase(phase string) {
	h.update(func(r *record) { r.Phase = phase }, true)
}

// SetFrames records the frame counters; progress is frames done over frames total, scaled into
// the rendering part of the job (0.05 .. 0.95), the rest is start-up and publishing.
func (h *Handle) SetFrames(done, total int) {
	h.update(func(r *record) {
		r.FramesDone, r.FramesTotal = done, total
		if total > 0 {
			r.Progress = 0.05 + 0.90*float64(done)/float64(total)
		}
	}, false)
}

// SetPrefixHash stores the hash of the render prefix of this job's listener.
func (h *Handle) SetPrefixHash(hash string) {
	h.update(func(r *record) { r.PrefixHash = hash }, true)
}

// SetProgress sets the progress directly (0..1).
func (h *Handle) SetProgress(p float64) {
	h.update(func(r *record) {
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		r.Progress = p
	}, false)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/jobs.go internal/storymaker/render/jobs_test.go
git commit -m "feat(storymaker): render job manager with persistence, queue cap and change events" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B4: Browser provisioning

**Files:**
- Create: `internal/storymaker/render/browser.go`
- Test: `internal/storymaker/render/browser_test.go`

Resolution order: `storymaker.render.browser_path` (a broken configured browser is reported, not replaced), `launcher.LookPath()`, then the pinned Chromium snapshot (rod's default revision 1321438, Chromium 128.0.6568.0). The SHA-256 and size of the four platform archives were computed once from `storage.googleapis.com/chromium-browser-snapshots` (md5 equals the host's ETag) and are recorded in `pinnedChromium`; there is no mirror race. A status request never downloads: it reports `can_download`. The first export downloads to a `.part` file, verifies size and SHA-256, unpacks with zip-slip, symlink and size protections, writes a verified marker and moves the folder into place. `ready` is true only after a cached headless launch probe; `newLauncher` removes the `--no-sandbox` flag rod adds inside containers (the only way to set it is `SetNoSandboxForTests`). `TestPinnedArchivesExtract` (gated by `AURAGO_SM_PINNED_DIR`) checks the real archives against the table.

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/browser_test.go`:

```go
package render

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func exeName() string {
	if runtime.GOOS == "windows" {
		return "chrome.exe"
	}
	return "chrome"
}

func fakeBrowserFile(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, exeName())
	if err := os.WriteFile(p, []byte("not a real browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestBrowsers(t *testing.T) (*Browsers, *atomic.Int32) {
	t.Helper()
	b := NewBrowsers(t.TempDir())
	probes := &atomic.Int32{}
	b.launchProbe = func(ctx context.Context, bin string) (string, error) {
		probes.Add(1)
		if strings.Contains(filepath.Base(bin), "broken") {
			return "", errors.New("exec format error")
		}
		return "FakeChrome/1.0", nil
	}
	b.lookPath = func() (string, bool) { return "", false }
	b.pins = map[string]PinnedChromium{}
	return b, probes
}

func TestBrowserResolutionOrder(t *testing.T) {
	b, _ := newTestBrowsers(t)
	configured := fakeBrowserFile(t, t.TempDir())
	system := fakeBrowserFile(t, t.TempDir())
	b.lookPath = func() (string, bool) { return system, true }

	st, bin := b.Ensure(context.Background(), configured, nil)
	if !st.Ready || st.Source != SourceConfig || bin != configured || st.Version != "FakeChrome/1.0" {
		t.Fatalf("configured: %+v %q", st, bin)
	}
	st, bin = b.Ensure(context.Background(), "", nil)
	if !st.Ready || st.Source != SourceSystem || bin != system {
		t.Fatalf("system: %+v %q", st, bin)
	}
	// A configured path that does not exist is reported, not replaced by the system browser.
	st, bin = b.Ensure(context.Background(), filepath.Join(t.TempDir(), "nope"), nil)
	if st.Ready || st.Source != SourceConfig || bin != "" || !strings.Contains(st.Issue, "browser_path") {
		t.Fatalf("missing configured: %+v %q", st, bin)
	}
	// So is a configured browser that does not start.
	broken := filepath.Join(t.TempDir(), "broken-chrome")
	if err := os.WriteFile(broken, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	st, _ = b.Ensure(context.Background(), broken, nil)
	if st.Ready || st.Source != SourceConfig || !strings.Contains(st.Issue, "exec format error") {
		t.Fatalf("broken configured: %+v", st)
	}
}

func TestBrowserMissingAndSystemProbeFailure(t *testing.T) {
	b, _ := newTestBrowsers(t)
	st := b.Status(context.Background(), "")
	if st.Ready || st.Source != SourceMissing || st.Issue == "" || st.CanDownload {
		t.Fatalf("nothing available (no pin): %+v", st)
	}
	broken := filepath.Join(t.TempDir(), "broken-chrome")
	if err := os.WriteFile(broken, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	b.lookPath = func() (string, bool) { return broken, true }
	st = b.Status(context.Background(), "")
	if st.Ready || st.Source != SourceSystem || !strings.Contains(st.Issue, "exec format error") {
		t.Fatalf("a system browser that does not start: %+v", st)
	}
}

func TestBrowserProbeIsCached(t *testing.T) {
	b, probes := newTestBrowsers(t)
	system := fakeBrowserFile(t, t.TempDir())
	b.lookPath = func() (string, bool) { return system, true }
	for i := 0; i < 5; i++ {
		if st := b.Status(context.Background(), ""); !st.Ready {
			t.Fatalf("status %d: %+v", i, st)
		}
	}
	if probes.Load() != 1 {
		t.Fatalf("%d probes for 5 status requests", probes.Load())
	}
	b.ForgetProbes()
	b.Status(context.Background(), "")
	if probes.Load() != 2 {
		t.Fatalf("%d probes after ForgetProbes", probes.Load())
	}
}

// fakeChromiumZip builds a snapshot-shaped archive: chrome-x/<exe>, a data file and a folder.
func fakeChromiumZip(t *testing.T, topDir string, extra func(w *zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add := func(name string, body string, mode os.FileMode) {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(body))
	}
	add(topDir+"/", "", os.ModeDir|0o755)
	add(topDir+"/"+exeName(), "MZ fake browser", 0o755)
	add(topDir+"/resources/data.pak", "data", 0o644)
	if extra != nil {
		extra(w)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pinFor(url string, archive []byte) PinnedChromium {
	sum := sha256.Sum256(archive)
	return PinnedChromium{URL: url, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(archive)), Dir: "chrome-x", Exe: exeName()}
}

func serveArchive(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPinnedChromiumDownloadVerifiesAndInstalls(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := serveArchive(t, archive)
	b, probes := newTestBrowsers(t)
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}

	// A status request never downloads.
	st := b.Status(context.Background(), "")
	if st.Ready || st.Source != SourcePinned || !st.CanDownload || !strings.Contains(st.Issue, "first export") {
		t.Fatalf("status before the download: %+v", st)
	}
	if _, err := os.Stat(b.PinnedDir()); !os.IsNotExist(err) {
		t.Fatal("a status request created the install folder")
	}

	var last int64
	st, bin := b.Ensure(context.Background(), "", func(done, total int64) { last = done })
	if !st.Ready || st.Source != SourcePinned || bin == "" {
		t.Fatalf("Ensure: %+v %q", st, bin)
	}
	if last != int64(len(archive)) {
		t.Fatalf("progress ended at %d of %d", last, len(archive))
	}
	if _, err := os.Stat(filepath.Join(b.PinnedDir(), verifiedMarker)); err != nil {
		t.Fatalf("no verified marker: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.PinnedDir(), "resources", "data.pak")); string(got) != "data" {
		t.Fatalf("data file = %q", got)
	}
	leftovers, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*.part"))
	tmp, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*.tmp-*"))
	if len(leftovers)+len(tmp) != 0 {
		t.Fatalf("temporary files left: %v %v", leftovers, tmp)
	}
	// Second time: installed, no new download, status is ready without Ensure.
	srv.Close()
	b.ForgetProbes()
	st = b.Status(context.Background(), "")
	if !st.Ready || st.Source != SourcePinned {
		t.Fatalf("status after the install: %+v", st)
	}
	_ = probes
}

func TestPinnedChromiumRefusesWrongChecksumSizeAndStatus(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := serveArchive(t, archive)
	cases := map[string]PinnedChromium{}
	badHash := pinFor(srv.URL, archive)
	badHash.SHA256 = strings.Repeat("0", 64)
	cases["checksum"] = badHash
	badSize := pinFor(srv.URL, archive)
	badSize.Size++
	cases["size"] = badSize
	notFound := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notFound.Close)
	cases["status"] = pinFor(notFound.URL, archive)
	for name, pin := range cases {
		b, _ := newTestBrowsers(t)
		b.pins = map[string]PinnedChromium{b.platform: pin}
		st, bin := b.Ensure(context.Background(), "", nil)
		if st.Ready || bin != "" || !strings.Contains(st.Issue, "could not be downloaded") {
			t.Errorf("%s: %+v %q", name, st, bin)
		}
		if _, err := os.Stat(b.PinnedDir()); !os.IsNotExist(err) {
			t.Errorf("%s: an unverified archive was installed", name)
		}
		if parts, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*")); len(parts) != 0 {
			t.Errorf("%s: left %v", name, parts)
		}
	}
}

func TestExtractChromiumRefusesEscapesAndForeignEntries(t *testing.T) {
	cases := map[string]func(w *zip.Writer){
		"dot-dot": func(w *zip.Writer) { f, _ := w.Create("chrome-x/../evil.txt"); _, _ = f.Write([]byte("x")) },
		"outside": func(w *zip.Writer) { f, _ := w.Create("other/evil.txt"); _, _ = f.Write([]byte("x")) },
		"abs":     func(w *zip.Writer) { f, _ := w.Create("chrome-x//etc/evil"); _, _ = f.Write([]byte("x")) },
		"backslash": func(w *zip.Writer) {
			f, _ := w.Create("chrome-x/a\\..\\..\\evil")
			_, _ = f.Write([]byte("x"))
		},
		"symlink-escape": func(w *zip.Writer) {
			h := &zip.FileHeader{Name: "chrome-x/link"}
			h.SetMode(os.ModeSymlink | 0o777)
			f, _ := w.CreateHeader(h)
			_, _ = f.Write([]byte("../../outside"))
		},
		"symlink-abs": func(w *zip.Writer) {
			h := &zip.FileHeader{Name: "chrome-x/link"}
			h.SetMode(os.ModeSymlink | 0o777)
			f, _ := w.CreateHeader(h)
			_, _ = f.Write([]byte("/etc/passwd"))
		},
	}
	for name, extra := range cases {
		dir := t.TempDir()
		zipPath := filepath.Join(dir, "a.zip")
		if err := os.WriteFile(zipPath, fakeChromiumZip(t, "chrome-x", extra), 0o600); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(dir, "out")
		if err := extractChromium(zipPath, dest, "chrome-x"); err == nil {
			t.Errorf("%s: the archive was extracted", name)
		}
		if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
			t.Errorf("%s: a file escaped the folder", name)
		}
	}
}

func TestExtractChromiumKeepsRelativeSymlinksInsideTheFolder(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	data := fakeChromiumZip(t, "chrome-x", func(w *zip.Writer) {
		h := &zip.FileHeader{Name: "chrome-x/resources/current"}
		h.SetMode(os.ModeSymlink | 0o777)
		f, _ := w.CreateHeader(h)
		_, _ = f.Write([]byte("data.pak"))
	})
	if err := os.WriteFile(zipPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	err := extractChromium(zipPath, dest, "chrome-x")
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks need a privilege on this Windows host: %v", err)
		}
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(dest, "resources", "current")); err != nil || target != "data.pak" {
		t.Fatalf("symlink = %q, %v", target, err)
	}
}

func TestPinnedTableCoversTheSupportedPlatforms(t *testing.T) {
	for _, platform := range []string{"windows_amd64", "linux_amd64", "darwin_amd64", "darwin_arm64"} {
		pin, ok := pinnedChromium[platform]
		if !ok {
			t.Errorf("no pin for %s", platform)
			continue
		}
		if len(pin.SHA256) != 64 || pin.Size < 100<<20 || !strings.HasPrefix(pin.URL, "https://storage.googleapis.com/chromium-browser-snapshots/") ||
			!strings.Contains(pin.URL, "/1321438/") || pin.Dir == "" || pin.Exe == "" {
			t.Errorf("%s: incomplete pin %+v", platform, pin)
		}
	}
}

// TestPinnedArchivesExtract checks the four real snapshot archives against the pinned table: SHA-256,
// size and a clean extraction (it does not start the browser). It runs when AURAGO_SM_PINNED_DIR names
// a folder holding windows_amd64.zip, linux_amd64.zip, darwin_amd64.zip and darwin_arm64.zip (the
// downloads of the pinned revision); symlink-bearing macOS archives are only extracted on hosts that
// may create symlinks.
func TestPinnedArchivesExtract(t *testing.T) {
	dir := os.Getenv("AURAGO_SM_PINNED_DIR")
	if dir == "" {
		t.Skip("AURAGO_SM_PINNED_DIR is not set")
	}
	for platform, pin := range pinnedChromium {
		t.Run(platform, func(t *testing.T) {
			archive := filepath.Join(dir, platform+".zip")
			data, err := os.ReadFile(archive)
			if err != nil {
				t.Skipf("no archive for %s: %v", platform, err)
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != pin.SHA256 || int64(len(data)) != pin.Size {
				t.Fatalf("%s does not match the pinned hash or size", archive)
			}
			if strings.HasPrefix(platform, "darwin") && runtime.GOOS == "windows" {
				t.Skip("macOS archives carry symlinks that Windows may not create")
			}
			dest := filepath.Join(t.TempDir(), "out")
			if err := extractChromium(archive, dest, pin.Dir); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(pin.Exe))); err != nil || !info.Mode().IsRegular() {
				t.Fatalf("executable %s missing after extraction: %v", pin.Exe, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestBrowser|TestPinned|TestExtract" -count=1`

Expected: FAIL (build failed: `undefined: Browsers`, `undefined: NewBrowsers`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/browser.go`:

```go
package render

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Browser sources reported by the status endpoint.
const (
	SourceConfig  = "config"
	SourceSystem  = "system"
	SourcePinned  = "pinned"
	SourceMissing = "missing"
)

// PinnedRevision is the Chromium snapshot revision AuraGo downloads when no usable browser is
// installed: the revision rod v0.116.2 ships as its default (Chromium 128.0.6568.0). The archives
// below were downloaded once from storage.googleapis.com/chromium-browser-snapshots and their
// SHA-256 was computed; a download is verified against it before anything is unpacked or run.
const PinnedRevision = 1321438

// PinnedChromium describes one platform's snapshot archive.
type PinnedChromium struct {
	URL    string
	SHA256 string
	Size   int64
	// Dir is the single top-level folder inside the archive; it is stripped on extraction.
	Dir string
	// Exe is the executable below the install folder.
	Exe string
}

func snapshotURL(prefix, zipName string) string {
	return fmt.Sprintf("https://storage.googleapis.com/chromium-browser-snapshots/%s/%d/%s", prefix, PinnedRevision, zipName)
}

// pinnedChromium is keyed by GOOS_GOARCH. There is no mirror list: one host, one checksum.
var pinnedChromium = map[string]PinnedChromium{
	"windows_amd64": {URL: snapshotURL("Win_x64", "chrome-win.zip"), SHA256: "f8ddee43d6865a974fc8acfcfcb9f6a42e84aadc427482ae2fa2e6a36a413454", Size: 267483258, Dir: "chrome-win", Exe: "chrome.exe"},
	"linux_amd64":   {URL: snapshotURL("Linux_x64", "chrome-linux.zip"), SHA256: "b5fb87b55fd81eaba9325d67220585cc12c12df529bd8ae46906d43dd5699e4e", Size: 190157449, Dir: "chrome-linux", Exe: "chrome"},
	"darwin_amd64":  {URL: snapshotURL("Mac", "chrome-mac.zip"), SHA256: "b25cd79dc73ebf423b05a4a0d65f772a78e3b87d394e9a2e7d0a4cc9201bda7b", Size: 149328540, Dir: "chrome-mac", Exe: "Chromium.app/Contents/MacOS/Chromium"},
	"darwin_arm64":  {URL: snapshotURL("Mac_Arm", "chrome-mac.zip"), SHA256: "4a478b542f1f6eaa160e64aab76100501e061d3bba46f6a480897ef1103c70c5", Size: 135605548, Dir: "chrome-mac", Exe: "Chromium.app/Contents/MacOS/Chromium"},
}

const (
	verifiedMarker      = ".storymaker-verified"
	maxExtractFiles     = 5000
	maxExtractBytes     = 2 << 30
	probeOKCacheFor     = 10 * time.Minute
	probeFailCacheFor   = 30 * time.Second
	browserProbeTimeout = 45 * time.Second
)

// BrowserStatus is the browser part of GET /status.
type BrowserStatus struct {
	// Ready is true only after a successful headless launch probe of the browser that Source names.
	Ready  bool   `json:"ready"`
	Source string `json:"source"` // config | system | pinned | missing
	Issue  string `json:"issue,omitempty"`
	// CanDownload tells that no browser is ready but the pinned Chromium can be downloaded by the
	// first export (the download is never started by a status request).
	CanDownload bool   `json:"can_download,omitempty"`
	Version     string `json:"version,omitempty"`
}

// BrowserProgress receives download progress (bytes done, bytes total).
type BrowserProgress func(done, total int64)

// Browsers resolves, provisions and probes the render browser.
type Browsers struct {
	dir string // <DataDir>/storymaker

	// Seams for tests.
	lookPath    func() (string, bool)
	launchProbe func(ctx context.Context, bin string) (string, error)
	client      *http.Client
	pins        map[string]PinnedChromium
	platform    string

	probeMu sync.Mutex
	probes  map[string]probeResult
	dlMu    sync.Mutex
}

type probeResult struct {
	version string
	err     error
	at      time.Time
}

// NewBrowsers returns a resolver that keeps the pinned Chromium below dir.
func NewBrowsers(dir string) *Browsers {
	return &Browsers{
		dir: dir, lookPath: launcher.LookPath, launchProbe: probeLaunch,
		client: &http.Client{Timeout: 0}, pins: pinnedChromium, platform: runtime.GOOS + "_" + runtime.GOARCH,
		probes: map[string]probeResult{},
	}
}

func (b *Browsers) pin() (PinnedChromium, bool) {
	p, ok := b.pins[b.platform]
	return p, ok
}

// PinnedDir is the install folder of the pinned Chromium.
func (b *Browsers) PinnedDir() string {
	return filepath.Join(b.dir, fmt.Sprintf("chromium-%d", PinnedRevision))
}

func (b *Browsers) pinnedInstalled() (string, bool) {
	pin, ok := b.pin()
	if !ok {
		return "", false
	}
	marker, err := os.ReadFile(filepath.Join(b.PinnedDir(), verifiedMarker))
	if err != nil || strings.TrimSpace(string(marker)) != pin.SHA256 {
		return "", false
	}
	exe := filepath.Join(b.PinnedDir(), filepath.FromSlash(pin.Exe))
	if info, err := os.Stat(exe); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return exe, true
}

// NewBrowsersWithProbe is NewBrowsers with the system browser lookup and the launch probe replaced
// (tests of the layers above this package use it to avoid starting a browser).
func NewBrowsersWithProbe(dir string, lookPath func() (string, bool), probe func(ctx context.Context, bin string) (string, error)) *Browsers {
	b := NewBrowsers(dir)
	b.lookPath, b.launchProbe = lookPath, probe
	b.pins = map[string]PinnedChromium{}
	return b
}

// Status reports the browser without downloading anything.
func (b *Browsers) Status(ctx context.Context, configuredPath string) BrowserStatus {
	st, _ := b.resolve(ctx, configuredPath, false, nil)
	return st
}

// Ensure resolves the browser like Status and, when only the pinned Chromium is left, downloads and
// verifies it first. It returns the executable to launch.
func (b *Browsers) Ensure(ctx context.Context, configuredPath string, progress BrowserProgress) (BrowserStatus, string) {
	return b.resolve(ctx, configuredPath, true, progress)
}

func (b *Browsers) resolve(ctx context.Context, configuredPath string, download bool, progress BrowserProgress) (BrowserStatus, string) {
	// 1. An operator-configured browser is the only candidate: a broken one is reported, not replaced.
	if p := strings.TrimSpace(configuredPath); p != "" {
		st := BrowserStatus{Source: SourceConfig}
		if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
			st.Issue = "The configured browser path (storymaker.render.browser_path) does not point to an executable file."
			return st, ""
		}
		if ver, err := b.probe(ctx, p); err != nil {
			st.Issue = launchIssue(err)
			return st, ""
		} else {
			st.Ready, st.Version = true, ver
		}
		return st, p
	}

	// 2. A browser the system provides.
	systemIssue := ""
	if found, ok := b.lookPath(); ok {
		if ver, err := b.probe(ctx, found); err == nil {
			return BrowserStatus{Ready: true, Source: SourceSystem, Version: ver}, found
		} else {
			systemIssue = launchIssue(err)
		}
	}

	// 3. The pinned Chromium.
	pin, hasPin := b.pin()
	if !hasPin {
		issue := "No Chrome or Chromium was found. Install one, or set storymaker.render.browser_path."
		if systemIssue != "" {
			return BrowserStatus{Source: SourceSystem, Issue: systemIssue}, ""
		}
		return BrowserStatus{Source: SourceMissing, Issue: issue}, ""
	}
	exe, installed := b.pinnedInstalled()
	if !installed && download {
		var err error
		if exe, err = b.install(ctx, pin, progress); err != nil {
			st := BrowserStatus{Source: SourcePinned, CanDownload: true, Issue: "The pinned Chromium could not be downloaded: " + clip(err.Error(), 300)}
			return st, ""
		}
		installed = true
	}
	if !installed {
		issue := fmt.Sprintf("No usable Chrome or Chromium was found. The first export downloads a pinned Chromium (about %d MB) into the data folder.", pin.Size>>20)
		if systemIssue != "" {
			issue = systemIssue + " " + issue
		}
		return BrowserStatus{Source: SourcePinned, CanDownload: true, Issue: issue}, ""
	}
	ver, err := b.probe(ctx, exe)
	if err != nil {
		return BrowserStatus{Source: SourcePinned, Issue: launchIssue(err)}, ""
	}
	return BrowserStatus{Ready: true, Source: SourcePinned, Version: ver}, exe
}

// launchIssue turns a launch error into the text the status shows: the cause, and for the sandbox
// case the way out (the sandbox is never turned off).
func launchIssue(err error) string {
	text := clip(strings.TrimSpace(err.Error()), 400)
	low := strings.ToLower(text)
	switch {
	case strings.Contains(low, "no usable sandbox"), strings.Contains(low, "sandbox"):
		return "The browser could not start with its sandbox: " + text + ". On Ubuntu 23.10 or later install the AppArmor profile from the installation guide, or set storymaker.render.browser_path to a Chrome that can run here."
	case strings.Contains(low, "error while loading shared libraries"), strings.Contains(low, "shared libraries"):
		return "The browser is missing system libraries: " + text + ". In a container, install Chromium's libraries or set storymaker.render.browser_path."
	}
	return "The browser could not be started: " + text
}

// probe runs (or reuses) the headless launch probe of one executable.
func (b *Browsers) probe(ctx context.Context, bin string) (string, error) {
	key := bin
	if info, err := os.Stat(bin); err == nil {
		key = fmt.Sprintf("%s|%d|%d", bin, info.Size(), info.ModTime().UnixNano())
	}
	b.probeMu.Lock()
	defer b.probeMu.Unlock()
	if r, ok := b.probes[key]; ok {
		ttl := probeOKCacheFor
		if r.err != nil {
			ttl = probeFailCacheFor
		}
		if time.Since(r.at) < ttl {
			return r.version, r.err
		}
	}
	version, err := b.launchProbe(ctx, bin)
	if ctx.Err() != nil {
		return version, ctx.Err() // an aborted probe says nothing about the browser
	}
	b.probes[key] = probeResult{version: version, err: err, at: time.Now()}
	return version, err
}

// ForgetProbes drops the cached probe results (the next status request launches again).
func (b *Browsers) ForgetProbes() {
	b.probeMu.Lock()
	b.probes = map[string]probeResult{}
	b.probeMu.Unlock()
}

// noSandboxForTests lets the tests of a host that cannot run Chrome's sandbox launch it anyway.
// It is a variable of this package only: no configuration or environment value reaches it.
var noSandboxForTests bool

// SetNoSandboxForTests is the test hook for the variable above. Production code never calls it.
func SetNoSandboxForTests(on bool) { noSandboxForTests = on }

// launchOptions describes one Chrome launch.
type launchOptions struct {
	Bin         string
	UserDataDir string
	// ProxyAddr is the strict capture proxy (host:port); empty launches without a proxy (the probe).
	ProxyAddr  string
	WindowSize [2]int
}

// newLauncher builds the rod launcher every Storymaker browser launch uses. It never sets
// NoSandbox (rod adds --no-sandbox by itself inside containers; that is removed here), except
// through the test hook.
func newLauncher(ctx context.Context, o launchOptions) *launcher.Launcher {
	l := launcher.New().Context(ctx).Bin(o.Bin).Headless(true)
	l.Delete("no-sandbox")
	if noSandboxForTests {
		l.Set("no-sandbox")
	}
	l.UserDataDir(o.UserDataDir)
	l.Set("disable-gpu")
	l.Set("hide-scrollbars")
	l.Set("mute-audio")
	l.Set("font-render-hinting", "none")
	l.Set("no-default-browser-check")
	l.Set("disable-extensions")
	l.Set("disable-component-update")
	l.Set("no-pings")
	l.Set("disable-domain-reliability")
	// rod's list keeps the site-per-process switch off; add what a render page never needs.
	l.Set("disable-features", "site-per-process", "TranslateUI", "OptimizationHints", "MediaRouter", "InterestFeedContentSuggestions", "DialMediaRouteProvider")
	if o.WindowSize[0] > 0 && o.WindowSize[1] > 0 {
		l.Set("window-size", fmt.Sprintf("%d,%d", o.WindowSize[0], o.WindowSize[1]))
	}
	if o.ProxyAddr != "" {
		// Every byte goes through the strict proxy, loopback included; STUN/WebRTC leaks are closed
		// (Chrome 156 ignores the force- spelling for STUN, so both are set).
		l.Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp")
		l.Set("webrtc-ip-handling-policy", "disable_non_proxied_udp")
		l.Proxy(o.ProxyAddr)
		l.Set("proxy-bypass-list", "<-loopback>")
	}
	return l
}

// probeLaunch starts the browser headless, opens about:blank, evaluates a trivial script and
// closes it again. It returns the product version.
func probeLaunch(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, browserProbeTimeout)
	defer cancel()
	profile, err := os.MkdirTemp("", "aurago-sm-probe-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(profile)
	l := newLauncher(ctx, launchOptions{Bin: bin, UserDataDir: profile})
	controlURL, err := l.Launch()
	if err != nil {
		return "", err
	}
	defer func() { l.Kill(); l.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return "", err
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = browser.Context(closeCtx).Close()
	}()
	version := ""
	if v, err := (proto.BrowserGetVersion{}).Call(browser); err == nil {
		version = v.Product
	}
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return version, err
	}
	res, err := page.Eval(`() => 6 * 7`)
	if err != nil {
		return version, err
	}
	if res.Value.Int() != 42 {
		return version, errors.New("the browser did not evaluate a script")
	}
	return version, nil
}

// install downloads the pinned archive, verifies its SHA-256, unpacks it next to the final folder
// and moves it into place. A verified install carries a marker with the archive hash.
func (b *Browsers) install(ctx context.Context, pin PinnedChromium, progress BrowserProgress) (string, error) {
	b.dlMu.Lock()
	defer b.dlMu.Unlock()
	if exe, ok := b.pinnedInstalled(); ok {
		return exe, nil // another job finished the install while this one waited
	}
	if err := os.MkdirAll(b.dir, 0o700); err != nil {
		return "", err
	}
	if free, err := fileutil.FreeDiskBytes(b.dir); err == nil && free < pin.Size*4 {
		return "", fmt.Errorf("not enough free disk space for the browser download (%d MB needed)", pin.Size*4>>20)
	}
	archive := filepath.Join(b.dir, fmt.Sprintf("chromium-%d.zip.part", PinnedRevision))
	defer os.Remove(archive)
	if err := b.download(ctx, pin, archive, progress); err != nil {
		return "", err
	}
	stage := filepath.Join(b.dir, fmt.Sprintf("chromium-%d.tmp-%s", PinnedRevision, rand.Text()))
	defer os.RemoveAll(stage)
	if err := extractChromium(archive, stage, pin.Dir); err != nil {
		return "", err
	}
	exe := filepath.Join(stage, filepath.FromSlash(pin.Exe))
	if info, err := os.Stat(exe); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("the archive does not contain the browser executable")
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(exe, 0o755)
	}
	if err := os.WriteFile(filepath.Join(stage, verifiedMarker), []byte(pin.SHA256+"\n"), 0o600); err != nil {
		return "", err
	}
	final := b.PinnedDir()
	_ = os.RemoveAll(final)
	if err := fileutil.Rename(stage, final); err != nil {
		return "", fmt.Errorf("move the browser into place: %w", err)
	}
	return filepath.Join(final, filepath.FromSlash(pin.Exe)), nil
}

func (b *Browsers) download(ctx context.Context, pin PinnedChromium, dest string, progress BrowserProgress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pin.URL, nil)
	if err != nil {
		return err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download: the server answered %d", res.StatusCode)
	}
	if res.ContentLength > 0 && res.ContentLength != pin.Size {
		return fmt.Errorf("download: the archive is %d bytes, the pinned size is %d", res.ContentLength, pin.Size)
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	counter := &countingWriter{progress: progress, total: pin.Size}
	n, copyErr := io.Copy(io.MultiWriter(out, hash, counter), io.LimitReader(res.Body, pin.Size+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("download: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if n != pin.Size {
		return fmt.Errorf("download: received %d bytes, the pinned size is %d", n, pin.Size)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != pin.SHA256 {
		return fmt.Errorf("download: the SHA-256 of the archive (%s) does not match the pinned value", got)
	}
	return nil
}

type countingWriter struct {
	done     int64
	total    int64
	progress BrowserProgress
	last     time.Time
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.done += int64(len(p))
	if c.progress != nil && (time.Since(c.last) > 200*time.Millisecond || c.done == c.total) {
		c.last = time.Now()
		c.progress(c.done, c.total)
	}
	return len(p), nil
}

// extractChromium unpacks the archive into dest, stripping the top-level folder topDir. It
// refuses names that leave dest, entries outside topDir, absolute or escaping symlinks, and
// archives over the file and size limits.
func extractChromium(archive, dest, topDir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open the archive: %w", err)
	}
	defer zr.Close()
	if len(zr.File) > maxExtractFiles {
		return errors.New("the archive has too many entries")
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	prefix := topDir + "/"
	for _, f := range zr.File {
		name := f.Name
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("the archive entry %q is outside %q", name, topDir)
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" {
			continue
		}
		if strings.Contains(rel, "\\") || strings.ContainsRune(rel, 0) || path.IsAbs(rel) || !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("the archive entry %q has an unsafe name", name)
		}
		localRel := filepath.FromSlash(strings.TrimSuffix(rel, "/"))
		mode := f.Mode()
		switch {
		case f.FileInfo().IsDir():
			if err := root.MkdirAll(localRel, 0o700); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			target, err := readZipEntry(f, 4096)
			if err != nil {
				return err
			}
			t := string(target)
			resolved := path.Join(path.Dir(strings.TrimSuffix(rel, "/")), t)
			if path.IsAbs(t) || strings.Contains(t, "\\") || !filepath.IsLocal(filepath.FromSlash(resolved)) {
				return fmt.Errorf("the archive symlink %q points outside the install folder", name)
			}
			if err := root.MkdirAll(filepath.Dir(localRel), 0o700); err != nil {
				return err
			}
			if err := root.Symlink(filepath.FromSlash(t), localRel); err != nil {
				return fmt.Errorf("create the symlink %q: %w", rel, err)
			}
		case mode.IsRegular():
			total += int64(f.UncompressedSize64)
			if total > maxExtractBytes || f.UncompressedSize64 > maxExtractBytes {
				return errors.New("the archive is larger than allowed")
			}
			if err := root.MkdirAll(filepath.Dir(localRel), 0o700); err != nil {
				return err
			}
			perm := os.FileMode(0o600)
			if mode&0o111 != 0 {
				perm = 0o700
			}
			if err := writeZipFile(root, f, localRel, perm); err != nil {
				return err
			}
		default:
			return fmt.Errorf("the archive entry %q has an unsupported type", name)
		}
	}
	return nil
}

func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("an archive entry is too large")
	}
	return data, nil
}

func writeZipFile(root *os.Root, f *zip.File, rel string, perm os.FileMode) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if uint64(n) != f.UncompressedSize64 {
		return fmt.Errorf("the archive entry %q does not match its recorded size", f.Name)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/browser.go internal/storymaker/render/browser_test.go
git commit -m "feat(storymaker): browser resolution with a pinned, checksum-verified Chromium" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B5: FFmpeg frame-input pipeline

**Files:**
- Create: `internal/storymaker/render/frame.go`
- Create: `internal/storymaker/render/ffmpeg.go`
- Test: `internal/storymaker/render/helpers_test.go`
- Test: `internal/storymaker/render/ffmpeg_test.go`

FFmpeg takes frames on stdin: `-protocol_whitelist file,pipe -format_whitelist image2pipe,png_pipe` (webp: `image2pipe,webp_pipe`, raw: `rawvideo`), a silent AAC input from `anullsrc` (`-format_whitelist lavfi`), `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p` plus the encoder filter tail (FFmpeg before 7.1 would otherwise convert with BT.601 while the stream is tagged BT.709), `+faststart`. A hardware encoder gets 1.5 s after the first frame to exit, because FFmpeg opens the encoder only once it has seen a frame. `frame.go` holds the transport names and the PNG/WebP header parsers shared with the ingest. The tests encode real frames and check codec, size, frame count, rate, BT.709 tags, pure-red luma (about 63, not the BT.601 82), `moov` before `mdat`, and the AAC stream. They skip when FFmpeg is missing.

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/helpers_test.go`:

```go
package render

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// testBinary finds ffmpeg or ffprobe: AURAGO_TEST_FFMPEG_DIR, PATH, then the WinGet package folder.
func testBinary(t *testing.T, name string) string {
	t.Helper()
	exe := name
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if dir := os.Getenv("AURAGO_TEST_FFMPEG_DIR"); dir != "" {
		if p := filepath.Join(dir, exe); fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			matches, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "*FFmpeg*", "*", "bin", exe))
			if len(matches) > 0 {
				return matches[0]
			}
		}
	}
	t.Skipf("%s is not available (set AURAGO_TEST_FFMPEG_DIR)", name)
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func solidPNG(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type probeDoc struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		PixFmt       string `json:"pix_fmt"`
		AvgFrameRate string `json:"avg_frame_rate"`
		NbFrames     string `json:"nb_frames"`
		ColorSpace   string `json:"color_space"`
		ColorRange   string `json:"color_range"`
		SampleRate   string `json:"sample_rate"`
		Duration     string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func ffprobeFile(t *testing.T, ffprobe, file string) probeDoc {
	t.Helper()
	out, err := exec.Command(ffprobe, "-v", "error", "-show_streams", "-show_format", "-of", "json", file).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var doc probeDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
```

`internal/storymaker/render/ffmpeg_test.go`:

```go
package render

import (
	"bytes"
	"context"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

func TestBuildFFmpegArgs(t *testing.T) {
	args, err := BuildFFmpegArgs(FFmpegConfig{Path: "ffmpeg", Transport: TransportPNG, Width: 1920, Height: 1080, FPS: 30, Frames: 90, Output: "out.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-protocol_whitelist file,pipe -format_whitelist image2pipe,png_pipe -f image2pipe -c:v png -framerate 30 -i pipe:0",
		"-protocol_whitelist file,pipe -format_whitelist lavfi -f lavfi -t 3.000000 -i anullsrc=r=48000:cl=stereo",
		"scale=out_color_matrix=bt709:out_range=tv,format=yuv420p",
		"-c:a aac", "-movflags +faststart", "-f mp4",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q:\n%s", want, joined)
		}
	}
	raw, _ := BuildFFmpegArgs(FFmpegConfig{Path: "ffmpeg", Transport: TransportRaw, Width: 1280, Height: 720, FPS: 25, Frames: 25, Output: "o.mp4"})
	if !strings.Contains(strings.Join(raw, " "), "-format_whitelist rawvideo -f rawvideo -pixel_format rgba -video_size 1280x720 -framerate 25") {
		t.Errorf("raw args: %v", raw)
	}
	hw, _ := BuildFFmpegArgs(FFmpegConfig{Path: "ffmpeg", Encoder: videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/renderD128"}, Transport: TransportPNG, Width: 64, Height: 64, FPS: 30, Frames: 3, Output: "o.mp4"})
	hj := strings.Join(hw, " ")
	if !strings.Contains(hj, "-vaapi_device /dev/dri/renderD128") || !strings.Contains(hj, "format=yuv420p,format=nv12,hwupload") {
		t.Errorf("hardware args: %s", hj)
	}
	if _, err := BuildFFmpegArgs(FFmpegConfig{Transport: "bmp", Width: 1, Height: 1, FPS: 1, Frames: 1, Output: "o"}); err == nil {
		t.Error("unknown transport accepted")
	}
}

func encodeFrames(t *testing.T, cfg FFmpegConfig, frames [][]byte) (*FFmpeg, error) {
	t.Helper()
	ff, err := StartFFmpeg(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ff.Discard)
	for _, f := range frames {
		if err := ff.WriteFrame(f); err != nil {
			return ff, err
		}
	}
	_ = ff.Finish()
	return ff, ff.Wait(context.Background())
}

func TestFFmpegEncodesPNGFramesToMP4WithBT709AndSilentAAC(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 320, 180, 30, 45
	out := filepath.Join(t.TempDir(), "out.mp4")
	var frames [][]byte
	red := solidPNG(t, w, h, color.NRGBA{R: 255, A: 255})
	for i := 0; i < n; i++ {
		frames = append(frames, red)
	}
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	doc := ffprobeFile(t, ffprobe, out)
	var video, audio = -1, -1
	for i, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			video = i
		case "audio":
			audio = i
		}
	}
	if video < 0 || audio < 0 {
		t.Fatalf("streams = %+v", doc.Streams)
	}
	v, a := doc.Streams[video], doc.Streams[audio]
	if v.CodecName != "h264" || v.Width != w || v.Height != h || v.PixFmt != "yuv420p" || v.AvgFrameRate != "30/1" || v.NbFrames != strconv.Itoa(n) {
		t.Errorf("video = %+v", v)
	}
	if v.ColorSpace != "bt709" || v.ColorRange != "tv" {
		t.Errorf("colour tags = %q %q", v.ColorSpace, v.ColorRange)
	}
	if a.CodecName != "aac" || a.SampleRate != "48000" {
		t.Errorf("audio = %+v", a)
	}
	if d, _ := strconv.ParseFloat(doc.Format.Duration, 64); math.Abs(d-float64(n)/fps) > 0.12 {
		t.Errorf("duration = %v, want about %v", d, float64(n)/fps)
	}
	// +faststart: the moov atom comes before the media data.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if moov, mdat := bytes.Index(data, []byte("moov")), bytes.Index(data, []byte("mdat")); moov < 0 || mdat < 0 || moov > mdat {
		t.Errorf("moov at %d, mdat at %d: not faststart", moov, mdat)
	}
	// Pure red must decode to the BT.709 limited-range luma (about 63), not the BT.601 one (about 82).
	raw, err := exec.Command(ffmpeg, "-v", "error", "-i", out, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "yuv420p", "-").Output()
	if err != nil || len(raw) < w*h {
		t.Fatalf("decode: %v", err)
	}
	if y := int(raw[w*h/2+w/2]); y < 60 || y > 66 {
		t.Errorf("luma of pure red = %d, want about 63 (BT.709 limited range)", y)
	}
}

func TestFFmpegAcceptsRawFramesToo(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 160, 90, 25, 10
	out := filepath.Join(t.TempDir(), "raw.mp4")
	frame := bytes.Repeat([]byte{0, 128, 255, 255}, w*h)
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = frame
	}
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportRaw, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	doc := ffprobeFile(t, ffprobe, out)
	found := false
	for _, s := range doc.Streams {
		if s.CodecType == "video" && s.Width == w && s.Height == h && s.NbFrames == strconv.Itoa(n) {
			found = true
		}
	}
	if !found {
		t.Fatalf("streams = %+v", doc.Streams)
	}
}

func TestFFmpegWebPFrames(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 128, 72, 30, 6
	dir := t.TempDir()
	src := filepath.Join(dir, "frame.png")
	if err := os.WriteFile(src, solidPNG(t, w, h, color.NRGBA{G: 200, A: 255}), 0o600); err != nil {
		t.Fatal(err)
	}
	webpFile := filepath.Join(dir, "frame.webp")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-i", src, "-c:v", "libwebp", "-lossless", "1", webpFile).CombinedOutput(); err != nil {
		t.Skipf("this FFmpeg cannot write WebP: %v %s", err, out)
	}
	webpBytes, _ := os.ReadFile(webpFile)
	if gw, gh, err := webpSize(webpBytes); err != nil || gw != w || gh != h {
		t.Fatalf("webpSize = %d %d %v", gw, gh, err)
	}
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = webpBytes
	}
	out := filepath.Join(dir, "webp.mp4")
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportWebP, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	doc := ffprobeFile(t, ffprobe, out)
	ok := false
	for _, s := range doc.Streams {
		ok = ok || (s.CodecType == "video" && s.NbFrames == strconv.Itoa(n))
	}
	if !ok {
		t.Fatalf("streams = %+v", doc.Streams)
	}
}

func TestFFmpegWriteFailsWithAMessageWhenTheProcessDies(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	// A 16:9 frame on a 4x4 declared canvas still starts, so kill the process instead.
	ff, err := StartFFmpeg(context.Background(), FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: 64, Height: 64, FPS: 30, Frames: 5, Output: filepath.Join(t.TempDir(), "x.mp4")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ff.Discard)
	ff.Abort()
	select {
	case <-ff.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("FFmpeg did not stop")
	}
	if err := ff.WriteFrame(solidPNG(t, 64, 64, color.NRGBA{A: 255})); err == nil {
		t.Fatal("a write to a dead FFmpeg succeeded")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestBuildFFmpegArgs|TestFFmpeg" -count=1`

Expected: FAIL (build failed: `undefined: BuildFFmpegArgs`, `undefined: FFmpegConfig`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/frame.go`:

```go
package render

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// Frame transports (storymaker.render.transport).
const (
	TransportPNG        = "png"
	TransportWebP       = "webp"
	TransportRaw        = "raw"
	TransportScreenshot = "screenshot"
)

// Frame headers. The ingest checks them before a frame reaches FFmpeg; the tests read them too.
var (
	pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	pngTrailer   = []byte{0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
)

// pngSize reads the IHDR and checks that the file is complete (it ends with the IEND chunk), so a
// truncated body never desynchronises the image2pipe demuxer.
func pngSize(data []byte) (int, int, error) {
	if len(data) < 33+len(pngTrailer) || !bytes.Equal(data[:8], pngSignature) {
		return 0, 0, errors.New("the frame is not a PNG")
	}
	if binary.BigEndian.Uint32(data[8:12]) != 13 || string(data[12:16]) != "IHDR" {
		return 0, 0, errors.New("the PNG has no IHDR chunk first")
	}
	if !bytes.Equal(data[len(data)-len(pngTrailer):], pngTrailer) {
		return 0, 0, errors.New("the PNG is truncated")
	}
	return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24])), nil
}

// webpSize reads the canvas size of a RIFF WebP (lossy VP8, lossless VP8L or extended VP8X) and checks
// that the RIFF length matches the body.
func webpSize(data []byte) (int, int, error) {
	if len(data) < 30 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, errors.New("the frame is not a WebP")
	}
	if riff := int(binary.LittleEndian.Uint32(data[4:8])); riff+8 != len(data) && riff+9 != len(data) {
		return 0, 0, errors.New("the WebP is truncated")
	}
	switch string(data[12:16]) {
	case "VP8X":
		w := 1 + (int(data[24]) | int(data[25])<<8 | int(data[26])<<16)
		h := 1 + (int(data[27]) | int(data[28])<<8 | int(data[29])<<16)
		return w, h, nil
	case "VP8L":
		if data[20] != 0x2f {
			return 0, 0, errors.New("the WebP lossless header is invalid")
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, nil
	case "VP8 ":
		if data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, errors.New("the WebP lossy header is invalid")
		}
		return int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff), nil
	}
	return 0, 0, errors.New("the WebP has an unknown first chunk")
}
```

`internal/storymaker/render/ffmpeg.go`:

```go
package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/videostudio"
)

const (
	ffmpegWriteTimeout = 30 * time.Second
	ffmpegWaitAfterEnd = 120 * time.Second
	ffmpegStderrKeep   = 8 << 10
	// hardwareSettle is how long a hardware encoder gets after the first frame to fail: FFmpeg opens the
	// encoder only once it has seen a frame, and a failure must surface before the page renders on.
	hardwareSettle      = 1500 * time.Millisecond
	silentAudioRate     = 48000
	silentAudioBitrate  = "64k"
	silentAudioLavfiSrc = "anullsrc=r=48000:cl=stereo"
)

// FFmpegConfig describes one frame-input encode.
type FFmpegConfig struct {
	Path      string
	Encoder   videostudio.Encoder
	Transport string // png, webp, raw or screenshot (PNG frames)
	Width     int
	Height    int
	FPS       int
	Frames    int
	Output    string
}

// BuildFFmpegArgs returns the FFmpeg arguments of a render: frames on stdin (video), a silent
// AAC track from anullsrc (so the MP4 has the stream layout later phases mix into), BT.709
// limited-range YUV 4:2:0 through an explicit scale (FFmpeg before 7.1 would otherwise convert with
// BT.601 while the stream is tagged BT.709), the encoder's filter tail, and +faststart. Both inputs
// carry protocol and format whitelists.
func BuildFFmpegArgs(cfg FFmpegConfig) ([]string, error) {
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.FPS <= 0 || cfg.Frames <= 0 || cfg.Output == "" {
		return nil, errors.New("storymaker: incomplete ffmpeg configuration")
	}
	fps := strconv.Itoa(cfg.FPS)
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-nostats"}
	args = append(args, cfg.Encoder.GlobalArgs()...)
	switch cfg.Transport {
	case TransportPNG, TransportScreenshot:
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "image2pipe,png_pipe",
			"-f", "image2pipe", "-c:v", "png", "-framerate", fps, "-i", "pipe:0")
	case TransportWebP:
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "image2pipe,webp_pipe",
			"-f", "image2pipe", "-c:v", "webp", "-framerate", fps, "-i", "pipe:0")
	case TransportRaw:
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "rawvideo",
			"-f", "rawvideo", "-pixel_format", "rgba", "-video_size", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
			"-framerate", fps, "-i", "pipe:0")
	default:
		return nil, fmt.Errorf("storymaker: unknown transport %q", cfg.Transport)
	}
	seconds := strconv.FormatFloat(float64(cfg.Frames)/float64(cfg.FPS), 'f', 6, 64)
	args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "lavfi",
		"-f", "lavfi", "-t", seconds, "-i", silentAudioLavfiSrc)
	args = append(args, "-map", "0:v:0", "-map", "1:a:0",
		"-vf", "scale=out_color_matrix=bt709:out_range=tv,format=yuv420p"+cfg.Encoder.FilterTail())
	args = append(args, cfg.Encoder.VideoArgs(videostudio.Rate{Num: int64(cfg.FPS), Den: 1})...)
	args = append(args, "-c:a", "aac", "-b:a", silentAudioBitrate, "-ar", strconv.Itoa(silentAudioRate), "-ac", "2",
		"-shortest", "-movflags", "+faststart", "-f", "mp4", "-y", cfg.Output)
	return args, nil
}

// FFmpeg is a running frame-input encode.
type FFmpeg struct {
	cmd   *exec.Cmd
	stdin interface {
		Write([]byte) (int, error)
		Close() error
	}
	output string
	settle time.Duration
	frames int
	stderr *tailBuffer
	done   chan struct{}

	mu        sync.Mutex
	waitErr   error
	stdinOnce sync.Once
}

// StartFFmpeg starts the process. ctx ends it.
func StartFFmpeg(ctx context.Context, cfg FFmpegConfig) (*FFmpeg, error) {
	args, err := BuildFFmpegArgs(cfg)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(cfg.Output)
	cmd := exec.CommandContext(ctx, cfg.Path, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	f := &FFmpeg{cmd: cmd, stdin: stdin, output: cfg.Output, stderr: &tailBuffer{limit: ffmpegStderrKeep}, done: make(chan struct{})}
	if cfg.Encoder.Hardware() {
		f.settle = hardwareSettle
	}
	cmd.Stderr = f.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start FFmpeg: %w", err)
	}
	go func() {
		err := cmd.Wait()
		f.mu.Lock()
		f.waitErr = err
		f.mu.Unlock()
		close(f.done)
	}()
	return f, nil
}

// Done is closed when the process has exited.
func (f *FFmpeg) Done() <-chan struct{} { return f.done }

// Err is the exit error once Done is closed.
func (f *FFmpeg) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.waitErr
}

// Message is the tail of FFmpeg's error output with the output path scrubbed.
func (f *FFmpeg) Message() string {
	text := strings.TrimSpace(f.stderr.String())
	if f.output != "" {
		text = strings.ReplaceAll(text, f.output, "<output>")
	}
	return clip(text, 600)
}

// WriteFrame implements FrameSink. A write that does not complete within 30 seconds kills FFmpeg.
func (f *FFmpeg) WriteFrame(data []byte) error {
	select {
	case <-f.done:
		return f.exitError()
	default:
	}
	timer := time.AfterFunc(ffmpegWriteTimeout, f.Abort)
	defer timer.Stop()
	if _, err := f.stdin.Write(data); err != nil {
		// Give the process a moment to exit so its message explains the broken pipe.
		select {
		case <-f.done:
		case <-time.After(2 * time.Second):
		}
		if exit := f.exitError(); exit != nil {
			return exit
		}
		return fmt.Errorf("write to FFmpeg: %w", err)
	}
	f.frames++
	if f.frames == 1 && f.settle > 0 {
		select {
		case <-f.done:
			return f.exitError()
		case <-time.After(f.settle):
		}
	}
	return nil
}

func (f *FFmpeg) exitError() error {
	if msg := f.Message(); msg != "" {
		return fmt.Errorf("FFmpeg stopped: %s", msg)
	}
	if err := f.Err(); err != nil {
		return fmt.Errorf("FFmpeg stopped: %w", err)
	}
	return errors.New("FFmpeg stopped")
}

// Finish implements FrameSink: it closes the frame input so FFmpeg writes the trailer and exits.
func (f *FFmpeg) Finish() error {
	f.closeStdin()
	return nil
}

func (f *FFmpeg) closeStdin() { f.stdinOnce.Do(func() { _ = f.stdin.Close() }) }

// Wait waits for the process to end after Finish and returns its error with FFmpeg's message.
func (f *FFmpeg) Wait(ctx context.Context) error {
	timer := time.NewTimer(ffmpegWaitAfterEnd)
	defer timer.Stop()
	select {
	case <-f.done:
	case <-ctx.Done():
		f.Abort()
		return ctx.Err()
	case <-timer.C:
		f.Abort()
		return errors.New("FFmpeg did not finish after the last frame")
	}
	if f.Err() != nil {
		return f.exitError()
	}
	return nil
}

// Abort kills the process and removes its partial output. It is safe to call more than once.
func (f *FFmpeg) Abort() {
	f.closeStdin()
	if f.cmd.Process != nil {
		_ = f.cmd.Process.Kill()
	}
}

// Discard aborts the process, waits for it and deletes the partial output file.
func (f *FFmpeg) Discard() {
	f.Abort()
	<-f.done
	_ = os.Remove(f.output)
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.limit {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.limit:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/frame.go internal/storymaker/render/ffmpeg.go internal/storymaker/render/helpers_test.go internal/storymaker/render/ffmpeg_test.go
git commit -m "feat(storymaker): FFmpeg frame-input pipeline with BT.709 conversion and silent AAC" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B6: Frame ingest

**Files:**
- Create: `internal/storymaker/render/ingest.go`
- Test: `internal/storymaker/render/ingest_test.go`

Frames are accepted only in the running state, strictly in order from 0, below the Go-computed limit (`render.max_duration_seconds` times fps) and the frame count the report announced; the Content-Type must be the transport's (or `text/plain`, the CORS-simple form), a PNG must be complete (IHDR size, IEND trailer) and exactly the canvas size, a WebP must have the right RIFF length and size, raw RGBA exactly width x height x 4 bytes. A failing sink may be replaced once (the software restart); the page then renders from frame 0 again.

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/ingest_test.go`:

```go
package render

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image/color"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/videostudio"
)

type fakeSink struct {
	frames   [][]byte
	finished bool
	failAt   int // 1-based write number that fails; 0 never
	writes   int
}

func (s *fakeSink) WriteFrame(data []byte) error {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return errors.New("pipe closed")
	}
	s.frames = append(s.frames, append([]byte(nil), data...))
	return nil
}

func (s *fakeSink) Finish() error { s.finished = true; return nil }

func startedIngest(t *testing.T, cfg IngestConfig, total int) (*Ingest, *fakeSink) {
	t.Helper()
	in := NewIngest(cfg)
	sink := &fakeSink{}
	in.Start(total, sink, nil, nil)
	return in, sink
}

func ingestCode(err error) string {
	var ie *IngestError
	if errors.As(err, &ie) {
		return ie.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

func TestIngestRefusesFramesUntilTheRenderRuns(t *testing.T) {
	cfg := IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 10}
	in := NewIngest(cfg)
	frame := solidPNG(t, 64, 36, color.NRGBA{A: 255})
	if _, err := in.Accept(0, "image/png", frame); ingestCode(err) != "not_running" {
		t.Fatalf("before Start: %v", err)
	}
	sink := &fakeSink{}
	in.Start(2, sink, nil, nil)
	if _, err := in.Accept(0, "image/png", frame); err != nil {
		t.Fatal(err)
	}
	in.Stop()
	if _, err := in.Accept(1, "image/png", frame); ingestCode(err) != "not_running" {
		t.Fatalf("after Stop: %v", err)
	}
	if len(sink.frames) != 1 {
		t.Fatalf("the sink got %d frames", len(sink.frames))
	}
}

func TestIngestEnforcesOrderLimitAndCompletion(t *testing.T) {
	cfg := IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 3}
	frame := solidPNG(t, 64, 36, color.NRGBA{A: 255})
	in, sink := startedIngest(t, cfg, 5) // the report announced 5 frames, the Go limit allows 3
	for _, tc := range []struct {
		n    int
		code string
	}{{1, "frame_order"}, {0, ""}, {0, "frame_order"}, {2, "frame_order"}, {1, ""}, {2, ""}, {3, "frame_limit"}} {
		_, err := in.Accept(tc.n, "image/png", frame)
		if ingestCode(err) != tc.code {
			t.Fatalf("frame %d: got %v, want %q", tc.n, err, tc.code)
		}
	}
	if len(sink.frames) != 3 || in.Bytes() != int64(3*len(frame)) {
		t.Fatalf("sink frames %d, bytes %d", len(sink.frames), in.Bytes())
	}

	in2, sink2 := startedIngest(t, IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 100}, 2)
	if out, err := in2.Accept(0, "image/png", frame); err != nil || out != FrameOK {
		t.Fatalf("first: %v %v", out, err)
	}
	if out, err := in2.Accept(1, "image/png", frame); err != nil || out != FrameDone || !sink2.finished {
		t.Fatalf("last: %v %v finished=%v", out, err, sink2.finished)
	}
	if _, err := in2.Accept(2, "image/png", frame); ingestCode(err) != "not_running" {
		t.Fatalf("after the last frame: %v", err)
	}
}

func TestIngestChecksContentTypeAndHeaderAgainstTheCanvas(t *testing.T) {
	good := solidPNG(t, 64, 36, color.NRGBA{A: 255})
	wrongSize := solidPNG(t, 65, 36, color.NRGBA{A: 255})
	truncated := good[:len(good)-5]
	cases := []struct {
		name        string
		cfg         IngestConfig
		contentType string
		data        []byte
		code        string
	}{
		{"ok png", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", good, ""},
		{"ok png with charset parameter", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png; x=y", good, ""},
		{"png wrong dimensions", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", wrongSize, "frame_size"},
		{"png truncated", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", truncated, "frame_format"},
		{"png garbage", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/png", bytes.Repeat([]byte{1}, 100), "frame_format"},
		{"png sent as raw", IngestConfig{Width: 64, Height: 36, Transport: TransportRaw}, "image/png", good, "content_type"},
		{"webp sent as png", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "image/webp", good, "content_type"},
		{"no content type", IngestConfig{Width: 64, Height: 36, Transport: TransportPNG}, "", good, "content_type"},
		{"ok raw", IngestConfig{Width: 4, Height: 2, Transport: TransportRaw}, "application/octet-stream", make([]byte, 32), ""},
		{"raw too short", IngestConfig{Width: 4, Height: 2, Transport: TransportRaw}, "application/octet-stream", make([]byte, 31), "frame_size"},
		{"raw too long", IngestConfig{Width: 4, Height: 2, Transport: TransportRaw}, "application/octet-stream", make([]byte, 33), "frame_size"},
		{"ok webp lossless", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8L", 640, 360), ""},
		{"ok webp extended", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8X", 640, 360), ""},
		{"ok webp lossy", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8 ", 640, 360), ""},
		{"webp wrong dimensions", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8L", 641, 360), "frame_size"},
		{"webp truncated", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", fakeWebP("VP8L", 640, 360)[:40], "frame_format"},
		{"not a webp", IngestConfig{Width: 640, Height: 360, Transport: TransportWebP}, "image/webp", good, "frame_format"},
	}
	for _, tc := range cases {
		tc.cfg.FrameLimit = 10
		in, sink := startedIngest(t, tc.cfg, 5)
		_, err := in.Accept(0, tc.contentType, tc.data)
		if ingestCode(err) != tc.code {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.code)
		}
		if tc.code != "" && len(sink.frames) != 0 {
			t.Errorf("%s: a refused frame reached the sink", tc.name)
		}
	}
}

// fakeWebP builds the header bytes of a WebP of the given chunk type and size, padded to a plausible body.
func fakeWebP(chunk string, w, h int) []byte {
	body := make([]byte, 40)
	// body[i] sits at file offset 20+i.
	switch chunk {
	case "VP8L":
		body[0] = 0x2f
		binary.LittleEndian.PutUint32(body[1:5], uint32(w-1)|uint32(h-1)<<14)
	case "VP8X":
		body[4], body[5], body[6] = byte(w-1), byte((w-1)>>8), byte((w-1)>>16)
		body[7], body[8], body[9] = byte(h-1), byte((h-1)>>8), byte((h-1)>>16)
	case "VP8 ":
		body[3], body[4], body[5] = 0x9d, 0x01, 0x2a
		binary.LittleEndian.PutUint16(body[6:8], uint16(w))
		binary.LittleEndian.PutUint16(body[8:10], uint16(h))
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(4+8+len(body)))
	out.WriteString("WEBP")
	out.WriteString(chunk)
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(body)))
	out.Write(body)
	return out.Bytes()
}

func TestIngestSinkFailureWithoutRecoveryIsFatalAndWithRecoveryRestarts(t *testing.T) {
	cfg := IngestConfig{Width: 64, Height: 36, Transport: TransportPNG, FrameLimit: 10}
	frame := solidPNG(t, 64, 36, color.NRGBA{A: 255})

	in := NewIngest(cfg)
	in.Start(3, &fakeSink{failAt: 1}, nil, nil)
	if _, err := in.Accept(0, "image/png", frame); ingestCode(err) != "encoder_failed" {
		t.Fatalf("no recovery: %v", err)
	}

	in = NewIngest(cfg)
	replacement := &fakeSink{}
	calls := 0
	in.Start(3, &fakeSink{failAt: 2}, func(error) (FrameSink, bool) {
		calls++
		return replacement, calls == 1
	}, nil)
	if _, err := in.Accept(0, "image/png", frame); err != nil {
		t.Fatal(err)
	}
	out, err := in.Accept(1, "image/png", frame)
	if err != nil || out != FrameRestart || in.Next() != 0 {
		t.Fatalf("restart: %v %v next=%d", out, err, in.Next())
	}
	for i := 0; i < 3; i++ {
		if _, err := in.Accept(i, "image/png", frame); err != nil {
			t.Fatalf("after the restart, frame %d: %v", i, err)
		}
	}
	if len(replacement.frames) != 3 || !replacement.finished {
		t.Fatalf("replacement sink got %d frames, finished=%v", len(replacement.frames), replacement.finished)
	}
}

func TestMaxFrameBytes(t *testing.T) {
	if got := MaxFrameBytes(IngestConfig{Width: 1920, Height: 1080, Transport: TransportPNG}); got != 4<<20 {
		t.Errorf("1080p png = %d", got)
	}
	if got := MaxFrameBytes(IngestConfig{Width: 1920, Height: 1080, Transport: TransportRaw}); got != 1920*1080*4 {
		t.Errorf("1080p raw = %d", got)
	}
	if got := MaxFrameBytes(IngestConfig{Width: 320, Height: 180, Transport: TransportPNG}); got != 4<<20 {
		t.Errorf("small png = %d, want the 4 MiB floor", got)
	}
	if got := MaxFrameBytes(IngestConfig{Width: 3840, Height: 2160, Transport: TransportWebP}); got != 16<<20 {
		t.Errorf("4K webp = %d", got)
	}
}

func TestIngestRestartsWithSoftwareWhenTheHardwareEncoderFails(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 160, 90, 30, 12
	dir := t.TempDir()
	base := FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: w, Height: h, FPS: fps, Frames: n, Output: filepath.Join(dir, "hw.mp4")}
	hwCfg := base
	// VAAPI is missing or has no device on this host, so FFmpeg exits during start-up.
	hwCfg.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	hw, err := StartFFmpeg(context.Background(), hwCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(hw.Discard)
	ing := NewIngest(IngestConfig{Width: w, Height: h, Transport: TransportPNG, FrameLimit: 100})
	var soft *FFmpeg
	recoverFn := func(err error) (FrameSink, bool) {
		var e error
		soft, e = StartFFmpeg(context.Background(), base)
		if e != nil {
			t.Fatal(e)
		}
		return soft, true
	}
	ing.Start(n, hw, recoverFn, nil)
	t.Cleanup(func() {
		if soft != nil {
			soft.Discard()
		}
	})
	frame := solidPNG(t, w, h, color.NRGBA{B: 255, A: 255})
	outcome, err := ing.Accept(0, "image/png", frame)
	if err != nil || outcome != FrameRestart {
		t.Fatalf("first frame = %v %v, want a restart", outcome, err)
	}
	if ing.Next() != 0 {
		t.Fatalf("next = %d after a restart", ing.Next())
	}
	for i := 0; i < n; i++ {
		outcome, err = ing.Accept(i, "image/png", frame)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	if outcome != FrameDone {
		t.Fatalf("last outcome = %v", outcome)
	}
	if err := soft.Wait(context.Background()); err != nil {
		t.Fatalf("software encode: %v", err)
	}
	doc := ffprobeFile(t, ffprobe, base.Output)
	ok := false
	for _, s := range doc.Streams {
		ok = ok || (s.CodecType == "video" && s.NbFrames == strconv.Itoa(n))
	}
	if !ok {
		t.Fatalf("streams = %+v", doc.Streams)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestIngest|TestMaxFrameBytes" -count=1`

Expected: FAIL (build failed: `undefined: IngestConfig`, `undefined: Ingest`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/ingest.go`:

```go
package render

import (
	"fmt"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"
)

// IngestError is a refused frame. Status is the HTTP status the listener answers with; the render
// job treats every IngestError as fatal.
type IngestError struct {
	Code    string
	Message string
	Status  int
}

func (e *IngestError) Error() string { return e.Code + ": " + e.Message }

func refuse(status int, code, format string, args ...any) *IngestError {
	return &IngestError{Code: code, Message: fmt.Sprintf(format, args...), Status: status}
}

// FrameSink consumes the accepted frames, normally the stdin of FFmpeg.
type FrameSink interface {
	// WriteFrame receives the validated bytes of one frame.
	WriteFrame(data []byte) error
	// Finish ends the stream after the last frame.
	Finish() error
}

// FrameOutcome is what the listener tells the page after a frame.
type FrameOutcome int

const (
	// FrameOK: the frame was taken, send the next one.
	FrameOK FrameOutcome = iota
	// FrameDone: the last frame was taken.
	FrameDone
	// FrameRestart: the encoder was replaced; the page renders from frame 0 again.
	FrameRestart
)

type ingestState int

const (
	ingestIdle ingestState = iota
	ingestRunning
	ingestFinished
	ingestStopped
)

// IngestConfig fixes what a frame must look like and how many are allowed.
type IngestConfig struct {
	Width, Height int
	Transport     string
	// FrameLimit is the most frames the job may ever take: Go computes it from
	// render.max_duration_seconds times fps, never from the page.
	FrameLimit int
}

// Ingest is the strict state machine behind POST <base>/frame: running state only, frames in order
// from 0, a Go-computed frame limit, Content-Type and header or size checks against the canvas.
type Ingest struct {
	cfg IngestConfig

	mu      sync.Mutex
	state   ingestState
	next    int
	total   int
	sink    FrameSink
	recover func(err error) (FrameSink, bool)
	onFrame func(n int)
	bytes   atomic.Int64
}

// NewIngest returns an idle ingest: every frame is refused until Start.
func NewIngest(cfg IngestConfig) *Ingest { return &Ingest{cfg: cfg} }

// MaxFrameBytes is the largest body the listener reads for one frame: 4 MiB for a 1080p PNG or
// WebP (scaled by pixels for other canvases), exactly width x height x 4 for raw RGBA.
func MaxFrameBytes(cfg IngestConfig) int64 {
	if cfg.Transport == TransportRaw {
		return int64(cfg.Width) * int64(cfg.Height) * 4
	}
	const base = 4 << 20
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if scaled := base * pixels / (1920 * 1080); scaled > base {
		return scaled
	}
	return base
}

// Start moves to the running state with the number of frames the validated report announced. recoverFn
// is called when the sink fails: it may return a replacement sink (the one-shot software restart),
// in which case the page is told to start over.
func (in *Ingest) Start(total int, sink FrameSink, recoverFn func(err error) (FrameSink, bool), onFrame func(n int)) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.state, in.next, in.total, in.sink, in.recover, in.onFrame = ingestRunning, 0, total, sink, recoverFn, onFrame
}

// Stop refuses every further frame (the job left the running state).
func (in *Ingest) Stop() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.state = ingestStopped
}

// Next returns the index of the next expected frame.
func (in *Ingest) Next() int {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.next
}

// Bytes is the total of accepted frame bytes.
func (in *Ingest) Bytes() int64 { return in.bytes.Load() }

// Accept validates and forwards one frame. n is the X-SM-Frame value.
func (in *Ingest) Accept(n int, contentType string, data []byte) (FrameOutcome, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.state != ingestRunning {
		return FrameOK, refuse(http.StatusConflict, "not_running", "The render is not accepting frames.")
	}
	if n < 0 || n != in.next {
		return FrameOK, refuse(http.StatusConflict, "frame_order", "Frame %d arrived, frame %d was expected.", n, in.next)
	}
	if n >= in.cfg.FrameLimit || n >= in.total {
		return FrameOK, refuse(http.StatusRequestEntityTooLarge, "frame_limit", "Frame %d is past the limit of %d frames.", n, min(in.cfg.FrameLimit, in.total))
	}
	if err := checkFrame(in.cfg, contentType, data); err != nil {
		return FrameOK, err
	}
	if err := in.sink.WriteFrame(data); err != nil {
		if in.recover != nil {
			if replacement, ok := in.recover(err); ok {
				in.sink, in.next = replacement, 0
				return FrameRestart, nil
			}
		}
		return FrameOK, refuse(http.StatusInternalServerError, "encoder_failed", "The encoder stopped: %v", err)
	}
	in.bytes.Add(int64(len(data)))
	in.next++
	if in.onFrame != nil {
		in.onFrame(n)
	}
	if in.next >= in.total {
		in.state = ingestFinished
		if err := in.sink.Finish(); err != nil {
			return FrameDone, refuse(http.StatusInternalServerError, "encoder_failed", "The encoder could not finish: %v", err)
		}
		return FrameDone, nil
	}
	return FrameOK, nil
}

// checkFrame verifies content type and payload against the canvas before any byte reaches FFmpeg.
func checkFrame(cfg IngestConfig, contentType string, data []byte) *IngestError {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return refuse(http.StatusUnsupportedMediaType, "content_type", "The frame has no usable Content-Type.")
	}
	want := map[string]string{TransportPNG: "image/png", TransportWebP: "image/webp", TransportRaw: "application/octet-stream", TransportScreenshot: "image/png"}[cfg.Transport]
	// text/plain is the CORS-safelisted type a page uses to send frames without a preflight request;
	// the bytes are checked against the transport below either way.
	if media != want && media != "text/plain" {
		return refuse(http.StatusUnsupportedMediaType, "content_type", "The transport %s expects %s frames, not %s.", cfg.Transport, want, media)
	}
	switch cfg.Transport {
	case TransportRaw:
		if want := cfg.Width * cfg.Height * 4; len(data) != want {
			return refuse(http.StatusBadRequest, "frame_size", "A raw frame has %d bytes, the canvas needs %d.", len(data), want)
		}
	case TransportPNG, TransportScreenshot:
		w, h, err := pngSize(data)
		if err != nil {
			return refuse(http.StatusBadRequest, "frame_format", "%v", err)
		}
		if w != cfg.Width || h != cfg.Height {
			return refuse(http.StatusBadRequest, "frame_size", "The frame is %dx%d, the canvas is %dx%d.", w, h, cfg.Width, cfg.Height)
		}
	case TransportWebP:
		w, h, err := webpSize(data)
		if err != nil {
			return refuse(http.StatusBadRequest, "frame_format", "%v", err)
		}
		if w != cfg.Width || h != cfg.Height {
			return refuse(http.StatusBadRequest, "frame_size", "The frame is %dx%d, the canvas is %dx%d.", w, h, cfg.Width, cfg.Height)
		}
	default:
		return refuse(http.StatusBadRequest, "transport", "Unknown transport %q.", cfg.Transport)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/ingest.go internal/storymaker/render/ingest_test.go
git commit -m "feat(storymaker): strict frame ingest with order, limit and header checks" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B7: Render listener

**Files:**
- Create: `internal/storymaker/render/listener.go`
- Test: `internal/storymaker/render/listener_test.go`

The listener binds `127.0.0.1:0`, uses plain HTTP with its own timeouts and no logging of paths, checks the Host header, and answers 404 for everything outside `/<prefix>/` (the prefix is 128 bits from `crypto/rand`). Below the base: the render page with the exact CSP, `Referrer-Policy: no-referrer` and `Cache-Control: no-store`; `boot.json`; the snapshot (`story.json`, `catalog/...`, `fonts/...`, `scenes/<id>.js`, the two player bundles); `report`, `frame` and `error`. Every response carries `Access-Control-Allow-Origin: *` because the sandboxed page has an opaque origin; `OPTIONS` answers the preflight (the frame number may be the `X-SM-Frame` header or the `n` query parameter).

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/listener_test.go`:

```go
package render

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type listenerFixture struct {
	l      *Listener
	client *http.Client
	mu     sync.Mutex
	report []byte
	frames []string
	errors []PageError
}

func newListenerFixture(t *testing.T) *listenerFixture {
	t.Helper()
	fx := &listenerFixture{client: &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}}
	prefix, err := NewPrefix()
	if err != nil {
		t.Fatal(err)
	}
	snap := &Snapshot{
		Story:        []byte(`{"schema":"storymaker/1"}`),
		CatalogFiles: map[string][]byte{"catalog.json": []byte(`{"schema":"storymaker-catalog/1"}`), "rig/human.json": []byte(`{"id":"rig.human/1"}`)},
		Fonts:        []SnapshotFont{{Name: "default", Family: "Geist", Weight: 400, File: "/fonts/Geist-Regular.woff2", Data: []byte("wOF2font")}},
		CodeBundles:  map[string][]byte{"s3": []byte("self.__smScene=function(){};")},
		HostBundle:   []byte("/*host*/"), PlayerBundle: []byte("/*player*/"),
	}
	params := PageParams{Width: 1920, Height: 1080, FPS: 30, Transport: TransportPNG, FrameLimit: 27000}
	hooks := Hooks{
		Report: func(body []byte) ReportAnswer {
			fx.mu.Lock()
			fx.report = append([]byte(nil), body...)
			fx.mu.Unlock()
			return ReportAnswer{Start: true}
		},
		Frame: func(n int, ct string, data []byte) (FrameAnswer, int) {
			fx.mu.Lock()
			fx.frames = append(fx.frames, ct+"|"+string(data))
			fx.mu.Unlock()
			if n == 2 {
				return FrameAnswer{OK: true, Done: true}, http.StatusOK
			}
			return FrameAnswer{OK: true}, http.StatusOK
		},
		Error: func(e PageError) {
			fx.mu.Lock()
			fx.errors = append(fx.errors, e)
			fx.mu.Unlock()
		},
	}
	l, err := StartListener(prefix, snap, params, hooks)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	fx.l = l
	return fx
}

func (fx *listenerFixture) do(t *testing.T, method, rawURL string, body []byte, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, rawURL, err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res, data
}

func TestListenerBindsLoopbackAndAnswers404OutsideTheBase(t *testing.T) {
	fx := newListenerFixture(t)
	if !strings.HasPrefix(fx.l.Origin(), "http://127.0.0.1:") {
		t.Fatalf("origin = %s", fx.l.Origin())
	}
	if len(fx.l.prefix) != 32 {
		t.Fatalf("prefix length = %d", len(fx.l.prefix))
	}
	other := strings.Repeat("0", 32)
	for _, p := range []string{
		"/", "/favicon.ico", "/" + fx.l.prefix, "/" + other + "/", "/" + other + "/boot.json",
		"/" + fx.l.prefix + "/../story.json", "/" + fx.l.prefix[:30] + "/", "/js/storymaker/host.bundle.js",
		"/" + fx.l.prefix + "/nothing", "/" + fx.l.prefix + "/catalog/missing.json", "/" + fx.l.prefix + "/catalog/../story.json",
		"/" + fx.l.prefix + "/fonts/Other.woff2", "/" + fx.l.prefix + "/scenes/nope.js", "/" + fx.l.prefix + "/story.json/",
		"/" + fx.l.prefix + "//story.json",
	} {
		res, _ := fx.do(t, http.MethodGet, fx.l.Origin()+p, nil, nil)
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, res.StatusCode)
		}
		if res.Header.Get("Access-Control-Allow-Origin") != "" && !strings.HasPrefix(p, "/"+fx.l.prefix) {
			t.Errorf("GET %s sent CORS headers outside the base", p)
		}
	}
	// A request that names another Host (DNS rebinding) is refused even with the right path.
	res, _ := fx.do(t, http.MethodGet, fx.l.Base()+"/boot.json", nil, map[string]string{"Host": "evil.example:80"})
	_ = res
	req, _ := http.NewRequest(http.MethodGet, fx.l.Base()+"/boot.json", nil)
	req.Host = "evil.example"
	rebound, err := fx.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = rebound.Body.Close()
	if rebound.StatusCode != http.StatusNotFound {
		t.Errorf("foreign Host header = %d, want 404", rebound.StatusCode)
	}
}

func TestListenerServesTheRenderPageWithTheExactHeaders(t *testing.T) {
	fx := newListenerFixture(t)
	res, body := fx.do(t, http.MethodGet, fx.l.Base()+"/", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	base := fx.l.Base()
	wantCSP := "sandbox allow-scripts; default-src 'none'; script-src " + base + "/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src " + base + "/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"
	if got := res.Header.Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("CSP = %q\nwant %q", got, wantCSP)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := res.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q", got)
	}
	want := `<!doctype html><html data-sm-mode="render" data-sm-base="` + base + `"><head><meta charset="utf-8"></head>` +
		`<body><canvas id="sm-stage" width="1920" height="1080"></canvas><script src="` + base + `/js/storymaker/host.bundle.js"></script></body></html>`
	if string(body) != want {
		t.Errorf("page:\n%s\nwant:\n%s", body, want)
	}
	if strings.Contains(string(body), "<style") || strings.Contains(string(body), "onload") || strings.Contains(string(body), "<script>") {
		t.Error("the render page carries inline script or style")
	}
	// HEAD works without a body.
	head, hb := fx.do(t, http.MethodHead, base+"/", nil, nil)
	if head.StatusCode != http.StatusOK || len(hb) != 0 {
		t.Errorf("HEAD = %d with %d body bytes", head.StatusCode, len(hb))
	}
}

func TestListenerServesTheSnapshotOnly(t *testing.T) {
	fx := newListenerFixture(t)
	base := fx.l.Base()
	_, boot := fx.do(t, http.MethodGet, base+"/boot.json", nil, nil)
	var doc map[string]any
	if err := json.Unmarshal(boot, &doc); err != nil {
		t.Fatalf("boot.json: %v\n%s", err, boot)
	}
	checks := map[string]any{"mode": "render", "fps": float64(30), "width": float64(1920), "height": float64(1080), "transport": "png",
		"frame_limit": float64(27000), "story_url": base + "/story.json", "catalog_url": base + "/catalog/catalog.json", "assets_base": base + "/catalog/"}
	for k, want := range checks {
		if doc[k] != want {
			t.Errorf("boot.%s = %v, want %v", k, doc[k], want)
		}
	}
	bundles, _ := doc["code_bundles"].(map[string]any)
	if bundles["s3"] != base+"/scenes/s3.js" {
		t.Errorf("code_bundles = %v", doc["code_bundles"])
	}
	fonts, _ := doc["fonts"].([]any)
	if len(fonts) != 1 {
		t.Fatalf("fonts = %v", doc["fonts"])
	}
	font := fonts[0].(map[string]any)
	if font["name"] != "default" || font["family"] != "Geist" || font["weight"] != float64(400) || font["url"] != base+"/fonts/Geist-Regular.woff2" {
		t.Errorf("font = %v", font)
	}
	if _, ok := doc["audio_info"].(map[string]any); !ok {
		t.Errorf("audio_info = %v", doc["audio_info"])
	}

	for path, want := range map[string]string{
		"/story.json": `{"schema":"storymaker/1"}`, "/catalog/catalog.json": `{"schema":"storymaker-catalog/1"}`,
		"/catalog/rig/human.json": `{"id":"rig.human/1"}`, "/fonts/Geist-Regular.woff2": "wOF2font", "/scenes/s3.js": "self.__smScene=function(){};",
		"/js/storymaker/host.bundle.js": "/*host*/", "/js/storymaker/player.bundle.js": "/*player*/",
	} {
		res, body := fx.do(t, http.MethodGet, base+path, nil, nil)
		if res.StatusCode != http.StatusOK || string(body) != want {
			t.Errorf("GET %s = %d %q", path, res.StatusCode, body)
		}
		if res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s lacks the response headers: %v", path, res.Header)
		}
	}
	if res, _ := fx.do(t, http.MethodGet, base+"/scenes/s3.js", nil, nil); !strings.HasPrefix(res.Header.Get("Content-Type"), "text/javascript") {
		t.Errorf("bundle Content-Type = %q", res.Header.Get("Content-Type"))
	}
	if res, _ := fx.do(t, http.MethodGet, base+"/fonts/Geist-Regular.woff2", nil, nil); res.Header.Get("Content-Type") != "font/woff2" {
		t.Errorf("font Content-Type = %q", res.Header.Get("Content-Type"))
	}
	// Static routes reject POST.
	if res, _ := fx.do(t, http.MethodPost, base+"/story.json", []byte("x"), nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST story.json = %d", res.StatusCode)
	}
}

func TestListenerAnswersCORSPreflight(t *testing.T) {
	fx := newListenerFixture(t)
	for _, p := range []string{"/frame", "/report", "/error", "/boot.json"} {
		res, _ := fx.do(t, http.MethodOptions, fx.l.Base()+p, nil, map[string]string{"Origin": "null", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "content-type, x-sm-frame"})
		if res.StatusCode != http.StatusNoContent {
			t.Errorf("OPTIONS %s = %d", p, res.StatusCode)
		}
		h := res.Header
		if h.Get("Access-Control-Allow-Origin") != "*" || h.Get("Access-Control-Allow-Methods") != "GET, POST" ||
			h.Get("Access-Control-Allow-Headers") != "Content-Type, X-SM-Frame" || h.Get("Access-Control-Max-Age") != "600" {
			t.Errorf("OPTIONS %s headers = %v", p, h)
		}
	}
	// A preflight outside the base is still a 404 without CORS headers.
	res, _ := fx.do(t, http.MethodOptions, fx.l.Origin()+"/frame", nil, nil)
	if res.StatusCode != http.StatusNotFound || res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("OPTIONS outside the base = %d %v", res.StatusCode, res.Header)
	}
}

func TestListenerReportFrameAndErrorEndpoints(t *testing.T) {
	fx := newListenerFixture(t)
	base := fx.l.Base()

	if res, _ := fx.do(t, http.MethodGet, base+"/report", nil, nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET report = %d", res.StatusCode)
	}
	res, body := fx.do(t, http.MethodPost, base+"/report", []byte(`{"frame_count":3}`), map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"start":true`) {
		t.Errorf("report = %d %s", res.StatusCode, body)
	}
	fx.mu.Lock()
	gotReport := string(fx.report)
	fx.mu.Unlock()
	if gotReport != `{"frame_count":3}` {
		t.Errorf("report body = %q", gotReport)
	}
	big := bytes.Repeat([]byte("x"), maxReportBytes+1)
	if res, _ := fx.do(t, http.MethodPost, base+"/report", big, nil); res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized report = %d", res.StatusCode)
	}

	// Frames: the header is required, the hook sees content type and bytes, and the answer is JSON.
	if res, _ := fx.do(t, http.MethodPost, base+"/frame", []byte("f"), map[string]string{"Content-Type": "image/png"}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("frame without header = %d", res.StatusCode)
	}
	res, body = fx.do(t, http.MethodPost, base+"/frame", []byte("frame-0"), map[string]string{"Content-Type": "image/png", "X-SM-Frame": "0"})
	if res.StatusCode != http.StatusOK || string(body) != `{"ok":true}` {
		t.Errorf("frame 0 = %d %s", res.StatusCode, body)
	}
	res, body = fx.do(t, http.MethodPost, base+"/frame", []byte("frame-2"), map[string]string{"Content-Type": "image/png", "X-SM-Frame": "2"})
	if !strings.Contains(string(body), `"done":true`) {
		t.Errorf("last frame answer = %s", body)
	}
	fx.mu.Lock()
	if len(fx.frames) != 2 || fx.frames[0] != "image/png|frame-0" {
		t.Errorf("frames seen = %v", fx.frames)
	}
	fx.mu.Unlock()
	tooBig := bytes.Repeat([]byte("x"), int(MaxFrameBytes(IngestConfig{Width: 1920, Height: 1080, Transport: TransportPNG}))+1)
	res, body = fx.do(t, http.MethodPost, base+"/frame", tooBig, map[string]string{"Content-Type": "image/png", "X-SM-Frame": "5"})
	if res.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), `"stop":true`) {
		t.Errorf("oversized frame = %d %s", res.StatusCode, body)
	}

	res, body = fx.do(t, http.MethodPost, base+"/error", []byte(`{"code":"code_error","message":"scene s3 threw","scene_id":"s3"}`), map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusOK {
		t.Errorf("error endpoint = %d %s", res.StatusCode, body)
	}
	fx.do(t, http.MethodPost, base+"/error", []byte(`not json`), nil)
	fx.mu.Lock()
	defer fx.mu.Unlock()
	if len(fx.errors) < 3 {
		t.Fatalf("errors seen = %+v", fx.errors)
	}
	// The oversized frame and the missing header were reported as failures too.
	if fx.errors[len(fx.errors)-2] != (PageError{Code: "code_error", Message: "scene s3 threw", SceneID: "s3"}) {
		t.Errorf("page error = %+v", fx.errors[len(fx.errors)-2])
	}
	if last := fx.errors[len(fx.errors)-1]; last.Code != "render_failed" {
		t.Errorf("unreadable error body mapped to %+v", last)
	}
}

func TestStartListenerRejectsWeakPrefixes(t *testing.T) {
	for _, prefix := range []string{"", "short", strings.Repeat("a/", 20), strings.Repeat("a", 31)} {
		if l, err := StartListener(prefix, &Snapshot{}, PageParams{}, Hooks{}); err == nil {
			_ = l.Close()
			t.Errorf("prefix %q accepted", prefix)
		}
	}
}

func TestListenerCloseStopsServing(t *testing.T) {
	fx := newListenerFixture(t)
	url := fx.l.Base() + "/boot.json"
	if res, _ := fx.do(t, http.MethodGet, url, nil, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	_ = fx.l.Close()
	if _, err := fx.client.Get(url); err == nil {
		t.Fatal("the listener still answers after Close")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestListener|TestStartListener" -count=1`

Expected: FAIL (build failed: `undefined: Listener`, `undefined: PageError`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/listener.go`:

```go
package render

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxReportBytes = 1 << 20
	maxErrorBytes  = 16 << 10
)

// SnapshotFont is one catalog font face as the render page receives it.
type SnapshotFont struct {
	Name   string
	Family string
	Weight int
	// File is the catalog path ("/fonts/Geist-400.woff2"); the listener serves it below <base>/fonts/.
	File string
	Data []byte
}

// Snapshot is everything the render page may read, copied when the job starts so edits during a
// render change nothing.
type Snapshot struct {
	Story []byte
	// CatalogFiles holds catalog.json and every file it names, keyed by the path below
	// ui/img/storymaker/ ("catalog.json", "rig/human.json", "props/lock.json", ...).
	CatalogFiles map[string][]byte
	Fonts        []SnapshotFont
	CodeBundles  map[string][]byte
	HostBundle   []byte
	PlayerBundle []byte
}

// PageParams are the values the boot document carries.
type PageParams struct {
	Width, Height int
	FPS           int
	Transport     string
	FrameLimit    int
	// FrameDeadlineMS is the per-frame render deadline the page may use; a render is not real time,
	// so it is far above the 250 ms of the app preview. A page that does not know the field keeps its
	// own deadline.
	FrameDeadlineMS int
}

// ReportAnswer is the reply to POST /report.
type ReportAnswer struct {
	Start     bool   `json:"start"`
	FromFrame int    `json:"from_frame,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

// FrameAnswer is the reply to POST /frame.
type FrameAnswer struct {
	OK      bool   `json:"ok,omitempty"`
	Done    bool   `json:"done,omitempty"`
	Restart bool   `json:"restart,omitempty"`
	Stop    bool   `json:"stop,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// PageError is a failure the page reports with POST /error.
type PageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	SceneID string `json:"scene_id,omitempty"`
}

// Hooks connect the listener to the render job.
type Hooks struct {
	// Report receives the compile report JSON.
	Report func(body []byte) ReportAnswer
	// Frame receives one frame; status is the HTTP status of the answer.
	Frame func(n int, contentType string, data []byte) (answer FrameAnswer, status int)
	// Error receives a page failure.
	Error func(e PageError)
}

// Listener is the per-job loopback HTTP server the headless browser talks to. It binds
// 127.0.0.1:0, speaks plain HTTP with its own timeouts, writes no access log and serves nothing
// outside <origin>/<prefix>/.
type Listener struct {
	prefix string
	origin string
	snap   *Snapshot
	params PageParams
	hooks  Hooks
	ln     net.Listener
	srv    *http.Server
	fonts  map[string]*SnapshotFont
	maxFrm int64

	statsMu sync.Mutex
	stats   map[string]int64
}

// NewPrefix returns a fresh 128-bit render prefix (32 hex characters).
func NewPrefix() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// StartListener binds a loopback port and starts serving.
func StartListener(prefix string, snap *Snapshot, params PageParams, hooks Hooks) (*Listener, error) {
	if len(prefix) < 32 || strings.ContainsAny(prefix, "/?#%") {
		return nil, errors.New("storymaker: render prefix must be 32 or more safe characters")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("storymaker: render listener: %w", err)
	}
	l := &Listener{
		prefix: prefix, origin: "http://" + ln.Addr().String(), snap: snap, params: params, hooks: hooks, ln: ln,
		fonts: map[string]*SnapshotFont{}, stats: map[string]int64{},
		maxFrm: MaxFrameBytes(IngestConfig{Width: params.Width, Height: params.Height, Transport: params.Transport}),
	}
	for i := range snap.Fonts {
		l.fonts[snap.Fonts[i].File] = &snap.Fonts[i]
	}
	l.srv = &http.Server{
		Handler:           http.HandlerFunc(l.serve),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          log.New(io.Discard, "", 0), // never log: error texts can carry request paths
	}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

// Origin is http://127.0.0.1:<port>.
func (l *Listener) Origin() string { return l.origin }

// Base is the render base: origin plus the secret prefix, without a trailing slash.
func (l *Listener) Base() string { return l.origin + "/" + l.prefix }

// Close stops the listener and drops its connections.
func (l *Listener) Close() error { return l.srv.Close() }

// RenderCSP is the render page's Content-Security-Policy (spec "Rendering and export", step 3).
func RenderCSP(base string) string {
	return "sandbox allow-scripts; default-src 'none'; script-src " + base + "/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src " + base + "/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"
}

// RenderPage is the render page markup (contract section 6).
func RenderPage(base string, width, height int) string {
	b := html.EscapeString(base)
	return `<!doctype html><html data-sm-mode="render" data-sm-base="` + b + `"><head><meta charset="utf-8"></head>` +
		`<body><canvas id="sm-stage" width="` + strconv.Itoa(width) + `" height="` + strconv.Itoa(height) + `"></canvas>` +
		`<script src="` + b + `/js/storymaker/host.bundle.js"></script></body></html>`
}

func (l *Listener) serve(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}()
	// A DNS-rebinding page would reach this port under another Host header.
	if r.Host != strings.TrimPrefix(l.origin, "http://") {
		notFound(w)
		return
	}
	root := "/" + l.prefix + "/"
	if !strings.HasPrefix(r.URL.Path, root) {
		notFound(w)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, root)
	if rest != "" && path.Clean("/" + rest)[1:] != rest {
		notFound(w)
		return
	}
	l.count(r.Method, rest)
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	if r.Method == http.MethodOptions {
		h.Set("Access-Control-Allow-Methods", "GET, POST")
		h.Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
		h.Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch {
	case rest == "":
		l.page(w, r)
	case rest == "boot.json":
		l.boot(w, r)
	case rest == "story.json":
		serveBytes(w, r, "application/json; charset=utf-8", l.snap.Story)
	case strings.HasPrefix(rest, "catalog/"):
		l.catalogFile(w, r, strings.TrimPrefix(rest, "catalog/"))
	case strings.HasPrefix(rest, "fonts/"):
		l.font(w, r, "/"+rest)
	case strings.HasPrefix(rest, "scenes/") && strings.HasSuffix(rest, ".js"):
		id := strings.TrimSuffix(strings.TrimPrefix(rest, "scenes/"), ".js")
		if data, ok := l.snap.CodeBundles[id]; ok {
			serveBytes(w, r, "text/javascript; charset=utf-8", data)
			return
		}
		notFound(w)
	case rest == "js/storymaker/host.bundle.js":
		serveBytes(w, r, "text/javascript; charset=utf-8", l.snap.HostBundle)
	case rest == "js/storymaker/player.bundle.js":
		serveBytes(w, r, "text/javascript; charset=utf-8", l.snap.PlayerBundle)
	case rest == "report":
		l.report(w, r)
	case rest == "frame":
		l.frame(w, r)
	case rest == "error":
		l.pageError(w, r)
	default:
		notFound(w)
	}
}

// count records one request under "METHOD route" (never the path itself).
func (l *Listener) count(method, rest string) {
	route := "static"
	switch rest {
	case "":
		route = "page"
	case "report", "frame", "error", "boot.json":
		route = rest
	}
	l.statsMu.Lock()
	l.stats[method+" "+route]++
	l.statsMu.Unlock()
}

// Counts returns how many requests each "METHOD route" received, for diagnostics.
func (l *Listener) Counts() map[string]int64 {
	l.statsMu.Lock()
	defer l.statsMu.Unlock()
	out := make(map[string]int64, len(l.stats))
	for k, v := range l.stats {
		out[k] = v
	}
	return out
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func serveBytes(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD, OPTIONS")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (l *Listener) page(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", RenderCSP(l.Base()))
	serveBytes(w, r, "text/html; charset=utf-8", []byte(RenderPage(l.Base(), l.params.Width, l.params.Height)))
}

type bootFont struct {
	Name   string `json:"name"`
	Family string `json:"family"`
	Weight int    `json:"weight"`
	URL    string `json:"url"`
}

type bootDoc struct {
	Mode            string            `json:"mode"`
	FPS             int               `json:"fps"`
	Width           int               `json:"width"`
	Height          int               `json:"height"`
	Transport       string            `json:"transport"`
	FrameLimit      int               `json:"frame_limit"`
	FrameDeadlineMS int               `json:"frame_deadline_ms,omitempty"`
	StoryURL        string            `json:"story_url"`
	CatalogURL      string            `json:"catalog_url"`
	AssetsBase      string            `json:"assets_base"`
	CodeBundles     map[string]string `json:"code_bundles"`
	Fonts           []bootFont        `json:"fonts"`
	AudioInfo       map[string]any    `json:"audio_info"`
}

func (l *Listener) boot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD, OPTIONS")
		return
	}
	base := l.Base()
	doc := bootDoc{
		Mode: "render", FPS: l.params.FPS, Width: l.params.Width, Height: l.params.Height, Transport: l.params.Transport,
		FrameLimit: l.params.FrameLimit, FrameDeadlineMS: l.params.FrameDeadlineMS, StoryURL: base + "/story.json", CatalogURL: base + "/catalog/catalog.json",
		AssetsBase: base + "/catalog/", CodeBundles: map[string]string{}, Fonts: []bootFont{}, AudioInfo: map[string]any{},
	}
	ids := make([]string, 0, len(l.snap.CodeBundles))
	for id := range l.snap.CodeBundles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		doc.CodeBundles[id] = base + "/scenes/" + url.PathEscape(id) + ".js"
	}
	for _, f := range l.snap.Fonts {
		doc.Fonts = append(doc.Fonts, bootFont{Name: f.Name, Family: f.Family, Weight: f.Weight, URL: base + f.File})
	}
	writeJSON(w, http.StatusOK, doc)
}

func (l *Listener) catalogFile(w http.ResponseWriter, r *http.Request, rel string) {
	data, ok := l.snap.CatalogFiles[rel]
	if !ok {
		notFound(w)
		return
	}
	serveBytes(w, r, "application/json; charset=utf-8", data)
}

func (l *Listener) font(w http.ResponseWriter, r *http.Request, file string) {
	f, ok := l.fonts[file]
	if !ok {
		notFound(w)
		return
	}
	serveBytes(w, r, "font/woff2", f.Data)
}

func (l *Listener) report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST, OPTIONS")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxReportBytes))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, ReportAnswer{Error: "report_too_large", Message: "The compile report is larger than 1 MiB."})
		return
	}
	if l.hooks.Report == nil {
		notFound(w)
		return
	}
	writeJSON(w, http.StatusOK, l.hooks.Report(body))
}

func (l *Listener) frame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST, OPTIONS")
		return
	}
	// The frame number is the X-SM-Frame header or, for a CORS-simple request that needs no preflight,
	// the n query parameter.
	number := r.Header.Get("X-SM-Frame")
	if number == "" {
		number = r.URL.Query().Get("n")
	}
	n, err := strconv.Atoi(number)
	if err != nil || n < 0 {
		writeJSON(w, http.StatusBadRequest, FrameAnswer{Stop: true, Error: "frame_header", Message: "A frame needs its number in X-SM-Frame or ?n=."})
		l.fail("frame_header", "A frame arrived without a valid frame number.")
		return
	}
	if l.hooks.Frame == nil {
		notFound(w)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, l.maxFrm))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, FrameAnswer{Stop: true, Error: "frame_too_large", Message: fmt.Sprintf("A frame is larger than %d bytes.", l.maxFrm)})
			l.fail("frame_too_large", fmt.Sprintf("Frame %d is larger than %d bytes.", n, l.maxFrm))
			return
		}
		writeJSON(w, http.StatusBadRequest, FrameAnswer{Stop: true, Error: "frame_read", Message: "The frame could not be read."})
		l.fail("frame_read", "A frame could not be read.")
		return
	}
	answer, status := l.hooks.Frame(n, r.Header.Get("Content-Type"), data)
	writeJSON(w, status, answer)
}

// fail reports an ingest violation that the frame hook never saw.
func (l *Listener) fail(code, message string) {
	if l.hooks.Error != nil {
		l.hooks.Error(PageError{Code: code, Message: message})
	}
}

func (l *Listener) pageError(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST, OPTIONS")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxErrorBytes))
	var e PageError
	if err != nil || json.Unmarshal(body, &e) != nil || e.Code == "" {
		e = PageError{Code: "render_failed", Message: "The page reported an unreadable error."}
	}
	e.Code, e.Message, e.SceneID = clip(e.Code, 40), clip(e.Message, 600), clip(e.SceneID, 60)
	if l.hooks.Error != nil {
		l.hooks.Error(e)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/listener.go internal/storymaker/render/listener_test.go
git commit -m "feat(storymaker): per-job loopback render listener" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B8: Report validation, egress guard and browser session

**Files:**
- Create: `internal/storymaker/render/report.go`
- Create: `internal/storymaker/render/egress.go`
- Create: `internal/storymaker/render/session.go`
- Test: `internal/storymaker/render/report_test.go`

`parseReport` validates the compile report with numbers Go knows independently (fps, canvas, catalog version, frame limit, scene ids and monotonic ranges, `frame_count` equal to the last scene end) and refuses any `error` issue. `egressGuard` is the CDP half of the egress control; `session.go` launches Chrome with the strict proxy (`--proxy-bypass-list=<-loopback>`), sizes the tab to the canvas at device pixel ratio 1 and installs the guard before navigation. Browser behaviour is proven in B10; the unit tests cover the decisions.

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/report_test.go`:

```go
package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func goodReport() map[string]any {
	return map[string]any{
		"frame_count": 300, "fps": 30, "width": 1920, "height": 1080, "duration_ms": 10000, "catalog_version": "70d5cf51a9a9",
		"scenes": []any{
			map[string]any{"id": "s1", "start_frame": 0, "end_frame": 130, "kind": "storyboard"},
			map[string]any{"id": "s2", "start_frame": 115, "end_frame": 210, "kind": "storyboard"},
			map[string]any{"id": "s3", "start_frame": 195, "end_frame": 300, "kind": "code"},
		},
		"audio_events": []any{}, "captions": []any{}, "checks": []any{}, "unverified": []any{},
		"issues": []any{map[string]any{"severity": "warning", "path": "/scenes/0/narration", "code": "narration_missing", "message": "estimated"}},
	}
}

func reportExpectation() reportExpect {
	return reportExpect{FPS: 30, Width: 1920, Height: 1080, MaxFrames: 27000, CatalogVersion: "70d5cf51a9a9", SceneIDs: []string{"s1", "s2", "s3"}}
}

func TestParseReportAcceptsAConsistentReport(t *testing.T) {
	body, _ := json.Marshal(goodReport())
	rep, refusal := parseReport(body, reportExpectation())
	if refusal != nil || rep.FrameCount != 300 || len(rep.Scenes) != 3 || len(rep.Issues) != 1 {
		t.Fatalf("parseReport = %+v, %v", rep, refusal)
	}
	// Unknown fields are ignored.
	m := goodReport()
	m["future_field"] = map[string]any{"x": 1}
	body, _ = json.Marshal(m)
	if _, refusal := parseReport(body, reportExpectation()); refusal != nil {
		t.Fatalf("unknown field refused: %v", refusal)
	}
}

func TestParseReportRefusals(t *testing.T) {
	scenes := func(m map[string]any) []any { return m["scenes"].([]any) }
	scene := func(m map[string]any, i int) map[string]any { return scenes(m)[i].(map[string]any) }
	cases := map[string]struct {
		mut  func(m map[string]any)
		code string
		text string
	}{
		"fps":                {func(m map[string]any) { m["fps"] = 24 }, CodeRenderFailed, "24 fps"},
		"width":              {func(m map[string]any) { m["width"] = 1280 }, CodeRenderFailed, "1280x1080"},
		"catalog version":    {func(m map[string]any) { m["catalog_version"] = "other" }, CodeRenderFailed, "catalog version"},
		"too many frames":    {func(m map[string]any) { m["frame_count"] = 27001; scene(m, 2)["end_frame"] = 27001 }, CodeInvalidStory, "27001 frames"},
		"no frames":          {func(m map[string]any) { m["frame_count"] = 0 }, CodeRenderFailed, "no frames"},
		"scene count":        {func(m map[string]any) { m["scenes"] = scenes(m)[:2] }, CodeRenderFailed, "2 scenes"},
		"scene order":        {func(m map[string]any) { scene(m, 0)["id"], scene(m, 1)["id"] = "s2", "s1" }, CodeRenderFailed, `scene 0 is "s2"`},
		"empty range":        {func(m map[string]any) { scene(m, 1)["end_frame"] = 115 }, CodeRenderFailed, "invalid range"},
		"past the end":       {func(m map[string]any) { scene(m, 1)["end_frame"] = 301 }, CodeRenderFailed, "invalid range"},
		"negative start":     {func(m map[string]any) { scene(m, 0)["start_frame"] = -1 }, CodeRenderFailed, "invalid range"},
		"start not forward":  {func(m map[string]any) { scene(m, 1)["start_frame"] = 0 }, CodeRenderFailed, "not monotonic"},
		"frame count vs end": {func(m map[string]any) { m["frame_count"] = 310 }, CodeRenderFailed, "last scene end"},
		"error issue": {func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s2", "code": "unknown_asset", "message": "no such prop"}}
		}, CodeInvalidStory, "Scene s2: no such prop"},
		"code error": {func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s3", "code": "deadline", "message": "scene took too long"}}
		}, CodeCodeError, "too long"},
	}
	for name, tc := range cases {
		m := goodReport()
		tc.mut(m)
		body, _ := json.Marshal(m)
		_, refusal := parseReport(body, reportExpectation())
		if refusal == nil || refusal.Code != tc.code || !strings.Contains(refusal.Message, tc.text) {
			t.Errorf("%s: refusal = %+v", name, refusal)
		}
	}
	// An end that goes back behind the previous scene's end is not monotonic; equal ends are fine.
	m := goodReport()
	scene(m, 0)["end_frame"] = 220
	scene(m, 1)["end_frame"] = 210
	body, _ := json.Marshal(m)
	if _, refusal := parseReport(body, reportExpectation()); refusal == nil || !strings.Contains(refusal.Message, "not monotonic") {
		t.Errorf("end going back: %v", refusal)
	}
	if _, refusal := parseReport([]byte(`{"frame_count":`), reportExpectation()); refusal == nil || refusal.Code != CodeRenderFailed {
		t.Errorf("broken JSON: %v", refusal)
	}
}

func TestEgressGuardDecisions(t *testing.T) {
	g := newEgressGuard("http://127.0.0.1:41000/0123456789abcdef0123456789abcdef")
	base := "http://127.0.0.1:41000/0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		url      string
		resource proto.NetworkResourceType
		want     bool
	}{
		{base + "/", proto.NetworkResourceTypeDocument, true},
		{base + "/boot.json", proto.NetworkResourceTypeFetch, true},
		{base + "/frame?n=3", proto.NetworkResourceTypeFetch, true},
		{base + "/js/storymaker/host.bundle.js", proto.NetworkResourceTypeScript, true},
		{"blob:null/6c1f", proto.NetworkResourceTypeScript, true},
		{"data:text/plain;base64,AAAA", proto.NetworkResourceTypeFetch, true},
		{"blob:null/6c1f", proto.NetworkResourceTypeDocument, false},
		{"data:text/html,<script>fetch(1)</script>", proto.NetworkResourceTypeDocument, false},
		{"http://127.0.0.1:41000/", proto.NetworkResourceTypeDocument, false},
		{"http://127.0.0.1:41000/favicon.ico", proto.NetworkResourceTypeImage, false},
		{"http://127.0.0.1:41001/0123456789abcdef0123456789abcdef/", proto.NetworkResourceTypeDocument, false},
		{"http://127.0.0.1:41000/0123456789abcdef0123456789abcdee/", proto.NetworkResourceTypeFetch, false},
		{base, proto.NetworkResourceTypeDocument, false}, // no trailing slash: not the base's page
		{"http://127.0.0.1:41000@evil.example/", proto.NetworkResourceTypeDocument, false},
		{"https://example.com/", proto.NetworkResourceTypeFetch, false},
		{"ws://127.0.0.1:41000/0123456789abcdef0123456789abcdef/x", proto.NetworkResourceTypeWebSocket, false},
		{"file:///etc/passwd", proto.NetworkResourceTypeDocument, false},
		{"", proto.NetworkResourceTypeFetch, false},
	} {
		if got := g.allowed(tc.url, tc.resource); got != tc.want {
			t.Errorf("allowed(%q, %s) = %v, want %v", tc.url, tc.resource, got, tc.want)
		}
	}
	g.noteBlocked("https://example.com/secret?token=1")
	g.noteBlocked("not a url at all, but long enough to be clipped somewhere around forty characters")
	if g.Blocked() != 2 || len(g.BlockedHosts()) != 2 || g.BlockedHosts()[0] != "https://example.com" || len(g.BlockedHosts()[1]) > 40 {
		t.Errorf("blocked = %d %v", g.Blocked(), g.BlockedHosts())
	}
	patterns := g.blockPatterns()
	if patterns[0]["urlPattern"] != base+"/*" || patterns[0]["block"] != false {
		t.Errorf("the allow rule must come first: %v", patterns[0])
	}
	for _, p := range patterns[1:] {
		if p["block"] != true {
			t.Errorf("%v does not block", p)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestParseReport|TestEgressGuard" -count=1`

Expected: FAIL (build failed: `undefined: reportExpect`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/report.go`:

```go
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/storymaker"
)

// compileReport is the part of the page's compile report Go uses (spec "Compile report and job
// order"); unknown fields are ignored.
type compileReport struct {
	FrameCount     int                `json:"frame_count"`
	FPS            int                `json:"fps"`
	Width          int                `json:"width"`
	Height         int                `json:"height"`
	DurationMS     float64            `json:"duration_ms"`
	CatalogVersion string             `json:"catalog_version"`
	Scenes         []reportScene      `json:"scenes"`
	Issues         []storymaker.Issue `json:"issues"`
}

type reportScene struct {
	ID         string `json:"id"`
	StartFrame int    `json:"start_frame"`
	EndFrame   int    `json:"end_frame"`
	Kind       string `json:"kind"`
}

// reportRefusal is a report Go will not render: the code is a job error code, the message is shown
// to the user and to the model.
type reportRefusal struct {
	Code    string
	Message string
}

func (r *reportRefusal) Error() string { return r.Code + ": " + r.Message }

// reportExpect is what Go knows independently of the page.
type reportExpect struct {
	FPS            int
	Width, Height  int
	MaxFrames      int
	CatalogVersion string
	SceneIDs       []string
}

// parseReport decodes and validates a compile report: the numbers must agree with the job (fps,
// canvas, catalog version), frame_count must not exceed the Go-computed limit, the scene ranges must
// be monotonic and cover frame_count exactly, and an error issue refuses the render.
func parseReport(body []byte, want reportExpect) (compileReport, *reportRefusal) {
	var rep compileReport
	if err := json.Unmarshal(body, &rep); err != nil {
		return rep, &reportRefusal{CodeRenderFailed, "The compile report is not valid JSON."}
	}
	bad := func(format string, args ...any) (compileReport, *reportRefusal) {
		return rep, &reportRefusal{CodeRenderFailed, "The compile report was refused: " + fmt.Sprintf(format, args...)}
	}
	if rep.FPS != want.FPS || rep.Width != want.Width || rep.Height != want.Height {
		return bad("it describes %dx%d at %d fps, the job is %dx%d at %d fps", rep.Width, rep.Height, rep.FPS, want.Width, want.Height, want.FPS)
	}
	if rep.CatalogVersion != want.CatalogVersion {
		return bad("the catalog version %q differs from %q", rep.CatalogVersion, want.CatalogVersion)
	}
	for _, is := range rep.Issues {
		if is.Severity == storymaker.SeverityError {
			code := CodeInvalidStory
			if is.Code == storymaker.CodeCodeError || is.Code == storymaker.CodeDeadline {
				code = CodeCodeError
			}
			msg := is.Message
			if is.SceneID != "" {
				msg = "Scene " + is.SceneID + ": " + msg
			}
			return rep, &reportRefusal{code, clip(strings.TrimSpace(msg), 400)}
		}
	}
	if rep.FrameCount < 1 {
		return bad("the story has no frames")
	}
	if rep.FrameCount > want.MaxFrames {
		return rep, &reportRefusal{CodeInvalidStory, fmt.Sprintf("The video would have %d frames; the limit is %d (render.max_duration_seconds).", rep.FrameCount, want.MaxFrames)}
	}
	if len(rep.Scenes) != len(want.SceneIDs) {
		return bad("it lists %d scenes, the story has %d", len(rep.Scenes), len(want.SceneIDs))
	}
	maxEnd := 0
	for i, sc := range rep.Scenes {
		if sc.ID != want.SceneIDs[i] {
			return bad("scene %d is %q, the story has %q", i, sc.ID, want.SceneIDs[i])
		}
		if sc.StartFrame < 0 || sc.EndFrame <= sc.StartFrame || sc.EndFrame > rep.FrameCount {
			return bad("scene %q has the invalid range %d to %d", sc.ID, sc.StartFrame, sc.EndFrame)
		}
		if i > 0 {
			prev := rep.Scenes[i-1]
			// Transitions overlap scenes, so a start may lie before the previous end, but the ranges
			// still move forward.
			if sc.StartFrame <= prev.StartFrame || sc.EndFrame < prev.EndFrame {
				return bad("the scene ranges are not monotonic at %q", sc.ID)
			}
		}
		if sc.EndFrame > maxEnd {
			maxEnd = sc.EndFrame
		}
	}
	if maxEnd != rep.FrameCount {
		return bad("frame_count %d is not the last scene end %d", rep.FrameCount, maxEnd)
	}
	return rep, nil
}
```

`internal/storymaker/render/egress.go`:

```go
package render

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// egressGuard is the CDP half of the render page's egress control (spec step 2, control 3). Together
// with the page's CSP and the strict capture proxy (controls 1 and 2) it forms three independent
// layers; each one alone keeps the page on the render base.
//
// Measured on Chrome 156 (plan task B10, notes "Egress"):
//   - Fetch.enable is not available on a dedicated worker's own CDP session ("'Fetch.enable' wasn't
//     found"), but the page session's interception also pauses the requests dedicated workers make
//     (fetch, XMLHttpRequest, importScripts).
//   - Pausing a request that carries a frame costs about 35 ms (the browser hands the whole body to
//     the DevTools agent): the catch-all interception halves the rendering rate (20 fps to 10 fps).
//     Enabling Fetch without pausing anything costs nothing.
//   - Network.setBlockedURLs with urlPatterns blocks requests natively, on the page session and on
//     worker sessions, without pausing them. It does not cover top-level navigations.
//
// The default (hybrid) mode therefore pauses only navigations (Fetch, resource type Document: a few
// requests per job) and blocks everything else natively on the page and on every worker target that
// auto-attaches (held until its block list is installed). When the browser lacks block patterns
// (Chrome before 126) the guard falls back to the catch-all interception, which is slower and covers
// the same requests. WebSocket handshakes are outside both mechanisms; the CSP and the proxy stop
// them. A popup opened by window.open is another target; the sandbox keeps it from opening (no
// allow-popups) and the guard closes any extra page target as a second barrier.
type egressGuard struct {
	base string // http://127.0.0.1:<port>/<prefix>, no trailing slash
	// full forces the catch-all interception (tests compare it).
	full bool
	// mode records the mechanism in use: "hybrid" or "fetch".
	mode atomic.Value

	blocked      atomic.Int64
	seen         atomic.Int64 // requests the guard paused and decided
	extra        atomic.Int64 // extra page targets that were closed
	workers      atomic.Int64 // worker targets guarded
	workerFailed atomic.Int64 // worker targets that could not be guarded and were closed
	mu           sync.Mutex
	hosts        []string // scheme://host of the first blocked requests, never the render base
}

func newEgressGuard(base string) *egressGuard { return &egressGuard{base: base} }

// allowed decides one paused request. Navigations (documents and frames) must lie below the base;
// other requests may also be blob: or data: URLs, which never leave the browser.
func (g *egressGuard) allowed(rawURL string, resource proto.NetworkResourceType) bool {
	if strings.HasPrefix(rawURL, g.base+"/") {
		return true
	}
	if resource == proto.NetworkResourceTypeDocument {
		return false
	}
	return strings.HasPrefix(rawURL, "blob:") || strings.HasPrefix(rawURL, "data:")
}

// Mode returns "hybrid" or "fetch" once the guard is installed.
func (g *egressGuard) Mode() string {
	m, _ := g.mode.Load().(string)
	return m
}

// Blocked is the number of paused requests the guard failed.
func (g *egressGuard) Blocked() int64 { return g.blocked.Load() }

// Seen is the number of requests the guard paused.
func (g *egressGuard) Seen() int64 { return g.seen.Load() }

// BlockedHosts returns the scheme://host of the first blocked requests.
func (g *egressGuard) BlockedHosts() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.hosts...)
}

func (g *egressGuard) noteBlocked(rawURL string) {
	g.blocked.Add(1)
	host := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		host = u.Scheme + "://" + u.Host
	} else if len(host) > 40 {
		host = host[:40]
	}
	g.mu.Lock()
	if len(g.hosts) < 16 {
		g.hosts = append(g.hosts, host)
	}
	g.mu.Unlock()
}

// pausedLite reads only what the decision needs from Fetch.requestPaused. The full event carries
// the request body (a frame is up to several hundred kilobytes), which the guard never looks at.
type pausedLite struct {
	RequestID    proto.FetchRequestID      `json:"requestId"`
	ResourceType proto.NetworkResourceType `json:"resourceType"`
	Request      struct {
		URL string `json:"url"`
	} `json:"request"`
}

func (pausedLite) ProtoEvent() string { return "Fetch.requestPaused" }

// sessionClient sends CDP commands on one target session of the browser connection.
type sessionClient struct {
	b   *rod.Browser
	sid proto.TargetSessionID
	ctx context.Context
}

func (c sessionClient) Call(ctx context.Context, _ string, method string, params interface{}) ([]byte, error) {
	return c.b.Call(ctx, string(c.sid), method, params)
}
func (c sessionClient) GetSessionID() proto.TargetSessionID { return c.sid }
func (c sessionClient) GetContext() context.Context         { return c.ctx }

var (
	interceptCatchAll  = []*proto.FetchRequestPattern{{URLPattern: "*", RequestStage: proto.FetchRequestStageRequest}}
	interceptDocuments = []*proto.FetchRequestPattern{{URLPattern: "*", ResourceType: proto.NetworkResourceTypeDocument, RequestStage: proto.FetchRequestStageRequest}}
)

// blockPatterns is the native block list: the render base is allowed, every other http(s) and ws(s)
// URL is blocked. blob: and data: URLs match none of the patterns.
func (g *egressGuard) blockPatterns() []map[string]any {
	return []map[string]any{
		{"urlPattern": g.base + "/*", "block": false},
		{"urlPattern": "http://*:*/*", "block": true},
		{"urlPattern": "https://*:*/*", "block": true},
		{"urlPattern": "ws://*:*/*", "block": true},
		{"urlPattern": "wss://*:*/*", "block": true},
	}
}

// blockOutside installs the native block list on one target session.
func (g *egressGuard) blockOutside(c sessionClient) error {
	if err := (proto.NetworkEnable{}).Call(c); err != nil {
		return err
	}
	_, err := c.Call(c.ctx, "", "Network.setBlockedURLs", map[string]any{"urlPatterns": g.blockPatterns()})
	return err
}

// install guards the page and starts the decision loop. It must run before the page navigates.
func (g *egressGuard) install(ctx context.Context, browser *rod.Browser, page *rod.Page) error {
	b := browser.Context(ctx)
	events := b.Event()
	hybrid := false
	if !g.full {
		hybrid = g.blockOutside(sessionClient{b: b, sid: page.SessionID, ctx: ctx}) == nil
	}
	patterns, mode := interceptCatchAll, "fetch"
	if hybrid {
		patterns, mode = interceptDocuments, "hybrid"
	}
	g.mode.Store(mode)
	if err := (proto.FetchEnable{Patterns: patterns}).Call(page); err != nil {
		return err
	}
	if hybrid {
		if err := (proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}).Call(page); err != nil {
			return err
		}
	}
	if err := (proto.TargetSetDiscoverTargets{Discover: true}).Call(b); err != nil {
		return err
	}
	ours := page.TargetID
	paused := (pausedLite{}).ProtoEvent()
	created := (proto.TargetTargetCreated{}).ProtoEvent()
	attached := (proto.TargetAttachedToTarget{}).ProtoEvent()
	go func() {
		for msg := range events {
			switch msg.Method {
			case paused:
				var ev pausedLite
				if !msg.Load(&ev) {
					continue
				}
				client := sessionClient{b: b, sid: msg.SessionID, ctx: ctx}
				g.seen.Add(1)
				// Replies run off the loop so a slow command cannot hold the next event back.
				go func() {
					if g.allowed(ev.Request.URL, ev.ResourceType) {
						_ = (proto.FetchContinueRequest{RequestID: ev.RequestID}).Call(client)
						return
					}
					g.noteBlocked(ev.Request.URL)
					_ = (proto.FetchFailRequest{RequestID: ev.RequestID, ErrorReason: proto.NetworkErrorReasonBlockedByClient}).Call(client)
				}()
			case created:
				var ev proto.TargetTargetCreated
				if !msg.Load(&ev) || ev.TargetInfo == nil {
					continue
				}
				if ev.TargetInfo.Type == "page" && ev.TargetInfo.TargetID != ours {
					g.extra.Add(1)
					go func(id proto.TargetTargetID) { _, _ = (proto.TargetCloseTarget{TargetID: id}).Call(b) }(ev.TargetInfo.TargetID)
				}
			case attached:
				var ev proto.TargetAttachedToTarget
				if !msg.Load(&ev) {
					continue
				}
				go g.guardWorker(ctx, b, ev)
			}
		}
	}()
	return nil
}

// guardWorker installs the block list on a freshly attached worker (and on workers it starts) and
// lets it run. A target that cannot be guarded is closed.
func (g *egressGuard) guardWorker(ctx context.Context, b *rod.Browser, ev proto.TargetAttachedToTarget) {
	client := sessionClient{b: b, sid: ev.SessionID, ctx: ctx}
	err := g.blockOutside(client)
	if err == nil {
		err = (proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}).Call(client)
	}
	if err != nil {
		g.workerFailed.Add(1)
		if ev.TargetInfo != nil {
			_, _ = (proto.TargetCloseTarget{TargetID: ev.TargetInfo.TargetID}).Call(b)
		}
		return
	}
	g.workers.Add(1)
	if ev.WaitingForDebugger {
		_ = (proto.RuntimeRunIfWaitingForDebugger{}).Call(client)
	}
}
```

`internal/storymaker/render/session.go`:

```go
package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// sessionOptions describes one render browser session.
type sessionOptions struct {
	Bin     string
	Profile string
	// ProxyAddr is the strict capture proxy; empty runs without a proxy (the egress test of the CDP
	// guard on its own).
	ProxyAddr string
	// Base is the render base the egress guard lets through.
	Base          string
	Width, Height int
	// NoGuard skips the CDP egress guard. Only the tests use it, to show that the guard (and the proxy)
	// are what stop a leak and to measure what they cost.
	NoGuard bool
	// GuardFull forces the catch-all request interception instead of the hybrid guard (tests, and
	// browsers without block patterns).
	GuardFull bool
}

// browserPage is the part of a headless browser tab the render job uses; tests replace it with a
// stand-in that talks to the listener over HTTP.
type browserPage interface {
	// Navigate opens url; it returns once the navigation committed.
	Navigate(ctx context.Context, url string) error
	// Status returns window.__smStatus as JSON ("" when unavailable).
	Status(ctx context.Context) string
	// WaitReady waits for the screenshot transport's ready phase.
	WaitReady(ctx context.Context) error
	// Capture renders frame n through window.__smRenderFrame and returns a PNG screenshot.
	Capture(ctx context.Context, n int) ([]byte, error)
	// Crashed is closed when the tab crashed.
	Crashed() <-chan struct{}
	// Close stops the browser and removes its profile.
	Close()
}

// pageOpener starts the browser; tests replace it.
type pageOpener func(ctx context.Context, o sessionOptions) (browserPage, error)

// session is the production browserPage: a launched Chrome with the egress guard on its tab.
type session struct {
	l       *launcher.Launcher
	browser *rod.Browser
	page    *rod.Page
	guard   *egressGuard
	crashed chan struct{}
	opts    sessionOptions
}

func openSession(ctx context.Context, o sessionOptions) (browserPage, error) {
	s, err := openRodSession(ctx, o)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func openRodSession(ctx context.Context, o sessionOptions) (*session, error) {
	if err := os.MkdirAll(o.Profile, 0o700); err != nil {
		return nil, err
	}
	l := newLauncher(ctx, launchOptions{Bin: o.Bin, UserDataDir: o.Profile, ProxyAddr: o.ProxyAddr, WindowSize: [2]int{o.Width, o.Height}})
	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch the browser: %w", err)
	}
	s := &session{l: l, opts: o, guard: newEgressGuard(o.Base), crashed: make(chan struct{})}
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		s.closeProcess()
		return nil, fmt.Errorf("connect to the browser: %w", err)
	}
	s.browser = browser
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("open a tab: %w", err)
	}
	s.page = page
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: o.Width, Height: o.Height, DeviceScaleFactor: 1, Mobile: false}); err != nil {
		s.Close()
		return nil, fmt.Errorf("size the tab: %w", err)
	}
	s.guard.full = o.GuardFull
	if !o.NoGuard {
		if err := s.guard.install(ctx, browser, page); err != nil {
			s.Close()
			return nil, fmt.Errorf("install the egress guard: %w", err)
		}
	}
	go page.EachEvent(func(*proto.InspectorTargetCrashed) bool {
		select {
		case <-s.crashed:
		default:
			close(s.crashed)
		}
		return true
	})()
	return s, nil
}

func (s *session) Navigate(ctx context.Context, url string) error {
	return s.page.Context(ctx).Navigate(url)
}

func (s *session) Status(ctx context.Context) string {
	res, err := s.page.Timeout(3 * time.Second).Eval(`() => JSON.stringify(window.__smStatus || null)`)
	if err != nil {
		return ""
	}
	return res.Value.Str()
}

func (s *session) WaitReady(ctx context.Context) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		var status struct {
			Phase   string `json:"phase"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if raw := s.Status(ctx); raw != "" && raw != "null" {
			_ = json.Unmarshal([]byte(raw), &status)
		}
		switch status.Phase {
		case "ready":
			return nil
		case "error", "refused", "stopped":
			return fmt.Errorf("the page stopped (%s): %s", status.Error, status.Message)
		}
		if time.Now().After(deadline) {
			return errors.New("the page never became ready for screenshots")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
	}
}

func (s *session) Capture(ctx context.Context, n int) ([]byte, error) {
	p := s.page.Context(ctx).Timeout(30 * time.Second)
	if _, err := p.Eval(`(n) => window.__smRenderFrame(n)`, n); err != nil {
		return nil, err
	}
	return p.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip:   &proto.PageViewport{X: 0, Y: 0, Width: float64(s.opts.Width), Height: float64(s.opts.Height), Scale: 1},
	})
}

func (s *session) Crashed() <-chan struct{} { return s.crashed }

// Guard exposes the egress guard to the tests.
func (s *session) Guard() *egressGuard { return s.guard }

func (s *session) Close() {
	if s.browser != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.browser.Context(closeCtx).Close()
		cancel()
	}
	s.closeProcess()
}

func (s *session) closeProcess() {
	s.l.Kill()
	s.l.Cleanup()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/report.go internal/storymaker/render/egress.go internal/storymaker/render/session.go internal/storymaker/render/report_test.go
git commit -m "feat(storymaker): compile-report validation, CDP egress guard and browser session" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B9: Render job

**Files:**
- Create: `internal/storymaker/render/render.go`
- Test: `internal/storymaker/render/render_test.go`

`Runner.Prepare` reads the story, checks the `If-Match` etag, decodes, applies defaults, validates (including code-scene bundles) and snapshots story and bundles. `Runner.Run` resolves FFmpeg and the browser (downloading the pinned Chromium if needed), snapshots catalog, fonts and bundles, starts the listener and the strict proxy, launches Chrome, waits for the report, starts FFmpeg, feeds frames, restarts once with libx264 when a hardware encoder fails, kills the job when no frame arrives within 10 s (`code_error`), enforces `max(render.timeout_seconds, 3 x frames / measured fps)` and a 3 minute report wait, treats every page error as fatal, publishes `exports/<slug>-<job8>.mp4` create-only plus `exports/<job id>.json`, and removes Chrome, listener, FFmpeg and staging files on every path. The tests play the render page with a fake that talks to the real listener over HTTP, so everything but Chrome is real (FFmpeg included).

- [ ] **Step 1: Write the failing test**

`internal/storymaker/render/render_test.go`:

```go
package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/videostudio"
)

const uiDirForTests = "../../../ui"

// memPublisher is the desktop workspace of the tests: create-only streams and replaceable small files.
type memPublisher struct {
	mu    sync.Mutex
	files map[string][]byte
	fail  error
}

func newMemPublisher() *memPublisher { return &memPublisher{files: map[string][]byte{}} }

func (p *memPublisher) WriteStream(_ context.Context, path string, r io.Reader, max int64) error {
	if p.fail != nil {
		return p.fail
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.files[path]; exists {
		return errors.New("exists")
	}
	p.files[path] = data
	return nil
}

func (p *memPublisher) WriteBytes(_ context.Context, path string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.files[path] = append([]byte(nil), data...)
	return nil
}

func (p *memPublisher) get(path string) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, ok := p.files[path]
	return d, ok
}

func (p *memPublisher) names() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for k := range p.files {
		out = append(out, k)
	}
	return out
}

// fakePage plays the render page against the listener over plain HTTP.
type fakePage struct {
	script  func(ctx context.Context, base string)
	crashed chan struct{}
	closed  atomic.Bool
	capture func(n int) []byte
}

func (p *fakePage) Navigate(ctx context.Context, u string) error {
	go p.script(ctx, strings.TrimSuffix(u, "/"))
	return nil
}
func (p *fakePage) Status(context.Context) string   { return `{"phase":"fake"}` }
func (p *fakePage) WaitReady(context.Context) error { return nil }
func (p *fakePage) Crashed() <-chan struct{}        { return p.crashed }
func (p *fakePage) Close()                          { p.closed.Store(true) }
func (p *fakePage) Capture(_ context.Context, n int) ([]byte, error) {
	if p.capture == nil {
		return nil, errors.New("no capture")
	}
	return p.capture(n), nil
}

// playOptions steer the fake page.
type playOptions struct {
	frames      int
	frame       func(n int) (contentType string, body []byte)
	mutateRep   func(rep map[string]any)
	stallAfter  int // stop sending after this many frames (0 = never)
	errorAfter  int // POST /error after this many frames (0 = never)
	errorBody   string
	onRefused   func(answer ReportAnswer)
	onRestart   func()
	framesSent  *atomic.Int64
	skipReport  bool
	reportTwice bool
}

func httpDo(ctx context.Context, method, url, contentType string, header map[string]string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return data, res.StatusCode, nil
}

func play(t *testing.T, o playOptions) func(ctx context.Context, base string) {
	return func(ctx context.Context, base string) {
		bootBytes, status, err := httpDo(ctx, "GET", base+"/boot.json", "", nil, nil)
		if err != nil || status != 200 {
			return
		}
		var boot struct {
			FPS, Width, Height int
			Transport          string
		}
		_ = json.Unmarshal(bootBytes, &boot)
		var rawBoot map[string]any
		_ = json.Unmarshal(bootBytes, &rawBoot)
		catalogBytes, _, _ := httpDo(ctx, "GET", base+"/catalog/catalog.json", "", nil, nil)
		var cat struct {
			Version string `json:"version"`
		}
		_ = json.Unmarshal(catalogBytes, &cat)
		storyBytes, _, _ := httpDo(ctx, "GET", base+"/story.json", "", nil, nil)
		var story struct {
			Scenes []struct {
				ID string `json:"id"`
			} `json:"scenes"`
		}
		_ = json.Unmarshal(storyBytes, &story)
		// Scene i starts i frames in and every scene ends with the video, so the ranges move forward.
		scenes := []map[string]any{}
		for i := range story.Scenes {
			scenes = append(scenes, map[string]any{"id": story.Scenes[i].ID, "start_frame": i, "end_frame": o.frames, "kind": "storyboard"})
		}
		rep := map[string]any{"frame_count": o.frames, "fps": boot.FPS, "width": boot.Width, "height": boot.Height,
			"duration_ms": o.frames * 1000 / max(boot.FPS, 1), "catalog_version": cat.Version, "scenes": scenes,
			"audio_events": []any{}, "captions": []any{}, "checks": []any{}, "unverified": []any{}, "issues": []any{}}
		if o.mutateRep != nil {
			o.mutateRep(rep)
		}
		body, _ := json.Marshal(rep)
		if !o.skipReport {
			out, st, err := httpDo(ctx, "POST", base+"/report", "application/json", nil, body)
			if err != nil || st != 200 {
				return
			}
			var answer ReportAnswer
			_ = json.Unmarshal(out, &answer)
			if o.reportTwice {
				out2, _, _ := httpDo(ctx, "POST", base+"/report", "application/json", nil, body)
				var second ReportAnswer
				_ = json.Unmarshal(out2, &second)
				if second.Start {
					t.Error("a second report was accepted")
				}
			}
			if !answer.Start {
				if o.onRefused != nil {
					o.onRefused(answer)
				}
				return
			}
		}
		if boot.Transport == TransportScreenshot {
			<-ctx.Done() // the Go side drives the screenshots
			return
		}
		for n := 0; n < o.frames; {
			if o.stallAfter > 0 && n >= o.stallAfter {
				<-ctx.Done()
				return
			}
			if o.errorAfter > 0 && n >= o.errorAfter {
				httpDo(ctx, "POST", base+"/error", "application/json", nil, []byte(o.errorBody))
				<-ctx.Done()
				return
			}
			ct, data := o.frame(n)
			out, st, err := httpDo(ctx, "POST", base+"/frame", ct, map[string]string{"X-SM-Frame": strconv.Itoa(n)}, data)
			if err != nil {
				return
			}
			if o.framesSent != nil {
				o.framesSent.Add(1)
			}
			var a FrameAnswer
			_ = json.Unmarshal(out, &a)
			switch {
			case a.Stop || st >= 400 && !a.Restart:
				return
			case a.Restart:
				if o.onRestart != nil {
					o.onRestart()
				}
				n = 0
			case a.Done:
				return
			default:
				n++
			}
		}
	}
}

type runFixture struct {
	t         *testing.T
	dir       string
	deps      Deps
	runner    *Runner
	mgr       *Manager
	projects  *storymaker.ProjectStore
	pub       *memPublisher
	projectID string
	settings  Settings
	page      *fakePage
	pageSetup func(p *fakePage)
	pngFrame  []byte
}

func newRunFixture(t *testing.T) *runFixture {
	t.Helper()
	ffmpeg := testBinary(t, "ffmpeg")
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	fx := &runFixture{t: t, dir: t.TempDir(), pub: newMemPublisher()}
	fx.projects = storymaker.NewProjectStore(storymaker.NewMemStore(), ProjectsRoot)
	proj, _, _, err := fx.projects.Create(context.Background(), "Was ist eine Firewall?", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	fx.projectID = proj.ID
	lim := storymaker.DefaultLimits()
	fx.settings = Settings{TimeoutSeconds: 60, MaxDurationSeconds: 900, FFmpegPath: ffmpeg, EncoderName: "libx264", Limits: lim}
	b := NewBrowsers(fx.dir)
	b.lookPath = func() (string, bool) { return "/fake/chrome", true }
	b.launchProbe = func(context.Context, string) (string, error) { return "Fake/1", nil }
	fx.page = &fakePage{crashed: make(chan struct{})}
	fx.deps = Deps{
		Dir: fx.dir, UI: ui, Catalog: cat, Projects: fx.projects, Browsers: b, Publisher: fx.pub,
		Settings: func(context.Context) (Settings, error) { return fx.settings, nil },
		OpenPage: func(ctx context.Context, o sessionOptions) (browserPage, error) {
			if fx.pageSetup != nil {
				fx.pageSetup(fx.page)
			}
			if !strings.HasPrefix(o.ProxyAddr, "127.0.0.1:") || o.Width != 1920 || o.Height != 1080 || !strings.HasPrefix(o.Base, "http://127.0.0.1:") {
				t.Errorf("session options = %+v", o)
			}
			return fx.page, nil
		},
	}
	fx.runner = NewRunner(fx.deps)
	mgr, err := NewManager(ManagerOptions{Dir: fx.dir, Run: fx.runner.Run})
	if err != nil {
		t.Fatal(err)
	}
	fx.mgr = mgr
	t.Cleanup(mgr.Close)
	fx.pngFrame = solidPNG(t, 1920, 1080, color.NRGBA{R: 30, G: 90, B: 160, A: 255})
	return fx
}

func (fx *runFixture) playing(o playOptions) {
	if o.frames == 0 {
		o.frames = 24
	}
	if o.frame == nil {
		o.frame = func(int) (string, []byte) { return "image/png", fx.pngFrame }
	}
	fx.page.script = play(fx.t, o)
}

func (fx *runFixture) render(t *testing.T) Job {
	t.Helper()
	plan, err := fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	job, _, err := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	if err != nil {
		t.Fatal(err)
	}
	return waitJobFor(t, fx.mgr, job.ID, terminal, 8*time.Minute)
}

func TestRunRendersAndPublishesTheVideo(t *testing.T) {
	ffprobe := testBinary(t, "ffprobe")
	fx := newRunFixture(t)
	var sent atomic.Int64
	fx.playing(playOptions{frames: 24, framesSent: &sent})
	job := fx.render(t)
	if job.Status != StatusSucceeded || job.Output == nil || job.Error != nil {
		t.Fatalf("job = %+v (error %+v)", job, job.Error)
	}
	wantPath := "Documents/Storymaker/" + fx.projectID + "/exports/Was-ist-eine-Firewall-" + job.ID[:8] + ".mp4"
	if job.Output.Path != wantPath {
		t.Fatalf("output path = %q, want %q", job.Output.Path, wantPath)
	}
	mp4, ok := fx.pub.get(wantPath)
	if !ok || int64(len(mp4)) != job.Output.Size || len(mp4) == 0 {
		t.Fatalf("published file: ok=%v size=%d output=%+v", ok, len(mp4), job.Output)
	}
	if job.FramesDone != 24 || job.FramesTotal != 24 || job.Progress != 1 {
		t.Errorf("progress = %+v", job)
	}
	meta, ok := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !ok {
		t.Fatal("no export record")
	}
	var rec map[string]any
	if err := json.Unmarshal(meta, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["job_id"] != job.ID || rec["output_path"] != wantPath || rec["frames"] != float64(24) || rec["encoder"] != "libx264" || rec["transport"] != "png" || rec["title"] != "Was ist eine Firewall?" {
		t.Errorf("record = %v", rec)
	}
	tmp := filepath.Join(t.TempDir(), "check.mp4")
	if err := os.WriteFile(tmp, mp4, 0o600); err != nil {
		t.Fatal(err)
	}
	doc := ffprobeFile(t, ffprobe, tmp)
	okVideo := false
	for _, s := range doc.Streams {
		if s.CodecType == "video" && s.Width == 1920 && s.Height == 1080 && s.AvgFrameRate == "30/1" && s.NbFrames == "24" {
			okVideo = true
		}
	}
	if !okVideo {
		t.Errorf("streams = %+v", doc.Streams)
	}
	if !fx.page.closed.Load() {
		t.Error("the browser page was not closed")
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
	// The stored job holds a prefix hash and never a prefix.
	raw, _ := os.ReadFile(filepath.Join(fx.dir, "jobs.json"))
	if h := fx.mgr.PrefixHash(job.ID); len(h) != 64 || !strings.Contains(string(raw), h) {
		t.Errorf("prefix hash = %q", h)
	}
	if strings.Contains(string(raw), "127.0.0.1") {
		t.Error("jobs.json mentions the listener address")
	}
}

func TestRunRefusesReportsThatDisagreeWithTheJob(t *testing.T) {
	cases := map[string]struct {
		mut  func(rep map[string]any)
		code string
		text string
	}{
		"catalog version": {func(rep map[string]any) { rep["catalog_version"] = "deadbeef0000" }, CodeRenderFailed, "catalog version"},
		"frame limit": {func(rep map[string]any) {
			rep["frame_count"] = 27001
			rep["scenes"].([]map[string]any)[0]["end_frame"] = 27001
		}, CodeInvalidStory, "limit"},
		"fps":               {func(rep map[string]any) { rep["fps"] = 24 }, CodeRenderFailed, "24 fps"},
		"canvas":            {func(rep map[string]any) { rep["width"] = 1280 }, CodeRenderFailed, "1280x"},
		"scene id":          {func(rep map[string]any) { rep["scenes"].([]map[string]any)[0]["id"] = "other" }, CodeRenderFailed, "other"},
		"scene count":       {func(rep map[string]any) { rep["scenes"] = []map[string]any{} }, CodeRenderFailed, "0 scenes"},
		"range":             {func(rep map[string]any) { rep["scenes"].([]map[string]any)[0]["end_frame"] = 0 }, CodeRenderFailed, "invalid range"},
		"short frame count": {func(rep map[string]any) { rep["frame_count"] = 10 }, CodeRenderFailed, "invalid range"},
		"zero frames":       {func(rep map[string]any) { rep["frame_count"] = 0 }, CodeRenderFailed, "no frames"},
		"error issue": {func(rep map[string]any) {
			rep["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "unknown_asset", "message": "no such asset"}}
		}, CodeInvalidStory, "no such asset"},
		"code error issue": {func(rep map[string]any) {
			rep["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "code_error", "message": "scene threw"}}
		}, CodeCodeError, "scene threw"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newRunFixture(t)
			var refused ReportAnswer
			fx.playing(playOptions{frames: 24, mutateRep: tc.mut, onRefused: func(a ReportAnswer) { refused = a }})
			job := fx.render(t)
			if job.Status != StatusFailed || job.Error == nil || job.Error.Code != tc.code || !strings.Contains(job.Error.Message, tc.text) {
				t.Fatalf("job = %+v error %+v", job, job.Error)
			}
			if refused.Start || refused.Error != tc.code {
				t.Errorf("the page was told %+v", refused)
			}
			if len(fx.pub.names()) != 0 {
				t.Errorf("files published for a refused report: %v", fx.pub.names())
			}
		})
	}
}

func TestRunRefusesASecondReport(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 12, reportTwice: true})
	if job := fx.render(t); job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestRunFailsWhenNoFrameArrivesAfterStart(t *testing.T) {
	old := noFrameWatchdog
	noFrameWatchdog = 600 * time.Millisecond
	t.Cleanup(func() { noFrameWatchdog = old })
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24, stallAfter: 3})
	started := time.Now()
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeCodeError || !strings.Contains(job.Error.Message, "no frame") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if time.Since(started) > 20*time.Second {
		t.Fatal("the watchdog was slow")
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
}

func TestRunTreatsAnyPageErrorAsFatal(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24, errorAfter: 4, errorBody: `{"code":"code_error","message":"boom in the scene","scene_id":"s1"}`})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeCodeError || !strings.Contains(job.Error.Message, "boom") || !strings.Contains(job.Error.Message, "s1") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	fx2 := newRunFixture(t)
	fx2.playing(playOptions{frames: 24, errorAfter: 2, errorBody: `{"code":"readiness","message":"fonts did not load"}`})
	job2 := fx2.render(t)
	if job2.Status != StatusFailed || job2.Error.Code != CodeRenderFailed {
		t.Fatalf("job2 = %+v %+v", job2, job2.Error)
	}
}

func TestRunRefusesBadFrames(t *testing.T) {
	fx := newRunFixture(t)
	wrong := solidPNG(t, 1280, 720, color.NRGBA{A: 255})
	fx.playing(playOptions{frames: 24, frame: func(n int) (string, []byte) {
		if n == 3 {
			return "image/png", wrong
		}
		return "image/png", fx.pngFrame
	}})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "frame_size") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestRunCancelStopsEverything(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 200, stallAfter: 5})
	plan, err := fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	job, _, _ := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	waitJob(t, fx.mgr, job.ID, func(j Job) bool { return j.Status == StatusRunning && j.FramesDone >= 4 })
	fx.mgr.Cancel(job.ID)
	done := waitJob(t, fx.mgr, job.ID, terminal)
	if done.Status != StatusCancelled || done.Error == nil || done.Error.Code != CodeCancelled {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if !fx.page.closed.Load() {
		t.Error("the page was not closed")
	}
	if len(fx.pub.names()) != 0 {
		t.Errorf("a cancelled render published %v", fx.pub.names())
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
}

func TestRunRestartsWithSoftwareWhenTheHardwareEncoderFails(t *testing.T) {
	fx := newRunFixture(t)
	fx.settings.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	fx.settings.EncoderName = videostudio.CodecVAAPI
	var restarts atomic.Int32
	fx.playing(playOptions{frames: 18, onRestart: func() { restarts.Add(1) }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if restarts.Load() != 1 {
		t.Fatalf("the page was told to restart %d times", restarts.Load())
	}
	meta, _ := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !strings.Contains(string(meta), `"encoder": "libx264"`) {
		t.Fatalf("record = %s", meta)
	}
}

func TestRunTimesOut(t *testing.T) {
	old := measuredFPS[TransportPNG]
	measuredFPS[TransportPNG] = 100000
	t.Cleanup(func() { measuredFPS[TransportPNG] = old })
	fx := newRunFixture(t)
	fx.settings.TimeoutSeconds = 1
	fx.playing(playOptions{frames: 400, frame: func(n int) (string, []byte) {
		time.Sleep(60 * time.Millisecond)
		return "image/png", fx.pngFrame
	}})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderTimeout {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestRunReportsCapabilityFailures(t *testing.T) {
	fx := newRunFixture(t)
	fx.settings.FFmpegPath = ""
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeFFmpegUnavailable {
		t.Fatalf("no ffmpeg: %+v %+v", job, job.Error)
	}

	fx = newRunFixture(t)
	fx.deps.Browsers.lookPath = func() (string, bool) { return "", false }
	fx.deps.Browsers.pins = map[string]PinnedChromium{}
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeBrowserUnavailable || !strings.Contains(job.Error.Message, "browser_path") {
		t.Fatalf("no browser: %+v %+v", job, job.Error)
	}

	fx = newRunFixture(t)
	fx.pub.fail = errors.New("disk full")
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "disk full") {
		t.Fatalf("publish failure: %+v %+v", job, job.Error)
	}
}

func TestRunRendersRawAndScreenshotTransports(t *testing.T) {
	ffprobe := testBinary(t, "ffprobe")
	for _, transport := range []string{TransportRaw, TransportScreenshot} {
		t.Run(transport, func(t *testing.T) {
			fx := newRunFixture(t)
			fx.settings.Transport = transport
			raw := bytes.Repeat([]byte{20, 60, 120, 255}, 1920*1080)
			fx.page.capture = func(int) []byte { return fx.pngFrame }
			fx.playing(playOptions{frames: 10, frame: func(int) (string, []byte) { return "application/octet-stream", raw }})
			job := fx.render(t)
			if job.Status != StatusSucceeded {
				t.Fatalf("job = %+v %+v", job, job.Error)
			}
			mp4, _ := fx.pub.get(job.Output.Path)
			tmp := filepath.Join(t.TempDir(), "t.mp4")
			_ = os.WriteFile(tmp, mp4, 0o600)
			doc := ffprobeFile(t, ffprobe, tmp)
			ok := false
			for _, s := range doc.Streams {
				ok = ok || (s.CodecType == "video" && s.NbFrames == "10")
			}
			if !ok {
				t.Fatalf("streams = %+v", doc.Streams)
			}
		})
	}
}

func TestRenderTimeoutFormula(t *testing.T) {
	// 15 minutes at 30 fps with the png rate must fit: 27000 frames x 3 / 6 fps = 13500 s.
	if got := RenderTimeout(3600, 27000, TransportPNG); got != 13500*time.Second {
		t.Errorf("15 minute png = %v", got)
	}
	if got := RenderTimeout(3600, 300, TransportPNG); got != 3600*time.Second {
		t.Errorf("short video keeps the configured timeout, got %v", got)
	}
	if got := RenderTimeout(60, 0, "unknown"); got != 60*time.Second {
		t.Errorf("zero frames = %v", got)
	}
}

func TestPrepareChecksTheStoryLikeSaveDoes(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil || plan.StoryETag == "" || len(plan.StoryJSON) == 0 || plan.Story.Scenes[0].Environment == "" {
		t.Fatalf("Prepare = %+v, %v", plan, err)
	}
	// A stale etag is reported as such.
	if _, err := fx.runner.Prepare(ctx, fx.projectID, `"stale"`, fx.settings); err == nil {
		t.Fatal("a stale etag was accepted")
	} else if pe := new(PrepareError); !errors.As(err, &pe) || !pe.Stale {
		t.Fatalf("stale error = %v", err)
	}
	// The current etag passes.
	if _, err := fx.runner.Prepare(ctx, fx.projectID, plan.StoryETag, fx.settings); err != nil {
		t.Fatalf("current etag: %v", err)
	}
	// A story with an error issue is refused with its issues.
	bad := `{"schema":"storymaker/1","title":"Bad","scenes":[{"id":"s1","environment":"env.nowhere"}]}`
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(bad), ""); err != nil {
		t.Fatal(err)
	}
	_, err = fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	var pe *PrepareError
	if !errors.As(err, &pe) || pe.Code != CodeInvalidStory || len(pe.Issues) == 0 || pe.Issues[0].Code != "unknown_asset" {
		t.Fatalf("invalid story error = %v (%+v)", err, pe)
	}
	// Unreadable JSON is an invalid story as well.
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(`{"schema":`), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = fx.runner.Prepare(ctx, fx.projectID, "", fx.settings); !errors.As(err, &pe) || pe.Code != CodeInvalidStory || len(pe.Issues) == 0 {
		t.Fatalf("broken JSON error = %v", err)
	}
	if _, err := fx.runner.Prepare(ctx, "33333333-3333-4333-8333-333333333333", "", fx.settings); err == nil {
		t.Fatal("a missing project was accepted")
	}
}

func TestPrepareCollectsCodeSceneBundles(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	story, err := os.ReadFile("../testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, bytes.ReplaceAll(story, []byte("\r\n"), []byte("\n")), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings); err == nil {
		t.Fatal("a code scene without a source was accepted")
	} else if pe := new(PrepareError); !errors.As(err, &pe) || pe.Code != CodeInvalidStory {
		t.Fatalf("missing code scene error = %v", err)
	}
	src, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(plan.Bundles) != 1 || !bytes.Contains(plan.Bundles["s3"], []byte("__smScene")) {
		t.Fatalf("bundles = %v", plan.Bundles)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/render/ -run "TestRun|TestPrepare|TestRenderTimeout" -count=1`

Expected: FAIL (build failed: `undefined: Deps`, `undefined: Runner`)

- [ ] **Step 3: Implement**

`internal/storymaker/render/render.go`:

```go
package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
	"aurago/internal/storymaker"
	"aurago/internal/videostudio"
)

// DefaultTransport is the frame transport used when storymaker.render.transport is empty. The
// transport spike (plan task B12) measured png as the fastest of the four deterministic transports
// on the production path (strict proxy and CDP guard active): 10 to 16 fps at 1080p against 4.5
// for lossless WebP, 4.1 for screenshots and 1.5 for raw RGBA on the development box.
const DefaultTransport = TransportPNG

// measuredFPS is the rendering speed the render timeout assumes per transport: the slowest rate the
// spike measured, cut by about 40 percent for hosts slower than the development box. The timeout is
// max(render.timeout_seconds, 3 x frames / measuredFPS), so a 15-minute video (27000 frames at 30
// fps) gets 13500 s with png, inside the 14400 s ceiling of render.timeout_seconds.
var measuredFPS = map[string]float64{TransportPNG: 6, TransportWebP: 2.5, TransportRaw: 1, TransportScreenshot: 2.5}

// ProjectsRoot is the desktop folder of the projects.
const ProjectsRoot = "Documents/Storymaker"

// The waits of a render; variables so the tests can shorten them.
var (
	reportWait      = 3 * time.Minute
	noFrameWatchdog = 10 * time.Second
)

// frameDeadlineMS is the per-frame deadline offered to the render page (see PageParams).
const frameDeadlineMS = 5000

const (
	maxOutputBytes     = int64(2) << 30
	minFreeStagingByte = int64(256) << 20
	proxyHeaderTimeout = 5 * time.Minute
)

// Publisher writes render outputs into the desktop workspace.
type Publisher interface {
	// WriteStream creates path from r and fails when it exists (create-only). max is the size budget.
	WriteStream(ctx context.Context, path string, r io.Reader, max int64) error
	// WriteBytes writes a small file, replacing an earlier one.
	WriteBytes(ctx context.Context, path string, data []byte) error
}

// Settings are the render settings of one job, read when it starts.
type Settings struct {
	TimeoutSeconds     int
	MaxDurationSeconds int
	BrowserPath        string
	Transport          string // "" selects DefaultTransport
	FFmpegPath         string
	Encoder            videostudio.Encoder
	EncoderName        string
	Limits             storymaker.Limits
}

// Deps are the services a render needs.
type Deps struct {
	// Dir is <DataDir>/storymaker.
	Dir string
	// UI is the ui tree (production: webassets.Namespace("ui")).
	UI        fs.FS
	Catalog   *storymaker.Catalog
	Projects  *storymaker.ProjectStore
	Browsers  *Browsers
	Publisher Publisher
	Settings  func(ctx context.Context) (Settings, error)
	Logger    *slog.Logger

	// OpenPage starts the browser; nil uses the real Chrome. Tests replace it.
	OpenPage pageOpener
	// FrameObserver, when set, sees every frame before it is encoded (the transport spike and the
	// determinism test hash them). It must not keep or change data.
	FrameObserver func(n int, transport string, data []byte)
	// SessionTweak, when set, may change the browser session options before the browser starts (tests
	// switch the guard or the proxy off to measure and to prove each layer).
	SessionTweak func(o *sessionOptions)
	// ReportObserver, when set, sees every compile report body the page posts.
	ReportObserver func(body []byte)
	// ListenerObserver, when set, receives the request counts of the listener when the job ends.
	ListenerObserver func(counts map[string]int64)
}

// Plan is the snapshot of a render request: the validated story after defaults and the bundles of
// its code scenes, taken when the job is queued (so edits while it waits change nothing).
type Plan struct {
	ProjectID string
	Story     storymaker.Story
	StoryJSON []byte
	Bundles   map[string][]byte
	StoryETag string
}

// PrepareError is a request that cannot become a job.
type PrepareError struct {
	Code    string
	Message string
	Issues  []storymaker.Issue
	// Stale is true when the story changed since the caller read it (HTTP 412).
	Stale bool
}

func (e *PrepareError) Error() string { return e.Code + ": " + e.Message }

// Runner executes render jobs.
type Runner struct{ deps Deps }

// NewRunner returns a Runner. Deps.Logger defaults to slog.Default().
func NewRunner(deps Deps) *Runner {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.OpenPage == nil {
		deps.OpenPage = openSession
	}
	return &Runner{deps: deps}
}

// Prepare reads the project's story, checks it against ifMatch (an etag; "" skips the check),
// decodes, defaults and validates it like the save endpoint does and collects the bundles of its
// code scenes. A story with an error issue is refused with CodeInvalidStory and the issues.
func (r *Runner) Prepare(ctx context.Context, projectID, ifMatch string, st Settings) (*Plan, error) {
	data, etag, err := r.deps.Projects.LoadStory(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if ifMatch != "" && ifMatch != etag {
		return nil, &PrepareError{Code: "stale_story", Message: "The story changed since it was loaded.", Stale: true}
	}
	story, issues, err := storymaker.DecodeStory(data, st.Limits)
	if err != nil {
		var de *storymaker.DecodeError
		if errors.As(err, &de) {
			return nil, &PrepareError{Code: CodeInvalidStory, Message: de.Message, Issues: de.Issues}
		}
		return nil, err
	}
	_ = issues
	storymaker.ApplyDefaults(&story)
	issues = storymaker.Validate(&story, r.deps.Catalog, st.Limits)
	status, err := r.deps.Projects.CodeSceneStatus(ctx, projectID)
	if err != nil {
		return nil, err
	}
	issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	if storymaker.HasErrors(issues) {
		first := issues[0]
		for _, is := range issues {
			if is.Severity == storymaker.SeverityError {
				first = is
				break
			}
		}
		return nil, &PrepareError{Code: CodeInvalidStory, Message: clip(first.Message, 300), Issues: issues}
	}
	bundles := map[string][]byte{}
	for _, sc := range story.Scenes {
		if sc.Kind != "code" {
			continue
		}
		b, err := r.deps.Projects.LoadCodeBundle(ctx, projectID, sc.ID)
		if err != nil {
			return nil, &PrepareError{Code: CodeCodeError, Message: "The code scene " + sc.ID + " has no bundle: " + clip(err.Error(), 200)}
		}
		bundles[sc.ID] = b
	}
	storyJSON, err := storymaker.CanonicalJSON(story)
	if err != nil {
		return nil, err
	}
	return &Plan{ProjectID: projectID, Story: story, StoryJSON: storyJSON, Bundles: bundles, StoryETag: etag}, nil
}

// CanvasSize returns the export size of an aspect: 1920x1080, 1080x1920 or 1080x1080.
func CanvasSize(aspect string) (int, int) {
	switch aspect {
	case "9:16":
		return 1080, 1920
	case "1:1":
		return 1080, 1080
	}
	return 1920, 1080
}

// ChooseTransport resolves the configured transport.
func ChooseTransport(configured string) string {
	switch configured {
	case TransportPNG, TransportWebP, TransportRaw, TransportScreenshot:
		return configured
	}
	return DefaultTransport
}

// RenderTimeout is max(render.timeout_seconds, 3 x frame_count / measured fps of the transport).
func RenderTimeout(timeoutSeconds, frames int, transport string) time.Duration {
	base := time.Duration(timeoutSeconds) * time.Second
	fps := measuredFPS[transport]
	if fps <= 0 {
		fps = 4
	}
	scaled := time.Duration(3 * float64(frames) / fps * float64(time.Second))
	if scaled > base {
		return scaled
	}
	return base
}

// Run is the RunFunc of the job manager.
func (r *Runner) Run(ctx context.Context, job Job, payload any, h *Handle) (Output, error) {
	plan, ok := payload.(*Plan)
	if !ok || plan == nil {
		return Output{}, Fail(CodeRenderFailed, "The render job has no snapshot.")
	}
	settings, err := r.deps.Settings(ctx)
	if err != nil {
		var je *JobError
		if errors.As(err, &je) {
			return Output{}, je
		}
		return Output{}, Fail(CodeFFmpegUnavailable, "%v", err)
	}
	run := &renderRun{r: r, job: job, h: h, plan: plan, st: settings, failCh: make(chan *JobError, 1), reportCh: make(chan struct{}), doneCh: make(chan struct{})}
	return run.execute(ctx)
}

// renderRun is the state of one running render.
type renderRun struct {
	r    *Runner
	job  Job
	h    *Handle
	plan *Plan
	st   Settings

	transport  string
	width      int
	height     int
	fps        int
	frameLimit int
	encoder    videostudio.Encoder
	encName    string
	output     string // staging MP4
	ingest     *Ingest
	base       string
	catVersion string

	failCh   chan *JobError
	reportCh chan struct{}
	doneCh   chan struct{}

	mu        sync.Mutex
	ctx       context.Context
	reported  bool
	restarted bool
	frames    int
	ff        *FFmpeg
	watchdog  *time.Timer
	closeDone sync.Once
	report    compileReport
	startedAt time.Time
	renderEnd time.Time
}

// fail records the first failure of the run.
func (run *renderRun) fail(err *JobError) {
	select {
	case run.failCh <- err:
	default:
	}
	if run.ingest != nil {
		run.ingest.Stop()
	}
}

func (run *renderRun) execute(ctx context.Context) (Output, error) {
	r, h := run.r, run.h
	run.ctx = ctx
	run.startedAt = time.Now()
	aspect := run.plan.Story.Format.Aspect
	run.width, run.height = CanvasSize(aspect)
	run.fps = run.plan.Story.Format.FPS
	if run.fps <= 0 {
		run.fps = storymaker.DefaultFPS
	}
	run.transport = ChooseTransport(run.st.Transport)
	run.frameLimit = int(run.st.Limits.MaxDurationSeconds) * run.fps
	if run.frameLimit <= 0 {
		run.frameLimit = run.st.MaxDurationSeconds * run.fps
	}
	run.encoder, run.encName = run.st.Encoder, run.st.EncoderName
	if run.encName == "" {
		run.encName = videostudio.CodecX264
		if run.encoder.Hardware() {
			run.encName = run.encoder.Codec
		}
	}
	run.catVersion = r.deps.Catalog.Version()
	h.SetPhase(PhaseStarting)

	if run.st.FFmpegPath == "" {
		return Output{}, Fail(CodeFFmpegUnavailable, "FFmpeg was not found.")
	}
	status, bin := r.deps.Browsers.Ensure(ctx, run.st.BrowserPath, func(done, total int64) {
		if total > 0 {
			h.SetProgress(0.05 * float64(done) / float64(total))
		}
	})
	if !status.Ready {
		return Output{}, Fail(CodeBrowserUnavailable, "%s", status.Issue)
	}

	staging := filepath.Join(StagingRoot(r.deps.Dir), run.job.ID)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return Output{}, Fail(CodeRenderFailed, "The render folder could not be created: %v", err)
	}
	defer os.RemoveAll(staging)
	defer func() {
		run.mu.Lock()
		if run.watchdog != nil {
			run.watchdog.Stop()
		}
		run.mu.Unlock()
	}()
	if free, err := fileutil.FreeDiskBytes(staging); err == nil && free < minFreeStagingByte {
		return Output{}, Fail(CodeRenderFailed, "Not enough free disk space for the render (%d MB free).", free>>20)
	}
	run.output = filepath.Join(staging, "out.mp4")

	snap, err := run.snapshot()
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	prefix, err := NewPrefix()
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	h.SetPrefixHash(HashPrefix(prefix))
	run.ingest = NewIngest(IngestConfig{Width: run.width, Height: run.height, Transport: run.transport, FrameLimit: run.frameLimit})
	listener, err := StartListener(prefix, snap, PageParams{Width: run.width, Height: run.height, FPS: run.fps, Transport: run.transport, FrameLimit: run.frameLimit, FrameDeadlineMS: frameDeadlineMS},
		Hooks{Report: run.onReport, Frame: run.onFrame, Error: run.onPageError})
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	defer listener.Close()
	run.base = listener.Base()
	if obs := r.deps.ListenerObserver; obs != nil {
		defer func() { obs(listener.Counts()) }()
	}

	origin, _ := url.Parse(listener.Origin())
	maxFrame := MaxFrameBytes(IngestConfig{Width: run.width, Height: run.height, Transport: run.transport})
	proxy, err := security.StartStrictCaptureProxy(origin, security.CaptureProxyLimits{
		MaxConcurrent:         16,
		MaxConnections:        400 + run.frameLimit,
		MaxBytes:              int64(run.frameLimit)*maxFrame + (256 << 20),
		ResponseHeaderTimeout: proxyHeaderTimeout,
	})
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	defer proxy.Close()

	opts := sessionOptions{Bin: bin, Profile: filepath.Join(staging, "profile"), ProxyAddr: proxy.Addr(), Base: run.base, Width: run.width, Height: run.height}
	if r.deps.SessionTweak != nil {
		r.deps.SessionTweak(&opts)
	}
	page, err := r.deps.OpenPage(ctx, opts)
	if err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeBrowserUnavailable, "%s", launchIssue(err))
	}
	defer page.Close()
	defer func() {
		run.mu.Lock()
		ff := run.ff
		run.mu.Unlock()
		if ff != nil {
			ff.Discard()
		}
	}()

	h.SetPhase(PhaseCompiling)
	h.SetProgress(0.05)
	if err := page.Navigate(ctx, run.base+"/"); err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "The render page could not be opened: %v", err)
	}

	if err := run.wait(ctx, page); err != nil {
		return Output{}, err
	}
	return run.finish(ctx)
}

// wait runs until the last frame was accepted or the job failed, timed out or was cancelled.
func (run *renderRun) wait(ctx context.Context, page browserPage) error {
	reportTimer := time.NewTimer(reportWait)
	defer reportTimer.Stop()
	overall := time.NewTimer(RenderTimeout(run.st.TimeoutSeconds, 0, run.transport))
	defer overall.Stop()
	reportC := run.reportCh
	reportTimeout := reportTimer.C
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case je := <-run.failCh:
			return je
		case <-page.Crashed():
			return Fail(CodeRenderFailed, "The browser tab crashed.")
		case <-reportC:
			reportC = nil
			reportTimeout = nil
			// The report is in: the frame count is known and fixes the timeout.
			if !overall.Stop() {
				select {
				case <-overall.C:
				default:
				}
			}
			overall.Reset(RenderTimeout(run.st.TimeoutSeconds, run.report.FrameCount, run.transport))
			if run.transport == TransportScreenshot {
				go run.driveScreenshots(ctx, page)
			}
		case <-reportTimeout:
			msg := "The render page did not report within " + reportWait.String() + "."
			if status := page.Status(ctx); status != "" && status != "null" {
				msg += " Page status: " + clip(status, 300)
			}
			return Fail(CodeRenderFailed, "%s", msg)
		case <-overall.C:
			return Fail(CodeRenderTimeout, "The render took longer than %s and was stopped.", RenderTimeout(run.st.TimeoutSeconds, run.report.FrameCount, run.transport).Round(time.Second))
		case <-run.doneCh:
			return nil
		}
	}
}

// finish encodes the tail, publishes the files and returns the output.
func (run *renderRun) finish(ctx context.Context) (Output, error) {
	h := run.h
	run.mu.Lock()
	ff := run.ff
	if run.watchdog != nil {
		run.watchdog.Stop()
	}
	run.mu.Unlock()
	h.SetPhase(PhaseEncoding)
	if ff == nil {
		return Output{}, Fail(CodeRenderFailed, "The encoder is missing.")
	}
	if err := ff.Wait(ctx); err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	info, err := os.Stat(run.output)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return Output{}, Fail(CodeRenderFailed, "The encoder produced no file.")
	}
	if info.Size() > maxOutputBytes {
		return Output{}, Fail(CodeRenderFailed, "The video is larger than %d GB.", maxOutputBytes>>30)
	}

	h.SetPhase(PhaseWriting)
	h.SetProgress(0.97)
	slug := storymaker.Slugify(run.plan.Story.Title, run.plan.ProjectID)
	exportDir := ProjectsRoot + "/" + run.plan.ProjectID + "/exports"
	outPath := exportDir + "/" + slug + "-" + run.job.ID[:8] + ".mp4"
	file, err := os.Open(run.output)
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "The video could not be read: %v", err)
	}
	defer file.Close()
	if err := run.r.deps.Publisher.WriteStream(ctx, outPath, file, maxOutputBytes); err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "The video could not be saved to the project: %v", err)
	}
	meta := map[string]any{
		"job_id": run.job.ID, "title": run.plan.Story.Title, "output_path": outPath, "size": info.Size(),
		"frames": run.report.FrameCount, "duration_ms": run.report.DurationMS, "fps": run.fps, "width": run.width, "height": run.height,
		"encoder": run.currentEncoderName(), "transport": run.transport, "catalog_version": run.catVersion,
		"created_at": time.Now().UTC().Format(time.RFC3339), "render_seconds": time.Since(run.startedAt).Seconds(),
	}
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		if err := run.r.deps.Publisher.WriteBytes(ctx, exportDir+"/"+run.job.ID+".json", append(data, '\n')); err != nil {
			run.r.deps.Logger.Warn("[Storymaker] The export record could not be written", "error", err)
		}
	}
	return Output{Path: outPath, Size: info.Size()}, nil
}

func (run *renderRun) currentEncoderName() string {
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.restarted {
		return videostudio.CodecX264
	}
	return run.encName
}

// snapshot copies everything the page may read.
func (run *renderRun) snapshot() (*Snapshot, error) {
	ui := run.r.deps.UI
	cat := run.r.deps.Catalog
	snap := &Snapshot{Story: run.plan.StoryJSON, CatalogFiles: map[string][]byte{}, CodeBundles: run.plan.Bundles}
	read := func(name string) ([]byte, error) {
		data, err := fs.ReadFile(ui, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		return data, nil
	}
	var err error
	if snap.HostBundle, err = read("js/storymaker/host.bundle.js"); err != nil {
		return nil, err
	}
	if snap.PlayerBundle, err = read("js/storymaker/player.bundle.js"); err != nil {
		return nil, err
	}
	files := map[string]bool{"catalog.json": true}
	if cat.Rig.File != "" {
		files[cat.Rig.File] = true
	}
	for _, e := range cat.Entries() {
		if e.File != "" {
			files[e.File] = true
		}
	}
	for rel := range files {
		clean := path.Clean(rel)
		if clean != rel || !fs.ValidPath(clean) || strings.HasPrefix(clean, "/") {
			return nil, fmt.Errorf("the catalog names an unsafe file %q", rel)
		}
		if snap.CatalogFiles[rel], err = read("img/storymaker/" + rel); err != nil {
			return nil, err
		}
	}
	for _, f := range cat.Fonts {
		clean := strings.TrimPrefix(path.Clean(f.File), "/")
		if !strings.HasPrefix(f.File, "/fonts/") || !fs.ValidPath(clean) {
			return nil, fmt.Errorf("the catalog font file %q is not a /fonts/ path", f.File)
		}
		data, err := read(clean)
		if err != nil {
			return nil, err
		}
		snap.Fonts = append(snap.Fonts, SnapshotFont{Name: f.Name, Family: f.Family, Weight: f.Weight, File: f.File, Data: data})
	}
	return snap, nil
}

// onReport is the hook behind POST /report: it validates the report, starts FFmpeg and tells the
// page to begin. The ingest starts after the run lock is released (the ingest calls back into the
// run while it holds its own lock, so the lock order is always ingest, then run).
func (run *renderRun) onReport(body []byte) ReportAnswer {
	if obs := run.r.deps.ReportObserver; obs != nil {
		obs(body)
	}
	run.mu.Lock()
	if run.reported {
		run.mu.Unlock()
		return ReportAnswer{Error: "report_repeated", Message: "A compile report was already accepted."}
	}
	run.reported = true
	ids := make([]string, len(run.plan.Story.Scenes))
	for i, sc := range run.plan.Story.Scenes {
		ids[i] = sc.ID
	}
	rep, refusal := parseReport(body, reportExpect{FPS: run.fps, Width: run.width, Height: run.height, MaxFrames: run.frameLimit, CatalogVersion: run.catVersion, SceneIDs: ids})
	if refusal != nil {
		run.mu.Unlock()
		run.fail(&JobError{Code: refusal.Code, Message: refusal.Message})
		return ReportAnswer{Error: refusal.Code, Message: refusal.Message}
	}
	ff, err := run.startFFmpeg(run.encoder, rep.FrameCount)
	if err != nil {
		run.mu.Unlock()
		run.fail(Fail(CodeFFmpegUnavailable, "%v", err))
		return ReportAnswer{Error: CodeFFmpegUnavailable, Message: err.Error()}
	}
	run.report = rep
	run.frames = rep.FrameCount
	run.ff = ff
	run.watchdog = time.AfterFunc(noFrameWatchdog, func() {
		run.fail(Fail(CodeCodeError, "The page rendered no frame for %s. A code scene may be stuck.", noFrameWatchdog))
	})
	run.mu.Unlock()

	run.ingest.Start(rep.FrameCount, ff, run.recoverEncoder, run.onFrameAccepted)
	run.h.SetPhase(PhaseRendering)
	run.h.SetFrames(0, rep.FrameCount)
	close(run.reportCh)
	return ReportAnswer{Start: true}
}

func (run *renderRun) startFFmpeg(enc videostudio.Encoder, frames int) (*FFmpeg, error) {
	return StartFFmpeg(run.ctx, FFmpegConfig{
		Path: run.st.FFmpegPath, Encoder: enc, Transport: run.transport,
		Width: run.width, Height: run.height, FPS: run.fps, Frames: frames, Output: run.output,
	})
}

// recoverEncoder is the one-shot software restart after a hardware encoder failed in the stream.
// It is called with the ingest lock held and the run lock free.
func (run *renderRun) recoverEncoder(cause error) (FrameSink, bool) {
	run.mu.Lock()
	defer run.mu.Unlock()
	if !run.encoder.Hardware() || run.restarted {
		return nil, false
	}
	run.restarted = true
	run.r.deps.Logger.Warn("[Storymaker] The hardware encoder failed, restarting the render with libx264", "encoder", run.encName, "error", cause)
	if run.ff != nil {
		run.ff.Discard()
	}
	ff, err := run.startFFmpeg(videostudio.Encoder{Quality: run.encoder.Quality}, run.frames)
	if err != nil {
		return nil, false
	}
	run.ff = ff
	return ff, true
}

func (run *renderRun) onFrameAccepted(n int) {
	run.mu.Lock()
	if run.watchdog != nil {
		run.watchdog.Reset(noFrameWatchdog)
	}
	total := run.frames
	run.mu.Unlock()
	run.h.SetFrames(n+1, total)
}

// accept feeds one frame to the ingest and maps its outcome; it is the common path of the HTTP hook
// and the screenshot driver.
func (run *renderRun) accept(n int, contentType string, data []byte) (FrameOutcome, *IngestError) {
	if obs := run.r.deps.FrameObserver; obs != nil {
		obs(n, run.transport, data)
	}
	outcome, err := run.ingest.Accept(n, contentType, data)
	if err != nil {
		var ie *IngestError
		if !errors.As(err, &ie) {
			ie = refuse(500, "encoder_failed", "%v", err)
		}
		if ie.Code == "encoder_failed" {
			run.fail(Fail(CodeRenderFailed, "%s", ie.Message))
		} else {
			run.fail(Fail(CodeRenderFailed, "A frame was refused (%s): %s", ie.Code, ie.Message))
		}
		return outcome, ie
	}
	if outcome == FrameRestart {
		run.onFrameAccepted(-1)
	}
	if outcome == FrameDone {
		run.closeDone.Do(func() { close(run.doneCh) })
		run.h.SetPhase(PhaseEncoding)
	}
	return outcome, nil
}

func (run *renderRun) onFrame(n int, contentType string, data []byte) (FrameAnswer, int) {
	outcome, ie := run.accept(n, contentType, data)
	if ie != nil {
		return FrameAnswer{Stop: true, Error: ie.Code, Message: ie.Message}, ie.Status
	}
	switch outcome {
	case FrameDone:
		return FrameAnswer{OK: true, Done: true}, 200
	case FrameRestart:
		return FrameAnswer{Restart: true}, 200
	}
	return FrameAnswer{OK: true}, 200
}

// onPageError makes every error the page reports fatal (spec: a code scene that fails at frame time
// keeps rendering a placeholder, which would produce a video that looks fine and is not).
func (run *renderRun) onPageError(e PageError) {
	code := CodeRenderFailed
	switch e.Code {
	case storymaker.CodeCodeError, storymaker.CodeDeadline:
		code = CodeCodeError
	}
	msg := e.Message
	if e.SceneID != "" {
		msg = "Scene " + e.SceneID + ": " + msg
	}
	run.fail(&JobError{Code: code, Message: clip(msg, 500)})
}

// driveScreenshots renders and captures every frame for the screenshot transport.
func (run *renderRun) driveScreenshots(ctx context.Context, page browserPage) {
	if err := page.WaitReady(ctx); err != nil {
		run.fail(Fail(CodeRenderFailed, "%v", err))
		return
	}
	total := run.frames
	for n := 0; n < total; {
		if ctx.Err() != nil {
			return
		}
		shot, err := page.Capture(ctx, n)
		if err != nil {
			if ctx.Err() == nil {
				run.fail(Fail(CodeRenderFailed, "Frame %d could not be captured: %v", n, err))
			}
			return
		}
		outcome, ie := run.accept(n, "image/png", shot)
		if ie != nil {
			return
		}
		switch outcome {
		case FrameDone:
			return
		case FrameRestart:
			n = 0
		default:
			n++
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/storymaker/render/ && go test ./internal/storymaker/render/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/render.go internal/storymaker/render/render_test.go
git commit -m "feat(storymaker): silent render job with outputs, timeouts and encoder fallback" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B10: Browser tests of the render pipeline

**Files:**
- Test: `internal/storymaker/render/egress_helpers_test.go`
- Test: `internal/storymaker/render/browser_smoke_test.go`
- Test: `internal/storymaker/render/egress_smoke_test.go`

This task adds tests only; there is no implementation step. They are gated by `AURAGO_RUN_BROWSER_SMOKE=1` (the spike by `AURAGO_STORYMAKER_SPIKE=1`, the pinned-Chromium acceptance by `AURAGO_STORYMAKER_PINNED=1`) and skip otherwise.

- `TestBrowserStatusProbeLaunchesRealChrome`: the launch probe against the installed Chrome.
- `TestRealRenderOfTheFirewallStory`: the real end-to-end silent export of `firewall.json` (Chrome, strict proxy, CDP guard, FFmpeg) checked with ffprobe: 1920x1080, 30 fps, every frame present, yuv420p, BT.709, an AAC stream, duration within 0.15 s of frames/30, staging removed.
- `TestRealRenderIsDeterministic`: the same story rendered twice gives identical decoded-pixel hashes for every frame.
- `TestEgressLayersEachStopTheProbe`: hostile code (`location`, `window.open`, `fetch` to a public host and to another loopback port, XMLHttpRequest, WebSocket, images, scripts, a beacon, and the same from a worker) against a loopback probe server. A control run shows the probe sees every request; the strict proxy alone, the CSP with sandbox alone (only the navigation gets through), the CDP guard alone and all layers together (hybrid and catch-all guard) are then tried.
- `TestTransportSpike`, `TestGuardAndRequestOverheadSpike`: the measurement harnesses of task B13.
- `TestPinnedChromiumDownloadsVerifiesAndLaunches`: the acceptance test of the pinned Chromium on a host without a browser (aurago-test).

- [ ] **Step 1: Write the tests**

`internal/storymaker/render/egress_helpers_test.go`:

```go
package render

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// probeServer is a loopback server on its own port that records every request it receives.
type probeServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newProbeServer(t *testing.T) *probeServer {
	t.Helper()
	p := &probeServer{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.hits = append(p.hits, r.Method+" "+r.URL.Path)
		p.mu.Unlock()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write([]byte("/* probe */"))
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *probeServer) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.hits)
}

// pageServer serves one page below a 32-character prefix: the markup of the render page (an external
// script, as the CSP demands) and that script at <base>/js/storymaker/host.bundle.js. It hands over
// what the page POSTs to <base>/result; every other path below the prefix answers with a harmless
// script.
type pageServer struct {
	*httptest.Server
	prefix  string
	results chan string
}

// newPageServer starts the server. csp, when not nil, computes the Content-Security-Policy of the
// page from the render base.
func newPageServer(t *testing.T, script string, csp func(base string) string) *pageServer {
	t.Helper()
	ps := &pageServer{prefix: strings.Repeat("a1", 16), results: make(chan string, 16)}
	ps.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := "/" + ps.prefix + "/"
		if !strings.HasPrefix(r.URL.Path, root) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch strings.TrimPrefix(r.URL.Path, root) {
		case "":
			if csp != nil {
				w.Header().Set("Content-Security-Policy", csp("http://"+r.Host+"/"+ps.prefix))
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderPage("http://"+r.Host+"/"+ps.prefix, 320, 180)))
		case "js/storymaker/host.bundle.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(script))
		case "result":
			body := make([]byte, 8192)
			n, _ := r.Body.Read(body)
			select {
			case ps.results <- string(body[:n]):
			default:
			}
		case "boot.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte("/* ok */"))
		}
	}))
	t.Cleanup(ps.Close)
	return ps
}

func (ps *pageServer) base() string { return ps.URL + "/" + ps.prefix }

func (ps *pageServer) wait(t *testing.T, d time.Duration) string {
	t.Helper()
	select {
	case r := <-ps.results:
		return r
	case <-time.After(d):
		t.Fatal("the page reported nothing")
		return ""
	}
}
```

`internal/storymaker/render/browser_smoke_test.go`:

```go
package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/image/webp"
)

// browserSmoke gates the tests that start a real Chrome: AURAGO_RUN_BROWSER_SMOKE=1. On the Windows
// development box run them through `go test` with GOTMPDIR=D:\tmp\go-build (repo-built executables
// cannot start Chrome there).
func browserSmoke(t *testing.T) string {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the real browser tests")
	}
	chrome := os.Getenv("AURAGO_TEST_CHROME")
	if chrome == "" {
		found, ok := launcher.LookPath()
		if !ok {
			t.Skip("no Chrome or Chromium is installed")
		}
		chrome = found
	}
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		SetNoSandboxForTests(true)
		t.Cleanup(func() { SetNoSandboxForTests(false) })
	}
	return chrome
}

// realFixture is a run fixture whose Browsers and OpenPage are the real ones.
func newRealFixture(t *testing.T, chrome string) *runFixture {
	t.Helper()
	fx := newRunFixture(t)
	fx.deps.Browsers = NewBrowsers(fx.dir)
	fx.deps.OpenPage = openSession
	fx.settings.BrowserPath = chrome
	return fx
}

func (fx *runFixture) rebuild(t *testing.T) {
	t.Helper()
	fx.mgr.Close()
	fx.runner = NewRunner(fx.deps)
	mgr, err := NewManager(ManagerOptions{Dir: fx.dir, Run: fx.runner.Run})
	if err != nil {
		t.Fatal(err)
	}
	fx.mgr = mgr
	t.Cleanup(mgr.Close)
}

// loadFirewall puts the firewall story and its code scene into the fixture's project.
func (fx *runFixture) loadFirewall(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	story, err := os.ReadFile("../testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, lf(story), ""); err != nil {
		t.Fatal(err)
	}
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", lf(src), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
}

// frameHashes records the decoded-pixel hash of every frame of a render.
type frameHashes struct {
	mu     sync.Mutex
	hashes map[int]string
}

func newFrameHashes() *frameHashes { return &frameHashes{hashes: map[int]string{}} }

func (f *frameHashes) observe(t *testing.T) func(n int, transport string, data []byte) {
	return func(n int, transport string, data []byte) {
		h := decodedHash(t, transport, data)
		f.mu.Lock()
		f.hashes[n] = h // a restart overwrites earlier frames, as the encoder does
		f.mu.Unlock()
	}
}

func (f *frameHashes) sequence() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]int, 0, len(f.hashes))
	for k := range f.hashes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	sum := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(sum, "%d:%s\n", k, f.hashes[k])
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func (f *frameHashes) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hashes)
}

// decodedHash hashes the straight RGBA pixels of a frame, whatever its transport.
func decodedHash(t *testing.T, transport string, data []byte) string {
	if transport == TransportRaw {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	var img image.Image
	var err error
	if transport == TransportWebP {
		img, err = webp.Decode(bytes.NewReader(data))
	} else {
		img, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		t.Errorf("a %s frame did not decode: %v", transport, err)
		return ""
	}
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)
	sum := sha256.Sum256(nrgba.Pix)
	return hex.EncodeToString(sum[:])
}

func TestBrowserStatusProbeLaunchesRealChrome(t *testing.T) {
	chrome := browserSmoke(t)
	b := NewBrowsers(t.TempDir())
	st := b.Status(context.Background(), chrome)
	if !st.Ready || st.Source != SourceConfig || st.Version == "" {
		t.Fatalf("status = %+v", st)
	}
	t.Logf("browser %s", st.Version)
	st = b.Status(context.Background(), "")
	t.Logf("resolution without a configured path: %+v", st)
}

func renderFirewall(t *testing.T, chrome, transport string, observer func(int, string, []byte)) (*runFixture, Job, time.Duration) {
	t.Helper()
	fx := newRealFixture(t, chrome)
	fx.settings.Transport = transport
	fx.deps.FrameObserver = observer
	fx.rebuild(t)
	fx.loadFirewall(t)
	started := time.Now()
	job := fx.render(t)
	return fx, job, time.Since(started)
}

func TestRealRenderOfTheFirewallStory(t *testing.T) {
	chrome := browserSmoke(t)
	ffprobe := testBinary(t, "ffprobe")
	hashes := newFrameHashes()
	fx, job, took := renderFirewall(t, chrome, "", hashes.observe(t))
	if job.Status != StatusSucceeded || job.Output == nil {
		t.Fatalf("job = %+v error %+v", job, job.Error)
	}
	t.Logf("firewall.json: %d frames in %v (%.1f fps overall)", job.FramesTotal, took.Round(time.Millisecond), float64(job.FramesTotal)/took.Seconds())
	mp4, ok := fx.pub.get(job.Output.Path)
	if !ok || len(mp4) < 10_000 {
		t.Fatalf("published file: %v %d bytes", ok, len(mp4))
	}
	tmp := filepath.Join(t.TempDir(), "firewall.mp4")
	if err := os.WriteFile(tmp, mp4, 0o600); err != nil {
		t.Fatal(err)
	}
	doc := ffprobeFile(t, ffprobe, tmp)
	var video, audio bool
	for _, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			video = s.Width == 1920 && s.Height == 1080 && s.AvgFrameRate == "30/1" && s.NbFrames == strconv.Itoa(job.FramesTotal) && s.PixFmt == "yuv420p" && s.ColorSpace == "bt709"
		case "audio":
			audio = s.CodecName == "aac"
		}
	}
	duration, _ := strconv.ParseFloat(doc.Format.Duration, 64)
	want := float64(job.FramesTotal) / 30
	if !video || !audio || duration < want-0.15 || duration > want+0.15 || job.FramesTotal < 250 {
		t.Fatalf("ffprobe: video=%v audio=%v duration=%v (want about %v) frames=%d streams=%+v", video, audio, duration, want, job.FramesTotal, doc.Streams)
	}
	if hashes.count() != job.FramesTotal {
		t.Errorf("observed %d frames of %d", hashes.count(), job.FramesTotal)
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging left: %v", entries)
	}
}

func TestRealRenderIsDeterministic(t *testing.T) {
	chrome := browserSmoke(t)
	var sequences []string
	for i := 0; i < 2; i++ {
		hashes := newFrameHashes()
		_, job, _ := renderFirewall(t, chrome, "", hashes.observe(t))
		if job.Status != StatusSucceeded {
			t.Fatalf("render %d: %+v %+v", i, job, job.Error)
		}
		sequences = append(sequences, hashes.sequence())
		if hashes.count() != job.FramesTotal {
			t.Fatalf("render %d observed %d of %d frames", i, hashes.count(), job.FramesTotal)
		}
	}
	if sequences[0] != sequences[1] {
		t.Fatalf("two renders of the same story differ:\n%s\n%s", sequences[0], sequences[1])
	}
}

// TestTransportSpike is the phase 1 frame-transport spike (spec step 4): firewall.json at 1080p30
// through every transport on the production path (strict proxy and CDP interception active), twice
// each. It prints frames per second, whether the two renders are identical, whether the transports
// agree with each other and the peak memory. Run it with AURAGO_STORYMAKER_SPIKE=1; the table is
// also written to AURAGO_STORYMAKER_SPIKE_OUT when that names a file.
func TestTransportSpike(t *testing.T) {
	chrome := browserSmoke(t)
	if os.Getenv("AURAGO_STORYMAKER_SPIKE") != "1" {
		t.Skip("set AURAGO_STORYMAKER_SPIKE=1 to run the transport spike")
	}
	type row struct {
		Transport   string  `json:"transport"`
		Frames      int     `json:"frames"`
		RenderFPS   float64 `json:"render_fps"`
		JobSeconds  float64 `json:"job_seconds"`
		Identical   bool    `json:"two_renders_identical"`
		Sequence    string  `json:"sequence_hash"`
		ChromePeak  int64   `json:"chrome_peak_rss_mb"`
		GoHeapPeak  int64   `json:"go_heap_peak_mb"`
		MP4Bytes    int     `json:"mp4_bytes"`
		FrameBytes  int64   `json:"frame_bytes_avg"`
		Environment string  `json:"environment"`
	}
	var rows []row
	transports := []string{TransportPNG, TransportWebP, TransportRaw, TransportScreenshot}
	if only := os.Getenv("AURAGO_STORYMAKER_SPIKE_TRANSPORTS"); only != "" {
		transports = strings.Split(only, ",")
	}
	for _, transport := range transports {
		var seqs []string
		var best row
		runs := 2
		if n, err := strconv.Atoi(os.Getenv("AURAGO_STORYMAKER_SPIKE_RUNS")); err == nil && n > 0 {
			runs = n
		}
		for run := 0; run < runs; run++ {
			hashes := newFrameHashes()
			var totalBytes int64
			var firstFrame, lastFrame time.Time
			var fmu sync.Mutex
			observer := func(n int, tr string, data []byte) {
				fmu.Lock()
				if firstFrame.IsZero() {
					firstFrame = time.Now()
				}
				lastFrame = time.Now()
				totalBytes += int64(len(data))
				fmu.Unlock()
				hashes.observe(t)(n, tr, data)
			}
			fx := newRealFixture(t, chrome)
			fx.settings.Transport = transport
			if os.Getenv("AURAGO_STORYMAKER_SPIKE_PLAYER") == "recommended" {
				fx.deps.UI = patchedUI{FS: fx.deps.UI, edit: recommendedPlayerChanges}
			}
			fx.deps.FrameObserver = observer
			var pid int
			var pidMu sync.Mutex
			fx.deps.OpenPage = func(ctx context.Context, o sessionOptions) (browserPage, error) {
				p, err := openSession(ctx, o)
				if s, ok := p.(*session); ok && err == nil {
					pidMu.Lock()
					pid = s.l.PID()
					pidMu.Unlock()
				}
				return p, err
			}
			fx.rebuild(t)
			fx.loadFirewall(t)

			stop := make(chan struct{})
			var chromePeak, heapPeak int64
			var wg sync.WaitGroup
			sample := os.Getenv("AURAGO_STORYMAKER_SPIKE_MEM") == "1"
			if sample {
				wg.Add(1)
			}
			go func() {
				if !sample {
					return
				}
				defer wg.Done()
				tick := time.NewTicker(250 * time.Millisecond)
				defer tick.Stop()
				for {
					select {
					case <-stop:
						return
					case <-tick.C:
						pidMu.Lock()
						p := pid
						pidMu.Unlock()
						if rss := treeRSS(p); rss > chromePeak {
							chromePeak = rss
						}
						var ms runtime.MemStats
						runtime.ReadMemStats(&ms)
						if h := int64(ms.HeapInuse); h > heapPeak {
							heapPeak = h
						}
					}
				}
			}()
			started := time.Now()
			job := fx.render(t)
			elapsed := time.Since(started)
			close(stop)
			wg.Wait()
			if job.Status != StatusSucceeded {
				t.Fatalf("%s run %d: %+v %+v", transport, run, job, job.Error)
			}
			seqs = append(seqs, hashes.sequence())
			span := lastFrame.Sub(firstFrame).Seconds()
			mp4, _ := fx.pub.get(job.Output.Path)
			r := row{Transport: transport, Frames: job.FramesTotal, RenderFPS: float64(job.FramesTotal) / span, JobSeconds: elapsed.Seconds(),
				Sequence: seqs[run], ChromePeak: chromePeak >> 20, GoHeapPeak: heapPeak >> 20, MP4Bytes: len(mp4), FrameBytes: totalBytes / int64(max(job.FramesTotal, 1)),
				Environment: fmt.Sprintf("%s/%s %d CPU", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())}
			t.Logf("%s run %d: %d frames, %.1f fps, job %.1fs, chrome peak %d MB, go heap peak %d MB, %d bytes/frame", transport, run, r.Frames, r.RenderFPS, r.JobSeconds, r.ChromePeak, r.GoHeapPeak, r.FrameBytes)
			if run == 0 || r.RenderFPS < best.RenderFPS {
				best = r // the slower run is the honest figure
			}
		}
		best.Identical = len(seqs) < 2 || seqs[0] == seqs[1]
		rows = append(rows, best)
	}
	for _, r := range rows[1:] {
		t.Logf("pixels of %s equal to %s: %v", r.Transport, rows[0].Transport, r.Sequence == rows[0].Sequence)
	}
	out, _ := json.MarshalIndent(rows, "", "  ")
	t.Logf("spike result:\n%s", out)
	if path := os.Getenv("AURAGO_STORYMAKER_SPIKE_OUT"); path != "" {
		_ = os.WriteFile(path, out, 0o644)
	}
}

// treeRSS sums the resident memory of a process and its descendants.
func treeRSS(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0
	}
	var total int64
	var walk func(p *process.Process)
	walk = func(p *process.Process) {
		if mem, err := p.MemoryInfo(); err == nil && mem != nil {
			total += int64(mem.RSS)
		}
		if kids, err := p.Children(); err == nil {
			for _, k := range kids {
				walk(k)
			}
		}
	}
	walk(root)
	return total
}

var _ = storymaker.Schema

// patchedUI returns the ui tree with host.bundle.js changed by edit, to try a change to the player
// without touching its files.
type patchedUI struct {
	fs.FS
	edit func([]byte) []byte
}

func (p patchedUI) Open(name string) (fs.File, error) {
	if name != "js/storymaker/host.bundle.js" {
		return p.FS.Open(name)
	}
	data, err := fs.ReadFile(p.FS, name)
	if err != nil {
		return nil, err
	}
	return &memFile{name: path.Base(name), r: bytes.NewReader(p.edit(data)), size: int64(len(p.edit(data)))}, nil
}

type memFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (f *memFile) Stat() (fs.FileInfo, error) { return memInfo{f}, nil }
func (f *memFile) Read(b []byte) (int, error) { return f.r.Read(b) }
func (f *memFile) Close() error               { return nil }

type memInfo struct{ f *memFile }

func (i memInfo) Name() string       { return i.f.name }
func (i memInfo) Size() int64        { return i.f.size }
func (i memInfo) Mode() fs.FileMode  { return 0o444 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }

// recommendedPlayerChanges are the two changes to the player the plan asks part C for: frames as
// CORS-simple requests (no preflight per frame) and a render-mode frame deadline from boot.json
// (frame_deadline_ms) instead of the preview's 250 ms. Until C lands them the spike applies them to
// the bundle text in memory.
func recommendedPlayerChanges(src []byte) []byte {
	src = simpleFrameRequests(src)
	return bytes.Replace(src, []byte("canvas.height = boot.height;\n"),
		[]byte("canvas.height = boot.height;\n            if (boot.frame_deadline_ms > 0) env.limits.frameDeadlineMs = Math.min(60000, boot.frame_deadline_ms);\n"), 1)
}

// simpleFrameRequests makes the page send frames as CORS-simple requests (text/plain, frame number in
// the query), which Chrome does not preflight.
func simpleFrameRequests(src []byte) []byte {
	return bytes.Replace(src,
		[]byte("post('/frame', body, { 'Content-Type': MIME[boot.transport], 'X-SM-Frame': String(n) })"),
		[]byte("post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' })"), 1)
}

// TestGuardAndRequestOverheadSpike times the png transport with the protection layers switched off
// one by one, and with the two player changes the plan asks for (AURAGO_STORYMAKER_SPIKE=1). It is
// the evidence for the hybrid guard and for frames as CORS-simple requests.
func TestGuardAndRequestOverheadSpike(t *testing.T) {
	chrome := browserSmoke(t)
	if os.Getenv("AURAGO_STORYMAKER_SPIKE") != "1" {
		t.Skip("set AURAGO_STORYMAKER_SPIKE=1")
	}
	type variant struct {
		name  string
		edit  func([]byte) []byte
		tweak func(o *sessionOptions)
	}
	for i, v := range []variant{
		{"preflight per frame, hybrid guard", func(b []byte) []byte { return b }, nil},
		{"simple requests, hybrid guard", simpleFrameRequests, nil},
		{"simple requests, full guard", simpleFrameRequests, func(o *sessionOptions) { o.GuardFull = true }},
		{"simple requests, no guard", simpleFrameRequests, func(o *sessionOptions) { o.NoGuard = true }},
		{"simple requests, no proxy, no guard", simpleFrameRequests, func(o *sessionOptions) { o.ProxyAddr, o.NoGuard = "", true }},
	} {
		var first, last time.Time
		var mu sync.Mutex
		var opened *session
		fx := newRealFixture(t, chrome)
		fx.settings.TimeoutSeconds = 3600
		fx.deps.UI = patchedUI{FS: fx.deps.UI, edit: v.edit}
		fx.deps.SessionTweak = v.tweak
		fx.deps.OpenPage = func(ctx context.Context, o sessionOptions) (browserPage, error) {
			p, err := openSession(ctx, o)
			opened, _ = p.(*session)
			return p, err
		}
		fx.deps.ListenerObserver = func(c map[string]int64) { t.Logf("variant %d listener requests: %v", i, c) }
		fx.deps.FrameObserver = func(n int, tr string, data []byte) {
			mu.Lock()
			if first.IsZero() {
				first = time.Now()
			}
			last = time.Now()
			mu.Unlock()
		}
		fx.rebuild(t)
		fx.loadFirewall(t)
		job := fx.render(t)
		if job.Status != StatusSucceeded {
			t.Fatalf("variant %d: %+v %+v", i, job, job.Error)
		}
		t.Logf("variant %d %s: %d frames in %.1fs = %.1f fps (guard %q: paused %d, blocked %d, workers %d guarded / %d failed)", i, v.name, job.FramesTotal, last.Sub(first).Seconds(),
			float64(job.FramesTotal)/last.Sub(first).Seconds(), opened.guard.Mode(), opened.guard.Seen(), opened.guard.Blocked(), opened.guard.workers.Load(), opened.guard.workerFailed.Load())
	}
}

// TestPinnedChromiumDownloadsVerifiesAndLaunches is the acceptance test of the pinned Chromium on a
// host that has none installed (aurago-test, a container): the first export's path through
// Browsers.Ensure with no configured path and no system browser. It downloads about 190 MB from the
// snapshot host, verifies the SHA-256, unpacks, and starts the browser headless with its sandbox.
// Run it with AURAGO_STORYMAKER_PINNED=1 (and AURAGO_RUN_BROWSER_SMOKE=1); it hides the system
// browser from the lookup so the pinned one is the only candidate.
func TestPinnedChromiumDownloadsVerifiesAndLaunches(t *testing.T) {
	browserSmoke(t)
	if os.Getenv("AURAGO_STORYMAKER_PINNED") != "1" {
		t.Skip("set AURAGO_STORYMAKER_PINNED=1 to download and start the pinned Chromium")
	}
	b := NewBrowsers(t.TempDir())
	b.lookPath = func() (string, bool) { return "", false }
	if st := b.Status(context.Background(), ""); st.Ready || !st.CanDownload || st.Source != SourcePinned {
		t.Fatalf("status before the download: %+v", st)
	}
	var last int64
	started := time.Now()
	st, bin := b.Ensure(context.Background(), "", func(done, total int64) { last = done })
	if !st.Ready || st.Source != SourcePinned || bin == "" {
		t.Fatalf("Ensure: %+v %q", st, bin)
	}
	t.Logf("pinned Chromium %s ready after %v (%d MB downloaded): %s", st.Version, time.Since(started).Round(time.Second), last>>20, bin)
	if again := b.Status(context.Background(), ""); !again.Ready {
		t.Fatalf("status after the install: %+v", again)
	}
	// The player must run on the pinned revision (Chromium 128) as well: export the firewall story with it.
	fx := newRealFixture(t, bin)
	fx.loadFirewall(t)
	if job := fx.render(t); job.Status != StatusSucceeded {
		t.Fatalf("export with the pinned Chromium: %+v error %+v", job, job.Error)
	}
}
```

`internal/storymaker/render/egress_smoke_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render/ -count=1 -run "TestBrowserStatusProbe|TestRealRender|TestEgressLayers" -v -timeout 15m`

Expected: PASS (`TestRealRenderOfTheFirewallStory` about 30 s, `TestRealRenderIsDeterministic` about 70 s, `TestEgressLayersEachStopTheProbe` about 40 s on the development box; the first Chrome launch takes 5 to 7 s)

- [ ] **Step 3: Run without the gate**

Run: `go test ./internal/storymaker/render/ -count=1`

Expected: PASS (the browser tests skip)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/egress_helpers_test.go internal/storymaker/render/browser_smoke_test.go internal/storymaker/render/egress_smoke_test.go
git commit -m "test(storymaker): real browser render, determinism and egress regression tests" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B11: Server API, player host page and desktop wiring

**Files:**
- Create: `internal/server/storymaker_store.go`
- Create: `internal/server/storymaker_service.go`
- Create: `internal/server/storymaker_handlers.go`
- Create: `internal/server/storymaker_player.go`
- Modify: `internal/server/server.go:78,364,2234` (import, fields, `allowDesktopIframe`)
- Modify: `internal/server/server_routes.go:527` (route)
- Modify: `internal/server/bluetooth_live.go:26` (capability `storymaker`)
- Modify: `internal/server/desktop_operation.go:44` (job cancel is `desktopStop`)
- Modify: `internal/server/server_shutdown.go:17,46` (close the runtime)
- Modify: `internal/server/config_patch.go:105` and `internal/server/config_handlers_main.go:692` (call `config.NormalizeStorymakerConfig`, part A's open item)
- Test: `internal/server/storymaker_api_test.go`

The API of contract section 5 under `/api/desktop/storymaker/`: `status`, `projects` (list, create, get, delete), `projects/{id}/story` (PUT with `If-Match`: 428 without it, 412 for a stale tag with the current tag in the `ETag` header, 422 `invalid_story` for an undecodable story, which is not saved), `projects/{id}/scenes/{scene}/code` (GET, PUT with `If-Match` or `If-None-Match: *`) and `.../bundle`, `projects/{id}/render` (202 `{job}`; 428, 412, 422 with issues, 409 `ffmpeg_unavailable`/`browser_unavailable`, 429 `busy`, 409 `idempotency_conflict`), `projects/{id}/jobs` (addition), `projects/{id}/events` (SSE), `jobs/{id}`, `jobs/{id}/cancel`, `player`. Errors are flat: `{"error": code, "code": code, "message": ..., "issues"?: [...]}`. Reads need `desktop:read`, mutations `desktop:write`; read-only refuses mutations with the shared 403 `desktop_readonly` (cancel is `desktopStop` and keeps working); a disabled feature answers 403 `storymaker_disabled` everywhere except `status`, which reports `enabled: false`. `storymakerStore` implements `storymaker.Store` and `render.Publisher` over `*desktop.Service` (etag checks run under the desktop mutation lock, `Remove` deletes a folder tree, outputs go through `WriteFileStreamConditional`). The player page is `GET /api/desktop/storymaker/player`: no inline script or style, the exact CSP of the spec with the request's origin, `Referrer-Policy: no-referrer`, `Cache-Control: no-store`, listed in `allowDesktopIframe`. GitNexus `impact` returned no answer for `securityHeadersMiddleware`, `desktopRequestOperation`, `HasCapability`, `shutdownDesktopStorage` and `closeRuntimeResources` (stale index); the callers were confirmed with `grep` and every edit is additive (one more path in a condition, one more `case`, one more call).

- [ ] **Step 1: Write the failing test**

`internal/server/storymaker_api_test.go`:

```go
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

// storymakerTestFFmpeg finds a usable ffmpeg for the render pre-checks (it is not started for a render).
func storymakerTestFFmpeg(t *testing.T) string {
	t.Helper()
	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}
	if dir := os.Getenv("AURAGO_TEST_FFMPEG_DIR"); dir != "" {
		if p := filepath.Join(dir, exe); fileReadable(p) {
			return p
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		if m, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "*FFmpeg*", "*", "bin", exe)); len(m) > 0 {
			return m[0]
		}
	}
	t.Skip("ffmpeg is not available (set AURAGO_TEST_FFMPEG_DIR)")
	return ""
}

func fileReadable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func storymakerAPIFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, readToken, _ := testDesktopPermissionServer(t)
	// The token of the app can read and write; the read token of the fixture only reads.
	writeToken, _, err := s.TokenManager.Create("storymaker rw", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopAppsHandlerTest(t, s)
	s.Logger = slog.Default()
	s.Cfg.Storymaker.Enabled = true
	if err := config.NormalizeStorymakerConfig(&s.Cfg.Storymaker); err != nil {
		t.Fatal(err)
	}
	s.storymakerUI = os.DirFS("../../ui")
	s.storymakerBrowsers = render.NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { return "/fake/chrome", true },
		func(context.Context, string) (string, error) { return "FakeChrome/1", nil })
	t.Cleanup(s.closeStorymaker)
	return s, readToken, writeToken
}

func storymakerRequest(s *Server, token, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, storymakerAPIBase+path, body)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, w.Body.String())
	}
}

type apiProject struct {
	Project storymaker.Project `json:"project"`
	Story   storymaker.Story   `json:"story"`
	ETag    string             `json:"etag"`
}

func createStorymakerProject(t *testing.T, s *Server, token string) apiProject {
	t.Helper()
	w := storymakerRequest(s, token, http.MethodPost, "/projects", strings.NewReader(`{"title":"Was ist eine Firewall?"}`), nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p apiProject
	decodeBody(t, w, &p)
	if w.Header().Get("ETag") != p.ETag || !storymaker.ValidProjectID(p.Project.ID) {
		t.Fatalf("create answer: %+v header %q", p, w.Header().Get("ETag"))
	}
	return p
}

func TestStorymakerGatesDisabledScopeAndReadOnly(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)

	// No credentials, wrong scope.
	if w := storymakerRequest(s, "", http.MethodGet, "/projects", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous GET /projects = %d", w.Code)
	}
	if w := storymakerRequest(s, readToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"x"}`), nil); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
		t.Fatalf("read token POST = %d %s", w.Code, w.Body.String())
	}

	// Read-only: mutations refuse with the shared middleware's code, reads work.
	project := createStorymakerProject(t, s, writeToken)
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/projects", `{"title":"y"}`},
		{http.MethodDelete, "/projects/" + project.Project.ID, ""},
		{http.MethodPut, "/projects/" + project.Project.ID + "/story", "{}"},
		{http.MethodPut, "/projects/" + project.Project.ID + "/scenes/s1/code", "x"},
		{http.MethodPost, "/projects/" + project.Project.ID + "/render", ""},
	} {
		w := storymakerRequest(s, writeToken, tc.method, tc.path, strings.NewReader(tc.body), map[string]string{"If-Match": project.ETag, "If-None-Match": "*"})
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_readonly") {
			t.Errorf("read-only %s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+project.Project.ID, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read-only GET project = %d %s", w.Code, w.Body.String())
	}
	var st struct {
		ReadOnly bool `json:"read_only"`
	}
	w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	decodeBody(t, w, &st)
	if !st.ReadOnly {
		t.Errorf("status read_only = false: %s", w.Body.String())
	}
	ro.VirtualDesktop.ReadOnly = false
	s.replaceConfigSnapshot(&ro)

	// Disabled: every endpoint refuses with storymaker_disabled, the status says so.
	off := *s.ConfigSnapshot()
	off.Storymaker.Enabled = false
	s.replaceConfigSnapshot(&off)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/projects"}, {http.MethodPost, "/projects"}, {http.MethodGet, "/projects/" + project.Project.ID},
		{http.MethodGet, "/player"}, {http.MethodGet, "/jobs/" + project.Project.ID}, {http.MethodGet, "/projects/" + project.Project.ID + "/events"},
	} {
		w := storymakerRequest(s, writeToken, tc.method, tc.path, strings.NewReader("{}"), nil)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"code":"storymaker_disabled"`) {
			t.Errorf("disabled %s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	w = storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Errorf("disabled status = %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerProjectCRUD(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	if p.Project.Aspect != "16:9" || p.Project.FPS != 30 || len(p.Story.Scenes) != 1 || p.Story.Scenes[0].Environment == "" {
		t.Fatalf("created project = %+v", p)
	}
	// Invalid create bodies.
	for _, body := range []string{`{}`, `{"title":"x","aspect":"4:3"}`, `{"title":"x","fps":60}`, `{"title":"x","extra":1}`, `nope`} {
		if w := storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(body), nil); w.Code != http.StatusBadRequest {
			t.Errorf("create %s = %d %s", body, w.Code, w.Body.String())
		}
	}
	other := storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"Vertical","aspect":"9:16","fps":25}`), nil)
	if other.Code != http.StatusCreated {
		t.Fatalf("vertical create = %d %s", other.Code, other.Body.String())
	}

	w := storymakerRequest(s, readToken, http.MethodGet, "/projects", nil, nil)
	var list struct {
		Projects []storymaker.Project `json:"projects"`
	}
	decodeBody(t, w, &list)
	if w.Code != http.StatusOK || len(list.Projects) != 2 {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}

	w = storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil)
	var got struct {
		Project    storymaker.Project           `json:"project"`
		Story      *storymaker.Story            `json:"story"`
		ETag       string                       `json:"etag"`
		Issues     []storymaker.Issue           `json:"issues"`
		CodeScenes []storymaker.CodeSceneStatus `json:"code_scenes"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK || got.ETag != p.ETag || got.Story == nil || got.Project.ID != p.Project.ID || got.CodeScenes == nil || got.Issues == nil {
		t.Fatalf("get = %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("ETag") != p.ETag {
		t.Errorf("ETag header = %q", w.Header().Get("ETag"))
	}

	// Unknown and malformed ids.
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/33333333-3333-4333-8333-333333333333", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown project = %d", w.Code)
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/..%2f..", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("bad id = %d", w.Code)
	}

	if w := storymakerRequest(s, writeToken, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("deleted project GET = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d", w.Code)
	}
}

func TestStorymakerStoryPutETagAndValidation(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	path := "/projects/" + p.Project.ID + "/story"
	story, err := os.ReadFile("../storymaker/testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	story = bytes.ReplaceAll(story, []byte("\r\n"), []byte("\n"))

	if w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), nil); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("PUT without If-Match = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": "W/\"weak\""}); w.Code != http.StatusBadRequest {
		t.Fatalf("PUT with a malformed If-Match = %d", w.Code)
	}
	stale := `"` + strings.Repeat("0", 64) + `"`
	w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": stale})
	if w.Code != http.StatusPreconditionFailed || w.Header().Get("ETag") != p.ETag {
		t.Fatalf("stale PUT = %d etag %q: %s", w.Code, w.Header().Get("ETag"), w.Body.String())
	}

	w = storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag     string                      `json:"etag"`
		Issues   []storymaker.Issue          `json:"issues"`
		Defaults []storymaker.DefaultApplied `json:"defaults_applied"`
		Story    storymaker.Story            `json:"story"`
	}
	decodeBody(t, w, &saved)
	if w.Code != http.StatusOK || saved.ETag == "" || saved.ETag == p.ETag || w.Header().Get("ETag") != saved.ETag || len(saved.Story.Scenes) != 3 {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	if saved.Defaults == nil || saved.Issues == nil {
		t.Fatalf("answer lacks defaults_applied or issues: %s", w.Body.String())
	}
	for _, is := range saved.Issues {
		if is.Severity == storymaker.SeverityError && is.Code != storymaker.CodeCodeError {
			t.Errorf("firewall.json gives an error issue: %+v", is)
		}
	}
	// s3 is a code scene without source: reported as code_error, the story is saved anyway.
	foundCode := false
	for _, is := range saved.Issues {
		foundCode = foundCode || (is.Code == storymaker.CodeCodeError && is.SceneID == "s3")
	}
	if !foundCode {
		t.Errorf("no code_error issue for the code scene without source: %+v", saved.Issues)
	}
	// The old version is stale now.
	if w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": p.ETag}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("second PUT with the old etag = %d", w.Code)
	}
	var again struct {
		ETag  string `json:"etag"`
		Story struct {
			Title string `json:"title"`
		} `json:"story"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil), &again)
	if again.ETag != saved.ETag || again.Story.Title != "Was ist eine Firewall?" {
		t.Fatalf("GET after save: %+v", again)
	}

	// A story with a bad reference is saved with error issues, so a draft is not lost.
	bad := `{"schema":"storymaker/1","title":"Draft","scenes":[{"id":"s1","environment":"env.nowhere"}]}`
	w = storymakerRequest(s, writeToken, http.MethodPut, path, strings.NewReader(bad), map[string]string{"If-Match": saved.ETag})
	var draft struct {
		ETag   string             `json:"etag"`
		Issues []storymaker.Issue `json:"issues"`
	}
	decodeBody(t, w, &draft)
	if w.Code != http.StatusOK || len(draft.Issues) == 0 || draft.Issues[0].Code != storymaker.CodeUnknownAsset {
		t.Fatalf("draft PUT = %d %s", w.Code, w.Body.String())
	}

	// Decode errors are 422 and nothing is written.
	for name, body := range map[string]string{
		"syntax":   `{"schema":`,
		"unknown":  `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1","narrtion":"typo"}]}`,
		"wrong":    `{"schema":"storymaker/2","title":"x","scenes":[]}`,
		"trailing": `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1"}]} {}`,
	} {
		w := storymakerRequest(s, writeToken, http.MethodPut, path, strings.NewReader(body), map[string]string{"If-Match": draft.ETag})
		var e struct {
			Error  string             `json:"error"`
			Code   string             `json:"code"`
			Issues []storymaker.Issue `json:"issues"`
		}
		decodeBody(t, w, &e)
		if w.Code != http.StatusUnprocessableEntity || e.Error != "invalid_story" || e.Code != "invalid_story" || len(e.Issues) == 0 {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	var current struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil), &current)
	if current.ETag != draft.ETag {
		t.Fatal("a refused story changed story.json")
	}

	// Too large.
	huge := `{"schema":"storymaker/1","title":"` + strings.Repeat("x", 1<<20) + `","scenes":[{"id":"s1"}]}`
	if w := storymakerRequest(s, writeToken, http.MethodPut, path, strings.NewReader(huge), map[string]string{"If-Match": draft.ETag}); w.Code != http.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), "too_large") {
		t.Fatalf("huge story = %d %.200s", w.Code, w.Body.String())
	}
}

func TestStorymakerCodeScenes(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	base := "/projects/" + p.Project.ID + "/scenes/s3/"
	src, err := os.ReadFile("../storymaker/testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))

	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), nil); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("code PUT without a precondition = %d", w.Code)
	}
	w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), map[string]string{"If-None-Match": "*"})
	var res struct {
		ETag   string                   `json:"etag"`
		OK     bool                     `json:"ok"`
		Errors []storymaker.BundleError `json:"errors"`
	}
	decodeBody(t, w, &res)
	if w.Code != http.StatusOK || !res.OK || res.ETag == "" || res.Errors == nil || len(res.Errors) != 0 {
		t.Fatalf("code PUT = %d %s", w.Code, w.Body.String())
	}
	// Creating again conflicts.
	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), map[string]string{"If-None-Match": "*"}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("second create = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), map[string]string{"If-Match": `"` + strings.Repeat("a", 64) + `"`}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale code PUT = %d", w.Code)
	}

	got := storymakerRequest(s, readToken, http.MethodGet, base+"code", nil, nil)
	if got.Code != http.StatusOK || got.Body.String() != string(src) || got.Header().Get("ETag") != res.ETag ||
		!strings.HasPrefix(got.Header().Get("Content-Type"), "text/javascript") || !strings.Contains(got.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("code GET = %d etag %q headers %v", got.Code, got.Header().Get("ETag"), got.Header())
	}
	bundle := storymakerRequest(s, readToken, http.MethodGet, base+"bundle", nil, nil)
	if bundle.Code != http.StatusOK || !strings.Contains(bundle.Body.String(), "__smScene") {
		t.Fatalf("bundle GET = %d %.200s", bundle.Code, bundle.Body.String())
	}

	// A syntax error saves the source, reports the position and removes the bundle.
	broken := []byte("export default function scene(api) {\n  api.duration(5;\n}\n")
	w = storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(broken), map[string]string{"If-Match": res.ETag})
	decodeBody(t, w, &res)
	if w.Code != http.StatusOK || res.OK || len(res.Errors) == 0 || res.Errors[0].Line != 2 {
		t.Fatalf("broken code PUT = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, base+"bundle", nil, nil); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "bundle_not_found") {
		t.Fatalf("bundle after a failed save = %d %s", w.Code, w.Body.String())
	}
	var project struct {
		CodeScenes []storymaker.CodeSceneStatus `json:"code_scenes"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil), &project)
	if len(project.CodeScenes) != 1 || project.CodeScenes[0].OK || len(project.CodeScenes[0].Errors) == 0 {
		t.Fatalf("code_scenes = %+v", project.CodeScenes)
	}

	// Limits and names.
	big := bytes.Repeat([]byte("a"), 64<<10+1)
	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(big), map[string]string{"If-Match": res.ETag}); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized code = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPut, "/projects/"+p.Project.ID+"/scenes/Bad_Name/code", bytes.NewReader(src), map[string]string{"If-None-Match": "*"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid scene id = %d", w.Code)
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID+"/scenes/nope/code", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing code = %d", w.Code)
	}
}

func TestStorymakerStatus(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	var st struct {
		Enabled bool `json:"enabled"`
		FFmpeg  struct {
			Ready bool `json:"ready"`
		} `json:"ffmpeg"`
		Browser struct {
			Ready  bool   `json:"ready"`
			Source string `json:"source"`
		} `json:"browser"`
		Limits         map[string]float64 `json:"limits"`
		Transport      string             `json:"transport"`
		CatalogVersion string             `json:"catalog_version"`
	}
	decodeBody(t, w, &st)
	if w.Code != http.StatusOK || !st.Enabled || !st.FFmpeg.Ready || !st.Browser.Ready || st.Browser.Source != "system" {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	if st.Limits["max_duration_seconds"] != 900 || st.Limits["max_scenes"] != 200 || st.Transport != render.DefaultTransport || len(st.CatalogVersion) != 12 {
		t.Errorf("status values: %+v", st)
	}
	// The configured transport and duration show up.
	s.Cfg.Storymaker.Render.Transport = "screenshot"
	s.Cfg.Storymaker.Render.MaxDurationSeconds = 120
	w = storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	decodeBody(t, w, &st)
	if st.Transport != "screenshot" || st.Limits["max_duration_seconds"] != 120 {
		t.Errorf("configured status: %s", w.Body.String())
	}
	// An FFmpeg that is not there is reported with its issue.
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	w = storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if !strings.Contains(w.Body.String(), `"ffmpeg":{"issue":`) && !strings.Contains(w.Body.String(), `"issue"`) {
		t.Errorf("missing ffmpeg is not explained: %s", w.Body.String())
	}
}

// renderFixture prepares a project with a valid story for the render endpoint tests.
func renderFixture(t *testing.T, run render.RunFunc) (*Server, string, apiProject) {
	t.Helper()
	s, _, writeToken := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.storymakerRunOverride = run
	p := createStorymakerProject(t, s, writeToken)
	return s, writeToken, p
}

func TestStorymakerRenderEndpointChecksAndQueue(t *testing.T) {
	var started atomic.Int32
	release := make(chan struct{})
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		started.Add(1)
		h.SetPhase(render.PhaseRendering)
		h.SetFrames(1, 10)
		select {
		case <-release:
			return render.Output{Path: "Documents/Storymaker/" + job.ProjectID + "/exports/x.mp4", Size: 3}, nil
		case <-ctx.Done():
			return render.Output{}, ctx.Err()
		}
	})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	path := "/projects/" + p.Project.ID + "/render"

	if w := storymakerRequest(s, token, http.MethodPost, path, nil, nil); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("render without If-Match = %d %s", w.Code, w.Body.String())
	}
	stale := `"` + strings.Repeat("0", 64) + `"`
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": stale}); w.Code != http.StatusPreconditionFailed || w.Header().Get("ETag") != p.ETag {
		t.Fatalf("stale render = %d etag %q", w.Code, w.Header().Get("ETag"))
	}
	if w := storymakerRequest(s, token, http.MethodPost, "/projects/33333333-3333-4333-8333-333333333333/render", nil, map[string]string{"If-Match": p.ETag}); w.Code != http.StatusNotFound {
		t.Fatalf("render of an unknown project = %d", w.Code)
	}
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": p.ETag, "Idempotency-Key": "bad key with spaces"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad idempotency key = %d", w.Code)
	}

	// An invalid story is refused with its issues.
	bad := `{"schema":"storymaker/1","title":"Bad","scenes":[{"id":"s1","environment":"env.nowhere"}]}`
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(bad), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	w = storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag})
	var refused struct {
		Code   string             `json:"code"`
		Issues []storymaker.Issue `json:"issues"`
	}
	decodeBody(t, w, &refused)
	if w.Code != http.StatusUnprocessableEntity || refused.Code != "invalid_story" || len(refused.Issues) == 0 {
		t.Fatalf("invalid story render = %d %s", w.Code, w.Body.String())
	}
	good := `{"schema":"storymaker/1","title":"Good","scenes":[{"id":"s1","narration":"Hallo."}]}`
	w = storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(good), map[string]string{"If-Match": saved.ETag})
	decodeBody(t, w, &saved)

	// Missing FFmpeg and missing browser are capability errors.
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag}); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ffmpeg_unavailable") {
		t.Fatalf("no ffmpeg = %d %s", w.Code, w.Body.String())
	}
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.Cfg.Storymaker.Render.BrowserPath = filepath.Join(t.TempDir(), "no-chrome")
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag}); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "browser_unavailable") {
		t.Fatalf("no browser = %d %s", w.Code, w.Body.String())
	}
	s.Cfg.Storymaker.Render.BrowserPath = ""

	// A valid request queues a job; the same Idempotency-Key returns it again.
	headers := map[string]string{"If-Match": saved.ETag, "Idempotency-Key": "export-1"}
	w = storymakerRequest(s, token, http.MethodPost, path, nil, headers)
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	if w.Code != http.StatusAccepted || accepted.Job.ID == "" || accepted.Job.ProjectID != p.Project.ID || accepted.Job.Kind != "render" {
		t.Fatalf("render = %d %s", w.Code, w.Body.String())
	}
	var replay struct {
		Job render.Job `json:"job"`
	}
	w = storymakerRequest(s, token, http.MethodPost, path, nil, headers)
	decodeBody(t, w, &replay)
	if w.Code != http.StatusAccepted || replay.Job.ID != accepted.Job.ID {
		t.Fatalf("idempotent replay = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag, "Idempotency-Key": "export-1-other"}); w.Code != http.StatusAccepted {
		t.Fatalf("second job = %d", w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for started.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	// GET job, list, cancel.
	w = storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil)
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK || got.Job.Status != "running" || got.Job.Phase != "rendering" || got.Job.FramesTotal != 10 {
		t.Fatalf("job = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, token, http.MethodGet, "/jobs/00000000-0000-4000-8000-000000000000", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown job = %d", w.Code)
	}
	w = storymakerRequest(s, token, http.MethodGet, "/projects/"+p.Project.ID+"/jobs", nil, nil)
	var listed struct {
		Jobs []render.Job `json:"jobs"`
	}
	decodeBody(t, w, &listed)
	if len(listed.Jobs) != 2 {
		t.Fatalf("job list = %s", w.Body.String())
	}

	// Cancel works while the desktop is read-only (desktopStop).
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, headers); w.Code != http.StatusForbidden {
		t.Fatalf("render while read-only = %d", w.Code)
	}
	w = storymakerRequest(s, token, http.MethodPost, "/jobs/"+accepted.Job.ID+"/cancel", nil, nil)
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel while read-only = %d %s", w.Code, w.Body.String())
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w = storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil)
		decodeBody(t, w, &got)
		if got.Job.Status == "cancelled" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got.Job.Status != "cancelled" {
		t.Fatalf("job after cancel = %+v", got.Job)
	}
	ro.VirtualDesktop.ReadOnly = false
	s.replaceConfigSnapshot(&ro)
	close(release)
}

func TestStorymakerRenderQueueIsBounded(t *testing.T) {
	release := make(chan struct{})
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return render.Output{}, nil
	})
	defer close(release)
	path := "/projects/" + p.Project.ID + "/render"
	busy := 0
	for i := 0; i < render.DefaultMaxQueued+3; i++ {
		w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": p.ETag})
		if w.Code == http.StatusTooManyRequests {
			if !strings.Contains(w.Body.String(), `"code":"busy"`) {
				t.Fatalf("429 without the busy code: %s", w.Body.String())
			}
			busy++
			continue
		}
		if w.Code != http.StatusAccepted {
			t.Fatalf("request %d = %d %s", i, w.Code, w.Body.String())
		}
	}
	// One job runs, sixteen wait, the rest is refused.
	if busy < 2 || busy > 3 {
		t.Fatalf("%d requests were refused as busy", busy)
	}
}

func TestStorymakerEventsStreamJobChanges(t *testing.T) {
	step := make(chan struct{})
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		h.SetPhase(render.PhaseRendering)
		select {
		case <-step:
		case <-ctx.Done():
			return render.Output{}, ctx.Err()
		}
		return render.Output{Path: "Documents/Storymaker/x/exports/a.mp4", Size: 9}, nil
	})
	srv := httptest.NewServer(handleStorymaker(s))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+p.Project.ID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("events = %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	events := make(chan render.Job, 32)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		var name string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: ") && name == "job":
				var d struct {
					Job render.Job `json:"job"`
				}
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &d) == nil {
					events <- d.Job
				}
			}
		}
		close(events)
	}()

	w := storymakerRequest(s, token, http.MethodPost, "/projects/"+p.Project.ID+"/render", nil, map[string]string{"If-Match": p.ETag})
	if w.Code != http.StatusAccepted {
		t.Fatalf("render = %d %s", w.Code, w.Body.String())
	}
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	seenRunning := false
	waitFor := func(pred func(render.Job) bool) render.Job {
		t.Helper()
		for {
			select {
			case j, ok := <-events:
				if !ok {
					t.Fatal("the event stream ended")
				}
				if j.ID == accepted.Job.ID && pred(j) {
					return j
				}
			case <-time.After(10 * time.Second):
				t.Fatal("no matching event")
			}
		}
	}
	waitFor(func(j render.Job) bool {
		seenRunning = j.Status == "running" && j.Phase == "rendering"
		return seenRunning
	})
	close(step)
	done := waitFor(func(j render.Job) bool { return j.Status == "succeeded" })
	if !seenRunning || done.Output == nil || done.Output.Size != 9 || done.Progress != 1 {
		t.Fatalf("final event = %+v", done)
	}
	cancel()

	// A new subscriber gets the latest job of the project at once.
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+p.Project.ID+"/events", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	res2, err := http.DefaultClient.Do(req2.WithContext(ctx2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	reader := bufio.NewReader(res2.Body)
	found := false
	for i := 0; i < 8 && !found; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		found = strings.Contains(line, accepted.Job.ID) && strings.Contains(line, `"succeeded"`)
	}
	if !found {
		t.Fatal("a new subscriber did not receive the finished job")
	}
}

func TestStorymakerDeleteProjectCancelsItsJobs(t *testing.T) {
	started := make(chan struct{}, 1)
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		started <- struct{}{}
		<-ctx.Done()
		return render.Output{}, ctx.Err()
	})
	w := storymakerRequest(s, token, http.MethodPost, "/projects/"+p.Project.ID+"/render", nil, map[string]string{"If-Match": p.ETag})
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	<-started
	if w := storymakerRequest(s, token, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var got struct {
			Job render.Job `json:"job"`
		}
		decodeBody(t, storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil), &got)
		if got.Job.Status == "cancelled" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the job of a deleted project kept running")
}

func TestStorymakerPlayerHostPage(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	r := httptest.NewRequest(http.MethodGet, storymakerAPIBase+"/player", nil)
	r.Host = "aurago.example:8443"
	r.Header.Set("Authorization", "Bearer "+readToken)
	w := httptest.NewRecorder()
	// Through the security middleware, as in production.
	securityHeadersMiddleware(handleStorymaker(s), false, false).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("player = %d %s", w.Code, w.Body.String())
	}
	origin := "http://aurago.example:8443"
	wantCSP := "sandbox allow-scripts; default-src 'none'; script-src " + origin + "/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src " + origin + "/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"
	h := w.Header()
	if h.Get("Content-Security-Policy") != wantCSP {
		t.Errorf("CSP =\n%s\nwant\n%s", h.Get("Content-Security-Policy"), wantCSP)
	}
	if h.Get("Referrer-Policy") != "no-referrer" || !strings.Contains(h.Get("Cache-Control"), "no-store") || h.Get("X-Frame-Options") == "DENY" || h.Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("headers: Referrer-Policy %q Cache-Control %q X-Frame-Options %q", h.Get("Referrer-Policy"), h.Get("Cache-Control"), h.Get("X-Frame-Options"))
	}
	if strings.Contains(h.Get("Content-Security-Policy"), "unsafe-inline") || strings.Contains(h.Get("Content-Security-Policy"), "unsafe-eval") {
		t.Error("the player CSP allows inline or eval")
	}
	wantPage := `<!doctype html><html data-sm-mode="app" data-sm-base="` + origin + `"><head><meta charset="utf-8"><link rel="stylesheet" href="` + origin + `/css/storymaker-player.css"></head>` +
		`<body><canvas id="sm-stage"></canvas><script src="` + origin + `/js/storymaker/host.bundle.js"></script></body></html>`
	if w.Body.String() != wantPage {
		t.Errorf("page:\n%s\nwant:\n%s", w.Body.String(), wantPage)
	}
	if strings.Contains(w.Body.String(), "<style") || strings.Contains(strings.ReplaceAll(w.Body.String(), `<script src=`, ""), "<script") {
		t.Error("inline script or style in the player page")
	}

	// HEAD answers the headers only; a hostile Host is refused; anonymous callers are refused.
	hr := httptest.NewRequest(http.MethodHead, storymakerAPIBase+"/player", nil)
	hr.Header.Set("Authorization", "Bearer "+readToken)
	hw := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(hw, hr)
	if hw.Code != http.StatusOK || hw.Body.Len() != 0 {
		t.Errorf("HEAD = %d with %d bytes", hw.Code, hw.Body.Len())
	}
	evil := httptest.NewRequest(http.MethodGet, storymakerAPIBase+"/player", nil)
	evil.Host = "evil.example/js/storymaker/ <script>"
	evil.Header.Set("Authorization", "Bearer "+readToken)
	ew := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(ew, evil)
	if ew.Code != http.StatusBadRequest {
		t.Errorf("hostile Host = %d", ew.Code)
	}
	if w := storymakerRequest(s, "", http.MethodGet, "/player", nil, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous player = %d", w.Code)
	}
	// Behind a trusted proxy that says https.
	if !strings.HasPrefix(StorymakerPlayerCSP("https://a.example"), "sandbox allow-scripts;") {
		t.Error("CSP builder")
	}
}

func TestStorymakerCapabilityAndOperations(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	c := desktopCapabilities{s}
	if !c.HasCapability("storymaker") {
		t.Fatal("storymaker capability is off although the feature is on")
	}
	off := *s.ConfigSnapshot()
	off.Storymaker.Enabled = false
	s.replaceConfigSnapshot(&off)
	if c.HasCapability("storymaker") {
		t.Fatal("storymaker capability is on although the feature is off")
	}
	off.Storymaker.Enabled = true
	off.VirtualDesktop.Enabled = false
	s.replaceConfigSnapshot(&off)
	if c.HasCapability("storymaker") {
		t.Fatal("storymaker capability is on without the virtual desktop")
	}

	cancel := httptest.NewRequest(http.MethodPost, storymakerAPIBase+"/jobs/11111111-1111-4111-8111-111111111111/cancel", nil)
	if desktopRequestOperation(cancel) != desktopStop {
		t.Error("job cancel is not a desktopStop operation")
	}
	for _, path := range []string{"/projects", "/projects/x/render", "/jobs/x"} {
		r := httptest.NewRequest(http.MethodPost, storymakerAPIBase+path, nil)
		if desktopRequestOperation(r) != desktopWrite {
			t.Errorf("POST %s is not a write operation", path)
		}
	}
}

func TestStorymakerConfigValidationRejectsBadValuesBeforeWrite(t *testing.T) {
	for _, setup := range []bool{true, false} {
		name := "config"
		if setup {
			name = "setup"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			original := []byte("server:\n  host: 127.0.0.1\nstorymaker:\n  enabled: true\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConfigPath = path
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			for _, patch := range []string{`{"storymaker":{"render":{"timeout_seconds":5}}}`, `{"storymaker":{"render":{"transport":"carrier-pigeon"}}}`, `{"storymaker":{"render":{"browser_path":"relative/chrome"}}}`} {
				if setup {
					var m map[string]interface{}
					if err := json.Unmarshal([]byte(patch), &m); err != nil {
						t.Fatal(err)
					}
					if _, err := applyConfigPatch(s, m); err == nil || !strings.Contains(err.Error(), "storymaker:") {
						t.Fatalf("patch %s: %v", patch, err)
					}
				} else {
					rec := httptest.NewRecorder()
					handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
					if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "storymaker:") {
						t.Fatalf("patch %s: %d %s", patch, rec.Code, rec.Body.String())
					}
				}
			}
			stored, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(stored, original) {
				t.Fatalf("a rejected patch changed the config file: %v", err)
			}
		})
	}
}

func TestStorymakerStoreOverTheDesktopWorkspace(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	// Any authorised request resolves the desktop service and points the store at it.
	p := createStorymakerProject(t, s, writeToken)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	st := rt.store
	ctx := context.Background()
	dir := "Documents/Storymaker/" + p.Project.ID

	// ReadFile / WriteFile with etags.
	data, etag, err := st.ReadFile(ctx, dir+"/story.json")
	if err != nil || etag != storymaker.ETag(data) {
		t.Fatalf("ReadFile = %v %q", err, etag)
	}
	if _, err := st.WriteFile(ctx, dir+"/story.json", []byte("{}"), `"`+strings.Repeat("1", 64)+`"`); !errors.Is(err, storymaker.ErrConflict) {
		t.Fatalf("stale WriteFile = %v", err)
	}
	if _, err := st.WriteFile(ctx, dir+"/story.json", []byte("{}"), storymaker.MustNotExist); !errors.Is(err, storymaker.ErrConflict) {
		t.Fatalf("MustNotExist on an existing file = %v", err)
	}
	newTag, err := st.WriteFile(ctx, dir+"/extra/notes.txt", []byte("hi"), storymaker.MustNotExist)
	if err != nil || newTag != storymaker.ETag([]byte("hi")) {
		t.Fatalf("create in a new folder = %v %q", err, newTag)
	}
	if _, err := st.WriteFile(ctx, dir+"/extra/notes.txt", []byte("hello"), newTag); err != nil {
		t.Fatalf("WriteFile with the current etag: %v", err)
	}
	if _, err := st.WriteFile(ctx, dir+"/extra/missing.txt", []byte("x"), newTag); !errors.Is(err, storymaker.ErrConflict) {
		t.Fatalf("etag for a file that does not exist = %v", err)
	}
	if _, _, err := st.ReadFile(ctx, dir+"/extra/none.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadFile of a missing file = %v", err)
	}

	// List: base names, sorted; a missing folder is (nil, nil).
	names, err := st.List(ctx, dir)
	if err != nil || len(names) < 3 {
		t.Fatalf("List = %v %v", names, err)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("List is not sorted: %v", names)
		}
	}
	if names, err := st.List(ctx, dir+"/nope"); err != nil || names != nil {
		t.Fatalf("List of a missing folder = %v %v", names, err)
	}

	// Remove: a folder tree goes, a missing path is fine.
	if err := st.Remove(ctx, dir+"/extra"); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove(ctx, dir+"/extra"); err != nil {
		t.Fatalf("Remove of a missing path = %v", err)
	}
	if _, _, err := st.ReadFile(ctx, dir+"/extra/notes.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file survived its folder: %v", err)
	}

	// Publisher: create-only streams.
	if err := st.WriteStream(ctx, dir+"/exports/a.mp4", strings.NewReader("video"), 1<<20); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteStream(ctx, dir+"/exports/a.mp4", strings.NewReader("again"), 1<<20); err == nil {
		t.Fatal("WriteStream replaced an existing file")
	}
	if err := st.WriteStream(ctx, dir+"/exports/b.mp4", strings.NewReader("too long"), 3); err == nil {
		t.Fatal("WriteStream ignored its size budget")
	}
	if err := st.WriteBytes(ctx, dir+"/exports/job.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteBytes(ctx, dir+"/exports/job.json", []byte("{\"x\":1}")); err != nil {
		t.Fatalf("WriteBytes replace: %v", err)
	}
}

func TestStorymakerSettingsRefuseWhenSwitchedOffOrReadOnly(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	st, err := s.storymakerSettings(context.Background())
	if err != nil || st.FFmpegPath == "" || st.MaxDurationSeconds != 900 || st.Limits.MaxDurationSeconds != 900 || st.TimeoutSeconds != 3600 {
		t.Fatalf("settings = %+v, %v", st, err)
	}
	off := *s.ConfigSnapshot()
	off.Storymaker.Enabled = false
	s.replaceConfigSnapshot(&off)
	var je *render.JobError
	if _, err := s.storymakerSettings(context.Background()); !errors.As(err, &je) || je.Code != render.CodeDisabled {
		t.Fatalf("disabled: %v", err)
	}
	off.Storymaker.Enabled, off.VirtualDesktop.ReadOnly = true, true
	s.replaceConfigSnapshot(&off)
	if _, err := s.storymakerSettings(context.Background()); !errors.As(err, &je) || je.Code != render.CodeRenderFailed || !strings.Contains(je.Message, "read-only") {
		t.Fatalf("read-only: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/server/ -run "TestStorymaker" -count=1`

Expected: FAIL (build failed: `s.storymakerUI undefined (type *Server has no field or method storymakerUI)`, `undefined: storymakerAPIBase`, `undefined: handleStorymaker`)

- [ ] **Step 3: Implement**

First the edits to existing files (diff against main HEAD `561685cde`; the drafting base `0248bc133` differs only in the hunk offsets of `config_handlers_main.go`):

```diff
diff --git a/internal/server/bluetooth_live.go b/internal/server/bluetooth_live.go
index 82f2834c5..9c8181dd9 100644
--- a/internal/server/bluetooth_live.go
+++ b/internal/server/bluetooth_live.go
@@ -24,6 +24,8 @@ func (c desktopCapabilities) HasCapability(name string) bool {
 		return c.s.localWikipediaAvailable()
 	case "retro_games":
 		return c.s.retroGamesAvailable()
+	case "storymaker":
+		return c.s.storymakerAvailable()
 	case "bookshelf":
 		cfg := c.s.ConfigSnapshot()
 		return cfg != nil && cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.BookshelfEnabled
diff --git a/internal/server/config_handlers_main.go b/internal/server/config_handlers_main.go
index 1329c1c87..18552be13 100644
--- a/internal/server/config_handlers_main.go
+++ b/internal/server/config_handlers_main.go
@@ -690,6 +690,10 @@ func handleUpdateConfig(s *Server) http.HandlerFunc {
 			jsonError(w, err.Error(), http.StatusBadRequest)
 			return
 		}
+		if err := config.NormalizeStorymakerConfig(&validateCfg.Storymaker); err != nil {
+			jsonError(w, err.Error(), http.StatusBadRequest)
+			return
+		}
 		if err := validateCYDCreatorSave(previousCYDCreator, validateCfg.CYDCreator); err != nil {
 			jsonError(w, err.Error(), http.StatusBadRequest)
 			return
diff --git a/internal/server/config_patch.go b/internal/server/config_patch.go
index 918068f2e..48bfd29ea 100644
--- a/internal/server/config_patch.go
+++ b/internal/server/config_patch.go
@@ -103,6 +103,9 @@ func applyConfigPatch(s *Server, patch map[string]interface{}) (*config.Config,
 	if err := config.NormalizeVideoStudioConfig(&validateCfg.VideoStudio); err != nil {
 		return nil, err
 	}
+	if err := config.NormalizeStorymakerConfig(&validateCfg.Storymaker); err != nil {
+		return nil, err
+	}
 	if err := validateCYDCreatorSave(previousCYDCreator, validateCfg.CYDCreator); err != nil {
 		return nil, err
 	}
diff --git a/internal/server/desktop_operation.go b/internal/server/desktop_operation.go
index fd8a07461..536582ecb 100644
--- a/internal/server/desktop_operation.go
+++ b/internal/server/desktop_operation.go
@@ -42,6 +42,7 @@ func desktopRequestOperation(r *http.Request) desktopOperation {
 		parts := strings.Split(strings.Trim(path, "/"), "/")
 		if (len(parts) == 5 && parts[0] == "api" && parts[1] == "game-maker" && parts[2] == "jobs" && parts[4] == "cancel") ||
 			(len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "video-studio" && parts[3] == "jobs" && parts[5] == "cancel") ||
+			(len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "storymaker" && parts[3] == "jobs" && parts[5] == "cancel") ||
 			(len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "personal-radio" && parts[3] == "stations" && (parts[5] == "stop" || parts[5] == "pause")) ||
 			path == "/api/desktop/rtl-sdr/stop" {
 			return desktopStop
diff --git a/internal/server/server.go b/internal/server/server.go
index d3b4af001..a1cb7c868 100644
--- a/internal/server/server.go
+++ b/internal/server/server.go
@@ -76,6 +76,7 @@ import (
 	"aurago/internal/speechlab"
 	"aurago/internal/speechlab/deployer"
 	"aurago/internal/sqlconnections"
+	"aurago/internal/storymaker/render"
 	"aurago/internal/telegram"
 	"aurago/internal/tools"
 	"aurago/internal/tsnetnode"
@@ -362,6 +363,18 @@ type Server struct {
 	videoStudioClosed         bool
 	videoStudioConfigRevoking atomic.Bool
 
+	// Storymaker: the lazily created runtime (project store, catalog, render jobs). storymakerUI is
+	// the ui tree it reads; nil selects the embedded asset set (tests inject a tree).
+	storymakerMu     sync.Mutex
+	storymaker       *storymakerRuntime
+	storymakerClosed bool
+	storymakerUI     fs.FS
+	// Test seams: a ready-made browser resolver and a replacement for the render function.
+	storymakerBrowsers    *render.Browsers
+	storymakerRunOverride render.RunFunc
+	// storymakerFrameObserver sees every rendered frame (the parity test hashes them).
+	storymakerFrameObserver func(n int, transport string, data []byte)
+
 	// Docker Manager: the host registry and its SQLite store (docker_manager.db).
 	// DockerHosts stays nil when the store could not be opened; DockerStacks (the
 	// stacks service on the same database) stays nil when it could not be opened.
@@ -2231,7 +2244,7 @@ func (s *Server) serveWithShutdown(server, redirectServer, ttsServer *http.Serve
 func securityHeadersMiddleware(next http.Handler, tlsActive, behindProxy bool) http.Handler {
 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
 		path := r.URL.Path
-		allowDesktopIframe := path == "/api/desktop/bookshelf/reader" || strings.HasPrefix(path, "/files/desktop/") ||
+		allowDesktopIframe := path == "/api/desktop/bookshelf/reader" || path == "/api/desktop/storymaker/player" || strings.HasPrefix(path, "/files/desktop/") ||
 			strings.HasPrefix(path, "/api/go2rtc/viewer/") ||
 			strings.HasPrefix(path, "/api/game-maker/preview/")
 
diff --git a/internal/server/server_routes.go b/internal/server/server_routes.go
index 6dd9ded54..73989a852 100644
--- a/internal/server/server_routes.go
+++ b/internal/server/server_routes.go
@@ -525,6 +525,7 @@ func (s *Server) run(shutdownCh chan struct{}) error {
 	mux.HandleFunc("/api/desktop/bootstrap", handleDesktopBootstrap(s))
 	mux.HandleFunc("/api/desktop/video-studio", handleVideoStudio(s))
 	mux.HandleFunc("/api/desktop/video-studio/", handleVideoStudio(s))
+	mux.HandleFunc("/api/desktop/storymaker/", handleStorymaker(s))
 	mux.HandleFunc("/api/desktop/notes", handleDesktopNotes(s))
 	mux.HandleFunc("/api/desktop/tresor", handleDesktopTresor(s))
 	mux.HandleFunc("/api/desktop/tresor/", handleDesktopTresor(s))
diff --git a/internal/server/server_shutdown.go b/internal/server/server_shutdown.go
index 4cb71a6e1..3352a92ef 100644
--- a/internal/server/server_shutdown.go
+++ b/internal/server/server_shutdown.go
@@ -15,6 +15,7 @@ import (
 // to reopen the service afterwards.
 func (s *Server) shutdownDesktopStorage() {
 	s.closeVideoStudioManager()
+	s.closeStorymaker()
 	s.revokeDesktopRuns()
 	s.DesktopMu.Lock()
 	s.desktopClosed = true
@@ -44,6 +45,7 @@ func (s *Server) closeRuntimeResources() {
 	}
 	s.closeProjectKnowledge()
 	s.closeVideoStudioManager()
+	s.closeStorymaker()
 	s.stopRocketChatBot()
 	s.stopTelegramBot()
 	s.stopWhatsApp()
```

Then the new files:

`internal/server/storymaker_store.go`:

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"sync/atomic"

	"aurago/internal/desktop"
	"aurago/internal/storymaker"
)

// storymakerStore implements storymaker.Store (and render.Publisher) over the desktop workspace.
// The desktop service can be replaced when the configuration changes, so the store holds an atomic
// pointer that every request refreshes with the service it just obtained.
type storymakerStore struct {
	svc atomic.Pointer[desktop.Service]
}

// use points the store at the current desktop service.
func (st *storymakerStore) use(svc *desktop.Service) { st.svc.Store(svc) }

func (st *storymakerStore) service() (*desktop.Service, error) {
	svc := st.svc.Load()
	if svc == nil {
		return nil, errors.New("storymaker: the desktop workspace is not available")
	}
	return svc, nil
}

// mapStoreError turns the desktop errors that carry meaning for the project store into its errors.
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storymaker: %w", fs.ErrNotExist)
	}
	var conflict *desktopFileConflict
	if errors.As(err, &conflict) {
		return storymaker.ErrConflict
	}
	var pathConflict *desktop.PathConflict
	if errors.As(err, &pathConflict) {
		return storymaker.ErrConflict
	}
	return err
}

// ReadFile implements storymaker.Store.
func (st *storymakerStore) ReadFile(ctx context.Context, p string) ([]byte, string, error) {
	svc, err := st.service()
	if err != nil {
		return nil, "", err
	}
	data, _, err := svc.ReadFileBytes(ctx, p)
	if err != nil {
		return nil, "", mapStoreError(err)
	}
	return data, storymaker.ETag(data), nil
}

// WriteFile implements storymaker.Store: "" is unconditional, storymaker.MustNotExist creates only,
// anything else is the etag the file must still have. The check runs under the desktop mutation
// lock, in the same step as the write.
func (st *storymakerStore) WriteFile(ctx context.Context, p string, data []byte, ifMatch string) (string, error) {
	svc, err := st.service()
	if err != nil {
		return "", err
	}
	var precondition desktop.FileWritePrecondition
	switch ifMatch {
	case "":
	case storymaker.MustNotExist:
		precondition = func(state desktop.FileWriteState) error {
			if state.Exists {
				return storymaker.ErrConflict
			}
			return nil
		}
	default:
		precondition = func(state desktop.FileWriteState) error {
			if !state.Exists || desktop.NoteVersion(state.Data) != ifMatch {
				return storymaker.ErrConflict
			}
			return nil
		}
	}
	if _, err := svc.WriteFileBytesConditional(ctx, p, data, desktop.SourceUser, precondition); err != nil {
		return "", mapStoreError(err)
	}
	return storymaker.ETag(data), nil
}

// Remove implements storymaker.Store: a missing path is not an error, a folder goes with everything in it.
func (st *storymakerStore) Remove(ctx context.Context, p string) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	if err := svc.DeletePath(ctx, p, desktop.SourceUser); err != nil {
		if mapped := mapStoreError(err); errors.Is(mapped, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// List implements storymaker.Store: the base names of the direct children, sorted; a missing folder is (nil, nil).
func (st *storymakerStore) List(ctx context.Context, dir string) ([]string, error) {
	svc, err := st.service()
	if err != nil {
		return nil, err
	}
	entries, err := svc.ListFiles(ctx, dir)
	if err != nil {
		if mapped := mapStoreError(err); errors.Is(mapped, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names, nil
}

// MkdirAll implements storymaker.Store.
func (st *storymakerStore) MkdirAll(ctx context.Context, dir string) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	return svc.CreateDirectory(ctx, dir, desktop.SourceUser)
}

// WriteStream implements render.Publisher: it creates p from r and fails when p exists. The write
// goes through WriteFileStreamConditional, which stages the data and renames it into place.
func (st *storymakerStore) WriteStream(ctx context.Context, p string, r io.Reader, max int64) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	_, err = svc.WriteFileStreamConditional(ctx, p, r, max, desktop.SourceUser, nil)
	return err
}

// WriteBytes implements render.Publisher: a small file, replaced when it exists.
func (st *storymakerStore) WriteBytes(ctx context.Context, p string, data []byte) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	return svc.WriteFileBytes(ctx, p, data, desktop.SourceUser)
}
```

`internal/server/storymaker_service.go`:

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

const storymakerCatalogPath = "img/storymaker/catalog.json"

// storymakerRuntime is everything Storymaker keeps alive between requests: the project store, the
// catalog, the browser resolver, the render job manager and its runner.
type storymakerRuntime struct {
	dataDir  string
	store    *storymakerStore
	projects *storymaker.ProjectStore
	catalog  *storymaker.Catalog
	browsers *render.Browsers
	runner   *render.Runner
	jobs     *render.Manager
}

// storymakerUIFiles returns the ui tree the catalog and the player bundles are read from: the
// injected one (tests) or the embedded asset set.
func (s *Server) storymakerUIFiles() fs.FS {
	if s.storymakerUI != nil {
		return s.storymakerUI
	}
	return uiFiles
}

// storymakerDir is the folder of jobs.json, the staging area and the pinned browser.
func storymakerDir(dataDir string) string { return filepath.Join(dataDir, "storymaker") }

// storymakerRuntime returns the runtime for the current data directory, creating it on first use.
func (s *Server) storymakerRuntime() (*storymakerRuntime, error) {
	if s == nil {
		return nil, errors.New("storymaker unavailable")
	}
	s.storymakerMu.Lock()
	defer s.storymakerMu.Unlock()
	if s.storymakerClosed {
		return nil, errors.New("storymaker is shutting down")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" {
		return nil, errors.New("the data directory is unavailable")
	}
	dataDir := normalizedVideoStudioRoot(cfg.Directories.DataDir)
	if s.storymaker != nil {
		if s.storymaker.dataDir == dataDir {
			return s.storymaker, nil
		}
		old := s.storymaker
		s.storymaker = nil
		old.jobs.Close()
	}
	rt, err := s.newStorymakerRuntime(dataDir)
	if err != nil {
		return nil, err
	}
	s.storymaker = rt
	return rt, nil
}

func (s *Server) newStorymakerRuntime(dataDir string) (*storymakerRuntime, error) {
	ui := s.storymakerUIFiles()
	catalog, err := storymaker.LoadCatalog(ui, storymakerCatalogPath)
	if err != nil {
		return nil, fmt.Errorf("the Storymaker catalog could not be loaded: %w", err)
	}
	dir := storymakerDir(dataDir)
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	store := &storymakerStore{}
	rt := &storymakerRuntime{
		dataDir:  dataDir,
		store:    store,
		projects: storymaker.NewProjectStore(store, render.ProjectsRoot),
		catalog:  catalog,
		browsers: render.NewBrowsers(dir),
	}
	if s.storymakerBrowsers != nil {
		rt.browsers = s.storymakerBrowsers
	}
	rt.runner = render.NewRunner(render.Deps{
		Dir: dir, UI: ui, Catalog: catalog, Projects: rt.projects, Browsers: rt.browsers, Publisher: store,
		Settings: func(ctx context.Context) (render.Settings, error) { return s.storymakerSettings(ctx) },
		Logger:   logger, FrameObserver: s.storymakerFrameObserver,
	})
	run := rt.runner.Run
	if s.storymakerRunOverride != nil {
		run = s.storymakerRunOverride
	}
	jobs, err := render.NewManager(render.ManagerOptions{Dir: dir, Run: run, Logger: logger})
	if err != nil {
		return nil, err
	}
	rt.jobs = jobs
	return rt, nil
}

// closeStorymaker stops the job manager (a running render ends as interrupted).
func (s *Server) closeStorymaker() {
	if s == nil {
		return
	}
	s.storymakerMu.Lock()
	s.storymakerClosed = true
	rt := s.storymaker
	s.storymaker = nil
	s.storymakerMu.Unlock()
	if rt != nil {
		rt.jobs.Close()
	}
}

// normalizedStorymakerConfig returns a copy of the configuration with the Storymaker section
// normalized.
func normalizedStorymakerConfig(s *Server) (*config.Config, error) {
	if s == nil {
		return nil, errors.New("server unavailable")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, errors.New("configuration unavailable")
	}
	cp := *cfg
	if err := config.NormalizeStorymakerConfig(&cp.Storymaker); err != nil {
		return nil, err
	}
	return &cp, nil
}

// storymakerLimits is storymaker.DefaultLimits with the configured maximum duration, so the
// validator and the render agree.
func storymakerLimits(cfg *config.Config) storymaker.Limits {
	lim := storymaker.DefaultLimits()
	lim.MaxDurationSeconds = float64(cfg.Storymaker.Render.MaxDurationSeconds)
	return lim
}

// storymakerSettings reads the render settings of a job: the Storymaker section, FFmpeg and the
// encoder (selected like Video Studio does and cached the same way).
func (s *Server) storymakerSettings(ctx context.Context) (render.Settings, error) {
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		return render.Settings{}, err
	}
	// A job that waited in the queue while the feature was switched off, or the desktop made read-only,
	// ends here instead of rendering for nothing.
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		return render.Settings{}, render.Fail(render.CodeDisabled, "Storymaker was switched off before this export started.")
	}
	if cfg.VirtualDesktop.ReadOnly {
		return render.Settings{}, render.Fail(render.CodeRenderFailed, "The desktop is read-only; the export was not started.")
	}
	ffmpeg, err := videostudio.ResolveFFmpeg(cfg.VideoStudio.FFmpegPath)
	if err != nil {
		return render.Settings{}, fmt.Errorf("FFmpeg is not available: %v", err)
	}
	enc := videoStudioEncoder(ctx, cfg)
	return render.Settings{
		TimeoutSeconds:     cfg.Storymaker.Render.TimeoutSeconds,
		MaxDurationSeconds: cfg.Storymaker.Render.MaxDurationSeconds,
		BrowserPath:        cfg.Storymaker.Render.BrowserPath,
		Transport:          cfg.Storymaker.Render.Transport,
		FFmpegPath:         ffmpeg,
		Encoder:            enc.Encoder,
		EncoderName:        enc.Name,
		Limits:             storymakerLimits(cfg),
	}, nil
}

// storymakerAvailable is the capability behind Requires: ["storymaker"]: the feature flag and an
// enabled virtual desktop.
func (s *Server) storymakerAvailable() bool {
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.Storymaker.Enabled && cfg.VirtualDesktop.Enabled
}

// warmStorymakerBrowser probes the browser in the background so the first status request finds the
// result cached.
func (rt *storymakerRuntime) warmBrowser(cfg *config.Config) {
	path := cfg.Storymaker.Render.BrowserPath
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_ = rt.browsers.Status(ctx, path)
	}()
}
```

`internal/server/storymaker_handlers.go`:

```go
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

const storymakerAPIBase = "/api/desktop/storymaker"

// storymakerJSONLimit bounds small JSON request bodies (create project).
const storymakerJSONLimit = 16 << 10

var storymakerIdempotencyKeyPattern = regexp.MustCompile(`^[\x21-\x7e]{1,128}$`)

// storymakerAccess is what a handler gets after the gates passed.
type storymakerAccess struct {
	cfg *config.Config
	rt  *storymakerRuntime
	svc *desktop.Service
}

func writeStorymakerJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// issuesOrEmpty caps the issues for an HTTP answer and never returns nil (the JSON stays an array).
func issuesOrEmpty(issues []storymaker.Issue) []storymaker.Issue {
	out := storymaker.CapIssues(issues, storymaker.MaxHTTPIssues)
	if out == nil {
		return []storymaker.Issue{}
	}
	return out
}

// writeStorymakerError writes the flat error document of the desktop APIs:
// {"error": code, "code": code, "message": ..., "issues"?: [...]}.
func writeStorymakerError(w http.ResponseWriter, status int, code, message string, issues []storymaker.Issue) {
	body := map[string]any{"error": code, "code": code, "message": message}
	if len(issues) > 0 {
		body["issues"] = issuesOrEmpty(issues)
	}
	writeStorymakerJSON(w, status, body)
}

func writeStorymakerMethodError(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeStorymakerError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed for this endpoint.", nil)
}

// storymakerAuthorize runs the gates every endpoint shares: desktop scope and operation (read-only
// refuses mutations with 403 desktop_readonly), the Storymaker feature flag (403
// storymaker_disabled), the desktop being enabled and the runtime. withService also resolves the
// desktop workspace.
func storymakerAuthorize(s *Server, w http.ResponseWriter, r *http.Request, scope string, op desktopOperation, withService bool) (*storymakerAccess, bool) {
	if !requireDesktopOperation(s, w, r, scope, op) {
		return nil, false
	}
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "configuration_invalid", "The Storymaker configuration is invalid.", nil)
		return nil, false
	}
	if !cfg.Storymaker.Enabled {
		writeStorymakerError(w, http.StatusForbidden, "storymaker_disabled", "Storymaker is disabled in the configuration.", nil)
		return nil, false
	}
	if !cfg.VirtualDesktop.Enabled {
		writeStorymakerError(w, http.StatusServiceUnavailable, "desktop_disabled", "The virtual desktop is disabled in the configuration.", nil)
		return nil, false
	}
	rt, err := s.storymakerRuntime()
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "storage_unavailable", "Storymaker storage is unavailable: "+err.Error(), nil)
		return nil, false
	}
	acc := &storymakerAccess{cfg: cfg, rt: rt}
	if withService {
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			writeStorymakerError(w, http.StatusServiceUnavailable, "desktop_unavailable", "The virtual desktop storage is unavailable.", nil)
			return nil, false
		}
		rt.store.use(svc)
		acc.svc = svc
	}
	return acc, true
}

func handleStorymaker(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, storymakerAPIBase+"/") {
			writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, storymakerAPIBase), "/"), "/")
		switch {
		case len(parts) == 1 && parts[0] == "status":
			handleStorymakerStatus(s, w, r)
		case len(parts) == 1 && parts[0] == "player":
			handleStorymakerPlayer(s, w, r)
		case len(parts) == 1 && parts[0] == "projects":
			handleStorymakerProjects(s, w, r)
		case len(parts) >= 2 && parts[0] == "projects":
			if !storymaker.ValidProjectID(parts[1]) {
				writeStorymakerError(w, http.StatusNotFound, "project_not_found", "Project not found.", nil)
				return
			}
			id := parts[1]
			switch {
			case len(parts) == 2:
				handleStorymakerProject(s, w, r, id)
			case len(parts) == 3 && parts[2] == "story":
				handleStorymakerStory(s, w, r, id)
			case len(parts) == 3 && parts[2] == "render":
				handleStorymakerRender(s, w, r, id)
			case len(parts) == 3 && parts[2] == "events":
				handleStorymakerEvents(s, w, r, id)
			case len(parts) == 3 && parts[2] == "jobs":
				handleStorymakerProjectJobs(s, w, r, id)
			case len(parts) == 5 && parts[2] == "scenes" && parts[4] == "code":
				handleStorymakerCode(s, w, r, id, parts[3])
			case len(parts) == 5 && parts[2] == "scenes" && parts[4] == "bundle":
				handleStorymakerBundle(s, w, r, id, parts[3])
			default:
				writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
			}
		case len(parts) == 2 && parts[0] == "jobs":
			handleStorymakerJob(s, w, r, parts[1])
		case len(parts) == 3 && parts[0] == "jobs" && parts[2] == "cancel":
			handleStorymakerJobCancel(s, w, r, parts[1])
		default:
			writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
		}
	}
}

// mapStorymakerError answers a project store error.
func mapStorymakerError(w http.ResponseWriter, err error, notFoundCode string) {
	var decode *storymaker.DecodeError
	switch {
	case errors.Is(err, storymaker.ErrInvalidID):
		writeStorymakerError(w, http.StatusBadRequest, "invalid_id", err.Error(), nil)
	case errors.Is(err, storymaker.ErrInvalidArgument):
		writeStorymakerError(w, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
	case errors.Is(err, storymaker.ErrTooLarge):
		writeStorymakerError(w, http.StatusRequestEntityTooLarge, "too_large", err.Error(), nil)
	case errors.Is(err, storymaker.ErrConflict):
		writeStorymakerError(w, http.StatusPreconditionFailed, "precondition_failed", "The document changed since it was loaded.", nil)
	case errors.Is(err, fs.ErrNotExist):
		writeStorymakerError(w, http.StatusNotFound, notFoundCode, "Not found.", nil)
	case errors.As(err, &decode):
		writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_story", decode.Message, decode.Issues)
	default:
		writeStorymakerError(w, http.StatusInternalServerError, "storage_failed", "The project storage failed.", nil)
	}
}

// storymakerETagPattern is the strong etag form: a quoted SHA-256.
var storymakerETagPattern = regexp.MustCompile(`^"[0-9a-f]{64}"$`)

// requireStorymakerMatch reads If-Match; it answers 428 without it and 400 for a malformed value.
func requireStorymakerMatch(w http.ResponseWriter, r *http.Request) (string, bool) {
	match := strings.TrimSpace(r.Header.Get("If-Match"))
	if match == "" {
		writeStorymakerError(w, http.StatusPreconditionRequired, "precondition_required", "Send the current version in If-Match.", nil)
		return "", false
	}
	if !storymakerETagPattern.MatchString(match) {
		writeStorymakerError(w, http.StatusBadRequest, "precondition_invalid", "If-Match must be a version taken from an earlier answer.", nil)
		return "", false
	}
	return match, true
}

func handleStorymakerStatus(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
		return
	}
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "configuration_invalid", "The Storymaker configuration is invalid.", nil)
		return
	}
	status := map[string]any{
		"enabled":         cfg.Storymaker.Enabled,
		"read_only":       cfg.VirtualDesktop.ReadOnly,
		"desktop_enabled": cfg.VirtualDesktop.Enabled,
	}
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		// Disabled: nothing is probed, the app is hidden anyway.
		writeStorymakerJSON(w, http.StatusOK, status)
		return
	}
	rt, err := s.storymakerRuntime()
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "storage_unavailable", "Storymaker storage is unavailable: "+err.Error(), nil)
		return
	}
	lim := storymakerLimits(cfg)
	ff := videostudio.CheckRuntime(r.Context(), cfg.VideoStudio.FFmpegPath)
	ffmpeg := map[string]any{"ready": ff.Ready}
	if ff.Version != "" {
		ffmpeg["version"] = ff.Version
	}
	if !ff.Ready && ff.Error != "" {
		ffmpeg["issue"] = ff.Error
	}
	browser := rt.browsers.Status(r.Context(), cfg.Storymaker.Render.BrowserPath)
	status["ffmpeg"] = ffmpeg
	status["browser"] = browser
	status["limits"] = map[string]any{
		"max_duration_seconds": cfg.Storymaker.Render.MaxDurationSeconds,
		"max_scenes":           lim.MaxScenes,
		"max_items_per_scene":  lim.MaxItemsPerScene,
		"max_beats_per_scene":  lim.MaxBeatsPerScene,
		"max_narration_chars":  lim.MaxNarrationChars,
		"max_story_bytes":      lim.MaxStoryBytes,
		"max_code_bytes":       lim.MaxCodeBytes,
		"max_queued_renders":   render.DefaultMaxQueued,
	}
	status["transport"] = render.ChooseTransport(cfg.Storymaker.Render.Transport)
	status["catalog_version"] = rt.catalog.Version()
	w.Header().Set("Cache-Control", "no-store")
	writeStorymakerJSON(w, http.StatusOK, status)
}

type storymakerCreateRequest struct {
	Title  string `json:"title"`
	Aspect string `json:"aspect"`
	FPS    int    `json:"fps"`
}

func handleStorymakerProjects(s *Server, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
		if !ok {
			return
		}
		projects, err := acc.rt.projects.List(r.Context())
		if err != nil {
			mapStorymakerError(w, err, "not_found")
			return
		}
		if projects == nil {
			projects = []storymaker.Project{}
		}
		acc.rt.warmBrowser(acc.cfg)
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"projects": projects})
	case http.MethodPost:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		var req storymakerCreateRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, storymakerJSONLimit))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body must be {\"title\": ..., \"aspect\"?: ..., \"fps\"?: ...}.", nil)
			return
		}
		project, story, etag, err := acc.rt.projects.Create(r.Context(), req.Title, req.Aspect, req.FPS)
		if err != nil {
			mapStorymakerError(w, err, "not_found")
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Location", storymakerAPIBase+"/projects/"+project.ID)
		writeStorymakerJSON(w, http.StatusCreated, map[string]any{"project": project, "story": story, "etag": etag})
	default:
		writeStorymakerMethodError(w, "GET, POST")
	}
}

func handleStorymakerProject(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
		if !ok {
			return
		}
		ctx := r.Context()
		project, err := acc.rt.projects.Get(ctx, id)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		data, etag, err := acc.rt.projects.LoadStory(ctx, id)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		lim := storymakerLimits(acc.cfg)
		status, err := acc.rt.projects.CodeSceneStatus(ctx, id)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		if status == nil {
			status = []storymaker.CodeSceneStatus{}
		}
		body := map[string]any{"project": project, "etag": etag, "code_scenes": status}
		story, decodeIssues, derr := storymaker.DecodeStory(data, lim)
		if derr != nil {
			// A story.json edited by hand can be unreadable: the app shows the issues and the text.
			body["story"] = nil
			body["story_text"] = string(data)
			body["issues"] = issuesOrEmpty(decodeIssues)
		} else {
			checked := story
			storymaker.ApplyDefaults(&checked)
			issues := storymaker.Validate(&checked, acc.rt.catalog, lim)
			issues = append(issues, storymaker.CodeSceneIssues(&checked, status)...)
			body["story"] = story
			body["issues"] = issuesOrEmpty(issues)
		}
		w.Header().Set("ETag", etag)
		writeStorymakerJSON(w, http.StatusOK, body)
	case http.MethodDelete:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		if _, err := acc.rt.projects.Get(r.Context(), id); err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		acc.rt.jobs.CancelProject(id)
		if err := acc.rt.projects.Delete(r.Context(), id); err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeStorymakerMethodError(w, "GET, DELETE")
	}
}

// handleStorymakerStory saves story.json: decode strictly (an unreadable story is a 422 and is not
// saved), fill the static defaults, validate and write the canonical form. Validation findings do
// not block the save (a draft may be incomplete); they are returned, and a render needs none of
// severity error.
func handleStorymakerStory(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		writeStorymakerMethodError(w, "PUT")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	lim := storymakerLimits(acc.cfg)
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(lim.MaxStoryBytes)+1))
	if err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body could not be read.", nil)
		return
	}
	story, issues, err := storymaker.DecodeStory(body, lim)
	if err != nil {
		var de *storymaker.DecodeError
		if errors.As(err, &de) {
			status, code := http.StatusUnprocessableEntity, "invalid_story"
			if de.Code == storymaker.CodeTooLarge {
				status, code = http.StatusRequestEntityTooLarge, "too_large"
			}
			writeStorymakerError(w, status, code, de.Message, de.Issues)
			return
		}
		writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_story", err.Error(), issues)
		return
	}
	defaults := storymaker.ApplyDefaults(&story)
	issues = storymaker.Validate(&story, acc.rt.catalog, lim)
	if status, serr := acc.rt.projects.CodeSceneStatus(r.Context(), id); serr == nil {
		issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	}
	canonical, err := storymaker.CanonicalJSON(story)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "encode_failed", "The story could not be encoded.", nil)
		return
	}
	etag, err := acc.rt.projects.SaveStory(r.Context(), id, canonical, match)
	if err != nil {
		if errors.Is(err, storymaker.ErrConflict) {
			// Tell the editor which version is current, so it can offer a reload.
			if _, current, readErr := acc.rt.projects.LoadStory(r.Context(), id); readErr == nil {
				w.Header().Set("ETag", current)
			}
		}
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	if defaults == nil {
		defaults = []storymaker.DefaultApplied{}
	}
	w.Header().Set("ETag", etag)
	writeStorymakerJSON(w, http.StatusOK, map[string]any{
		"etag": etag, "issues": issuesOrEmpty(issues), "defaults_applied": defaults, "story": story,
	})
}

func handleStorymakerCode(s *Server, w http.ResponseWriter, r *http.Request, id, sceneID string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
		if !ok {
			return
		}
		src, etag, err := acc.rt.projects.LoadCodeSource(r.Context(), id, sceneID)
		if err != nil {
			mapStorymakerError(w, err, "code_not_found")
			return
		}
		writeStorymakerScript(w, r, src, etag)
	case http.MethodPut:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		ifMatch := ""
		switch {
		case strings.TrimSpace(r.Header.Get("If-None-Match")) == "*":
			ifMatch = storymaker.MustNotExist
		case strings.TrimSpace(r.Header.Get("If-Match")) != "":
			var good bool
			if ifMatch, good = requireStorymakerMatch(w, r); !good {
				return
			}
		default:
			writeStorymakerError(w, http.StatusPreconditionRequired, "precondition_required", "Send the current version in If-Match, or If-None-Match: * for a new code scene.", nil)
			return
		}
		lim := storymakerLimits(acc.cfg)
		src, err := io.ReadAll(io.LimitReader(r.Body, int64(lim.MaxCodeBytes)+1))
		if err != nil {
			writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body could not be read.", nil)
			return
		}
		if len(src) > lim.MaxCodeBytes {
			writeStorymakerError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("A code scene is limited to %d bytes.", lim.MaxCodeBytes), nil)
			return
		}
		etag, res, err := acc.rt.projects.SaveCodeScene(r.Context(), id, sceneID, src, ifMatch)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		errs := res.Errors
		if errs == nil {
			errs = []storymaker.BundleError{}
		}
		w.Header().Set("ETag", etag)
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"etag": etag, "ok": len(res.Errors) == 0, "errors": errs})
	default:
		writeStorymakerMethodError(w, "GET, PUT")
	}
}

func handleStorymakerBundle(s *Server, w http.ResponseWriter, r *http.Request, id, sceneID string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	bundle, err := acc.rt.projects.LoadCodeBundle(r.Context(), id, sceneID)
	if err != nil {
		mapStorymakerError(w, err, "bundle_not_found")
		return
	}
	writeStorymakerScript(w, r, bundle, storymaker.ETag(bundle))
}

// writeStorymakerScript serves author-written JavaScript as inert text: a browser that navigates to
// the URL cannot run it (sandbox without allow-scripts, nosniff).
func writeStorymakerScript(w http.ResponseWriter, r *http.Request, body []byte, etag string) {
	h := w.Header()
	h.Set("Content-Type", "text/javascript; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "no-store")
	h.Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func handleStorymakerRender(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key != "" && !storymakerIdempotencyKeyPattern.MatchString(key) {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 printable characters.", nil)
		return
	}
	ctx := r.Context()
	if _, err := acc.rt.projects.Get(ctx, id); err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	settings, err := s.storymakerSettings(ctx)
	if err != nil {
		writeStorymakerError(w, http.StatusConflict, render.CodeFFmpegUnavailable, err.Error(), nil)
		return
	}
	if ff := videostudio.CheckRuntime(ctx, acc.cfg.VideoStudio.FFmpegPath); !ff.Ready {
		writeStorymakerError(w, http.StatusConflict, render.CodeFFmpegUnavailable, "FFmpeg is not ready: "+ff.Error, nil)
		return
	}
	if b := acc.rt.browsers.Status(ctx, settings.BrowserPath); !b.Ready && !b.CanDownload {
		writeStorymakerError(w, http.StatusConflict, render.CodeBrowserUnavailable, b.Issue, nil)
		return
	}
	plan, err := acc.rt.runner.Prepare(ctx, id, match, settings)
	if err != nil {
		var pe *render.PrepareError
		if errors.As(err, &pe) {
			switch {
			case pe.Stale:
				if _, current, readErr := acc.rt.projects.LoadStory(ctx, id); readErr == nil {
					w.Header().Set("ETag", current)
				}
				writeStorymakerError(w, http.StatusPreconditionFailed, "precondition_failed", pe.Message, nil)
			case pe.Code == render.CodeCodeError:
				writeStorymakerError(w, http.StatusUnprocessableEntity, render.CodeCodeError, pe.Message, pe.Issues)
			default:
				writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_story", pe.Message, pe.Issues)
			}
			return
		}
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	job, _, err := acc.rt.jobs.Enqueue(render.EnqueueRequest{ProjectID: id, Key: key, Fingerprint: id + "\x00" + match, Payload: plan})
	switch {
	case errors.Is(err, render.ErrBusy):
		writeStorymakerError(w, http.StatusTooManyRequests, render.CodeBusy, "Sixteen exports are already waiting. Try again when one has finished.", nil)
		return
	case errors.Is(err, render.ErrIdempotencyConflict):
		writeStorymakerError(w, http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was used for a different request.", nil)
		return
	case err != nil:
		writeStorymakerError(w, http.StatusInternalServerError, "job_failed", "The export could not be queued.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func handleStorymakerJob(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	job, found := acc.rt.jobs.Get(jobID)
	if !found {
		writeStorymakerError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleStorymakerProjectJobs lists the jobs of a project, newest first (an addition to the contract:
// the app needs it to find a running export after a reload).
func handleStorymakerProjectJobs(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	jobs := acc.rt.jobs.List(id)
	if jobs == nil {
		jobs = []render.Job{}
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

// handleStorymakerJobCancel maps to desktopStop: it works while the desktop is read-only, like the
// Video Studio cancel.
func handleStorymakerJobCancel(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopStop, false)
	if !ok {
		return
	}
	job, found := acc.rt.jobs.Cancel(jobID)
	if !found {
		writeStorymakerError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleStorymakerEvents streams job changes of a project as server-sent events: an event "job"
// with {"job": ...} for every state or progress change. On connect it sends the project's latest jobs
// so a reloaded app finds a running export.
func handleStorymakerEvents(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeStorymakerError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming is not supported.", nil)
		return
	}
	// A stream outlives the server's usual write window.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	jobs := acc.rt.jobs
	events, unsubscribe := jobs.Subscribe(id)
	defer unsubscribe()
	rev := jobs.Revision()
	send := func(job render.Job) bool {
		data, err := json.Marshal(map[string]any{"job": job})
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "event: job\ndata: %s\n\n", data); err != nil {
			return false
		}
		return true
	}
	if _, err := io.WriteString(w, ": storymaker events\n\n"); err != nil {
		return
	}
	initial := jobs.List(id)
	if len(initial) > 5 {
		initial = initial[:5]
	}
	for i := len(initial) - 1; i >= 0; i-- { // oldest first
		if !send(initial[i]) {
			return
		}
	}
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-events:
			var changed []render.Job
			changed, rev = jobs.ChangedSince(id, rev)
			for _, job := range changed {
				if !send(job) {
					return
				}
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
```

`internal/server/storymaker_player.go`:

```go
package server

// The Storymaker player host page (GET|HEAD /api/desktop/storymaker/player) is the document of the
// sandboxed iframe in the Storymaker app. The page has no inline script or style; its two assets
// are named in the CSP, which has no unsafe-inline and no unsafe-eval, and connect-src is 'none':
// code scenes run in workers that cannot reach any network. The sandbox directive has no
// allow-same-origin, so the page has an opaque origin and cannot read the desktop's cookies,
// storage or API.

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// storymakerPlayerHostPattern is the host form the page accepts; the host ends up in a header and
// in markup.
var storymakerPlayerHostPattern = regexp.MustCompile(`^[A-Za-z0-9.\-]+(:[0-9]{1,5})?$|^\[[0-9A-Fa-f:.]+\](:[0-9]{1,5})?$`)

// StorymakerPlayerCSP is the Content-Security-Policy of the player host page for an origin (spec
// "User interface", Player iframe).
func StorymakerPlayerCSP(origin string) string {
	return "sandbox allow-scripts; default-src 'none'; script-src " + origin + "/js/storymaker/ blob:; worker-src blob:; " +
		"img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src " + origin + "/css/storymaker-player.css; " +
		"form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"
}

// StorymakerPlayerPage is the markup of the player host page (contract section 6).
func StorymakerPlayerPage(origin string) string {
	o := html.EscapeString(origin)
	return `<!doctype html><html data-sm-mode="app" data-sm-base="` + o + `"><head><meta charset="utf-8">` +
		`<link rel="stylesheet" href="` + o + `/css/storymaker-player.css"></head>` +
		`<body><canvas id="sm-stage"></canvas><script src="` + o + `/js/storymaker/host.bundle.js"></script></body></html>`
}

// storymakerRequestOrigin returns the origin the browser used for this request: the scheme from
// TLS or a trusted proxy header, the host from the request. The postMessage target of the iframe is
// this origin, so it must be what the browser sees.
func storymakerRequestOrigin(r *http.Request) (string, bool) {
	host := strings.TrimSpace(r.Host)
	if host == "" || !storymakerPlayerHostPattern.MatchString(host) {
		return "", false
	}
	scheme := "http"
	if r.TLS != nil || trustedForwardedRequest(r) && strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: host}).String(), true
}

func handleStorymakerPlayer(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeStorymakerMethodError(w, "GET, HEAD")
		return
	}
	// The page itself needs no workspace; it needs the feature to be on and a signed-in desktop.
	if _, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false); !ok {
		return
	}
	origin, ok := storymakerRequestOrigin(r)
	if !ok {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_host", "The request has no usable Host header.", nil)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", StorymakerPlayerCSP(origin))
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	// securityHeadersMiddleware leaves X-Frame-Options off for this path (allowDesktopIframe); the
	// CSP's frame-ancestors 'self' is the rule, and this header says the same to older browsers.
	h.Set("X-Frame-Options", "SAMEORIGIN")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(StorymakerPlayerPage(origin)))
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./internal/server/ && go test ./internal/server/ -run "TestStorymaker" -count=1 -v`

Expected: PASS (14 tests; the tests that queue renders need an FFmpeg binary and skip without one). The full `go test ./internal/server` has one failure on the Windows box that is not Storymaker's: `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults` cannot make a `D:` temp path relative to a `C:` checkout.

- [ ] **Step 5: Check the whole package and the graph**

```bash
go test ./internal/server -count=1 -timeout 30m
node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo .
```

Expected: no failure beyond the one named above; `detect-changes` lists only the Storymaker files and the eight edited server files.

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_routes.go internal/server/bluetooth_live.go internal/server/desktop_operation.go internal/server/server_shutdown.go internal/server/config_patch.go internal/server/config_handlers_main.go internal/server/storymaker_store.go internal/server/storymaker_service.go internal/server/storymaker_handlers.go internal/server/storymaker_player.go internal/server/storymaker_api_test.go
git commit -m "feat(storymaker): server API, player host page and desktop wiring" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B12: Parity test between the app player host and the render page

**Files:**
- Test: `internal/server/storymaker_parity_browser_test.go`

The spec states parity in two parts: timeline identity everywhere, pixel identity within the same Chromium build at the export canvas size, device pixel ratio 1, after the readiness barrier. This test exports `firewall.json` through the real server (project API, story, code scene, render job, Chrome, FFmpeg), then loads the app player host in an `<iframe sandbox="allow-scripts">` of a parent page through the real `GET /api/desktop/storymaker/player` handler, asks it for nine frames at the export width (`thumb` with `frame`, `width: 1920`), hashes the decoded RGBA with `crypto.subtle` and compares with the hashes the render pipeline saw for the same frames (`storymakerFrameObserver`). It pins no pixel value, so player changes do not break it; a real difference does.

- [ ] **Step 1: Write the test**

`internal/server/storymaker_parity_browser_test.go`:

```go
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// storymakerParityParent is the parent page of the parity test: it embeds the app player host exactly as the
// desktop app does (sandbox="allow-scripts" only), loads the story with the catalog, the four fonts and the code
// bundle, waits for the first frame and then asks for full-size frames (thumb with width = the export width), whose
// decoded pixels it hashes.
const storymakerParityParent = `<!doctype html><meta charset="utf-8"><title>parity</title>
<iframe id="f" sandbox="allow-scripts" style="width:480px;height:270px;border:0"></iframe>
<canvas id="c" width="1920" height="1080"></canvas>
<script>
window.__par = { hashes: {}, errors: [], log: [], done: false, failure: null };
const par = window.__par, iframe = document.getElementById('f');
const API = '/api/desktop/storymaker';
const PROJECT = '%s';
const FRACTIONS = %s;
let TARGETS = [];
async function getJSON(u) { const r = await fetch(u); if (!r.ok) throw new Error(u + ' ' + r.status); return r.json(); }
function post(msg) { iframe.contentWindow.postMessage(Object.assign({ sm: 1 }, msg), '*'); }
async function sha(bitmap) {
  const c = document.getElementById('c'), ctx = c.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, c.width, c.height);
  ctx.drawImage(bitmap, 0, 0);
  const data = ctx.getImageData(0, 0, bitmap.width, bitmap.height).data;
  const digest = await crypto.subtle.digest('SHA-256', data);
  return { size: bitmap.width + 'x' + bitmap.height, hash: Array.from(new Uint8Array(digest)).map(b => b.toString(16).padStart(2, '0')).join('') };
}
let next = 0;
function ask() {
  if (next >= TARGETS.length) { par.done = true; return; }
  const t = TARGETS[next];
  post({ type: 'thumb', req: 't' + next, scene_id: t.scene, frame: t.frame, width: 1920 });
}
window.addEventListener('message', async (ev) => {
  if (ev.source !== iframe.contentWindow) return;
  const m = ev.data;
  if (!m || m.sm !== 1) return;
  par.log.push(m.type);
  try {
    if (m.type === 'ready') {
      const project = await getJSON(API + '/projects/' + PROJECT);
      const catalog = await getJSON('/img/storymaker/catalog.json');
      const files = {};
      for (const p of [catalog.rig.file].concat(catalog.entries.filter(e => e.file).map(e => e.file))) files['catalog:' + p] = await getJSON('/img/storymaker/' + p);
      const fonts = [];
      for (const f of catalog.fonts) fonts.push({ name: f.name, family: f.family, weight: f.weight, data: await (await fetch(f.file)).arrayBuffer() });
      const bundles = {};
      for (const sc of project.story.scenes) if (sc.kind === 'code') bundles[sc.id] = await (await fetch(API + '/projects/' + PROJECT + '/scenes/' + sc.id + '/bundle')).text();
      post({ type: 'load', story: project.story, catalog, audio_info: {}, assets: files, fonts, code_bundles: bundles, format: { aspect: '16:9', fps: 30, width: 1920 } });
    } else if (m.type === 'compiled') {
      par.report = { frame_count: m.report.frame_count, scenes: m.report.scenes };
      TARGETS = FRACTIONS.map((f) => {
        const frame = Math.round(f * (m.report.frame_count - 1));
        const scenes = m.report.scenes.filter((s) => s.start_frame <= frame);
        return { frame, scene: scenes[scenes.length - 1].id };
      });
    } else if (m.type === 'time' && !par.asked) {
      par.asked = true;
      ask();
    } else if (m.type === 'thumb') {
      const t = TARGETS[next];
      par.hashes[t.frame] = await sha(m.bitmap);
      next++;
      ask();
    } else if (m.type === 'error') {
      par.errors.push({ code: m.code, message: m.message, scene_id: m.scene_id });
    }
  } catch (e) { par.failure = String(e && e.stack || e); par.done = true; }
});
iframe.src = API + '/player';
</script>`

// pixelHash hashes the straight RGBA pixels of a PNG frame, like the parent page hashes getImageData.
func pixelHash(t *testing.T, data []byte) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("a frame did not decode: %v", err)
		return ""
	}
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)
	sum := sha256.Sum256(nrgba.Pix)
	return hex.EncodeToString(sum[:])
}

// TestStorymakerParityAppPlayerAndRenderPage renders firewall.json through the whole export pipeline and
// compares the decoded RGBA of several frames with the same frames drawn by the app player host in the sandboxed
// iframe: same Chromium, export canvas size, device pixel ratio 1, after the readiness barrier (the app host loads
// fonts and assets before it draws frame 0). Gated by AURAGO_RUN_BROWSER_SMOKE=1.
func TestStorymakerParityAppPlayerAndRenderPage(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker parity test")
	}
	chrome := os.Getenv("AURAGO_TEST_CHROME")
	if chrome == "" {
		found, ok := launcher.LookPath()
		if !ok {
			t.Skip("no Chrome or Chromium is installed")
		}
		chrome = found
	}
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		render.SetNoSandboxForTests(true)
		t.Cleanup(func() { render.SetNoSandboxForTests(false) })
	}

	s, _, token := storymakerAPIFixture(t)
	s.Cfg.Auth.Enabled = false // the iframe document request carries no bearer token
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.Cfg.Storymaker.Render.BrowserPath = chrome
	s.storymakerBrowsers = nil // the real resolver
	var mu sync.Mutex
	rendered := map[int]string{}
	s.storymakerFrameObserver = func(n int, transport string, data []byte) {
		h := pixelHash(t, data)
		mu.Lock()
		rendered[n] = h
		mu.Unlock()
	}

	// The project: create, save firewall.json and its code scene, all through the HTTP API.
	p := createStorymakerProject(t, s, token)
	story, err := os.ReadFile("../storymaker/testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	scene, err := os.ReadFile("../storymaker/testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", bytes.NewReader(lf(story)), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	if w.Code != http.StatusOK {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/scenes/s3/code", bytes.NewReader(lf(scene)), map[string]string{"If-None-Match": "*"}); w.Code != http.StatusOK {
		t.Fatalf("save code scene: %d %s", w.Code, w.Body.String())
	}
	w = storymakerRequest(s, token, http.MethodGet, "/projects/"+p.Project.ID, nil, nil)
	var current struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &current)

	// The export.
	w = storymakerRequest(s, token, http.MethodPost, "/projects/"+p.Project.ID+"/render", nil, map[string]string{"If-Match": current.ETag})
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	if w.Code != http.StatusAccepted {
		t.Fatalf("render: %d %s", w.Code, w.Body.String())
	}
	var job render.Job
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		var got struct {
			Job render.Job `json:"job"`
		}
		decodeBody(t, storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil), &got)
		job = got.Job
		if job.Terminal() {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if job.Status != render.StatusSucceeded {
		t.Fatalf("export: %+v error %+v", job, job.Error)
	}
	t.Logf("export: %d frames, %s", job.FramesTotal, job.Output.Path)

	// Frames to compare: the first, the middle of every scene, and the transitions between them.
	fractions := []float64{0, 0.17, 0.36, 0.5, 0.58, 0.62, 0.77, 0.9, 1}
	fractionsJSON, _ := json.Marshal(fractions)

	// The app side: the real player handler and API behind one origin, the static assets from the ui tree.
	uiDir, _ := filepath.Abs("../../ui")
	mux := http.NewServeMux()
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	mux.Handle("/js/storymaker/", http.FileServer(http.Dir(uiDir)))
	mux.Handle("/css/", http.FileServer(http.Dir(uiDir)))
	mux.Handle("/img/storymaker/", http.FileServer(http.Dir(uiDir)))
	mux.Handle("/fonts/", http.FileServer(http.Dir(uiDir)))
	mux.HandleFunc("/parent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, storymakerParityParent, p.Project.ID, fractionsJSON)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(chrome).Headless(true).Set("disable-gpu").Set("force-device-scale-factor", "1")
	l.Delete("no-sandbox")
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		l.Set("no-sandbox")
	}
	u, err := l.Launch()
	if err != nil {
		t.Skipf("the viewer browser did not start: %v", err)
	}
	defer func() { l.Kill(); l.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(u)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = browser.Close() }()
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1920, Height: 1080, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	if err := page.Navigate(srv.URL + "/parent"); err != nil {
		t.Fatal(err)
	}
	if err := page.Timeout(3 * time.Minute).Wait(rod.Eval(`() => window.__par && window.__par.done`)); err != nil {
		t.Fatalf("the player never finished: %v\n%s", err, page.MustEval(`() => JSON.stringify(window.__par)`).Str())
	}
	var res struct {
		Hashes  map[int]struct{ Size, Hash string } `json:"hashes"`
		Errors  []struct{ Code, Message string }    `json:"errors"`
		Failure string                              `json:"failure"`
		Report  struct {
			FrameCount int `json:"frame_count"`
		} `json:"report"`
	}
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__par)`).Str()), &res); err != nil {
		t.Fatal(err)
	}
	if res.Failure != "" || len(res.Errors) != 0 {
		t.Fatalf("player failure %q errors %+v", res.Failure, res.Errors)
	}
	if res.Report.FrameCount != job.FramesTotal {
		t.Fatalf("the app compiled %d frames, the export has %d", res.Report.FrameCount, job.FramesTotal)
	}
	if len(res.Hashes) < len(fractions)-2 {
		t.Fatalf("the app produced only %d frames", len(res.Hashes))
	}
	var mismatches []string
	for frame, app := range res.Hashes {
		mu.Lock()
		exp := rendered[frame]
		mu.Unlock()
		switch {
		case app.Hash == "":
			mismatches = append(mismatches, fmt.Sprintf("frame %d: the app produced no frame", frame))
		case app.Size != "1920x1080":
			mismatches = append(mismatches, fmt.Sprintf("frame %d: the app frame is %s", frame, app.Size))
		case exp == "":
			mismatches = append(mismatches, fmt.Sprintf("frame %d: the export has no such frame", frame))
		case app.Hash != exp:
			mismatches = append(mismatches, fmt.Sprintf("frame %d: app %s != export %s", frame, app.Hash[:12], exp[:12]))
		}
	}
	if len(mismatches) != 0 {
		sort.Strings(mismatches)
		t.Fatalf("app player and render page differ:\n%s", strings.Join(mismatches, "\n"))
	}
	t.Logf("%d frames identical between the app player host and the render page", len(res.Hashes))
	_ = storymaker.Schema
}
```

- [ ] **Step 2: Run the test**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server/ -run "TestStorymakerParity" -count=1 -v -timeout 10m`

Expected: PASS (`9 frames identical between the app player host and the render page`, about 45 s)

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_parity_browser_test.go
git commit -m "test(storymaker): parity of app player host and render page" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B13: Frame-transport spike, decision and the repeatable step for aurago-test

**Files:**
- Modify (only if a measurement on aurago-test says so): `internal/storymaker/render/render.go` (`DefaultTransport`, `measuredFPS`)
- Harness (task B10): `TestTransportSpike`, `TestGuardAndRequestOverheadSpike` in `internal/storymaker/render/browser_smoke_test.go`

The spike answers the spec question "throughput and determinism of the frame transports" on the production path (strict proxy and CDP guard active, `firewall.json` at 1920x1080 and 30 fps, 342 frames, each transport rendered twice). It prints frames per second, whether the two renders and the transports agree pixel for pixel, and (with `AURAGO_STORYMAKER_SPIKE_MEM=1`) the peak memory of the Chrome process tree and of Go. The memory sampler walks the process tree and slows a Windows host, so fps and memory are measured in separate runs.

**Result on the development box** (Windows 11, 16 logical CPUs, Chrome 156, FFmpeg 8.1.1; the player with the two changes of task B15; frame counts and times of the whole firewall story):

| transport | fps (two runs) | job time | frame size | peak memory Chrome / Go heap | two renders identical | same pixels as png |
| --- | --- | --- | --- | --- | --- | --- |
| png (`canvas.toBlob`) | 10.5, 10.0 (11.6 and 15.8 in quieter runs) | 36 s | 578 KB | 690 MB / 50 MB | yes | - |
| webp (lossless) | 4.9, 4.5 | 77 s | 287 KB | 600 MB / 44 MB | yes | yes |
| screenshot (CDP) | 4.1, 4.1 | 87 s | 261 KB | not sampled | yes | yes |
| raw RGBA (`getImageData`) | 1.5 (sampled run) | 227 s | 8.3 MB | 1.39 GB / 349 MB | not compared | not compared |

Without the two player changes the png rate on the production path is 13.8 fps against 15.8 fps in the same quiet session (one preflight request per frame; the opaque-origin page gets no preflight cache in Chrome 156). Guard and proxy cost (png, quiet session): proxy plus hybrid guard 15.8 fps, no guard 16.0, neither 16.5, catch-all guard 8.4. **Decision: png is the default** (`DefaultTransport`); the render timeout assumes 6 fps for png, 2.5 for WebP and screenshot and 1 for raw (`measuredFPS`), so a 15 minute video gets `max(3600, 3 x 27000 / 6) = 13500` seconds.

- [ ] **Step 1: Run the spike on the development box**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_SPIKE=1 AURAGO_STORYMAKER_SPIKE_PLAYER=recommended \
  AURAGO_STORYMAKER_SPIKE_TRANSPORTS=png,webp,screenshot AURAGO_STORYMAKER_SPIKE_OUT=spike.json \
  go test ./internal/storymaker/render/ -run "TestTransportSpike" -count=1 -v -timeout 40m
# memory in a second pass (slows the run):
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_SPIKE=1 AURAGO_STORYMAKER_SPIKE_MEM=1 AURAGO_STORYMAKER_SPIKE_TRANSPORTS=png AURAGO_STORYMAKER_SPIKE_RUNS=1 \
  go test ./internal/storymaker/render/ -run "TestTransportSpike" -count=1 -v -timeout 40m
```

`AURAGO_STORYMAKER_SPIKE_PLAYER=recommended` applies the changes of task B15 to the bundle text in memory, so the spike can run before C changes its files; drop it once B15 landed. Expected: PASS; `two_renders_identical` true for every transport and one `sequence_hash` for all of them.

- [ ] **Step 2: Repeat it on aurago-test (Linux)**

aurago-test has no Go toolchain requirement: build the test binary here and run it there. The tests read `../testdata/...` and `../../../ui` relative to their folder, so the box needs the same layout.

```bash
# on the development machine
GOOS=linux GOARCH=amd64 go test -c -o /tmp/render.test ./internal/storymaker/render/
ssh aurago-test 'mkdir -p /tmp/smspike/internal/storymaker/render'
scp /tmp/render.test aurago-test:/tmp/smspike/internal/storymaker/render/
rsync -a internal/storymaker/testdata aurago-test:/tmp/smspike/internal/storymaker/
rsync -a ui aurago-test:/tmp/smspike/
# on aurago-test: FFmpeg and a Chrome or Chromium must be installed (the spike uses what the box has, like an export does)
ssh aurago-test 'cd /tmp/smspike/internal/storymaker/render && \
  AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_SPIKE=1 AURAGO_STORYMAKER_SPIKE_PLAYER=recommended \
  AURAGO_STORYMAKER_SPIKE_TRANSPORTS=png,webp,screenshot AURAGO_STORYMAKER_SPIKE_OUT=/tmp/smspike/spike.json \
  ./render.test -test.run TestTransportSpike -test.v -test.timeout 40m'
ssh aurago-test 'cat /tmp/smspike/spike.json; rm -rf /tmp/smspike'
# acceptance of the pinned Chromium on a box without a browser (downloads about 190 MB, needs egress to storage.googleapis.com):
ssh aurago-test 'cd /tmp/smspike/internal/storymaker/render && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_PINNED=1 ./render.test -test.run TestPinnedChromiumDownloadsVerifiesAndLaunches -test.v'
# FFmpeg floor (production minimum 5.1.9): the frame-input tests run against whatever FFmpeg the box has; note `ffmpeg -version` in the notes:
ssh aurago-test 'cd /tmp/smspike/internal/storymaker/render && ffmpeg -version | head -1 && ./render.test -test.run "TestBuildFFmpegArgs|TestFFmpeg|TestIngestRestarts" -test.v'
```

The box must allow Chrome's sandbox (Ubuntu 23.10 or later needs the AppArmor profile of `documentation/installation.md:271-302`); if it does not, the status shows the launch error, and `AURAGO_TEST_NO_SANDBOX=1` runs the tests only (production never sets it). Do not stop or restart the live AuraGo or touch its containers; the spike uses its own temp folder and ports.

- [ ] **Step 3: Apply the decision rule**

Criteria of the spec: at least 10 rendered frames per second at 1080p on an aurago-test-class CPU, bit-identical repeat renders, bounded memory. Keep png when it reaches 10 fps and is deterministic. If png stays below 10 fps, the spec says to take the fastest deterministic candidate and record its rate: set `DefaultTransport` to it and put 60 percent of the measured rate into `measuredFPS` for each transport (the render timeout then grows to `3 x frames / rate`). If the rate on aurago-test is below 4 fps, say so in the user-facing documentation (a 15 minute video takes longer than an hour). Record the numbers in `internal/storymaker/render/AGENTS.md`.

- [ ] **Step 4: Commit (only when the constants or the recorded numbers changed)**

```bash
git add internal/storymaker/render/render.go
git commit -m "chore(storymaker): record the transport spike of aurago-test" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B14: Documentation

**Files:**
- Create: `internal/storymaker/render/AGENTS.md`
- Modify: `internal/storymaker/AGENTS.md` (created by part A; add the section below before its `## Child DOX Index` and name the child there)

- [ ] **Step 1: Write the render contract**

`internal/storymaker/render/AGENTS.md`:

```markdown
# Storymaker Render Contract

This package turns a validated Storymaker story into an MP4: the job manager, the per-job loopback listener the headless browser talks to,
frame ingest, browser provisioning, the egress guard and the FFmpeg frame-input pipeline. The HTTP handlers live in `internal/server`
(`storymaker_*.go`); nothing here imports it. The design is `docs/superpowers/specs/2026-11-01-storymaker-design.md` (Rendering and export).

- **Jobs** (`jobs.go`): one render runs at a time from a queue of at most 16 (`ErrBusy`, HTTP 429 `busy`). `jobs.json` holds the public job
  fields plus the SHA-256 of the render prefix (`HashPrefix`); the prefix itself is never stored or logged. A job that was queued or running at
  start-up becomes `interrupted`; `<DataDir>/storymaker/staging/` is emptied at start. Progress is throttled (150 ms notification, 3 s
  persistence); state changes are announced and persisted at once. The request snapshot (`Plan`, built by `Runner.Prepare` at enqueue time) travels
  as the in-memory payload, so edits while a job waits change nothing.
- **Listener** (`listener.go`): binds `127.0.0.1:0`, plain HTTP, own timeouts, no logging of paths, a Host check, and answers 404 for everything
  outside `/<prefix>/`. The prefix is 128 bits from `crypto/rand`. Every response below the base carries `Access-Control-Allow-Origin: *`
  (the render page has an opaque origin and sends `Origin: null`); the render page is the only response with the CSP (`RenderCSP`). It serves only
  the snapshot: the story, `catalog.json` and the files it names, its fonts, the code bundles and the two player bundles, copied when the job
  starts. A code scene can read the prefix from its worker (the blob text names the base): the prefix is a capability for this one job, not a
  secret against scene code. What a scene could do with it (post frames, errors, a report) only affects its own job.
- **Ingest** (`ingest.go`): frames only in the running state, strictly in order from 0, below the Go-computed limit (`render.max_duration_seconds`
  times fps), with the Content-Type of the transport (or `text/plain`, the CORS-simple form), a complete PNG (IHDR size, IEND trailer) or WebP
  (RIFF size, VP8/VP8L/VP8X size) of exactly the canvas size, or exactly width x height x 4 raw bytes. A frame is read fully before it is
  written (at most 4 MiB at 1080p, scaled by pixels) so a truncated body never reaches FFmpeg. Every refusal fails the job.
- **FFmpeg** (`ffmpeg.go`): frames on stdin, `-protocol_whitelist file,pipe` and `-format_whitelist` on every input, an explicit
  `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p` plus the encoder's filter tail, `+faststart`, and a silent AAC track from `anullsrc`
  (so the stream layout is video plus audio for later phases). A hardware encoder gets 1.5 s after the first frame to fail; a failure in the
  stream restarts once with libx264 (`{restart:true}` to the page).
- **Egress** (`egress.go`, `session.go`, `security.StartStrictCaptureProxy`): three independent layers keep the page on the render base. The CSP
  (`default-src 'none'`, `connect-src <base>/`, sandbox without `allow-popups` and `allow-same-origin`); a strict proxy that dials only the
  render origin (other loopback ports and `AURAGO_SSRF_ALLOW_LOOPBACK` do not count); and the CDP guard: navigations are paused and decided
  (`Fetch`, resource type Document), everything else is blocked natively by `Network.setBlockedURLs` patterns on the page and on every worker
  that auto-attaches (held until its block list is installed). `Fetch.enable` does not exist on a worker session in Chrome 156, and pausing a
  request that carries a frame costs about 35 ms, so the catch-all interception is only the fallback for a browser without block patterns
  (`GuardFull`). WebSocket handshakes are outside both mechanisms; CSP and proxy stop them. `TestEgressLayersEachStopTheProbe` proves each
  layer alone and all together against a loopback probe server.
- **Browser** (`browser.go`): `render.browser_path`, else `launcher.LookPath()`, else the pinned Chromium (revision 1321438, SHA-256 per
  platform in `pinnedChromium`, downloaded into `<DataDir>/storymaker/chromium-<rev>/` by the first export, never by a status request). A
  browser is `ready` only after a cached headless launch probe. Production launches never set `NoSandbox` (`newLauncher` deletes the flag rod adds
  inside containers); `SetNoSandboxForTests` is the only way to set it and nothing outside tests calls it.
- **Timeouts**: report within 3 minutes of the navigation, no frame within 10 s after `start` kills the job as `code_error`, overall
  `max(render.timeout_seconds, 3 x frames / measuredFPS[transport])`. Any `POST /error` from the page is fatal.
- **Tests**: everything but `browser_smoke_test.go`, `egress_smoke_test.go` and the parity test in `internal/server` runs without a browser (a
  fake page plays the render page against the real listener). The browser tests need `AURAGO_RUN_BROWSER_SMOKE=1`; the transport spike also
  needs `AURAGO_STORYMAKER_SPIKE=1`. On the Windows development box run them through `go test` with `GOTMPDIR=D:\tmp\go-build` (a repo-built
  executable cannot start Chrome there).
```

- [ ] **Step 2: Extend `internal/storymaker/AGENTS.md`**

Insert this section before the `## Child DOX Index` heading, and replace the `None yet.` under that heading with ``- `render/AGENTS.md`: the render pipeline contract (job manager, listener, ingest, browser, egress guard, FFmpeg).``

```markdown
## Render and server (part B)

- `render/` turns a story into an MP4 (job manager, listener, ingest, browser, egress guard, FFmpeg); its contract is `render/AGENTS.md`. The package has no dependency on `internal/server`; the handlers are `internal/server/storymaker_*.go`.
- The server implements `Store` and `render.Publisher` over `*desktop.Service` (`storymakerStore`), passes `Limits.MaxDurationSeconds = storymaker.render.max_duration_seconds` to the validator and gives every request its own `Story` copy (`Validate` and `ApplyDefaults` mutate).
- Catalog version, canvas sizes (1920x1080, 1080x1920, 1080x1080), the support matrix and the error codes of a job (`invalid_story`, `browser_unavailable`, `ffmpeg_unavailable`, `render_failed`, `render_timeout`, `cancelled`, `busy`, `code_error`, `interrupted`, `storymaker_disabled`) are shared with the app through `GET /status` and the job document.
```

- [ ] **Step 3: Commit**

```bash
git add -f internal/storymaker/render/AGENTS.md internal/storymaker/AGENTS.md   # .gitignore ignores nested AGENTS.md files; -f is needed for a new one
git commit -m "docs(storymaker): render and server contract" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B15: Two small player changes (requested from part C)

**Files:**
- Modify: `ui/js/storymaker/host-render.js` (owned by part C, as part R left it)
- Modify: `scripts/storymaker-tests/09-host-render.mjs` (part R's version: `TEST_LIMITS`)
- Modify: `ui/storymaker_player_gate_browser_test.go` (part C's browser tests read the frame number)
- Modify: `ui/storymaker_parity_browser_test.go` (part R's parity harness reads the frame number)
- Modify: `internal/storymaker/render/browser_smoke_test.go` (the spike knows both forms of the player)
- Regenerate: `ui/js/storymaker/host.bundle.js` (`npm run build:ui`)

Two measured findings of the transport spike, both on the render page only. The task runs after part R (it edits R's files) and needs the listener of B7, which accepts the frame number from `?n=` as well as from `X-SM-Frame`.

1. **Frames as CORS-simple requests.** The render page has an opaque origin and Chrome 156 sends a preflight `OPTIONS` before every `POST /frame` (342 preflights for 342 frames), although the listener answers `Access-Control-Max-Age: 600`. A request with `Content-Type: text/plain` and no custom header needs none. The page now posts to `<base>/frame?n=<frame>` with `Content-Type: text/plain`; the real type of the body (PNG, WebP, raw RGBA) is still checked by the ingest from the bytes. Effect on the production path: 13.8 to 15.8 fps for png (14 percent). The browser test of C counts the preflights that hit `/frame` and requires none.
2. **A deadline of its own for renders.** The page enforces 250 ms per frame (the app preview's budget) and kills the engine on an overrun. A render is not real time, and one overrun at frame 134 failed a screenshot-transport job on a loaded host. The listener sends `frame_deadline_ms` (5000) in `boot.json`; the page uses it (at most 60 s), and Go's own 10 s no-frame watchdog stays the backstop.

The diffs below were applied to the merged files of C and R (integration tree on main `561685cde`). The Node suite has one check more than R's 71: it records the timers the engine asks for and requires the offered deadline (and the 60 s cap) instead of the 250 ms of the preview.

- [ ] **Step 1: Change the tests**

```diff
--- a/scripts/storymaker-tests/09-host-render.mjs
+++ b/scripts/storymaker-tests/09-host-render.mjs
@@ -19,7 +19,8 @@ function makeServer(opts = {}) {
         mode: 'render', fps: 30, width: W, height: H, transport, frame_limit: opts.frameLimit || 5000, story_url: RBASE + '/story.json',
         catalog_url: RBASE + '/catalog/catalog.json', assets_base: RBASE + '/catalog/',
         code_bundles: opts.noCode ? {} : { s3: RBASE + '/scenes/s3.js' },
-        fonts: [{ name: 'default', family: 'Storymaker Default', weight: 400, url: RBASE + '/fonts/placeholder.woff2' }], audio_info: {}
+        fonts: [{ name: 'default', family: 'Storymaker Default', weight: 400, url: RBASE + '/fonts/placeholder.woff2' }], audio_info: {},
+        ...(opts.boot || {})
     };
     const res = (body, status = 200) => ({
         ok: status >= 200 && status < 300, status, json: async () => body, text: async () => typeof body === 'string' ? body : JSON.stringify(body), arrayBuffer: async () => new ArrayBuffer(16)
@@ -37,10 +38,10 @@ function makeServer(opts = {}) {
                 frames = rep.frame_count;
                 return res(opts.report ? opts.report(rep) : { start: true, from_frame: 0 });
             }
-            if (p === '/frame') {
-                const n = Number(init.headers['X-SM-Frame']);
+            if (p.startsWith('/frame?n=')) {
+                const n = Number(p.slice('/frame?n='.length));
                 rec.n = n;
-                const custom = answerFor(n, posts.filter(x => x.path === '/frame').length);
+                const custom = answerFor(n, posts.filter(x => x.path.startsWith('/frame?n=')).length);
                 if (custom) return res(custom);
                 return res(n >= frames - 1 ? { ok: true, done: true } : { ok: true });
             }
@@ -76,10 +77,10 @@ async function runRender(opts = {}) {
     const env = {
         hasWorkers: true,
         createWorker(role, text) { const w = new InProcessWorker(role, text, {}); workers.push(w); return w; },
-        setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => Date.now(), limits: { ...TEST_LIMITS, ...(opts.limits || {}) }
+        setTimer: (fn, ms) => { if (opts.timers) opts.timers.push(ms); return setTimeout(fn, ms); }, clearTimer: h => clearTimeout(h), now: () => Date.now(), limits: { ...TEST_LIMITS, ...(opts.limits || {}) }
     };
     const status = await H.mountRender({ canvas, base: RBASE, fetch: server.fetch, window: win, env });
-    return { server, status: plain(status), win, canvas, workers, frames: server.posts.filter(p => p.path === '/frame') };
+    return { server, status: plain(status), win, canvas, workers, frames: server.posts.filter(p => p.path.startsWith('/frame?n=')) };
 }
 
 check('render mode: boot.json, report, then every frame in order (png)', async () => {
@@ -88,14 +89,15 @@ check('render mode: boot.json, report, then every frame in order (png)', async (
     assert.equal(r.win.document.body.style.margin, '0', 'the body margin is set through the CSSOM');
     assert.equal(r.win.document.documentElement.style.overflow, 'hidden', 'no scrollbars: the screenshot transport captures the viewport');
     assert.equal(r.canvas.style.display, 'block', 'no inline-canvas gap below the canvas');
-    assert.deepEqual(r.server.posts.map(p => p.path).slice(0, 2), ['/report', '/frame']);
+    assert.deepEqual(r.server.posts.map(p => p.path).slice(0, 2), ['/report', '/frame?n=0']);
     const rep = r.server.posts[0].report;
     assert.equal(rep.frame_count, 354); assert.equal(rep.width, 192); assert.equal(rep.height, 108);
     assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
     assert.equal(r.server.posts[0].headers['Content-Type'], 'application/json');
     assert.equal(r.frames.length, 354);
     r.frames.forEach((f, i) => {
-        assert.equal(f.headers['X-SM-Frame'], String(i)); assert.equal(f.headers['Content-Type'], 'image/png'); assert.equal(f.body.type, 'image/png');
+        assert.equal(f.path, '/frame?n=' + i); assert.equal(f.headers['Content-Type'], 'text/plain', 'a CORS-simple request: no preflight per frame');
+        assert.equal(f.headers['X-SM-Frame'], undefined); assert.equal(f.body.type, 'image/png');
     });
     assert.equal(r.canvas.width, 192); assert.equal(r.canvas.height, 108);
     assert.equal(r.server.posts.filter(p => p.path === '/error').length, 0);
@@ -109,14 +111,14 @@ check('render mode: boot.json, report, then every frame in order (png)', async (
 check('render mode: webp, raw RGBA and the screenshot transport', async () => {
     const webp = await runRender({ transport: 'webp', story: minimal, noCode: true });
     assert.equal(webp.status.phase, 'done'); assert.equal(webp.frames.length, 42);
-    assert.equal(webp.frames[0].headers['Content-Type'], 'image/webp'); assert.equal(webp.frames[0].body.quality, 1, 'lossless webp');
+    assert.equal(webp.frames[0].headers['Content-Type'], 'text/plain'); assert.equal(webp.frames[0].body.type, 'image/webp'); assert.equal(webp.frames[0].body.quality, 1, 'lossless webp');
     const raw = await runRender({ transport: 'raw', story: minimal, noCode: true });
     assert.equal(raw.status.phase, 'done'); assert.equal(raw.frames.length, 42);
-    assert.equal(raw.frames[3].headers['Content-Type'], 'application/octet-stream');
+    assert.equal(raw.frames[3].headers['Content-Type'], 'text/plain');
     assert.equal(raw.frames[3].body.byteLength, 192 * 108 * 4, 'raw RGBA is w*h*4 bytes');
     const shot = await runRender({ transport: 'screenshot', story: minimal, noCode: true });
     assert.equal(shot.status.phase, 'ready'); assert.equal(shot.status.frame_count, 42);
-    assert.equal(shot.server.posts.filter(p => p.path === '/frame').length, 0, 'the screenshot transport posts no frames');
+    assert.equal(shot.server.posts.filter(p => p.path.startsWith('/frame')).length, 0, 'the screenshot transport posts no frames');
     assert.equal(typeof shot.win.__smRenderFrame, 'function');
     const before = shot.canvas.ctx.log.filter(l => l.startsWith('drawImage')).length;
     assert.equal(await shot.win.__smRenderFrame(7), 7);
@@ -125,6 +127,17 @@ check('render mode: webp, raw RGBA and the screenshot transport', async () => {
     await assert.rejects(() => shot.win.__smRenderFrame(42), /outside 0\.\.41/);
 });
 
+check('render mode: boot.frame_deadline_ms replaces the 250 ms frame deadline of the preview', async () => {
+    const limits = { warmupFrames: 0, frameDeadlineMs: 40 };
+    const control = [], offered = [], absurd = [];
+    assert.equal((await runRender({ story: minimal, noCode: true, limits, timers: control })).status.phase, 'done');
+    assert.ok(control.includes(40), 'without frame_deadline_ms every frame call waits at most the preview deadline');
+    assert.equal((await runRender({ story: minimal, noCode: true, limits, timers: offered, boot: { frame_deadline_ms: 5000 } })).status.phase, 'done');
+    assert.ok(offered.includes(5000) && !offered.includes(40), 'the deadline Go offers in boot.json is the frame deadline of the render');
+    assert.equal((await runRender({ story: minimal, noCode: true, limits, timers: absurd, boot: { frame_deadline_ms: 3600000 } })).status.phase, 'done');
+    assert.ok(absurd.includes(60000) && !absurd.includes(3600000), 'a render page never waits longer than a minute for one frame');
+});
+
 check('render mode: restart, stop, refusal, frame limit and failures', async () => {
     let restarted = false;
     const restart = await runRender({ story: minimal, noCode: true, answer: n => { if (n === 5 && !restarted) { restarted = true; return { restart: true }; } return null; } });
--- a/ui/storymaker_player_gate_browser_test.go
+++ b/ui/storymaker_player_gate_browser_test.go
@@ -345,6 +345,8 @@ type storymakerIngest struct {
 	frame5    []byte
 	shot      []byte
 	saveDir   string
+	frameHdr  int // frame requests that carried an X-SM-Frame header (none should)
+	framePre  int // preflights of POST /frame (none should)
 }
 
 func (in *storymakerIngest) cors(w http.ResponseWriter, r *http.Request) {
@@ -382,12 +384,15 @@ func TestStorymakerRenderModeBrowser(t *testing.T) {
 			if fmt.Sprint(in.frames) != fmt.Sprint(wantFrames) {
 				t.Fatalf("frames posted = %v, want %v", in.frames, wantFrames)
 			}
-			mime := map[string]string{"png": "image/png", "webp": "image/webp", "raw": "application/octet-stream"}[transport]
+			// frames are CORS-simple requests (frame number in the query, text/plain): Chrome sends no preflight per frame
 			for i, ct := range in.types {
-				if ct != mime {
-					t.Fatalf("frame %d content type %q, want %q", i, ct, mime)
+				if ct != "text/plain" {
+					t.Fatalf("frame %d content type %q, want text/plain", i, ct)
 				}
 			}
+			if in.frameHdr != 0 || in.framePre != 0 {
+				t.Errorf("%d frame requests carried an X-SM-Frame header and %d preflights hit /frame: frames must be CORS-simple", in.frameHdr, in.framePre)
+			}
 			if transport == "raw" {
 				for i, n := range in.sizes {
 					if n != in.width*in.height*4 {
@@ -442,6 +447,9 @@ func runStorymakerRenderPage(t *testing.T, browser *rod.Browser, transport, stor
 			}
 			if r.Method == http.MethodOptions {
 				in.options++
+				if strings.HasSuffix(r.URL.Path, "/frame") {
+					in.framePre++
+				}
 			}
 			in.mu.Unlock()
 			in.cors(w, r)
@@ -526,7 +534,7 @@ func runStorymakerRenderPage(t *testing.T, browser *rod.Browser, transport, stor
 	handle("/frame", func(w http.ResponseWriter, r *http.Request) {
 		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
 		var n int
-		fmt.Sscanf(r.Header.Get("X-SM-Frame"), "%d", &n)
+		fmt.Sscanf(r.URL.Query().Get("n"), "%d", &n)
 		in.mu.Lock()
 		defer in.mu.Unlock()
 		in.frames = append(in.frames, n)
@@ -540,6 +548,9 @@ func runStorymakerRenderPage(t *testing.T, browser *rod.Browser, transport, stor
 			_ = os.WriteFile(filepath.Join(in.saveDir, fmt.Sprintf("storymaker-%s-%03d.png", in.story, n)), body, 0o644)
 		}
 		in.types = append(in.types, r.Header.Get("Content-Type"))
+		if r.Header.Get("X-SM-Frame") != "" {
+			in.frameHdr++
+		}
 		in.sizes = append(in.sizes, len(body))
 		if transport == "png" && len(body) > 24 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
 			in.pngDims = append(in.pngDims, [2]int{int(binary.BigEndian.Uint32(body[16:20])), int(binary.BigEndian.Uint32(body[20:24]))})
--- a/ui/storymaker_parity_browser_test.go
+++ b/ui/storymaker_parity_browser_test.go
@@ -212,7 +212,7 @@ func runStorymakerParity(t *testing.T, browser *rod.Browser, job storymakerParit
 	handle("/frame", func(w http.ResponseWriter, r *http.Request) {
 		data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
 		var n int
-		_, _ = fmt.Sscanf(r.Header.Get("X-SM-Frame"), "%d", &n)
+		_, _ = fmt.Sscanf(r.URL.Query().Get("n"), "%d", &n)
 		mu.Lock()
 		defer mu.Unlock()
 		if name, ok := wanted[n]; ok {
```

- [ ] **Step 2: Run the Node suite to verify it fails**

```bash
node scripts/test-storymaker-player.mjs
```

Expected: FAIL. Recorded:

```text
FAIL render mode: webp, raw RGBA and the screenshot transport: AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
FAIL render mode: boot.frame_deadline_ms replaces the 250 ms frame deadline of the preview: AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
FAIL render mode: restart, stop, refusal, frame limit and failures: AssertionError [ERR_ASSERTION]: Expected values to be strictly equal:
FAIL render mode: boot.json, report, then every frame in order (png): AssertionError [ERR_ASSERTION]: {"phase":"error","frame":0,"error":"render_failed","message":"POST /frame answered 404: {}"}
68 passed, 4 failed
```

- [ ] **Step 3: Implement**

```diff
--- a/ui/js/storymaker/host-render.js
+++ b/ui/js/storymaker/host-render.js
@@ -1,7 +1,8 @@
 // Storymaker host (document realm): render mode, the page that headless Chrome opens for an export. It drives itself from
 // <base>/boot.json (contract section 7): fetches story, catalog, fonts (only from boot.fonts) and code-scene bundles from the
 // base, compiles, POSTs the compile report, waits for {start: true}, then renders frames in order and POSTs each one to the
-// fixed <base>/frame with the header X-SM-Frame. Transports: png, webp, raw RGBA, or screenshot (the page only draws on
+// fixed <base>/frame?n=<frame> as a CORS-simple request (Content-Type text/plain, no custom header, so Chrome sends no
+// preflight per frame). Transports: png, webp, raw RGBA, or screenshot (the page only draws on
 // window.__smRenderFrame(n); Go captures). Answers: {ok}, {ok, done}, {restart}, {stop}. Failures are POSTed to <base>/error.
 // window.__smStatus mirrors the phase for the driver. Classic script; publishes StorymakerHost.mountRender.
 (function (root) {
@@ -57,7 +58,9 @@
             }
             canvas.width = boot.width;
             canvas.height = boot.height;
-            var ctx = canvas.getContext('2d', { willReadFrequently: boot.transport === 'raw' });
+            // A render is not real time: Go offers a per-frame deadline far above the 250 ms of the app preview.
+            if (boot.frame_deadline_ms > 0) env.limits.frameDeadlineMs = Math.min(60000, boot.frame_deadline_ms);
+            var ctx =canvas.getContext('2d', { willReadFrequently: boot.transport === 'raw' });
             var story = await getJSON(boot.story_url);
             var catalog = await getJSON(boot.catalog_url);
             var bundles = {}, ids = Object.keys(boot.code_bundles || {});
@@ -157,7 +160,7 @@
                 }
                 await drawFrame(n);
                 var body = await encode(boot.transport);
-                var reply = await post('/frame', body, { 'Content-Type': MIME[boot.transport], 'X-SM-Frame': String(n) });
+                var reply = await post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' });
                 if (reply.stop) { status.phase = 'stopped'; status.error = reply.error || ''; status.message = reply.message || ''; return status; }
                 if (reply.restart) { n = 0; status.restarts = (status.restarts || 0) + 1; continue; }
                 if (reply.done) { status.phase = 'done'; return status; }
--- a/internal/storymaker/render/browser_smoke_test.go
+++ b/internal/storymaker/render/browser_smoke_test.go
@@ -438,12 +438,15 @@ func (i memInfo) ModTime() time.Time { return time.Time{} }
 func (i memInfo) IsDir() bool        { return false }
 func (i memInfo) Sys() any           { return nil }
 
-// recommendedPlayerChanges are the two changes to the player the plan asks part C for: frames as
-// CORS-simple requests (no preflight per frame) and a render-mode frame deadline from boot.json
-// (frame_deadline_ms) instead of the preview's 250 ms. Until C lands them the spike applies them to
-// the bundle text in memory.
+// recommendedPlayerChanges are the two changes of task B15 to the player: frames as CORS-simple
+// requests (no preflight per frame) and a render-mode frame deadline from boot.json
+// (frame_deadline_ms) instead of the preview's 250 ms. Before B15 the spike applies them to the
+// bundle text in memory; on a player that has them they change nothing.
 func recommendedPlayerChanges(src []byte) []byte {
 	src = simpleFrameRequests(src)
+	if bytes.Contains(src, []byte("boot.frame_deadline_ms")) {
+		return src
+	}
 	return bytes.Replace(src, []byte("canvas.height = boot.height;\n"),
 		[]byte("canvas.height = boot.height;\n            if (boot.frame_deadline_ms > 0) env.limits.frameDeadlineMs = Math.min(60000, boot.frame_deadline_ms);\n"), 1)
 }
@@ -456,6 +459,14 @@ func simpleFrameRequests(src []byte) []byte {
 		[]byte("post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' })"), 1)
 }
 
+// preflightFrameRequests is the reverse of simpleFrameRequests: the page sends frames with the
+// X-SM-Frame header, which Chrome preflights for an opaque origin (the player before task B15).
+func preflightFrameRequests(src []byte) []byte {
+	return bytes.Replace(src,
+		[]byte("post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' })"),
+		[]byte("post('/frame', body, { 'Content-Type': MIME[boot.transport], 'X-SM-Frame': String(n) })"), 1)
+}
+
 // TestGuardAndRequestOverheadSpike times the png transport with the protection layers switched off
 // one by one, and with the two player changes the plan asks for (AURAGO_STORYMAKER_SPIKE=1). It is
 // the evidence for the hybrid guard and for frames as CORS-simple requests.
@@ -470,7 +481,7 @@ func TestGuardAndRequestOverheadSpike(t *testing.T) {
 		tweak func(o *sessionOptions)
 	}
 	for i, v := range []variant{
-		{"preflight per frame, hybrid guard", func(b []byte) []byte { return b }, nil},
+		{"preflight per frame, hybrid guard", preflightFrameRequests, nil},
 		{"simple requests, hybrid guard", simpleFrameRequests, nil},
 		{"simple requests, full guard", simpleFrameRequests, func(o *sessionOptions) { o.GuardFull = true }},
 		{"simple requests, no guard", simpleFrameRequests, func(o *sessionOptions) { o.NoGuard = true }},
```

Rebuild the bundle:

```bash
npm run build:ui
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
npm run check:ui
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run "TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser" -count=1 -timeout 20m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_PARITY_DIR=<an absolute folder> go test ./ui -run TestStorymakerParityBrowser -count=1 -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render/ -run "TestRealRender|TestEgressLayers" -count=1 -timeout 20m
```

Expected: PASS. Recorded at integration time:

```text
node scripts/test-storymaker-player.mjs: 72 passed, 0 failed
npm run check:ui: green
C browser tests:
--- PASS: TestStorymakerPlayerGateBrowser (1.63s)
--- PASS: TestStorymakerRenderModeBrowser (2.56s)
    --- PASS: TestStorymakerRenderModeBrowser/png (1.12s)
    --- PASS: TestStorymakerRenderModeBrowser/webp (0.65s)
    --- PASS: TestStorymakerRenderModeBrowser/raw (0.43s)
--- PASS: TestStorymakerFirewallRenderBrowser (12.79s)
--- PASS: TestStorymakerRenderScreenshotBrowser (7.91s)
PASS
ok  	aurago/ui	25.247s
R parity (cast sheet 27 frames, firewall in 16:9, 9:16 and 1:1):
--- PASS: TestStorymakerParityBrowser (94.08s)
    --- PASS: TestStorymakerParityBrowser/castsheet (53.50s)
    --- PASS: TestStorymakerParityBrowser/firewall-16x9 (14.38s)
    --- PASS: TestStorymakerParityBrowser/firewall-9x16 (16.11s)
    --- PASS: TestStorymakerParityBrowser/firewall-1x1 (9.85s)
PASS
ok  	aurago/ui	94.477s
B render package:
--- PASS: TestBrowserStatusProbeLaunchesRealChrome (1.81s)
--- PASS: TestRealRenderOfTheFirewallStory (33.20s)
--- PASS: TestRealRenderIsDeterministic (68.16s)
--- PASS: TestEgressLayersEachStopTheProbe (23.79s)
ok  	aurago/internal/storymaker/render	127.004s
```

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js scripts/storymaker-tests/09-host-render.mjs ui/storymaker_player_gate_browser_test.go ui/storymaker_parity_browser_test.go internal/storymaker/render/browser_smoke_test.go
git commit -m "feat(storymaker): render page sends frames as simple requests and honours the frame deadline" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
