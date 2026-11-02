# FindIt artwork

## Purpose

Original realistic miniature sprites and app artwork for the physical inventory.

## Ownership

- `sources/` retains ImageGen master PNGs; `prompts.md` records their generation contract.
- `../../scripts/export-findit-assets.py` owns deterministic runtime exports to `ui/img/findit`.
- `../../internal/findit/AGENTS.md` owns catalog semantics; the Desktop app guide owns presentation.

## Local Contracts

- Preserve 64 row-major motif IDs, realistic materials, consistent camera/lighting and transparent alpha.
- Artwork depicts families, never a verified dimensional drawing or guaranteed pin count. Technical values remain text.
- Runtime assets are local and version-bound. Do not expand the global Desktop icon sprite.
- Photos override an explicitly selected sprite, which overrides the package/template default.

## Work Guidance

Keep masters and prompts. Export with Pillow 13.0.0; no network is needed to rebuild. New icons require a stable ID and manifest coverage.

## Verification

- `py -3.13 scripts/export-findit-assets.py --check` checks manifest, hashes, dimensions and deterministic bytes.
- Review the atlas on light/dark backgrounds and miniature readability in the browser fixture.

## Child DOX Index

None.
