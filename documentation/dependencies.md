# Dependency maintenance

AuraGo pins Go 1.27.1 in `go.mod`, container builders, installers and release
scripts. `GOTOOLCHAIN=auto` downloads that compiler when the host has an older Go.
Release scripts select the exact compiler explicitly. The stripped executable
budget is 125 MB; first-party embeds remain capped at 10 MB and recovery at 1 MB.
The model/provider catalog embeds deterministic gzip copies of its JSON sources.
`go run scripts/sync_ohmypi_catalog.go --write` regenerates both forms; `--check`
checks both. Catalog tests verify byte equality after decompression, preserving
all models and provider metadata without embedding the large plain JSON files.

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
| Unsloth training | Unsloth 2026.10.3; Torch 2.14.1, Transformers 5.17.0, TRL 0.24.0, Datasets 4.8.5 | Unsloth and Unsloth Zoo require Datasets <5 and Transformers <=5.17.0. Retain the existing TRL API; see the outstanding advisories below. |
| KaTeX | 0.18.9 override | Mermaid and micromark-extension-math still request the vulnerable 0.16 line. Share the fixed version already used by Milkdown and rebuild both browser and Notes vendors. |
| lodash-es | 4.18.1 override | Mermaid's Chevrotain dependency otherwise selects a vulnerable older pin. |
| argon2id | 1.0.1 exact pin | Desktop Tresor ships its local JS/WASM with the upstream license; cryptographic vector and vendor drift checks gate changes. |
| ulikunitz/xz | ≥ 0.5.15 | Local Wikipedia decompresses xz clusters from downloaded ZIM files; earlier releases leak memory on corrupted multi-stream LZMA input (GO-2025-3922, CVE-2025-58058). |
| blevesearch/snowballstem | 0.9.0 exact pin | Local Wikipedia must stem query words exactly like the Xapian index inside a downloaded ZIM file; its output was verified against Xapian 1.4 on the Snowball vocabularies of ten languages. Re-run the stemmer parity tests before bumping it. |

### Outstanding training advisories (2026-10-08)

The optional GPU training environment still has three Dependabot alerts for two
advisories; these are not resolved by the dependency refresh:

- [GHSA-379c-qx7v-6h59](https://github.com/advisories/GHSA-379c-qx7v-6h59):
  Datasets folder builders can read files outside their dataset directory through
  crafted `file_name` metadata. The fix requires Datasets >=5.0.1, incompatible
  with Unsloth 2026.10.3 and Unsloth Zoo 2026.10.3. Both `requirements.txt` and
  `uv.lock` are flagged.
- [GHSA-27vj-qcqg-25rc](https://github.com/advisories/GHSA-27vj-qcqg-25rc):
  fsspec ReferenceFileSystem template injection can execute code. The fix requires
  fsspec >=2026.6.0, but Datasets 4.8.5 requires fsspec <=2026.2.0.

Do not use untrusted folder-builder metadata or ReferenceFileSystem templates in
this environment. AuraGo's trainer constructs datasets from reviewed JSONL rows
with `Dataset.from_list`; that does not make other uses of these packages safe.
Keep the alerts open. Revisit both pins when Unsloth supports a fixed Datasets
release; do not bypass upstream requirements with resolver overrides. The Torch
and Transformers updates require a GPU smoke run before production training;
offline loader tests and lock resolution do not establish CUDA compatibility.

Managed ACE-Step, sanoTTS, RTL-SDR and other service/model releases retain their
qualified image, native-library and model hashes. Updating these is a separate
release operation with the owning `AGENTS.md` checks and actual hardware/media
qualification; editing a Docker build input does not update a published image.
ACE-Step still exports its upstream frozen Python lock and uses matched hardware
wheels. Its build-time uv is 0.12.20. Supertonic remains on upstream 1.3.1.
The Ansible sidecar pins 14.4.0 and gives its unprivileged user a writable home.

## Browser resources

Synth Studio vendors `@tonejs/midi` 2.0.28 (MIT) and only the factory synthesis
tables from `webaudio-tinysynth` 1.1.4 (Apache-2.0) under
`ui/js/vendor/synth-studio/`. The native Web Audio renderer owns timing for both
playback and offline WAV export; the upstream TinySynth player is not loaded.
Retain the source/version, checksum and license records beside these assets.

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
- CI (`security-gates.yml`) runs `npm audit --omit=dev --audit-level=moderate`
  in the root and `browser_automation_sidecar` projects. Before a release, run a
  full `npm audit` (dev dependencies included) in both locally, plus all vendor
  `--check` commands, UI bundle checks and focused real-browser tests for
  consumers of changed libraries.
- Run `uv lock --check` in `training` and `training/needle3`; run the training
  workflow's exporter, canonical catalog, dataset and Python tests.
- Run `cargo test --locked` in `tools/aurago-tui`.
- Build the affected Dockerfiles and test the actual unprivileged runtime.
- Build resource-bound binaries and run `--check-assets`; local builds and
  synthetic browser checks do not establish a deployed or hardware-qualified
  release. Keep execution logs and analysis in ignored `reports/`.
