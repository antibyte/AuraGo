# Oh-My-Pi Catalog Attribution

AuraGo vendors a normalized snapshot of model and provider metadata from the npm package `@oh-my-pi/pi-catalog`.

- Upstream project: https://github.com/can1357/oh-my-pi
- Package: https://www.npmjs.com/package/@oh-my-pi/pi-catalog
- License: MIT
- Copyright: Can Boluk and oh-my-pi contributors

The vendored files live under `internal/llm/catalog/`:

- `ohmypi_models.json`
- `ohmypi_models.json.gz`
- `ohmypi_providers.json`
- `ohmypi_providers.json.gz`
- `ohmypi_metadata.json`

The snapshot is updated manually with:

```bash
go run scripts/sync_ohmypi_catalog.go --version latest --check
go run scripts/sync_ohmypi_catalog.go --version latest --write
```

For a reproducible refresh, pass the same explicit npm version to `--write` and the subsequent `--check`. The metadata file records the imported version and tarball URL. The generated gzip files are embedded in the Go binary, so a running installation receives the refreshed catalog after a rebuild and restart.

The sync downloads one npm release tarball and extracts only `package/src/models.json`, `package/src/compat/rules.json`, and `package/package.json`. Models and compiled provider rules therefore come from the same release. AuraGo does not execute upstream TypeScript at runtime or during sync.

Since 19.x upstream ships `models.json` in a compact encoding (`version: 1`, interned key table, shared nodes, boolean bitmasks, per-provider constants; see upstream `src/model-catalog-format.ts`). `internal/llm/catalogsync/compact.go` expands it to the older provider → model ID → model layout before parsing; older plain-map releases still parse unchanged. The 19.1.1 expansion was checked against upstream's own decoder (84 providers, 6,104 models, no differences).
