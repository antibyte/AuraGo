# AuraPinball production

## Purpose

Original ImageGen artwork for the built-in Pinball app, retained with its prompts and source hashes.

## Ownership

- `masters/` retains the nine generated images: a concept, a backglass and a flat playfield painting per
  table (`neon-*`, `leviathan-*`, `temple-*`). `prompts.json` records the exact prompts and `artwork.json`
  records the source hashes.
- `prepare.py` converts the backglass and playfield masters into the delivered WebP images in
  `../../ui/3d/pinball/v1/`. The concept masters are source art only and are not delivered: the lobby
  previews are renders of the real tables (see `../../ui/3d/pinball/AGENTS.md` for how to re-make them).
- Runtime ownership is `../../ui/js/desktop/apps/pinball/AGENTS.md`. Payload ownership is
  `../../ui/3d/pinball/AGENTS.md`.

## Local Contracts

- There is no Blender pipeline. Tables, cabinet, ramps, wires, flippers, toys and collision surfaces are
  built procedurally in code, so no `.blend`, GLB, physics JSON or generator script belongs here.
- Generated art contains no controls and no localized instructions. Keep the lower playfield readable,
  because the game draws its own inserts, labels and lamps on top.
- Retain every source master in this folder. Payload hashes in the delivered images must be reproducible
  from these masters through `prepare.py`.
- Runtime needs neither Blender nor any external service.

## Work Guidance

- Use the image generator for new paintings, and record each new prompt in `prompts.json` and its source
  hash in `artwork.json` before preparing the payload.
- Keep test captures out of this folder. Survey screenshots go to `reports/pinball-survey/` and audio
  renders to `reports/pinball-audio/`. Both are git-ignored.

## Verification

- `node scripts/test-pinball-assets.mjs` (nine artworks with prompts and hashes, delivered images match the
  manifest)
- `npm run test:pinball`

## Child DOX Index

None.
