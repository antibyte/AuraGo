# Storymaker desktop app — design

Date: 2026-11-01. Status: approved by the user section by section (authoring = storyboard JSON plus JavaScript code scenes; catalog style = flat vector with rigs; agent in the app and as a chat tool; storyboard editor without a per-object keyframe timeline; export = one JavaScript player everywhere, rendered by server-side headless Chrome into FFmpeg; five phases). Revised after a five-lens review (fidelity, feasibility, weak-model, security, completeness). Updated 2026-11-02 after phase-1 drafting, again for the motion research, after a review of the motion section, again for the huashu-motion research, and after the second review of that research; the changes are listed at the end, in four lists.

Related documents: interface contract `docs/superpowers/specs/2026-11-01-storymaker-contract.md`; phase 1 plan `docs/superpowers/plans/2026-11-01-storymaker-phase1-foundation.md`. Motion sources: the HyperFrames documentation and source (Apache-2.0), and huashu-motion (MIT, v0.3.1), both read on 2026-11-02; see Motion craft.

## Goal

Storymaker is a desktop app for explainer videos and animated videos in general. Videos are authored as code: a declarative storyboard document plus optional JavaScript code scenes. The app previews them live and exports them as a real MP4, optionally handing them to Video Studio for further editing. A prepared catalog of people (rigged characters), props and environments lets a weaker LLM compose a video by referencing catalog IDs instead of drawing assets. Narration uses the active configured TTS by default, and any other configured TTS can be chosen. An asset generator lets the agent create new assets or revise existing ones. When music generation is configured, background music can be generated and mixed under the narration.

Reference concepts the user named: freshstack.ai "Free AI Animation" (stick-figure "What If" cartoons made by an agent skill with Remotion and ElevenLabs; one code rig draws every character; storyboard stills are checked before animating) and github.com/iart-ai/explainer-video-skills (MIT skill prompts; GSAP HTML or Remotion; a verify loop with stills; no catalog, no TTS or music integration).

Storymaker takes from them:

- code-drawn parametric character rigs;
- a still-frame review gate;
- frame-driven determinism (no timers, `Date.now` or `Math.random` in animation);
- one timing source for cuts and captions;
- word-budget sizing of narration, using about 2.3 words per second as a planning estimate only. Real timing comes from measured audio.

It does not take Remotion (its company licence and React toolchain) or any CDN loading.

## Decisions taken with the user

| Topic | Decision |
| --- | --- |
| Authoring | Storyboard (JSON, schema `storymaker/1`) is the default and the weak-model path. JavaScript code scenes handle special animation (charts, diagrams, custom effects) through a small animation API. Both run in the same JavaScript player for preview and export. Rejected: JavaScript only (weak models fail more; Game Maker forensics name free code the costliest failure class), and storyboard only (no escape hatch). |
| Catalog style | Flat vector with rigs. Characters are parameter sets on one shared 2D rig. Parts are vector paths drawn with Path2D, sharp at any size, and the agent can extend them through code even without an image generator. Generated raster images are allowed in addition, as props and backgrounds. Rejected: low-poly 3D (no faces or mouths in the existing 12 humans, and WebGL capture), stick figures, and a second whiteboard style in v1 (out of scope, possible later on the same rig). |
| Agent | In the app (an agent panel that starts an isolated Storymaker job) and as a chat tool `storymaker` (create, status, revise, render, open). |
| Editor | Storyboard editor: scene strip, stage with drag and drop, inspector forms, live preview with scrubbing, and a code view. No per-object keyframe timeline; fine editing happens in Video Studio. |
| Rendering | One deterministic JavaScript player everywhere. The app preview uses it. Export loads the same player in server-side headless Chrome, steps frame by frame and feeds FFmpeg. Rejected: a browser-side WebCodecs export (the window must stay open, which is the Game Maker weakness, and browser support varies), and compiling to a Video Studio project (it cannot express limb animation, lip-sync or code scenes). |
| Chromium | A configured binary (`storymaker.render.browser_path`), then an installed Chrome or Chromium, then a pinned Chromium revision that AuraGo downloads at runtime and verifies with a SHA-256 check before first launch. Not added to the Docker image in v1 (the user accepted the runtime download). |
| Catalog size v1 | About 24 characters, about 16 poses, 10 expressions, 7 mouth shapes, walk, idle, nod and head-shake cycles, about 120 props, about 24 environments, text styles, transitions, chart and diagram blocks, and a small CC0 SFX set. |
| Phases | 1 foundation, 2 sound and export, 3 agent and asset generator, 4 catalog expansion, 5 editor comfort. Each phase gets its own plan, written after the previous phase lands. |
| Motion craft | Adopt the motion recommendations of the 2026-11-02 research, rewritten in Storymaker's terms (section Motion craft). Each rule lands in phase 2 or later, so phase 1 is unchanged. Approved by the user on 2026-11-02. |
| Cue timing | A cue-triggered arrival lands on its cue word: it starts early, so that it is visibly landed on the word. Sound effects sit on the landing. Decided by the user on 2026-11-02; phase 2 (Motion craft, Easing and timing). |
| Point-to-edit comments | A comment on the stage points at an instance and at the beat that last changed it. Comments are kept per comment in the project, with story snapshots before and after each claim. Decided by the user on 2026-11-02; phase 3 (Agent, Point-to-edit comments). |

## Constraints that shape the design

- No external assets (fonts, libraries, analytics). `TestShippedWebAssetsLoadNothingExternal` (`ui/external_assets_test.go:13-81`) guards this. All fonts come from `ui/fonts`, and all libraries come from `ui/js/vendor`. CodeMirror (`ui/js/vendor/codemirror-bundle.esm.js`) and DOMPurify (`ui/js/vendor/purify.min.js`) are already vendored.
- Sixteen desktop locales: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv and zh. No `t('key', 'English')` fallbacks (`ui/i18n_lint_test.go:149-156`). The key `desktop.app_storymaker` must exist in all sixteen (`ui/desktop_i18n_cleanup_test.go:180-204`). Storymaker's own strings live in `ui/lang/storymaker/` and its catalog names in `ui/lang/storymaker_catalog/`, both under the `storymaker` section prefix, so the parts do not share files. The 16-locale used-key test has a hard-coded prefix list that has no new sections (`ui/desktop_i18n_cleanup_test.go:145`), so Storymaker adds the `storymaker` prefix there or gets its own 16-locale test (precedent: `ui/desktop_cyd_creator_designer_i18n_test.go:40`).
- First-party JavaScript under `ui/js/desktop` stays under 1100 lines per file (`ui/desktop_js_line_budget_test.go:10-67`), and no file uses a `-part-` name. The shared player lives outside `ui/js/desktop`, but it follows the same budget by convention.
- Colours come from `--vd-mat-*` and `--ds-color-*` tokens (`TestDesktopAppMaterialsAdoptedByApps`, `ui/desktop_app_materials_test.go:675-703`; `ui/js/desktop/apps/AGENTS.md:2981-3005`). The CSS prefix is `stm-`, never `cc-` (`ui/plugin_safe_classes_test.go:14`). There are no `alert`, `confirm` or `prompt` calls. Dialogs go through the app context (`ui/js/desktop/apps/AGENTS.md:1164`).
- Desktop APIs live under `/api/desktop/` to get desktop scope (`internal/server/desktop_auth.go:14-21`). The read-only gate is respected (`internal/desktop/service_files.go:441-443`).
- Shipped files under `ui/` must sit in an allowlisted directory and have an allowlisted extension, or the asset-set build drops them silently (`assets/web-assets.json:5`, `cmd/assetpack/main.go:69,74-80`). The allowlist includes the `img`, `js` and `fonts` directories and the `.json`, `.svg`, `.png`, `.webp` and `.ogg` extensions. The asset-set ceilings are 20000 files and 2 GiB (`internal/webassets/assets.go:24-25`).
- Video Studio's fixed canvases and frame rates (`internal/videostudio/model.go:257-262`, `internal/videostudio/validate.go:20`) decide the export formats, so a handoff never needs re-scaling.
- No TTS integration in the repository returns word, phoneme or viseme timing. Timing therefore comes from measured audio.
- `TTSSynthesize` truncates input to 500 runes (`internal/tools/tts.go:94,112-117`). Phonon silently falls back to sanoTTS (`internal/tools/phonon.go:26-33`). The global `tts.language` beats the request language for sanoTTS and Phonon (`internal/config/tts.go:20-28`). The TTS listing routes are admin-only (`internal/server/server_routes_config.go:312-314`). The `/tts/` static files need a ticket when auth is disabled (`internal/server/server_routes_ui.go:1236-1243`). The design avoids all five (see Narration).
- Headless Chrome has no AuraGo session. The public static paths are `/css/`, `/fonts/`, `/img/`, `/js/` and `/cfg/` (`internal/server/auth.go:800-805`). The rod v0.116.2 `launcher.New()` downloads Chromium at runtime when no binary is set, from several mirror hosts without a checksum (`go.mod:29`, rod `lib/launcher/browser.go:105,128-130`). The Dockerfile has no Chromium (`Dockerfile:55-68`). Ubuntu 23.10 and later need the AppArmor profile (`documentation/installation.md:271-302`).
- Headless Chrome fires `requestAnimationFrame` at about 32 Hz without vsync (`ui/desktop_video_studio_benchmark_browser_test.go:75-77`). Frames are therefore stepped explicitly and never captured on the wall clock.
- Game Maker lessons: validation must not depend on an open browser tab (`documentation/game-maker-studio.md:529-535`). "Passed" comes only from observed evidence, and missing evidence stays unverified (`internal/gamemaker/game_evidence.go:100`). Per-call payloads stay small, because whole files passed as one tool argument stall the stream (`documentation/game-maker-studio.md:147-154`).

## Approach: one JS player everywhere

Storymaker uses one deterministic JavaScript compiler and player. `compile(story, audioInfo, catalog) → timeline` and `renderFrame(timeline, n, ctx2d)` depend only on their inputs. The output depends only on those inputs and on the frame number `n`.

The app preview runs this code. Export runs the same code in server-side headless Chrome, stepped frame by frame. FFmpeg encodes the frames and mixes the audio. Parity is stated in two parts. Timeline identity holds everywhere, because it is the same code on the same inputs. Pixel identity holds within the same Chromium build: the render page and the parity test compare decoded RGBA at the export canvas size, device pixel ratio 1, after the readiness barrier. The app preview in other browsers (Firefox, Safari, GPU raster) may differ slightly in anti-aliasing and text metrics. The headless render is authoritative, and the layout checks run there. In phase 1 the app player in the sandboxed iframe and the render page gave identical frames in Chrome 156, and repeat renders were bit-identical.

Rejected options, with reasons:

- **Browser WebCodecs export.** The browser window would have to stay open (the Game Maker weakness), and browser support varies. The repository has no WebCodecs or muxer code (`ui/js/vendor` has none), and MediaRecorder runs in real time.
- **Compiling to a Video Studio project.** Video Studio's vocabulary is fixed by its effects and animation registries (`internal/videostudio/effects.go` and `internal/videostudio/anim.go`). It cannot express limb animation, lip-sync or code scenes.

Cost: Chromium runs on the server for export, for agent layout checks and for stills. A new frame-input FFmpeg path is also needed, because none exists today. `internal/videostudio/segment_graph.go:78-91` takes files only.

Division of labour (this avoids a second implementation):

- JavaScript owns compile, the player and the layout checks, in one codebase.
- Go owns schema validation, catalog ID checks, limits, storage, audio (narration, envelope, music and mix), jobs and render orchestration.
- Go never recomputes the timeline. It receives the compile report from the JavaScript side (see Rendering and export, Compile report and job order), and it trusts reports only from its own headless jobs.

### Player runtime

- **Classic scripts.** All player scripts under `ui/js/storymaker/` are classic scripts, with no ES modules at runtime. `scripts/build-ui-bundles.js`, run by `npm run build:ui` (`package.json:34`), builds them into two bundles: `ui/js/storymaker/host.bundle.js` for the document realm and `ui/js/storymaker/player.bundle.js` for the worker realm. Node tests load them through `vm` (`scripts/test-storymaker-player.mjs`; precedent: `scripts/test-pixel-core.mjs`, `scripts/test-ecalc-core.mjs`).
- **Three realms.** They are the same in the app and on the render page.
  1. **Host** (trusted, our code). In the app this is the player iframe's top document; on the render page, the top document. It receives resources, creates the workers, routes frames and owns the deadlines.
  2. **Player worker** (trusted, our code only). It is created from a blob URL whose body calls `importScripts(<base>/js/storymaker/player.bundle.js)`, where `<base>` is the AuraGo origin in the app and the render base (Rendering and export, step 2) on the render page. It runs `compile`, `renderFrame` for storyboard scenes, transitions and composition, captions and the layout checks (`checks.js`). Story JSON is data, so no untrusted code runs in this realm, and its compile report and checks are trustworthy.
  3. **Code-scene worker** (untrusted, one per code scene). It runs exactly one code scene and returns only its declared duration, its frames (ImageBitmap from its own OffscreenCanvas), its errors and, from phase 3, the declarations of its parameters (Typed parameters). It never touches the player worker's realm. Layout checks inside code scenes, including the glyphs of their text (Glyph coverage), are reported as **unverified** (`layout_unverified`); only `code_error`, the deadlines and the duration bounds are verified by the host. From phase 3, `dead_stretch` and `first_frame_empty` are also verified for code scenes, because the host measures the frames it routes; their pixel rules and sampling are in Motion craft (Motion lint).
- **Code-scene delivery.** Authors write an ES module exporting `default function scene(api)`. On save, Go bundles it in process with esbuild (`go.mod:27`, `github.com/evanw/esbuild`; precedent: Game Maker's in-process build, `internal/gamemaker/build.go:14-195`) into one classic IIFE that registers the scene function on a fixed global. `import` statements are rejected, because there are no imports in v1. A static pre-check rejects `eval`, `Function`, `import()`, `globalThis`, `self`, `window`, `document`, `constructor` access, `importScripts`, `fetch`, `XMLHttpRequest` and `WebSocket`. Because esbuild's Go API exposes no AST, the pre-check is a tokenizer over the author's source, and it also requires an `export default`. The pre-check is defence in depth only. The real barriers are the CSP (no `unsafe-eval`, `connect-src` as below) and the worker realm. The bundled text reaches the host as data and becomes the code-scene worker's blob body, which imports `player.bundle.js` for the scene API before it runs the scene.
- **Determinism.** The player and code-scene workers shadow `Date`, `Math.random`, `performance.now`, `crypto.getRandomValues`, `setTimeout`, `setInterval` and `requestAnimationFrame`. Shadowing is for determinism; it is not the security barrier. `api.random(seed)` is the only randomness.
- **History check (phase 3).** `code_history_dependent` (warning) tests whether a scene keeps state between frames. The check job's sampling worker renders the scene's samples in order (Motion lint), and four probe times are added to that sequence: 23, 47, 71 and 93 % of the scene's duration. At each probe, the sampling worker's frame is compared with the same frame rendered by a fresh worker. A mean difference above 0.5 grey levels means the scene keeps state between frames. Export plays through from the start, so only scrubbing and stills would show the difference, which is why the check exists (huashu-motion cli/lib/check.js:266-271; the thresholds are To verify). The fresh worker renders only the four probe frames, so the check adds four frames to the sampling. A scene that runs out of the 60 s check budget stays unverified.
- **Deadlines (watchdog = `worker.terminate()`).** Code-scene compile has 2 s; one frame has 250 ms in the app preview, and the first three frames after a load get 4 s. On the render page the host enforces `frame_deadline_ms` from `boot.json` (5 s), because a render is not real time. Go also kills Chrome when no frame arrives within 10 s after the report is accepted. A frame overrun is reported as `deadline`, and the 10 s watchdog as `code_error`. `storymaker_check` has its own 60 s bound.
- **Fonts and assets.** The host passes font bytes (from `ui/fonts`), part JSON, images, envelopes and code-scene bundles to the workers as transferable ArrayBuffers. Workers build fonts with `new FontFace(name, buffer)` and `self.fonts.add`. A typed `need_assets` message lets the player worker ask for catalog or project assets that a code scene names at compile time. The host only answers with known catalog, project or library asset IDs.
- **Readiness barrier.** Frame 0 is rendered only after all fonts in use are loaded and all images are decoded.
- **Fallback if workers are unavailable in the sandboxed iframe (phase 1 gate).** Storyboard scenes render in the host realm, which runs trusted code only. Code scenes are not previewed live in the app. The app shows a placeholder frame, and the "Render stills" action arrives with the checks in phase 3. The phase 1 gate passed in Chrome 156, so this fallback is not used there.

## Architecture

```
internal/storymaker/            Go package, no server dependency
  model.go        Story document types (schema storymaker/1), strict decode (DisallowUnknownFields), limits
  validate.go     schema + reference validation, issues {severity, scene_id, path, code, message, at_seconds, fix}, max 8 per answer, catalog alternatives
  catalog.go      catalog index read from webassets.Namespace("ui") (ui/img/storymaker/catalog.json), search/describe
  project.go      project folder layout in the desktop workspace, revisions (ETag), library
  codebundle.go   in-process esbuild bundle of code scenes (classic IIFE), import rejection, static pre-check
  narration.go    voice resolution, synthesis plan, cache keys, chunking, measured durations, envelope
  music.go        music generation/loop/trim, file selection
  mix.go          FFmpeg audio graph (narration + music + SFX, ducking, loudness), mixed WAV and stems
  render/         subpackage: job manager (kinds narrate, music, render, agent, check; phase 1 runs render), per-job listener, Chromium provisioning and egress guard, frame ingest → FFmpeg stdin, encoder selection, outputs
  svg.go          SVG subset sanitizer + converter to the vector part format (asset generator)
internal/server/storymaker_*.go HTTP handlers under /api/desktop/storymaker/ (player host at /api/desktop/storymaker/player)
internal/agent/native_tools_storymaker.go, agent_dispatch_storymaker.go  studio tools + chat tool
internal/server/storymaker_agent.go   isolated job runner (prompt profiles, model pin, broker)
ui/js/storymaker/               player and host scripts (classic scripts, outside ui/js/desktop), built into host.bundle.js and player.bundle.js
  compile.js  plan.js  schedule.js  beats.js  scene.js  timeline.js  ease.js  rig.js  parts.js  stage.js  camera.js  text.js
  transitions.js  narration.js  render.js  determinism.js  codescene.js  checks.js  worker-main.js  worker-player.js  worker-scene.js
  host-core.js  host-app.js  host-render.js  host.js (postMessage protocol, workers, deadlines, ingest)
  host.bundle.js (document realm) and player.bundle.js (worker realm), built by scripts/build-ui-bundles.js
ui/js/desktop/apps/storymaker*.js  desktop app UI modules (scene strip, stage, inspector, catalog, agent, code, export)
ui/css/desktop-app-storymaker.css
ui/css/storymaker-player.css     the only stylesheet of the player host page (named in its CSP)
ui/img/storymaker/              generated catalog: catalog.json, rig/, props/, environments/, thumbs/ (SVG); sfx/ comes in phase 2 (characters are parameter sets on the rig)
scripts/build-storymaker-catalog.mjs  code-authored parametric SVG parts → part JSON, manifest, thumbnails (deterministic)
assets/storymaker/              catalog sources (part generators, palettes, README with licence + budgets)
prompts/tools_manuals/storymaker.md  chat tool manual
```

Phase 1 adds `decode.go`, `defaults.go`, `issues.go`, `slug.go` and `memstore.go` to the Go package, and the `render/` subpackage. `narration.go`, `music.go`, `mix.go`, `svg.go`, `captions.js`, `charts.js` and `diagrams.js` come in later phases. From phase 2, `glyph.go` reads the cmaps and applies the fallback faces. From phase 3, `comments.go`, `provenance.go` and `filmprint.go` hold the comment store, the provenance table and the filmprints.

The two HTML pages are written by Go, not shipped as static files, and each gets its own CSP header (see User interface, Player iframe, and Rendering and export, step 3):

- the sandboxed player host for the app iframe, at `GET /api/desktop/storymaker/player`;
- the render page for headless Chrome, served by the per-job loopback listener of the export job.

Their scripts are static files under `ui/js/storymaker/`, which is an allowlisted directory (`assets/web-assets.json:5`), built into `host.bundle.js` and `player.bundle.js`.

Project storage. Each project is a folder in the desktop workspace, `Documents/Storymaker/<project id>/`, like Video Studio's `Documents/Video Studio/<id>/`:

```
story.json        the story document (ETag-protected writes)
project.json      the project record: id, title, aspect, fps, created_at and updated_at; target_seconds, audience and template are set by the agent (phase 3)
scenes/<id>.js    code-scene sources; scenes/<id>.bundle.js is the bundled classic script written on save, and scenes/<id>.errors.json holds the errors of a failed bundle
assets/           project assets (my.*): part JSON for characters, props and backgrounds; validated PNGs
audio/narration/  narration audio <hash>.<ext> and envelopes <hash>.<fps>.env.json
audio/music/      generated or copied music
exports/          export records (phase 1: job ID, output path, frames, encoder, transport; stems and SRT/VTT from phase 2)
```

The personal library lives in `Documents/Storymaker/Library/` with a `library.json` index (Asset generator, promote). From phase 3 the project also holds `comments.jsonl`, the snapshots `comment-snapshots/<comment id>.before.json` and `.after.json` (the 50 most recent pairs), and `provenance.json`. The user's filmprints are in `Documents/Storymaker/filmprints.json`.

Enablement. `storymaker.enabled` gates the app and its API: the `BuiltinApps` entry carries `Requires: ["storymaker"]`, and `desktopCapabilities.HasCapability` (`internal/server/bluetooth_live.go:14-38`) gets a `storymaker` case that returns the config flag (the `retro_games` pattern). The flag defaults to true, so the app is visible and its export shows the capability states. With the flag off, the app is hidden and `/api/desktop/storymaker/*` answers 403 `storymaker_disabled`, except `GET /status`, which answers 200 with `enabled: false`. `storymaker.agent.enabled` additionally gates the agent panel, the studio job runner and the chat tool; with it off, the app works as an editor without the agent.

## Story document (schema storymaker/1)

The example, using the field names of this section:

```json
{ "schema": "storymaker/1", "title": "Was ist eine Firewall?",
  "format": { "aspect": "16:9", "fps": 30 }, "style": { "palette": "sunny", "font": "default", "captions": true },
  "voice": { "provider": "piper", "voice": "de_DE-thorsten-high", "language": "de" },
  "music": { "mode": "generated", "mood": "upbeat-corporate", "volume": 0.25 },
  "cast": [{ "id": "anna", "name": "Anna", "character": "char.office-woman-1", "voice": null }],
  "scenes": [
    { "id": "s1", "environment": "env.office", "transition": "fade",
      "narration": "Anna arbeitet im Homeoffice. {1}Plötzlich klopft ein Unbekannter an.",
      "speaker": "narrator",
      "actors": [{ "cast": "anna", "at": "desk", "pose": "typing" }],
      "beats": [{ "cue": 1, "do": "emote", "who": "anna", "expression": "surprised" },
                { "do": "show", "item": "prop.lock", "at": "right", "entry": "pop" }] },
    { "id": "s2", "kind": "code", "narration": "So prüft eine Firewall jedes Paket." } ] }
```

The voice is the real Piper voice ID `de_DE-thorsten-high` (`internal/config/config_types.go:1947`).

Field rules:

- **Format.** `format.aspect` is one of 16:9 (1920x1080), 9:16 (1080x1920) or 1:1 (1080x1080). `fps` is one of 24, 25 or 30, with a default of 30. These are all Video Studio canvases and rates. 4K and 60 fps are out of scope for v1.
- **Style.** `style.palette` names a catalog palette; the phase 1 starter palettes are `sunny` and `calm`. `style.font` is `default` (the catalog's default font) or the name of another font from a small set of bundled OFL fonts (their licences are to be verified in `ui/fonts`). Phase 1 ships only the default, Geist. `style.captions` is a boolean. From phase 2, `style.rhythm` is `calm`, `standard` or `lively` (Motion craft). It scales only compile defaults. An absent value means `standard`, and Go does not write it. From phase 2, `style.auto_sfx` is a boolean that turns on the auto-SFX of Music and sound. It is off when absent, and Go does not write it. From phase 2, text in a script that the font lacks falls back as set out in Glyph coverage.
- **Voice.** `voice` is the project default. `provider` is a concrete provider ID from the voice provider table (Narration) and is never "active" in the saved file. A new project snapshots the active TTS provider and its configured voice at creation, so a later change of the global TTS does not change old projects. `cast[].voice` is an optional per-character override for dialogue. `language` is always explicit. Phase 1 validates the shape of `voice` and its provider ID (one of the dispatcher IDs of the voice provider table; `active` is refused, an unknown ID gets `unknown_name` with alternatives) and synthesizes nothing.
- **Music.** `music.mode` is one of none, generated or file. `mood` comes from the presets. `prompt` is optional extra text. When the mode is file, `music.file` names a rooted desktop path (no URL or protocol prefix), and `music.track` names a Noisemaker media ID. `volume` is between 0 and 1, default 0.25; from phase 2 it is an offset from the bed's reference level, which is the narration-referenced level when there is narration and −18 LUFS when there is none (Music and sound). Ducking is automatic.
- **Cast.** Each `cast[]` entry has an `id` (a slug), a `character` (a catalog ID, or a project or library asset ID), a `name` (the display name, default = id) and an optional `voice`.
- **Scenes.** Each `scenes[]` entry has:
  - `id`, a stable slug;
  - `kind`, either `storyboard` (the default) or `code`, and from phase 4 `layout` (Layout scenes);
  - `environment`, an ID, and `variant`, either `day` or `night` (default day);
  - `narration`, text with `{n}` cue markers;
  - `silent`, a boolean (default false). A silent scene ignores its narration;
  - `speaker`, either `narrator` (the default) or a cast ID. A cast speaker gets lip-sync and an optional speech bubble;
  - `actors`, each with a cast ID, `at`, `pose`, `facing` (left, right or auto; default auto, which means towards the stage centre), `expression` and `scale` (0.5 to 2; default 1);
  - `props`, each with an instance ID `id`, an `item`, an `at`, a `layer` (back or front; default front) and a `scale` (0.5 to 2; default 1);
  - `beats`, as set out in Beats below;
  - `transition`, applied on entry from the previous scene, one of cut, fade, slide-left, slide-right, slide-up, slide-down, wipe, zoom, iris or whip (`dip` comes in phase 4, see Motion craft);
  - `duration`, either `"auto"` (the default) or a number of seconds;
  - `camera`, `{framing: wide|medium|close, target?}`, the camera's starting framing (default wide);
  - `params`, from phase 3: the typed parameter values of a code scene (Typed parameters);
  - `layout` and `cues`, from phase 4: the fields of a layout scene (Layout scenes).
- **Beats.** The beat kinds are a closed set. Each beat names its kind in `do`. The arguments of each kind are in the Beats table.
- **Timing.** Beats run in order. `"with": "prev"` runs a beat together with the previous one. `cue: n` ties a beat to narration marker `{n}`: in phase 1 the beat starts at the marker; from phase 2 an arrival lands on it (Motion craft, Easing and timing). A beat can also set an optional `delay`, `duration` and `ease`. `ease` comes from a closed list: linear, ease-in, ease-out, ease-in-out, back-out and bounce-out. Defaults per beat kind (for example, enter takes 0.6 s and walk takes a time based on distance at a catalog speed) are server or compile defaults that the model never has to set. In phase 1, enter and exit take a fixed 0.6 s. From phase 2 a hide, and an exit that does not walk, takes 0.35 s, and beats on different targets overlap (Motion craft). A distance-based default is open. Timing defaults are never written into the story; compile.js owns them.
- **Positions.** A position is either an environment spot (`desk`, `door`, `board`, `window`, `left`, `center`, `right`, `far-left`, `far-right`, or a per-environment spot) or `{x, y}` in percent of the safe area. Depth comes from the environment's floor line. Spot coordinates are percent of the 16:9 design frame. In 9:16 and 1:1 a spot without an override keeps its design position unless it falls outside the visible slice, and is then moved inside it with a 3 % margin; `aspects` overrides replace spots that would fall outside. The `{x, y}` form is percent of the visible safe area of the output.
- **Scene duration.** `duration` is `auto` or a number of seconds. The `auto` rule, the transition overlap and the minimum for silent scenes are the scene-local timing rule in Rendering and export (Compile report and job order).
- **Limits.** These are constants, to be adjusted in the plan: 200 scenes; 40 actors and props per scene; 60 beats per scene; 1500 narration characters per scene; 15 minutes of total video; `story.json` up to 1 MiB; a code scene up to 64 KiB. Model-facing writes (`write_svg`, and `storymaker_code` with `write`) aim for 16 KiB or less per call (compact profile 8 KiB). Larger SVGs grow through `revise` steps, and larger code through `replace` steps with a unique old block and `expected_sha256` (pattern: `internal/gamemaker/source_edit.go:99-119`). An over-size argument is answered with an error code that names this guidance. The limits of typed parameters, comments and layouts are stated with their rules (Typed parameters, Point-to-edit comments, Layout scenes).
- **Versioning.** The `schema` string is checked. Migrations in Go run on load (there are none for v1). Unknown fields are rejected with a hint, using strict decoding (pattern: `internal/gamemaker/game_plan.go:281-300`). `storymaker/1` grows additively through phases 2 to 4 (`style.rhythm`, `style.auto_sfx`, the text entries and `dip`, the scene `params` of phase 3, and the `layout` kind with its `layout`, `cues` and chart `data.source` fields of phase 4) without a schema bump. No migration is needed, because an absent value keeps its meaning.
- **Validation output.** Go validation and the JS checks use one issue shape: `{severity, scene_id, path, code, message, at_seconds?, fix}`. `severity` is `error` (it blocks a render), `warning` or `minor`. `path` is a JSON Pointer from the story root, and `fix` is one sentence plus an example patch. At most 8 issues are returned per answer, in scene order (Agent, storymaker_check). Unknown IDs return the exact catalog alternatives (pattern: `internal/gamemaker/design_errors.go:9-86`, `internal/gamemaker/guided_design.go:336-357`).
- **ID form.** Fields that select an asset (`environment`, `character`, `item`, `sound`) take the full prefixed ID: `env.office`, `char.office-woman-1`, `prop.lock`, `sfx.pop`, or a project or library asset such as `my.char.<slug>`, `my.prop.<slug>`, `my.bg.<slug>` or `lib.prop.<slug>`. A background asset (`my.bg.*`, `lib.bg.*`) is valid in `environment`, with the default spots `left`, `center` and `right` and a default floor line. Enum-like fields (`pose`, `expression`, `gesture`, `transition`, `palette`, `ease`, and text `style`) take the bare name. The validator canonicalises a prefixed value (`pose.typing` becomes `typing`) with a note instead of rejecting it, and alternatives are given in the form the field expects. Every catalog entry's `usage_example` shows the field form, for example `{"pose":"typing"}`.

### Beats

Each row lists the arguments of one beat kind. Required arguments have no marker. Optional arguments are listed in the optional column.

| `do` | required | optional | example |
| --- | --- | --- | --- |
| `enter` | `who`; `from` (left, right or door); `to` (spot) | | `{"do":"enter","who":"anna","from":"left","to":"desk"}` |
| `exit` | `who`; `to` | | `{"do":"exit","who":"anna","to":"door"}` |
| `walk` / `move` | `who` or `target`; `to` | | `{"do":"walk","who":"anna","to":"door"}` |
| `pose` | `who`; `pose` | | `{"do":"pose","who":"anna","pose":"typing"}` |
| `gesture` | `who`; `gesture` | `target`; `count` (1 to 5) | `{"do":"gesture","who":"anna","gesture":"point","count":2}` |
| `emote` | `who`; `expression` | | `{"do":"emote","who":"anna","expression":"surprised"}` |
| `look` | `who`; `target` (cast ID, instance ID, spot or camera) | | `{"do":"look","who":"anna","target":"camera"}` |
| `say` | `who`; `style` (speech or thought) | `text` | `{"do":"say","who":"anna","text":"Das ist ein Paket.","style":"speech"}` |
| `show` | one of `item`, `text`, `icon` or `chart`; `at`; `entry` (pop, fade, slide or none; the text entries `words`, `letters`, `ink` and `blur-line` come in phase 4) | `id` (instance ID; `<item-slug>-<n>` when omitted, and echoed back) | `{"do":"show","item":"prop.lock","at":"right","entry":"pop"}` |
| `hide` | `target` (instance ID, or item ID when unique in the scene) | | `{"do":"hide","target":"prop.lock"}` |
| `highlight` | `target`; `style` (pulse, outline or arrow) | | `{"do":"highlight","target":"prop.lock","style":"pulse"}` |
| `camera` | `op` (zoom, pan, focus or shake) | `target`; `amount` | `{"do":"camera","op":"focus","target":"anna"}` |
| `chart` | `chart` (bar, line, pie or counter); `data` (`{labels[], values[]}`); `at` | `data.source` (phase 4, optional) | `{"do":"chart","chart":"bar","data":{"labels":["Mo","Di"],"values":[2,5]},"at":"right"}` |
| `sfx` | `sound` (`sfx.*` ID) | | `{"do":"sfx","sound":"sfx.pop"}` |
| `wait` | `seconds` | | `{"do":"wait","seconds":1}` |

Every beat also accepts the common optional fields `with` (`prev`), `cue` (a narration marker), `delay`, `duration` and `ease`.

### Defaults

Go fills the defaults in this table before saving, only for fields the model omitted, and lists each one it filled in the answer as `defaults_applied: [{path, value}]` (at most 8 entries). The document fields `format` and `style` are filled the same way (16:9, 30 fps, `sunny`, `default`, captions on). Phase 1 does not write a prop's `id` or `scale`; an absent scale means 1.

| Field | Default when omitted |
| --- | --- |
| scene `environment` | the previous scene's environment; `env.plain-stage` for the first scene |
| scene `speaker` | `narrator` |
| scene `transition` | `fade`; `cut` for the first scene |
| scene `duration` | `auto` |
| scene `variant` | `day` |
| scene `camera` | `{framing: wide}` |
| actor `at` | `center` for one actor; for more than one, `left`, `right`, `far-left` and `far-right` in cast order |
| actor `pose` | `stand` |
| actor `facing` | `auto` (towards the stage centre) |
| actor `expression` | `neutral` |
| actor `scale` | 1 |
| prop `layer` | `front` |
| cast `name` | the cast `id` |

A minimal scene is valid: `{ "id": "s3", "narration": "Danach blockiert sie unerwünschte Verbindungen." }`.

### Code scenes

- A code scene is a file at `scenes/<scene id>.js`. The path is always derived from the scene ID, so the scene JSON has no `code` key. The file is an ES module that exports `default function scene(api)`. Go bundles it on save (Player runtime).
- The function runs once to build a timeline, like a GSAP master timeline. Rendering frame n evaluates that timeline. An optional `api.onFrame(fn)` hook draws through a restricted 2D facade, where `fn(t, g)` is a pure function of `t`.
- The API. `start` is seconds or `api.cue(n)`, and `at` is always a position:
  - `api.duration(sec)`;
  - `api.add(catalogId, {at, pose, scale, layer})`, which returns a handle `h` with `h.moveTo(position, {start, dur, ease})`, `h.walkTo(position, {start})`, `h.pose(name, {start})`, `h.emote(expression, {start})`, `h.gesture(name, {start})`, `h.fadeIn({start, dur})`, `h.fadeOut({start, dur})`, `h.scaleTo(s, {start, dur, ease})` and `h.rotateTo(deg, {start, dur, ease})`;
  - `api.text(str, style)`, `api.shape(kind, opts)`, `api.chart(kind, data, opts)` and `api.diagram(nodes, edges, opts)`;
  - `api.tween(target, props, {start, dur, ease})`;
  - `api.camera`, `api.random(seed)` and `api.cue(n)`, where `api.cue(n)` gives the time of narration marker `n`;
  - `api.onFrame(fn)`, the optional draw hook, where `fn(t, g)` draws through the restricted 2D facade `g` as a pure function of `t`.
- Phase 1 implements `duration`, `add`, `text`, `shape` (rect, circle and line), `tween`, `camera`, `random`, `cue` and `onFrame`. `chart` and `diagram` are stubs that report `not_supported_yet` until phase 4, and a `grow` tween property is refused for the same reason. Actors and props in a code scene are ignored with a warning, and its beats with a minor issue, because the scene draws everything itself. From phase 3 the API also has `api.param` (Typed parameters).
- Forbidden: timers, `Date`, `Math.random`, the DOM, the network and `eval`, and the other names that the static pre-check rejects (Player runtime). Shadowing exists for determinism. The CSP and the worker realm are the barriers.
- Bundling errors carry the author's line and column. Run-time errors of a scene carry a position in the bundled script, which esbuild reformats, so they are approximate. Both are returned to the editor and to the agent.

Example scene. The narration has a `{1}` marker, so `api.cue(1)` resolves. The chart's `grow` property runs from 0 to 1.

```js
export default function scene(api) {
  api.duration(6);
  const anna = api.add('char.office-woman-1', { at: 'left', pose: 'stand' });
  const chart = api.chart('bar', { labels: ['Mo', 'Di', 'Mi'], values: [2, 5, 8] }, { at: 'right' });
  anna.gesture('point', { start: api.cue(1) });
  api.tween(chart, { grow: 1 }, { start: api.cue(1), dur: 2, ease: 'ease-out' });
  anna.emote('happy', { start: 3.5 });
}
```

### Typed parameters (phase 3)

- **Declaration.** A code scene declares its parameters with `api.param(name, {type, default, min, max, label})`. The types are `string`, `number`, `color`, `boolean` and `enum`; an enum takes `options`. The declarations come from untrusted code, so only the code-scene worker reads them, at build time, and the build reply carries them to the app.
- **Values.** Values live in the scene's `params` in `story.json`: a flat object of scalars, at most 16 entries, with slug names. A string is at most 500 characters, and a number is finite. Go checks this shape and size on save and refuses anything else with `invalid_value` at the parameter's path. Defaults are never written into `params`; a missing value means the declared default.
- **Checks.** The code-scene worker checks each value against its declaration, with the coercion rules of huashu-motion (cli/lib/project.js:109-131): a number must lie within `min` and `max`, a colour must be a hex value (#rgb, #rgba, #rrggbb or #rrggbbaa), an enum must be one of its options, and a boolean is true or false. A value that fails falls back to the declared default and gives a `warning` `invalid_value` at its path. A declaration without a default takes the zero value of its type, or the first option of an enum.
- **Editing.** The inspector shows one field per declaration, with its label as text. `patch_scene` sets values through `params` (a top-level replace, as for other fields), so the agent can change a title, a number or a colour without `storymaker_code`.

### Layout scenes (phase 4)

- **Kind.** `kind: "layout"` is a third scene kind. It has a `layout` value from a closed set and a `cues` list of `{kind, text?, cue?, data?}`. Trusted player code draws it, as it draws storyboard scenes, so its layout checks are exact and count as verified.
- **Layouts.** Two layouts start the set. `kinetic-type` takes the cue kinds `slam` (a title lands with the pop spring), `wipe` (a page wipe to the next line) and `count` (a 01, 02, 03 track). `data-story` takes the cue kinds `text` and `chart`, where a chart cue carries `data` of `{labels, values}`. An unknown layout is `unknown_name`, with alternatives.
- **Cues.** A cue's `cue` is a narration marker, as for beats, and it lands on its marker under the cue rule (Motion craft). A cue without a marker lands after the cue before it. Cues count against the beat limit of 60 per scene.
- **Other fields.** Actors, props and beats in a layout scene are ignored with the same warning as in a code scene.
- **Chart rules (phase 4, for `data-story` and the chart beat).** Bars start at zero. An axis that does not start at zero carries a note in the subtitle. Ticks fall on 1, 2, 2.5 or 5 times 10 to the power k (huashu-motion runtime/clip/lib/chart.js:3, :13-17). A chart with `data.source` (an optional string) shows it as its source line. Without one, it shows a localised "illustrative" badge, in all sixteen locales.

## Catalog and rig

Rig `rig.human/1`:

- The rig has 13 bones: root, hips, torso, neck, head, upper arm L and R, lower arm L and R, upper leg L and R, and lower leg L and R. The hand and the foot are end parts with their own pivots, not extra bones. Hand poses swap the hand part (open, fist, point, hold).
- Views are front, three-quarter and profile. Left-facing views are mirrored.
- Part format (JSON): each part lists its shapes as `{d (SVG path data), fill, stroke, stroke_width, opacity, gradient}` entries, and gradient stops may name palette tokens, and carries a pivot and a parent bone. Linear and radial gradients are allowed in a small form. Parts are drawn with Path2D on Canvas2D, so they stay sharp at any size.
- A character is a parameter set:
  - body type: slim, average, broad, child or elder;
  - skin tone: 8 options;
  - hairstyle: about 16 options, with a hair colour;
  - eyes, brows, facial hair and glasses;
  - outfit (top, bottom and shoes): business, casual, doctor or nurse, worker hi-vis, chef, student, lab coat, or a police-free generic uniform;
  - accessory: headscarf, cap, helmet or headset.
- Expressions (10): neutral, happy, laugh, sad, angry, surprised, confused, thinking, worried and determined. Each sets the eyes, brows and mouth.
- Mouth shapes (7): rest, A/I, E, O, U, M/B/P and F/V. Approximate lip-sync uses them.
- Poses (about 16): stand, sit, typing, point L and R, wave, arms crossed, hands on hips, shrug, think, present, thumbs up, celebrate, facepalm, phone, hold object, and lean.
- Motion cycles: walk; idle breathing and blink; talk gestures; nod; head shake. Run and jump are optional.
- Props attach to the hands through sockets. The pattern is the humanoid sockets in `internal/gamemaker/asset_packs/aurago-low-poly/manifest.json:34089-34106`. Phase 1 draws props on the stage without attaching them to hands; the `hold` flag is ignored until then.

Content counts for v1:

- Props (about 120), in these categories: office and IT (laptop, phone, monitor, server, router, cloud, database, lock, key, mail and similar); business and money; home and everyday; science and health; transport; nature and weather; symbols and icons (check, cross, warning, idea, question, arrows, gears); and UI cards.
- Environments (about 24), each with parallax layers, named spots, a floor line and a day and night palette: office, meeting room, home office or living room, kitchen, classroom, lab, hospital, server room, shop, street or city, park, factory, warehouse, presenter studio, whiteboard wall, abstract tech grid, plain or gradient stages, space, night city, and countryside.
- Text styles: about 8 titles, lower thirds, captions, speech and thought bubbles, labels with arrows, counters, and check lists.
- Transitions, and chart and diagram blocks.
- A small CC0 SFX set: whoosh, pop, click, ding, success, error and typing. Each `sfx` entry has `impact_ms` (phase 2): the time of its impact in the file, so that a whoosh can start early and peak on the landing.

Manifest: `ui/img/storymaker/catalog.json`, with `schema` (`storymaker-catalog/1`), a `version` hash of the other output files, `stage`, `rig`, `fonts` and `palettes`. Each entry has `{id, kind, name, name_key, tags[], thumb, usage_example}` and kind fields such as `file`, `spots`, `params` or `keys`. IDs are stable, using the prefixes `char.*`, `prop.*`, `env.*`, `pose.*`, `gesture.*`, `expr.*`, `mouth.*`, `cycle.*`, `text.*`, `trans.*`, `palette.*` and `sfx.*` (phase 2). Project and library assets use the prefixes `my.*` and `lib.*`. Names come from `storymaker.catalog.<id>` keys in all 16 locales. The pattern is the 100 `stickers.name.*` keys in `ui/lang/desktop/en.json:229`. The agent works with English IDs and tags.

Production: `scripts/build-storymaker-catalog.mjs` builds the parts from code-authored parametric generators in `assets/storymaker/`. This follows the Blender kits, which are dev-time generators that commit their outputs (`assets/chat-ocean-life/README.md`). The build (`npm run build:storymaker-catalog`) is deterministic and has a `--check` mode (`npm run check:storymaker-catalog`). Thumbnails are deterministic SVG files (`thumbs/<id>.svg`). Cast sheets are rendered for review, and the outputs are committed.

Budget: the catalog stays within 30 MiB and 2000 files. Phase 1 uses 105 files and 2.37 MiB. With phase 1 the whole asset set holds 17986 files and 1.05 GiB (measured on main `561685cde` plus phase 1 with the rules of `assets/web-assets.json`), against ceilings of 20000 files and 2 GiB. About 2000 files of headroom remain, so the phase 4 catalog expansion must fit into them (for example by bundling thumbnails into sprite files) or raise `MaxFiles` in the same change. Licence: the catalog is original work, recorded in `assets/storymaker/README.md`.

Reuse: the shared sticker catalog (`ui/js/desktop/shared/sticker-catalog.js:3`, `window.AuraStickerCatalog`, 100 entries) is offered as an extra "Stickers" tab of raster props. The picker is `window.AuraStickerLibrary.mount` (`ui/js/desktop/shared/sticker-library.js:30`, exported at `:86`).

### Glyph coverage (phase 2)

Storymaker checks that every glyph it draws exists in the font that draws it. Headless Chromium draws a missing glyph with whatever font the host has, so the same story can look different on two hosts, and a host without the font can draw boxes.

- **Check.** Go reads the character map (cmap) of each font in use, Geist or the style font, and checks every on-screen text, caption and speech bubble of the story against it. Geist ships as WOFF2, so how Go reads it is decided in the plan (To verify). The check is static, so it runs in validation without a browser. Code-scene text is not covered by this check: its strings exist only in the code-scene worker, which returns none of them (Player runtime), so the glyphs of a code scene's text stay unverified, reported as a minor `layout_unverified` issue and never as `glyph_missing`.
- **Fallback faces.** A character that Geist lacks is looked up in the Noto faces that the QR module already ships in `ui/fonts/qr/`: `NotoSansCJKsc.otf` (Han and kana, for zh and ja), `NotoSansDevanagari.ttf` (for hi) and `NotoSansArabic.ttf` (for Arabic text in a story). Greek, for el, is checked rather than assumed: if Geist lacks Greek, no face in that set covers it, and the phase 2 plan picks one (To verify). The QR module's notes say these faces supply missing scripts only, which is how Storymaker uses them. The monochrome emoji face is not used in v1, so an emoji is a `glyph_missing` error (To verify).
- **Decision (2026-11-02).** Storymaker references the Noto files read-only through the ui asset namespace, by their fixed basenames, as the QR module does. It ships its own copies only if the QR owner objects. A note goes into `ui/fonts/qr/AGENTS.md` and into `internal/storymaker/AGENTS.md`.
- **Error.** In storyboard, caption and bubble text, a character that no face in the list covers is an error, `glyph_missing`, at the path of the text (for example `/scenes/1/beats/2/text`), with a fix that names the character. It blocks a render.
- **Size.** `NotoSansCJKsc.otf` is 16.4 MB (16,437,364 bytes, `ui/fonts/qr/manifest.json`), above the 8 MiB per-font cap that the host applies to font bytes (contract section 8), and every player worker would hold it. The phase 2 plan decides between a cap for this face and a subset, which would be a new file owned with the QR module, and it measures the memory of one worker with the face (To verify).

## Motion craft

The motion quality comes from two places: compile defaults and catalog data. A model that sets nothing still gets the eases, exit lengths, holds and rhythm, because `compile.js` supplies them, and timing defaults are never written into the story. Catalog data carries the per-asset values, such as the text sizes the lint reads and the cycle periods of idle motion. The agent prompt adds a short house style for the choices a model must make itself: beat order, transitions and entries. The ideas come from the HyperFrames documentation and source (Apache-2.0), and from huashu-motion (MIT), and are rewritten in Storymaker's vocabulary. No HyperFrames code is copied: the CPU transition code is written again, no text is quoted, and GSAP is not vendored. Three thresholds are taken from its source as numbers: the 3 s dead stretch and the 2.6 % text floor (craftAudit.ts:9-28), and the −1 dBTP ceiling (producer audioPadTrim.ts:50). The other timing numbers are starting points taken from HyperFrames' own launch films, tuned on Storymaker's fixture stories. The rhythm factors are first guesses. An explicit value in the story always wins over a default. Every rule keeps frame n a pure function of the story, the catalog and n. Each rule names the phase that delivers it. Numbers taken from huashu-motion are cited where they appear, as `huashu-motion <path>:<lines>`, with the path relative to the root of its clone. The licence rules are in Sources and licences.

### Easing and timing

- **Eases by role (phase 2).** When a story does not set `ease`, compile picks one from the beat's role.
  - Walking (`walk`, and `enter` and `exit`, which run the walk cycle) is linear at the catalog speed, as in phase 1. Linear is a role default only here.
  - Arrivals that do not walk (`move`, and the landing of a `show`) use quart-out, which compile runs as an internal long-tail curve. These are not new values in the closed ease list, and an explicit ease-out keeps the existing curve.
  - Exits that do not walk (`hide`) use ease-in.
  - Camera focus uses expo-out. Camera pan and zoom, and other movement (`pose`, `gesture`, `emote`), use ease-in-out. Zoom is interpolated in log space (phase 2): z = z0 × (z1 ÷ z0)^s, with s the eased progress, so a push feels even at every zoom level (huashu-motion runtime/om.js:144).
  - Pop entries become a closed-form spring (damping about 0.75) in place of the quart-out landing of a show. Opacity never overshoots; only position and scale do.
  - Bounce-out is never a role default.
- **Exits are shorter (phase 2).** A hide, and an exit that does not walk, takes 0.35 s, which is 40 to 50 % shorter than the 0.6 s of a walking enter. Walking enter and exit keep their phase 1 timing of 0.6 s.
- **Overlap (phase 2).** Overlap applies only when the previous beat and this beat both have a target and the targets differ. Then this beat starts at 40 % of the previous beat's scheduled duration (after rhythm). Otherwise it starts when the previous beat ends. A beat's target is its `who`, or else its `target`, and a `show` beat's target is its instance ID. `wait`, `sfx`, `chart` and a `camera` with no `target` have none. Beats on the same target stay in sequence. A `with: "prev"` beat without a cue still runs together with the previous beat. A beat with a cue is placed by the cue rule below, not by overlap. The cue rule and the overlap rule move the timing of the phase 1 fixtures and goldens, which are regenerated in phase 2.
- **Cue timing: lands on the word (phase 2).** A cue-triggered arrival lands on its marker. Its lead is the time from its start to its landing: the ease's 90 % point, or for the pop spring its first crossing of the target. Compile starts the beat at the marker minus its lead, and never before scene time 0. The beat keeps its duration. A beat also never starts before the previous beat on the same target has ended; where that beat is still running, the arrival lands late, and the compile report gives a `cue_late` warning. Each beat kind with a cue follows one of three rules (To verify, early arrival):
  - **Arrival.** `enter`, `walk`, `move`, the landing of a `show` (for `pop`, the first crossing of the spring), a `chart` (its growth reaches the 90 % point), a `camera` focus, and a `camera` zoom with a `target`. The lead and the rule above apply.
  - **Impact.** `sfx`. Its impact, `start + impact_ms`, sits on the marker, so its start is the marker minus `impact_ms`.
  - **Start.** `exit`, `hide`, `pose`, `gesture`, `emote`, `look`, `wait`, a `camera` pan, shake or zoom with no `target`, `highlight`, and `say`, which starts at the marker or when the block before it ends, whichever is later (Narration). The beat starts at the marker, as in phase 1.

  `delay` moves the landing, the impact or the start, so that the marker plus `delay` is that point: an arrival starts at marker + delay − lead, an `sfx` at marker + delay − `impact_ms`, and the others at marker + delay. An `sfx` without a cue that uses `with: "prev"` puts its impact on the landing of the previous beat (on that beat's start, when it has no landing). An `sfx` with neither starts when the previous beat ends. No beat starts before scene time 0. The marker itself is placed in the measured speech (Narration).
- **Holds (phase 2).** A text stays on screen for at least 0.5 s plus 0.3 s per word after it lands. Compile extends the automatic scene length to cover this. A secondary line (a caption or label beside a title) stays on screen at least 1.2 s after it lands. An explicit `duration` is never extended. A text that leaves sooner gets a `line_unread` warning.
- **Text stagger (phase 4).** The `words` and `letters` entries stagger their pieces: letters one frame apart, words 60 to 120 ms apart (90 ms by default).
- **Rhythm (phase 2).** `style.rhythm` is `calm`, `standard` or `lively`. It scales only compile defaults: default beat durations and default text holds (the overlap point follows from the scaled durations). It never scales an explicit `duration`, `delay` or `wait` seconds, a transition duration, a cue time, narration, the 0.4 s tail or the 1.0 s silent minimum. A hold is never shorter than its unscaled length, so lively does not shorten holds, and `line_unread` uses the unscaled hold as its floor. Rhythm does not change eases. The first factors are calm 1.25, standard 1.0 and lively 0.8, and they are open to tuning (To verify).
- **Readability (phase 2).** The catalog text styles keep the phase 1 sizes, as fractions of the short side: title 0.07, caption 0.042 and label 0.034 (phase 1 catalog plan, part D). The `text_too_small` floor of 2.6 % of the short side applies to every style, and the phase 1 sizes clear it at a 1080-pixel short side: the label is about 37 px, and the floor about 28 px. The sizes keep a plain order, title > caption > label. The 3.4 : 1 : 0.4 ratio of huashu-motion (runtime/clip/lib/typo.js:3) belongs to the phase 4 kinetic-type layout only (Layout scenes), where the slam word, the secondary line and the label are set together. There, a label at 0.4 needs a secondary line of about 70 px, and a title of about 239 px, to clear the floor.
- **Shrink to fit (phase 2).** Text that does not fit its box shrinks in steps of 0.94 of its size, down to 0.45 of it, and its lines are then balanced before `text_overflow` fires (huashu-motion runtime/clip/lib/typo.js:33-38). The 2.6 % floor still applies, so a shrink stops at 0.45 or at that floor, whichever is larger. The hard floor of 0.2 is not used.

### Transitions

- **Transitions have a cause (house style in phase 3).** A film uses three to five transition kinds from those that play in its phase (other kinds play as fade until they land), and each one answers something on screen. Before phase 4 that is cut, fade, slide-left and wipe: a cut for an abrupt change, a slide-left for a move on to the next place, a wipe for a change of topic, and a fade inside one world. From phase 4 the house style adds zoom, which continues a camera move, iris, which opens from the camera target, whip, which answers a sharp change of direction, and dip, which marks a break in time.
- **Transition details (phase 4).** `dip` darkens the outgoing scene by 40 % over 3 frames, then cuts to the next scene. `whip` gets a directional smear. `iris` and `zoom` open from the scene's `camera.target`, or from the centre of the frame when none is set. No other origin is read.
- **Worlds do not crossfade (house style in phase 3; lint in phase 2).** A scene that changes environment does not enter with fade. Before phase 4 it uses cut or wipe (slide-left if wanted); from phase 4 it may also use iris, zoom or dip. A change of variant alone (day to night) is not a new world. The motion lint flags a fade between worlds, including a fade that the player plays as a fallback.
- **Default.** The default transition stays `fade` for now. Whether it should follow environment changes is open (To verify).

### Character acting (phase 4)

- The head leads a turn or an emote, and the arms follow 2 to 4 frames later. The left and right arms move with different timing, so they never move in step.
- An emote is a take: anticipation, squash, the new face, then settle. It replaces the 0.25 s snap of phase 1. A mood change goes through a take, not a cut.
- Blink and breath phases are seeded per actor from the project seed, so no two actors blink or breathe in unison.
- Art changes are reviewed on the cast sheet before they ship.

### Text entries (phase 4)

- New `entry` values for text shows: `words`, `letters`, `ink` and `blur-line`. `blur-line` uses a canvas blur. If that blur is not deterministic in a worker, it falls back to alpha only (To verify).
- A landed title stays still. Nothing on a text moves after its entry has finished.

### House style for the agent (phase 3)

The normal prompt profile carries the full house style and asks for three visual directions, then three candidate storylines, before the outline is written (phase 3). The compact profile carries six rules.

- A product or concept explainer opens on a person, reveals each part on the cue that names it, and uses one growing chart or diagram rather than several. The chart needs phase 4.
- A product or concept explainer plans about 19 words per scene, which is about 8 s of speech at 2.3 words per second. The outline answer gives that budget and suggests the scene count it implies, round(`target_seconds` × 2.3 ÷ 19). Every template follows the outline's budgets.
- The banned tells: stacked bullet points as on-screen text; three cards or labels in a row with the same entry; the narrator reading the on-screen text word for word; every element fading in and sliding up; glow, neon and glass-card looks; a title that keeps moving after it has landed.
- Each transition is chosen for a reason, and a scene that changes environment never crossfades.
- **Rhythm plan (phase 3).** Before the scenes are filled, the outline answer carries a rhythm plan: the route (continuous, montage or hybrid, whichever the message needs) and an energy level for each scene (low, medium or high), set from its narration pace and beat count. The plan has no tempo, because narrated films follow the narration and not a music beat. A film whose scenes all share one level is named as flat in the answer.
- **Message shape (phase 3).** The outline answer names the shape of the main message and gives its row of this table, or one row per part when the film has two shapes. Both profiles carry the table. Until phase 4 the layout column reads storyboard.

| Message shape | Template | Layout | Rhythm | Wrong choice |
| --- | --- | --- | --- | --- |
| Claim | concept explainer | storyboard; kinetic-type | standard | a list of cards |
| Workflow | how-to | storyboard | standard | the steps as text only |
| Mechanism | concept explainer | storyboard, with a diagram | calm | a fast run of cuts |
| Evidence | data story | data-story | calm | numbers that flash by |
| List | product explainer | storyboard; kinetic-type count track | standard | five equal cards in a row |
| Story | dialogue story | storyboard with speaker scenes | standard | bullet points on screen |

- **Three directions (phase 3).** Before the outline, the job shows three visual directions: fit, amplify and unexpected. Each is a palette, a rhythm, a transition set, a text style and an environment family, shown as one key-frame still. The directions must differ in language or material, not only in colour, so the job refuses a set in which two directions share their transition set, text style and environment family, because such a pair differs at most in palette and rhythm. The user picks one in the app; the chat tool's `create` takes the first.

### Motion lint (phase 2 static, phase 3 in checks.js)

For storyboard scenes the lint is computed from the trusted schedule, not sampled, so each finding is exact; the code-scene checks are sampled (below the table). Every code is a warning or a minor issue. A finding never blocks a render and is never charged to the agent budget. For the job outcome, lint findings are advisory: they are not model-fixable issues, so they never prevent `ready` or `ready_unverified`, and they stay in the check result without being listed in `open_checks`. HyperFrames' default render does not block on its lint or craft checks either; its agent loop runs them.

| Code | Severity | Rule |
| --- | --- | --- |
| `line_unread` | warning | A text is on screen for less than its hold, from its landing to its `hide` or to the end of the scene. |
| `dead_stretch` | warning | More than 3 s of a scene body with no beat in progress, no narration playing and no camera move. Idle cycles do not count. A text counts as activity from its landing to the end of its hold, so the holds that compile adds never trigger it. A `wait` is not a beat in progress, because it is a pause the model wrote. |
| `first_frame_empty` | warning | The first frame after a scene's entry transition shows no actor, prop, text or chart. The environment alone does not count, and an actor who is off stage at that frame does not count. |
| `text_too_small` | warning | A rendered font size below 2.6 % of the short side of the output frame. |
| `fade_between_worlds` | warning | A scene plays fade, as stored or as the fallback for a transition the player does not play yet, and its environment differs from the previous scene's. The lint reads the transition the player plays, not the stored name. |
| `entry_monotony` | minor | At least three show beats in the film, all with the same entry value. |
| `transition_variety` | minor | More than five transition kinds in one film. |
| `cue_late` | warning | A cue beat lands after its marker, because its target is still busy with an earlier beat (compile, phase 2). |
| `slow_open` | warning | In a film of 6 s or more, the first 5 s show no motion: the strongest 1 s window is below the flat level, or a still stretch of 1.5 s or more starts before 5 s. It reads rendered frames, so it runs in `checks.js` (phase 3). The starting values, in Storymaker's units, are in the pixel rules below, with huashu-motion (cli/lib/qa.js:51, :66-67) as their source (To verify). |

Phase 2 emits the codes of this table, except `slow_open`, as compile warnings in the compile report's `issues`, for storyboard scenes, where the schedule is trusted. Code scenes have no trusted schedule. Phase 3 moves the lint into `checks.js` and adds two code-scene frame checks, `dead_stretch` and `first_frame_empty`. They read the frames the host routes, so they are verified (Player runtime). The storyboard rules do not apply to code scenes, because pixels cannot show beats or narration. The checks use these pixel rules, which are starting values tuned under To verify:

- `dead_stretch` (code scene): a run of consecutive samples that spans more than 3 s, in which the mean absolute difference per colour channel between neighbouring samples stays below 1 % of full scale.
- `first_frame_empty` (code scene): the first frame after the entry transition is a single colour, with every pixel's channels within 1 % of full scale of the frame's mean.
- `slow_open` (film, storyboard and code scenes): the first 5 s are sampled at every second frame, and a step is the mean absolute difference per colour channel between consecutive samples, in percent of full scale. The film is flat when its strongest 1 s window, the largest mean step over any 1 s of those 5 s, is below 1 %. A still step is one below 0.25 %, and a still stretch is 1.5 s of still steps. The sampling is 75 frames at 30 fps, about 7.5 s at 10 fps, inside the 60 s check budget. If the budget is short, `slow_open` is reported as unverified.

The check job samples one frame every 0.5 s of each code scene, in order, in one sampling worker, so that the history check can compare the sampled state with a fresh worker (History check, Player runtime). The four probe times are added to that sequence. Frame n should depend on n alone, and the history check tests that. A 60 s scene needs 121 samples and the probes, about 13 s at the 10 fps measured for the production path. When the 60 s check budget runs out, the scenes not yet sampled stay unverified, and they do not pass. `text_too_small` is already in the layout check list of the Agent section, and phase 3 merges the two into one rule. The codes join the contract's issue table when each phase lands.

**Contrast from pixels (phase 3, `text_contrast`).** A warning, in `checks.js`, on the frames of the check job, for storyboard and code scenes. For each text box, the 3rd and 97th percentile luminance inside the box give a contrast ratio by WCAG's formula. The percentiles are taken over a grid of at most about 20 000 samples. A ratio below 4.5:1 is reported, or below 3:1 for title styles, which count as large text. A ratio under 1.15:1 means the text is masked or not drawn, and it is not reported (huashu-motion cli/lib/check.js:254-255, :334-343).

### Review sheets (phase 3)

The stills call returns these sheets. They are for multimodal routes only.

- **Beat sheet.** One still for each beat landing, taken 0.3 s after the landing. The sheet holds at most 12 tiles: the landings that a layout check flagged come first, then the earliest of the others. The answer names any landings it leaves out.
- **Seam strips.** At the entry and exit cuts of the scene, three stills at −0.2 s, 0 and +0.2 s around each cut, in one image with one row per cut.
- **Phone sheet.** For 9:16 stories, the scene's stills are also drawn at 360 px wide, the size of a phone screen, in one image.
- **Findings.** Each finding is ranked P0 (a fault a viewer would notice, such as an overlap, unreadable text or an empty frame), P1 (a weakness worth one fix) or P2 (a refinement). Each finding carries an exact patch for `patch_scene`. There is no score. The agent makes at most one repair per stills round.
- **Image cap.** A stills call covers one scene and returns at most three images: the beat sheet, the seam strip image and, for 9:16, the phone sheet. Image tokens are the cost on small multimodal models, so the cap applies to every profile.
- **Limits.** A still cannot prove motion, and a frame match shows determinism, not quality. The review sheets therefore sit beside the motion lint and the cold reviewer (Provenance, sameness and a cold reviewer), and do not replace them.

### Provenance, sameness and a cold reviewer (phase 3)

- **Provenance.** Each fact on screen and in the narration gets a row in `provenance.json` in the project: the scene, its path, the text, whether it is a `fact` or an `illustration`, and its source. The source is a file or note that the user gave, or `unsourced`. The job has no web tool, so sources come from the user's material and the project's assets. A fact without a source stays `unsourced`, and the agent does not invent a source. The job's answer lists the unsourced facts, and the table goes to the reviewer below.
- **Sameness.** At each export the film's filmprint (palette, rhythm, transition set, text style, music mood and template) is added to `Documents/Storymaker/filmprints.json`, which keeps the user's last 10 films. A new film that shares its palette and template with one of them, or matches it on four of the six fields, gets a minor `look_repeat` issue in the outline answer. The numbers are starting values (To verify).
- **Cold reviewer (optional).** On a multimodal route in the normal profile, a fresh call reviews the final film. It gets the original brief, the confirmed direction, the outline, the provenance table and the final MP4 (or sampled frames when the route takes no video). It does not get the maker's explanations. It returns findings, each with a timecode, a defect class (unexplained, logic error, unreadable text, rough craft, monotony or loss of control, unmotivated camera, technical fault), a severity (P0, P1 or P2) and its coverage and limits. It must say where attention starts to drift and where the film flattens. The findings are bound to the film's SHA-256. The reviewer gives no score and does not block a render. The agent may use its findings in one repair round, as for stills. The image budget is set in the plan (To verify).

### Not adopted from HyperFrames

- DOM or HTML authoring run as page code. It would run untrusted markup in the page realm, give up the worker barrier and move free code onto the weak-model path.
- GSAP and its bridge (the `__hf` and `__timelines` globals). GSAP is under the GSAP Standard License, which is not an open-source licence, so it is not vendored. Storymaker's compile step is already the clock. Open Design's motion route also copies GSAP into each film folder.
- WebGL shaders and drawElementImage. HyperFrames' drawElement path needs Chrome 151 or later, and its HTML-in-Canvas capture is unflagged only in Chrome 157 and 158 (it needs a flag in 155). The pinned Chromium is 128.
- Runtime loads from a CDN, and npm or npx installs or registry downloads at run time. These break the no-external-assets rule.
- Open Design's house style: breathing glow, neon accents and a three-second slideshow.
- Distributed chunk rendering (Lambda and Cloud Run), HDR, ProRes and VP9 alpha.
- Aesthetic scores, SSIM gates and reference replication. The review gives no score, and copying third-party work carries copyright risk.
- Studio's three-layer history, inspector service and WebMCP. They are too heavy for a storyboard editor. Undo uses the simpler design in User interface.

### Sources and licences (in force from 2026-11-02)

- HyperFrames: its documentation and source are used for ideas only, rewritten in Storymaker's terms. No code is copied, no text is quoted, and GSAP is not vendored.
- huashu-motion (MIT, v0.3.1, HEAD 52467f9, read on 2026-11-02): its ideas and numbers are used, with the citation form given above. Its helper modules under `runtime/clip/lib` (spring, nice ticks, balanced wrap, arc-length resampling) may be ported with the MIT notice in `THIRD_PARTY_LIBRARIES.md`. A file that the NOTICE of `runtime/clip` names as copied from huashu-art-motion keeps that project's attribution too.
- Never used, whatever the licence: `runtime/clip/lib/toon.js`, which carries the author's likeness and is not MIT, and GSAP, which is under the GSAP Standard License and is not open source. Huashu's bundled fonts are SIL OFL 1.1 and are not used. The Noto faces that Storymaker uses are OFL 1.1, and their notices go into `THIRD_PARTY_LIBRARIES.md`.

### Not adopted from huashu-motion

- HTML and GSAP authoring, and the `__timelines` bridge: page-realm code, under GSAP's licence.
- Runtime downloads: Kokoro, whisper.cpp, a Python venv and the sample-pack download.
- ABC composer mode and the 16 music worlds: a new failure class for weak models, and the procedural score renderer is effort L and sounds thin without samples.
- The storytime bean character and `runtime/clip/lib/toon.js`.
- The whiteboard grammar, the Vox collage (fake evidence without real clippings), `om grab` (URL capture outside the egress rules), the signature and share kit, and HTML export with `source.zip`.
- The reviewer with a score threshold, which the shipped skill does not have and Storymaker forbids.
- Launch-film rhythm numbers (cut counts and tempo ranges), and the music-film burst and hook rules. Explainers follow the narration.
- 3D, footage frame tables, ProRes and VP9 alpha, and `tmix` motion blur. Storymaker's blur is its own (Rendering and export).
- Deferred, not in this revision: cuts snapped to the music beat, which would port huashu's beat estimator, and taste notes (huashu's `learn`), which would live in AuraGo memory and not in a new file.

## Narration

- **Voices endpoint.** `GET /api/desktop/storymaker/voices` (scope `desktop:read`, `internal/server/desktop_auth.go:8-12`) returns providers in the form `{id, active, ready, voices[] | free_text, languages, needs_install}`. The active TTS comes first. Secrets are never returned.
- **Readiness.** Readiness is checked per provider. The endpoint copies the config, sets `TTS.Provider` to that provider, and calls `tools.TTSReady(&copy, tools.TTSRouteTool)` (`internal/tools/tts_config.go:161`). It uses the tool route, not the chat route, because `internal/tools/tts_config.go:35-37` forces speech_lab on the chat route. `TTSReady` returns true unconditionally for google, sanotts and phonon (`internal/tools/tts_config.go:166-167`), so for phonon the endpoint also requires `phonon.Ready` (`internal/phonon/files.go:67`) and `phonon.VoicePath` (`internal/phonon/files.go:88`). `needs_install` is true when the voice is not installed. `phonon.Voices` sets `Installed` for each voice (`internal/phonon/files.go:103-110`).
- **Voice providers.** The provider IDs are the dispatcher IDs (`internal/tools/tts.go:367-395`). Each row gives the voice source, the TTSConfig voice field that an override sets, and the chunk limit.

| Provider | Voice source | TTSConfig voice field | Chunk limit |
| --- | --- | --- | --- |
| phonon | `phonon.Voices(root, dataDir)` (`internal/phonon/files.go:103-110`) | `Phonon.VoiceID` | 2000 characters (`internal/phonon/synthesize.go:21`) |
| piper | `tools.PiperListVoices` (`internal/tools/piper_tts.go:194`) | `Piper.Voice` | 400 characters (shared default) |
| supertonic | `tools.SupertonicListStyles` (`internal/tools/supertonic_tts.go:346`) | `Supertonic.Voice` | 400 characters; the request is for WAV |
| xai | `FetchXAIVoices` (`internal/realtimespeech/client.go:454`) | `XAI.VoiceID` | 400 characters |
| openai | a fixed TTS voice list in Storymaker (to verify in the plan; `realtimespeech.Catalog()` is the realtime set) | `OpenAI.VoiceID` | 400 characters |
| elevenlabs, minimax, mistral, resemble, stepfun | free-text voice ID | their `VoiceID` fields (`internal/tools/tts.go:29-88`) | 400 characters |
| sanotts | no list; the voice is derived from the language (`internal/sanotts/sanotts.go:48`) | none | 2000 characters (`internal/sanotts/sanotts.go:69`) |
| google | no list; language only; unofficial endpoint | none | 200 runes (`internal/tools/tts.go:95`); offered only when it is the active provider |
| speech_lab | the single voice of the `/ready` snapshot (`internal/tools/tts_config.go:204-234`) | none | 400 characters |

Gemini is not a TTS provider in the dispatcher and is not offered. The shared default of 400 characters is a conservative sentence-split size for providers without a documented limit.

**Per-language notes (phase 2).** The voice provider table gets a quality note for zh, ja, hi and el on each provider, written from listening tests on the fixture stories. The voices endpoint returns the note with the voice. A provider with no note for a language is still offered for it, marked `not tested` in the narration panel.

- **Voice IDs.** Voice IDs are validated with `^[A-Za-z0-9._-]{1,128}$`, and against the list where one exists. A pre-existing bug is fixed in the same change: the ElevenLabs URL is built with `fmt.Sprintf` without escaping (`internal/tools/tts.go:485`). It now uses `url.PathEscape`, with a test.
- **Synthesis.** Each narration block is split at sentence ends into pieces no longer than the provider's chunk limit in the table. Each piece is synthesised with `tools.TTSSynthesizeInMemoryContext` (`internal/tools/tts_memory.go:20`). The TTSConfig comes from `BuildTTSConfig(cfg, TTSRouteTool, lang)` (`internal/tools/tts_config.go:22`) and is then overridden for each call: the provider, the provider's voice field from the table, `Language` set explicitly, and `NoProviderFallback = true`.
- **Serialisation.** Phonon is serialised by its process slot (`internal/phonon/synthesize.go:18`). sanoTTS has no serialisation today (`internal/sanotts/sanotts.go:68-79`), so the narrate queue limits it to one synthesis at a time. The narrate queue holds at most 128 queued jobs and returns `busy` when it is full.
- **Storage.** The output goes to the project folder at `audio/narration/<sha256(provider|voice|lang|text)>.<ext>`. Speed is a fixed 1.0 in v1 and is not part of the key or the schema. The audio is never stored in `data/tts`, which has retention cleanup, and it is never served through `/tts/`, which has the ticket rule. The Storymaker API serves it from the project folder.
- **Duration and envelope without FFmpeg.** Durations and envelopes come from WAV through `voice.DecodeWAVPCM16Source` (`internal/voice/pcm.go:100`), and from MP3 through `go-mp3` (`go.mod:39`, used in `internal/server/sip_voice.go`). Supertonic is asked for WAV. ffprobe (`internal/videostudio/probe.go:55-82`) is only a fallback. Narration timing therefore works without FFmpeg.
- **Envelope.** A per-frame RMS envelope is computed once per project fps from the decoded audio. It is stored as `<hash>.<fps>.env.json`. The player maps levels to mouth shapes deterministically: rest at silence; low, mid and high levels to E, A and O, with a seeded variation; and M, B and P closures at short dips. The lip-sync is approximate and works with every TTS.
- **Cues and captions.** JavaScript (`compile.js`, `captions.js`) is the single owner of cue placement, caption line splitting and caption timing. From phase 2 the `{n}` markers are placed inside the measured speech of their block (Cue placement in speech). From phase 2 each caption line uses the cue mapping of Cue placement in speech, so a line starts and ends with the words it shows. Before phase 2, and for a block that falls back to `placed: proportion`, the caption line times stay in proportion to character position, over the measured duration of their block. Captions are at most 2 lines of 42 characters, and each lasts at least 1.0 s. In 9:16, the safe zones are the top 12 % and the bottom 18 %. Go writes SRT and VTT from `captions[]` of the compile report. `narration.go` supplies only the measured durations per block and per piece, and the envelope's speech spans (Cue placement in speech).
- **Cue placement in speech (phase 2).** The envelope gives the speech spans of each block: runs of frames above a threshold, which starts at −40 dB (huashu-motion cli/lib/voice.js:643, to be tuned for the RMS envelope). The weights of the block's words are spread over its spans: a Han character or a digit weighs 1; a Latin word weighs its vowel groups (at least 1), with accented vowels counted as vowels; a Greek word weighs its vowel groups in the same way; a kana mora weighs 1; a Devanagari akshara weighs 1; an Arabic word weighs 1; and punctuation weighs 0.1 (huashu-motion cli/lib/voice.js:86-94, extended to these scripts). A marker sits where the weighted share of the words before it is reached inside the spans, so it lands on the word it names. A block that contains a script without a rule, or whose envelope shows no clear speech, is placed as a whole by the phase 1 proportional rule, its cues and its caption lines alike, with `placed: proportion`. Each cue records `placed: span` or `placed: proportion`, and the compile report gives `cue_precision` (Compile report and job order). JavaScript places the cues and their captions; `narration.go` supplies the envelope spans.
- **Measurement (phase 2).** Each narration clip is resampled to the mix rate, including clips at 24 kHz, so that the clips assemble at one rate. The level is measured once, on the assembled narration stem, with the same loudness measurement as the master gain. A short clip, such as a dialogue interjection, has no level of its own and is never measured alone, so it cannot block the export. A stem that cannot be measured at all, for example one that is silent, fails the render with `narration_failed`. huashu-motion writes 24 kHz clips (cli/lib/voice.js:641) and skips their voice measurement (cli/lib/sound.js:498-500), a gap this design does not copy.
- **Dialogue.** A `say` beat with `text` becomes its own narration block in the speaker's voice (`cast[].voice`, or the project voice), with the same cache key scheme. Blocks in a scene play in order and never overlap. The scene narration plays first, then the say blocks in beat order. Each say block starts at its cue, or when the block before it ends, whichever is later. Each block is measured, lip-synced on its speaker and captioned. `auto` covers the last block plus the 0.4 s tail. A `say` without `text` shows a bubble with the scene narration, and the scene speaker must be that cast ID. The dialogue template recommends one scene per speaker turn (`speaker` = cast ID) as the weak-model default, with say beats for short interjections.
- **Failure.** If a provider is not ready, or synthesis fails, the scene gets an issue. The preview then uses an estimated duration (2.3 words per second) and shows a visible "no audio" marker. Export refuses until narration exists, unless the user sets the scene to `silent`.
- **Language resolution.** At project or chat creation, if the snapshotted voice does not support the language, the project uses that provider's default voice for the language (`phonon.DefaultVoice`, `internal/phonon/catalog.go:82-94`). Otherwise it uses the first ready provider that supports the language. If none does, creation fails with `narration_unavailable` and the list of supported languages.
- **Agent voice discovery.** `storymaker_catalog` with `kind=voice` (search and describe) returns the ready voices as `{provider, voice, languages, gender?}`, with the active provider first. Validation of project and cast voices returns alternatives from the same list.
- **Cost.** Storymaker's own caps apply to cloud TTS providers. Local piper, phonon, sanotts, supertonic and speech_lab are exempt. The caps are `storymaker.narration.max_daily_chars` (default 100000) and a per-job cap (default 20000 characters). The budget tracker has no TTS category (`internal/budget/tracker.go:384`).

## Music and sound

- Background music is shown only when `cfg.MusicConfigured()` is true (`internal/config/music_local.go:58`), and only for mode `generated`. Mode `file` is always available. It uses a desktop audio file or a Noisemaker track.
- **Generated music.** Mood presets (upbeat-corporate, calm-ambient, inspiring, playful, tech-pulse, dramatic, lo-fi and acoustic) map to a prompt template, plus an optional user prompt, with instrumental output. Generation goes through `tools.GenerateMusicResult` (`internal/tools/music_generation.go:154`). The track length follows the length rule in Rendering and export (Compile report and job order).
  - ACE-Step receives a duration equal to the track length, between 10 and 600 s (`internal/acestep/types.go:63-70`).
  - MiniMax and Lyria have no duration control, so their track loops with `acrossfade` and is trimmed. Videos longer than 600 s also loop.
  - The daily music quota and the budget tracking are reused. The result is copied into the project folder `audio/music/`.
- **Mix (export).** `mix.go` writes the mixed WAV and the stems.
  - The narration track is assembled from its blocks, at the positions in the compile report's `audio_events`.
  - Music gets a fade in and out, and sidechain ducking keyed by the narration (`sidechaincompress`).
  - **Bed level under narration (phase 2).** When a story has narration, the bed's reference is the narration's measured level minus 12 LU. The narration is measured once, as Measurement sets out, with the same loudness measurement as the master gain, so the two stay consistent. `music.volume` (default 0.25) is an offset from whichever reference applies: the bed sits 20 × log10(volume ÷ 0.25) dB from it, and volume 0 mutes the bed. Without narration the reference is a fixed −18 LUFS, applied before the master gain (huashu-motion cli/lib/sound.js:442-444), so `volume` means the same offset there. The filter order in `mix.go` is fixed: (1) the bed gain sets the bed to its reference, measured before any ducking; (2) sidechain ducking keyed by the narration; (3) under narration, the presence carve, a band around 2.2 kHz (a band-pass at 2200 Hz) cut by 5 dB while speech plays (huashu-motion cli/lib/sound.js:216), gated by the same speech spans; (4) the master linear gain (Loudness, below). The reference is set before ducking and the carve, so their reductions under speech are not measured back into it. The bed level and the carve are applied before the stems are written, so the stems still sum to the mix. Huashu's −14 LUFS target applies only to stories without narration (huashu-motion cli/lib/sound.js:452). Storymaker keeps its −16 LUFS integrated target for every story, and the 12 LU rule only sets the balance inside the mix, so the two do not conflict.
  - SFX come from the beats. Each impact is placed in the compile report by the cue rule of Motion craft (Easing and timing): on the marker, on the landing of the previous beat, or where the sequence puts it. A whoosh starts its `impact_ms` before its impact, so its peak falls on the event.
  - Loudness is a measured linear gain. A two-pass `loudnorm` measurement gives the level to −16 LUFS integrated, and a linear gain applies it, identically to the mix and to every stem (Video Studio handoff). From phase 2 the gain is the lower of the −16 LUFS gain and the gain that holds the true peak at −1 dBTP. A linear gain cannot limit, so a loud mix ends below −16 LUFS rather than above −1 dBTP. The same gain applies to every stem, so the stems still sum to the mix before encoding (To verify). The result is encoded as AAC at 48 kHz (`internal/videostudio/time.go:9`).
  - **Levels and variety (phase 2).** Hit sounds (pop entries, and from phase 4 whips, success and error) sit 3 to 6 dB over the music bed at their landing. Each impact hit also dips the bed by 2 to 6 dB for about 80 ms, then returns over 0.3 s, with one depth per hit kind (huashu-motion cli/lib/sound.js:228-249). A repeated sound picks among seeded takes and never repeats the take that just played, so the CC0 set needs at least two takes for each sound that repeats. Auto-SFX is the story option `style.auto_sfx` (phase 2, off by default). Because it lives in the story, `compile(story, audioInfo, catalog)` keeps its inputs, and the app, check and render jobs compile the same audio events. When on, it adds a pop on the landing of each pop entry and, from phase 4, a whoosh to each whip, starting early by its `impact_ms`, unless an `sfx` beat already sits at that cue.
  - **Procedural effects (phase 2, option).** The CC0 set can be complemented by effects that `mix.go` synthesises from fixed formulas. Their randomness is seeded from the project seed, the sound's ID, the hit's index and its cue time, so each hit repeats exactly. They need no sample files and no licence, and they enter the mix and the stems like any other sound.
  - **Ceiling source.** The −1 dBTP ceiling comes from HyperFrames' producer (audioPadTrim.ts:50), which applies it to its AAC output. Open Design's default path has no ceiling, so Storymaker does not inherit one from it. huashu-motion uses a look-ahead limiter at −1.5 dBFS after its gain instead (huashu-motion cli/lib/sound.js:453). Storymaker keeps the linear gain, because a limiter is not linear and would break the rule that the stems sum to the mix. A limiter stays open (To verify).
- **Preview.** The app preview ducks with WebAudio gain, driven by the stored envelope (pattern: `ui/js/desktop/apps/minigolf-audio-music.js:142-149`).

## Rendering and export

Flow of a render job:

1. **Preconditions.** FFmpeg must be ready (`videostudio.CheckRuntime`, `internal/videostudio/runtime.go:34-89`), a browser must be available, and the story must be valid. Narration must be complete. That condition is vacuous when a story has no narration, so phase 1 renders are silent. The status endpoint reports each capability.
2. **Listener and browser.** Go starts a per-job ephemeral listener and a headless Chrome for the job. Both are closed when the job ends.
   - **Listener.** It binds `127.0.0.1:0`, uses plain HTTP and its own timeouts, and writes no access log of request paths. Every path it serves starts with an unguessable 128-bit prefix from `crypto/rand`, as defence in depth. The **render origin** is `http://127.0.0.1:<port>`, and the **render base** is `<render origin>/<prefix>`. Under the render base it serves only the render page (`<render base>/`), `boot.json` and the job's snapshot (story, compile inputs, part JSON, images, fonts and code-scene bundles, read-only), the two player bundles (`<render base>/js/storymaker/host.bundle.js` and `player.bundle.js`), and the ingest endpoints. Anything else gets 404. The prefix is never logged and never persisted; `jobs.json` stores only its hash. No `authBypassSubtrees` entry on the public server is needed. The listener is not a public route. AuraGo's own loopback URL is HTTPS on TLS installs, which headless Chrome rejects or autocert refuses (`internal/server/server_internal_http.go:49-65`, `internal/server/tls.go:166-169,226-229`). The main server's 30 s read timeout (`internal/server/server_internal_http.go:19`) and access log (`internal/server/server.go:2410,2448-2451`) do not fit a streamed ingest. The render page also need not be reachable from the LAN or the tunnel.
   - **Snapshot.** The job takes its story (after defaults) and code bundles when it is queued, and its catalog files, fonts and player scripts when it starts, so edits during a render do not change it.
   - **Chromium.** Storymaker uses, in this order, a configured binary (`storymaker.render.browser_path`; a configured path that fails is reported, never replaced), an installed Chrome or Chromium that `launcher.LookPath()` finds, and a pinned Chromium revision (1321438, which is Chromium 128.0.6568.0) that AuraGo downloads into the data directory. `status.browser.source` reports the one in use. A download with no bytes for 60 s is abandoned, and the whole setup is bounded by `render.timeout_seconds`. The SHA-256 of that revision for each platform is recorded in code and verified before first launch: download to a temp path, verify, then move into place. Storymaker does not use the rod default mirror race, which races Google storage, npmmirror and playwright mirrors without a checksum (rod `lib/launcher/browser.go:105,128-130`). Production launches never set `NoSandbox`. The render browser has no DevTools port: it is started with a DevTools pipe (`--remote-debugging-pipe`, or `--remote-debugging-io-pipes` on Windows). The Ubuntu 23.10+ AppArmor step stays a documented prerequisite (`documentation/installation.md:271-302`). The `-rod=bin=` flag is implemented by rod (`lib/defaults/defaults.go:113`).
   - **Egress controls.** The enforced controls are these. (1) Chrome runs per job behind `security.StartCaptureProxy` in a new strict mode that dials only the render origin. Today the proxy refuses only private and loopback targets and dials any public host (`internal/security/capture_proxy.go:260-277`, `internal/security/ssrf.go:180-186`). (2) Chrome is launched with `--proxy-bypass-list=<-loopback>`, so loopback also goes through the proxy (precedent: `internal/tools/homepage_annotation_capture.go:124-132`). (3) A hybrid guard. CDP `Fetch` pauses only document navigations and fails those whose origin is not the render origin. Chrome 156 rejects `Fetch.enable` on a dedicated worker's session, so the requests of the page and of its auto-attached workers, subresources included, are blocked natively with `Network.setBlockedURLs` (an allow rule for the render origin first, then the block rules). Extra page targets are closed. Chrome before 126, or the `GuardFull` setting, pauses every request instead, which halves the frame rate. (4) There are no popups, because the sandbox has no `allow-popups`. A regression test has scene code assign `location`, call `window.open` and call `fetch` to a public host, and asserts that all three fail.
3. **Render page.** Chrome opens `<render base>/`. The listener serves the page with this CSP:

   ```
   sandbox allow-scripts; default-src 'none'; script-src <render base>/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src <render base>/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'
   ```

   The page is also served with `Referrer-Policy: no-referrer` and `Cache-Control: no-store`. The host fetches the snapshot from the render base, which `connect-src` allows, and transfers it to the workers.
4. **Frames.** The page compiles, posts the compile report (see Compile report and job order), and renders frames at a fixed step. From phase 2, N player workers render storyboard frames in fixed chunks of C frames (start value 8): chunk j always goes to worker j mod N, so a render repeats exactly. `renderFrame` depends only on the timeline, n and the canvas (Approach), so a worker does not replay earlier frames. Chrome's painting can still depend on what the canvas held before, so each chunk is preceded by the 2 frames before it, which the worker paints but does not post (PRE_ROLL, as in huashu-motion cli/lib/render-om.js:19 and :196-200); the parity test decides whether that pre-roll is needed. The host reorders the frames and posts them in order, so the ingest rule below is unchanged. Its reorder buffer is capped at N × C × 2 frames: a worker waits before it starts a chunk that lies more than that many frames ahead of the next frame the host must post. A parallel parity test (Testing) renders one story with N workers and with one, and compares every frame. N is capped by memory, measured in the phase 2 plan. Frames that need code-scene output keep the single-worker path. Each frame is still a function of n alone. Frames reach FFmpeg through the transport chosen by the phase 1 spike. The candidates are:
   - (a) the page POSTs encoded frames (PNG or lossless WebP), in order, to the ingest endpoint, which writes to FFmpeg stdin (`-f image2pipe`);
   - (b) `Page.captureScreenshot` per frame, over CDP;
   - (c) a canvas blob returned by `Runtime.evaluate`;
   - (d) raw RGBA from `getImageData`, POSTed to the ingest endpoint and read by FFmpeg as `-f rawvideo -pix_fmt rgba -s WxH -r fps -i pipe:0`.

   The decision criteria are at least 10 rendered frames per second at 1080p on an aurago-test-class CPU, bit-identical repeat renders, and bounded memory. Only raw RGBA over CDP stays excluded, because it is 8.3 MB per frame. If no candidate reaches 10 frames per second, the plan takes the fastest deterministic candidate and records its measured rate, which the render timeout then uses (Limits below). Phase 1 measured PNG as the default. It rendered at 10.0 to 10.5 fps at 1080p on a 16-CPU desktop with the production path (up to 15.8 in quieter runs), against 4.5 to 4.9 for lossless WebP, about 4.1 for screenshot and 1.5 to 1.6 for raw RGBA. WebP and screenshot gave the same pixels as PNG, and repeat renders were bit-identical (raw RGBA was not compared). A Debian 12 container with Chromium 156 and FFmpeg 5.1.9 reached 10.2 fps. The aurago-test rate is not measured yet.
   - **Ingest hardening.** Frames are POSTed as `text/plain` to `<render base>/frame?n=<n>`, a CORS-simple request, because Chrome sends a preflight before every frame from the opaque origin. `http.MaxBytesReader` caps each frame (4 MiB at 1080p PNG; the raw RGBA size for transport (d)). The frame index must equal the next expected index. The maximum frame count is computed in Go from `render.max_duration_seconds` × fps, not taken from the page. The PNG IHDR or WebP header (or the raw size) must match the canvas before the bytes reach FFmpeg stdin. All frames are refused once the job leaves the running state.
   - **Motion blur (phase 4, opt-in).** Off unless the export asks for it. When on, fast storyboard movers (whip transitions, enter, exit and shake) draw 4 to 8 alpha-weighted copies along their velocity, computed from the schedule, at offsets fixed by t. Other items are not blurred. The shutter is 180 degrees: copy k of N is drawn at the time i/fps + (k/N − 0.5) × 0.5/fps, clamped to the film, which is the sampling of huashu-motion cli/lib/render-om.js:20 and :174, so copies and a time-sampled average agree (phase 4). It is a render option, not a story field, so the compile report is the same with blur on or off. The app preview never blurs, so the parity test runs with blur off. If the phase 4 plan is full, this moves to phase 5.
5. **Encoding.** FFmpeg takes the frames and, from phase 2, the mixed audio WAV, and produces H.264 through the existing encoder selection (`videostudio.SelectEncoder`, `internal/videostudio/encode.go:117-173`, cached like `internal/server/video_studio_encoder.go:14-42`, with software fallback). The audio is AAC, and the container is an MP4 with `+faststart`. Phase 1 adds a silent AAC track from `anullsrc` (48 kHz stereo), so every phase has the same stream layout. Progress comes from the frame count (phase 1 does not use `-progress pipe:1`; pattern: `internal/videostudio/render.go:579-627,677-700`). **Frame-count gate (phase 2).** When FFmpeg finishes, ffprobe counts the video packets, and the count must equal the frame count. Otherwise the job fails with `render_failed`, and no export record or library copy is written.
   - **Colour.** The frame-input graph converts colour explicitly: `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p`, followed by `Encoder.filterTail()` before the encoder args, as `internal/videostudio/segment_graph.go:154-161` does. The encoder args only tag BT.709 (`internal/videostudio/encode.go:62-93`), and FFmpeg before 7.1 would otherwise convert with BT.601.
   - **Inputs.** Every new FFmpeg input (mix, encode, frame pipe, envelope) uses `-protocol_whitelist file,pipe` and an explicit `-format_whitelist`. The format list is the one in `internal/videostudio/probe.go:16-17`, plus `image2pipe`, `png_pipe`, `webp_pipe` and `rawvideo` as needed.
   - **Hardware fallback.** If the hardware encoder fails in the streamed path, Go sends the host a `restart` message, and the job restarts from frame 0 with libx264, once (pattern: the one-shot fallback at `internal/videostudio/render.go:94-109`).
6. **Outputs.**
   - **Phase 1:** `Documents/Storymaker/<project id>/exports/<slug>-<first 8 of the job id>.mp4`, and `exports/<job id>.json` beside it. Both are written create-only through `WriteFileStreamConditional`.
   - **Phase 2:** `Videos/Storymaker/<slug>.mp4`, the library copy. The slug comes from `Slugify` (`internal/storymaker/slug.go`): ASCII letters, digits and dashes, umlauts and accents transliterated, Windows reserved names refused, at most 80 characters, and the project ID when nothing is left. On a collision, `-2`, `-3` and so on are appended. Example: "Was ist eine Firewall?" becomes `Was-ist-eine-Firewall.mp4`.
   - **Why `Videos/`.** The library copy goes under `Videos/` so that the MP4 is discoverable where camera clips are imported. The camera import accepts only `Videos/` paths (`ui/js/desktop/apps/video-studio-projects.js:432-447`). The Video Studio handoff imports by `source_path` and does not enforce `Videos/` (`internal/server/video_studio_handlers.go:664-684`), so the plan must not rely on that rule. Phase 1 does not write there, because `Videos` is a read-only media mount for desktop writes (`internal/desktop/service.go:663`, refused by `service_file_stream.go:29-33`).
   - An optional SRT or VTT file, written from `captions[]` (Compile report and job order).
   - Stems `narration.wav`, `music.wav` and `sfx.wav`. They are always produced when "Open in Video Studio" is chosen, and optional otherwise (Video Studio handoff).
   - Large writes go through `WriteFileStreamConditional` (`internal/desktop/service_file_stream.go:19-31`), not the 50 MB upload path, whose default limit is set in `internal/desktop/service.go:172-173`.
7. **Cancel and persistence.** Cancel kills Chrome and FFmpeg, and closes the listener. Jobs persist in `<DataDir>/storymaker/jobs.json`, and a job that is running at restart is marked `interrupted` (pattern: `internal/server/video_studio_service.go:367-442`). Progress is streamed over SSE (pattern: the Game Maker broker and SSE, `internal/server/game_maker_handlers.go:358-396`).

Limits: the render timeout is max(`render.timeout_seconds`, 3 × frame_count ÷ the fps of the transport), where `render.timeout_seconds` defaults to 3600 s. The render code uses 6 fps for PNG, 2.5 for WebP and screenshot, and 1 for raw RGBA. These are all below the 10 fps measured on a desktop, so a 15-minute video (27 000 frames) gets 13 500 s. The maximum video is 15 minutes (`render.max_duration_seconds`, default 900). One render runs at a time, through a queue of its own that holds at most 16 queued jobs and returns `busy` when it is full. That queue is separate from the Video Studio and Game Maker queues.

Missing Chromium or FFmpeg: the editor and the preview still work. Export is possible when FFmpeg is ready and the browser is ready or downloadable (`status.browser.can_download`). The first export then downloads the pinned Chromium (135 to 267 MB by platform). Otherwise export and the agent's layout checks show a clear capability message. Agent checks report the layout as unverified. Narration timing does not need FFmpeg (see Narration).

### Compile report and job order

The player worker compiles the story and produces a compile report. The host sends the report to Go. Only render and check jobs send one. Go trusts reports only from its own headless jobs, never from the app iframe. In the app, the player iframe sends its report to the parent in a `compiled` message, only for display (scrubber, scene durations, local warnings); the parent never forwards it to Go.

```
{frame_count, fps, width, height, duration_ms, catalog_version,
 scenes: [{id, kind, start_frame, end_frame}],
 audio_events: [{kind: narration|dialogue|sfx, ref: narration hash or sfx id, start_ms, gain}],
 captions: [{start_ms, end_ms, lines[]}],
 checks: [check result], unverified: [Issue], issues: [Issue]}
```

Go validates the report: known narration hashes and SFX IDs, bounds, monotonic times, `frame_count` ≤ `render.max_duration_seconds` × fps, and code-scene durations clamped. Go then uses the report for the mix, the stems, SRT and VTT, the music length and the duration limit.

From phase 2 the report also carries `cue_precision` (the share of cues placed by a span) and, in each `scenes[]` entry, `cues: [{marker, at_ms, placed}]`, where `placed` is `span` or `proportion` (Narration, Cue placement in speech). The interface contract (`docs/superpowers/specs/2026-11-01-storymaker-contract.md`) changes in phase 2 to match its report shape and its caption and cue timing rule.

- **Caption and cue ownership.** JavaScript (`captions.js`, `compile.js`) is the single owner of cue placement, caption line splitting and caption timing. `narration.go` supplies only the measured durations per block and per piece, and the envelope's speech spans (Cue placement in speech). Go writes SRT and VTT from `captions[]`.
- **Scene-local timing rule.** Scene time 0 is the start of the scene's entry transition. The scene's narration starts when that transition ends (offset = transition duration, 0 for cut). Cue and beat times are scene-local, from the same origin. From phase 2 a cue-triggered arrival may start before its marker, down to scene time 0 (Motion craft, Easing and timing). The auto length does not change. `auto` duration = entry transition + narration blocks + 0.4 s tail, or the end of the last beat (from phase 2, also the end of the last text hold, see Motion craft) if that is later. Silent scenes and scenes without narration have a 1.0 s minimum. Transitions overlap scenes: a scene starts its entry transition's duration (0.5 s by default) before the previous scene ends, so its narration begins after the previous scene's narration and tail. A Node compile test asserts that no two narration or dialogue events overlap. The first scene has no entry transition, so its `transition` value is ignored. An explicit `duration` is the whole scene length, entry transition included. Until audio exists, narration timing is estimated from the words (2.3 per second, minimum 1 s), and a scene with narration gets a `narration_missing` warning.
- **Render job order.** The job starts the listener and Chrome, and the page compiles. The page posts the compile report, and Go validates it. `mix.go` writes the mixed WAV and the stems. FFmpeg starts with the WAV and the frame input. The page steps frames, the frames go to the ingest endpoint, FFmpeg finishes, and the outputs are written.
- **Music length.** Generated music uses `duration_ms` from the latest check-job report. If none exists, a check job runs first. Later length changes are absorbed by a loop (`acrossfade`) or by a trim with fade-out. The music is not regenerated.
- **Phase 1 reports.** A phase 1 render reports the frame count, the scene ranges and the issues. `audio_events` and `captions` are empty, `checks` stays empty until phase 3, and each code scene is listed in `unverified` as a `layout_unverified` issue. `end_frame` is exclusive.

## Video Studio handoff

"Open in Video Studio" runs as browser JavaScript in the Storymaker app (`ui/js/desktop/apps/storymaker-export.js`), with the user's session and the public Video Studio API:

1. It checks `GET /api/desktop/video-studio/status` (enabled and `ffmpeg_ready`) before it offers the button.
2. It creates a project with `POST /api/desktop/video-studio/projects` (`internal/server/video_studio_handlers.go:362-421`), using a canvas and frame rate that match the story.
3. It imports the MP4 and each stem with a `POST` of media by `source_path`, with an `Idempotency-Key` (`internal/server/video_studio_handlers.go:655-690`).
4. It polls the probe jobs until they finish.
5. It gets the project again. Each probe rewrites `project.json` and its ETag through `mergeProjectAsset` (`internal/server/video_studio_worker.go:897-965`), so the fresh ETag and the server-owned asset metadata come from this GET.
6. It builds the tracks (below). The track builder is a JavaScript function in `storymaker-export.js`, covered by a Node test.
7. It PUTs the project with `If-Match`, using strict JSON (`internal/server/video_studio_handlers.go:464-549`).
8. It opens Video Studio on that project.

- **Stems.** Stems are always produced when "Open in Video Studio" is chosen, and optional otherwise: narration (including dialogue), music (with the same fades and ducking already applied as in the mix), and sfx. Loudness normalisation is a measured linear gain (a two-pass `loudnorm` measurement, then a linear gain), applied identically to the mix and to every stem, so the stems sum to the export mix.
- **Tracks.** The video track holds the MP4 clip with `AudioDetached`, so its audio is removed from the render (`internal/videostudio/plan.go:251`; the semantics are in `internal/videostudio/model.go:97-131`). Audio track 1 holds the narration. Audio track 2 holds the music. Audio track 3 holds the sfx.
- **Project launch.** Video Studio needs one new piece for this. It gets a `projectId` launch parameter, used as `openApp('video-studio', {projectId})`. Today `initialize` loads `projects[0]` (`ui/js/desktop/apps/video-studio.js:348-381`), and only camera imports are accepted (`ui/js/desktop/apps/video-studio-projects.js:435-447`). The change is small and contained, in `video-studio.js` and `window-shell-runtime.js`.

## Agent

The agent job is isolated, like Game Maker's but leaner:

- It starts from the app panel or from the chat tool. There is one active agent job per project, a global limit of concurrently running agent jobs (`storymaker.agent.max_concurrent_jobs`, default 1), and an own queue of 8 queued jobs. A full queue returns `busy`.
- **Model.** The model is the configured main route. An optional `storymaker.agent.provider` and `storymaker.agent.model` can override it. The provider is pinned for the job, with no silent switch (pattern: `internal/server/game_maker_service.go:609-631`). Small-model detection and the compact profile are chosen by context and output size (pattern: `internal/server/game_maker_model_limits.go:14-43`).
- **Phases.**
  1. Outline: choose a template (product explainer, how-to, concept explainer, dialogue story, data story or onboarding). Build the arc hook, problem, solution, how, benefit, close. Write the narration per scene with `storymaker_story` `outline`, which returns per-scene word budgets. The dialogue story template uses one scene per speaker turn (Narration, Dialogue). From phase 3 the direction step (House style) comes first, and the outline answer carries the rhythm plan and the message-shape row.
  2. Casting: choose characters and environments through catalog search and describe. Choose voices through `storymaker_catalog` with `kind=voice`.
  3. Scene filling: one scene per call.
  4. Server check: validation, narration synthesis, and compile and layout checks in headless Chrome.
  5. Still review: on multimodal routes, stills for self-critique (Stills below; pattern: visual gating, `internal/server/game_maker_visual.go:20-26`).
  6. Fixes. Export happens only when the user asks for it.
- **Budget.** The job timeout defaults to 1800 s. Per scene there are 3 charged corrections, where an issue-count drop is free and new scenes and first writes are never charged. There is also a per-job cap of 6 check rounds. Dry runs are uncharged but capped at 2 per scene and 20 per job. Charged corrections do not charge progress (pattern: `internal/gamemaker/plan_guard.go:13-61`). Game Maker's 3/5 constants (`internal/gamemaker/plan_guard.go:13-17`) are per design document, and they are not copied unchanged.
- An invalid `put_scene` or `patch_scene` is kept as a per-scene **draft**, which is not saved to `story.json`. The answer carries the issues and `draft: true`. `patch_scene` merges into the draft until the draft validates, and `remove_scene` discards a draft.
- **Events.** Progress is persisted as SSE events, with tool names and token counts. Model prose is released only with a `ready` or `ready_unverified` outcome (pattern: `internal/server/game_maker_service.go:860-962`).
- **Prompt profiles.** Profiles are versioned and never edited in place (pattern: `internal/server/game_maker_prompt_profile.go:83-153`). User text is wrapped as external data (pattern: `internal/server/game_maker_prompt_profile.go:18`).
- **Compact profile per phase.** The compact profile shows each phase only the tools and operations it needs. Outline is `storymaker_story` {outline, get}. Cast is `storymaker_catalog` + `storymaker_story` {set_meta, get} + `storymaker_asset` {create_character}. Fill is `storymaker_catalog` + `storymaker_story` {put_scene, patch_scene, dry_run, get}. Check and fix is `storymaker_check` + `storymaker_story` {patch_scene, get}. The compact profile hides the `storymaker_code` tool and the `write_svg` operation of `storymaker_asset` unless the user asked for them. The `stills` operation is removed on non-multimodal routes. The `generate_image` operation and image `revise` are removed when image generation is not configured or the daily limit is used up. Each turn gets a context packet, wrapped in `external_data`, with the phase, `target_seconds`, the word budget, the cast, the current scene's outline narration, that environment's spots, the open issues (at most 8) and the remaining budget.
- **Stills (phase 3).** One scene per stills call, which returns at most three review images: a beat sheet, the seam strip image and, for 9:16, a 360 px phone sheet (Review sheets). Findings come back ranked P0, P1 and P2, each with an exact patch, and the agent makes at most one repair per round. Earlier stills are replaced by a text placeholder in the history. The compact profile reviews only the flagged scenes, or the first and last scene when nothing is flagged (pattern: `internal/server/game_maker_service.go:780-787`).
- **Call repair.** `canonicalizeStorymakerOperationCall` sends an operation name that was called as a tool to its owning `storymaker_` tool. It wraps flat scene fields into `data`. It applies only inside a bound Storymaker job, and it never widens scope (pattern: `internal/agent/canonical_action.go:162-170`). A recovery nudge names the current phase's tools and gives one call example (pattern: `internal/server/game_maker_recovery_nudge.go:22-72`).
- **Catalog miss.** When the catalog has no match, the agent tries these in order: (1) the nearest catalog alternative; (2) a text, callout or icon show beat; (3) `create_character` for people; (4) `write_svg`; (5) `generate_image`, only when it is configured. The phase 1 starter props cover office and IT and symbols (lock, key, shield, laptop, phone, cloud, check, cross, warning, idea), so the phase 3 acceptance can run on the starter catalog.

### Job outcome

The job ends when the model replies without a tool call after at least one validate (the server then runs a final validate itself), or when the timeout or the budget runs out. The outcomes are:

- `ready`: no model-fixable issues (motion lint findings are advisory, see Motion lint), and the layout is verified. A story with a code scene can reach at best `ready_unverified`, because the layout checks inside code scenes stay unverified (only the two frame checks of phase 3 are verified; Player runtime).
- `ready_unverified`: no model-fixable issues (motion lint findings are advisory, see Motion lint). `open_checks` names the unverified or environment items.
- `incomplete`: a timeout or budget stop. Valid scenes are kept, drafts are dropped, and `open_checks` lists the failing scenes.
- `failed`: with an error code (Error handling).

Model prose is released for `ready` and `ready_unverified` (pattern: `internal/gamemaker/job_outcome.go:158-181`). Agent-created length is bounded to 15 to 180 s by default (config `storymaker.agent.max_length_seconds`).

### Point-to-edit comments (phase 3)

- **Stage tool.** A comment tool on the stage pins a note to a point of the frame at a time t. The host asks the trusted player worker with `hit_test` (t, x, y, in percent of the frame). The answer is the instance ID under the point and the JSON Pointer of the beat that last changed that instance, such as `/scenes/1/beats/3`; the worker keeps that pointer for each instance during compile. A point on a code scene, or on the backdrop, resolves to the scene only (`/scenes/<i>`), because the drawing of a code scene is not in the story.
- **Store.** Comments are kept in `comments.jsonl`, which is append-only: one JSON event per line (created, claimed, done, released, stale or reopened), and a comment's state is its last event. A torn last line is skipped on read. Each comment holds its time, scene ID, instance, pointer, creation time and text. The pointer is stored with its scene ID, its beat index and the story rev at creation, because an index moves when beats or scenes move. The text is untrusted and at most 500 characters (starting value). States move from pending to claimed when a job takes the comment, and to done when the job ends. A claimed comment goes back to pending as `released` when its job fails. A comment moves to reopened when the user reopens it or reverts it. At claim time, if the story rev has moved on, the job re-resolves the pointer with a fresh `hit_test` at the stored time and point; if the instance is no longer under that point, or its scene is gone, the comment becomes `stale`. A stale comment is shown to the user, and no job patches it.
- **Revise job.** A revise job takes one pending or reopened comment, never a stale one. Its context packet carries the comment text, the pointer, the scene JSON and, on multimodal routes, the frame at the comment's time and a 2× crop of that frame around the click. The job makes the smallest change, with `patch_scene` on that scene.
- **Snapshots and revert.** Before the claim, the story is saved as `comment-snapshots/<comment id>.before.json`; when the claimed job ends, whatever its outcome, it is saved as `<comment id>.after.json`, so that the partial edits of a failed job can be reverted. The project keeps the 50 most recent pairs (starting value). A revert restores the before snapshot as an ETag-checked save, but only while the story still matches the after snapshot. Otherwise the revert is refused and the user's later edits stay.
- **Undo.** The manual undo is session-only, as in User interface.

### Studio tools

The studio tools are job-scoped, with the prefix `storymaker_`. Model-facing schemas never carry IDs that the server binds.

- **Tool wiring.** Add `storymaker_` to the private-execution prefix test (`internal/agent/action_policy.go:314`) and to the studio flag (`internal/agent/tool_scope.go:108`). Add `storymaker_*` cases to the dispatch switch (`internal/agent/agent_dispatch_exec.go:226-234`). Add the chat tool case next to retro_games (`internal/agent/dispatch_platform.go:312-313`). The schemas go in `internal/agent/native_tools_storymaker.go`, modelled on `internal/agent/native_tools_game_maker.go:16`. The job's DispatchContext sets `ToolScopeRestricted = true` and `AllowedTools` to exactly the `storymaker_*` tools of the current phase (precedent: `internal/server/game_maker_implementation_stream.go:80`). A test asserts that a general tool name is refused.
- `storymaker_catalog` `{operation: search | describe, query, kind, id}`. The default is 6 results and the cap is 12. It searches the built-in catalog first, then the project (`my.*`) and library (`lib.*`) assets, and voices with `kind=voice`, which return ready voices as `{provider, voice, languages, gender?}` with the active provider first. It returns exact IDs, a usage example and alternatives (pattern: `internal/gamemaker/asset_usage.go:152-299`).
- `storymaker_story` `{operation, scene_id, data, expected_rev}`. The operations take these arguments and give these answers:

| Operation | Arguments | Answer |
| --- | --- | --- |
| `get` | optional `scene_id` | without `scene_id`, a summary: rev, meta, cast, and per scene the id, environment, first 80 narration characters, beat count and open issue codes; with `scene_id`, that scene |
| `outline` | `{template, target_seconds?, scenes:[{id, narration}]}`; only while no scene is filled (otherwise use `put_scene`) | per-scene word budgets, computed as `target_seconds` × 2.3 ÷ scenes; for the product and concept explainers, 19 words per scene and the suggested scene count (House style); from phase 3 also the message-shape row and the rhythm plan |
| `set_meta` | `data`: a subset of `{title, style, music, voice, cast}`; `cast` is replaced whole | the new rev |
| `put_scene` | `{scene_id, data}` | the new rev; or, when invalid, the issues with `draft: true` (kept as a draft, not saved) |
| `patch_scene` | `{scene_id, data}`: top-level fields replace, arrays replace whole, `null` removes | the new rev; or the merged draft with its issues |
| `remove_scene` | `{scene_id}` | the new rev; a draft is discarded |
| `move_scene` | `{scene_id, after}`; `after` is a scene ID, or `""` for first | the new rev |
| `dry_run` | `{op, scene_id?, data}` | the issues and `defaults_applied`; Go validation without saving and without Chrome |

- `expected_rev` is optional inside a job. On a mismatch the answer carries the current rev and the current scene. Every write returns the new rev. A patch is a delta merge (pattern: `internal/gamemaker/guided_design.go:76-155`).
- Model-facing `data` is a typed object with properties, so it stays a native object and not a JSON string (`internal/agent/native_tools.go:463-464,494-505`). Beats are one flat object with a `do` enum and all-optional fields. Per-kind required fields are checked in Go, and the answer gives an example. Polymorphic `at` and `duration` accept a string form that the server canonicalises.
- The project record (not `story.json`) stores `target_seconds`, `audience` and `template`.
- The schema is compact for small models, and the server fills in the defaults (Story document, Defaults).
- `storymaker_code` `{operation: read | write | replace, scene_id, content | old/new, expected_sha256}`. Writes are checked (pattern: `internal/gamemaker/source_edit.go:77-119`). Bundling and compile errors come back with a line and a column.
- `storymaker_asset` `{operation: list | create_character | write_svg | generate_image | revise | promote}`. See Asset generator.
- `storymaker_check` `{operation: validate | stills, scene_ids}`. It returns a validation report and layout checks. Stills are only for multimodal routes. Missing evidence stays unverified.
  - **Check result.** Go validation and JS checks return one shape: `{severity, scene_id, path, code, message, at_seconds?, fix}`, where `fix` is one sentence plus an example patch. The result holds at most 8 issues, in scene order. Each code has a fix template. `overlap_face` names the prop instance and the actor, and suggests the nearest free spot. `text_overflow` gives the maximum number of characters. Go's validator also reports the static equivalents (unknown cue marker, empty narration, unknown asset or spot, with alternatives), so they work without a browser.
  - Each answer includes, per scene, `{narration_seconds, cue_times, end_seconds}`, and the total against `target_seconds`, with an `over_target` issue above +15 %.
  - Narration failures, and a missing browser or FFmpeg, are `environment` issues with `model_fixable: false`. They are never charged and never block the outcome.
  - The check is bounded to 60 s. Scenes that are still synthesising are `pending`, with the 2.3 words-per-second estimate, and the model calls again.
  - Layout check codes (in `checks.js`): `off_stage`, `overlap_face` (a prop covers a face), `text_overflow`, `text_too_small`, `caption_long`, `actor_unreachable_spot`, `beat_after_scene_end`, `cue_missing`, `narration_missing`, `asset_missing` and `code_error`, plus the motion lint codes of Motion craft (compile warnings from phase 2, `checks.js` from phase 3), with `text_too_small` merged into one rule. From phase 3 the list also has `text_contrast`, `code_history_dependent` and `slow_open`; `glyph_missing` is a Go check from phase 2 (Glyph coverage). Go's validator reports the static codes as well.
- Narration synthesis is triggered by the server during checks. The model never handles audio files.

## Asset generator

The asset generator is the `storymaker_asset` tool. The Assets panel (phase 3) offers the same operations to the user.

- **create_character.** Arguments: `{base?: char.* ID, params: {body, skin, hair, hair_color, eyes, brows, facial_hair, glasses, outfit, accessory}}`. The allowed values come from `storymaker_catalog describe id=rig.human/1`, and the same values are enums in the schema. The set is validated against the parameter catalog, and because it uses the shared rig and part library it matches the style of the rest of the cast. The answer is `{id: "my.char.<slug>", cast_example}`, with a preview cast sheet.
- **write_svg.** Arguments: `{kind: prop|background, name, svg}`. The model writes an SVG in an allowed subset: `svg`, `g`, `path`, `rect`, `circle`, `ellipse`, `line`, `polyline`, `polygon`, `linearGradient`, `radialGradient` and `stop`, with `transform`, `fill`, `stroke` and `opacity` attributes. `svg.go` sanitises the file: no script, no `foreignObject`, no `use` or `href` to external targets, no `style` with `url()`, and no event attributes. `fill` and `stroke` follow a colour grammar: a hex value, `rgb()`, a named colour, or `url(#id)` to a local gradient only. The file is capped at 256 KiB and converted to the part format. The answer is `{id, rejected_elements[], bbox, path_count, usage_example}`, where the ID is `my.prop.<slug>` for kind prop and `my.bg.<slug>` for kind background, with a render only on multimodal routes. Rejected elements are listed back to the model.
- **generate_image.** Arguments: `{kind, name, prompt, placeholder?: bool}`. It goes through the configured image generation (`tools.GenerateImageContext`, `internal/tools/image_generation.go:291-355`), with a style prompt that matches the flat palette. A cut-out uses the Pixel AI matte when a model is downloaded (`internal/pixelmatte/manager.go:319-374`). Otherwise it uses a client-free key matte for flat backgrounds. The server-side equivalent is to be decided in plan 3, because `ui/js/desktop/apps/pixel-lib-cutout.js:713,915-985` runs only in the browser. Every PNG is validated by decoding it, and by checking its dimensions and alpha. The Game Maker generate path does not decode the bytes (`internal/agent/agent_dispatch_game_maker.go:803-814`), and Storymaker decodes them.
  - The resulting ID follows the kind, as for `write_svg` (`my.prop.<slug>` or `my.bg.<slug>`).
  - Without image generation, `generate_image` is removed from the enum. A direct call returns `image_generation_unavailable` with next actions, in this order: a catalog alternative, then a text or icon callout, then the `write_svg` operation.
  - A placeholder is written only when `placeholder: true` is set. It is reported with status `placeholder`, never `ok`. The SVG placeholder format follows `internal/agent/agent_dispatch_game_maker.go:855-869`, but that function's automatic trigger (`internal/agent/agent_dispatch_game_maker.go:778-805`) is not copied.
- **revise.** Arguments: `{id, instruction | svg}`. It edits an existing project or library asset. For SVG, the edit is made in the SVG, through `svg.go`. For an image, the edit uses the source image, where the provider supports one: openai, openrouter, stability and agnes (`internal/tools/image_generation.go`, and the `image_gen_*` files). Revise of images reads the bytes from the asset ID through the project's asset registry, never from a model-supplied path. A pre-existing hole is fixed in the same change, in its own commit. The agent's generate_image `source_image` path goes through `ResolveSourceImagePath` with a DataDir fallback and no protected-path check (`internal/agent/agent_dispatch_exec.go:1575-1579`, `internal/tools/image_gen_helpers.go:28-37,279-308`), so `vault.bin` could be sent to a provider. The fix adds `tools.IsProtectedSystemPath` (`internal/tools/protected_paths.go:94`) and restricts source images to workspace files. Tests refuse `vault.bin`, `../x` and an absolute path.
- **promote.** Arguments: `{id}`. It copies a project asset into the personal library, `Documents/Storymaker/Library/`, which has a `library.json` file, and returns the library ID `lib.<kind>.<slug>`, where the kind is `char`, `prop` or `bg`.
- **list.** It returns the project and library assets.
- **Storage.** Only the converted part JSON is stored. A deterministic SVG export of the parts is produced for revise. Every SVG write (write_svg, revise, and Assets panel imports) goes through `svg.go`. Raw SVG is never inserted into the DOM.
- **Without image generation,** `create_character` and `write_svg` still work.
- **Assets panel (phase 3).** The panel offers a new-character wizard, SVG and PNG import, generate, revise, and edit in Pixel through `ctx.openApp('pixel', {path})` (`ui/js/desktop/apps/camera.js:711`).
- **Cost.** Image calls go through the existing budget tracking. `storymaker.assets.max_daily_generations` limits them, with a default of 20.

## Chat tool `storymaker`

Operations: `create`, `status`, `revise`, `render`, `open` and `list`. Their arguments and answers are in the contract below.

| Operation | Arguments | Answer |
| --- | --- | --- |
| `create` | `{topic, audience?, length_seconds?, format?, language?, music?, voice?}` | `{project_id, job_id, state: queued, eta_hint_seconds}` |
| `status` | `{project_id}` | see below |
| `revise` | `{project_id, instruction}` | starts a revise job on the existing story (same job runner; the context packet carries the story summary) and returns `{job_id, state: queued}`, or `busy` while a job of that project is active |
| `render` | `{project_id}` | `{render_job_id}`, or a precondition code with the next action |
| `open` | `{project_id}` | opens the Storymaker app on that project through the same mechanism as the `virtual_desktop` `open_in_app` operation (`internal/agent/native_tools_integrations.go:392`), and returns `{opened: true}` |
| `list` | none | up to 20 recent projects as `{project_id, title, updated_at, state, duration_seconds}` |

- **Contract.** Only `topic` is required. Defaults: audience "general"; `length_seconds` 60, bounded as in Job outcome; `format` as the aspect enum `16:9`, `9:16` or `1:1` (fps 30); `language` as the chat's language; `music` as a mood enum or `"none"`; and `voice` as an optional `{provider?, voice}`, validated like project voices (Narration).
- `status` returns `{state: queued|running|ready|ready_unverified|incomplete|failed, phase, scenes_done, scenes_total, open_checks[≤8], error_code, next_action, poll_after_seconds (≥ 30), output_path?}`.
- The `render` precondition codes are `narration_incomplete`, `browser_unavailable`, `ffmpeg_unavailable` and `invalid_story`.
- Completion posts a desktop notification. The tool manual tells the model not to poll faster than `poll_after_seconds`, or to use the existing background notify path (`internal/agent/agent_dispatch_comm.go:869-935`).
- Per-day limits for `create` and `render` from the chat tool: `storymaker.agent.max_daily_jobs`, default 10.
- **Gating.** The tool needs `storymaker.enabled` and `storymaker.agent.enabled`. Registration follows the chain that `retro_games` uses:
  - the native schema, and its dispatch;
  - the manual `prompts/tools_manuals/storymaker.md`, with a `NonDisplacingTools` entry (`internal/prompts/tool_manual_bindings.go:14`);
  - an intent trigger (`internal/agent/agent_loop_helpers.go:706-736`);
  - the metadata (`internal/agent/tool_registry_metadata.go:28-52`);
  - the category (`internal/agent/tool_categories.go:94-106`);
  - the search terms (`internal/agent/tool_search_terms/*.yaml`).

## User interface

Phase 1 delivers the play and export controls, the scene strip, the stage with its scrubber, the code tab, a minimal inspector, a simple catalog list and the app states. The rest of this section comes with the phases in the table below.

- **Toolbar.** Project menu, format, play and pause, time, and export.
- **Undo and redo (phase 5).** Each edit is stored as a JSON Pointer inverse patch, so undo applies the inverse and redo applies the patch. Consecutive edits to the same path within 300 ms merge into one entry, such as a drag; edits to different paths never merge. Each agent turn is one named entry. Undo writes through the normal story save with its ETag, so the save's conflict rules apply. The stack lives in the open session and is not saved with the project. The manual undo stays session-only. The revert of a claimed comment is different: from phase 3 its before and after snapshots are kept with the project, so the revert survives a reload (Point-to-edit comments).
- **Left: scene strip.** Thumbnails rendered by the player, the duration of each scene, add scene, and templates.
- **Centre: stage.** Selection handles, drag and drop from the catalog, and snapping to environment spots. From phase 3 a comment tool pins a note to a point of the frame, to its instance and to the beat that changed it (Point-to-edit comments). Below the stage is a scrubber, with scene blocks, a narration waveform and beat markers. There are no per-object keyframe tracks.
- **Right: inspector and panels.** The inspector covers the scene, the actor, the beat and the voice. From phase 3 it also covers the typed parameters of a code scene (Typed parameters). A docked agent panel, an Assets panel and a catalog drawer sit alongside it. The catalog drawer has the tabs People, Props, Environments, Text, Transitions, Stickers and Mine, with search.
- **Code tab.** `story.json` and the code scenes, in the vendored CodeMirror. Validation issues are shown inline.
- **Export dialog.** Quality, burn-in captions, SRT and VTT, stems, and "Open in Video Studio". In phase 1 it is a popover that shows the FFmpeg and browser state, any blockers, progress and the output path; the options come in phase 2.
- **States.** No FFmpeg, no browser, no TTS ready, music not configured, or read-only. Each state has a clear message and lists the parts that still work.
- **Player iframe.** The app runs the player host in a sandboxed iframe with `sandbox="allow-scripts"` only, and no `allow` permissions. The iframe has an opaque origin, with no `allow-same-origin`. Do not reuse the `makeSandboxedFrame` defaults, which add allow-forms, allow-modals and clipboard delegation (`ui/js/desktop/apps/quickconnect-launchpad-chat.js:1733-1744`). The iframe loads from `GET /api/desktop/storymaker/player`, a Go handler with this CSP header:

  ```
  sandbox allow-scripts; default-src 'none'; script-src <origin>/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src <origin>/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'
  ```

  The handler also sends `Referrer-Policy: no-referrer` and `Cache-Control: no-store`. The CSP has no `unsafe-inline` and no `unsafe-eval`. Do not copy `gameMakerPreviewCSP` (defined at `internal/server/game_maker_handlers.go:527`, used at `:534`), which allows `unsafe-inline` and `connect-src 'self'`.
  - The global security middleware sets `X-Frame-Options DENY` and `frame-ancestors 'none'` outside its allow-list (`internal/server/server.go:2233-2249,2275`). `/api/desktop/storymaker/player` is therefore added to `allowDesktopIframe`, as `/api/desktop/bookshelf/reader` is (`internal/server/server.go:2234`).
  - The parent accepts a message only when `event.source === iframe.contentWindow` and the message matches a fixed, typed, size-capped schema.
  - The parent sends the story and the assets only after a `ready` message from that source. It resets state on every iframe `load`, and it never sends tokens, paths or API data.
  - The messages are named in the interface contract. Parent to host: `load`, `assets`, `seek`, `play`, `pause`, `thumb` and `reset`. Host to parent: `ready`, `need_assets`, `compiled`, `time`, `thumb` and `error`. From phase 3 the parent also sends `hit_test` (t, x and y in percent), answered by the trusted player worker, and the host answers with `hit` (instance ID and pointer).
- **Untrusted text.** Story text, narration, names, tags and library names are untrusted. In `ui/js/storymaker` and `ui/js/desktop/apps/storymaker*.js`, they are set with `textContent` or drawn on canvas. No `innerHTML`, `insertAdjacentHTML` or template string with user data is used there. A grep test enforces it.
- **Registration.**
  - A `BuiltinApps` entry in `internal/desktop/types.go:796-862` with `Requires: ["storymaker"]` (Architecture, Enablement; pattern: retro-games at `internal/desktop/types.go:843` and its capability case at `internal/server/bluetooth_live.go:25`). The category is `creative`, like Video Studio (`internal/desktop/types.go:800`). In the player CSP below, `<origin>` is the AuraGo origin.
  - `logo_path` artwork at `ui/img/desktop-icons/storymaker.svg`, shown in every icon theme because the app is in none of the theme lists (as ECalc is).
  - Module-loader assets and an i18n section: `ui/js/desktop/core/module-loader.js:22-548` (`DESKTOP_APP_ASSETS`) and `:648` (`APP_I18N_SECTIONS`).
  - The render dispatch in `ui/js/desktop/core/menus-and-routing.js:1553-2027` (`renderAppContent`).
  - The global name in `ui/js/desktop/core/desktop-foundation.js:586-651` (`appGlobalName`).
  - The window size and minimum size, and open maximized, in `ui/js/desktop/core/window-shell-runtime.js:1296-1374`.
  - `npm run build:ui` for the shell files and `player.bundle.js` (`package.json:34`), and `npm run check:ui` (`package.json:39`).
  - A registration test, modelled on `ui/desktop_retro_games_registration_test.go:13-78`.
  - A root AGENTS.md child-DOX row, and new child AGENTS.md files.
- **Responsive layout.** Container queries handle narrow windows and phones. The scene strip collapses to a row, and the inspector becomes a sheet.
- **Config.** The `storymaker` section is a `StorymakerConfig` field next to VideoStudio (`internal/config/config_types.go:2194`), in its own file modelled on `internal/config/video_studio.go:10`. It holds: `enabled`; `agent.enabled`; `agent.provider` and `agent.model`; `agent.job_timeout_seconds` (default 1800); `agent.max_concurrent_jobs` (default 1); `agent.max_length_seconds` (default 180); `agent.max_daily_jobs` (default 10); `render.timeout_seconds` (default 3600); `render.max_duration_seconds` (default 900); `render.browser_path`; `narration.max_daily_chars` (default 100000) and `narration.max_job_chars` (default 20000); and `assets.max_daily_generations` (default 20). Phase 1 reads only `enabled` and the `render.*` keys, with `render.transport` (empty means PNG). The config page shows those, with labels in all 16 locales.

## Security

- **Realms and CSP.** Code scenes run only in code-scene workers, inside the sandboxed player iframe (in the app) or on the render page (for export). The barriers are the CSP (no `unsafe-eval`; `connect-src 'none'` in the app; `connect-src <render base>/` on the render page), the opaque origin and the worker realm. Shadowing is for determinism and is not a barrier. The player worker is a separate realm, so scene code cannot alter the layout checks.
- **Egress (render page).** Scene code cannot reach any origin other than the render origin. The enforced controls are listed in Rendering and export, step 2, and a browser regression test covers them.
- **Render listener.** The listener is per job, binds only loopback, is closed when the job ends, and has no public route. Its path prefix is 128-bit random, never logged and never persisted; `jobs.json` stores only its hash. Scene code can read the prefix, because the worker source names the base, so it can post frames or errors to its own job's listener; the controls in Rendering and export, step 2, keep that off every other origin. Ingest accepts frames only for the running job, in order, and within the limits in Rendering and export, step 4. The render browser has no DevTools port (a pipe is used instead), so no other process can reach its DevTools protocol. A loopback port would be open to every local account, which could read the render URL from `/json/list`.
- **Browser.** Chromium is verified against its pinned SHA-256 before first launch. Production launches never set `NoSandbox`.
- **Player messages.** The parent accepts messages only from its own iframe and only in the typed schema (User interface, Player iframe). Untrusted text is set as text only (User interface, Untrusted text).
- **Jobs and mutations.** The read-only flag is checked in the job starter, the chat tool and every paid provider call, not only in workspace writes (`internal/desktop/service_files.go:441-443`). Every job queue is capped and returns `busy` when it is full: render 16, narrate 128 and agent 8 queued. A Storymaker job calls only the `storymaker_*` tools of its current phase (Agent, Tool wiring). Budgets apply to image, music and narration calls.
- **Paths.** Paths follow the rooted desktop workspace rules, with no traversal and no symlinks (`internal/desktop/secure_open.go:11,63,133`). Asset IDs never map to arbitrary paths. Revise reads source images through the asset registry, and protected system paths are refused (Asset generator).
- **Output and FFmpeg.** Every new FFmpeg input has protocol and format whitelists, and output names are sanitised (Rendering and export, steps 5 and 6).
- **Voices.** Voice IDs are validated, and the ElevenLabs path is escaped (Narration).
- **SVG and images.** The SVG sanitiser works from an allow-list. Raw SVG is never inserted into the DOM. PNG decoding has size and dimension limits (pattern: `internal/gamemaker/visual.go:70-84`; the pixelmatte limits are 32 Mi pixels and 32 MiB).
- **Secrets.** Secrets never reach the browser. The voices endpoint and the narration both run through the server.
- **Data.** Story text, narration, catalog text and tool results are data, never instructions (pattern: `internal/server/game_maker_prompt_profile.go:18`).
- **Typed parameters and comments.** Parameter values and comment text are data. Go checks their shape and size, the worker checks values against the declarations, and both are set as text only (Typed parameters, Point-to-edit comments).
- **Fallback fonts.** The Noto faces are read-only, through the ui namespace by their fixed basenames (Glyph coverage).

## Error handling

Stable job error codes (pattern: `internal/gamemaker/job_outcome.go:35-48`): `invalid_story`, `narration_failed`, `narration_unavailable`, `narration_incomplete`, `storymaker_disabled`, `music_failed`, `browser_unavailable`, `ffmpeg_unavailable`, `render_failed`, `render_timeout`, `cancelled`, `interrupted`, `budget_exceeded`, `image_generation_unavailable`, `busy`, `code_error` and `model_error`. `failed` is the job outcome that carries one of these codes (Agent, Job outcome). `not_supported_yet` is a minor issue code, not a job error. The UI maps each code to a localised message that includes the next action.

## Testing

Lean, per the user's preference: the essential tests only.

- **Go unit tests:**
  - decoding and validation (strict decoding, limits, alternatives);
  - voice resolution, cache keys and voice-ID escaping (the ElevenLabs path with `url.PathEscape`);
  - chunking;
  - the SVG sanitiser and the colour grammar;
  - the mix graph construction, including the stems;
  - the job lifecycle, including the job outcome;
  - the ingest limits (size, frame index, frame count, header dimensions, running state);
  - the refusal of protected paths for source images;
  - the refusal of a general tool name inside a Storymaker job;
- from phases 2 and 3: the glyph check against a font's cmap (a missing script is refused at its path); the typed-parameter shape and size; the comment store (append-only, a torn last line skipped, the released and stale events) and the refusal of a revert whose story has moved on; the frame-count gate.
- **JavaScript tests (Node, through `vm`):** compile timing (cues, the early arrival and its clamp at 0, the lead of each cue-led beat kind, the sfx impact placement, `delay`, auto durations, transitions); cue placement in speech spans, with de, el, hi and ja fixtures; the history probes and the contrast percentiles; no two narration or dialogue events overlapping; easing; the lip-sync mapping; caption splitting and the cue mapping of caption times; the checks; the Video Studio track builder (it replaces the Go handoff test); and the host's postMessage source check.
- **Static guard:** a grep test finds no `innerHTML`, `insertAdjacentHTML` or template string with user data in `ui/js/storymaker` and `ui/js/desktop/apps/storymaker*.js`.
- **Browser egress regression:** scene code assigns `location`, calls `window.open` and calls `fetch` to a public host. All three fail.
- **Determinism test:** the same story rendered twice gives identical frame hashes of the decoded frames.
- **Parity test:** the decoded RGBA of frame n from the app player host and from the render page are identical, at the export canvas size, with device pixel ratio 1, after the readiness barrier, in the same Chromium build, with motion blur off.
- **Parallel parity test (phase 2, headless Chrome):** one story rendered with N player workers and with one gives the same decoded RGBA for every frame, with motion blur off. The result decides whether the 2-frame pre-roll is kept (Rendering and export, step 4).
- **One real export probe:** a short story, with FFmpeg and Chrome. It is gated like the other browser smokes, with `AURAGO_RUN_BROWSER_SMOKE=1`.
- **One browser smoke tour of the app.**
- **Guards:** registration, i18n in all 16 locales, the external-asset guard and the line budget.

## Phases

Each phase gets its own plan, written after the previous phase lands.

| Phase | Delivers |
| --- | --- |
| 1 Foundation | Go model, validation, project and catalog index, and CRUD API; config section (enabled, render.*); player runtime (host, player worker, code-scene worker, fallback) with compile, timeline, easing, transitions (cut, fade, slide-left and wipe), text, rig renderer, props, environments, camera, and the enter, exit, walk, move, pose, gesture, emote, look, show (item and text), hide, camera and wait beats; starter catalog (about 4 characters, about 20 props covering office/IT and symbols, about 4 environments, base poses, expressions, mouth shapes, walk and idle cycles, 2 palettes, 3 text styles, 4 transitions) with names in all 16 locales; app shell (scene strip, stage preview and scrubber, code view, minimal scene inspector, simple catalog list to insert items); code-scene API basics; render spike, a silent render job (listener, Chrome provisioning, egress controls, ingest, FFmpeg) and a status endpoint; registration and tests. The compile report is the frame count, the scene ranges and the issues. |
| 2 Sound & export | Voices endpoint; narration jobs, cache, envelope and cues, with cue placement in speech (`placed`, `cue_precision`) and per-language voice notes for zh, ja, hi and el; dialogue (say); speech bubbles; captions (burn-in, SRT and VTT); lip-sync; music (generated, file or track); SFX beat and the CC0 set with `impact_ms`; mix, stems and loudness with the narration-referenced bed, the presence carve, the impact dip and the frame-count gate; export dialog; Video Studio handoff and the `projectId` launch parameter; highlight beat; motion craft in compile (role eases, a hide or non-walking exit at 0.35 s, cue timing with the lead for arrivals, camera focus and charts, and the sfx impact rule, log-space zoom, overlap at 40 %, text holds with the 1.2 s secondary floor, shrink to fit, the 2.6 % floor on the phase 1 text sizes, `style.rhythm`); the static motion lint as compile warnings (`cue_late` included); true-peak ceiling, hit levels, take rotation and optional auto-SFX (`style.auto_sfx`); glyph coverage with the Noto fallback and `glyph_missing`; procedural effects (option); parallel player workers in fixed chunks of C frames dealt round-robin, with the reorder window and the parallel parity test. |
| 3 Agent & asset generator | Job runner, SSE and agent panel; studio tools; prompt profiles (normal and compact); checks.js and checks in headless Chrome; stills as review sheets; prompt house style, banned tells, rhythm plan and message-shape table in both profiles; three visual directions as key-frame stills; provenance table; filmprint and `look_repeat`; optional cold reviewer (normal profile, multimodal routes); typed parameters for code scenes (`api.param`, `params`, inspector fields, `patch_scene`); point-to-edit comments (`hit_test`, `comments.jsonl`, per-comment snapshots and revert); contrast from pixels (`text_contrast`); the code-scene history check and the code-scene frame checks; the code-scene glyph report, unverified (Glyph coverage); `slow_open`; motion lint in `checks.js`; outline templates; call repair; job outcome; asset generator, Assets panel and library; chat tool, manual and registration chain; budgets and caps; config for agent. |
| 4 Catalog expansion | Full v1 counts; chart and diagram blocks (chart beat, `api.chart`, `api.diagram`) with the chart rules (bars from zero, a note on an axis that does not start at zero, ticks on 1, 2, 2.5 or 5 times 10 to the k, `data.source` or the illustrative badge); layout scenes `kinetic-type` and `data-story` (`kind: "layout"`, `layout`, `cues`); remaining text styles, transitions and palettes; cast sheet review; 16-locale names; catalog drawer polish; kinetic text entries (`words`, `letters`, `ink`, `blur-line`) with their stagger; the `iris`, `zoom` and `whip` transitions (fade until this phase), the `dip` transition, the whip smear and the iris and zoom origins; character acting (head lead, arm follow, emote takes, seeded blink and breath); opt-in canvas motion blur with a 180-degree shutter (phase 5 if this phase is full). |
| 5 Editor comfort | Drag and drop staging; inspector forms for all beat kinds; template gallery; undo and redo as JSON-Pointer inverse patches (rules in User interface; session-only, unlike the saved comment reverts); shortcuts; phone layout polish; canvas motion blur if phase 4 did not take it. |

Rules: from phase 1 the validator knows the full schema, except for the additive values of Motion craft: `style.rhythm` and `style.auto_sfx` (phase 2), and the text entries and the `dip` transition (phase 4), and the additive fields of phases 3 and 4: the scene `params` (phase 3), and the `layout` scene kind with its `layout` and `cues` fields and the chart `data.source` (phase 4). They are not in the phase 1 closed sets, so phase 1 refuses them as errors. A story without them is valid in every phase. A story that uses one is shown by an older build with an error, and that build refuses to render it: for `style.rhythm` and `style.auto_sfx` it shows the raw text with `unknown_field`, and for `dip` or a text entry it opens the story with `unknown_name` or `invalid_value`. For `params`, `layout`, `cues` or `data.source` it shows the raw text with `unknown_field`, and for `kind: "layout"` it opens the story with `invalid_value` at the kind path. Beats and fields that the closed sets already name, but whose phase has not landed, are accepted, ignored by the player, and reported as a minor `not_supported_yet` issue. The narration precondition is vacuous when a story has no narration, so phase 1 renders are silent. Phase 1 plays the beats enter, exit, walk, move, pose, gesture, emote, look, show (item and text only), hide, camera and wait, and the transitions cut, fade, slide-left and wipe. Any other transition plays as a fade, with a minor `not_supported_yet` issue. The validator reports `say`, `highlight`, `chart`, `sfx`, a show with an icon or chart, a cast speaker and music other than `none` as minor issues, and the player ignores them. A known pose, expression or gesture that is missing from the catalog falls back to stand, neutral or no gesture, with the same issue.

## Out of scope for v1

3D scenes; a per-object keyframe timeline; 4K and 60 fps; a whiteboard or draw-on style; word-level ASR alignment for lip-sync and cues; voice cloning; publishing to video platforms; real-time collaboration; Chromium bundled in the Docker image; Remotion; and the HyperFrames and huashu-motion items under Motion craft (Not adopted from HyperFrames, Not adopted from huashu-motion).

## To verify in the plans

Phase-1 drafting resolved or narrowed the items below. Each carries its result.

- **Frame transport (phase 1 spike).** The throughput and determinism of ingest POST, `captureScreenshot`, an evaluate blob and raw RGBA POSTed to the loopback ingest. The encode cost of PNG against lossless WebP in Chrome at 1080p. The measured fps feeds the render timeout (Rendering and export). **Resolved.** PNG encodes faster than lossless WebP at 1080p (21.7 against 7.0 fps in the first spike, and WebP files are about half the size). PNG is the default. The numbers are in Rendering and export, step 4. The timeout uses 6 fps for PNG, a conservative figure until aurago-test is measured.
- **Chromium availability.** The pinned revision and its SHA-256 per platform. The runtime download inside Docker (which needs egress), and on Windows. Repo-built executables have failed to start Chrome with a crashpad access-denied error, and the known workaround is the `-rod=bin=` flag. The AppArmor note for Ubuntu 23.10 and later. **Partly resolved.** The pinned revision is 1321438 (Chromium 128.0.6568.0). The Windows and Linux archives were hash-checked and unpacked in tests, and the macOS archives only in a synthetic test. Starting the pinned browser is not done: this box refused to run a freshly downloaded executable. Docker and Ubuntu are not tested either (the Docker image lacks Chromium's shared libraries, so containers usually need `render.browser_path`). Browser tests run through `go test` on this box. Open until the aurago-test acceptance run.
- **Phase 1 gate (player runtime).** Blob workers with OffscreenCanvas inside the opaque-origin sandboxed player iframe, `importScripts` of the absolute player bundle URL under the stated CSP, and FontFace in workers. The fallback is as stated in Player runtime. **Resolved: passed in Chrome 156, no fallback needed.** Blob workers that `importScripts` the absolute bundle URL, `OffscreenCanvas` with `Path2D`, `FontFace` in workers and `ImageBitmap` transfer to the parent all worked under the player CSP in the opaque-origin iframe. A `fetch` from a code scene was blocked, and so was an `importScripts` outside `/js/storymaker/`. Firefox and Safari were not tried.
- **FFmpeg versions.** FFmpeg 5.1.9 (the production floor) against 8.1.1: the behaviour of `sidechaincompress`, `loudnorm`, `acrossfade` and `image2pipe`. **Partly resolved.** Phase 1 ran the real render on FFmpeg 5.1.9 (a Debian 12 container: 342 frames at 10.2 fps) and on 8.1.1 (Windows). Both work with the phase 1 flags. Phase 2 closes the rest. The mix uses neither `sidechaincompress` nor `acrossfade`: `internal/storymaker/render/mix.go` is pure Go, although both filters exist with the same options on both versions. The meter reads a float pipe with `ebur128` and `loudnorm` in one graph (`internal/storymaker/render/meter.go`, run on both versions by its test). The same pipe metering gave identical numbers on 8.1.1 and 5.1.9: -22.02 LUFS and -5.84 dBTP on a 900 s film (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mixReal.md`).
- **Colour and gain on 5.1.9.** The `image2pipe` and `rawvideo` colour round-trip (swatch check), and the loudness gain. **Resolved.** `TestFFmpegSwatchAndGate` passes on FFmpeg 8.1.1 (Windows) and on 5.1.9 (one `debian:12` container): a PNG and a raw swatch each decode within 2 levels of their BT.709 limited-range values, and the frame gate accepts the file on both versions. The meter replaced the two-pass `loudnorm` gain (see "Changes from the phase 2 plan"), and Task A10 ran its graph on both versions.
- **Audio rates and voices.** The sample rates of sanoTTS and Phonon output. Whether Speech Lab honours a per-call voice. The OpenAI TTS voice list. The length of Lyria output. **Resolved for local providers.** sanoTTS writes mono s16 WAV at 22050 Hz (24000 Hz for English), Phonon mono s16 WAV at 24000 Hz, the keyless google path mono MP3 at 24000 Hz (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/tts.md`). `internal/storymaker/audio_decode.go` reads WAV and MP3 by their bytes and the mix brings every clip to 48 kHz, so a cloud rate needs no answer here. Speech Lab uses the `/ready` snapshot and no per-call voice. The OpenAI list is the 13 voices of `OpenAITTSVoices`. Lyria: the loop and the trim cover any length.
- **Font licences.** The licences of the bundled fonts in `ui/fonts` that are used for text styles. **Resolved for the phase 1 default.** The default font is Geist (400, 500, 600 and 700; SIL OFL 1.1, listed in `THIRD_PARTY_LIBRARIES.md`). Montserrat was rejected because `ui/fonts` holds only its weight 500, split by unicode range. Glyph coverage of ä, ö, ü, ß and Latin Extended was checked in Chrome.
- **Clip semantics.** The `Clip.AudioDetached` semantics for the handoff video clip. **Resolved.** A clip with `audio_detached:true` needs a video asset with sound (`internal/videostudio/validate.go`). The MP4's AAC track always exists, so the handoff passes. The detached clip is silent on its video track and the audio clips play instead (`internal/videostudio/plan.go`). The output of `S.handoff.buildTracks` decodes strictly into `videostudio.Project` and passes `videostudio.Validate`.
- **Asset-set size.** The current file count and size of the asset set, against `MaxFiles` 20000 and `MaxExpandedBytes` 2 GiB, before the catalog is added. **Resolved, with a warning for phase 4.** The phase 1 catalog is 105 files and 2.37 MiB, within its own budget. The asset set holds 17986 files and 1.05 GiB with phase 1, so about 2000 files of headroom remain under `MaxFiles` (Catalog and rig, Budget).
- **Key matte.** The server-side key matte for generated images, for when no AI matte model is downloaded. **Open (phase 3).**
- **Cross-browser preview differences.** How the app preview in Firefox, Safari and with GPU raster differs from the headless render (anti-aliasing and text metrics), against the authoritative headless render. **Open.** Only Chrome 156 has been tested so far.
- **Strict-mode capture proxy.** The strict mode of `security.StartCaptureProxy` and CDP Fetch interception for worker targets, checked by the egress regression test. **Resolved, with a change.** The strict mode has its own tests. CDP `Fetch` cannot intercept worker targets (`Fetch.enable` fails on a worker session), so the guard is the hybrid in Rendering and export, step 2. With all layers on, the regression test (`TestEgressLayersEachStopTheProbe`) stops every hostile request. The strict proxy alone also stops them all. The CSP and sandbox alone let a top-level navigation through, which is why the guard blocks navigations too.
- **Worker blur (phase 4).** `ctx.filter` blur for the `blur-line` entry: determinism and speed in a worker's OffscreenCanvas on the pinned Chromium 128 and on the installed Chrome. If either fails, `blur-line` falls back to alpha only. **Open.**
- **Motion lint false positives (phases 2 and 3).** The rate of each motion code on the fixture stories, with the thresholds tuned before the phase 3 gate, including the code-scene pixel thresholds (Motion lint). **Rates recorded, tuning open for phase 3.** Rate per scene of each static code on the fixtures, without audio: line_unread 0.548, dead_stretch 0.000, first_frame_empty 0.032, text_too_small 0.000, fade_between_worlds 0.032, entry_monotony 0.032, transition_variety 0.000, cue_late 0.000.
- **Parallel render memory (phase 2).** The memory and frame rate of N player workers at 1080p on a CPU like aurago-test, which sets N. **Resolved on the development box; aurago-test is not measured.** `firewall.json` at 1080p30 on 16 logical CPUs: 12-15 fps serial, 24 fps at N=4 with PNG on the host, 30-36 fps at N=4 and 36-41 fps at N=6 with PNG in the worker. Each added worker costs about 18 MB in the renderer plus about 9 MB of reorder buffer. The Chrome tree holds 773 MB at N=4 with host PNG and 876 MB at N=6 with worker PNG (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/parallelWorkers.md`). N comes from `ChooseWorkers` in `internal/storymaker/render/workers.go`. `TestParallelParityBrowser` holds on `firewall.json` and `speech.json`.
- **Ceiling and stems (phase 2).** Whether the −1 dBTP gain keeps the narration, music and sfx stems summing to the mix after AAC encoding. The linear part is exact before encoding. **Resolved.** The stems sum to the mix exactly before encoding: the integer null test found 0 nonzero samples of 86.4 million at 900 s. After AAC the residual is about 18 dB below the mix at 64k and 23 dB at 192k, so the guarantee says "before encoding", and audio is AAC 192k. No overshoot in the measured cases: decoded true peak -1.48 dBTP on the synthetic film, -1.07 and -1.10 dBTP on real shapes A and B (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mix.md`, `docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mixReal.md`).
- **Blind comparison (phase 3 acceptance, then after phase 4).** Storymaker output against two or three HyperFrames template renders of one brief, judged blind. The default HyperFrames path needs the Open Design desktop app, so the template renders come from there. **Open.**
- **Rhythm factors (phase 2).** The first factors (calm 1.25, standard 1.0, lively 0.8) are tuned on the fixture stories. **Open for the user's eyes and ears.** The factors ship as first guesses.
- **Default fade (phase 2 decision).** Whether the default `fade` follows environment changes. Go writes `fade` into stored scenes, so a changed default reaches only new scenes. Until it is decided, `fade_between_worlds` flags the default at each environment change, and the agent fixes it by setting cut or wipe (iris and zoom play as fade until phase 4). **Resolved.** A new scene gets `cut` as scene 1, `wipe` when its environment differs from the previous scene's, else `fade`. Stored scenes keep their transition, so a stored fade between worlds still gets `fade_between_worlds`.

- **Early arrival (phase 2).** Whether a cue-triggered arrival reads as landed on its word on the fixture stories, how often `cue_late` fires, and whether emote and gesture beats with a cue should lead as well. Only arrivals, camera focus, a zoom with a target and charts lead in this revision. **Partly resolved.** Rate of `cue_late` per scene on the fixtures, without audio: 0.000. Cued beats: 1, with cue_late: 0. Whether an arrival reads as landed on its word, and whether emote and gesture beats should lead as well, stay open for the user's eyes.
- **Cue placement in speech (phase 2).** The span threshold (−40 dB, from huashu-motion cli/lib/voice.js:643, is a starting value for the RMS envelope) and the share of cues that the spans place on the fixtures, including de, el, hi and ja stories for the Unicode weights and the proportion fallback. **Replaced.** The vowel-weight rule is dropped: it erred 581-730 ms mean against 341 ms for proportional placement (n=30). `cueMap` places a marker at a sentence start on that sentence's onset and proportionally over the sentence's speech inside it, and each block has `placed: span` or `proportion`. The span threshold stays -40 dBFS with gaps under 100 ms bridged. Share of cues placed by spans on hand-built blocks in de, el, hi and ja: de 1, el 1, hi 1, ja 1. Those blocks prove the rule and its fallback. They do not measure a provider, so the share with a real voice stays open.
- **Mix limiter (phase 2).** Whether a look-ahead limiter after the gain is needed for loud films (huashu-motion cli/lib/sound.js:453 uses −1.5 dBFS). A limiter is not linear, so the stems would stop summing to the mix unless it also applies to each stem. The default is the linear gain. **Resolved: no limiter.** Normal shapes land at -16.0 to -17.6 LUFS on real material (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/mixReal.md`). An sfx-only film ends near -22 LUFS, accepted.
- **Music shape checks (phase 2).** A 30 Hz high-pass on the bed (huashu-motion cli/lib/sound.js:450) and a loudness check per stretch against the energy curve (huashu-motion cli/lib/sound.js:548-554). **Not adopted in phase 2.**
- **Glyph fallback (phase 2).** Whether Geist covers Greek (the check decides; `el` is a locale), which face covers Greek if it does not, whether the monochrome emoji face is added, and how Go reads a WOFF2 cmap. **Resolved.** Geist has only 6 Greek letters, so Inter Greek and Greek Extended come first with their `unicodeRange`; otherwise Chrome draws ω as Ω. No emoji face is added: emoji, Thai and Hebrew are `glyph_missing`. Go does not read WOFF2: `internal/storymaker/glyphs.go` reads the build-generated `ui/img/storymaker/glyphs.json` (`docs/superpowers/plans/2026-11-04-storymaker-phase2/evidence/fonts.md`).
- **CJK face in workers (phase 2).** The 16.4 MB face is above the 8 MiB per-font cap. The memory of one player worker with it, and the choice between a cap for this face and a subset owned with the QR module. The file is already in the asset set. **Resolved.** No subset, because a subset breaks the per-character check. `NotoSansCJKsc.otf` gets a named 17 MiB exception through `max_bytes`. Measured: 80-120 MiB per worker, 292-330 ms load per worker. The face loads only when compile needs it.
- **Readability ratio (phase 2).** The phase 1 text sizes (title 0.07, caption 0.042 and label 0.034 of the short side, phase 1 catalog plan part D) clear the 2.6 % floor at a 1080-pixel short side, and a phase 2 test keeps them so. For the phase 4 kinetic-type layout, a label at 0.4 of its secondary line needs that line at about 70 px or more, and then a title at about 239 px, at a 1080-pixel short side; whether that layout keeps the 3.4 : 1 : 0.4 ratio is open. Huashu's shorter exit of about 0.18 s (huashu-motion runtime/clip/lib/typo.js:3) is not adopted; exits stay at 0.35 s. **Partly resolved.** The phase 2 lint suite pins that title, caption and label clear 2.6 %. The phase 4 ratio stays open.
- **Whole-film flatness (phase 3).** A flat-film test (mean motion below 0.6 with fewer than 3 cuts a minute, huashu-motion cli/lib/qa.js:68, in huashu's units) and the average shot length could check the rhythm plan after the fact. Not adopted in this revision. **Open.**
- **Per-language voice notes (phase 2).** The listening tests for zh, ja, hi and el, and the wording of the notes. **Open for the user's ears.** The notes hold facts only. The `not tested` mark ships on the provider and language, shown for every provider in phase 2.
- **Procedural effects (phase 2).** Whether the synthesised effects sit well beside the CC0 set on the fixture stories. **Not built.** 25 WAVs rendered once with earshot ship in `assets/storymaker/sfx/`, with provenance in `assets/storymaker/README.md`. The build copies them. Their taste stays open for the user's ears.
- **History-check thresholds (phase 3).** The 0.5 grey level mean and the four probes (huashu-motion cli/lib/check.js:266-271) are tuned on the fixture stories, including a scene that is meant to change over time. **Open.**
- **Contrast false positives (phase 3).** The percentile method on text over images and busy backgrounds, the 1.15:1 masked-text rule (huashu-motion cli/lib/check.js:254-255), and whether title styles count as large text. **Open.**
- **`slow_open` (phase 3).** Its 5 s window, its 1 s flat level, its 0.25 % still step and its 1.5 s still stretch are starting values in Storymaker's units (Motion lint, pixel rules), with huashu-motion (cli/lib/qa.js:51, :66-67) in its motion units as their source. They are tuned on the fixtures, and their sampling cost is checked against the 60 s budget. **Open.**
- **Film sameness, reviewer and comments (phase 3).** The last 10 films, the four-of-six rule, the cold reviewer's image budget per route, and the 50 snapshot pairs and 500 characters per comment are starting values. **Open.**
- **Colour per stretch (phase 3).** huashu's colour guidance asks for colour planned per stretch, while this spec keeps one palette per story. The filmprint records the palette, so the rule can be checked on real films before it changes. **Open.**
- **Shutter (phase 4).** Whether a canvas blur with a 180-degree shutter matches the time-sampled average on a fixture. **Open.**
- **Deferred ideas.** Cuts snapped to the music beat and taste notes are not in this revision (Not adopted from huashu-motion). **Decision pending.**

## Changes after phase-1 drafting

- Header: related documents line added; the update date is recorded.
- Chromium: the browser order is the configured binary, an installed Chrome or Chromium, then the pinned Chromium 128 (revision 1321438), and a stalled download ends after 60 s.
- Architecture: two player bundles; the `render/` subpackage; phase 1 file lists; no `characters/` or `sfx/` folders in phase 1.
- Config: `storymaker.enabled` defaults to true; `render.transport` added (empty means PNG); phase 1 reads `enabled` and `render.*` only.
- Render: PNG is the default, with the measured rates; frames are POSTed as `text/plain` to `<render base>/frame?n=<n>`; the render-page frame deadline is 5 s from `boot.json`; the snapshot is taken at queue time; the hybrid egress guard; the render browser has no DevTools port; `interrupted` error code; `jobs.json` keeps only the prefix hash.
- Export: phase 1 writes into the project folder, and the `Videos/` copy moves to phase 2; `Slugify` produces the slug; a silent AAC track; progress by frame count; the export dialog is a popover.
- Validation: the voice provider ID is checked against the dispatcher IDs from phase 1 on.
- Budget: the asset set holds 17986 files with phase 1, about 2000 files below `MaxFiles`.
- Story and player: the first scene's entry transition is ignored; enter and exit take 0.6 s; unimplemented transitions fall back to fade; the code-scene API subset, with `chart` and `diagram` as stubs; the code-scene pre-check is a tokenizer.
- Protocol and data: app message names; `Prop.scale`; `severity` in issues; compile report fields; `stroke_width`; `mouth.*` and `cycle.*` prefixes; SVG thumbnails; the app icon in `ui/img/desktop-icons/`.
- Fonts and i18n: Geist as the default font; `ui/lang/storymaker/` and `ui/lang/storymaker_catalog/`.
- To verify: each item carries its phase 1 result; the phase 1 gate passed in Chrome 156.

## Changes from the motion research, 2026-11-02

- Decisions table: a Motion craft row, approved by the user on 2026-11-02.
- Motion craft section added after Catalog and rig: principle, easing and timing, transitions, character acting, text entries, house style and banned tells, motion lint, review sheets, and the ideas not adopted.
- Story document: `style.rhythm` (phase 2), the `dip` transition and the text entries (phase 4) are noted in place; they are additive, and no phase 1 value changes.
- Phases rule: the exception for the additive motion values is explicit, and the "accepted and ignored" rule is limited to values the closed sets already name.
- Timing: from phase 2, a hide and a non-walking exit take 0.35 s, beats on different targets overlap at 40 % of the previous beat's scheduled duration (cue beats were exempt until the cue rule of 2026-11-02, and beats without a target stay sequential), and text holds extend the automatic scene length; phase 1 keeps 0.6 s and sequential beats.
- Player runtime: from phase 3, code-scene frame checks make `dead_stretch` and `first_frame_empty` verified for code scenes.
- Agent: stills become review sheets in phase 3; the motion codes join the layout check codes, and `text_too_small` becomes one rule in both lists.
- Music and sound: a −1 dBTP ceiling on the linear gain with the same gain on the stems, hit levels 3 to 6 dB over the bed, seeded take rotation, and an optional auto-SFX that is off by default (phase 2).
- Rendering: parallel player workers for storyboard frames with in-order posting (phase 2); opt-in canvas motion blur (phase 4, or phase 5).
- User interface: undo and redo as JSON-Pointer inverse patches, merged within 300 ms for the same paths, one entry per agent turn (phase 5).
- Phases table: rows 2 to 5 extended; row 1 unchanged.
- Out of scope: a pointer to the Not adopted list.
- To verify: seven items added, including the default fade as an open decision.
- Review of the motion section (2026-11-02). Transitions: the house style and the lint name only the transitions that play in their phase (cut, fade, slide-left and wipe before phase 4); `fade_between_worlds` reads the transition the player plays; `dip` is not named before phase 4.
- Eases and rhythm: walking is linear at catalog speed; non-walking arrivals use quart-out and camera focus uses expo-out; rhythm scales compile defaults only and no longer chooses eases.
- Timing: overlap applies only between beats with differing targets, at 40 % of the previous beat's scheduled duration; a hide and a non-walking exit take 0.35 s; walking enter and exit keep the phase 1 timing.
- Lint and outcome: lint findings are advisory for the job outcome; a text hold counts as activity for `dead_stretch`; code-scene checks have pixel rules and a 0.5 s sampling step inside the 60 s check.
- Options: `style.auto_sfx` is a story field (phase 2); motion blur is a render option, and the parity test runs with it off.
- Review sheets: one scene and at most three images per stills call, with at most 12 tiles per beat sheet. Product and concept explainers plan 19 words per scene from the outline.
- Versioning and sources: additive values stay in `storymaker/1`, and older builds show them with an error; Chrome 151 is a floor; the sources are the HyperFrames documentation and source (Apache-2.0).

## Changes from the huashu-motion research, 2026-11-02

- Decisions table: two rows, cue timing (phase 2) and point-to-edit comments (phase 3), both decided by the user.
- Story document: `params` (phase 3), and the `layout` kind with `layout`, `cues` and chart `data.source` (phase 4), noted in place; `music.volume` is an offset from the narration-referenced bed (phase 2). The Rules paragraph and the versioning line are extended. No phase 1 value changes.
- Motion craft: cue timing (an arrival lands on its word, with a lead, the `cue_late` warning, sound effects on the landing, `impact_ms`); log-space zoom; a 1.2 s floor for secondary lines; the readability ratio and shrink to fit (phase 2); `slow_open` and the pixel contrast rule (phase 3); the house style with the rhythm plan, the message-shape table and three directions (phase 3); sources and licences; the not-adopted list.
- Glyph coverage (phase 2): the cmap check, the Noto fallback from `ui/fonts/qr/` by decision with the QR module, `glyph_missing`, and the CJK face size. Code-scene text is not checked here; its glyphs stay unverified (`layout_unverified`).
- Narration and music (phase 2): cue placement in speech with `placed` and `cue_precision`; the narration resampled to the mix rate and measured once on the stem; the bed at voice minus 12 LU with the 5 dB carve; the impact dip; procedural effects; the limiter stays open; per-language voice notes.
- Rendering (phases 2 and 4): fixed chunks of C frames dealt round-robin to the workers, with the reorder window, the pre-roll and the parallel parity test; the ffprobe frame-count gate; the 180-degree shutter for motion blur.
- Agent (phase 3): the code-scene history check, the provenance table, the filmprint with `look_repeat`, the optional cold reviewer, typed parameters for code scenes, and point-to-edit comments with per-comment snapshots.
- User interface: the undo bullet separates the session-only manual undo from the saved comment reverts; the player messages gain `hit_test` and `hit` (phase 3).
- Corrections from the fact-check applied: the −14 LUFS target applies only without narration; the limiter is a −1.5 dBFS look-ahead; the presence carve is 5 dB; `slow_open` is a 5 s motion check on the rendered film, not a 3 s rule; learn has no 1 KB cap; the reviewer watches the final film and gets a provenance table; the 2.6 % floor is HyperFrames', not huashu's; the zoom citation is huashu-motion runtime/om.js:144, not cli/om.js.
- Phases table rows 2 to 5 and To verify extended; row 1 unchanged.

## Changes from the second review of the huashu-motion section, 2026-11-02

- Rendering: storyboard frames go to the workers in fixed chunks of C frames (start value 8), dealt round-robin; the host's reorder buffer is capped at N × C × 2 frames; the replay is dropped, the 2-frame pre-roll is kept under the parallel parity test, and that test is a headless-Chrome test.
- Captions and cues: caption line times use the cue mapping; a block that falls back to proportion does so for its cues and captions together; the compile report gains `cue_precision` and per-scene `cues` with `placed`. The interface contract changes in phase 2 to match.
- Cue timing: every beat kind with a cue has a rule (arrival, sfx impact or marker start); `delay` moves the landing; an sfx that uses `with: "prev"` sits on the previous beat's landing; camera focus, a zoom with a target and charts lead like arrivals.
- Text sizes: the 3.4 : 1 : 0.4 ratio applies to the phase 4 kinetic-type layout only; the phase 1 catalog sizes clear the 2.6 % floor.
- Loudness: the narration level is measured once on the assembled stem; the bed reference without narration is −18 LUFS; the filter order in `mix.go` is fixed.
- Word weights: Greek, kana, Devanagari and Arabic rules are defined, and a block with any other script is placed by proportion.
- Directions: a set is refused when two directions share their transition set, text style and environment family.
- Comments: `released` and `stale` events; the pointer is stored with its scene ID, beat index and story rev; the after snapshot is written whatever the outcome.
- History check and `slow_open`: the sampling worker renders in order and its probes are compared with a fresh worker; `slow_open` has starting values and sampling in Storymaker's units.
- Glyphs: code-scene text is not glyph-checked and stays `layout_unverified`; the code-scene build reply carries the phase 3 parameter declarations.

## Changes from the phase 2 plan

**Narration**
- Line 135, 499: the envelope is `audio/narration/<hash>.env.json`, a 10 ms RMS series free of the frame rate. A per-fps file per piece drifts when pieces are joined. Go builds per-frame block levels and spans at answer time.
- Line 84: envelopes are not transferred to workers. Levels travel inline in `audio_info`.
- Line 97: voice resolution is in `voices.go`, not `narration.go`.
- Line 495: one sentence is one piece, not sentences grouped up to the chunk limit. An edit then re-synthesises one sentence and sentence starts are exact. Fragments under 20 width or of 3 words or fewer merge into a neighbour.
- Line 496: sanoTTS is already serialised (`internal/tools/sanotts.go:18-27`). The narrate queue adds one call at a time for every provider.
- Line 497: the key is computed per piece, with provider and language trimmed and lower-cased.
- Line 498: decoding uses a new `audio_decode.go`, because `voice.DecodeWAVPCM16Source` accepts only mono s16. ffprobe is not a fallback. Music durations come from the WAV header or go-mp3's length, with no full decode.
- Line 490: no code enforces a 400-character limit. It stays our own conservative size.
- Line 492: per-language notes hold facts only. Listening notes wait for the user's tests, because nothing here can judge speech. The `not tested` mark ships, on the provider and language, shown for every provider in phase 2.
- Line 494: the ElevenLabs body also needs real JSON (`tts.go:487-488`), not only the path escape.
- Lines 494, 506: `Validate` checks the shape only, because the package does not import `internal/tools`. The check against the list runs in `POST /narrate`, which answers 409 `voice_unknown` with at most 5 alternatives. The agent's `kind=voice` discovery is phase 3.
- Lines 484, 837: the OpenAI list is the 13 voices named in the plan; `gpt-4o-mini-tts` is hard-coded (`tts_cloud.go:21`) and shuts down 2027-01-06.
- Line 483: `FetchXAIVoices` is a live client method, cached 5 minutes.
- Line 487: google is offered only when active, and `POST /narrate` refuses it otherwise. Providers are listed active first, then local, then cloud, so resolution never picks a paid provider ahead of a free one.
- Line 488: Speech Lab uses the `/ready` snapshot; no per-call voice exists.
- Line 501: the vowel-weight rule is dropped. It erred 581-730 ms mean against 341 ms for proportional (n=30). `cueMap` places a marker at a sentence start on that sentence's onset, and proportionally over the sentence's speech inside it. Each block has `placed: span` or `proportion`, and the fallback no longer depends on the script.
- Line 502: narration clips of different voices are matched before the stem is measured, capped at +6 dB, so a film with two speakers lands near target without a meter per clip. Clips reach the mix rate through FFmpeg in job staging.
- Line 504: the `narration_incomplete` gate lands in batch 2C with a seeded-audio test helper. Until then a narrated story exports silently.
- Line 505: `POST /projects` takes an optional `language`. Without it the project has `voice:null`, as in phase 1, and the voice panel writes one. `POST /narrate` answers 409 `voice_required` without a voice.
- Line 503: a silent scene silences its narration only; say text stays spoken. A say without text draws the active caption lines of the scene narration until that narration ends, has no block and does not move the schedule. In a silent scene it draws nothing and gets a warning. A say whose actor is off stage still plays its audio, with no bubble and no lip-sync. Say text has the 1500-character limit of narration.
- Audio is untrusted on disk: `env.json` and `meta.json` are checked when read, an invalid one counts as missing, and a piece that no longer decodes is synthesised again.
- Daily and job caps reserve characters before the call, because a provider that answered has billed.

**Motion**
- Lines 189, 345: `exit` always walks today, so 0.35 s applies to `hide`.
- Line 229, 364, 850: the default transition is `wipe` when the environment changes, else `fade`, for new scenes only. A stored fade between worlds still gets the lint.
- Line 342: `emote` has no ease. The expression stays a step until phase 4.
- Line 346: overlap measures from the anchor, the longest beat of a `with:"prev"` group, not strictly the previous beat. A short beat that runs with a long one must not start the next target's beat early.
- Lines 348, 817: the chart lead lands with the chart beat in phase 4. Phase 2 leaves `chart` out of the schedule.
- Line 353: for holds a run of CJK characters counts 1 word per 2 characters. The same count feeds the estimate.
- Line 355: rhythm factors are first guesses. Code-scene API defaults are not scaled.
- Line 357: shrink to fit runs in `text.layout`, because compile has no canvas and runs before fonts. Line 409: the phase 2 `text_too_small` reads the style size only.
- Determinism: every seed comes from strings, since no project seed exists among the compile inputs. `Intl.Segmenter`, `\p{Script}` and `normalize` are not used in compile or the wrap, because they vary with ICU. Generated range tables from a committed Unicode 16.0.0 data file replace them.

**Glyphs**
- Lines 119, 326: Go does not read WOFF2. `glyphs.go` reads a build-generated `glyphs.json` that carries each face's coverage.
- Line 327: Geist has only 6 Greek letters. Inter Greek and Greek Extended come first with their `unicodeRange`, or Chrome draws ω as Ω. Emoji, Thai and Hebrew are `glyph_missing`. Default-ignorables and JS whitespace are exempt, and the NFC form is checked. A silent scene's narration is not checked, because it is never drawn.
- Line 330: no subset. `NotoSansCJKsc.otf` gets a named 17 MiB exception, because a subset breaks the per-character check. Measured: 80-120 MiB per worker, 292-330 ms load.
- Fallback faces load on need. Render mode fetches fonts lazily.

**Sound**
- Lines 33, 312, 522, 817: the SFX set is not a CC0 download. 25 WAVs are rendered once with earshot: forge sounds are code-made, and whoosh, click, ding and success derive from CC0 Freesound recordings named in `assets/storymaker/README.md`. The output-rights reading stays open.
- Line 99: `mix.go` lives in `render/` and is pure Go, not an FFmpeg audio graph.
- Line 514, 589: looping uses an equal-power crossfade in `mix.go`, not `acrossfade`.
- Lines 516-519: ducking uses a feed-forward stage in `mix.go`, not `sidechaincompress`. Both filters exist on both FFmpeg versions, so the reasons are exact stems, a 1000-clip graph, look-ahead and unit tests.
- Lines 521, 605, 836: the meter is one FFmpeg process reading a float pipe with `ebur128` and `loudnorm` in one graph, I from ebur128 (loudnorm under-reads an 8 s film by 1.6 LU), TP from loudnorm. It is not a two-pass `loudnorm`. The gain is `min(-16-I, -1-TP, -0.1-max stem peak)`.
- Line 522: take rotation is seeded from scene id, beat index and sound id, and the last take spans the film. Hit levels: pop 4.5/3, ding 4.5/3, success 4.5/4, error 4.5/4. Auto-SFX also skips a pop whose landing already has an sfx beat running `with:"prev"` on it.
- Line 523: procedural effects are not built. A dev-time render with earshot gives the same repeatability. 25 WAVs ship, copied by the build.
- Line 524, 854: no limiter. Normal shapes land at -16.0 to -17.6 LUFS. An sfx-only film ends near -22 LUFS, accepted.
- Line 525: the preview bed plays through `<audio>`, and narration decodes in a sliding window, because a 15-minute decoded bed is 345 MB. No carve in preview.
- Line 589: generated music length comes from the app's last compile until phase 3's check job. A stale length is absorbed by the loop or the trim. A local profile below 600 s gets the profile's limit as its duration, and the loop covers the rest.
- Line 511 stays: modes are `generated` and `file`, with `file` XOR `track`.
- Music jobs share the narrate queue. `POST /music` also copies file and track sources to a hash name. Decided here: wav and mp3 sources only, at most 1 GiB, 1 s to 3600 s. A moved source reports `music_issue: source_missing` and does not stop narration.
- Generated music starts only from an explicit click, because a key changes at each pause in typing.

**Rendering and export**
- Line 83: the 10 s no-frame watchdog starts with FFmpeg, after the mix, not when the report is accepted.
- Line 544: PNG is encoded in the worker. Worker encoding exists only for PNG, so other transports stay serial. Pre-roll is off by default and stays switchable, because every frame was byte-identical with and without it. `N = clamp(floor((cpus-2)/3),1,6)`, bounded by memory, and Go passes it in `boot.json`.
- Line 553: the frame gate also checks audio against video within 25 ms, because the packet count does not notice a wrong audio length. `-shortest` is removed.
- Lines 555, 773: new inputs use narrower format lists than `videostudio/probe.go:16-17`: `wav,mp3` for decoding, `f32le` for the meter, `wav` for the mix, `mov` for the gate.
- Line 556: the restart stays an answer to `/frame`.
- Line 560: the camera-import reason is wrong for a nested path (`video-studio-projects.js:433` accepts top-level `Videos/<file>` only). The library copy is for discoverability, and the handoff imports from `exports/`. Writing to `Videos` needs new code, `WriteStorymakerVideo`.
- Line 562: stems and subtitles go into `exports/<slug>-<job8>/`. There is no `handoff` render option: handoff is a client action on any finished job with stems, so a closed window cannot lose it.
- Line 563: the per-file cap in phase 2 is 2 GiB (`render.go:50`).
- Line 577: `audio_events` has one entry per piece, adds `scene_id`, `block_id`, `piece`, `take`, and has no `gain`. Go computes every gain.
- Line 578: `captions[]` adds `scene_id` and `block_id`.
- Line 582, 768: a one-shot token guards `/report`, `/frame`, `/error`, `/start`, and Go validates refs, counts, times and caption text. Go also requires each measured block's pieces in order, so an empty `audio_events` cannot defeat the gate. `maxReportBytes` is 8 MiB.
- Line 588: mixing no longer runs inside the report request, because three timers would cut it. The page long-polls `/start`.
- Line 771: the read-only flag is re-checked before every paid call, and a revocable desktop run also cancels a call in flight. Addition: disk is estimated before any copy or decode, because the 256 MiB floor is too low for a 900 s film with stems. Every narration piece has a decode limit too, not only music.
- Line 783: `voice_required` and `music_incomplete` join the stable code list.
- `alimiter` is in use in Video Studio's mux at `videostudio/render.go:394`, unrelated to this mix.

**Handoff and app**
- Lines 594, 601: the handoff and the track builder live in `storymaker-handoff.js`.
- Line 605: stems sum to the mix exactly before encoding only. After AAC the residual is about 18 dB below the mix at 64k and 23 dB at 192k. Audio is AAC 192k.
- Line 607: the launch parameter also touches `menus-and-routing.js`, because the existing window is always reused.
- Line 737: the export dialog is a native `<dialog>` in its own module.

**After the dry run and the review**
- A dot is no sentence end before a lower-case word or a digit, or after 1 to 3 digits, a single letter or a listed abbreviation, so "19." and "z. B." stay inside a sentence. German closing quotes count as closers.
- A code scene without `api.duration` covers its narration. Narration or a say that runs past an explicit scene duration warns.
- Generated music calls a provider only from the Generate click, pinned by a key in the job. A source never chosen is not a missing source.
- The preview follows the export's bed and SFX levels from stored measurements. It is close, not equal, and the panel says so.
- SRT writes `&` literally; only VTT uses entities.
- The export panel offers the newest finished render after a reopen, which keeps the promise that a closed window cannot lose the handoff.
- Both paid panels have a stop control. A scene can be set silent in the inspector. Deleting a project waits up to 15 s for its jobs.
- `interfaces.md` section 10 in the plan folder lists the changed names and the items decided against.

**Phase 1 projects after the update**
- `style.captions` defaults to true (`contract line 61`), so every narrated phase 1 story starts drawing captions in preview and export from 2B. The voice panel has a toggle.
- A say without text whose `who` is not the scene speaker moves from a minor issue to a blocking error.
- A `mood` outside the presets moves from a minor issue to a blocking error in 2C.

**To verify, closed:** Clip semantics, audio rates (local), OpenAI list, Ceiling and stems, Default fade, Glyph fallback, CJK in workers, Parallel memory, Procedural effects (not built), Speech Lab per-call voice (none used), Lyria length (the loop and trim cover any length), Mix limiter (none), Cue placement in speech (replaced by `cueMap`), Music shape checks (spec line 855, not adopted in phase 2).

**Still open for the user's ears or hands:** voice notes and rhythm factors; sentence pause and trim margins; taste of the SFX set, especially `error` and `success`; whether per-sentence synthesis keeps good prosody, on cloud voices above all; the output-rights reading of earshot renders; whether emote and gesture beats with a cue should lead as well (spec line 852); the audible sentence gap, about 490 ms against the 350 ms the rule names; the level of non-hit SFX, which follows the music slider; the mood prompt texts. aurago-test was unreachable on 2026-11-04, so its acceptance stays open.

**Still open as decisions:**
- Whether code-scene workers should get language faces only when the scene registers `onFrame`. It would save about 100 MiB per worker, but D13 says they get them, so phase 2 keeps D13 and counts the memory in `ChooseWorkers`.
- `audio/narration` grows with every edit and voice change, and nothing prunes it. A prune needs a rule for queued renders, whose `Plan` names hashes fixed at queue time. Deleting a project frees it.

**Deviations found during execution**
- Report refusals scrub the prefix and token before clipping (`want.clean`; `bad()` cleans its whole reason).
- The audio answer is capped in Go (`MaxBlockLevels` 27000, `MaxTotalLevels` 200000, `maxBlockSpans` 2000, helper `limitAudioBlock`). Levels over the cap are dropped.
- The loudness meter uses `framelog=verbose`, because FFmpeg 5.1.9 rejects `framelog=quiet`.
- A generated music file that is not WAV or MP3, or is over 1 GiB, drops its pending file, so the next Generate pays for a new one.
- When the pending write fails after a paid call, the runtime remembers the result for the next click. A restart in between loses it.
- The music key holds the mood's name and not the template text, so editing a template keeps stored tracks.
- The preview's duck reaches -6 dB 50 ms before speech and the export reaches it at speech start. The preview has no carve, voice match or impact dips, as the plan's E2 text says.

## Changes from the agent core plan

The agent core is the first slice of the Agent section, ahead of the rest of phase 3. Its plan is `docs/superpowers/plans/2026-11-06-storymaker-agent-core/`.

**Left out of this slice:** `storymaker_code`, `storymaker_asset`, stills and review sheets, `checks.js`, three directions, the rhythm plan, the message-shape table, provenance, filmprints, the cold reviewer, typed parameters, point-to-edit comments and the Assets panel.

**Scope corrections**

| # | Correction | Reason |
| --- | --- | --- |
| C1 | A compile check in Chrome, `render.Runner.CompileCheck` | It is the only source of real scene lengths and motion lint. It stops after the report, needs no FFmpeg, is bounded to 60 s and never downloads Chromium. |
| C2 | A Go job lint with `scene_unfilled` | A scene with only an id and narration passes `Validate`, so a weak model could stop after the outline with a "valid" film of empty stages. |
| C3 | `POST /projects/{id}/make-silent` | Ticking `silent` by hand breaks the film: the scene falls to its minimum length, captions vanish and cues never start. |
| C4 | No `agent.max_concurrent_jobs` | `render.Manager` allows 0 or 1 workers. The limit is fixed at 1 running and 8 queued. |
| C5 | No `music` in `create` or `set_meta` | Generation stays the user's click. A story with generated music and no file would be refused at export. |
| C6 | Narration runs after the outcome is known, not during check rounds | The audio key contains the text, so each patch during fixes would bill cloud characters again. |
| C7 | Job tools stay out of the normal catalog and go through `ExecutionHooks.HandleTool` | They have no feature flag, category, search term, manual or dispatch case. |
| C8 | No voice is not a failure | The story keeps `voice: null`, and the job reports `narration_unavailable` as an open check. |

**Corrections found in review**

| # | Correction |
| --- | --- |
| R1 | The render and narrate refusals became functions without an `http.ResponseWriter`, so the job uses the same refusal order as the routes. |
| R2 | Chat `render` takes `silent: true`, runs the `MakeSilent` path and says that speech was removed. The `narration_incomplete` answer names this option. |
| R3 | After the final compile, when model-fixable issues remain and rounds are left, one more check loop runs and narration runs again. Only changed scenes are billed. |
| R4 | Render and agent exclude each other: a render answers 409 `agent_job_active`, and an agent start or revise answers 409 `busy` while a render, narrate or music job of the project is queued or running. |
| R5 | The runner does not set `cfg.LLM.UseNativeFunctions`. It reads the mode from the tooling policy and sets `req.ToolChoice` on native routes only. |
| R6 | The check and revise packets carry the same `cast` and `environments` blocks as the compact fill packet. |
| R7 | A loop is restarted at most once after a model error or an empty end. The tool-call limit counts tool calls, so a third outline call fits inside the limit of 6. |
| R8 | `max_daily_jobs` counts every create and revise, from the app and from chat, plus chat `render`. The counter is `<data>/storymaker/agent/usage.json` and fails closed. App routes answer 429 `daily_limit_reached`. |
| R9 | Each batch compares its failures with a baseline recorded on main. |
| R10 | `make-silent` first writes `story.before-silent.json`. `POST /projects/{id}/restore-speech` puts it back. Project reads carry `silent_backup: true` while the file exists. |

**Plan sections 7, 9 and 10.** Section 7 of the plan is the list R1 to R10 above. Section 9 adds the word check (`words_tell`: a dash or a stock phrase in narration or on-screen text gets one optional fix round as advice, and never blocks) and the revise lists (`changed_scenes`, `unchanged_scenes` and `removed_scenes`, in the job and in the chat status). Section 10 settles the seams between the code parts: one routing table for Studio operations, `over_export_limit` for a film longer than the export allows, and the bundle path `ui/js/desktop/bundles/main.bundle.js`.

**Plan section 11.** The review of the applied code added corrections V1 to V9, and an agent job has no token ceiling of its own: its spend is bounded by the job timeout, the tool-call limits per step and `max_daily_jobs`.

**Changed lines of this spec**
- Line 613: one running and eight queued agent jobs are fixed, and one job per project is enforced with `Coalesce` on a third `render.Manager`.
- Line 614: "no silent switch" holds for a pinned provider or model. Without a pin the job uses the main route with its configured failover, because turning that off would make jobs fail for no gain.
- Line 617: the compact profile hides `kind=voice` in cast. The first job step resolves the voice and writes it into the story.
- Line 619: compile runs in the check step. Narration runs after it, as C6 says.
- Line 622: a scene has a hard ceiling of 8 writes (`StudioSceneWriteLimit`). Tool calls per loop are outline 6, cast 12, fill 10, check 20 and revise 30. A failed or exhausted scene keeps its outline text, is recorded as `scene_unfilled`, and the job moves on.
- Line 628: `canonicalizeStorymakerOperationCall` only routes a call to its owner. Studio wraps flat fields into `data`. The recovery nudge lives in `internal/agent/game_maker_recovery_nudge.go` and `RunConfig.RecoveryNudge`.
- Lines 635-638: this slice never emits `ready`, because layout is not verified. `failed` means no outline was saved. `incomplete` means a timeout, a model error, or any unfilled scene, model-fixable issue or `compile_failed` left. `over_target` and environment items are listed and never downgrade. The job status is `failed` only for outcome `failed`.
- Line 640: `length_seconds` is clamped, not refused, so a chat call does not fail.
- Line 654: `action_policy.go:314` is now `:321`, `agent_dispatch_exec.go:226-234` is `:228-234`, and `dispatch_platform.go:312-313` is `:307-308`. The dispatch-switch edit is dropped, as C7 says.
- Line 662: `set_meta` has no `music`, as C5 says.
- Line 671: `ProjectStore.SetAgentMeta` writes `target_seconds`, `audience` and `template`.
- Line 681: the server narrates after the check, waits at most `min(300 s, time left)`, and reports the result as an open check. It never blocks the outcome.

**User edits during a job:** nothing merges. The editor is locked, and `PUT story`, `PUT code` and `POST narrate` answer 409 `agent_job_active`. Unsaved local text stays, and the existing conflict banner offers reload or overwrite after the job.

**Still open:** the real-model acceptance on aurago-test (task F4), real text-mode replies on nested `data`, whether weak models can write `{n}` markers, and the visual quality of films, because layout stays unchecked until `checks.js`.
