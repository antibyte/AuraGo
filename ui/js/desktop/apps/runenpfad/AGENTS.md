# Runenpfad — Virtual Desktop 2D Action-Platformer

## Purpose

Offline 2D action-platformer game (`runenpfad`, category `games`) for the AuraGo Virtual Desktop with a 25-level campaign across five biomes, multi-phase world bosses, two playable heroes, persistent Rune Workshop upgrades, per-level Rune Trials, and an Endless Rune Tower mode.

## Ownership

- `ui/js/desktop/apps/runenpfad/levels.js` — Biome definitions, hero/enemy/upgrade tables, sprite manifest validator (`validateSpriteManifests`), 25-level campaign stages, zone archetypes, and seeded endless generator.
- `ui/js/desktop/apps/runenpfad/sim.js` — Pure `60 Hz` (`1/60 s`) simulation, physics, Knight/Ranger combat, boss AI, collectibles, checkpoints, star scoring, and the schema-clamped `localStorage` persistence re-export.
- `ui/js/desktop/apps/runenpfad/profile.js` — Save schema (`aurago.runenpfad.v1`), clamping of `unlockedLevel`, `endlessBestFloor`, `coins`, `totalScore`, `upgrades`, `starsByLevel`, `bestTimeByLevel`, plus workshop costs and hero stats.
- `ui/js/desktop/apps/runenpfad/behaviors.js` — Enemy behaviour profiles (`patrol`, `charger`, `jumper`, `flyer`, `turret`, `ambusher`, `bomber`) driven by the tile helpers the simulation passes in.
- `ui/js/desktop/apps/runenpfad/variety.js` — Rune Trials (per-level modifiers) and the shrine blessing state.
- `ui/js/desktop/apps/runenpfad/fx.js` — Deterministic particle and screen-effect system (particles, shake, flash, impact freeze).
- `ui/js/desktop/apps/runenpfad/biomes.js` — Per-world presentation model: sky, parallax layers, light, weather, colour grade, ambience beds.
- `ui/js/desktop/apps/runenpfad/atmosphere.js` — Canvas painter for backdrops, weather, the 2D light map, colour grade and vignette.
- `ui/js/desktop/apps/runenpfad/audio.js` — Gesture-unlocked Web Audio controller with a cue table, pitch variation, master limiter and layered ambience, plus `aurago-sounds` manifest validator (`validateSoundManifest`).
- `ui/js/desktop/apps/runenpfad/view.js` — Canvas 2D renderer using real `sheet.json` frame coordinates, biome backdrops, lighting, glow and particle drawing.
- `ui/js/desktop/apps/runenpfad/app.js` — Window shell integration (`RunenpfadApp`), HUD, dialogs, keyboard/touch input, resize/visibility handling, and disposal.
- `ui/css/desktop-app-runenpfad.css`, `ui/img/runenpfad/icon.svg`, `ui/lang/runenpfad/*.json` (16 locales), and `ui/desktop_runenpfad_test.go`.

## Local Contracts

- **Game Maker asset reuse**: Reads `/api/game-maker/asset-packs/<pack>/...` for `jump-and-run`, `human-characters-animated`, `monsters-animals-animated`, `robots-drones-animated`, `assets-animated`, `nature-assets`, and `aurago-sounds`. Never copy PNGs, WAVs, or manifests into `ui/`.
- **DOM-free core**: `levels.js`, `sim.js`, `profile.js`, `behaviors.js`, `variety.js`, `fx.js`, `biomes.js` and `audio.js` carry zero DOM references so `scripts/test-runenpfad-sim.mjs` executes them directly under Node `vm`. `atmosphere.js` and `view.js` are the only canvas-facing modules.
- **Safe UI & styling**: `app.js` builds DOM via `document.createElement` and `textContent` only (no `.innerHTML`, no external URLs). `desktop-app-runenpfad.css` is listed in `desktopAppMaterialsExempt` (`ui/desktop_app_materials_test.go`), scopes classes to `.rp-*` (never `cc-` or `ad-`), and avoids `!important` and CSS `animation:`.
- **Save integrity**: `importProfile` validates `v === 1` and clamps `unlockedLevel`, `endlessBestFloor`, `coins`, `totalScore`, `upgrades`, `starsByLevel`, and `bestTimeByLevel`; invalid saves fall back cleanly to defaults.
- **Registration & bundles**: Registered in `internal/desktop/types.go` (`games`, `open_maximized: "true"`, `logo_path: "/img/runenpfad/icon.svg"`), `desktop-foundation.js`, `module-loader.js`, `menus-and-routing.js`, and `window-shell-runtime.js`; bundled into `ui/js/desktop/bundles/main.bundle.js`.
- **Module load order**: `module-loader.js` must list `biomes.js` → `fx.js` → `behaviors.js` → `variety.js` → `profile.js` → `levels.js` → `sim.js` → `audio.js` → `atmosphere.js` → `view.js` → `app.js`, because each module reads its dependencies from the shared global scope at call time.

## Work Guidance

- Keep each first-party JS file under `ui/js/desktop/` below `1100` lines with semantic filenames (`TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`, `TestVirtualDesktopJSUsesSemanticChunkNames`). Split new subsystems into their own module rather than growing `sim.js` or `levels.js`.
- Sound is enabled by default (`profile.sound`), unlocks Web Audio on the first user gesture, and can be toggled in either the top HUD bar or the pause menu; suspend/pause audio whenever the window becomes inactive, minimized, hidden, or a modal is open.
- The impact freeze (`fx.hitstop`) freezes the world but must keep resolving player input (attack, heal) and pickups, otherwise gameplay and the existing simulation tests break.
- Preserve test names `TestRunenpfadRegistration` and `TestDesktopRunenpfadBrowser` so CI browser shards (`TestUIBrowserCIShardsCoverEveryTest`) cover both cleanly without modifying workflow selectors.
- Maintain genuine translations for all `runenpfad.*` keys and `desktop.app_runenpfad` across all 16 locales.

## Verification

- Simulation & real pack manifest checks: `node scripts/test-runenpfad-sim.mjs`
- UI registration, materials, line budget, CSS safety & i18n checks: `go test ./ui -run 'TestRunenpfadRegistration|TestDesktopAppMaterials|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestVirtualDesktopJSUsesSemanticChunkNames|TestUIAvoidsCookieConsentClassPrefix|TestAuraDriftAvoidsAdBlockerClassPrefix|TestTranslations_|TestFrontend_' -count=1`
- Real Chrome desktop smoke test: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopRunenpfadBrowser' -count=1 -v`

## Child DOX Index

None.
