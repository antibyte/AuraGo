# Repository scripts

## Purpose

Developer builds, asset generation, checks, local start/stop, service installation and release entrypoints.

## Ownership

This folder owns the maintained helpers and the `start.*`, `kill_all.*`, `install_service_linux.sh`, `make_deploy.*` and `make_release.*` entrypoints. Root retains public `install.sh`, `install.ps1`, `update.sh` and `repair-update.sh` URLs. Release security and publication policy remain in root and `../internal/upkeep/AGENTS.md`.

## Local Contracts

- Resolve the repository root from the script location before using project-relative inputs or outputs; invoking an entrypoint from another working directory must not relocate state, locks or artifacts.
- `install-ci-cosign.ps1` stages the pinned Windows/Linux X64 tool through the GitHub releases API. It uses CI runner paths and exposes only size/SHA-256-verified binaries on `GITHUB_PATH`; it never manages signing keys.
- Storymaker model/catalog builders use committed Desktop pet derivatives and image masters offline. Preserve source pets, per-model provenance and the asset contract in ../assets/storymaker/AGENTS.md; publish catalog files atomically and skip unchanged bytes.
- Retained license inputs copied verbatim by vendor generators must be LF-pinned in `.gitattributes`. The Windows runtime CI job checks Storymaker vendor bytes after its clean npm install, before release cross-builds.
- Storymaker's vendor input scan accepts dependency paths outside the checkout when a worktree links `node_modules`; these inputs must retain the same dependency notices as a local install. Verify `node scripts/build-ui-bundles.js --check` without changing vendor bytes.
- `check-web-assets.mjs` caps first-party package embeds at 12 MB, including bundled speech runtimes, firmware and design templates. Full UI embedding stays forbidden; recovery assets stay below 1 MB and stripped binaries below 200 MB. Security Gates checks embeds before the Go test suite, and release CI checks them before cross-building.
- Sibling helpers are relative to this folder; `deploy/`, `bin/`, `.env` and source inputs remain relative to the repository root.
- Source paths may change; release asset names and installed config/notice paths stay stable. The English third-party notices come from `../licences/`.
- `build-modern-web-guidance.py --archive <zip>` imports only the SHA-256-pinned upstream distribution into the tools-owned XZ-compressed skill ZIP; `--check` validates its manifest, adapter and index offline. No upstream executable is run. Corpus and startup contracts: `../internal/tools/bundled_agent_skills/AGENTS.md`.
- Keep the Bluetooth kit synchronized into root installers/updater and `scripts/install_service_linux.sh` through `sync-bluetooth-kit.sh`.

## Work Guidance

Use the existing helper for each job. Check release scripts with harmless fixtures and syntax parsing; publication and real service/process operations are not verification steps.

## Verification

- `test-kicad-export.py` checks fixed CLI arguments, missing/escaping libraries and
  result manifests offline; `--image` runs both real exports plus the read-only
  probe in an isolated container. Both architectures gate image publication.

- Everyday Desktop controls: `test-ui-regressions.mjs` runs the focused `test-desktop-{window-layouts,window-previews,search,start-search,settings-runtime,file-association-writes,notifications,accessibility,file-history,file-producers,quick-settings,clock-calendar,media-session,custom-wallpaper,workspace-settings,workspaces,session-actions,session-adapters,window-overview-runtime}.mjs` scripts. Each can also run directly with Node. Keep receipt/partial/session-clear, revision-conflict, logout save/cancel and guarded preview close tests alongside actual browser flows. `ui/desktop_window_overview_browser_test.go` covers grouped pins, workspace scope, minimized windows and shared snap selection. Browser acceptance needs the central opt-in; rebuild/check `scripts/build-ui-bundles.js` after shell/module changes.

<!-- pinball:start -->
- Pinball: `npm run build:pinball` (`build-pinball-assets.mjs` then `build-pinball.mjs`),
  `npm run test:pinball` (`test-pinball.mjs` runs the `test-pinball-*.mjs` suites and the
  `lib/pinball-sim.mjs` / `lib/pinball-shots.mjs` helpers), and the read-only `--check` modes of
  `build-pinball.mjs` and `build-pinball-assets.mjs`. `pinball-dev-server.mjs` serves the harness at
  `/harness`. No Blender or Python generator is involved; `assets/pinball/prepare.py` only converts
  retained ImageGen masters. Browser verification is
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopPinball`; `AURAGO_PINBALL_GPU=1` permits the
  available hardware renderer for captures.
<!-- pinball:end -->

- `node scripts/test-desktop-icon-resolver.mjs` checks all seven Desktop icon
  catalogs and their files, shared mini aliases/SVG symbols, and EasyDrag's
  literal app-local icon calls. `TestDesktopAllAppIconsBrowser` verifies every
  built-in app identity and the shared action supplements in light and dark.
- `python scripts/test-pet-notices.py` reproduces the catalog's scoped model notices
  and verbatim humanoid license copies without Blender or touching model bytes.
- 3D pets: Blender runs `assets/desktop-pets/build_model.py` (Punk),
  `build_zombert.py -- --pet=<folder>` (fifteen active VARCO humanoids),
  `build_quadruped_pet.py -- --pet=<folder>` (three native quadruped rigs), or
  `build_creature_pet.py -- --pet=<folder>` (eight active anatomical creatures).
  `node scripts/build-punk-pet-motions.mjs --pet=<folder>` retargets pinned
  Mesh2Motion clips; `build-creature-pet-motions.mjs --pet=<folder>` generates
  original anatomical creature movements. Both use `pet-skeleton.mjs` offline.
  `node scripts/test-punk-pet.mjs` checks encoded texture sizes, skin/motion
  provenance and real Chrome/Edge WebGL interaction/failure/disposal behavior.
  Rebuild all eleven active creature packs after their generator changes. The test
  accepts all 27 catalog folders with `--pet=<folder>`; its default
  and the humanoid motion builder's default remain Punk.
  `node scripts/test-desktop-pet-motion.mjs` checks sprite/model roaming cadence,
  refresh continuity, explicit settings, motion preferences and teardown with
  a deterministic clock. The native test also walks through repeated refreshes.
  Hana and Vampir additionally check edge deformation across ninety sampled poses.
  `vampire-pet-motions.mjs` adds the original, hash-bound tentacle layer to the
  vampire's CC0 body clips; its 69-joint model is authored by `vampire_rig.py`.
  `repair-hana-pet.mjs` applies its current hash-bound report to isolated LOD
  faces (never more than 5%); rebuild Hana's motions and repeat the test afterward.

- Aura Drift mascots: `build-aura-drift-pets.mjs [--check]` curates nine stationary
  three-clip packs from existing Desktop models/motions; original assets stay
  unchanged. `test-aura-drift-pets.mjs` checks track clearance, prop reservations,
  source hashes, budgets and anchored motion. Native checks use
  `TestDesktopAuraDriftPetDecorBrowser`.
- Aura Drift: `node scripts/test-aura-drift-audio.mjs --fast` checks the pack and
  requested-music queue priority; the full run also decodes assets with FFmpeg.
  `node scripts/test-aura-drift-fx-resources.mjs` checks ordered FX initialization
  and the optional fallback when a stale loader omits its resource module.
- Runenpfad: `node scripts/test-runenpfad-sim.mjs` validates all 6 2D sprite packs, `aurago-sounds`, all 25 campaign stages, endless mode, hero abilities, resonance paths, workshop upgrades, save clamping, the new-best flag and the floating-label translation keys in all 16 locales. `node scripts/test-runenpfad-audio.mjs` checks cue routing; `scripts/build_runenpfad_art.py --check` verifies generated sheets and provenance. Browser play is `TestDesktopRunenpfadBrowser`.
- Sprite packing: `pack_game_sprites.py` fills the declared ten-column grid from
  reviewed rectangles and anchors in the production manifest; human packs append
  two climbing rows without renumbering existing cells. `test_pack_game_sprites.py`
  checks reproducibility and animation controls; `test-game-maker-sprites.mjs`
  checks actual helper loading, registration, facing and climbing transitions.
  Artifact replacement stages a complete temporary file beside its destination,
  preserves the previous file on failure and skips unchanged bytes.
- Chat regression fixtures load the shared `shouldShowAgentToolStatus` policy before
  the Desktop chat module, matching production order. Camera preference assertions
  allow only the non-secret viewing fields and the optional `sound` flag (default false).
- Camera creative controls: `node scripts/test-camera-creative.mjs` (also
  `npm run test:camera`, chained into `test:ui-regressions`) checks the
  cover-aware crop maths, zoom/ratio masks, look availability, panel lifecycle
  and disposal with a DOM-free controller stub. `go test ./ui -run DesktopCameraCreative`
  pins the module contract and all sixteen `desktop.camera_*` locale sets.
- Sheets: `build-sheets-vendor.js [--check]` bundles the pinned Univer packages and applies the reviewed formula-progress timer disposal patch without modifying `node_modules`; regenerate engine, worker and manifest together.
- Storymaker: `build-storymaker-vendor.mjs [--check]` bundles the exact PixiJS/GSAP lock versions and included notices; `build-storymaker-catalog.mjs [--check]` packages retained ImageGen masters offline. `test-storymaker-raster-browser.mjs` owns native renderer, rig-matrix, environment and seek-parity acceptance; see `../assets/storymaker/ASSET-CONTRACT.md`.
- Storymaker agent references: `storymaker-tests/24-agent-reference.mjs` executes the embedded effects and SVG-asset examples with GSAP and checks reverse/random/restarted seeks in every aspect. `test-storymaker-raster-browser.mjs --agent-reference --canvas` or `--svg-reference --canvas` substitutes the selected example in the real opaque-origin player, retaining strict CSP and software fallback checks.
- Storymaker articulation: `storymaker-tests/25-model-characters.mjs` checks real model topology, attached lip boundaries, recessed mouth openings, stable skulls, smoothed speech, poses and reverse seeks. `build-storymaker-models.mjs` writes derived models atomically and skips unchanged bytes.
- Native `--matrix`, `--combinations` and `--speech --character char.hoodie --canvas` render poses and speech through the real player; repeat for `char.professor` and `char.punk`. Speech clips are three-second face close-ups covering every mouth shape and the closing pause; matrix frames retain full figures. FFmpeg is required for speech clips; numeric topology checks do not replace visual acceptance.
- Strudel: `node scripts/build-strudel-vendor.mjs` rebuilds the pinned runtime,
  license notices, source ZIP and integrity manifest from the npm lock. `--check`
  regenerates in memory and compares without writing. Resolve inputs from the
  repository root; source archives include the integration and its build inputs.
- System World: `node scripts/build-system-world.js --check` verifies the pinned
  Three.js 0.186.1 renderer bundle; `node scripts/build-system-world-spot.mjs --check`
  reproduces the meshopt companion from its hash-pinned BSD-3-Clause source;
  `node scripts/test-system-world-companion.mjs` checks its patrol, provenance,
  licence texts and the 12 MiB model budget.

- Run `bash -n <path>` for each changed shell script.
- Parse changed PowerShell with `System.Management.Automation.Language.Parser`.
- Run `node scripts/test-easydrag.mjs` for EasyDrag UI module and pack wire-contract checks.
- Run `node scripts/test-easydrag-approvals.mjs` for approval DTO, bounds, review-safety and blocked-reason checks.
- `python scripts/test-prepare-phonon.py` covers archive integrity, API routing, bounds and GitHub/Hugging Face credential separation. Run it with relevant `go test ./internal/audit` release/install/build contracts.

## Child DOX Index

None.
