# Storymaker desktop app — design

Date: 2026-11-01. Status: approved by the user section by section (authoring = storyboard JSON plus JavaScript code scenes; catalog style = flat vector with rigs; agent in the app and as a chat tool; storyboard editor without a per-object keyframe timeline; export = one JavaScript player everywhere, rendered by server-side headless Chrome into FFmpeg; five phases). Revised after a five-lens review (fidelity, feasibility, weak-model, security, completeness).

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
| Chromium | A configured binary (`storymaker.render.browser_path`), or a pinned Chromium revision that AuraGo downloads at runtime and verifies with a SHA-256 check before first launch. Not added to the Docker image in v1 (the user accepted the runtime download). |
| Catalog size v1 | About 24 characters, about 16 poses, 10 expressions, 7 mouth shapes, walk, idle, nod and head-shake cycles, about 120 props, about 24 environments, text styles, transitions, chart and diagram blocks, and a small CC0 SFX set. |
| Phases | 1 foundation, 2 sound and export, 3 agent and asset generator, 4 catalog expansion, 5 editor comfort. Each phase gets its own plan, written after the previous phase lands. |

## Constraints that shape the design

- No external assets (fonts, libraries, analytics). `TestShippedWebAssetsLoadNothingExternal` (`ui/external_assets_test.go:13-81`) guards this. All fonts come from `ui/fonts`, and all libraries come from `ui/js/vendor`. CodeMirror (`ui/js/vendor/codemirror-bundle.esm.js`) and DOMPurify (`ui/js/vendor/purify.min.js`) are already vendored.
- Sixteen desktop locales: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv and zh. No `t('key', 'English')` fallbacks (`ui/i18n_lint_test.go:149-156`). The key `desktop.app_storymaker` must exist in all sixteen (`ui/desktop_i18n_cleanup_test.go:180-204`). The 16-locale used-key test has a hard-coded prefix list that has no new sections (`ui/desktop_i18n_cleanup_test.go:145`), so Storymaker adds the `storymaker` prefix there or gets its own 16-locale test (precedent: `ui/desktop_cyd_creator_designer_i18n_test.go:40`).
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

The app preview runs this code. Export runs the same code in server-side headless Chrome, stepped frame by frame. FFmpeg encodes the frames and mixes the audio. Parity is stated in two parts. Timeline identity holds everywhere, because it is the same code on the same inputs. Pixel identity holds within the same Chromium build: the render page and the parity test compare decoded RGBA at the export canvas size, device pixel ratio 1, after the readiness barrier. The app preview in other browsers (Firefox, Safari, GPU raster) may differ slightly in anti-aliasing and text metrics. The headless render is authoritative, and the layout checks run there.

Rejected options, with reasons:

- **Browser WebCodecs export.** The browser window would have to stay open (the Game Maker weakness), and browser support varies. The repository has no WebCodecs or muxer code (`ui/js/vendor` has none), and MediaRecorder runs in real time.
- **Compiling to a Video Studio project.** Video Studio's vocabulary is fixed by its effects and animation registries (`internal/videostudio/effects.go` and `internal/videostudio/anim.go`). It cannot express limb animation, lip-sync or code scenes.

Cost: Chromium runs on the server for export, for agent layout checks and for stills. A new frame-input FFmpeg path is also needed, because none exists today. `internal/videostudio/segment_graph.go:78-91` takes files only.

Division of labour (this avoids a second implementation):

- JavaScript owns compile, the player and the layout checks, in one codebase.
- Go owns schema validation, catalog ID checks, limits, storage, audio (narration, envelope, music and mix), jobs and render orchestration.
- Go never recomputes the timeline. It receives the compile report from the JavaScript side (see Rendering and export, Compile report and job order), and it trusts reports only from its own headless jobs.

### Player runtime

- **Classic scripts.** All player scripts under `ui/js/storymaker/` are classic scripts, with no ES modules at runtime. `npm run build:ui` (`package.json:34`), or a small build step, builds them into one bundle, `ui/js/storymaker/player.bundle.js`. Node tests load them through `vm` (precedent: `scripts/test-pixel-core.mjs`, `scripts/test-ecalc-core.mjs`).
- **Three realms.** They are the same in the app and on the render page.
  1. **Host** (trusted, our code). In the app this is the player iframe's top document; on the render page, the top document. It receives resources, creates the workers, routes frames and owns the deadlines.
  2. **Player worker** (trusted, our code only). It is created from a blob URL whose body calls `importScripts(<base>/js/storymaker/player.bundle.js)`, where `<base>` is the AuraGo origin in the app and the render base (Rendering and export, step 2) on the render page. It runs `compile`, `renderFrame` for storyboard scenes, transitions and composition, captions and the layout checks (`checks.js`). Story JSON is data, so no untrusted code runs in this realm, and its compile report and checks are trustworthy.
  3. **Code-scene worker** (untrusted, one per code scene). It runs exactly one code scene and returns only its declared duration, its frames (ImageBitmap from its own OffscreenCanvas) and errors. It never touches the player worker's realm. Layout checks inside code scenes are reported as **unverified**; only `code_error`, the deadlines and the duration bounds are verified by the host.
- **Code-scene delivery.** Authors write an ES module exporting `default function scene(api)`. On save, Go bundles it in process with esbuild (`go.mod:27`, `github.com/evanw/esbuild`; precedent: Game Maker's in-process build, `internal/gamemaker/build.go:14-195`) into one classic IIFE that registers the scene function on a fixed global. `import` statements are rejected, because there are no imports in v1. A static pre-check on the esbuild AST rejects `eval`, `Function`, `import()`, `globalThis`, `self`, `constructor` access, `importScripts`, `fetch`, `XMLHttpRequest` and `WebSocket`. The pre-check is defence in depth only. The real barriers are the CSP (no `unsafe-eval`, `connect-src` as below) and the worker realm. The bundled text reaches the host as data and becomes the code-scene worker's blob body, which imports `player.bundle.js` for the scene API before it runs the scene.
- **Determinism.** The player and code-scene workers shadow `Date`, `Math.random`, `performance.now`, `crypto.getRandomValues`, `setTimeout`, `setInterval` and `requestAnimationFrame`. Shadowing is for determinism; it is not the security barrier. `api.random(seed)` is the only randomness.
- **Deadlines (watchdog = `worker.terminate()`).** Code-scene compile has 2 s; one frame has 250 ms in the app preview. On the render page the host enforces the same per-frame deadline, and Go additionally kills Chrome when no frame arrives within 10 s. Either overrun is reported as `code_error`. `storymaker_check` has its own 60 s bound.
- **Fonts and assets.** The host passes font bytes (from `ui/fonts`), part JSON, images, envelopes and code-scene bundles to the workers as transferable ArrayBuffers. Workers build fonts with `new FontFace(name, buffer)` and `self.fonts.add`. A typed `need_assets` message lets the player worker ask for catalog or project assets that a code scene names at compile time. The host only answers with known catalog, project or library asset IDs.
- **Readiness barrier.** Frame 0 is rendered only after all fonts in use are loaded and all images are decoded.
- **Fallback if workers are unavailable in the sandboxed iframe (phase 1 gate).** Storyboard scenes render in the host realm, which runs trusted code only. Code scenes are not previewed live in the app. The app shows a placeholder frame and a "Render stills" action, which runs a check job server-side.

## Architecture

```
internal/storymaker/            Go package, no server dependency
  model.go        Story document types (schema storymaker/1), strict decode (DisallowUnknownFields), limits
  validate.go     schema + reference validation, issues {scene_id, path, code, message, at_seconds, fix}, max 8 per answer, catalog alternatives
  catalog.go      catalog index read from webassets.Namespace("ui") (ui/img/storymaker/catalog.json), search/describe
  project.go      project folder layout in the desktop workspace, revisions (ETag), library
  codebundle.go   in-process esbuild bundle of code scenes (classic IIFE), import rejection, static pre-check
  narration.go    voice resolution, synthesis plan, cache keys, chunking, measured durations, envelope
  music.go        music generation/loop/trim, file selection
  mix.go          FFmpeg audio graph (narration + music + SFX, ducking, loudness), mixed WAV and stems
  render.go       render job: per-job listener, Chromium provisioning, frame ingest → FFmpeg stdin, encoder selection reuse, outputs
  jobs.go         own job manager (kinds narrate, music, render, agent, check), jobs.json persistence, SSE events, cancel
  svg.go          SVG subset sanitizer + converter to the vector part format (asset generator)
internal/server/storymaker_*.go HTTP handlers under /api/desktop/storymaker/ (player host at /api/desktop/storymaker/player)
internal/agent/native_tools_storymaker.go, agent_dispatch_storymaker.go  studio tools + chat tool
internal/server/storymaker_agent.go   isolated job runner (prompt profiles, model pin, broker)
ui/js/storymaker/               player realm scripts (classic scripts, outside ui/js/desktop), built into player.bundle.js
  compile.js  timeline.js  ease.js  rig.js  parts.js  stage.js  camera.js  text.js  captions.js  transitions.js
  charts.js  diagrams.js  codescene.js  checks.js  host.js (postMessage protocol, workers, deadlines, ingest)
  player.bundle.js  built by npm run build:ui
ui/js/desktop/apps/storymaker*.js  desktop app UI modules (scene strip, stage, inspector, catalog, agent, code, export)
ui/css/desktop-app-storymaker.css
ui/css/storymaker-player.css     the only stylesheet of the player host page (named in its CSP)
ui/img/storymaker/              generated catalog: catalog.json, rig/, characters/, props/, environments/, thumbs/, sfx/
scripts/build-storymaker-catalog.mjs  code-authored parametric SVG parts → part JSON, manifest, thumbnails (deterministic)
assets/storymaker/              catalog sources (part generators, palettes, README with licence + budgets)
prompts/tools_manuals/storymaker.md  chat tool manual
```

The two HTML pages are written by Go, not shipped as static files, and each gets its own CSP header (see User interface, Player iframe, and Rendering and export, step 3):

- the sandboxed player host for the app iframe, at `GET /api/desktop/storymaker/player`;
- the render page for headless Chrome, served by the per-job loopback listener of the export job.

Their scripts are static files under `ui/js/storymaker/`, which is an allowlisted directory (`assets/web-assets.json:5`), built into `player.bundle.js`.

Project storage. Each project is a folder in the desktop workspace, `Documents/Storymaker/<project id>/`, like Video Studio's `Documents/Video Studio/<id>/`:

```
story.json        the story document (ETag-protected writes)
project.json      the project record: target_seconds, audience, template, created_at, and the job history references
scenes/<id>.js    code-scene sources; scenes/<id>.bundle.js is the bundled classic script written on save
assets/           project assets (my.*): part JSON for characters, props and backgrounds; validated PNGs
audio/narration/  narration audio <hash>.<ext> and envelopes <hash>.<fps>.env.json
audio/music/      generated or copied music
exports/          export records (job ID, output path, stems, SRT/VTT)
```

The personal library lives in `Documents/Storymaker/Library/` with a `library.json` index (Asset generator, promote).

Enablement. `storymaker.enabled` gates the app and its API: the `BuiltinApps` entry carries `Requires: ["storymaker"]`, and `desktopCapabilities.HasCapability` (`internal/server/bluetooth_live.go:14-38`) gets a `storymaker` case that returns the config flag (the `retro_games` pattern). With the flag off, the app is hidden and `/api/desktop/storymaker/*` answers 403 `storymaker_disabled`. `storymaker.agent.enabled` additionally gates the agent panel, the studio job runner and the chat tool; with it off, the app works as an editor without the agent.

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
- **Style.** `style.palette` names a catalog palette; the phase 1 starter palettes are `sunny` and `calm`. `style.font` is `default` (the catalog's default font) or the name of another font from a small set of bundled OFL fonts (their licences are to be verified in `ui/fonts`). `style.captions` is a boolean.
- **Voice.** `voice` is the project default. `provider` is a concrete provider ID from the voice provider table (Narration) and is never "active" in the saved file. A new project snapshots the active TTS provider and its configured voice at creation, so a later change of the global TTS does not change old projects. `cast[].voice` is an optional per-character override for dialogue. `language` is always explicit.
- **Music.** `music.mode` is one of none, generated or file. `mood` comes from the presets. `prompt` is optional extra text. When the mode is file, `music.file` names a rooted desktop path (no URL or protocol prefix), and `music.track` names a Noisemaker media ID. `volume` is between 0 and 1, default 0.25. Ducking is automatic.
- **Cast.** Each `cast[]` entry has an `id` (a slug), a `character` (a catalog ID, or a project or library asset ID), a `name` (the display name, default = id) and an optional `voice`.
- **Scenes.** Each `scenes[]` entry has:
  - `id`, a stable slug;
  - `kind`, either `storyboard` (the default) or `code`;
  - `environment`, an ID, and `variant`, either `day` or `night` (default day);
  - `narration`, text with `{n}` cue markers;
  - `silent`, a boolean (default false). A silent scene ignores its narration;
  - `speaker`, either `narrator` (the default) or a cast ID. A cast speaker gets lip-sync and an optional speech bubble;
  - `actors`, each with a cast ID, `at`, `pose`, `facing` (left, right or auto; default auto, which means towards the stage centre), `expression` and `scale` (0.5 to 2; default 1);
  - `props`, each with an instance ID `id`, an `item`, an `at` and a `layer` (back or front; default front);
  - `beats`, as set out in Beats below;
  - `transition`, applied on entry from the previous scene, one of cut, fade, slide-left, slide-right, slide-up, slide-down, wipe, zoom, iris or whip;
  - `duration`, either `"auto"` (the default) or a number of seconds;
  - `camera`, `{framing: wide|medium|close, target?}`, the camera's starting framing (default wide).
- **Beats.** The beat kinds are a closed set. Each beat names its kind in `do`. The arguments of each kind are in the Beats table.
- **Timing.** Beats run in order. `"with": "prev"` runs a beat together with the previous one. `cue: n` starts a beat at narration marker `{n}`. A beat can also set an optional `delay`, `duration` and `ease`. `ease` comes from a closed list: linear, ease-in, ease-out, ease-in-out, back-out and bounce-out. Defaults per beat kind (for example, enter takes 0.6 s and walk takes a time based on distance at a catalog speed) are server or compile defaults that the model never has to set. Timing defaults are never written into the story; compile.js owns them.
- **Positions.** A position is either an environment spot (`desk`, `door`, `board`, `window`, `left`, `center`, `right`, `far-left`, `far-right`, or a per-environment spot) or `{x, y}` in percent of the safe area. Depth comes from the environment's floor line.
- **Scene duration.** `duration` is `auto` or a number of seconds. The `auto` rule, the transition overlap and the minimum for silent scenes are the scene-local timing rule in Rendering and export (Compile report and job order).
- **Limits.** These are constants, to be adjusted in the plan: 200 scenes; 40 actors and props per scene; 60 beats per scene; 1500 narration characters per scene; 15 minutes of total video; `story.json` up to 1 MiB; a code scene up to 64 KiB. Model-facing writes (`write_svg`, and `storymaker_code` with `write`) aim for 16 KiB or less per call (compact profile 8 KiB). Larger SVGs grow through `revise` steps, and larger code through `replace` steps with a unique old block and `expected_sha256` (pattern: `internal/gamemaker/source_edit.go:99-119`). An over-size argument is answered with an error code that names this guidance.
- **Versioning.** The `schema` string is checked. Migrations in Go run on load (there are none for v1). Unknown fields are rejected with a hint, using strict decoding (pattern: `internal/gamemaker/game_plan.go:281-300`).
- **Validation output.** Go validation and the JS checks use one issue shape: `{scene_id, path, code, message, at_seconds?, fix}`, where `fix` is one sentence plus an example patch. At most 8 issues are returned per answer, in scene order (Agent, storymaker_check). Unknown IDs return the exact catalog alternatives (pattern: `internal/gamemaker/design_errors.go:9-86`, `internal/gamemaker/guided_design.go:336-357`).
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
| `show` | one of `item`, `text`, `icon` or `chart`; `at`; `entry` (pop, fade, slide or none) | `id` (instance ID; `<item-slug>-<n>` when omitted, and echoed back) | `{"do":"show","item":"prop.lock","at":"right","entry":"pop"}` |
| `hide` | `target` (instance ID, or item ID when unique in the scene) | | `{"do":"hide","target":"prop.lock"}` |
| `highlight` | `target`; `style` (pulse, outline or arrow) | | `{"do":"highlight","target":"prop.lock","style":"pulse"}` |
| `camera` | `op` (zoom, pan, focus or shake) | `target`; `amount` | `{"do":"camera","op":"focus","target":"anna"}` |
| `chart` | `chart` (bar, line, pie or counter); `data` (`{labels[], values[]}`); `at` | | `{"do":"chart","chart":"bar","data":{"labels":["Mo","Di"],"values":[2,5]},"at":"right"}` |
| `sfx` | `sound` (`sfx.*` ID) | | `{"do":"sfx","sound":"sfx.pop"}` |
| `wait` | `seconds` | | `{"do":"wait","seconds":1}` |

Every beat also accepts the common optional fields `with` (`prev`), `cue` (a narration marker), `delay`, `duration` and `ease`.

### Defaults

Go fills the defaults in this table before saving, only for fields the model omitted, and lists each one it filled in the answer as `defaults_applied: [{path, value}]` (at most 8 entries).

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
- Forbidden: timers, `Date`, `Math.random`, the DOM, the network and `eval`, and the other names that the static pre-check rejects (Player runtime). Shadowing exists for determinism. The CSP and the worker realm are the barriers.
- Errors from the bundling and the compile carry a line and a column. They are returned to the editor and to the agent.

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

## Catalog and rig

Rig `rig.human/1`:

- The rig has 13 bones: root, hips, torso, neck, head, upper arm L and R, lower arm L and R, upper leg L and R, and lower leg L and R. The hand and the foot are end parts with their own pivots, not extra bones. Hand poses swap the hand part (open, fist, point, hold).
- Views are front, three-quarter and profile. Left-facing views are mirrored.
- Part format (JSON): each part lists its shapes as `{d (SVG path data), fill, stroke, strokeWidth, opacity}` entries, and carries a pivot and a parent bone. Linear and radial gradients are allowed in a small form. Parts are drawn with Path2D on Canvas2D, so they stay sharp at any size.
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
- Props attach to the hands through sockets. The pattern is the humanoid sockets in `internal/gamemaker/asset_packs/aurago-low-poly/manifest.json:34089-34106`.

Content counts for v1:

- Props (about 120), in these categories: office and IT (laptop, phone, monitor, server, router, cloud, database, lock, key, mail and similar); business and money; home and everyday; science and health; transport; nature and weather; symbols and icons (check, cross, warning, idea, question, arrows, gears); and UI cards.
- Environments (about 24), each with parallax layers, named spots, a floor line and a day and night palette: office, meeting room, home office or living room, kitchen, classroom, lab, hospital, server room, shop, street or city, park, factory, warehouse, presenter studio, whiteboard wall, abstract tech grid, plain or gradient stages, space, night city, and countryside.
- Text styles: about 8 titles, lower thirds, captions, speech and thought bubbles, labels with arrows, counters, and check lists.
- Transitions, and chart and diagram blocks.
- A small CC0 SFX set: whoosh, pop, click, ding, success, error and typing.

Manifest: `ui/img/storymaker/catalog.json`. Each entry has the form `{id, kind, tags[], name_key, thumb, file, spots?, palette?, params?, defaults?, usage_example}`. IDs are stable, using the prefixes `char.*`, `prop.*`, `env.*`, `pose.*`, `expr.*`, `gesture.*`, `text.*`, `trans.*`, `sfx.*` and `palette.*`. Project and library assets use the prefixes `my.*` and `lib.*`. Names come from `storymaker.catalog.<id>` keys in all 16 locales. The pattern is the 100 `stickers.name.*` keys in `ui/lang/desktop/en.json:229`. The agent works with English IDs and tags.

Production: `scripts/build-storymaker-catalog.mjs` builds the parts from code-authored parametric generators in `assets/storymaker/`. This follows the Blender kits, which are dev-time generators that commit their outputs (`assets/chat-ocean-life/README.md`). The build is deterministic and has a `--check` mode. Cast sheets are rendered for review, and the outputs are committed.

Budget: the catalog stays within 30 MiB and 2000 files. That fits the 20000-file ceiling; the current total is to be measured in the plan. Licence: the catalog is original work, recorded in `assets/storymaker/README.md`.

Reuse: the shared sticker catalog (`ui/js/desktop/shared/sticker-catalog.js:3`, `window.AuraStickerCatalog`, 100 entries) is offered as an extra "Stickers" tab of raster props. The picker is `window.AuraStickerLibrary.mount` (`ui/js/desktop/shared/sticker-library.js:30`, exported at `:86`).

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

- **Voice IDs.** Voice IDs are validated with `^[A-Za-z0-9._-]{1,128}$`, and against the list where one exists. A pre-existing bug is fixed in the same change: the ElevenLabs URL is built with `fmt.Sprintf` without escaping (`internal/tools/tts.go:485`). It now uses `url.PathEscape`, with a test.
- **Synthesis.** Each narration block is split at sentence ends into pieces no longer than the provider's chunk limit in the table. Each piece is synthesised with `tools.TTSSynthesizeInMemoryContext` (`internal/tools/tts_memory.go:20`). The TTSConfig comes from `BuildTTSConfig(cfg, TTSRouteTool, lang)` (`internal/tools/tts_config.go:22`) and is then overridden for each call: the provider, the provider's voice field from the table, `Language` set explicitly, and `NoProviderFallback = true`.
- **Serialisation.** Phonon is serialised by its process slot (`internal/phonon/synthesize.go:18`). sanoTTS has no serialisation today (`internal/sanotts/sanotts.go:68-79`), so the narrate queue limits it to one synthesis at a time. The narrate queue holds at most 128 queued jobs and returns `busy` when it is full.
- **Storage.** The output goes to the project folder at `audio/narration/<sha256(provider|voice|lang|text)>.<ext>`. Speed is a fixed 1.0 in v1 and is not part of the key or the schema. The audio is never stored in `data/tts`, which has retention cleanup, and it is never served through `/tts/`, which has the ticket rule. The Storymaker API serves it from the project folder.
- **Duration and envelope without FFmpeg.** Durations and envelopes come from WAV through `voice.DecodeWAVPCM16Source` (`internal/voice/pcm.go:100`), and from MP3 through `go-mp3` (`go.mod:39`, used in `internal/server/sip_voice.go`). Supertonic is asked for WAV. ffprobe (`internal/videostudio/probe.go:55-82`) is only a fallback. Narration timing therefore works without FFmpeg.
- **Envelope.** A per-frame RMS envelope is computed once per project fps from the decoded audio. It is stored as `<hash>.<fps>.env.json`. The player maps levels to mouth shapes deterministically: rest at silence; low, mid and high levels to E, A and O, with a seeded variation; and M, B and P closures at short dips. The lip-sync is approximate and works with every TTS.
- **Cues and captions.** JavaScript (`compile.js`, `captions.js`) is the single owner of cue placement, caption line splitting and caption timing. The `{n}` markers and the caption line times are placed in proportion to character position, over the measured duration of their block. Captions are at most 2 lines of 42 characters, and each lasts at least 1.0 s. In 9:16, the safe zones are the top 12 % and the bottom 18 %. Go writes SRT and VTT from `captions[]` of the compile report. `narration.go` supplies only the measured durations per block and per piece.
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
  - SFX come from the beats, at their event times in the compile report.
  - Loudness is a measured linear gain. A two-pass `loudnorm` measurement gives the level to −16 LUFS integrated, and a linear gain applies it, identically to the mix and to every stem (Video Studio handoff). The result is encoded as AAC at 48 kHz (`internal/videostudio/time.go:9`).
- **Preview.** The app preview ducks with WebAudio gain, driven by the stored envelope (pattern: `ui/js/desktop/apps/minigolf-audio-music.js:142-149`).

## Rendering and export

Flow of a render job:

1. **Preconditions.** FFmpeg must be ready (`videostudio.CheckRuntime`, `internal/videostudio/runtime.go:34-89`), a browser must be available, and the story must be valid. Narration must be complete. That condition is vacuous when a story has no narration, so phase 1 renders are silent. The status endpoint reports each capability.
2. **Listener and browser.** Go starts a per-job ephemeral listener and a headless Chrome for the job. Both are closed when the job ends.
   - **Listener.** It binds `127.0.0.1:0`, uses plain HTTP and its own timeouts, and writes no access log of request paths. Every path it serves starts with an unguessable 128-bit prefix from `crypto/rand`, as defence in depth. The **render origin** is `http://127.0.0.1:<port>`, and the **render base** is `<render origin>/<prefix>`. Under the render base it serves only the render page (`<render base>/`), the job's snapshot (story, compile inputs, audio info, part JSON, images, fonts and code-scene bundles, read-only), the player bundle (`<render base>/js/storymaker/player.bundle.js`), and the ingest endpoint. Anything else gets 404. The prefix is never logged and never persisted; `jobs.json` stores only its hash. No `authBypassSubtrees` entry on the public server is needed. The listener is not a public route. AuraGo's own loopback URL is HTTPS on TLS installs, which headless Chrome rejects or autocert refuses (`internal/server/server_internal_http.go:49-65`, `internal/server/tls.go:166-169,226-229`). The main server's 30 s read timeout (`internal/server/server_internal_http.go:19`) and access log (`internal/server/server.go:2410,2448-2451`) do not fit a streamed ingest. The render page also need not be reachable from the LAN or the tunnel.
   - **Snapshot.** The job snapshots story, assets and audio at start, so edits during a render do not change it.
   - **Chromium.** Storymaker uses a configured binary (`storymaker.render.browser_path`) or else a pinned Chromium revision that AuraGo downloads into the data directory. The SHA-256 of that revision for each platform is recorded in code and verified before first launch: download to a temp path, verify, then move into place. Storymaker does not use the rod default mirror race, which races Google storage, npmmirror and playwright mirrors without a checksum (rod `lib/launcher/browser.go:105,128-130`). Production launches never set `NoSandbox`. The Ubuntu 23.10+ AppArmor step stays a documented prerequisite (`documentation/installation.md:271-302`). The `-rod=bin=` flag is implemented by rod (`lib/defaults/defaults.go:113`).
   - **Egress controls.** The enforced controls are these. (1) Chrome runs per job behind `security.StartCaptureProxy` in a new strict mode that dials only the render origin. Today the proxy refuses only private and loopback targets and dials any public host (`internal/security/capture_proxy.go:260-277`, `internal/security/ssrf.go:180-186`). (2) Chrome is launched with `--proxy-bypass-list=<-loopback>`, so loopback also goes through the proxy (precedent: `internal/tools/homepage_annotation_capture.go:124-132`). (3) CDP Fetch interception at request stage, for the page and its auto-attached worker targets, fails every request and navigation whose origin is not the render origin. (4) There are no popups, because the sandbox has no `allow-popups`. A regression test has scene code assign `location`, call `window.open` and call `fetch` to a public host, and asserts that all three fail.
3. **Render page.** Chrome opens `<render base>/`. The listener serves the page with this CSP:

   ```
   sandbox allow-scripts; default-src 'none'; script-src <render base>/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src <render base>/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'
   ```

   The page is also served with `Referrer-Policy: no-referrer` and `Cache-Control: no-store`. The host fetches the snapshot from the render base, which `connect-src` allows, and transfers it to the workers.
4. **Frames.** The page compiles, posts the compile report (see Compile report and job order), and renders frames in order at a fixed step. Frames reach FFmpeg through the transport chosen by the phase 1 spike. The candidates are:
   - (a) the page POSTs encoded frames (PNG or lossless WebP), in order, to the ingest endpoint, which writes to FFmpeg stdin (`-f image2pipe`);
   - (b) `Page.captureScreenshot` per frame, over CDP;
   - (c) a canvas blob returned by `Runtime.evaluate`;
   - (d) raw RGBA from `getImageData`, POSTed to the ingest endpoint and read by FFmpeg as `-f rawvideo -pix_fmt rgba -s WxH -r fps -i pipe:0`.

   The decision criteria are at least 10 rendered frames per second at 1080p on an aurago-test-class CPU, bit-identical repeat renders, and bounded memory. Only raw RGBA over CDP stays excluded, because it is 8.3 MB per frame. If no candidate reaches 10 frames per second, the plan takes the fastest deterministic candidate and records its measured rate, which the render timeout then uses (Limits below).
   - **Ingest hardening.** `http.MaxBytesReader` caps each frame (4 MiB at 1080p PNG; the raw RGBA size for transport (d)). The frame index must equal the next expected index. The maximum frame count is computed in Go from `render.max_duration_seconds` × fps, not taken from the page. The PNG IHDR or WebP header (or the raw size) must match the canvas before the bytes reach FFmpeg stdin. All frames are refused once the job leaves the running state.
5. **Encoding.** FFmpeg takes the frames and the mixed audio WAV, and produces H.264 through the existing encoder selection (`videostudio.SelectEncoder`, `internal/videostudio/encode.go:117-173`, cached like `internal/server/video_studio_encoder.go:14-42`, with software fallback). The audio is AAC, and the container is an MP4 with `+faststart`. Progress comes from the frame count, and from `-progress pipe:1` (pattern: `internal/videostudio/render.go:579-627,677-700`).
   - **Colour.** The frame-input graph converts colour explicitly: `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p`, followed by `Encoder.filterTail()` before the encoder args, as `internal/videostudio/segment_graph.go:154-161` does. The encoder args only tag BT.709 (`internal/videostudio/encode.go:62-93`), and FFmpeg before 7.1 would otherwise convert with BT.601.
   - **Inputs.** Every new FFmpeg input (mix, encode, frame pipe, envelope) uses `-protocol_whitelist file,pipe` and an explicit `-format_whitelist`. The format list is the one in `internal/videostudio/probe.go:16-17`, plus `image2pipe`, `png_pipe`, `webp_pipe` and `rawvideo` as needed.
   - **Hardware fallback.** If the hardware encoder fails in the streamed path, Go sends the host a `restart` message, and the job restarts from frame 0 with libx264, once (pattern: the one-shot fallback at `internal/videostudio/render.go:94-109`).
6. **Outputs.**
   - `Videos/Storymaker/<slug>.mp4`. The slug is the title sanitised by `safeVideoStudioFilename` (`internal/server/video_studio_service.go:1239`) or a successor: ASCII letters, digits and dashes, Windows reserved names refused, at most 80 characters. The project ID is the fallback when the slug is empty. On a collision, `-2`, `-3` and so on are appended. The file is written create-only through `WriteFileStreamConditional`. Example: "Was ist eine Firewall?" becomes `Was-ist-eine-Firewall.mp4`.
   - **Why `Videos/`.** Storymaker writes under `Videos/` so that the MP4 is discoverable where camera clips are imported. The camera import accepts only `Videos/` paths (`ui/js/desktop/apps/video-studio-projects.js:432-447`). The Video Studio handoff imports by `source_path` and does not enforce `Videos/` (`internal/server/video_studio_handlers.go:664-684`), so the plan must not rely on that rule.
   - A copy record in the project folder `exports/`, which keeps the job ID.
   - An optional SRT or VTT file, written from `captions[]` (Compile report and job order).
   - Stems `narration.wav`, `music.wav` and `sfx.wav`. They are always produced when "Open in Video Studio" is chosen, and optional otherwise (Video Studio handoff).
   - Large writes go through `WriteFileStreamConditional` (`internal/desktop/service_file_stream.go:19-31`), not the 50 MB upload path, whose default limit is set in `internal/desktop/service.go:172-173`.
7. **Cancel and persistence.** Cancel kills Chrome and FFmpeg, and closes the listener. Jobs persist in `<DataDir>/storymaker/jobs.json`, and a job that is running at restart is marked interrupted (pattern: `internal/server/video_studio_service.go:367-442`). Progress is streamed over SSE (pattern: the Game Maker broker and SSE, `internal/server/game_maker_handlers.go:358-396`).

Limits: the render timeout is max(`render.timeout_seconds`, 3 × frame_count ÷ the measured spike fps), where `render.timeout_seconds` defaults to 3600 s. The spike records the measured fps, and a 15-minute video must fit. The maximum video is 15 minutes (`render.max_duration_seconds`, default 900). One render runs at a time, through a queue of its own that holds at most 16 queued jobs and returns `busy` when it is full. That queue is separate from the Video Studio and Game Maker queues.

Missing Chromium or FFmpeg: the editor and the preview still work. Export and the agent's layout checks show a clear capability message. Agent checks report the layout as unverified. Narration timing does not need FFmpeg (see Narration).

### Compile report and job order

The player worker compiles the story and produces a compile report. The host sends the report to Go. Only render and check jobs send one. Go trusts reports only from its own headless jobs, never from the app iframe. In the app, the player iframe posts its `compile_report` to the parent only for display (scrubber, scene durations, local warnings); the parent never forwards it to Go.

```
{frame_count, fps, duration_ms,
 scenes: [{id, start_frame, end_frame}],
 audio_events: [{kind: narration|dialogue|sfx, ref: narration hash or sfx id, start_ms, gain}],
 captions: [{start_ms, end_ms, lines[]}],
 checks: [check result], unverified: [check result]}
```

Go validates the report: known narration hashes and SFX IDs, bounds, monotonic times, `frame_count` ≤ `render.max_duration_seconds` × fps, and code-scene durations clamped. Go then uses the report for the mix, the stems, SRT and VTT, the music length and the duration limit.

- **Caption and cue ownership.** JavaScript (`captions.js`, `compile.js`) is the single owner of cue placement, caption line splitting and caption timing. `narration.go` supplies only the measured durations per block and per piece. Go writes SRT and VTT from `captions[]`.
- **Scene-local timing rule.** Scene time 0 is the start of the scene's entry transition. The scene's narration starts when that transition ends (offset = transition duration, 0 for cut). Cue and beat times are scene-local, from the same origin. `auto` duration = entry transition + narration blocks + 0.4 s tail, or the end of the last beat if that is later. Silent scenes have a 1.0 s minimum. Transitions overlap scenes: a scene starts its entry transition's duration (0.5 s by default) before the previous scene ends, so its narration begins after the previous scene's narration and tail. A Node compile test asserts that no two narration or dialogue events overlap.
- **Render job order.** The job starts the listener and Chrome, and the page compiles. The page posts the compile report, and Go validates it. `mix.go` writes the mixed WAV and the stems. FFmpeg starts with the WAV and the frame input. The page steps frames, the frames go to the ingest endpoint, FFmpeg finishes, and the outputs are written.
- **Music length.** Generated music uses `duration_ms` from the latest check-job report. If none exists, a check job runs first. Later length changes are absorbed by a loop (`acrossfade`) or by a trim with fade-out. The music is not regenerated.
- **Phase 1 reports.** A phase 1 render reports only the frame count and the scene ranges. `checks[]` arrives in phase 3.

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
  1. Outline: choose a template (product explainer, how-to, concept explainer, dialogue story, data story or onboarding). Build the arc hook, problem, solution, how, benefit, close. Write the narration per scene with `storymaker_story` `outline`, which returns per-scene word budgets. The dialogue story template uses one scene per speaker turn (Narration, Dialogue).
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
- **Stills.** One scene (at most two) per stills call. The call returns one contact-sheet image at low detail, with the start, middle and end of the scene side by side, about 1024 px wide. Earlier stills are replaced by a text placeholder in the history. The compact profile reviews only the flagged scenes, or the first and last scene when nothing is flagged (pattern: `internal/server/game_maker_service.go:780-787`).
- **Call repair.** `canonicalizeStorymakerOperationCall` sends an operation name that was called as a tool to its owning `storymaker_` tool. It wraps flat scene fields into `data`. It applies only inside a bound Storymaker job, and it never widens scope (pattern: `internal/agent/canonical_action.go:162-170`). A recovery nudge names the current phase's tools and gives one call example (pattern: `internal/server/game_maker_recovery_nudge.go:22-72`).
- **Catalog miss.** When the catalog has no match, the agent tries these in order: (1) the nearest catalog alternative; (2) a text, callout or icon show beat; (3) `create_character` for people; (4) `write_svg`; (5) `generate_image`, only when it is configured. The phase 1 starter props cover office and IT and symbols (lock, key, shield, laptop, phone, cloud, check, cross, warning, idea), so the phase 3 acceptance can run on the starter catalog.

### Job outcome

The job ends when the model replies without a tool call after at least one validate (the server then runs a final validate itself), or when the timeout or the budget runs out. The outcomes are:

- `ready`: no model-fixable issues, and the layout is verified. A story with a code scene can reach at best `ready_unverified`, because layout inside code scenes is always unverified (Player runtime).
- `ready_unverified`: no model-fixable issues. `open_checks` names the unverified or environment items.
- `incomplete`: a timeout or budget stop. Valid scenes are kept, drafts are dropped, and `open_checks` lists the failing scenes.
- `failed`: with an error code (Error handling).

Model prose is released for `ready` and `ready_unverified` (pattern: `internal/gamemaker/job_outcome.go:158-181`). Agent-created length is bounded to 15 to 180 s by default (config `storymaker.agent.max_length_seconds`).

### Studio tools

The studio tools are job-scoped, with the prefix `storymaker_`. Model-facing schemas never carry IDs that the server binds.

- **Tool wiring.** Add `storymaker_` to the private-execution prefix test (`internal/agent/action_policy.go:314`) and to the studio flag (`internal/agent/tool_scope.go:108`). Add `storymaker_*` cases to the dispatch switch (`internal/agent/agent_dispatch_exec.go:226-234`). Add the chat tool case next to retro_games (`internal/agent/dispatch_platform.go:312-313`). The schemas go in `internal/agent/native_tools_storymaker.go`, modelled on `internal/agent/native_tools_game_maker.go:16`. The job's DispatchContext sets `ToolScopeRestricted = true` and `AllowedTools` to exactly the `storymaker_*` tools of the current phase (precedent: `internal/server/game_maker_implementation_stream.go:80`). A test asserts that a general tool name is refused.
- `storymaker_catalog` `{operation: search | describe, query, kind, id}`. The default is 6 results and the cap is 12. It searches the built-in catalog first, then the project (`my.*`) and library (`lib.*`) assets, and voices with `kind=voice`, which return ready voices as `{provider, voice, languages, gender?}` with the active provider first. It returns exact IDs, a usage example and alternatives (pattern: `internal/gamemaker/asset_usage.go:152-299`).
- `storymaker_story` `{operation, scene_id, data, expected_rev}`. The operations take these arguments and give these answers:

| Operation | Arguments | Answer |
| --- | --- | --- |
| `get` | optional `scene_id` | without `scene_id`, a summary: rev, meta, cast, and per scene the id, environment, first 80 narration characters, beat count and open issue codes; with `scene_id`, that scene |
| `outline` | `{template, target_seconds?, scenes:[{id, narration}]}`; only while no scene is filled (otherwise use `put_scene`) | per-scene word budgets, computed as `target_seconds` × 2.3 ÷ scenes |
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
  - **Check result.** Go validation and JS checks return one shape: `{scene_id, path, code, message, at_seconds?, fix}`, where `fix` is one sentence plus an example patch. The result holds at most 8 issues, in scene order. Each code has a fix template. `overlap_face` names the prop instance and the actor, and suggests the nearest free spot. `text_overflow` gives the maximum number of characters. Go's validator also reports the static equivalents (unknown cue marker, empty narration, unknown asset or spot, with alternatives), so they work without a browser.
  - Each answer includes, per scene, `{narration_seconds, cue_times, end_seconds}`, and the total against `target_seconds`, with an `over_target` issue above +15 %.
  - Narration failures, and a missing browser or FFmpeg, are `environment` issues with `model_fixable: false`. They are never charged and never block the outcome.
  - The check is bounded to 60 s. Scenes that are still synthesising are `pending`, with the 2.3 words-per-second estimate, and the model calls again.
  - Layout check codes (in `checks.js`): `off_stage`, `overlap_face` (a prop covers a face), `text_overflow`, `text_too_small`, `caption_long`, `actor_unreachable_spot`, `beat_after_scene_end`, `cue_missing`, `narration_missing`, `asset_missing` and `code_error`. Go's validator reports the static codes as well.
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

- **Toolbar.** Project menu, format, play and pause, time, and export.
- **Left: scene strip.** Thumbnails rendered by the player, the duration of each scene, add scene, and templates.
- **Centre: stage.** Selection handles, drag and drop from the catalog, and snapping to environment spots. Below the stage is a scrubber, with scene blocks, a narration waveform and beat markers. There are no per-object keyframe tracks.
- **Right: inspector and panels.** The inspector covers the scene, the actor, the beat and the voice. A docked agent panel, an Assets panel and a catalog drawer sit alongside it. The catalog drawer has the tabs People, Props, Environments, Text, Transitions, Stickers and Mine, with search.
- **Code tab.** `story.json` and the code scenes, in the vendored CodeMirror. Validation issues are shown inline.
- **Export dialog.** Quality, burn-in captions, SRT and VTT, stems, and "Open in Video Studio".
- **States.** No FFmpeg, no browser, no TTS ready, music not configured, or read-only. Each state has a clear message and lists the parts that still work.
- **Player iframe.** The app runs the player host in a sandboxed iframe with `sandbox="allow-scripts"` only, and no `allow` permissions. The iframe has an opaque origin, with no `allow-same-origin`. Do not reuse the `makeSandboxedFrame` defaults, which add allow-forms, allow-modals and clipboard delegation (`ui/js/desktop/apps/quickconnect-launchpad-chat.js:1733-1744`). The iframe loads from `GET /api/desktop/storymaker/player`, a Go handler with this CSP header:

  ```
  sandbox allow-scripts; default-src 'none'; script-src <origin>/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src <origin>/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'
  ```

  The handler also sends `Referrer-Policy: no-referrer` and `Cache-Control: no-store`. The CSP has no `unsafe-inline` and no `unsafe-eval`. Do not copy `gameMakerPreviewCSP` (defined at `internal/server/game_maker_handlers.go:527`, used at `:534`), which allows `unsafe-inline` and `connect-src 'self'`.
  - The global security middleware sets `X-Frame-Options DENY` and `frame-ancestors 'none'` outside its allow-list (`internal/server/server.go:2233-2249,2275`). `/api/desktop/storymaker/player` is therefore added to `allowDesktopIframe`, as `/api/desktop/bookshelf/reader` is (`internal/server/server.go:2234`).
  - The parent accepts a message only when `event.source === iframe.contentWindow` and the message matches a fixed, typed, size-capped schema.
  - The parent sends the story and the assets only after a `ready` message from that source. It resets state on every iframe `load`, and it never sends tokens, paths or API data.
  - The messages are: ready, load_story, assets, need_assets, seek, play, pause, frame, compile_report and error.
- **Untrusted text.** Story text, narration, names, tags and library names are untrusted. In `ui/js/storymaker` and `ui/js/desktop/apps/storymaker*.js`, they are set with `textContent` or drawn on canvas. No `innerHTML`, `insertAdjacentHTML` or template string with user data is used there. A grep test enforces it.
- **Registration.**
  - A `BuiltinApps` entry in `internal/desktop/types.go:796-862` with `Requires: ["storymaker"]` (Architecture, Enablement; pattern: retro-games at `internal/desktop/types.go:843` and its capability case at `internal/server/bluetooth_live.go:25`). The category is `creative`, like Video Studio (`internal/desktop/types.go:800`). In the player CSP below, `<origin>` is the AuraGo origin.
  - An icon in every icon theme, or `logo_path` artwork, as game-maker-studio does (`internal/desktop/types.go:846`).
  - Module-loader assets and an i18n section: `ui/js/desktop/core/module-loader.js:22-548` (`DESKTOP_APP_ASSETS`) and `:648` (`APP_I18N_SECTIONS`).
  - The render dispatch in `ui/js/desktop/core/menus-and-routing.js:1553-2027` (`renderAppContent`).
  - The global name in `ui/js/desktop/core/desktop-foundation.js:586-651` (`appGlobalName`).
  - The window size and minimum size, and open maximized, in `ui/js/desktop/core/window-shell-runtime.js:1296-1374`.
  - `npm run build:ui` for the shell files and `player.bundle.js` (`package.json:34`), and `npm run check:ui` (`package.json:39`).
  - A registration test, modelled on `ui/desktop_retro_games_registration_test.go:13-78`.
  - A root AGENTS.md child-DOX row, and new child AGENTS.md files.
- **Responsive layout.** Container queries handle narrow windows and phones. The scene strip collapses to a row, and the inspector becomes a sheet.
- **Config.** The `storymaker` section is a `StorymakerConfig` field next to VideoStudio (`internal/config/config_types.go:2194`), in its own file modelled on `internal/config/video_studio.go:10`. It holds: `enabled`; `agent.enabled`; `agent.provider` and `agent.model`; `agent.job_timeout_seconds` (default 1800); `agent.max_concurrent_jobs` (default 1); `agent.max_length_seconds` (default 180); `agent.max_daily_jobs` (default 10); `render.timeout_seconds` (default 3600); `render.max_duration_seconds` (default 900); `render.browser_path`; `narration.max_daily_chars` (default 100000) and `narration.max_job_chars` (default 20000); and `assets.max_daily_generations` (default 20). It appears in the config UI, with labels in all 16 locales.

## Security

- **Realms and CSP.** Code scenes run only in code-scene workers, inside the sandboxed player iframe (in the app) or on the render page (for export). The barriers are the CSP (no `unsafe-eval`; `connect-src 'none'` in the app; `connect-src <render base>/` on the render page), the opaque origin and the worker realm. Shadowing is for determinism and is not a barrier. The player worker is a separate realm, so scene code cannot alter the layout checks.
- **Egress (render page).** Scene code cannot reach any origin other than the render origin. The enforced controls are listed in Rendering and export, step 2, and a browser regression test covers them.
- **Render listener.** The listener is per job, binds only loopback, is closed when the job ends, and has no public route. Its path prefix is 128-bit random, never logged and never persisted; `jobs.json` stores only its hash. Ingest accepts frames only for the running job, in order, and within the limits in Rendering and export, step 4.
- **Browser.** Chromium is verified against its pinned SHA-256 before first launch. Production launches never set `NoSandbox`.
- **Player messages.** The parent accepts messages only from its own iframe and only in the typed schema (User interface, Player iframe). Untrusted text is set as text only (User interface, Untrusted text).
- **Jobs and mutations.** The read-only flag is checked in the job starter, the chat tool and every paid provider call, not only in workspace writes (`internal/desktop/service_files.go:441-443`). Every job queue is capped and returns `busy` when it is full: render 16, narrate 128 and agent 8 queued. A Storymaker job calls only the `storymaker_*` tools of its current phase (Agent, Tool wiring). Budgets apply to image, music and narration calls.
- **Paths.** Paths follow the rooted desktop workspace rules, with no traversal and no symlinks (`internal/desktop/secure_open.go:11,63,133`). Asset IDs never map to arbitrary paths. Revise reads source images through the asset registry, and protected system paths are refused (Asset generator).
- **Output and FFmpeg.** Every new FFmpeg input has protocol and format whitelists, and output names are sanitised (Rendering and export, steps 5 and 6).
- **Voices.** Voice IDs are validated, and the ElevenLabs path is escaped (Narration).
- **SVG and images.** The SVG sanitiser works from an allow-list. Raw SVG is never inserted into the DOM. PNG decoding has size and dimension limits (pattern: `internal/gamemaker/visual.go:70-84`; the pixelmatte limits are 32 Mi pixels and 32 MiB).
- **Secrets.** Secrets never reach the browser. The voices endpoint and the narration both run through the server.
- **Data.** Story text, narration, catalog text and tool results are data, never instructions (pattern: `internal/server/game_maker_prompt_profile.go:18`).

## Error handling

Stable job error codes (pattern: `internal/gamemaker/job_outcome.go:35-48`): `invalid_story`, `narration_failed`, `narration_unavailable`, `narration_incomplete`, `storymaker_disabled`, `music_failed`, `browser_unavailable`, `ffmpeg_unavailable`, `render_failed`, `render_timeout`, `cancelled`, `budget_exceeded`, `image_generation_unavailable`, `busy`, `code_error` and `model_error`. `failed` is the job outcome that carries one of these codes (Agent, Job outcome). `not_supported_yet` is a minor issue code, not a job error. The UI maps each code to a localised message that includes the next action.

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
  - the refusal of a general tool name inside a Storymaker job.
- **JavaScript tests (Node, through `vm`):** compile timing (cues, auto durations, transitions); no two narration or dialogue events overlapping; easing; the lip-sync mapping; caption splitting; the checks; the Video Studio track builder (it replaces the Go handoff test); and the host's postMessage source check.
- **Static guard:** a grep test finds no `innerHTML`, `insertAdjacentHTML` or template string with user data in `ui/js/storymaker` and `ui/js/desktop/apps/storymaker*.js`.
- **Browser egress regression:** scene code assigns `location`, calls `window.open` and calls `fetch` to a public host. All three fail.
- **Determinism test:** the same story rendered twice gives identical frame hashes of the decoded frames.
- **Parity test:** the decoded RGBA of frame n from the app player host and from the render page are identical, at the export canvas size, with device pixel ratio 1, after the readiness barrier, in the same Chromium build.
- **One real export probe:** a short story, with FFmpeg and Chrome. It is gated like the other browser smokes, with `AURAGO_RUN_BROWSER_SMOKE=1`.
- **One browser smoke tour of the app.**
- **Guards:** registration, i18n in all 16 locales, the external-asset guard and the line budget.

## Phases

Each phase gets its own plan, written after the previous phase lands.

| Phase | Delivers |
| --- | --- |
| 1 Foundation | Go model, validation, project and catalog index, and CRUD API; config section (enabled, render.*); player runtime (host, player worker, code-scene worker, fallback) with compile, timeline, easing, transitions, text, rig renderer, props, environments, camera, and the show, hide, enter, exit, walk, move, pose, gesture, emote, look, camera and wait beats; starter catalog (about 4 characters, about 20 props covering office/IT and symbols, about 4 environments, base poses, expressions, mouth shapes, walk and idle cycles, 2 palettes, 3 text styles, 4 transitions) with names in all 16 locales; app shell (scene strip, stage preview and scrubber, code view, minimal scene inspector, simple catalog list to insert items); code-scene API basics; render spike, a silent render job (listener, Chrome provisioning, egress controls, ingest, FFmpeg) and a status endpoint; registration and tests. The compile report is the frame count and the scene ranges. |
| 2 Sound & export | Voices endpoint; narration jobs, cache, envelope and cues; dialogue (say); speech bubbles; captions (burn-in, SRT and VTT); lip-sync; music (generated, file or track); SFX beat and the CC0 set; mix, stems and loudness; export dialog; Video Studio handoff and the `projectId` launch parameter; highlight beat. |
| 3 Agent & asset generator | Job runner, SSE and agent panel; studio tools; prompt profiles (normal and compact); checks.js and checks in headless Chrome; stills; outline templates; call repair; job outcome; asset generator, Assets panel and library; chat tool, manual and registration chain; budgets and caps; config for agent. |
| 4 Catalog expansion | Full v1 counts; chart and diagram blocks (chart beat, `api.chart`, `api.diagram`); remaining text styles, transitions and palettes; cast sheet review; 16-locale names; catalog drawer polish. |
| 5 Editor comfort | Drag and drop staging; inspector forms for all beat kinds; template gallery; undo and redo; shortcuts; phone layout polish. |

Rules: from phase 1 the validator knows the full schema. Beats and fields whose phase has not landed are accepted, ignored by the player, and reported as a minor `not_supported_yet` issue. The narration precondition is vacuous when a story has no narration, so phase 1 renders are silent.

## Out of scope for v1

3D scenes; a per-object keyframe timeline; 4K and 60 fps; a whiteboard or draw-on style; word-level ASR alignment for lip-sync and cues; voice cloning; publishing to video platforms; real-time collaboration; Chromium bundled in the Docker image; and Remotion.

## To verify in the plans

- **Frame transport (phase 1 spike).** The throughput and determinism of ingest POST, `captureScreenshot`, an evaluate blob and raw RGBA POSTed to the loopback ingest. The encode cost of PNG against lossless WebP in Chrome at 1080p. The measured fps feeds the render timeout (Rendering and export).
- **Chromium availability.** The pinned revision and its SHA-256 per platform. The runtime download inside Docker (which needs egress), and on Windows. Repo-built executables have failed to start Chrome with a crashpad access-denied error, and the known workaround is the `-rod=bin=` flag. The AppArmor note for Ubuntu 23.10 and later.
- **Phase 1 gate (player runtime).** Blob workers with OffscreenCanvas inside the opaque-origin sandboxed player iframe, `importScripts` of the absolute player bundle URL under the stated CSP, and FontFace in workers. The fallback is as stated in Player runtime.
- **FFmpeg versions.** FFmpeg 5.1.9 (the production floor) against 8.1.1: the behaviour of `sidechaincompress`, `loudnorm`, `acrossfade` and `image2pipe`.
- **Colour and gain on 5.1.9.** The `image2pipe` and `rawvideo` colour round-trip (swatch check), and the two-pass `loudnorm` linear gain.
- **Audio rates and voices.** The sample rates of sanoTTS and Phonon output. Whether Speech Lab honours a per-call voice. The OpenAI TTS voice list. The length of Lyria output.
- **Font licences.** The licences of the bundled fonts in `ui/fonts` that are used for text styles.
- **Clip semantics.** The `Clip.AudioDetached` semantics for the handoff video clip.
- **Asset-set size.** The current file count and size of the asset set, against `MaxFiles` 20000 and `MaxExpandedBytes` 2 GiB, before the catalog is added.
- **Key matte.** The server-side key matte for generated images, for when no AI matte model is downloaded.
- **Cross-browser preview differences.** How the app preview in Firefox, Safari and with GPU raster differs from the headless render (anti-aliasing and text metrics), against the authoritative headless render.
- **Strict-mode capture proxy.** The strict mode of `security.StartCaptureProxy` and CDP Fetch interception for worker targets, checked by the egress regression test.
