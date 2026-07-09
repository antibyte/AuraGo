# Web UI - Child DOX Contract

## Purpose

This subtree owns AuraGo's embedded HTML, CSS, JavaScript, translations, fonts,
images, and browser-oriented regression tests.

## Ownership

- Precision Workspace is an opt-in design system for operational web pages.
- `/config` is its first consumer and activates it with `.pw-page` plus the
  dedicated Precision Workspace and Config Workspace assets.
- Web Chat (`index.html`) and Virtual Desktop (`desktop.html`) retain their own
  established visual systems and are not Precision Workspace consumers.

## Local Contracts

- Never load Precision Workspace assets from `index.html` or `desktop.html`.
- Do not change Chat, Virtual Desktop, or an asset they share as a side effect
  of Precision Workspace work. This includes `shared-variables.css`,
  `shared-utilities.css`, `shared-components.css`, `shared-animations.css`,
  `fonts/fonts.css`, `js/shared/`, Chat bundles, and Desktop bundles/modules.
- Precision Workspace CSS must remain scoped under `.pw-page`; no unscoped
  reset, token, component, or motion rule may leak to another page.
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

- `node --check ui/js/config/main.js`
- `node --check ui/js/config/state.js`
- `node --check ui/js/config/actions.js`
- `go test ./ui/... -run 'Config|I18N'`
- `go test ./ui/...`
- Compare protected-surface hashes and confirm their diff is empty.

## Child DOX Index

- `js/desktop/apps/AGENTS.md` - Built-in Virtual Desktop application modules
  and their lifecycle, asset, and app-specific contracts.
