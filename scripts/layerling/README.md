# Layerling adapter build

Upstream: https://github.com/henmedia/layerling, AGPL-3.0-only, version 1.154.1,
commit `bdf50b068f7b0475dece5307e1b2019aa10d5305` (tag `v1.154.1`).

Download https://codeload.github.com/henmedia/layerling/zip/bdf50b068f7b0475dece5307e1b2019aa10d5305
as `disposable/_layerling/upstream-1.154.1.zip`. SHA-256:
`be64e8f895bad8e5a2df961d3a345f7a576bbf550f0f749b10b943625b16f080`.
Extract there, run `npm ci` in the extracted source, then from AuraGo run
`node scripts/build-layerling-vendor.mjs`. Use exactly Node 24.21.0.
`node scripts/build-layerling-vendor.mjs --check` verifies the shipped files.

The build applies checked source patches for the fixed URL prefix, substitutes
the Desktop entry page, connects the existing CAD dispatcher to the document
MessageChannel, directs exports to Desktop storage and disables the upstream
service worker. The upstream lockfile and CAD implementation are retained.
`operations.json` keeps the existing AuraGo agent operation set explicit;
upstream browser launch, font-download and drawing-export operations are not
automatically granted by a dependency update. Parameter schemas are regenerated
for the supported operations and checked against their training fixtures.

Patches are checked replacements in `build-layerling-vendor.mjs`: fixed
`basePath`, deterministic build ID, two build workers, the standalone entry
page and bridge, local font/guide/OCCT URLs, suppressed service worker and HTTP
MCP effect, Desktop import/export callbacks, mesh export completion (returned
where upstream still fires it, otherwise the awaited STL/3MF/OBJ path),
scoped static guide links, independent footer project links and GitHub release
notes. The local archive remains the corresponding source. The build also
runs `scripts/build-japanese-typeface.mjs` and copies the Zen Maru Gothic
OFL-1.1 notice next to the generated face. The upstream worker verifier checks the fixed prefix. No runtime
dependency is loaded from a CDN. OCCT's Emscripten binding requires JavaScript
`Function` constructors; `unsafe-eval` is restricted to this verified app's CSP.

After building, run `node scripts/test-layerling-vendor.mjs` and the `--check`
command above. The manifest hashes every output and the adapter build inputs.
`source.zip` contains the complete modified upstream source, lockfile, license,
adapter/build script and standalone build instructions. Archive timestamps are
fixed. Runtime browser tests are `TestLayerlingDesktopBrowser` with
`AURAGO_RUN_BROWSER_SMOKE=1`; the integration also has broker, file and permission
tests. The original upstream unit tests run with `npm test` in the source tree.
Their OCCT and theme URL expectations are patched to the fixed AuraGo prefix;
all geometry assertions remain unchanged. Adapter source uses canonical LF
bytes for hashing and the archive, independent of Git's Windows line endings.
The browser package omits upstream's optional `store.php` and public-site sitemap;
their source remains in the corresponding-source archive.
