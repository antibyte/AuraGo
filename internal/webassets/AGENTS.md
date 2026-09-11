# External browser resources

This package owns immutable, version-bound local web resources, integrity checks,
archive/directory installation and filesystem resolution. `cmd/assetpack` builds
the set from `assets/web-assets.json`; `cmd/aurago/assets.go` configures the root
before services start. `internal/server/assets_handlers.go` owns recovery/auth
integration and versioned HTTP serving. See `documentation/web-assets.md`.

- The release executable pins manifest ID, archive SHA-256/length and optional
  immutable GitHub URL through generated ldflags. No latest resolution, CDN UI,
  source/CWD fallback or new dependency. Unpinned builds expose recovery only.
- Verify every file before use; keep reads under `os.Root` and manifest paths.
  Reject archive links, traversal, duplicates, modes and size/count violations.
  Serialize installs; publish a verified directory atomically. Keep old sets.
- Set `Default` once before services start. Installation never hot-swaps captured
  templates or the active filesystem; restart activates the matching set.
- Keep tests runnable with `go test ./internal/webassets ./cmd/assetpack`.
  `scripts/check-web-assets.mjs` enforces a 10 MB first-party embed budget and
  the separate recovery-page and stripped-binary budgets.
- `ui.Content` and per-package test fixtures are test inputs, never production
  fallback sources. Maintain missing-set behavior for Desktop and Game Maker.

No child contracts.
