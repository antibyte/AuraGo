# Dependency maintenance

AuraGo pins Go 1.27.1 in `go.mod`, container builders, installers and release
scripts. `GOTOOLCHAIN=auto` downloads that compiler when the host has an older Go.
Release scripts select the exact compiler explicitly. The stripped executable
budget is 125 MB; first-party embeds remain capped at 10 MB and recovery at 1 MB.

The root and browser-sidecar npm lockfiles, both training `uv.lock` files and
`tools/aurago-tui/Cargo.lock` are authoritative reproducible dependency inputs.
The Virtual Computers guest has its own `guest_workspace_agent/module.txt` and
`sum.txt`; these become its standalone `go.mod` and `go.sum` on the build host.
Use stable upstream releases; a larger version is not sufficient when it breaks
the consuming library or reintroduces a vulnerability.

## Compatibility bounds

| Dependency | Supported version | Reason to retain the bound |
| --- | --- | --- |
| gRPC Go | 1.83.2 | 1.84.0 is affected by GO-2026-6443; use a fixed stable successor when available. |
| pdfcpu | 0.15.0 | 0.16 changes `MergeRaw`; Maroto 2.4.2 still calls the previous API. |
| fetchup | 0.2.4 | Rod 0.116.2 still needs its older browser-download API. |
| gobwas/glob | 0.2.3 | Colly 2.3.0 uses the `Glob` API removed in 1.0. |
| gVisor | Tailscale's February 2026 generated Go revision | Upstream main contains Bazel/template packages that do not build as a Go module. |
| Pion media family | ICE 4.3, WebRTC 4.2, TURN 5.0, SRTP 3.0, DTLS 3.1 | Diago 0.40 requires this coherent family; newer minors change STUN/transport types. Exact patches are in `go.mod`. |
| Unsloth training | Unsloth 2026.9.12; Torch 2.12.1, Transformers 5.5.0, TRL 0.24.0, Datasets 4.3.0 | These are the newest versions satisfying upstream Unsloth's declared constraints. |
| lodash-es | 4.18.1 override | Mermaid's Chevrotain dependency otherwise selects a vulnerable older pin. |
| argon2id | 1.0.1 exact pin | Desktop Tresor ships its local JS/WASM with the upstream license; cryptographic vector and vendor drift checks gate changes. |

Managed ACE-Step, sanoTTS, RTL-SDR and other service/model releases retain their
qualified image, native-library and model hashes. Updating these is a separate
release operation with the owning `AGENTS.md` checks and actual hardware/media
qualification; editing a Docker build input does not update a published image.
ACE-Step still exports its upstream frozen Python lock and uses matched hardware
wheels. Its build-time uv is 0.12.20. Supertonic remains on upstream 1.3.1.
The Ansible sidecar pins 14.4.0 and gives its unprivileged user a writable home.

## Browser resources

Run `npm ci` before generators. `npm run build:browser-vendor` rebuilds shared
libraries, licenses and hashes, including Three.js and its matching loaders,
PDF.js workers/fonts, noVNC, Rive and ESP Web Tools. `-- --check` verifies without
writing. Other `build:*-vendor`, `build:game-maker-3d`, `build:system-world`,
`build:screensaver-abyss` scripts retain their own verified manifests. Also run
`node scripts/build-game-maker-presentation.js` and `npm run build:ui`.

Tresor's MIT `argon2id` resources are copied with `npm run build:tresor-vendor`.
Run `npm run check:tresor-vendor` and `npm run test:tresor-crypto` after changes.
Its wrapper clears the complete WASM workspace after each derivation.

PDF.js and noVNC are awaited ES modules. Three.js uses modern color-space APIs
and WebGL2; keep each app's fallback. Game Maker versions both module and core
filenames so updates do not overwrite runtimes used by older games. Unused Quill,
Tailwind and the replaced xterm canvas addon are not shipped.

## Verification

- Run the Go test wrapper from root `AGENTS.md`, `go vet` and `govulncheck`.
- Run `npm audit` in both npm projects, all vendor `--check` commands, UI bundle
  checks and focused real-browser tests for consumers of changed libraries.
- Run `uv lock --check` in `training` and `training/needle3`; run the training
  workflow's exporter, canonical catalog, dataset and Python tests.
- Run `cargo test --locked` in `tools/aurago-tui`.
- Build the affected Dockerfiles and test the actual unprivileged runtime.
- Build resource-bound binaries and run `--check-assets`; local builds and
  synthetic browser checks do not establish a deployed or hardware-qualified
  release. Keep execution logs and analysis in ignored `reports/`.
