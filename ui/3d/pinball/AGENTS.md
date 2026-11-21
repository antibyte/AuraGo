# AuraPinball delivered tables

## Purpose

Versioned local payloads consumed by the Desktop Pinball app: the table artwork images, the observatory's
toy model, the shared CC0 sound samples and the integrity manifest (version 3).

## Ownership

- `v1/` contains the artwork for each table: a painted playfield underlay, a backglass, a printed plastics
  sheet, a side art image and a 640x320 lobby preview. All four lobby previews are real renders of the tables, not painted art. `v1/` also holds one room wall image shared by all tables, one GLB file
  (`observatory-toys.glb`), 29 CC0 OGG clips under `v1/audio/`, and the generated `v1/manifest.json`.
- Original sources, prompts, guides and the GLB build belong to `../../../assets/pinball/AGENTS.md`.
- Runtime behaviour belongs to `../../js/desktop/apps/pinball/AGENTS.md`.

## Payload contents

- Tables are procedural in code. The only model file is `observatory-toys.glb`, an unchanged copy of
  `assets/pinball/models/observatory-toys.glb`. The payload holds no other GLB, no physics JSON, no normal,
  AO or baked collision files, and no other Blender output.
- Manifest version 3 lists:
  - `tables`, in `TABLE_IDS` order: `id`, `underlay` (the painted playfield), `backglass`, `preview`, `artRev`
    (2 when the playfield was painted over a layout guide, otherwise 1), `plastics` and `side`, and `glb`
    (`{ "toys": "observatory-toys.glb" }`, on the observatory only). The apron has no image; the runtime draws it.
  - `room`: `{ "wall": "room-wall.webp" }`, the back wall of the room shared by all tables.
  - `files`: `path`, `bytes` and `sha256` for every file in `v1/`.
  - `audio`: each clip's `seconds`, `source`, `member` and `license`.
  - `sources`: the archives the clips came from (page, download URL, archive `file`, `bytes`, `sha256`,
    licence and licence URL).
- The legacy keys `model`, `physics`, `normal`, `ao` and `playfield` are not used.
- Image formats:
  - playfield: WebP at the master's size, never upscaled, quality 92;
  - backglass: WebP, at most 2048×1152, quality 92;
  - plastics: 1024×1536 lossy RGBA WebP with a lossless alpha plane, quality 92. Alpha below 40 is cleared
    and alpha 216 or above is made opaque before encoding, so the die-cut edges stay clean;
  - side: 2048×682 WebP, quality 90;
  - room wall: WebP, at most 2048×2048, quality 88;
  - preview: 640×320 WebP.
- GLB files are shipped only when `assets/pinball/models.json` lists them. The builder checks each source's
  sha256 against that record and copies the file unchanged. Limits: at most 1.5 MiB per file, 4 MiB for all
  GLB files together, and 40,000 triangles per file. The allowed extensions are `KHR_materials_emissive_strength`,
  `KHR_texture_transform`, `EXT_texture_webp`, `KHR_materials_transmission` and `KHR_materials_ior`. Draco,
  meshopt and basisu are not allowed.
- The observatory's `observatory-toys.glb` is about 0.45 MiB, and `models.json` records its triangle count.
- The audio clips are Kenney Impact Sounds and Music Jingles (CC0 1.0) and clips from five other CC0 archives,
  copied unchanged from the shared sample packs and cut at runtime. The record for each clip keeps its exact
  source member.

## Local Contracts

- Re-make the previews (`<id>-preview.webp`) from the repo root: run
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 AURAGO_PINBALL_SCENES="neon:preview:high,leviathan:preview:high,temple:preview:high" go test -run TestDesktopPinballSurvey -count=1 -timeout 20m ./ui`
  (the `preview` harness scene, 1280x640, spots in `PREVIEW_SPOTS` of `ui/testdata/pinball-harness.js`),
  find `reports/pinball-survey/<id>-preview-high.png`, then encode each with
  `ffmpeg -i <png> -vf scale=640:320:flags=lanczos -c:v libwebp -quality 82 ui/3d/pinball/v1/<id>-preview.webp`
  (keep each under about 120 KB) and rerun the asset builder.
- Do not hand-edit generated files. Rebuild the images with `py -3.13 assets/pinball/prepare.py` (Pillow is
  needed for the plastics), then rebuild the payload with `node scripts/build-pinball-assets.mjs`. A table
  is delivered once it is in `TABLE_IDS` (`ui/js/desktop/apps/pinball/config.js`) and its playfield exists.
- Every file in `v1/` is listed in the manifest with its hash, and no unlisted file is shipped.
- Payloads stay below 20 MiB in total.
- All audio records keep their CC0 source, member, archive pin and delivered hash.
- Use local URLs only. The runtime fetches nothing remote.

## Work Guidance

- Keep masters, prompts and source hashes outside the payload tree, in `assets/pinball/`.
- Adding a sample means adding it to `SAMPLE_FILES` in `ui/js/desktop/apps/pinball/audio/samples.js`,
  then running the builder, which copies it and records its provenance. Never copy it in by hand.

## Verification

- `node scripts/test-pinball-assets.mjs` (manifest version 3 and its tables, manifest hashes, no orphan
  files, CC0 audio, artwork and prompts, GLB rules, `pinball.*` keys in all sixteen locales)
- `node scripts/build-pinball-assets.mjs --check`
- Real browser captures and reference-hardware frame times complement static validation.

## Child DOX Index

None.
