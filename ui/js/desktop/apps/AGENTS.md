# Desktop App Modules - Child DOX Contract

## Synth Studio

- `synth-studio` is a native Creative app with the `synthStudio.*` locale prefix.
  Keep its fifty presets (five groups of ten), 128 synthesized GM programs and
  percussion bank local. See `documentation/synth-studio.md` for supported music
  formats, controls and resource limits.
- Model validation owns project trust boundaries. Notes/controllers use clip-local
  ticks; clips use arrangement ticks. Live playback and offline WAV rendering
  share the native Web Audio implementation. Never schedule musical timing from
  animation frames or copy an upstream live-only synthesizer into offline export.
  `synth-studio-voices.js` (patches, mix buses, voices, controllers) loads before
  `synth-studio-audio.js` (project timeline, live transport, WAV render) in the
  `module-loader.js` asset list and the browser fixtures.
- Reuse OfficeSession drafts/write queues and the conditional Desktop file API.
  Preserve the latest dirty revision on failures; fence asynchronous file, MIDI
  and render results after a project change, permission revocation or disposal.
  A reopened project is paused. Desktop minimization keeps playback; browser-tab
  hiding pauses it. Closing releases notes, input ports, timers and audio nodes.
- MIDI is input-only, explicitly permission-gated with SysEx disabled. The Desktop
  document alone receives `midi=(self)`; untrusted sandboxed app frames deny MIDI.
  Read-only policy changes stop recording and block all project mutations.
- Verify the model/storage checks, `TestDesktopSynthStudioBrowser`,
  `TestDesktopSynthStudioAudioBrowser` and
  `TestSynthStudioMIDIPolicyOnlyAllowsDesktop`. Keep all sixteen locales and both
  Desktop themes usable, including compact library/sound-panel controls.

## Newspaper

- `newspaper.js` owns one window reader for today's immutable edition, articles, archive and preferences. When only an earlier issue exists, keep it readable and offer creation of today's issue; do not offer today's correction flow on older issues. Closing the window cancels only UI work; the server owns ongoing research and delivery.
- Preferences include section-bound RSS feeds. A new revision can carry an optional correction note; the reader displays it on the published issue without changing older revisions.
- Show configured research capabilities and actionable setup reasons separately from last-attempt errors. Preserve optional run counters (candidates/read pages/accepted stories) and missing-topic labels after completion or failure. Escape topic labels like all other external text; older run JSON without counters must remain readable. Config exposes `max_searches` with default 32 and range 1–64. Verify `TestNewspaperResearchTranslations` and `TestNewspaperConfigResearchBrowser` alongside the reader matrix.
- Escape all source text and allow only safe HTTP(S) source links. Show actual publication and source times, partial coverage, single-source disclosure, run progress and delivery uncertainty. Keep the same issue when moving between views and preserve reading position.
- Show localized email-confirmation bounce suppression, rejection, configuration failure, uncertainty and cooldown messages from structured API codes. Send code first saves the selected sender account and recipient address, without saving unrelated draft preferences; never send through a stale stored account. Do not render raw AgentMail response text or repeat a blocked send automatically.
- Load `desktop-app-newspaper.css` lazily, use both Desktop themes, all 16 `newspaper.*` locale entries and the Papirus/WhiteSur Newspaper icons. Verify with `TestDesktopNewspaperBrowser` and `TestNewspaperTranslations`.

## Personal Radio

- `personal-radio-player.js`, `personal-radio-runtime.js`,
  `personal-radio-settings.js` and `personal-radio.js` load in that order.
  Register `personalRadio` in the module loader's `APP_I18N_SECTIONS`; the
  desktop shell does not initially embed this app's translation prefix.
  Window disposal cancels only window work. The singleton desktop runtime owns
  playback, listener heartbeat and the mini control until explicit stop or
  pagehide; reopening must not create a second player.
- Show startup immediately in the central player with an explicit current task,
  actual ready tracks/minutes against both requirements, and the automatic-start
  explanation. Keep this visible throughout preparation, including after the
  opening. Do not use the small reserve/job label as the only startup feedback.
  Expose registry search, preparation and failure states. A zero minute reserve
  means automatic start with the required prepared track count; show available
  minutes without an invented 30-minute target or division by zero. Explain that
  matching existing music plays while new fitting music is produced in background.
- Use one Web Audio clock, bounded PCM windows and two prepared segments for
  initial music playback. A typed opening may play alone before `music_ready`,
  without priming/bypassing music preparation. Fence asynchronous work by generation and server epoch.
  Never claim airtime from a server timer. Expired speech must not resume.
- Blend music into and out of speech on that clock. Keep speech clear above the
  ducked music; every third eligible moderation may retain a quiet outgoing music
  bed for up to 4.5 seconds. During overlaps, the newer segment owns the title
  and playback position.
- Keep Standard/Fruity tokens, reduced motion and all 16 locale dictionaries.
  The original Radio app, Noisemaker and Webamp retain their own lifecycles.
- Verify `TestPersonalRadioBrowser`, `TestPersonalRadioAudioContinuityBrowser`,
  `TestPersonalRadioVoiceMixBrowser`
  and `TestPersonalRadioTranslations`. Backend: `internal/personalradio/AGENTS.md`.

## RTL-SDR

- `rtl-sdr-runtime.js`, `rtl-sdr-panel.js`, `rtl-sdr-scope.js` and `rtl-sdr.js`
  load in that order. `rtl-sdr.js` resolves local `t('key')` calls under
  `rtlSdr.`; the built-in app name uses `desktop.app_rtl_sdr`. Keep both in all
  16 desktop locales. The panel and scope modules receive translated text and
  never call `t()` themselves; the static i18n checker resolves the app-local
  prefix for `rtl-sdr.js` only.
- The receive view is a graphite receiver front panel: station memory, display
  (status indicators, seven-segment frequency, station text, signal meter, band
  scope with waterfall), tuning column and key strip. It fits the default
  1000x760 window without scrolling; narrower windows stack the sections and
  scroll. Recordings, schedules and receiver setup are views of the same chassis.
- `s.tuning` is the single source of the pending tuning. Every control commits
  into it and `controls()` repaints all widgets. Mode keys and AGC/stereo are
  native radio/checkbox inputs, the four small knobs wrap native range inputs,
  and the gain knob steps through the tuner's reported gain list by index.
  A manual frequency or mode change drops the station label.
- Frequency digits are created once and repainted, so focus survives tuning.
  They accept wheel, arrow keys and typed digits. While the panel itself
  scrolls, the wheel only turns the control that has focus. The waterfall keeps
  its history across view changes and shifts old rows with the tuned frequency.
- The singleton runtime owns the desktop mini control. Windows report their
  visibility through `RTLSDRRuntime.present(id, visible)`; the mini control
  shows only while listening without a visible receiver window.
- Verify with `TestRTLSDRTranslations`,
  `TestFrontend_StaticI18nKeysExistInEnglishBundle` and the opt-in
  `TestRTLSDRDesktopBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`). Its
  `AURAGO_RTLSDR_SCREENSHOT` path also receives `-fruity` and `-compact` views.

## Bluetooth

- `bluetooth-views.js` precedes `bluetooth.js`; strings live under `bluetooth.*`
  plus `desktop.app_bluetooth` in all 16 desktop locales and load through
  `APP_I18N_SECTIONS`. Views use literal translation keys so the static i18n
  checker sees every call.
- Hardware-bound built-in apps declare `Requires` in their manifest; the
  server's `CapabilityProvider` decides visibility at read time and a
  capability change broadcasts `desktop_changed` with
  `operation: "app_availability"`. An open window is never closed by a
  capability change; it shows its unavailable state and resumes.
- The app never trusts event payloads: every `aurago:bluetooth-change` with a
  different revision re-fetches `GET /api/bluetooth/status`. Fetches never
  overlap. Server-owned state (scan, visibility, running operations) survives
  closing the window.
- Device actions post `wait: false`; progress and errors come back as
  `operation`/`error` codes on the device and are shown from
  `bluetooth.error_<CODE>` keys, never from English server text.
- Pairing questions render in an in-window dialog with a 20 s countdown;
  passkey/PIN fields validate locally. With the app closed, the shell shows a
  "Pairing request" notification.
- Verify `TestBluetoothTranslations`,
  `TestFrontend_StaticI18nKeysExistInEnglishBundle` and the opt-in
  `TestDesktopBluetoothBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`, screenshots via
  `AURAGO_BLUETOOTH_SCREENSHOT`). Backend: `internal/bluetooth/AGENTS.md`.

## Detective
- `detective-views.js` precedes `detective.js`; both use the native Desktop theme
  tokens and `desktop-app-detective.css`. Register the built-in `detective` app
  with the existing search icon. All sixteen locales use `desktop.detective_*`.
- `DetectiveApp` owns a per-window instance with cancellable polling. Disposal
  ends UI requests only; research remains owned by the server. Render evidence
  and structured report blocks as escaped text. Only HTTP(S) source links are
  clickable; private integration receipts never masquerade as website links.
- Export links always name an immutable report revision. Autor opens a new,
  create-only DOCX copy. No private model continuation is returned to the UI.
- Verify `TestDetectiveTranslations`, `TestDesktopDetectiveBrowser` and the
  Detective config section. Backend contracts: `internal/detective/AGENTS.md`.

## Looper
- `looper-monitor.js` precedes `looper.js`. The monitor owns the run view
  (`createRunView`), the log-delta merge (`mergeStatus`) and the history views;
  `looper.js` owns the form, presets, drafts and the action bar.
- `/api/desktop/looper/status` sends the whole state first and afterwards only
  log entries appended since (`logs_from` = absolute index of the first sent
  entry, `log_total`, `run_id`). A message without `logs_from` is a full
  replacement. The client merges by absolute index and never rebuilds the
  timeline: new entries are appended in place, finished rounds fold away, and
  following the newest entry stops as soon as the user scrolls up.
- While a run is active the editor is replaced by the summary card
  (`is-focus`); `Edit loop` brings the fields back and is disabled while a run
  is executing. Preset switches are blocked during a run. Unsaved edits show the
  `vd-looper-dirty` dot, are confirmed before being discarded and are kept as a
  draft in `localStorage` (`aurago.looper.draft.v1`, always wrapped in try/catch).
- A pending pause (`pause_requested`) disables the pause button and says so;
  Stop on a paused run discards it (the server files it as `stopped`). A review
  without a usable score is a `failed` evaluate entry, never score 0.
- Chart, gauge and hero use theme tokens; keep the grid rows explicit so the
  action bar cannot absorb the free row. Every new string needs all 16 locales.
- The unused run view shows the decorative transparent `img/looper-empty.png`
  illustration with the existing empty-state copy. It shares the monitor's idle
  predicate and disappears for running, paused and finished runs; keep it responsive.
- Costs come from the budget tracker's model rates (`EstimateCost`); `cost_approximate`
  marks a fallback price. Starting or resuming under an exhausted budget answers
  402 with `code: "budget_exceeded"`, and a running loop pauses itself with
  `pause_reason: "budget"`. API errors carry `code` in `err.body.code` (the shared
  `api()` helper sets no `status`), never match on the English message.
- The active run is checkpointed to `desktop_looper_active` after every round and
  restored paused (`pause_reason: "interrupted"`) on the next start; a graceful
  shutdown keeps the checkpoint instead of filing the run as stopped. A window
  that finds an active run with an empty editor adopts `GET /api/desktop/looper/active`;
  Resume with an empty editor sends `{}` and the server uses the stored settings.
  `LooperRunner.checkpoint` only writes while the run is running or paused and shares
  `checkpointMu` with the removal in `persistFinishedRun`, so a discarded run cannot be
  brought back by a late write. `init()` waits for the first status (max 1.5 s) before
  `restoreDraft()`, otherwise a stale draft would shadow the settings of a restored run.
- History records keep their run settings (`config` on `GET .../runs/{id}`), which
  drive "Load into editor" and "Run again"; older records have none.
- The desktop opens a window only when a `virtual_desktop_event` arrives over SSE.
  Looper tool calls run with `looperDesktopBroker`, which forwards just those events
  and drops all other tool feedback; `open_app`/`open_widget` pass only in the finish
  step, because an early Writer window would merely be refocused there, still showing
  an old draft (`TestLooperFinishOpensTheResultInTheDesktop`).
- Verify `TestDesktopLooperUIContract`, `TestDesktopLooperCostI18n`,
  `TestDesktopLooperDurationI18n`, `TestDesktopLooperActionButtonsShareConsistentStyle`
  and the opt-in `TestDesktopLooperBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`).
  Backend: `internal/desktop/looper.go`, `internal/server/looper_service.go`.

## EasyDrag
- Nineteen scripts (`easydrag-*.js` and `easydrag.js`) share `window.EasyDrag` and load in the
  order of `DESKTOP_APP_ASSETS.easydrag` (`core/module-loader.js`), `easydrag.js` last.
  `window.EasyDragApp = { render, open, dispose }`; `open` re-routes an existing window by
  `flowId`/`flow_id` and `runId`/`run_id`, and `{section: 'home'}` (Mission Control's New flow)
  shows the start page once the editor's `leave()` allowed it.
- Pure modules (template, model, geometry, start-page preview, shortcut table) run in Node; the
  template filters mirror `internal/flows`. Every model command is one undo step. Pans and zooms
  change only `ed.view`, stored per flow on the device (`aurago.easydrag.view.<id>`): the model
  has no viewport command, so they never bump `model.version`, save, or count as unpublished
  changes; a document's own `viewport` (old drafts, imports) is kept as it came and not read.
- State words match Mission Control and the missions page: `state_draft` reads "Not published yet"
  (never published), `state_inactive` and `home_filter_inactive` read "Paused" (published,
  switched off; the filter lists only those). "Draft" is the editable version. The MC hint
  `desktop.mc_flow_cancel_in_easydrag` uses EasyDrag's Stop.
- The editor follows `flows_changed` for its flow (`flow_id`): `enabled`/`published` re-read the
  record (`refreshRecord`: chip, Active switch, Run now), `deleted` goes home with a notice
  (`goneElsewhere`). An event without an id and reason `deleted` or `enabled` is "maybe mine": the
  record is read again and `FLOW_NOT_FOUND` means deleted. Every delete (editor, start-page card,
  elsewhere, a broadcast on the start page) calls `core.forgetFlow(id)`, which drops the four
  per-flow keys (`core.FLOW_KEYS`: emergency copy, view, effects-ok, test-trigger).
- Run now is off while a published flow is paused (`runNowState`), in the ⋯ menu, the Flow menu
  and a card's menu, with `disabledHint` (`error_flow_disabled`), a menu-item field the desktop
  draws as the disabled item's tooltip; a stale click gets the server's 409 `FLOW_DISABLED`.
- DOM modules get the editor state `ed` and talk over `ed.bus`, never through other modules'
  DOM. Focus moves synchronously when a popover or dialog opens; `.ed-app [hidden]` forces
  `display: none`. `ED.canvas.portLabel(t, node, port)` is the one port label.
- Keyboard model of the canvas: the canvas is one tab stop, the zoom bar follows. Arrow keys move
  the selection between steps (announced through the live region, `core.announcer`), Enter opens
  the detail view, Delete/D/Ctrl+D/C act on the selection, Tab adds a step. A card's tool buttons
  are tab stops only while it is selected (`syncTools`; `tabindex="-1"` otherwise), and
  `.ed-node:focus-within` shows a focused tool's toolbar. Tab on the canvas opens quick-add, so
  the zoom bar and a selected card's tools are reached with Shift+Tab from the footer. Every tool
  has a key as well: D (off/on), Ctrl+D (duplicate), Del (delete); Test step sits in the detail
  view (Enter). The screen-reader step list (`.ed-node-list`) duplicates the arrow keys, so its
  buttons are `tabindex="-1"`: it serves a screen reader's browse mode, not Tab. With 5 steps
  and no selection the canvas has 5 tab stops (the browser smoke test counts them). Screen
  changes move the focus: leaving the run view to the canvas, the start page to New flow and then
  to the card of the flow just left (not for Mission Control's New flow or the templates link),
  the shell's error card to Retry.
- Strings: `easydrag.ui.*` in `ui/lang/easydrag/<16>.json` (shared with the server's catalog
  keys); built keys (`core.tr`) belong to a family of `TestEasyDragUIKeysExistInAllLocales`.
- `createApi` (`easydrag-core.js`) errors carry `err.body.code`, shown as
  `easydrag.ui.error_<code>`. SSE (`/runs/{id}/events?after=<seq>`) is idempotent and
  reconnects at once on `event: resync`.
- Test effects (`ED.runs.effects`): each step that runs counts with its catalog effects (default
  params) and those of its real params from `GET publish-preview` (`CollectEffects`). Without
  the preview, or with a running step whose catalog entry is risky without effects (a failed
  effects hook), the dialog says so and never starts silently; an edit after the flush is checked
  again before Run. A step test counts what the engine runs (`engine_state.go` `run`,
  `fireTrigger`, `collectReady`); keep the two in step.
- Runs that have not ended (`ED.runs.isActive`) get Stop in the drawer and the run-view banner
  (`runs.stopRun`: any run but a test asks first, 409 `FLOW_RUN_FINISHED` refreshes quietly, a
  run no stream here shows is asked for after 1 to 16 s); `run_finished` refreshes both
  (`runs.runFinished`: 250 ms, at most 1 s). The secret field deletes the chosen secret and warns
  with `used_by`.
- Saver: 1 s after the last change; network errors, 5xx, 429 and `FLOWS_DISABLED` go `offline`
  (retry 5 s, doubling to 60 s), `PERMANENT_CODES` and a 4xx without a code go `failed` (a retry
  button), `FLOW_INVALID` waits for the next change. The emergency copy is written at most once
  per 500 ms while changes keep coming (a drag); a held-back change is written by a later change,
  `saver.flushCopy()` (the interact bus event `gesture-end`, `pagehide`, a `visibilitychange`
  to hidden), the next save, `flush()`, `dispose()`, and at the latest by a trailing write
  500 ms later, so a crash loses less than 500 ms; never once the draft is saved. Model change
  sets and `model.node` look ids up in Maps (`node` rebuilds its Map after each write of the
  node list); `test-easydrag-extra5.mjs` guards a 200-step drag and the 100-to-400-step scaling.
- Run view: `ed.model` is the stored run's document there, `ed.draftModel` always the draft. The
  hints (`publish.refreshIssues`) validate the draft, wait while the run view shows and run again
  on exit; a live run that starts meanwhile is parked (`runs.attach`) and followed on exit.
- Dialog holds (`holdAction`, a 429's Retry-After) are kept per button and end when the dialog
  closes; a tree drag's document listeners (`dragend`, `drop`) end with the drag or the next one.
- Window menus pass canonical keys ("Ctrl+S"); the shell dispatches a `shortcut` item before the
  editor sees the key, a `shortcutHint` ("?") is only drawn. Mod+S is always prevented in the
  editor and saves only when no EasyDrag dialog is open.
- `editor.leave()` aborts a drag, flushes the saver and shares one in-flight promise. Sessions
  and notification contexts keep `flowId` only when it matches `^flow_[a-z0-9]{10}$`.
- Opening: a stored view `{cx, cy, zoom}` per flow (editors wider than 560 px), else the readable
  fit (zoom ≥ 0.8, trigger first); below zoom 0.7 cards show labels only. Under 900 px the
  palette floats over the canvas; the run view keeps it hidden and inert.
- Other surfaces: Mission Control (`MissionControlTriggers.isFlow`/`isUnpublishedFlow`/
  `upcomingRun`), `ui/js/missions/main.js`, `ui/cfg/flows.js` and the dashboard's cron list
  (`managed_by: easydrag`). User docs: manual chapter 24.
- Verify: `node scripts/test-easydrag.mjs` (with `-extra.mjs` to `-extra5.mjs`),
  `npm run test:mission-control`, `npm run test:dashboard-cron`, `npm run test:missions-page`,
  `go test ./ui -run 'EasyDrag|MissionControlShowsFlow|StandaloneMissionsPage'` and the opt-in
  `TestDesktopEasyDragBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`, screenshots in
  `reports/easydrag/`). Backend: `internal/flows`, `internal/server/flows_*.go`.

## Purpose

This subtree owns built-in virtual desktop app modules that are loaded lazily by
`ui/js/desktop/core/module-loader.js`.

- `tresor.js` owns setup, recovery confirmation, note/file workflows, encrypted note autosave/draft recovery, the five-minute idle lock and window disposal. Desktop close awaits saves and reports failures. Explicit, idle and pagehide locks immediately clear decrypted state even during failed/pending writes; only completed ciphertext drafts survive. Late operations cannot alter a newer session. Commit note selection only after loading/decryption succeeds, and keep both draft fields in state across redraws. `tresor-crypto.js` derives Argon2id keys with locally copied `argon2id@1.0.1` WASM, wipes its entire workspace and encrypts metadata and bodies with Web Crypto AES-256-GCM. Never send passwords, recovery keys or plaintext to `/api/desktop/tresor` or persistent browser storage. The door artwork is decorative; forms stay keyboard-accessible and reduced motion disables the opening effect. Verify `TestDesktopTresorBrowser` (including `checkTresorFailureRecovery`) and `npm run test:tresor-crypto`.

- Game Maker's existing asset browser includes sprite packs and individual 3D
  models. `game-maker-studio-models.js` owns one disposable viewer per modal,
  sharing the pinned local 0.186.1 runtime helper. Cancel pending fetches and
  release rigs, controls, shadow maps and renderer on replacement/close/dispose.
  Render only on interaction or while an animation is playing and visible.
  Selection uses up to 64 `model_asset_ids`, separate from sprite pack selection.
  Keep all labels in the sixteen desktop locales and preserve both import flows.

Shell chrome helpers live in the main desktop bundle (not lazy apps):
`core/sound-runtime.js` (opt-in synthesized UI sounds; lazy
`bundles/desktop-sounds.bundle.js` with `sound/synth-core.js` and five theme
modules), `core/screensaver-runtime.js` (opt-in idle detection and wake
handling; the lazy overlay and scenes live in `js/desktop/screensavers/`,
see its child contract), `core/session-runtime.js` (session restore, dock pins, recent files, default
apps), `core/spaces-runtime.js` (three virtual desktops / Spaces v1: window
`spaceId`, hide-without-dispose, session snapshot v2, taskbar pager, Ctrl+Alt
arrows; disabled on compact viewport), `core/shell-chrome-runtime.js`
(notification center, clock popup, hold window switcher, shortcuts overlay),
`core/spotlight-runtime.js` (Ctrl+K mixed search), and
`core/window-shell-runtime.js` (widget frames, standalone widgets,
`openApp`). Widget-frame and standalone-widget empty-state load
failures use `desktop.load_failed`. Standalone Webamp notifications
map `desktop.winamp_unsupported`. Weather HTTP throws the sentinel
`HTTP` without a status; the catch interpolates
`desktop.weather_load_error` with `desktop.weather_network_error`
only. Widget persist toasts (location, auto-size, bounds) use
`desktop.widget_update_failed`. Styles:
`ui/css/desktop-chrome.css` (bundled into `desktop-shell.bundle.css`).
Persisted keys: `windows.restore_session`, `appearance.dock_pins`,
`session.windows` (snapshot v2 with `activeSpaceId` and per-window `spaceId`
plus optional `alwaysOnTop`),
and `files.default_apps` via `/api/desktop/settings`.
The shell owns its settings writer; the pet runtime's private writer is not
available to session, dock or default-app helpers. Enabling session restore
captures current windows immediately; `pagehide` flushes pending geometry with
a keepalive request. Startup restores before opening an `?app=` deep link,
preserving separate instances and the normal bounds of maximized windows.
Verify actual save/reload behavior with `TestDesktopSessionRestoreBrowser`.

### Desktop UI sound contract

- Opt-in via `sound.enabled` (default off). Persisted keys:
  `sound.theme`, `sound.volume`, `sound.windows`, `sound.notifications`,
  `sound.navigation`, `sound.files` (`internal/desktop/types.go`).
- Shell API: `desktopSound(eventId)` only. No playback when disabled, tab hidden,
  session restore, category off, or before the first user gesture unlocks Web
  Audio. Preview in Settings may render/play without changing the saved theme.
- Event vocabulary (19 ids, all five themes): `window.*`, `notify.*`, `menu.*`,
  `space.switch`, `dialog.*`, `file.*`. Hooks live in window shell/interactions,
  notifications, menus, spaces, dialogs and file drops — no business-logic changes.
- Offline render once per theme into cached `AudioBuffer`s; master gain uses
  perceptual `volume²`, category gains, compressor and voice rate limits.
- Settings app section `sound` exposes master toggle, theme cards with preview,
  volume range and category toggles. Keep `desktop.settings_sound_*` in all 16
  desktop locales. Saving `sound.volume` must not rebuild the Settings pane
  (that resets scroll). Other setting saves restore `.vd-settings-pane` scroll.
  `sound-symbolic` maps to the speaker mini icon; theme-SVG fallbacks use a
  currentColor mask so they are not black on dark chrome.
- Verify with `TestDesktopSoundRuntimeMarkers`, `TestDesktopSoundHookMarkers`,
  `TestDesktopSoundThemeEvents`, `TestDesktopSoundTranslations`,
  `TestDesktopSoundSettingsUIMarkers`, `TestDesktopSettingsSymbolicIconAssetsStayCompact`,
  service settings tests, and opt-in `TestDesktopSoundBrowser`
  (`AURAGO_RUN_BROWSER_SMOKE=1`).

Resize handles in `core/window-interactions-runtime.js` honor the shell's
per-window minimum width/height, including the fixed opposite edge on west/north drags.
Free titlebar/menubar space supports dragging and double-click maximizing;
buttons and menu popovers remain excluded from those gestures.

### God's Eye View Store setup

- Store operation failures remain as escaped, accessible text on the app card
  until retry, including when install rollback removes the installed record.
  Notify before refreshing Desktop bootstrap; refresh failures must not hide
  the original operation error. Polling errors must remain visible as well.
- `software-store.js` adds `Einrichten` for installed `gods-eye-view` only.
  GET/PUT `/api/desktop/store/apps/gods-eye-view/config` returns flags/origins,
  never saved keys. Empty fields preserve, checkboxes explicitly delete, and
  pending changes remain visibly inactive after a failed container replacement.
  Keep the dialog keyboard accessible, focus trapped and disposed on app close. Every Store modal close clears credential values and attributes, removes its overlay, and ignores late asynchronous responses.
- Installation preselects the current AuraGo origin. The dialog accepts further
  exact HTTP(S) origins and explains optional LAN access/provider quota use,
  browser-visible Google/Cesium credentials, and secure-context microphone
  permission. Keep `desktop.store.gev_*` in all 16 Desktop locale files.
- `quickconnect-launchpad-chat.js` uses the existing sandboxed container-app
  frame, adds microphone only for this app, passes `aurago_lang` to the managed
  notice and keeps external-open disabled. No new proxy path or auto-start is
  introduced. Logo/license live under `img/desktop/store/gods-eye-view.*`.
  Papirus and WhiteSur register `gods-eye-view` with copies of that SVG carrying
  its MIT notice; keep them synchronized with the backend preferred-icon list.
- Verify `TestGodsEyeDesktopTranslations`, bundle `--check`, and opt-in
  `TestGodsEyeDesktopBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1` and
  `AURAGO_GEV_BROWSER=1`: reviewed image on 127.0.0.1:14173, allowed frame origin
  `http://127.0.0.1:18099`. The test exercises the globe, free live data, controls,
  resizing, close/reopen and key-free setup retrieval. Real provider acceptance
  remains separate from image adapter tests with synthetic responses.

### Spaces v1 contract

- Exactly three spaces (`1`, `2`, `3`); no create/delete in v1.
- Normal windows get `spaceId` from the active space at open time; session
  restore reads stored `spaceId` (fallback space `1`).
- Space switch hides other windows (`vd-space-hidden` / `data-space-hidden`) but
  does not dispose them; minimize and space-hide stay independent.
- Desktop icons, widgets, and gadgets stay global across spaces. Wallpaper is
  per space (`appearance.wallpaper_by_space`) and falls back to
  `appearance.wallpaper`; compact viewport keeps one shared wallpaper.
- Taskbar/dock pins stay global; running window buttons and Ctrl+Tab switcher
  list only the active space.
- Fruity lists every user-facing app with `dock_visible !== false`, with pins
  first and no fixed item limit. Hidden apps appear temporarily only while
  running on the active space. Keep hover-label headroom inside `.vd-dock-scroll`
  so its vertical overflow clip cannot leave label fragments above the dock.
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

### Window always-on-top contract

- Normal windows can pin above other windows via the window context menu.
  Pet and SIP gadgets keep their own always-on-top settings; they are not
  windows and stay siblings above `#vd-window-layer`.
- On-top windows use Z-band `200000` inside the window layer. Focusing a
  normal window must not jump that band. `normalizeWindowZIndexes` keeps both
  bands. Gadgets stay out of this band.
- Always-on-top stays space-scoped. Hidden on other spaces. Not global.
- Session snapshot v2 stores `alwaysOnTop` additively. No version bump to 3.

### Widget config contract

- Sticky notes are individual `type: sticky-note` records. Their manager and
  context-menu deletion uses `desktop.sticky_delete` / `sticky_delete_msg` to
  explain that only this note and its content are removed; creation remains
  available. Reserve `widget_delete_permanent` for custom widget registrations.

- `builtin-printer` is hidden by default and added through the widget drawer.
  It uses the shared 320px widget width, remembers the selected configured ID in browser storage,
  polls read-only status every 30 seconds while visible, and shows progress,
  available estimated remaining time, layers, nozzle/bed temperatures and filename.
  Elegoo `TotalTicks - CurrentTicks` is seconds; absent timing is never estimated
  from progress. Text uses `desktop.widget_printer_*` in all 16 desktop locales.

- `builtin-fritzbox` (`core/widget-fritzbox-runtime.js` plus the pure SVG/format
  helpers in `core/widget-fritzbox-charts.js`) is hidden by default, uses the
  shared 320px width at 300px height and reads only
  `GET /api/desktop/fritzbox/overview?sections=...`. Pages (connection, traffic,
  devices, telephony) are a pager with `role="tablist"` dots, arrows,
  Arrow/Home/End keys, horizontal wheel/trackpad gestures and touch/pen swipe;
  the last page persists in `localStorage` key `aurago.desktop.fritzbox.page`.
  The widget card captures every pointer on `pointerdown` so it can be moved,
  therefore a mouse never starts a swipe and swipe move/up/cancel handling
  listens on `window` in the capture phase (removed in the cleanup); a swipe
  aborts when the card enters `vd-dragging` or a long press triggers. Never
  attach the swipe's `pointerup` to the viewport alone. Sections whose capability
  is off are dropped; `fritzbox_disabled` shows `widget_fritzbox_disabled_hint`
  and stops polling. Polling: connection every 5 s while visible (keeps the
  traffic history continuous on every page), devices/telephony every 60 s plus
  a targeted refresh when their page opens with data older than 20 s, system
  every 5 min; `visibilitychange` pauses, `AbortController`
  cancels in-flight requests, and `registerWidgetCleanup` releases timers,
  observer and listeners. Traffic history is a client-only ring buffer
  (`fritzMergeMonitorSamples`, max 180 samples) seeded from the router online
  monitor; no backend sampler exists. All router text (host names, caller
  names, numbers, IPs, SSIDs) renders via `textContent`; `innerHTML` receives
  only the static shell, numeric SVG markup and trusted glyphs. Compact mode
  (`is-compact`) engages below 280px via `ResizeObserver`. Strings use
  `desktop.widget_fritzbox_*` plus reused `desktop.copy`, `desktop.copied`,
  `desktop.retry`, `desktop.load_failed`, `desktop.system_info_updated` and
  the sysmon byte/uptime formatters in all 16 desktop locales.
- Fritz!Box overview errors belong to individual sections. Preserve a failed
  section across unrelated partial polls and clear it only after its own
  successful read or capability removal. Name the affected sections in the
  banner; a telephony/device failure must not mark fresh connection data stale
  or claim the router is unreachable. Request failures use a neutral data-load
  message. Verify the partial-error and recovery cases in
  `TestDesktopFritzBoxWidgetBrowser`.

- All widget cards, including sticky notes and generated iframe widgets, use
  `widgetWidth()` (320px, reduced only for a narrower workspace). Content resize
  changes height only. Stored legacy widths must not override the shared width.
  `snapWidgetPosition()` applies an 8px grid during drag and restore, with edges
  rounded inward after workspace clamping. Default stacks stay on that grid;
  drag persistence stores the displayed position and dimensions.

- Weather location lives in `widget.Config.location` (`{ lat, lon, name, country }`).
  `localStorage` key `vd-weather-location` is a one-time import only. After a
  successful POST upsert, stop using it as the source of truth.
- Auto-size persists as `widget.Config.auto_size`. Default is on when the flag
  is missing. Top-level `auto_size` is dropped by the Go `Widget` struct — do
  not add a Go field unless Config cannot round-trip.
- POST `/api/desktop/widgets` replaces the entire `config_json`. Always send a
  merged full config. PATCH stays visibility-only.
- The widget context menu owns the auto-size toggle (`desktop.widget_auto_size`).
  Readonly denies the mutation. Weather location saves use `skipReload` so the
  card does not remount.
- Weather chrome and WMO labels use `desktop.weather_*` keys in all 16 desktop
  locales. Do not hardcode English weather UI or WMO labels.
- Builtin widget catalog titles are UI-only via `widgetDisplayTitle`. Stored
  `widget.Title` stays the seed. Do not send the translated title in POST/PATCH.
- Widget chrome errors and the sysmon host label use `desktop.quickchat_error`,
  `desktop.widget_update_failed`, and `desktop.system_info_host`. Weather
  HTTP throws the sentinel `HTTP` without a status. The weather catch
  interpolates `desktop.weather_load_error` with
  `desktop.weather_network_error` only. Persist toasts for location,
  auto-size, and bounds use `desktop.widget_update_failed`. Do not dump
  `err.message` or an HTTP status there. Do not hardcode English there.
- Sysmon uptime units and weather wind speed use
  `desktop.system_info_uptime_days_hours`,
  `desktop.system_info_uptime_hours_minutes`,
  `desktop.system_info_uptime_minutes`, and `desktop.weather_wind_kmh`. Do not
  hardcode `d`/`h`/`m` or `km/h`.
- Sysmon memory, disk, and network sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do not
  hardcode `B`/`KiB`/`MiB`/`GiB`/`TiB` there. Leave the `/s` rate suffix.
- The opt-in `builtin-meshcore` widget (`core/widget-meshcore-runtime.js`)
  reads only `GET /api/meshcore/messenger/bootstrap` (conversations, status,
  enabled) and refreshes on the metadata-only `aurago:meshcore-change`
  document event plus a 30 s visibility-gated poll; all listeners and timers
  are released via `registerWidgetCleanup`. Rows open the `meshcore` app with
  a validated 64-hex `conversation_id`. Protected previews show the lock
  placeholder and are never revealed; radio text renders via `textContent`
  only. Strings use `desktop.widget_meshcore_*` plus reused
  `desktop.meshcore_*` keys in all 16 desktop locales.
- The System Info app reuses `hours_minutes` and `minutes`, and uses
  `desktop.system_info_uptime_days_hours_minutes` when days are present.
  Sysmon stays without minutes in the days/hours form.
- System Info network totals use `desktop.system_info_network_io` with
  `{{sent}}` and `{{recv}}`. Do not hardcode English `up / down`.
- System Info history-chart dataset labels reuse `desktop.system_info_cpu`,
  `desktop.system_info_memory`, and `desktop.system_info_disk`. Gauge
  dataset labels use `desktop.system_info_used` and
  `desktop.system_info_free`. Do not hardcode English CPU/Memory/Disk or
  Used/Free there. Leave Chart legend and tooltip off.
- System Info memory, disk, and network sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do not
  hardcode `B`/`KiB`/`MiB`/`GiB`/`TiB` there. Leave File Manager and
  OpenSCAD byte formatters unchanged.
- Log Viewer file-list sizes use `desktop.bytes`, `desktop.kib`,
  `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do not hardcode
  `B`/`KiB`/`MiB` there.
- Sheets search match counts use `desktop.sheets_match_count` with
  `{{current}}` and `{{total}}`. Do not hardcode English `of` there.
- Writer search match counts use `desktop.writer_match_count` with
  `{{current}}` and `{{total}}`. The search-close tooltip uses
  `desktop.close`. Do not hardcode `1/5` or `Esc` there.
- Pet fallback `aria-label` uses `desktop.pet_aria_label`. After a pet
  loads, keep `display_name` / `id`. Pet Picker scale text uses
  `desktop.pet_scale_value` with `{{value}}`. Do not hardcode
  `Desktop pet` or `1.0x` there.
- Radio station click counts use `desktop.radio_compact_thousands` and
  `desktop.radio_compact_millions` with `{{count}}`. MediaSession title
  fallback uses `desktop.app_radio`; album uses `desktop.radio_album`.
  Pass `t` into `updateMediaSession`. Radio `t` stays key-only;
  interpolate via `.replace`. Do not hardcode `K`/`M`, `Radio`, or
  `AuraGo Radio` there.
- Mission Control formatting rules live in the "Mission Control contract"
  section. Keep SIP/Noisemaker duration formatting unchanged.
  System World shows localized timestamps; compact uptime reuses
  `desktop.system_info_uptime_days_hours`, `desktop.system_info_uptime_hours_minutes`
  and `desktop.system_info_uptime_minutes`. Budget uses `desktop.looper_cost`;
  success rate and identity rows use `sysworld.panel.success_rate` and
  `sysworld.panel.id`. Interpolation goes through `inst.ctx.t`.
- People birthday countdowns in the sidebar, cards, list, and detail
  use `desktop.people_today`, `desktop.people_tomorrow`, and
  `desktop.people_days_until_birthday`. Do not hardcode `d` or `days`
  there.
- People KG toggle, active label, and card badge use
  `desktop.people_kg`. Do not hardcode `KG` there.
- Virtual Computers duration and expiry-day labels use
  `desktop.virtual_computers_duration_*` and
  `desktop.virtual_computers_expiry_days`. Volume TTL options use
  `formatDuration` for 1/7/30 days. Do not hardcode `s`/`min`/`h`/`d`
  there. Leave `tx` as a key-only helper.
- Virtual Computers volume sizes use `desktop.bytes`, `desktop.kib`,
  `desktop.mib`, and `desktop.gib`. Do not hardcode `KB`/`MB`/`GB` there.
- The New computer dialog shows the per-machine Network choice for both Python
  and Desktop templates. When the server reports Internet capability, default
  to Internet and allow Offline; otherwise explain why only Offline is available.
  Send the explicit selection on launch and keep labels in all 16 Desktop locales.
- Looper log durations use `desktop.looper_duration_ms` and
  `desktop.looper_duration_s`. Do not hardcode `ms`/`s` there. Leave SIP
  and Noisemaker `formatDuration` unchanged.
- OpenSCAD, Homepage Studio, and Noisemaker elapsed busy times use
  `desktop.noisemaker_progress_elapsed` with `{{seconds}}`. Do not
  hardcode `s` there. OpenSCAD `t` stays key-only; interpolate via
  `ctx.t`.
- Looper status cost and token labels use `desktop.looper_cost`,
  `desktop.looper_cost_under`, and `desktop.looper_tokens`. Sysworld
  HUD budget reuses `desktop.looper_cost` only. Do not hardcode `$` /
  `<$0.01` / `tok` there.
- Zipper status uses `zipper.selected` and `zipper.compressed_size`.
  Zipper sizes use `desktop.bytes`, `desktop.kib`, `desktop.mib`,
  `desktop.gib`, and `desktop.tib`. The open-dialog filter uses
  `desktop.file_dialog_zip`. Do not hardcode `selected`,
  `compressed`, `ZIP Archives`, or `B`/`KiB`/`MiB` there. Zipper
  `t` stays key-only.   Pixel open-dialog filter uses
  `desktop.file_dialog_images`. Pixel save-dialog filters use
  `desktop.file_dialog_png` and `desktop.file_dialog_jpeg` (the server validates
  the actual PNG/JPEG data). WebP remains an export format. Do not hardcode `Images`,
  `PNG Image`, `JPEG Image`, or `WebP Image` there.
  Pixel `t` stays key-only. Leave File Manager and
  OpenSCAD byte formatters unchanged.
- Quick Connect SFTP status uses `desktop.qc_sftp_items`. SFTP sizes
  use `desktop.bytes`, `desktop.kib`, `desktop.mib`, `desktop.gib`,
  and `desktop.tib`. Do not hardcode `items` or `B`/`KiB`/`MiB`/`GiB`
  there.
- Quick Connect SFTP navigation uses one `createSFTPNavigator` per open panel
  (`quickconnect-sftp-navigator.js`, bundled before `menus-and-routing.js`). A newer listing aborts the
  older request, the shown path changes only after a successful listing, rows carry their absolute
  `data-path` for every action, and closing the panel or window disposes the navigator. Verify with
  `npm run test:ui-regressions`.
- `quickconnect-serial.js` owns the serial session controller: profiles, bounded
  in-memory RX/TX capture, ANSI/hex display and the browser/host connection
  lifecycle. `quickconnect-serial-model.js` (profile schema, hex codec, error
  keys), `-views.js` (markup) and `-transport.js` (control signals, teardown)
  precede it in `desktopMainParts`, all before the Quick Connect shell. Serial
  tests load the parts in that order through `readQuickConnectSerialSources`.
  Profiles use the versioned `quick_connect.serial_profiles` setting;
  payload bytes never reach logs, persistence or an LLM. See
  `documentation/quick-connect-serial.md` and `internal/desktop/AGENTS.md`.
- Quick Connect has one active connection per window. Serial, SSH and VNC switches
  invalidate old callbacks before cleanup; the Files tab remains SSH-only. Cancel
  pending device choices and close late opens after policy/auth/window disposal.
  Browser serial requires a trusted user gesture and always uses the browser's
  picker. No automatic reconnect. Embedded apps deny `serial` in their frame policy.
- Keep serial writes ordered, capture and render queues bounded, initial DTR/RTS
  off, and clear Break after 250 ms including cleanup races. Device traffic never
  counts as user activity. Verify `TestQuickConnectSerialTranslations`, serial
  browser tests, existing Quick Connect/SFTP checks and both themes at narrow and
  wide window sizes. USB adapter acceptance is a separate hardware check.
- Quick Connect synthetic AuraGo host uses `desktop.qc_aurago_host`
  and `desktop.qc_aurago_host_description`. Detect the host by
  `id === '__aurago-host__'`, matching IP, or the English sentinel
  `aurago host`. Do not hardcode `AuraGo Host` or
  `Current AuraGo web host` there. Shared `t(k, p)` interpolates
  `{{name}}`; call these keys without a second argument.
- Calculator backspace labels use `desktop.calc_back`. Do not hardcode
  English `Back` there.
- Calculator display maps the parser sentinel `Invalid expression` to
  `desktop.calc_invalid_expression`. Do not change the throw messages.
- Code Studio agent markdown copy buttons use `desktop.copy` and
  `desktop.copied`. Do not hardcode English Copy/Copied there.
- Code Studio status cursor text uses `codeStudio.cursorPosition` with
  `{{line}}` and `{{column}}`. Sidebar file sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`. Do
  not hardcode `Ln`/`Col` or `B`/`KiB`/`MiB` there.
- Code Studio terminal tabs and the xterm welcome line use
  `codeStudio.shell_n` with `{{n}}` plus `codeStudio.title`. The zen
  exit tooltip reuses `codeStudio.exitZen`. Do not hardcode English
  `Shell N`, `Code Studio - Shell`, or `Exit Zen Mode (Esc)` there.
- Shell new-file and new-folder prompt defaults use
  `desktop.new_file_default` and `desktop.new_folder`. Do not hardcode
  `untitled.txt` or `New Folder` in those prompts. Leave editor path
  fallbacks as `untitled.txt`.
- File Manager new-file template labels use `desktop.fm.new_file_kind_*`
  and `desktop.fm.new_file_template_label`. ZIP/rename success toasts use
  `desktop.fm.zip_created`, `desktop.fm.zip_extracted`, and
  `desktop.fm.batch_rename_success`. Preview and Quick Look chrome use
  `desktop.fm.preview_loading`, `desktop.fm.preview_unavailable`,
  `desktop.fm.quick_look_close`, and `desktop.fm.quick_look_error`.   The new-folder
  prompt default uses `desktop.fm.new_folder`. The new-file prompt and
  template filename default use `desktop.new_file_default`. Do not
  hardcode `new-file.txt` there.
- `renderAppError` shows `desktop.app_error_title` plus `err.message` or
  `desktop.app_error_fallback`. Do not hardcode English `Error` there.
- Calendar, Todo, and Gallery empty-state load failures use
  `desktop.load_failed`. Do not dump raw `err.message` there.
- Quick Connect device-list, generated-app host, and People content
  empty-state load failures reuse `desktop.load_failed`. Do not dump
  raw `err.message` there. People `t` takes `(context, key)`.
- Editor fallback file-list empty-state load failures reuse
  `desktop.load_failed`. Do not dump raw `err.message` there.
- Webamp unsupported-browser errors use
  `desktop.winamp_unsupported`. `notifyError` maps the English
  sentinel `Webamp is not supported in this browser.` and the
  localized throw; other load failures reuse
  `desktop.load_failed`. Do not dump raw `err.message` there.
  Leave the Webamp skin unchanged.
- Widget-frame and standalone-widget empty-state load failures reuse
  `desktop.load_failed`. Standalone Webamp notifications map the
  same unsupported sentinel and reuse `desktop.load_failed` for
  other load failures. Do not dump raw `err.message` there.
- Missing Agent Chat / Live Speech renderers use
  `desktop.app_error_renderer_missing` with `{{app}}`. Do not hardcode
  English "renderer is not loaded" strings.
- Agent Chat and Live Speech missing-host throws reuse
  `desktop.load_failed`. `renderAppError` may show that localized
  message. Do not hardcode English `Desktop chat window content is
  not available` or `Live Speech window content is not available`.
  Agent Chat uses `desktopText(key)` with no second argument. Live
  Speech `text` stays `(key, fallback)`; call this key without a
  fallback.
- Agent Chat generic request errors use `desktop.chat_request_failed`.
  Homepage Studio chat-stream failures reuse the same key, including
  the missing stream-parser throw. Chat live-stream fallbacks use
  `desktop.chat_live_stream`. Unknown document-format badges use
  `desktop.chat_document_format_unknown`. Do not hardcode English
  `Request failed`, `Chat stream parser not loaded`, `Live stream`,
  or `FILE` there.
- Viewer missing-markdown-it errors show `viewer.error` only. Do
  not hardcode English `markdown-it not loaded` there.
- Pixel image-decode failures throw `pixel.error_load`. Pixel `t`
  stays key-only. Do not hardcode English `Failed to load image`.
- Teevee catalog timeouts and HTTP failures show
  `desktop.teevee_catalog_error`. `fetchJSON` throws the sentinel
  `iptv-org HTTP` without a status and must not call `t()` (it is
  module-level). `loadCatalog` must not dump `err.message`. Call
  Teevee `t(key)` with no second argument. Do not hardcode English
  `Catalog request timed out` or `iptv-org HTTP ` plus a status.
  Leave playback `formatPlaybackError`.
- Viewer 3D missing-STLLoader errors throw and map to
  `viewer.error`. Map the English sentinel
  `Three.js STLLoader is unavailable`. Other init failures may
  still use `viewer.error` plus `err.message`.
- Game Maker missing-modals throws use
  `game_maker.modules_load_failed` via `state.context.t(key)` with
  no placeholder map. Do not change `fail()` itself. Do not
  hardcode English `Game Maker Studio modules failed to load`.
- Radio catalog HTTP throws the sentinel `Radio Browser HTTP`
  without a status. `loadActive` and `searchStations` show
  `desktop.radio_catalog_error`. Playback `play()` shows
  `desktop.radio_error` only, including the player toggle.
  Radio `t` stays key-only. Do not dump `err.message` in those
  catalog catches or the play catches.
- People save/delete notifies and Notes `notifyError` use
  `desktop.request_failed`. Keep Notes rename conflicts on
  `desktop.notes_rename_exists` via `notesCode`. People uses
  `t(context, key)`. Notes uses `state.t(key)` with no fallback
  string. Do not dump `err.message` there except the rename
  conflict.
- Pet Picker load, activate, settings, and import notifies use
  `desktop.request_failed`. Call Pet `t(key)` with no fallback
  string as the second argument. Leave `desktop.pet_import_invalid`
  for a bad ZIP name. Leave `pet-runtime.js` setting toasts.
- Viewer and Sheets missing print-frame errors use
  `desktop.print_failed`. Viewer still prefixes `viewer.error`
  plus the throw message. Sheets notifies and returns.
  Call `t(key)` with no fallback string. Do not hardcode English
  `print frame unavailable`.
- Notes, Writer, Sheets and Viewer share `AuraDesktopPrint` from
  `core/print-runtime.js`. Set iframe sandbox before insertion to
  `allow-same-origin allow-modals`, never allow scripts; only the parent prints.
  Preserve sanitization, await fonts/images, bind the frame to its owner signal,
  and translate timeout errors at the caller. Browser regression:
  `TestDesktopPrintSandboxBrowser`.
- Store container-app frame errors, terminal-preview frame errors,
  store start toasts, and external-open notifications reuse
  `desktop.load_failed`. Do not dump raw `err.message` there.
- Generated-app iframe title fallback uses
  `desktop.embed_frame_title`. Host SDK error fallback uses
  `desktop.embed_bridge_failed`. Do not hardcode English
  `Aura Desktop app` or `Desktop bridge request failed` there.
  Leave `aura-desktop-sdk.js` last-resort English when the parent
  sends no error text.
- Store terminal-preview module load and in-preview stylesheet/script
  loads use `desktop.store_terminal_load_failed`. Wrap AuraLazyAssets
  and the fallback `onerror` so the asset URL does not leak. Call
  `t(key)` with no second argument. Leave
  `desktop.store_terminal_module_unavailable` for a loaded module
  without `render`.
- Host clipboard throws use `desktop.clipboard_read_unavailable` and
  `desktop.clipboard_write_unavailable`. Do not hardcode English
  `Clipboard read/write is not available` there. Leave other SDK
  throws and `aura-desktop-sdk.js` last-resort English unchanged.
- Chess result-modal fallbacks use `desktop.chess_new_game` and
  `desktop.ok`. Pass `t` into `createChessFx`. Callers may still
  pass localized labels. Do not hardcode English `New game` or
  `OK` there.
- Chess opponent-move toasts and status use
  `desktop.chess_agent_unavailable`, `desktop.chess_agent_no_move`,
  `desktop.chess_engine_unavailable`, `desktop.chess_engine_no_move`,
  `desktop.chess_engine_worker_failed`, `desktop.chess_engine_timeout`,
  `desktop.chess_engine_illegal`, `desktop.chess_opponent_illegal`,
  or `desktop.chess_move_failed`. Map `chessCode` or known English
  sentinels. Do not show raw Stockfish, worker, HTTP, or
  `err.message` text there.
- Chess vendor-load status uses `desktop.chess_load_failed`.
  Do not show raw vendor `err.message` there. Leave the hint
  path and opponent-error mapper unchanged. The missing-template
  HTML fallback reuses the same key via `ctx.t` and `ctx.esc`.
  Do not hardcode English `Chess UI failed to load.` there.
- Homepage Studio local webhost name fallback uses
  `homepage_studio.default_name`. Do not hardcode English `Homepage`
  there.

### Trash restore contract

- Restore is File Manager only. The desktop trash icon keeps Open, Empty, and
  Properties — never a Restore action (no single target).
- Restore only moves paths under `trash/…` through the batch `/api/desktop/trash`
  operation. Ordinary legacy entries restore to `Desktop/<name>`. Notes under
  `Trash/Notes/<uuid>/<subpath>` restore to `Documents/Notes/<subpath>`.
  No origin is guessed for legacy ordinary entries. Conflicts require the shared
  Replace / Keep copy / Cancel dialog; replacement rechecks the observed version.
- Trash drops from desktop icons and File Manager preflight the entire selection
  server-side before moving the first entry; Notes roots and ancestors are protected.
- Readonly denies restore and empty-trash mutations. Delete inside Trash stays
  a permanent DELETE.
- Shell callbacks `restoreFromTrash` and `emptyTrash` are injected from
  `menus-and-routing.js`. Empty Trash in the File Manager empty-folder menu
  calls that callback; do not reimplement emptying in the File Manager.

### Gallery contract

- The Gallery is a lazy app. `module-loader.js` `DESKTOP_APP_ASSETS.gallery`
  loads `/css/desktop-app-gallery.css` and, in this order,
  `gallery-library.js` (`window.GalleryLibrary`: item model, prefs
  `aurago.desktop.gallery.v1`, `TILE_SIZES`, `countLabel`), `gallery-view.js`
  (`window.GalleryView`: shell/card/section/info HTML), `gallery-menus.js`
  (`window.GalleryMenus`), `gallery-lightbox.js` (`window.GalleryLightbox`)
  and `gallery.js` (`window.GalleryApp.render(host, windowId, ctx)` /
  `dispose(windowId)`). Keep every module below the desktop JS line budget.
- `planning-gallery-music.js` only delegates: `renderGallery` builds the ctx via
  `galleryAppContext(context)` (`t`, `esc`, `api`, `iconMarkup`, `fmtBytes`,
  `notify`, `mediaPreviewURL`, `mediaDownloadURL`, `mediaPreviewKind`,
  `readonly`, `pageSize`, `animationsEnabled`, window-menu and context-menu
  callbacks, `confirmDialog`, `promptDialog`, `settingBool`, `desktopSound`,
  `openApp`, `downloadMediaPath`, `afterFileChange`). `openMediaLightbox` is
  the shared image/video/audio viewer; `openMediaPreview` in
  `editor-filemenu.js` routes media there and keeps `openLegacyMediaPreview`
  for documents/iframes.
- Every window-menu and context-menu item must carry a stable `id` (`open`,
  `download`, `sort-<sort>`, `tab-photos`, ...). `normalizeWindowMenuItems`
  derives action keys from `id`, so items without one collide. Context menus
  do not render `checked`; `withCheckIcons` mirrors the checked state into the
  `check`/`square` icon.
- Interaction: plain click opens the lightbox, Ctrl/Cmd-click, Shift-click,
  the tile checkbox or selection mode select; arrow keys/Home/End navigate,
  Space toggles, Enter opens, Delete deletes, Ctrl/Cmd-A selects, Escape clears.
  Lightbox: arrow keys/PageUp/PageDown/Home/End navigate, `+`/`-`/`0`/`1`
  zoom, `I` info, `D` download, `F2` rename, Delete deletes, Space toggles the
  slideshow, Escape closes; chrome auto-hides via `is-idle`, and backdrop
  clicks within 450 ms of opening are ignored (tile double-click).
- Readonly omits rename/delete controls and shows the readonly hint instead of
  rendering disabled buttons. There is no upload because media mounts are
  server-side read-only.
- Width/height/duration are derived client-side from loaded `<img>`/`<video>`
  elements; `FileEntry` carries none. Live refresh listens to `desktop_changed`
  SSE payload paths, polls every 45 s and reloads on `visibilitychange`.
- Count strings use the `_one` singular convention through
  `GalleryLibrary.countLabel(t, key, count, vars)`:
  `desktop.gallery_item_count(_one)`, `desktop.gallery_status_summary(_one)`
  with `{{size}}`, `desktop.gallery_status_results(_one)` and
  `desktop.gallery_delete_failed(_one)`. Keep all `desktop.gallery_*` keys in
  every `ui/lang/desktop/*.json`.
- `.vd-gallery [hidden]` / `.vd-lightbox [hidden]` force `display: none`
  because author display rules otherwise beat the UA hidden default. Broken
  tiles (`.is-broken`) hide media, play glyph and badge.
- Verify with `go test ./ui -run 'Gallery|WindowMenu|ContextMenu'` and
  `go test ./internal/desktop -run RecursiveCacheInvalidated` (recursive list
  cache invalidation after desktop mutations). See
  `documentation/desktop-gallery.md` for the user-facing description.

### Video Studio contract

- Video Studio is a lazy app. Load `desktop-app-video-studio.css`, then
  `video-studio-preview.js` and `video-studio-timeline.js` before
  `video-studio.js`; the shell exposes `VideoStudioApp.render(host, id, ctx)`
  and `dispose(id)`. The preview owns its media elements and shared frame clock;
  dispose stops local playback, timers and requests but does not cancel
  server-owned jobs.
- Persist the canonical project with strong `If-Match` ETags. A 412/428 save
  must present Reload, Replace latest or Keep editing; never retry a conflict
  without an explicit choice and the observed latest ETag. Bind imports, jobs,
  project refreshes and draft recovery to the captured project ID and epoch.
  Window close awaits saving until clean or blocked by a visible failure or
  conflict choice.
- Timing and transitions use integer 30 fps frames. The outgoing clip owns an
  exact overlap with the next clip on its track. Titles use full-canvas PNG
  assets; static stickers are separate image overlays. Preserve editable
  text/style metadata and use only same-origin staged media URLs.
- Bound preview media to the active/near clips during playback and release clips
  that leave that window. Preserve both players for an actual transition overlap;
  backward seeks must rehydrate released media and dispose must release all media.
- Apply generated title artwork only if its project/selection, text, normalized
  style and source asset still match, and its monotonic Apply revision is current.
  Invalidate pending Applies on Undo/Redo and edits; stale results must not mutate
  history. Await PNG rendering before upload. Style comparison must ignore JSON
  key order and omitted default values.
- Keep editor shortcuts off native controls: Space on buttons/links remains native,
  default-prevented events and unknown Ctrl/Meta/Alt shortcuts are ignored, while
  drawer Escape/Tab focus handling runs before control guards. Ctrl+Z remains
  available for editor Undo without intercepting text-field Undo.
- Offer/send AI aspect ratio only when the provider supports it (MiniMax currently
  does not). Localize stable job error codes; a terminal
  `external_status_unknown` warning takes precedence over retry-safe failure text,
  but a running job must not show that warning.
- Keep the media bin, preview and inspector responsive; below 920 px the
  Inspector control opens a keyboard-dismissable drawer without hiding the
  timeline. Use the real Standard/Fruity theme tokens and all sixteen Desktop
  locale dictionaries. Verify UI behavior with
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopVideoStudioBrowser$'`.
- Without a project (zero projects or the feature switched off),
  `.vs-preview-stage.vs-no-project` hides the canvas and preview placeholder and
  gives the empty-project card the whole stage; `vs-stage` height container
  queries compact it so New project stays visible in a ~135px stage. `renderUI`
  clears the state once a project loads. `TestDesktopVideoStudioStartMenuBrowser`
  asserts the card is visibly hit-testable at 1280x800 and 1910x760 in both themes.

### Mission Control contract

- Mission Control is a master-detail workbench composed of
  `mission-control-schedule.js` (pure cron builder/describer),
  `mission-control-triggers.js` (grouped trigger catalog, picker, config
  panel), `mission-control-menus.js` (inline SVG icons, window + context menu
  builders), `mission-control-list.js`, `mission-control-detail.js`,
  `mission-control-editor.js` and the shell `mission-control.js`. Load order in
  `module-loader.js` is helpers first, shell last; there is no modal module.
  Keep `window.MissionControlApp = { render, dispose }` and every module below
  the desktop JS line budget.
- Mission Control UI text lives under `desktop.mc_*` (all 16 locales,
  `TestDesktopMissionControlTranslationsCoverAllLocales`); trigger labels and
  hints reuse `missions.*`, with leading legacy emoji stripped through
  `MissionControlTriggers.label`. Priorities use the emoji-free
  `desktop.mc_priority_*`, never `missions.form_priority_*`. Window menus use
  `desktop.menu_file` / `desktop.menu_view`; the trigger min-interval uses
  `desktop.rel_time_seconds` with `{{count}}`. Menu `icon` values are desktop
  icon keys (mini/Papirus); the inline `ICONS` map is for in-app markup only.
- The shell persists filter/sort/list width/collapsed state under
  `aurago.desktop.mission-control.prefs`, switches to single-pane below 720 px,
  registers a `beforeClose` dirty guard through `setWindowBeforeClose`, and
  scopes document shortcuts with `isActive()`. Local running missions can be
  cancelled via `POST /api/missions/v2/{id}/cancel`; `next_run` comes from the
  mission payload. The editor re-validates live after the first failed save.
- `.vd-mc [hidden]` forces `display: none` because module display rules
  otherwise beat the UA hidden default; keep the stylesheet theme-native
  (`--vd-theme-*`, `--vd-accent`) and free of dark-only literals.
- Verify with `go test ./ui -run 'MissionControl|RelTime'`,
  `npm run test:mission-control` (schedule, plus flow missions on a stub DOM in
  `scripts/test-mission-control-flows.mjs`), `npm run test:dashboard-cron` and
  the opt-in `TestDesktopMissionControlBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`).

### App theme bridge contract

- Everyday apps Writer/Sheets, Todo/Calendar, Settings, Calculator, Chat, File
  Manager, Quick Connect, Mission Control, Software Store, Gallery, Notes,
  Viewer, Looper, Cheater, People, Launchpad, Zipper, Pixel, Log Viewer,
  System Info, Pet Picker, Radio, Camera, Code Studio, Network Cameras,
  Noisemaker, Live Speech, Homepage Studio, Game Maker, OpenSCAD, the Webamp
  launcher, Chess chrome, and Nasscad read `--vd-theme-*` for chrome,
  panels, controls, borders, and shadows.
- Calculator programmer display (`.vd-calc-prog-display`) and history chrome
  use `--vd-theme-panel-bg` / `--vd-theme-border` / `--vd-theme-muted`. HEX,
  DEC, OCT, and BIN labels stay muted. Do not put `.vd-calc-base button.active`
  or `.vd-calc-tabs button.active` in the control `!important` bridge.
- Writer page content may stay on a light paper surface (`--vd-editor-page-bg`);
  toolbars, status bars, and sheet chrome follow the active desktop theme.
- Chat user bubbles may keep an accent wash (`--vd-theme-accent-soft`); agent
  bubbles use theme panel material instead of fixed white glass.
- Quick Connect xterm/VNC viewports and the built-in Terminal screen may stay
  on a dark terminal surface (`#0d1117` / `#0f172a`); toolbars follow theme.
- Gallery thumbnails (`.vd-gallery-thumb`), the info-panel preview
  (`.vd-gallery-info-preview`) and the lightbox stage (`.vd-lightbox-stage`)
  may stay dark for media contrast; card chrome, toolbar, info panel and the
  lightbox bar follow theme.
- Cheater code blocks keep a dark readable code surface (`--cheater-code-bg` /
  `--cheater-code-fg`); app chrome uses `--vd-theme-*` through `--cheater-*`.
- People status badges keep semantic colors (`#e8a020` / `#6495ed` / `#32cd32`);
  danger stays `#e74c3c`. Accent-on-white remains on primary People buttons.
- Zipper local `--zipper-*` aliases map to `--vd-theme-*`; the in-app preview
  overlay uses `--vd-theme-panel-bg`.
- Pixel canvas, checkerboard, and image viewport stay a dark work surface
  (`#0e1117`); toolbars, rails, panels, and dialogs follow theme via
  `--pixel-*` aliases.
- Log Viewer level colors and log-line semantics stay readable; toolbar,
  sidebar, and pane chrome use theme tokens. System Info gauge and chart
  accents may stay.
- Radio owns a theme-independent wood/champagne-metal receiver skin, including
  the existing window chrome scoped to `.vd-window[data-app-id="radio"]`.
  Material assets live under `ui/img/radio/`; retain real shell menus/resize
  handlers. Tuning selects current results without playing until activation.
  Late catalog/stream results and disposal must not resume playback. Favorites
  remain available when empty; meters animate only during playback and respect
  reduced motion. Verify with `TestDesktopRadioBrowser`.
- RTL-SDR owns a theme-independent graphite receiver skin scoped under
  `.sdr-app` with local `--sdr-*` tokens and its own label/display faces. Keep
  all `.sdr-*` out of the theme bridge; the shell window chrome stays native.
- Camera viewport stays black (`#000`) for live preview; toolbar and controls
  use `--cam-*` aliases mapped to `--vd-theme-*`. Error banner keeps semantic
  danger colors.
- Code Studio editor/terminal surfaces follow `--cs-*` aliases mapped to
  `--vd-theme-*`; CodeMirror theme selection stays in `code-studio/editor.js`.
  Editor text defaults to 12px in CodeMirror and the textarea fallback; saved
  zoom preferences remain authoritative and View > Reset Zoom restores 12px.
- Network Cameras live tiles and detail video stay dark viewports (`#05080d` /
  `#030509`); toolbar, cards, and modal chrome use `--nc-*` aliases mapped to
  `--vd-theme-*`. Online/offline and danger badges stay semantic.
- Noisemaker keeps the brand pink/purple gradient (`--nm-accent-2`) on create
  and play controls; chrome, library cards, and the player bar use `--nm-*`
  aliases mapped to `--vd-theme-*`. Cover play overlays may stay dark.
- Live Speech canvas FX stay decorative; the lab panel and realtime surface
  use `--vd-theme-panel-bg`.
- Homepage Studio preview letterbox (`.vd-hp-preview-zone`) and the website
  paper panel (`#f8fafc`) stay work surfaces; chrome uses `--hp-*` aliases
  mapped to `--vd-theme-*`. Icon filters stay light/dark aware. History type
  colors stay semantic.
- Game Maker preview checkerboard and iframe stay dark (`#0d0e14` / `#050509`);
  chrome uses `--gm-*` aliases mapped to `--vd-theme-*`. Brand purple/teal
  (`--gm-accent`, `--gm-accent-2`) and status badges stay.
- OpenSCAD chrome uses `--oscad-*` aliases mapped to `--vd-theme-*`. The 3D
  preview letterbox and panel stay dark work surfaces (`#071018` / `#050a10`);
  `light-preview` may flip the panel to `#f2f6f8`. Warm/danger and accent-on
  `#061014` stay semantic. Icon filters stay light/dark aware. Do not put
  `.oscad-primary` in the control `!important` bridge.
- Webamp launcher chrome uses `--vd-theme-*`. The embedded Winamp player skin
  stays authentic and stays out of this bridge.
- TeeVee is a theme-independent wood/metal CRT receiver. Its visual source is
  `documentation/assets/teevee-retro-reference.png`; retain the real shell menus
  and window actions. Keep all `.teevee-*` out of the theme bridge.
  CRT bezel/tube proportions follow the reference at every window size; fit the
  initial receiver bounds proportionally to the desktop, then enforce the
  1140x540 minimum so the left sidebar stays visible, including session restores.
  Only desktops smaller than that minimum may use the compact layout; the tube
  remains proportional independently of window bounds.
  Free header space, including the decorative model logo, stays draggable.
  Hide the on-tube fullscreen button two seconds after the first `playing` event
  per stream; tube hover or keyboard focus reveals it. Reset the timer on source
  reset/stop/disposal, and preserve this behavior with reduced motion enabled.
  Material and icon provenance lives in `ui/img/teevee/README.md`. CRT and glass
  have separate, persisted View switches. The one existing video element owns decoding/audio;
  `teevee-crt.js` owns only rendering. Blocked texture access or WebGL failure
  preserves native playback with a labelled basic filter. Never force CORS or
  globally proxy all streams to enable effects. Explicit reconnect applies only
  to the current station. Cancel renderer work while hidden/minimized and dispose
  callbacks, observers and GPU resources. Keep `playbackID` and catalog guards.
- Chess chrome uses `--chess-*` aliases mapped to `--vd-theme-*`. The wood
  frame (`--chess-board-frame*`) and felt (`--chess-felt`) stay the board
  surface. Warn/danger/good stay semantic.
- Nasscad shell uses `--vd-theme-app-bg`; the bundled iframe viewport stays
  `#111318`.
- Sysworld HUD uses `--sw-*` aliases mapped to `--vd-theme-*`. The 3D canvas
  and vignette stay a dark work surface (`#020208`). Brand cyan
  (`--sw-accent`), event/tone semantics, and `.sw-btn.active` stay. Do not put
  `.sw-btn.active` in the control `!important` bridge.
- Galaxa, Quake, and SIP-phone hardware chrome remain excluded.

### Desktop Phase 3 contract

- Standard-theme taskbar window buttons and Fruity dock app buttons show a
  hover/focus thumbnail preview (`.vd-taskbar-thumbnail`) for windows on the
  active space only. Compact viewport and coarse-pointer layouts disable
  previews. Dock hover must not call `findExistingAppWindow` (that switches
  spaces). Win/Meta+Arrow snaps the active window; Ctrl+Alt+Arrow keeps
  cycling spaces.
- Thumbnails clone DOM window content when possible; iframe-heavy windows show
  a live-window fallback instead of a blank capture.
- `AuraDesktopMediaSession` owns OS media handlers/metadata. Registrations are
  owner-bound; disposal releases only that owner. Personal Radio has priority
  over Webamp, then the latest active Radio/TeeVee/RTL-SDR player. Metadata
  refreshes do not steal ownership. Radio direct streams use native no-CORS
  audio; background TeeVee playback remains intentional. No global volume control.
- Chess disposes its AudioContext and scheduled tones. Galaxa cancels RAF while
  inactive and restarts at most one loop on visibility/focus return. Gallery
  errors use translated messages; Detective renders invalid URLs as text.

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
  scoped under `.vd-logviewer`. The virtualized log scroller must keep
  native incremental wheel movement in both directions; disable browser
  scroll anchoring where rows are replaced. Visible strings use
  `desktop.app_log_viewer` plus `desktop.log_viewer_*` in all 16
  `ui/lang/desktop/*.json` files. File-list sizes use `desktop.bytes`,
  `desktop.kib`, `desktop.mib`, `desktop.gib`, and `desktop.tib`.
  Download is hidden when the desktop
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
- `writer.js` owns Autor's native DOCX document, menus and lifecycle;
  `writer-session.js` owns revision-bound autosave and IndexedDB recovery;
  `writer-panels.js` owns formatting, search, review, AI and snapshot printing.
  The Apache-2.0 core and local fonts are pinned to 2.23.0 and generated by
  `scripts/build-writer-vendor.js`. Do not add Pro packages or Quill to Autor.
  Opening a DOCX that a Writer window already shows (Files, the agent's
  `open_in_app`) focuses it and calls `reloadIfChanged()`: the window reloads
  when the file's ETag moved since load or the last save, and unsaved edits are
  never replaced (conflict notice instead). Markdown/HTML/TXT open as a new
  import copy, so their windows never match a re-open. Verify
  `TestDesktopWriterReloadsChangedFileOnReopen` (`AURAGO_RUN_BROWSER_SMOKE=1`).
- The Calendar is three continuations inside the shared Desktop IIFE, bundled
  in this order after `core/media-keys-runtime.js`: `calendar-views.js` (date
  math, `Intl` formatting, pure HTML renderers for month grid, week/day time
  grid, agenda, mini month, sidebar agenda and skeletons), `calendar-editor.js`
  (appointment editor dialog, quick-peek popover, recurring creation, contact
  participant picker) and `calendar.js` (per-window session state, incremental
  `paintCalendar`, navigation, keyboard, drag/drop, snackbar with undo, window
  menus and cleanup). None of them is loaded lazily. Styles live in
  `ui/css/desktop-app-calendar.css`, registered for `calendar` in
  `module-loader.js`; `desktop-app-planning.css` owns only Todo and the music
  player and must not regain `.vd-calendar-*` rules.
- Calendar contracts: views are `day`, `week`, `month`, `agenda`; view,
  sidebar and completed/cancelled filters persist in localStorage
  `aurago.desktop.calendar.prefs`. Rendering is incremental per session (no
  full shell re-render); loading uses skeletons, failures use
  `desktop.load_failed` plus a retry button. Reschedule (drag/drop, 15-minute
  snap in time columns, time kept on month cells), status changes and deletes
  are optimistic with rollback and an inline `[data-cal-snackbar]` undo; they
  never call `showDesktopNotification`. Overdue is server-derived: moving an
  overdue appointment into the future sends `status: 'upcoming'`. Reminder
  presets map to `notification_at`; enabling "wake the agent" forces a reminder.
  Single-click on a day/slot opens the editor; clicking an event opens the peek
  popover inside `.vd-calendar-shell`. Keyboard: `T`/`Home` today, `N` new,
  `D`/`W`/`M`/`A` views, `/` search, arrows/PageUp/PageDown navigate, arrows on
  month cells move the roving selection, `Esc` clears search. Searching
  switches to the agenda view and restores the previous view when cleared.
  All shell clicks go through one delegated handler in `wireCalendarShell`
  that matches `data-cal-*` selectors with `closest`, so the shell element
  itself must never carry one of those attributes: the active view is exposed
  as `data-cal-mode` and `data-cal-view` belongs to the toolbar tabs only. The
  editor is appended to `document.body` as a modal; a document-level
  capture `keydown` closes it on `Escape` regardless of focus and is removed
  in `closeCalendarEditor`. The stylesheet keeps `[hidden]` authoritative
  inside `.vd-calendar-shell`, `.vd-calendar-editor` and `.vd-calendar-peek`
  because several fields use flex/grid classes. Verify with
  `go test ./ui -run TestDesktopCalendar` (the browser flow needs
  `AURAGO_RUN_BROWSER_SMOKE=1`).
- `cheater*.js` implements the Cheater app, a cheat-sheet manager with a
  textarea-based Markdown editor, live preview, Markdown toolbar, command
  palette (spotlight), and attachments side panel.
- `sheets.js` owns Tabellen's Autor-style chrome, document lifecycle and native
  Univer OSS 1.0.3 instance. `sheets-data.js` owns localized inputs, CSV and
  templates; `sheets-panels.js` owns formatting, data, search, AI and print;
  `sheets-charts.js` renders five editable Chart.js chart types on the sheet.
- `code-studio/*.js` implements Code Studio, a full IDE with file explorer,
  CodeMirror editor, terminal, search, agent chat, Git integration, a synced
  split editor, and keyboard shortcuts. Split across `core.js`
  (state management, API client, lifecycle, shell, tabs, status bar, dialogs,
  shared context menu), `sidebar.js` (keyboard-navigable file tree, path
  crumbs, tree context menu), `editor.js` (CodeMirror/textarea views),
  `terminal.js` (xterm.js sessions, ResizeObserver fit), `search.js`
  (search-in-files), `agent.js` (agent chat, markdown, diff preview),
  `git.js` (Git panel, closable diff view, commit), `panels.js` (two synced
  split panes), `shortcuts.js` (keyboard shortcuts, command table,
  window.CodeStudioApp), and `command-palette.js` (separate IIFE).
- `sysworld*.js` implements System World's Blender-authored data metropolis:
  seven fixed districts, live read-only inspector, entity search, map, explicit
  tour and street-level WASD/touch exploration. The isolated Three.js 0.186.1
  ESM renderer does not replace the legacy global used by other apps. The
  moon is a sky-dome disc, never a billboard quad. Opens
  maximized; existing dashboard/KG/mission APIs and shared SSE supply live data.
  Persistent 24-hour history remains a subsequent stage in
  `documentation/system-world-plan.md`.
- `looper.js` plus `looper-monitor.js` implement Looper v2. See the Looper
  contract in the Child DOX Index.
- `game-maker-studio-api.js`, `game-maker-studio-preview.js`,
  `game-maker-studio-modals.js`, and `game-maker-studio.js` implement Game
  Maker Studio: a project library, 2D/3D creation dialog with idea chips,
  bounded agent progress (job banner with phase, elapsed time, and repair-pass
  indicator plus a phase stepper), result cards for ready/failed jobs,
  revision history, change requests, and a live game preview in one maximized
  desktop window.
- `live-speech.js` mounts the shared realtime-speech panel on the desktop in a
  compact window (preset 440×520, min 340×460 in
  `window-shell-runtime.js`; panel mounted with `compact: true`).
  The desktop header's speaker control lives beside the FX toggle and opens
  the shared audio settings dialog; pass its `[data-live-speech-audio-controls]`
  host through the mount `audioControls` option.
  The shared panel places its animated persona beside wrapping, scrollable
  captions; small windows scroll vertically. Avatar disposal uses the existing
  panel unmount and preserves `keepSession`. The decorative FX remain separate
  from the avatar and never control its mouth from microphone levels.
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

- MeshCore loads `meshcore-device.js` before `meshcore.js`. The companion module
  owns Device/Settings pages and reception/contact details. Settings use topic
  navigation (a native select in narrow windows); inactive forms stay mounted so
  drafts survive section/page changes and refresh. Show disabled reasons and
  readback conflicts/partial results; keep uncertain-state reconciliation visible
  above every section. Radio edits require a before/after confirmation. The Device
  overview separates identity, capacity, radio/features and timestamped diagnostics;
  unavailable values remain unknown. Local diagnostics poll only on the visible
  Device page, at least 30 seconds apart; remote diagnostics start only from
  explicit contact actions and poll bounded server jobs. Settings drafts and
  diagnostics never enter browser storage. Keep device flags/favorites distinct
  from Messenger favorites and agent trust. Dispose timers, requests and dialogs;
  verify both themes and narrow windows with `TestDesktopMeshCoreDeviceBrowser`.
  Firmware/API/privacy contracts are owned by `internal/meshcore/AGENTS.md`.

- Agent Chat displays transient typed `llm_route` metadata through the shared
  `AuraLLMRouteBadge` helper. Update the originating turn's badge on fallback,
  render model/provider strings as text, and keep metadata out of answer bubbles
  and persisted chat history. Runtime routing is owned by `internal/agent`.

- Built-in app load order is defined in `ui/js/desktop/core/module-loader.js`.
- `calendar-views.js`, `calendar-editor.js` and `calendar.js` are the Calendar
  source of truth. `desktopMainParts` in `scripts/build-ui-bundles.js` must
  keep exactly that order, after `core/media-keys-runtime.js` and before
  `core/menus-and-routing.js`, so `renderCalendar` and its helpers stay inside
  the shared Desktop runtime closure without duplication.
- Game Maker Studio loads in the order `game-maker-studio-api.js`,
  `game-maker-studio-activity.js` (`window.GameMakerStudioActivity`: per-window
  background progress terminal),
  `game-maker-studio-preview.js` (`window.GameMakerStudioPreview`: loading
  overlay, stale badge, fullscreen, new-tab), `game-maker-studio-modals.js`
  (`window.GameMakerStudioModals`: shared modal lifecycle, media toggles,
  skills and revisions; confirmation stays shell-mediated),
  `game-maker-studio-assets.js` (`window.GameMakerStudioAssets`: offline pack
  catalog, selection, sprite/animation previews), then
  `game-maker-studio.js`.
- Game Maker Studio lays out by container, not by viewport: `.gm-studio` is the
  `gm-studio` inline-size container and `.gm-preview-pane` the `gm-preview`
  container. Every width rule exists as `@container` and as its `@media` twin,
  because fixtures render panes outside the shell; change both together. Below
  980px the library is hidden, so the project select and `.gm-narrow-new` in the
  agent pane head are the only way to switch or create. Keep the library button
  the first `[data-gm-action="new"]` in document order and update all of them in
  `applyCapabilities`. Only `.gm-conversation` may stretch in the agent pane;
  banner and notice come and go. Dialogs size against the Studio window, pin
  header and footer with offsets of `-var(--gm-modal-pad)`, and `modalError`
  inserts above the footer. Signal colours use the `--gm-tone-*` variables, never
  fixed light tints. Failure cards show `game_maker.failure_hint_<cause>` and
  keep the backend text inside `details`; library states use
  `game_maker.status_<status>`. Verify with `TestGameMakerStudioLayoutBrowser`
  (`GAMEMAKER_STUDIO_REPORTS` writes screenshots).
- Game Maker Studio exposes `window.GameMakerStudioApp = { render, dispose,
  instances }`. Every window owns and closes its EventSource, preview iframe,
  channel ID, diagnostics, modal handlers, job-elapsed and busy-poll timers,
  document-level overflow-menu listeners, and `message` listener.
- Voxel is the third creation choice (`dimension: 3d, variant: voxel`), with
  localized ideas/badges/capabilities. These fields stay immutable after creation.
  `game-maker-studio-preview.js` forwards only load/save/reset from the exact
  current frame/channel/project/published revision, using a parent-only grant.
  Reject oversized/unknown messages; expose conflicts and keep drafts/validation
  temporary. Flush before restore mutations and before project/frame replacement;
  failed flushes require explicit discard through the existing shell confirmation.
  External published revisions must pass that same resolution before replacing a
  Voxel frame. Restore controls are disabled with the localized read-only reason
  when `capabilities.allow_edit` is false.
- Voxel new-window play uses verified `game-maker-player.html` and
  `game-maker-player.js` with the same bridge and opaque iframe. Keep credentials
  out of iframe messages, source URLs and exported games. A play-state GET 409
  marks the revision-bound grant stale; recovery refreshes the trusted host on
  initial-load races and the explicit Load latest action. Same-revision save CAS
  conflicts still recover in place through Load latest. Verify
  `TestGameMakerVoxelBridgeBrowser` alongside the existing Game Maker UI tests.
- Scene diagnostics use the current preview channel and parent-window binding.
  The toggle exposes boundaries, colliders, IDs and routes only in Studio; ZIP
  exports never enable the overlay. Keep rule evidence separate from startup
  status and display unverified custom gameplay honestly. The shared scene and
  mechanics helpers use the existing engine loop, pause and disposal owners.
  Verify with `GAMEMAKER_BUILDER_BROWSER=1` and `TestBuilderBrowser`, including
  actual inputs, repeated resource counts, exported games and narrow layouts.
- Sprite selection is window-local and prepares the next create/edit request
  via `asset_pack_ids`; selecting packs never starts a job. Clear selection only
  after job acceptance. The asset browser owns an AbortController and animation
  timer, released on modal replacement/close and disposal. Metadata stays English
  for agents; controls and pack titles cover all 16 locales. Preview backgrounds
  are CSS only: PNGs contain genuine alpha, no painted checkerboard.
- Modular pack previews default to a complete assembly. Parts use metadata
  coordinates and shared animation time; selecting an individual sprite exits
  assembly mode. Reuse the window's existing preview timer and cleanup path.
- Game Maker previews must use `sandbox="allow-scripts allow-pointer-lock"` without
  `allow-same-origin` (`allowfullscreen` on the iframe is permitted).
  Preview HTTP responses also send CSP `sandbox allow-scripts allow-pointer-lock` so "open in
  new tab" cannot become a first-party AuraGo origin.
  Accept diagnostics only from the instance iframe when `event.source`, the
  random channel ID, the fixed source marker, and the bounded event type all
  match. Forward bounded, deduplicated reports through the authenticated
  `preview-report` API using the parent-held preview token; never grant the
  iframe API credentials. Reports bind to a specific validation build. Forward
  at most five sanitized dist/game.js line/column frames; strip other fields,
  paths and invalid coordinates. The backend owns source mapping. Verify with
  `TestGameMakerDiagnosticFramesBrowser` and `AURAGO_RUN_BROWSER_SMOKE=1`.
  Include current-preview errors as untrusted diagnostics in the next change request.
  Readiness requires the injected boot's `boot: true, visible: true` report;
  game-authored ready messages alone never qualify browser validation.
  Full gameplay grants contain bounded scenarios. Send them only to this iframe;
  forward at most 16 observations and two 700,000-character PNG data URLs through
  the existing authenticated parent report route. Never accept JavaScript test
  expressions or an iframe-authored pass verdict. Validation and advisory image
  results use localized SSE activity; planning/model deltas stay internal until
  the server publishes a verified revision.
  `game-maker-studio-preview.js` owns the message bridge and its report limits;
  the app supplies its instance state and diagnostic callback.
  Clear the visible diagnostics, error badge, and next-request diagnostics when
  replacing a preview or opening a project. Late responses from an old preview
  must not repopulate them; a ready message never clears current-run errors.
  Stop validation reports when the grant expires or its job reaches a terminal
  state, including late API failures. Published-game runtime diagnostics remain
  visible locally.
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
- The preview placeholder's adventure terminal consumes only public phase,
  allowlisted tool names, file/asset updates and check status events. Never feed
  it model deltas, reasoning, arguments or diagnostic bodies. Render values as
  bounded text, deduplicate replay IDs and scope them to the current job/project.
  Keep the existing icon/copy stationary. Limit pending lines to 32 and DOM rows
  to 24; suspend typing while hidden, offscreen, inactive or showing a game iframe.
  Reduced motion reveals complete lines without a blinking cursor. Project
  changes and disposal clear its timeout and history; disposal also disconnects
  intersection/resize observers and visibility/media listeners.
  Verify with `TestGameMakerActivityTerminalBrowser`; optional screenshots use
  `AURAGO_GAME_TERMINAL_SCREENSHOTS` under ignored `reports/`.
- Allowlisted `model_progress` statuses show waiting, receiving, retry and code
  recovery without exposing model text. Failed/cancelled runs retain a static
  terminal transcript with no animation timer. Recreate the activity controller
  whenever deleting a project replaces the shell, so later jobs target the new
  preview node. Keep all 16 desktop locales aligned.
- Render terminal text at 16px (12px on narrow views) in local Press Start 2P with full opacity,
  warm yellow fill and a narrow dark outline. Do not use gray fill, white outlines, text strokes or scanlines
  over glyphs; keep the stage-card fade localized so surrounding lines stay legible.
- EventSource open restores status even without replayed events; stale callbacks
  cannot affect another project or disposed window. Terminal job status survives
  reconnects and project refreshes. Cancellation shows its server reason and retry
  action, with no active phase. Verify with `TestGameMakerEventsReconnectBrowser`.
- Game Maker retry buttons submit `resume: true` and
  `validate_restored_draft: true` directly through the existing
  job-start flow, even after reopening the app. Preserve unsent editor text,
  lock immediately against duplicate requests, and retain normal permission
  and global-job gates. Private reasoning stays server-side, outside the
  progress terminal and chat.
- Game Maker visible strings use `game_maker.*` plus
  `desktop.app_game_maker_studio` in all 16 `ui/lang/desktop/*.json` files.
  Missing skills/revisions modals throw `game_maker.modules_load_failed`.
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
- `galaxa-campaign.js` owns seeded run randomness, the pause-aware game
  scheduler, 30-stage sector waves and six sector-boss encounters. Classic,
  Mirror and Daily use `GC.getStagePlan`; Boss Rush uses those six bosses.
  Gauntlet remains twelve waves; Endless and Hyperdrive retain their spawners.
  Sector bosses use `sectorBoss` and three HP phases; legacy `boss` enemies
  remain formation commanders. Route all damage through `ctx.damageEnemy`
  and finalize sector-boss rewards/destruction once in `updateCampaign`.
- `galaxa-sprites.js` preloads and validates local `img/galaxa/atlas.json`
  and PNG sheets before the title screen. All atlas requests use the page's
  BuildVersion; fixed art revisions fail the resource server's version check.
  Verify with `node scripts/test-galaxa-assets.mjs`. Animation rectangles, durations,
  loops and anchors belong in that manifest; never restore inline pixel
  definitions or fixed frame counts. Collision geometry is independent of art.
  Sprite canvases have a 768-entry cap. Asset failures expose localized retry.
- `ctx.stepFrame` runs a fixed 60 Hz simulation independently of rendering.
  Use `ctx.scheduleGame`, not wall-clock timers, for combat transitions.
  `resetRun` resets the run generation and seeded RNG; delayed work cannot
  cross run/disposal boundaries. Desktop `isActive` gates all controls;
  focus loss pauses and releases held input. The 540x720 field fits the
  available viewport at DPR 1/2 without cropping; touch has Parry/Super buttons.
- New gameplay audio is synthesized. Separate music/SFX buses feed the
  master compressor; SFX have a 32-voice priority cap and cached noise.
  Music is scheduled against the audio clock with 120 ms lookahead and
  bar-boundary transitions. Keep `img/audio/galaga.mp3` unchanged for title
  and demo, and stop it before synthesized gameplay. Persist volume zero.
- Arcade acceptance: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run
  'Galaxa|AdaptiveMusic' -count=1`; the 30-minute real-time browser run also
  needs `AURAGO_GALAXA_SOAK_SECONDS=1800` and Go `-timeout 35m`.
  `AURAGO_BROWSER_ARTIFACT_DIR` optionally saves screenshots/audio/metrics.
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
  `/js/vendor/stockfish/stockfish-19-lite-single.js` and browser-side agent
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
- Sheets exposes `window.SheetsApp = { render, dispose, instances }`. Each
  window owns its engine/worker, requests, panels, chart canvases, operation
  journal and save queue. Load OfficeSession, data, panels and charts before
  sheets.js. The local vendor build imports only Apache-2.0 Univer OSS 1.0.3.
- Failed workbook loads release the editor/save queue and show New, Open and
  Retry actions with an error status. A confirmed 404 clears the shell's stored
  path; temporary failures retain it for retry. New uses a fresh create-only
  path and never recreates the missing file. Verify `TestDesktopSheetsLoadRecoveryBrowser`.
- Use the engine's formula, selection, clipboard, structural-reference and undo
  APIs. Do not restore the removed HTML grid or browser formula evaluator.
  Expand shared formulas before persistence, preserve forced strings, and parse
  native edits through the same locale-aware path as the formula bar.
- Current engine selection takes priority over a pinned toolbar fallback.
  Await clipboard commands; format disjoint selections as one native command.
  Formula recalculation events must not dirty the workbook.
- PATCH editor-v2 uses ETag preconditions and an original XLSX source. Keep the
  structure journal until its own save succeeds; stale responses cannot clear
  newer edits. Save As and recovery copies preserve the original source bytes.
  Shared OfficeSession autosaves at 800 ms / five seconds and owns IndexedDB
  drafts separately for Writer and Sheets. Dispose all document resources.
- Use native chart/print mutations on the same undo stack. Charts remain real
  OOXML charts; unsupported chart types and workbook references are preserved
  and guarded. AI only proposes bounded changes to the explicit selection;
  apply once through native undo, and reject stale revisions.
- The sheet stays light in every desktop theme. Use shared desktop menu/icons,
  16 desktop locales, local fonts and a <900px overlay inspector. Print/export
  captures current edits and waits for calculation, without moving selection.
- Writer exposes `window.WriterApp = { render, dispose, instances }`. Each window
  owns its editor, abort controllers, panels, draft and serial save queue.
  Autosave waits 800 ms, with a five-second ceiling during continuous typing.
  Acknowledgements cover only the captured revision. Suspend the queue during
  Save As; native writes require ETag preconditions. Keep the async close guard
  installed until cleanup and never replace failed loads with blank content.
- Writer exposes New/Open after load failures and clears the persisted window
  path only on a confirmed 404. Transient errors retain it for Retry. Verify
  `TestDesktopFileLoadRecoveryBrowser` alongside the Sheets recovery check.
- Writer pointer selection must preserve the viewport, including clicks near its
  edges after toolbar focus. Core 2.23.0 supplies this behavior upstream; keep
  keyboard/programmatic reveal and drag edge autoscroll enabled. Retain the
  browser regressions when rebuilding or upgrading the core.
- Writer opens new, loaded and recovered documents at the first paragraph with
  the viewport at the top. Restore typing focus only in the active editable
  window. Verify `TestDesktopWriterAppBrowser` and the Writer shell matrix.
- Writer font-size controls display points and convert to integer half-points at
  the command boundary, including command availability checks. Reject invalid
  inputs before editing. Map app actions to existing shared icon keys; action IDs
  are not automatically icon names.
- Writer visible UI strings use `desktop.writer_*` keys in all
  `ui/lang/desktop/*.json` files. New keys require translations across all 16
  supported languages.
- Writer search uses the core's semantic matches and replace commands, preserving
  document formatting and undo. Review uses public EditorModule/store contracts.
  Structural commands without reliable tracked revisions stay disabled while
  tracking. AI suggestions bind to the source revision and require explicit apply.
  Print and exports serialize the current editor, never stale server content.
  Unknown package parts and authored font names must survive native saves.
- Code Studio exposes `window.CodeStudioApp = { render, dispose, state, instances,
  api, command, knownFiles, loadState, saveState, refreshFiles, openFile, openPath,
  openFileFromDialog, saveCurrentFile, uploadFile, downloadFile }`. All
  non-command-palette modules share a single IIFE closure; `core.js` opens the
  IIFE, `shortcuts.js` closes it. Function declarations are hoisted across the
  entire IIFE scope. All `const`/`let` declarations must stay in `core.js` (the
  first module in the bundle load order).
- The command palette drives Code Studio only through
  `CodeStudioApp.command(name, args)` (table in `shortcuts.js`) and lists tree
  files from `CodeStudioApp.knownFiles()`; never dispatch synthetic key events
  or click hidden buttons from the palette.
- Code Studio binds shared `state` only during synchronous work. Async handlers
  capture their instance, reject disposed/superseded requests, and explicitly
  bind UI updates after awaits. Never leave `state` switched while awaiting.
- Save, Save All and Run share a per-tab serialized save queue. A completion
  clears dirty state only for the saved buffer; later edits remain intact.
  Rename/delete wait for pending saves and block new saves until the path change
  completes. Run retains its document and terminal session and stops after a
  failed/superseded save or a removed/renamed document.
- Code Studio AI suggestions retain their window, tab identity, path, revision,
  range and action intent. Only a single code block from Refactor/Comments may
  replace source; Apply revalidates the original document and uses one editor
  transaction. Other output and stale suggestions stay copyable. Cancelled or
  superseded replies cannot change messages, busy state or pending suggestions.
- Code Studio receives the office app context (`officeAppContext` in
  `menus-and-routing.js`): `setWindowBeforeClose`, `showContextMenu`,
  `promptDialog`, `confirmDialog`, `notify`. Modified tabs block tab close,
  close-others/all and window close until the user confirms with
  `codeStudio.unsavedClosePrompt` / `unsavedTabsPrompt` / `unsavedWindowPrompt`.
- `.code-studio-body` and `.code-studio-main` are flex layouts; hidden
  sidebar/agent/git/terminal panels use `display: none`. Never model panel
  visibility with collapsed grid tracks (zen mode and the Git panel must keep
  the editor visible).
- Explorer rows are `role="treeitem"` with `tabindex="0"`: arrows, Home/End,
  Enter, F2 and Delete work; the active tab is highlighted; right-click opens
  the shared context menu. New file/folder/upload target `targetDirectory()`
  (the selected, expanded folder) and reload only that directory via
  `reloadTreeDirectory`. Typing `?` inside inputs or the editor must not open
  the shortcut overlay; Ctrl+= / Ctrl+- / Ctrl+0 and Ctrl+wheel zoom the editor.
- Split Right/Down renders two synced CodeMirror views of the active tab
  (`tab.view`, `tab.secondaryView`, `tab.views`); `renderEditor` re-applies the
  split on tab changes. `destroyTabView` retains editor state, selection and
  scroll while disposing view DOM/listeners; closing a tab releases snapshots.
  CodeMirror presentation is reconfigured through a compartment. Only the
  primary pane owns history; secondary undo/redo commands use that history.
  AI context uses the last focused pane's selection.
- Initial and reused Code Studio launches use `openPath`, resolving directories
  before reading files. `openFile` remains the file-specific API. Launch errors
  use the existing Code Studio status/error surface.
- The Code Studio terminal refits through a ResizeObserver and sends
  `{"type":"resize","cols","rows"}` JSON, which the server line terminal ignores.
  The active session owns terminal/WebSocket/fit aliases. Callbacks retain the
  session object across tab removal; delayed Run output never moves to another
  terminal when its original session closes.
- Browser acceptance: `TestDesktopCodeStudioBrowser` renders the real shell,
  bundle, CodeMirror and xterm against an in-memory backend (no Docker) with
  `AURAGO_RUN_BROWSER_SMOKE=1`; set `AURAGO_BROWSER_ARTIFACT_DIR` for
  screenshots.
- Code Studio bundle load order in `scripts/build-ui-bundles.js` must be:
  core.js, sidebar.js, editor.js, terminal.js, search.js, agent.js, git.js,
  panels.js, shortcuts.js, command-palette.js.
- Code Studio visible UI strings use `codeStudio.*` keys in all
  `ui/lang/desktop/*.json` files.
- Code Studio New File uses the file API's `create_only` mode; a conflict leaves
  the existing disk file and open editor unchanged and uses the shared localized
  `desktop.fm.paste_exists` message. Save retains overwrite semantics.
- Code Studio Git commands run via Docker exec in the container workspace (`/workspace`).
  Git API endpoints are in `internal/server/code_studio_handlers.go`.
- Code Studio recognizes C source/header files and reuses the bundled CodeMirror
  C/C++ parser. Run compiles `.c` as C17 with GCC into a private temporary
  directory, executes only a successful build and cleans up afterward. Headers
  remain editable, not standalone programs. With no restored tabs or launch
  path, open available `hello.go`, `hello.py` and `hello.c` samples as tabs.
  Verify `TestDesktopCodeStudioC` and the CodeContainer sample/runtime tests.
- System World loads `sysworld-data.js`, `sysworld-hud.js`, `sysworld-controls.js`,
  then `sysworld.js`.
  The first two expose `window.SysWorld.data/createHud`; the entry owns per-window
  instances and exports `SysWorldApp.render/dispose/inspect`. It imports
  versioned `/js/vendor/system-world/city.esm.js` only when opened.
- `sysworld-scene.js` is build input, not a classic lazy script. Rebuild with
  `node scripts/build-system-world.js` after changes; `--check` verifies exact
  output. Three.js and matching addons stay pinned to MIT 0.186.1.
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
- System World keeps one browser data subscription/polling set across windows.
  Reuse `system_metrics` SSE, with REST bootstrap/fallback; never poll the OS per
  window. Closing the last subscriber aborts requests and removes timers/SSE.
  Preserve labelled stale values, measured zeroes and configured/unknown states;
  enabled integration flags do not prove connectivity. Older REST responses or
  failures must not replace newer SSE samples. No raw tool arguments, prompts
  or issue details enter the bounded 60-event feed.
- The hologram artifact feed in `sysworld.js` polls GET
  `/api/desktop/system-world/memory-artifacts` (same-origin, `no-store`, 10 s
  abort) every 24 s only while the city is visible and not in map mode; failures
  back off 90 s and HTTP 429 keeps the normal interval. Until the first live
  response, and after failures, the archive's own localized counters feed the
  hologram (`source: 'local'`). Close clears the timer and aborts the request.
  `inspect()` exposes `artifacts.{source,count,failures,polling}`, never text.
- `sysworld-life.js` owns exactly five decorative white ThreeDee robots, rounded
  street routes, shared assets/soft hover lights and state-driven district rings.
  Normalize the GLB face axis (+X) to route-forward (+Z) before cloning;
  keep faces aligned with travel on straights and rounded turns.
  Only fresh actual error/running states or bounded recent events animate district
  signals; stale/unknown stays neutral. The operations signal uses private material
  clones so its red pulse cannot recolor other buildings. Rebind after LOD swaps.
  The effective System World motion preference freezes residents and pulses.
  Paused patrols render their accepted collision pose without submitting route
  adjustments; suspension may rebase the navigator but must not move the actor.
- `sysworld-navigation.js` steers those robots. Routes are right-turn block
  circuits sampled by arc length with 7.5 m corner handles; robots drive the
  right-hand lane (`2.2`) and may use `0/-2.2/±3.9`. Obstacles are circles
  derived from kit bounds (`obstaclesFrom(placements)`: tram, server rack,
  lamp pole, planter) expanded by the 1.3 m robot radius. Every frame checks
  lane probes up to 18 m ahead and trajectory conflicts up to 12 m; robots
  sidestep only away from where oncoming traffic is heading, yield to the
  robot closer to a shared crossing, brake behind slower leaders, and after
  2.5 s blocked they reverse direction (`state: 'turn'`, 4 s cooldown). Lane
  changes are velocity-limited to the current speed so heading always follows
  travel; stopped robots square to the street. These are movement intentions:
  `sysworld-traffic.js` validates every final pose before rendering. Spawns scan
  for a clear lane spot using the normalized exported model envelope.
- `sysworld-hologram.js` projects the memory archive hologram (roof 21 m):
  additive cone, base glow, rings, wireframe core, GPU motes, one billboard main
  panel and two orbiting fragment panels. Text is drawn with canvas `fillText`
  only and never interpreted as markup; `sanitizeArtifacts` trims to 24 entries
  and 140 characters. Panels cycle randomly every 4.5–7.5 s (12–15 s under
  reduced motion) with glitch/fade transitions. `stats()` reports counts,
  source and switch counters, never text.
- `sysworld-atmosphere.js` owns the sky dome (dusk gradient, weather-driven fbm
  clouds, Milky Way, meteors, city light dome, aurora), 1400 twinkling stars,
  moon, animated sea with shore foam and city-light reflections, sea mist, 500
  dust motes, the rotating spire beacon (faster while the agent is busy),
  instanced lamp cones, three night searchlights (always leaning away from the
  island) and up to 96 blinking aviation lights on skyline roofs. The sky disc
  follows its own path (behind the skyline at dusk/night); the key light keeps
  its front-lit path so glass facades never mirror a backlight into the camera.
  `sysworld-cinema.js` is the final `ShaderPass`: grade/split-tone, S-curve,
  edge chromatic aberration, anamorphic streaks, sun/moon shafts limited to a
  tight radius (no depth, so windows must not become shaft sources), vignette,
  grain and tour-only letterboxing. One shared `time` uniform advances only while
  animated, so hidden windows and reduced motion freeze the layer. Low tier hides
  mist/dust/lamp cones/searchlights and uses fewer cloud octaves; the post pass
  runs on high/ultra only. `sysworld-weather.js` adds soft lightning in rain
  (animated, outdoors only, 9–25 s apart, never a strobe) and reports thunder
  delay through `onThunder`. Clamp every
  `pow()` base: multisampled edge extrapolation yields NaN otherwise, and bloom
  smears one NaN over the whole frame.
- `sysworld-surfaces.js` gives the UV-less kit surface detail: six generated tileable maps
  (`ui/3d/system-world/textures/v1/`, R albedo / G roughness / B height) projected triplanar
  through `onBeforeCompile` on materials matched by name (`city.road|stone|graphite|titanium|
  bronze|ceramic|leaf` and the island ground). Static geometry uses world space; moving or
  articulated assets (tram, robots, doors, lifts, animated installations, drone clones) use
  object space so detail never slides. Bump uses screen-space derivatives of a metre-scaled
  height and fades out between 35 and 140 m. Low = no detail and no download, medium =
  albedo/roughness, high/ultra = bump and anisotropy. Maps load independently behind neutral
  placeholders; failures retry on the next quality change. Rain wetness eases (instant without
  motion); puddles form per material threshold on up-facing ground, reflect the fog colour and
  stay dry under the three pavilions. `city.ivory|warm` panes (four-vertex components) switch
  individually by time of day; larger lit parts and horizontal fixtures only dim. Pane light
  is decorative, never telemetry. Static walls darken towards their foot and the quay wall
  carries a wet waterline (world space only). Verify `node scripts/test-system-world-surfaces.mjs`.
- Look by time of day: bloom and exposure rise towards night; the day keeps thin fog, low
  sea mist and restrained ambient fill for readable shadows. The skyline placements are
  seeded (jitter, gaps, turned towers, taller towards the middle) plus a lower far row; they
  stay scenery outside navigation. `sysworld-atmosphere.js` also draws a faint distant
  coastline (fixed haze tint, not scene fog) behind and beside the city, open towards the
  camera side, with twinkling settlement lights at night.
- `sysworld-drones.js` flies up to six service-drone patrols (two on low) on closed Catmull-Rom
  loops with spinning rotors, navigation lights and banking; the template is the
  cached kit GLB and instances share geometry.
- `sysworld-scene.js` renders through `SceneCapturePass` (multisampled HDR scene
  target resolved and NaN-guarded into plain composer buffers) → bloom → output
  → atmosphere post. Never let bloom blend into a multisampled buffer.
- City radio waves consume typed `agent_action` starts and executed results, plus
  co-agent progress. Exact tool names map to districts; unknown tools, previews,
  blocked proposals and metric snapshots must not invent building-to-building traffic.
  Keep only route/state/tool-name metadata, never arguments, result/error text or
  session content. Deduplicate action states in a bounded map and clear it on close.
  Unknown, deferred and needs-setup results close the tracked action without
  emitting successful execution traffic.
  `sysworld-life.js` shares twelve wave packets and six route curves on its existing
  RAF. Coalesce bursts, expire by wall time, discard hidden/map/reduced-motion starts,
  and never replay retained events when a window opens or resumes. Robot geometry
  and the shared legacy Three.js remain unchanged.
- `sysworld-audio.js` owns quiet native Web Audio synthesis. Sound is opt-in,
  persisted, gesture-unlocked, volume-bounded and fades/suspends when the app is
  hidden, unfocused or in map mode. Close releases oscillators, nodes and AudioContexts.
  The soundscape is a sub drone, a detuned saw pad gliding through four chords
  (16 s) into a generated convolution reverb, air with wind gusts and at most 12
  transient events (swooshes, chimes, day birds, night harbour horn, thunder).
  Event/chord timers run only while audible and `quiet()` releases every voice on
  inactivity. `setMood` (busy/day/evening/weather from the scene) opens the pad
  filter. Do not add a `DynamicsCompressorNode`: its automatic makeup gain lifts
  the mix above the browser test's 0.05 RMS voice ceiling.
  `sysworld-voice.js` adds transient tower speech to that same opt-in mixer: one
  cancellable POST `/api/desktop/system-world/voice`, 20–40 seconds of quiet after
  each short phrase, with 60-second failure backoff. Camera pose updates through
  the existing scene RAF; no extra render loop. Distance to the tower attenuates
  the complete dry/85-ms echo/0.85-second stereo-room mix while leaving speech
  audible across the road grid. Normalize each decoded
  phrase by voiced-window RMS with a bounded input gain so quiet TTS backends remain
  intelligible; retain low voice fundamentals and a calibrated room impulse instead
  of browser-dependent convolver normalization. Limit peaks before distance gain
  and stereo placement. Master volume remains bounded to 35%; no media cache,
  raw text diagnostics, automatic retries without backoff or browser-TTS fallback.
  Hide, focus loss, map, mute/zero-volume and close abort requests, stop playback,
  release phrase nodes/tails and prevent late fetch/decode responses from replaying.
  Verify with `node scripts/test-system-world-voice.mjs` and the real-shell city test.
- Rendering owns one RAF per visible window. Minimize, Spaces, document hiding
  and map mode stop it. Close aborts loaders and frees GPU resources, listeners
  and observers. Context loss falls back to the usable map. Keep all models,
  local materials and licenses build-versioned; no remote textures or services.
- System World 2 adds `sysworld-experience/exploration/weather/effects.js` to the
  same renderer build and RAF. The v2 Blender manifest has 33 designs, three LODs,
  articulated clips and navigation metadata. Keep the complete app payload below
  48 MiB and first display below 12 MiB; exterior shells load with the city and
  interior furnishings load by proximity. Five original
  robots plus 19 new residents are the high-tier cap (eight total on low).
- `sysworld-traffic.js` owns one shared collision world for both resident groups,
  trams, drones, moving freight and the visitor. Fixed 1/60-second steps consume
  at most six steps per frame; discarded hidden time is never replayed. Resolve
  all intentions from the same snapshot, with swept circles/boxes and vertical
  intervals, iterating after rejections. Priority never pushes a stopped actor.
  Followers yield to their leader; trams retain their rails. Only a passenger's
  own vehicle is excluded. `sysworld-colliders.js` preserves ground apertures
  separately from upper building envelopes. Moving doors wait for clear panels.
  Trams accelerate and brake before obstacles/stops; yielding residents wait
  outside the rail corridor until the complete vehicle has passed. Guides
  retain their destination while blocked and resume when a route becomes free.
  Camera flights validate each segment, use nearby clear exits before climbing
  above roofs, and wait or stop when blocked. Manual orbit input cancels a flight.
- `sysworld-places/society/signals/machinery.js` own a bounded waypoint graph,
  reserved work slots, courier pickup/delivery, maintenance, archive exchanges,
  visitor greetings/guides and six local installation demonstrations. Guides
  wait for the visitor; the camera remains user-controlled. Residents clear
  approaching rails; conflicting drones use distinct vertical escape decisions.
  Cancel, hide, replay and close release reservations and packets. Local exchanges
  are labelled City life, never successful system actions. Eight reusable local
  wave packets show direction, receipt and reply; genuine building waves retain
  their existing twelve-packet metadata contract. No new endpoints or LLM calls.
- New installation geometry starts at LOD2 and upgrades by proximity (55/32 m),
  within the quality cap. Colliders use LOD0 envelopes/exported navigation data
  at every quality. Machinery particles are bounded and omitted on low; reduced
  motion freezes demonstrations and presents a localized quiet-state message.
  Effect envelopes fade within their existing lifetime, clear expired live colors
  and freeze under reduced motion; verify with the expansion script.
  Diagnostics expose bounded counts, actor poses, blockers and effect provenance,
  never content or invented telemetry. Verify shared movement with
  `node scripts/test-system-world-traffic.mjs` and real exported controllers with
  `node scripts/test-system-world-living.mjs` (`SYSTEM_WORLD_SIM_SECONDS=3600`
  for the extended run).
- `sysworld-layout.js` owns roads, clear building plots, station platforms and
  exposed surface heights. Tram rails and motion share `streetCurve`; test the
  swept vehicle body through bends. Do not use overshooting splines across plots.
  Quay, ground and interior floors must not be coplanar. Upper navigation is
  restricted to the rendered guarded gallery and docked lift platform. Verify
  `node scripts/test-system-world-layout.mjs` against actual exported GLBs and
  the browser expansion round including leaving, traversing and returning from
  the upper floor; lift arrival alone does not prove a usable landing.
- Sky deck and viewers (`sysworld-experience.js`, `skyDeck` in `sysworld-layout.js`):
  the open east lift of the agent tower travels 71.32 m (mirrors `SKY_TRAVEL` in
  `build_expansion.py`) to the 72.2 m crown terrace. Up there only the ring between
  `inner` and `outer` is walkable, plus the bridge while the cab is docked. The
  agent's collision uses stepped `upperSolids` (`sysworld-exploration.js`) instead of
  its bounding box, so shaft and terrace stay free. Rides take 15 s, calls 7 s; idle
  trips happen only while nobody is within 30 m; reduced motion makes all of them
  instant. Viewers on galleries and deck move the camera to their pivot (deck: a
  virtual objective beyond the balustrade), clamp yaw/pitch to their field, zoom by
  wheel or +/-, hide their model while in use and report the reticle's district via
  `onScope`; the HUD draws lens, reticle and a live tag. Escape leaves a viewer
  before it leaves street mode. Verify with the layout script and the expansion round.
- `sysworld-controls.js` owns destinations, discoveries, environment, three audio
  buses, the typed terminal and 24-hour replay. UI hints must not mutate the DOM
  every frame. Replay immediately disables system actions and private memory feeds;
  missing history shows a gap, never substituted live values. Confirm stop/cancel/
  restart with the concrete target and lock before opening the confirmation dialog.
  Accepted actions remain pending until a newer matching state confirms the result.
  Opening an encounter, terminal or discovery releases this city's pointer lock.
  Close/Escape returns focus to the canvas; encounter updates preserve keyboard
  focus and the chosen guide destination. Reserve space for walking controls and
  hide keyboard-only interaction hints on touch devices. Interaction prompts yield
  to the open exploration panel. Verify with the living browser scenario.
- Server `internal/systemworld` stores bounded telemetry in the Desktop database.
  `/api/desktop/system-world/{snapshot,history,events,entity,actions}` requires admin
  scope; reuse existing services/write gates. The shared ten-second metrics worker
  owns collection, not windows. No prompts, reasoning, tool results, credentials or
  memory excerpts belong in history. Detailed contracts and acceptance commands:
  `documentation/system-world-2.md`.
- Quality `auto/low/medium/high/ultra` persists under
  `aurago.desktop.sysworld.quality`. LODs are separately fetched and cached;
  instanced scenery uses shared geometry/materials, distant towers always LOD2.
  Quality changes rebuild instances after assets finish without moving focus.
  Auto uses frame-time hysteresis; shadow maps update only after geometry/tier
  changes. No per-frame geometry or material creation.
- District placement and entity IDs stay fixed through data refresh. User
  navigation cancels the explicit tour; inactivity never takes the camera.
  Motion `auto/on/off` persists under `aurago.desktop.sysworld.motion`. Default
  `auto` honors reduced motion and Desktop animation settings; explicit `on`
  enables this city's animation without changing global preferences, and `off`
  freezes it. The exploration selector controls the existing simulation; its
  summary visibly labels a paused city. Reapply the effective preference after
  asynchronous city loading. Verify `AURAGO_SYSTEM_WORLD_MOTION=1` with the city
  browser test (all moving groups, pause, persistence and system changes).
  Effective reduced motion suppresses camera flights/tours and the decorative
  pulse. Street mode owns WASD only while its canvas is
  focused; pointer lock is explicit and Escape/blur/close release control.
- HUD presentation: district labels are ranked (selected, hovered, error, running,
  then distance with hysteresis) and never overlap each other or HUD panels; a
  label cut by a panel's lower edge slides down its building by at most 90 px.
  Panel and label rectangles come from a ResizeObserver, never from per-frame
  layout reads; label state and transforms are written only on change. Hover is
  a raycast throttled to 80 ms (ring, cursor, highlighted label and rail entry);
  the selection ring takes the district's state colour. Loading shows real byte
  progress (tier LODs from the manifest plus surface textures); the first display
  fades the canvas in with a one-time opening glide. Camera teleports dip through
  a short fade, but `city.setMode` is never delayed. The tour shows a lower third
  per station. Photo mode (camera button or `H`; `H`/Escape leave it before the
  canvas sees the key) hides the whole interface and saves the composed frame
  right after `composer.render()` in the same task, without
  `preserveDrawingBuffer`. CPU/RAM sparklines keep 60 live samples and never
  ingest replay data. The street compass (north is -z) redraws only after a 1.5°
  turn or a 1.5 m step. Reduced motion skips the glide, fades and caption motion.
  The city browser matrix checks label overlap, hover, compass, tour caption and
  the photo PNG.
- The map is an SVG plan drawn from `NS.map`/`NS.districts`, mirrored constants of
  `sysworld-layout.js` and the scene's districts (`node
  scripts/test-system-world-layout.mjs` keeps them in step): water, quay, streets,
  blocks in state colours with counts, tram loop and stops, pavilions, drone pad,
  legend and the last camera position. It needs no WebGL and stays the
  context-loss fallback; the exploration control hides in map mode. At 760 px and
  below the rail is an icon column whose toggle opens a drawer with search
  (choosing an entry or Escape closes it and clears the query), and the inspector
  is a bottom sheet beside the rail that yields to street, tour, the open
  exploration panel and the drawer. The browser matrix fails when the exploration
  control, rail, inspector or fixed bars overlap at any tested size.
- Verify `node scripts/test-system-world.mjs` (includes the navigation
  blockade/sidestep units and a 60-minute five-robot simulation), `node
  scripts/build-system-world.js --check`, focused Sysworld Go tests, `go test
  ./internal/server -run SystemWorld`, and the real-shell browser matrix:
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SYSTEM_WORLD_MATRIX=1 go test ./ui
  -run '^TestDesktopAuroraBrowser$'`. The browser run mocks the artifact feed,
  asserts live hologram/atmosphere/drone stats without text leaks, captures
  Three.js console errors, and fails when the canvas renders flat (blank
  post-processing). Screenshot/performance reports stay ignored.

- Terminal uses xterm 6 with the WebGL addon. CRT capture includes every canvas
  below `.xterm-screen`, including the unnamed WebGL text canvas. Preserve the
  drawing buffer and usable DOM/CSS fallback after context loss.
- Sheets 1.0 runs write interceptors sequentially; localized input parsing runs
  after the format parsers and before the terminal interceptor (priority -0.5),
  so the native formatter cannot reinterpret numeric values.
  Read function metadata from `engine-formula.functionList`. Browser
  input tests wait for the loading overlay and emit both clicks of a double click.

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
- Combat-juice pack (2026-09) adds `superReady` / `heartbeat` / `multiKill`
  SFX (mute-guarded) plus a shimmer layer on `respawn`; `FX_SUPER_READY_DUR`,
  `FX_LASTLIFE_INTERVAL`, `FX_MULTIKILL_WINDOW/COUNT/HITSTOP` constants live in
  `galaxa-constants.js`.   `registerKill(x, y)` tracks the
  `multiKillCount`/`multiKillWindow` cluster and fires `fxMultiKill` once per
  cluster; `superReadyFired` (reset in `startSuper` and below 100 % meter)
  gates the one-shot super-ready cue in `updateFX`. Menu states (SHOP, evo
  choice) tick `updateFX` so leftover gameplay FX decay instead of freezing
  as artifacts over the menu panels.
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
- Keep Sheets lifecycle, data conversion, panels and charts in their four
  existing modules; reuse the shared OfficeSession queue and native engine.
- Keep Code Studio split across `core.js`, `sidebar.js`, `editor.js`,
  `terminal.js`, `search.js`, `agent.js`, `git.js`, `panels.js`, `shortcuts.js`,
  and `command-palette.js`; do not fold domain modules into core.js.
- Keep System World's data, HUD, scene, life, navigation, hologram, atmosphere,
  drones, audio and lifecycle in their owned files. Do not merge renderer
  dependencies or data polling into the Desktop shell.
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
- The OpenSCAD STL preview keeps one scene record per `renderSTL` with its own `AbortController`; a late
  download for a replaced scene is ignored and never touches the current record. PNG, SVG, PDF, empty and
  download-only previews call `cleanupPreview` before replacing the panel. Verify
  `TestDesktopOpenSCADSTLPreviewIgnoresStaleLoads`.
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
- Homepage Studio chrome uses the shared `--vd-theme-*` materials and Desktop
  accent in `css/desktop-app-homepage-studio.css`. Keep panels, empty/loading
  previews, controls and history badges legible in Standard and Fruity light/dark;
  avoid a separate gradient palette. The website iframe retains its own colors.
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
  16 `ui/lang/desktop/*.json` files. The local webhost name fallback
  uses `homepage_studio.default_name`. Chat-stream failures reuse
  `desktop.chat_request_failed`.
- Keep Writer's document lifecycle, save queue and panels in their existing three
  modules; load session and panels before writer.js. Rebuild and check local vendor
  assets with `node scripts/build-writer-vendor.js [--check]`.
- Build/check Sheets with `node scripts/build-sheets-vendor.js [--check]`.
  Both the main engine and worker register AVG as the AVERAGE compatibility
  alias. English formula syntax is independent from localized input/display.
  The worker's cycle guard uses the public dependency graph and runtime result
  API to mark cycles and their dependents as #REF!, instead of Univer's default
  one-iteration numbers. Keep graph emission enabled and test self/cross-cell
  cycles, unaffected formulas and recovery after breaking the cycle.
- Rebuild chess vendor assets with `npm run build:chess-vendor` after changing
  vendored chess package versions or copied Stockfish assets.

## Verification

- `node scripts/test-sheets-data.mjs`, `node scripts/test-writer-session.mjs`
- `node scripts/build-sheets-vendor.js --check`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopSheets(Engine|App)Browser' -count=1`
- `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SHEETS_MATRIX=1 go test ./ui -run '^TestDesktopAuroraBrowser$' -count=1` (18 theme/density/size screenshots)

- `go test ./ui/ -run TestDesktopFeeling`
- `go test ./ui/ -run TestDesktopWidgetConfigPersistence`
- `go test ./ui/ -run TestDesktopWeatherWidgetI18n`
- `go test ./ui/ -run TestDesktopVirtualComputersDurationI18n`
- `go test ./ui/ -run TestDesktopCalculatorBackI18n`
- `go test ./ui/ -run 'TestDesktopMissionControl|TestMissionControlDispose'`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopMissionControlBrowser -count=1`
- `node scripts/test-mission-control-schedule.mjs`
- `node scripts/test-mission-control-flows.mjs` (or `npm run test:mission-control` for both)
- `node scripts/test-dashboard-cronjobs.mjs` (`npm run test:dashboard-cron`)
- `go test ./ui/ -run TestDesktopFileManagerTemplateI18n`
- `go test ./ui/ -run TestDesktopWidgetDisplayTitle`
- `go test ./ui/ -run 'LineBudget|GalaxaMode|DesktopAppAssets|AdaptiveMusic'`
- `go test ./ui/ -run TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`
- `go test ./ui/ -run TestGalaxaDeluxeCachesCanvasResources`
- `go test ./ui/ -run TestVirtualDesktopJSUsesSemanticChunkNames`
- `go test ./ui/ -run "TestDesktopChess|TestDesktopAppsExposeDisposeLifecycle|TestDesktopAppAssetsRegistry"`
- `go test ./ui/ -run "TestDesktopCheater"`
- `go test ./ui/ -run "TestDesktopSheets"`
- `go test ./ui/ -run TestDesktopSheetsMatchCountI18n`
- `go test ./ui/ -run TestDesktopPeopleDaysI18n`
- `go test ./ui/ -run TestDesktopElapsedSecondsI18n`
- `go test ./ui/ -run TestDesktopShellPromptDefaultsI18n`
- `go test ./ui/ -run TestDesktopChatFallbackI18n`
- `go test ./ui/ -run TestDesktopFileManagerNewFileDefaultI18n`
- `go test ./ui/ -run TestDesktopWriterSearchI18n`
- `node scripts/test-writer-session.mjs`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopWriter(App|Engine)Browser'`
- `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_WRITER_MATRIX=1 go test ./ui -run '^TestDesktopAuroraBrowser$'`
- `go test ./ui/ -run TestDesktopRelTimeI18n`
- `go test ./ui/ -run TestDesktopPetChromeI18n`
- `go test ./ui/ -run TestDesktopRadioCompactI18n`
- `go test ./ui/ -run TestDesktopRadioMediaSessionI18n`
- `go test ./ui/ -run TestDesktopPeopleKgI18n`
- `go test ./ui/ -run TestDesktopSysworldHudUptimeI18n`
- `go test ./ui/ -run TestDesktopSysworldHudMoneyI18n`
- `go test ./ui/ -run TestDesktopSystemInfoGaugeI18n`
- `go test ./ui/ -run TestDesktopEmbedFrameI18n`
- `go test ./ui/ -run TestDesktopChessResultI18n`
- `go test ./ui/ -run TestDesktopChessErrorI18n`
- `go test ./ui/ -run TestDesktopChessLoadI18n`
- `go test ./ui/ -run TestDesktopChessTemplateI18n`
- `go test ./ui/ -run TestDesktopSysworldSuccessRateI18n`
- `go test ./ui/ -run TestDesktopSysworldPanelIdI18n`
- `go test ./ui/ -run TestDesktopHomepageStudioFallbackI18n`
- `go test ./ui/ -run TestDesktopZipperFilterI18n`
- `go test ./ui/ -run TestDesktopPixelOpenFilterI18n`
- `go test ./ui/ -run TestDesktopPixelSaveFilterI18n`
- `go test ./ui/ -run TestDesktopTerminal`
- `go test ./ui/ -run TestDesktopCodeStudioShellI18n`
- `go test ./ui/ -run TestDesktopQcAuragoHostI18n`
- `go test ./ui/ -run TestDesktopHostErrorI18n`
- `go test ./ui/ -run TestDesktopLoadFailedI18n`
- `go test ./ui/ -run TestDesktopAppAssetsRegistry`
- `go test ./ui/ -run TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`
- `go test ./ui/ -run TestDesktopLooper`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopLooperBrowser`
- `go build ./cmd/aurago`

## Child DOX Index

- `looper.js` / `looper-monitor.js` own Looper v2 (`window.LooperApp` +
  `window.LooperMonitor`). A loop is goal + work + evaluate + optional finish.
  Visible controls are rounds (1–50), target score (50–100), stall rounds
  (0–10, 0 off), provider and model. No context mode, confidence or truncation
  UI. Layout is three columns from 820 px (list | editor | run/history) with
  one footer action bar; below that, compact tabs Setup / Run / History.
  The loop completes when the score reaches the target; evaluator `done`
  is logged only. Review is tool-free JSON against the work report; a failed
  or invalid review logs score 0 and the loop continues. Monitor shows
  `error`, a pending current step, the sparkline, timeline and last 20 saved
  runs. Status SSE
  is `/api/desktop/looper/status`; start/resume stay admin POST. Shell must
  pass `promptDialog` and `confirmDialog`. Readonly disables start/save/delete
  and form edits. Keep `desktop.looper_*` in all 16 desktop locales, including
  cost/token/duration keys shared with System World. No emoji in the app
  sources. Verify `TestDesktopLooper*`, `TestDesktopAuditedI18n*`, bundle
  `--check`, and opt-in `TestDesktopLooperBrowser`
  (`AURAGO_RUN_BROWSER_SMOKE=1`). Backend: `internal/desktop/looper*.go`,
  `internal/server/looper_service.go`, `desktop_looper_handlers.go`.
  No child DOX file needed.

- `ha-switchboard.js` exposes `window.HASwitchboardApp.render/dispose`. The
  scoped `css/ha-switchboard.css` retains the real shell controls and uses the
  original local walnut/three-pose silver atlas (SVG clips remove its background).
  The SVG dial shares a 240-degree scale/needle pivot and keeps labels below the
  sweep. Size-container queries shrink cabinet chrome and levers for short windows;
  a single row must fit at 1300 x 540/620/700 in every theme. Keep entity IDs in
  plaque/switch tooltips so similarly named HA entities can be distinguished.
  Use the existing admin-only `/api/desktop/home-assistant/{entities,states,switch}`
  routes and shared validated `ha_switchboard.board` setting, never browser HA
  credentials or a second connection setup. The native dialog searches locally,
  preserves failed-save drafts and detects changed selections before saving.
  Poll every five seconds only while visible, cancel obsolete reads, pause in
  inactive Spaces, and release requests/timers/listeners on dispose. Explicit
  on/off writes remain pending until a subsequent state read confirms the target;
  never retry a write automatically. Keep missing entities visible, escape HA
  names, respect desktop/HA read-only and service capabilities, preserve keyboard
  switches and reduced motion. Verify `TestHASwitchboardBrowser` with
  `AURAGO_RUN_BROWSER_SMOKE=1`, `TestHASwitchboardLocales`, focused backend
  `TestHASwitchboard|TestHAContext`, bundle `--check` and pinned asset validation.
  Implementation and acceptance details: `documentation/ha-switchboard-plan.md`.

- `meshcore.js`: native Messenger (`window.MeshCoreApp.render/dispose/openConversation`),
  using `/api/meshcore/messenger/` and the existing Companion manager. Owns
  direct/channel conversations, protected-text reveal, contact/channel dialogs,
  native BarcodeDetector import and the existing QRCode renderer. No direct
  hardware connection or agent-tool sending. Labeled Messages/Device/Settings
  navigation stays visible on all pages. Theme styles live in
  `css/desktop-app-meshcore.css`; the 300px list switches to single-pane navigation
  below 700px. Every selector stays scoped under `.vd-meshcore` because Mission
  Control reuses `mc-*` class names. Scoped `--mc-*` tokens provide teal surfaces
  on a dot-grid canvas, per-theme variants, tinted hash-hue avatars with contact
  type badges, relative non-sticky day dividers, SNR signal bars and skeleton
  loading; Geist Mono renders keys, times and radio values. Healthy send states
  are icon-only with a screen-reader label; failed/uncertain states keep a
  visible label. Inline
  SVG icons use `icon()`/`iconEl()` without external assets. History search opens
  on demand; closing it clears the query. Distinguish empty conversations from
  filtered results and provide filter reset. Hide the composer until a conversation
  is selected; grow its input with the draft and show packet preview for multipart
  messages. Messages group by direction/origin within a 300 s gap via
  `mc-group-start/mid/end`; day changes break groups. Message actions
  (reveal/copy/retry) reveal via `opacity` on hover/focus-within, never via
  `pointer-events`, `visibility` or `display`; they remain visible on touch/narrow
  layouts. They live in `.mc-message-actions`, floating beside the bubble so
  hidden actions never shift the meta line, and render inline on touch/narrow. Preserve ArrowUp/ArrowDown roving focus through list refreshes. Esc
  closes details/search before leaving the chat, unless a dialog is open. Drafts
  and pending send IDs are local per device/conversation; invitation keys never
  enter browser storage. Persistent request IDs reconcile HTTP retries; explicit
  resend warns about duplicates. Abort all requests and remove document
  listeners/timers on dispose. Session/notification context contains only a
  validated conversation ID. All 16 desktop locales must include
  `desktop.meshcore_*`. Verify `TestDesktopMeshCoreBrowser` in standard and fruity
  light/dark themes at wide and narrow widths.
  New channels default to the hashtag type so a name such as `#bot` uses its derived key; the invitation field also accepts `#bot` as shorthand when the name is empty. Selecting Public fixes the channel name to `Public` and restores the draft name when switching back.

- `file-manager/` (under `ui/js/desktop/file-manager/`, bundled to
  `file-manager.bundle.js`) - File Manager restore and empty-trash menus follow
  the Trash restore contract above. New-file templates, new-file default
  names, and ZIP/rename success toasts follow the i18n keys in Local
  Contracts. No child DOX file needed.
- `galaxa-modes.js` - Game mode contracts (`gauntlet`, `hyperdrive`, `mirror`)
  and hooks (`modesOnRunStart`, `modesOnStageStart`, `modesShouldOpenShop`,
  `modesGetBaseMusicTheme`). Settings mode cycle reads/writes `settings.mode`.
  Achievements: `gauntlet_clear`, `hyper_survivor`, `mirror_master`.
- `galaxa-fx.js` retains capped particle/ring producers and `fxDrawBack`.
  Atlas explosions, impacts, shields and parries render through the common
  world renderer. Decorative effects precede `renderDangers`; HUD uses a
  fixed transform after camera shake. Do not restore removed overlay/ghost
  drawing passes or ordinary-hit hitstop. Respect particles, shake and
  reduced-motion settings. No child DOX file needed.
- `writer.js`, `writer-session.js`, `writer-panels.js` - Native DOCX Autor,
  revision-aware persistence, document panels and isolated AI suggestions.
  `desktop-app-writer.css` owns chrome only; the engine owns document geometry.
  Public table merge/split operations live in `scripts/writer-table-ops.js`.
  Exposes `window.WriterApp`. No child DOX file needed.
- `pet-picker.js` - Pet catalog, scale/enabled/always-on-top settings, and
  ZIP import. Scale text uses `desktop.pet_scale_value`. Load,
  activate, settings, and import notifies use
  `desktop.request_failed`. Invalid ZIP names stay on
  `desktop.pet_import_invalid`. Exposes `window.PetPickerApp`.
  The companion shell runtime `core/pet-runtime.js` uses
  `desktop.pet_aria_label` for the fallback sprite label. No child
  DOX file needed.
- `radio.js` - Station browser and player. Click counts use
  `desktop.radio_compact_thousands` and `desktop.radio_compact_millions`.
  MediaSession title fallback uses `desktop.app_radio`; album uses
  `desktop.radio_album`. Catalog HTTP throws the sentinel
  `Radio Browser HTTP` without a status; `loadActive` and
  `searchStations` show `desktop.radio_catalog_error`. Playback
  `play()` and the player toggle show `desktop.radio_error` only
  and must not dump `err.message`. Radio `t` stays key-only.
  Exposes `window.RadioApp`. No child DOX file needed.
- `people.js` - Address-book app. KG toggle, active label, and card badge
  use `desktop.people_kg`. Content empty-state load failures use
  `desktop.load_failed` via `t(inst.context, key)`. Save and delete
  notifies use `desktop.request_failed`. Exposes
  `window.PeopleApp`. No child DOX file needed.
- `chess.js` / `chess-fx.js` - Chess app and board FX. Result-modal
  fallbacks use `desktop.chess_new_game` and `desktop.ok`; pass `t`
  into `createChessFx`. Opponent-move errors use
  `desktop.chess_*` codes via `formatOpponentError`. Vendor-load
  status and the missing-template fallback use
  `desktop.chess_load_failed`. Exposes
  `window.ChessApp` and `window.createChessFx`. No child DOX file
  needed.
- `calendar.js` / `calendar-views.js` / `calendar-editor.js` - Calendar
  session shell, pure view renderers and editor/peek continuations bundled
  inside the shared Desktop IIFE (see Local Contracts for order and behavior).
  Empty-state load failures use `desktop.load_failed`. No child DOX file
  needed.
- `looper.js` / `looper-monitor.js` - Desktop Looper: goal/work/evaluate/optional
  finish. Evaluate is tool-free. A broken review records score 0 and continues.
  Work recovers from `unexpected tool-call text` when the failed step already
  produced tool output; MeshCore still fail-closes on that error. Finish format
  errors keep the completed loop status. Verify
  `go test ./internal/agent ./internal/server -run 'Looper|MinimalLoop|ParseEvaluation'`.
  No child DOX file needed.
- `agent-chat.js` - Desktop Agent Chat. Missing-host throws reuse
  `desktop.load_failed` via `desktopText(key)` with no second
  argument. Loaded lazily. Exposes `window.AgentChatApp`. No child
  DOX file needed. Finish and release the active streaming bubble at each
  `tool_call`; its narration replaces that round's streamed draft instead of
  duplicating it. `final_response` replaces the current final draft. Pending
  scrolls target the newest log item, never an earlier round's bubble.
  Verify with `node scripts/test-ui-regressions.mjs`.
- `live-speech.js` - Desktop Live Speech. Missing-host throws reuse
  `desktop.load_failed` via `text(key)` with no fallback. Loaded
  lazily. Exposes `window.LiveSpeechApp`. No child DOX file needed.
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
  The programmer section starts hidden; mode tabs must fit the content width.
  Verify all three modes in the default and narrow windows with
  `TestDesktopCalculatorLayoutBrowser`.
- `settings.js` implements the Settings app, a virtual desktop configuration
  panel with sidebar navigation, global search, hamburger menu on mobile,
  and full desktop shell re-render on changes (icons, widgets, start menu,
  start button). Loaded lazily by `module-loader.js`. Exposes
  `window.SettingsApp`. Info and editable rows share inset spacing; bounded
  control columns must not let long provider/model names squeeze their labels.
  Keep full workspace paths wrappable and Info values at normal text weight.
- `camera.js` implements the Camera app (device webcam): photo mode with
  optional self-timer countdown, mirror toggle and rule-of-thirds grid, and
  MediaRecorder-based video mode with a centered REC badge. Live video, photo
  preview and clip preview stack absolutely inside the viewport (never in-flow
  flex siblings — that squeezed each into one half); the `[hidden]` guard in
  `camera.css` must keep beating author display rules. Photo previews offer
  retake/save (`Pictures`)/download/clipboard-copy/send-to-agent; clip previews
  offer retake/save (`Videos`)/download. A session-only recents strip (max 12,
  data-URL thumbnails) re-opens captures. Network access is limited to
  `/api/desktop/upload` and `/api/desktop/chat/stream`; nothing is persisted
  across windows. `dispose()` must stop tracks, cancel countdown/record clocks,
  stop the recorder (discarding the clip), revoke the clip object URL and
  detach `devicechange`. Loaded lazily by `module-loader.js` and exposes
  `window.CameraApp { render, dispose }`. Visible strings use
  `desktop.camera_*` keys in all `ui/lang/desktop/*.json` locales. No child
  DOX file needed.
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
- `noisemaker.js` — Noisemaker shell (`window.NoisemakerApp = { render, dispose }`):
  workbench layout (create pane | splitter | library, player bar), prefs in
  `aurago.desktop.noisemaker.prefs`, API calls to `/api/desktop/noisemaker/*`
  (state, enhance, generate, tracks, PATCH/DELETE tracks/{id}), window +
  context menus, compact mode (< 860 px). Loads after the four sub-modules
  below (see `module-loader.js`). The shell is the only module that talks to
  the network; it must call only `/api/desktop/noisemaker/*` and never
  receives provider credentials. Capability gating (music disabled, no LLM,
  no cover AI, lyrics unsupported) is driven by `/api/desktop/noisemaker/state`;
  a disabled integration renders the onboarding card instead of the workbench.
  A failed state request renders a retryable connection error, never setup;
  background refresh failures preserve the current capabilities and workbench.
  Tracks are server-paginated (`limit`/`offset`/`q`/`favorites=1`, newest
  first); favorites are the `favorite` media tag toggled via PATCH. HTTP 200
  `{status:error}` track pages throw and leave the current list in place; toasts
  use the server message or `desktop.noisemaker_error_unknown` (never on
  `AbortError`). `needmore` runs `continueQueue`: enqueue remaining loaded
  tracks via `queueHas` (library order), else wait for a library load that is
  already running (`tracksInFlight`) and re-check, else fetch the next page;
  `cancelPendingAutoplay` only when nothing more can arrive. Toasts go through
  the shell's `toast()` helper, which builds the `{ title, message, type,
  appId }` payload `showDesktopNotification` expects. Compact windows (`is-compact`, < 860 px) apply the same narrow
  player, list-row and Now-Playing rules as the 720 px viewport fallback.
  Volume `change` events still save prefs; the menubar rebuilds only when
  shuffle, repeat, visualizer or muted change. Visible UI strings use
  `desktop.noisemaker_*` keys plus `desktop.app_noisemaker` in all
  `ui/lang/desktop/*.json` files.
- `noisemaker-menus.js` — `window.NoisemakerMenus = { windowMenus(m),
  trackContextItems(m, track), libraryContextItems(m), withCheckIcons }`;
  pure builders over a model `{ t, tFull, readonly, s, actions }`.
  `t(key, params, fallback)` expects full keys (`desktop.noisemaker_*`),
  `tFull` is an alias; all five modules pass full keys so the static i18n key
  lint covers them.
- `noisemaker-library.js` — `window.NoisemakerLibrary = { create(deps),
  formatDuration, formatDate }`; grid/list views, favorites filter, search,
  multi-select, load-more; emits `play, enqueue, favorite, delete, template,
  download, contextmenu, create, loadmore, search, filter, view, selection`.
- `noisemaker-player.js` — `window.NoisemakerPlayer = { create(deps) }`; single
  `<audio>`, queue with shuffle/repeat/autoplay, Web-Audio visualizer,
  Now-Playing view; exposes `queueHas` and `cancelPendingAutoplay`;
  `pendingAutoplay` clears on play/setQueue/clearQueue/stopAudio. Volume slider
  `input` updates audio/UI only; `change` and mute emit prefs. Visualizer
  availability is queryable at mount; Now Playing shares `.is-viz-off`.
  Emits `state, change, favorite, delete, template, download, expand,
  needmore, error, visualizer-unavailable`.
- `noisemaker-create.js` — `window.NoisemakerCreate = { create(deps), PRESETS }`;
  Simple/Custom modes, genre presets, AI enhance, local ACE-Step controls,
  progress + result card; the root carries the current mode as
  `data-nm-create-mode`; the segment buttons carry
  `data-nm-mode="simple|custom"`. Emits `generate, change, mode, play-result,
  show-in-library, new-song`.
  Read-only mode blocks generation and enhancement at both UI and action entry
  points. Validate retained local controls in both modes; show the cover toggle
  in both modes and required lyrics when the local model has no language model.
  Enhancement results may only replace the unchanged field of the latest
  request. `TestDesktopNoisemakerAuditBrowser` covers these flows and compact
  list geometry in Standard and Fruity themes.
- `editor-filemenu.js` implements `renderFiles`, file management helpers and the inline text
  editor with window menus (file, edit, agent, help). Fallback file-list
  empty-state load failures use `desktop.load_failed`. Bundled in the
  main shell bundle (`desktopMainParts` in `build-ui-bundles.js`) because
  it is referenced directly by the desktop foundation runtime.
  Failed text-file loads retain Retry/New/Open and block editing/saving the
  unloaded document. A 404 starts an empty buffer only for explicit creation;
  restored/opened files keep load intent through both shell entry points.
  Clear their persisted path only on confirmed 404s, retaining temporary failures
  for Retry. Verify `TestDesktopFileLoadRecoveryBrowser`.
- `planning-gallery-music.js` - Planner/todo, gallery and Webamp music.
  Bundled in the main shell. Todo and Gallery empty-state load failures
  use `desktop.load_failed`. Webamp unsupported-browser errors use
  `desktop.winamp_unsupported`; launcher `notifyError` maps the
  English sentinel and reuses `desktop.load_failed` for other
  load failures. Leave the Webamp skin unchanged. No child DOX
  file needed.
- `quickconnect-launchpad-chat.js` - Quick Connect (`renderQuickConnect`
  device list and session chrome), store/launchpad and generated-app
  host. The synthetic AuraGo host card uses `desktop.qc_aurago_host` and
  `desktop.qc_aurago_host_description`; device-list empty-state load
  failures use `desktop.load_failed`. Store terminal-preview load
  failures use `desktop.store_terminal_load_failed`. Generated-app
  host empty-state, store container-app frame errors, start toasts,
  and external-open notifications use `desktop.load_failed`. Bundled
  in the main shell. No child DOX file needed.
- `store-terminal-preview.js` - CommandCode console-plus-preview
  host. CommandCode stays visible above an initially hidden shell drawer.
  Terminal toggles reuse its live sessions; Plus adds a shell in the same
  container working directory (`/workspace`). Only tab close, explicit restart
  and window disposal close shell sockets. Clipboard, focus and status remain
  session-scoped; hiding restores CommandCode focus. Keep all 16 locale labels,
  narrow-window layout and `TestDesktopStoreTerminalDrawerBrowser` aligned.
  Frame empty-state and start-toast failures reuse
  `desktop.load_failed`. Stylesheet and script loads wrap
  AuraLazyAssets and fallback `onerror` with
  `desktop.store_terminal_load_failed` so the asset URL does not
  leak. Loaded lazily. Exposes
  `window.StoreTerminalPreviewApp`. No child DOX file needed.
- `sheets.js` - Native workbook host and lifecycle. Exposes
  `window.SheetsApp`. No child DOX file needed.
- `sheets-data.js` - Typed localized inputs, CSV preview/import, templates and
  native locale supplementation. Exposes `window.SheetsData`.
- `sheets-panels.js` - Format/data/chart/AI/search/navigation/print panels.
  Exposes `window.SheetsPanels`.
- `sheets-charts.js` - Chart.js overlay, sheet anchoring and undoable chart
  operations. Exposes `window.SheetsCharts`.
- `code-studio/core.js` - Code Studio core: state management, API client, path
  utilities, lifecycle (render/dispose), shell markup, toolbar, tabs, breadcrumbs,
  status bar, file operations, window menus. Zen-mode exit title reuses
  `codeStudio.exitZen`. Opens the shared IIFE. No child DOX
  file needed.
- `code-studio/sidebar.js` - File explorer: keyboard-navigable tree view,
  path crumbs and header tools, expand/collapse, drag & drop upload into the
  hovered folder, file actions (rename/delete/download), tree context menu,
  active-file highlight, activity bar. No child DOX file needed.
- `code-studio/editor.js` - CodeMirror and textarea editor views, linked-view
  sync for split panes, Ctrl+wheel zoom, syntax highlighting integration. No
  child DOX file needed.
- `code-studio/terminal.js` - Terminal sessions with xterm.js, WebSocket
  connection, multi-session management. Tab names and the xterm welcome
  line use `codeStudio.shell_n` plus `codeStudio.title`. No child DOX
  file needed.
- `code-studio/search.js` - Search-in-files panel with grep, result navigation.
  No child DOX file needed.
- `code-studio/agent.js` - Agent chat panel, SSE streaming, diff preview,
  code actions (explain/comments/tests/refactor), markdown rendering. No child
  DOX file needed.
- `code-studio/git.js` - Git panel: branch display, change list, diff view,
  commit dialog, recent log. No child DOX file needed.
- `code-studio/panels.js` - Split editor (right/down) with two synced views of
  the active tab and a resizable divider. No child DOX file needed.
- `code-studio/shortcuts.js` - Keyboard shortcuts, shortcut overlay, command
  table, exposed API, `window.CodeStudioApp` assignment. Closes the shared
  IIFE. No child DOX file needed.
- `code-studio/command-palette.js` - Command palette overlay with fuzzy search
  over commands, open/recent/tree files, keyboard navigation; executes through
  `CodeStudioApp.command`. Separate IIFE. No child DOX file needed.
- `sysworld.js` - Per-window lifecycle, lazy ESM loading, visibility, menus,
  selection, bounded neighbourhood requests and the visibility-gated memory
  artifact feed for the hologram. Exposes `window.SysWorldApp`.
- `sysworld-data.js` - Shared read-only REST/SSE data lifecycle and stable entity
  records with source timestamp/status; no persistent history store.
- `sysworld-scene.js` - Isolated Three.js city renderer, GLB LOD cache,
  instancing, PBR/PMREM, shadows, `SceneCapturePass` + bloom + output +
  atmosphere post chain, camera modes and disposal. Build to
  `ui/js/vendor/system-world/`; never classic-script load this source.
- `sysworld-hud.js` - Theme-native, localized HTML metrics with CPU/RAM sparklines,
  district navigation with state badges, entity search, inspector, SVG plan map, street
  controls and compass, tour lower third, loading progress, photo mode and
  decluttered projected district labels.
- `sysworld-life.js` - Shared robot assets, street routes, hover lights and district
  status effects; imports only into the city bundle.
- `sysworld-navigation.js` - Pure robot steering: arc-length routes, lanes,
  obstacle circles, conflict prediction, yielding, U-turns and separation.
  Node-testable without Three.js.
- `sysworld-hologram.js` - Memory archive hologram: cone, rings, motes and
  canvas-text panels cycling sanitized artifacts.
- `sysworld-atmosphere.js` - Sky, aurora, stars, moon, sea, mist, dust, spire
  beacon, lamp cones and the vignette/grain post pass.
- `sysworld-drones.js` - Three patrolling service drones on closed spline loops.
- `sysworld-audio.js` - Gesture-unlocked opt-in ambient audio and shared master mixer.
- `sysworld-voice.js` - Transient TTS phrase playback, spatial echo/reverb and cancellation.
  These modules need no additional child DOX.
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
  pinned in `homepage-studio.js`. Local webhost name fallback uses
  `homepage_studio.default_name`; chat-stream failures and the
  missing parser throw reuse `desktop.chat_request_failed`. Exposes `window.HomepageStudioApp` plus
  `HomepageStudioPreview`/`HomepageStudioSites`/`HomepageStudioHistory`
  factories. No child DOX file needed.
- `log-viewer-filters.js` / `log-viewer.js` - Log Viewer: file sidebar,
  virtualized tail list, level/search filters, dedicated per-window
  EventSource to `/api/desktop/logs/stream`, readonly-gated download.
  Exposes `window.LogViewerFilters` then `window.LogViewerApp`. No child
  DOX file needed.
- `noisemaker-menus.js` - Noisemaker window/context menu builders. Exposes
  `window.NoisemakerMenus`; pure builders over `{ t, tFull, readonly, s,
  actions }`. Loads before `noisemaker.js`. No child DOX file needed.
- `noisemaker-library.js` - Noisemaker library grid/list, favorites, search,
  multi-select and pagination. Exposes
  `window.NoisemakerLibrary { create, formatDuration, formatDate }`. No child
  DOX file needed.
- `noisemaker-player.js` - Noisemaker player bar, queue, visualizer and Now
  Playing view. Exposes `window.NoisemakerPlayer { create }`. No child DOX
  file needed.
- `noisemaker-create.js` - Noisemaker song creation (Simple/Custom modes,
  presets, ACE-Step controls). Exposes
  `window.NoisemakerCreate { create, PRESETS }`. No child DOX file needed.
- `noisemaker.js` - Noisemaker workbench shell orchestrating create pane,
  library and player. Exposes `window.NoisemakerApp { render, dispose }`;
  loads after the four sub-modules. No child DOX file needed.
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
- `zipper.js` - Zip archive browser: list/create/extract, desktop and host
  file drops, breadcrumb navigation inside an archive. Double-click, Enter, or
  File → Open preview a member without extracting the whole zip. Images, text,
  audio, and video render in an in-app overlay from
  `GET /api/desktop/archive/entry`. PDF, Markdown, and Office files open Viewer
  with `{ path, archiveEntry, forceNew: true }`; STL opens Viewer 3D the same
  way. Executables and other blocked types stay closed. Visible strings use
  `zipper.*` in all 16 `ui/lang/desktop/*.json` files. The open-dialog
  filter uses `desktop.file_dialog_zip`. Exposes
  `window.ZipperApp`. No child DOX file needed.
- `viewer.js` / `viewer-3d.js` - Viewer and STL viewer accept optional
  `archiveEntry` with the zip `path` and `forceNew: true`. Archive members
  load from `/api/desktop/viewer/content?path=&entry=` or
  `/api/desktop/archive/entry`; Viewer hides Edit for archive members.
  Missing markdown-it shows `viewer.error` only. Viewer 3D missing
  STLLoader throws and maps `viewer.error`. Missing print-frame
  errors throw `desktop.print_failed`; the print catch still
  prefixes `viewer.error`. No child DOX file needed.
- `teevee.js` - IPTV player; `teevee-catalog.js` owns catalog loading,
  stream identities, search and display labels. Catalog HTTP throws the
  sentinel `iptv-org HTTP` without a status. `fetchJSON` must
  not call `t()`. `loadCatalog` shows `desktop.teevee_catalog_error`.
  Loaded lazily after `teevee-crt.js` and `teevee-catalog.js` (which exposes
  `window.AuraTeeVeeCatalog`). Exposes `window.TeeVeeApp`. The real-shell
  `TestDesktopTeeVeeBrowser` covers video, HLS/AES, origin fallback, controls and
  lifecycle; opt in with `AURAGO_RUN_BROWSER_SMOKE=1`. No child DOX file needed.
- `game-maker-studio.js` - Game Maker Studio shell. Missing
  skills/revisions modals throw `game_maker.modules_load_failed`
  via `state.context.t(key)`. Exposes `window.GameMakerStudioApp`.
  Its optional screenshot-review strip has an explicit, height-bounded grid row;
  status text and thumbnails must not squeeze the live game into a narrow band.
  Verify wide/narrow layout and stale-review cleanup with
  `TestGameMakerVisualStripLayoutBrowser` / `TestGameMakerManualVisualLifecycleBrowser`.
  No child DOX file needed.
- `pixel-state.js`, `pixel-view.js`, `pixel-canvas.js`, `pixel-tools.js`,
  `pixel-actions.js`, `pixel-filters.js`, `pixel-events.js`, `pixel.js` -
  Pixel image editor: tool rail + options bar layout, 17 tools (magic wand
  with mask selections, gradient, airbrush, dodge/burn, blur brush plus the
  classic set), 21-filter catalog gallery with live thumbnails and strength
  slider, layers, click-to-jump history panel, AI generate/enhance. The
  open-dialog filter uses `desktop.file_dialog_images`. Save-dialog
  filters use `desktop.file_dialog_png`, `desktop.file_dialog_jpeg`,
  and `desktop.file_dialog_webp`. Image-decode failures throw
  `pixel.error_load`. Exposes `window.PixelApp`. No child DOX file
  needed.
- `terminal.js` - Standalone workspace terminal: one xterm.js session to
  `/api/code-studio/terminal`. Style catalog in `terminal-styles.js`
  (`window.TerminalStyles`: `ids`, `normalize`, `load`, `save`, `profile`,
  `applyXterm`, `effectControls`, `loadEffects`, `saveEffects`, `resetEffects`).
  IDs: `modern`, `amber`, `green`, `apple2`, `commodore64`,
  `ibm3278`, `vintage`, `mono-green`, `transparent-green`. Persist
  `aurago.desktop.terminal.style` and audio mute
  `aurago.desktop.terminal.audioMuted`. Retro styles use vendored
  xterm 6 with its WebGL addon, original WebGL CRT in `terminal-crt.js`
  (`window.TerminalCrt.create` → `setProfile`/`setEnabled`/`resize`/`dispose`/`usesFallback`;
  captures every `.xterm-screen` canvas at its CSS offset and scale;
  output is capped at DPR 1.25 and 30 fps, never stretches text to fill the tube),
  CSS bezels, and Web Audio key-clicks in `terminal-audio.js`
  (`window.TerminalAudio.create` → `setProfile`/`setMuted`/`playKey`/`dispose`).
  Load order: xterm.css, desktop-app-terminal.css, xterm, fit, WebGL addon,
  styles, crt, audio, terminal.js. Scope is this app only. Reduced motion
  and `dataset.animations === 'false'` disable flicker, burn-in, animated grain, and audio.
  The native Effects dialog applies bounded sliders to the existing renderer,
  without recreating xterm or its socket. Preferences live per style in
  `aurago.desktop.terminal.effects.v1`; malformed/blocked storage falls back
  to presets, and reset affects only the selected style. Overall intensity zero
  restores unwarped source output and removes reflection. Brightness, bloom,
  scanlines, curvature, afterglow, phosphor mask, vignette, glass reflection,
  noise, flicker, jitter, rolling interference and color fringing are independent.
  Reduced motion also pauses jitter/interference; static adjustments remain.
  Glass reflection is a pointer-transparent CSS layer, available in fallback.
  CSS fallback supports intensity/brightness/bloom/scanlines/vignette/reflection;
  the dialog disables unsupported effects and explains the limitation. Dispose
  closes the dialog and releases its media-query/mutation observers. Zero burn
  removes temporal persistence even when instantaneous bloom remains enabled.
  Retro appearance follows cool-retro-term's luminous phosphor, scanlines,
  subtly curved glass and recessed bezel using original rendering code. Keep
  profile curvature gentle so text rows remain nearly straight. Share Tech
  Mono is embedded as `Aura Terminal`; pixel profiles retain Press Start 2P.
  Additive bloom and decaying persistence share a half-resolution blurred
  source buffer; never feed warped output back into the source. The native
  xterm layer stays interactive and is visually hidden only after a WebGL frame.
  Housing materials, seams, vents and localized wear live in the app CSS;
  older Apple II/Vintage cases show more wear. Decorative hardware is hidden
  from accessibility and pointer input. Its LED follows the existing localized
  socket status via `data-terminal-state`; Modern stays frameless. Keep compact
  cases inside the app without changing xterm's measured screen padding.
  Browser verification is
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopTerminalRetroBrowser -count=1`.
  WebGL/canvas failure uses CSS fallback and keeps the WebSocket. Style
  changes wait for `document.fonts.load` before changing xterm options or
  creating the canvas renderer, then fit once. Ignore stale font completions
  after another style selection or disposal. Exposes
  `window.TerminalApp = { render, dispose }` with a per-window instances Map.
  The standalone window opens at 960x720 (4:3 CRT framing), scaling both axes
  together on smaller desktops; saved session bounds, user resizing and mobile
  maximization remain unchanged. Browser coverage delays the C64 font on first
  load and checks its measured glyph width as well as constrained opening sizes.
  Visible strings use `desktop.terminal_*` plus `desktop.terminal_style*` and
  `desktop.terminal_audio*` in all 16 desktop locales. No child DOX file
  needed.
- Notes uses the compact Autor-style shell with library, outline/info/search/AI panels,
  global Fruity menus and a responsive flowing note surface. Markdown stays authoritative
  under Documents/Notes. notes.js owns each window, versioned /api/desktop/notes I/O,
  explicit IndexedDB recovery and the shared OfficeSession close/save lifecycle.
- notes-editor.js uses local Milkdown/Crepe/Kit 7.22.2 (MIT), public ProseMirror APIs
  and the existing CodeMirror bundle. Rich editing is default; unsupported syntax and
  notes above 200k characters remain in source mode. notes-frontmatter.js preserves
  unknown fields and line endings while updating tags. No obsolete NotesList or
  NotesToolbar runtime remains. Load order: frontmatter, writer-session, editor, entry.
- The notes.meta.json sidecar keeps version/pinned/sort/last_note. Full-text search is
  server-side, shared with desktop_notes; no browser 500-file index. Relative attachments
  and link destinations survive moves; user trash preserves original folder paths.
  A missing sidecar returns 404 through the real Notes handler so first use can
  create it conditionally; wrapped filesystem errors must retain that status.
- Agent Notes access is list/search/read/create only in native APIs, including
  agent-created notes. Permitted local execution follows the sandbox setting;
  disabled isolation or explicit unsafe fallback can bypass native file guards.
  Active isolation still checks Notes write-path overlap. See documentation/desktop-notes.md.
- Keep .vd-notes-app and .vd-notes-toolbar for the common theme bridge. Use the shared
  icon renderer and notes/writer/common translations in all 16 Desktop locales.
  Validate TestDesktopNotesAppBrowser, the AURAGO_NOTES_MATRIX shell fixture, vendor
  --check and OfficeSession tests. The API, permission and packaging contracts are in
  documentation/desktop-notes.md. No child DOX file needed.
