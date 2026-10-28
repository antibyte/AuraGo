# Web UI - Child DOX Contract

## Purpose

This subtree owns AuraGo's externally packaged HTML, CSS, JavaScript, translations, fonts,
images, and browser-oriented regression tests.

Production uses the verified resource set from `internal/webassets`, built with
`cmd/assetpack` and `assets/web-assets.json`. `ui.Content` exposes source files to
tests only; this tree must never be embedded into the server. BuildVersion is
the asset digest. Unversioned subresources remain network-only in the service
worker. Keep packaging, recovery and offline instructions in
`documentation/web-assets.md` synchronized.

Shipped pages load fonts, images, styles and libraries only from AuraGo itself.
Vendoring scripts must rewrite upstream CDN, Google Fonts and analytics references;
`TestShippedWebAssetsLoadNothingExternal` guards `ui/` and the bundled desktop apps.
Never use the cookieconsent `cc-` prefix for classes or IDs: cookie-banner blockers hide
`.cc-banner`, `.cc-modal-backdrop` and similar on every site (CYD Creator uses `cydc-`,
the config device tables/dialogs `cfgdev-`); `TestUIAvoidsCookieConsentClassPrefix` guards it.

## Ownership

- Document Creator uses a native boolean checkbox for the default-off saved
  public-resource grant. Its help names images, fonts, stylesheets and scripts;
  all 16 locales carry the same policy. Native checkbox drafts/discard/save use
  boolean `checked`, never the string `value`. The connection test refuses dirty
  drafts, uses saved settings and distinguishes unreachable, unverified,
  verified and restart-required states. Ignore detached results and announce
  status accessibly. Verify `TestDocumentCreatorPolicyAndDiagnosticsBrowser`,
  `TestDocumentCreatorLocales*` and config rerender/draft browser tests; runtime
  contract: `../documentation/document-creator.md`.

- `cfg/s3.js` owns external S3 settings and optional managed RustFS. Preserve external fields on provider changes; bucket/prefix rows replace one atomic list in the shared draft. Lifecycle and probes use saved settings; polling is fenced by view and saved-generation identity and ends on navigation. Docker read-only disables lifecycle actions, not observations. Dashboard distinguishes configured from observed runtime readiness and links to `/config#s3`. Verify `TestRustFSS3SettingsBrowser`, `TestRustFSS3LocalesAreComplete` and `TestRustFSDashboardSeparatesConfiguredAndObservedState`; runtime contract: `../internal/rustfs/AGENTS.md`.

- Provider editors own the revision and draft they opened. Save with that revision, disable duplicate saves, retain drafts on conflicts and require explicit reload. Background refresh must not lend a newer revision to an old draft. Modal close cancels/ignores its requests and picker results; model selection never mutates another settings section's budget draft.
- Credentials have explicit removal actions; masks only retain stored values. Allow incomplete drafts and valid custom/local HTTP(S) hosts. Diagnostics are passive; metadata/generation tests use saved settings, with a separate cost confirmation for generation and no automatic generation fallback. Keep all changed copy translated across 16 locales. Verify `TestProviderRevisionAndSavedTestsBrowser`, `TestProviderSafetyLocalesComplete` and provider usage round-trip coverage.
- Explicit credential removal survives a change of authentication type in the same modal. Verify `TestProviderClearKeySurvivesAuthTypeChangeBrowser` in addition to same-authentication removal.

- Ollama config displays a derived read-only URL in managed mode. Connection tests and Recreate require saved settings; Recreate confirms interruption and polls its receipt through slow image/model downloads. Docker read-only disables lifecycle mutations, not observation. Polls end on terminal state/navigation and ignore detached or stale responses. Embeddings show managed Ollama model state. All operation labels and help remain translated in all 16 locales. Verify `TestOllamaManagedLifecycleBrowser` and `TestOllamaOperationLocalesAreComplete`.

- Action-policy blocks are terminal outcomes, not queued/running work. Dashboard
  background tasks render `blocked` with `last_error: action_rule_required` as a
  translated missing-rule diagnosis linking to `/config#danger`; active counters
  continue to count only queued, waiting and running tasks. Missions show the same
  diagnosis in both card views and Desktop Mission Control, including history.
  EasyDrag localizes `FLOW_ACTION_RULE_REQUIRED` through its existing error-code
  renderer. Status rendering must never retry or request approval automatically.
  Keep the four locale families aligned in all sixteen languages. Verify with
  `node scripts/test-action-policy-status-ui.mjs`; the fixture uses source modules,
  real styles and synthetic records without local config or providers.

- File Manager cloud references keep provider, Koofr mount and relative path
  separate through navigation, split panes, tabs, favorites, window context, clipboard and
  drag-and-drop. Remote payloads never enter local editor/file APIs. Preserve
  lazy mount loading, readonly previews, abortable transfers and the shared
  Replace / Keep copy / Cancel dialog. Config visibility defaults off in all
  locales. Policy updates preserve allowed listings; only changed storage grants
  cancel transfers. Drops use the destination pane's identity and permissions.
  Verify `TestDesktopCloudStorageBrowser`, `TestDesktopCloudStorageReviewBrowser` and existing File Manager
  instance/clipboard suites; see `documentation/desktop-cloud-storage.md`.

- Desktop file mutations use the shared conflict runtime: strong ETags for
  observed versions, create-only requests for new files, and explicit Replace /
  Keep copy / Cancel after a server conflict. Each editor/SDK client owns the
  version it read; a global path cache must not grant another window an overwrite.
  Adopt the server's returned path after a copy decision. Rebuild Desktop bundles
  and verify `TestDesktopFileConflictBrowser` for changes to this flow.
  Autor keeps its own DOCX conflict actions, documented in `documentation/autor.md`:
  Save As and reopen, with create-only rename.

- File Manager paste, archive, extract and batch rename retain their originating
  instance and captured paths across dialogs and requests. Late results cannot
  repaint another window or replace its undo state. Shared cut completion clears
  only the clipboard object it started with; a newer clipboard survives. Verify
  `TestFileManagerAsyncActionsStayWithOriginBrowser` and
  `TestDesktopFileClipboardPastePreservesNewSameContentBrowser`.
- Generated-app keyboard relays use the existing document-bound SDK MessagePort.
  The injected receiver supports SDK-less apps and preserves normal keyboard
  events; navigation or frame disposal revokes forwarding. Verify
  `TestDesktopSDKKeyboardBridgeUsesLivePortForSDKAndLegacyApps`.
- Desktop icon keys (Enter, Delete, F2) act on the selected icon only from the
  desktop: a focused icon, or the page body after the last pointer press landed
  on the desktop surface (`trackDesktopKeyboardScope`). Keys from windows,
  menus, dialogs, widgets or the taskbar, and keys another handler already
  consumed (`defaultPrevented`), never reach the icon; apps need no
  `stopPropagation` workaround. Global shell shortcuts (Ctrl+K, F1, F11,
  Alt+F4, switcher, snap) keep working inside windows. Verify
  `TestDesktopIconShortcutsStayOnDesktopBrowser`.

- Precision Workspace is an opt-in design system. Operational consumers are
  `config.html`, `dashboard.html`, `plans.html`, `missions_v2.html`,
  `cheatsheets.html`, `knowledge.html`, `skills.html`, `containers.html`,
  `media.html`, `truenas.html`, and `invasion_control.html`.
- Entry consumers are `login.html`, `setup.html`, and `404.html`. They use the
  navigation-free `.pw-entry-page` layer without density controls or the
  operational client.
- Web Chat (`index.html`) and Virtual Desktop (`desktop.html`) retain their own
  established visual systems. `gallery.html` is also protected because the
  `/gallery` route redirects to `/media`.
- Chat HTML sanitizer (`js/shared/chat-core.js`) may keep `iframe` only with a
  forced `sandbox="allow-scripts"`. Never `allow-same-origin`. Protocol-relative
  `//` URLs are rejected. Rebuild `chat-runtime.bundle.js` after sanitizer edits.
- Chat code fences: with a `codeBlockFactory`, `js/shared/render-markdown.js`
  emits code, Mermaid and chart blocks verbatim; markdown-it must not wrap them
  in `<pre><code>`, where layout whitespace renders as empty bands.
  `CodeBlocks.createCodeBlock` returns whitespace-free markup without a button;
  the sanitizer drops `<button>` and inline handlers, so `CodeBlocks.enhance`
  attaches copy buttons after insertion. Line numbers match the code's font
  size. Verify `TestChatCodeBlocksRenderCompactBrowser` (all chat themes).
- Chat YouTube players are DOM-built in `js/chat/chat-messages.js`. They use
  `referrerpolicy="no-referrer"`, omit `clipboard-write` and `web-share`, and
  set the youtube-nocookie `src` only after a local play click. They do not use
  `sandbox="allow-same-origin"`. Start times over 48 hours are rejected. Verify
  `TestChatFrontend_VideoPlayerFlowRemainsPresent`.

## Local Contracts

- Shared QR editing is lazy key `qr`: `js/desktop/shared/qr-dialog.js` and
  `css/desktop-qr.css`. Writer, Sheets and Pixel use the same local Go renderer,
  scanner, configuration normalizer and CSV batch service. Payloads stay in POST
  bodies, never URLs; previews do not publish files. `desktop.qr_*` and QR agent
  Config/help keys live in all sixteen Desktop locale files.
- Calendar QR drafts capture one RFC3339 creation timestamp for stable DTSTAMP
  bytes across preview, styling and export; configuration import retains the
  normalized timestamp. vCards expose separate work/private/mobile numbers;
  local telephone numbers are retained with the core's country-context warning.
- A QR dialog belongs to its initiating window and captured document/selection.
  Preview, export and insertion use the same configuration revision; superseded
  responses cannot replace a preview or mutate another document. Close/dispose
  aborts requests and file readers, clears drafts/listeners/timers, revokes blob
  URLs and stops all camera tracks, including a permission request that resolves
  after close. Scanned content is text only and never executes.
- QR localStorage contains only compact/full-view and pinned-control preferences.
  Full configuration files include content, captions and embedded resources only
  after explicit save; `/api/qr/config` normalizes images before download. Raster
  and restricted SVG logo/background uploads are normalized by Go, never browser
  SVG execution; scanning accepts PNG/JPEG/GIF/WebP. Explicit guarded URL imports
  never create rendering-time network dependencies. Fonts use the fixed local
  catalog in `fonts/qr/AGENTS.md`.
- QR agent exposure (`tools.qr_code.enabled`, default off; `readonly`, default on)
  is separate from human editor access. Read-only agent access permits scanning
  and catalog reads; publication remains disabled. Existing editor restrictions
  still govern insertion. Verify `TestQRDialogWriterPixelBrowser`,
  `node scripts/test-pixel-qrcode.mjs`, `node scripts/test-qr-config.mjs`, the
  Pixel suite and Desktop i18n checks;
  actual camera hardware and printed-page scanning remain separate acceptance.
  Operator workflow and input/output limits are in
  [QR codes](../documentation/qr-code.md).
- QR CSV file imports decode UTF-8 strictly, accept an optional UTF-8 BOM and
  retain the prior draft on invalid encoding. Late file reads cannot replace a
  newer selection or edits. Insertion callbacks receive the operation's abort
  signal; closing or changing the dialog must cancel pending insertion work.

- Dashboard Tools owns the self-test card (`js/dashboard/self-tests.js`): explicit Start/Stop, read/fixture/exact-binding profiles, KPI summary before a closed details table and fixed same-origin Markdown/JSON downloads. Dynamic values use `textContent`; prevent duplicate starts, discard stale requests and stop polling while hidden. Keep `dashboard.selftest_*` aligned in all 16 locales. Verify `TestSelfTestDashboard*` and `node ui/testdata/selftests.cjs`; backend/evidence contracts belong to `internal/selftest/AGENTS.md`.

- Speech Output defaults to `phonon` with automatic language/voice on fresh setup.
  The Phonon picker uses `AuraConfigState`, retains manual voices across navigation
  and async catalog refreshes, and separates downloads from selection/saving.
  Catalog reads are passive; only explicit download buttons install a voice.
  Late responses cannot repaint another section. Keep all sixteen TTS locales
  and `TestPhononConfigDownloadAndDraftBrowser` aligned.

### Render and asynchronous ownership

- Dashboard prompt analytics show module selection, base formatting and request
  fitting separately. Base history uses `recent`, budget history `recent_fits`;
  never stack or sum these different populations. Label text sizes as UTF-8
  bytes, distinguish buffered windows from lifetime request/cache counters, and
  show missing metrics as unknown rather than zero. Positive percentages below
  0.1 display `< 0.1 %` in the active locale. Verify `TestDashboardPromptLocales`
  and `TestDashboardPromptStagesBrowser` in both dashboard themes and widths.

- Dashboard big cards (prompt, tooling telemetry, KG health and quality, memory)
  show a KPI row first (`renderDashKpiRow` in `js/dashboard/main.js`: page-locale
  numbers, an em dash for missing values, `data-action` buttons) and keep every
  detail block in `<details class="dash-disclosure" data-disclosure="<card-id>:<key>">`,
  closed in the markup. `js/dashboard/dash-disclosures.js` stores open blocks in
  localStorage `dash-disclosures`, re-opens them at boot and after each section
  load, resizes charts on open and fills summary counts (`setCount`). CardState
  error/restore replaces card bodies, so listeners must be delegated. A new block
  needs a key, a `dashboard.*` summary label in 16 locales and an entry in its
  `assertDashboardBigCard` test. Verify `TestDashboardDisclosureState`,
  `TestDashboardKPIHelpers`, the `*PutsKPIsFirst` tests and
  `TestDashboardDisclosuresBrowser`.

- Google Workspace exposes separate Gmail read, send and label-change grants. Revocation applies after saving; enabling additional OAuth scopes may require reconnecting. Label changes default off and readonly overrides them. Verify TestGmailLabelPermissionBrowser and all sixteen locale bundles.

- LDAP Config offers one transport dropdown (LDAPS, StartTLS, plain), preserves the legacy use_tls selection until edited, and retains custom ports when switching modes. All sixteen LDAP locale bundles describe the transport; verify TestLDAPTransportConfigBrowser.

- Escape text before every Markdown fallback. Generic image/audio/video sources use the shared same-origin media allowlist; external images become deliberate links. Preview iframes retain sandboxing. Logout sends same-origin POST.
- Config saves bind to the sent snapshot and retain edits made while requests are in flight. Chat HTTP/SSE/recovery responses bind to session and request generation. All close gestures await the window's asynchronous guard; file dialogs remain pending while a write is in flight.
- A Config switch handler that re-renders `#content` first sets the clicked `.toggle` to its new value (`on`, `aria-checked`), before `AuraConfigState.set`, `setDirty` or `markDirty`: their `syncFromDOM` and the bubbling document click listener still read that switch, even once detached, so a stale one reverts the draft. Deferring the re-render does not help. Verify `TestConfigRerenderingTogglesKeepDraftBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1`.
- Shared modals queue independent promises. Vault prompts have bounded expiry and cancellation, clear sensitive drafts on close, and never let late results close a newer prompt. Highlight original text nodes rather than escaped HTML.
- Verify `TestCodeQualityBrowserBoundaries` with `AURAGO_RUN_BROWSER_SMOKE=1`, existing chat/config/Tresor browser checks and rebuilt bundles.

- `js/shared/shared-core.js` renews an authenticated session through
  `/api/auth/activity` only for trusted pointer, keyboard, input or wheel events
  in a visible page, throttled to once per 30 seconds. Background polling, SSE,
  focus and visibility changes never count as activity. Expiry checks read the
  server status so another tab's renewal is respected; logout stops renewal and
  waits for an in-flight cookie update before clearing it. Keep the timeout help
  in all sixteen Config locales aligned. Verify `TestAuthSessionBrowser`.

- Quick Connect serial sessions use the live bootstrap capabilities, fail closed
  on the Desktop WebSocket disconnect, and dispose on `aurago:auth-ended` before
  logout or expiry redirects. `aurago:desktop-policy` carries serial grants,
  readonly and remote timeout values; device traffic never renews authentication.
  Embedded apps retain `serial 'none'`. Keep all sixteen serial/profile and
  configuration translations aligned. See `js/desktop/apps/AGENTS.md` and
  `documentation/quick-connect-serial.md` for lifecycle and hardware acceptance.
- Retro-Net in the Desktop Terminal follows the live `retronet_enabled` grant
  (bootstrap and `aurago:desktop-policy`; default on, off when readonly). The
  browser names only directory entry IDs, never a host or port. Keep all
  sixteen `desktop.terminal_retronet_*` and Config toggle translations aligned.
  See `js/desktop/apps/AGENTS.md` (Terminal), `internal/retronet/AGENTS.md`
  and `documentation/retro-net.md`.
- The Config CYD web flasher (`cfg/cyd.js`) needs a secure context,
  `navigator.serial` and a Permissions-Policy that allows `serial`
  (`document.permissionsPolicy || document.featurePolicy`). Otherwise it shows
  `config.cyd.flash_unsupported`, disables flashing and creates no token or ESP
  Web Tools installer. Verify `TestCYDFlasherDetectsPolicyBlockedWebSerialBrowser`.

- `cfg/llm_router.js` uses the shared saved/draft config path. Empty provider
  clears its model override; missing saved providers remain visible. Preview
  requires a clean draft and uses saved settings, with a separate explicit
  helper action and cancellation on disposal. `common.llm_router.*` covers all
  sixteen locales. Chat/Desktop `llm_route` badges use text-only, turn-scoped
  metadata. Their `AuraLLMRouteLabel`/`AuraLLMRouteBadge` helpers live in the
  production `js/shared/shared-core.js`; browser tests must use the page's
  loaded helpers without injecting them from legacy `shared.js`.
  Dashboard counters never imply unreported usage is free. Verify
  `TestLLMRouterConfigBrowser`, `TestTaskRouter*` and canonical UI bundles.
  Offer Agnes chat providers and their configured models, including models newer
  than the catalog; exclude Agnes image/video generation choices.
- Config personality/provider dropdowns (`personalities_ref`, `provider_ref`) never drop a saved value: an
  unknown value stays selected as "(missing)", and a failed list load keeps it as "(list unavailable)" with an
  inline retry that refreshes the options in place. Keys live in `lang/config/common/` for all sixteen
  locales. Verify `TestConfigChoiceSelectsKeepSavedValueWhenListsFailBrowser`.
- `PUT /api/config` replaces lists wholesale, so a Config section with per-index
  `data-path` controls (`list.<index>.<field>`) must never patch single indexes.
  It calls `AuraConfigState.markAtomic('<list>')`, adopts the draft list before
  rendering or changing it, and publishes additions and removals with
  `AuraConfigState.replace('<list>', list, render)`, then `setDirty(true)`.
  `replace` re-renders before it notifies: main.js subscribes `setDirty`, which
  reads every rendered control back, so a plain `set` would write the old list's
  controls over the new one. The patch then carries the complete list, including
  fields the cards do not render. Lists: `go2rtc.streams` (`cfg/go2rtc.js`, keeping
  `source_configured`, `detection`), `a2a.server.skills` and
  `a2a.client.remote_agents` (`cfg/a2a.js`, re-rendering only the list container;
  keeping skill `tags` and the masked agent credentials), both
  `three_d_printers.*.printers` (`cfg/three_d_printers.js`). Verify the Config
  list cases in `node scripts/test-ui-regressions.mjs`.

- go2rtc Config treats both absent media enums and legacy empty strings as their
  neutral defaults in the draft and saved comparison baseline. Opening a section
  or undoing edits must leave it clean; normalization must preserve actual draft
  changes and masked sources. Verify `TestGo2RTCLegacyMediaConfigStaysCleanBrowser`.
  Auto playback uses separate RTC-signaling and alternative-media sockets: the
  losing media socket closes on RTC selection, while signaling loss still stops
  the peer. Explicit disconnect clears pending reconnects. Verify
  `TestGo2RTCPlayerProtocolsBrowser` against the pinned fixtures.

- Newspaper is a lazy built-in Desktop reader with a separate editorial paper surface and Config/Dashboard entry. Keep all sixteen Desktop, Config and Dashboard locale keys aligned, both icon manifests registered, and the generated main Desktop bundle current. Config exposes the search cap (32 by default, 1–64 including retries); shared capability responses distinguish setup reasons from last-attempt failures. The reader preserves optional research counters and topic gaps, including legacy runs without them. A bounce-suppression message must not imply that the recipient address is invalid without the original SMTP evidence. Verify `TestNewspaperResearchTranslations`, `TestNewspaperConfigResearchBrowser` and `TestDesktopNewspaperBrowser` at wide/narrow sizes in both themes; backend contracts live in `internal/newspaper/AGENTS.md`.

- Tresor is a lazy built-in Desktop app with locally packaged Argon2id JS/WASM, a decorative vault-door SVG, and browser-only AES-GCM keys. Keep both theme icon manifests, `desktop.app_tresor` and `tresor.*` in all sixteen Desktop locales, and the generated main bundle current. Notes autosave with encrypted local drafts; Desktop close awaits saves and shows failures. Explicit, idle and pagehide locks clear decrypted state immediately, independently of network writes. Preserve editor/record association after failed loads and draft titles across dialogs. Verify vendor drift, crypto vectors, `TestTresorTranslations` and `TestDesktopTresorBrowser` (including failure recovery); the storage contract is `internal/tresor/AGENTS.md`.
- Chat history loads are session-generation bound through every rendered batch; a late fetch or animation-frame continuation cannot append to a newer session. Store dialogs clear input values and attributes on close/dispose, and late API responses cannot reopen them. Verify `TestChatHistorySwitchDropsLateBatchesBrowser` and `TestSoftwareStoreDisposesSensitiveDialogsBrowser`.

- Radio, Personal Radio, and RTL-SDR use separate icon keys and assets in both Papirus and WhiteSur. Keep the backend app manifests, Desktop mappings, theme manifests, and generated main bundle aligned; verify `TestDesktopRadioAppsHaveDistinctThemeIcons`.
- Theme manifests reference only assets inside their own theme directory; an app logo such as `img/desktop-icons/layerling.svg` stays `logo_path` metadata. Tela, Candy, Fluent and Buuf use version-2 action/scheme/folder palettes with per-use provenance; their import owner is `../assets/desktop-icon-themes/AGENTS.md`. `icon-theme-resolver.js` is shared by the shell and generated SDK; edit SDK behavior in `sdk-runtime.js`. Explicit `folder` roles distinguish folders from files, and `action` roles select small controls. Appearance updates use current document-bound MessagePorts and invalidate SDK caches/render epochs. Preserve readable missing-asset fallbacks. Verify `TestDesktopAdditionalIcon*`, `TestDesktopWhiteSurAssetsAreEmbedded` and the importer checks in `../documentation/desktop-icon-themes.md`.

- Desktop theme system: `DESKTOP_THEMES` in `js/desktop/core/theme-runtime.js` is the only theme registry. It loads right after `desktop-foundation.js` and holds one `Object.freeze({ id: '...', ... })` descriptor per line, with the keys in the order `desktopThemeKeyOrder` lists in `ui/desktop_theme_registry_test.go`. Its ids and order must equal `desktop.DesktopThemeIDs()` (`internal/desktop/themes.go`); `light` and `dark` are reserved. Verify `TestDesktopThemeRegistryMatchesGoAndAssets`.
- `applyDesktopThemeAttributes()`, called by `applyDesktopSettings()`, sets the `<body>` attributes `data-theme` (skin rules), `data-shell` (`taskbar` or `dock`; layout rules), `data-scheme` (`light` or `dark`; apps read and observe it) and `data-polish` (`on` or `off`; gates `desktop-polish.css`). Shell code asks `shellLayout()`, `themeUsesGlobalMenu()`, `themeScheme()`, `themeMotionMs(kind)` and `themeIconFamily()`, never a theme id; `isFruityTheme()` is a test-marker alias only.
- An open desktop radial menu must not stay inside backdrop-filter chrome. The taskbar, dock status pill and global menu bar blur their backdrop, and overflowing descendants paint behind that blur. `placeLiftedRadialMenu()` parks a placeholder in the tray and holds `#radialMenuAnchor` on `document.body` until the menu closes. Verify `TestVirtualDesktopCreatesRadialMenuAnchorDynamically` and `TestDesktopFruityDockBrowser`.
- Skins are `css/desktop-theme-<id>.css` (underscores become dashes), bundled after `desktop-wallpaper-live.css` and before `desktop-polish.css`, which stays last. Standard and Fruity have no skin file. Their core tokens live in `desktop-base.css` and their `--vd-theme-*` bridge in `desktop-shell-overrides.css`, whose `.desktop-body[data-theme]` rule resolves the legacy control tokens through `--vd-theme-*`.
- Skin contract (`ui/desktop_theme_css_contract_test.go`): every selector starts with `.desktop-body[data-theme="<id>"]`. The first rule is that bare root rule and defines the whole token set: `color-scheme` (equal to the registry scheme), core `--vd-*`, the 17 `--vd-theme-*` bridge tokens, `--vd-chrome-*`, `--ds-*` and `--vd-accent-contrast` (text colour on the accent; required on every theme root, at least 4.5:1 on fixed accents). Fixed accents also define `--vd-accent`, `--vd-accent-r/-g/-b` and `--vd-accent-glow`; user-accent themes must not. `--vd-text` on the app and panel backgrounds, and `--vd-theme-muted` on the app background, reach 4.5:1. A theme with a `schemeSetting` (Minimal) also has a plain dark root rule, `.desktop-body[data-theme="<id>"][data-scheme="dark"]`, that sets `color-scheme: dark`. It may set the root-only tokens (`color-scheme`, `--vd-theme-app-bg`, `--vd-theme-panel-bg`, `--vd-text`, `--vd-theme-muted`, `--vd-accent` and `--vd-accent-contrast`). The contract merges it over the root and measures its contrast and scheme the same way. Those tokens set in any other rule fail the contract. The optional token `--vd-theme-overlay-blur` defaults to `blur(10px) saturate(1.08)` on `.desktop-body` and drives the backdrop blur of the app modal overlays in `desktop-app-common.css`; Amiga sets it to `none`.
- Motion: `prefers-reduced-motion: reduce` is enforced globally by the override in `desktop-shell-overrides.css`, which sets transition and animation durations to 0 ms and animation iteration counts to one, so skins need no reduced-motion query of their own. Skins additionally gate every `animation` behind `:not([data-animations="false"])` (the Window animations switch). Keyframes are named `vd-theme-<id>-*` and animate only `transform`, `opacity`, `offset` and `animation-timing-function`. A skin stays under 900 lines and Crazy has at most 6 `infinite` animations. `will-change` goes on decorative animated layers and the effects layer, never on a window; `!important` only beats a base rule that is itself `!important` (say so in a comment); `clip-path` only on decorative pseudo-elements. `@font-face` families start with `Desktop Theme `.
- Skins restyle shell surfaces (taskbar or dock, menus, window frames, controls, icons, dialogs) and must not target an app's own classes, except an App tokens rule on an app root that sets the tokens the app exposes for fixed accents (`.vs-app` `--vs-accent-strong`/`--vs-accent-hover`, `.code-studio` `--cs-accent-contrast`). App interiors read `--vd-theme-*`; see the App theme bridge contract in `js/desktop/apps/AGENTS.md`. On coarse pointers the global menu bar is 44 px with 44 px tray controls, and below 1024 px the window menus take a second 44 px row (`desktop-chrome.css`); Amiga sizes its gadgets from the bar height and opts out of the second row in its skin.
- Effects: `fx: true` loads the lazy `desktop-theme-fx` bundle (`theme-fx/fx-core.js`, then `theme-fx/<id>.js`, which calls `register('<id>', { start })`). `staticFx: true` keeps material rendering when motion is disabled; the host receives `{ motion, wallpaper }`. Mobile uses the complete CSS skin. Theme/wallpaper changes invalidate late loads and dispose the previous instance. The host owns `.vd-theme-fx-layer`, the AbortSignal, hidden-tab pause and pagehide cleanup. `AuraDesktopThemeFx.inspect()` exposes copied diagnostics for browser verification.
- Experience themes consume `aurago:desktop-window` from `core/window-effects-runtime.js` and `aurago:desktop-windows-refresh`; never mutate window/application state from effects. `close` fires only after the close guard accepts. `silent` suppresses bursts/sound but never static materials. Geometry snapshots exclude closed, hidden and inactive-space windows. Motion updates follow the manager's animation-frame batching. App contents and foreground windows must be masked; desktop icons/widgets behind a window must not cut holes in its frame. Dragged widgets, menus and dialogs remain foreground masks. All effect layers use `pointer-events: none`. Electronic Dreams reserves a separate component strip in the titlebar grid; never paint it over menus or app content. Verify `TestDesktopLivingMaterialsIdleBrowser` and `TestDesktopExperienceEffectOcclusionBrowser`.
- Green Slime springs store local offsets, never delayed screen positions. Fill the membrane between the deforming outer contour and the fixed, rounded DOM aperture; use the actual corner radii for masks. Paint manager motion events in the same compositor frame. Keep window overflow visible for resize handles and menu popovers while content clips to its own corners. Invariants: one canvas, `.vd-fx-slime-canvas`; the Canvas 2D fallback replaces it and never adds a second, because a canvas keeps its WebGL context. The simulation (`green_slime_geometry.js`, `_bubbles.js`, `_touch.js`, `_sim.js`) stays DOM-free and runs in Node via `vm` (`testGreenSlimeSimulation`, `scripts/test-ui-regressions.mjs`). Gel radius is clamped to `MIN_GEL_RADIUS` (10 px) and `MAX_GEL_RADIUS` (44 px). Coverage contract: alpha of at least 220 near the frame (2 px and 7 px outside each edge and in the corners), alpha 0 inside content. Pointer input uses passive capture listeners on motion only and never calls `preventDefault` or `stopPropagation`; presses on `POKE_BLOCKERS` never poke the gel. WebGL2 (`green_slime_gl.js`) falls back to Canvas 2D on missing WebGL2, software GL (unless `window.__auraSlimeSoftwareGL`), shader build failure, `webglcontextlost` (for the rest of the instance) and sustained slow frames. Keep each file under 1100 lines; the split follows that convention (`scripts/test-infra-ui.mjs` enforces the limit only for the infra app). Sounds: bursts and strand snaps reach the sound system as `slime.pop`/`slime.snap` through `DesktopSounds.play` (the sim queues events, `takeEvents()` and `soundCue()` map them, the bridge's `sound()` calls); the effect never plays audio itself, and `sound-runtime.js` (theme FX table `THEME_FX`, shared with Voxel World) owns the gates and rate limits. Verify real drag/reversal/resize pixel coverage with `TestDesktopGreenSlimeAttachedMembraneBrowser`, six-window pause/disposal with `TestDesktopGreenSlimeSixWindowsBrowser`, the bubble, strand and drop paths with `TestDesktopGreenSlimeInteractionBrowser`, and review close-ups with `TestDesktopGreenSlimeCloseUpsBrowser` (`AURAGO_THEME_RECORDINGS=1`).
- Voxel World uses its own lazy, pinned Three.js/Rapier bundle in `js/vendor/desktop-themes/`, built by `scripts/build-desktop-theme-vendor.mjs`; no game singleton. Use bounded instancing, fixed physics steps, disposal and a complete no-WebGL fallback. Verify `npm run check:desktop-themes` plus `TestDesktopExperience*` browser tests. Special sounds route through the normal sound gate as `window.fracture`/`window.impact`, only for the effective `voxel_world` sound bank. An impact requires incoming normal velocity, not support force from resting piles. Bound the group rate and volume, discard expired audio renders and cancel pending/active sounds on mute; verify `TestDesktopVoxelRestingPileQuietBrowser` and `TestDesktopSoundBrowser`.
- Orbit (`theme-fx/orbit*.js`, `css/desktop-theme-orbit.css`) has its own lazy bundle `js/vendor/desktop-themes/orbit-runtime.js`, built by the same vendor script as Voxel World, and one scene canvas, `.vd-fx-orbit-scene`, built only with the Orbit wallpaper. The GLB pack in `ui/3d/desktop-theme-orbit/v1/` is verified by `scripts/test-desktop-theme-orbit-assets.mjs`. The choreography (`orbit-choreo.js`) and the particles (`orbit-particles.js`) stay DOM-free and run in Node (`testOrbitChoreoAndParticles`). The window bridge (`orbit-windows.js`) writes only `data-orbit-tilt` and its three tilt properties, never app state or input. `orbit-scene.js` is about 1,060 lines: keep it under 1,100 and split new code into its own part before it passes that limit. `orbit.js` publishes the scene's quality as `data-orbit-quality` on the body, which the skin's blur rule reads. The poster `ui/img/wallpapers/themes/orbit.png` is a still frame that `TestDesktopOrbitPosterBrowser` regenerates with `AURAGO_ORBIT_POSTER=1`; regenerate it after visible scene changes. Verify with `node scripts/test-ui-regressions.mjs`, the `TestDesktopOrbit*` browser tests and the Orbit close-ups.
- `startMenu` is a declarative descriptor (`classic` or `xp`), published as `data-start-menu`. XP adds two-column shortcut navigation and its own system icon manifest; app identities can retain their existing local artwork.
- XP shares `--vd-xp-title-height`/`--vd-xp-menu-height` between window tracks and titlebar tracks, including touch sizing. Keep the menubar inside those tracks. Its Start menu uses a single app list and a continuous pale-blue shortcut/category column with content-sized shortcut rows; use the canonical `desktop` icon key. Verify gaps, icon fallbacks, scrolling, touch targets and real menu/shortcut actions with `TestDesktopWindowsXPChromeBrowser`.
- Wallpaper follow: `followThemeWallpaper()` remains available for conditional hand-over outside the explicit theme picker. Merely applying saved settings or restarting must preserve wallpaper and sound overrides.
- Settings picker (`js/desktop/apps/settings.js`): choosing a theme deliberately applies its complete preset in one PUT: `appearance.theme`, `appearance.icon_theme`, `appearance.wallpaper`, empty `appearance.wallpaper_by_space`, enabled `appearance.theme_cursors`, and `sound.theme`. Do not enable sound or motion master switches. Subsequent individual customizations persist until another theme is selected. A failed save changes none of the preset. Verify `TestDesktopExperienceThemeAtomicSelectionBrowser`. The light/dark row shows only for themes with `schemeSetting`; the accent row only for `accent: 'user'`.
- Thumbnails are `img/themes/<id>.svg`: schematic, at most 8192 bytes, with no script, `foreignObject`, event handler or URL except the XML namespace declarations. Every theme needs `desktop.settings_theme_<id>` and `desktop.settings_theme_<id>_desc` in all 16 `lang/desktop/*.json` files.
- Standard/Fruity neutrality: a re-key of their shared tokens must leave every `reports/desktop-audit-browser/` screenshot from `TestDesktopAuditAppearanceBrowser` pixel-identical.
- Add a theme: (1) descriptor line in `theme-runtime.js`; (2) the id at the same position in `DesktopThemeIDs()`; (3) `css/desktop-theme-<id>.css` in the `desktop-shell.bundle.css` entry of `scripts/build-ui-bundles.js`, plus `theme-fx/<id>.js` in the `desktop-theme-fx` bundle when `fx: true`; (4) `img/themes/<id>.svg`; (5) the two locale keys in 16 locales; (6) `npm run build:ui` and `npm run check:ui`, then `go test ./ui -run DesktopTheme -count=1` and `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_SWEEP=<id> go test ./ui -run TestDesktopThemeSweepBrowser -timeout 30m`.

- Tool configuration consumes `_effective_tool_policy` and `_config_migrations`
  as response diagnostics only, through the shared config-response loader on
  initial load and refresh/save paths. Never persist these keys. Removed discovery
  TTL, LTSC-lite and legacy memory-analysis switches have no controls or help keys.
  Skill settings use `tools.skill_manager` and `readonly`; personality activation
  merges the canonical saved value without discarding unrelated drafts. An empty
  MCP-server export allowlist selects no tools; select-all saves explicit names.
  Output archiving remains editable while compression is off. Verify with
  `TestConfigDisclosureSettingsBrowser`, the API/runtime round-trip tests and
  the standard bundle check. Optimizer dashboard counts verified exposures and
  shows variant decisions separately from legacy traces. Chat action indicators
  distinguish unknown, deferred and needs-setup results from successful execution.

- System World uses the Blender GLB kit in `3d/system-world/v1/` and its own
  MIT Three.js 0.186.1 ESM bundle, isolated from the shared global Three.js.
  Run `node scripts/build-system-world.js --check` before packaging. The app
  owns city exploration, five optimized ThreeDee robots that
  steer around furniture and each other, state-driven effects, a night
  atmosphere (aurora sky, stars, moon, sea, mist, drones, lamp cones,
  vignette/grain). The moon is drawn in the sky dome, not as a billboard, so it
  cannot show a square frame. Memory-archive hologram is fed by sanitized excerpts from
  `/api/desktop/system-world/memory-artifacts`, rendered as canvas text only.
  Optional Web Audio ambience pauses with the inactive scene. The same sound
  control includes quiet spatial tower TTS with echo/reverb;
  generation and playback stop on mute, hidden or inactive windows. Its responsive layout must fit the touch viewport; do not put it in
  the mobile wide-window allowlist. V2 adds a bounded 24-hour telemetry history
  and confirmed terminal actions through existing server permissions.
  Rendering/lifecycle and real-shell test contracts live in
  `js/desktop/apps/AGENTS.md`. The v2 expansion adds three waterfront interiors,
  street-bound trams and recorded telemetry. `sysworld-layout.js` owns the shared
  road/plot/surface contract; geometry acceptance uses actual exported GLBs.

- Desktop UI sounds are opt-in shell feedback: `core/sound-runtime.js` in
  `main.bundle.js`, synthesis in lazy `bundles/desktop-sounds.bundle.js`
  (`scripts/build-ui-bundles.js`, `module-loader.js` bundle path
  `desktop-sounds`). Default off; Settings configures theme, volume and category
  toggles. Volume saves must keep the Settings pane scroll; `sound-symbolic`
  uses the speaker mini icon or a currentColor mask. Verify markers with
  `TestDesktopSound*` and browser smoke with `AURAGO_RUN_BROWSER_SMOKE=1`.
  Theme effect sounds (Voxel World `window.fracture`/`window.impact`, Green Slime
  `slime.pop`/`slime.snap`) are one `THEME_FX` family table in `sound-runtime.js`
  with coalescing, an expiry, per-family and per-event rate limits and lazily
  rendered `specialFx[event][variant]` buffers; add a family there, never a second
  queue.

- The desktop screensaver is opt-in (`screensaver.*` settings, off by default).
  `core/screensaver-runtime.js` in `main.bundle.js` owns idle detection, suppression
  and swallowed wake input; the overlay host, five scenes and CSS load lazily from
  `js/desktop/screensavers/` and `css/desktop-screensaver.css`. Tiefsee uses its own
  Three.js 0.186.1 bundle (`js/vendor/screensaver-abyss/`) and Blender GLBs in
  `3d/screensaver/`. Posters in `img/screensaver/` are the reduced-motion/no-WebGL
  fallback and Settings thumbnails. See the child contract.

- HA Switchboard is a lazy built-in desktop app with a theme-independent walnut
  cabinet and silver lever assets under `img/ha-switchboard/`. Retain real shell
  controls, semantic switches and the source asset provenance. All 16 desktop
  locales own `desktop.ha_*`; the Papirus/WhiteSur manifests register its icon.
  `TestHASwitchboardBrowser` covers the real shell with simulated HA responses;
  do not actuate arbitrary home devices for acceptance. App contracts live in
  `js/desktop/apps/AGENTS.md`.

- Speech Output retains local CPU `sanotts` as a fallback, using English for unavailable
  local voices. Existing saved provider
  choices and the disabled option remain editable. Keep all `lang/config/tts/` locales
  aligned; verify with `TestTTSConfigSanoDefaultsBrowser`. Cloud choices include xAI,
  OpenAI, StepFun and archived Resemble with separate Vault key saves, custom voices
  and model selection. Keep provider availability notices visible and verify draft,
  navigation and key masking with `TestTTSCloudProviderConfigBrowser`.
  Shared config state resolves custom dropdown values through the adjacent
  `data-custom-for` input during editing, snapshot collection and DOM restoration.

- Speech Lab Config keeps a persistent live action status for refresh, deployment,
  hardware profile saves, and stack activation. Deployment actions poll the status API
  for their real phase while pending, use an indeterminate progress bar because the
  deployer does not report download percentages, and report completion or failure.
  Disable concurrent action controls. Removal uses `DELETE /api/speech-lab/deployment`.
  Show stable ASR/TTS catalog entries even before their runtime is ready; mark those
  choices unavailable and disable activation until the catalog reports them ready.
  Keep `lang/config/speech_lab/` aligned across all 16 locales and verify with
  `TestConfigSpeechLabFeedbackBrowser`.
  Browser links remain relative `/speech-lab/`; external mode exposes the separate
  server-side `browser_backend_url` and never links to legacy `advanced_ui_url`.

- Fresh setup displays HTTPS enabled/mode/port and uses `window.isSecureContext`
  for browser capability guidance; support and device permission remain separate.
  HTTPS changes belong to installer/Config. Config confirms HTTPS opt-out through
  the shared modal, preserves drafts on cancel and only persists through Save.
  Show the restart address and navigate to its protocol/port for direct access;
  keep proxy and unknown mapped origins. Localize every notice in all 16 locales.
  Verify `TestHTTPSConfigOptOutBrowser` and `TestSetupHTTPSContextBrowser`.

- Server Config shows reverse-proxy trust independently of local HTTPS mode and
  edits `server.https.trusted_proxy_cidrs` as a list of immediate proxy IPs or
  CIDRs. Keep its labels in all 16 `lang/config/server/` locales.

- The Chat integrations drawer omits `boring_computers` from its webhost list.
  The shared webhosts API remains available to other consumers.

- Tabellen uses the local Univer OSS 1.0.4 resource set under `js/vendor/sheets/`.
  Keep its own Autor-style chrome and permanently light grid. Localized input,
  native formulas/clipboard/undo, ETag saves, recovery and Chart.js overlays are
  covered by `TestDesktopSheetsAppBrowser`; the Aurora fixture's
  `AURAGO_SHEETS_MATRIX=1` mode checks all three themes, both densities and
  desktop/touch sizes. Source and lifecycle contracts live in
  `js/desktop/apps/AGENTS.md`.

- Desktop sticky notes use the existing widget API/storage (`type: sticky-note`),
  plain `config.text`, and stable ID-derived paper wear. The background menu adds
  notes; hover/focus/touch exposes edit, duplicate and delete. Preserve server
  read-only gates, escaped text, retryable drafts and existing widget dragging.
  Verify with `TestDesktopStickyNotesBrowser` and the widget persistence tests.

- Leafy is an opt-in installation-wide plant. Its foreground runtime in
  `js/desktop/leafy/` belongs to the widget cleanup scope; never simulate biology
  from frames or client time. Mount foliage inside `vd-workspace`, above icons
  and widgets but below the window layer. Preserve click-through foliage, the shell escape
  control, reduced motion, context-loss fallback, and the 16-locale care UI.
  Server state and revisioned actions live in `internal/desktop/plant_*.go`.
  Healthy uncapped stems add a node each server hour. Scheduled snapshots must
  redraw this growth even when no care action changes the revision.
  Drag the pot or its grip with mouse/touch; a click/tap still opens care.
  Keep desktop selection out of these gestures, persist the local anchor on
  release, and clear drag capture/offsets on cancellation, blur and disposal.
  Pruning uses a 64px scissors cursor and a direct primary click/tap on a stem.
  Keep the segment preview when leaving foliage; the branch list and Cut here
  remain keyboard alternatives. Empty clicks never cut the last selection.
  Only acknowledged cuts animate removed stems/leaves/flowers down behind the
  taskbar, in both renderers. Capture the complete removed geometry, including
  offscreen foliage, within four million pixels. Bound the transient canvas to
  one and dispose it on completion, undo, hiding, resize, dragging, context loss
  or widget cleanup.
  Reduced motion and disabled animations skip the fall; pending writes block cuts.
  See `documentation/leafy.md` for care, atlas baking and browser checks.
- The opt-in `builtin-printer` widget lives in `js/desktop/core/widget-printer-runtime.js`.
  Keep missing printer metrics unknown, render filenames as text, and stop status
  requests/camera streams on document hiding or widget disposal. Its native dialog
  moves the existing camera image rather than opening a second stream. Verify with
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopPrinterWidgetBrowser`.
- The opt-in `builtin-fritzbox` widget lives in `js/desktop/core/widget-fritzbox-runtime.js`
  with hand-built SVG charts in `widget-fritzbox-charts.js` and styles in
  `css/desktop-widget-fritzbox.css` (part of the desktop shell CSS bundle). It is
  read-only, reads only the sanitized `/api/desktop/fritzbox/overview` endpoint,
  renders router text as text, keeps its traffic history in browser memory only and
  releases timers, observer and listeners on disposal. Its pager, polling and i18n
  contract lives in `js/desktop/apps/AGENTS.md`. Verify with
  `go test ./ui -run TestDesktopFritzBoxWidget`.

- Operational pages share one canonical skeleton inside `main.pw-page-frame`:
  a page heading (`pw-page-heading` with `pw-page-heading-copy` and
  `h1.pw-page-title`, optional `p.pw-page-description`), then an optional status strip (`pw-status-strip` with
  value before label), then the primary tab strip (`pw-tabs`/`pw-tab` with
  tablist semantics), then an optional `pw-toolbar`, then content. Page
  titles live in the content area, never in the app header.
- Every operational app header uses `.header-left` wrapping the logo link,
  plus density and theme toggles in `.header-actions`.
- Search fields on operational pages use `pw-search` (often dual-classed with
  legacy ids such as `ct-search`, `sk-search`, `kc-search`, or
  `gallery-search`). List pages expose one page-level `pw-search` in the
  page toolbar; Knowledge keeps panel-local search fields with `kc-search
  pw-search`.
- Operational tab labels are text-only. Dashboard may keep SVG tab icons for
  scanability; other operational pages must not put emoji in tab labels.
- Tab lists use `data-i18n-aria-label` for localized tablist labels, not hard-
  coded English `aria-label` values.
- Filter chips and origin/security pills use badge/chip styling (for example
  `pw-badge`); they must not also carry `pw-tab`. List filters belong in the
  page toolbar as `pw-badge` chips inside a `role="group"`, not as a second
  `pw-tabs` strip.
- Operational chrome (tabs, filters, search, view toggles, heading CTAs,
  pagination, bulk actions) binds through listeners or `data-action` /
  `data-*` hooks — not inline `onclick`. Modal form logic may keep existing
  handlers until migrated separately.
- Empty states use `.empty-state` with `.empty-icon`, optional `h3`, `p`, and
  CTA buttons — not legacy `.icon` wrappers alone.
- Every operational app header contains a density toggle
  (`[data-pw-density-toggle]`, styled `pw-density-toggle`) next to the theme
  toggle; `js/precision/workspace.js` discovers it automatically.
- Tab strips use the boxed Precision style only. Page tab classes
  (`.kc-tab`, `.sk-tab`, `.media-tab`, `.invasion-tab`,
  `.cheatsheet-tab`, `.tab` with `.ct-filter-btn`) must not re-skin tabs with
  underline indicators, pill shapes, or per-page tab backgrounds; shared tab
  visuals belong to `precision-pages.css`. Knowledge has no tab indicator
  element or indicator JS.
- Never load Precision Workspace assets from `index.html`, `desktop.html`, or
  `gallery.html`.
- Do not change Chat, Virtual Desktop, or an asset they share as a side effect
  of Precision Workspace work. This includes `shared-variables.css`,
  `shared-utilities.css`, `shared-components.css`, `shared-animations.css`,
  `fonts/fonts.css`, `js/shared/`, Chat bundles, and Desktop bundles/modules.
- Precision Workspace CSS must remain scoped under `.pw-page`; no unscoped
  reset, token, component, or motion rule may leak to another page.
- Operational templates opt in with `.pw-page`, a unique
  `data-workspace-page`, `precision-workspace.css`, `precision-pages.css`, and
  `js/precision/workspace.js`. Entry templates use `.pw-page.pw-entry-page`,
  `precision-workspace.css`, and `precision-entry.css` only.
- Migrated templates must not contain `style` attributes or `<style>` blocks.
  Put page-specific rules in the owning stylesheet. Operational Precision
  declarations must be consolidated selector-by-selector with functional page
  rules in that stylesheet's normal rule structure. Every operational selector
  must be scoped with the page's `data-workspace-page`; do not keep separate
  Precision and legacy layers, permanently appended/delimited adapter blocks,
  superseded legacy surface tokens, glassmorphism, or glows. Gradients, shadows
  and decorative animations remain prohibited outside Config; Config's bounded
  surface and interaction effects are defined below. Shared Precision `--pw-*`
  hues match Config's blue/slate palette so operational and entry pages share
  one canvas/accent family. Entry-page additions remain scoped with
  `data-entry-page`.
- `window.AuraPrecisionWorkspace` owns the browser-local
  `aurago.workspace.density.v1` preference and exposes `init()`,
  `getDensity()`, and `setDensity("comfortable"|"compact")`. It migrates the
  legacy `aurago.config.density.v1` key once; Config must not access either key
  directly.
- Configuration connection tests operate only on saved configuration. Dirty,
  incomplete, or credential-missing sections expose a visible locked reason.
- Config owns form presentation in `config-workspace.css` and
  `js/config/presentation.js`, scoped to `data-workspace-page="config"`.
  Shared Precision Workspace owns the blue/slate `--pw-*` palette in
  `precision-workspace.css` for every opted-in operational and entry page.
  Config may still remap the same hues plus `--cfg-*` card tints and bounded
  shadows. Keep Geist, 16px inputs and 44px controls in both densities. Below 1100px the
  labeled sidebar becomes a keyboard-accessible drawer; the save dock stays in
  the viewport layout without covering the scrollable form.
- Every Config section icon needs its `SECTIONS` key in
  `CONFIG_SIDEBAR_ICON_SLOTS`, a `CONFIG_SIDEBAR_ICON_SYMBOLS` entry, the
  matching 128px cell in `img/config-sidebar-icons.svg` and `.json`, and a
  `.config-icon-slot-N` position in `config-workspace.css`. The sidebar draws the
  inline symbol, so a missing one renders blank; it must match the sprite cell.
  The sprite has 11 columns and 12 rows (slots 0–131; `gog` 121, `ruview` 122, `cyd_creator` 123, `decisions` 124):
  x = column × 10 %, y = row × 100 / 11 %. A thirteenth row needs the same
  growth in `main.js`, the CSS, the SVG canvas, the metadata and the test
  constants. Keep all slot rules in one ordered CSS block and the sprite
  well-formed XML without repeated attributes.
  Verify `TestConfigSidebarIconSpriteContract`.
- Config uses one visible card level: named topic cards containing flat fields,
  with a compact variant for independent objects. Reuse `AuraConfigForm` and the shared presentation pass for lazy
  integration renderers. Preserve their data bindings, independent provider /
  credential save paths and native checkbox semantics.
- Config topic boundaries are explicit renderer headings/groups or exact selectors
  in `AuraConfigCatalog.presentation`; never infer cards from nested field wrappers.
  Card headings use 18px text, fields 16px, help 14px, with 24px spacing (16px
  compact/mobile). Switches sit beside their label and help. Keep the shared
  blue/slate surfaces, softly tinted blue/cyan card heads and subtle blue-tinted shadows.
  Use 16px card corners, the `--cfg-card-heading` tint and `--surface-shadow`
  (0 6px 20px, 16% dark / 7% light); dialogs use `--surface-shadow-strong`.
  Interactive entry cards may lift at most 2px; switches/disclosures use 140–180ms
  feedback and save success may animate once. Reduced motion disables these effects.
  The overview has no save dock; independent saves identify their scope.
- Every Config topic has one `.pw-panel-body` owning all four content insets;
  do not simulate body padding with margins on individual card children. Keep
  dynamic list refreshes inside that body so headings survive. Switch fields
  use the shared label-left/control-right row; hidden action reasons consume
  no layout space and visible reasons follow the action row. Empty-state
  decorations must never cover their text. Check these details geometrically
  and visually with the real integration renderers.
- Config advanced fields require explicit `sectionTiers` or `data-tier` metadata.
  Never infer tiers from name fragments or move fields across topic boundaries.
  Required fields and security notices stay visible. Search and validation open
  ancestor disclosures; `searchSections` maps fields to their actual editor.
  Failed saves retain inputs and persistent inline feedback.
- Config and Setup share the managed local model choices: Qwen remains the
  default for missing `model_family`; Ling selects Q4_K_L, MTP off, and 16K.
  Model changes reset incompatible options; Setup also resets its probe acknowledgement.
  runtime status and setup progress display the selected model name.
  Cache help distinguishes native reuse from idle qualification; a transient
  qualification failure must not claim that subsequent requests disable reuse.
- Provider model-limit fields use `0` for automatic resolution. Provider cards
  show compact effective context/output values; the source and the full
  configured/effective sentence belong in the card tooltip and in the editor.
  Unknown-model warnings appear as a compact status badge on the card plus the
  localized conservative-limit text in `title` / `aria-describedby` and in the
  editor. Cards show name, type, model, auth state, usage roles from
  `references`, and the internal ID. They must not dump Base URL, capability
  pills, or raw key masks. Assignment of roles stays in the owning Config
  sections.
- Integration actions that depend on credentials remain locked until the
  authoritative saved runtime status is ready. After `aurago:config-saved`,
  visible integration sections refresh that status; independent catalog
  requests preserve successful results when a sibling request fails. A section
  that caches its runtime status across renders (Netlify) drops that cache on
  `aurago:config-saved` so a new token or toggle is visible without a reload.
- Telephone agent is a dedicated lazy Config section under Agent & AI. SIP Phone
  owns account/network/trust/browser-media settings and only links to the
  telephone profile; it must not expose a second editable copy of `sip.voice`
  or the agent inbound route/delay.
- Speech Lab voice upload carries a single-use turn token into the next matching
  chat submit. Without AudioWorklet, use browser SpeechRecognition when
  available; never feed MediaRecorder output to the WAV-only endpoint. Map the
  `speech_lab_no_speech` response to the localized no-audio retry message rather
  than showing a generic Speech Lab failure.
- Desktop Live Speech may select a keyless `speech_lab` profile. That adapter
  uses local VAD, `/api/realtime-speech/transcribe`, `aurago_execute`, and
  `/api/realtime-speech/synthesize` against the managed or external s2s
  container. It must not send `llm_id` or replace the OpenAI, xAI, or Gemini
  streaming adapters. The app may poll `/api/speech-lab/status` and start a
  managed container via `/api/speech-lab/deployment/start`.
- Realtime Speech consumes the answer from `final_response`; `done` is a
  contentless terminator. While `aurago_execute` runs, display backend `progress`
  events as captions and play their server-synthesized audio through the active
  Speech Lab voice or effective chat TTS. Never use browser speech synthesis or
  add progress to the chat transcript/tool result. Do not speak over the user or
  provider; stop in-flight audio on completion, cancellation, or session close.
  Feed audible progress clips to the same analyser-driven Persona mouth motion.
  Forward the exact progress caption as `message` to the audio endpoint; the
  server validates its translated phrase allowlist so spoken variants match.
- SIP Phone surfaces `outbound_policy_migration_required` as a localized setup
  blocker.
- Live Speech remembers the selected/started profile ID in browser storage
  `aurago.realtimeSpeech.profileId.v1`, shared by Chat and Desktop. Restore only
  selectable profiles; otherwise use the available configured default or first
  available profile. Blocked storage keeps the selection in memory. Never start
  a session automatically when restoring a selection.
- Live Speech audio selection lives in a native dialog opened by a labelled
  speaker button, placed in the Desktop app header through the shared panel's
  `audioControls` mount option. Keep browser/headset selection and storage shared
  with Webchat; opening the dialog never starts audio. Preserve native Escape,
  focus return and mount cleanup. Verify `TestRealtimeSpeechAudioPickerBrowser`.
- Live Speech's shared panel owns one `AuraRealtimeSpeechAvatar` per mount.
  Webchat passes `visible: false` until its overlay opens and calls
  `AuraRealtimeSpeechUI.setVisible`; unmount disposes the avatar. Desktop
  visibility also follows intersection/tab state. Animation never starts audio.
  Catalog membership selects local Rive assets; initial personality resolution
  must also work before Desktop Agent Chat opens. Preserve custom PNG fallback.
  While resolving a persona or loading its animation, leave the portrait empty.
  Do not show the static PNG as a stand-in. Show only the selected persona.
  Use the selected PNG only for reduced motion or an actual load failure, and
  the Custom image only for custom personas or an unknown key.
  Both Rive asset CDN and WASM fallback CDN are disabled. Reduced motion uses
  the selected PNG; stale loads and detached mounts cannot revive a player.
  Output analysers alone drive the mouth, including queued audio tails; paused
  media and suspended contexts return zero. `mouthOpen` is a gate in these
  assets, not a morph: use energy-dependent discrete poses and prompt closure
  in short pauses, never a permanently open AA pose or claimed phoneme timing.
  Quiet idle/passive listening may briefly vary existing facial expressions
  (8–16 second gaps, 1.2–2.2 second duration), then restore the runtime face.
  Speech, user voice, pending output, actions and errors override decoration;
  hidden/reduced-motion views and persona switches reset its schedule.
  Verify with `node scripts/test-realtime-speech.mjs` (includes avatar checks).
- Configuration density is a browser-local presentation preference and never
  belongs in `config.yaml`.
- Every visible UI string must use translations in all supported locales.
- Keep one canonical section definition per locale key where possible. If a key
  occurs in multiple sections, its values must agree in every language; verify
  with `TestTranslations_NoConflictingCrossSectionValues`.
- Qualify translation-audit candidates by locale and key before editing; preserve
  technical names, URLs and placeholder-only formats. Use targeted checks such as
  `TestTranslationsAuditCorrections`, not blanket ASCII or German pronoun bans.
- Spanish addresses the user with informal `tú` imperatives (`Elige`,
  `Introduce`, `Selecciona`), never `usted` (`Elija`, `Introduzca`,
  `Seleccione`). Tests that pin Spanish strings use the same register.
- File Manager undo/redo error translations must interpolate `{{error}}` in
  every locale so the failure details passed by the action remain visible.
- God's Eye View uses the regular container-app window, starts maximized, and
  grants microphone capability only to its own frame. Closing removes the frame;
  service start/stop remains in the Store. The Store setup dialog owns optional
  provider keys and allowed AuraGo origins; never prefill stored credentials or
  copy AuraGo provider keys. Keep all 16 Desktop locales and the local upstream
  logo plus MIT notice synchronized. See the Desktop app child contract.
- MeshCore's serial-port field is a native dropdown populated by `/api/meshcore/devices`. Refresh preserves the selected draft value; a missing saved port remains selected and visibly marked, and enumeration failure never clears it.
- MeshCore's optional `additional_prompt` textarea uses the shared saved/draft path, a 2000-character limit, and all 16 Config locales. Clearing and saving removes the MeshCore-specific agent guidance; it never changes permissions or manual Messenger sending.
- MeshCore connection tests show immediate busy feedback and a persistent result beside the action buttons, independently of the general runtime status. Successful transport tests still show required identity confirmation; they never confirm identity automatically.
- MeshCore's synchronized node list has a viewport-bounded independent vertical scroll area, keyboard focus and wrapping full keys; large contact tables must not stretch the settings page over many screens.
- MeshCore node search matches names and full keys locally. Click adoption adds the full key once to an explicitly selected trust or proactive-target draft list (default: send targets); only other chat nodes are eligible. Persist through the shared Save action and never enable proactive sending implicitly.
- MeshCore Messenger is the builtin `meshcore` Desktop app, separate from connection/agent settings. Reuse its single manager, administrative Messenger API and metadata-only desktop events. Protected content requires explicit reveal; never render radio text as HTML. Private invitations are transient, explicit admin exports with no caching/storage. Keep all 16 `lang/desktop` locales, responsive chat layout and request/listener cleanup covered. Its "Mesh" visual system (scoped `--mc-*` tokens, grouping, opacity-only hover actions, keyboard navigation) is contracted in `js/desktop/apps/AGENTS.md`. The hidden-by-default `builtin-meshcore` widget shows the latest conversations read-only via the messenger bootstrap endpoint and `aurago:meshcore-change` events, never reveals protected previews, and opens the app with a validated conversation ID on click.
- MeshCore Config uses the shared saved/draft path, explicit device/channel binding confirmation, full-key trust lists and separate reply/proactive permissions. The trust list stores the contact; direct replies must be on before that contact starts the agent and may receive a radio reply. The direct-replies toggle explains this in every `lang/config/meshcore/` locale. Its inbox and discovered device names render external text with `textContent`; pairing PINs are transient. Discovery shows a live spinner and selectable results; selecting a device updates only the draft Bluetooth address/transport, never trust or pairing automatically. Device tests and pairing use saved settings, bounded fetches and inline status. Removing a device channel requires a confirmation, a clean Config draft and the active identity/slot/binding conversation; use the Messenger manager API, then rebase the saved MeshCore draft from device status. Removing an orphaned channel rule remains a draft-only Save action. Keep hardware acceptance visibly unverified and all locale files under `lang/config/meshcore/` complete.
- Chat loads typed notifications from `/api/system/notifications`, renders `morning_briefing` separately from generic notices, and acknowledges only displayed IDs through `/api/system/notifications/read`. The legacy string endpoints remain server-compatible but are not the primary Chat UI path; generic notices must never be labeled as morning or system briefings.
- Chat tool icons normally use the fixed 10x10 PNG sprite. A provider that needs a distinct icon after those cells are allocated may declare one embedded transparent custom asset in `tool-icons.js`; `applyIcon` must add the build-version cache key, clear custom inline background state when an element returns to a sprite icon, and UI regression tests must verify the asset is embedded.
- The Dashboard operational-issues view uses the sanitized admin API only, renders dynamic issue data with `textContent`, and requires an inline confirmation before archival or resolution. It must never decode or display internal fingerprints, raw logs, or unredacted error text.
- Dashboard Audit and Cronjobs searches use one `createLatestRequestGate` per card: a newer request aborts the older one, only the newest response renders or reports an error, and the audit offset changes only after its page loaded. Verify `npm run test:ui-regressions`.
- Config treats an unreadable `/api/vault/status` or `/api/runtime` as unknown, never as absent or available: runtime-gated sections (Firewall, Sandbox, Updates, Invasion Control) stay locked behind a retry banner, the master-key field is disabled, and the Knowledge Secrets tab shows `config.secrets.vault_status_unknown` with a retry instead of "No vault found". Retries never re-render over unsaved edits. Verify `TestConfigRuntimeStatusUnknownLocksSectionsBrowser` and `TestSecretsSectionShowsUnknownVaultStatusBrowser`.
- The Dashboard knowledge-graph visual (`js/dashboard/widgets-knowledge.js`) ships two renderers: a 3D WebGL constellation (`ui/js/vendor/3d-force-graph.min.js`, default) and an enhanced 2D canvas (`force-graph.min.js`), switched by the 2D|3D toggle persisted in `aurago.dashboard.kgview.v1` with automatic 2D fallback when WebGL or the vendor bundle is unavailable. `3d-force-graph` **1.80.1** and its loaders share the vendored Three.js **0.186.1** global through `build-browser-vendor.js`; regenerate them together. Construct renderers with `new ForceGraph3D(element, {controlType: "orbit"})` / `new ForceGraph(element)`. WebGL2 is required for 3D; preserve the 2D fallback. Node clicks keep opening the KG detail modal; effects (glow, particles, pulse, starfields, auto-rotation) live inside the canvas only, honor `prefers-reduced-motion`, pause on hidden tabs, and rebuild on `aurago:themechange`. Renderer instances, ResizeObserver, visibility listener, and 3D geometries are disposed through `destroyKnowledgeGraphVisual` on every mode switch and empty state.
- The Dashboard personality card shows affect valence/arousal, recent sanitized affect events, and lived notes. Affect cause codes and sources are localized; chat-sourced event details never include the raw user message. Config Personality and Prompts warn when V2 mood, emotion synthesis, or inner voice need the Helper LLM; Prompts lists live notes as read-only and points management to the Dashboard. Note and event text use `textContent`.
- Config Personality and the Dashboard share `js/shared/personality-dynamics.js` for load, familiarity, friction, trend and the short-term reset. Preserve config drafts, reject older state revisions and show reset failures without clearing current values. Keep all 16 common locale bundles complete; verify `TestPersonalityDynamicsBrowser` and `npm run check:ui`.
- Skill-card list fields must render in deterministic sorted order matching
  their DOM-diff snapshots so API-only reordering cannot leave stale cards.
- The service worker caches only same-origin static assets, retains full
  versioned request URLs, and keeps HTML, API, event, and auth traffic network-only.
  `site.webmanifest` must keep `id`, `start_url` and `scope` at `/`, expose
  `any` plus `maskable` icons, a description, and Chat/Desktop shortcuts.
  Pages that link the manifest declare `apple-mobile-web-app-capable` and
  `mobile-web-app-capable`. `initPWA` registers the service worker even without
  `PushManager`. Chat registration uses `updateViaCache: 'none'`. Precache
  failures must not block install. Standalone PWA resume reconnects `AuraSSE`
  after hide/freeze/offline and reloads once when `/api/ready` reports a
  different `X-AuraGo-Asset-Set`.
- CanvasUI components are vendored as local framework-free ESM under
  `ui/js/vendor/canvasui/` with committed `manifest.json`, `LICENSE.txt`, and
  pinned upstream provenance. They must not load remote assets at runtime and
  must not introduce React solely for effects. Static ES-module imports of
  vendored files (e.g. `from "/js/vendor/canvasui/droplets.js"`) carry no
  `?v=` and would let the service worker / HTTP cache pin stale vendor builds
  across server updates; each page that loads such a module must therefore
  declare a `type="importmap"` entry mapping the bare URL to its
  `?v={{.BuildVersion}}` form (desktop.html → droplets.js, login.html →
  flame-wrap.js). Login uses Flame Wrap on the
  auth card. The login shell stays viewport-locked and centered; Flame Wrap
  must paint outside the card without expanding document scroll or uncentering
  the form. Droplets are desktop-only for the `city_rain` wallpaper via
  `ui/js/desktop/city-rain-droplets.js`, painted on `#vd-wallpaper-fx`
  (`z-index: -1`) behind icons/widgets/windows, with graceful WebGL2 /
  reduced-motion fallbacks and no HTML capture of foreground UI.
  Droplets must refract the `city_rain` wallpaper bitmap; missing content
  stays transparent and must never fall back to gray procedural glass. The
  refraction source is the fully decoded `HTMLImageElement` (no
  `createImageBitmap` — freshly decoded bitmaps can rasterize black on some
  Chrome GPU paths on first uncached load), and `paintBitmap()` must verify
  via a pixel probe that the blit received non-black pixels, otherwise it
  reports not-ready so the next frame retries instead of locking in a black
  texture.
- Animated wallpapers (`silk_flow`, `firefly_dusk` calm; `neon_overdrive`,
  `fractal_trip` wild) are fragment shaders in the module
  `js/desktop/live-wallpapers.js`, painted on one WebGL canvas in
  `#vd-wallpaper-live` (fixed, `z-index: -1`, behind the whole shell). A new
  one needs its id in `internal/desktop/types.go`, `spaces-runtime.js`, the
  settings app, the context menu (below the separator), a CSS fallback in
  `css/desktop-wallpaper-live.css`, ambient colours and 16 translations.
  Render below device resolution within `MAX_PIXELS`, cap the frame rate,
  pause while hidden, covered by a maximized window or the screensaver, draw a
  still frame for reduced motion or `data-animations="false"`, release the
  context for other wallpapers and keep brightness pulses gentle (no flashes).
  Verify with `TestDesktopLiveWallpapersBrowser`.
- `wallpaper-visibility.js` owns the shared maximized-window/screensaver
  occlusion predicate. City Rain also stops while hidden or covered; repeated
  pagehide/pageshow must reinstall observers exactly once and reject stale loads.
- Shell session v2 adds stable window keys and an active-window key; restore
  focus independently of always-on-top stacking and suppress restore sounds.
  Compact mode preserves the logical active space. Resize/snap share clamped
  app minimums, with reachable viewport bounds taking precedence.
- Maximized windows end above the taskbar or dock shell (`data-shell`) at every width via
  `--vd-maximized-window-reserve` (`desktop-windows.css`); keep it, the narrow
  overrides and `--vd-fruity-dock-reserve` in step with the bar/dock geometry.
  The touch phone layout keeps its own `--vd-mobile-taskbar-reserve`. Verify
  `TestDesktopMaximizedWindowClearsTaskbarBrowser`.
- The start menu is categorized: `AppManifest.Category` (set for every builtin
  and copied from `CatalogEntry.Category` for Store apps,
  `desktop.DesktopAppCategories()` in display order; only apps without a known
  category, e.g. agent-generated ones, fall into `installed`) drives a rail (`#vd-start-rail`, tablist) beside
  the app pane (`#vd-start-pane-head` + `#vd-start-apps`). "Recent" appears only
  with history, "All apps" lists sections per category, search shows a flat result
  list and dims the rail. `START_MENU_CATEGORIES` in `core/window-shell-runtime.js`
  must mirror the Go list; labels are `desktop.category_<id>` plus
  `desktop.start_*` in all sixteen desktop locales. Hover switches after a short
  intent delay (mouse only), arrows move within the rail and hand focus to the
  pane and back; the active pill slides via `--vd-rail-y`, pane switches reuse the
  open cascades, everything gated by `data-animations`/reduced motion. The
  selection persists in `aurago.desktop.startCategory.v1`. Verify
  `TestDesktopStartMenuCategoriesStayInSync` and
  `TestBuiltinAppsCarryStartMenuCategories`.
- Spotlight responses belong to a search generation and open instance. Quick
  Chat owns its AbortController/reader through widget cleanup. Shell popups
  close on Escape and return focus to their accessible opener.
- `scripts/build-ui-bundles.js` is the source of truth for generated Chat and
  Desktop bundles; `npm run build:ui -- --check` must be read-only and pass.
  Every Desktop main-bundle part starts and ends at a function boundary inside the shell IIFE (only
  `desktop-foundation.js` opens and `sdk-events-bootstrap.js` closes it); never cut a function across
  part files to meet the line budget. Verify with `npm run test:ui-regressions`.
- The shell receives server Desktop events over both its WebSocket and SSE.
  `handleDesktopEvent` drops an event whose `id` it handled in the last minute
  (`desktopEventAlreadyHandled`), so one notification shows one toast and one
  bell count. Events without an id (agent tool events, SSE only) always run.
  Verify the regression "Desktop event delivered over WebSocket and SSE is handled once".
- `ui/css/desktop-polish.css` is the Desktop's finishing layer and stays the
  last part of the shell CSS bundle: it only refines existing surfaces (ambient
  wallpaper light in taskbar, dock, menubar and menus, focus edge light and
  glow, vignette, pointer light, taskbar indicators, toast countdown and hold,
  bell ring, tray popover rise/drop, tray focus rings and press feedback, icon
  glyph lift). Tray popovers (clock popup, notification centre) open away from
  their bar through `placeTrayPopover` (`data-placement`), never off-screen
  under the Fruity menubar. Desktop icon labels clamp to two lines and reveal
  the full name on hover, selection and keyboard focus (`desktop-icons.css`).
  Every wallpaper needs `--vd-ambient-top/-bottom/-glow`; photo values
  come from `python scripts/wallpaper-ambient.py`. Its selectors double
  `.desktop-body` where `desktop-base.css` has mode-specific glass rules.
  Motion is gated by `data-animations` and `prefers-reduced-motion`; the
  pointer-light target list in `ui/js/desktop/core/polish-runtime.js` must
  match the CSS (`desktop_polish_layer_test.go`).
- ThreeDee combat uses a fixed four-light impact pool, at most 240 sprites
  (40 slots reserved for smoke/debris), six unlit, shadow-free blink ghosts,
  four surface mines, and five fuel slicks. The white robot deploys fuel slicks
  that ignite and incapacitate the red robot (`burningUntil`); the red robot
  counters with stasis mines (`mineType: 'stasis'`) that stun and incapacitate
  the white robot (`stunnedUntil`) with lightning arcs. Continuous particle
  emission follows simulation time; cinematic slow motion expires on wall time,
  and every blink entry point enforces its cooldown. Helix volleys and
  damage-triggered EMP counterpulses reuse projectile/effect cleanup and
  respect the existing 18-projectile limit. EMP must not interrupt a paired
  nova clash. Robot GLTF/Draco assets resolve through `BUILD_VERSION`, retain
  live Draco workers across in-flight theme resets, and retry failed loads with
  bounded backoff. Keep reduced-motion and theme-exit disposal intact.
- Cyberwar uses the generated `img/cyberwar-city.webp` as a static, centered
  cover background in `#chat-box`, beneath a navy dimming gradient and the HUD.
  Keep the image position fixed in the scanline keyframes and retain readable
  message surfaces, including on narrow screens and with reduced motion.
- Dark Sun is an eclipse scene. `body` paints the violet-black sky, star specks
  and the horizon glow; `body::before` carries the static eclipse and lava
  horizon SVG (the no-JS, reduced-motion and narrow-screen baseline) and fades
  out while the engine is live. `js/chat/dark-sun-shader.js` is the whole
  engine: `#dark-sun-sky` (WebGL, at most 1280x720: stars, breathing corona,
  prominence loops, the black disc, a travelling diamond-ring glint, the
  eruption plume and light wave) and `#dark-sun-scene` (2D basalt plain with
  cached glowing cracks, ridge silhouettes and the 2D eclipse fallback) sit
  behind the chat; `#dark-sun-overlay` (2D, screen blend) carries embers that
  rise from the cracks, sparks when an ember meets a bubble, pointer heat and
  the light wave over the chat. Pools: 160 embers, 240 sparks; a single RAF
  loop; canvas bounds never transition. Every 20–35 s a 5 s eruption triples
  the ember spawn, launches the wave after one second and publishes
  `html[data-darksun="calm"|"flare"]` plus `--darksun-flash` on
  `.app-header`/`.app-footer`; `css/chat-themes.css` reacts and `stop()`
  clears both. Bubble tails keep the anchored `::before` contract. Gates:
  theme, hidden tab, reduced motion and `innerWidth >= 768`. The DOM ember
  layer (`dark-sun-embers.js`) is retired; `theme-effects.js` loads only the
  engine. Contracts: `TestDarkSunEclipseBrowserSmoke`
  (`AURAGO_RUN_BROWSER_SMOKE=1`, `AURAGO_DARKSUN_BENCHMARK=1` for 120
  native-RAF frames at 1920x1080) and `TestChatFrontend_DarkSunSceneStaysPolished`.
- Sandstorm is a layered desert scene. `body` paints sky, sun bloom and static
  SVG dunes (the no-JS, reduced-motion and narrow-screen baseline). The engine
  adds `#sandstorm-fog` (WebGL sky, sun, crepuscular rays, dust, the dust wall
  of a storm and the lightning tint) and `#sandstorm-scene` (three cached dune
  ridges with parallax, clouds, dust rolls, the lightning bolt and the 2D sky
  fallback) behind the chat, and `#sandstorm-overlay` (grains, trails, ground
  pile, sand resting on bubbles) above it. Dust, grains and ground lift share
  a smooth wind/gust envelope; three moving counter-rotating eddies drive the
  fog and particle velocity field; grains must visibly turn, rise and
  recirculate, and wind changes direction gradually. Each 9 s storm sends a
  dust wall across the scene from the windward side (`u_front`), dims the sun,
  schedules one to three dry-lightning strikes (the first always shortly after
  the attack) and sweeps the sand off bubbles. While live the engine publishes
  `html[data-sandstorm="calm"|"storm"]` and sets `--sandstorm-flash` on
  `.app-header`/`.app-footer` during strikes; `css/chat-themes.css` reacts to
  both and `stop()` clears them. A `pointermove` gust pushes nearby grains
  without extra loops or pointer capture. Keep the fixed particle pools and
  the fog buffer at most 960x540 pixels; soft dust does not need device-pixel
  resolution. Canvas bounds must not transition. Preserve the 2D fallback,
  hidden-tab pause and reduced-motion and narrow-screen gates. Static
  contract: `TestChatFrontend_SandstormSceneStaysPolished`.
- Sandstorm desert life: `ui/js/chat/sandstorm-life.js` (exports
  `window.AuraGoSandstormLife`) loads lazily (`loadDesertLife()`, again in
  `start()`); a failed load removes its script element and the scene runs without it.
  No timers or frames: the engine's RAF loop calls `update` and `draw`, the latter
  between the dune layers. Draws: worm, heat shimmer, crest wisps, vultures,
  tumbleweed, beetle, caravan. The worm, beetle and caravan pick spots against the
  live bubble, avatar and mascot rectangles (`#chat-content .msg-row .bubble` and
  `.avatar`, clipped to `#chat-box`) and `#chat-robot-mascot`, read once per spawn.
  The vultures' thermal is picked against the sky-band rectangles, re-checked every
  15 s; it drifts there at most 24 px/s. Nothing reads the chat per frame. A storm
  fades every creature, sends the vultures off, dissolves the caravan and makes the
  beetle burrow; a running worm finishes. Calm ahead needed: worm 9 s, beetle and
  caravan 6 s. Fixed pools (`MAX_*` caps). Harness: `stats()`, `force*()`,
  `debug.where()`. Tests: `TestSandstormDesertLifeCompanionStaysWired`,
  `TestSandstormDesertLifeVulturesAvoidTheChat` and
  `TestSandstormDesertLifeBrowserSmoke`.
- Galaxy uses the shared Three.js 0.186.1 and a single lazy renderer/RAF loop.
  Keep the ten draw calls, shared sphere geometry and fixed 3500/850-star
  buffers. Exactly 20 stars flicker subtly with individually randomized pauses;
  four distinct low-poly ships (saucer cruiser, cargo freighter, ring explorer,
  shuttle) each bake their lit hull, bridge, engines and thruster plumes into
  one mesh with shader-driven thruster/strobe glow. Keep diagonal courses
  visible at every aspect ratio and wrap fully offscreen. Planet/backdrop
  shaders include subtle aurora, city-light shimmer, storm lightning and
  micro-meteors; pointer movement drives gentle parallax and `--galaxy-mx` /
  `--galaxy-my` glass highlights. All effects use scene time and the existing
  pause/disposal lifecycle.
  No full-screen postprocessing. Start desktop at high quality with
  at most 1.5 DPR / 3840x2160 pixels, then lower resolution after sustained
  slow frames. Calibrate the idle display cadence during the first-frame fade;
  30/32 Hz displays must not trigger quality reduction. Mobile starts with 2K
  maps and the simpler atmosphere.
  Pause hidden tabs; release all GPU resources on exit and reject stale loads.
  Reduced motion, unavailable WebGL, missing textures and context loss expose
  the complete local poster. Keep posters aligned with the rendered scene and
  preserve source provenance in `img/galaxy/CREDITS.md`.
  Galaxy chat is an orbital-glass cockpit: a violet/cyan/gold outlined header
  with the local orbit wordmark and a clock segment at its right end, a footer
  composer ordered Voice, Live, File, Tools, input, Send, and two equal glass
  rails between them: navigation on the left (Desktop, Integrations,
  Conversations, Dashboard, Missions, Config as icon + caption) and the live
  status pills (connection, tokens, budget, credits, debug) on the right, moved
  there by `galaxy-interface.js` and restored on exit. Header and footer span
  the full width with 16px gaps on every side and share their resting height;
  the rails sit 16px below/above them with the same width (`--galaxy-rail-width`)
  and the chat lane is centered between them (`--galaxy-lane-inset`). The orb
  welcome card is a centered column with a connection chip. No floating clock
  plate, no mottos, no horizontal offsets. Narrow touch views hide logo, clock
  and status rail, keep a 46px icon-only navigation rail, retain the input above
  the controls and use 12px edge gaps. Static contract:
  `TestChatFrontend_GalaxyFrameStaysSymmetric`.
  Keep its styles scoped to `[data-theme="galaxy"]` in `css/chat-themes.css`.
  `galaxy-interface.js` lazily relocates the real composer/drawer controls;
  comment anchors restore the exact original order on theme exit. Do not clone
  actionable controls or change other themes' inline desktop toolbar behavior.
  Live is the existing realtime-speech launcher and retains its dialog and session
  indicators. The Tools popover owns the remaining controls, including Stop.
  Suggestions only prepare a draft and never send automatically. Rebuild the
  welcome card after chat reset.
  Use local Geist, the licensed Lucide control sprite and generated image assets.
  Preserve 44px targets, visible keyboard focus, real connection/persona/mood and
  notification state, and all 16 chat locales. The welcome status mirrors the
  real connection pill. On mobile use a scrollable header, compact left rail,
  input above the five composer controls and an accessible Tools popover.
  Theme exit removes owned nodes/observers/timers; hidden tabs pause the clock.
  No additional render loop. Keep matching full-scene fallback posters current.


- RuView's pinned upstream UI is regenerated through `scripts/ruview/vendor-ui.mjs`;
  never edit vendor outputs without updating that adapter and its provenance hashes.
  Enabled, writable managed runtimes start once with AuraGo; external runtimes
  remain operator-managed. Background health reads update the cached status;
  app opening stays passive. Show unchecked/starting separately from unavailable.
  Manual stop, process exit and uncertain stop must not trigger an automatic restart.
  The Desktop frame is sandboxed with `allow-scripts` only. Its UI responses use
  `connect-src 'none'`. API calls and WebSockets reach the parent through
  `postMessage`; the parent relays reviewed proxy paths plus the status,
  capabilities, and UI `.wasm` reads the opaque frame cannot make, and replies
  with target origin `*`. The frame accepts a reply only from its parent. An iframe
  fetch of any other path, including `/api/config`, must not arrive. The pinned
  UI files and public fonts are readable without the session cookie so the
  opaque frame can load its modules.
  Remember skipped/completed Dashboard tours across iframe, app and page reopening
  using only the boolean `aurago.ruview.onboarding.done` browser preference. The
  opaque frame learns that flag from its name and asks the parent to store it. Keep
  explicit replay available; all other upstream storage stays memory-only and
  clearing it must never clear unrelated AuraGo preferences. Blocked storage falls
  back to memory without breaking the Dashboard.
  Connected services without frames show waiting, never synthetic data. Only explicit
  synthetic sources show simulation; fresh frames recover the live state and expire
  after ten seconds. Source probes and timers must respect connection disposal.
  The pinned Rust pose WebSocket accepts no zone/confidence/FPS query options.
  Strip those in the reproducible client adapter; keep proxy query restrictions
  and Vault authentication unchanged. Browser coverage must open the actual
  Live Demo stream and deliver a pose frame through its renderer callbacks.
  The overview shows artifact verification, the storage budget, UDP scope, the
  last error, why a control is disabled, and that a managed runtime was not
  started in this session. Rendering does not start or probe. Devices show
  a relative measurement age. An omitted measurement time stays blank. A
  retained offline RSSI is the last value for the same receiver only. An idle
  tab waits for the new snapshot before showing board ages. A tab switch during
  a refresh paints the last snapshot and does not start a second request. Those
  passive fields are defined in `internal/ruview/AGENTS.md`.
  Verify `scripts/ruview/upstream.test.mjs`, `scripts/ruview/browser.test.mjs` and
  backend `TestDashboardHealthEndpointsAreReadOnly`; keep `documentation/ruview.md` aligned.
  The shared board wizard uses host theme tokens and native checkbox semantics.
  Label firmware, WLAN/receiver setup and sensor checks as numbered steps; keep
  restart confirmation inside step 2, and show action feedback beside its button.
  Preserve WLAN drafts for the entire open dialog, including acknowledged setup,
  explicit retries, cancellation and readback/sensor checks. Keep them in memory
  only; clear them on dialog disposal, including logout or policy revocation.
  Clear each temporary transfer copy after its request finishes. Readback and
  fresh CSI verification require their preceding acknowledgements. Load saved
  receiver/port defaults through the administrative `board-setup` read, preserve
  typed values and require explicit acceptance of unverified LAN suggestions.
  On each wizard opening, suggest the lowest unused node ID in 1–255 from the
  AuraGo nodes read, including saved offline nodes. An explicit `nodeID` from
  the Devices list is marked edited before that suggestion, so discovery cannot
  replace it. Preserve manually typed IDs;
  the suggestion is not a reservation and cannot account for boards never observed
  or confirmed. After matching readback, register only the node ID with AuraGo;
  report a save failure without claiming that device registration succeeded.
  Keep known boards visible after empty/upstream failure replies and app/runtime
  restarts, separately from fresh CSI. Retained readings cannot cross receivers.
  Failed/malformed reads or exhausted IDs leave the unedited field empty with
  an explanation. Configuration requires a valid ID; manual IDs still allow
  reconfiguring an existing board. Never change IDs on retries or readback.
  Never enable UDP from the wizard. S3 N16R8 boards use the exact 16 MiB profile
  (8 MiB is PSRAM); capacity errors show detected and selected sizes before writes.
  Recognize Espressif CDC USB ID `303a:4001` as a native USB transport. USB IDs
  never establish chip or flash capacity: verify both through the loader before
  erase/write, and keep unknown vendor/product combinations rejected.
  Keep the pinned firmware build, catalog, artifacts and profile validation aligned.
  Keep the wizard, protocol, flasher, serial loader and firmware URLs bound to the
  caller's resource version in Config and Desktop. Each explicit flash attempt
  refreshes the catalog, preserves the selected profile and verifies its revision;
  an open dialog must not retain broken metadata after a server repair. Never
  relax offset validation or automatically retry an uncertain write.
  Flashing reports file checks, connection, optional erase, writing, readback and
  restart as separate phases. Show byte-weighted write progress and a busy indicator
  until verification/reset finish. Only then show completion and an explicit step-2
  navigation action; navigation must not start serial I/O. Failure clears stale success.
  Transient policy/status failures retain the draft, but never bypass the fresh
  permission check before an action; confirmed revocation still closes and clears it.
  Verify `scripts/ruview/wizard-browser.test.mjs`
  in Standard and Fruity light/dark, all locales and narrow/wide layouts.

## Work Guidance

- AI Gateway tests select saved providers and distinguish local routing, non-generating metadata and explicit billable model tests. Dirty drafts block tests. Abort on provider changes; stale responses or section disposal cannot repaint another selection. Keep all sixteen gateway/help/provider locale fragments aligned. Verify `TestAIGatewaySavedProviderDiagnosticsBrowser` and the provider/config roundtrip tests.
- AI Gateway metadata retains incomplete, duplicate and excessive rows as draft row pairs across rendering/navigation; only validated maps may be saved. Removing a row uses its element identity, so renaming a key cannot break removal. Verify `TestAIGatewayMetadataDraftBrowser` in both themes.

- `cfg/treg.js` keeps endpoint grants in the shared draft until Save, with no
  preselected action class. Vault saves update saved token state without losing
  the draft; connection tests require saved settings. Parse USD as integer
  micro-USD, preserving a zero cap. Verify `TestTregConfigBrowser` and all locales.

- Keep the generic Config UI state/action contracts in `ui/js/config/` and
  integration-specific behavior in `ui/cfg/`.
- Schema-rendered integration connection tests use the shared registry in
  `ui/js/config/integration_actions.js`; keep their saved-state/Vault gating,
  inline status region, and all-locale translation coverage intact.
- Preserve lazy section loading and existing REST request shapes.
- Prefer semantic controls, visible focus, inline validation, and live status
  regions. Do not use native `alert()`, `confirm()`, or `prompt()`.
- Use `apply_patch` for edits and keep temporary browser artifacts under
  `disposable/` or ignored `reports/` paths.

- Shared browser libraries are built from `package-lock.json` by
  `scripts/build-browser-vendor.js`. Its read-only `--check` verifies all outputs;
  the manifest records versions and SHA-256 hashes. PDF.js and noVNC load as
  awaited ES modules before their Desktop app scripts.

## Verification

- Desktop apps use the server-owned `/api/desktop/integrations/` entry points
  for shared APIs, including fetch, EventSource and WebSocket URLs. Preserve the
  integration's own gates. `desktop_policy` updates the live shell policy and
  emits `aurago:desktop-policy` without disposing open editors or Tresor drafts.

- Syntax for every rollout JavaScript change:
  `$files = git diff --name-only 0773dfa52e3d21f420f9009c480bdd817e761882 -- '*.js'; foreach ($file in $files) { node --check $file; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE } }`.
- Static contracts: `go test -count=1 ./ui/... -run 'Precision|Config|I18N'`.
- Operational stylesheet integration and cache release keys:
  `go test -count=1 ./ui/... -run 'TestPrecisionOperationalStylesAreIntegratedAndPageScoped|TestPrecisionChangedPageAssetsUseReleaseBuildVersion'`.
- Browser contracts (Chrome or Edge):
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_BROWSER_ARTIFACT_DIR='../disposable/browser-artifacts'; go test -count=1 ./ui/... -run 'Precision.*Browser|Config.*Browser'`.
- Config refresh browser coverage uses the real lazy renderers with local API
  fixtures, every section at five widths in both themes/densities, plus draft,
  failure, search, disclosure and dialog interactions. Run an individual matrix
  section with `-run 'TestConfigRefreshRealSectionsBrowser/matrix/server$'`.
- Full UI: `go test -count=1 ./ui/...`.
- Generated bundles: `npm run build:ui -- --check`.
- UI delivery regressions: `npm run test:ui-regressions`.
- ThreeDee real-model WebGL combat, resource budgets and lifecycle:
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 ./ui -run ThreeDeeCombatBrowserSmoke`.
- Sandstorm WebGL/2D weather, resource bounds and lifecycle:
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 ./ui -run SandstormWeatherBrowserSmoke`.
  With `AURAGO_BROWSER_ARTIFACT_DIR` and `AURAGO_SANDSTORM_RECORD=1`, capture
  96 frames per renderer at 10 FPS for visual motion review.
  `AURAGO_SANDSTORM_BENCHMARK=1` measures 120 native-RAF frames at 1920x1080
  with synchronized WebGL completion for each renderer.
- Sandstorm desert life (worm, storm response, critters, occluder-aware spots):
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 ./ui -run SandstormDesertLife`.
- Galaxy picker, real rendering, responsive controls, drawers, dialogs,
  disposal and failure modes:
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 ./ui -run GalaxyBrowserSmoke`.
  Add `AURAGO_GALAXY_BENCHMARK=1` for the two-minute native-RAF benchmark and
  `AURAGO_BROWSER_ARTIFACT_DIR` for screenshots and measured GPU/FPS data.
- Dashboard knowledge-graph visual (3D constellation, 2D fallback, view switch,
  reduced motion, theme change):
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test -count=1 ./ui -run TestDashboardKnowledgeGraphVisualBrowserSmoke`.
- Full repository: `go test -count=1 ./...`.
- Protected surfaces from the rollout base:
  `git diff --exit-code 0773dfa52e3d21f420f9009c480bdd817e761882 -- ui/index.html ui/desktop.html ui/gallery.html ui/js/shared ui/js/chat ui/js/desktop ui/fonts ui/shared-variables.css ui/shared-utilities.css ui/shared-components.css ui/shared-animations.css`.

## Decisions settings

- LLM router settings expose `decisions_fallback`, passive service/model status
  and an explicit Decisions preview. Local preview never invokes either model
  classifier; preview uses saved settings and no production classification cache.
  Retain the legacy helper API flag, visible external transfer/cost information,
  separate Decisions statistics, and all sixteen router locales. Verify
  `TestLLMRouterConfigBrowser` in both themes at narrow and wide sizes.

- External mode selects a saved OpenRouter connection and a separate decision
  model; catalog loading is explicit and the paid test uses fixed example data.
  Keep transfer/cost information visible. Never expose local lifecycle or CPU
  controls as external provider actions. Dedicated Decisions provider profiles
  are excluded from ordinary provider selectors; preserve invalid saved choices
  visibly until corrected. Cancel stale catalog/test responses on mode, provider
  or section changes. Keep setup errors and the last successful request visible.
- `cfg/decisions.js` uses the shared saved/draft state for activation, deployment,
  and explicit CPU/mixed-offload permission. Its status polling is passive,
  bounded, cancelled on disposal, and cannot repaint another section. Start
  and test require saved settings; stop remains available for containment,
  including while a test is pending. A superseded test response must not replace the stop result.
  Local setup uses the single setup action for runtime/model download and start;
  advanced controls retain the independent check and start. Show setup guidance
  only after a failed setup or prerequisite check. The explicit check
  never runs on page load, uses saved settings, and stale replies cannot unlock
  start after a deployment/backend change. Keep missing-package guidance,
  installed-versus-ready status and a single visible outcome clear; technical
  codes belong in diagnostic details. Verify `TestDecisionsOneClickSetupBrowser`
  and `TestDecisionsLifecycleFeedbackBrowser` at narrow and wide sizes.
  Keep `config.decisions.*` in all sixteen locales aligned and verify
  `TestDecisionsConfigBrowser` and `TestDecisionsLocaleKeys`. The backend and
  hardware acceptance contract lives in `../internal/decisions/AGENTS.md`.

## Passkey and FIDO2 login

- `cfg/auth_webauthn.js` owns protected settings and named credential management;
  its own save must preserve unrelated `AuraConfigState` drafts. Use
  `data-search-path` for Config search, never ordinary `data-path` persistence.
  Enrollment requires saved RP/origin settings. Keep step-up, inline outcomes,
  cancellation and disposal guards; never start biometric prompts on status reads.
  Without enabled login protection and an authenticated owner, show the translated
  prerequisite instead of requesting protected credential metadata. Watch the stable
  `#content` ancestor so removing a nested section aborts requests and prompts.
- Full login and embedded recovery share `internal/server/auth_webauthn_client.js`.
  Require secure context/API capability while preserving external security-key
  support. In second-factor mode without TOTP, unsupported clients must never
  reveal a password-only alternative. Native JSON conversion has a tested legacy
  ArrayBuffer adapter. Do not automatically retry writes or finished assertions;
  distinguish an uncertain network outcome from cancelled authenticator selection.
  Use `AuraAuth.nativeFetch` when available: wrong step-up credentials and expired
  ceremonies stay inline without discarding drafts; genuine session failures still
  reach the shared auth response handler. Keep the browser fixture nested under
  `.cfg-section` and load `shared-core.js` to exercise both integrations.
- Translate `webauthn.*` in all 16 `lang/common/` files and regenerate the small
  recovery subset with `node scripts/build-webauthn-locales.mjs`. Verify its
  `--check`, `node scripts/webauthn-client.test.mjs`, server `TestWebAuthn*` and
  the UI bundle check. Device/proxy acceptance remains separate from virtual
  authenticator browser tests. See `../documentation/webauthn.md`.

## Child DOX Index

- `fonts/qr/AGENTS.md` — Fixed local QR caption/fallback fonts, source revisions,
  integrity manifests and OFL notices; runtime schema/rendering belongs to
  `../internal/qrcode/AGENTS.md`.

- `3d/screensaver/AGENTS.md` — Compact Tiefsee screensaver creature GLBs and their shader-facing UV contract.
- `3d/chess/AGENTS.md` — Versioned AuraGo chess piece GLBs and their manifest contract for the Desktop 3D board.
- `3d/system-world/AGENTS.md` — Compact versioned Blender GLBs and asset integrity/LOD contracts.

- `img/personas/animated/AGENTS.md` - Reviewed Rive persona payloads, catalog
  mapping and the paired local runtime under `js/vendor/rive/`; authoring lives
  in the separate sibling `personas` project.
- `js/desktop/apps/AGENTS.md` - Built-in Virtual Desktop application modules
  and their lifecycle, asset, and app-specific contracts.
- `js/desktop/screensavers/AGENTS.md` - Screensaver overlay host, scene
  lifecycle, idle/wake rules and verification.
