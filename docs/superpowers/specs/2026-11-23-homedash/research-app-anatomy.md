HomeDash desktop-app contract report (read-only; nothing modified). Items marked UNVERIFIED were not confirmed in this pass.

Key finding: an app is wired in about 6 places. Four are core files compiled into main.bundle.js (so `npm run build:ui` is required), plus a lazy-load entry, a Go catalog row, and tests. Docker Manager is the cleanest template because a Go test pins its whole wiring.

## 1. Registration

Go catalog (source of truth for the shell's app list):
- `internal/desktop/types.go:157-180` AppManifest struct (id, name, version, icon, entry, category, permissions, requires, metadata, builtin, deletable, internal, dock_visible, start_visible).
- `internal/desktop/types.go:802-887` BuiltinApps(). Each row is `{ID, Name, Version, Icon, Entry: "builtin://<id>", Runtime: BuiltinRuntime, Category, Description, Permissions?, Requires?, Metadata?}`. After the list, every app gets Builtin=true, Deletable=false, DockVisible/StartVisible=true (lines 876-884); `viewer` and `openscad` are hidden.
- Example rows:
  - Docker Manager, types.go:871: `{ID: "docker-manager", ..., Icon: "docker-manager", Category: "system", Requires: []string{"docker"}}`
  - ECalc, types.go:858: `{ID: "ecalc", ..., Category: "dev", Metadata: {"logo_path": "/img/desktop-icons/ecalc.svg"}}`
  - Quick Connect, types.go:828: Category "dev", icon "terminal".
  - Notes, types.go:832: Category "office", Permissions files:read/write, notifications.
  - Code Studio, types.go:830.
- Start-menu categories: `DesktopAppCategories()` at types.go:788-800 (office, media, creative, ai, dev, system, comms, games, installed). The JS mirror `START_MENU_CATEGORIES` at `ui/js/desktop/core/window-shell-runtime.js:792-801` must stay in sync (test name TestDesktopStartMenuCategoriesStayInSync, per the Go comment at types.go:786-787).

Frontend registries:
- `ui/js/desktop/core/module-loader.js:8-14` bundlePaths (main, file-manager, code-studio, desktop-sounds, desktop-theme-fx).
- `module-loader.js:15-20` commonAppStyles = desktop-app-common.css + desktop-gog.css; `appStyles(...)` prepends them.
- `module-loader.js:21` DESKTOP_APP_ASSETS: key = app id, value `{styles, scripts, modules?}`.
  - ecalc:29-31 (about 30 ordered scripts, ecalc.js last)
  - notes:294-309
  - quick-connect:636-643
  - docker-manager:172 (xterm + 17 modules)
- `module-loader.js:846-883` APP_I18N_SECTIONS: app id → i18n section prefixes. docker-manager:853, notes/ecalc absent.
- `ui/js/desktop/core/desktop-foundation.js:624-684` appGlobalName(): app id → `window.<Global>` (docker-manager:635, ecalc:643). Quick Connect and Notes are absent from this map; Notes is special-cased in disposeAppWindow (see section 2).

Window sizing and reuse (`ui/js/desktop/core/window-shell-runtime.js`):
- appWindowSize presets, 1327-1388 (docker-manager 1347, notes 1352, ecalc 1354). Fallback is defaultWindowSize() (desktop-foundation.js:924), which uses the desktop size and the windows.default_size preset.
- appWindowMinSize, 1393-1407 (docker-manager 1396 = 440x400).
- Singleton behaviour is the default. openApp reuses a matching window (line 1639) unless `context.forceNew`. matchesExistingAppWindow (1491-1499) matches by appId only. Exceptions: quick-connect needs device_id; editor/writer/sheets/notes/synth-studio/layerling match by path (1494-1496); virtual-computers bypasses reuse (1514).

Render dispatch (`ui/js/desktop/core/menus-and-routing.js`):
- Docker Manager, 1697-1701: `if (appId === 'docker-manager' && window.DockerManagerApp) return window.DockerManagerApp.render(contentEl(id), id, Object.assign({}, context||{}, {esc, t, api, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary, readonly: desktopReadonly()}));`
- ECalc, 1794-1800: `if (!window.ECalcApp) {...}` then `ECalcApp.render(contentEl(id), id, {esc, api, t, ...})`.
- Notes, 1773.

Build: core files are compiled into the main bundle. `scripts/build-ui-bundles.js` lists desktopMainParts (lines 10-89, which include window-shell-runtime.js:38, desktop-foundation.js:24, menus-and-routing.js:83) and jsBundles (130-260). `main.bundle.js` is generated, and `--check` mode exists (line 8). `ui/js/desktop/bundles/main.bundle.js` is a committed artifact. Docker Manager's own modules are NOT in desktopMainParts; they load lazily via DESKTOP_APP_ASSETS.

## 2. Module contract

- Each app is an IIFE that exposes a global: `window.DockerManagerApp = { render, dispose };` (`ui/js/desktop/apps/docker-manager.js:1086`).
- `render(host, id, ctx)` (docker-manager.js:30). `host` is the window content element (contentEl(id)). `id` is the window id. `ctx` carries esc, t, api, menu helpers and `readonly`.
- `dispose(id)` (docker-manager.js:14) calls the stored disposer for that window id. render() calls dispose(id) first (line 31), so re-render is idempotent.
- Close path: `desktop-foundation.js:1131-1152` disposeAppWindow(). It runs `state.windowCleanups` callbacks, then the per-app special cases (notes:1147, radio, teevee, etc.), then `window[appGlobalName(appId)].dispose(win.id)`, then the fallback global.
- Cleanup pattern in Docker Manager: a `lifetime = new AbortController()` (line 47) passed as `{ signal }` to every addEventListener (e.g. 802, 900); a `disposed` flag checked in async continuations (e.g. 141, 176); timers and rAF handles cleared in dispose (lines 97-125, 411-414). ECalc: cancel its ResizeObserver frame on disposal (apps/AGENTS.md ECalc section).
- Multi-file split: Docker Manager has 17 files in dependency order (module-loader.js:172; wiring test lines 24-28), each an IIFE assigning `window.DockerManagerModel`, `...Store`, etc. Composition root is docker-manager.js. Notes uses the same pattern: notes-frontmatter, notes-index, notes-templates, writer-session, notes-editor, notes-panels, notes-graph, notes (module-loader.js:296-308). Chess: UNVERIFIED (chess-*.js internals not read).
- Security/style rules: untrusted text via textContent only; Storymaker and Chibi tests grep for innerHTML/eval; modules under 1100 lines (apps/AGENTS.md:86 and :280, and :488 for Docker Manager).
- Quick Connect: its render path is not in appGlobalName, so it is special-cased in routing; exact location UNVERIFIED.

## 3. CSS

- Naming: `ui/css/desktop-app-<id>.css` (e.g. desktop-app-docker-manager.css, desktop-app-ecalc.css, desktop-app-notes.css). Quick Connect uses desktop-app-quick-connect.css.
- Loading: listed in module-loader.js DESKTOP_APP_ASSETS `styles:`, wrapped with appStyles() so common CSS loads first. Failed optional CSS only warns (module-loader.js:813-817).
- Rules (desktop-app-docker-manager.css:1-4): tokens only (`--vd-*`, `--ds-*`), no hex or literal colour functions outside color-mix(), no theme ids, no uppercase labels. Transitions and animations only inside `@media (prefers-reduced-motion: no-preference)` on selectors starting with `body:not([data-animations="false"])`. Enforced by scripts/test-docker-manager-ui.mjs.
- Class prefix: a short app prefix (`dm-`, `dm-app`, `dm-btn`). The `.vd-*` prefix is the shell's.
- Size limits: no CSS size limit found in this pass (UNVERIFIED). Ui theme contract tests exist: ui/desktop_theme_css_contract_test.go, ui/desktop_theme_consistency_test.go (contents not read).

## 4. i18n

- Files: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`. Sixteen locales, flat dotted keys with the app prefix (`docker_manager.*`, about 545 occurrences in en.json; ecalc/notes prefixes UNVERIFIED).
- Helper: app code calls `ctx.t('docker_manager.<key>', vars)` (docker-manager.js:38-43). Missing keys show the key and warn once.
- Lazy section loading: `GET /api/i18n?lang=..&sections=..` from module-loader.js:887-908 (APP_I18N_SECTIONS). The server handler is UNVERIFIED.
- Lint/enforcement:
  - ui/i18n_lint_test.go:669 TestTranslations_AllKeysPresentInAllLanguages
  - :446 TestFrontend_StaticI18nKeysExistInEnglishBundle
  - :142 TestFrontend_JS_NoLiteralTranslationFallbacks
  - :1195 TestDesktopDynamicI18nPrefixKeysExistInAllLanguages
  - :1276 TestTranslations_SectionFilesOnlyOwnPrefixes
  - :1098-1110 maps each docker-manager file to its `t` prefix.
- Rule: never fill a locale with English; German uses Du and real umlauts (root AGENTS.md:166).

## 5. Icons

- The AppManifest.Icon value is a name from the icon allowlist `desktopPreferredIconNames` (types.go:330 onward; "docker-manager" at :371). Add the name there.
- Artwork: SVG files under `ui/img/desktop-icons/<id>.svg`, referenced via Metadata `logo_path` (types.go:858-859 and others).
- Theme manifests: `ui/img/<theme>/manifest.json` (papirus, whitesur, fluent, candy, tela, windows-xp, buuf-nestort) and `assets/desktop-icon-themes/*.json` (source). Docker Manager appears in these manifests. ui/AGENTS.md:633 says Papirus/WhiteSur manifests register app icons.
- Checks: scripts/test-desktop-icon-resolver.mjs checks every literal `core.icon` (apps/AGENTS.md:22). Icon sprite: UNVERIFIED (no sprite file was found; SVG files plus manifests are the observed mechanism).

## 6. Backend

- Routes: `internal/server/server_routes.go` 528-588 registers `/api/desktop/*` (bootstrap at 528, apps at 574, settings at 579, widgets at 576, notes at 533, home-assistant at 582-584).
- Docker Manager prefix: `internal/server/server_routes_tools.go:150` `mux.HandleFunc("/api/docker-manager/", handleDockerManager(s))`.
- Handler: `internal/server/docker_manager_api.go`:
  - :14-18 auth: the generic /api/ middleware (cookie session or bearer token with admin scope).
  - :194-214 gating: `containerDockerConfig(s)` returns enabled and readOnly. If Docker is disabled, every route answers 503 `docker_disabled`. ReadOnly blocks mutations.
  - Envelope: `{"status":"ok", ...}` or `{"status":"error","code","message"}`.
- Other gates: chibi_sensei_service.go:223 and :319 check `cfg.ChibiSensei.Enabled && cfg.VirtualDesktop.Enabled`.
- Persistence:
  - Notes is file-based: `/api/desktop/notes` → `internal/server/desktop_notes_handlers.go:38`, stored under Documents/Notes.
  - Docker Manager persists UI prefs client-side (localStorage key `docker-manager.ui.v1`, docker-manager.js:12, 28). Its host/stack data lives in `internal/dockerhost` and `internal/dockerstacks` (per AGENTS.md child index).
  - `/api/desktop/settings` → `internal/server/desktop_handlers_settings.go:11`. Backend storage: UNVERIFIED.
  - `/api/desktop/storage/` (desktop_storage.go:127) is a location-to-location file transfer session, not a per-app KV store.
  - A generic per-app key/value state API: NOT FOUND in this pass (UNVERIFIED that none exists).
- Requires gating: AppManifest.Requires is set in the catalog (e.g. docker, storymaker, chibi_sensei). The code that hides an app when its requirement is off is NOT FOUND in internal/desktop or the server handler (UNVERIFIED).

## 7. Asset pipeline

- The full UI is NOT go:embed'ed in the main binary. Root AGENTS.md:50 and :22 say the UI is a verified, version-bound external resource set packaged by `cmd/assetpack` (`-stage assets/web`), with integrity in internal/webassets. The `ui/` package has embed.go fixtures.
- No hand-maintained file list was found for desktop JS/CSS in internal/webassets (grep found nothing). New files are picked up by the module-loader lists plus the asset packaging. Full packaging behaviour: UNVERIFIED.
- Must-rebuild artifacts: `npm run build:ui` regenerates main.bundle.js after core edits (per the message in desktop_docker_manager_wiring_test.go:52). Security Gates checks bundle drift (root AGENTS.md:70).
- Service worker cache list: none found in this pass (UNVERIFIED).

## 8. Tests

- Go wiring test pattern: `ui/desktop_docker_manager_wiring_test.go` TestDockerManagerWiring (lines 12-56). It regex-matches the module-loader entry and checks script order, xterm-first, no webgl, the i18n section, and markers in menus-and-routing.js, desktop-foundation.js, window-shell-runtime.js and main.bundle.js. Copy this for HomeDash.
- Other registration tests: ui/desktop_chibi_sensei_registration_test.go, ui/desktop_ecalc_*_test.go, ui/desktop_runenpfad_test.go (TestRunenpfadRegistration).
- Browser tests (rod): `ui/desktop_docker_manager_browser_test.go` (fixture `dockerManagerFixture`, `auditTest.openApp('docker-manager')` at :448). Name pattern TestDesktopDockerManagerBrowser, gated by `AURAGO_RUN_BROWSER_SMOKE=1`. Other examples: desktop_ecalc_browser_test.go, desktop_notes_browser_test.go, desktop_chess_app_test.go.
- Shards: root AGENTS.md:70 says Desktop browser tests are split into name ranges (A-D, E-G, H-R, S-Z) and `TestUIBrowserCIShardsCoverEveryTest` checks complete, non-overlapping coverage. A new Desktop browser test name must land in a shard range.
- Node tests: `ui/js/desktop/apps/*.test.cjs` (notes-*.test.cjs) and `scripts/test-*.mjs`. Docker Manager: `npm run test:docker-manager` → scripts/test-docker-manager-ui.mjs (vm harness). Also `scripts/test-ui-regressions.mjs`, `scripts/build-ui-bundles.js --check`, `node --check`.
- Size guard: TestStorymakerSourcesAreSafeAndSmall / Chibi equivalents (apps/AGENTS.md:86, :115) enforce a 1100-line budget per module. Docker Manager has a similar check in scripts/test-docker-manager-ui.mjs.
- Expected for a new app: the Go wiring test, a Node UI test, a browser smoke test, the i18n tests above, and `TestDesktop*` theme/CSS contract tests (UNVERIFIED which apply to every app).

## 9. Rules (AGENTS.md)

- Root AGENTS.md (C:\Users\Andi\Documents\repo\AuraGo\AGENTS.md):
  - Every meaningful change needs a DOX pass (:9). Child guides own domain details.
  - Translate into all 16 locales; never fill with English. German: Du, real umlauts (:166).
  - "modals, never `alert()`" and pending/success/failure/disabled feedback (:164).
  - Use dropdowns for defined choices (:164).
  - Themes: fruity = WhiteSur/dock; standard = Papirus/structured start menu (:165).
  - Build/verify: `node --check <file.js>` (:66); `git diff --check` (:66); LF normalization, BOM-free writes (:65-66).
  - Browser shard coverage (:70).
- ui/AGENTS.md: external packaged UI, 16 locales, verification notes (:1-5). Desktop apps no external assets: "No external service, backend route or capability switch is required" and "ships no external resources" appear in its app sections.
- ui/js/desktop/apps/AGENTS.md (about 590 KB; read by heading and key-rule grep only):
  - Modules under 1100 lines and no DOM touch at load (:280, :488).
  - Untrusted text via textContent only (:86, :115).
  - Browser test per app (e.g. `AURAGO_RUN_BROWSER_SMOKE=1`).
  - Touch targets 44 px, focus-visible (:265 Infra; docker css:9).
  - Reduced motion gating via `app.animationsEnabled()` (:419).
  - Desktop app locale: `lang/<section>/` for 16 locales (:116, :122).
- Auto-memory (informational): always commit after every code change; scratch work in a disposable folder, not the repo.

## 10. Existing widget / dashboard collisions

- Desktop widgets: `Widget` struct in types.go:207-232 with `/api/desktop/widgets` (server_routes.go:576, handleDesktopWidgets at desktop_handlers_widgets.go:12). Client runtimes in `ui/js/desktop/core/`: widget-drawer-runtime, widget-sysmon-runtime, widget-meshcore-runtime, widget-printer-runtime, widget-fritzbox-runtime, widget-leafy-runtime, sticky-notes-runtime (desktop bundle part list, build-ui-bundles.js:63-71). Standalone widget files live as `Widgets/<name>.html` (window-shell-runtime.js:1526-1529, findExistingStandaloneWidgetWindow at 1559).
- Smart-home overlap: `ha-switchboard` app (types.go; handler `/api/desktop/home-assistant/entities|states|switch`, server_routes.go:582-584, and admin-only routes per root AGENTS.md:170). HomeDash should reuse these endpoints rather than add a second HA path.
- Other overlap: the Infra map (apps/AGENTS.md:250) and Fritz!Box widget (root AGENTS.md:252).
- "Dashboard" tile concepts also exist in WhatsApp and Dashboard features (root AGENTS.md:305, ui/AGENTS.md:433). Naming: use the id `homedash` and avoid widget ids `widget-*`.

## Checklist: files to add / edit for a new app (id `homedash`)

Add:
1. `ui/js/desktop/apps/homedash.js` (composition root, `window.HomeDashApp = { render, dispose }`)
2. Optional split modules `ui/js/desktop/apps/homedash-*.js` (keep each under 1100 lines)
3. `ui/css/desktop-app-homedash.css` (tokens only, reduced-motion gating)
4. `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`: add `homedash.*` keys to all 16
5. Icon artwork `ui/img/desktop-icons/homedash.svg` (+ `logo_path` metadata), and theme manifest entries (`ui/img/*/manifest.json`, `assets/desktop-icon-themes/*.json`) if themed
6. Go: `internal/server/homedash_api.go` (if backend endpoints) and routes in `internal/server/server_routes.go` (near 528-588) or `server_routes_tools.go`, plus gating via config
7. Go tests: `ui/desktop_homedash_wiring_test.go` (copy of TestDockerManagerWiring), `ui/desktop_homedash_browser_test.go` (name in a shard range), `scripts/test-homedash-ui.mjs` (+ package.json script)
8. Docs: `documentation/homedash.md`; update `ui/js/desktop/apps/AGENTS.md` (child section) and root AGENTS.md child index if a new child guide is created

Edit:
1. `internal/desktop/types.go`: BuiltinApps() row (near 871); `desktopPreferredIconNames` (near 330-371); optionally `Requires`
2. `ui/js/desktop/core/module-loader.js`: DESKTOP_APP_ASSETS entry (near 172 or 636); APP_I18N_SECTIONS entry (near 853, if you use a section)
3. `ui/js/desktop/core/desktop-foundation.js`: appGlobalName map (near 635)
4. `ui/js/desktop/core/menus-and-routing.js`: render dispatch (near 1697-1701)
5. `ui/js/desktop/core/window-shell-runtime.js`: appWindowSize preset (near 1347), appWindowMinSize (near 1396), and START_MENU_CATEGORIES only if adding a category (must match types.go:788)
6. Regenerate `ui/js/desktop/bundles/main.bundle.js` via `npm run build:ui` (or `node scripts/build-ui-bundles.js`) and check with `--check`
7. `package.json`: test:homedash script
8. Desktop config/app-visibility toggles if any: UNVERIFIED

UNVERIFIED list (not confirmed): Requires enforcement point; generic per-app KV storage; desktop settings backend; /api/i18n server handler; CSS size lint; icon sprite; service-worker cache list; chess internals; Quick Connect routing location; asset manifest generation in cmd/assetpack.
