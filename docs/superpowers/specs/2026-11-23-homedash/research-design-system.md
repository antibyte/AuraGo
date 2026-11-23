Research report for HomeDash (read-only; nothing modified). Repo root: C:\Users\Andi\Documents\repo\AuraGo (abbreviated R\ below; all paths absolute under that root).

1. THEMES
Desktop theme registry has 14 IDs, not 7-9: standard, fruity, amiga, cyberpunk, black_gloss, crazy, lollipop, organic, electronic_dreams, windows_xp, green_slime, voxel_world, orbit, minimal.
- Source of truth: R\internal\desktop\themes.go:9 and R\ui\js\desktop\core\theme-runtime.js:5-20 (DESKTOP_THEMES; test-parsed, order must match Go).
- Fruity and Minimal have light/dark toggles (appearance.fruity_mode, appearance.minimal_mode); scheme is body[data-scheme] (theme-runtime.js:44-47, 65).
- applyDesktopThemeAttributes sets data-theme, data-shell, data-start-menu, data-scheme, data-polish (theme-runtime.js:57-68). Unknown id falls back to Standard (:33-35).
- CSS: R\ui\css\desktop-base.css holds Standard (:472) and Fruity (:651). Other themes are R\ui\css\desktop-theme-<id>.css.
- Your named list is partly wrong: Sandstorm, Dark Sun, Galaxy, Ocean and Black Matrix are CHAT themes in R\ui\css\chat-themes.css (:6346, :3360, :5844, :5001, :9524), not desktop themes. Galaxy and Ocean exist only as desktop wallpapers (R\ui\js\desktop\core\spaces-runtime.js:43).
- Tokens, R\ui\css\desktop-base.css:
  - Legacy --vd-*, :3-52: --vd-bg, --vd-surface, --vd-surface-strong, --vd-border, --vd-text, --vd-muted, --vd-accent (+ -r/-g/-b), --vd-coral, --vd-amber, --vd-shadow, --vd-shadow-window, --vd-control-bg, --vd-control-hover, --vd-window-radius, --vd-z-*.
  - Design system --ds-*, :54-205 (preferred):
    - text: --ds-color-fg-primary/secondary/muted/disabled (:58-61)
    - surfaces: --ds-color-bg-canvas/raised/overlay/modal (:64-67); --ds-color-surface-0..3 (:70-73)
    - borders: --ds-color-border-subtle/default/strong/focus (:76-79)
    - accent scale: --ds-color-accent-50..900, -coral, -amber, -glow (:82-94)
    - status: --ds-color-success-fg, warning-fg, danger-fg, info-fg (:103-106)
    - spacing: --ds-space-1..10, 4px base (:109-118)
    - radius: --ds-radius-xs..xl, -full (:121-126)
    - elevation: --ds-elevation-1..5 (:129-133)
    - motion: --ds-motion-duration-fast/base/slow (120/200/220ms), --ds-motion-ease-standard/emphasized/decel/accel (:136-143)
    - type: --ds-type-size-xs..display (11-40px), --ds-type-weight-*, --ds-type-leading-* (:146-163)
    - z, blur, focus: --ds-z-* (:166-174), --ds-blur-fine/coarse (:177-178), --ds-focus-ring-width/offset (:181-182)
  - Font: --vd-font-ui: 'Geist', system-ui, sans-serif (R\ui\css\desktop-chrome.css:802); themes override it (e.g. Inter at R\ui\css\desktop-shell.bundle.css:14321). Orbit declares @font-face (R\ui\css\desktop-theme-orbit.css:15-38).

2. APP MATERIALS (--vd-mat-*)
- Vocabulary and rules: R\ui\css\desktop-materials.css:1-6. Surfaces: canvas, bar, side, card, well, field, btn, primary, selected, each with -color, -edge, -shadow, -border, -radius, -hover, -text. Every selector starts with .desktop-body; each theme sets every token; -color is opaque #rrggbb.
- Defaults: R\ui\css\desktop-materials.css:7-45. Per-theme files: R\ui\css\desktop-materials-<id>.css (Organic example: desktop-materials-organic.css:8-52).
- Opt-in: no flag. An app reads var(--vd-mat-x, old-value). Example, R\ui\css\desktop-app-notes.css:6, 10-12:
  .vd-notes-app button{...border-radius:var(--vd-mat-btn-radius,6px)...}
  .vd-notes-app button[aria-pressed=true]{background:var(--vd-mat-selected,...);color:var(--vd-mat-selected-text,inherit);box-shadow:var(--vd-mat-selected-edge,none)}
  .vd-notes-app input{border:1px solid var(--vd-mat-field-border,...);box-shadow:var(--vd-mat-field-shadow,none)}
- About 20 app stylesheets already use them (R\ui\css\desktop-app-*.css).
- Static test: R\ui\desktop_app_materials_test.go:11-30 (file limit 260 lines, :18; data-URI limit 4096 bytes, :21).

3. REUSABLE CONTROLS
- Switch: R\ui\js\desktop\apps\ha-switchboard.js:125 (role="switch", class ha-switch); CSS R\ui\css\ha-switchboard.css. Only role=switch in the desktop, and it sits in a Home Assistant app.
- Settings toggle/select: R\ui\js\desktop\core\settings-catalog-runtime.js:9 (settingSelect), :13 (settingToggle). Markup class UNVERIFIED.
- Slider: R\ui\js\desktop\core\quick-settings-runtime.js:55 (native input type=range). No shared slider class.
- Segmented control: none found (UNVERIFIED).
- Buttons: R\ui\css\desktop-app-common.css:412 (.vd-tool-button, .vd-button). Per-app styles dominate.
- Context menu: R\ui\js\desktop\core\menus-and-routing.js:387 (showContextMenu); submenu role=menu at :405. Window menu popover :1417, :1469.
- Dialogs: menus-and-routing.js:715 promptDialog, :719 confirmDialog, :752 modalDialog, :764 .vd-modal role=dialog. File dialog: R\ui\js\desktop\core\file-dialog-runtime.js:211.
- Toasts: R\ui\js\desktop\core\sdk-events-bootstrap.js:511 (showDesktopToast); R\ui\js\desktop\core\media-helpers.js:92 (createToast); CSS R\ui\css\desktop-base.css:268, 292, 302-305. Two toast implementations.
- Tooltips: UNVERIFIED (no vd-tooltip class found).
- Tabs: R\ui\js\desktop\core\widget-fritzbox-runtime.js:122 (role=tablist). No shared tabs component.
- Colour pickers: native input type=color in several apps; not a shared helper (UNVERIFIED).
- Icons: iconMarkup(key, fallback, className, size, usage) at R\ui\js\desktop\core\desktop-foundation.js:509. Packs in R\internal\desktop\types.go:276 (papirus, whitesur, windows_xp, tela, candy, fluent, buuf_nestort). SVGs under R\ui\img\<pack>\icons\.
- Empty state: .vd-empty at R\ui\css\desktop-base.css:2305.
- Skeleton loader: none found in desktop-base (UNVERIFIED elsewhere).

4. DATA VISUALISATION
- Chart.js 4.5.1 at R\ui\chart.min.js (header line 2). Loaded by the main Dashboard (R\ui\dashboard.html:40). Used by R\ui\js\dashboard\dashboard-charts.js (gauge :1, doughnut :44, radar :162, line :229, bar :296).
- three.js at R\ui\js\vendor\three.min.js, plus GLTFLoader, STLLoader, DRACOLoader, OrbitControls in the same folder. Version UNVERIFIED (minified; only a REVISION symbol).
- force-graph 1.52.0 at R\ui\js\vendor\force-graph.min.js:1; also 3d-force-graph.min.js.
- Custom SVG/canvas gauges, sparklines and dash-array rings are hand-written in: R\ui\js\desktop\apps\docker-manager-charts.js, R\ui\js\desktop\core\widget-fritzbox-charts.js, R\ui\js\desktop\apps\system-info.js, R\ui\js\desktop\core\widget-sysmon-runtime.js, R\ui\js\desktop\apps\sysworld-hud.js, R\ui\js\desktop\apps\looper-monitor.js. No shared gauge or sparkline helper.
- No uPlot, ApexCharts, ECharts or D3 in R\ui\js\vendor.

5. DRAG AND DROP / DESIGNERS
- No vendored grid or DnD library (no gridstack, muuri, sortable, interact).
- Window drag/resize (pointer-based): R\ui\js\desktop\core\window-interactions-runtime.js:380-419 (titlebar), :684-719 (resize handles), :753-759 (bounds), :789 (aspect lock). Minimum window 360x280 (R\ui\js\desktop\core\desktop-foundation.js:50-51).
- Desktop icons: pointer handlers desktop-foundation.js:878, :1397; HTML5 dragstart at :1629. Snapping and undo UNVERIFIED.
- Infra canvas: pointer gestures in R\ui\js\desktop\apps\infra-gestures.js (about 19 hits); minimap pointer capture R\ui\js\desktop\apps\infra-canvas.js:726-732; keyboard tools :785. R\ui\js\desktop\apps\infra-store.js has many history-like hits; undo implementation UNVERIFIED.
- Video Studio timeline (best snapping model): R\ui\js\desktop\apps\video-studio-timeline.js. snapFrames :453 (8px threshold), dragClip :382, snapDelta :504. Keyboard and undo UNVERIFIED.
- EasyDrag: R\ui\js\desktop\apps\easydrag-core.js has about 11 pointer/drag-like hits (not verified in depth). R\ui\js\desktop\apps\easydrag-home-grid.js is a card grid with no DnD.
- Homepage Studio: no DnD found (UNVERIFIED).
- Docker Manager: R\ui\js\desktop\apps\docker-manager-editor.js has about 2 hits (UNVERIFIED).
- Main Dashboard: no drag or reorder in R\ui\js\dashboard\. Layout is not user-arrangeable (grep-based).
- Desktop widgets: widget manager dialog at menus-and-routing.js:1148, :1246; drawer R\ui\js\desktop\core\widget-drawer-runtime.js. Placement and persistence UNVERIFIED.

6. MAIN DASHBOARD
- Standalone page R\ui\dashboard.html, styled by R\ui\css\dashboard.css and R\ui\css\dashboard-shell.css. Loads shared-variables.css, shared-utilities.css, shared-components.css and css/tokens.css (dashboard.html:22-33). Its tokens are a separate system from --vd-*/--ds-*. Mapping UNVERIFIED.
- Cards: .dash-card (R\ui\css\dashboard.css:91, :106); grid repeat(auto-fill, minmax(260px,1fr)) near :2398. Uses pw-* precision-workspace classes.
- Layout is fixed and not persisted by user (grep-based).

7. MOTION AND ACCESSIBILITY
- Reduced motion: R\ui\css\desktop-base.css:319, :448, :1908. Theme FX listener R\ui\js\desktop\core\theme-runtime.js:70-76. Animation off switch: .desktop-body[data-animations="false"] (desktop-base.css:302-305).
- Per-theme motion durations: theme-runtime.js:6-19 (Standard open 240ms, Orbit 620ms, Windows XP 120ms).
- Focus-visible: desktop-base.css:268, :292; app example desktop-app-notes.css:8 (2px outline, 2px offset).
- ARIA: role=dialog aria-modal on modals (menus-and-routing.js:764, :1148); role=menu (:405, :1417); aria-pressed in notes; role=switch (ha-switchboard.js:125).
- Hit size: no global minimum found (UNVERIFIED). Notes buttons use min-height 30px (desktop-app-notes.css:6).
- Contrast rule: R\ui\desktop_theme_token_contrast_browser_test.go:33-37 enforces WCAG 4.5:1 on status text over the three shell surfaces and on accent text. Also R\ui\desktop_fruity_contrast_test.go. Organic materials cite 4.5:1 (desktop-materials-organic.css:5-6).
- Status colours: success #4ad29c, warning #f2b84b, danger #ff7a72, info #5aa9ff (desktop-base.css:103-106).

8. ASSETS
- No-external rule enforced: R\ui\external_assets_test.go:12-22 (forbids Google Fonts, gstatic, GA/GTM, perplexity fonts; remote script src, CSS @import/url(), link loads).
- Vendor folder: R\ui\js\vendor\ (about 1,267 files). Chart.js at R\ui\chart.min.js. Libraries: three.js and loaders, force-graph, 3d-force-graph, xterm (fit, webgl), pdf.js, mermaid, hls, marked, purify, highlight, novnc, rive, qrcode, Tone MIDI, Stockfish lite.
- Fonts: R\ui\fonts\fonts.css with bundled files: Geist (Light, Regular, Medium, SemiBold, Bold), Geist Mono, JetBrains Mono, Inter, Montserrat, Barlow Condensed, Roboto Condensed, Share Tech Mono, Special Elite, Press Start 2P, Schoolbell, Px437, C64 Pro Mono, Datatype, Darker Grotesque.
- WebGL: three.js local (version UNVERIFIED). Canvas helpers in R\ui\js\vendor\canvasui\.

9. RESPONSIVE AND FULLSCREEN
- Mobile detection: R\ui\js\desktop\core\desktop-foundation.js:761-762 (max-width 820px; tablet max-width 1024px, hover:none, pointer:coarse). Mobile workspace width 920px (desktop-base.css:31); mobile taskbar reserve with safe-area (:38).
- Window minimum 360x280 (desktop-foundation.js:50-51); window-layouts-runtime.js:28 checks constraints.
- ResizeObserver: about 19 hits in 6 files (widget-autosize-runtime.js, fruity-dock-scroll.js, window-shell-runtime.js, desktop-foundation.js). No @container in desktop core JS. CSS container queries UNVERIFIED.
- Fullscreen: whole desktop only. Setting desktop.open_fullscreen (settings-catalog-runtime.js:133); wiring R\ui\js\desktop\core\shell-chrome-runtime.js:348-390. No per-app fullscreen or kiosk mode found (UNVERIFIED for a full search).

10. VISUAL TESTING TOOLING
- Theme sweep: R\ui\desktop_theme_sweep_browser_test.go:23-32. Rod-driven; loads every registry theme at wide and narrow viewports; checks body attributes, layout, colour scheme, windows and the Settings picker. Run: AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopTheme -timeout 30m. Screenshots to reports/desktop-theme-sweep/. Slow.
- Other theme tests: R\ui\desktop_theme_token_contrast_browser_test.go, desktop_theme_living_materials_browser_test.go, desktop_theme_motion_recording_test.go, desktop_app_materials_test.go (static), desktop_theme_css_contract_test.go. Registry test referenced at theme-runtime.js:3 (file not opened).
- Dashboard browser tests: R\ui\dashboard_overview_browser_test.go, dashboard_big_cards_test.go, dashboard_knowledge_visual_browser_test.go.
- Existing smart-home app: R\ui\js\desktop\apps\ha-switchboard.js and R\ui\css\ha-switchboard.css. Its backend integration is UNVERIFIED. HomeDash will overlap with it.

REUSE DIRECTLY
1. Theme registry and tokens: --ds-* and --vd-* (desktop-base.css:3-205) and the DESKTOP_THEMES capability model (theme-runtime.js). Keep HomeDash on the same 14 IDs.
2. --vd-mat-* materials with var(--vd-mat-x, fallback), plus the static materials test.
3. showContextMenu, modalDialog, confirmDialog, promptDialog (menus-and-routing.js:387, 715-752).
4. iconMarkup and icon packs (desktop-foundation.js:509).
5. .vd-empty (desktop-base.css:2305), toast CSS (:268-305), focus-visible patterns.
6. Chart.js 4.5.1 (R\ui\chart.min.js), three.js (R\ui\js\vendor\), Geist fonts (R\ui\fonts\).
7. Pointer window drag/resize code, and the Video Studio snapping model (video-studio-timeline.js:382-504) as reference.
8. Test harness pattern from desktop_theme_sweep_browser_test.go.

MISSING: HOMEDASH MUST BUILD ITSELF
1. Designer grid: no vendored grid, DnD, snapping or collision library. Dashboard layout is not user-arrangeable. No reusable undo/redo.
2. Control widgets: no shared switch, slider, segmented control, stepper or dimmer (ha-switch is scoped to its app).
3. Gauges, sparklines, rings: no shared component; each app hand-writes SVG or canvas.
4. Home widget/card contract: no shared card tokens or widget contract; dashboard cards use a separate token system.
5. Per-app fullscreen/kiosk: not present.
6. Tooltips, tabs, skeleton loaders, colour pickers: no shared component found (UNVERIFIED).
7. Camera tiles: no shared component found in this scan (UNVERIFIED).
8. Theme mapping for the Dashboard: it uses its own tokens, so HomeDash needs a mapping or must adopt --vd-mat-*.

UNVERIFIED: three.js version; Infra undo; Video Studio undo and keyboard; EasyDrag canvas interaction; Homepage Studio and Docker Manager DnD; desktop widget placement persistence; CSS @container use; global hit-size minimum; per-app fullscreen; shared tabs, tooltips, skeleton and colour-picker helpers; the light data-theme selectors at desktop-base.css:554 and :611 (not in the registry, likely legacy).
