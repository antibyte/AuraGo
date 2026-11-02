# Part C notes: JS player runtime (`ui/js/storymaker`)

Worktree: `C:/Users/Andi/AppData/Local/Temp/claude/C--Users-Andi-Documents-repo-AuraGo/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/sm-wt-c`
(uncommitted, base `0248bc133`). Plan: `part-C.md` (12 tasks, C1-C12, generated from the verified files; every task was replayed in a scratch
tree: its test module fails before the implementation and the whole suite passes after it, pass counts 4, 6, 10, 12, 20, 24, 29, 33, 45, 50, 53).

## Result

- `node scripts/test-storymaker-player.mjs` (`npm run test:storymaker`): **53 passed, 0 failed**, about 8 s.
- Cross-checks against part A (its worktree, read only): my copies of `stories/*.json`, `scenes/s3.js` and `catalog-mini/catalog.json` are **byte-identical** to A's, and the Node esbuild form of the tests reproduces A's golden `s3.bundle.js` **byte for byte** (1365 bytes); module 06 asserts that whenever the golden file is in the tree (it is skipped while A's file has not landed).
- `npm run build:ui` and `npm run check:ui` green (`host.bundle.js` 50 KB / 5 parts, `player.bundle.js` 139 KB / 21 parts; no other bundle changed).
- Browser tests (`AURAGO_RUN_BROWSER_SMOKE=1`, installed Chrome 156.0.8078.74 on this Windows box, through `go test` with `GOTMPDIR=D:\tmp\go-build`):
  `TestStorymakerPlayerGateBrowser`, `TestStorymakerRenderModeBrowser` (png, webp, raw), `TestStorymakerFirewallRenderBrowser` (whole story twice, bit-identical), `TestStorymakerRenderScreenshotBrowser`: all four PASS.
- `go test ./ui -count=1` (browser tests skipped): two failures that are not mine and do not name a storymaker file:
  `TestChatFrontend_IntegrationsDrawerRemainsWired` (marker `function speechLabBrowserURL()`) and `TestFrontend_StaticI18nKeysExistInEnglishBundle`
  (seven keys in `findit-*.js`). Everything else in the package passes, including the external-asset guard.

## Phase-1 gate result (spec "Phase 1 gate (player runtime)")

**PASSED, no fallback needed.** In an opaque-origin `sandbox="allow-scripts"` iframe served under the exact player CSP of the spec (origin = the test server):

| Check | Result |
| --- | --- |
| blob workers whose prelude `importScripts` the **absolute** `player.bundle.js` URL | works (`ready` caps `{workers:true, offscreen:true}`; compile report with 324 frames) |
| `OffscreenCanvas`, `Path2D` in workers | works (thumbnail of scene s1: 160x90, 13 distinct colours, not blank) |
| `FontFace` from an ArrayBuffer + `self.fonts.add` in workers | works (code scene reads its worker's `self.fonts` and sees `Storymaker Default` loaded) |
| `ImageBitmap` transfer worker -> iframe host -> parent | works (`thumb`, `bitmap instanceof ImageBitmap`) |
| `fetch` from a code scene | blocked by `connect-src 'none'`; the server saw **0** requests to the probe URL |
| `importScripts` of a URL outside `/js/storymaker/` from a code scene | blocked by `script-src` |
| the two bundles and the stylesheet load under the CSP | yes (the browser log shows exactly the 3 expected entries: two probe refusals and a favicon 404) |
| the parent cannot read the iframe document | yes (`contentWindow.document` throws) |

The fallback path (storyboard in the host realm, code scenes as a placeholder) is implemented and covered by Node tests (workers unavailable, or a worker that
never says `hello` within `helloDeadlineMs`).

## Render numbers (for part B's transport decision; local loopback ingest, no capture proxy, no CDP interception)

Firewall story, 354 frames, headless Chrome 156 with software raster on this dev box, POST per frame in order, repeat renders **bit-identical** for every transport:

| transport | 640x360 | 1280x720 | 1920x1080 |
| --- | --- | --- | --- |
| png (`canvas.toBlob`) | 47 fps | 35 fps | **21.7 fps** |
| webp (quality 1 = lossless) | | | 7.0 fps |
| raw RGBA (`getImageData`, 8.3 MB/frame) | | | 5.5 fps |

The fourth transport is exact too: a CDP screenshot of the render page (viewport = canvas size, DPR 1) shows **0 of 230400 pixels different** from the posted PNG of the same frame. PNG is the clear winner on speed here; B must still measure on the production path (strict proxy, CDP interception, aurago-test CPU). The page renders one frame, then POSTs it and
awaits the answer before the next (strict order, back-pressure); pipelining render n+1 behind POST n is a possible later gain.

## Decisions (mine, not fixed by the contract)

**Files.** Beyond the brief's list: `schedule.js` (in both bundles), `plan.js`, `beats.js`, `scene.js`, `narration.js`, `determinism.js`, `worker-main.js`
(role dispatch), and the host split `host-core.js` / `host-app.js` / `host-render.js` / `host.js`. The part lists live in `scripts/storymaker-bundle-parts.js`, shared by
`build-ui-bundles.js` and the tests. Tests: `scripts/test-storymaker-player.mjs` is the runner; the checks are modules under `scripts/storymaker-tests/`.

**Fallback loading.** The host bundle does not contain the player code. When workers are unavailable it loads `<base>/js/storymaker/player.bundle.js` with a `<script>` element
(allowed by `script-src <origin>/js/storymaker/`) and instantiates the same `Runtime` that the player worker uses. With no `__SM_ROLE__` the bundle installs nothing in the
document realm (no shadowing of the host's timers).

**Deadlines.** Scene build 2 s (includes the worker's start), frame 250 ms, but the **first 3 frames after a load get 4 s** (JIT warm-up), compile 10 s, assets 20 s, `hello` 5 s. A
deadline terminates the worker and reports code `deadline`. A player-worker overrun kills the engine (all scene workers too); a code-scene frame error kills only that scene's
worker once (placeholder from then on, one `error` message, no flood).

**Timing (scene-local rule).** Scene time 0 = start of the entry transition; narration starts at the transition's end; beats start there too (cursor = T). Cue time =
T + (character position / text length) x block duration (estimate words / 2.3, minimum 1 s, or the measured first block). Auto duration = max(T + narration + 0.4 s, end of the last beat,
T + 1.0 s for scenes without narration). **Scene 0 never has an entry transition**, whatever its `transition` says (the firewall fixture says `fade` on s1; Go's default is `cut`). Scene
boundaries are whole frames: `start_frame[i] = max(start_frame[i-1] + 1, end_frame[i-1] - tf[i])`, `tf = round(duration x fps)`; during an entry transition the previous scene is drawn at its last
frame if it already ended. Explicit `duration` is the total local duration (including the entry transition); a narration longer than it is a `warning invalid_value` at `/scenes/i/duration`.
The narration windows never overlap (the 0.4 s tail); `timeline.narrationEvents` (internal) are what the Node test asserts on; `report.audio_events` is empty in phase 1 and holds
`{kind:'narration', ref:hash, start_ms, gain:1}` per measured block when `audioInfo` carries blocks (phase 2 can use it unchanged); `captions` stays empty.

**Beats.** Defaults: enter and exit 0.6 s (enter walks in from `from` = left/right edge or the `door` spot to `to`; exit walks to the `to` spot and fades during its last 0.2 s), walk = distance at
`cycle.walk.speed` (percent of the design width per second at depth 1, scaled by the mean depth scale along the path, minimum 0.3 s), move 0.8 s, pose 0.3 s (blend), gesture = catalog
duration x `count` (1-5), emote 0.25 s, look 0.3 s, show 0.5 s (0 for `entry: none`), hide 0.3 s, camera 0.8 s (shake 0.5 s), wait `seconds` (default 1). `with: "prev"` starts together with the
previous beat and the cursor becomes the later end; a `cue` beat re-anchors the cursor at its own end; `delay` shifts the start. An actor that has an `enter` beat is invisible until then.
`show` instance ids are `<item-slug>-<n>` (n counts per item per scene, props from `props[]` first) unless `id` is given; `hide target` is an instance id, else the most recently shown instance
of that item. `show` text defaults to the `title` style at the upper third (`y` = 28% of the frame height for the five floor spots; `board` and custom spots use their own y; `{x,y}` is
the centre of the text). Texts are drawn in **screen space** (camera movement does not move them). Camera: framings wide 1.0 / medium 1.35 / close 1.9; `zoom amount` multiplies the current zoom
(default 1.25), `focus` zooms to `amount` (default 1.6) on the target, `pan` moves to the target (or by `amount` percent of the width), `shake` is `amount` design pixels (default 14).

**Positions.** Spots and catalog spot overrides are percent of the **design frame** (as the contract says); `{x, y}` positions are percent of the **visible** safe area of the output aspect (so
they stay on screen in 9:16 and 1:1). Without an override a spot keeps its design position, and is squeezed into the visible slice of a cropped aspect only when it would be outside it (3 percent margin; part R). `y` is always the feet line. Depth scale: linear between
`y_far -> scale_far` and `y_near -> scale_near`, clamped; a spot's optional `depth` (0 far .. 1 near) overrides the y lookup.

**Rig semantics that D must keep (additions to contract 3a, none contradicts it).** Variant keys: `body` -> `params.body`, `skin` -> `params.skin`, `hair` and `hair_back` -> `params.hair`,
`eyes`/`brows` flat keys `<value>/<expression variant>` with `/neutral` fallback (and `<value>/blink` if present: optional blink for 0.12 s every 3.6 s, desynchronised per actor), `mouth` = the
speech shape else the expression mouth then `rest`, `hand_l`/`hand_r` by slot name, `top/bottom/shoes` from `params.outfit[<outfit>]`, others `params[param]`; `"none"` draws nothing; a missing
variant draws nothing. Missing view arrays fall back to `three_quarter`, then `front`. View: walking -> `profile`, `look camera` -> `front`, else `pose.view` or `three_quarter`. `rig.height` is a
fraction of the stage height at depth 1; screen scale = `height x 1080 x depth x actor.scale x body.scale / units_height`. A pose lowers the root by `pose.offset_y` (rig units, positive = down) when it is a number; a `seated` pose without it by `rig.seat_drop`, then
`rig.seated_offset`, then the `lower_leg_l` length, then 0.24 x units_height; the drop blends with the pose blend (part R). `point-left` and
`point-right` swap when the actor faces left, so the name keeps meaning the screen direction (part R). `bones[].length` and `bodies[].bones[].length` are data only (the player uses `rest` overrides). Gesture/cycle keys are smoothed
(ease-in-out per segment); the hands of a gesture key hold, per side, until a later key sets that side again, and the pose hands return at the end of the gesture (part R). Actors get a soft ground shadow. Prop size:
`entry.size.h` is a fraction of the design height at depth 1 (x depth scale x `Prop.scale`), anchored at `entry.anchor` (fractions of the viewbox). Text styles: `size` is a fraction of the **canvas** height,
`box.pad` and `box.radius` are fractions of the canvas height like `size` (a box without numbers falls back to 0.4 em padding and 0.3 em corners; part R); text colours and the box fill resolve from `env.tokens[variant]` first, then the palette (part R, so a dark environment carries its own `text` and `bg2`), `font: "default"` means the story's `style.font`, the catalog font of that name with the nearest weight is used (register one face per
weight in `catalog.fonts`). Environment layers draw in their listed order (back to front); `parallax` p only says that a layer moves by `(1 - p)` x the camera pan (part R: it is not a draw order).

**Code scenes.** Time origin = scene start, like cues and beats. `api.duration` is the total local duration (an explicit story `duration` wins). Positions are spots or `{x,y}` percent; `tween` takes
`opacity x y scale rotation width height` (x and y in percent, width and height in design pixels, shapes only) with an optional `from` map; a text or shape handle is visible from the start (a
`tween` to the current value changes nothing: use `fadeIn` or `from`); `fadeIn` makes the handle invisible before its start. `api.text(str, style, {at})`, `api.shape('rect'|'circle'|'line', {at, w, h, fill,
stroke})`, `api.camera.zoom/pan/shake`. `onFrame(fn)` gets `(t, g)` with a frozen facade `{save restore translate rotate scale alpha fill stroke rect strokeRect circle line path text}` in design
coordinates. Errors from the API throw with the fix in the message; a thrown error carries `line`/`column` of the worker script (the host subtracts the one-line prelude; esbuild reformats the
source, so lines are approximate, not the author's lines).

**No template strings with data, no `innerHTML`, no `eval`** anywhere under `ui/js/storymaker` (a guard test greps for it; E's grep test over `storymaker*.js` can rely on the same rule).

## Message shapes

**Compile report** (`compiled.report` in app mode, `POST /report` body): `{frame_count, fps, width, height, duration_ms, catalog_version, scenes:[{id,start_frame,end_frame,kind}], audio_events:[],
captions:[], checks:[], unverified:[Issue], issues:[Issue]}`. `end_frame` exclusive, `frame_count = max end_frame`, `duration_ms = round(frame_count x 1000 / fps)`. Phase-1 `unverified` = one
`minor layout_unverified` per code scene. Issues the compile emits (union them with Go's validation, dedupe by `path`+`code` if wanted): `narration_missing` (warning, one per scene with narration and
no audio), `not_supported_yet` (minor), `unknown_cue`, `unknown_spot`, `unknown_cast`, `unknown_asset`, `unknown_name`, `missing_field`, `duplicate_id`, `invalid_value` (all warning, tolerant fallbacks), and
`code_error` / `deadline` (**error**) for a code scene that failed to build or has no bundle; `error missing_field` at `/scenes` for a story without scenes. A render job should refuse on any `error` issue.

**Host -> parent** (all carry `sm:1`): `ready{caps:{workers,offscreen}}`, `need_assets{req,keys}`, `compiled{report}`, `time{frame,playing}`, `thumb{req,scene_id,bitmap}`,
`error{code,message,scene_id?,line?,column?}`. Error codes: `code_error`, `deadline`, `readiness`, `bad_message`, `unknown_name` (thumb of an unknown scene). Posts go to the origin of `data-sm-base`.
**Parent -> host**: `load{story,catalog,audio_info?,assets?,fonts?,code_bundles?,format?}`, `assets{req,assets,missing?}`, `seek{frame}`, `play`, `pause`, `thumb{req,scene_id,frame?,width}`, `reset`.
Caps (`StorymakerHost.CAPS`): 400 scenes, 6000 catalog entries, 2000 asset keys (`^(catalog|font|bundle):\S{1,230}$`), 16 fonts of at most 8 MiB (32 MiB total), 400 bundles of at most 256 KiB, thumb width 16-1920,
`format.aspect` in 16:9/9:16/1:1, `fps` 24/25/30, `width` 64-3840. `format.width` is an extension: the preview canvas width (the height follows the aspect); without it the canvas has the export size.
`load.assets` keys are `catalog:<path>` (parsed JSON), fonts come in `load.fonts` as `{name,family,weight,data}` (the `font:<name>/<weight>` key is only used in `need_assets`/`assets`, value = ArrayBuffer). The host
posts `compiled` as soon as it has the report, then asks for the assets it still lacks (`need_assets`, sorted keys), then draws frame 0 and posts `time{frame:0}` (that is the "ready to play" signal). A new `load`
or `reset` terminates all workers. A bad or foreign message is dropped; a schema violation from the parent is answered with `error{code:'bad_message'}`. After an iframe reload the host posts `ready` again: send `load` after every `ready`.
`time` is throttled to every 90 ms while playing and always sent on seek, pause, play, end. Playback is real time (`requestAnimationFrame`, `performance.now`) with frame dropping; at the end `playing:false`.
**Parity recipe for B:** `thumb` with `width` = the export width and a `frame` returns exactly the bitmap that frame n draws on the canvas (the worker renders at the full size when `width` equals the timeline width), so a parent
page can compare decoded RGBA with the render page's PNG.
**Postmessage origin risk:** the host posts to `new URL(data-sm-base).origin`. If AuraGo sits behind a proxy that rewrites the host, ORIGIN in the markup must be the origin the browser used, or the parent never hears the iframe.

**Player worker protocol** (host <-> worker, internal; `reply{req,ok,...}` / `reply{req,ok:false,error:{code,message,line?,column?}}`): `plan{story,catalog,audioInfo,opts}` -> `plan` (per scene: index, kind,
transition_frames, narration window, `cues`; plus palette, fontFamilies, styleFont); `compile{story,catalog,audioInfo,opts:{width,height,codeScenes:{id:{duration|error}}}}` -> `{report,schedule,needs,missing}`;
`assets{assets,fonts}` -> `{missing}` (this is the readiness barrier: fonts are loaded and added before it answers); `render{n,width?,codeFrames,codeErrors}` -> `{bitmap,width,height}`. Scene worker:
`build{opts}` -> `{duration,needs,issues}`, `assets`, `frame{t,width?}` -> `{bitmap}`. Both post `{type:'hello',role}` once they run. `needs` keys: `catalog:<path>` and `font:<name>/<weight>`.

## For part B (render listener and page)

- The render page auto-mounts from `<html data-sm-mode="render" data-sm-base="BASE">`, `<canvas id="sm-stage" width="W" height="H">` and the host script. The page lays itself out through the CSSOM (body margin 0, `documentElement` overflow hidden, canvas `display: block`; without that the page scrolls and a CDP screenshot contains scrollbars). For the screenshot transport set the viewport to W x H at device pixel ratio 1. The CSP of the spec (step 3) works as written
  (browser-tested): `script-src BASE/js/ blob:` covers `host.bundle.js`, `player.bundle.js` (workers and the fallback script element); no `style-src` is needed.
- `boot.json` fields the page requires: `mode:"render"`, `fps`, `width`, `height` (positive numbers), `transport` in `png|webp|raw|screenshot`; used: `frame_limit`, `story_url`, `catalog_url`, `assets_base`
  (default `BASE/catalog/`), `code_bundles:{id:url}`, `fonts:[{name,family,weight,url}]`, `audio_info`. URLs may be absolute or relative to the page. **`fps` and `width/height` from boot.json win over the story's format**; Go must
  send the story's fps and the canvas size of its aspect. Catalog files are fetched as `assets_base + <path from catalog.json>`; fonts only from `boot.fonts` (render mode never requests `catalog.fonts[].file`).
- Flow: GET boot, story, catalog, bundles, fonts -> compile -> `POST /report` (JSON) -> wait for `{start:true, from_frame?}` (`{start:false,error,message}` ends the page with status `refused`) -> frames. Frame `POST /frame`, header `X-SM-Frame: n`,
  `Content-Type` image/png | image/webp (lossless: `toBlob(..., 1)`) | application/octet-stream (RGBA, w*h*4, `willReadFrequently` context). The page awaits each answer: `{ok:true}` continue, `{ok:true,done:true}` stop with phase `done`,
  `{restart:true}` render again from frame 0 (no new report), `{stop:true,error?,message?}` stop with phase `stopped`; a non-2xx without `stop`/`restart` is fatal. The last frame must be answered with `done:true`
  (the page reports `ran past the last frame` via `POST /error` if frame `frame_count` is requested; `frame_limit` caps `frame_count`).
- `POST /error` body `{code, message, scene_id?}`; codes `readiness`, `deadline`, `code_error`, `render_failed`. A code scene that fails to *build* is also an `error` issue in the report, and the page still POSTs the report
  (Go decides; render the placeholder only if you accept the report). A code scene that fails at *frame* time sends `POST /error` and keeps rendering with the placeholder, so Go should treat any `/error` as fatal.
- `window.__smStatus = {phase, frame, error?, message?}` (phases `boot compiling reporting rendering ready(screenshot) done stopped refused error`) is for the driver to poll. `screenshot` transport: after `/report` the page
  sets phase `ready` and exposes `window.__smRenderFrame(n)` (returns a promise that resolves after two animation frames or 150 ms; rejects with an `Error` that has `.code`); Go captures after it resolves.
- Per-frame render deadline 250 ms (first 3 frames 4 s), enforced by the page; Go's 10 s no-frame watchdog is independent. Everything in the page talks only to `BASE/...` (verified: the Node test asserts every request starts with the base).
- Strict proxy mode: the render page issues GET/POST requests to `BASE/` only (no other origin), including `OPTIONS` preflights for `/frame` and `/error`; workers created from blob URLs inherit the page CSP.

## For part E (app)

- Markup is exactly contract section 6; `ui/css/storymaker-player.css` makes the canvas fill the page with `object-fit: contain` on black. Send `load` only after `ready`, post with targetOrigin `'*'`, accept messages only from the iframe's
  `contentWindow`. For a 960 px preview pass `format:{aspect, fps, width:960}`. `load.code_bundles` is the bundled text of every code scene (`GET .../scenes/<id>/bundle`). A scene without a bundle gives `error{code:'code_error', scene_id}` and an error issue in the report.
- The scrubber can be built from `compiled.report.scenes` (`start_frame`, `end_frame`, `kind`) and `report.fps`. Scene thumbnails: `thumb` (default frame = 40% into the scene after its entry transition).
- Issues in the report include a `narration_missing` warning for every scene with narration in phase 1; filter them in the UI if you do not want to show them while renders are silent.

## Contract deviations

1. `compile()` returns `{timeline, report, schedule, needs}` (a superset of `{timeline, report}`); `StorymakerCore.planTiming(story, audioInfo, catalog, opts)` is new (cue times for code scenes before they build).
2. The first scene's entry transition is ignored (duration 0); transitions the player does not implement fall back to fade and report `minor not_supported_yet`.
3. `host.bundle.js` does not contain the player code; the storyboard fallback loads `player.bundle.js` by script element into the host realm.
4. Extra host -> parent error codes `bad_message`, `readiness`, `deadline`; extra `load.format.width`; `thumb` accepts an optional `frame`.
5. `.gitattributes` gets one line (`ui/js/storymaker/*.bundle.js text eol=lf`) so the generated bundles stay byte-identical; `package.json` gets the three scripts of contract section 10; `esbuild` 0.28.2 was already a `dependencies` entry, so no devDependency was added.
6. The tests are split into `scripts/storymaker-tests/*.mjs` plus `scripts/storymaker-test-support.mjs` and `scripts/storymaker-test-context.mjs` behind the single runner `scripts/test-storymaker-player.mjs`.
7. `catalog-mini/rig/human.json` is a stub with 25 slots (the contract lists "slots for every param"; `skin` has an `ears` slot).

## Open issues and follow-ups

- Not implemented in phase 1, by design: captions, lip-sync (the rig takes a `mouth` override), speech bubbles, highlight, chart, diagram, sfx, say, zoom/iris/whip/slide-right/up/down transitions (fall back to fade), `Prop` layers other than back/front, `{x,y}` text styles other than the catalog's.
- `enter`/`exit` use the contract's 0.6 s fixed default, so a long entrance looks fast (the legs cycle at walking speed only for `walk`). A distance-based default is a one-function change in `beats.js` (`kinds.enter.dur`).
- The first frames after a load use the long 4 s deadline; if B's slow CPU needs more than 250 ms per steady-state frame the per-frame deadline can be raised through `env.limits.frameDeadlineMs` (not reachable from markup yet).
- Firefox and Safari were not tried; the gate ran only on Chrome 156. `OffscreenCanvas` 2D in workers needs Safari 16.4+/Firefox 105+.
- `line`/`column` of scene errors refer to the esbuild output, not the author's source (esbuild reformats). Go's bundle errors (syntax) carry the author's positions.
- The `ui/js/storymaker/AGENTS.md` doc and the root `AGENTS.md` child row belong to part E's docs task; I did not write them. GitNexus impact was not needed (no existing function edited: `scripts/build-ui-bundles.js` got one import and two array entries, `package.json` and `.gitattributes` one block each).
- At execution time `node_modules` must resolve `esbuild` for the tests (the worktree `.worktrees/storymaker-p1` sits inside the repo, so Node finds the main checkout's `node_modules`).
