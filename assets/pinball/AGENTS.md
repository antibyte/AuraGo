# AuraPinball production

## Purpose

Original ImageGen artwork for the built-in Pinball app.

## Ownership

- `masters/` retains the nine generated concepts, backglasses and flat playfield paintings; `prompts.json` records exact prompts and `artwork.json` records source hashes.
- `prepare.py` converts the masters into the delivered WebP images in `../../ui/3d/pinball/v1/`.
- Runtime ownership is `../../ui/js/desktop/apps/pinball/AGENTS.md`.

## Local Contracts

- Generated art contains no controls or localized instructions. Keep the lower playfield readable.
- Runtime needs neither Blender nor an external service; tables are procedural in code.
- Retain source artwork and recipes; payload hashes must be reproducible.

## Work Guidance

- Use the image generator for new paintings.
- Keep test captures in ignored `reports/pinball-implementation/`.

## Verification

- `node scripts/test-pinball-assets.mjs`
- `npm run test:pinball`

## Child DOX Index

None.
