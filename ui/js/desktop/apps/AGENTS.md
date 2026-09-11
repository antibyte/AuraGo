# Desktop App Modules - Child DOX Contract

## Purpose

This subtree owns built-in virtual desktop app modules that are loaded lazily by
`ui/js/desktop/core/module-loader.js`.

- Game Maker's existing asset browser includes sprite packs and individual 3D
  models. `game-maker-studio-models.js` owns one disposable viewer per modal,
  sharing the pinned local 0.185.1 runtime helper. Cancel pending fetches and
  release rigs, controls, shadow maps and renderer on replacement/close/dispose.
  Render only on interaction or while an animation is playing and visible.
  Selection uses up to 64 `model_asset_ids`, separate from sprite pack selection.
  Keep all labels in the sixteen desktop locales and preserve both import flows.

Shell chrome helpers live in the main desktop bundle (not lazy apps):
`core/session-runtime.js` (session restore, dock pins, recent files, default
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
  Keep the dialog keyboard accessible, focus trapped and disposed on app close.
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
- Mission Control trigger min-interval uses `desktop.rel_time_seconds` with
  `{{count}}`. Keep SIP/Noisemaker duration formatting unchanged.
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
  `desktop.file_dialog_png`, `desktop.file_dialog_jpeg`, and
  `desktop.file_dialog_webp`. Do not hardcode `Images`,
  `PNG Image`, `JPEG Image`, or `WebP Image` there.
  Pixel `t` stays key-only. Leave File Manager and
  OpenSCAD byte formatters unchanged.
- Quick Connect SFTP status uses `desktop.qc_sftp_items`. SFTP sizes
  use `desktop.bytes`, `desktop.kib`, `desktop.mib`, `desktop.gib`,
  and `desktop.tib`. Do not hardcode `items` or `B`/`KiB`/`MiB`/`GiB`
  there.
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
- Mission Control window menus use `desktop.menu_file` and
  `desktop.menu_view`. Do not hardcode English File/View there.
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
- Restore only moves paths under `trash/…`. Destination is `Desktop/<name>`
  with unique names via `trashNameCandidate`. Never overwrite an existing
  Desktop name. Nested `Trash/foo/bar` restores the basename to `Desktop/bar`.
- Origin paths are not persisted in v1. Do not change `listTrashEntries` or
  `movePathToTrash` to record origin.
- Readonly denies restore and empty-trash mutations. Delete inside Trash stays
  a permanent DELETE.
- Shell callbacks `restoreFromTrash` and `emptyTrash` are injected from
  `menus-and-routing.js`. Empty Trash in the File Manager empty-folder menu
  calls that callback; do not reimplement emptying in the File Manager.

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
- Gallery preview letterboxes (`.vd-gallery-preview`) may stay dark for media
  contrast; card chrome and media preview bars follow theme.
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
  The Apache-2.0 core and local fonts are pinned to 2.16.0 and generated by
  `scripts/build-writer-vendor.js`. Do not add Pro packages or Quill to Autor.
- `calendar.js` implements the Calendar renderer, appointment menus, drag/drop,
  recurring appointment creation, and modal lifecycle. It is a continuation
  inside the shared Desktop IIFE and is bundled immediately before
  `core/sdk-events-bootstrap.js`; it is not loaded lazily.
- `cheater*.js` implements the Cheater app, a cheat-sheet manager with a
  textarea-based Markdown editor, live preview, Markdown toolbar, command
  palette (spotlight), and attachments side panel.
- `sheets.js` owns Tabellen's Autor-style chrome, document lifecycle and native
  Univer OSS 0.25.1 instance. `sheets-data.js` owns localized inputs, CSV and
  templates; `sheets-panels.js` owns formatting, data, search, AI and print;
  `sheets-charts.js` renders five editable Chart.js chart types on the sheet.
- `code-studio/*.js` implements Code Studio, a full IDE with file explorer,
  CodeMirror editor, terminal, search, agent chat with SSE streaming, Git
  integration, split editor, and keyboard shortcuts. Split across `core.js`
  (state management, API client, lifecycle, shell), `sidebar.js` (file tree),
  `editor.js` (CodeMirror/textarea), `terminal.js` (xterm.js sessions),
  `search.js` (search-in-files), `agent.js` (agent chat, diff preview),
  `git.js` (Git panel, diff view, commit), `panels.js` (split editor,
  panel management), `shortcuts.js` (keyboard shortcuts, window.CodeStudioApp),
  and `command-palette.js` (separate IIFE).
- `sysworld*.js` implements System World's Blender-authored data metropolis:
  seven fixed districts, live read-only inspector, entity search, map, explicit
  tour and street-level WASD/touch exploration. The isolated Three.js 0.185.1
  ESM renderer does not replace the legacy global used by other apps. Opens
  maximized; existing dashboard/KG/mission APIs and shared SSE supply live data.
  Persistent 24-hour history remains a subsequent stage in
  `documentation/system-world-plan.md`.
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
  compact window (preset 440×520, min 340×460 in
  `window-shell-runtime.js`; panel mounted with `compact: true`).
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

- Built-in app load order is defined in `ui/js/desktop/core/module-loader.js`.
- `calendar.js` is the Calendar source of truth. `desktopMainParts` in
  `scripts/build-ui-bundles.js` must place it after the split app continuations
  and before `core/sdk-events-bootstrap.js` so `renderCalendar` stays inside
  the shared Desktop runtime closure without duplication.
- Game Maker Studio loads in the order `game-maker-studio-api.js`,
  `game-maker-studio-preview.js` (`window.GameMakerStudioPreview`: loading
  overlay, stale badge, fullscreen, new-tab), `game-maker-studio-modals.js`
  (`window.GameMakerStudioModals`: shared modal lifecycle, media toggles,
  skills and revisions; confirmation stays shell-mediated),
  `game-maker-studio-assets.js` (`window.GameMakerStudioAssets`: offline pack
  catalog, selection, sprite/animation previews), then
  `game-maker-studio.js`.
- Game Maker Studio exposes `window.GameMakerStudioApp = { render, dispose,
  instances }`. Every window owns and closes its EventSource, preview iframe,
  channel ID, diagnostics, modal handlers, job-elapsed and busy-poll timers,
  document-level overflow-menu listeners, and `message` listener.
- Sprite selection is window-local and prepares the next create/edit request
  via `asset_pack_ids`; selecting packs never starts a job. Clear selection only
  after job acceptance. The asset browser owns an AbortController and animation
  timer, released on modal replacement/close and disposal. Metadata stays English
  for agents; controls and pack titles cover all 16 locales. Preview backgrounds
  are CSS only: PNGs contain genuine alpha, no painted checkerboard.
- Modular pack previews default to a complete assembly. Parts use metadata
  coordinates and shared animation time; selecting an individual sprite exits
  assembly mode. Reuse the window's existing preview timer and cleanup path.
- Game Maker previews must use `sandbox="allow-scripts"` without
  `allow-same-origin` (`allowfullscreen` on the iframe is permitted).
  Preview HTTP responses also send CSP `sandbox allow-scripts` so "open in
  new tab" cannot become a first-party AuraGo origin.
  Accept diagnostics only from the instance iframe when `event.source`, the
  random channel ID, the fixed source marker, and the bounded event type all
  match. Forward bounded, deduplicated reports through the authenticated
  `preview-report` API using the parent-held preview token; never grant the
  iframe API credentials. Reports bind to a specific validation build. Include
  current-preview errors as untrusted diagnostics in the next change request.
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
- Sheets exposes `window.SheetsApp = { render, dispose, instances }`. Each
  window owns its engine/worker, requests, panels, chart canvases, operation
  journal and save queue. Load OfficeSession, data, panels and charts before
  sheets.js. The local vendor build imports only Apache-2.0 Univer OSS 0.25.1.
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
- Writer pointer selection must preserve the viewport, including clicks near its
  edges after toolbar focus. The core 2.16.0 vendor build carries a guarded,
  reproducible patch for pointer-only caret reveal; keyboard/programmatic reveal
  and drag edge autoscroll remain enabled. Keep the vendor modification notice
  and browser regressions when rebuilding or upgrading the core.
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
- System World loads `sysworld-data.js`, `sysworld-hud.js`, then `sysworld.js`.
  The first two expose `window.SysWorld.data/createHud`; the entry owns per-window
  instances and exports `SysWorldApp.render/dispose/inspect`. It imports
  versioned `/js/vendor/system-world/city.esm.js` only when opened.
- `sysworld-scene.js` is build input, not a classic lazy script. Rebuild with
  `node scripts/build-system-world.js` after changes; `--check` verifies exact
  output. Three.js and matching addons stay pinned to MIT 0.185.1.
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
- `sysworld-life.js` owns exactly five decorative white ThreeDee robots, rounded
  street routes, shared assets/soft hover lights and state-driven district rings.
  Normalize the GLB face axis (+X) to route-forward (+Z) before cloning;
  keep faces aligned with travel on straights and rounded turns.
  Only fresh actual error/running states or bounded recent events animate district
  signals; stale/unknown stays neutral. The operations signal uses private material
  clones so its red pulse cannot recolor other buildings. Rebind after LOD swaps.
  Reduced motion/Desktop animation settings freeze residents and pulses.
- City radio waves consume typed `agent_action` starts and executed results, plus
  co-agent progress. Exact tool names map to districts; unknown tools, previews,
  blocked proposals and metric snapshots must not invent building-to-building traffic.
  Keep only route/state/tool-name metadata, never arguments, result/error text or
  session content. Deduplicate action states in a bounded map and clear it on close.
  `sysworld-life.js` shares twelve wave packets and six route curves on its existing
  RAF. Coalesce bursts, expire by wall time, discard hidden/map/reduced-motion starts,
  and never replay retained events when a window opens or resumes. Robot geometry
  and the shared legacy Three.js remain unchanged.
- `sysworld-audio.js` owns quiet native Web Audio synthesis. Sound is opt-in,
  persisted, gesture-unlocked, volume-bounded and fades/suspends when the app is
  hidden, unfocused or in map mode. Close releases oscillators, nodes and AudioContexts.
  `sysworld-voice.js` adds transient tower speech to that same opt-in mixer: one
  cancellable POST `/api/desktop/system-world/voice`, 20–40 seconds of quiet after
  each short phrase, with 60-second failure backoff. Camera pose updates through
  the existing scene RAF; no extra render loop. Distance to the tower attenuates
  the complete dry/85-ms echo/0.85-second stereo-room mix, with peak limiting before
  gain and stereo placement. Master volume remains bounded to 35%; no media cache,
  raw text diagnostics, automatic retries without backoff or browser-TTS fallback.
  Hide, focus loss, map, mute/zero-volume and close abort requests, stop playback,
  release phrase nodes/tails and prevent late fetch/decode responses from replaying.
  Verify with `node scripts/test-system-world-voice.mjs` and the real-shell city test.
- Rendering owns one RAF per visible window. Minimize, Spaces, document hiding
  and map mode stop it. Close aborts loaders and frees GPU resources, listeners
  and observers. Context loss falls back to the usable map. Keep all models,
  local materials and licenses build-versioned; no remote textures or services.
- Quality `auto/low/medium/high/ultra` persists under
  `aurago.desktop.sysworld.quality`. LODs are separately fetched and cached;
  instanced scenery uses shared geometry/materials, distant towers always LOD2.
  Quality changes rebuild instances after assets finish without moving focus.
  Auto uses frame-time hysteresis; shadow maps update only after geometry/tier
  changes. No per-frame geometry or material creation.
- District placement and entity IDs stay fixed through data refresh. User
  navigation cancels the explicit tour; inactivity never takes the camera.
  Reduced motion and Desktop animation settings suppress camera flights/tours
  and the decorative pulse. Street mode owns WASD only while its canvas is
  focused; pointer lock is explicit and Escape/blur/close release control.
- Verify `node scripts/test-system-world.mjs`, `node scripts/build-system-world.js
  --check`, focused Sysworld Go tests, and the real-shell browser matrix:
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_SYSTEM_WORLD_MATRIX=1 go test ./ui
  -run '^TestDesktopAuroraBrowser$'`. Screenshot/performance reports stay ignored.

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
- Keep System World's data, HUD, scene, life, audio and lifecycle in their owned files.
  Do not merge renderer dependencies or data polling into the Desktop shell.
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
- `go test ./ui/ -run TestDesktopMissionControlMenuI18n`
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
- `go build ./cmd/aurago`

## Child DOX Index

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

- `meshcore.js`: native Messenger (`window.MeshCoreApp.render/dispose/openConversation`), using `/api/meshcore/messenger/` and the existing Companion manager. Owns direct/channel conversations, protected-text reveal, contact/channel dialogs, native BarcodeDetector import and the existing QRCode renderer. No direct hardware connection or agent-tool sending. Theme styles live in `css/desktop-app-meshcore.css`; the 300px list switches to single-pane navigation below 700px. The "Mesh" visual system owns a scoped `--mc-*` token layer in that stylesheet (teal signature accent with per-theme variants, gradient outgoing bubbles, hash-hue avatars, frosted sticky day pills, skeleton loading). Inline stroke SVG icons are built into the JS via `icon()`/`iconEl()` — no external assets. Messages group by direction/origin within a 300 s gap via `mc-group-start/mid/end` classes; day changes always break groups. Message actions (reveal/copy/retry) reveal on hover/focus-within via `opacity` only — never `pointer-events`, `visibility`, or `display` — because the browser test clicks them by coordinates; they stay visible on touch and narrow layouts. The conversation list supports ArrowUp/ArrowDown roving focus; Esc closes the detail panel or leaves the chat pane unless a dialog is open. Drafts and pending send IDs are local per device/conversation; invitation keys never enter browser storage. Persistent request IDs reconcile HTTP retries, explicit resend warns about duplicates. Abort all requests and remove document listeners/timers on dispose. Session/notification context contains only a validated conversation ID. All 16 desktop locales must include `desktop.meshcore_*`.

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
- `calendar.js` - Calendar renderer and appointment UI continuation bundled
  inside the shared Desktop IIFE immediately before `sdk-events-bootstrap.js`.
  Empty-state load failures use `desktop.load_failed`. No child DOX file
  needed.
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
  editor with window menus (file, edit, agent, help). Fallback file-list
  empty-state load failures use `desktop.load_failed`. Bundled in the
  main shell bundle (`desktopMainParts` in `build-ui-bundles.js`) because
  it is referenced directly by the desktop foundation runtime.
- `planning-gallery-music.js` - Planner/todo, gallery, Webamp music, and
  Quick Connect device list. Bundled in the main shell. The synthetic
  AuraGo host card uses `desktop.qc_aurago_host` and
  `desktop.qc_aurago_host_description`. Todo, Gallery, and Quick
  Connect device-list empty-state load failures use
  `desktop.load_failed`. Webamp unsupported-browser errors use
  `desktop.winamp_unsupported`; launcher `notifyError` maps the
  English sentinel and reuses `desktop.load_failed` for other
  load failures. Leave the Webamp skin unchanged. No child DOX
  file needed.
- `quickconnect-launchpad-chat.js` - Store/launchpad, generated-app
  host, and Quick Connect session chrome. Store terminal-preview load
  failures use `desktop.store_terminal_load_failed`. Generated-app
  host empty-state, store container-app frame errors, start toasts,
  and external-open notifications use `desktop.load_failed`. Bundled
  in the main shell. No child DOX file needed.
- `store-terminal-preview.js` - CommandCode console-plus-preview
  host. Frame empty-state and start-toast failures reuse
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
- `code-studio/sidebar.js` - File explorer: tree view, expand/collapse, drag &
  drop upload, file actions (rename/delete/download), activity bar. No child DOX
  file needed.
- `code-studio/editor.js` - CodeMirror and textarea editors, syntax highlighting
  integration. No child DOX file needed.
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
- `code-studio/panels.js` - Split editor (horizontal/vertical), resizable
  divider, panel pinning. No child DOX file needed.
- `code-studio/shortcuts.js` - Keyboard shortcuts, shortcut overlay, exposed
  API, `window.CodeStudioApp` assignment. Closes the shared IIFE. No child DOX
  file needed.
- `code-studio/command-palette.js` - Command palette overlay with fuzzy search,
  keyboard navigation. Separate IIFE. No child DOX file needed.
- `sysworld.js` - Per-window lifecycle, lazy ESM loading, visibility, menus,
  selection and bounded neighbourhood requests. Exposes `window.SysWorldApp`.
- `sysworld-data.js` - Shared read-only REST/SSE data lifecycle and stable entity
  records with source timestamp/status; no persistent history store.
- `sysworld-scene.js` - Isolated Three.js city renderer, GLB LOD cache,
  instancing, PBR/PMREM, shadows, bloom, camera modes and disposal. Build to
  `ui/js/vendor/system-world/`; never classic-script load this source.
- `sysworld-hud.js` - Theme-native, localized HTML metrics, district navigation,
  entity search, inspector, map, street controls and projected district labels.
- `sysworld-life.js` - Shared robot assets, street routes, hover lights and district
  status effects; imports only into the city bundle.
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
  `applyXterm`). IDs: `modern`, `amber`, `green`, `apple2`, `commodore64`,
  `ibm3278`, `vintage`, `mono-green`, `transparent-green`. Persist
  `aurago.desktop.terminal.style` and audio mute
  `aurago.desktop.terminal.audioMuted`. Retro styles use vendored
  `xterm-addon-canvas`, original WebGL CRT in `terminal-crt.js`
  (`window.TerminalCrt.create` → `setProfile`/`setEnabled`/`resize`/`dispose`/`usesFallback`;
  captures only `xterm-*-layer` canvases at their CSS offsets and scale;
  output is capped at DPR 1.25 and 30 fps, never stretches text to fill the tube),
  CSS bezels, and Web Audio key-clicks in `terminal-audio.js`
  (`window.TerminalAudio.create` → `setProfile`/`setMuted`/`playKey`/`dispose`).
  Load order: xterm.css, desktop-app-terminal.css, xterm, fit, canvas,
  styles, crt, audio, terminal.js. Scope is this app only. Reduced motion
  and `dataset.animations === 'false'` disable flicker, burn-in, animated grain, and audio.
  Retro appearance follows cool-retro-term's luminous phosphor, scanlines,
  subtly curved glass and recessed bezel using original rendering code. Keep
  profile curvature gentle so text rows remain nearly straight. Share Tech
  Mono is embedded as `Aura Terminal`; pixel profiles retain Press Start 2P.
  Additive bloom and decaying persistence share a half-resolution blurred
  source buffer; never feed warped output back into the source. The native
  xterm layer stays interactive and is visually hidden only after a WebGL frame.
  Keep canvas addon 0.5.0 paired with xterm 5.3.0; provenance and license are
  beside `js/vendor/xterm-addon-canvas.min.js`. Browser verification is
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
- notes-editor.js uses local Milkdown/Crepe/Kit 7.22.1 (MIT), public ProseMirror APIs
  and the existing CodeMirror bundle. Rich editing is default; unsupported syntax and
  notes above 200k characters remain in source mode. notes-frontmatter.js preserves
  unknown fields and line endings while updating tags. No obsolete NotesList or
  NotesToolbar runtime remains. Load order: frontmatter, writer-session, editor, entry.
- The notes.meta.json sidecar keeps version/pinned/sort/last_note. Full-text search is
  server-side, shared with desktop_notes; no browser 500-file index. Relative attachments
  and link destinations survive moves; user trash preserves original folder paths.
- Agent access is list/search/read/create only, enforced in the backend and native
  mutation paths, including agent-created notes. Local execution requires checked
  isolation while Notes exist; see documentation/desktop-notes.md for platform limits.
- Keep .vd-notes-app and .vd-notes-toolbar for the common theme bridge. Use the shared
  icon renderer and notes/writer/common translations in all 16 Desktop locales.
  Validate TestDesktopNotesAppBrowser, the AURAGO_NOTES_MATRIX shell fixture, vendor
  --check and OfficeSession tests. The API, permission and packaging contracts are in
  documentation/desktop-notes.md. No child DOX file needed.
