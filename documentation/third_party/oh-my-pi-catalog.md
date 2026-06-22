# Oh-My-Pi Catalog Attribution

AuraGo vendors a normalized snapshot of model and provider metadata from the npm package `@oh-my-pi/pi-catalog`.

- Upstream project: https://github.com/can1357/oh-my-pi
- Package: https://www.npmjs.com/package/@oh-my-pi/pi-catalog
- License: MIT
- Copyright: Can Boluk and oh-my-pi contributors

The vendored files live under `internal/llm/catalog/`:

- `ohmypi_models.json`
- `ohmypi_providers.json`
- `ohmypi_metadata.json`

The snapshot is updated manually with:

```bash
go run scripts/sync_ohmypi_catalog.go --version latest --check
go run scripts/sync_ohmypi_catalog.go --version latest --write
```

The sync downloads the npm release tarball and extracts only `package/src/models.json`, `package/src/provider-models/descriptors.ts`, and `package/package.json`. AuraGo does not execute upstream TypeScript at runtime or during sync.
