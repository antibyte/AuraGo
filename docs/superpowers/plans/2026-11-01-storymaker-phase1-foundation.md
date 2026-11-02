# Storymaker Phase 1 (Foundation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A new desktop app "Storymaker" that turns a storyboard JSON (plus optional JavaScript code scenes) into an animated explainer video: catalog-based characters, props and environments, a live preview in the app, and a silent MP4 export rendered server-side.

**Architecture:** One deterministic JavaScript player (`ui/js/storymaker/`) runs in three realms: the host document, a trusted player worker and one untrusted worker per code scene. The app shows it in an opaque `sandbox="allow-scripts"` iframe; the export runs the same player in a headless Chrome on a per-job `127.0.0.1` listener behind a strict capture proxy and a CDP egress guard, and pipes the frames into FFmpeg. Go (`internal/storymaker`) owns the story schema, strict decoding, validation, defaults, the catalog index, esbuild bundling of code scenes and the project store; `internal/storymaker/render` owns jobs, browser provisioning, the listener, ingest and encoding; `internal/server/storymaker_*.go` serves the API.

**Tech Stack:** Go 1.27 (esbuild Go API, go-rod, existing desktop service and capture proxy), FFmpeg (existing Video Studio encoder selection), plain classic-script JavaScript (no framework, concatenated by `scripts/build-ui-bundles.js`), Canvas2D/OffscreenCanvas/Path2D in workers, Node 24 for the generator and the vm-based player tests, Chrome/Chromium headless.

---

This index is `docs/superpowers/plans/2026-11-01-storymaker-phase1-foundation.md`. The parts and the drafting notes the part intros refer to sit in `docs/superpowers/plans/2026-11-01-storymaker-phase1/` next to it, and every link below resolves from there. The notes are historical records of the drafting: paths in them that name scratch worktrees (`sm-wt-*`, `sm-plan`) no longer exist.

## Documents

| Document | Role |
| --- | --- |
| [Design spec](../specs/2026-11-01-storymaker-design.md) | What and why; phase table; security model. |
| [Interface contract](../specs/2026-11-01-storymaker-contract.md) | Normative formats and protocols: story schema `storymaker/1` (section 2), catalog `storymaker-catalog/1` and rig semantics (3, 3a, 3b), Go API (4), HTTP API (5), player markup and bundles (6), render listener (7), app/iframe protocol (8), desktop app (9). Code comments and tasks cite it as "contract section N". Sections 0, 1, 10 and 11 were drafting-only and are omitted; the deviations from the spec (old section 12) are folded into the spec, see its list "Changes after phase-1 drafting". |
| Parts below | The tasks, with complete code. |

## Execution order

Run the parts in this order; inside a part, run the tasks in order. Every task that changes behaviour was drafted as working code first and replayed: its tests fail before the implementation and pass after it. Tasks A8, R1, B13, B14, E10, I5 and I6 have no failing-test step (documentation, checks or the aurago-test step), as their own texts say. The integration step ran all parts together on main `561685cde` and recorded the matrix (see Verification at the end).

| Order | Part | Tasks | Needs | Delivers |
| --- | --- | --- | --- | --- |
| 1 | [A: Go core](2026-11-01-storymaker-phase1/part-A-go-core.md) | A1–A8 | main | `internal/storymaker` (schema, decode, validate, defaults, catalog, code-scene bundling, project store), `storymaker` config section |
| 2 | [D: Starter catalog](2026-11-01-storymaker-phase1/part-D-catalog.md) | D1–D8 | – | generator `assets/storymaker/`, `scripts/build-storymaker-catalog.mjs`, outputs `ui/img/storymaker/**`, `ui/lang/storymaker_catalog/*` |
| 3 | [C: JS player](2026-11-01-storymaker-phase1/part-C-player.md) | C1–C12 | A (fixtures) | `ui/js/storymaker/*`, `host.bundle.js`, `player.bundle.js`, Node tests, phase-1 gate browser test |
| 4 | [R: Player vs real catalog](2026-11-01-storymaker-phase1/part-R-reconcile.md) | R1–R9 | A, D, C | the player renders the real catalog correctly (seat drop, text box units, mirrored point poses, layer order, aspect spots, dark-environment text), catalog version `ab33e88a6b77`, parity browser test |
| 5 | [B: Server and render pipeline](2026-11-01-storymaker-phase1/part-B-server-render.md) | B1–B15 | A, C, D, R | strict capture proxy, job manager, browser provisioning (pinned Chromium), listener, ingest, FFmpeg pipeline, render job, HTTP API, player host page, frame-transport spike, CORS-simple frame posts |
| 6 | [E: Desktop app](2026-11-01-storymaker-phase1/part-E-desktop-app.md) | E1–E11 | A, C, D | registration, app modules `ui/js/desktop/apps/storymaker*.js`, stylesheet, 16 locales, config UI, docs, browser tour against a stand-in server |
| 7 | [I: Integration and review fixes](2026-11-01-storymaker-phase1/part-I-integration.md) | I1–I10 | all | real-server end-to-end export and app tour, app/server mismatch fixes, the review fixes (stale bundle, bounded first export, no DevTools port, app open/leave/add races), the full matrix as its last step |

## Setup for the executor

- **Worktree.** `git worktree add .worktrees/storymaker-p1 -b storymaker-p1 561685cde` (superpowers:using-git-worktrees). Every anchor and line number in the parts was verified at `561685cde`, so the branch starts there, not at whatever `main` is when you run this. If `main` has moved, rebase the branch onto it and re-check each diff block with `git apply --check` before editing. All tasks run there; commit each task on the branch with plain `git add` / `git commit` and the attribution line the tasks show. Nested `AGENTS.md` files are ignored by `.gitignore` (only the root one is excepted): the tasks that create them use `git add -f`.
- **Node.** The worktree sits inside the repository, so Node resolves the main checkout's `node_modules` (or run `npm ci` once). esbuild 0.28.2 is already a dependency.
- **GitNexus (CLAUDE.md).** `.gitnexus/` is git-excluded, so the worktree has no copy of it. The tasks call the runner of the main checkout by its absolute path, `node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs ...`, so the path does not depend on the working directory; `--repo .` is kept as written. The index reflects main, not the branch. Before editing an existing function run `impact` on it and report callers and risk; before every commit run `detect_changes`. Functions created by earlier tasks of this plan are not in the index until it is rebuilt: an empty or `UNKNOWN` answer is not an all-clear, confirm callers with a text search and say so in the commit review.
- **Go on the Windows development box.** `GOCACHE=D:\dev-cache\go-build`, `GOTMPDIR=D:\tmp\go-build` (backslash form).
- **Browser tests** are gated by `AURAGO_RUN_BROWSER_SMOKE=1`. They need the installed Chrome (`AURAGO_TEST_CHROME` overrides the lookup) and FFmpeg (`AURAGO_TEST_FFMPEG_DIR`, `PATH` or the WinGet package folder; FFmpeg is not on the Git Bash `PATH`). On the Windows box a repo-built executable cannot start Chrome, so run them through `go test` (test binaries live under `D:\tmp\go-build`) with the shell sandbox off. `AURAGO_TEST_NO_SANDBOX=1` is only for a container without a usable Chrome sandbox; production never disables it. Further opt-ins: `AURAGO_STORYMAKER_PARITY_DIR=<absolute folder>` for R's `TestStorymakerParityBrowser` (it skips without it and writes the frames there); `AURAGO_STORYMAKER_SPIKE=1` for B13's transport spike; `AURAGO_STORYMAKER_PINNED=1` for the pinned Chromium download and launch (about 190 MB, not in the matrix); `AURAGO_SM_PINNED_DIR=<folder>` for `TestPinnedArchivesExtract`, which checks the four downloaded archives in that folder.
- **Line endings.** The checkout may convert to CRLF. C11 adds one `.gitattributes` line, `ui/js/storymaker/*.bundle.js text eol=lf`, so the generated bundles stay byte-identical; the code-scene fixtures are not pinned (the tests normalise CRLF). Apply edits to existing files with the Edit tool or `git apply --ignore-whitespace`, never by pasting whole files over them.

## Tasks

- [Part A](2026-11-01-storymaker-phase1/part-A-go-core.md): A1 config section · A2 story model and strict decoding · A3 catalog index · A4 defaults and issue helpers · A5 validation · A6 code scene bundling · A7 slugs, project store, in-memory store · A8 package contract and final check
- [Part D](2026-11-01-storymaker-phase1/part-D-catalog.md): D1 path reader, shape helpers, section 3a renderer · D2 heads, faces, hair · D3 bodies, garments, assembled rig · D4 poses, gestures, cycles · D5 23 props · D6 four environments · D7 manifest, thumbnails, locale names, build script, committed outputs · D8 README and final verification
- [Part C](2026-11-01-storymaker-phase1/part-C-player.md): C1 test harness, shared fixtures, catalog-mini stubs · C2 easing, tween tracks, frame schedule · C3 stage mapping, vector parts, text, camera · C4 2D rig · C5 compile (scenes, narration timing, beats, report) · C6 renderFrame and transitions · C7 code scenes and determinism shadows · C8 worker realm · C9 host engine and app protocol · C10 render driver and host entry · C11 bundles, player stylesheet, static guards · C12 phase-1 gate and render page in a real browser
- [Part R](2026-11-01-storymaker-phase1/part-R-reconcile.md): R1 machine-speed independent host tests · R2 seated root offset · R3 text box units · R4 mirrored point poses · R5 gesture hand shapes hold · R6 layer order and aspect spots · R7 dark-environment text, actor framing (catalog data fix) · R8 real-catalog checks, cast sheet story, Go guard · R9 visual parity browser test
- [Part B](2026-11-01-storymaker-phase1/part-B-server-render.md): B1 strict capture proxy · B2 encoder exports · B3 job manager · B4 browser provisioning · B5 FFmpeg frame-input pipeline · B6 frame ingest · B7 render listener · B8 report validation, egress guard, browser session · B9 render job · B10 render pipeline browser tests · B11 server API, player host page, desktop wiring · B12 app/render parity test · B13 frame-transport spike and the aurago-test step · B14 documentation · B15 CORS-simple frame posts and the frame deadline
- [Part E](2026-11-01-storymaker-phase1/part-E-desktop-app.md): E1 manifest, logo, app name in 16 languages · E2 app core, model, player bridge · E3 API client and asset resolver · E4 app strings in 16 languages · E5 stylesheet · E6 desktop shell registration · E7 view modules and static guards · E8 composition root and browser tour · E9 configuration section · E10 documentation · E11 full verification and graph check
- [Part I](2026-11-01-storymaker-phase1/part-I-integration.md): I1 export end to end and config round trip · I2 app tour against the real handlers · I3 test resolver keeps the pinned Chromium · I4 app adapts to the real server · I5 documentation after integration · I6 the whole matrix · I7 no stale bundle after a failed code save · I8 bounded browser setup of a first export · I9 no DevTools port on the render browser · I10 leaving, opening and adding a code scene

## Verification at the end (last step of task I10)

The table was recorded on main `561685cde` at integration time (task I6). Task I10 step 5 runs these rows on the finished branch, with the opt-in variables of Setup.

| Command | Expected |
| --- | --- |
| `npm run build:ui`, `npm run check:ui` | green |
| `node scripts/test-storymaker-player.mjs` | 72 passed, 0 failed |
| `node scripts/build-storymaker-catalog.mjs --check` | `storymaker catalog ab33e88a6b77 is up to date` |
| `node --test assets/storymaker/test/<file>.test.mjs`, one call per file (7 files) | 6, 3, 5, 4, 3, 4, 6 passing |
| `go test ./internal/storymaker/... ./internal/config/ ./internal/desktop/ ./internal/security/ ./internal/videostudio/ ./cmd/config-merger ./internal/setup -count=1` | ok |
| `go test ./internal/server/ -count=1`, `go test ./ui -count=1` | ok except the pre-existing failures below |
| `GOOS=linux go build ./...`; `go vet` of the touched packages; `gofmt -l` | clean |
| Browser (`AURAGO_RUN_BROWSER_SMOKE=1`; R's parity test also needs `AURAGO_STORYMAKER_PARITY_DIR`): C's four player tests, R's `TestStorymakerParityBrowser`, B's `TestBrowserStatusProbe`, `TestRealRender*`, `TestEgressLayers*`, `TestStorymakerParity*`, E's `TestStorymakerApp*` and `TestConfigStorymakerBrowser`, I's `TestRenderBrowserOpensNoDevToolsPort` (task I9), `TestStorymakerAppRealServer*` and `TestStorymakerExportEndToEnd` | PASS |

Pre-existing failures on main `561685cde` (verified on a clean checkout, not caused by this plan): `ui` `TestChatFrontend_IntegrationsDrawerRemainsWired` and `TestFrontend_StaticI18nKeysExistInEnglishBundle` (seven `findit-*` keys); `internal/server` `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults` (only when `GOTMPDIR` is on another drive than the checkout).

## Measured during drafting

- Phase-1 gate passed in Chrome 156: blob workers with `importScripts`, OffscreenCanvas, Path2D, FontFace and ImageBitmap transfer work in the opaque sandboxed iframe under the player CSP; `fetch` from a code scene is blocked. No fallback needed (the storyboard fallback exists and is tested).
- Export of `firewall.json` (342 frames, 1080p30): about 31 s on the Windows box, 33 s in a Debian 12 container with FFmpeg 5.1.9; two renders give identical decoded frames; app preview and render page draw identical pixels.
- Frame transport on the production path (strict proxy, CDP guard): png 10.5 fps, webp 4.9, screenshot 4.1, raw 1.5; default `png`.

## Deviations from the spec

Everything phase-1 drafting changed against the approved spec is folded into the spec itself (its list "Changes after phase-1 drafting" at the end) and into the contract: CORS-simple frame posts (`POST <base>/frame?n=<n>`, `text/plain`), `boot.frame_deadline_ms`, the hybrid CDP egress guard, the render browser without a DevTools port, `status.browser.can_download`, `GET /projects/{id}/jobs`, the queue-time snapshot, the `interrupted` job code, the silent AAC track, the export into the project folder, the pose/rig/text additions of parts D and R, Geist as the default font, and the voice provider check of task A5.

## Open after phase 1

- **aurago-test acceptance (task B13 steps):** Linux transport spike (target 10 fps), Chrome sandbox under AppArmor, hardware encoders, the real 190 MB pinned Chromium download and launch.
- **Docker deployments:** the image has no Chromium; exports need `storymaker.render.browser_path` or the pinned download, and Chrome refuses to start as root without `--no-sandbox`, which production never passes. Decide before deploying in containers.
- **Not yet run:** an asset-pack-built binary with the full production middleware chain (a local scratch instance covers it).
- **Export location:** the MP4 is written into the project folder (`exports/<slug>-<job8>.mp4`), not into `Videos/Storymaker/` as the spec says; the library copy moves to phase 2 (contract section 12, item 6).
- **FFmpeg 5.1.9 floor:** the colour-conversion test of B13 has never run on FFmpeg 5.1.9.
- **Other browsers:** the app preview is tested in Chrome only; Firefox and Safari are untested.
- **Asset-set headroom:** with phase 1 the asset set holds 17986 files and 1.05 GiB against ceilings of 20000 files and 2 GiB; the phase 4 catalog expansion has about 2000 files of room (or must raise `MaxFiles`).
- **Scrubber:** the scrubber shows no beat markers.
- **Minor review findings kept for later:** the id `constructor` resolves to an `Object.prototype` member in the player's name lookups; a superseded preview load leaks its workers until the next load; deleting a project also deletes its exports (the dialog says so); scene code can read the render listener prefix (capability limited to its own job, documented).
- **Known limits:** one time-based playback check in the player suite failed once in about six runs (watch it); the start screen waits for the first status call (3–7 s, it probes the browser); code-scene source files are never deleted (no DELETE route); `enter`/`exit` keep the 0.6 s default (a distance-based default needs a spec decision); art weaknesses listed in part D (arms, short-hair profiles, child body, thumbs-up).
- **Code-scene error positions:** Go bundling errors carry the author's line and column. Errors from a scene's build or run in the worker carry the line and column of the esbuild output, which are only approximate; the app lists them as received.

## Landing

Do these after the last task, in this order.

1. Confirm the full matrix passed on `storymaker-p1` (task I10 step 5). If `main` moved past `561685cde`, rebase first and re-check the diff blocks with `git apply --check`.
2. If the implementation had to deviate from a task, update the contract (`docs/superpowers/specs/2026-11-01-storymaker-contract.md`) and the spec's "Changes after phase-1 drafting" in the same branch.
3. Run a local scratch instance built with the asset pack (the production middleware chain and the embedded asset set were not exercised by the tests) and open the app once.
4. Merge `storymaker-p1` into local `main`. Do not push unless the user asks. Before the merge, check `git status` in the main checkout and leave its uncommitted edits alone: the drafting notes name root `AGENTS.md`, `.gitignore`, `ui/lang/desktop/*.json` and the apps `AGENTS.md`, among others. Root `AGENTS.md` can carry an assume-unchanged flag (`git ls-files -v AGENTS.md` shows a lowercase `h` for it), which hides its edits from `git status`; check the flag before the merge.
