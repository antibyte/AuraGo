# Desktop App Modules - Child DOX Contract

## Purpose

This subtree owns built-in virtual desktop app modules that are loaded lazily by
`ui/js/desktop/core/module-loader.js`.

- `galaxa-*.js` implements Galaxa Deluxe, a modular Canvas 2D arcade shooter
  with procedural audio, biomed progression, parry/super combat, and persistent
  meta-progression.
- `chess*.js` implements Chess, a desktop chess app using `cm-chessboard`,
  `chess.js`, a local Stockfish WebWorker, and the optional AuraGo agent move
  endpoint. Features three opponent modes (Computer, Agent, Local 2P),
  optional chess clocks (3/5/10 min), captured-material tray, material balance
  bar, board-skin selector (green/blue/wood/classic, persisted in
  `aurago.desktop.chess.boardSkin`), move-evaluation panel, hint engine
  (Stockfish), move-history click-to-review with first/prev/next/last scrubber,
  last-move and check highlights, resign/draw confirmation modals with cancel,
  game-over result overlay with win confetti, CSS move/capture/check/thinking
  effects (`prefers-reduced-motion` aware), and Web Audio synthesized
  move/capture/check/castle/promote/game-over sounds. Split across `chess.js`
  (core game loop), `chess-fx.js` (template, effects, audio, skin helpers),
  `chess-engine.js` (Stockfish worker bridge), and `chess-agent.js`
  (AuraGo agent move API client).
- `writer.js` implements the Writer app, a word-processing editor with Quill
  rich-text, auto-save with debounce, dirty-state tracking, word/character/page
  count in a status bar, find & replace overlay with match highlighting, an
  enhanced formatting toolbar (font, size, color, background, alignment,
  blockquote, code-block, image), and agent integration.
- `cheater*.js` implements the Cheater app, a cheat-sheet manager with a
  textarea-based Markdown editor, live preview, Markdown toolbar, command
  palette (spotlight), and attachments side panel.
- `sheets*.js` implements the Sheets app, a spreadsheet editor with formula
  engine, cell formatting, undo/redo, auto-save, search/replace, and multi-sheet
  support. Split across `sheets.js` (core), `sheets-formulas.js` (formula
  engine), `sheets-format.js` (format toolbar), and `sheets-search.js`
  (find/replace overlay).
- `code-studio/*.js` implements Code Studio, a full IDE with file explorer,
  CodeMirror editor, terminal, search, agent chat with SSE streaming, Git
  integration, split editor, and keyboard shortcuts. Split across `core.js`
  (state management, API client, lifecycle, shell), `sidebar.js` (file tree),
  `editor.js` (CodeMirror/textarea), `terminal.js` (xterm.js sessions),
  `search.js` (search-in-files), `agent.js` (agent chat, diff preview),
  `git.js` (Git panel, diff view, commit), `panels.js` (split editor,
  panel management), `shortcuts.js` (keyboard shortcuts, window.CodeStudioApp),
  and `command-palette.js` (separate IIFE).
- `sysworld*.js` implements System World, an immersive Three.js (r128) 3D
  universe that visualizes the live AuraGo system: agent core with memory
  nebula, integration satellites, knowledge-graph constellation, mission ring,
  cron dial, co-agent drones, tool belt, and infrastructure field. Visual
  polish includes multi-layer starfields, floating dust, aurora ribbons, floor
  energy rings, core gyro/halo animation, ambient comets/beams/sparkles (gated
  by Effects toggle), and HUD glass/scanline chrome. Opens maximized via
  `open_maximized` metadata; live data from dashboard/KG/mission REST APIs and
  the shared `AuraSSE` event stream.
- `looper.js` implements Looper, an iterative agent workflow (prepare → plan →
  action → test → exit → optional finish) with presets, context modes,
  pause/resume, incremental status SSE, cost/token meta, and advanced options
  (`finish_context`, `prepare_truncation`, `summarize_iterations`, exit
  confidence, stuck detection). Backend lives in `internal/desktop/looper*.go`
  and `internal/server/looper_service.go` / `desktop_looper_handlers.go`.
  Desktop shell must pass `promptDialog` and `confirmDialog` (no native
  `prompt`/`confirm`). Readonly mode disables start/save/delete/edit.

## Ownership

Owned by this subtree. Backend integration lives in `internal/server/` and app
registration lives in `internal/desktop/types.go`.

## Local Contracts

- Built-in app load order is defined in `ui/js/desktop/core/module-loader.js`.
- Galaxa modules attach to the shared `window.GalaxaCore` (GC) namespace and
  expose `GC.create<Name>(ctx)` factories that augment a per-instance `ctx`
  created via `Object.create(GC)` in `galaxa-deluxe.js`.
- Galaxa load order is defined under the `galaxa-deluxe` entry.
  `galaxa-constants.js` and `galaxa-tweens.js` must load before factory modules.
- New Galaxa constants (biomes, super defs, parry tuning, explosion profiles)
  are added to `galaxa-constants.js`, not duplicated in game logic files.
- Galaxa visible UI strings use `galaxa.*` keys in all
  `ui/lang/desktop/*.json` files and must not rely on inline fallback text.
- Chess exposes `window.ChessApp = { render, dispose }`; every desktop window
  instance must own and clean up its own `chess.js` game, `cm-chessboard`
  board, Stockfish worker, Agent client, event handlers, and pending search
  token state.
- Chess loads `ui/js/vendor/chess-vendor.esm.js` with dynamic `import()` from
  `chess.js`; the lazy loader remains classic-script based.
- Chess engine code must load Stockfish only from
  `/js/vendor/stockfish/stockfish-18-lite-single.js` and browser-side agent
  moves must call `/api/desktop/chess/agent-move`.
- Chess visible UI strings use `desktop.*` keys in all
  `ui/lang/desktop/*.json` files.
- Cheater exposes `window.CheaterApp = { render, dispose, openSheet,
  openCreateModal, formatRelativeShort }`; every desktop window instance owns
  its own save debounce timer, preview debounce timer, polling timer, and
  AbortController for in-flight saves.
- Cheater editor uses a stable `<textarea>` source (NOT `contenteditable`) so
  cursor, selection, and native undo stay intact. Live preview is rendered into
  a separate `.cheater-preview` panel via `window.marked`, sanitized with
  `window.DOMPurify`, and highlighted with `window.hljs`.
- Cheater view modes (`edit`/`split`/`preview`) are persisted per-window in
  `localStorage` under `cheater.viewMode`.
- Cheater toolbar is a separate `cheater-toolbar.js` module exposing
  `window.CheaterToolbar.mount(state, slot)`; toolbar buttons use
  `textarea.setRangeText` to stay caret-safe. Do not inline the toolbar into
  `cheater.js`.
- Cheater visible UI strings use `cheater.*` keys in all
  `ui/lang/desktop/*.json` files.
- Cheater tags are persisted through the `/api/cheatsheets` JSON API and must
  remain part of list normalization, creation, search, and card rendering.
- Cheater attachment uploads use `multipart/form-data` to
  `/api/cheatsheets/{id}/attachments`; client validation stays aligned with
  backend limits: `.txt`/`.md`, 1 MiB upload size, and 25,000 text characters
  per sheet.
- Sheets exposes `window.SheetsApp = { render, dispose }`; every desktop window
  instance owns its own undo/redo stacks, auto-save timer, dirty state, and
  context menu state.
- Sheets formula engine lives in `sheets-formulas.js` and exposes
  `window.SheetsFormulas = { evaluate, tokenize, parseCellRef, cellName,
  columnName, numericCellValue, rangeValues }`.
- Sheets format toolbar lives in `sheets-format.js` and exposes
  `window.SheetsFormat = { renderToolbar, applyFormat, getFormatForCell,
  renderFormatStyles, updateToolbarState }`.
- Sheets search/replace lives in `sheets-search.js` and exposes
  `window.SheetsSearch = { openSearch, closeSearch, findNext, findPrev,
  replace, replaceAll }`.
- Sheets sub-module load order in `module-loader.js` must be: formulas, format,
  search, then sheets.js (core). This is because sheets.js references
  `window.SheetsFormulas` at render time.
- Sheets visible UI strings use `desktop.sheets_*` keys in all
  `ui/lang/desktop/*.json` files.
- Writer exposes `window.WriterApp = { render, dispose }`; every desktop window
  instance owns its own Quill editor, auto-save timer, dirty state flag, and
  search/overlay state. Auto-save debounces at 800 ms via `markDirty()` triggered
  on Quill `text-change` and input events.
- Writer visible UI strings use `desktop.writer_*` keys in all
  `ui/lang/desktop/*.json` files. New keys require translations across all 16
  supported languages.
- Writer search/find uses Quill's `deleteText`/`insertText` in `silent` mode
  with regex-based match detection, formatted highlight via `formatText`
  background, and scroll-to-match via `getBounds`. Highlight cleanup on save
  and close avoids stale formats leaking into saved content.
- Code Studio exposes `window.CodeStudioApp = { render, dispose, state, instances,
  api, loadState, saveState, refreshFiles, openFile, openFileFromDialog,
  saveCurrentFile, uploadFile, downloadFile }`. All non-command-palette modules
  share a single IIFE closure; `core.js` opens the IIFE, `shortcuts.js` closes it.
  Function declarations are hoisted across the entire IIFE scope. All `const`/`let`
  declarations must stay in `core.js` (the first module in the bundle load order).
- Code Studio bundle load order in `scripts/build-ui-bundles.js` must be:
  core.js, sidebar.js, editor.js, terminal.js, search.js, agent.js, git.js,
  panels.js, shortcuts.js, command-palette.js.
- Code Studio visible UI strings use `codeStudio.*` keys in all
  `ui/lang/desktop/*.json` files.
- Code Studio Git commands run via Docker exec in the container workspace (`/workspace`).
  Git API endpoints are in `internal/server/code_studio_handlers.go`.
- System World modules attach to the shared `window.SysWorld` (NS) namespace and
  expose `NS.create<Name>(inst)` factories invoked by the entry; per-window state
  lives in the `instances` Map in `sysworld.js`, which exposes
  `window.SysWorldApp = { render, dispose }`.
- System World load order in `module-loader.js` must be: three.min.js,
  OrbitControls.min.js, sysworld-effects.js (`NS.PALETTE`, `createFx`),
  sysworld-scene.js (`NS.LAYOUT`, `createStage`), sysworld-core.js,
  sysworld-orbit.js, sysworld-graph.js, sysworld-fleet.js, sysworld-hud.js,
  then sysworld.js (entry). Modules read `NS.PALETTE`/`NS.LAYOUT` and call
  sibling factories only lazily inside functions, never at IIFE top level.
- System World visible UI strings use `sysworld.*` keys in all
  `ui/lang/desktop/*.json` files (section registered as `'system-world':
  ['sysworld']` in `APP_I18N_SECTIONS`); the dock/start name uses
  `desktop.app_system_world`.
- System World performance contracts: glow textures are cached in Maps, comet/
  burst/ring effects come from capped recycled pools, no `new THREE.*`
  allocations inside per-frame update paths, and every module's `dispose()`
  frees geometries/materials/textures plus listeners, timers, and SSE handlers.
- System World quality tiers: `low/medium/high/ultra` cycle from the HUD
  quality button (`aurago.desktop.sysworld.quality`). Particle buffers are
  always allocated at ultra capacity; the entry's `applyQuality` applies
  per-tier live levers (star/dust/corona `setDrawRange`, dust/nebula/aurora/
  trail visibility, fx pool caps, renderer pixel ratio, ambient FX rate)
  through each module's `setQuality(tier)` — never rebuild the world on a
  tier switch. Ultra exclusives: electric arcs (pooled in sysworld-effects),
  twinkle starfield, animated aurora flow shaders, and the
  `sysworld-energy-wave` floor shader.
- System World shared fx contracts: `fx.textSprite(text, hex, {opacity, scale})`
  returns a cached-canvas label sprite (never dispose its `.map` texture);
  `fx.selectBeacon/clearBeacon` owns the selection halo (rings, glow, light
  pillar, orbiter sparks); `fx.hoverRing(mesh|null, radius, hex)` is the single
  pooled hover ring. Object labels use textSprite with distance-based opacity
  fading in the owning module's update loop.
- System World selection UX: the entry pins `inst.focused = {mesh, ud, radius}`
  and the HUD `sw-sel-label` chip follows it via per-frame camera projection
  (`positionSelLabel`); `graph.highlightNeighbors(id|null)` boosts KG
  neighborhoods on hover; legend zone items emit `onZoneHover`/`onZoneFocus`
  (pulse + camera flight via `zoneAnchor`); arrow keys cycle pickables,
  O/G/E mirror the HUD buttons, idle >45s enables OrbitControls autoRotate.
- System World camera follow: focusing an object also sets
  `inst.follow = {mesh, pending}`; `updateFollowTarget` runs per frame and,
  once the focus flight re-enables the controls, glues `controls.target` to
  the object's live position and translates the camera by the same (clamped)
  delta so orbit/zoom keep working around the moving target. Pan is disabled
  only while a moving object is chased. `clearFollow` runs from `clearFocus`,
  zone-focus flights and the O key and always restores `enablePan`.

## Work Guidance

- Files exceeding 1100 lines must be added to `knownOversizedContinuations` in
  `ui/desktop_js_line_budget_test.go`; use the map there as the current
  source of truth for oversized continuation files.
- Performance-sensitive Galaxa rendering respects the `ctx.settings.particles`
  setting (`low`/`medium`/`high`); particle/trail caps must scale accordingly.
- Galaxa audio uses Web Audio API synthesis only (no sample files). New SFX
  must check `ctx.G.muted` and respect `ctx.G.vol`.
- Galaxa canvas resource caches (`cachedRadialGradient`, `spriteAtlasCache`,
  `ensureNebulaCanvas`) must be reused; see
  `ui/desktop_runtime_performance_test.go` for enforced markers.
- Keep Chess split across `chess.js`, `chess-fx.js`, `chess-engine.js`, and
  `chess-agent.js`; do not fold worker, API bridge, template, or FX/audio
  helpers into the main app file. `chess-fx.js` must load before `chess.js`.
- Keep Cheater split across `cheater.js`, `cheater-toolbar.js`,
  `cheater-spotlight.js`, `cheater-templates.js`, and `cheater-attachments.js`;
  do not fold the toolbar, spotlight, or attachment logic into the main app
  file.
- Keep Sheets split across `sheets.js`, `sheets-formulas.js`,
  `sheets-format.js`, and `sheets-search.js`; do not fold the formula engine,
  format toolbar, or search/replace logic into the main app file.
- Keep Code Studio split across `core.js`, `sidebar.js`, `editor.js`,
  `terminal.js`, `search.js`, `agent.js`, `git.js`, `panels.js`, `shortcuts.js`,
  and `command-palette.js`; do not fold domain modules into core.js.
- Keep System World split across `sysworld.js`, `sysworld-effects.js`,
  `sysworld-scene.js`, `sysworld-core.js`, `sysworld-orbit.js`,
  `sysworld-graph.js`, `sysworld-fleet.js`, and `sysworld-hud.js`; do not fold
  the effects, scene, district, or HUD logic into the entry file.
- Keep OpenSCAD split across `openscad.js`, `openscad-editor.js`, and
  `openscad-defines.js`; do not fold the CodeMirror editor or defines slider
  logic into the main app file.
- OpenSCAD exposes `window.OpenSCADApp = { render, dispose }`. Every window
  instance owns its draft timer, SSE listeners, editor, and preview resources.
- OpenSCAD drafts persist per `windowId` under
  `aurago.desktop.openscad.draft.<windowId>`.
- OpenSCAD result events must filter on `window_id` when present; without it,
  idle multi-window instances must ignore global `openscad_result` events.
- OpenSCAD readonly mode disables CodeMirror/`textarea` editing, defines
  inputs, and the agent prompt.
- OpenSCAD visible UI strings use `desktop.openscad.*` keys in all
  `ui/lang/desktop/*.json` files.
- Keep Writer self-contained in `writer.js` below the 1100-line budget;
  if find/replace grows unwieldy, extract into `writer-search.js` and register
  in `module-loader.js` and `DESKTOP_APP_ASSETS`.
- New formula functions must be added to `sheets-formulas.js` and kept in sync
  with the Go evaluator in `internal/office/` (see `EvaluateFormulaForSheet`).
- Rebuild chess vendor assets with `npm run build:chess-vendor` after changing
  vendored chess package versions or copied Stockfish assets.

## Verification

- `go test ./ui/ -run TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`
- `go test ./ui/ -run TestGalaxaDeluxeCachesCanvasResources`
- `go test ./ui/ -run TestVirtualDesktopJSUsesSemanticChunkNames`
- `go test ./ui/ -run "TestDesktopChess|TestDesktopAppsExposeDisposeLifecycle|TestDesktopAppAssetsRegistry"`
- `go test ./ui/ -run "TestDesktopCheater"`
- `go test ./ui/ -run "TestDesktopSheets"`
- `go test ./ui/ -run TestDesktopAppAssetsRegistry`
- `go test ./ui/ -run TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`
- `go build ./cmd/aurago`

## Child DOX Index

- `galaxa-fx.js` - Supplementary Galaxa visual-effects package: chromatic boss
  shockwave rings, warp speed-line streaks, powerup sparkle bursts + rising
  glints, directional bullet-impact spark cones, combo screen-edge pulses, and
  ship afterimage ghosts. Attaches `ctx.fxBossShockwave()`, `ctx.fxWarpStart()`,
  `ctx.fxPowerupSparkle()`, `ctx.fxSparkCone()`, `ctx.fxComboPulse()`,
  `ctx.updateFX(dt)` and `ctx.fxDraw{Back,Mid,Ghosts,Overlay}(c)` via
  `GC.createFx(ctx)`; caps scale with `ctx.settings.particles` via `GC.FX_CAPS`.
  No child DOX file needed.
- `writer.js` - Word-processing editor: Quill rich-text, auto-save with 800 ms
  debounce, dirty-state tracking, word/character/page status bar, find &
  replace overlay with match highlighting, enhanced formatting toolbar (font,
  size, color, background, alignment, blockquote, code-block, image), and agent
  integration. Exposes `window.WriterApp`. No child DOX file needed.
- `galaxa-demo.js` - AI pilot and demo lifecycle; reactive combat AI (aim, fire,
  dodge, collect powerups), menu auto-tap for shop/evo, and game-over
  auto-restart loop. Attaches `ctx.startDemo()` and `ctx.updateDemo(dt)` via
  `GC.createDemo(ctx)`. Uses the `ctx.G.ai` input source merged in
  `galaxa-game.js` when `ctx.G.demoMode` is true. No child DOX file needed.
- `cheater.js` - Cheater app entry: library, editor, create modal, auto-save,
  polling, view-mode toggle. Exposes `window.CheaterApp`. Editor uses a stable
  `<textarea>` source and renders a separate live preview via marked,
  DOMPurify, and hljs. No child DOX file needed.
- `cheater-toolbar.js` - Markdown formatting toolbar (bold, italic, code,
  link, heading, lists, quote, divider) plus shortcut help modal. Mounts into
  the editor toolbar slot via `window.CheaterToolbar.mount(state, slot)`. No
  child DOX file needed.
- `cheater-spotlight.js` - Command-palette overlay with fuzzy search, keyboard
  navigation, delete confirmation, and create-from-query fallback. No child DOX
  file needed.
- `cheater-templates.js` - New-sheet templates (empty, deployment, debug,
  routine, API, backup) returning localized names via `cheater.template.*`
  keys. No child DOX file needed.
- `cheater-attachments.js` - Attachment upload/delete side panel with
  drag-and-drop, multipart `.txt`/`.md` uploads, backend-aligned 1 MiB and
  25,000-character validation, and 5-second undo. No child DOX file needed.
- `calculator.js` implements the Calculator app, a three-mode calculator
  (standard, scientific, programmer) with expression tokenizer/parser, context
  menu for clipboard operations, and window cleanup. Loaded lazily by
  `module-loader.js` as a standalone app. Exposes `window.CalculatorApp`.
- `settings.js` implements the Settings app, a virtual desktop configuration
  panel with sidebar navigation, global search, hamburger menu on mobile,
  and full desktop shell re-render on changes (icons, widgets, start menu,
  start button). Loaded lazily by `module-loader.js`. Exposes
  `window.SettingsApp`.
- `network-cameras.js` implements the Network Cameras app with a bounded
  snapshot grid, one selected live viewer, an optional four-stream live grid,
  administrator-only ONVIF/manual setup and stream management, and cleanup on
  minimize or close. It must use AuraGo viewer/thumbnail APIs only, must never
  receive or persist camera credentials, and stores only non-sensitive grid
  mode and selected-stream preferences. Loaded lazily by `module-loader.js` and
  exposes `window.NetworkCamerasApp { render, dispose }`. An empty visible-ID
  set means no thumbnail requests; the no-`IntersectionObserver` fallback must
  explicitly mark all cards visible, and focus mode must stop grid polling.
  HTTP 202 mutation responses are saved partial successes: close the dialog,
  refresh state, and show the localized reconciliation warning. True save
  failures keep retryable manual sources and setup tokens in window memory.
- `noisemaker.js` implements the Noisemaker app, a Suno-style AI music studio:
  create view (song idea, style with suggestion chips, optional lyrics/title,
  AI enhancement buttons, optional AI cover), generation progress with elapsed
  timer, result card, library grid, and bottom player bar. It must call only
  `/api/desktop/noisemaker/*` endpoints and never receives provider credentials;
  capability gating (music disabled, no LLM, no cover AI, lyrics unsupported)
  is driven by `/api/desktop/noisemaker/state`, and a disabled integration
  renders the onboarding card instead of the studio. Exposes
  `window.NoisemakerApp { render, dispose }`; every window instance owns its
  enhance/generate AbortControllers, the elapsed timer, form preferences under
  `aurago.desktop.noisemaker.prefs`, and its NoisemakerLibrary instance.
- `noisemaker-library.js` implements the library grid plus bottom player bar as
  `window.NoisemakerLibrary.create(deps)` (factory pattern like
  CheaterToolbar). It loads before `noisemaker.js` in `module-loader.js`, owns
  exactly one `<audio>` element per window instance, and its `dispose()` stops
  playback and detaches all listeners.
- Noisemaker visible UI strings use `desktop.noisemaker_*` keys plus
  `desktop.app_noisemaker` in all `ui/lang/desktop/*.json` files.
- `editor-filemenu.js` implements file management helpers and the inline text
  editor with window menus (file, edit, agent, help). Bundled in the main shell
  bundle (`desktopMainParts` in `build-ui-bundles.js`) because it is referenced
  directly by the desktop foundation runtime.
- `sheets-formulas.js` - Formula engine: tokenizer, recursive-descent parser,
  cell/range evaluation, extended functions (IF, VLOOKUP, CONCAT, DATE, string
  functions, etc.). Exposes `window.SheetsFormulas`. No child DOX file needed.
- `sheets-format.js` - Format toolbar: bold/italic/underline toggles, color
  pickers, alignment buttons, number format dropdown, border dropdown. Exposes
  `window.SheetsFormat`. No child DOX file needed.
- `sheets-search.js` - Search/replace overlay: find next/prev, match case,
  replace current, replace all, match highlighting. Exposes
  `window.SheetsSearch`. No child DOX file needed.
- `code-studio/core.js` - Code Studio core: state management, API client, path
  utilities, lifecycle (render/dispose), shell markup, toolbar, tabs, breadcrumbs,
  status bar, file operations, window menus. Opens the shared IIFE. No child DOX
  file needed.
- `code-studio/sidebar.js` - File explorer: tree view, expand/collapse, drag &
  drop upload, file actions (rename/delete/download), activity bar. No child DOX
  file needed.
- `code-studio/editor.js` - CodeMirror and textarea editors, syntax highlighting
  integration. No child DOX file needed.
- `code-studio/terminal.js` - Terminal sessions with xterm.js, WebSocket
  connection, multi-session management. No child DOX file needed.
- `code-studio/search.js` - Search-in-files panel with grep, result navigation.
  No child DOX file needed.
- `code-studio/agent.js` - Agent chat panel, SSE streaming, diff preview,
  code actions (explain/comments/tests/refactor), markdown rendering. No child
  DOX file needed.
- `code-studio/git.js` - Git panel: branch display, change list, diff view,
  commit dialog, recent log. No child DOX file needed.
- `code-studio/panels.js` - Split editor (horizontal/vertical), resizable
  divider, panel pinning. No child DOX file needed.
- `code-studio/shortcuts.js` - Keyboard shortcuts, shortcut overlay, exposed
  API, `window.CodeStudioApp` assignment. Closes the shared IIFE. No child DOX
  file needed.
- `code-studio/command-palette.js` - Command palette overlay with fuzzy search,
  keyboard navigation. Separate IIFE. No child DOX file needed.
- `sysworld.js` - System World entry: per-window `instances` Map,
  `render(container, windowId, context)` / `dispose(windowId)`, data polling
  (dashboard overview/memory/activity, missions, tool-stats, containers,
  daemons, KG nodes/edges, personality, budget), `AuraSSE` subscriptions,
  RAF loop, pointer interaction (hover tooltip + hover ring, click fly-to +
  info panel, dblclick empty resets view), selection label projection
  (`inst.focused` + `updateSelLabel`), zone anchors (`zoneAnchor`), arrow-key
  cycling (`cycleFocus`), idle autoRotate, WebGL fallback. Exposes
  `window.SysWorldApp`. No child DOX file needed.
- `sysworld-effects.js` - `NS.PALETTE`, cached glow textures, `textSprite`
  label factory (cached canvas textures), pooled comets/bursts/pulse rings,
  drone trails, selection beacon (rings + light pillar + orbiter sparks),
  pooled hover ring, mini tween runner (`createFx`). No child DOX file needed.
- `sysworld-scene.js` - `NS.LAYOUT`, renderer/scene/camera/OrbitControls,
  starfield layers, grid floor, `flyTo`/`resetView`/`introFlight`, raycast
  helper, per-frame resize check (`createStage`). No child DOX file needed.
- `sysworld-core.js` - Agent core: fresnel ShaderMaterial sun, icosahedron
  lattice, gyro rings, corona, memory nebula (vectordb/core facts/journal),
  mood/metrics/busy reactivity (`createCore`). No child DOX file needed.
- `sysworld-orbit.js` - Integration satellites on 3 inclined rings with
  category clustering, per-category geometry identities, inner core +
  wireframe shells, distance-faded textSprite labels, spawn-in stagger,
  enable beams, diff-driven updates (`createOrbit`). No child DOX file needed.
- `sysworld-graph.js` - Knowledge-graph constellation: one-shot 3D force
  layout, core+shell node meshes, protected-node gold rings, synapse comet
  pulses, `highlightNeighbors` hover boost, expand-on-click, visibility
  toggle (`createGraph`). No child DOX file needed.
- `sysworld-fleet.js` - Mission ring, cron dial, co-agent drones with trails,
  tool belt with `flashTool`, container/daemon infrastructure field
  (`createFleet`). No child DOX file needed.
- `sysworld-hud.js` - HTML overlay: stats card, action buttons, interactive
  legend (zone hover/click), live event feed, tooltip, selection label chip
  (`sw-sel-label`), slide-in info panel with badges/tone pills/sections/
  bars/relations (`createHud`). No child DOX file needed.
- `openscad-editor.js` - CodeMirror editor integration for SCAD source with
  syntax highlighting (using javascript()), error line highlighting, and
  fallback textarea. Exposes `window.OpenSCADEditor { create, parse }`. The
  `parse` function extracts line-numbered errors from OpenSCAD stderr output.
  No child DOX file needed.
- `openscad-defines.js` - Parametric define slider panel: parses name=value
  pairs, renders numeric values as range sliders with number inputs, and text
  values as plain inputs. Exposes `window.OpenSCADDefines { parse, render, toText }`.
  No child DOX file needed.
- `noisemaker.js` - Noisemaker app entry: capability state, create view with AI
  enhancement helpers, synchronous generation flow with progress/result/error
  slots, onboarding for unconfigured music generation, tab shell. Exposes
  `window.NoisemakerApp`. No child DOX file needed.
- `noisemaker-library.js` - Noisemaker library grid and bottom player bar
  (search, cards, template/download/delete actions, seek/volume, prev/next).
  Exposes `window.NoisemakerLibrary { create }`; loads before `noisemaker.js`.
  No child DOX file needed.
