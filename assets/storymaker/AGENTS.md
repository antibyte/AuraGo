# Storymaker artwork

## Purpose

ImageGen-authored illustration masters and the deterministic built-in Storymaker catalog. Runtime ownership belongs to `../../ui/js/storymaker/AGENTS.md`; catalog validation and export belong to `../../internal/storymaker/AGENTS.md`.

## Ownership

- `masters/`: original generated images, retained without destructive edits.
- `production.json`: prompts, source paths, references and production specifications.
- `lib/`: catalog assembly, names, skeleton and motion data.
- `test/`: catalog, geometry, animation and provenance checks.
- Generated delivery files belong to `../../ui/img/storymaker/`.
- `ASSET-CONTRACT.md` owns the concrete delivery schema, bounds, production sequence and rights records.

## Local Contracts

- Use the agreed illustrated, textured style with softly modeled depth. Characters are fixed identities, not freely interchangeable vector parts.
- Keep all four existing character IDs, four environment IDs, 23 prop IDs and all pose, gesture, expression and mouth names compatible.
- The target library is twelve characters, twelve environments and eighty props. Build-time image generation is a production step only; ordinary builds must use committed masters without network access.
- Preserve alpha, calibrated pivots, view-specific draw order and joint overlap. Atlas packing must not invent artwork or silently substitute vector placeholders.
- `lib/raster-attachments.mjs` owns measured segment placement and wrist sizing. Validate opaque joint coverage and inspect high-resolution pose frames; small contact-sheet thumbnails alone do not establish rig quality. Keep source pixels intact when correcting placement.
- Catalog v2 names every deliverable dependency, including image dimensions, byte size and SHA-256. Maximum atlas edge: 2048; background: 3840 x 2160; delivered catalog: 256 MiB.
- Keep prompts and reference relationships with production sources; never claim a generated turnaround is a completed rig.

## Work Guidance

- Generate and inspect a reference before producing a related rig or environment variant.
- Review standing, walking, sitting, speaking and strong gestures in motion; inspect hands, feet, facial swaps and joint seams.
- Retain accepted image masters. Pack/copy them deterministically; do not regenerate images during build or test.

## Verification

- `npm run test:storymaker`.
- `node scripts/build-storymaker-catalog.mjs --check`.
- `go test ./ui -run TestStorymakerCatalog -count=1`.

## Child DOX Index

- `ASSET-CONTRACT.md`: catalog v2 attachments, inventories, bitmap production and validation.
