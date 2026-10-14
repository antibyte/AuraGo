# Layerling adapter build

Upstream: https://github.com/henmedia/layerling, AGPL-3.0-only, version 1.71.0,
commit `8704d5c99adfeb981be31bb5eddc9c17f75ab281`.

Download https://codeload.github.com/henmedia/layerling/zip/8704d5c99adfeb981be31bb5eddc9c17f75ab281
as `disposable/_layerling/upstream-1.71.0.zip`. SHA-256:
`53a613e5fa04e003d47d5aedf552595c8774a1c846984aa8cab0058358ed41d6`.
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
