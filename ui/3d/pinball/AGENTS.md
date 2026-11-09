# AuraPinball delivered tables

## Purpose

Versioned local payloads consumed by the Desktop Pinball app: the table artwork images, the shared
CC0 sound samples and the integrity manifest (version 2).

## Ownership

- `v1/` contains nine WebP images (one flat playfield underlay and one backglass per table, plus one
  640x320 lobby preview per table that is a real render of the procedural table, not painted art), thirteen CC0 OGG clips under `v1/audio/`, and the generated `v1/manifest.json`.
- Original sources, prompts and recipes belong to `../../../assets/pinball/AGENTS.md`.
- Runtime behaviour belongs to `../../js/desktop/apps/pinball/AGENTS.md`.

## Payload contents

- Tables are procedural in code. The payload contains no GLB, physics JSON, normal, AO or baked
  collision files, and no Blender output.
- Manifest version 2 lists `tables` (`id`, `backglass`, `preview`, `underlay`), `files` (`path`,
  `bytes`, `sha256` for every file in `v1/`), `audio` (each clip's `seconds`, `source`, `member`,
  `license`) and `sources` (the Kenney archives with page, download URL, archive `file`, `bytes`,
  `sha256`, licence and licence URL).
- The audio clips are Kenney Impact Sounds and Music Jingles (CC0 1.0), copied unchanged from the
  shared sample packs and cut at runtime. The record for each clip keeps its exact source member.

## Local Contracts

- Re-make the previews (`<id>-preview.webp`) from the repo root: run
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 AURAGO_PINBALL_SCENES="neon:preview:high,leviathan:preview:high,temple:preview:high" go test -run TestDesktopPinballSurvey -count=1 -timeout 20m ./ui`
  (the `preview` harness scene, 1280x640, spots in `PREVIEW_SPOTS` of `ui/testdata/pinball-harness.js`),
  find `reports/pinball-survey/<id>-preview-high.png`, then encode each with
  `ffmpeg -i <png> -vf scale=640:320:flags=lanczos -c:v libwebp -quality 82 ui/3d/pinball/v1/<id>-preview.webp`
  (keep each under about 120 KB) and rerun the asset builder.
- Do not hand-edit generated files. Rebuild artwork with `assets/pinball/prepare.py`, then rebuild
  the payload with `node scripts/build-pinball-assets.mjs`.
- Every file in `v1/` is listed in the manifest with its hash, and no unlisted file is shipped.
- Payloads stay below 20 MiB in total.
- All audio records keep their CC0 source, member, archive pin and delivered hash.
- Use local URLs only. The runtime fetches nothing remote.

## Work Guidance

- Keep ImageGen masters, prompts and source hashes outside the payload tree, in `assets/pinball/`.
- Adding a sample means adding it to `SAMPLE_FILES` in `ui/js/desktop/apps/pinball/audio/samples.js`,
  then running the builder, which copies it and records its provenance. Never copy it in by hand.

## Verification

- `node scripts/test-pinball-assets.mjs` (manifest hashes, no orphan files, CC0 audio, artwork and
  prompts, `pinball.*` keys in all sixteen locales)
- `node scripts/build-pinball-assets.mjs --check`
- Real browser captures and reference-hardware frame times complement static validation.

## Child DOX Index

None.
