# AuraPinball production

## Purpose

Original artwork for the built-in Pinball app, retained with its prompts and source hashes, and the Blender
script that builds the observatory's toy model.

## Ownership

- `masters/` retains 20 images:
  - `<id>-playfield.png`, `<id>-plastics.png`, `<id>-side.png` and `<id>-backglass.png` for each of the four
    tables (`neon`, `leviathan`, `temple`, `observatory`);
  - `<id>-concept.png` for the first three tables, kept as source art only;
  - `room-wall.png`, the back wall of the room, shared by all tables.
- `prompts.json` records the prompt of each image as `{ id, kind, prompt, generator, guide?, reference? }`,
  with the prompt copied verbatim from the prompt files. `artwork.json` records the source hash of each master
  and the `rev` of each playfield (2 when its prompt record has `guide: true`, otherwise 1).
- `models/observatory-toys.glb` is the only model. `models.json` records it as
  `{ table, key, file, sha256, triangles, script }`.
- `blender/observatory_toys.py` builds that GLB. The script is the source of the model.
- `prepare.py` converts the masters into the delivered WebP images in `../../ui/3d/pinball/v1/`. The concept
  masters are source art only and are not delivered; the lobby previews are renders of the real tables (see
  `../../ui/3d/pinball/AGENTS.md` for how to re-make them).
- `scripts/pinball-art-guide.mjs` draws the layout guides that the paintings are made over.
- Runtime ownership is `../../ui/js/desktop/apps/pinball/AGENTS.md`. Payload ownership is
  `../../ui/3d/pinball/AGENTS.md`.

## Local Contracts

- **Painting.** Each painted playfield, plastics sheet, side art, the observatory backglass and the room wall
  is made with the Codex CLI image tool (`generator: "codex-cli"`). A playfield or plastics sheet is painted
  over its guide: the guide PNG is the reference image named by the record's `reference`, and the record has
  `guide: true`. The concept and backglass masters of the first three tables were made with the built-in
  ImageGen and have no generator field.
- **Guides.** `node scripts/pinball-art-guide.mjs playfield <id|all>` writes `<id>-guide.svg` and `<id>-guide.png`,
  and `node scripts/pinball-art-guide.mjs plastics [--check] [--no-png]` writes `plastics-template.svg`,
  `.png` and `.json`. Both go to the git-ignored `reports/pinball-guide/`. `plastics --check` fails when `SHEET` in
  `ui/js/desktop/apps/pinball/tables/kit/plastics.js` differs from the template.
- **Prompts and masters.** Record each new prompt in `prompts.json` and its source hash in `artwork.json`
  before preparing the payload. When a master is replaced, its prompt record and `artwork.json` entry are
  replaced with it. Retain every master in this folder: the delivered images must be reproducible from these
  masters through `prepare.py`.
- **prepare.py.** Run it with `py -3.13` (it needs Pillow). It converts whichever masters exist:
  - playfield: WebP at the master's size, never upscaled, quality 92;
  - backglass: WebP, at most 2048×1152, quality 92;
  - plastics: 1024×1536 lossy RGBA WebP, quality 92, encoded with Pillow with `alpha_quality` 100. Before
    encoding, alpha below 40 becomes 0 and alpha 216 or above becomes 255;
  - side: 2048×682 WebP, quality 90;
  - room wall: WebP, at most 2048×2048, quality 88.
  It records every master in `artwork.json`. It delivers only tables listed in `TABLE_IDS`
  (`ui/js/desktop/apps/pinball/config.js`); `--all` delivers every table. After a table joins `TABLE_IDS`,
  run `prepare.py` and then the builder again.
- **Observatory toy model.** Blender is needed only to rebuild the GLB. `blender/observatory_toys.py` runs in
  Blender's background mode and exports `models/observatory-toys.glb` as glTF with +Y up and transforms
  applied. No `.blend` file is kept, and `build/` is git-ignored. Each toy is one root node named after its
  kind, with its origin at the toy's base point. The procedural fallbacks in
  `ui/js/desktop/apps/pinball/tables/observatory/toys.js` use the same node names (the node contract is in
  `ui/js/desktop/apps/pinball/AGENTS.md`). Units are metres: Blender X is table x, Blender Y is table y and
  Z is height. After a rebuild, update `sha256` and `triangles` in `models.json`, then run
  `node scripts/build-pinball-assets.mjs`, which fails when a source's hash does not match `models.json`.
- **Content.** Generated art contains no controls and no localized instructions. Keep the lower playfield
  readable, because the game draws its own inserts, labels and lamps on top.
- Runtime needs neither Blender nor any external service.

## Work Guidance

- Use the Codex CLI image tool for new paintings, over a guide from `scripts/pinball-art-guide.mjs`.
- Keep test captures out of this folder. Survey screenshots go to `reports/pinball-survey/` and audio
  renders to `reports/pinball-audio/`. Both are git-ignored.

## Verification

- `node scripts/test-pinball-assets.mjs` (manifest version 3, artwork records and prompts, delivered images
  match the manifest, GLB rules against `models.json`, `pinball.*` keys in all sixteen locales)
- `node scripts/pinball-art-guide.mjs plastics --check`
- `node scripts/build-pinball-assets.mjs --check`
- `npm run test:pinball`

## Child DOX Index

None.
