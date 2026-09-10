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

## Ownership

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

## Local Contracts

- Tabellen uses the local Univer OSS 0.25.1 resource set under `js/vendor/sheets/`.
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
  from frames or client time. Preserve click-through foliage, the shell escape
  control, reduced motion, context-loss fallback, and the 16-locale care UI.
  Server state and revisioned actions live in `internal/desktop/plant_*.go`.
  Healthy uncapped stems add a node each server hour. Scheduled snapshots must
  redraw this growth even when no care action changes the revision.
  See `documentation/leafy.md` for care, atlas baking and browser checks.
- The opt-in `builtin-printer` widget lives in `js/desktop/core/widget-printer-runtime.js`.
  Keep missing printer metrics unknown, render filenames as text, and stop status
  requests/camera streams on document hiding or widget disposal. Its native dialog
  moves the existing camera image rather than opening a second stream. Verify with
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopPrinterWidgetBrowser`.

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
  (`.kc-tab`, `.sk-tab`, `.media-tab`, `.dash-tab`, `.invasion-tab`,
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
  requests preserve successful results when a sibling request fails.
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
  contentless terminator. SIP Phone surfaces
  `outbound_policy_migration_required` as a localized setup blocker.
- Live Speech's shared panel owns one `AuraRealtimeSpeechAvatar` per mount.
  Webchat passes `visible: false` until its overlay opens and calls
  `AuraRealtimeSpeechUI.setVisible`; unmount disposes the avatar. Desktop
  visibility also follows intersection/tab state. Animation never starts audio.
  Catalog membership selects local Rive assets; initial personality resolution
  must also work before Desktop Agent Chat opens. Preserve custom PNG fallback.
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
- MeshCore Config uses the shared saved/draft path, explicit device/channel binding confirmation, full-key trust lists and separate reply/proactive permissions. Its inbox and discovered device names render external text with `textContent`; pairing PINs are transient. Discovery shows a live spinner and selectable results; selecting a device updates only the draft Bluetooth address/transport, never trust or pairing automatically. Device tests and pairing use saved settings, bounded fetches and inline status. Keep hardware acceptance visibly unverified and all locale files under `lang/config/meshcore/` complete.
- Chat loads typed notifications from `/api/system/notifications`, renders `morning_briefing` separately from generic notices, and acknowledges only displayed IDs through `/api/system/notifications/read`. The legacy string endpoints remain server-compatible but are not the primary Chat UI path; generic notices must never be labeled as morning or system briefings.
- Chat tool icons normally use the fixed 10x10 PNG sprite. A provider that needs a distinct icon after those cells are allocated may declare one embedded transparent custom asset in `tool-icons.js`; `applyIcon` must add the build-version cache key, clear custom inline background state when an element returns to a sprite icon, and UI regression tests must verify the asset is embedded.
- The Dashboard operational-issues view uses the sanitized admin API only, renders dynamic issue data with `textContent`, and requires an inline confirmation before archival or resolution. It must never decode or display internal fingerprints, raw logs, or unredacted error text.
- The Dashboard knowledge-graph visual (`js/dashboard/widgets-knowledge.js`) ships two renderers: a 3D WebGL constellation (`ui/js/vendor/3d-force-graph.min.js`, default) and an enhanced 2D canvas (`force-graph.min.js`), switched by the 2D|3D toggle persisted in `aurago.dashboard.kgview.v1` with automatic 2D fallback when WebGL or the vendor bundle is unavailable. `3d-force-graph` is pinned to **1.70.2** because that release targets three.js `^0.128.0`, matching the vendored r128 `three.min.js` loaded before it (the bundle prefers `window.THREE`); never upgrade either file independently, and note `controlType` is a constructor option there, not a chainable setter. Node clicks keep opening the KG detail modal; effects (glow, particles, pulse, starfields, auto-rotation) live inside the canvas only, honor `prefers-reduced-motion`, pause on hidden tabs, and rebuild on `aurago:themechange`. Renderer instances, ResizeObserver, visibility listener, and 3D geometries are disposed through `destroyKnowledgeGraphVisual` on every mode switch and empty state.
- The Dashboard personality card shows affect valence/arousal, recent sanitized affect events, and lived notes. Affect cause codes and sources are localized; chat-sourced event details never include the raw user message. Config Personality and Prompts warn when V2 mood, emotion synthesis, or inner voice need the Helper LLM; Prompts lists live notes as read-only and points management to the Dashboard. Note and event text use `textContent`.
- Skill-card list fields must render in deterministic sorted order matching
  their DOM-diff snapshots so API-only reordering cannot leave stale cards.
- The service worker caches only same-origin static assets, retains full
  versioned request URLs, and keeps HTML, API, event, and auth traffic network-only.
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
- `scripts/build-ui-bundles.js` is the source of truth for generated Chat and
  Desktop bundles; `npm run build:ui -- --check` must be read-only and pass.
- ThreeDee combat uses a fixed four-light impact pool, at most 240 sprites
  (40 slots reserved for smoke/debris), and six unlit, shadow-free blink ghosts.
  Continuous particle emission follows simulation time; cinematic slow motion
  expires on wall time, and every blink entry point enforces its cooldown.
  Helix volleys and damage-triggered EMP counterpulses reuse projectile/effect
  cleanup and respect the existing 18-projectile limit. EMP must not interrupt
  a paired nova clash. Keep reduced-motion and theme-exit disposal intact.
- Sandstorm dust, grains and ground lift share a smooth wind/gust envelope.
  Three moving counter-rotating eddies drive the fog and particle velocity
  field; grains must visibly turn, rise and recirculate. Wind changes direction
  gradually. Soft dust rolls preserve visible circulation in the 2D fallback.
  Keep the fixed particle pools and the fog buffer at most 960x540 pixels;
  soft dust does not need device-pixel resolution. Canvas bounds must not
  transition. Preserve the 2D fallback, hidden-tab pause and reduced-motion
  and narrow-screen gates.
- Galaxy uses the existing Three.js r128 and a single lazy renderer/RAF loop.
  Keep the ten draw calls, shared sphere geometry and fixed 3500/850-star
  buffers. Exactly 20 stars flicker subtly with individually randomized pauses;
  four distinct low-poly ships (saucer cruiser, cargo freighter, ring explorer,
  shuttle) each bake their lit hull, bridge and engines into one mesh. Keep
  diagonal courses visible at every aspect ratio and wrap fully offscreen.
  Both effects use scene time and the existing pause/disposal lifecycle.
  No full-screen postprocessing. Start desktop at high quality with
  at most 1.5 DPR / 3840x2160 pixels, then lower resolution after sustained
  slow frames. Calibrate the idle display cadence during the first-frame fade;
  30/32 Hz displays must not trigger quality reduction. Mobile starts with 2K
  maps and the simpler atmosphere.
  Pause hidden tabs; release all GPU resources on exit and reject stale loads.
  Reduced motion, unavailable WebGL, missing textures and context loss expose
  the complete local poster. Keep posters aligned with the rendered scene and
  preserve source provenance in `img/galaxy/CREDITS.md`.
  Galaxy chat follows the supplied orbital-glass reference: a violet/cyan/gold
  outlined header, local orbit wordmark, left navigation rail, orb welcome card
  and a floating composer ordered Voice, Live, File, Tools, input, Send.
  Desktop header/composer share width and resting height, with 16px edge gaps;
  narrow touch views retain the input above the controls and 12px edge gaps.
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


## Work Guidance

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

## Verification

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

## Child DOX Index

- `img/personas/animated/AGENTS.md` - Reviewed Rive persona payloads, catalog
  mapping and the paired local runtime under `js/vendor/rive/`; authoring lives
  in the separate sibling `personas` project.
- `js/desktop/apps/AGENTS.md` - Built-in Virtual Desktop application modules
  and their lifecycle, asset, and app-specific contracts.
