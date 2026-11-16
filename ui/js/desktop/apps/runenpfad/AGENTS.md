# Runenpfad — Virtual Desktop 2D Action-Platformer

## Purpose

Offline 2D action-platformer game (`runenpfad`, category `games`) for the AuraGo Virtual Desktop with a 25-level campaign across five biomes, multi-phase world bosses with telegraphed attack patterns, two playable heroes, persistent Rune Workshop upgrades, per-level Rune Trials, an Endless Rune Tower mode, parallax biome backdrops, adaptive procedural music and an optional WebGL2 post-processing stage.

## Ownership

- `ui/js/desktop/apps/runenpfad/levels.js` — Biome, hero, enemy and upgrade tables, sprite manifest validator (`validateSpriteManifests`), the 25-level campaign stages and the seeded endless generator.
- `ui/js/desktop/apps/runenpfad/sim.js` — Pure `60 Hz` (`1/60 s`) simulation loop and stage state; delegates to `physics`, `combat`, `bosses` and `zones`. Also owns collectibles, checkpoints, star scoring and the persistence re-export.
- `ui/js/desktop/apps/runenpfad/physics.js` — Tile collision, the event queue (`pushEvent`), fx wrappers and the seeded gameplay RNG (`Physics.rand` over `sim.rngState`).
- `ui/js/desktop/apps/runenpfad/combat.js` — Player and enemy damage, hurt and invulnerability, crates, hero attacks, projectiles, the kill combo and timed temporary hazards.
- `ui/js/desktop/apps/runenpfad/bosses.js` — World boss AI: intro, telegraphed attack state machine, per-pattern telegraphs and the phase 2 escalation.
- `ui/js/desktop/apps/runenpfad/zones.js` — Variety zone builders: `bounce_grove`, `updraft_shaft`, `conveyor_works`, `pendulum_hall`, `secret_vault`, `lantern_path`, `rope_chasm`, plus their per-tick steps, forces and interactions.
- `ui/js/desktop/apps/runenpfad/behaviors.js` — Enemy behaviour profiles (`patrol`, `charger`, `jumper`, `flyer`, `turret`, `ambusher`, `bomber`) driven by the tile helpers the simulation passes in.
- `ui/js/desktop/apps/runenpfad/variety.js` — Rune Trials (per-level modifiers) and the shrine blessing state.
- `ui/js/desktop/apps/runenpfad/profile.js` — Save schema (`aurago.runenpfad.v1`), clamping, `music` and `gfx` preferences, workshop costs and hero stats.
- `ui/js/desktop/apps/runenpfad/fx.js` — Deterministic particle and screen-effect system (particles, shake, flash, impact freeze). Owns its own visual RNG.
- `ui/js/desktop/apps/runenpfad/biomes.js` — Per-biome presentation data: sky, parallax layer specs, light, weather, colour grade and ambience beds.
- `ui/js/desktop/apps/runenpfad/audio.js` — Gesture-unlocked Web Audio controller: cue table, sample and synth layers, buses, per-biome reverb, master limiter and `validateSoundManifest` for `aurago-sounds`.
- `ui/js/desktop/apps/runenpfad/music.js` — Procedural adaptive music: per-biome themes and boss themes, seeded composition and the synthesis hooks.
- `ui/js/desktop/apps/runenpfad/backdrops.js` — Backdrop painters, part 1: shared noise helpers, woodland, cavern and cave-ceiling silhouettes.
- `ui/js/desktop/apps/runenpfad/backdrops-biomes.js` — Backdrop painters, part 2: desert, glacier and citadel silhouettes and peak decoration.
- `ui/js/desktop/apps/runenpfad/atmosphere.js` — Canvas painter for cached parallax strips, clouds, weather, the 2D light map, colour grade and vignette.
- `ui/js/desktop/apps/runenpfad/decor.js` — Canvas decor layer: torches, lanterns, ledge caps, floaters, grass sway, lava shimmer and ripples.
- `ui/js/desktop/apps/runenpfad/actors.js` — Hero, enemy and projectile drawing with squash and stretch, afterimages, hit flash, dissolve and the climb clips.
- `ui/js/desktop/apps/runenpfad/stage-art.js` — Drawing of zone objects, boss telegraphs and timed hazards.
- `ui/js/desktop/apps/runenpfad/postfx.js` — Plain WebGL2 compositor (bloom, god rays, shockwaves, heat haze, grading, vignette, grain) with a canvas fallback.
- `ui/js/desktop/apps/runenpfad/view.js` — Canvas 2D renderer that composes the layers above, reads simulation state and feeds the post-processing stage.
- `ui/js/desktop/apps/runenpfad/app.js` — Window shell (`RunenpfadApp`), HUD, dialogs, keyboard, touch and pointer input, resize and visibility handling, and disposal.
- `ui/css/desktop-app-runenpfad.css`, `ui/img/runenpfad/icon.svg`, `ui/lang/runenpfad/*.json` (16 locales), and `ui/desktop_runenpfad_test.go`.

## Local Contracts

- **Game Maker asset reuse**: Reads `/api/game-maker/asset-packs/<pack>/...` for `jump-and-run`, `human-characters-animated`, `monsters-animals-animated`, `robots-drones-animated`, `assets-animated`, `nature-assets`, and `aurago-sounds`. Never copy PNGs, WAVs, or manifests into `ui/`.
- **DOM-free core**: `levels.js`, `sim.js`, `profile.js`, `behaviors.js`, `variety.js`, `fx.js`, `biomes.js`, `audio.js`, `physics.js`, `combat.js`, `bosses.js`, `zones.js` and `music.js` carry zero DOM references so `scripts/test-runenpfad-sim.mjs` executes them directly under Node `vm`. Do not add `document`, `window` or canvas use to them.
- **Canvas-facing modules**: `backdrops.js`, `backdrops-biomes.js`, `atmosphere.js`, `decor.js`, `actors.js`, `stage-art.js`, `postfx.js` and `view.js`. `app.js` is the only module that builds DOM.
- **Deterministic gameplay randomness**: No `Math.random` and no `Date.now` in the DOM-free gameplay and audio modules. Gameplay decisions use only `Physics.rand` on `sim.rngState`. Never use `fx.rng` for gameplay.
- **Read-only renderer**: The canvas-facing modules never mutate simulation state. Visual effects are driven by events and read-only state.
- **Event vocabulary**: Simulation events are emitted through `Physics.pushEvent`. Besides the existing set, the new events are `pickup_star`, `pickup_treasure`, `hazard`, `door_open`, `enemy_attack`, `spike_rise`, `player_fall`, `boss_intro`, `boss_telegraph`, `boss_attack`, `boss_slam`, `combo`, `bounce`, `lantern_lit` and `secret_break`.
- **Post-processing never breaks the game**: `postfx` must never throw and never leave a rejected promise. Every failure calls `onFallback` once and falls back to the 2D canvas. `app.js` then sets `data-rp-gfx` (`gpu` or `canvas`) on the shell, and `data-rp-gfx-reason` records the fallback reason. The 2D `.rp-canvas` always stays in the DOM because the browser test reads its pixels.
- **Pause dialog order**: `resume`, `restart`, `sound`, `music`, `gfx`, `reset`. The browser test clicks the first `aria-pressed` button, so keep `resume` first and keep the toggles in this order. `gfx` is hidden while no GPU stage is attached and the preference is `auto`.
- **Save integrity**: `importProfile` validates `v === 1` and clamps `unlockedLevel`, `endlessBestFloor`, `coins`, `totalScore`, `upgrades`, `starsByLevel`, `bestTimeByLevel`, and the `music` (boolean, default on) and `gfx` (`auto` or `canvas`) preferences. Invalid saves fall back cleanly to defaults. The save format stays `aurago.runenpfad.v1`.
- **Climb clips**: Ladder and rope climbing use `<hero>_climb_ladder` and `<hero>_climb_rope`. These are rear-view clips and are never flipped. The clock runs only while moving, reverses when descending, and falls back to the `<hero>_idle` clip when a clip is missing.
- **No third-party runtime**: PixiJS and Phaser are deliberately not used. The GPU stage is a full-screen post-process, and raw WebGL2 avoids an 800 KB vendor bundle and a CSP `unsafe-eval` shim. Do not add a vendor game library.
- **Safe UI & styling**: `app.js` builds DOM via `document.createElement` and `textContent` only (no `.innerHTML`, no external URLs). `desktop-app-runenpfad.css` is listed in `desktopAppMaterialsExempt` (`ui/desktop_app_materials_test.go`), scopes classes to `.rp-*` (never `cc-` or `ad-`), and avoids `!important` and CSS `animation:`.
- **Registration & bundles**: Registered in `internal/desktop/types.go` (`games`, `open_maximized: "true"`, `logo_path: "/img/runenpfad/icon.svg"`), `desktop-foundation.js`, `module-loader.js`, `menus-and-routing.js`, and `window-shell-runtime.js`; bundled into `ui/js/desktop/bundles/main.bundle.js`.
- **Module load order**: `module-loader.js` must list `biomes` → `fx` → `physics` → `behaviors` → `bosses` → `variety` → `profile` → `zones` → `levels` → `combat` → `sim` → `audio` → `music` → `backdrops` → `backdrops-biomes` → `atmosphere` → `decor` → `actors` → `stage-art` → `postfx` → `view` → `app`. Each module reads its dependencies from the shared global scope at call time, so a new module must be placed after everything it reads.

## Work Guidance

- Keep each first-party JS file under `ui/js/desktop/` below `1100` lines with semantic filenames (`TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget`, `TestVirtualDesktopJSUsesSemanticChunkNames`). Split new subsystems into their own module rather than growing `sim.js` or `levels.js`.
- Sound (`profile.sound`) and music (`profile.music`) are enabled by default. Sound toggles from the top HUD bar or the pause menu. Music toggles from the pause menu. Audio unlocks on the first user gesture. Suspend audio whenever the window becomes inactive, minimized, hidden, or a modal is open.
- Graphics (`profile.gfx`) defaults to `auto`, which uses the GPU stage when available. The pause menu switches it to `canvas`.
- The impact freeze (`fx.hitstop`) freezes the world but must keep resolving player input (attack, heal) and pickups, otherwise gameplay and the existing simulation tests break.
- Preserve test names `TestRunenpfadRegistration` and `TestDesktopRunenpfadBrowser` so CI browser shards (`TestUIBrowserCIShardsCoverEveryTest`) cover both cleanly without modifying workflow selectors.
- Maintain genuine translations for all `runenpfad.*` keys and `desktop.app_runenpfad` across all 16 locales.
- After changing the module list, rebuild and check the bundles with `node scripts/build-ui-bundles.js --check`.

## Verification

- Simulation and real pack manifest checks: `node scripts/test-runenpfad-sim.mjs`
- UI registration, materials, line budget, semantic chunk names, i18n and frontend checks: `go test ./ui -run 'TestRunenpfadRegistration|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestVirtualDesktopJSUsesSemanticChunkNames|TestUIAvoidsCookieConsentClassPrefix|TestAuraDriftAvoidsAdBlockerClassPrefix|TestShippedWebAssetsLoadNothingExternal|TestTranslations_|TestFrontend_|TestDesktopAppMaterials' -count=1`
- Real Chrome desktop smoke test: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestDesktopRunenpfadBrowser' -count=1 -v`
- Bundle freshness: `node scripts/build-ui-bundles.js --check`

## Child DOX Index

None.
