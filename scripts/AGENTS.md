# Repository scripts

## Purpose

Developer builds, asset generation, checks, local start/stop, service installation and release entrypoints.

## Ownership

This folder owns the maintained helpers and the `start.*`, `kill_all.*`, `install_service_linux.sh`, `make_deploy.*` and `make_release.*` entrypoints. Root retains public `install.sh`, `install.ps1`, `update.sh` and `repair-update.sh` URLs. Release security and publication policy remain in root and `../internal/upkeep/AGENTS.md`.

## Local Contracts

- Resolve the repository root from the script location before using project-relative inputs or outputs; invoking an entrypoint from another working directory must not relocate state, locks or artifacts.
- `install-ci-cosign.ps1` stages the pinned Windows/Linux X64 tool through the GitHub releases API. It uses CI runner paths and exposes only size/SHA-256-verified binaries on `GITHUB_PATH`; it never manages signing keys.
- `check-web-assets.mjs` caps first-party package embeds at 12 MB, including bundled speech runtimes, firmware and design templates. Full UI embedding stays forbidden; recovery assets stay below 1 MB and stripped binaries below 200 MB. Security Gates checks embeds before the Go test suite, and release CI checks them before cross-building.
- Sibling helpers are relative to this folder; `deploy/`, `bin/`, `.env` and source inputs remain relative to the repository root.
- Source paths may change; release asset names and installed config/notice paths stay stable. The English third-party notices come from `../licences/`.
- Keep the Bluetooth kit synchronized into root installers/updater and `scripts/install_service_linux.sh` through `sync-bluetooth-kit.sh`.

## Work Guidance

Use the existing helper for each job. Check release scripts with harmless fixtures and syntax parsing; publication and real service/process operations are not verification steps.

## Verification

- `node scripts/test-desktop-icon-resolver.mjs` checks all seven Desktop icon
  catalogs and their files, shared mini aliases/SVG symbols, and EasyDrag's
  literal app-local icon calls. `TestDesktopAllAppIconsBrowser` verifies every
  built-in app identity and the shared action supplements in light and dark.
- Punk 3D pet: Blender runs `assets/desktop-pets/build_model.py`, then
  `node scripts/build-punk-pet-motions.mjs` retargets the pinned Mesh2Motion clips.
  `node scripts/test-punk-pet.mjs` checks encoded texture sizes, skin/motion
  provenance and real Chrome/Edge WebGL interaction/failure/disposal behavior.

- Aura Drift: `node scripts/test-aura-drift-audio.mjs --fast` checks the pack and
  requested-music queue priority; the full run also decodes assets with FFmpeg.
- Sheets: `build-sheets-vendor.js [--check]` bundles the pinned Univer packages and applies the reviewed formula-progress timer disposal patch without modifying `node_modules`; regenerate engine, worker and manifest together.
- Storymaker: `build-storymaker-vendor.mjs [--check]` bundles the exact PixiJS/GSAP lock versions and included notices; `build-storymaker-catalog.mjs [--check]` packages retained ImageGen masters offline. `test-storymaker-raster-browser.mjs` owns native renderer, rig-matrix, environment and seek-parity acceptance; see `../assets/storymaker/ASSET-CONTRACT.md`.
- Storymaker agent references: `storymaker-tests/24-agent-reference.mjs` executes the embedded effects example with GSAP and checks reverse/random/restarted seeks in every aspect. `test-storymaker-raster-browser.mjs --agent-reference --canvas` substitutes that example in the real opaque-origin player, retaining strict CSP and software fallback checks.
- Storymaker articulation: `storymaker-tests/25-sprite-joints.mjs` checks calibrated endpoints, joint alpha coverage, widths and hand proportions across the delivered cast. Native `--matrix --character char.<id> --canvas` saves 512-pixel cells plus individual pose/gesture frames for detailed inspection.
- Strudel: `node scripts/build-strudel-vendor.mjs` rebuilds the pinned runtime,
  license notices, source ZIP and integrity manifest from the npm lock. `--check`
  regenerates in memory and compares without writing. Resolve inputs from the
  repository root; source archives include the integration and its build inputs.
- System World: `node scripts/build-system-world.js --check` verifies the pinned
  Three.js 0.186.1 renderer bundle; `node scripts/build-system-world-spot.mjs --check`
  reproduces the meshopt companion from its hash-pinned BSD-3-Clause source;
  `node scripts/test-system-world-companion.mjs` checks its patrol, provenance,
  licence texts and the 12 MiB model budget.
- Goldfish Adventure: `node scripts/test-goldfish-state.mjs`,
  `node scripts/test-goldfish-animation.mjs`, `node scripts/test-goldfish-save.mjs`,
  `node scripts/test-goldfish-app.mjs` and `node scripts/verify-goldfish-assets.mjs`.
  Save/app tests use a real isolated Chrome/Edge profile and local loopback;
  state/animation/asset checks require no service or provider credentials.

- Run `bash -n <path>` for each changed shell script.
- Parse changed PowerShell with `System.Management.Automation.Language.Parser`.
- Run `node scripts/test-easydrag.mjs` for EasyDrag UI module and pack wire-contract checks.
- Run `node scripts/test-easydrag-approvals.mjs` for approval DTO, bounds, review-safety and blocked-reason checks.
- `python scripts/test-prepare-phonon.py` covers archive integrity, API routing, bounds and GitHub/Hugging Face credential separation. Run it with relevant `go test ./internal/audit` release/install/build contracts.

## Child DOX Index

None.
