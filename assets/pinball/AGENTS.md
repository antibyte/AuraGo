# AuraPinball production

## Purpose

Original ImageGen artwork and reproducible Blender tables for the built-in Pinball app.

## Ownership

- `masters/` retains the nine generated concepts, backglasses and flat playfield paintings; `prompts.json` records exact prompts and `artwork.json` records source hashes.
- `generate.py` and `kit/` build the three tables. Delivered GLBs, collision descriptions, textures, previews and audio live in `../../ui/3d/pinball/v1/`.
- Runtime ownership is `../../ui/js/desktop/apps/pinball/AGENTS.md`.

## Local Contracts

- Author in metres, using the same geometry for rendering and exported physics. Stable mechanism IDs connect contacts, rules and animated parts.
- Decorative meshes never become accidental colliders. Moving collision parts use convex shapes; static ramps use welded triangle meshes.
- Generated art contains no controls or localized instructions. Keep the lower playfield readable.
- Build from factory startup with Blender 5.2.2. Runtime needs neither Blender nor an external service.
- Retain source artwork and recipes; payload hashes must be reproducible. Blender previews are visual references rather than deterministic render outputs.
- `kit/glb.py` canonicalizes triangle order, five-decimal UVs, area-weighted vertex normals and shared accessor packing; collision geometry and winding stay unchanged.

## Work Guidance

- Reuse shared cabinet and mechanical builders while preserving distinct ramp layouts and hero models.
- Use the image generator for new paintings; normal/AO/roughness and emissive data belong to the Blender material pipeline.
- Keep production artifacts in this boundary and test captures in ignored `reports/pinball-implementation/`.

## Verification

- `blender --background --factory-startup --python-exit-code 1 --python assets/pinball/generate.py`
- `node scripts/test-pinball-assets.mjs`
- `npm run test:pinball`

## Child DOX Index

None.
