# AuraPinball delivered tables

## Purpose

Versioned local table payloads consumed by the Desktop Pinball app.

## Ownership

- `v1/` contains nine WebP images (flat playfield underlay, backglass and preview per table),
  shared CC0 audio and the generated integrity manifest (version 2).
- Original sources and recipes belong to `../../../assets/pinball/AGENTS.md`.
- Runtime behaviour belongs to `../../js/desktop/apps/pinball/AGENTS.md`.

## Local Contracts

- Do not hand-edit generated files. Rebuild artwork with `assets/pinball/prepare.py`,
  then `node scripts/build-pinball-assets.mjs`.
- Tables are procedural in code; there are no GLB, physics JSON or baked map files.
- Payloads remain below 20 MiB total and every file in `v1/` is listed in the manifest.
- All audio records retain their CC0 source, member, archive pin and delivered hash.

## Work Guidance

- Preserve ImageGen masters outside the payload tree. Use local URLs only.

## Verification

- `node scripts/test-pinball-assets.mjs`
- `node scripts/build-pinball-assets.mjs --check`
- Real browser captures and reference-hardware frame times complement static validation.

## Child DOX Index

None.
