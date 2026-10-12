# Layerling adapter build

Upstream: https://github.com/henmedia/layerling, AGPL-3.0-only, version 1.57.0,
commit `25f12513802e322cdccecaf138f874ff526f331d`.

Download https://codeload.github.com/henmedia/layerling/zip/25f12513802e322cdccecaf138f874ff526f331d
as `disposable/_layerling/upstream.zip`. SHA-256:
`9d81fcb78d13afb07511ec75279c341cce87c4ec7efa10ed4cc6a972c43bc5e8`.
Extract there, run `npm ci` in the extracted source, then from AuraGo run
`node scripts/build-layerling-vendor.mjs`. Use exactly Node 24.15.0.
`node scripts/build-layerling-vendor.mjs --check` verifies the shipped files.

The build applies checked source patches for the fixed URL prefix, substitutes
the Desktop entry page, connects the existing CAD dispatcher to the document
MessageChannel, directs exports to Desktop storage and disables the upstream
service worker. The upstream lockfile and CAD implementation are retained.

Patches are checked replacements in `build-layerling-vendor.mjs`: fixed
`basePath`, deterministic build ID, two build workers, the standalone entry
page and bridge, local font/guide/OCCT URLs, suppressed service worker and HTTP
MCP effect, Desktop import/export callbacks, awaited mesh export completion,
scoped static guide links and independent footer project links. The upstream worker verifier checks the fixed prefix. No runtime
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
