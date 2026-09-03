# Desktop App Modules - Child DOX Contract

## Purpose

This subtree owns built-in virtual desktop app modules that are loaded lazily by
`ui/js/desktop/core/module-loader.js`.

Shell chrome helpers live in the main desktop bundle (not lazy apps):
`core/session-runtime.js` (session restore, dock pins, recent files, default
apps), `core/spaces-runtime.js` (three virtual desktops / Spaces v1: window
`spaceId`, hide-without-dispose, session snapshot v2, taskbar pager, Ctrl+Alt
arrows; disabled on compact viewport), `core/shell-chrome-runtime.js`
(notification center, clock popup, hold window switcher, shortcuts overlay),
and `core/spotlight-runtime.js` (Ctrl+K mixed search). Styles:
`ui/css/desktop-chrome.css` (bundled into `desktop-shell.bundle.css`).
Persisted keys: `windows.restore_session`, `appearance.dock_pins`,
`session.windows` (snapshot v2 with `activeSpaceId` and per-window `spaceId`),
and `files.default_apps` via `/api/desktop/settings`.

### Spaces v1 contract

- Exactly three spaces (`1`, `2`, `3`); no create/delete in v1.
- Normal windows get `spaceId` from the active space at open time; session
  restore reads stored `spaceId` (fallback space `1`).
- Space switch hides other windows (`vd-space-hidden` / `data-space-hidden`) but
  does not dispose them; minimize and space-hide stay independent.
- Desktop icons, widgets, wallpaper, and gadgets stay global across spaces.
- Taskbar/dock pins stay global; running window buttons and Ctrl+Tab switcher
  list only the active space.
- `findExistingAppWindow` prefers the current space; a match in another space
  triggers `switchSpace` then focus (no duplicate window).
- Compact viewport (`isCompactViewport()`) keeps single-space behavior and hides
  the pager.

### Spaces overview contract

- UI label is **Flächenübersicht / Spaces overview** — never reuse the Mission
  Control app name.
- Exactly three columns (`1`, `2`, `3`); overview does not create spaces.
- Window cards reuse `windowPreviewMarkup()` from the taskbar thumbnail helper;
  minimized windows stay visible as dimmed cards.
- Click column background switches space; click card switches, focuses, and
  closes; drag card to another column calls `moveWindowToSpace()` and re-renders.
- Shortcuts: `Ctrl+Alt+ArrowUp` and `F3` toggle; `Ctrl+Alt+ArrowDown` closes;
  `Ctrl+Alt+ArrowLeft/Right` keep cycling spaces.
- Pager: short click switches; ~400ms hold or second click on the active space
  opens overview.
- Show Desktop uses `toggleShowDesktop()` peek/restore for visible windows on the
  active space only; peek set clears on focus or `openApp()`.
- Compact viewport disables overview and keeps legacy single-window Show Desktop.
- Snap left/right entries in the window context menu call `applyWindowSnap()`.

### App theme bridge contract

- Everyday apps Writer/Sheets, Todo/Calendar, Settings, Calculator, Chat, File
  Manager, Quick Connect, Mission Control, Software Store, Gallery, Notes,
  Viewer, Looper, and Cheater read `--vd-theme-*` for chrome, panels, controls,
  borders, and shadows.
- Writer page content may stay on a light paper surface (`--vd-editor-page-bg`);
  toolbars, status bars, and sheet chrome follow the active desktop theme.
- Chat user bubbles may keep an accent wash (`--vd-theme-accent-soft`); agent
  bubbles use theme panel material instead of fixed white glass.
- Quick Connect xterm/VNC viewports and the built-in Terminal screen may stay
  on a dark terminal surface (`#0d1117` / `#0f172a`); toolbars follow theme.
- Gallery preview letterboxes (`.vd-gallery-preview`) may stay dark for media
  contrast; card chrome and media preview bars follow theme.
- Cheater code blocks keep a dark readable code surface (`--cheater-code-bg` /
  `--cheater-code-fg`); app chrome uses `--vd-theme-*` through `--cheater-*`.
- OpenSCAD (`--oscad-*`) and Webamp launcher styling remain excluded from this
  bridge; migrate other apps in later polish waves.

### Desktop Phase 3 contract

- Standard-theme taskbar window buttons show a hover/focus thumbnail preview
  (`.vd-taskbar-thumbnail`) for windows on the active space only; Fruity dock,
  compact viewport, and coarse-pointer layouts disable previews.
- Thumbnails clone DOM window content when possible; iframe-heavy windows show
  a live-window fallback instead of a blank capture.
- Media keys and `navigator.mediaSession` route to the active Webamp music
  player only while `state.webampMusic` is alive; no global OS volume control.

- `galaxa-*.js` implements Galaxa Deluxe, a modular Canvas 2D arcade shooter
  with procedural audio, biomed progression, parry/super combat, and persistent
  meta-progression.
- `log-viewer*.js` implements Log Viewer, a first-party desktop app for
  browsing and tailing AuraGo log files. Load order is
  `log-viewer-filters.js` then `log-viewer.js`. Exposes
  `window.LogViewerApp = { render, dispose }` and
  `window.LogViewerFilters = { create }`. Every window owns its
  `EventSource` (or 2s tail poll fallback), ring buffer, keyboard
  handlers, and timers and must close them in `dispose`. Styles are
  scoped under `.vd-logviewer`. Visible strings use
  `desktop.app_log_viewer` plus `desktop.log_viewer_*` in all 16
  `ui/lang/desktop/*.json` files. Download is hidden when the desktop
  is readonly; the backend also returns HTTP 403 for
  `/api/desktop/logs/download` in that mode.
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
- `calendar.js` implements the Calendar renderer, appointment menus, drag/drop,
  recurring appointment creation, and modal lifecycle. It is a continuation
  inside the shared Desktop IIFE and is bundled immediately before
  `core/sdk-events-bootstrap.js`; it is not loaded lazily.
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
- `game-maker-studio-api.js`, `game-maker-studio-preview.js`,
  `game-maker-studio-modals.js`, and `game-maker-studio.js` implement Game
  Maker Studio: a project library, 2D/3D creation dialog with idea chips,
  bounded agent progress (job banner with phase, elapsed time, and repair-pass
  indicator plus a phase stepper), result cards for ready/failed jobs,
  revision history, change requests, and a live game preview in one maximized
  desktop window.
- `live-speech.js` mounts the shared realtime-speech panel on the desktop in a
  compact single-column window (preset 440×600, min 340×460 in
  `window-shell-runtime.js`; panel mounted with `compact: true`).
  OpenAI, xAI, and Gemini stay on their existing streaming adapters. Speech
  Lab is a keyless `local_s2s` profile that transcribes and speaks through
  the managed or external s2s container. The app shows `/api/speech-lab/status`
  and may start a managed container through `/api/speech-lab/deployment/start`.
  `live-speech-fx.js` loads before `live-speech.js` and renders the
  audio-reactive background canvas (`window.LiveSpeechFX.create`, per-window
  instance with `setEnabled`/`dispose`, FX toggle persisted under
  `aurago.desktop.livespeech.fx`). Reactivity comes from the runtime `level`
  event (mic RMS emitted by `RealtimeAudioGate`) and the optional adapter
  output taps `getOutputLevel()` / `getOutputSpectrum()` (PCMPlayer for
  Gemini/xAI, zero-gain MediaStream tap for OpenAI, MediaElement tap for
  Speech Lab); the FX must keep its pooled particles, DPR cap, and
  `prefers-reduced-motion` static fallback.
- `sip-phone.js` implements the Phone app, an iPhone-inspired SIP softphone
  rendered as a realistic device (brushed titanium frame, separate mute/
  volume/power hardware buttons, glossy Dynamic Island, live status-bar
  clock, signal/battery indicators, glass screen glare, aurora mesh
  wallpaper) on an ambient stage with a light halo and floor shadow. The
  screen hosts five tab views (Favorites, Recents, Contacts, Keypad, Settings)
  above a glass tab bar, plus a full-screen active-call takeover with
  contact-hue avatars and incoming-call answer/decline actions. The Contacts
  tab lists AuraGo address book entries (`/api/contacts`) that carry a phone or
  mobile number, one tap-to-dial row per number with a client-side search
  filter. The app can also run
  windowless as a floating desktop gadget (see `sip-phone-gadget-runtime.js`
  in the Child DOX Index).
- `pixel*.js` implements Pixel, a canvas image editor: left tool rail (20
  tools in 5 groups, incl. magic wand, lasso, move, clone stamp, mask
  selections, gradient, airbrush, dodge/burn and blur brush), contextual
  options bar, adjustments, a 29-filter gallery in 4 categories with
  favorites and before/after compare, plus colors, transform, layers (max
  20), click-to-jump history and AI generate/enhance/remove-bg/upscale
  panels. Split across `pixel-state.js` (constants, tool SVGs/groups,
  canvas pool, MAX_LAYERS), `pixel-view.js` (rail/panel markup),
  `pixel-canvas.js` (canvas, history stack, zoom, adjustments,
  crop/resize/rotate, expandCanvasToFit for oversized AI layers), `pixel-tools.js`
  (tools, selections, floating move, clone-stamp stroke snapshot, layers, history
  panel), `pixel-actions.js` (file I/O, AI calls, photos), `pixel-filters.js`
  (filter catalog, gallery, non-destructive preview), `pixel-events.js`
  (mouse handlers, shortcuts modal, context menu, option wiring) and
  `pixel.js` (shell, runtime, event wiring).

## Ownership

Owned by this subtree. Backend integration lives in `internal/server/` and app
registration lives in `internal/desktop/types.go`.

## Local Contracts

- Built-in app load order is defined in `ui/js/desktop/core/module-loader.js`.
- `calendar.js` is the Calendar source of truth. `desktopMainParts` in
  `scripts/build-ui-bundles.js` must place it after the split app continuations
  and before `core/sdk-events-bootstrap.js` so `renderCalendar` stays inside
  the shared Desktop runtime closure without duplication.
- Game Maker Studio loads in the order `game-maker-studio-api.js`,
  `game-maker-studio-preview.js` (`window.GameMakerStudioPreview`: loading
  overlay, stale badge, fullscreen, new-tab), `game-maker-studio-modals.js`
  (`window.GameMakerStudioModals`: skills and revisions modals; the modal
  framework and security-checked helpers stay in the main module), then
  `game-maker-studio.js`.
- Game Maker Studio exposes `window.GameMakerStudioApp = { render, dispose,
  instances }`. Every window owns and closes its EventSource, preview iframe,
  channel ID, diagnostics, modal handlers, job-elapsed and busy-poll timers,
  document-level overflow-menu listeners, and `message` listener.
- Game Maker previews must use `sandbox="allow-scripts"` without
  `allow-same-origin` (`allowfullscreen` on the iframe is permitted).
  Accept diagnostics only from the instance iframe when `event.source`, the
  random channel ID, the fixed source marker, and the bounded event type all
  match. The channel is read-only.
- Because the preview sandbox is opaque, game diagnostics must
  `postMessage(..., "*")` (never `location.origin`, which is the string
  `"null"`). The parent still validates source/channel/`event.source`.
  Arm the loading overlay before assigning `iframe.src`, clear it on
  `ready` or iframe `load`, and rely on the server-injected preview boot
  script to hide leftover in-game `Loading…` HUD pills for agent-rewritten
  `index.html` files.
- Game Maker job progress UX: the job banner, phase stepper, and result
  cards are driven by the project-scoped SSE stream; `capabilities.active_job`
  marks the single globally running job (library spinner, disabled change
  form plus hint in other projects) and is polled only while another project
  is busy.
- Game Maker visible strings use `game_maker.*` plus
  `desktop.app_game_maker_studio` in all 16 `ui/lang/desktop/*.json` files.
  Destructive actions use shell-provided dialogs and never native browser
  dialogs.
- Galaxa modules attach to the shared `window.GalaxaCore` (GC) namespace and
  expose `GC.create<Name>(ctx)` factories that augment a per-instance `ctx`
  created via `Object.create(GC)` in `galaxa-deluxe.js`.
- Galaxa load order is defined under the `galaxa-deluxe` entry.
  `galaxa-constants.js` and `galaxa-tweens.js` must load before factory modules.
  Split modules (soft budget **≤1000 lines** per file): `galaxa-entities-{core,
  spawning, behaviors, combat, weapons}.js`, `galaxa-render-{effects, stage, hud,
  world}.js`, `galaxa-audio-{core, sfx, music}.js`, `galaxa-enemy-motion.js`.
  Orchestrator glue stays in `galaxa-entities.js`, `galaxa-render.js`, and
  `galaxa-audio.js`.
- Enemy movement visuals live in `galaxa-enemy-motion.js` with per-type presets
  in `GC.ENEMY_MOTION_FX` (pulse scale, dive trails, blink invisibility).
- Weapon power-ups (rare/legendary): `rocket_launcher` / `mega_rocket` (homing
  salvo), `mine_layer` / `mega_mine_layer` (player mines via `ctx.pushPlayerMine`,
  cap `GC.PLAYER_MINE_MAX`), and instant `megabomb`. Mirror shots use
  `ctx.mirrorDuplicateBullets(fromIdx)` for rockets and normal fire. SFX:
  `rocketLaunch`, `rocketHit`, `mineDrop`, `mineExplode`, `megabomb`.
  `collectPU` tracks every pickup in `collectedPU` at entry for `power_collector`.
- Game mode logic lives in `galaxa-modes.js` (`GC.createModes(ctx)`): `gauntlet`
  (12 curated waves, no shop, 3 lives), `hyperdrive` (endless speed ramp +
  rotating modifiers; spawning/HP scaling matches `endless`), `mirror`
  (permanent horizontal mirror + 50% ghost damage; delayed enemy bullets mirror
  on spawn). Export `ctx.modesRestoreTimeScale()` for hitstop/slow-mo/continue/
  bullet-time recovery. Settings `mode` cycle plays `modeSelect` once via
  `GC.applySettingsInput`; daily challenge remains title hotkey `D`.
- Adaptive biome layers in `galaxa-adaptive-music.js` attach to the active base
  theme from `ctx.modesGetBaseMusicTheme`, not hardcoded `gameplay`.
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
- SIP Phone exposes `window.SipPhoneApp = { render, dispose }`; every window
  instance owns its runtime subscription, 1-second clock/duration timer, tab
  state (`keypad`/`favorites`/`recents`/`contacts`/`settings`), address-book
  search state, long-press `0`→`+`
  handling, and its Web Audio keypad-tone context (DTMF hold-to-play
  feedback, closed on dispose). All call media flows through
  `window.SipPhoneRuntime`; the app
  must keep the `data-sip-phone*` hooks, observer-disabled controls, and the
  always-enabled hangup control asserted in `ui/desktop_sip_phone_test.go`,
  and must not introduce voicemail/mailbox UI.
- The floating phone gadget (`ui/js/desktop/core/sip-phone-gadget-runtime.js`,
  part of the desktop main bundle) mounts the same app windowless on a
  fixed, draggable layer inside `#vd-workspace` (sibling of
  `#vd-window-layer`, so its z-index is validly compared against
  `--vd-z-window`) under the reserved instance id
  `sip-phone-gadget`. It exposes `window.SipPhoneGadget = { init, sync }`,
  renders the app into a 400×830 stage scaled via
  `.vd-sip-phone-gadget-scale`, drags only from the status bar / Dynamic
  Island / hardware buttons / device frame, and persists
  `phone_gadget.enabled|position_x|position_y|always_on_top` through
  `/api/desktop/settings`. `applyDesktopSettings()` re-syncs the gadget, the
  Settings app owns the `phone_gadget.enabled` toggle, and the gadget
  context menu offers open-in-window, always-on-top, and remove. The layer
  hides below 640 px viewport width.
- SIP Phone visible UI strings use `desktop.sip_phone_*` keys in all 16
  `ui/lang/sip_phone/*.json` files (identical key sets); the dock/start name
  uses `desktop.app_sip_phone` in `ui/lang/desktop/*.json`.
- Pixel load order in `module-loader.js` must be: state, view, canvas, tools,
  actions, filters, events, pixel.js. Modules attach via
  `Pixel.install<Domain>(runtime)` with `bindRuntime` (no eval); shared state
  lives in the getter/setter runtime object created by `pixel.js`.
- Pixel exposes `window.PixelApp = { render, dispose }`; every window
  instance owns its history stack, layers, filter preview state, marching-ants
  RAF, and document-level key handlers (all removed on dispose).
- Pixel layout contract: toolbar + panel tabs on top, contextual options bar
  (`data-options-bar`) below it, tool rail (`data-tool-rail`) left, canvas
  center, panel right. Panel tabs: adjust, filters, colors, transform,
  layers, history, ai. Tool options render into the options bar via
  `renderOptionsBar()`; there is no draw panel tab.
- Pixel filters live in the `Pixel.FILTERS` catalog (`pixel-filters.js`) with
  css/pixel/canvas implementations across the categories color/light/style/
  detail; legacy filter IDs stay valid. Selecting a filter previews
  non-destructively (layer snapshot + strength blend); Apply commits to
  history, leaving the filters tab resets the preview.
- Pixel visible UI strings use `pixel.*` keys in all 16
  `ui/lang/desktop/*.json` files.
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
- Juice pass (2026-08): `shootTyped`, `puCollectRarity`, `weaponArm`,
  `bossKillFanfare`, `megaComboStinger`, `stageClearFanfare`, plus
  `fxMuzzleSparks` / `fxBossKillSetPiece` / `fxMegaCombo` /
  `fxStageClearSetPiece`; signature FX honor `FX_CAPS` and
  `prefers-reduced-motion`.
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
- Keep Homepage Studio split across `homepage-studio.js`,
  `homepage-studio-preview.js`, `homepage-studio-sites.js`, and
  `homepage-studio-history.js`; do not fold the preview chrome, sites, or
  history panels into the main app file. Load order in
  `module-loader.js`: preview, sites, history, then `homepage-studio.js`.
- OpenSCAD exposes `window.OpenSCADApp = { render, dispose }`. Every window
  instance owns its draft timer, SSE listeners, editor, and preview resources.
- OpenSCAD uses a preview-first workbench layout: a slim header bar with
  primary actions (render, generate, cancel, download, save) and toggle
  buttons for the three panels; the main grid is
  `[inspector | splitter | preview | splitter | parameters]` with explicit
  grid tracks so the preview keeps its column when panels collapse. The left
  inspector column hosts the Source/Files/Log tabs plus an auto-hiding
  issues bar (hidden when empty) and is resizable/collapsible; the center
  preview zone is edge-to-edge with floating glass overlays (title chip,
  viewport toolbar pill, status pill) so the viewport loses no space to
  chrome; the right parameter sidebar holds exports with a selected-count
  badge, segmented Render/Preview mode, timeout, and defines sliders; the
  agent panel is an absolute slide-over (never squeezes the preview) hosting
  the streaming chat transcript, a composer, and an apply-changes bar that
  stages agent-proposed source instead of silently overwriting.
- OpenSCAD viewport controls include zoom in/out, perspective/orthographic
  projection, shaded/wireframe shading, auto-rotate (disabled under
  `prefers-reduced-motion`), dark/light background, grid/axes, fit view,
  double-click-to-reset, and fullscreen on the whole preview zone (overlays
  stay visible). The busy overlay is scoped to the preview zone (editor
  stays usable) and shows elapsed seconds.
- OpenSCAD 3D preview uses a gradient canvas-texture background, a
  ShadowMaterial contact-shadow plane plus a model-sized grid grounded at
  the model's bounding box, `framePreviewCamera` bounding-sphere fit
  targeting the box center, and a `ResizeObserver` that keeps renderer size
  and camera aspect in sync (no stretched canvas on window/fullscreen
  resize). `cleanupPreview` disposes geometry, materials, background
  texture, and disconnects the observer.
- OpenSCAD keyboard shortcuts (Ctrl+Enter render, Esc cancel, F fit view,
  Ctrl+S save draft) are attached in `wireKeyboardShortcuts` and removed
  in `dispose`; splitters support pointer drag, double-click to collapse,
  and ArrowLeft/ArrowRight resizing.
- OpenSCAD drafts persist per `windowId` under
  `aurago.desktop.openscad.draft.<windowId>` and include additive viewport
  preferences (projection, shading, auto-rotate), panel collapse state, and
  inspector/sidebar widths.
- OpenSCAD toolbar icons use themed manifest icons (`sliders`, `cube`,
  `mesh`, `contrast` were added for this app to `ui/img/papirus`,
  `ui/img/whitesur`, and the backend icon catalog in
  `internal/desktop/types.go`); button icons are retinted via CSS
  `--oscad-icon-filter` so they stay legible on dark glass.
- OpenSCAD result events must filter on `window_id` when present; without it,
  idle multi-window instances must ignore global `openscad_result` events.
- OpenSCAD readonly mode disables CodeMirror/`textarea` editing, defines
  inputs, and the agent prompt.
- OpenSCAD visible UI strings use `desktop.openscad.*` keys in all
  `ui/lang/desktop/*.json` files.
- `homepage-studio.js`, `homepage-studio-preview.js`,
  `homepage-studio-sites.js`, and `homepage-studio-history.js` implement
  Homepage Studio as a preview-first workbench: slim header (brand, status
  pill with server/fallback/tunnel state, target selector, panel toggles),
  main grid `[chat | splitter | preview | splitter | inspector]` with
  explicit tracks, collapsible/resizable chat and inspector panels, a
  floating viewport chrome (URL pill, desktop/tablet/mobile device
  segmented, refresh, external, fullscreen), a site/drift pill and an
  agent-busy pill with elapsed time, and an inspector with Sites (managed
  site cards, drift badges, deploy targets/deployments/remote observations,
  reconcile) and History (search, type filter, offset pagination) tabs.
  The welcome hero offers localized prompt suggestion chips.
- Homepage Studio exposes `window.HomepageStudioApp = { render, dispose }`;
  every window instance owns its AbortControllers, busy/persist timers,
  listeners, and sub-module instances and must release them in `dispose`.
  Workbench state persists per `windowId` under
  `aurago.desktop.homepage.draft.<windowId>` (target, device, panel
  widths/collapse, inspector tab, history filters, selected site).
- Homepage Studio URL validation and preview sandboxing
  (`safeExternalURL`, `firstPreviewURL`, `homepageStatusPreviewURL`,
  `updatePreviewUrl`, `showPreview`, `refreshPreview`; iframe
  `allow-scripts allow-forms` + `referrerPolicy no-referrer`, never
  `allow-same-origin`) stay in `homepage-studio.js` — they are pinned by
  `ui/security_lint_test.go` and must not move into sub-modules.
- `homepage-studio-preview.js` (`window.HomepageStudioPreview { create }`)
  owns only the preview chrome (device widths, fullscreen); the sites panel
  (`window.HomepageStudioSites { create }`) and history panel
  (`window.HomepageStudioHistory { create }`) receive their dependencies
  via a deps object like `NoisemakerLibrary`.
- Homepage Studio honors `context.readonly` (composer, suggestion chips,
  reconcile, and history delete disabled). Destructive history deletes use
  the shell `confirmDialog` passed by `menus-and-routing.js`; native
  `alert`/`confirm`/`prompt` are forbidden in all four modules.
- Homepage Studio visible UI strings use `homepage_studio.*` keys in all
  16 `ui/lang/desktop/*.json` files.
- Keep Writer self-contained in `writer.js` below the 1100-line budget;
  if find/replace grows unwieldy, extract into `writer-search.js` and register
  in `module-loader.js` and `DESKTOP_APP_ASSETS`.
- New formula functions must be added to `sheets-formulas.js` and kept in sync
  with the Go evaluator in `internal/office/` (see `EvaluateFormulaForSheet`).
- Rebuild chess vendor assets with `npm run build:chess-vendor` after changing
  vendored chess package versions or copied Stockfish assets.

## Verification

- `go test ./ui/ -run TestDesktopFeeling`
- `go test ./ui/ -run 'LineBudget|GalaxaMode|DesktopAppAssets|AdaptiveMusic'`
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

- `galaxa-modes.js` - Game mode contracts (`gauntlet`, `hyperdrive`, `mirror`)
  and hooks (`modesOnRunStart`, `modesOnStageStart`, `modesShouldOpenShop`,
  `modesGetBaseMusicTheme`). Settings mode cycle reads/writes `settings.mode`.
  Achievements: `gauntlet_clear`, `hyper_survivor`, `mirror_master`.
- `galaxa-fx.js` - Supplementary Galaxa visual-effects package: chromatic boss
  shockwave rings, warp speed-line streaks, powerup sparkle bursts + rising
  glints, directional bullet-impact spark cones, combo screen-edge pulses, ship
  afterimage ghosts, plus mode/FX juice: `fxScreenShatter`, `fxBulletTime`,
  `fxBiomeWeather`, `fxRankSlam` (pixel-rect flash, no soft gradients),
  `fxHyperTunnel`, `fxMirrorRefract`, `fxHeatHaze`. Signature set-pieces:
  `fxMuzzleSparks`, `fxBossKillSetPiece`, `fxMegaCombo`, `fxStageClearSetPiece`
  (all honor `FX_CAPS` and `prefers-reduced-motion`). Attaches
  `ctx.fxBossShockwave()`, `ctx.fxWarpStart()`, `ctx.fxPowerupSparkle()`,
  `ctx.fxSparkCone()`, `ctx.fxComboPulse()`, `ctx.updateFX(dt)` and
  `ctx.fxDraw{Back,Mid,Ghosts,Overlay}(c)` via `GC.createFx(ctx)`; caps scale
  with `ctx.settings.particles` via `GC.FX_CAPS`. No child DOX file needed.
- `writer.js` - Word-processing editor: Quill rich-text, auto-save with 800 ms
  debounce, dirty-state tracking, word/character/page status bar, find &
  replace overlay with match highlighting, enhanced formatting toolbar (font,
  size, color, background, alignment, blockquote, code-block, image), and agent
  integration. Exposes `window.WriterApp`. No child DOX file needed.
- `calendar.js` - Calendar renderer and appointment UI continuation bundled
  inside the shared Desktop IIFE immediately before `sdk-events-bootstrap.js`.
  No child DOX file needed.
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
  playback and detaches all listeners. The track list is server-paginated
  (`GET /api/desktop/noisemaker/tracks?limit=&offset=&q=`, newest first via
  `created_at DESC`): the app feeds pages through `setTracks` (reset) and
  `appendTracks` (next page) and drives `setPagination({total, hasMore,
  loading})`; the library emits `loadmore` from an IntersectionObserver
  sentinel (fallback "load more" button), `search` with a 300 ms debounce for
  server-side search, and `needmore-for-play` when the player reaches the end
  of the loaded list while more tracks exist.
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
  syntax highlighting (using javascript()), error line highlighting, fallback
  textarea, and `revealLine(line)` for jumping to an issue. Exposes
  `window.OpenSCADEditor { create, parse, revealLine }`. The `parse` function
  extracts line-numbered errors from OpenSCAD stderr output. No child DOX
  file needed.
- `openscad-defines.js` - Parametric define slider panel: parses name=value
  pairs, renders numeric values as range sliders (with negative-value support)
  plus number inputs, text values as plain inputs, and per-row reset/remove
  buttons with an add-define control. Exposes
  `window.OpenSCADDefines { parse, render, toText }`. No child DOX file needed.
- `homepage-studio.js`, `homepage-studio-preview.js`,
  `homepage-studio-sites.js`, `homepage-studio-history.js` - Homepage Studio
  workbench: assistant chat with suggestion chips, preview-first viewport
  with device switcher and fullscreen, Sites inspector (drift badges,
  deployments, reconcile) and History inspector (search/filter/pagination,
  shell-dialog deletes). URL validation and the iframe sandbox contract stay
  pinned in `homepage-studio.js`. Exposes `window.HomepageStudioApp` plus
  `HomepageStudioPreview`/`HomepageStudioSites`/`HomepageStudioHistory`
  factories. No child DOX file needed.
- `log-viewer-filters.js` / `log-viewer.js` - Log Viewer: file sidebar,
  virtualized tail list, level/search filters, dedicated per-window
  EventSource to `/api/desktop/logs/stream`, readonly-gated download.
  Exposes `window.LogViewerFilters` then `window.LogViewerApp`. No child
  DOX file needed.
- `noisemaker.js` - Noisemaker app entry: capability state, create view with AI
  enhancement helpers, synchronous generation flow with progress/result/error
  slots, onboarding for unconfigured music generation, tab shell. Exposes
  `window.NoisemakerApp`. No child DOX file needed.
- `noisemaker-library.js` - Noisemaker library grid and bottom player bar
  (search, cards, template/download/delete actions, seek/volume, prev/next).
  Exposes `window.NoisemakerLibrary { create }`; loads before `noisemaker.js`.
  No child DOX file needed.
- `sip-phone.js` - iPhone-inspired SIP softphone: device chassis with glossy
  Dynamic Island and status bar, separate `.sip-phone-hw-*` hardware buttons,
  black screen bezel, aurora mesh wallpaper, `.sip-phone-glare` glass
  reflection, and ambient stage halo/floor shadow. Five tab views
  (Favorites, Recents, Contacts, Keypad, Settings) with glass tab bar; the
  Contacts tab renders the AuraGo address book (entries with phone/mobile,
  one dial row per number, client-side search filter, refresh on tab
  activation), active-call
  takeover with answer/decline for inbound ringing calls, contact-hue
  avatars, and frameless small-window fallback (hides hardware buttons,
  glare, and stage effects). Styling lives in
  `ui/css/desktop-app-sip-phone.css`. Exposes `window.SipPhoneApp`. No child
  DOX file needed.
- `sip-phone-gadget-runtime.js` (core, main bundle) - Floating phone gadget:
  mounts `sip-phone.js` windowless on a draggable body-level layer
  (`#vd-sip-phone-gadget`, scaled 400×830 stage, instance id
  `sip-phone-gadget`). Drag handles are the status bar, Dynamic Island,
  hardware buttons, and device frame; right-click opens a shell context menu
  (open in window, always on top, remove from desktop). Settings keys
  `phone_gadget.*` (defaults in `desktop-foundation.js`, toggle in the
  Settings app), layer CSS in `ui/css/desktop-sip-phone-shell.css`, gadget
  overrides in `ui/css/desktop-app-sip-phone.css`. Exposes
  `window.SipPhoneGadget { init, sync }`. The per-frame audio-visualization
  writer must skip gadget-hosted phones (the viz is `display:none` there and
  writes would invalidate the whole scaled subtree, visibly flickering the
  screen) and only write custom properties when a rounded level changed.
  Active-call texts (party name/URI, status, button and volume labels) must
  wrap inside the screen via `overflow-wrap` and never bleed past the device
  edges. No child DOX file needed.
- `pixel-state.js`, `pixel-view.js`, `pixel-canvas.js`, `pixel-tools.js`,
  `pixel-actions.js`, `pixel-filters.js`, `pixel-events.js`, `pixel.js` -
  Pixel image editor: tool rail + options bar layout, 17 tools (magic wand
  with mask selections, gradient, airbrush, dodge/burn, blur brush plus the
  classic set), 21-filter catalog gallery with live thumbnails and strength
  slider, layers, click-to-jump history panel, AI generate/enhance. Exposes
  `window.PixelApp`. No child DOX file needed.
- `terminal.js` - Standalone workspace terminal: one xterm.js session to
  `/api/code-studio/terminal`. Exposes `window.TerminalApp`. No child DOX file
  needed.
- `notes.js` - Lightweight markdown notes under `Documents/Notes/` with open/
  save via desktop file APIs. Exposes `window.NotesApp`. Visible strings use
  `desktop.notes_*` and `desktop.app_notes` in all 16 `ui/lang/desktop/*.json`
  files. No child DOX file needed.
