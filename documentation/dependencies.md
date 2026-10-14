# Dependency maintenance

AuraGo pins Go 1.27.2 in `go.mod`, container builders, installers and release
scripts. `GOTOOLCHAIN=auto` downloads that compiler when the host has an older Go.
Release scripts select the exact compiler explicitly. The stripped executable
budget is 130 MB (130,000,000 bytes); first-party embeds remain capped at 10 MB and recovery at 1 MB.
The model/provider catalog embeds deterministic gzip copies of its JSON sources.
`go run scripts/sync_ohmypi_catalog.go --write` regenerates both forms; `--check`
checks both. Catalog tests verify byte equality after decompression, preserving
all models and provider metadata without embedding the large plain JSON files.

The root and browser-sidecar npm lockfiles and
`tools/aurago-tui/Cargo.lock` are authoritative reproducible dependency inputs.
The Virtual Computers guest has its own `guest_workspace_agent/module.txt` and
`sum.txt`; these become its standalone `go.mod` and `go.sum` on the build host.
Use stable upstream releases; a larger version is not sufficient when it breaks
the consuming library or reintroduces a vulnerability.

## Compatibility bounds

| Dependency | Supported version | Reason to retain the bound |
| --- | --- | --- |
| CYD builder PlatformIO / Starlette | PlatformIO 6.2.0; Starlette >=1.3.1,<2 (hash-pinned lock); SCons 4.11.1 | PlatformIO 6.1.19 restricts Starlette below 0.53, excluding the fixes for GHSA-82w8-qh3p-5jfq, GHSA-jp82-jpqv-5vv3, GHSA-wqp7-x3pw-xc5r, GHSA-x746-7m8f-x49c and GHSA-86qp-5c8j-p5mr. PlatformIO 6.2 also requires the matching SCons update. Rebuild the builder and run its offline self-test before publishing an image. |
| CYD builder setuptools | >=83.0.0 (hash-pinned lock) | Python 3.11's venv seeds an older version outside PlatformIO's dependency graph. Pin its replacement too, and audit the installed Python environment as well as the requirements file. |
| cloudflared | 2026.10.0; Docker manifest digest and Linux amd64/arm64 SHA-256 pins | Reviewed release metadata in `internal/tools/cloudflare_install.go` and `cloudflare_tunnel.go`; no moving latest URL or checksum sidecar. Run Cloudflare/HomepageQuick tests when refreshing. See [Cloudflare Tunnel](cloudflare-tunnel.md). |
| gRPC Go | 1.83.2 | 1.84.0 is affected by GO-2026-6443; use a fixed stable successor when available. |
| Excelize | 2.11.1-0.20261003002531-6258dcebc4e2 | Security exception to stable releases: 2.11.0 remains affected by 15 workbook-parsing advisories. This exact upstream commit contains all their fixes; move to a fixed stable release when available and rerun the Office preservation and security tests. |
| pdfcpu | 0.15.0 | 0.16 changes `MergeRaw`; Maroto 2.4.3 still calls the previous API. |
| fetchup | 0.2.4 | Rod 0.116.2 still needs its older browser-download API. |
| ysmood/got and gop | 0.41.0 and 0.2.0 | Rod 0.116.2 imports `got/lib/lcs`, which newer got releases remove. |
| gobwas/glob | 0.2.3 | Colly 2.3.0 uses the `Glob` API removed in 1.0. |
| gVisor | Tailscale 1.104.1's September 2026 generated Go revision | Upstream main contains Bazel/template packages that do not build as a Go module. |
| Pion media family | ICE 4.3.0, WebRTC 4.2.17, TURN 5.0.13, SRTP 3.0.15, DTLS 3.1.8 | Diago 0.41 requires this coherent family; later releases change STUN/transport types, including some patch releases. |
| KaTeX | 0.18.9 override | Mermaid and micromark-extension-math still request the vulnerable 0.16 line. Share the fixed version already used by Milkdown and rebuild both browser and Notes vendors. |
| lodash-es | 4.18.1 override | Mermaid's Chevrotain dependency otherwise selects a vulnerable older pin. |
| argon2id | 1.0.1 exact pin | Desktop Tresor ships its local JS/WASM with the upstream license; cryptographic vector and vendor drift checks gate changes. |
| ulikunitz/xz | ≥ 0.5.15 | Local Wikipedia decompresses xz clusters from downloaded ZIM files; earlier releases leak memory on corrupted multi-stream LZMA input (GO-2025-3922, CVE-2025-58058). |
| blevesearch/snowballstem | 0.9.0 exact pin | Local Wikipedia must stem query words exactly like the Xapian index inside a downloaded ZIM file; its output was verified against Xapian 1.4 on the Snowball vocabularies of ten languages. Re-run the stemmer parity tests before bumping it. |

### External training environment

Python training environments, both uv locks, and their outstanding Datasets/fsspec
advisories live in [antibyte/agotrain](https://github.com/antibyte/agotrain).
See its `SECURITY.md` for compatibility blockers and `aurago-revision` for the
canonical exporter source. Moving the environment does not fix its vulnerabilities.
[Training integration](training.md) documents the pinned cross-repository checks.

Managed ACE-Step, sanoTTS, RTL-SDR and other service/model releases retain their
qualified image, native-library and model hashes. Updating these is a separate
release operation with the owning `AGENTS.md` checks and actual hardware/media
qualification; editing a Docker build input does not update a published image.
ACE-Step still exports its upstream frozen Python lock and uses matched hardware
wheels. Its build-time uv is 0.13.1. Supertonic remains on upstream 1.3.1.
The Ansible sidecar pins 14.5.0 and gives its unprivileged user a writable home.
The CYD builder is 1.0.2; its refreshed hash lock includes Starlette 1.8.0 and
setuptools 84.0.0. All nine offline board builds and both library-gating cases
must pass before publishing this image. Hardware-qualified firmware libraries
and board toolchains remain a coherent pinned set.

The sanoTTS Python wheel remains the qualified 0.6.0 source revision. PyPI still
publishes 0.5.0; upstream development 0.7.0 changes voice support and the root
licensing scope and is not a drop-in released wheel. RuView retains its existing
runtime/firmware release set; its separately tagged harness release does not
qualify a replacement sensing runtime. Native ONNX/llama.cpp and Decisions pins
likewise require the owning runtime's device and model checks, not just a newer
upstream tag.

## Browser resources

NASSCAD uses the v4.7.0-desktop release at
`279da7c0f0ca28dfae19c11c8be231f326dfda4f`. Reproduce its browser adaptation with
`node scripts/vendor-nasscad.mjs --source=<upstream checkout>` and verify it with
`--check`. Provenance records each original and adapted file. The adaptation removes
Google Analytics from the main page and the NassScript guide and drops the guide's Google
Fonts link; its font stacks fall back to local fonts. The local browser
test is `node scripts/test-nasscad-browser.mjs`; existing workspace entry files
remain user-owned. Layerling 1.71.0 has its own pinned build and complete source
archive, documented in `scripts/layerling/README.md`.

Synth Studio vendors `@tonejs/midi` 2.0.28 (MIT) and only the factory synthesis
tables from `webaudio-tinysynth` 1.1.4 (Apache-2.0) under
`ui/js/vendor/synth-studio/`. The native Web Audio renderer owns timing for both
playback and offline WAV export; the upstream TinySynth player is not loaded.
Retain the source/version, checksum and license records beside these assets.

Run `npm ci` before generators. `npm run build:browser-vendor` rebuilds shared
libraries, licenses and hashes, including Three.js and its matching loaders,
PDF.js workers/fonts, noVNC, Rive, ESP Web Tools and esptool-js (the CYD Creator
flasher bundle). `-- --check` verifies without
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
- Run the pinned Training Dataset Gates integration workflow. Python locks and
  dataset/loader checks belong to agotrain; see [training integration](training.md).
- Run `cargo test --locked` in `tools/aurago-tui`.
- Build the affected Dockerfiles and test the actual unprivileged runtime.
- Build resource-bound binaries and run `--check-assets`; local builds and
  synthetic browser checks do not establish a deployed or hardware-qualified
  release. Keep execution logs and analysis in ignored `reports/`.
