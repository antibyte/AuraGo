# Managed ACE-Step runtime

## Purpose and ownership

This package owns local music setup, hardware qualification, the private worker,
model/cache volumes, Vault authentication and serial generation.

## Local contracts

- `aurago-acestep-local` is a music-only provider. All callers use the shared
  music path, quantity limit and media registry; local music has zero cloud cost.
- Keep Docker resource and Python secret-export protections. Browser and agent
  responses never contain the runtime key. Cancel/timeout stops actual inference.
- `release.json` pins images, models and effective Python model code; upstream
  replaces Hugging Face Python copies during loading. Downloads verify size/hash.
- Restore the previous qualified container after failed/interrupted replacement.
  No silent CPU/cloud fallback and no profile change during generation.

## Verification

- `go test ./internal/acestep` and `runtime/test_runtime.py` cover the contracts.
- Image builds and CPU tests do not establish GPU acceptance. Follow
  `documentation/ace-step-local.md` for hardware, UI and release checks.
