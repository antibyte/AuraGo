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
  secret against scene code. The prefix is the credential for reads only; a scene cannot obtain the token that `/report`, `/frame` and `/error` need.
- **Narrate jobs** (`jobs.go`): the server runs a second `Manager` for narrate jobs (`MaxQueued` 128). `Kinds` names what a manager loads and accepts, and `Workers` other than 0 or 1 is refused. `EnqueueRequest.Coalesce` returns a queued or running job of the same project and kind with replayed true, under `m.mu` in the same step as the insert. `SetPieces` fills `pieces_done` and `pieces_total`. A narrate job has no `output` key.
- **Token** (`listener.go`): `boot.json` serves a 32-hex token from `crypto/rand` on its first GET only (`atomic.Bool` swap; HEAD consumes nothing). `/report`, `/frame` and `/error` compare `t` in constant time. A mismatch answers 403 `bad_token`, calls no hook and consumes nothing. `StartListener` refuses a shorter token.
- **Report** (`report.go`): at most 8 MiB. `parseReport` checks every field Go uses against its own plan, with `filmMS` from `frame_count` and `fps`, never the report's own value: known block, kind and scene, `Hashes[piece] == ref`, unique pairs, a piece prefix without a gap, at least piece 0 for each measured narration block, times inside the film, at most `MaxEvents` events and 8000 captions, and caption text that is a substring of its block (`captionKey`). An empty `audio_events` with measured blocks is refused. A `gain` from JS is ignored.
- **Audio info and fonts** (`render.go`, `listener.go`): `Plan.AudioJSON` is marshalled once and served in `boot.audio_info`, so it equals the `narration` of `GET /narration`; nil serves `{}`. `boot.fonts` lists the story's faces and every fallback face. The page fetches a face only when compile needs it, and a response over `max_bytes` is a readiness failure. `glyphs.json` is in the snapshot. Exports stay silent in batch 2B: the audio track is still `anullsrc`.
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
- **DevTools pipe** (`devtools_pipe*.go`): the render browser has no DevTools port. A port on loopback is reachable by every account of the
  host: `GET /json/list` names the render URL (the secret prefix) and the websocket drives the browser past the CSP and the CDP guard. So the
  browser starts with `--remote-debugging-pipe` (descriptors 3 and 4; on Windows the handles named by `--remote-debugging-io-pipes`) and rod
  gets a pipe `cdp.Client`, because rod's launcher only knows the websocket. The browser quits when its input closes, so a crashed server
  leaves no browser behind. `newLauncher` only builds the flag set. `TestRenderBrowserOpensNoDevToolsPort` checks `DevToolsActivePort` and the
  listening sockets of the browser tree; `devtools_pipe_test.go` covers the framing, start-up failures (with the browser's own output in the
  error, which `launchIssue` reads) and the kill path with the test binary as a fake browser, on Windows and Linux.
- **Browser** (`browser.go`): `render.browser_path`, else `launcher.LookPath()`, else the pinned Chromium (revision 1321438, SHA-256 per
  platform in `pinnedChromium`, downloaded into `<DataDir>/storymaker/chromium-<rev>/` by the first export, never by a status request). A
  browser is `ready` only after a cached headless launch probe. Production launches never set `NoSandbox` (`newLauncher` deletes the flag rod adds
  inside containers); `SetNoSandboxForTests` is the only way to set it and nothing outside tests calls it. The pinned download gives up when no
  byte arrives for 60 s (`downloadIdleTimeout`; a slow link that keeps delivering is left alone). On Ubuntu 23.10+ the pinned
  download only starts with its sandbox when `install.sh`/`update.sh` wrote the AppArmor profile that names
  `<install dir>/data/storymaker/chromium-*/chrome` (second profile of `/etc/apparmor.d/aurago-chromium-<user>`); the folder must stay
  below the data folder, because the hardened systemd unit can write only below the install directory. Moving it needs the profile
  (`chromium_sandbox_profile_text` in both scripts, pinned by `internal/audit/chromium_sandbox_profile_test.go`) and the installation
  guide changed with it.
- **Timeouts**: the browser setup of a first export (the Chromium download) runs under `render.timeout_seconds` before the render clock starts
  (`OpenPage` keeps the job context: the browser lives as long as it). Report within 3 minutes of the navigation, no frame within 10 s after `start` kills the job as `code_error` (the timer is held while Go itself works on a delivered frame: FFmpeg write, hardware settle, software restart), overall
  `max(render.timeout_seconds, 3 x frames / measuredFPS[transport])`. Any `POST /error` from the page is fatal.
- **Tests**: everything but `browser_smoke_test.go`, `egress_smoke_test.go` and the parity test in `internal/server` runs without a browser (a
  fake page plays the render page against the real listener). The browser tests need `AURAGO_RUN_BROWSER_SMOKE=1`; the transport spike also
  needs `AURAGO_STORYMAKER_SPIKE=1`. On the Windows development box run them through `go test` with `GOTMPDIR=D:\tmp\go-build` (a repo-built
  executable cannot start Chrome there).
- **Transport spike** (task B13; `firewall.json`, 342 frames, 1920x1080, 30 fps, strict proxy and CDP guard active, every transport rendered
  twice; the two renders and all three transports give one sequence hash, so every transport is deterministic and pixel-identical to png).
  Development box (Windows, 16 CPUs, Chrome 156, FFmpeg 8.1.1): png 10.2 fps, WebP 4.9 to 5.3, screenshot 3.8; png peaks at 708 MB Chrome and
  40 MB Go heap. aurago-test (Linux, 12 CPUs, about 2 load from the live service, pinned Chromium 128, FFmpeg 8.0.1): png 8.7 to 8.9 fps
  (342 frames in 42 s), WebP 4.9, screenshot 4.0. png stays `DefaultTransport` (the fastest deterministic one), but on aurago-test it is below
  the 10 fps target of the spec; `measuredFPS` (6, 2.5, 1, 2.5) was not recalculated: 60 percent of 8.8 fps is 5.3, which would give a 15 minute
  video a 15300 s watchdog, above the 14400 s ceiling of `render.timeout_seconds` and the 13500 s that `TestRenderTimeoutFormula` pins.
  Pinned Chromium on aurago-test: the download (181 MB), checksum, unpacking, launch (Chrome 128.0.6568.0) and the export of `firewall.json`
  pass with `AURAGO_TEST_NO_SANDBOX=1`. Without it the launch failed with `No usable sandbox`: the AppArmor profile of the installation guide
  only named `~/.cache/rod/browser/chromium-*/chrome`, not the data folder the pinned download goes to. `install.sh`/`update.sh` and the
  guide now also name `<install dir>/data/storymaker/chromium-*/chrome`; the rerun of this step on aurago-test without
  `AURAGO_TEST_NO_SANDBOX=1` (after `./update.sh` there) is still open. FFmpeg 8.0.1 there passes the frame-input tests (the 5.1.9 floor is
  still unmeasured).
