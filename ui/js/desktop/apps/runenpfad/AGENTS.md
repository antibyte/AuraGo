# Runenpfad — Virtual Desktop 2D Action-Platformer

## Purpose

Offline 2D action-platformer game (`runenpfad`, category `games`) for the AuraGo Virtual Desktop with a 25-level campaign across five biomes, multi-phase world bosses, two playable heroes, persistent Rune Workshop upgrades, and an Endless Rune Tower mode.

## Ownership

- `ui/js/desktop/apps/runenpfad/levels.js` — Biome definitions, hero/enemy/upgrade tables, sprite manifest validator (`validateSpriteManifests`), 25-level campaign stages, and seeded endless generator.
- `ui/js/desktop/apps/runenpfad/sim.js` — Pure `60 Hz` (`1/60 s`) simulation, physics, Knight/Ranger combat, boss AI, collectibles, checkpoints, star scoring, and schema-clamped `localStorage` persistence (`aurago.runenpfad.v1`).
- `ui/js/desktop/apps/runenpfad/audio.js` — Gesture-unlocked Web Audio controller and `aurago-sounds` manifest validator (`validateSoundManifest`).
- `ui/js/desktop/apps/runenpfad/view.js` — Canvas 2D renderer using real `sheet.json` frame coordinates and parallax biome backdrops.
- `ui/js/desktop/apps/runenpfad/app.js` — Window shell integration (`RunenpfadApp`), HUD, dialogs, keyboard/touch input, resize/visibility handling, and disposal.
- `ui/css/desktop-app-runenpfad.css`, `ui/img/runenpfad/icon.svg`, `ui/lang/runenpfad/*.json` (16 locales), and `ui/desktop_runenpfad_test.go`.

## Local Contracts

- **Game Maker asset reuse**: Reads `/api/game-maker/asset-packs/<pack>/...` for `jump-and-run`, `human-characters-animated`, `monsters-animals-animated`, `robots-drones-animated`, `assets-animated`, `nature-assets`, and `aurago-sounds`. Never copy PNGs, WAVs, or manifests into `ui/`.
- **DOM-free core**: `levels.js` and `sim.js` carry zero DOM references so `scripts/test-runenpfad-sim.mjs` executes them directly under Node `vm`.
- **Safe UI & styling**: `app.js` builds DOM via `document.createElement` and `textContent` only (no `.innerHTML`, no external URLs). `desktop-app-runenpfad.css` is listed in `desktopAppMaterialsExempt` (`ui/desktop_app_materials_test.go`), scopes classes to `.rp-*` (never `cc-` or `ad-`), and avoids `!important` and CSS `animation:`.
- **Save integrity**: `importProfile` validates `v === 1` and clamps `unlockedLevel`, `endlessBestFloor`, `coins`, `totalScore`, `upgrades`, `starsByLevel`, and `bestTimeByLevel`; invalid saves fall back cleanly to defaults.
- **Registration & bundles**: Registered in `internal/desktop/types.go` (`games`, `open_maximized: "true"`, `logo_path: "/img/runenpfad/icon.svg"`), `desktop-foundation.js`, `module-loader.js`, `menus-and-routing.js`, and `window-shell-runtime.js`; bundled into `ui/js/desktop/bundles/main.bundle.js`.

## Work Guidance

- Keep each first-party JS file under `ui/js/desktop/` below `1100` lines with semantic filenames (`TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`, `TestVirtualDesktopJSUsesSemanticChunkNames`).
- Sound is enabled by default (`profile.sound`), unlocks Web Audio on the first user gesture, and can be toggled in either the top HUD bar or the pause menu; suspend/pause audio whenever the window becomes inactive, minimized, hidden, or a modal is open.
- Preserve test names `TestRunenpfadRegistration` and `TestDesktopRunenpfadBrowser` so CI browser shards (`TestUIBrowserCIShardsCoverEveryTest`) cover both cleanly without modifying workflow selectors.
- Maintain genuine translations for all 99 `runenpfad.*` keys and `desktop.app_runenpfad` across all 16 locales.

## Verification

- Simulation & real pack manifest checks: `node scripts/test-runenpfad-sim.mjs`
- UI registration, materials, line budget, CSS safety & i18n checks: `go test ./ui -run 'TestRunenpfadRegistration|TestDesktopAppMaterials|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestVirtualDesktopJSUsesSemanticChunkNames|TestUIAvoidsCookieConsentClassPrefix|TestAuraDriftAvoidsAdBlockerClassPrefix|TestTranslations_|TestFrontend_' -count=1`
- Real Chrome desktop smoke test: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopRunenpfadBrowser' -count=1 -v`

## Child DOX Index

None.
