# Web UI - Child DOX Contract

## Purpose

This subtree owns AuraGo's embedded HTML, CSS, JavaScript, translations, fonts,
images, and browser-oriented regression tests.

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

## Local Contracts

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
  surface and interaction effects are defined below. Entry-page additions remain scoped with
  `data-entry-page`.
- `window.AuraPrecisionWorkspace` owns the browser-local
  `aurago.workspace.density.v1` preference and exposes `init()`,
  `getDensity()`, and `setDensity("comfortable"|"compact")`. It migrates the
  legacy `aurago.config.density.v1` key once; Config must not access either key
  directly.
- Configuration connection tests operate only on saved configuration. Dirty,
  incomplete, or credential-missing sections expose a visible locked reason.
- Config owns its blue/slate palette and form presentation in `config-workspace.css`
  and `js/config/presentation.js`, scoped to `data-workspace-page="config"`.
  Keep Geist, 16px inputs and 44px controls in both densities. Below 1100px the
  labeled sidebar becomes a keyboard-accessible drawer; the save dock stays in
  the viewport layout without covering the scrollable form.
- Config uses one visible card level: named topic cards containing flat fields,
  with a compact variant for independent objects. Reuse `AuraConfigForm` and the shared presentation pass for lazy
  integration renderers. Preserve their data bindings, independent provider /
  credential save paths and native checkbox semantics.
- Config topic boundaries are explicit renderer headings/groups or exact selectors
  in `AuraConfigCatalog.presentation`; never infer cards from nested field wrappers.
  Card headings use 18px text, fields 16px, help 14px, with 24px spacing (16px
  compact/mobile). Switches sit beside their label and help. Keep blue/slate
  surfaces, softly tinted blue/cyan card heads and subtle blue-tinted shadows.
  Use 16px card corners, the `--cfg-card-heading` tint and `--surface-shadow`
  (0 6px 20px, 16% dark / 7% light); dialogs use `--surface-shadow-strong`.
  Interactive entry cards may lift at most 2px; switches/disclosures use 140–180ms
  feedback and save success may animate once. Reduced motion disables these effects.
  The overview has no save dock; independent saves identify their scope.
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
- Configuration density is a browser-local presentation preference and never
  belongs in `config.yaml`.
- Every visible UI string must use translations in all supported locales.
- Chat loads typed notifications from `/api/system/notifications`, renders `morning_briefing` separately from generic notices, and acknowledges only displayed IDs through `/api/system/notifications/read`. The legacy string endpoints remain server-compatible but are not the primary Chat UI path; generic notices must never be labeled as morning or system briefings.
- Chat tool icons normally use the fixed 10x10 PNG sprite. A provider that needs a distinct icon after those cells are allocated may declare one embedded transparent custom asset in `tool-icons.js`; `applyIcon` must add the build-version cache key, clear custom inline background state when an element returns to a sprite icon, and UI regression tests must verify the asset is embedded.
- The Dashboard operational-issues view uses the sanitized admin API only, renders dynamic issue data with `textContent`, and requires an inline confirmation before archival or resolution. It must never decode or display internal fingerprints, raw logs, or unredacted error text.
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
- Full repository: `go test -count=1 ./...`.
- Protected surfaces from the rollout base:
  `git diff --exit-code 0773dfa52e3d21f420f9009c480bdd817e761882 -- ui/index.html ui/desktop.html ui/gallery.html ui/js/shared ui/js/chat ui/js/desktop ui/fonts ui/shared-variables.css ui/shared-utilities.css ui/shared-components.css ui/shared-animations.css`.

## Child DOX Index

- `js/desktop/apps/AGENTS.md` - Built-in Virtual Desktop application modules
  and their lifecycle, asset, and app-specific contracts.
