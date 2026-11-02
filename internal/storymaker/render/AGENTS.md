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
