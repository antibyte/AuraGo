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
  superseded legacy surface tokens, gradients, glassmorphism, glows, shadows,
  or decorative animations. Entry-page additions remain scoped with
  `data-entry-page`.
- `window.AuraPrecisionWorkspace` owns the browser-local
  `aurago.workspace.density.v1` preference and exposes `init()`,
  `getDensity()`, and `setDensity("comfortable"|"compact")`. It migrates the
  legacy `aurago.config.density.v1` key once; Config must not access either key
  directly.
- Configuration connection tests operate only on saved configuration. Dirty,
  incomplete, or credential-missing sections expose a visible locked reason.
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
- Realtime Speech consumes the answer from `final_response`; `done` is a
  contentless terminator. SIP Phone surfaces
  `outbound_policy_migration_required` as a localized setup blocker.
- Configuration density is a browser-local presentation preference and never
  belongs in `config.yaml`.
- Every visible UI string must use translations in all supported locales.
- The Dashboard operational-issues view uses the sanitized admin API only, renders dynamic issue data with `textContent`, and requires an inline confirmation before archival or resolution. It must never decode or display internal fingerprints, raw logs, or unredacted error text.
- The Dashboard personality card shows affect valence/arousal, recent sanitized affect events, and lived notes. Affect cause codes and sources are localized; chat-sourced event details never include the raw user message. Config Personality and Prompts warn when V2 mood, emotion synthesis, or inner voice need the Helper LLM; Prompts lists live notes as read-only and points management to the Dashboard. Note and event text use `textContent`.
- Skill-card list fields must render in deterministic sorted order matching
  their DOM-diff snapshots so API-only reordering cannot leave stale cards.
- The service worker caches only same-origin static assets, retains full
  versioned request URLs, and keeps HTML, API, event, and auth traffic network-only.
- CanvasUI components are vendored as local framework-free ESM under
  `ui/js/vendor/canvasui/` with committed `manifest.json`, `LICENSE.txt`, and
  pinned upstream provenance. They must not load remote assets at runtime and
  must not introduce React solely for effects. Login uses Flame Wrap on the
  auth card. Droplets are desktop-only for the `city_rain` wallpaper via
  `ui/js/desktop/city-rain-droplets.js`, painted on `#vd-wallpaper-fx`
  (`z-index: -1`) behind icons/widgets/windows, with graceful WebGL2 /
  reduced-motion fallbacks and no HTML capture of foreground UI.
- `scripts/build-ui-bundles.js` is the source of truth for generated Chat and
  Desktop bundles; `npm run build:ui -- --check` must be read-only and pass.

## Work Guidance

- Keep the generic Config UI state/action contracts in `ui/js/config/` and
  integration-specific behavior in `ui/cfg/`.
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
  `$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_BROWSER_ARTIFACT_DIR='disposable/browser-artifacts'; go test -count=1 ./ui/... -run 'Precision.*Browser|ConfigPrecisionWorkspaceBrowserMatrix'`.
- Full UI: `go test -count=1 ./ui/...`.
- Generated bundles: `npm run build:ui -- --check`.
- UI delivery regressions: `npm run test:ui-regressions`.
- Full repository: `go test -count=1 ./...`.
- Protected surfaces from the rollout base:
  `git diff --exit-code 0773dfa52e3d21f420f9009c480bdd817e761882 -- ui/index.html ui/desktop.html ui/gallery.html ui/js/shared ui/js/chat ui/js/desktop ui/fonts ui/shared-variables.css ui/shared-utilities.css ui/shared-components.css ui/shared-animations.css`.

## Child DOX Index

- `js/desktop/apps/AGENTS.md` - Built-in Virtual Desktop application modules
  and their lifecycle, asset, and app-specific contracts.
