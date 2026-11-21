# Storymaker artwork

## Purpose

Offline production of three articulated Desktop-pet characters, illustrated environments and props. Runtime: ../../ui/js/storymaker/AGENTS.md; validation/export: ../../internal/storymaker/AGENTS.md.

## Ownership

- masters/ and production.json: retained ImageGen environments/props, prompts, references and rendered previews.
- models/: derived GLBs, authored motions and provenance. Original Desktop pets stay under ui/3d/pets; their owner is ../desktop-pets/AGENTS.md.
- lib/model-characters.mjs, model-face.mjs, model-motions.mjs: selected identities, old-ID aliases, facial landmarks/morphs and explainer poses.
- lib/ and test/: deterministic catalog assembly, names, shared motion vocabulary and checks.
- Delivery: ../../ui/img/storymaker/. ASSET-CONTRACT.md owns schemas, bounds and workflow.

## Local Contracts

- Deliver Hoodie, Professor and Punk, twelve day/night environments and eighty props. Human bitmap sprites are retired. Never reintroduce them as a fallback or modify source Desktop pets.
- Preserve storymaker/1, all twelve old character IDs through aliases, and pose/gesture/expression/mouth names. Aliases resolve saved stories but never add library entries.
- Mouths are actual openings in the face mesh, with attached textured lips, recessed oral cavities, teeth and tongues. Keep matching skin weights at the cut boundary; never project a dark mouth plate onto closed skin. Speech deforms the lips/lower jaw; it never swaps heads or translates the skull. Review hands, proportions, sitting and walking in rendered motion at display size.
- Model production is deterministic and offline. Keep source hashes, copied notices and generator hashes. Geometry and motion rights remain distinct. Builds never invoke providers.
- The delivered model notices and licenses use LF on every checkout so the catalog hashes match on Windows.
- Inventory every delivery dependency with exact size/hash and image dimensions. Total budget: 256 MiB; per-file limit: 8 MiB. Model bounds belong to ASSET-CONTRACT.md.

## Work Guidance

- Reuse accepted pets as separate derivatives; author poses/mouth shapes against each actual skeleton and face.
- Calibrate each closed lip line against its original painted mouth. Reproject existing face colors for new lip triangles rather than interpolating across source UV islands; keep the original identity at rest.
- Retain accepted ImageGen environment/prop originals and prompts. Regenerate models, catalog and actual player previews together after changes.
- Inspect moving full figures and face close-ups, not only thumbnails.

## Verification

- node scripts/build-storymaker-models.mjs --check
- node scripts/build-storymaker-catalog.mjs --check
- npm run test:storymaker
- go test ./ui -run TestStorymakerCatalog -count=1
- Browser/MP4 procedures: ASSET-CONTRACT.md.

## Child DOX Index

- ASSET-CONTRACT.md: model/image inventory, bounds, provenance and production verification.
