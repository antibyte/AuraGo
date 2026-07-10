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
- Configuration density is a browser-local presentation preference and never
  belongs in `config.yaml`.
- Every visible UI string must use translations in all supported locales.

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
- Full repository: `go test -count=1 ./...`.
- Protected surfaces from the rollout base:
  `git diff --exit-code 0773dfa52e3d21f420f9009c480bdd817e761882 -- ui/index.html ui/desktop.html ui/gallery.html ui/js/shared ui/js/chat ui/js/desktop ui/fonts ui/shared-variables.css ui/shared-utilities.css ui/shared-components.css ui/shared-animations.css`.

## Child DOX Index

- `js/desktop/apps/AGENTS.md` - Built-in Virtual Desktop application modules
  and their lifecycle, asset, and app-specific contracts.
